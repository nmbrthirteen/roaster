package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/upgaming/roaster/internal/audit"
	"github.com/upgaming/roaster/internal/roast"
)

const token = "test-terminal-token-not-a-real-one"

// provider stands in for the audit. It counts what it was asked, and can be
// held open until the test lets it go.
type provider struct {
	calls    atomic.Int32
	finished atomic.Int32
	err      error
	release  chan struct{} // nil: answer at once
	started  chan struct{} // closed when the first audit begins
	once     sync.Once
}

func (p *provider) Roast(ctx context.Context, req roast.Request, emit func(roast.Update)) (roast.Roast, error) {
	p.calls.Add(1)
	if p.started != nil {
		p.once.Do(func() { close(p.started) })
	}
	emit(roast.Update{Phase: roast.PhaseFetch, Label: "Reading public commits"})
	if p.release != nil {
		<-p.release
	}
	if p.err != nil {
		return roast.Roast{}, p.err
	}
	r := roast.Sample(req.Handle)
	r.Code = roast.Code()
	p.finished.Add(1)
	emit(roast.Update{Phase: roast.PhaseVerdict, Verdict: r.Verdict})
	emit(roast.Update{Phase: roast.PhaseDone, Roast: &r})
	return r, nil
}

func roomy() limits {
	return limits{slots: 4, queue: time.Second, budget: 5 * time.Second, every: time.Millisecond, burst: 100}
}

func serve(t *testing.T, p roast.Provider, lim limits) string {
	t.Helper()
	srv := httptest.NewServer(newService(p, []string{token}, lim).handler())
	t.Cleanup(srv.Close)
	return srv.URL
}

// stand is the kiosk's own client, so these tests prove the two ends of the
// wire agree rather than that the service agrees with itself.
func stand(url string) roast.Remote {
	return roast.Remote{URL: url + "/roast", Token: token}
}

func ask(t *testing.T, url, handle string) (roast.Roast, []roast.Update, error) {
	t.Helper()
	var updates []roast.Update
	r, err := stand(url).Roast(context.Background(), roast.Request{Handle: handle}, func(u roast.Update) {
		updates = append(updates, u)
	})
	return r, updates, err
}

func TestTheKiosksOwnClientReadsTheStream(t *testing.T) {
	url := serve(t, &provider{}, roomy())

	r, updates, err := ask(t, url, "nmbrthirteen")
	if err != nil {
		t.Fatal(err)
	}
	if r.Code == "" || r.Verdict == "" {
		t.Errorf("the stand should end up with a printable roast, got %+v", r)
	}
	if len(updates) < 2 || updates[0].Phase != roast.PhaseFetch {
		t.Errorf("the stand should see the audit as it happens, got %d updates", len(updates))
	}
}

func TestHealthNeedsNoToken(t *testing.T) {
	url := serve(t, &provider{}, roomy())
	res, err := http.Get(url + "/health")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("a load balancer checks this with no credentials, got %d", res.StatusCode)
	}
}

func TestAnUnknownStandIsTurnedAway(t *testing.T) {
	p := &provider{}
	url := serve(t, p, roomy())

	for name, header := range map[string]string{
		"no token":    "",
		"wrong token": "Bearer test-terminal-token-guessed-wrong",
		"not bearer":  "Basic " + token,
	} {
		req, _ := http.NewRequest(http.MethodPost, url+"/roast", strings.NewReader(`{"handle":"a"}`))
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", name, res.StatusCode)
		}
	}
	if p.calls.Load() != 0 {
		t.Errorf("an unknown stand cost an audit")
	}
}

func TestARequestWithNoHandleIsRefused(t *testing.T) {
	url := serve(t, &provider{}, roomy())
	req, _ := http.NewRequest(http.MethodPost, url+"/roast", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("got %d, want 400", res.StatusCode)
	}
}

// A visitor who walks away mid-audit is usually back a second later. The audit
// runs to the end anyway, so their account is already read when they return.
func TestAVisitorLeavingDoesNotStopTheAudit(t *testing.T) {
	p := &provider{release: make(chan struct{}), started: make(chan struct{})}
	url := serve(t, p, roomy())

	ctx, leave := context.WithCancel(context.Background())
	go stand(url).Roast(ctx, roast.Request{Handle: "torvalds"}, func(roast.Update) {})
	<-p.started
	leave()
	time.Sleep(50 * time.Millisecond) // the server notices the visitor has gone
	close(p.release)

	deadline := time.Now().Add(2 * time.Second)
	for p.finished.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the audit was abandoned when the visitor left")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAStandGoingTooFastIsToldToSlowDown(t *testing.T) {
	lim := roomy()
	lim.every, lim.burst = time.Hour, 2
	url := serve(t, &provider{}, lim)

	for _, h := range []string{"a1", "a2"} {
		if _, _, err := ask(t, url, h); err != nil {
			t.Fatalf("within the burst: %v", err)
		}
	}
	_, _, err := ask(t, url, "a3")
	if err == nil || !strings.Contains(err.Error(), "going faster") {
		t.Errorf("want a slow-down message the stand can show, got %v", err)
	}
}

func TestAFullServiceSaysBusyRatherThanHanging(t *testing.T) {
	lim := roomy()
	lim.slots, lim.queue = 1, 50*time.Millisecond
	p := &provider{release: make(chan struct{}), started: make(chan struct{})}
	url := serve(t, p, lim)
	t.Cleanup(func() { close(p.release) })

	go ask(t, url, "holds-the-slot")
	<-p.started

	_, _, err := ask(t, url, "someone-else")
	if err == nil || !strings.Contains(err.Error(), "busy") {
		t.Errorf("want a busy message the stand can show, got %v", err)
	}
}

func TestAnAuditErrorReachesTheStandInWords(t *testing.T) {
	url := serve(t, &provider{err: audit.ErrOptedOut}, roomy())

	_, _, err := ask(t, url, "someone")
	if err == nil || err.Error() != audit.ErrOptedOut.Error() {
		t.Errorf("want %q, got %v", audit.ErrOptedOut, err)
	}
}
