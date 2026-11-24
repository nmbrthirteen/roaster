package linkedin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strings"
	"time"
	"unicode"

	"github.com/upgaming/roaster/internal/audit"
	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/verdict"
)

const (
	// A LinkedIn brief is longer than a GitHub one and the model takes 30
	// seconds or so on it. The screen plays its reading for about 43, so the
	// verdict has that long; roastd's budget still bounds the whole roast.
	verdictTimeout = 48 * time.Second

	// Enough faces to find yourself among, few enough to fit one screen.
	maxCandidates = 6

	// Memory keys carry the pack, so a LinkedIn profile never collides with a
	// GitHub account of the same name.
	profileKey = "linkedin:"
	searchKey  = "linkedin-search:"
)

var (
	ErrOptedOut = errors.New("this person has asked not to be roasted")
	ErrBadName  = errors.New("type a first and last name")
)

// Audit is the LinkedIn roast: find the profile, read it, measure it, and have
// the verdict written. It runs the same way as the GitHub audit, so a visitor
// who picked a real profile always leaves with a receipt.
type Audit struct {
	Source Source

	// Writer writes the verdict. Nil runs on the numbers alone.
	Writer verdict.Writer

	// Blocked holds profile references, lowercased, that must never be
	// roasted or shown in a search.
	Blocked map[string]bool

	// Memory keeps profiles and lines between visitors. Nil reads the vendor
	// every time.
	Memory *audit.Memory

	Timeout time.Duration
	Now     func() time.Time
}

// WithWriter is this audit with the verdict written by w, for a stand that
// picks its own model.
func (a Audit) WithWriter(w verdict.Writer) roast.Provider {
	a.Writer = w
	return a
}

func (a Audit) Search(ctx context.Context, s roast.Search) ([]roast.Candidate, error) {
	name := strings.Join(strings.Fields(s.Query), " ")
	if !plausibleName(name) {
		return nil, ErrBadName
	}
	read := func() ([]roast.Candidate, error) { return a.Source.Search(ctx, name) }
	var (
		found []roast.Candidate
		err   error
	)
	if a.Memory != nil {
		found, err = audit.Measure(a.Memory, searchKey+strings.ToLower(name), read)
	} else {
		found, err = read()
	}
	if err != nil {
		return nil, visible(err)
	}
	out := make([]roast.Candidate, 0, maxCandidates)
	for _, c := range found {
		c.Ref = Ref(c.Ref)
		if !Valid(c.Ref) || a.Blocked[strings.ToLower(c.Ref)] {
			continue
		}
		out = append(out, c)
		if len(out) == maxCandidates {
			break
		}
	}
	return out, nil
}

