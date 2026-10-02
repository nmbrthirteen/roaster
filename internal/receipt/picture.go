package receipt

import (
	"image"
	"image/color"

	"github.com/skip2/go-qrcode"
)

// paperMargin is the 4mm each side of an 80mm roll the head cannot reach.
const paperMargin = 32

// Picture draws the receipt dot for dot as the printer lays it down, from the
// same rasters the ESC/POS job carries. The QR is drawn here, where the printer
// builds its own.
func (d *Doc) Picture(assets Assets) *image.Gray {
	p := &pictureSink{width: Width() * dotsPerCol}
	d.walk(assets, p, true)

	img := image.NewGray(image.Rect(0, 0, p.width+2*paperMargin, len(p.rows)+2*paperMargin))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}
	for y, row := range p.rows {
		for x, on := range row {
			if on {
				img.SetGray(x+paperMargin, y+paperMargin, color.Gray{})
			}
		}
	}
	return img
}

type pictureSink struct {
	width int
	rows  [][]bool
}

func (p *pictureSink) band(r Raster) { p.place(r.Width, r.Height, AlignLeft, r.at) }

// text never arrives: the picture always draws the letters, in the fonts the
// default mode sends.
func (p *pictureSink) text(Line) {}

func (p *pictureSink) image(r Raster, a Align) {
	p.place(r.Width, r.Height, a, r.at)
	p.feed(rowDots) // the line feed the encoder sends after a logo
}

func (p *pictureSink) qr(data string, size int, a Align) {
	code, err := qrcode.New(data, qrcode.Medium)
	if err != nil {
		return
	}
	code.DisableBorder = true
	bits := code.Bitmap()
	n := len(bits) * size
	p.place(n, n, a, func(x, y int) bool { return bits[y/size][x/size] })
	p.feed(rowDots)
}

func (p *pictureSink) cut() {
	feed := cutFeedMM * dotsPerMM
	p.feed(int(feed))
	row := make([]bool, p.width)
	for x := range row {
		row[x] = x%12 < 6
	}
	p.rows = append(p.rows, row)
}

func (p *pictureSink) feed(dots int) {
	for range dots {
		p.rows = append(p.rows, make([]bool, p.width))
	}
}

func (p *pictureSink) place(w, h int, a Align, at func(x, y int) bool) {
	left := 0
	switch a {
	case AlignCenter:
		left = (p.width - w) / 2
	case AlignRight:
		left = p.width - w
	}
	for y := range h {
		row := make([]bool, p.width)
		for x := range w {
			if left+x >= 0 && left+x < p.width && at(x, y) {
				row[left+x] = true
			}
		}
		p.rows = append(p.rows, row)
	}
}
