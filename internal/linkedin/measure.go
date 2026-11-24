package linkedin

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/upgaming/roaster/internal/roast"
)

// Every number here is arithmetic over the profile, for the same reason as on
// GitHub: a receipt people photograph has to survive being checked.

// A job left inside this many months counts as a short one.
const shortStay = 18

// bigTitle is a title that claims rank. Two at once is a pattern.
var bigTitle = regexp.MustCompile(`(?i)\b(co-?founder|founder|ceo|cto|coo|cfo|cmo|cpo|chief|head|director|vp|vice president|president|partner|principal|lead|evangelist|visionary)\b`)

// buzzwords are the words that fit every profile and so say nothing about
// this one. Phrases first, so "thought leader" counts once.
var buzzwords = []string{
	"proven track record", "thought leader", "growth mindset", "problem solver", "team player",
	"game changer", "game-changer", "self-starter", "go-getter", "results-driven", "results-oriented",
	"detail-oriented", "passionate", "driven", "visionary", "innovative", "innovator", "synergy",
	"ninja", "guru", "rockstar", "evangelist", "storyteller", "disruptive", "strategic", "dynamic",
	"motivated", "hustler", "enthusiast", "leverage", "seasoned", "creative", "builder", "10x",
}

var buzzPattern = func() *regexp.Regexp {
	quoted := make([]string, len(buzzwords))
	for i, b := range buzzwords {
		quoted[i] = regexp.QuoteMeta(b)
	}
	return regexp.MustCompile(`(?i)(^|[^\p{L}\p{N}])(` + strings.Join(quoted, "|") + `)([^\p{L}\p{N}]|$)`)
}()

// headlineParts splits a headline on the separators people stack claims with.
var headlineParts = regexp.MustCompile(`\s*[|•·/]\s*`)

// Metrics is the damage report: five gauges and the average stay, in the order
// the receipt prints them.
func Metrics(p Profile, now time.Time) []roast.Metric {
	m := []roast.Metric{
		gauge("Titles with a big word", inflated(p), [3]string{"modest", "ambitious", "self-crowned"}),
		gauge("Jobs under 18 months", hopped(p, now), [3]string{"loyal", "restless", "flight risk"}),
		gauge("Roles with no description", undescribed(p), [3]string{"documented", "vague", "ghosted"}),
		gauge("Skills no job backs up", unbacked(p), [3]string{"proven", "hopeful", "wishlist"}),
		gauge("Headline in buzzwords", buzzShare(p.Headline), [3]string{"plain", "polished", "poster"}),
		{Label: "Average stay", Value: duration(averageStay(p, now))},
	}
	blank := func(i int) { m[i] = roast.Metric{Label: m[i].Label, Value: "none", Tag: "untested"} }
	if len(p.Positions) == 0 {
		blank(0)
		blank(2)
		m[5].Value = "none"
	}
	if finished(p) == 0 {
		blank(1)
	}
	if len(p.Skills) == 0 {
		blank(3)
	}
	if strings.TrimSpace(p.Headline) == "" {
		blank(4)
	}
	return m
}

// Empty is a profile with nothing to roast: no roles, no headline, no About.
func Empty(p Profile) bool {
	return len(p.Positions) == 0 && strings.TrimSpace(p.Headline) == "" && strings.TrimSpace(p.About) == ""
}

// Score is how much of LinkedIn has got into the profile: rank claimed, jobs
// left early, roles and skills nobody can check, and words that fit anyone.
func Score(p Profile, now time.Time) int {
	if Empty(p) {
		return 0
	}
	burn := 25*inflated(p) + 25*hopped(p, now) + 15*undescribed(p) + 15*unbacked(p) + 20*buzzShare(p.Headline)
	return min(100, (burn+50)/100)
}

type Shape string

const (
	Fresh   Shape = "fresh"
	Hopper  Shape = "hopper"
	Lifer   Shape = "lifer"
	Crowned Shape = "crowned"
	Regular Shape = "regular"
)

var shapes = map[Shape]string{
	Fresh:   "Early in the career. Never call it long, stale or senior.",
	Hopper:  "Moves on fast. Never call it loyal or stuck.",
	Lifer:   "Years in one place. Never call it restless.",
	Crowned: "Rank in most titles. Never call it modest.",
	Regular: "A normal career. Never call it a mess or a triumph.",
}

func ShapeOf(p Profile, now time.Time) Shape {
	switch {
	case careerMonths(p, now) < 24:
		return Fresh
	case finished(p) >= 3 && averageStay(p, now) < shortStay:
		return Hopper
	case currentStay(p, now) >= 72:
		return Lifer
	case len(p.Positions) >= 2 && inflated(p) >= 66:
		return Crowned
	default:
		return Regular
	}
}

func inflated(p Profile) int {
	n := 0
	for _, pos := range p.Positions {
		if bigTitle.MatchString(pos.Title) {
			n++
		}
	}
	return percent(n, len(p.Positions))
}

func hopped(p Profile, now time.Time) int {
	n, of := 0, 0
	for _, pos := range p.Positions {
		if pos.Current() || pos.Start.IsZero() {
			continue
		}
		of++
		if stay(pos, now) < shortStay {
			n++
		}
	}
	return percent(n, of)
}

