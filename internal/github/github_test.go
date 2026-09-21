package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

// One account, shaped the way GitHub sends it: a repository whose history
// holds somebody else's commit, and one whose history is attributed to nobody.
const canned = `{"data":{"rateLimit":{"cost":7,"remaining":4993},"user":{
  "login":"nmbrthirteen",
  "name":"Nika",
  "createdAt":"2016-04-02T10:00:00Z",
  "followers":{"totalCount":41},
  "following":{"totalCount":9},
  "gists":{"totalCount":3},
  "starredRepositories":{"totalCount":812},
  "forks":{"totalCount":17},
  "profile":{"object":{"text":"# Hi, I'm Nika\n![Rust](https://img.shields.io/badge/Rust-000?logo=rust)"}},
  "contributionsCollection":{
    "totalCommitContributions":1204,
    "totalPullRequestContributions":88,
    "totalIssueContributions":31,
    "totalPullRequestReviewContributions":62,
    "restrictedContributionsCount":530,
    "contributionCalendar":{"weeks":[
      {"contributionDays":[{"date":"2026-01-01","contributionCount":4},{"date":"2026-01-02","contributionCount":0}]},
      {"contributionDays":[{"date":"2026-01-03","contributionCount":0}]}
    ]}
  },
  "repositories":{"totalCount":48,"nodes":[
    {"name":"roaster","description":"A conference kiosk","stargazerCount":12,"forkCount":2,
     "isArchived":false,"createdAt":"2026-09-18T10:00:00Z","pushedAt":"2026-09-20T02:00:00Z",
     "primaryLanguage":{"name":"Go"},"licenseInfo":{"key":"mit"},"issues":{"totalCount":3},
     "defaultBranchRef":{"target":{"history":{"nodes":[
       {"messageHeadline":"fix","committedDate":"2026-09-20T02:13:00+04:00","author":{"user":{"login":"nmbrthirteen"}}},
       {"messageHeadline":"Stop the printed receipt reading as random","committedDate":"2026-09-19T14:02:00+04:00","author":{"user":{"login":"NMBRTHIRTEEN"}}},
       {"messageHeadline":"tidy up the launcher","committedDate":"2026-09-18T11:00:00+04:00","author":{"user":{"login":"someone-else"}}}
     ]}}}},
    {"name":"unattributed","description":null,"stargazerCount":0,"forkCount":0,
     "isArchived":false,"createdAt":"2024-01-01T10:00:00Z","pushedAt":"2024-06-01T10:00:00Z",
     "primaryLanguage":null,"licenseInfo":null,"issues":{"totalCount":0},
     "defaultBranchRef":{"target":{"history":{"nodes":[
       {"messageHeadline":"asdf","committedDate":"2024-06-01T03:30:00+04:00","author":{"user":null}}
     ]}}}}
  ]}
}}}`

func serve(t *testing.T, body string) Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("the token should travel as a bearer header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return Client{Token: "test-token", URL: srv.URL, HTTP: srv.Client()}
}

func TestReadsAnAccount(t *testing.T) {
	f, err := serve(t, canned).Read(context.Background(), "nmbrthirteen")
	if err != nil {
		t.Fatal(err)
	}

	if f.Handle != "nmbrthirteen" || f.Followers != 41 || f.Starred != 812 {
		t.Errorf("the profile did not survive the trip: %+v", f)
	}
	if f.Cost != 7 || f.Remaining != 4993 {
		t.Errorf("what the read cost should come back with it, got cost %d remaining %d", f.Cost, f.Remaining)
	}
	if f.Owned != 48 || f.Forked != 17 {
		t.Errorf("owned %d and forked %d, want 48 and 17", f.Owned, f.Forked)
	}
	if f.Year.Commits != 1204 || f.Year.Private != 530 {
		t.Errorf("the contribution year is wrong: %+v", f.Year)
	}
	if len(f.Year.Days) != 3 {
		t.Errorf("the calendar should flatten to one entry a day, got %d", len(f.Year.Days))
	}
	if len(f.Repos) != 2 {
		t.Fatalf("want two repositories, got %d", len(f.Repos))
	}
	if f.Repos[0].Language != "Go" || f.Repos[0].License != "mit" {
		t.Errorf("language and licence should come through: %+v", f.Repos[0])
	}
	if f.Repos[1].Language != "" {
		t.Errorf("a repository with no language should read as empty, not crash")
	}
}

func TestTheProfileReadmeComesInTheSameRequest(t *testing.T) {
	f, err := serve(t, canned).Read(context.Background(), "nmbrthirteen")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.Readme, "img.shields.io/badge/Rust") {
		t.Errorf("the profile README should come through, got %q", f.Readme)
	}
}

func TestAnAccountWithNoProfileReadmeReadsAsEmpty(t *testing.T) {
	body := strings.Replace(canned, `"profile":{"object":{"text":"# Hi, I'm Nika\n![Rust](https://img.shields.io/badge/Rust-000?logo=rust)"}},`, `"profile":null,`, 1)
	f, err := serve(t, body).Read(context.Background(), "nmbrthirteen")
	if err != nil {
		t.Fatal(err)
	}
	if f.Readme != "" {
		t.Errorf("no profile repository should mean no README, got %q", f.Readme)
	}
}

