package metric

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/upgaming/roaster/internal/github"
	"github.com/upgaming/roaster/internal/roast"
)

// Story is the account read the way a developer would read it: real commands
// and what those commands print, with the judgement kept to # comments. Every
// value comes from the account, so anyone watching can check it.
func Story(f github.Facts, now time.Time) []roast.Section {
	return []roast.Section{overview(f), profileReadme(f), repos(f, now), contributions(f)}
}

func overview(f github.Facts) roast.Section {
	created := "null"
	if !f.Created.IsZero() {
		created = `"` + f.Created.UTC().Format(time.RFC3339) + `"`
	}
	return roast.Section{
		Cmd: "gh api users/" + f.Handle + " --jq '{login, created_at, public_repos, followers, following}'",
		Lines: []string{
			"{",
			fmt.Sprintf(`  "login": %q,`, f.Handle),
			fmt.Sprintf(`  "created_at": %s,`, created),
			fmt.Sprintf(`  "public_repos": %d,`, f.Owned),
			fmt.Sprintf(`  "followers": %d,`, f.Followers),
			fmt.Sprintf(`  "following": %d`, f.Following),
			"}",
		},
	}
}

// readmeLines is how much of a profile README the screen shows: the top is
// where people say who they are.
const readmeLines = 5

func profileReadme(f github.Facts) roast.Section {
	s := roast.Section{Cmd: "gh api repos/" + f.Handle + "/" + f.Handle + "/readme -H 'Accept: application/vnd.github.raw' | head -" + fmt.Sprint(readmeLines)}
	if strings.TrimSpace(f.Readme) == "" {
		s.Lines = []string{"gh: Not Found (HTTP 404)", "# no profile README. the strong, silent type."}
		return s
	}
	for _, l := range strings.Split(f.Readme, "\n") {
		if len(s.Lines) == readmeLines {
			break
		}
		if l = shown(l, 64); l != "" {
			s.Lines = append(s.Lines, l)
		}
	}
	if n := Badges(f.Readme); n > 0 {
		s.Lines = append(s.Lines, fmt.Sprintf("# %s", count(n, "skill badge")))
	}
	claims := Claimed(f.Readme)
	if len(claims) > 0 {
		s.Lines = append(s.Lines, "# claims: "+strings.Join(claims, ", "))
	}
	if unused := Unused(claims, f.Repos); len(unused) > 0 {
		s.Lines = append(s.Lines, "# not one repo written in "+strings.Join(unused, " or ")+". bold claim.")
	}
	return s
}

const repoLines = 5

func repos(f github.Facts, now time.Time) roast.Section {
	s := roast.Section{Cmd: fmt.Sprintf("gh repo list %s --limit %d", f.Handle, repoLines)}
	if len(f.Repos) == 0 {
		s.Lines = []string{fmt.Sprintf("no repositories match your search in @%s", f.Handle), "# an empty stage. we roast what we can."}
		return s
	}
	shownN := min(repoLines, len(f.Repos))
	s.Lines = []string{fmt.Sprintf("Showing %d of %d repositories in @%s", shownN, f.Owned, f.Handle), fmt.Sprintf("%-18s %-11s %s", "NAME", "LANGUAGE", "UPDATED")}
	noDescription, noReadme := 0, 0
	for i, r := range f.Repos {
		if strings.TrimSpace(r.Description) == "" {
			noDescription++
		}
		if !r.Readme {
			noReadme++
		}
		if i >= repoLines {
			continue
		}
		lang := r.Language
		if lang == "" {
			lang = "-"
		}
		updated := ago(r.Pushed, now)
		if r.Archived {
			updated += "  archived"
		}
		s.Lines = append(s.Lines, fmt.Sprintf("%-18s %-11s %s", shown(r.Name, 18), shown(lang, 11), updated))
	}
	if noDescription > 0 {
		s.Lines = append(s.Lines, fmt.Sprintf("# %d of %d have no description. guess the plot.", noDescription, len(f.Repos)))
	}
	if noReadme > 0 {
		s.Lines = append(s.Lines, fmt.Sprintf("# %d of %d have no README. good luck, whoever clones these.", noReadme, len(f.Repos)))
	}
	return s
}

func contributions(f github.Facts) roast.Section {
	y := f.Year
	s := roast.Section{
		Cmd: "gh api graphql -F login=" + f.Handle + " -f query=@contributions.graphql --jq .data.user.contributionsCollection",
		Lines: []string{
			"{",
			fmt.Sprintf(`  "totalCommitContributions": %d,`, y.Commits),
			fmt.Sprintf(`  "totalPullRequestContributions": %d,`, y.PullRequests),
			fmt.Sprintf(`  "totalPullRequestReviewContributions": %d,`, y.Reviews),
			fmt.Sprintf(`  "restrictedContributionsCount": %d`, y.Private),
			"}",
		},
	}
	if gap := longestGap(f); gap.Value != "unknown" {
		s.Lines = append(s.Lines, "# longest quiet stretch: "+gap.Value+". we assume a sabbatical.")
	}
	return s
}

// ago is how gh prints an update time, rounded to the largest unit.
func ago(t, now time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := now.Sub(t)
	switch {
	case d < time.Hour:
		return "just now"
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/24/30))
	}
	return fmt.Sprintf("%dy ago", yearsSince(t, now))
}

// shown is one line of someone else's text made safe to put on screen, cut to
// fit a column.
func shown(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
	if r := []rune(s); len(r) > max {
		s = string(r[:max-1]) + "."
	}
	return s
}

func count(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func yearsSince(t, now time.Time) int {
	y := now.Year() - t.Year()
	if now.Month() < t.Month() || now.Month() == t.Month() && now.Day() < t.Day() {
		y--
	}
	if y < 0 {
		return 0
	}
	return y
}
