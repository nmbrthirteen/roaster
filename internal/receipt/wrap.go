package receipt

import "strings"

// wrap breaks text to the given column count on word boundaries, splitting any
// word that cannot fit on a line of its own. Explicit newlines are preserved.
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
