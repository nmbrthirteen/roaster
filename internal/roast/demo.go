package roast

import (
	"context"
	"fmt"
	"hash/fnv"
	"math/rand"
	"strings"
	"time"

	"github.com/upgaming/roaster/internal/github"
)

// Demo stands in for the real audit until the GitHub adapter and the model call
// land.
type Demo struct{}

// Reserved handles for exercising paths that are otherwise hard to reach.
const (
	handleMissing = "notfound" // the account does not exist
	handleSlow    = "slowpoke" // a sluggish upstream
	handleEmpty   = "ghost"    // an account with nothing public
)

func (Demo) Roast(ctx context.Context, req Request, emit func(Update)) (Roast, error) {
	handle := strings.TrimSpace(req.Handle)
	rng := rand.New(rand.NewSource(seed(handle)))

	pace := time.Duration(1)
	if strings.EqualFold(handle, handleSlow) {
		pace = 3
	}
	step := func(d time.Duration) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d * pace):
			return nil
		}
	}

	r := build(handle, rng)

	emit(Update{Phase: PhaseFetch, Label: "Reading public commits"})
	if err := step(900 * time.Millisecond); err != nil {
		return Roast{}, err
	}

	if strings.EqualFold(handle, handleMissing) {
		return Roast{}, fmt.Errorf("no GitHub account called @%s", handle)
	}
	// The same check the real audit makes, so a rehearsal refuses the same names.
	if !github.Valid(strings.TrimPrefix(handle, "@")) {
		return Roast{}, fmt.Errorf("%q %w", handle, github.ErrBadHandle)
	}
	r.Pack = req.Pack
	var feed []Item
	if strings.EqualFold(handle, handleEmpty) {
		r.Exhibit = nil
		for w := range r.Heat {
			for d := range r.Heat[w] {
				r.Heat[w][d] = min(r.Heat[w][d], 0)
			}
		}
	} else {
		feed = history(rng, r.At)
		r.Exhibit = &feed[len(feed)/3]
	}
	r.Story = demoStory(handle, len(feed) == 0)
	if len(feed) == 0 {
		r.Actions = []string{"Create a repository. Git is free, we checked.", "Write a profile README. Recruiters can't read silence.", "Frame this receipt. It won't get better."}
		r.Findings = demoFindings[:2]
		r.Strengths = []string{"Zero bugs in production. Technically.", "Showed up to a roast stand voluntarily. Brave."}
		r.Habits = nil
		r.Archetype = "Stealth Mode Founder"
	}
	for _, s := range r.Story {
		section := s
		emit(Update{Phase: PhaseSection, Section: &section})
	}
	emit(Update{Phase: PhaseFeed, Feed: feed})

	for _, m := range r.Metrics {
		metric := m
		emit(Update{Phase: PhaseMetric, Metric: &metric})
		if err := step(280 * time.Millisecond); err != nil {
			return Roast{}, err
		}
	}

	emit(Update{Phase: PhaseVerdict, Label: "Writing the verdict"})
	if err := step(1500 * time.Millisecond); err != nil {
		return Roast{}, err
	}
	emit(Update{Phase: PhaseVerdict, Verdict: r.Verdict})

	emit(Update{Phase: PhaseDone, Roast: &r})
	return r, nil
}

func seed(handle string) int64 {
	h := fnv.New64a()
	h.Write([]byte(strings.ToLower(handle)))
	return int64(h.Sum64())
}

func Sample(handle string) Roast {
	rng := rand.New(rand.NewSource(7))
	r := build(handle, rng)
	feed := history(rng, r.At)
	r.Exhibit = &feed[len(feed)/3]
	return r
}

func build(handle string, rng *rand.Rand) Roast {
	metrics := []Metric{
		gauge("Commits after midnight", rng, 15, 90, [3]string{"sleeps", "owl", "vampire"}),
		gauge("Active weekends", rng, 5, 85, [3]string{"rested", "restless", "no brakes"}),
		gauge("Active days", rng, 10, 95, [3]string{"casual", "committed", "no off switch"}),
		gauge("Repos with no description", rng, 20, 95, [3]string{"clear", "vague", "ghosted"}),
		gauge("One-word commit messages", rng, 20, 95, [3]string{"poet", "brief", "caveman"}),
		{Label: "Longest quiet stretch", Value: fmt.Sprintf("%d days", 40+rng.Intn(400))},
	}

	score := Score(metrics)
	now := time.Now()

	return Roast{
		Code:      Code(),
		Handle:    handle,
		At:        now,
		Score:     fmt.Sprintf("%d / 100", score),
		ScoreTag:  Severity(score),
		Archetype: pick(rng, []string{"Head of Night Shifts", "Senior Fix Engineer", "Chief Weekend Officer", "VP of Force Pushing"}),
		Metrics:   metrics,
		Verdict:   pick(rng, verdicts),
		Actions:   demoActions,
		Strengths: []string{
			"1,204 contributions this year. Genuinely impressive. Please sleep.",
			"62 code reviews this year. Somebody has to read all that code.",
		},
		Findings: demoFindings,
		Habits:   demoHabits,
		Heat:     calendar(rng, now),
	}
}

