package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/upgaming/roaster/internal/config"
	"github.com/upgaming/roaster/internal/roast"
)

// stub answers instantly, so these tests are about the routes rather than about
// how long an audit takes.
type stub struct{}

func (stub) Roast(ctx context.Context, req roast.Request, emit func(roast.Update)) (roast.Roast, error) {
	emit(roast.Update{Phase: roast.PhaseFetch, Label: "Reading public commits"})
	r := roast.Sample(req.Handle)
	r.Code = "abc12"
	emit(roast.Update{Phase: roast.PhaseDone, Roast: &r})
	return r, nil
}

func testServer(t *testing.T) http.Handler {
	t.Helper()

	cfg := config.Defaults()
	cfg.EventsDir = ""    // the events compiled into the binary
	cfg.AdminPIN = "1379" // the code these tests type
	cfg.Printer = ""      // no hardware to open

	s, err := New(Options{
		Config:   cfg,
		Saved:    cfg,
		Path:     filepath.Join(t.TempDir(), "roaster.json"),
		Provider: stub{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s.Handler()
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestTheStandAnswers(t *testing.T) {
	h := testServer(t)

	if w := get(t, h, "/health"); w.Code != http.StatusOK {
		t.Errorf("/health is what the launcher polls, got %d", w.Code)
	} else {
		var health struct{ OK bool }
		if err := json.Unmarshal(w.Body.Bytes(), &health); err != nil || !health.OK {
			t.Errorf("/health should say it is ok, got %q", w.Body.String())
		}
	}

	w := get(t, h, "/kiosk")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<!doctype html>") {
		t.Errorf("/kiosk should render the page, got %d", w.Code)
	}

	if w := get(t, h, "/"); w.Code != http.StatusFound || w.Header().Get("Location") != "/kiosk" {
		t.Errorf("the root should land on the kiosk, got %d to %q", w.Code, w.Header().Get("Location"))
	}

	if w := get(t, h, "/preview"); w.Code != http.StatusOK {
		t.Errorf("/preview is the designer, got %d", w.Code)
	}
}

// An audit has to reach the page as it happens, and what it produces has to be
// printable afterwards without sending the document back and forth.
func TestAnAuditStreamsAndLeavesSomethingToPrint(t *testing.T) {
	h := testServer(t)

	w := get(t, h, "/api/roast?handle=nmbrthirteen")
	if w.Code != http.StatusOK {
		t.Fatalf("the audit should stream, got %d", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("the page reads this as events, got %q", got)
	}
	for _, phase := range []string{roast.PhaseFetch, roast.PhaseDone} {
		if !strings.Contains(w.Body.String(), `"phase":"`+phase+`"`) {
			t.Errorf("the stream is missing the %s phase", phase)
		}
	}

	if w := get(t, h, "/receipt.bin?code=abc12"); w.Code != http.StatusOK || w.Body.Len() == 0 {
		t.Errorf("the finished roast should be printable, got %d and %d bytes", w.Code, w.Body.Len())
	}
	if w := get(t, h, "/receipt.bin?code=nosuch"); w.Code != http.StatusNotFound {
		t.Errorf("a roast nobody ran should be a 404, got %d", w.Code)
	}
}

func TestTheCodeGuardsEverythingThatChangesTheDevice(t *testing.T) {
	h := testServer(t)

	guarded := []string{
		"/admin/state", "/admin/settings", "/admin/reprint", "/admin/reboot",
		"/printer", "/print", "/print/test", "/event", "/events",
	}
	for _, path := range guarded {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		if w.Code != http.StatusForbidden {
			t.Errorf("%s answered %d without the code", path, w.Code)
		}
	}

	// The visitor's own receipt is the one printing route with no code on it.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/receipt/print", nil))
	if w.Code == http.StatusForbidden {
		t.Errorf("a visitor printing their own receipt should not need the code")
	}
}

func TestTheMenuOpensWithTheCode(t *testing.T) {
	h := testServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/state", nil)
	req.Header.Set("X-Admin-Pin", "1379")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("the menu should open with the code, got %d", w.Code)
	}
	var state adminState
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatalf("the menu reads this as JSON: %v", err)
	}
	if state.Terminal != config.Defaults().Terminal {
		t.Errorf("the menu should report the terminal, got %q", state.Terminal)
	}
	if len(state.Events) == 0 {
		t.Errorf("the menu should list the events it can switch to")
	}
}

// A settings route that only takes POST must say so rather than half-working.
func TestChangingSettingsIsAPost(t *testing.T) {
	h := testServer(t)

	req := httptest.NewRequest(http.MethodGet, "/printer", nil)
	req.Header.Set("X-Admin-Pin", "1379")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("a GET should be turned away, got %d", w.Code)
	}
}
