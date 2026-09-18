// Package receipt models a printed receipt as an ordered list of blocks and
// flattens it into fixed-width lines. Both the ESC/POS encoder and the HTML
// preview consume the same flattened lines, so what you design in the browser
// is byte-for-byte what the printer lays down.
package receipt

import "strings"

// Width is the printable column count in Font A. Epson 80mm heads give 42
// columns, Star and most clones give 48. Locking to 42 prints correctly on both.
const Width = 42

// Gutter is the blank columns held at each edge. Printing edge to edge reads as
// a receipt for a sandwich; the margin is what makes it read as a document.
const Gutter = 2

type Align int

const (
	AlignLeft Align = iota
	AlignCenter
	AlignRight
)

// Style carries the character attributes a single line can hold. ESC/POS
// applies these per line, not per character, so the model matches the hardware.
type Style struct {
	Bold   bool
	Double bool // double width and height
	Tall   bool // double height, single width
	Under  bool
	Invert bool // white on black
	Align  Align
}

// Cols is the column count available to content, after the gutter and any
// double-width multiplier.
func (s Style) Cols() int { return s.ColsBleed(false) }

// ColsBleed is Cols for content allowed to use the margin. Double-width text
// only gets 17 columns inside the gutter, which is too few for a masthead.
func (s Style) ColsBleed(bleed bool) int {
	w := Width
	if s.Double {
		w = Width / 2
	}
	if bleed {
		return w
	}
	return w - 2*Gutter
}

type Block interface{ lines() []Line }

// Line is the flattened unit both encoders render. Exactly one of Text, QR or
// Image carries content.
type Line struct {
	Style Style
	Text  string

	QR     string
	QRSize int // module size, 1 to 16

	Image string // asset name

	// Bleed pads to the full printable width instead of sitting inside the
	// gutter. Reversed bars need it: a bar that keeps the gutter prints the
	// margin black on the left and leaves it white on the right.
	Bleed bool

	Feed int // blank lines to emit after this line
	Cut  bool
}

type Doc struct{ Blocks []Block }

func (d *Doc) Add(b ...Block) { d.Blocks = append(d.Blocks, b...) }

func (d *Doc) Lines() []Line {
	var out []Line
	for _, b := range d.Blocks {
		for _, ln := range b.lines() {
			if ln.Text != "" {
				// Alignment is resolved here rather than left to the printer's
				// ESC a command, so the gutter survives centring.
				if ln.Bleed {
					ln.Text = pad(ln.Text, ln.Style.ColsBleed(true), ln.Style.Align)
				} else {
					ln.Text = strings.Repeat(" ", Gutter) + pad(ln.Text, ln.Style.Cols(), ln.Style.Align)
				}
				ln.Style.Align = AlignLeft
			}
			out = append(out, ln)
		}
	}
	return out
}

// Text is one or more lines of literal text, wrapped to the column width.
type Text struct {
	Value string
	Style Style
	Bleed bool // let the line use the gutter, for mastheads and rules
}

func (t Text) lines() []Line {
	var out []Line
	for _, s := range wrap(t.Value, t.Style.ColsBleed(t.Bleed)) {
		out = append(out, Line{Style: t.Style, Text: s, Bleed: t.Bleed})
	}
	return out
}

// Rule draws a full-width horizontal divider.
type Rule struct {
	Char  rune
	Style Style
}

func (r Rule) lines() []Line {
	c := r.Char
	if c == 0 {
		c = '-'
	}
	return []Line{{Style: r.Style, Text: strings.Repeat(string(c), r.Style.Cols())}}
}

// KV is a label on the left and a value hard-right, padded apart. Leader fills
// the gap, defaulting to spaces.
type KV struct {
	Label  string
	Value  string
	Leader rune
	Style  Style
}

func (k KV) lines() []Line {
	leader := k.Leader
	if leader == 0 {
		leader = ' '
	}
	cols := k.Style.Cols()
	label, value := k.Label, k.Value

	// A value never gets truncated; the label gives way first.
	if displayWidth(label)+displayWidth(value)+1 > cols {
		keep := cols - displayWidth(value) - 1
		if keep < 0 {
			keep = 0
		}
		label = truncate(label, keep)
	}
	gap := cols - displayWidth(label) - displayWidth(value)
	if gap < 1 {
		gap = 1
	}
	return []Line{{Style: k.Style, Text: label + strings.Repeat(string(leader), gap) + value}}
}

// Para is body copy: wrapped, with an optional hanging indent on every line.
type Para struct {
	Value  string
	Indent int
	Style  Style
}

func (p Para) lines() []Line {
	pad := strings.Repeat(" ", p.Indent)
	var out []Line
	for _, s := range wrap(p.Value, p.Style.Cols()-p.Indent) {
		out = append(out, Line{Style: p.Style, Text: pad + s})
	}
	return out
}

// QR prints a native QR code. The printer generates the symbol itself, so this
// costs a few bytes instead of a bitmap.
type QR struct {
	Data  string
	Size  int // module size, 1 to 16; 6 scans reliably from ~20cm
	Style Style
}

func (q QR) lines() []Line {
	size := q.Size
	if size == 0 {
		size = 6
	}
	st := q.Style
	st.Align = AlignCenter
	return []Line{{Style: st, QR: q.Data, QRSize: size}}
}

// Image prints a named 1-bit raster from the asset set.
type Image struct {
	Name  string
	Style Style
}

func (i Image) lines() []Line {
	st := i.Style
	st.Align = AlignCenter
	return []Line{{Style: st, Image: i.Name}}
}

// Feed emits blank lines.
type Feed struct{ Lines int }

func (f Feed) lines() []Line { return []Line{{Feed: f.Lines}} }

// Cut feeds clear of the cutter blade and partial-cuts the paper. The blade
// sits about 15mm above the print head, so the feed is not optional.
type Cut struct{}

func (Cut) lines() []Line { return []Line{{Feed: 5, Cut: true}} }
