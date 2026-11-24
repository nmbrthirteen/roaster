package roast

import (
	"context"
	"crypto/rand"
)

// Request is what a stand sends. It crosses the network to the roast service,
// so its field names are part of that contract.
type Request struct {
	Handle        string `json:"handle"`
	Pack          string `json:"pack,omitempty"`
	Event         string `json:"event,omitempty"`
	Terminal      string `json:"terminal,omitempty"` // the number printed on the receipt
	ModelProvider string `json:"modelProvider,omitempty"`

	// Offset is the stand's UTC offset in seconds. GitHub gives commit times
	// in UTC only, and the people at a stand nearly always live on its clock,
	// so this is the best guess at what "after midnight" means for them.
	Offset int `json:"offset,omitempty"`
}

// Phase names the stages the kiosk shows while it waits.
const (
	PhaseFetch   = "fetch"   // reading the account
	PhaseSection = "section" // one step of reading the account, in order
	PhaseFeed    = "feed"    // the items read, played while the verdict is written
	PhaseMetric  = "metric"  // one computed measurement, revealed as it lands
	PhaseVerdict = "verdict" // the written roast, streamed
	PhaseDone    = "done"    // the finished roast, ready to print
	PhaseError   = "error"
)

type Update struct {
	Phase   string   `json:"phase"`
	Label   string   `json:"label,omitempty"`
	Metric  *Metric  `json:"metric,omitempty"`
	Feed    []Item   `json:"feed,omitempty"`
	Section *Section `json:"section,omitempty"`
	Verdict string   `json:"verdict,omitempty"`
	Roast   *Roast   `json:"roast,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// Provider turns a handle into a roast.
type Provider interface {
	Roast(ctx context.Context, req Request, emit func(Update)) (Roast, error)
}

func Code() string {
	const alphabet = "23456789abcdefghjkmnpqrstuvwxyz"
	b := make([]byte, 5)
	rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// Candidate is one account a search found: enough for a visitor to tell
// whether it is them.
type Candidate struct {
	Ref      string `json:"ref"` // what the audit is then asked for
	Name     string `json:"name"`
	Headline string `json:"headline,omitempty"`
	Location string `json:"location,omitempty"`
	Photo    string `json:"photo,omitempty"` // an image URL
}

// Search is a visitor's name, sent to a pack that looks people up instead of
// taking a handle. It crosses the network like Request.
type Search struct {
	Query    string `json:"query"`
	Pack     string `json:"pack,omitempty"`
	Terminal string `json:"terminal,omitempty"`
}

// Searcher is a provider for a pack whose visitors name themselves rather than
// type a handle.
type Searcher interface {
	Search(ctx context.Context, s Search) ([]Candidate, error)
}
