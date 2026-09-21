package audit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/upgaming/roaster/internal/github"
	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/verdict"
)

type reader struct {
	facts   github.Facts
	err     error
	asked   atomic.Int32
	release chan struct{} // nil: answer at once
}

func (r *reader) Read(ctx context.Context, handle string) (github.Facts, error) {
	r.asked.Add(1)
	if r.release != nil {
		<-r.release
	}
	return r.facts, r.err
}

type writer struct {
	line  string
	err   error
	delay time.Duration
}

func (w writer) Write(ctx context.Context, b verdict.Brief) (string, error) {
	select {
	case <-time.After(w.delay):
		return w.line, w.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func account() github.Facts {
	zone := time.FixedZone("GST", 4*60*60)
	return github.Facts{
		Handle:  "nmbrthirteen",
		Created: time.Date(2016, 4, 2, 0, 0, 0, 0, time.UTC),
		Owned:   3,
		Repos:   []github.Repo{{Name: "roaster", Language: "Go"}},
		Commits: []github.Commit{
			{Message: "fix", At: time.Date(2026, 9, 19, 2, 0, 0, 0, zone)},
			{Message: "fix", At: time.Date(2026, 9, 20, 3, 0, 0, 0, zone)},
			{Message: "wip", At: time.Date(2026, 9, 21, 4, 0, 0, 0, zone)},
		},
	}
}

func run(t *testing.T, a Audit, handle string) (roast.Roast, []roast.Update, error) {
	t.Helper()
	var updates []roast.Update
	r, err := a.Roast(context.Background(), roast.Request{Handle: handle}, func(u roast.Update) {
		updates = append(updates, u)
	})
	return r, updates, err
}

// The page animates each stage as it lands, in this order, and the done update
// is what it prints from.
func TestAnAuditArrivesInOrder(t *testing.T) {
	a := Audit{GitHub: &reader{facts: account()}, Writer: writer{line: "You commit at 3am and it shows."}}
	r, updates, err := run(t, a, "@nmbrthirteen")
	if err != nil {
		t.Fatal(err)
	}

	var phases []string
	for _, u := range updates {
		phases = append(phases, u.Phase)
	}
	want := "fetch section section section section feed metric metric metric metric metric metric verdict verdict done"
	if got := strings.Join(phases, " "); got != want {
		t.Errorf("phases\n got %s\nwant %s", got, want)
	}

	if r.Verdict != "You commit at 3am and it shows." {
		t.Errorf("the verdict should be the one written, got %q", r.Verdict)
	}
	if r.Code == "" || r.Handle != "nmbrthirteen" || len(r.Actions) != 3 {
		t.Errorf("the roast is missing parts it prints: %+v", r)
	}
	if last := updates[len(updates)-1]; last.Roast == nil || last.Roast.Code != r.Code {
		t.Errorf("the done update should carry the finished roast")
	}
}

// However the verdict fails, the receipt still prints, and what it says is
// taken from the numbers so it is still true.
func TestTheVerdictAlwaysHasAWayThrough(t *testing.T) {
	for name, w := range map[string]verdict.Writer{
		"no writer":   nil,
		"an outage":   writer{err: errors.New("503")},
		"a refusal":   writer{err: verdict.ErrDeclined},
		"unprintable": writer{err: verdict.ErrUnusable},
		"too slow":    writer{line: "late", delay: time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			a := Audit{GitHub: &reader{facts: account()}, Writer: w, Timeout: 50 * time.Millisecond}
			r, updates, err := run(t, a, "nmbrthirteen")
			if err != nil {
				t.Fatalf("a verdict failing should not fail the audit: %v", err)
			}
			if r.Verdict == "" || r.Verdict == "late" {
				t.Errorf("want a verdict written from the numbers, got %q", r.Verdict)
			}
			if updates[len(updates)-1].Phase != roast.PhaseDone {
				t.Errorf("the audit should still finish")
			}
		})
	}
}

func TestAnAccountThatDoesNotExistIsSaidPlainly(t *testing.T) {
	a := Audit{GitHub: &reader{err: fmt.Errorf("%w: @ghost", github.ErrNoAccount)}}
	_, updates, err := run(t, a, "ghost")
	if err == nil || !strings.Contains(err.Error(), "no GitHub account called @ghost") {
		t.Errorf("got %v", err)
	}
	for _, u := range updates {
		if u.Phase == roast.PhaseMetric {
			t.Errorf("nothing should be measured for an account that is not there")
		}
	}
}

// An opted-out account is never read, not just never printed.
func TestAnOptedOutAccountIsNeverRead(t *testing.T) {
	r := &reader{facts: account()}
	a := Audit{GitHub: r, Blocked: map[string]bool{"nmbrthirteen": true}}

	_, _, err := run(t, a, "@NmbrThirteen")
	if !errors.Is(err, ErrOptedOut) {
		t.Errorf("want ErrOptedOut, got %v", err)
	}
	if r.asked.Load() != 0 {
		t.Errorf("GitHub was asked about an account that opted out")
	}
}

// What went wrong goes to the log. The visitor gets a line they can act on.
func TestAGitHubFailureIsWordedForTheVisitor(t *testing.T) {
	a := Audit{GitHub: &reader{err: errors.New("dial tcp 140.82.112.6:443: i/o timeout")}}
	_, _, err := run(t, a, "nmbrthirteen")
	if !errors.Is(err, ErrGitHub) {
		t.Errorf("want ErrGitHub, got %v", err)
	}
	if strings.Contains(err.Error(), "140.82") {
		t.Errorf("network detail should not reach the screen: %v", err)
	}
}

// A handle that cannot exist is turned away before the screen claims to be
// reading anything, and before GitHub is asked.
func TestABadHandleIsTurnedAwayBeforeAnythingHappens(t *testing.T) {
	r := &reader{facts: account()}
	_, updates, err := run(t, Audit{GitHub: r}, "not a handle")
	if !errors.Is(err, github.ErrBadHandle) {
		t.Errorf("want a bad-handle error, got %v", err)
	}
	if len(updates) != 0 {
		t.Errorf("nothing should reach the screen for a handle that cannot exist, got %v", updates)
	}
	if r.asked.Load() != 0 {
		t.Errorf("GitHub was asked about a handle that cannot exist")
	}
}

// GitHub gives commit times in UTC, so the hour is read on the stand's clock.
func TestCommitHoursAreReadOnTheStandsClock(t *testing.T) {
	night := func(offset int) string {
		f := account()
		for i := range f.Commits {
			f.Commits[i].At = f.Commits[i].At.UTC()
		}
		r, err := Audit{GitHub: &reader{facts: f}}.Roast(context.Background(),
			roast.Request{Handle: "nmbrthirteen", Offset: offset}, func(roast.Update) {})
		if err != nil {
			t.Fatal(err)
		}
		return r.Metrics[0].Value
	}
	if got := night(4 * 60 * 60); got != "100%" {
		t.Errorf("at +04:00 all three commits are after midnight, got %s", got)
	}
	// 22:00, 23:00 and 00:00 in UTC: one of three.
	if got := night(0); got != "33%" {
		t.Errorf("in UTC only the last one is, got %s", got)
	}
}
