package receipt

import (
	"sort"
	"strings"
)

// A thermal head has one ink and one typeface, so hierarchy comes from reversed
// video, character size and gauges.

// Section prints a full-bleed inverted bar.
type Section struct{ Label string }

func (s Section) lines() []Line {
	return []Line{{
		Style: Style{Invert: true, Bold: true},
		Text:  strings.Repeat(" ", Gutter) + s.Label,
		Bleed: true,
	}}
}

// Bar is a labelled value with a printed gauge under it.
type Bar struct {
	Label   string
	Value   string
	Percent int
	Tag     string
}

func (b Bar) lines() []Line {
	// The tag rides on the label row so the gauge can run the full column.
	value := b.Value
	if b.Tag != "" {
		value += "  " + b.Tag
	}
	out := KV{Label: b.Label, Value: value}.lines()

	cells := Width - 2*Gutter
	filled := b.Percent * cells / 100
	switch {
	case filled < 0:
		filled = 0
	case filled > cells:
		filled = cells
	}
	gauge := strings.Repeat("█", filled) + strings.Repeat("░", cells-filled)
	return append(out, Line{Text: gauge}, Line{Feed: 1})
}

// Hero is the one number people photograph, set at double size.
type Hero struct {
	Caption string
	Value   string
}

func (h Hero) lines() []Line {
	var out []Line
	if h.Caption != "" {
		out = append(out, Line{Style: Style{Align: AlignCenter}, Text: h.Caption})
	}
	return append(out, Line{Style: Style{Align: AlignCenter, Bold: true, Double: true}, Text: h.Value, Bleed: true})
}

// Tear marks the stub below the fold.
type Tear struct{}

func (Tear) lines() []Line {
	return []Line{{Text: strings.Repeat("- ", (Width-2*Gutter)/2)}}
}

// Heatmap is a contribution calendar: a column a week, a row a weekday, each
// cell shaded by how busy the day was against the account's own busy days.
type Heatmap struct {
	Weeks [][7]int // -1 is a day still to come
}

// The dot keeps an empty day visible, so a quiet stretch prints as a hole.
var shades = []rune{'·', '░', '▒', '▓', '█'}

func (h Heatmap) lines() []Line {
	var busy []int
	for _, w := range h.Weeks {
		for _, n := range w {
			if n > 0 {
				busy = append(busy, n)
			}
		}
	}
	sort.Ints(busy)
	// Quartiles of the days with anything on them, as GitHub shades its own.
	var cuts [3]int
	for i := range cuts {
		if len(busy) > 0 {
			cuts[i] = busy[len(busy)*(i+1)/4]
		}
	}
	level := func(n int) rune {
		switch {
		case n < 0:
			return ' '
		case n == 0:
			return shades[0]
		case n <= cuts[0]:
			return shades[1]
		case n <= cuts[1]:
			return shades[2]
		case n <= cuts[2]:
			return shades[3]
		}
		return shades[4]
	}

	out := make([]Line, 7)
	for d := range out {
		var row strings.Builder
		for _, w := range h.Weeks {
			row.WriteRune(level(w[d]))
		}
		out[d] = Line{Text: row.String()}
	}
	return out
}
