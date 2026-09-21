package metric

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

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

	var noReadme, undescribed, unlicensed []string
	for _, r := range f.Repos {
		if !r.Readme {
			noReadme = append(noReadme, r.Name)
		}
		if strings.TrimSpace(r.Description) == "" {
			undescribed = append(undescribed, r.Name)
		}
		if r.License == "" {
			unlicensed = append(unlicensed, r.Name)
		}
	}
	gap := quietest(f)

	add(len(f.Repos) == 0, "Create a repository. Git is free, we checked.")
	add(len(f.Repos) > 0 && len(f.Commits) == 0, "Push a commit. Your repos think you died.")
	add(share(f.Commits, oneWord) >= 33, `Learn a second word. "fix" is lonely.`)
	add(share(f.Commits, atNight) >= 33, "Sleep. Commits at 3am are a cry for help.")
	add(len(noReadme) > 0, fmt.Sprintf("Write a README for %s. You will forget how it works by Tuesday.", naming(noReadme)))
	add(len(undescribed) > 0, fmt.Sprintf("Describe %s. Mystery is not a feature.", naming(undescribed)))
	add(gap >= 30, fmt.Sprintf("Beat your %d-day disappearing act", gap))
	add(days(f, func(time.Time) bool { return true }) >= 90, "Take one day off. Just one. We'll wait.")
	add(days(f, isWeekend) >= 50, "Take one Saturday off. The repo will not notice.")
	add(strings.TrimSpace(f.Readme) == "", "Write a profile README. Recruiters can't read silence.")
	add(len(unlicensed) > 0, fmt.Sprintf("License %s before a lawyer does", naming(unlicensed)))
	add(true, "Keep it up. Nobody knows how you do it.")
	add(true, "Frame this receipt. It won't get better.")
	add(true, "Mentor someone. Carefully.")
	return out
}

func naming(repos []string) string {
	switch len(repos) {
	case 0:
		return ""
	case 1:
		return repos[0]
	}
	return fmt.Sprintf("%s and %d more", repos[0], len(repos)-1)
}

// StrengthsMax is how many strengths a review opens with. Two is credit; more
// would be a compliment, and this is a roast.
const StrengthsMax = 2

// Strengths is the credit the account has earned, each line with its sting.
// The strongest evidence goes first; an account with none still gets a line,
// because every review starts with something nice.
func Strengths(f github.Facts) []string {
	var out []string
	add := func(ok bool, s string) {
		if ok && len(out) < StrengthsMax {
			out = append(out, s)
		}
	}

	total, stars := contributed(f), 0
	for _, r := range f.Repos {
		stars += r.Stars
	}
	gap := quietest(f)
	y := f.Year

	add(total >= 1000, fmt.Sprintf("%s contributions this year. Genuinely impressive. Please sleep.", thousands(total)))
	add(len(f.Year.Days) > 0 && total > 0 && gap <= 3, fmt.Sprintf("Never more than %s off all year. Admirable. Mildly alarming.", count(gap, "day")))
	add(stars >= 100, fmt.Sprintf("%s stars. People actually use your stuff, which is brave of them.", thousands(stars)))
	add(y.Reviews >= 20, fmt.Sprintf("%d code reviews this year. Somebody has to read all that code.", y.Reviews))
	add(f.Followers >= 100, fmt.Sprintf("%s followers. They want to see what breaks next.", thousands(f.Followers)))
	add(len(f.Repos) >= 3 && undescribed(f.Repos) == 0, "Every repo has a description. Rare. Almost suspicious.")
	add(len(f.Commits) >= 20 && share(f.Commits, oneWord) == 0, "Zero one-word commit messages. Your reviewers wept with joy.")
	add(total > 0 && total < 1000, fmt.Sprintf("%s this year. It counts. We checked.", count(total, "contribution")))
	add(len(f.Repos) == 0, "Zero bugs in production. Technically.")
	add(true, "Showed up to a roast stand voluntarily. Brave.")
	add(true, "Typed the username correctly on the first try. Probably.")
	return out
}

