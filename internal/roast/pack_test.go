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