var (
	demoRepos    = []string{"dotfiles", "api-server", "portfolio-v3", "todo-app"}
	demoMessages = []string{
		"fix", "wip", "asdf", "Add login page", "fix typo", "update", ".",
		"Refactor the refactor", "please work", "remove console.log", "final",
		"Bump dependencies", "revert revert", "it works on my machine",
	}
)

func history(rng *rand.Rand, now time.Time) []Item {
	out := make([]Item, 12+rng.Intn(10))
	at := now
	for i := range out {
		at = at.Add(-time.Duration(1+rng.Intn(40)) * time.Hour)
		out[i] = Item{
			Ref:   fmt.Sprintf("%07x", rng.Int63n(1<<28)),
			Where: pick(rng, demoRepos),
			Text:  pick(rng, demoMessages),
			At:    at,
		}
	}
	return out
}

// demoStory is the reading a real audit would show, for a rehearsal.
func demoStory(handle string, empty bool) []Section {
	user := func(created string, repos, followers, following int) Section {
		return Section{
			Cmd: "gh api users/" + handle + " --jq '{login, created_at, public_repos, followers, following}'",
			Lines: []string{"{", fmt.Sprintf(`  "login": %q,`, handle), fmt.Sprintf(`  "created_at": %q,`, created),
				fmt.Sprintf(`  "public_repos": %d,`, repos), fmt.Sprintf(`  "followers": %d,`, followers),
				fmt.Sprintf(`  "following": %d`, following), "}"},
		}
	}
	readme := "gh api repos/" + handle + "/" + handle + "/readme -H 'Accept: application/vnd.github.raw' | head -5"
	list := "gh repo list " + handle + " --limit 5"
	year := func(commits, prs, reviews, private int, gap string) Section {
		return Section{
			Cmd: "gh api graphql -F login=" + handle + " -f query=@contributions.graphql --jq .data.user.contributionsCollection",
			Lines: []string{"{", fmt.Sprintf(`  "totalCommitContributions": %d,`, commits),
				fmt.Sprintf(`  "totalPullRequestContributions": %d,`, prs),
				fmt.Sprintf(`  "totalPullRequestReviewContributions": %d,`, reviews),
				fmt.Sprintf(`  "restrictedContributionsCount": %d`, private), "}", "# longest quiet stretch: " + gap + ". we assume a sabbatical."},
		}
	}
	if empty {
		return []Section{
			user("2021-04-11T09:30:12Z", 0, 0, 3),
			{Cmd: readme, Lines: []string{"gh: Not Found (HTTP 404)", "# no profile README. the strong, silent type."}},
			{Cmd: list, Lines: []string{"no repositories match your search in @" + handle, "# an empty stage. we roast what we can."}},
			year(0, 0, 0, 0, "365 days"),
		}
	}
	return []Section{
		user("2017-03-02T10:12:40Z", 23, 41, 212),
		{Cmd: readme, Lines: []string{
			"<h1 align=\"center\">Hi, I'm " + handle + " 👋</h1>",
			"<p align=\"center\">Full-stack developer. Rust enthusiast.</p>",
			"![Go](https://img.shields.io/badge/Go-00ADD8?logo=go)",
			"![Rust](https://img.shields.io/badge/Rust-000?logo=rust)",
			"![TypeScript](https://img.shields.io/badge/TypeScript-3178C6)",
			"# 14 skill badges", "# claims: Go, Rust, TypeScript", "# not one repo written in Rust. bold claim."}},
		{Cmd: list, Lines: []string{
			"Showing 5 of 23 repositories in @" + handle,
			fmt.Sprintf("%-18s %-11s %s", "NAME", "LANGUAGE", "UPDATED"),
			fmt.Sprintf("%-18s %-11s %s", "api-server", "Go", "2d ago"),
			fmt.Sprintf("%-18s %-11s %s", "dotfiles", "Shell", "5d ago"),
			fmt.Sprintf("%-18s %-11s %s", "portfolio-v3", "TypeScript", "2mo ago"),
			fmt.Sprintf("%-18s %-11s %s", "todo-app", "JavaScript", "3y ago"),
			fmt.Sprintf("%-18s %-11s %s", "rust-learning", "Rust", "4y ago  archived"),
			"# 11 of 23 have no description. guess the plot.", "# 9 of 23 have no README. good luck, whoever clones these."}},
		year(412, 18, 3, 97, "41 days"),
	}
}