// thousands writes 85367 as 85,367, the way a receipt should.
func thousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
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
			line = "Fresh account. The bad habits are still installing."
		}
		out = append(out, roast.Finding{Title: "Time served", Value: "since " + f.Created.Format("Jan 2006"), Line: line})
	}

	social := roast.Finding{Title: "Social standing", Value: fmt.Sprintf("%d followers, following %d", f.Followers, f.Following)}
	switch {
	case f.Followers == 0:
		social.Line = "Zero followers. Nobody's watching, so commit whatever you like."
	case f.Following > 2*f.Followers:
		social.Line = fmt.Sprintf("Follows %d, followed back by %d. Networking is going great.", f.Following, f.Followers)
	case f.Followers > 10*max(f.Following, 1):
		social.Line = fmt.Sprintf("%d people follow you. You follow %d back. Celebrity behaviour.", f.Followers, f.Following)
	default:
		social.Line = "A normal follow ratio. We checked twice."
	}
	out = append(out, social)

	if len(f.Repos) == 0 {
		return append(out, roast.Finding{Title: "Stars collected", Value: "0", Line: "No repos, no stars. The maths is flawless."})
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
		starred.Line = fmt.Sprintf("Zero stars across %s. Not even a pity star from yourself.", count(len(f.Repos), "repo"))
	case top.Stars*10 >= stars*8 && len(f.Repos) > 1:
		starred.Line = fmt.Sprintf("%s carries the whole account. The rest are backup dancers.", top.Name)
	case top.Stars*2 > stars:
		starred.Line = fmt.Sprintf("Most of them on %s. The others are still waiting to be discovered.", top.Name)
	default:
		starred.Line = fmt.Sprintf("%s leads with %d. The rest share the crumbs.", top.Name, top.Stars)
	}
	out = append(out, starred)

	if langs := Languages(f.Repos); len(langs) > 0 {
		l := roast.Finding{Title: "Main language", Value: fmt.Sprintf("%s, %d%% of repos", langs[0].Language, langs[0].Percent)}
		switch {
		case len(langs) >= 5:
			l.Line = fmt.Sprintf("%d languages. A polyglot, or just indecisive.", len(langs))
		case langs[0].Percent == 100:
			l.Line = "One language, every repo. Loyal to a fault."
		default:
			l.Line = fmt.Sprintf("Plus %s as side quests.", count(len(langs)-1, "other language"))
		}
		out = append(out, l)
	}

	if f.Forked > 0 {
		line := "Forks a few, builds the rest. Respectable."
		if f.Forked > f.Owned {
			line = "More forks than originals. Great taste in other people's code."
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
			Line:  "Still marked active. Bless it.",
		})
	}

	y := f.Year
	work := roast.Finding{Title: "Teamwork this year", Value: fmt.Sprintf("%s, %s", count(y.PullRequests, "pull request"), count(y.Reviews, "review"))}
	switch {
	case y.PullRequests == 0 && y.Reviews == 0:
		work.Line = "Zero pull requests, zero reviews. Main branch, no witnesses."
	case y.Reviews == 0:
		work.Line = "Opens pull requests, never reviews one. Generous to yourself."
	case y.Reviews > y.PullRequests:
		work.Line = "Reviews more than writes. The team's designated critic."
	default:
		work.Line = "Writes code and reviews it. Somebody hire this person."
	}
	out = append(out, work)

	if y.Private > 0 {
		out = append(out, roast.Finding{
			Title: "Private work",
			Value: count(y.Private, "private contribution"),
			Line:  "The good stuff is private. Allegedly.",
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
			Line:  "Still public, still untouched. A museum nobody visits.",
		})
	}
	if issues > 0 {
		line := "Somebody is still waiting on these."
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
		for _, w := range strings.Fields(strings.ToLower(headline(c.Message))) {
			if w = strings.TrimFunc(w, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }); w != "" {
				counts[w]++
				break
			}
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
	line := fmt.Sprintf(`Starts most messages with "%s". A catchphrase is born.`, top[0])
	switch top[0] {
	case "fix", "fixed", "fixes", "hotfix", "bugfix":
		line = "Mostly fixes. Bold of you to write the bugs first."
	case "wip", "update", "updates", "updated", "changes", "stuff", "misc":
		line = fmt.Sprintf(`"%s" tells nobody anything, and you know it.`, top[0])
	case "add", "added", "adds", "feat", "feature":
		line = "Mostly adding. Deleting is someone else's problem."
	case "merge":
		line = "Mostly merges. Middle management energy."
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
		f.Line = "Before 9am. Suspiciously well rested."
	case peak < 18:
		f.Line = "Office hours. Your manager thanks you."
	default:
		f.Line = "Evening shift. Dinner can wait, apparently."
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
		f.Line = "Weekend warrior. The weekend did not ask for this."
	case time.Friday:
		f.Line = "Ships on Fridays. Brave, or unsupervised."
	case time.Monday:
		f.Line = "Mondays. Fixing whatever Friday shipped."
	default:
		f.Line = "Midweek peak. Boringly professional."
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
		f.Line = "Hemingway would ask for more."
	case tenths < 50:
		f.Line = "Short and sweet. Mostly short."
	case tenths < 90:
		f.Line = "A sensible length. Suspiciously professional."
	default:
		f.Line = "Writes commit messages like cover letters."
	}
	return f
}
