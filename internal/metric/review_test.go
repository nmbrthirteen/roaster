package metric

import (
	"strings"
	"testing"
	"time"

	"github.com/upgaming/roaster/internal/github"
)

func TestActionsEmptyAccount(t *testing.T) {
	got := Actions(github.Facts{Handle: "ghost"})
	if len(got) != ActionsMax {
		t.Fatalf("got %d actions, want %d: %v", len(got), ActionsMax, got)
	}
	if got[0] != "Create a repository. Git is free, we checked." {
		t.Errorf("first action = %q", got[0])
	}
}

func TestActionsNameTheirNumbers(t *testing.T) {
	at := time.Date(2026, 3, 4, 2, 0, 0, 0, time.UTC)
	f := github.Facts{
		Repos:   []github.Repo{{Name: "a"}, {Name: "b", Readme: true, Description: "b"}},
		Commits: []github.Commit{{Message: "fix", At: at}, {Message: "wip", At: at}},
		Readme:  "hi",
	}
	got := strings.Join(Actions(f), "\n")
	for _, want := range []string{"second word", "Sleep.", "README for a."} {
		if !strings.Contains(got, want) {
			t.Errorf("actions missing %q:\n%s", want, got)
		}
	}
}

func TestFindingsOnlyWhatTheDataBacks(t *testing.T) {
	now := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	empty := Findings(github.Facts{Handle: "ghost"}, now)
	for _, f := range empty {
		if f.Title == "Main language" || f.Title == "Forks" || f.Title == "Private work" {
			t.Errorf("empty account got %q", f.Title)
		}
	}

	full := Findings(github.Facts{
		Created: now.AddDate(-5, 0, 0),
		Owned:   2,
		Forked:  3,
		Repos: []github.Repo{
			{Name: "big", Stars: 90, Language: "Go", Pushed: now.AddDate(-2, 0, 0)},
			{Name: "small", Stars: 1, Language: "Go", Pushed: now},
		},
		Year: github.Year{PullRequests: 2, Private: 5},
	}, now)
	titles := map[string]string{}
	for _, f := range full {
		titles[f.Title] = f.Value + " | " + f.Line
	}
	for _, want := range []string{"Time served", "Stars collected", "Main language", "Forks", "Oldest untouched repo", "Teamwork this year", "Private work"} {
		if _, ok := titles[want]; !ok {
			t.Errorf("missing finding %q in %v", want, titles)
		}
	}
	if !strings.Contains(titles["Stars collected"], "big carries") {
		t.Errorf("stars = %q", titles["Stars collected"])
	}
	if !strings.Contains(titles["Oldest untouched repo"], "big, 2y ago") {
		t.Errorf("oldest = %q", titles["Oldest untouched repo"])
	}
}

func TestHabits(t *testing.T) {
	now := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	if got := Habits(github.Facts{}, now); len(got) != 0 {
		t.Errorf("empty account got habits %v", got)
	}
	fri := time.Date(2026, 9, 18, 2, 30, 0, 0, time.UTC)
	got := Habits(github.Facts{
		Commits: []github.Commit{{Message: "fix: login", At: fri}, {Message: "Fix typo", At: fri}, {Message: "wip", At: fri}},
		Repos:   []github.Repo{{Name: "old", Pushed: now.AddDate(-2, 0, 0), OpenIssues: 3}},
		Year: github.Year{Days: []github.Day{
			{Date: time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC), Count: 1},
			{Date: fri, Count: 3},
		}},
	}, now)
	byTitle := map[string]string{}
	for _, f := range got {
		byTitle[f.Title] = f.Value
	}
	want := map[string]string{
		"Favourite first words": "fix ×2, wip ×1",
		"Peak hour":             "02:00",
		"Busiest day":           "Friday",
		"Words per message":     "1.6",
		"The graveyard":         "1 repo untouched for a year",
		"Open issues":           "3",
	}
	for k, v := range want {
		if byTitle[k] != v {
			t.Errorf("%s = %q, want %q", k, byTitle[k], v)
		}
	}
}

