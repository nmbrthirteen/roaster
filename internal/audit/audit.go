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

const verdictTimeout = 40 * time.Second

var (
	// ErrOptedOut is an account that asked not to be roasted. Said plainly, so
	// whoever typed it knows it was a choice and not a fault.
	ErrOptedOut = errors.New("this account has asked not to be roasted")

	// ErrGitHub is GitHub not answering. The detail goes to the log; the
	// visitor gets something they can act on.
	ErrGitHub = errors.New("GitHub is not answering right now; try again in a moment")

	ErrGitHubBusy = errors.New("GitHub is too busy to read accounts right now; try again in a few minutes")
)

const lowBudget = 400

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

	got, err := a.measure(ctx, handle, req.Offset)
	switch {
	case err == nil:
	case errors.Is(err, github.ErrNoAccount):
		return roast.Roast{}, fmt.Errorf("no GitHub account called @%s", handle)
	case errors.Is(err, github.ErrBadHandle):
		return roast.Roast{}, err
	case errors.As(err, new(*github.RateLimited)):
		slog.Warn("github", "handle", handle, "err", err)
		return roast.Roast{}, ErrGitHubBusy
	case ctx.Err() != nil:
		return roast.Roast{}, ctx.Err()
	default:
		slog.Error("github", "handle", handle, "err", err)
		return roast.Roast{}, ErrGitHub
	}

	for i := range got.story {
		emit(roast.Update{Phase: roast.PhaseSection, Section: &got.story[i]})
	}
	emit(roast.Update{Phase: roast.PhaseFeed, Feed: got.feed})

	metrics := got.brief.Metrics
	for _, m := range metrics {
		emit(roast.Update{Phase: roast.PhaseMetric, Metric: &m})
	}
	evidence := metric.Evidence(got.handle, got.habits, got.findings)
	emit(roast.Update{Phase: roast.PhaseSection, Section: &evidence})

	emit(roast.Update{Phase: roast.PhaseVerdict, Label: "Writing the verdict"})
	b := got.brief
	if a.Memory != nil {
		b.Avoid = a.Memory.avoid(key(handle))
	}
	page := a.write(ctx, b)
	line := page.Verdict
	if a.Memory != nil {
		a.Memory.said(key(handle), line)
	}

	score := got.brief.Score
	r := roast.Roast{
		Code:      roast.Code(),
		Pack:      roast.GitHub,
		Handle:    got.handle,
		At:        a.now(),
		Score:     fmt.Sprintf("%d / 100", score),
		ScoreTag:  roast.Severity(score),
		Archetype: page.Archetype,
		Metrics:   metrics,
		Verdict:   line,
		Actions:   page.Actions,
		Strengths: page.Strengths,
		Findings:  lined(got.findings, page.Findings),
		Habits:    lined(got.habits, page.Habits),
		Story:     got.story,
		Exhibit:   got.exhibit,
		Heat:      got.heat,
	}

	emit(roast.Update{Phase: roast.PhaseVerdict, Verdict: line})
	emit(roast.Update{Phase: roast.PhaseDone, Roast: &r})
	return r, nil
}

// measure reads, measures and briefs the account, through Memory when there is
// one, so a repeat costs GitHub nothing and a crowd typing one handle costs a
// single read.
func (a Audit) measure(ctx context.Context, handle string, offset int) (measured, error) {
	read := func() (measured, error) {
		started := time.Now()
		facts, err := a.GitHub.Read(ctx, handle)
		if err != nil {
			return measured{}, err
		}
		slog.Info("github", "handle", handle, "cost", facts.Cost, "remaining", facts.Remaining,
			"ms", time.Since(started).Milliseconds())
		if facts.Cost > 0 && facts.Remaining < lowBudget {
			slog.Warn("github budget low", "remaining", facts.Remaining)
		}

		zone := time.FixedZone("stand", offset)
		for i := range facts.Commits {
			facts.Commits[i].At = facts.Commits[i].At.In(zone)
		}

		metrics := metric.From(facts)
		m := measured{
			handle:    facts.Handle,
			brief:     verdict.From(facts, metrics, a.now()),
			story:     metric.Story(facts, a.now()),
			actions:   metric.Actions(facts),
			strengths: metric.Strengths(facts),
			findings:  metric.Findings(facts, a.now()),
			habits:    metric.Habits(facts, a.now()),
			feed:      metric.Feed(facts.Commits, metric.FeedMax),
			exhibit:   metric.Worst(facts.Commits),
			heat:      metric.Heat(facts.Year.Days, metric.HeatWeeks),
		}
		m.brief.Strengths, m.brief.Actions = m.strengths, m.actions
		m.brief.Findings, m.brief.Habits = m.findings, m.habits
		return m, nil
	}

	if a.Memory == nil {
		return read()
	}
	// Hours depend on the clock they are read on, so each offset is its own entry.
	return a.Memory.measure(fmt.Sprintf("%s@%d", key(handle), offset), read)
}

func (a Audit) write(ctx context.Context, b verdict.Brief) verdict.Page {
	if a.Writer == nil {
		return verdict.Written(b)
	}

	timeout := a.Timeout
	if timeout == 0 {
		timeout = verdictTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	p, err := a.Writer.Write(ctx, b)
	if err != nil {
		slog.Warn("page fell back to the numbers", "err", err)
		return verdict.Written(b)
	}
	return verdict.Merge(b, p)
}

func lined(fs []roast.Finding, lines []string) []roast.Finding {
	out := append([]roast.Finding(nil), fs...)
	for i := range out {
		if i < len(lines) {
			out[i].Line = lines[i]
		}
	}
	return out
}

func (a Audit) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}
