package audit

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/verdict"
)

// numbered writes a different line every time, and keeps what it was told to
// avoid, so tests can see what the model would have been shown.
type numbered struct {
	mu     sync.Mutex
	n      int
	avoids [][]string
}

func (w *numbered) Write(ctx context.Context, b verdict.Brief) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.n++
	w.avoids = append(w.avoids, b.Avoid)
	return fmt.Sprintf("Line number %d.", w.n), nil
}

func (w *numbered) lastAvoid() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.avoids[len(w.avoids)-1]
}

func contains(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}

// A repeat costs GitHub nothing, and still gets a joke it has not heard.
func TestARepeatIsReadOnceAndToldSomethingNew(t *testing.T) {
	r := &reader{facts: account()}
	w := &numbered{}
	a := Audit{GitHub: r, Writer: w, Memory: NewMemory(time.Minute, 100)}

	first, _, err := run(t, a, "nmbrthirteen")
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := run(t, a, "@NmbrThirteen")
	if err != nil {
		t.Fatal(err)
	}

	if n := r.asked.Load(); n != 1 {
		t.Errorf("GitHub was read %d times for one account, want once", n)
	}
	if first.Verdict == second.Verdict {
		t.Errorf("a repeat got the same verdict: %q", first.Verdict)
	}
	if !contains(w.lastAvoid(), first.Verdict) {
		t.Errorf("the model should have been told what this account heard last time")
	}
	if second.Score != first.Score {
		t.Errorf("the numbers should not change between visits a minute apart")
	}
}

// The next person in the queue has read the last receipt.
func TestTheQueueIsNotToldTheSameJoke(t *testing.T) {
	facts := account()
	w := &numbered{}
	a := Audit{GitHub: &reader{facts: facts}, Writer: w, Memory: NewMemory(time.Minute, 100)}

	earlier, _, err := run(t, a, "nmbrthirteen")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := run(t, a, "somebody-else"); err != nil {
		t.Fatal(err)
	}
	if !contains(w.lastAvoid(), earlier.Verdict) {
		t.Errorf("the model should have been told what the queue just read")
	}
}

// Five stands typing the speaker's handle at once cost one GitHub read.
func TestACrowdTypingOneHandleCostsOneRead(t *testing.T) {
	r := &reader{facts: account(), release: make(chan struct{})}
	a := Audit{GitHub: r, Writer: &numbered{}, Memory: NewMemory(time.Minute, 100)}

	var wg sync.WaitGroup
	var failed atomic.Int32
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := a.Roast(context.Background(), roast.Request{Handle: "torvalds"}, func(roast.Update) {})
			if err != nil {
				failed.Add(1)
			}
		}()
	}
	time.Sleep(50 * time.Millisecond) // let them all arrive
	close(r.release)
	wg.Wait()

	if failed.Load() != 0 {
		t.Errorf("%d of the five failed", failed.Load())
	}
	if n := r.asked.Load(); n != 1 {
		t.Errorf("GitHub was read %d times, want once", n)
	}
}

func TestAFailureIsNotKept(t *testing.T) {
	r := &reader{err: fmt.Errorf("timeout")}
	a := Audit{GitHub: r, Memory: NewMemory(time.Minute, 100)}

	run(t, a, "flaky")
	run(t, a, "flaky")
	if n := r.asked.Load(); n != 2 {
		t.Errorf("a failed read was remembered: GitHub asked %d times", n)
	}
}

// An account is read again once its time is up, and what it was told before
// survives the re-read.
func TestMemoryForgetsTheAccountButNotWhatItSaid(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	m := NewMemory(10*time.Minute, 100)
	m.now = func() time.Time { return now }

	r := &reader{facts: account()}
	w := &numbered{}
	a := Audit{GitHub: r, Writer: w, Memory: m}

	first, _, _ := run(t, a, "nmbrthirteen")
	now = now.Add(11 * time.Minute)
	run(t, a, "nmbrthirteen")

	if n := r.asked.Load(); n != 2 {
		t.Errorf("an expired account should be read again, GitHub asked %d times", n)
	}
	if !contains(w.lastAvoid(), first.Verdict) {
		t.Errorf("the re-read forgot what this account was told before")
	}
}

func TestMemoryNeverOutgrowsItsCap(t *testing.T) {
	m := NewMemory(time.Hour, 3)
	a := Audit{GitHub: &reader{facts: account()}, Memory: m}

	for i := range 10 {
		if _, _, err := run(t, a, fmt.Sprintf("handle-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if n := len(m.accounts); n > 3 {
		t.Errorf("memory grew to %d accounts, past its cap of 3", n)
	}
}

// What the model is shown to avoid stays short, however busy the day.
func TestTheAvoidListStaysShort(t *testing.T) {
	w := &numbered{}
	a := Audit{GitHub: &reader{facts: account()}, Writer: w, Memory: NewMemory(time.Hour, 100)}

	for i := range 40 {
		run(t, a, fmt.Sprintf("handle-%d", i%5))
	}
	if n := len(w.lastAvoid()); n > keepRecent+keepSaid {
		t.Errorf("the model was shown %d lines to avoid, past %d", n, keepRecent+keepSaid)
	}
}
