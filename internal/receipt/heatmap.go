package receipt

import (
	"image"
	"sort"
	"time"
)

// Heatmap is a contribution calendar: a column a week, a row a weekday, each
// day a square shaded by how busy it was against the account's own busy days.
// A thermal head has one ink, so the shade is a share of the square's dots.
type Heatmap struct {
	Weeks  [][7]int  // Sunday first; -1 is a day still to come
	Ending time.Time // a day in the last week, for the month labels; zero leaves them off
}

// Dots, at 203 dpi.
const (
	labelEm   = 15 // Go Mono at 15 dots to the em is 9 dots a character
	labelCell = 9
	labelRow  = 20 // height of the month row and the legend row

	// minDayPitch keeps two rows between weekday labels at least a label tall.
	minDayPitch = labelRow / 2
)

var weekdayLabels = [7]string{1: "Mon", 3: "Wed", 5: "Fri"}

func (h Heatmap) lines() []Line {
	if len(h.Weeks) == 0 {
		return nil
	}
	drawMu.Lock()
	defer drawMu.Unlock()

	width := Width() * dotsPerCol
	inner := width - 2*Gutter*dotsPerCol
	dayW := 3*labelCell + 6

	cell, gap, showDays := h.grid(len(h.Weeks), inner, dayW)
	if !showDays {
		dayW = 0
	}
	// Past what even the smallest square fits, the oldest weeks give way.
	if keep := (inner + gap) / (cell + gap); len(h.Weeks) > keep {
		h.Weeks = h.Weeks[len(h.Weeks)-keep:]
	}
	n := len(h.Weeks)
	pitch := cell + gap
	gridW := n*pitch - gap

	left := (width-dayW-gridW)/2 + dayW // the grid's left edge, the whole block centred
	top := 0
	if !h.Ending.IsZero() {
		top = labelRow + 4
	}
	gridH := 7*pitch - gap
	legendTop := top + gridH + 8
	img := image.NewAlpha(image.Rect(0, 0, width, legendTop+labelRow))

	f := facesFor(false, labelEm)
	text := func(s string, x, rowTop int) {
		f.draw(img, []rune(s), x, rowTop, labelRow, labelCell)
	}
	square := func(x, y, level int) {
		for dy := range cell {
			for dx := range cell {
				if inked(level, dx, dy) {
					img.Pix[(y+dy)*img.Stride+x+dx] = 0xFF
				}
			}
		}
	}

	for d, s := range weekdayLabels {
		if s != "" && showDays {
			text(s, left-dayW, top+d*pitch+(cell-labelRow)/2)
		}
	}
	for i, m := range h.months(pitch, gridW) {
		if m != "" {
			text(m, left+i*pitch, 0)
		}
	}
	level := h.leveller()
	for w, week := range h.Weeks {
		for d, c := range week {
			if c >= 0 {
				square(left+w*pitch, top+d*pitch, level(c))
			}
		}
	}

	// The legend sits flush with the grid's right edge, as GitHub sets its own.
	more, less := "More", "Less"
	x := left + gridW - len(more)*labelCell
	text(more, x, legendTop)
	x -= 6 + 5*pitch - gap
	for l := range shades {
		square(x+l*pitch, legendTop+(labelRow-cell)/2, l)
	}
	text(less, x-6-len(less)*labelCell, legendTop)

	r := threshold(img)
	return []Line{{Bitmap: &r, Alt: "Contribution calendar"}}
}

// shades runs from an empty day to the busiest, lightest first, as GitHub's
// greens do. A thermal head has one ink, so a lighter square is a sparser grid
// of dots: a quarter, a half, three quarters, seven eighths, then solid. The
// lightest keeps its dots two apart, closer than the gap between squares, so
// a quiet stretch still prints as separate squares rather than one grey field.
var shades = [5]func(x, y int) bool{
	func(x, y int) bool { return x%2 == 0 && y%2 == 0 },
	func(x, y int) bool { return (x+y)%2 == 0 },
	func(x, y int) bool { return x%2 == 0 || y%2 == 0 },
	func(x, y int) bool { return x%2 == 0 || y%2 == 0 || (x+y)%4 == 0 },
	func(x, y int) bool { return true },
}

