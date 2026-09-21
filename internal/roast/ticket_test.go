package roast

import (
	"strings"
	"testing"
	"time"

	"github.com/upgaming/roaster/internal/event"
	"github.com/upgaming/roaster/internal/receipt"
)

func printed(r Roast) string {
	var s strings.Builder
	for _, ln := range r.Doc(event.Event{}, "001").Lines() {
		s.WriteString(ln.Text + "\n")
	}
	return s.String()
}

func TestTheHeatmapFillsTheColumnExactly(t *testing.T) {
	heat := make([][7]int, 38)
	heat[37][6] = -1
	heat[10][2] = 9

	for _, ln := range (&receipt.Doc{Blocks: []receipt.Block{receipt.Heatmap{Weeks: heat}}}).Lines() {
		if n := len([]rune(ln.Text)); n != receipt.Width-receipt.Gutter {
			t.Errorf("want each row %d columns with the gutter, got %d: %q", receipt.Width-receipt.Gutter, n, ln.Text)
		}
	}
}

func TestAnAccountWithNothingPublicStillGetsAFullReceipt(t *testing.T) {
	r := Sample("ghost")
	r.Exhibit = nil
	r.Heat = make([][7]int, 38)

	out := printed(r)

	for _, want := range []string{"technically flawless", "Days with none", "266 of 266", "A spotless calendar."} {
		if !strings.Contains(out, want) {
			t.Errorf("the receipt should say %q", want)
		}
	}
}

func TestTheWorstCommitIsQuotedWithWhereAndWhen(t *testing.T) {
	r := Sample("octocat")
	r.Exhibit = &Item{Ref: "a1b2c3d", Where: "api", Text: "asdf",
		At: time.Date(2026, time.September, 15, 3, 12, 0, 0, time.FixedZone("GST", 4*60*60))}

	out := printed(r)

	for _, want := range []string{"“asdf”", "a1b2c3d in api", "Tue 15 Sep 2026, 03:12"} {
		if !strings.Contains(out, want) {
			t.Errorf("the receipt should say %q", want)
		}
	}
}
