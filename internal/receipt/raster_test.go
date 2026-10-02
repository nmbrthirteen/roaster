package receipt

import (
	"bytes"
	"testing"
)

// rasterRows walks a job and returns the GS v 0 commands' heights and row
// widths, failing on anything that is printable text.
func rasterRows(t *testing.T, job []byte) (widths, heights []int) {
	t.Helper()
	for i := 0; i < len(job); {
		switch {
		case bytes.HasPrefix(job[i:], []byte{gs, 'v', '0', 0}):
			xb := int(job[i+4]) | int(job[i+5])<<8
			h := int(job[i+6]) | int(job[i+7])<<8
			widths, heights = append(widths, xb), append(heights, h)
			i += 8 + xb*h
		case bytes.HasPrefix(job[i:], []byte{esc, '@'}):
			i += 2
		case bytes.HasPrefix(job[i:], []byte{gs, 'V'}):
			i += 4
		case job[i] == esc || job[i] == gs:
			i += 3 // the rest of what this encoder sends is three bytes
		case job[i] == lf:
			i++
		default:
			t.Fatalf("byte %#x at %d would print in the printer's font", job[i], i)
		}
	}
	return widths, heights
}

func TestTextAsImageSendsNoCharacters(t *testing.T) {
	SetTextAsImage(true)
	defer SetTextAsImage(true)

	d := &Doc{}
	d.Add(
		Text{Value: "Printer test", Style: Style{Bold: true, Double: true}, Bleed: true},
		Feed{Lines: 1},
		KV{Label: "Target", Value: "win:POS-80"},
		Section{Label: "Reversed"},
		Bar{Label: "Gauge", Value: "60%", Percent: 60},
		Para{Value: "“Quoted” and ünknown."},
	)
	widths, heights := rasterRows(t, d.ESCPOS(nil))
	if len(widths) != 1 {
		t.Fatalf("want the text in one band, got %d", len(widths))
	}
	if want := Width() * dotsPerCol / 8; widths[0] != want {
		t.Errorf("band is %d bytes wide, want %d", widths[0], want)
	}
	// The double row, then the feed, the KV, the section, the gauge's two rows
	// and its feed, and the paragraph.
	if want := TallSpacing + 7*LineSpacing; heights[0] != want {
		t.Errorf("band is %d dots tall, want %d", heights[0], want)
	}
}

func TestTextAsFontStillSendsCharacters(t *testing.T) {
	SetTextAsImage(false)
	defer SetTextAsImage(true)

	d := &Doc{}
	d.Add(Text{Value: "hello"})
	if !bytes.Contains(d.ESCPOS(nil), []byte("hello")) {
		t.Fatal("font mode should send the characters")
	}
}
