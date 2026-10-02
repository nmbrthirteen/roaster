package receipt

import (
	"testing"
	"time"
)

func TestTheHeatmapStaysInsideTheMarginsAtEveryWidth(t *testing.T) {
	defer SetWidth(WideColumns)
	for _, weeks := range []int{1, 38, 53, 80} {
		heat := make([][7]int, weeks)
		for w := range heat {
			heat[w][w%7] = w + 1
		}
		for _, cols := range []int{WideColumns, NarrowColumns, SmallColumns} {
			SetWidth(cols)
			lines := Heatmap{Weeks: heat, Ending: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)}.lines()
			if len(lines) != 1 || lines[0].Bitmap == nil {
				t.Fatalf("%d weeks on %d columns: want one graphic, got %d lines", weeks, cols, len(lines))
			}
			r := *lines[0].Bitmap
			if r.Width != cols*dotsPerCol {
				t.Errorf("%d weeks on %d columns: want %d dots wide, got %d", weeks, cols, cols*dotsPerCol, r.Width)
			}
			margin := Gutter * dotsPerCol
			for y := range r.Height {
				for x := range margin {
					if r.at(x, y) || r.at(r.Width-1-x, y) {
						t.Fatalf("%d weeks on %d columns: ink in the margin at %d,%d", weeks, cols, x, y)
					}
				}
			}
		}
	}
}

func TestABusierDayInksMoreOfItsSquare(t *testing.T) {
	for _, c := range cells {
		prev := -1
		for l := range shades {
			n := 0
			for y := range c.side {
				for x := range c.side {
					if inked(l, x, y) {
						n++
					}
				}
			}
			if n <= prev {
				t.Errorf("%d dot square: level %d inks %d dots, no more than level %d", c.side, l, n, l-1)
			}
			prev = n
		}
	}
}

func TestEveryShadeReachesTheSquaresCorners(t *testing.T) {
	for _, c := range cells {
		e := c.side - 1
		for l := range shades {
			if !inked(l, 0, 0) || !inked(l, e, 0) || !inked(l, 0, e) || !inked(l, e, e) {
				t.Errorf("%d dot square: level %d leaves a corner bare, so the square loses its edge", c.side, l)
			}
		}
	}
}

func TestMonthLabelsLandOnTheWeekTheMonthStarts(t *testing.T) {
	// Fri 2 Oct 2026; the last column is the week of Sun 27 Sep.
	h := Heatmap{Weeks: make([][7]int, 5), Ending: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)}
	got := h.months(16, 5*16+3*labelCell)
	// Weeks start 30 Aug, 6 Sep, 13 Sep, 20 Sep, 27 Sep: September starts in the
	// first, October in the last.
	want := []string{"Sep", "", "", "", "Oct"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("want %q, got %q", want, got)
		}
	}
}

func TestAMonthLabelThatWouldRunOffTheGridIsLeftOff(t *testing.T) {
	h := Heatmap{Weeks: make([][7]int, 5), Ending: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)}
	if got := h.months(16, 5*16-3); got[4] != "" {
		t.Errorf("October starts in the last column but has no room there, got %q", got)
	}
}
