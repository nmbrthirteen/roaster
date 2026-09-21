// Package github reads the public record of one account.
//
// Everything here is one request. A stand has a queue in front of it, and the
// difference between one round trip and five is the difference between a visitor
// watching the screen and a visitor watching the floor. REST would need a call
// for the profile, one for the repositories, one per repository for commits, and
// one for the contribution calendar. GraphQL answers all of it at once.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	endpoint = "https://api.github.com/graphql"

	// How much of an account to read. Thirty repositories by most recently
	// pushed covers anyone's working year, and twenty commit headlines each is
	// enough to see a habit without pulling a whole history.
	repoCount   = 30
	commitCount = 20

	// A stand cannot wait longer than this and still feel like a stand.
	timeout = 10 * time.Second
)

var (
	// ErrNoAccount is a handle GitHub has never heard of, which is worth saying
	// in those words rather than as a failure.
	ErrNoAccount = errors.New("no such account")

	// ErrBadHandle is text that could never be a handle. Nothing was asked of
	// GitHub, and the message is written to be shown to whoever typed it.
	ErrBadHandle = errors.New("can't be a GitHub username. Those use only letters, numbers and single hyphens")
)

// handleRule is GitHub's own: letters, digits and single hyphens, never at
// either end. Checking it here means a typo costs nothing instead of a round
// trip. The length is checked beside it, because RE2 has no lookahead and the
// two rules do not fold into one readable pattern.
var handleRule = regexp.MustCompile(`^[A-Za-z0-9](?:-?[A-Za-z0-9])*$`)

const handleMax = 39

type Client struct {
	Token string       // a token with no scopes; everything read is public
	HTTP  *http.Client // nil means one with the timeout above
	URL   string       // nil means GitHub; set by tests
}

// Facts is the account as GitHub has it. Nothing here is judged or scored yet.
type Facts struct {
	Handle    string
	Name      string
	Bio       string
	Company   string
	Created   time.Time
	Followers int
	Following int
	Gists     int
	Starred   int // repositories this account has starred

	Year   Year
	Repos  []Repo
	Owned  int // repositories that are not forks
	Forked int

	// Commits across every repository read, newest first, and only this
	// account's own.
	Commits []Commit

	// Readme is the profile README, the one in the repository named after the
	// account. It is how someone describes themselves as a developer, which
	// makes it the best place to find a claim the rest of the account does not
	// back up. Capped, since a few pages is all anyone reads.
	Readme string

	Read time.Time

	// Cost and Remaining are not about the account: they are what reading it
	// cost against GitHub's hourly budget and what is left, reported in the
	// same response for free. At any volume this is the number to watch.
	Cost      int
	Remaining int
}

// Year is the contribution calendar and its totals, which is the only place
// private work shows up at all, and then only as a number.
type Year struct {
	Commits      int
	PullRequests int
	Issues       int
	Reviews      int
	Private      int
	Days         []Day
}

type Day struct {
	Date  time.Time
	Count int
}

type Repo struct {
	Name        string
	Description string
	Stars       int
	Forks       int
	Language    string
	License     string
	Archived    bool
	Created     time.Time
	Pushed      time.Time
	OpenIssues  int
	Commits     []Commit

	// Readme is false only when there is certainly none: no README of any
	// spelling at the top, and no .github or docs folder GitHub would also look
	// in. A repository is never accused of lacking one on a guess.
	Readme bool
}

type Commit struct {
	Hash    string // abbreviated, as git log prints it
	Repo    string
	Message string

	// At keeps the committer's own offset rather than UTC, which is what makes
	// "half your commits happen after midnight" a fact about them instead of a
	// fact about a timezone.
	At time.Time
}

