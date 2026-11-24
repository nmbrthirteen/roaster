package linkedin

import "github.com/upgaming/roaster/internal/verdict"

// fallbackLines are keyed by the gauge they are about, so each one only states
// what was measured. The fact comes first and the joke second, with a few per
// gauge because the person behind in the queue read the last one.
var fallbackLines = map[string][]string{
	"Titles with a big word": {
		"%s of your titles claim rank. Everyone is a leader here, nobody is led.",
		"Rank in %s of your titles. Somebody has to do the actual work.",
	},
	"Jobs under 18 months": {
		"%s of your jobs ended inside 18 months. The farewell cake costs more than the onboarding.",
		"Gone inside 18 months from %s of your jobs. HR knows you by your exit interview.",
	},
	"Roles with no description": {
		"%s of your roles have no description. The job title is doing all the work.",
		"No description on %s of your roles. Whatever happened there, it stays there.",
	},
	"Skills no job backs up": {
		"%s of your skills appear in no job. A wishlist with a profile photo.",
		"No job mentions %s of your skills. Endorsed by friends, proven by nobody.",
	},
	"Headline in buzzwords": {
		"%s of your headline is buzzwords. A motivational poster with a login.",
		"Buzzwords make up %s of your headline. Recruiters skim past it, and so would I.",
	},
}

var fallbackOrder = []string{
	"Headline in buzzwords", "Jobs under 18 months", "Titles with a big word",
	"Skills no job backs up", "Roles with no description",
}

// Fallback writes the verdict from the numbers when the model cannot. It only
// says what was measured, and it never fails.
func Fallback(b Brief) string {
	if b.Empty {
		return verdict.Fresh(b.Avoid, "",
			"An empty LinkedIn profile. The only one on the site with nothing to exaggerate.",
			"No roles, no headline, no About. Somehow still more honest than most profiles.",
		)
	}
	best, value, high := "", "", -1
	for _, label := range fallbackOrder {
		for _, m := range b.Metrics {
			if m.Label == label && m.Percent != nil && *m.Percent > high && *m.Percent >= 33 {
				best, value, high = label, m.Value, *m.Percent
			}
		}
	}
	if best != "" {
		return verdict.Fresh(b.Avoid, value, fallbackLines[best]...)
	}
	return verdict.Fresh(b.Avoid, "",
		"A tidy, honest profile. On LinkedIn, that is the most suspicious thing of all.",
		"Nothing here is inflated. Recruiters will assume you are hiding something.",
	)
}

// Archetype is the parody job title when the model writes none.
func Archetype(b Brief) string {
	switch {
	case b.Empty:
		return "Senior Profile Owner"
	case b.Shape == Hopper:
		return "Head of Leaving Early"
	case b.Shape == Crowned:
		return "Chief of Many Titles"
	case b.Shape == Lifer:
		return "Principal Furniture"
	case len(b.Buzz) >= 3:
		return "VP of Buzzwords"
	case len(b.Unbacked) >= 5:
		return "Director of Wishlists"
	case b.Shape == Fresh:
		return "Junior Visionary"
	}
	return "Suspiciously Normal Professional"
}

// Written is the page from the numbers alone: what prints when there is no
// model, and what every line the model writes is laid over.
func Written(b Brief) verdict.Page {
	return verdict.Page{
		Archetype: Archetype(b),
		Verdict:   Fallback(b),
		Strengths: b.Strengths,
		Actions:   b.Actions,
		Findings:  verdict.LinesOf(b.Findings),
		Habits:    verdict.LinesOf(b.Habits),
	}
}

func Merge(b Brief, p verdict.Page) verdict.Page {
	return verdict.Lay(Written(b), b.Findings, b.Habits, p)
}
