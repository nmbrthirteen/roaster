package receipt

import "strings"

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
