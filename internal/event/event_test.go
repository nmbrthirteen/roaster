package event

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormaliseFillsAnEmptyEvent(t *testing.T) {
	ev := Event{Code: "bare"}
	ev.Normalise()

	if ev.Receipt.Logo == "" || ev.Receipt.ShareBase == "" || ev.Receipt.Title == "" {
		t.Fatalf("an event made from a form must still print a logo and a QR target: %+v", ev.Receipt)
	}
	if ev.Name != "bare" {
		t.Errorf("Name = %q, want the code as a fallback", ev.Name)
	}
	if got := ev.ShareURL("abc12"); got != "https://lifeat.upgaming.com/k/abc12" {
		t.Errorf("ShareURL = %q", got)
	}
}

func TestNormaliseKeepsWhatIsSet(t *testing.T) {
	ev := Event{Code: "x", Name: "Real Name"}
	ev.Receipt.Title = "Annual code appraisal"
	ev.Receipt.ShareBase = "https://example.test/k/"
	ev.Normalise()

	if ev.Name != "Real Name" || ev.Receipt.Title != "Annual code appraisal" {
		t.Errorf("Normalise overwrote set fields: %+v", ev)
	}
	if got := ev.ShareURL("abc12"); got != "https://example.test/k/abc12" {
		t.Errorf("ShareURL = %q, want the trailing slash collapsed", got)
	}
}

func TestRuleFallbacks(t *testing.T) {
	var ev Event
	if ev.RuleChar() != '-' || ev.HeavyRuleChar() != '=' {
		t.Errorf("dividers must fall back rather than vanish")
	}
	ev.Receipt.Rule, ev.Receipt.Heavy = "*", "#"
	if ev.RuleChar() != '*' || ev.HeavyRuleChar() != '#' {
		t.Errorf("set dividers must win")
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Berlin JS Conf 2026": "berlin-js-conf-2026",
		"  SF AI Summit 2025": "sf-ai-summit-2025",
		"Tbilisi // Dev Days": "tbilisi-dev-days",
		"!!!":                 "",
	} {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestABrokenEventFileIsSkippedNotFatal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	set, err := Load(dir)
	if err != nil {
		t.Fatalf("one bad file should not stop the stand, got %v", err)
	}
	if len(set.Skipped) != 1 || !strings.HasPrefix(set.Skipped[0], "broken.json") {
		t.Errorf("the bad file should be named for the menu, got %v", set.Skipped)
	}
	if len(set.Codes()) == 0 {
		t.Errorf("the built-in events should still load")
	}
}
