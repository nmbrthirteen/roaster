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
		gauge("Commits after midnight", rng, 15, 90, [3]string{"diurnal", "owl", "nocturnal"}),
		gauge("Friday deploys", rng, 5, 85, [3]string{"careful", "bold", "reckless"}),
		gauge("Repos with no description", rng, 20, 95, [3]string{"documented", "sparse", "silent"}),
		gauge("One-word commit messages", rng, 20, 95, [3]string{"wordy", "brief", "terse"}),
		{Label: "Longest gap between commits", Value: fmt.Sprintf("%d days", 40+rng.Intn(400))},
	}

	// The score is the plain average of the gauges. Weighting the worst one
	// made receipts where three bars were short still read as serious, which
	// contradicts the picture directly above it.
	total, n := 0, 0
	for _, m := range metrics {
		if m.Percent == nil {
			continue
		}
		total += *m.Percent
		n++
	}
	score := total / n

	return Roast{
		Code:     Code(),
		Handle:   handle,
		At:       time.Now(),
		Score:    fmt.Sprintf("%d / 100", score),
		ScoreTag: severity(score),
		Metrics:  metrics,
		Verdict:  pick(rng, verdicts),
		// Odds are read off the score rather than drawn, so a bad audit really
		// does pay worse. Numbers nobody can trace back look invented.
		Odds: []Odd{
			{Label: "You survive a prod crash", Price: odds(2.0 + float64(score)/12)},
			{Label: "A Friday ship goes unnoticed", Price: odds(3.0 + float64(score)/6)},
			{Label: "You blame a junior", Price: odds(2.2 - float64(score)/120), Tag: "sure thing"},
		},
	}
}

// gauge always carries a tag. Tagging only the bad rows left the column ragged
// and made the whole block look arbitrary on paper.
func gauge(label string, rng *rand.Rand, lo, hi int, bands [3]string) Metric {
	n := lo + rng.Intn(hi-lo+1)
	band := bands[0]
	switch {
	case n >= 66:
		band = bands[2]
	case n >= 33:
		band = bands[1]
	}
	return Metric{Label: label, Value: fmt.Sprintf("%d%%", n), Tag: band, Percent: &n}
}

// odds formats a decimal price, clamped to something a bookmaker would print.
func odds(v float64) string {
	if v < 1.05 {
		v = 1.05
	}
	if v > 99 {
		v = 99
	}
	return fmt.Sprintf("%.2f", v)
}

// severity bands are set against where the average of four gauges actually
// lands, not against a tidy quartering of nought to a hundred. Bands that never
// fire are worse than no bands.
func severity(score int) string {
	switch {
	case score >= 62:
		return "critical"
	case score >= 50:
		return "serious"
	case score >= 38:
		return "survivable"
	default:
		return "suspiciously tidy"
	}
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
