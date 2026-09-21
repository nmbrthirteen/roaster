package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/upgaming/roaster/internal/roast"
)

// limits is everything about how much the service does at once. The defaults
// are in main; tests shrink them.
type limits struct {
	slots  int           // audits running at once, across every stand
	queue  time.Duration // how long a request waits for a slot before hearing "busy"
	budget time.Duration // how long one audit may take, start to finish
	every  time.Duration // one roast per this, per stand, sustained
	burst  int           // roasts a stand may run back to back
}

type service struct {
	provider roast.Provider
	lim      limits

	terminals map[[32]byte]*rate.Limiter
	slots     chan struct{}
}

var errBusy = errors.New("busy")

func newService(p roast.Provider, tokens []string, lim limits) *service {
	s := &service{
		provider:  p,
		lim:       lim,
		terminals: map[[32]byte]*rate.Limiter{},
		slots:     make(chan struct{}, lim.slots),
	}
	for _, t := range tokens {
		s.terminals[sha256.Sum256([]byte(t))] = rate.NewLimiter(rate.Every(lim.every), lim.burst)
	}
	return s
}

func (s *service) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /roast", s.roast)
	return mux
}

func (s *service) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"running": len(s.slots),
	})
}

// roast is the one route a stand calls. The order matters: who you are, then
// how fast you are going, then what you asked, and only then does anything cost
// money. Repeats and crowds are the audit's business: it remembers accounts and
// what it has said, so this layer has nothing to cache.
func (s *service) roast(w http.ResponseWriter, r *http.Request) {
	started := time.Now()

	terminal, limiter, ok := s.authenticate(r)
	if !ok {
		w.Header().Set("WWW-Authenticate", `Bearer realm="roast"`)
		http.Error(w, "unknown terminal", http.StatusUnauthorized)
		return
	}
	if res := limiter.Reserve(); res.Delay() > 0 {
		res.Cancel()
		w.Header().Set("Retry-After", strconv.Itoa(int(res.Delay().Seconds())+1))
		http.Error(w, "slow down", http.StatusTooManyRequests)
		return
	}

	var req roast.Request
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Handle) == "" {
		http.Error(w, "send a JSON body with a handle", http.StatusBadRequest)
		return
	}

	out := &stream{w: w}
	_, err := s.run(r.Context(), req, out.send)

	switch {
	case errors.Is(err, errBusy) && !out.started:
		w.Header().Set("Retry-After", "5")
		http.Error(w, "busy", http.StatusServiceUnavailable)
	case err != nil:
		out.send(roast.Update{Phase: roast.PhaseError, Error: err.Error()})
	}

	slog.Info("roast",
		"terminal", terminal,
		"handle", req.Handle,
		"ok", err == nil,
		"err", errText(err),
		"ms", time.Since(started).Milliseconds(),
	)
}

// run is the part that costs: a slot, then the audit. It deliberately does not
// stop when the visitor leaves. A cancelled roast is usually retried a second
// later, and finishing it means the account is already read when they do. The
// budget still bounds it.
func (s *service) run(ctx context.Context, req roast.Request, emit func(roast.Update)) (roast.Roast, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.lim.budget)
	defer cancel()

	wait := time.NewTimer(s.lim.queue)
	defer wait.Stop()
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-wait.C:
		return roast.Roast{}, errBusy
	}

	return s.provider.Roast(ctx, req, emit)
}

// authenticate compares digests in constant time, so how long the check takes
// says nothing about how close a guess came. The name it returns is a short
// fingerprint of the token, for the log: enough to tell stands apart, useless
// to anyone reading it.
func (s *service) authenticate(r *http.Request) (string, *rate.Limiter, bool) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || token == "" {
		return "", nil, false
	}
	presented := sha256.Sum256([]byte(token))

	var found *rate.Limiter
	for digest, limiter := range s.terminals {
		if subtle.ConstantTimeCompare(presented[:], digest[:]) == 1 {
			found = limiter
		}
	}
	if found == nil {
		return "", nil, false
	}
	return hex.EncodeToString(presented[:4]), found, true
}

// stream is server-sent events, in the shape roast.Remote reads. Headers go out
// with the first update and not before, so a request refused for being busy can
// still be refused with a status code.
type stream struct {
	mu      sync.Mutex
	w       http.ResponseWriter
	started bool
}

func (s *stream) send(u roast.Update) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		h := s.w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("X-Accel-Buffering", "no") // proxies that buffer would hold the audit back until it finished
		s.w.WriteHeader(http.StatusOK)
		s.started = true
	}

	data, err := json.Marshal(u)
	if err != nil {
		return
	}
	// A visitor who walked away has closed the connection. Nothing to do about
	// that here, and the audit carries on for the cache.
	fmt.Fprintf(s.w, "data: %s\n\n", data)
	if f, ok := s.w.(http.Flusher); ok {
		f.Flush()
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
