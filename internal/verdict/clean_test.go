package verdict

import (
	"strings"
	"testing"
)

func TestCleanTidiesWhatTheModelWrote(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`"Your commits are one word long."`, "Your commits are one word long."},
		{"“Quoted the posh way.”", "Quoted the posh way."},
		{"**Bold** move,\n\ncommitting at 3am.", "Bold move, committing at 3am."},
		{"# A heading it should not have written", "A heading it should not have written"},
		{"You claim C# and write C.", "You claim C# and write C."},
		{`"Day 1: Schema." 328 days later, day two never showed.`, `"Day 1: Schema." 328 days later, day two never showed.`},
		{`"fix" and "wip". The whole history.`, `"fix" and "wip". The whole history.`},
	} {
		got, ok := Clean(c.in)
		if !ok || got != c.want {
			t.Errorf("Clean(%q) = %q, %v; want %q", c.in, got, ok, c.want)
		}
	}
}

// Anything that should not reach paper is thrown away, and the numbers write
// the line instead.
func TestCleanRefusesWhatShouldNotPrint(t *testing.T) {
	for _, in := range []string{
		"",
		"   ",
		`""`,
		"Your code is shit.",
		"Visit https://example.com for a prize.",
		"Scan www.example.com now.",
	} {
		if got, ok := Clean(in); ok {
			t.Errorf("Clean(%q) let through %q", in, got)
		}
	}
}

// A word that merely contains a blocked one is not blocked.
func TestCleanDoesNotTripOnInnocentWords(t *testing.T) {
	for _, in := range []string{"You wrote a Dickens-length README.", "Scunthorpe would be proud.", "Cocktail-hour commits."} {
		if _, ok := Clean(in); !ok {
			t.Errorf("Clean(%q) was refused over a word it only contains", in)
		}
	}
}

func TestALongVerdictIsCutAtASentence(t *testing.T) {
	long := "First sentence lands. " + strings.Repeat("And then it keeps going without a break ", 20)
	got, ok := Clean(long)
	if !ok {
		t.Fatal("a long verdict should be cut, not refused")
	}
	if len([]rune(got)) > maxVerdict {
		t.Errorf("the verdict ran to %d characters, past %d", len([]rune(got)), maxVerdict)
	}
	if !strings.HasSuffix(got, ".") && !strings.HasSuffix(got, "…") {
		t.Errorf("the cut should land on a sentence or a word, got %q", got)
	}
}
