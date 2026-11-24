package roast

import (
	"context"
	"fmt"
	"sort"
)

// GitHub is the pack every stand falls back to.
const GitHub = "github"

const LinkedIn = "linkedin"

// Pack is everything the stand shows or prints that names a social. Adding a
// social is a Pack here and a Provider behind the Router.
type Pack struct {
	Key         string `json:"key"`
	Name        string `json:"name"`     // in the hidden menu
	Headline    string `json:"headline"` // replaces the stock event headline
	Lede        string `json:"lede"`     // under the headline: what to type
	Note        string `json:"note"`     // under the field: what they get
	Placeholder string `json:"placeholder"`
	HandleChars string `json:"handleChars"` // the body of a regexp character class
	HandleMax   int    `json:"handleMax"`

	// Lookup is how a visitor names their account: LookupHandle sends what
	// they type straight to the audit, LookupSearch shows the people it
	// matches and audits the one they pick.
	Lookup string `json:"lookup"`
	// Link is a regexp whose first group pulls the handle out of a pasted
	// profile link. A link skips the search.
	Link string `json:"link"`
	// At goes before the handle wherever it is shown.
	At string `json:"at"`

	// Audit is how the reading plays: AuditTerminal types it into a shell,
	// AuditFeed posts it as cards and notifications. In a feed, Quiet and
	// Waiting are (who, what) pairs rather than (kind, text).
	Audit string `json:"audit"`

	Fetching string `json:"fetching"` // the line under the first step
	// Steps are the progress bar, in order. Sections names the step each
	// story section the audit sends belongs to, in the order they arrive.
	// The feed is the "items" step, the gauges "gauges", the verdict "verdict".
	Steps    []Step   `json:"steps"`
	Sections []string `json:"sections"`

	Feed     string `json:"feed"`     // the command typed before the feed plays
	FeedKind string `json:"feedKind"` // FeedLog draws a git graph, FeedList one line an item
	// Quiet plays for an account with nothing public, as (kind, text) pairs,
	// with {handle} filled in. It must read as a joke, never as a failure.
	Quiet [][2]string `json:"quiet"`

	Waiting [][2]string `json:"waiting"`

	Calendar  string    `json:"-"` // section label over the heatmap
	Spotless  string    `json:"-"` // under a calendar with nothing on it
	Worst     string    `json:"-"` // the section the quoted item prints under
	Exhibit   string    `json:"-"` // over the quoted item
	When      string    `json:"-"` // time layout for the quoted item's date
	NoExhibit [2]string `json:"-"` // in its place when there is none
}

// Step is one segment of the audit screen's progress bar.
type Step struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

const (
	LookupHandle = "handle"
	LookupSearch = "search"

	FeedLog  = "log"
	FeedList = "list"

	AuditTerminal = "terminal"
	AuditFeed     = "feed"
)

