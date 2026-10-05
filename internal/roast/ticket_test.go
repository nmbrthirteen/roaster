package roast

import (
	"strings"
	"testing"
	"time"

	"github.com/upgaming/roaster/internal/event"
	"github.com/upgaming/roaster/internal/receipt"
)

func printed(r Roast) string {
	var s strings.Builder
	for _, ln := range r.Doc(event.Event{}, "001").Lines() {
		s.WriteString(ln.Text + ln.Alt + "\n")
	}
	return s.String()
}

func TestAnAccountWithNothingPublicStillGetsAFullReceipt(t *testing.T) {
	r := Sample("ghost")
	r.Exhibit = nil
	r.Heat = make([][7]int, 38)

	out := printed(r)

	for _, want := range []string{"technically flawless", "Days with none", "266 of 266", "A spotless calendar."} {
		if !strings.Contains(out, want) {
			t.Errorf("the receipt should say %q", want)
		}
	}
}

func TestTheWorstCommitIsQuotedWithWhereAndWhen(t *testing.T) {
	r := Sample("octocat")
	r.Exhibit = &Item{Ref: "a1b2c3d", Where: "api", Text: "asdf",
		At: time.Date(2026, time.September, 15, 3, 12, 0, 0, time.FixedZone("GST", 4*60*60))}

	out := printed(r)

	for _, want := range []string{"\nasdf\n", "a1b2c3d in api", "Tue 15 Sep 2026, 03:12"} {
		if !strings.Contains(out, want) {
			t.Errorf("the receipt should say %q", want)
		}
	}
}

func TestTheReceiptQRSaysItWasScannedFromPaper(t *testing.T) {
	r := Sample("octocat")
	ev := event.Event{}
	ev.Receipt.ShareBase = "https://example.test/k"
	doc := r.Doc(ev, "001")

	var data []string
	for _, b := range doc.Blocks {
		if qr, ok := b.(receipt.QR); ok {
			data = append(data, qr.Data)
		}
	}
	want := "https://example.test/k/" + r.Code + "?s=receipt"
	if len(data) != 1 || data[0] != want {
		t.Errorf("QR data = %q, want %q", data, want)
	}
	var text strings.Builder
	for _, ln := range doc.Lines() {
		text.WriteString(ln.Text + ln.Alt + "\n")
	}
	if !strings.Contains(text.String(), "example.test/k/"+r.Code) || strings.Contains(text.String(), "?s=") {
		t.Errorf("the printed address should stay clean")
	}
}
