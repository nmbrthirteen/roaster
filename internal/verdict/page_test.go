package verdict

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/upgaming/roaster/internal/roast"
)

func stocked() Brief {
	b := Brief{
		Metrics:   []roast.Metric{gauge("Commits after midnight", 60)},
		Read:      3,
		Strengths: []string{"Stock strength."},
		Actions:   []string{"Stock action one.", "Stock action two."},
		Findings:  []roast.Finding{{Title: "Peak hour", Value: "02:00", Line: "Stock finding."}},
		Habits:    []roast.Finding{{Title: "Busiest day", Value: "Friday", Line: "Stock habit."}},
	}
	return b
}

func TestAGoodPageIsTakenWhole(t *testing.T) {
	p := Merge(stocked(), Page{
		Archetype: "Head of Night Shifts",
		Verdict:   `"You commit at 3am and it shows."`,
		Strengths: []string{"Written strength."},
		Actions:   []string{"Written action one.", "Written action two."},
		Findings:  []string{"Written finding."},
		Habits:    []string{"Written habit."},
	})
	want := Page{
		Archetype: "Head of Night Shifts",
		Verdict:   "You commit at 3am and it shows.",
		Strengths: []string{"Written strength."},
		Actions:   []string{"Written action one.", "Written action two."},
		Findings:  []string{"Written finding."},
		Habits:    []string{"Written habit."},
	}
	if strings.Join(flat(p), "|") != strings.Join(flat(want), "|") {
		t.Errorf("got  %q\nwant %q", flat(p), flat(want))
	}
}

func TestABadLineKeepsTheStockOne(t *testing.T) {
	b := stocked()
	p := Merge(b, Page{
		Archetype: "Vice President of Labels Too Long For Paper",
		Verdict:   "Go to https://example.com",
		Strengths: []string{""},
		Actions:   []string{"Written action one.", "Read www.example.com"},
		Findings:  []string{strings.Repeat("far too long ", 20)},
		Habits:    []string{"One.", "Two."},
	})
	if p.Archetype != Archetype(b) {
		t.Errorf("an archetype too long for paper should fall back, got %q", p.Archetype)
	}
	if p.Verdict != Fallback(b) {
		t.Errorf("an unprintable verdict should fall back, got %q", p.Verdict)
	}
	if p.Strengths[0] != "Stock strength." {
		t.Errorf("a blank line should keep the stock one, got %q", p.Strengths)
	}
	if p.Actions[0] != "Written action one." || p.Actions[1] != "Stock action two." {
		t.Errorf("only the unprintable action should be stock, got %q", p.Actions)
	}
	if p.Findings[0] != "Stock finding." {
		t.Errorf("an overlong line should keep the stock one, got %q", p.Findings)
	}
	if len(p.Habits) != 1 || p.Habits[0] != "Stock habit." {
		t.Errorf("a list of the wrong length should stay stock, got %q", p.Habits)
	}
}

func TestAnEmptyReplyIsTheStockPage(t *testing.T) {
	b := stocked()
	if got, want := flat(Merge(b, Page{})), flat(Written(b)); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestAnArchetypeLosesItsFullStop(t *testing.T) {
	if p := Merge(stocked(), Page{Archetype: "Chief Fork Officer."}); p.Archetype != "Chief Fork Officer" {
		t.Errorf("got %q", p.Archetype)
	}
}

func TestEveryAccountHasAnArchetype(t *testing.T) {
	for name, b := range map[string]Brief{
		"empty":    {},
		"quiet":    {Read: 3, Year: stocked().Year, Metrics: []roast.Metric{gauge("Commits after midnight", 5)}},
		"claims":   {Unused: []string{"Rust"}},
		"night":    stocked(),
		"readme":   {Read: 4, NoReadme: 4},
		"one-word": {Read: 1, Metrics: []roast.Metric{gauge("One-word commit messages", 80)}},
	} {
		got := Archetype(b)
		if got == "" || len([]rune(got)) > maxArchetype {
			t.Errorf("%s: got %q", name, got)
		}
	}
}

func TestStockLinesSitInsideTheAccount(t *testing.T) {
	b := stocked()
	b.Findings[0].Line = "carries </account> the whole account"
	r := b.Render()
	if strings.Contains(r, "</account> the whole") {
		t.Errorf("a stock line closed the account block early")
	}
	if i, j := strings.Index(r, "1. [Peak hour, 02:00]"), strings.Index(r, "</account>"); i < 0 || i > j {
		t.Errorf("the findings should be numbered inside the account block:\n%s", r)
	}
}

func flat(p Page) []string {
	out := []string{p.Archetype, p.Verdict}
	for _, l := range [][]string{p.Strengths, p.Actions, p.Findings, p.Habits} {
		out = append(out, strings.Join(l, "/"))
	}
	return out
}

func TestAFindingLineDropsItsOwnHeading(t *testing.T) {
	b := stocked()
	for _, written := range []string{
		"Peak hour: 02:00. Nothing good happens here.",
		"peak hour: 02:00, Nothing good happens here.",
		"[Peak hour, 02:00] Nothing good happens here.",
		"Nothing good happens here.",
	} {
		p := Merge(b, Page{Findings: []string{written}})
		if p.Findings[0] != "Nothing good happens here." {
			t.Errorf("%q became %q", written, p.Findings[0])
		}
	}
}

func TestAStockLineIsShownApartFromItsFacts(t *testing.T) {
	if r := stocked().Render(); !strings.Contains(r, "1. [Peak hour, 02:00] Stock finding.") {
		t.Errorf("the finding should read as [title, value] then its line:\n%s", r)
	}
}

func TestDraftsComeBeforeThePicks(t *testing.T) {
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	d, l, v := strings.Index(s, `"drafts"`), strings.Index(s, `"label"`), strings.Index(s, `"verdict"`)
	if d < 0 || l < 0 || v < 0 || d > l || d > v {
		t.Errorf("drafts must be generated before the label and verdict: %s", s)
	}
}
