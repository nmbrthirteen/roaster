package verdict

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

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
		{Label: "Longest gap between commits", Value: "12 days"},
	}}
	got := Fallback(b)
	if !strings.HasPrefix(got, "71% of your commit messages") {
		t.Errorf("got %q", got)
	}
}

func TestAQuietAccountStillGetsALine(t *testing.T) {
	b := Brief{Metrics: []roast.Metric{gauge("Commits after midnight", 5)}}
	if got := Fallback(b); got == "" || strings.Contains(got, "%") {
		t.Errorf("a quiet account should get the quiet line, got %q", got)
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
	if got := Fallback(b); !strings.HasPrefix(got, "80% of your repos have no README") {
		t.Errorf("got %q", got)
	}
}

func TestAWorseGaugeStillBeatsMissingReadmes(t *testing.T) {
	b := Brief{
		Read:     10,
		NoReadme: 4,
		Metrics:  []roast.Metric{gauge("One-word commit messages", 90)},
	}
	if got := Fallback(b); !strings.HasPrefix(got, "90% of your commit messages") {
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
