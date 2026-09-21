// Package audit is the real roast: read the account, measure it, and have the
// verdict written. It is a roast.Provider, so the stand cannot tell it from the
// rehearsal and needs no change to use it.
//
// The rule it is built around: a visitor who typed a real handle always leaves
// with a receipt. GitHub failing is the only thing that ends an audit early,
// because without the account there is nothing true to print. Everything after
// that has a way through: a verdict that is slow, refused, unprintable or never
// asked for is written from the numbers instead.
package audit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/upgaming/roaster/internal/github"
	"github.com/upgaming/roaster/internal/metric"
	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/verdict"
)

// The verdict gets this long before the numbers write it instead. Long enough
// for a good line on a normal day; short enough that nobody at the stand notices
// on a bad one.
const verdictTimeout = 15 * time.Second

var (
	// ErrOptedOut is an account that asked not to be roasted. Said plainly, so
	// whoever typed it knows it was a choice and not a fault.
	ErrOptedOut = errors.New("this account has asked not to be roasted")

	// ErrGitHub is GitHub not answering. The detail goes to the log; the
	// visitor gets something they can act on.
	ErrGitHub = errors.New("GitHub is not answering right now; try again in a moment")
)

// Reader is where the account comes from. github.Client is the real one.
type Reader interface {
	Read(ctx context.Context, handle string) (github.Facts, error)
}

type Audit struct {
	GitHub Reader

	// Writer writes the verdict. Nil runs on the numbers alone, which is a
	// complete and honest roast with no model and no key.
	Writer verdict.Writer

	// Blocked holds handles, lowercased, that must never be roasted.
	Blocked map[string]bool

	// Memory keeps accounts and lines between visitors. Nil reads GitHub
	// every time and has nothing to say about repeats.
	Memory *Memory

	// Timeout overrides how long the verdict may take. Tests set it; nothing
	// else should need to.
	Timeout time.Duration

	// Now is the clock, for tests. Nil is time.Now.
	Now func() time.Time
}

func (a Audit) Roast(ctx context.Context, req roast.Request, emit func(roast.Update)) (roast.Roast, error) {
	handle := strings.TrimPrefix(strings.TrimSpace(req.Handle), "@")
	if !github.Valid(handle) {
		return roast.Roast{}, fmt.Errorf("%q %w", handle, github.ErrBadHandle)
	}
	if a.Blocked[strings.ToLower(handle)] {
		return roast.Roast{}, ErrOptedOut
	}

	emit(roast.Update{Phase: roast.PhaseFetch, Label: "Reading public commits"})

	got, err := a.measure(ctx, handle)
	switch {
	case err == nil:
	case errors.Is(err, github.ErrNoAccount):
		return roast.Roast{}, fmt.Errorf("no GitHub account called @%s", handle)
	case errors.Is(err, github.ErrBadHandle):
		return roast.Roast{}, err
	case ctx.Err() != nil:
		return roast.Roast{}, ctx.Err()
	default:
		slog.Error("github", "handle", handle, "err", err)
		return roast.Roast{}, ErrGitHub
	}

	metrics := got.brief.Metrics
	for _, m := range metrics {
		emit(roast.Update{Phase: roast.PhaseMetric, Metric: &m})
	}

	emit(roast.Update{Phase: roast.PhaseVerdict, Label: "Writing the verdict"})
	b := got.brief
	if a.Memory != nil {
		b.Avoid = a.Memory.avoid(key(handle))
	}
	line := a.verdict(ctx, b)
	if a.Memory != nil {
		a.Memory.said(key(handle), line)
	}

	score := roast.Score(metrics)
	r := roast.Roast{
		Code:     roast.Code(),
		Handle:   got.handle,
		At:       a.now(),
		Score:    fmt.Sprintf("%d / 100", score),
		ScoreTag: roast.Severity(score),
		Metrics:  metrics,
		Verdict:  line,
		Odds:     roast.Odds(score),
	}

	emit(roast.Update{Phase: roast.PhaseVerdict, Verdict: line})
	emit(roast.Update{Phase: roast.PhaseDone, Roast: &r})
	return r, nil
}

// measure reads, measures and briefs the account, through Memory when there is
// one, so a repeat costs GitHub nothing and a crowd typing one handle costs a
// single read.
func (a Audit) measure(ctx context.Context, handle string) (measured, error) {
	read := func() (measured, error) {
		started := time.Now()
		facts, err := a.GitHub.Read(ctx, handle)
		if err != nil {
			return measured{}, err
		}
		slog.Info("github", "handle", handle, "cost", facts.Cost, "remaining", facts.Remaining,
			"ms", time.Since(started).Milliseconds())

		metrics := metric.From(facts)
		return measured{handle: facts.Handle, brief: verdict.From(facts, metrics, a.now())}, nil
	}

	if a.Memory == nil {
		return read()
	}
	return a.Memory.measure(key(handle), read)
}

// verdict never fails. Whatever goes wrong with the model, the numbers can
// still say something true.
func (a Audit) verdict(ctx context.Context, b verdict.Brief) string {
	if a.Writer == nil {
		return verdict.Fallback(b)
	}

	timeout := a.Timeout
	if timeout == 0 {
		timeout = verdictTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	line, err := a.Writer.Write(ctx, b)
	if err != nil {
		slog.Warn("verdict fell back to the numbers", "err", err)
		return verdict.Fallback(b)
	}
	return line
}

func (a Audit) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}
