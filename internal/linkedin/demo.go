package linkedin

import (
	"context"
	"strings"
	"time"

	"github.com/upgaming/roaster/internal/roast"
)

// Demo is the rehearsal set: made-up profiles, so a stand can run the whole
// LinkedIn flow with no vendor and no key. Every search finds the same three
// people, the first given the name that was typed.
type Demo struct{}

// Reserved references for the paths that are otherwise hard to reach.
const (
	demoMissing = "notfound"
	demoEmpty   = "ghost"
	demoSlow    = "slowpoke"
)

func (Demo) Search(ctx context.Context, name string) ([]roast.Candidate, error) {
	if err := pause(ctx, 700*time.Millisecond); err != nil {
		return nil, err
	}
	typed := titled(name)
	return []roast.Candidate{
		{Ref: "demo-hopper", Name: typed, Headline: demoHopper.Headline, Location: "Tbilisi, Georgia"},
		{Ref: "demo-founder", Name: typed, Headline: demoFounder.Headline, Location: "Batumi, Georgia"},
		{Ref: demoEmpty, Name: typed, Location: "Kutaisi, Georgia"},
	}, nil
}

func (Demo) Profile(ctx context.Context, ref string) (Profile, error) {
	wait := 900 * time.Millisecond
	if ref == demoSlow {
		wait *= 4
	}
	if err := pause(ctx, wait); err != nil {
		return Profile{}, err
	}
	switch ref {
	case demoMissing:
		return Profile{}, ErrNoProfile
	case demoEmpty:
		return Profile{Ref: ref, Name: "Quiet Person"}, nil
	case "demo-founder":
		return demoFounder, nil
	default:
		p := demoHopper
		p.Ref = ref
		return p, nil
	}
}

func pause(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func titled(name string) string {
	words := strings.Fields(name)
	for i, w := range words {
		r := []rune(strings.ToLower(w))
		if len(r) > 0 {
			r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		}
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

func month(y int, m time.Month) time.Time { return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC) }

var demoHopper = Profile{
	Ref:       "demo-hopper",
	Name:      "Jordan Sample",
	Headline:  "Passionate Product Leader | Visionary Builder | Growth Mindset | Coffee Enthusiast",
	About:     "Results-driven product leader with a proven track record of driving synergy across teams. Passionate about building innovative products that customers love.",
	Location:  "Tbilisi, Georgia",
	Followers: 2140,
	Positions: []Position{
		{Title: "Head of Product", Company: "Northwind Labs", Start: month(2025, time.March)},
		{Title: "Lead Product Manager", Company: "Contoso Pay", Start: month(2024, time.January), End: month(2025, time.February), Description: "Drove innovative synergy across the payments squad."},
		{Title: "Senior Product Manager", Company: "Fabrikam", Start: month(2023, time.February), End: month(2023, time.December)},
		{Title: "Product Manager", Company: "Tailspin Toys", Start: month(2021, time.June), End: month(2022, time.November), Description: "Shipped the mobile checkout and cut drop-off by 12%."},
		{Title: "Product Owner", Company: "Wingtip", Start: month(2020, time.January), End: month(2021, time.March)},
	},
	Education:      []School{{Name: "Example State University", Degree: "BSc Business", End: month(2019, time.June)}},
	Skills:         []string{"Product Strategy", "Leadership", "Blockchain", "Machine Learning", "Public Speaking", "Checkout", "Agile", "Growth Hacking"},
	Certifications: []string{"Scrum Product Owner", "Design Thinking", "Growth Marketing", "AI for Leaders"},
}

var demoFounder = Profile{
	Ref:       "demo-founder",
	Name:      "Riley Fixture",
	Headline:  "Founder & CEO | Serial Entrepreneur | Thought Leader",
	Location:  "Batumi, Georgia",
	Followers: 512,
	Positions: []Position{
		{Title: "Founder & CEO", Company: "Riley Ventures", Start: month(2022, time.May)},
		{Title: "Co-founder", Company: "Stealth Startup", Start: month(2023, time.August)},
		{Title: "Chief Vision Officer", Company: "Riley Ventures", Start: month(2022, time.May)},
		{Title: "Marketing Intern", Company: "Adventure Works", Start: month(2021, time.June), End: month(2021, time.September), Description: "Scheduled social posts and ran the team calendar."},
	},
	Skills: []string{"Entrepreneurship", "Fundraising", "Leadership", "Web3", "Marketing"},
}
