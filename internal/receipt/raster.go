package receipt

import (
	"bytes"
	"image"
	"sync"
	"sync/atomic"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Some printers ship with a broken or missing character font: the logo and the
// QR come out sharp while every letter prints as a solid black cell. Drawing
// the text here and sending it as the same raster the logo uses takes the
// printer's font out of the job entirely, at the cost of a bigger job.
var textAsImage atomic.Bool

func init() { textAsImage.Store(true) }

// TextAsImage reports whether text is drawn here rather than by the printer.
func TextAsImage() bool { return textAsImage.Load() }

func SetTextAsImage(v bool) { textAsImage.Store(v) }

// glyphDots is the height of a Font A cell, which the drawn text matches so a
// receipt is the same length whichever way it prints.
const glyphDots = 24

// bandRows caps one GS v 0 command, since cheaper printers buffer a whole
// command before they print any of it.
const bandRows = 256

var faces struct {
	once          sync.Once
	regular, bold font.Face
	ascent        int
}

func loadFaces() {
	faces.once.Do(func() {
		// Go Mono advances 0.6 em, so 20 dots gives Font A's 12 dot column.
		open := func(ttf []byte) font.Face {
			f, err := opentype.Parse(ttf)
			if err != nil {
				panic(err)
			}
			face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: 20, DPI: 72, Hinting: font.HintingFull})
			if err != nil {
				panic(err)
			}
			return face
		}
		faces.regular = open(gomono.TTF)
		faces.bold = open(gomonobold.TTF)
		m := faces.regular.Metrics()
		// Centre the font's full height in the cell.
		pad := (glyphDots - (m.Ascent + m.Descent).Ceil()) / 2
		faces.ascent = pad + m.Ascent.Ceil()
	})
}

// band collects drawn text lines until something that is not text, a QR or the
// logo or the cutter, needs the printer.
type band struct {
	width int // dots
	rows  [][]byte
}

func newBand() *band {
	return &band{width: Width() * dotsPerCol}
}

func (bd *band) bytesPerRow() int { return (bd.width + 7) / 8 }

func (bd *band) feed(dots int) {
	for i := 0; i < dots; i++ {
		bd.rows = append(bd.rows, make([]byte, bd.bytesPerRow()))
	}
}

// text draws one flattened line, scaled the way the printer scales its own
// font: each dot repeated, so double size looks as it would natively.
func (bd *band) text(s string, st Style) {
	loadFaces()
	runes := []rune(s)
	cell := image.NewAlpha(image.Rect(0, 0, len(runes)*dotsPerCol, glyphDots))

	face := faces.regular
	if st.Bold {
		face = faces.bold
	}
	for i, r := range runes {
		x := i * dotsPerCol
		if !drawBlock(cell, x, r) {
			dr := font.Drawer{Dst: cell, Src: image.Opaque, Face: face,
				Dot: fixed.P(x, faces.ascent)}
			dr.DrawString(string(printable(r)))
		}
	}
	if st.Under {
		for x := 0; x < cell.Rect.Dx(); x++ {
			cell.Pix[(glyphDots-2)*cell.Stride+x] = 0xFF
		}
	}

	sx, sy, pitch := 1, 1, LineSpacing
	switch {
	case st.Double:
		sx, sy, pitch = 2, 2, TallSpacing
	case st.Tall:
		sy, pitch = 2, TallSpacing
	}

	for y := 0; y < pitch; y++ {
		row := make([]byte, bd.bytesPerRow())
		gy := y / sy
		for x := 0; x < bd.width; x++ {
			gx := x / sx
			if gx >= cell.Rect.Dx() {
				break
			}
			// Reversed video fills the line's own cells, spaces included, as the
			// printer does, and leaves the gap below the glyphs as paper.
			on := gy < glyphDots && cell.Pix[gy*cell.Stride+gx] >= 0x80
			if st.Invert && gy < glyphDots {
				on = !on
			}
			if on {
				row[x/8] |= 0x80 >> (x % 8)
			}
		}
		bd.rows = append(bd.rows, row)
	}
}

// flush hands everything collected so far to the printer.
func (bd *band) flush(b *bytes.Buffer) {
	if len(bd.rows) == 0 {
		return
	}
	writeAlign(b, AlignLeft)
	xb := bd.bytesPerRow()
	for start := 0; start < len(bd.rows); start += bandRows {
		end := min(start+bandRows, len(bd.rows))
		bits := make([]byte, 0, xb*(end-start))
		for _, row := range bd.rows[start:end] {
			bits = append(bits, row...)
		}
		b.Write([]byte{gs, 'v', '0', 0,
			byte(xb % 256), byte(xb / 256),
			byte((end - start) % 256), byte((end - start) / 256)})
		b.Write(bits)
	}
	bd.rows = bd.rows[:0]
}

// printable keeps the drawn receipt to what the printer's own font would show,
// so the two modes print the same words.
func printable(r rune) rune {
	if r < 0x80 {
		return r
	}
	if _, ok := cp437[r]; ok {
		return r
	}
	return '?'
}

// drawBlock draws the gauge and heatmap glyphs as exact dot patterns, which a
// thermal head prints far more evenly than an outline font's version of them.
func drawBlock(dst *image.Alpha, x int, r rune) bool {
	var on func(cx, cy int) bool
	switch r {
	case '█':
		on = func(cx, cy int) bool { return true }
	case '▓':
		on = func(cx, cy int) bool { return cx%2 == 0 || cy%2 == 0 }
	case '▒':
		on = func(cx, cy int) bool { return (cx+cy)%2 == 0 }
	case '░':
		on = func(cx, cy int) bool { return cx%2 == 0 && cy%2 == 0 }
	case '▌':
		on = func(cx, cy int) bool { return cx < dotsPerCol/2 }
	case '▐':
		on = func(cx, cy int) bool { return cx >= dotsPerCol/2 }
	case '■':
		on = func(cx, cy int) bool { return cx >= 2 && cx < 10 && cy >= 8 && cy < 16 }
	default:
		return false
	}
	for cy := 0; cy < glyphDots; cy++ {
		for cx := 0; cx < dotsPerCol; cx++ {
			if on(cx, cy) {
				dst.Pix[cy*dst.Stride+x+cx] = 0xFF
			}
		}
	}
	return true
}
