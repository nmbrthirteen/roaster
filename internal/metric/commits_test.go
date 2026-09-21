package metric

import (
	"testing"
	"time"

	"github.com/upgaming/roaster/internal/github"
)

func TestTheFeedRunsNewestFirstAcrossRepositories(t *testing.T) {
	older, newer := at(10, 14, "Add the login page"), at(18, 9, "fix")
	older.Repo, newer.Repo = "api", "web"

	feed := Feed([]github.Commit{older, newer}, FeedMax)

	if len(feed) != 2 || feed[0].Text != "fix" {
		t.Fatalf("want the newest commit first, got %+v", feed)
	}
	if got := Feed([]github.Commit{older, newer}, 1); len(got) != 1 {
		t.Errorf("the feed should stop at the cap, got %d", len(got))
	}
}

func TestTheWorstCommitIsTheLaziestMessage(t *testing.T) {
	commits := []github.Commit{
		at(15, 14, "Stop the receipt reading as random"),
		at(16, 3, "asdf"),
		at(17, 11, "fix"),
	}

	w := Worst(commits)

	if w == nil || w.Text != "asdf" {
		t.Fatalf("a one-word message at three in the morning should win, got %+v", w)
	}
}

func TestWhatGitHubWroteIsNotTheWorstCommit(t *testing.T) {
	w := Worst([]github.Commit{
		at(15, 3, "Initial commit"),
		at(16, 2, "Merge pull request #4 from someone/branch"),
		at(17, 11, "Tidy the launcher"),
	})

	if w == nil || w.Text != "Tidy the launcher" {
		t.Fatalf("merges and the first commit should lose to anything written by hand, got %+v", w)
	}
}

func TestAnAccountWithNoCommitsHasNothingToQuote(t *testing.T) {
	if w := Worst(nil); w != nil {
		t.Errorf("want nil, got %+v", w)
	}
}

func TestAQuotedMessageIsOneCleanLine(t *testing.T) {
	w := Worst([]github.Commit{at(15, 3, "fix\x07 it\nsecond line")})

	if w.Text != "fix it" {
		t.Errorf("want the first line without control characters, got %q", w.Text)
	}
}

func TestTheCalendarEndsOnTheLastDayGitHubReturned(t *testing.T) {
	// 5 to 23 September 2026: Saturday to Wednesday.
	var days []github.Day
	for d := 5; d <= 23; d++ {
		days = append(days, github.Day{Date: time.Date(2026, time.September, d, 0, 0, 0, 0, time.UTC), Count: d})
	}

	heat := Heat(days, 3)

	if len(heat) != 3 {
		t.Fatalf("want three weeks, got %d", len(heat))
	}
	if heat[0][0] != 6 {
		t.Errorf("the first column should open on Sunday the 6th, got %d", heat[0][0])
	}
	if last := heat[2]; last[3] != 23 || last[4] != -1 || last[6] != -1 {
		t.Errorf("this week should run to Wednesday and leave the rest to come, got %v", last)
	}
}

func TestAnEmptyCalendarStillPrintsAGrid(t *testing.T) {
	days := []github.Day{
		{Date: time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)},
		{Date: time.Date(2026, time.September, 21, 0, 0, 0, 0, time.UTC)},
	}

	heat := Heat(days, HeatWeeks)

	if len(heat) != HeatWeeks {
		t.Fatalf("want %d weeks, got %d", HeatWeeks, len(heat))
	}
	if heat[HeatWeeks-1][1] != 0 || heat[HeatWeeks-2][3] != 0 {
		t.Errorf("a day GitHub skipped inside the window is a quiet day, got %v", heat[HeatWeeks-2:])
	}
	if Heat(nil, HeatWeeks) != nil {
		t.Errorf("no calendar at all should print nothing")
	}
}
