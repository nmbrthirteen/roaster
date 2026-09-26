package verdict

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/upgaming/roaster/internal/metric"
	"github.com/upgaming/roaster/internal/roast"
)

func gauge(label string, n int) roast.Metric {
	return roast.Metric{Label: label, Value: strconv.Itoa(n) + "%", Percent: &n}
}

// A claim the code does not back up is the best joke on offer, and it is true.
func TestAnUnbackedClaimWins(t *testing.T) {
	b := Brief{
		Unused:  []string{"Haskell", "Rust"},
		Metrics: []roast.Metric{gauge("Commits after midnight", 90)},
	}
	got := Fallback(b)
	if !strings.Contains(got, "Haskell and Rust") {
		t.Errorf("got %q", got)
	}
}

func TestTheWorstGaugeIsTheOneMentioned(t *testing.T) {
	b := Brief{Metrics: []roast.Metric{
		gauge("Commits after midnight", 40),
		gauge("One-word commit messages", 71),
		gauge("Repos with no description", 50),
		{Label: "Longest quiet stretch", Value: "12 days"},
	}}
	got := Fallback(b)
	if !strings.HasPrefix(got, "71% of commit messages") {
		t.Errorf("got %q", got)
	}
}

func TestAQuietAccountStillGetsALine(t *testing.T) {
	b := Brief{Metrics: []roast.Metric{gauge("Commits after midnight", 5)}}
	if got := Fallback(b); got == "" || strings.Contains(got, "%") {
		t.Errorf("a quiet account should get the quiet line, got %q", got)
	}
}

func TestAGhostIsRoastedForBeingEmpty(t *testing.T) {
	b := Brief{
		Shape: metric.Ghost, Contributions: 9, ActiveDays: 5, CalendarDays: 264, QuietestRun: 223, Read: 1,
		Metrics: []roast.Metric{gauge("Repos with no description", 100)},
	}
	if got := Fallback(b); !strings.Contains(got, "9 contributions") {
		t.Errorf("a near empty account should hear about its emptiness, got %q", got)
	}
	if got := Archetype(b); got != "Director of Empty Calendars" {
		t.Errorf("archetype %q", got)
	}
}

func TestAMachineIsRoastedForTheVolume(t *testing.T) {
	b := Brief{
		Shape: metric.Machine, Contributions: 298858, Read: 15,
		Unused:  []string{"JavaScript"},
		Metrics: []roast.Metric{gauge("Active days", 99)},
	}
	got := Fallback(b)
	if !strings.Contains(got, "298,858") {
		t.Errorf("a huge account should hear about the volume, got %q", got)
	}
	if _, ok := Clean(got); !ok {
		t.Errorf("the machine line would not print: %q", got)
	}
}

func TestNoFallbackLineNarrates(t *testing.T) {
	briefs := []Brief{
		{},
		{Unused: []string{"Rust"}, Read: 30},
		{Unused: []string{"Rust"}, Read: 1},
		{Shape: metric.Ghost, Contributions: 9, ActiveDays: 5, CalendarDays: 264, QuietestRun: 223},
		{Shape: metric.Machine, Contributions: 298858},
	}
	var all []string
	for _, b := range briefs {
		for range 3 {
			line := Fallback(b)
			all = append(all, line)
			b.Avoid = append(b.Avoid, line)
		}
	}
	for _, options := range lines {
		for _, o := range options {
			all = append(all, fmt.Sprintf(o, "80%"))
		}
	}
	for _, l := range all {
		if !direct(l) {
			t.Errorf("a fallback narrates instead of punching: %q", l)
		}
	}
}

func TestTheCorrectionShapeIsReplacedByACleanDraft(t *testing.T) {
	for _, v := range []string{
		"Init. That is not a commit message. That is you shrugging at git.",
		"A whole package so Electron can right click. That is not a library, that is a menu.",
		"Update 02_data_insertion.sql is not a message. It is you reading the filename.",
		"This isn't a portfolio, it's a warning.",
	} {
		if direct(v) {
			t.Errorf("%q uses the correction shape and should be rejected", v)
		}
	}
	for _, v := range []string{
		"A repo named test is running production. I have seen braver deploys.",
		"Not a streak. A hostage situation.",
		"Nothing about this is final, and we both know it.",
	} {
		if !direct(v) {
			t.Errorf("%q is a direct line and should pass", v)
		}
	}

	var p Page
	p.Verdict = "Init. That is not a commit message. That is you shrugging."
	p.Drafts.Verdicts = []string{"This isn't a repo, it's a note.", "Init, and then silence. I have seen tombstones with more detail."}
	if got := Merge(Brief{}, p).Verdict; got != p.Drafts.Verdicts[1] {
		t.Errorf("the first clean draft should replace a worn verdict, got %q", got)
	}
}

func TestANarratedVerdictIsReplaced(t *testing.T) {
	for _, v := range []string{"You commit at 3am.", "Your repos are empty.", "You’re busy.", "you've shipped nothing."} {
		if direct(v) {
			t.Errorf("%q opens with you and should be rejected", v)
		}
	}
	if !direct("Youtube-dl fork number four. Bold.") {
		t.Errorf("a word that merely starts with you is not narration")
	}
}

// Every fallback line has to be printable by the same rule the model's are.
func TestEveryFallbackLinePrints(t *testing.T) {
	for label, options := range lines {
		for _, o := range options {
			if _, ok := Clean(fmt.Sprintf(o, "80%")); !ok {
				t.Errorf("a fallback for %q would not survive Clean: %q", label, o)
			}
		}
	}
	for _, b := range []Brief{{}, {Unused: []string{"Rust"}}, {Unused: []string{"Rust", "Go", "Zig"}}} {
		if _, ok := Clean(Fallback(b)); !ok {
			t.Errorf("a fallback would not survive Clean: %q", Fallback(b))
		}
	}
}

// For an account with little else to say, the missing READMEs are the line.
func TestMissingReadmesCanBeTheLine(t *testing.T) {
	b := Brief{
		Read:     10,
		NoReadme: 8,
		Metrics:  []roast.Metric{gauge("Commits after midnight", 20)},
	}
	if got := Fallback(b); !strings.HasPrefix(got, "80% of repos have no README") {
		t.Errorf("got %q", got)
	}
}

func TestAWorseGaugeStillBeatsMissingReadmes(t *testing.T) {
	b := Brief{
		Read:     10,
		NoReadme: 4,
		Metrics:  []roast.Metric{gauge("One-word commit messages", 90)},
	}
	if got := Fallback(b); !strings.HasPrefix(got, "90% of commit messages") {
		t.Errorf("got %q", got)
	}
}

// The person behind in the queue has read the last receipt.
func TestTheFallbackDoesNotRepeatItself(t *testing.T) {
	b := Brief{Metrics: []roast.Metric{gauge("One-word commit messages", 80)}}

	seen := map[string]bool{}
	for range len(lines["One-word commit messages"]) {
		line := Fallback(b)
		if seen[line] {
			t.Fatalf("printed %q twice while fresh lines were left", line)
		}
		seen[line] = true
		b.Avoid = append(b.Avoid, line)
	}

	// Every line used: it starts again rather than printing nothing.
	if got := Fallback(b); got == "" {
		t.Errorf("with every line used, the fallback should still print one")
	}
}
