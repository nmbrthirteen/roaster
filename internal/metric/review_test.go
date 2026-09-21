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
	for _, want := range []string{"second word", "Sleep.", "1 README."} {
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