var packs = map[string]Pack{
	GitHub: {
		Key:         GitHub,
		Name:        "GitHub",
		Headline:    "Brave enough?",
		Lede:        "Enter your GitHub username",
		Note:        "Get your coding insights and a roast receipt",
		Placeholder: "yourusername",
		HandleChars: "A-Za-z0-9-",
		HandleMax:   39,
		Lookup:      LookupHandle,
		Link:        `github\.com/([^/?#]+)`,
		At:          "@",
		Audit:       AuditTerminal,
		Fetching:    "querying api.github.com/graphql...",
		Steps: []Step{
			{"profile", "Reading the profile"},
			{"readme", "Reading the README"},
			{"repos", "Going through the repos"},
			{"year", "Counting contributions"},
			{"items", "Reading commits"},
			{"gauges", "Measuring the damage"},
			{"verdict", "Writing the verdict"},
		},
		Sections: []string{"profile", "readme", "repos", "year", "verdict"},
		Feed:     "git log --graph --oneline",
		FeedKind: FeedLog,
		Quiet: [][2]string{
			{"cmd", "git log --oneline"},
			{"out", "(no public commits. stealth mode, or witness protection.)"},
			{"cmd", "git shortlog -sn --all"},
			{"out", "     0  {handle}"},
			{"cmd", "git blame"},
			{"out", "nobody to blame, which is a first for this stand."},
			{"cmd", "git status"},
			{"out", "nothing to commit, working tree clean"},
			{"cmd", "echo \"we respect the mystery\""},
			{"out", "we respect the mystery"},
		},
		Waiting: [][2]string{
			{"./roast draft --count 5", "five drafts written. keeping the meanest one."},
			{"./roast sharpen --harder", "the first draft was too polite. rewriting."},
			{"sed -i 's/nice/honest/g' verdict.txt", "compliments removed."},
			{"./roast ask --senior-engineer", "they laughed. that counts as approval."},
			{"./roast check --read-aloud-safe", "clean enough for the queue behind you."},
			{"git blame verdict.txt", "blame assigned. you know where."},
			{"./roast fact-check verdict.txt", "every number is yours. we only added the pain."},
			{"sleep 1  # dramatic pause", "..."},
		},
		Calendar: "Contribution calendar",
		Spotless: "A spotless calendar. Nothing here can be used against you.",
		Worst:    "Your worst commit",
		Exhibit:  "Copied word for word:",
		When:     "Mon 2 Jan 2006, 15:04",
		NoExhibit: [2]string{
			"We went looking for your worst commit.",
			"With no public commits, your record is technically flawless.",
		},
	},
	LinkedIn: {
		Key:         LinkedIn,
		Name:        "LinkedIn",
		Headline:    "Open to roast?",
		Lede:        "Enter your first and last name",
		Note:        "Get your career insights and a roast receipt",
		Placeholder: "First Last",
		HandleChars: `\p{L}\p{M}' .-`,
		HandleMax:   80,
		Lookup:      LookupSearch,
		Link:        `linkedin\.com/in/([^/?#]+)`,
		Audit:       AuditFeed,
		Fetching:    "is opening your public profile.",
		Steps: []Step{
			{"profile", "Opening the profile"},
			{"about", "Reading the About"},
			{"experience", "Going through the jobs"},
			{"skills", "Counting the skills"},
			{"items", "Replaying the career"},
			{"gauges", "Running the analytics"},
			{"verdict", "Writing the verdict"},
		},
		Sections: []string{"profile", "about", "experience", "skills", "verdict"},
		Feed:     "Career, replayed",
		FeedKind: FeedList,
		Quiet: [][2]string{
			{"The roaster", "found no roles listed."},
			{"The roaster", "found no headline either."},
			{"Your network", "respects the mystery."},
		},
		Waiting: [][2]string{
			{"The roaster", "is drafting five versions of your verdict."},
			{"A senior recruiter", "viewed your profile and sighed."},
			{"The roaster", "deleted the word \"passionate\" from the first draft."},
			{"Your network", "is typing..."},
			{"Legal", "checked the verdict is safe to read aloud."},
			{"The roaster", "is keeping the meanest draft."},
		},
		Calendar: "Career calendar",
		Spotless: "Nothing on the calendar. Nothing here can be used against you.",
		Worst:    "Your worst line",
		Exhibit:  "Copied word for word:",
		When:     "Jan 2006",
		NoExhibit: [2]string{
			"We went looking for your worst line.",
			"With nothing written, your profile is technically flawless.",
		},
	},
}

// PackFor falls back to GitHub, so a roast stored before packs existed still
// prints.
func PackFor(key string) Pack {
	if p, ok := packs[key]; ok {
		return p
	}
	return packs[GitHub]
}

func Known(key string) bool {
	_, ok := packs[key]
	return ok
}

// All is the hidden menu's list: GitHub first, the rest by name.
func All() []Pack {
	out := make([]Pack, 0, len(packs))
	for _, p := range packs {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Key == GitHub) != (out[j].Key == GitHub) {
			return out[i].Key == GitHub
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Router lets one roast service serve every social. A pack with no provider
// is refused in words a visitor can read.
type Router map[string]Provider

func (r Router) Roast(ctx context.Context, req Request, emit func(Update)) (Roast, error) {
	key := req.Pack
	if key == "" {
		key = GitHub
	}
	p, ok := r[key]
	if !ok {
		name := key
		if known, ok := packs[key]; ok {
			name = known.Name
		}
		return Roast{}, fmt.Errorf("%s roasts are not available yet", name)
	}
	req.Pack = key
	return p.Roast(ctx, req, emit)
}

// Search sends a visitor's name to the pack's provider, when it can search.
func (r Router) Search(ctx context.Context, s Search) ([]Candidate, error) {
	key := s.Pack
	if key == "" {
		key = GitHub
	}
	p, ok := r[key]
	if !ok {
		return nil, fmt.Errorf("%s roasts are not available yet", PackFor(key).Name)
	}
	searcher, ok := p.(Searcher)
	if !ok {
		return nil, fmt.Errorf("%s accounts are found by handle, not by name", PackFor(key).Name)
	}
	s.Pack = key
	return searcher.Search(ctx, s)
}
