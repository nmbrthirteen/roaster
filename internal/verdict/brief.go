// Package verdict writes the one line on the receipt that is not arithmetic.
//
// Everything else a visitor sees is computed from their account. The verdict
// is the joke, and it is the only thing a model is asked for. That keeps the
// model's job small, which keeps it fast and cheap, and it means a model that is
// slow, down or unwilling costs a single line: Fallback writes one from the
// numbers, and the receipt still prints.
package verdict

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/upgaming/roaster/internal/github"
	"github.com/upgaming/roaster/internal/metric"
	"github.com/upgaming/roaster/internal/roast"
)

// How much untrusted text the model sees. Enough to spot a habit, not enough to
// hand anyone a long channel into the prompt.
const (
	maxCommits    = 40
	maxRepos      = 15
	maxCommitText = 72
	maxRepoText   = 100
	maxStockText  = 200
)

// Brief is everything the model is shown, and it is deliberately less than we
// know. No name, no bio, no employer, no location, no followers: the roast is
// about the work, and a model cannot make a joke about the person out of
// something it was never given.
type Brief struct {
	Years   int
	Score   int
	Metrics []roast.Metric

	Shape         metric.Shape
	Contributions int
	ActiveDays    int
	CalendarDays  int
	QuietestRun   int

	Owned     int
	Forked    int
	Read      int // repositories read, the most recently pushed
	Abandoned int // of those, untouched for a year and not archived
	NoReadme  int // of those, certainly without a README

	StarsEarned int
	StarsGiven  int

	Year    github.Year
	Private int // percent of last year's contributions made in private

	Languages []metric.Share
	Profile   bool // there is a profile README at all
	Badges    int
	Claimed   []string
	Unused    []string // claimed on the README, main language of nothing

	Commits []string
	Repos   []string

	Strengths []string
	Actions   []string
	Findings  []roast.Finding
	Habits    []roast.Finding

	Angle string

	// Avoid is lines already printed: to this account on an earlier visit,
	// and to whoever was in the queue before. They are the stand's own
	// output, so they are trusted, and they sit outside the account block.
	Avoid []string
}

// From assembles the brief. now is passed in so the same account always briefs
// the same way under test.
func From(f github.Facts, metrics []roast.Metric, now time.Time) Brief {
	claimed := metric.Claimed(f.Readme)
	b := Brief{
		Years:   years(f.Created, now),
		Score:   metric.Score(f),
		Metrics: metrics,

		Shape:         metric.ShapeOf(f),
		Contributions: metric.Contributed(f),
		ActiveDays:    metric.ActiveDays(f),
		CalendarDays:  len(f.Year.Days),
		QuietestRun:   metric.Quietest(f),

		Owned:      f.Owned,
		Forked:     f.Forked,
		Read:       len(f.Repos),
		StarsGiven: f.Starred,
		Year:       f.Year,
		Private:    private(f.Year),
		Languages:  metric.Languages(f.Repos),
		Profile:    strings.TrimSpace(f.Readme) != "",
		Badges:     metric.Badges(f.Readme),
		Claimed:    claimed,
		Unused:     metric.Unused(claimed, f.Repos),
	}

	for _, r := range f.Repos {
		b.StarsEarned += r.Stars
		if !r.Archived && !r.Pushed.IsZero() && now.Sub(r.Pushed) > 365*24*time.Hour {
			b.Abandoned++
		}
		if !r.Readme {
			b.NoReadme++
		}
		if len(b.Repos) < maxRepos {
			line := scrub(r.Name, maxRepoText)
			if d := scrub(r.Description, maxRepoText); d != "" {
				line += ": " + d
			}
			b.Repos = append(b.Repos, line)
		}
	}
	for _, c := range f.Commits {
		if len(b.Commits) == maxCommits {
			break
		}
		if msg := scrub(c.Message, maxCommitText); msg != "" {
			b.Commits = append(b.Commits, msg)
		}
	}
	return b
}

