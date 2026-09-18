package roast

import (
	"context"
	"fmt"
	"hash/fnv"
	"math/rand"
	"strings"
	"time"
)

// Demo stands in for the real audit until the GitHub adapter and the model call
// land.
type Demo struct{}

// Reserved handles for exercising paths that are otherwise hard to reach.
const (
	handleMissing = "notfound" // the account does not exist
	handleSlow    = "slowpoke" // a sluggish upstream
)

func (Demo) Roast(ctx context.Context, req Request, emit func(Update)) (Roast, error) {
	handle := strings.TrimSpace(req.Handle)
	rng := rand.New(rand.NewSource(seed(handle)))

	pace := time.Duration(1)
	if strings.EqualFold(handle, handleSlow) {
		pace = 3
	}
	step := func(d time.Duration) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d * pace):
			return nil
		}
	}

	emit(Update{Phase: PhaseFetch, Label: "Reading public commits"})
	if err := step(900 * time.Millisecond); err != nil {
		return Roast{}, err
	}

	if strings.EqualFold(handle, handleMissing) {
		return Roast{}, fmt.Errorf("no GitHub account called @%s", handle)
	}

	r := build(handle, rng)
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

func seed(handle string) int64 {
	h := fnv.New64a()
	h.Write([]byte(strings.ToLower(handle)))
	return int64(h.Sum64())
}

func Sample(handle string) Roast {
	return build(handle, rand.New(rand.NewSource(7)))
}

func build(handle string, rng *rand.Rand) Roast {
	metrics := []Metric{
		gauge("Commits after midnight", rng, 15, 70, "owl", "nocturnal"),
		gauge("Friday deploys", rng, 5, 45, "reckless", "brave"),
		gauge("Repos with no description", rng, 30, 95, "", "silent"),
		gauge("One-word commit messages", rng, 25, 90, "terse", "wordless"),
		{Label: "Longest gap between commits", Value: fmt.Sprintf("%d days", 40+rng.Intn(400))},
	}

	// The score follows the gauges rather than being drawn separately, so the
	// headline number and the detail under it never contradict each other.
	total := 0
	for _, m := range metrics {
		if m.Percent != nil {
			total += *m.Percent
		}
	}
	score := total / 4

	return Roast{
		Code:     Code(),
		Handle:   handle,
		At:       time.Now(),
		Score:    fmt.Sprintf("%d / 100", score),
		ScoreTag: severity(score),
		Metrics:  metrics,
		Verdict:  pick(rng, verdicts),
		Odds: []Odd{
			{Label: "You survive a prod crash", Price: price(rng, 3, 14)},
			{Label: "A Friday ship goes unnoticed", Price: price(rng, 6, 22)},
			{Label: "You blame a junior", Price: price(rng, 1, 2), Tag: "sure thing"},
		},
		Hiring: fmt.Sprintf("%d open roles match your stack", 3+rng.Intn(8)),
	}
}

func gauge(label string, rng *rand.Rand, lo, hi int, tags ...string) Metric {
	n := lo + rng.Intn(hi-lo+1)
	m := Metric{Label: label, Value: fmt.Sprintf("%d%%", n), Percent: &n}
	if n > (lo+hi)/2 {
		for _, t := range tags {
			if t != "" {
				m.Tag = t
				break
			}
		}
	}
	return m
}

func severity(score int) string {
	switch {
	case score >= 75:
		return "critical"
	case score >= 55:
		return "serious"
	case score >= 35:
		return "survivable"
	default:
		return "suspiciously tidy"
	}
}

func price(rng *rand.Rand, lo, hi int) string {
	whole := lo + rng.Intn(hi-lo+1)
	return fmt.Sprintf("%d.%02d", whole, rng.Intn(4)*25)
}

func pick(rng *rand.Rand, from []string) string { return from[rng.Intn(len(from))] }

// Written to roast the work, never the person.
var verdicts = []string{
	"Your architecture diagram looks like a bowl of spaghetti dropped on AWS. Upgaming gives you a 12% survival rate in production.",
	"You have written the same utility function in four repositories and named it something different every time.",
	"Your commit history reads like a hostage note. Half the messages are the word fix and the other half are a full stop.",
	"You open pull requests the way other people open browser tabs, and you close them about as often.",
	"Every project starts with a README and a plan. Both are abandoned by the third commit, which is where the real code begins.",
	"Your test suite is aspirational. It describes a codebase that would be lovely to have.",
	"You refactor in production and call it observability. Bold, and we respect it, but bold.",
	"There is a branch in your account from two years ago called temp-fix-final-2. It is still ahead of main.",
}
