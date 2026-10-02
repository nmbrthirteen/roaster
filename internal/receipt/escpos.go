package receipt

import (
	"bytes"
	"sync/atomic"
)

const (
	esc = 0x1B
	gs  = 0x1D
	lf  = 0x0A
)

// Row heights in dots. Font A is 24 dots tall, and the ESC/POS default of 1/6
// inch leaves ten dots of leading, which costs about 60mm of paper per receipt.
// Drawn text keeps the same rows, so the paper estimate holds.
const (
	LineSpacing = 28
	TallSpacing = 56
)

// Text is drawn here and sent as a raster unless the stand is set to the
// printer's own font. Some printers print every letter of their font as a
// black block while the logo comes out sharp; a printer whose font is good can
// take characters instead, a much smaller job that only carries code page 437.
var textAsImage atomic.Bool

func init() { textAsImage.Store(true) }

// TextAsImage reports whether text is drawn here rather than by the printer.
func TextAsImage() bool { return textAsImage.Load() }

func SetTextAsImage(v bool) { textAsImage.Store(v) }

// ESCPOS encodes the document into the byte stream an 80mm thermal printer
// speaks.
func (d *Doc) ESCPOS(assets Assets) []byte {
	var b bytes.Buffer
	b.Write([]byte{esc, '@'})              // initialise, clears any state left by a prior job
	b.Write([]byte{esc, 't', 0})           // PC437, for when the printer sets the letters
	b.Write([]byte{esc, '3', LineSpacing}) // feeds after a logo or QR keep the old row height
	d.walk(assets, escposSink{&b}, TextAsImage())
	return b.Bytes()
}

// A sink receives the document as the printer sees it: raster bands, QR codes
// and cuts. The printer and the preview picture both take it from walk.
type sink interface {
	band(r Raster)
	text(ln Line) // a line the printer sets in its own font
	image(r Raster, a Align)
	qr(data string, size int, a Align)
	cut()
}

// walk draws consecutive text lines and feeds into one band, so the printer
// gets a handful of rasters rather than one per line. With drawn false, text
// lines go to the printer's font instead and only the graphics are drawn.
func (d *Doc) walk(assets Assets, s sink, drawn bool) {
	var text Raster
	flush := func() {
		if text.Height > 0 {
			s.band(text)
		}
		text = Raster{}
	}
	add := func(r Raster) {
		if text.Height > 0 && text.Width != r.Width {
			flush() // the paper width changed mid-document
		}
		if text.Height == 0 {
			text = Raster{Width: r.Width}
		}
		text.below(r)
	}

	for _, ln := range d.Lines() {
		switch {
		case ln.QR != "":
			flush()
			s.qr(ln.QR, ln.QRSize, ln.Style.Align)
		case ln.Image != "":
			if r, ok := assets[ln.Image]; ok {
				flush()
				s.image(r, ln.Style.Align)
			}
		case ln.Bitmap != nil:
			add(*ln.Bitmap)
		case ln.Text != "" && drawn:
			add(drawText(ln))
		case ln.Text != "":
			flush()
			s.text(ln)
		}
		if ln.Feed > 0 {
			add(blankRaster(Width()*dotsPerCol, ln.Feed*rowDots))
		}
		if ln.Cut {
			flush()
			s.cut()
		}
	}
	flush()
}

type escposSink struct{ b *bytes.Buffer }

// bandRows keeps each GS v 0 block small enough for printers with a short
// receive buffer. The slices print edge to edge with no gap.
const bandRows = 256

