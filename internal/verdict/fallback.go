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
		"%s of your commits land after midnight. Your rubber duck has filed for overtime.",
		"%s of your commits ship after midnight. Your code has never once seen daylight.",
		"%s of your commits happen after midnight. The 3am bugs were written at 2am.",
	},
	"Weekends with commits": {
		"You committed on %s of weekend days this year. Your calendar says Saturday. Your git log says sprint.",
		"%s of your weekends had commits in them. HR would like a word. So would your friends.",
		"You coded through %s of your weekends. Brunch exists. We checked.",
	},
	"Days with commits": {
		"You committed on %s of days this year. Impressive stamina. Worrying hobby.",
		"Commits on %s of days this year. Your laptop has asked for a holiday.",
		"%s of this year's days have your commits on them. The other days are presumably recovery.",
	},
	"Repos with no description": {
		"%s of your repos have no description. Even you have to open them to find out.",
		"%s of your repos have no description. Mystery boxes, and all of them free.",
		"%s of your repos come with no description. Naming things was hard. Describing them was apparently impossible.",
	},
	"One-word commit messages": {
		"%s of your commit messages are one word long. Your git log reads like a ransom note.",
		"%s of your commit messages are a single word. Even git blame just shrugs.",
		`%s of your commit messages are one word. "fix" what? We will never know.`,
	},
	noReadme: {
		"%s of your repos have no README. Installation instructions: vibes.",
		"%s of your repos have no README. Onboarding is a treasure hunt with no map.",
		"%s of your repos have no README. The docs live in your head, and your head isn't on GitHub.",
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
			"Your README claims %s. Your repos have never met it.",
			"Your README lists %s. Your repos would like to see some ID.",
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
		"Your public GitHub is so empty it echoes. No commits, no bugs, no evidence.",
		"We found nothing public to roast. Stealth genius, or a very long draft.",
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