// Render is the brief as the model reads it. Everything the account wrote sits
// inside one block, and scrub has already made sure nothing in it can close
// that block early.
func (b Brief) Render() string {
	var s strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&s, format+"\n", args...) }

	line("<account>")
	line("On GitHub for %s. Roast score %d / 100 (%s).", plural(b.Years, "year"), b.Score, roast.Severity(b.Score))
	line("Account shape: %s. %s", b.Shape, shapes[b.Shape])
	line("Contribution calendar, private included: %s contributions over %d days. %d days active, %d days empty. Longest run of empty days in a row: %d.",
		metric.Thousands(b.Contributions), b.CalendarDays, b.ActiveDays, b.CalendarDays-b.ActiveDays, b.QuietestRun)
	line("")
	line("Measured:")
	for _, m := range b.Metrics {
		line("- %s: %s", m.Label, m.Value)
	}
	line("")
	line("Repositories: %d owned, %d forks.", b.Owned, b.Forked)
	if b.Read > 0 {
		line("Of the %d most recently pushed, %s been touched in a year and %s no README.",
			b.Read, agree(b.Abandoned, "has not", "have not"), agree(b.NoReadme, "has", "have"))
		line("Stars: %d earned across those, %d handed out to other people's.", b.StarsEarned, b.StarsGiven)
	}
	if len(b.Languages) > 0 {
		var parts []string
		for _, l := range b.Languages {
			if len(parts) == 3 {
				break
			}
			parts = append(parts, fmt.Sprintf("%s %d%%", l.Language, l.Percent))
		}
		line("Main languages: %s.", strings.Join(parts, ", "))
	}
	line("Last year: %d commits, %d pull requests, %d reviews, %d issues. %d%% of all contributions were private.",
		b.Year.Commits, b.Year.PullRequests, b.Year.Reviews, b.Year.Issues, b.Private)

	switch {
	case !b.Profile:
		line("Profile README: none.")
	case b.Badges == 0:
		line("Profile README: yes, with no skill badges on it.")
	default:
		line("Profile README: %s.", plural(b.Badges, "skill badge"))
		if len(b.Claimed) > 0 {
			line("Languages claimed on those badges: %s.", strings.Join(b.Claimed, ", "))
		}
		if len(b.Unused) > 0 {
			line("Claimed, but the main language of no repository: %s.", strings.Join(b.Unused, ", "))
		}
	}

	if len(b.Commits) > 0 {
		line("")
		line("Recent commit messages:")
		for _, c := range b.Commits {
			line("- %s", c)
		}
	}
	if len(b.Repos) > 0 {
		line("")
		line("Repositories (name: description):")
		for _, r := range b.Repos {
			line("- %s", r)
		}
	}
	stock := func(title string, items []string) {
		if len(items) == 0 {
			return
		}
		line("")
		line("%s:", title)
		for i, it := range items {
			line("%d. %s", i+1, scrub(it, maxStockText))
		}
	}
	stock("Strengths", b.Strengths)
	stock("Actions", b.Actions)
	stock("Findings", facts(b.Findings))
	stock("Habits", facts(b.Habits))
	if b.Angle != "" {
		line("")
		line("Angle for the verdict: %s", scrub(b.Angle, maxStockText))
	}
	line("</account>")

	if len(b.Avoid) > 0 {
		line("")
		line("Already printed at this stand. Write something that shares none of their jokes:")
		for _, a := range b.Avoid {
			line("- %s", a)
		}
	}
	return s.String()
}

var shapes = map[metric.Shape]string{
	metric.Ghost:   "Barely here. Never call it busy.",
	metric.Dabbler: "Shows up now and then. Never call it busy or dead.",
	metric.Regular: "A normal pace. Never call it lazy or obsessed.",
	metric.Grinder: "Busy most days, weekends too. Never call it lazy.",
	metric.Machine: "Enormous volume, almost no days off. Never call it lazy.",
}

var (
	ticketRef = regexp.MustCompile(`\s*\(#\d+\)`)
	plainWord = regexp.MustCompile(`^[A-Za-z][A-Za-z']*[,.:!?]?$`)
	noise     = regexp.MustCompile(`\d+\.\d+|://|\w[_/]\w+\.\w+`)
)

func readable(commit string) bool {
	if noise.MatchString(commit) {
		return false
	}
	fields, words := strings.Fields(commit), 0
	for _, w := range fields {
		if plainWord.MatchString(w) {
			words++
		}
	}
	return words >= 3 && words*10 >= len(fields)*6
}

var characterful = map[string]bool{
	"Favourite first words": true,
	"Busiest day":           true,
	"Peak hour":             true,
	"The graveyard":         true,
	"Oldest untouched repo": true,
	"Main language":         true,
	"Forks":                 true,
}

func Angles(b Brief) []string {
	var out []string
	for _, c := range b.Commits {
		if c = strings.TrimSpace(ticketRef.ReplaceAllString(c, "")); readable(c) {
			out = append(out, fmt.Sprintf("the commit message %q, quoted word for word.", c))
		}
	}
	for _, r := range b.Repos {
		out = append(out, "the repository "+r+".")
	}
	if len(b.Unused) > 0 {
		out = append(out, "the profile badges claim "+strings.Join(b.Unused, ", ")+", and no repo read is written in it.")
	}
	for _, f := range append(append([]roast.Finding(nil), b.Habits...), b.Findings...) {
		if characterful[f.Title] {
			out = append(out, fmt.Sprintf("%s: %s.", strings.ToLower(f.Title), f.Value))
		}
	}
	switch b.Shape {
	case metric.Ghost:
		out = append(out, "how empty this account is.")
	case metric.Machine, metric.Grinder:
		out = append(out, "the days off that never happen.")
	}
	return out
}

func PickAngle(b Brief, n func(int) int) string {
	angles := Angles(b)
	if len(angles) == 0 {
		return ""
	}
	return angles[n(len(angles))]
}

func facts(fs []roast.Finding) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = "[" + f.Title + ", " + f.Value + "] " + f.Line
	}
	return out
}

// scrub makes a piece of account text safe to put in front of a model: control
// characters out, whitespace collapsed, capped at a rune boundary, and angle
// brackets swapped for lookalikes, so a commit message cannot close the
// <account> block and start talking as if it were us.
func scrub(s string, max int) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		switch {
		case r == '<':
			r = '‹'
		case r == '>':
			r = '›'
		case unicode.IsSpace(r):
			space = true
			continue
		case unicode.IsControl(r) || !unicode.IsPrint(r):
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}

	out := b.String()
	if utf8.RuneCountInString(out) > max {
		out = string([]rune(out)[:max-1]) + "…"
	}
	return out
}

func private(y github.Year) int {
	public := y.Commits + y.PullRequests + y.Issues + y.Reviews
	total := public + y.Private
	if total == 0 {
		return 0
	}
	return (y.Private*200 + total) / (total * 2)
}

func years(created, now time.Time) int {
	if created.IsZero() || now.Before(created) {
		return 0
	}
	y := now.Year() - created.Year()
	if now.YearDay() < created.YearDay() {
		y--
	}
	return y
}

// agree puts the verb in agreement with the count. The model copies what it
// reads, so a slip in the brief can end up on the paper.
func agree(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
