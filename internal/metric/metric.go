// Package metric turns what GitHub says into the five numbers printed on the
// receipt.
//
// Every one of them is arithmetic over fetched facts. Nothing here is invented,
// estimated or asked of a model, because a receipt someone photographs and
// shows to the person next to them has to survive being checked.
package metric

import (
	"fmt"
	"strings"
	"time"

	"github.com/upgaming/roaster/internal/github"
	"github.com/upgaming/roaster/internal/roast"
)

// Small hours run to six. Anyone committing at half past five is not having an
// early start.
const nightEnds = 6

// From reads the account into the row of gauges the receipt is laid out for:
// five with bars and one without, in that order.
//
// The two calendar gauges are what make a busy account score as busy: they
// count days, private work included, where the others count shares of what
// was read.
func From(f github.Facts) []roast.Metric {
	m := []roast.Metric{
		gauge("Commits after midnight", share(f.Commits, atNight),
			[3]string{"sleeps", "owl", "vampire"}),
		gauge("Weekends with commits", days(f, isWeekend),
			[3]string{"rested", "restless", "no brakes"}),
		gauge("Days with commits", days(f, func(time.Time) bool { return true }),
			[3]string{"casual", "committed", "no off switch"}),
		gauge("Repos with no description", undescribed(f.Repos),
			[3]string{"clear", "vague", "ghosted"}),
		gauge("One-word commit messages", share(f.Commits, oneWord),
			[3]string{"poet", "brief", "caveman"}),
		longestGap(f),
	}
	// Nothing to measure is not a virtue. "0% poet" on an empty account reads
	// as praise, so a gauge with no data says so.
	if len(f.Commits) == 0 {
		m[0].Tag, m[4].Tag = untested, untested
	}
	if len(f.Repos) == 0 {
		m[3].Tag = untested
	}
	if contributed(f) == 0 {
		m[1].Tag, m[2].Tag = untested, untested
	}
	return m
}

const untested = "untested"

// Score and its severity band are the roast's own, so the demo and the real
// audit cannot drift apart on what a number means.
func Score(metrics []roast.Metric) int { return roast.Score(metrics) }

func atNight(c github.Commit) bool {
	return c.At.Hour() < nightEnds
}

func atWeekend(c github.Commit) bool { return isWeekend(c.At) }

func isWeekend(t time.Time) bool {
	switch t.Weekday() {
	case time.Saturday, time.Sunday:
		return true
	}
	return false
}

// days is the share of calendar days picked by which that have at least one
// contribution on them.
func days(f github.Facts, which func(time.Time) bool) int {
	n, active := 0, 0
	for _, d := range f.Year.Days {
		if !which(d.Date) {
			continue
		}
		n++
		if d.Count > 0 {
			active++
		}
	}
	return percent(active, n)
}

func contributed(f github.Facts) int {
	n := 0
	for _, d := range f.Year.Days {
		n += d.Count
	}
	return n
}

// oneWord counts the headline nobody will read twice: "fix", "wip", "asdf",
// and the full stop on its own.
func oneWord(c github.Commit) bool {
	return len(strings.Fields(c.Message)) <= 1
}

// share is a percentage of the commits read, rounded to the nearest whole one.
// An account with nothing public reads as zero rather than as an error: the
// receipt still prints, and there is a joke in the blank.
func share(commits []github.Commit, is func(github.Commit) bool) int {
	if len(commits) == 0 {
		return 0
	}
	n := 0
	for _, c := range commits {
		if is(c) {
			n++
		}
	}
	return percent(n, len(commits))
}

func undescribed(repos []github.Repo) int {
	if len(repos) == 0 {
		return 0
	}
	n := 0
	for _, r := range repos {
		if strings.TrimSpace(r.Description) == "" {
			n++
		}
	}
	return percent(n, len(repos))
}

// longestGap is the longest run of days in the contribution year with nothing
// on it. The calendar is the only source that covers private work, and then
// only as a count, so a quiet stretch here is a genuinely quiet stretch.
func longestGap(f github.Facts) roast.Metric {
	const label = "Longest gap between commits"

	if len(f.Year.Days) == 0 {
		return roast.Metric{Label: label, Value: "unknown"}
	}

	longest := quietest(f)
	unit := "days"
	if longest == 1 {
		unit = "day"
	}
	return roast.Metric{Label: label, Value: fmt.Sprintf("%d %s", longest, unit)}
}

// gauge always carries a tag. Tagging only the bad rows left the column ragged
// and made the whole block look arbitrary on paper.
func gauge(label string, n int, bands [3]string) roast.Metric {
	band := bands[0]
	switch {
	case n >= 66:
		band = bands[2]
	case n >= 33:
		band = bands[1]
	}
	return roast.Metric{
		Label:   label,
		Value:   fmt.Sprintf("%d%%", n),
		Tag:     band,
		Percent: &n,
	}
}

func percent(n, of int) int {
	if of == 0 {
		return 0
	}
	return (n*200 + of) / (of * 2) // rounded, without floating point
}
