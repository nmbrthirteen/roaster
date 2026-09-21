package metric

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/upgaming/roaster/internal/github"
	"github.com/upgaming/roaster/internal/roast"
)

// ActionsMax is how many action items a review ends on. Three fit the paper
// and still read as a list someone might ignore.
const ActionsMax = 3

// Actions is the to-do list at the end of the review, worst habit first. Each
// item is there because a number earned it, so it names the number.
func Actions(f github.Facts) []string {
	var out []string
	add := func(ok bool, s string) {
		if ok && len(out) < ActionsMax {
			out = append(out, s)
		}
	}

	noReadme, undescribedN, unlicensed := 0, 0, 0
	for _, r := range f.Repos {
		if !r.Readme {
			noReadme++
		}
		if strings.TrimSpace(r.Description) == "" {
			undescribedN++
		}
		if r.License == "" {
			unlicensed++
		}
	}
	gap := quietest(f)

	add(len(f.Repos) == 0, "Create a repository. Git is free, we checked.")
	add(len(f.Repos) > 0 && len(f.Commits) == 0, "Push a commit. Your repos think you died.")
	add(share(f.Commits, oneWord) >= 33, `Learn a second word. "fix" is lonely.`)
	add(share(f.Commits, atNight) >= 33, "Sleep. Commits at 3am are a cry for help.")
	add(noReadme > 0, fmt.Sprintf("Write %s. Future you forgot already.", count(noReadme, "README")))
	add(undescribedN > 0, fmt.Sprintf("Describe %s. Mystery is not a feature.", count(undescribedN, "repo")))
	add(gap >= 30, fmt.Sprintf("Beat your %d-day disappearing act", gap))
	add(share(f.Commits, atWeekend) >= 33, "Touch grass on a Saturday. Git will wait.")
	add(strings.TrimSpace(f.Readme) == "", "Write a profile README. Recruiters can't read silence.")
	add(unlicensed > 0, fmt.Sprintf("License %s before a lawyer finds them", count(unlicensed, "repo")))
	add(true, "Keep it up. Nobody knows how you do it.")
	add(true, "Frame this receipt. It won't get better.")
	add(true, "Mentor someone. Carefully.")
	return out
}