func TestBusiestDayFollowsTheCalendarNotTheLastFewCommits(t *testing.T) {
	sunday := time.Date(2026, 9, 20, 15, 0, 0, 0, time.UTC)
	var days []github.Day
	for i := range 7 {
		d := github.Day{Date: time.Date(2026, 9, 14+i, 0, 0, 0, 0, time.UTC), Count: 1}
		if d.Date.Weekday() == time.Monday {
			d.Count = 200
		}
		days = append(days, d)
	}
	got := busiestDay(github.Facts{
		Commits: []github.Commit{{Message: "a b", At: sunday}, {Message: "c d", At: sunday}},
		Year:    github.Year{Days: days},
	})
	if got.Value != "Monday" {
		t.Errorf("busiest day = %q, want Monday from the calendar", got.Value)
	}

	quiet := busiestDay(github.Facts{Year: github.Year{Days: []github.Day{{Date: sunday, Count: 4}}}})
	if strings.Contains(quiet.Line, "Weekend warrior") || !strings.Contains(quiet.Line, "4 contributions") {
		t.Errorf("four contributions are not a weekend habit, got %q", quiet.Line)
	}
}

func TestAnEmptyAccountIsUntestedNotPraised(t *testing.T) {
	for _, m := range From(github.Facts{}) {
		if m.Percent != nil && m.Tag != "untested" {
			t.Errorf("%s tagged %q on an account with nothing to measure", m.Label, m.Tag)
		}
	}
}

func TestPunctuationIsNotAWord(t *testing.T) {
	got := vocabulary([]github.Commit{{Message: "- fix"}, {Message: "fix."}, {Message: "..."}})
	if got.Value != "fix ×2" {
		t.Errorf("vocabulary = %q, want fix ×2", got.Value)
	}
}

func TestStrengthsCreditTheWorkOnce(t *testing.T) {
	f := github.Facts{Year: github.Year{Days: []github.Day{
		{Date: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Count: 900},
		{Date: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), Count: 1400},
	}}}
	got := Strengths(f)
	if len(got) != StrengthsMax {
		t.Fatalf("got %d strengths: %v", len(got), got)
	}
	if got[0] != "2,300 contributions this year. Genuinely impressive. Please sleep." {
		t.Errorf("first strength = %q", got[0])
	}
	for _, s := range got[1:] {
		if strings.Contains(s, "contributions this year") {
			t.Errorf("the count is credited twice: %v", got)
		}
	}
	if empty := Strengths(github.Facts{}); len(empty) != StrengthsMax || empty[0] != "Zero bugs in production. Technically." {
		t.Errorf("an empty account still gets its strengths, got %v", empty)
	}
}

func TestThousands(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 297218: "297,218", 1234567: "1,234,567"} {
		if got := Thousands(n); got != want {
			t.Errorf("Thousands(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestStarsSayMostOnlyWhenItIs(t *testing.T) {
	now := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	line := func(stars ...int) string {
		f := github.Facts{}
		for i, n := range stars {
			f.Repos = append(f.Repos, github.Repo{Name: string(rune('a' + i)), Stars: n})
		}
		for _, x := range Findings(f, now) {
			if x.Title == "Stars collected" {
				return x.Line
			}
		}
		return ""
	}
	if got := line(5, 3, 3, 1); strings.HasPrefix(got, "Most") {
		t.Errorf("5 of 12 is not most: %q", got)
	}
	if got := line(7, 3, 2); !strings.HasPrefix(got, "Most of them on a") {
		t.Errorf("7 of 12 is most: %q", got)
	}
}

func TestAnOrdinaryMessageLengthIsNotMocked(t *testing.T) {
	msg := github.Commit{Message: "add retry to the upload client"}
	if got := wordiness([]github.Commit{msg, msg}).Line; strings.Contains(got, "cover letters") {
		t.Errorf("six words got %q", got)
	}
}
