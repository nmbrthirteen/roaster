package metric

import (
	"fmt"
	"testing"
	"time"

	"github.com/upgaming/roaster/internal/github"
	"github.com/upgaming/roaster/internal/roast"
)

// at builds a commit at a wall-clock time in a fixed offset, which is how
// GitHub hands them over and the only reason the night metric means anything.
func at(day int, hour int, message string) github.Commit {
	zone := time.FixedZone("GST", 4*60*60)
	return github.Commit{
		Repo:    "roaster",
		Message: message,
		At:      time.Date(2026, time.September, day, hour, 0, 0, 0, zone),
	}
}

func find(t *testing.T, metrics []roast.Metric, label string) roast.Metric {
	t.Helper()
	for _, m := range metrics {
		if m.Label == label {
			return m
		}
	}
	t.Fatalf("no metric called %q", label)
	return roast.Metric{}
}

func TestTheReceiptGetsFiveGaugesAndOnePlainRow(t *testing.T) {
	metrics := From(github.Facts{})

	if len(metrics) != 6 {
		t.Fatalf("the document is laid out for six rows, got %d", len(metrics))
	}
	for i, m := range metrics[:5] {
		if m.Percent == nil {
			t.Errorf("row %d should carry a bar", i)
		}
		if m.Tag == "" {
			t.Errorf("row %d should carry a tag; an untagged row leaves the column ragged", i)
		}
	}
	if metrics[5].Percent != nil {
		t.Errorf("the last row is a plain number, not a gauge")
	}
}

// An account with nothing public still has to print.
func TestAnEmptyAccountStillPrints(t *testing.T) {
	metrics := From(github.Facts{})

	for _, m := range metrics[:5] {
		if m.Value != "0%" || *m.Percent != 0 {
			t.Errorf("%s should read as zero, got %q", m.Label, m.Value)
		}
	}
	if got := find(t, metrics, "Longest quiet stretch").Value; got != "unknown" {
		t.Errorf("with no calendar the gap is unknown, got %q", got)
	}
}

func TestTheNumbersAreTheArithmetic(t *testing.T) {
	// 20:00, 02:00, 03:00, 04:00: three of four after midnight.
	// The calendar week of the 14th has three active days of seven, and one
	// of its two weekend days. Two of the four headlines are a single word.
	f := github.Facts{
		Commits: []github.Commit{
			at(18, 20, "Stop the printed receipt reading as random"),
			at(19, 2, "fix"),
			at(20, 3, "wip"),
			at(21, 4, "Split the HTTP surface out of main"),
		},
		Repos: []github.Repo{
			{Name: "a", Description: "does a thing"},
			{Name: "b"},
			{Name: "c", Description: "   "},
		},
	}
	for i, n := range []int{1, 0, 0, 1, 0, 1, 0} {
		f.Year.Days = append(f.Year.Days, github.Day{Date: time.Date(2026, time.September, 14+i, 0, 0, 0, 0, time.UTC), Count: n})
	}

	metrics := From(f)
	for _, want := range []struct {
		label string
		value string
		tag   string
	}{
		{"Commits after midnight", "75%", "vampire"},
		{"Active weekends", "50%", "restless"},
		{"Active days", "43%", "committed"},
		{"Repos with no description", "67%", "ghosted"},
		{"One-word commit messages", "50%", "brief"},
	} {
		got := find(t, metrics, want.label)
		if got.Value != want.value {
			t.Errorf("%s: got %q, want %q", want.label, got.Value, want.value)
		}
		if got.Tag != want.tag {
			t.Errorf("%s: tag %q, want %q", want.label, got.Tag, want.tag)
		}
	}
}

func year(active func(i int) int) []github.Day {
	start := time.Date(2025, time.September, 22, 0, 0, 0, 0, time.UTC)
	out := make([]github.Day, 365)
	for i := range out {
		out[i] = github.Day{Date: start.AddDate(0, 0, i), Count: active(i)}
	}
	return out
}

func someRepos(n int, described bool) []github.Repo {
	out := make([]github.Repo, n)
	for i := range out {
		out[i] = github.Repo{Name: fmt.Sprint("r", i), Readme: described}
		if described {
			out[i].Description = "does a thing"
		}
	}
	return out
}

func TestTheScoreTellsAccountsApart(t *testing.T) {
	ghost := github.Facts{
		Repos: someRepos(1, false),
		Year: github.Year{Days: year(func(i int) int {
			if i == 10 || i == 340 || i == 350 {
				return 3
			}
			return 0
		})},
	}
	balanced := github.Facts{
		Repos: someRepos(10, true),
		Year: github.Year{Days: year(func(i int) int {
			if i%7 == 1 || i%7 == 3 || i%7 == 4 {
				return 3
			}
			return 0
		})},
	}
	grinder := github.Facts{
		Repos: someRepos(15, true),
		Year:  github.Year{Days: year(func(i int) int { return 20 + i%3 })},
	}
	machine := github.Facts{
		Repos: someRepos(99, true),
		Year:  github.Year{Days: year(func(i int) int { return 800 })},
	}

	for _, c := range []struct {
		name  string
		f     github.Facts
		shape Shape
	}{
		{"ghost", ghost, Ghost},
		{"balanced", balanced, Regular},
		{"grinder", grinder, Grinder},
		{"machine", machine, Machine},
	} {
		if got := ShapeOf(c.f); got != c.shape {
			t.Errorf("%s: shape %q, want %q", c.name, got, c.shape)
		}
	}

	g, b, gr, m := Score(ghost), Score(balanced), Score(grinder), Score(machine)
	if b >= 40 {
		t.Errorf("a balanced account should score low, got %d", b)
	}
	if g < 75 || m < 75 {
		t.Errorf("both extremes should score high: ghost %d, machine %d", g, m)
	}
	if !(b < gr && gr < m) {
		t.Errorf("more grind should score higher: balanced %d, grinder %d, machine %d", b, gr, m)
	}
	if m-gr < 10 {
		t.Errorf("a machine should clearly outscore a grinder: %d vs %d", m, gr)
	}
}

func TestAnEmptyAccountScoresAsNeglected(t *testing.T) {
	f := github.Facts{Year: github.Year{Days: year(func(int) int { return 0 })}}
	if got := Score(f); got < 75 {
		t.Errorf("an account with nothing in it is the most roastable kind, got %d", got)
	}
}

func TestTheLongestGapIsTheLongestRunOfEmptyDays(t *testing.T) {
	day := func(n, count int) github.Day {
		return github.Day{Date: time.Date(2026, time.March, n, 0, 0, 0, 0, time.UTC), Count: count}
	}
	f := github.Facts{Year: github.Year{Days: []github.Day{
		day(1, 3), day(2, 0), day(3, 0), day(4, 0), day(5, 1), day(6, 0), day(7, 2),
	}}}

	if got := find(t, From(f), "Longest quiet stretch").Value; got != "3 days" {
		t.Errorf("got %q, want \"3 days\"", got)
	}
}

func TestOneEmptyDayIsADayNotDays(t *testing.T) {
	f := github.Facts{Year: github.Year{Days: []github.Day{
		{Date: time.Now(), Count: 1}, {Date: time.Now(), Count: 0}, {Date: time.Now(), Count: 4},
	}}}

	if got := find(t, From(f), "Longest quiet stretch").Value; got != "1 day" {
		t.Errorf("got %q, want \"1 day\"", got)
	}
}