// inked counts from the square's own corner, so every square of a level wears
// the same pattern.
func inked(level, x, y int) bool { return shades[level](x, y) }

// cells are the square sizes the calendar draws in, largest first, with the
// gap after each. Every side is odd, so every shade starts and ends on an
// inked dot and each square keeps a hard edge. The gap is three dots, wider
// than the lightest shade's spacing, so the squares stay apart.
var cells = []struct{ side, gap int }{{13, 3}, {11, 3}, {9, 3}, {7, 3}, {5, 3}}

// grid picks the largest square that fits the weeks across, keeping the
// weekday labels when the rows are tall enough to set them apart.
func (h Heatmap) grid(weeks, inner, dayW int) (side, gap int, days bool) {
	fits := func(side, gap, room int) bool { return weeks*(side+gap)-gap <= room }
	for _, c := range cells {
		if c.side+c.gap >= minDayPitch && fits(c.side, c.gap, inner-dayW) {
			return c.side, c.gap, true
		}
	}
	for _, c := range cells {
		if fits(c.side, c.gap, inner) {
			return c.side, c.gap, false
		}
	}
	last := cells[len(cells)-1]
	return last.side, last.gap, false
}

// leveller buckets a day by the quartiles of the days with anything on them,
// as GitHub shades its own.
func (h Heatmap) leveller() func(int) int {
	var busy []int
	for _, w := range h.Weeks {
		for _, n := range w {
			if n > 0 {
				busy = append(busy, n)
			}
		}
	}
	sort.Ints(busy)
	var cuts [3]int
	for i := range cuts {
		if len(busy) > 0 {
			cuts[i] = busy[len(busy)*(i+1)/4]
		}
	}
	return func(n int) int {
		switch {
		case n == 0:
			return 0
		case n <= cuts[0]:
			return 1
		case n <= cuts[1]:
			return 2
		case n <= cuts[2]:
			return 3
		}
		return 4
	}
}

// months labels the week a month starts in, skipping any label that would run
// into the one before it or past the grid's right edge.
func (h Heatmap) months(pitch, gridW int) []string {
	out := make([]string, len(h.Weeks))
	if h.Ending.IsZero() {
		return out
	}
	end := time.Date(h.Ending.Year(), h.Ending.Month(), h.Ending.Day(), 12, 0, 0, 0, time.UTC)
	// The calendar ends on GitHub's today, which can be a day either side of
	// the stand's around midnight. The last week's first day still to come says
	// which weekday that was, so the labels follow the data.
	last := h.Weeks[len(h.Weeks)-1]
	today := 6
	for d, n := range last {
		if n < 0 {
			today = d - 1
			break
		}
	}
	if today >= 0 {
		shift := today - int(end.Weekday())
		switch {
		case shift > 3:
			shift -= 7
		case shift < -3:
			shift += 7
		}
		end = end.AddDate(0, 0, shift)
	}
	sunday := end.AddDate(0, 0, -int(end.Weekday()))
	free := 0 // the first week a new label may start in
	for w := range h.Weeks {
		start := sunday.AddDate(0, 0, -7*(len(h.Weeks)-1-w))
		label := ""
		for d := range 7 {
			if day := start.AddDate(0, 0, d); day.Day() == 1 {
				label = day.Format("Jan")
			}
		}
		// The first column names its month too, unless a new one starts right after.
		if w == 0 && label == "" && start.AddDate(0, 0, 7*3).Month() == start.Month() {
			label = start.Format("Jan")
		}
		if label != "" && w >= free && w*pitch+3*labelCell <= gridW {
			out[w] = label
			free = w + (3*labelCell+6+pitch-1)/pitch
		}
	}
	return out
}
