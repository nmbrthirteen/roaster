package receipt

import (
	"fmt"
	"html"
	"net/url"
	"strings"
)

// HTML renders the same flattened lines the printer receives into markup for
// the on-screen designer. Every attribute maps to a CSS class, so the preview
// and the paper cannot drift apart: change the document, both follow.
func (d *Doc) HTML(assets Assets) string {
	var b strings.Builder
	for _, ln := range d.Lines() {
		switch {
		case ln.QR != "":
			cols := float64((qrModules+qrQuiet)*ln.QRSize) / dotsPerCol
			fmt.Fprintf(&b, `<div class="ln qr"><img style="width:%.2fch" src="/qr?m=%d&d=%s" alt="QR code"></div>`,
				cols, ln.QRSize, url.QueryEscape(ln.QR))
		case ln.Image != "":
			cols := 0.0
			if r, ok := assets[ln.Image]; ok {
				cols = float64(r.Width) / dotsPerCol
			}
			fmt.Fprintf(&b, `<div class="ln img"><img style="width:%.2fch" src="/assets/%s.png" alt=""></div>`,
				cols, html.EscapeString(ln.Image))
		case ln.Text != "":
			fmt.Fprintf(&b, `<div class="ln%s">%s</div>`,
				classes(ln.Style), html.EscapeString(ln.Text))
		}
		for i := 0; i < ln.Feed; i++ {
			b.WriteString(`<div class="ln">&nbsp;</div>`)
		}
		if ln.Cut {
			b.WriteString(`<div class="cut"></div>`)
		}
	}
	return b.String()
}

func classes(s Style) string {
	var c []string
	if s.Bold {
		c = append(c, "b")
	}
	if s.Under {
		c = append(c, "u")
	}
	if s.Invert {
		c = append(c, "inv")
	}
	if s.Double {
		c = append(c, "dw")
	}
	if s.Tall {
		c = append(c, "th")
	}
	if len(c) == 0 {
		return ""
	}
	return " " + strings.Join(c, " ")
}

// Plain renders the document as monospace text. Useful for diffing a layout
// change in a terminal and for the fallback log when no printer is attached.
func (d *Doc) Plain() string {
	var b strings.Builder
	for _, ln := range d.Lines() {
		switch {
		case ln.QR != "":
			fmt.Fprintf(&b, "%s\n", pad("[QR "+ln.QR+"]", Width, AlignCenter))
		case ln.Image != "":
			fmt.Fprintf(&b, "%s\n", pad("["+ln.Image+"]", Width, AlignCenter))
		case ln.Text != "":
			b.WriteString(ln.Text + "\n")
		}
		b.WriteString(strings.Repeat("\n", ln.Feed))
		if ln.Cut {
			b.WriteString(strings.Repeat("- ", Width/2) + "\n")
		}
	}
	return b.String()
}