// Findings are the extra rounds the share page has room for: facts the
// gauges leave out, each with its line. Each one only appears when the
// account has the data behind it.
func Findings(f github.Facts, now time.Time) []roast.Finding {
	var out []roast.Finding

	if !f.Created.IsZero() {
		years := yearsSince(f.Created, now)
		line := fmt.Sprintf("%s on GitHub and %s to show for it.", count(years, "year"), count(f.Owned, "repo"))
		if years == 0 {
			line = "New here. The bad habits are still loading."
		}
		out = append(out, roast.Finding{Title: "Time served", Value: "since " + f.Created.Format("Jan 2006"), Line: line})
	}

	social := roast.Finding{Title: "Social standing", Value: fmt.Sprintf("%d followers, following %d", f.Followers, f.Following)}
	switch {
	case f.Followers == 0:
		social.Line = "Zero followers. The purest audience there is."
	case f.Following > 2*f.Followers:
		social.Line = "Follows more people than follow back. Networking is going great."
	case f.Followers > 10*max(f.Following, 1):
		social.Line = "People follow this account. It follows almost no one back."
	default:
		social.Line = "A balanced social graph. Suspiciously healthy."
	}
	out = append(out, social)

	if len(f.Repos) == 0 {
		return append(out, roast.Finding{Title: "Stars collected", Value: "0", Line: "No repos, no stars. The maths checks out."})
	}

	stars, top := 0, f.Repos[0]
	for _, r := range f.Repos {
		stars += r.Stars
		if r.Stars > top.Stars {
			top = r
		}
	}
	starred := roast.Finding{Title: "Stars collected", Value: fmt.Sprint(stars)}
	switch {
	case stars == 0:
		starred.Line = fmt.Sprintf("Zero stars across %s. Not even from yourself.", count(len(f.Repos), "repo"))
	case top.Stars*10 >= stars*8 && len(f.Repos) > 1:
		starred.Line = fmt.Sprintf("%s carries the whole account. The rest are along for the ride.", top.Name)
	default:
		starred.Line = fmt.Sprintf("Most of them on %s. Framed and hung, presumably.", top.Name)
	}
	out = append(out, starred)

	if langs := Languages(f.Repos); len(langs) > 0 {
		l := roast.Finding{Title: "Main language", Value: fmt.Sprintf("%s, %d%% of repos", langs[0].Language, langs[0].Percent)}
		switch {
		case len(langs) >= 5:
			l.Line = fmt.Sprintf("%d languages in total. Fluent in all of them, surely.", len(langs))
		case langs[0].Percent == 100:
			l.Line = "One language, every time. Loyalty is rare."
		default:
			l.Line = fmt.Sprintf("Dabbles in %s. Commitment is a work in progress.", count(len(langs)-1, "other language"))
		}
		out = append(out, l)
	}

	if f.Forked > 0 {
		line := "Forks some, writes more. A healthy ratio."
		if f.Forked > f.Owned {
			line = "More forks than originals. A collector, not a builder."
		}
		out = append(out, roast.Finding{Title: "Forks", Value: fmt.Sprintf("%d forked, %d original", f.Forked, f.Owned), Line: line})
	}

	var stale *github.Repo
	for i, r := range f.Repos {
		if r.Archived || r.Pushed.IsZero() {
			continue
		}
		if stale == nil || r.Pushed.Before(stale.Pushed) {
			stale = &f.Repos[i]
		}
	}
	if stale != nil && now.Sub(stale.Pushed) > 180*24*time.Hour {
		out = append(out, roast.Finding{
			Title: "Oldest untouched repo",
			Value: stale.Name + ", " + ago(stale.Pushed, now),
			Line:  "Never archived. Just left running, like a tap.",
		})
	}

	y := f.Year
	work := roast.Finding{Title: "Teamwork this year", Value: fmt.Sprintf("%s, %s", count(y.PullRequests, "pull request"), count(y.Reviews, "review"))}
	switch {
	case y.PullRequests == 0 && y.Reviews == 0:
		work.Line = "Works alone. Merges alone. Deploys alone."
	case y.Reviews == 0:
		work.Line = "Opens pull requests, reviews none. Takes, never gives."
	case y.Reviews > y.PullRequests:
		work.Line = "Reviews more than writes. A critic in the making."
	default:
		work.Line = "Writes and reviews. Someone might actually hire this."
	}
	out = append(out, work)

	if y.Private > 0 {
		out = append(out, roast.Finding{
			Title: "Private work",
			Value: count(y.Private, "private contribution"),
			Line:  "The best code is where nobody can see it. Allegedly.",
		})
	}
	return out
}

// quietest is the longest run of days in the contribution year with nothing
// on it, in days.
func quietest(f github.Facts) int {
	longest, run := 0, 0
	for _, d := range f.Year.Days {
		if d.Count > 0 {
			run = 0
			continue
		}
		run++
		longest = max(longest, run)
	}
	return longest
}

// Habits are what the commits say about how someone works: the words they
// reach for, the hour they reach for them. Nil with no commits to read.
func Habits(f github.Facts, now time.Time) []roast.Finding {
	var out []roast.Finding
	if len(f.Commits) > 0 {
		out = append(out, vocabulary(f.Commits), peakHour(f.Commits), busiestDay(f.Commits), wordiness(f.Commits))
	}

	dead, issues := 0, 0
	for _, r := range f.Repos {
		issues += r.OpenIssues
		if !r.Archived && !r.Pushed.IsZero() && now.Sub(r.Pushed) > 365*24*time.Hour {
			dead++
		}
	}
	if dead > 0 {
		out = append(out, roast.Finding{
			Title: "The graveyard",
			Value: fmt.Sprintf("%s untouched for a year", count(dead, "repo")),
			Line:  "Not archived, not deleted. Just resting.",
		})
	}
	if issues > 0 {
		line := "Somebody out there is still waiting."
		if issues >= 20 {
			line = "A backlog with its own weather system."
		}
		out = append(out, roast.Finding{Title: "Open issues", Value: fmt.Sprint(issues), Line: line})
	}
	return out
}