// GitHub gives one GraphQL request about ten seconds, then answers 502. A
// busy account read in a single query runs past that, so the read is four
// queries sent at once, and the stand waits for the slowest one rather than
// the sum.
const (
	accountQuery = `
query($login: String!) {
  rateLimit { cost remaining }
  user(login: $login) {
    login
    name
    bio
    company
    createdAt
    followers { totalCount }
    following { totalCount }
    gists { totalCount }
    starredRepositories { totalCount }
    forks: repositories(ownerAffiliations: OWNER, isFork: true) { totalCount }
    profile: repository(name: $login) {
      object(expression: "HEAD:README.md") { ... on Blob { text } }
    }
  }
}`

	calendarQuery = `
query($login: String!) {
  rateLimit { cost remaining }
  user(login: $login) {
    contributionsCollection {
      totalCommitContributions
      totalPullRequestContributions
      totalIssueContributions
      totalPullRequestReviewContributions
      restrictedContributionsCount
      contributionCalendar {
        weeks { contributionDays { date contributionCount } }
      }
    }
  }
}`

	reposQuery = `
query($login: String!, $repos: Int!) {
  rateLimit { cost remaining }
  user(login: $login) {
    repositories(first: $repos, ownerAffiliations: OWNER, isFork: false,
                 orderBy: {field: PUSHED_AT, direction: DESC}) {
      totalCount
      nodes {
        name
        description
        stargazerCount
        forkCount
        isArchived
        createdAt
        pushedAt
        primaryLanguage { name }
        licenseInfo { key }
        issues(states: OPEN) { totalCount }
        root: object(expression: "HEAD:") { ... on Tree { entries { name type } } }
      }
    }
  }
}`

	historyQuery = `
query($login: String!, $repos: Int!, $commits: Int!) {
  rateLimit { cost remaining }
  user(login: $login) {
    repositories(first: $repos, ownerAffiliations: OWNER, isFork: false,
                 orderBy: {field: PUSHED_AT, direction: DESC}) {
      nodes {
        name
        defaultBranchRef {
          target {
            ... on Commit {
              history(first: $commits) {
                nodes {
                  abbreviatedOid
                  messageHeadline
                  committedDate
                  author { user { login } }
                }
              }
            }
          }
        }
      }
    }
  }
}`
)

// Valid reports whether text could be a GitHub handle at all.
func Valid(handle string) bool {
	return len(handle) <= handleMax && handleRule.MatchString(handle)
}

// Read fetches the account. The error is worth showing to a visitor as it
// stands: they are the one who typed the handle.
func (c Client) Read(ctx context.Context, handle string) (Facts, error) {
	handle = strings.TrimSpace(handle)
	if !Valid(handle) {
		return Facts{}, fmt.Errorf("%q %w", handle, ErrBadHandle)
	}
	if c.Token == "" {
		return Facts{}, fmt.Errorf("no GitHub token; the GraphQL API refuses anonymous calls")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	login := map[string]any{"login": handle}
	asks := []struct {
		query string
		vars  map[string]any
	}{
		{accountQuery, login},
		{calendarQuery, login},
		{reposQuery, map[string]any{"login": handle, "repos": repoCount}},
		{historyQuery, map[string]any{"login": handle, "repos": repoCount, "commits": commitCount}},
	}
	outs := make([]response, len(asks))
	errs := make([]error, len(asks))
	var wg sync.WaitGroup
	for i, a := range asks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if outs[i], errs[i] = c.ask(ctx, a.query, a.vars); errs[i] != nil {
				cancel() // one part failing sinks the read, so stop the rest
			}
		}()
	}
	wg.Wait()
	// The first failure is the cause; later ones are usually the cancel.
	for i := range asks {
		if errs[i] != nil && !errors.Is(errs[i], context.Canceled) {
			return Facts{}, errs[i]
		}
	}
	for _, err := range errs {
		if err != nil {
			return Facts{}, err
		}
	}

	// A missing account comes back as a 200 with the user null and a note in
	// errors, so the status code alone never tells you.
	for _, out := range outs {
		if out.Data.User == nil {
			if len(out.Errors) > 0 {
				return Facts{}, fmt.Errorf("%w: @%s (%s)", ErrNoAccount, handle, out.Errors[0].Message)
			}
			return Facts{}, fmt.Errorf("%w: @%s", ErrNoAccount, handle)
		}
	}

	u := outs[0].Data.User
	u.Contributions = outs[1].Data.User.Contributions
	u.Repositories = outs[2].Data.User.Repositories
	history := map[string]*repo{}
	for i := range outs[3].Data.User.Repositories.Nodes {
		r := &outs[3].Data.User.Repositories.Nodes[i]
		history[r.Name] = r
	}
	for i := range u.Repositories.Nodes {
		if h, ok := history[u.Repositories.Nodes[i].Name]; ok {
			u.Repositories.Nodes[i].DefaultBranchRef = h.DefaultBranchRef
		}
	}

	f := u.facts()
	for i, out := range outs {
		if rl := out.Data.RateLimit; rl != nil {
			f.Cost += rl.Cost
			if i == 0 || rl.Remaining < f.Remaining {
				f.Remaining = rl.Remaining
			}
		}
	}
	return f, nil
}

// ask sends one query and decodes the answer.
func (c Client) ask(ctx context.Context, query string, vars map[string]any) (response, error) {
	body, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return response{}, err
	}

	url := c.URL
	if url == "" {
		url = endpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	res, err := client.Do(req)
	if err != nil {
		return response{}, fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return response{}, fmt.Errorf("GitHub rejected the token")
	default:
		return response{}, fmt.Errorf("GitHub returned %s", res.Status)
	}

	var out response
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return response{}, fmt.Errorf("GitHub sent something unreadable: %w", err)
	}
	return out, nil
}
