package receipt

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// wrap breaks text to the given column count on word boundaries, splitting any
// word that cannot fit on a line of its own.
func wrap(s string, cols int) []string {
	if cols < 1 {
		cols = 1
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := ""
		for _, w := range words {
			for displayWidth(w) > cols {
				if line != "" {
					out = append(out, line)
					line = ""
				}
				r := []rune(w)
				out = append(out, string(r[:cols]))
				w = string(r[cols:])
			}
			switch {
			case line == "":
				line = w
			case displayWidth(line)+1+displayWidth(w) <= cols:
				line += " " + w
			default:
				out = append(out, line)
				line = w
			}
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func truncate(s string, n int) string {
	r := []rune(s)
	if n <= 0 {
		return ""
	}
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "."
}

func pad(s string, cols int, a Align) string {
	if displayWidth(s) >= cols {
		return s
	}
	gap := cols - displayWidth(s)
	switch a {
	case AlignCenter:
		left := gap / 2
		return strings.Repeat(" ", left) + s + strings.Repeat(" ", gap-left)
	case AlignRight:
		return strings.Repeat(" ", gap) + s
	default:
		return s + strings.Repeat(" ", gap)
	}
}

// displayWidth counts printed columns.
func displayWidth(s string) int { return len([]rune(s)) }

// printable makes text safe to set one rune to a cell. Composed forms first, so
// an accent typed as a separate mark joins its letter instead of taking a
// column of its own. Then whatever prints nothing is dropped: leftover marks,
// joiners, emoji variation selectors and control codes, which would otherwise
// leave a gap or a stray "?". Emoji go too, since no font here draws them and
// a rocket printed as "?" reads as a fault. A letter no font has still prints
// "?", so missing text shows. Tabs become spaces; newlines stay, since wrap
// breaks paragraphs on them.
func printable(s string) string {
	s = norm.NFC.String(s)
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n':
			return r
		case r == '\t':
			return ' '
		case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Cc):
			return -1
		case r >= 0x1F3FB && r <= 0x1F3FF: // emoji skin tones, which only modify the emoji before
			return -1
		case unicode.Is(unicode.So, r) && !hasGlyph(r):
			return -1
		}
		return r
	}, s)
}

// tidy runs printable over every piece of text a block carries.
func tidy(b Block) Block {
	switch v := b.(type) {
	case Text:
		v.Value = printable(v.Value)
		return v
	case KV:
		v.Label, v.Value = printable(v.Label), printable(v.Value)
		return v
	case Para:
		v.Value, v.Mark = printable(v.Value), printable(v.Mark)
		return v
	case Section:
		v.Label = printable(v.Label)
		return v
	case Bar:
		v.Label, v.Value, v.Tag = printable(v.Label), printable(v.Value), printable(v.Tag)
		return v
	case Hero:
		v.Caption, v.Value = printable(v.Caption), printable(v.Value)
		return v
	case Commit:
		v.Ref, v.Where, v.When, v.Message = printable(v.Ref), printable(v.Where), printable(v.When), printable(v.Message)
		return v
	}
	return b
}
