package receipt

// Thermal heads print at 203 dpi, so one millimetre is just under 8 dots.
const dotsPerMM = 203.0 / 25.4

// dotsPerCol is the width of one Font A character cell.
const dotsPerCol = 12

// Dot heights for one printed row at the spacing the encoder sets.
const (
	rowDots   = LineSpacing
	tallRow   = TallSpacing
	cutFeedMM = 15 // the blade sits above the head, so a cut always costs paper
	qrQuiet   = 8  // quiet zone, in modules
	qrModules = 29 // version 3 at error correction M, which covers a short URL
)

func (d *Doc) EstimateHeightMM(assets Assets) float64 {
	dots := 0.0
	for _, ln := range d.Lines() {
		switch {
		case ln.QR != "":
			dots += float64((qrModules + qrQuiet) * ln.QRSize)
		case ln.Image != "":
			if r, ok := assets[ln.Image]; ok {
				dots += float64(r.Height)
			}
		case ln.Text != "":
			if ln.Style.Double || ln.Style.Tall {
				dots += tallRow
			} else {
				dots += rowDots
			}
		}
		dots += float64(ln.Feed * rowDots)
		if ln.Cut {
			dots += cutFeedMM * dotsPerMM
		}
	}
	return dots / dotsPerMM
}