// calendar ends on today's weekday, the way GitHub's does.
func calendar(rng *rand.Rand, now time.Time) [][7]int {
	out := make([][7]int, 38)
	for w := range out {
		for d := range out[w] {
			switch {
			case w == len(out)-1 && d > int(now.Weekday()):
				out[w][d] = -1
			case rng.Intn(3) == 0:
				out[w][d] = 1 + rng.Intn(12)
			}
		}
	}
	return out
}

// gauge always carries a tag. Tagging only the bad rows left the column ragged
// and made the whole block look arbitrary on paper.
func gauge(label string, rng *rand.Rand, lo, hi int, bands [3]string) Metric {
	n := lo + rng.Intn(hi-lo+1)
	band := bands[0]
	switch {
	case n >= 66:
		band = bands[2]
	case n >= 33:
		band = bands[1]
	}
	return Metric{Label: label, Value: fmt.Sprintf("%d%%", n), Tag: band, Percent: &n}
}

var (
	demoActions = []string{
		`Learn a second word. "fix" is lonely.`,
		"Sleep. Commits at 3am are a cry for help.",
		"Write a README for roaster and 6 more. You will forget how it works by Tuesday.",
	}
	demoFindings = []Finding{
		{Title: "Time served", Value: "since Mar 2016", Line: "10 years on GitHub and 14 repos to show for it."},
		{Title: "Social standing", Value: "12 followers, following 87", Line: "Follows 87, followed back by 12. Networking is going great."},
		{Title: "Stars collected", Value: "9", Line: "dotfiles carries the whole account. The rest are backup dancers."},
		{Title: "Main language", Value: "TypeScript, 57% of repos", Line: "Plus 3 other languages as side quests."},
		{Title: "Teamwork this year", Value: "4 pull requests, 0 reviews", Line: "Opens pull requests, never reviews one. Generous to yourself."},
	}
	demoHabits = []Finding{
		{Title: "Favourite first words", Value: "fix ×23, update ×11, wip ×5", Line: "Mostly fixes. Bold of you to write the bugs first."},
		{Title: "Peak hour", Value: "02:00", Line: "Nothing good was ever committed at this hour."},
		{Title: "Busiest day", Value: "Friday", Line: "Ships on Fridays. Brave, or unsupervised."},
		{Title: "Words per message", Value: "1.8", Line: "Hemingway would ask for more."},
		{Title: "The graveyard", Value: "6 repos untouched for a year", Line: "Still public, still untouched. A museum nobody visits."},
	}
)

// Score leans on the tallest gauge as much as on the average. The tallest bar
// is the headline of the receipt, and a plain average let four quiet bars
// cancel it: commits on 77% of days still scored a calm 36.
func Score(metrics []Metric) int {
	total, top, n := 0, 0, 0
	for _, m := range metrics {
		if m.Percent == nil {
			continue
		}
		total += *m.Percent
		top = max(top, *m.Percent)
		n++
	}
	if n == 0 {
		return 0
	}
	return (top*n + total + n) / (2 * n)
}

// Severity bands sit where real accounts land under the score above: most
// between 15 and 70. Bands that never fire are worse than no bands.
func Severity(score int) string {
	switch {
	case score >= 60:
		return "critical"
	case score >= 40:
		return "serious"
	case score >= 20:
		return "survivable"
	default:
		return "suspiciously tidy"
	}
}

func pick(rng *rand.Rand, from []string) string { return from[rng.Intn(len(from))] }

// Written to roast the work, never the person.
var verdicts = []string{
	"Your repos have more forks than documentation. A stranger maintains your side project better than you do.",
	"Same utility function, four repos, four different names. Consistency is clearly a later milestone.",
	"Your commit history reads like a ransom note. Half the messages say fix. The other half are a full stop.",
	"You open pull requests like browser tabs. You close them about as often.",
	"Every project starts with a README and a plan. Both are gone by the third commit.",
	"Your test suite is aspirational. It describes a codebase that would be lovely to have.",
	"You refactor in production and call it observability. Bold. We respect it. Still bold.",
	"There's a branch called temp-fix-final-2 from two years ago. It's still ahead of main.",
}