func vocabulary(commits []github.Commit) roast.Finding {
	counts := map[string]int{}
	for _, c := range commits {
		if w := strings.Fields(strings.ToLower(headline(c.Message))); len(w) > 0 {
			counts[strings.Trim(w[0], ".:!,")]++
		}
	}
	words := make([]string, 0, len(counts))
	for w := range counts {
		if w != "" {
			words = append(words, w)
		}
	}
	sort.Slice(words, func(i, j int) bool {
		if counts[words[i]] != counts[words[j]] {
			return counts[words[i]] > counts[words[j]]
		}
		return words[i] < words[j]
	})
	if len(words) == 0 {
		return roast.Finding{Title: "Favourite first word", Value: "none", Line: "Commit messages with no words. Bold."}
	}
	top := words[:min(3, len(words))]
	parts := make([]string, len(top))
	for i, w := range top {
		parts[i] = fmt.Sprintf("%s ×%d", w, counts[w])
	}
	line := fmt.Sprintf(`Opens with "%s" more than anything. A signature move.`, top[0])
	switch top[0] {
	case "fix", "fixed", "fixes", "hotfix", "bugfix":
		line = "Mostly fixing. Who wrote all these bugs, then?"
	case "wip", "update", "updates", "updated", "changes", "stuff", "misc":
		line = fmt.Sprintf(`"%s" is not a description. It's a shrug.`, top[0])
	case "add", "added", "adds", "feat", "feature":
		line = "Mostly adding things. Removing them is somebody else's job."
	case "merge":
		line = "Mostly merges. A manager in developer's clothing."
	}
	return roast.Finding{Title: "Favourite first words", Value: strings.Join(parts, ", "), Line: line}
}

func peakHour(commits []github.Commit) roast.Finding {
	var hours [24]int
	for _, c := range commits {
		hours[c.At.Hour()]++
	}
	peak := 0
	for h, n := range hours {
		if n > hours[peak] {
			peak = h
		}
	}
	f := roast.Finding{Title: "Peak hour", Value: fmt.Sprintf("%02d:00", peak)}
	switch {
	case peak < nightEnds:
		f.Line = "Nothing good was ever committed at this hour."
	case peak < 9:
		f.Line = "An early riser. Suspicious, frankly."
	case peak < 18:
		f.Line = "Commits in office hours. Somebody's manager is happy."
	default:
		f.Line = "The evening shift. Dinner can wait, apparently."
	}
	return f
}

func busiestDay(commits []github.Commit) roast.Finding {
	var days [7]int
	for _, c := range commits {
		days[c.At.Weekday()]++
	}
	top := 0
	for d, n := range days {
		if n > days[top] {
			top = d
		}
	}
	day := time.Weekday(top)
	f := roast.Finding{Title: "Busiest day", Value: day.String()}
	switch day {
	case time.Saturday, time.Sunday:
		f.Line = "Weekends are for coding, apparently. And only coding."
	case time.Friday:
		f.Line = "Ships on Fridays. Brave, or unsupervised."
	case time.Monday:
		f.Line = "Peaks on Monday. Fixing what Friday shipped."
	default:
		f.Line = "A midweek worker. Unremarkable, in the best way."
	}
	return f
}

func wordiness(commits []github.Commit) roast.Finding {
	words := 0
	for _, c := range commits {
		words += len(strings.Fields(headline(c.Message)))
	}
	tenths := words * 10 / len(commits)
	f := roast.Finding{Title: "Words per message", Value: fmt.Sprintf("%d.%d", tenths/10, tenths%10)}
	switch {
	case tenths < 20:
		f.Line = "Hemingway would find this too short."
	case tenths < 50:
		f.Line = "Short and to the point. Mostly short."
	default:
		f.Line = "Writes commit messages like cover letters."
	}
	return f
}
