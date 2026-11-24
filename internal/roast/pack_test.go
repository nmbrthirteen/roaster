package roast

import (
	"context"
	"strings"
	"testing"
)

type fixed struct{ pack *string }

func (f fixed) Roast(_ context.Context, req Request, _ func(Update)) (Roast, error) {
	*f.pack = req.Pack
	return Roast{Code: "x", Pack: req.Pack}, nil
}

func TestTheRouterSendsEachPackToItsOwnProvider(t *testing.T) {
	var got string
	r := Router{GitHub: fixed{&got}}

	if _, err := r.Roast(context.Background(), Request{Handle: "octocat"}, func(Update) {}); err != nil {
		t.Fatal(err)
	}
	if got != GitHub {
		t.Errorf("a request with no pack is a GitHub one, got %q", got)
	}
}

func TestAPackWithNoProviderIsRefusedInPlainWords(t *testing.T) {
	_, err := Router{}.Roast(context.Background(), Request{Handle: "someone", Pack: "gitlab"}, func(Update) {})

	if err == nil || !strings.Contains(err.Error(), "gitlab roasts are not available yet") {
		t.Errorf("want a refusal a visitor can read, got %v", err)
	}
}

func TestTheMenuListsGitHubFirst(t *testing.T) {
	if all := All(); len(all) == 0 || all[0].Key != GitHub {
		t.Errorf("want GitHub first, got %+v", all)
	}
	if Known("myspace") || !Known(GitHub) {
		t.Errorf("only packs in this build are known")
	}
	if PackFor("myspace").Key != GitHub {
		t.Errorf("an unknown pack should read as GitHub, so an old receipt still prints")
	}
}

func TestEveryPackCanFillTheScreenAndTheReceipt(t *testing.T) {
	for key, p := range packs {
		if p.Key != key || p.Name == "" || p.Headline == "" || p.HandleChars == "" || p.HandleMax == 0 {
			t.Errorf("%s: the entry screen needs a key, name, headline and handle rule", key)
		}
		if p.Feed == "" || len(p.Quiet) == 0 {
			t.Errorf("%s: the audit screen needs a feed command and a quiet bit", key)
		}
		if p.Calendar == "" || p.Spotless == "" || p.Exhibit == "" || p.NoExhibit[0] == "" || p.NoExhibit[1] == "" {
			t.Errorf("%s: the receipt needs its calendar and exhibit copy", key)
		}
	}
}

type searching struct{ fixed }

func (searching) Search(_ context.Context, s Search) ([]Candidate, error) {
	return []Candidate{{Ref: "found", Name: s.Query}}, nil
}

func TestASearchGoesToThePackThatCanSearch(t *testing.T) {
	var got string
	r := Router{GitHub: fixed{&got}, LinkedIn: searching{fixed{&got}}}

	found, err := r.Search(context.Background(), Search{Query: "Jordan Sample", Pack: LinkedIn})
	if err != nil || len(found) != 1 || found[0].Name != "Jordan Sample" {
		t.Fatalf("got %v %v", found, err)
	}
	if _, err := r.Search(context.Background(), Search{Query: "octocat"}); err == nil || !strings.Contains(err.Error(), "found by handle") {
		t.Errorf("GitHub takes a handle, so a search there should be refused in words, got %v", err)
	}
}

func TestEveryPackTellsTheKioskHowToLookPeopleUp(t *testing.T) {
	for key, p := range packs {
		if p.Lede == "" || p.Note == "" || p.Fetching == "" || p.Worst == "" || p.When == "" {
			t.Errorf("%s: the entry screen, the audit screen and the receipt need their copy", key)
		}
		if p.Lookup != LookupHandle && p.Lookup != LookupSearch {
			t.Errorf("%s: lookup %q is neither handle nor search", key, p.Lookup)
		}
		if p.Audit != AuditTerminal && p.Audit != AuditFeed {
			t.Errorf("%s: audit %q is neither terminal nor feed", key, p.Audit)
		}
		if p.FeedKind != FeedLog && p.FeedKind != FeedList {
			t.Errorf("%s: feed kind %q is neither log nor list", key, p.FeedKind)
		}
		steps := map[string]bool{}
		for _, s := range p.Steps {
			steps[s.Key] = true
		}
		for _, k := range append([]string{"items", "gauges", "verdict"}, p.Sections...) {
			if !steps[k] {
				t.Errorf("%s: the progress bar has no %q step", key, k)
			}
		}
	}
}
