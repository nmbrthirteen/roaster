package roast

import (
	"context"
	"fmt"
	"sort"
)

// GitHub is the pack every stand falls back to.
const GitHub = "github"

// Pack is everything the stand shows or prints that names a social. Adding a
// social is a Pack here and a Provider behind the Router.
type Pack struct {
	Key         string `json:"key"`
	Name        string `json:"name"`     // in the hidden menu
	Headline    string `json:"headline"` // replaces the stock event headline
	Placeholder string `json:"placeholder"`
	HandleChars string `json:"handleChars"` // the body of a regexp character class
	HandleMax   int    `json:"handleMax"`

	Feed string `json:"feed"` // the command typed before the feed plays
	// Quiet plays for an account with nothing public, as (kind, text) pairs.
	// It must read as a joke, never as the audit failing.
	Quiet [][2]string `json:"quiet"`

	Calendar  string    `json:"-"` // section label over the heatmap
	Spotless  string    `json:"-"` // under a calendar with nothing on it
	Exhibit   string    `json:"-"` // over the quoted item
	NoExhibit [2]string `json:"-"` // in its place when there is none
}

var packs = map[string]Pack{
	GitHub: {
		Key:         GitHub,
		Name:        "GitHub",
		Headline:    "Roast your GitHub",
		Placeholder: "torvalds",
		HandleChars: "A-Za-z0-9-",
		HandleMax:   39,
		Feed:        "git log --graph --oneline",
		Quiet: [][2]string{
			{"cmd", "git log --graph --oneline"},
			{"out", "(empty. not one commit to judge.)"},
			{"cmd", "git status"},
			{"out", "nothing to commit, working tree clean"},
			{"cmd", "git blame"},
			{"out", "nobody to blame. a spotless record."},
			{"cmd", "git log --all --since=forever"},
			{"out", "still nothing. a true minimalist."},
		},
		Calendar: "Contribution calendar",
		Spotless: "A spotless calendar. Nothing here can be used against you.",
		Exhibit:  "Your worst commit, verbatim.",
		NoExhibit: [2]string{
			"We went looking for your worst commit.",
			"With no public commits, your record is technically flawless.",
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
