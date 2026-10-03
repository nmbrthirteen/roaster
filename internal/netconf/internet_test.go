package netconf

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func probeAt(t *testing.T, h http.HandlerFunc) Internet {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	t.Cleanup(func() { probeURLFor = probeURL })
	probeURLFor = srv.URL
	return CheckInternet(context.Background())
}

func TestCheckInternet(t *testing.T) {
	if got := probeAt(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(probeBody))
	}); got.State != "online" {
		t.Errorf("real answer: got %+v", got)
	}

	if got := probeAt(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://portal.example/login?x=1", http.StatusFound)
	}); got.State != "signin" || got.Portal != "http://portal.example/login?x=1" {
		t.Errorf("redirect: got %+v", got)
	}

	if got := probeAt(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html>Accept the terms</html>"))
	}); got.State != "signin" || got.Portal != probeURL {
		t.Errorf("page in place: got %+v", got)
	}

	srv := httptest.NewServer(nil)
	probeURLFor = srv.URL
	srv.Close()
	t.Cleanup(func() { probeURLFor = probeURL })
	if got := CheckInternet(context.Background()); got.State != "offline" {
		t.Errorf("no network: got %+v", got)
	}
}