func (a Audit) Roast(ctx context.Context, req roast.Request, emit func(roast.Update)) (roast.Roast, error) {
	ref := Ref(req.Handle)
	if !Valid(ref) {
		return roast.Roast{}, fmt.Errorf("%q %w", req.Handle, ErrBadRef)
	}
	if a.Blocked[strings.ToLower(ref)] {
		return roast.Roast{}, ErrOptedOut
	}

	emit(roast.Update{Phase: roast.PhaseFetch, Label: "Reading the profile"})
	got, err := a.measure(ctx, ref)
	if err != nil {
		return roast.Roast{}, visible(err)
	}

	for i := range got.story {
		emit(roast.Update{Phase: roast.PhaseSection, Section: &got.story[i]})
	}
	emit(roast.Update{Phase: roast.PhaseFeed, Feed: got.feed})
	for _, m := range got.brief.Metrics {
		emit(roast.Update{Phase: roast.PhaseMetric, Metric: &m})
	}
	evidence := Evidence(got.brief.Habits, got.brief.Findings)
	emit(roast.Update{Phase: roast.PhaseSection, Section: &evidence})

	emit(roast.Update{Phase: roast.PhaseVerdict, Label: "Writing the verdict"})
	b := got.brief
	key := profileKey + strings.ToLower(ref)
	if a.Memory != nil {
		b.Avoid = a.Memory.Avoid(key)
	}
	b.Angle = PickAngle(b, rand.IntN)
	page := a.write(ctx, b)
	if a.Memory != nil {
		a.Memory.Said(key, page.Verdict)
	}

	tag := roast.Severity(b.Score)
	if b.Empty {
		tag = "nothing to roast"
	}
	r := roast.Roast{
		Code:      roast.Code(),
		Pack:      roast.LinkedIn,
		Handle:    got.name,
		At:        a.now(),
		Score:     fmt.Sprintf("%d / 100", b.Score),
		ScoreTag:  tag,
		Archetype: page.Archetype,
		Metrics:   b.Metrics,
		Verdict:   page.Verdict,
		Actions:   page.Actions,
		Strengths: page.Strengths,
		Findings:  lined(b.Findings, page.Findings),
		Habits:    lined(b.Habits, page.Habits),
		Story:     got.story,
		Exhibit:   got.exhibit,
	}
	emit(roast.Update{Phase: roast.PhaseVerdict, Verdict: page.Verdict})
	emit(roast.Update{Phase: roast.PhaseDone, Roast: &r})
	return r, nil
}

type measured struct {
	name    string
	brief   Brief
	story   []roast.Section
	feed    []roast.Item
	exhibit *roast.Item
}

// measure reads and briefs the profile, through Memory when there is one, so
// a repeat costs the vendor nothing.
func (a Audit) measure(ctx context.Context, ref string) (measured, error) {
	read := func() (measured, error) {
		p, err := a.Source.Profile(ctx, ref)
		if err != nil {
			return measured{}, err
		}
		if p.Ref == "" {
			p.Ref = ref
		}
		now := a.now()
		b := NewBrief(p, now)
		b.Strengths, b.Actions = Strengths(p, now), Actions(p, now)
		b.Findings, b.Habits = Findings(p, now), Habits(p)
		name := shown(p.Name, 40)
		if name == "" {
			name = ref
		}
		return measured{name: name, brief: b, story: Story(p, now), feed: Feed(p), exhibit: Worst(p)}, nil
	}
	if a.Memory == nil {
		return read()
	}
	return audit.Measure(a.Memory, profileKey+strings.ToLower(ref), read)
}

func (a Audit) write(ctx context.Context, b Brief) verdict.Page {
	if a.Writer == nil {
		return Written(b)
	}
	timeout := a.Timeout
	if timeout == 0 {
		timeout = verdictTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	p, err := a.Writer.Write(ctx, b.Prompt())
	if err != nil {
		slog.Warn("page fell back to the numbers", "pack", roast.LinkedIn, "err", err)
		return Written(b)
	}
	return Merge(b, p)
}

func (a Audit) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// visible is an error a visitor can read. The vendor's own words go to the log.
func visible(err error) error {
	switch {
	case errors.Is(err, ErrNoProfile), errors.Is(err, ErrVendors), errors.Is(err, context.Canceled):
		return err
	case errors.Is(err, context.DeadlineExceeded):
		return ErrVendors
	}
	slog.Error("linkedin", "err", err)
	return ErrVendors
}

// plausibleName is two words or more with a letter in each: enough to search
// on, and not a wasted vendor call on "a" or "123".
func plausibleName(name string) bool {
	words := strings.Fields(name)
	if len(words) < 2 || len([]rune(name)) > 80 {
		return false
	}
	for _, w := range words {
		if strings.IndexFunc(w, unicode.IsLetter) < 0 {
			return false
		}
	}
	return true
}

func lined(fs []roast.Finding, lines []string) []roast.Finding {
	out := append([]roast.Finding(nil), fs...)
	for i := range out {
		if i < len(lines) {
			out[i].Line = lines[i]
		}
	}
	return out
}
