package metric

import (
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/upgaming/roaster/internal/github"
	"github.com/upgaming/roaster/internal/roast"
)

const (
	// FeedMax outlasts the slowest verdict at the pace the page plays them.
	FeedMax = 40

	// HeatWeeks is one column a week across the 38 columns of paper.
	HeatWeeks = 38

	messageMax = 72
)

// Feed is newest first across repositories. GitHub returns them grouped by
// repository, and a log that jumps back a year at each boundary reads wrong.
func Feed(commits []github.Commit, n int) []roast.Item {
	sorted := append([]github.Commit(nil), commits...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].At.After(sorted[j].At) })
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	out := make([]roast.Item, 0, len(sorted))
	for _, c := range sorted {
		out = append(out, quote(c))
	}
	return out
}

// Messages that say nothing, whoever wrote them.
var lazy = map[string]bool{
	"fix": true, "fixes": true, "fixed": true, "fix bug": true, "bug fix": true,
	"wip": true, "asdf": true, "asd": true, "qwe": true, "qwerty": true,
	"test": true, "testing": true, "update": true, "updates": true, "changes": true,
	"stuff": true, "temp": true, "tmp": true, "commit": true, "minor": true,
	"done": true, "final": true, "idk": true, "oops": true, "ok": true,
	"lol": true, "save": true, "push": true, "x": true, "a": true, "": true,
}

// Worst passes over merges and the first commit while anything else is left,
// because GitHub wrote those words and the person only clicked.
func Worst(commits []github.Commit) *roast.Item {
	var best *github.Commit
	bestScore := 0
	for i := range commits {
		c := &commits[i]
		s := badness(*c)
		if best == nil || s > bestScore || s == bestScore && better(*c, *best) {
			best, bestScore = c, s
		}
	}
	if best == nil {
		return nil
	}
	w := quote(*best)
	return &w
}

func badness(c github.Commit) int {
	msg := strings.ToLower(strings.TrimSpace(c.Message))
	if strings.HasPrefix(msg, "merge ") || msg == "initial commit" {
		return -10
	}
	s := 0
	if oneWord(c) {
		s += 4
	}
	if lazy[strings.Trim(msg, ".!?-_ ")] {
		s += 3
	}
	if atNight(c) {
		s += 2
	}
	if atWeekend(c) {
		s++
	}
	return s
}

// better breaks a tie: the shorter message is the worse one, then the newer.
func better(a, b github.Commit) bool {
	la, lb := len([]rune(a.Message)), len([]rune(b.Message))
	if la != lb {
		return la < lb
	}
	return a.At.After(b.At)
}

func quote(c github.Commit) roast.Item {
	return roast.Item{Ref: c.Hash, Where: c.Repo, Text: headline(c.Message), At: c.At}
}

// headline strips what would upset a terminal or a print head.
func headline(msg string) string {
	msg, _, _ = strings.Cut(msg, "\n")
	msg = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(msg))
	if r := []rune(msg); len(r) > messageMax {
		msg = string(r[:messageMax-1]) + "."
	}
	return msg
}

// Heat lays the calendar out a column a week, Sunday first, as GitHub does.
// Days after the last one returned are -1, so this week prints only the past.
func Heat(days []github.Day, weeks int) [][7]int {
	if len(days) == 0 || weeks < 1 {
		return nil
	}
	last := dayNumber(days[len(days)-1].Date)
	lastSunday := last - weekday(days[len(days)-1].Date)
	start := lastSunday - 7*(weeks-1)

	out := make([][7]int, weeks)
	for w := range out {
		for d := range out[w] {
			out[w][d] = -1
		}
	}
	for _, d := range days {
		n := dayNumber(d.Date)
		if n < start || n > last {
			continue
		}
		out[(n-start)/7][(n-start)%7] = d.Count
	}
	// A day inside the window that GitHub skipped happened all the same.
	for i := 0; i <= last-start; i++ {
		if out[i/7][i%7] < 0 {
			out[i/7][i%7] = 0
		}
	}
	return out
}

// Counting plain days from the epoch keeps a timezone from moving a day into
// the next column.
func dayNumber(t time.Time) int {
	y, m, d := t.Date()
	return int(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400)
}

func weekday(t time.Time) int { return int(t.Weekday()) }