func undescribed(p Profile) int {
	n := 0
	for _, pos := range p.Positions {
		if strings.TrimSpace(pos.Description) == "" {
			n++
		}
	}
	return percent(n, len(p.Positions))
}

func unbacked(p Profile) int {
	return percent(len(Unbacked(p)), len(p.Skills))
}

// Unbacked is the skills no role's title or description mentions.
func Unbacked(p Profile) []string {
	var work strings.Builder
	for _, pos := range p.Positions {
		work.WriteString(strings.ToLower(pos.Title + " " + pos.Description + " "))
	}
	text := work.String()
	var out []string
	for _, s := range p.Skills {
		if k := strings.ToLower(strings.TrimSpace(s)); k != "" && !strings.Contains(text, k) {
			out = append(out, s)
		}
	}
	return out
}

func buzzShare(headline string) int {
	words := len(strings.FieldsFunc(headline, notWord))
	if words == 0 {
		return 0
	}
	used := 0
	for _, b := range Buzzwords(headline) {
		used += len(strings.Fields(b))
	}
	return min(100, percent(used, words))
}

// Buzzwords is every buzzword in the text, lowercased, in the order they
// appear, repeats included.
func Buzzwords(text string) []string {
	var out []string
	rest := text
	for {
		loc := buzzPattern.FindStringSubmatchIndex(rest)
		if loc == nil {
			return out
		}
		out = append(out, strings.ToLower(rest[loc[4]:loc[5]]))
		rest = rest[loc[5]:]
	}
}

func notWord(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '-' }

// stay is a role's length in whole months, a role held now counting to now.
func stay(p Position, now time.Time) int {
	if p.Start.IsZero() {
		return 0
	}
	end := p.End
	if end.IsZero() {
		end = now
	}
	return months(p.Start, end)
}

func months(from, to time.Time) int {
	n := (to.Year()-from.Year())*12 + int(to.Month()-from.Month())
	return max(n, 0)
}

func finished(p Profile) int {
	n := 0
	for _, pos := range p.Positions {
		if !pos.Current() && !pos.Start.IsZero() {
			n++
		}
	}
	return n
}

func averageStay(p Profile, now time.Time) int {
	total, n := 0, 0
	for _, pos := range p.Positions {
		if pos.Start.IsZero() {
			continue
		}
		total += stay(pos, now)
		n++
	}
	if n == 0 {
		return 0
	}
	return (total + n/2) / n
}

func currentStay(p Profile, now time.Time) int {
	longest := 0
	for _, pos := range p.Positions {
		if pos.Current() {
			longest = max(longest, stay(pos, now))
		}
	}
	return longest
}

func careerStart(p Profile) time.Time {
	var start time.Time
	for _, pos := range p.Positions {
		if !pos.Start.IsZero() && (start.IsZero() || pos.Start.Before(start)) {
			start = pos.Start
		}
	}
	return start
}

func careerMonths(p Profile, now time.Time) int {
	start := careerStart(p)
	if start.IsZero() {
		return 0
	}
	return months(start, now)
}

// longestGap is the longest stretch between one role ending and the next
// starting, with no other role covering it.
func longestGap(p Profile, now time.Time) int {
	type span struct{ from, to time.Time }
	var spans []span
	for _, pos := range p.Positions {
		if pos.Start.IsZero() {
			continue
		}
		to := pos.End
		if to.IsZero() {
			to = now
		}
		spans = append(spans, span{pos.Start, to})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].from.Before(spans[j].from) })
	gap := 0
	var reach time.Time
	for i, s := range spans {
		if i > 0 && s.from.After(reach) {
			gap = max(gap, months(reach, s.from))
		}
		if s.to.After(reach) {
			reach = s.to
		}
	}
	return gap
}

// atOnce is the most roles held in the same month.
func atOnce(p Profile, now time.Time) int {
	most := 0
	for _, a := range p.Positions {
		if a.Start.IsZero() {
			continue
		}
		n := 0
		for _, b := range p.Positions {
			if b.Start.IsZero() {
				continue
			}
			end := b.End
			if end.IsZero() {
				end = now
			}
			if !a.Start.Before(b.Start) && a.Start.Before(end) {
				n++
			}
		}
		most = max(most, n)
	}
	return most
}

func founded(p Profile) int {
	n := 0
	for _, pos := range p.Positions {
		if strings.Contains(strings.ToLower(pos.Title), "founder") {
			n++
		}
	}
	return n
}

// duration is months the way people say them: "3 yrs 2 mo".
func duration(m int) string {
	y, mo := m/12, m%12
	switch {
	case y == 0:
		return fmt.Sprintf("%d mo", mo)
	case mo == 0:
		return fmt.Sprintf("%d %s", y, plural(y, "yr"))
	default:
		return fmt.Sprintf("%d %s %d mo", y, plural(y, "yr"), mo)
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

func count(n int, word string) string {
	return fmt.Sprintf("%d %s", n, plural(n, word))
}

func gauge(label string, n int, bands [3]string) roast.Metric {
	band := bands[0]
	switch {
	case n >= 66:
		band = bands[2]
	case n >= 33:
		band = bands[1]
	}
	return roast.Metric{Label: label, Value: fmt.Sprintf("%d%%", n), Tag: band, Percent: &n}
}

func percent(n, of int) int {
	if of == 0 {
		return 0
	}
	return (n*200 + of) / (of * 2)
}
