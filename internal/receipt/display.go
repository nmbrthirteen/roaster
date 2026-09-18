package receipt

import "strings"

// A thermal head has one ink and one typeface. Hierarchy has to come from
// reversed video, character size, and gauges rather than weight and colour.

// Section prints a full-bleed inverted bar. It is the strongest break the
// hardware can draw, so it marks every major division and nothing else.
type Section struct{ Label string }

func (s Section) lines() []Line {
	return []Line{{
		Style: Style{Invert: true, Bold: true},
		Text:  strings.Repeat(" ", Gutter) + s.Label,
		Bleed: true,
	}}
}

// Bar is a labelled value with a printed gauge under it. A number whose length
// you can see gets photographed; the same number in a table does not.
type Bar struct {
	Label   string
	Value   string
	Percent int
	Tag     string
}

func (b Bar) lines() []Line {
	// The tag rides on the label row so the gauge can run the full column. A
	// gauge that stops two thirds across reads as a rendering bug.
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

// Hero is the one number people photograph, set at double size. The caption
// prints above it: read alone, a big "89 / 100" looks like a good grade.
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

// Tear marks the stub below the fold. Spaced dashes read as a perforation in a
// way a solid rule does not.
type Tear struct{}

func (Tear) lines() []Line {
	return []Line{{Text: strings.Repeat("- ", (Width-2*Gutter)/2)}}
}