// band sends a raster in slices, each cut back to its rightmost ink. A run of
// blank rows goes as a one byte wide image, so a feed costs a byte a row and the
// paper still moves exactly as far. Over a 115200 baud serial link every byte
// saved is time a visitor is not standing there.
func (s escposSink) band(r Raster) {
	writeAlign(s.b, AlignLeft)
	full := (r.Width + 7) / 8
	used := make([]int, r.Height)
	for y := range used {
		row := r.Bits[y*full : (y+1)*full]
		for x := len(row) - 1; x >= 0; x-- {
			if row[x] != 0 {
				used[y] = x + 1
				break
			}
		}
	}

	for y := 0; y < r.Height; {
		end := y
		if used[y] == 0 {
			for end < r.Height && used[end] == 0 && end-y < bandRows {
				end++
			}
		} else {
			// Ink runs on through the leading between lines and stops at a gap of
			// a whole blank line, which goes cheaper as its own slice.
			last := y
			for end < r.Height && end-y < bandRows {
				if used[end] > 0 {
					last = end
				} else if end-last >= rowDots {
					break
				}
				end++
			}
			end = last + 1
		}

		xb := 1
		for _, u := range used[y:end] {
			xb = max(xb, u)
		}
		writeRasterHeader(s.b, xb, end-y)
		for row := y; row < end; row++ {
			s.b.Write(r.Bits[row*full : row*full+xb])
		}
		y = end
	}
}

func (s escposSink) text(ln Line) {
	tall := ln.Style.Double || ln.Style.Tall
	if tall {
		s.b.Write([]byte{esc, '3', TallSpacing})
	}
	writeStyle(s.b, ln.Style)
	s.b.Write(encodeText(ln.Text))
	s.b.WriteByte(lf)
	writeStyle(s.b, Style{}) // leave the printer in a known state
	if tall {
		s.b.Write([]byte{esc, '3', LineSpacing})
	}
}

func (s escposSink) image(r Raster, a Align) {
	writeAlign(s.b, a)
	writeRaster(s.b, r)
}

func (s escposSink) qr(data string, size int, a Align) {
	writeAlign(s.b, a)
	writeQR(s.b, data, size)
}

func (s escposSink) cut() {
	s.b.Write([]byte{gs, 'V', 66, 0}) // function B: feed to cutter, partial cut
}

func writeAlign(b *bytes.Buffer, a Align) {
	b.Write([]byte{esc, 'a', byte(a)})
}

func writeStyle(b *bytes.Buffer, s Style) {
	writeAlign(b, s.Align)
	b.Write([]byte{esc, 'E', boolByte(s.Bold)})
	b.Write([]byte{esc, '-', boolByte(s.Under)})
	b.Write([]byte{gs, 'B', boolByte(s.Invert)})

	var size byte
	switch {
	case s.Double:
		size = 0x11 // double width and height
	case s.Tall:
		size = 0x01 // double height only
	}
	b.Write([]byte{gs, '!', size})
}

func boolByte(v bool) byte {
	if v {
		return 1
	}
	return 0
}

// writeQR uses the printer's built-in QR engine.
func writeQR(b *bytes.Buffer, data string, size int) {
	if size < 1 {
		size = 1
	}
	if size > 16 {
		size = 16
	}
	b.Write([]byte{gs, '(', 'k', 4, 0, 49, 65, 50, 0})      // model 2
	b.Write([]byte{gs, '(', 'k', 3, 0, 49, 67, byte(size)}) // module size
	b.Write([]byte{gs, '(', 'k', 3, 0, 49, 69, 49})         // error correction M

	n := len(data) + 3
	b.Write([]byte{gs, '(', 'k', byte(n % 256), byte(n / 256), 49, 80, 48})
	b.WriteString(data)

	b.Write([]byte{gs, '(', 'k', 3, 0, 49, 81, 48}) // print
	b.WriteByte(lf)
}

func writeRaster(b *bytes.Buffer, r Raster) {
	writeRasterHeader(b, (r.Width+7)/8, r.Height)
	b.Write(r.Bits)
	b.WriteByte(lf)
}

func writeRasterHeader(b *bytes.Buffer, xb, rows int) {
	b.Write([]byte{gs, 'v', '0', 0, byte(xb % 256), byte(xb / 256), byte(rows % 256), byte(rows / 256)})
}
