package receipt

import (
	_ "embed"
	"image"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// The printer's own font only carries code page 437, so an ellipsis, an
// accented name or a Georgian commit message came out as question marks. Text
// is drawn here instead, with fonts built into the binary, and sent as a raster.
// Go Mono sets the look. DejaVu Sans Mono, the same width, covers most of what
// it lacks: the rest of Latin, currency signs, arrows. Noto Sans Georgian
// covers Georgian, and Noto Sans the last gaps, Vietnamese among them.

//go:embed fonts/dejavu-sans-mono.ttf
var dejavuRegular []byte

//go:embed fonts/dejavu-sans-mono-bold.ttf
var dejavuBold []byte

//go:embed fonts/noto-sans-regular.ttf
var notoRegular []byte

//go:embed fonts/noto-sans-bold.ttf
var notoBold []byte

//go:embed fonts/noto-sans-georgian-regular.ttf
var georgianRegular []byte

//go:embed fonts/noto-sans-georgian-bold.ttf
var georgianBold []byte

// FontFiles are the receipt's fonts by file name, so the designer page can set
// its mock-up in the same faces the printer gets.
var FontFiles = map[string][]byte{
	"go-mono.ttf":                    gomono.TTF,
	"go-mono-bold.ttf":               gomonobold.TTF,
	"dejavu-sans-mono.ttf":           dejavuRegular,
	"dejavu-sans-mono-bold.ttf":      dejavuBold,
	"noto-sans-regular.ttf":          notoRegular,
	"noto-sans-bold.ttf":             notoBold,
	"noto-sans-georgian-regular.ttf": georgianRegular,
	"noto-sans-georgian-bold.ttf":    georgianBold,
}

// Go Mono advances 0.6 em, so 20 dots to the em fills a 12 dot cell.
const emDots = 20

type faceKey struct {
	bold bool
	em   int
}

// faceSet is the faces a glyph is looked for in, in order.
type faceSet struct{ mono, wide, georgian, last font.Face }

var (
	// A font.Face holds a scratch buffer, so drawing is one job at a time.
	drawMu sync.Mutex
	faces  = map[faceKey]faceSet{}
)

func facesFor(bold bool, em int) faceSet {
	k := faceKey{bold, em}
	if f, ok := faces[k]; ok {
		return f
	}
	mono, wide, georgian, last := gomono.TTF, dejavuRegular, georgianRegular, notoRegular
	if bold {
		mono, wide, georgian, last = gomonobold.TTF, dejavuBold, georgianBold, notoBold
	}
	f := faceSet{mono: newFace(mono, em), wide: newFace(wide, em), georgian: newFace(georgian, em), last: newFace(last, em)}
	faces[k] = f
	return f
}

// baseline centres the font's full height in a row, as Font A sits in its own.
func (f faceSet) baseline(row int) int {
	m := f.mono.Metrics()
	asc, desc := m.Ascent.Round(), m.Descent.Round()
	return (row-asc-desc)/2 + asc
}

// glyph finds the first face that has r, or a question mark when none does.
func (f faceSet) glyph(r rune) (font.Face, rune) {
	for _, face := range []font.Face{f.mono, f.wide, f.georgian, f.last} {
		if _, ok := face.GlyphAdvance(r); ok {
			return face, r
		}
	}
	return f.mono, '?'
}

// draw sets runes one to a cell from x, whatever their own advance, so the
// columns hold, in a row of the given top and height. The caller holds drawMu.
func (f faceSet) draw(img *image.Alpha, runes []rune, x, top, row, cell int) {
	baseline := top + f.baseline(row)
	for i, r := range runes {
		if r == ' ' {
			continue
		}
		face, r := f.glyph(r)
		drawInCell(img, face, r, x+i*cell, top, baseline, cell, row)
	}
}

// drawInCell keeps a glyph inside its cell and row. One that fits sits where
// its own advance puts it, centred. One that would not, a Georgian letter
// wider than the cell or a capital with a stacked accent taller than the row,
// is drawn aside and fitted: moved in when it is small enough, squeezed when
// it is not. Letters never touch their neighbours or lose their tops.
func drawInCell(img *image.Alpha, face font.Face, r rune, left, top, baseline, cell, row int) {
	b, adv, _ := face.GlyphBounds(r)
	dot := fixed.Point26_6{X: fixed.I(left) + (fixed.I(cell)-adv)/2, Y: fixed.I(baseline)}
	inkL, inkR := (dot.X + b.Min.X).Floor(), (dot.X + b.Max.X).Ceil()
	inkT, inkB := (dot.Y + b.Min.Y).Floor(), (dot.Y + b.Max.Y).Ceil()
	if inkL >= left && inkR <= left+cell && inkT >= top && inkB <= top+row {
		d := font.Drawer{Dst: img, Src: image.Opaque, Face: face, Dot: dot}
		d.DrawString(string(r))
		return
	}

	inkW, inkH := inkR-inkL, inkB-inkT
	scratch := image.NewAlpha(image.Rect(0, 0, inkW, inkH))
	d := font.Drawer{Dst: scratch, Src: image.Opaque, Face: face,
		Dot: fixed.Point26_6{X: dot.X - fixed.I(inkL), Y: dot.Y - fixed.I(inkT)}}
	d.DrawString(string(r))

	// The target box: the ink's own size where it fits, else the room there is,
	// with a dot of air beside each letter shared with the neighbour.
	w, h := min(inkW, cell-1), min(inkH, row)
	x0 := inkL
	if inkL < left || inkR > left+cell {
		x0 = left + (cell-w)/2
	}
	y0 := min(max(inkT, top), top+row-h)

	// Each target dot takes the darkest of the dots it covers, so a thin stroke
	// survives a squeeze rather than averaging away.
	for ty := range h {
		fy, toY := ty*inkH/h, max((ty+1)*inkH/h, ty*inkH/h+1)
		for tx := range w {
			fx, toX := tx*inkW/w, max((tx+1)*inkW/w, tx*inkW/w+1)
			var a uint8
			for sy := fy; sy < toY; sy++ {
				for sx := fx; sx < toX; sx++ {
					a = max(a, scratch.Pix[sy*scratch.Stride+sx])
				}
			}
			x, y := x0+tx, y0+ty
			if x >= 0 && x < img.Rect.Dx() && y >= 0 && y < img.Rect.Dy() && a > img.Pix[y*img.Stride+x] {
				img.Pix[y*img.Stride+x] = a
			}
		}
	}
}

var coverage struct {
	once  sync.Once
	fonts []*sfnt.Font
}

// hasGlyph says whether any of the receipt's fonts can draw r. It parses its
// own copies, so it is safe to call without drawMu.
func hasGlyph(r rune) bool {
	coverage.once.Do(func() {
		for _, ttf := range [][]byte{gomono.TTF, dejavuRegular, georgianRegular, notoRegular} {
			f, err := sfnt.Parse(ttf)
			if err != nil {
				panic("receipt: embedded font: " + err.Error())
			}
			coverage.fonts = append(coverage.fonts, f)
		}
	})
	var b sfnt.Buffer
	for _, f := range coverage.fonts {
		if i, err := f.GlyphIndex(&b, r); err == nil && i != 0 {
			return true
		}
	}
	return false
}

func newFace(ttf []byte, size int) font.Face {
	f, err := opentype.Parse(ttf)
	if err != nil {
		panic("receipt: embedded font: " + err.Error())
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic("receipt: embedded font: " + err.Error())
	}
	return face
}

// drawText renders one line of text on the column grid, the full printable
// width across, at the same row height the printer's own font used.
func drawText(ln Line) Raster {
	drawMu.Lock()
	defer drawMu.Unlock()

	scale := 1
	if ln.Style.Double || ln.Style.Tall {
		scale = 2
	}
	cell, height := dotsPerCol*scale, rowDots*scale
	f := facesFor(ln.Style.Bold, emDots*scale)
	base := f.baseline(height)

	runes := []rune(ln.Text)
	img := image.NewAlpha(image.Rect(0, 0, len(runes)*cell, height))
	for i, r := range runes {
		if fillBlock(img, r, i*cell, cell, scale) {
			runes[i] = ' '
		}
	}
	f.draw(img, runes, 0, 0, height, cell)
	if ln.Style.Under {
		for y := base + scale; y < base+2*scale; y++ {
			for x := 0; x < img.Rect.Dx(); x++ {
				img.Pix[y*img.Stride+x] = 0xFF
			}
		}
	}

	width := Width() * dotsPerCol
	out := blankRaster(width, height)
	// Tall is double height at single width: drawn at double size, then
	// squeezed back to one cell a character.
	squeeze := 1
	if ln.Style.Tall && !ln.Style.Double {
		squeeze = 2
	}
	inked := img.Rect.Dx() / squeeze
	for y := 0; y < height; y++ {
		for x := 0; x < width && x < inked; x++ {
			on := false
			for s := 0; s < squeeze; s++ {
				on = on || img.Pix[y*img.Stride+x*squeeze+s] >= 0x80
			}
			if on != ln.Style.Invert {
				out.set(x, y)
			}
		}
	}
	return out
}

// fillBlock draws the gauge and heatmap glyphs as dot patterns rather than
// trusting a font's rendering of them, so neighbouring cells tile without seams.
func fillBlock(img *image.Alpha, r rune, x0, cell, scale int) bool {
	var on func(x, y int) bool
	switch r {
	case '█':
		on = func(x, y int) bool { return true }
	case '▓':
		on = func(x, y int) bool { return x%2 == 0 || y%2 == 0 }
	case '▒':
		on = func(x, y int) bool { return (x+y)%2 == 0 }
	case '░':
		on = func(x, y int) bool { return x%2 == 0 && y%2 == 0 }
	case '▌':
		on = func(x, y int) bool { return x-x0 < cell/2 }
	case '▐':
		on = func(x, y int) bool { return x-x0 >= cell/2 }
	case '■':
		side := cell * 2 / 3
		mid := img.Rect.Dy() / 2
		on = func(x, y int) bool {
			dx, dy := x-x0-(cell-side)/2, y-(mid-side/2)
			return dx >= 0 && dx < side && dy >= 0 && dy < side
		}
	default:
		return false
	}
	// Two dots of leading top and bottom, as Font A leaves between rows.
	for y := 2 * scale; y < img.Rect.Dy()-2*scale; y++ {
		for x := x0; x < x0+cell; x++ {
			if on(x, y) {
				img.Pix[y*img.Stride+x] = 0xFF
			}
		}
	}
	return true
}

// threshold turns drawn coverage into dots, half covered and up inked.
func threshold(img *image.Alpha) Raster {
	r := blankRaster(img.Rect.Dx(), img.Rect.Dy())
	for y := range r.Height {
		for x := range r.Width {
			if img.Pix[y*img.Stride+x] >= 0x80 {
				r.set(x, y)
			}
		}
	}
	return r
}

func blankRaster(width, height int) Raster {
	return Raster{Width: width, Height: height, Bits: make([]byte, (width+7)/8*height)}
}

func (r *Raster) set(x, y int) {
	r.Bits[y*((r.Width+7)/8)+x/8] |= 0x80 >> (x % 8)
}

func (r Raster) at(x, y int) bool {
	return r.Bits[y*((r.Width+7)/8)+x/8]&(0x80>>(x%8)) != 0
}

// below stacks another raster of the same width under this one.
func (r *Raster) below(o Raster) {
	r.Bits = append(r.Bits, o.Bits...)
	r.Height += o.Height
}
