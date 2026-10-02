package receipt

import (
	"bytes"
	"testing"
	"time"
)

// decode reads back the rasters an ESC/POS job carries, padded to the full
// width, skipping the few other commands the encoder writes. Any other byte
// would print in the printer's own font, and fails the test.
func decode(t *testing.T, job []byte) [][]byte {
	t.Helper()
	full := Width() * dotsPerCol / 8
	var rows [][]byte
	for i := 0; i < len(job); {
		switch {
		case bytes.HasPrefix(job[i:], []byte{esc, '@'}):
			i += 2
		case bytes.HasPrefix(job[i:], []byte{esc, '3'}), bytes.HasPrefix(job[i:], []byte{esc, 'a'}),
			bytes.HasPrefix(job[i:], []byte{esc, 't'}):
			i += 3
		case bytes.HasPrefix(job[i:], []byte{gs, 'V'}):
			i += 4
		case bytes.HasPrefix(job[i:], []byte{gs, 'v', '0', 0}):
			xb := int(job[i+4]) + int(job[i+5])*256
			h := int(job[i+6]) + int(job[i+7])*256
			i += 8
			for range h {
				row := make([]byte, full)
				copy(row, job[i:i+xb])
				rows = append(rows, row)
				i += xb
			}
		default:
			t.Fatalf("unexpected byte %#x at %d", job[i], i)
		}
	}
	return rows
}

func TestTheJobPrintsWhatThePictureShows(t *testing.T) {
	SetTextAsImage(true)
	heat := make([][7]int, 38)
	for w := range heat {
		heat[w][w%7] = w
	}
	d := &Doc{}
	d.Add(
		Text{Value: "Annual code appraisal", Style: Style{Align: AlignCenter, Bold: true, Double: true}, Bleed: true},
		Feed{Lines: 3},
		Section{Label: "Contribution calendar"},
		Heatmap{Weeks: heat, Ending: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)},
		Bar{Label: "Active days", Value: "60%", Percent: 60},
		Para{Value: "“Ships at 3am — then explains why… for a week.” ბაგი შესწორდა"},
		Commit{Ref: "eb443ec", Where: "todo-app", When: "Tue 29 Sep 2026, 16:30", Message: "გამოსწორება: ბაგი შესწორდა"},
		Feed{Lines: 12},
		Text{Value: "x"},
	)

	var want bytes.Buffer
	p := &pictureSink{width: Width() * dotsPerCol}
	d.walk(nil, p, true)
	for _, row := range p.rows {
		packed := blankRaster(p.width, 1)
		for x, on := range row {
			if on {
				packed.set(x, 0)
			}
		}
		want.Write(packed.Bits)
	}

	job := d.ESCPOS(nil)
	var got bytes.Buffer
	for _, row := range decode(t, job) {
		got.Write(row)
	}
	if !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Fatalf("the job decodes to %d bytes of dots, the picture has %d, and they differ", got.Len(), want.Len())
	}
	if full := len(want.Bytes()); len(job) > full/2 {
		t.Errorf("the job is %d bytes for %d bytes of dots; cropping should at least halve it", len(job), full)
	}
}

func TestFontModeStillSendsCharacters(t *testing.T) {
	SetTextAsImage(false)
	defer SetTextAsImage(true)

	d := &Doc{}
	d.Add(
		Text{Value: "hello"},
		Heatmap{Weeks: make([][7]int, 4)},
	)
	job := d.ESCPOS(nil)
	if !bytes.Contains(job, []byte("hello")) {
		t.Error("font mode should send the letters as characters")
	}
	if !bytes.Contains(job, []byte{gs, 'v', '0', 0}) {
		t.Error("the calendar is a graphic and should still go as a raster")
	}
}
