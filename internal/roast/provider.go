package roast

import (
	"context"
	"crypto/rand"
	"time"
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

// Demo is the stand-in until the GitHub adapter and the model call land. It
// emits the same updates on the same timings a real audit would, so the kiosk
// and the wait experience can be built and judged before either exists.
type Demo struct{}

func (Demo) Roast(ctx context.Context, req Request, emit func(Update)) (Roast, error) {
	step := func(d time.Duration) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
			return nil
		}
	}

	emit(Update{Phase: PhaseFetch, Label: "Reading public commits"})
	if err := step(900 * time.Millisecond); err != nil {
		return Roast{}, err
	}

	r := Sample(req.Handle)
	for _, m := range r.Metrics {
		metric := m
		emit(Update{Phase: PhaseMetric, Metric: &metric})
		if err := step(280 * time.Millisecond); err != nil {
			return Roast{}, err
		}
	}

	emit(Update{Phase: PhaseVerdict, Label: "Writing the verdict"})
	if err := step(1500 * time.Millisecond); err != nil {
		return Roast{}, err
	}
	emit(Update{Phase: PhaseVerdict, Verdict: r.Verdict})

	emit(Update{Phase: PhaseDone, Roast: &r})
	return r, nil
}

func pct(n int) *int { return &n }

// Sample is the fixed example the designer renders, so a layout change is
// judged against the same content every time.
func Sample(handle string) Roast {
	return Roast{
		Code:     Code(),
		Handle:   handle,
		At:       time.Now(),
		Score:    "89 / 100",
		ScoreTag: "critical",
		Metrics: []Metric{
			{Label: "Commits after midnight", Value: "34%", Tag: "owl", Percent: pct(34)},
			{Label: "Friday deploys", Value: "14%", Tag: "reckless", Percent: pct(14)},
			{Label: "Repos with no description", Value: "71%", Percent: pct(71)},
			{Label: "One-word commit messages", Value: "62%", Tag: "terse", Percent: pct(62)},
			{Label: "Longest gap between commits", Value: "214 days"},
		},
		Verdict: "Your architecture diagram looks like a bowl of spaghetti dropped on AWS. " +
			"Upgaming gives you a 12% survival rate in production.",
		Odds: []Odd{
			{Label: "You survive a prod crash", Price: "7.50"},
			{Label: "A Friday ship goes unnoticed", Price: "13.00"},
			{Label: "You blame a junior", Price: "1.25", Tag: "sure thing"},
		},
		Hiring: "6 open roles match your stack",
	}
}
