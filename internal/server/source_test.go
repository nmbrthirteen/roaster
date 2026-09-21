package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/upgaming/roaster/internal/config"
)

// A stand set to a live service it cannot reach must still come up, tell the
// visitor something they can act on, and tell the menu what to fix.
func TestAStandWithNoServiceSaysSoInsteadOfDying(t *testing.T) {
	cfg := config.Defaults()
	cfg.EventsDir = ""
	cfg.AdminPIN = "1379"
	cfg.Provider = "remote"

	s, err := New(Options{Config: cfg, Saved: cfg, Path: filepath.Join(t.TempDir(), "roaster.json")})
	if err != nil {
		t.Fatalf("the stand should start without a roast service, got %v", err)
	}
	h := s.Handler()

	w := get(t, h, "/api/roast?handle=octocat")
	if !strings.Contains(w.Body.String(), "not set up to roast yet") {
		t.Errorf("the visitor should be told the stand is not set up, got %q", w.Body.String())
	}

	problems := strings.Join(s.problemList(), " ")
	if !strings.Contains(problems, "No roast service address yet") {
		t.Errorf("the menu should name what is missing, got %q", problems)
	}
}

func TestTheMenuCodeCanOnlyBeDigits(t *testing.T) {
	h := testServer(t)
	post := func(pin string) int {
		r := httptest.NewRequest(http.MethodPost, "/admin/settings", strings.NewReader(url.Values{"adminPin": {pin}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("X-Admin-Pin", "1379")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if code := post("12ab"); code != http.StatusBadRequest {
		t.Errorf("a code the entry screen cannot type should be refused, got %d", code)
	}
	if code := post("2468"); code != http.StatusOK {
		t.Errorf("a four-digit code should save, got %d", code)
	}
}
