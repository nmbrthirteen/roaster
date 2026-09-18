package roast

import (
	"context"
	"crypto/rand"
)

// Request is one audit: who to roast, which pack of questions to ask, and which
// event's branding the answer gets printed under.
type Request struct {
	Handle string
	Pack   string
	Event  string
}

// Phase names the stages the kiosk shows while it waits. The wait is the worst
// part of the experience, so the screen reports real progress rather than
// spinning.
const (
	PhaseFetch   = "fetch"   // reading the account
	PhaseMetric  = "metric"  // one computed measurement, revealed as it lands
	PhaseVerdict = "verdict" // the written roast, streamed
	PhaseDone    = "done"    // the finished roast, ready to print
	PhaseError   = "error"
)

// Update is one step of progress, streamed to the kiosk as it happens.
type Update struct {
	Phase   string  `json:"phase"`
	Label   string  `json:"label,omitempty"`
	Metric  *Metric `json:"metric,omitempty"`
	Verdict string  `json:"verdict,omitempty"`
	Roast   *Roast  `json:"roast,omitempty"`
	Error   string  `json:"error,omitempty"`
}

// Provider turns a handle into a roast. Everything that talks to the outside
// world sits behind this: the GitHub fetch, the metric computation, and the
// model call. The kiosk knows none of it, which is what lets the generation run
// on a server that holds the keys while the screen stays dumb.
type Provider interface {
	Roast(ctx context.Context, req Request, emit func(Update)) (Roast, error)
}

// Code is the short identifier the share URL and the printed QR carry.
// Ambiguous characters are left out: these get read off paper by hand.
func Code() string {
	const alphabet = "23456789abcdefghjkmnpqrstuvwxyz"
	b := make([]byte, 5)
	rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}
