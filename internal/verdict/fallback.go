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
		"%s of your commits land after midnight. The code will be fine; it is the sleep we are worried about.",
		"%s of your commits happen after midnight. Your best work and your worst decisions keep the same hours.",
		"%s of your commits arrive after midnight. The bugs you fix at 3am are the ones you wrote at 2.",
	},
	"Commits at the weekend": {
		"%s of your commits happen at the weekend. Somebody should tell you about Saturdays.",
		"%s of your commits land on a weekend. Your commit graph has no concept of a day off.",
		"%s of your commits are made at the weekend. The build never rests, and apparently neither do you.",
	},
	"Repos with no description": {
		"%s of your repositories have no description. They are not mysterious, just unexplained.",
		"%s of your repositories have no description. Future you will open them like a stranger's fridge.",
		"%s of your repositories come with no description. Each one is a surprise, mostly to you.",
	},
	"One-word commit messages": {
		"%s of your commit messages are one word long. Every one of them was a story you chose not to tell.",
		"%s of your commit messages are a single word. Git blame is going to be a very short conversation.",
		"%s of your commit messages are one word. Brevity is a virtue up to about here.",
	},
	noReadme: {
		"%s of your repositories have no README. Not even you know what they do.",
		"%s of your repositories have no README. Installation instructions: guess.",
		"%s of your repositories have no README. Somewhere a new contributor is still reading the source.",
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
			"Your README claims %s. Not one of your repositories is written in it.",
			"Your README lists %s. Your repositories have never heard of it.",
		)
	default:
		return fresh(b.Avoid, and(b.Unused),
			"Your README claims %s. Not one of your repositories is written in any of them.",
			"Your README lists %s. Your repositories disagree on every count.",
		)
	}

	// The worst of the gauges and the missing READMEs is the one to mention.
	best, top := "", noteworthy-1
	value := ""
	for _, m := range b.Metrics {
		if m.Percent == nil || *m.Percent <= top {
			continue
		}
		if _, ok := lines[m.Label]; ok {
			best, top, value = m.Label, *m.Percent, m.Value
		}
	}
	if b.Read > 0 {
		if n := b.NoReadme * 100 / b.Read; n > top {
			best, value = noReadme, fmt.Sprintf("%d%%", n)
		}
	}
	if best != "" {
		return fresh(b.Avoid, value, lines[best]...)
	}

	return fresh(b.Avoid, "",
		"Your public account is so quiet we could hear the fans spin. Whatever you are building, you are building it somewhere else.",
		"Your public account is mostly silence. Either the real work is private, or so are your plans for it.",
	)
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
