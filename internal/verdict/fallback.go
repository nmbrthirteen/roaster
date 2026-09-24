package verdict

import (
	"fmt"
	"strings"

	"github.com/upgaming/roaster/internal/metric"
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
		"%s of commits after midnight. The bugs work the night shift.",
		"%s of commits after midnight. Written half asleep, reviewed by nobody.",
		"%s of commits after midnight. Sleep was optional. So was testing.",
	},
	"Active weekends": {
		"Activity on %s of weekends. Saturday is a workday with no salary.",
		"%s of weekends busy. The repo sees more Saturdays than the sofa.",
		"%s of weekends busy. A second job nobody is paying for.",
	},
	"Active days": {
		"Activity on %s of days. Not a streak. A hostage situation.",
		"%s of days active. A day off is a rumor here.",
		"%s of days green. The calendar forgot what grey looks like.",
	},
	"Repos with no description": {
		"%s of repos with no description. Mystery is not a feature.",
		"No description on %s of repos. Naming them was the whole plan.",
		"%s of repos undescribed. A shop with no signs and no customers.",
	},
	"One-word commit messages": {
		"%s of commit messages are one word. The next reader gets zero help.",
		"%s of commit messages are one word. A shopping list for bugs.",
		`%s of commit messages are one word. "fix" fixed what? Nobody knows.`,
	},
	noReadme: {
		"%s of repos have no README. Install guide: read the code and pray.",
		"No README on %s of repos. The docs live in one head.",
		"%s of repos ship with no README. Good luck to whoever clones one.",
	},
}

// Fallback writes the verdict from the numbers when the model cannot: it is
// down, slow, declined, or wrote something unprintable. It is always true,
// because it only says what was measured, and it never fails, so a visitor
// always leaves with a receipt.
func Fallback(b Brief) string {
	if options := shaped(b); len(options) > 0 {
		return fresh(b.Avoid, "", options...)
	}

	if options := claimed(b); len(options) > 0 {
		return fresh(b.Avoid, "", options...)
	}

	if best, value := worst(b); best != "" {
		return fresh(b.Avoid, value, lines[best]...)
	}

	return fresh(b.Avoid, "",
		"Nothing public. Hard to ship bugs with no code.",
		"Nothing public to roast. The safest way to never ship a bug is to never ship.",
		"Zero public activity. The perfect codebase is the one nobody can see.",
	)
}

func claimed(b Brief) []string {
	if len(b.Unused) == 0 {
		return nil
	}
	langs := and(b.Unused)
	switch b.Read {
	case 0:
		return []string{fmt.Sprintf("%s on the badges. Not a single repo to back it up.", langs)}
	case 1:
		return []string{fmt.Sprintf("%s on the badges. The only repo never heard of it.", langs)}
	}
	return []string{
		fmt.Sprintf("%s on the badges. Not one of the %d latest repos agrees. Decoration.", langs, b.Read),
		fmt.Sprintf("README says %s. The %d latest repos say otherwise.", langs, b.Read),
	}
}

func shaped(b Brief) []string {
	switch b.Shape {
	case metric.Ghost:
		if b.Contributions == 0 || b.CalendarDays == 0 {
			return nil
		}
		empty := b.CalendarDays - b.ActiveDays
		return []string{
			fmt.Sprintf("%s in a whole year. The keyboard still has the plastic on.", plural(b.Contributions, "contribution")),
			fmt.Sprintf("%d of %d days empty. The green squares filed a missing person report.", empty, b.CalendarDays),
			fmt.Sprintf("%d days in a row with nothing. Not a break. A retirement.", b.QuietestRun),
		}
	case metric.Machine:
		n := metric.Thousands(b.Contributions)
		return []string{
			fmt.Sprintf("%s contributions this year. GitHub should be paying rent.", n),
			fmt.Sprintf("%s contributions in one year. The laptop needs a holiday.", n),
			fmt.Sprintf("%s contributions this year. The green squares ran out of green.", n),
		}
	default:
		return nil
	}
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
	"Active weekends":           "Chief Weekend Officer",
	"Active days":               "Senior Always-On Engineer",
	"Repos with no description": "Director of Mystery Repos",
	"One-word commit messages":  "Senior Fix Engineer",
	noReadme:                    "Head of Undocumented Features",
}

func Archetype(b Brief) string {
	switch {
	case b.Read == 0 && b.Year.Commits == 0:
		return "Stealth Mode Founder"
	case b.Shape == metric.Ghost:
		return "Director of Empty Calendars"
	case b.Shape == metric.Machine:
		return "Principal Commit Machine"
	case len(b.Unused) > 0:
		return "Principal Badge Collector"
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
