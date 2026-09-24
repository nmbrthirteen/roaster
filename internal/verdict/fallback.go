package verdict

import (
	"fmt"
	"strings"
)

// A gauge below this is not the most interesting thing about anybody.
const noteworthy = 33

// noReadme is not a gauge on the receipt, but it is often the truest thing about
// an account with little else to say.
const noReadme = "Repos with no README"

// lines are keyed by the metric they are about, so each one only ever states
// what was measured. The joke is in the second sentence; the first is a fact.
// Each has a few, because the person behind in the queue has read the last one.
var lines = map[string][]string{
	"Commits after midnight": {
		"%s of your commits land after midnight. Nobody reviews code at 3am, and it shows.",
		"%s of your commits come after midnight. It reads like it was written half asleep.",
		"%s of your commits happen after midnight. The bugs work the night shift here.",
	},
	"Weekends with commits": {
		"Commits on %s of weekends. Saturday is a workday here, and nobody is paying for it.",
		"%s of weekends have commits in them. Your friends stopped asking. The repo never did.",
		"Weekend commits on %s of weekends. That is a second job with no salary.",
	},
	"Days with commits": {
		"Commits on %s of days this year. It isn't a streak, it's a hostage situation.",
		"Commits on %s of days this year. The green squares get more sunlight than you do.",
		"%s of this year's days have commits on them. A day off is a rumor in this repo.",
	},
	"Repos with no description": {
		"%s of your repos have no description. Even you have to click in to find out.",
		"No description on %s of your repos. Naming them was the whole plan.",
		"%s of your repos have no description. A shop with no signs and no customers.",
	},
	"One-word commit messages": {
		"%s of your commit messages are one word. The next person on this code gets zero help.",
		"%s of your commit messages are one word. The history reads like a shopping list for bugs.",
		`%s of your commit messages are one word. "fix" fixed what? Nobody will ever know.`,
	},
	noReadme: {
		"%s of your repos have no README. To install one, read the code and pray.",
		"No README on %s of your repos. The setup guide lives in your head.",
		"%s of your repos ship with no README. Good luck to whoever finds them next.",
	},
}

// Fallback writes the verdict from the numbers when the model cannot: it is
// down, slow, declined, or wrote something unprintable. It is always true,
// because it only says what was measured, and it never fails, so a visitor
// always leaves with a receipt.
func Fallback(b Brief) string {
	switch len(b.Unused) {
	case 0:
	case 1:
		return fresh(b.Avoid, b.Unused[0],
			"Your README claims %s. Your repos have never heard of it.",
			"Your README lists %s. Your code has no idea.",
		)
	default:
		return fresh(b.Avoid, and(b.Unused),
			"Your README claims %s. Your repos can't back up a single one.",
			"Your README lists %s. Nobody told your code.",
		)
	}

	if best, value := worst(b); best != "" {
		return fresh(b.Avoid, value, lines[best]...)
	}

	return fresh(b.Avoid, "",
		"Your public GitHub is so empty it echoes. Hard to write bugs with no code.",
		"Nothing public to roast. The safest way to never ship a bug is to never ship.",
		"Zero public activity. The perfect codebase is the one nobody can see.",
	)
}

func worst(b Brief) (label, value string) {
	top := noteworthy - 1
	for _, m := range b.Metrics {
		if m.Percent == nil || *m.Percent <= top {
			continue
		}
		if _, ok := lines[m.Label]; ok {
			label, top, value = m.Label, *m.Percent, m.Value
		}
	}
	if b.Read > 0 {
		if n := b.NoReadme * 100 / b.Read; n > top {
			label, value = noReadme, fmt.Sprintf("%d%%", n)
		}
	}
	return label, value
}

var archetypes = map[string]string{
	"Commits after midnight":    "Head of Night Shifts",
	"Weekends with commits":     "Chief Weekend Officer",
	"Days with commits":         "Senior Always-On Engineer",
	"Repos with no description": "Director of Mystery Repos",
	"One-word commit messages":  "Senior Fix Engineer",
	noReadme:                    "Head of Undocumented Features",
}

func Archetype(b Brief) string {
	switch {
	case len(b.Unused) > 0:
		return "Principal Badge Collector"
	case b.Read == 0 && b.Year.Commits == 0:
		return "Stealth Mode Founder"
	}
	if best, _ := worst(b); best != "" {
		return archetypes[best]
	}
	return "Suspiciously Normal Engineer"
}

// fresh fills the first line not already printed. When every one has been,
// it starts again from the top: repeating is better than printing nothing.
func fresh(avoid []string, value string, options ...string) string {
	fill := func(o string) string {
		if strings.Contains(o, "%s") {
			return fmt.Sprintf(o, value)
		}
		return o
	}
	used := map[string]bool{}
	for _, a := range avoid {
		used[a] = true
	}
	for _, o := range options {
		if line := fill(o); !used[line] {
			return line
		}
	}
	return fill(options[0])
}

func and(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
