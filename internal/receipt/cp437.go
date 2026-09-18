package receipt

// Thermal printers speak single-byte code pages, not UTF-8. PC437 is the
// default on every ESC/POS printer worth buying and carries the block glyphs
// the gauges are drawn from, so the encoder selects it and maps into it.
var cp437 = map[rune]byte{
	'█': 0xDB, '▓': 0xB2, '▒': 0xB1, '░': 0xB0,
	'▌': 0xDD, '▐': 0xDE, '■': 0xFE,
	'·': 0xFA, '°': 0xF8, '«': 0xAE, '»': 0xAF,
	'±': 0xF1, '“': '"', '”': '"', '‘': '\'', '’': '\'',
}

// encodeText converts a Go string into PC437 bytes. Anything with no mapping
// becomes a question mark, which is visible on paper rather than silently wrong.
func encodeText(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r < 0x80:
			out = append(out, byte(r))
		default:
			if b, ok := cp437[r]; ok {
				out = append(out, b)
			} else {
				out = append(out, '?')
			}
		}
	}
	return out
}

// displayWidth counts printed columns. Every mapped glyph occupies one cell, so
// counting runes rather than bytes is what keeps the layout honest.
func displayWidth(s string) int { return len([]rune(s)) }
