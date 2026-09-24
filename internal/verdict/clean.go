package verdict

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// maxVerdict is what fits under the gauges without the receipt turning into a
// scroll. The prompt asks for less; this is the line nothing crosses.
const maxVerdict = 280

// blocked is a last check on what comes back, not the first. The prompt and the
// model do the real work; this catches the case where a commit message written
// to hijack the prompt got further than it should have. Any match throws the
// line away and the numbers write it instead, so a false alarm costs a tamer
// joke and nothing else.
var blocked = regexp.MustCompile(`(?i)\b(fuck\w*|shit\w*|cunts?|bitch\w*|dicks?|cocks?|piss\w*|bastards?|whores?|sluts?)\b|https?://|www\.`)

// Clean turns what the model wrote into what a receipt can carry, and says
// whether it can carry it at all.
func Clean(s string) (string, bool) {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.Join(strings.Fields(s), " ")
	s = strings.TrimLeft(s, "# ")
	s = unwrap(strings.TrimSpace(s))

	if s == "" || blocked.MatchString(s) {
		return "", false
	}
	return fit(s, maxVerdict), true
}

var quotePairs = map[rune]rune{'"': '"', '\'': '\'', '“': '”', '‘': '’'}

func unwrap(s string) string {
	r := []rune(s)
	if len(r) < 2 {
		return s
	}
	if closing, ok := quotePairs[r[0]]; ok && r[len(r)-1] == closing && strings.Count(s, string(r[0])) <= 2 {
		return strings.TrimSpace(string(r[1 : len(r)-1]))
	}
	return s
}

// fit cuts an overlong verdict at the last sentence that fits, or failing that
// at the last word, so paper never ends mid-thought.
func fit(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	cut := string([]rune(s)[:max])

	if end := strings.LastIndexAny(cut, ".!?"); end > max/3 {
		return cut[:end+1]
	}
	if space := strings.LastIndexByte(cut, ' '); space > 0 {
		return strings.TrimRight(cut[:space], ",;:") + "…"
	}
	return cut
}