func TestALongReadmeIsCutOnACharacterBoundary(t *testing.T) {
	long := strings.Repeat("é", readmeMax) // two bytes a character
	got := capped(long, readmeMax+1)
	if !utf8.ValidString(got) {
		t.Errorf("the cut split a character in half")
	}
	if len(got) > readmeMax+1 {
		t.Errorf("the cut overran the cap: %d bytes", len(got))
	}
}

// Somebody else's commit message is theirs to answer for.
func TestOtherPeoplesCommitsAreLeftOut(t *testing.T) {
	f, err := serve(t, canned).Read(context.Background(), "nmbrthirteen")
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range f.Commits {
		if c.Message == "tidy up the launcher" {
			t.Errorf("a commit by someone else was counted against this account")
		}
	}
	if len(f.Repos[0].Commits) != 2 {
		t.Errorf("want the two commits that are theirs, got %d", len(f.Repos[0].Commits))
	}
}

// An account whose email GitHub cannot match to a login has nothing attributed
// at all. Dropping the lot would read as somebody with no commits, which is a
// worse answer than assuming the owner of the repository wrote them.
func TestAnUnattributedRepoKeepsItsCommits(t *testing.T) {
	f, err := serve(t, canned).Read(context.Background(), "nmbrthirteen")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Repos[1].Commits) != 1 {
		t.Fatalf("want the one commit, got %d", len(f.Repos[1].Commits))
	}
	if f.Repos[1].Commits[0].Message != "asdf" {
		t.Errorf("got %q", f.Repos[1].Commits[0].Message)
	}
}

// The offset a commit was made in is the whole of the after-midnight metric.
func TestCommitTimesKeepTheirOwnOffset(t *testing.T) {
	f, err := serve(t, canned).Read(context.Background(), "nmbrthirteen")
	if err != nil {
		t.Fatal(err)
	}
	first := f.Commits[0]
	if got := first.At.Hour(); got != 2 {
		t.Errorf("committed at 02:13 in their own timezone, read back as hour %d", got)
	}
	if _, offset := first.At.Zone(); offset != 4*60*60 {
		t.Errorf("the offset should survive parsing, got %d seconds", offset)
	}
}

func TestAHandleNobodyHasIsSaidPlainly(t *testing.T) {
	c := serve(t, `{"data":{"user":null},"errors":[{"message":"Could not resolve to a User"}]}`)
	_, err := c.Read(context.Background(), "definitelynotarealaccount")
	if !errors.Is(err, ErrNoAccount) {
		t.Errorf("want a no-account error, got %v", err)
	}
}

// A typo should cost nothing. GitHub's own rule is cheap to check here.
func TestAnImpossibleHandleNeverLeavesTheDevice(t *testing.T) {
	asked := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = true
	}))
	defer srv.Close()

	c := Client{Token: "test-token", URL: srv.URL, HTTP: srv.Client()}
	for _, handle := range []string{"", "not a handle", "-leading", "trailing-", "a--b", strings.Repeat("a", 40)} {
		if _, err := c.Read(context.Background(), handle); !errors.Is(err, ErrBadHandle) {
			t.Errorf("%q should have been turned away as a bad handle, got %v", handle, err)
		}
	}
	if asked {
		t.Errorf("a handle that cannot exist should never reach GitHub")
	}
}

func tree(entries ...[2]string) repo {
	var r repo
	r.Root = &struct {
		Entries []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"entries"`
	}{}
	for _, e := range entries {
		r.Root.Entries = append(r.Root.Entries, struct {
			Name string `json:"name"`
			Type string `json:"type"`
		}{Name: e[0], Type: e[1]})
	}
	return r
}

// A repository is only said to have no README when that is certain.
func TestAReadmeIsFoundHoweverItIsSpelled(t *testing.T) {
	for name, r := range map[string]repo{
		"README.md":     tree([2]string{"README.md", "blob"}),
		"readme.rst":    tree([2]string{"main.go", "blob"}, [2]string{"readme.rst", "blob"}),
		"README bare":   tree([2]string{"README", "blob"}),
		"in .github":    tree([2]string{".github", "tree"}),
		"maybe in docs": tree([2]string{"Docs", "tree"}),
	} {
		if !hasReadme(r) {
			t.Errorf("%s: should count as having a README", name)
		}
	}
}

func TestNoReadmeIsOnlySaidWhenCertain(t *testing.T) {
	for name, r := range map[string]repo{
		"just code":               tree([2]string{"main.go", "blob"}, [2]string{"go.mod", "blob"}),
		"a folder called readme":  tree([2]string{"readme", "tree"}),
		"a file that only starts": tree([2]string{"README-old.txt.bak", "blob"}),
		"an empty repository":     {},
	} {
		if hasReadme(r) {
			t.Errorf("%s: should count as having no README", name)
		}
	}
}
