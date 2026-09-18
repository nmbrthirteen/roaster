package roast

import (
	"context"
	"crypto/rand"
)

type Request struct {
	Handle string
	Pack   string
	Event  string
}

// Phase names the stages the kiosk shows while it waits.
const (
	PhaseFetch   = "fetch"   // reading the account
	PhaseMetric  = "metric"  // one computed measurement, revealed as it lands
	PhaseVerdict = "verdict" // the written roast, streamed
	PhaseDone    = "done"    // the finished roast, ready to print
	PhaseError   = "error"
)

type Update struct {
	Phase   string  `json:"phase"`
	Label   string  `json:"label,omitempty"`
	Metric  *Metric `json:"metric,omitempty"`
	Verdict string  `json:"verdict,omitempty"`
	Roast   *Roast  `json:"roast,omitempty"`
	Error   string  `json:"error,omitempty"`
}

// Provider turns a handle into a roast.
type Provider interface {
	Roast(ctx context.Context, req Request, emit func(Update)) (Roast, error)
}

func Code() string {
	const alphabet = "23456789abcdefghjkmnpqrstuvwxyz"
	b := make([]byte, 5)
	rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}
