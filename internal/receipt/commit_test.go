package receipt

import (
	"strings"
	"testing"
)

func TestTheCommitCardFitsEveryPaperWidth(t *testing.T) {
	defer SetWidth(WideColumns)
	c := Commit{
		Ref:     "a1b2c3d",
		Where:   "a-repository-name-far-too-long-for-any-receipt-to-carry",
		When:    "Tue 15 Sep 2026, 03:12",
		Message: strings.Repeat("fix the thing again ", 4),
	}
	for _, cols := range []int{WideColumns, NarrowColumns, SmallColumns} {
		SetWidth(cols)
		lines := c.lines()
		if len(lines) != 1 || lines[0].Bitmap == nil {
			t.Fatalf("%d columns: want one graphic, got %d lines", cols, len(lines))
		}
		if w := lines[0].Bitmap.Width; w != cols*dotsPerCol {
			t.Errorf("%d columns: want the card %d dots wide, got %d", cols, cols*dotsPerCol, w)
		}
		if !strings.Contains(lines[0].Alt, "fix the thing") {
			t.Errorf("%d columns: the alt text should carry the message, got %q", cols, lines[0].Alt)
		}
	}
}
