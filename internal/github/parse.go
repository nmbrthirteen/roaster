package github

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// The wire shapes. They exist only to be turned into Facts, which is what the
// rest of the program sees, so a change at GitHub lands in this file alone.
type response struct {
	Data struct {
		RateLimit *struct {
			Cost      int `json:"cost"`
			Remaining int `json:"remaining"`
		} `json:"rateLimit"`
		User *user `json:"user"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type count struct {
	TotalCount int `json:"totalCount"`
}

type user struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	Bio       string `json:"bio"`
	Company   string `json:"company"`
	CreatedAt string `json:"createdAt"`
	Followers count  `json:"followers"`
	Following count  `json:"following"`
	Gists     count  `json:"gists"`
	Starred   count  `json:"starredRepositories"`
	Forks     count  `json:"forks"`

	Profile *struct {
		Object *struct {
			Text *string `json:"text"`
		} `json:"object"`
	} `json:"profile"`

	Contributions struct {
		Commits      int `json:"totalCommitContributions"`
		PullRequests int `json:"totalPullRequestContributions"`
		Issues       int `json:"totalIssueContributions"`
		Reviews      int `json:"totalPullRequestReviewContributions"`
		Private      int `json:"restrictedContributionsCount"`
		Calendar     struct {
			Weeks []struct {
				Days []struct {
					Date  string `json:"date"`
					Count int    `json:"contributionCount"`
				} `json:"contributionDays"`
			} `json:"weeks"`
		} `json:"contributionCalendar"`
	} `json:"contributionsCollection"`

	Repositories struct {
		TotalCount int    `json:"totalCount"`
		Nodes      []repo `json:"nodes"`
	} `json:"repositories"`
}

type repo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Stars       int    `json:"stargazerCount"`
	Forks       int    `json:"forkCount"`
	Archived    bool   `json:"isArchived"`
	CreatedAt   string `json:"createdAt"`
	PushedAt    string `json:"pushedAt"`

	PrimaryLanguage *struct {
		Name string `json:"name"`
	} `json:"primaryLanguage"`
	License *struct {
		Key string `json:"key"`
	} `json:"licenseInfo"`
	Issues count `json:"issues"`

	Root *struct {
		Entries []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"entries"`
	} `json:"root"`

	DefaultBranchRef *struct {
		Target struct {
			History struct {
				Nodes []commit `json:"nodes"`
			} `json:"history"`
		} `json:"target"`
	} `json:"defaultBranchRef"`
}

type commit struct {
	Hash    string `json:"abbreviatedOid"`
	Message string `json:"messageHeadline"`
	At      string `json:"authoredDate"`
	Author  struct {
		User *struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"author"`
}

func (u *user) facts() Facts {
	f := Facts{
		Handle:    u.Login,
		Name:      u.Name,
		Bio:       u.Bio,
		Company:   u.Company,
		Created:   when(u.CreatedAt),
		Followers: u.Followers.TotalCount,
		Following: u.Following.TotalCount,
		Gists:     u.Gists.TotalCount,
		Starred:   u.Starred.TotalCount,
		Owned:     u.Repositories.TotalCount,
		Forked:    u.Forks.TotalCount,
		Year: Year{
			Commits:      u.Contributions.Commits,
			PullRequests: u.Contributions.PullRequests,
			Issues:       u.Contributions.Issues,
			Reviews:      u.Contributions.Reviews,
			Private:      u.Contributions.Private,
		},
		Read: time.Now(),
	}

	if u.Profile != nil && u.Profile.Object != nil && u.Profile.Object.Text != nil {
		f.Readme = capped(*u.Profile.Object.Text, readmeMax)
	}

	for _, week := range u.Contributions.Calendar.Weeks {
		for _, d := range week.Days {
			f.Year.Days = append(f.Year.Days, Day{Date: when(d.Date), Count: d.Count})
		}
	}

	// Commits by other people are theirs to answer for. An account whose email
	// GitHub cannot match to a login has no commits attributed at all, so a
	// repository that comes back with nothing of its own owner's keeps the lot:
	// they own it, and somebody wrote those messages.
	for _, r := range u.Repositories.Nodes {
		out := Repo{
			Name:        r.Name,
			Description: r.Description,
			Stars:       r.Stars,
			Forks:       r.Forks,
			Archived:    r.Archived,
			Created:     when(r.CreatedAt),
			Pushed:      when(r.PushedAt),
			OpenIssues:  r.Issues.TotalCount,
		}
		if r.PrimaryLanguage != nil {
			out.Language = r.PrimaryLanguage.Name
		}
		if r.License != nil {
			out.License = r.License.Key
		}
		out.Readme = hasReadme(r)

		var all, mine []Commit
		if r.DefaultBranchRef != nil {
			for _, c := range r.DefaultBranchRef.Target.History.Nodes {
				one := Commit{Hash: c.Hash, Repo: r.Name, Message: strings.TrimSpace(c.Message), At: when(c.At)}
				all = append(all, one)
				if c.Author.User != nil && strings.EqualFold(c.Author.User.Login, u.Login) {
					mine = append(mine, one)
				}
			}
		}
		out.Commits = mine
		if len(mine) == 0 {
			out.Commits = all
		}

		f.Repos = append(f.Repos, out)
		f.Commits = append(f.Commits, out.Commits...)
	}

	return f
}

var readmeName = regexp.MustCompile(`(?i)^readme(\.[a-z0-9]+)?$`)

// hasReadme gives the benefit of the doubt. GitHub shows a README from the top
// of a repository, from .github or from docs; the last two would need another
// look to be sure of, so either folder counts as having one.
func hasReadme(r repo) bool {
	if r.Root == nil {
		return false
	}
	for _, e := range r.Root.Entries {
		if e.Type == "blob" && readmeName.MatchString(e.Name) {
			return true
		}
		if e.Type == "tree" && (e.Name == ".github" || strings.EqualFold(e.Name, "docs")) {
			return true
		}
	}
	return false
}

// A profile README past this is badges and a GIF. The claims worth reading are
// near the top.
const readmeMax = 16 << 10

// capped cuts at a rune boundary, so a README in any script survives the cut.
func capped(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// when parses either shape GitHub sends. DateTime is normalised to UTC;
// GitTimestamp, which is what commits carry, keeps the offset it was made in,
// and that offset is the whole of the after-midnight metric.
func when(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
