package receipt

import (
	"image"
	"sync"
	"testing"
	"unicode"
)

// What a receipt can realistically carry: names, repos, commit messages and
// the model's verdict, from a stand in Tbilisi with visitors from anywhere.
var scripts = map[string][2]rune{
	"Basic Latin":                {0x20, 0x7E},
	"Latin-1":                    {0xA0, 0xFF},
	"Latin Extended-A":           {0x100, 0x17F},
	"Latin Extended-B":           {0x180, 0x24F},
	"Latin Extended Additional":  {0x1E00, 0x1EFF}, // Vietnamese
	"Greek":                      {0x391, 0x3C9},
	"Cyrillic":                   {0x400, 0x45F},
	"Georgian":                   {0x10D0, 0x10FA},
	"Georgian Mtavruli":          {0x1C90, 0x1CBA},
	"General Punctuation":        {0x2010, 0x2027},
	"Currency":                   {0x20A0, 0x20C0},
	"Arrows":                     {0x2190, 0x2199},
	"Box drawing and block bars": {0x2500, 0x259F},
}

func TestEveryCharacterAReceiptCarriesHasAGlyph(t *testing.T) {
	drawMu.Lock()
	defer drawMu.Unlock()
	for _, bold := range []bool{false, true} {
		f := facesFor(bold, emDots)
		for name, rg := range scripts {
			var missing []rune
			for r := rg[0]; r <= rg[1]; r++ {
				if !unicode.IsPrint(r) || unicode.IsSpace(r) {
					continue
				}
				if _, got := f.glyph(r); got == '?' && r != '?' {
					missing = append(missing, r)
				}
			}
			if len(missing) > 0 {
				t.Errorf("bold %v, %s: no glyph for %q", bold, name, string(missing))
			}
		}
	}
}

// Every glyph, at every size the receipt uses, stays inside its own cell and
// row, so no letter ever runs into its neighbour or gets its top cut off.
func TestEveryGlyphStaysInsideItsCell(t *testing.T) {
	drawMu.Lock()
	defer drawMu.Unlock()
	for _, bold := range []bool{false, true} {
		for _, size := range []struct{ em, cell, row int }{
			{labelEm, labelCell, labelRow},
			{emDots, dotsPerCol, rowDots},
			{emDots * 2, dotsPerCol * 2, rowDots * 2},
		} {
			f := facesFor(bold, size.em)
			base := f.baseline(size.row)
			for name, rg := range scripts {
				for r := rg[0]; r <= rg[1]; r++ {
					if !unicode.IsPrint(r) || unicode.IsSpace(r) {
						continue
					}
					face, g := f.glyph(r)
					// Three cells; the glyph goes in the middle one.
					img := image.NewAlpha(image.Rect(0, 0, 3*size.cell, size.row+8))
					drawInCell(img, face, g, size.cell, 4, base+4, size.cell, size.row)
					for y := range img.Rect.Dy() {
						for x := range img.Rect.Dx() {
							inside := x >= size.cell && x < 2*size.cell && y >= 4 && y < size.row+4
							if img.Pix[y*img.Stride+x] >= 0x80 && !inside {
								t.Errorf("bold %v, em %d, %s: %q inks outside its cell at %d,%d", bold, size.em, name, r, x-size.cell, y-4)
								goto next
							}
						}
					}
				next:
				}
			}
		}
	}
}

func TestDrawingFromManyPrintsAtOnceIsSafe(t *testing.T) {
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d := &Doc{}
			d.Add(Text{Value: "Ships at 3am — then explains why… ბაგი", Style: Style{Bold: i%2 == 0, Double: i%3 == 0}})
			d.ESCPOS(nil)
		}()
	}
	wg.Wait()
}

func TestPrintableJoinsAccentsAndDropsWhatPrintsNothing(t *testing.T) {
	for in, want := range map[string]string{
		"Jose\u0301":             "Jos\u00e9", // a separate accent joins its letter
		"a\u200db":               "ab",        // zero-width joiner
		"ship \U0001F680 it":     "ship  it",  // an emoji no font draws
		"ok\U0001F44D\U0001F3FD": "ok",        // and its skin tone
		"\u2713\ufe0f":           "\u2713",    // variation selector; the check mark itself has a glyph
		"\u4e2d":                 "\u4e2d",    // a letter no font has stays, and prints as "?"
		"tab\there":              "tab here",
		"line\nbreak":            "line\nbreak",
		"bell\a":                 "bell",
	} {
		if got := printable(in); got != want {
			t.Errorf("printable(%q) = %q, want %q", in, got, want)
		}
	}
}
