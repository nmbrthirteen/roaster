package receipt

import "bytes"

const (
	esc = 0x1B
	gs  = 0x1D
	lf  = 0x0A
)

// Font A is 24 dots tall. The ESC/POS default of 1/6 inch leaves ten dots of
// leading, which costs about 60mm of paper per receipt.
const (
	LineSpacing = 28
	TallSpacing = 56 // a double-height row needs room for a 48 dot glyph
)

// ESCPOS encodes the document into the byte stream an 80mm thermal printer
// speaks.
func (d *Doc) ESCPOS(assets Assets) []byte {
	var b bytes.Buffer
	b.Write([]byte{esc, '@'})              // initialise, clears any state left by a prior job
	b.Write([]byte{esc, 't', 0})           // select PC437, which carries the gauge glyphs
	b.Write([]byte{esc, '3', LineSpacing}) // tighten the default leading

	for _, ln := range d.Lines() {
		switch {
		case ln.QR != "":
			writeAlign(&b, ln.Style.Align)
			writeQR(&b, ln.QR, ln.QRSize)
		case ln.Image != "":
			if r, ok := assets[ln.Image]; ok {
				writeAlign(&b, ln.Style.Align)
				writeRaster(&b, r)
			}
		case ln.Text != "":
			tall := ln.Style.Double || ln.Style.Tall
			if tall {
				b.Write([]byte{esc, '3', TallSpacing})
			}
			writeStyle(&b, ln.Style)
			b.Write(encodeText(ln.Text))
			b.WriteByte(lf)
			writeStyle(&b, Style{}) // leave the printer in a known state
			if tall {
				b.Write([]byte{esc, '3', LineSpacing})
			}
		}

		if ln.Feed > 0 {
			b.Write([]byte{esc, 'd', byte(ln.Feed)})
		}
		if ln.Cut {
			b.Write([]byte{gs, 'V', 66, 0}) // function B: feed to cutter, partial cut
		}
	}
	return b.Bytes()
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
	xb := (r.Width + 7) / 8
	b.Write([]byte{gs, 'v', '0', 0,
		byte(xb % 256), byte(xb / 256),
		byte(r.Height % 256), byte(r.Height / 256)})
	b.Write(r.Bits)
	b.WriteByte(lf)
}

func boolByte(v bool) byte {
	if v {
		return 1
	}
	return 0
}
