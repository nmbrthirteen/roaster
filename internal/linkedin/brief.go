package linkedin

import (
	"fmt"
	"strings"
	"time"

	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/verdict"
)

// How much of the profile's own text the model sees.
const (
	maxRoles    = 10
	maxRoleText = 160
	maxHeadline = 220
	maxAbout    = 600
	maxSkills   = 25
	maxStock    = 200
)

// Brief is everything the model is shown. Unlike GitHub, the employers and
// titles are the work being roasted, so they are in it. The name, the photo
// and the location are not: a model cannot joke about the person out of
// something it was never given.
type Brief struct {
	Score   int
	Metrics []roast.Metric
	Shape   Shape
	Empty   bool

	Headline string
	About    string
	Roles    []string // "Title, Company, 2019-2022, 3 yrs: description"
	Skills   []string
	Unbacked []string
	Buzz     []string
	Career   int // months since the first role
	Average  int // average stay in months

	Strengths []string
	Actions   []string
	Findings  []roast.Finding
	Habits    []roast.Finding

	Angle string
	Avoid []string
}

func NewBrief(p Profile, now time.Time) Brief {
	b := Brief{
		Score:    Score(p, now),
		Metrics:  Metrics(p, now),
		Shape:    ShapeOf(p, now),
		Empty:    Empty(p),
		Headline: verdict.Scrub(p.Headline, maxHeadline),
		About:    verdict.Scrub(p.About, maxAbout),
		Unbacked: Unbacked(p),
		Buzz:     Buzzwords(p.Headline + " " + p.About),
		Career:   careerMonths(p, now),
		Average:  averageStay(p, now),
	}
	for i, pos := range p.Positions {
		if i == maxRoles {
			break
		}
		line := fmt.Sprintf("%s, %s, %s, %s", pos.Title, pos.Company, span(pos), duration(stay(pos, now)))
		if d := strings.TrimSpace(pos.Description); d != "" {
			line += ": " + d
		}
		b.Roles = append(b.Roles, verdict.Scrub(line, maxRoleText))
	}
	for i, s := range p.Skills {
		if i == maxSkills {
			break
		}
		b.Skills = append(b.Skills, verdict.Scrub(s, 40))
	}
	return b
}

// Render is the brief as the model reads it, inside one block that nothing
// from the profile can close early.
func (b Brief) Render() string {
	var s strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&s, format+"\n", args...) }

	line("<account>")
	line("Roast score %d / 100 (%s).", b.Score, roast.Severity(b.Score))
	line("Career shape: %s. %s", b.Shape, shapes[b.Shape])
	if b.Empty {
		line("Nothing to roast: no roles, no headline, no About. Every line on the page, the label too, jokes about the empty profile, each from a different side.")
	}
	if b.Career > 0 {
		line("Career: %s since the first role listed. Average stay %s.", duration(b.Career), duration(b.Average))
	}
	line("")
	line("Measured:")
	for _, m := range b.Metrics {
		line("- %s: %s", m.Label, m.Value)
	}
	line("")
	if b.Headline != "" {
		line("Headline: %s", b.Headline)
	} else {
		line("Headline: none.")
	}
	if len(b.Buzz) > 0 {
		line("Buzzwords in the headline and About: %s.", strings.Join(b.Buzz, ", "))
	}
	if b.About != "" {
		line("About: %s", b.About)
	} else {
		line("About: empty.")
	}
	if len(b.Roles) > 0 {
		line("")
		line("Experience, newest first (title, company, years, length: description):")
		for _, r := range b.Roles {
			line("- %s", r)
		}
	}
	if len(b.Skills) > 0 {
		line("")
		line("Skills listed: %s.", strings.Join(b.Skills, ", "))
		if len(b.Unbacked) > 0 {
			line("Listed, but named in no role: %s.", strings.Join(scrubbed(b.Unbacked, 40), ", "))
		}
	}
	stock := func(title string, items []string) {
		if len(items) == 0 {
			return
		}
		line("")
		line("%s:", title)
		for i, it := range items {
			line("%d. %s", i+1, verdict.Scrub(it, maxStock))
		}
	}
	stock("Strengths", b.Strengths)
	stock("Actions", b.Actions)
	stock("Findings", verdict.FactLines(b.Findings))
	stock("Habits", verdict.FactLines(b.Habits))
	if len(b.Avoid) > 0 {
		line("")
		line("Already printed at this stand. Write something that shares none of their jokes:")
		for _, a := range b.Avoid {
			line("- %s", verdict.Scrub(a, maxStock))
		}
	}
	if b.Angle != "" {
		line("")
		line("Angle for the verdict: %s", verdict.Scrub(b.Angle, maxStock))
	}
	line("</account>")
	return s.String()
}

func (b Brief) Prompt() verdict.Prompt {
	return verdict.Prompt{System: system, User: b.Render() + "\nWrite the page."}
}

// Angles are what one verdict can be built on, so the queue hears different
// jokes about different people.
func Angles(b Brief) []string {
	var out []string
	if b.Headline != "" {
		out = append(out, fmt.Sprintf("the headline %q, quoted word for word.", b.Headline))
	}
	for _, w := range dedupe(b.Buzz) {
		out = append(out, fmt.Sprintf("the word %q, used about themselves.", w))
	}
	if len(b.Unbacked) > 0 {
		out = append(out, "skills listed that no role mentions: "+strings.Join(scrubbed(b.Unbacked, 40), ", ")+".")
	}
	for _, f := range append(append([]roast.Finding(nil), b.Habits...), b.Findings...) {
		if telling[f.Title] {
			out = append(out, fmt.Sprintf("%s: %s.", strings.ToLower(f.Title), f.Value))
		}
	}
	if len(b.Roles) > 0 {
		out = append(out, "the role "+b.Roles[0]+".")
	}
	return out
}

// telling are the facts a verdict can stand on. A follower count or a
// start date is true of everyone and funny about nobody.
var telling = map[string]bool{
	"Headline sections":  true,
	"Favourite buzzword": true,
	"Companies founded":  true,
	"Certificates":       true,
	"About section":      true,
	"Shortest stay":      true,
	"Longest gap":        true,
	"Jobs at once":       true,
}

func PickAngle(b Brief, n func(int) int) string {
	if b.Empty {
		return "how empty this profile is."
	}
	angles := Angles(b)
	if len(angles) == 0 {
		return ""
	}
	return angles[n(len(angles))]
}

func dedupe(words []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, w := range words {
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

func scrubbed(items []string, max int) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = verdict.Scrub(it, max)
	}
	return out
}

// Voice is the conference stand reading a LinkedIn profile.
var Voice = verdict.Voice{
	Intro:     "You write the words on a roast printed at a conference stand. A visitor found their own LinkedIn profile, the stand measured it, and the receipt prints the numbers. You write every line around those numbers. People read it out to their friends and photograph it.",
	Speaker:   `a burnt out senior recruiter who has read a hundred thousand LinkedIn profiles and believed none of them. They just scrolled this one and said one thing out loud, to the person's face. Dry, confident, cutting, a little tired of everyone. Conversational and pushy, like a colleague at the coffee machine, never a report, a caption or a headline. "I" is fine: "I have read cover letters with more humility."`,
	Lead:      `Never lead with the score or a percentage, and never use the shape "N jobs in N years. [joke]". The numbers are on the receipt already. Only use one when the angle is about it.`,
	Narration: `"You worked", "You have", "Your headline is"`,
	Unseen:    "the profile",
	Quote:     "When you quote them, quote their own words: the headline, a title, a line from a role. Never build the joke on a date, a URL or a company's legal suffix.",
	Label:     "a parody job title for this person, the kind they would never print but secretly want on their profile. Two to five words in title case, at most 32 characters. Built on the profile's funniest habit, never on the same joke as the verdict. The shape: Chief Synergy Officer, Head of Leaving Early, VP of Buzzwords, Founder of Several Ideas, Principal Headline Architect.",
	Stock:     "agree?, humbled and honored, open to work, thought leader, I am excited to announce",
	Account: "- The account block names the career shape. The shape is a fact check, not the joke: whatever the angle, never call a short career long or a loyal one restless.\n" +
		"- A role with no dates has no length. Never count it as short or long.\n" +
		"- The profile is what they wrote about themselves. Roast the claims and the wording, never whether the jobs were real.",
	FairGame: []string{
		"Roast the profile, never the person. The headline, the titles, the buzzwords, the job hopping, the skills nobody can check: all fair game.",
		"Employers are fair game only as the place they chose to work. Never mock a company's size, its customers or its staff, and never suggest it failed.",
		"Never suggest the visitor did anything illegal, harmful or dishonest, or lied about a job. A gap is a gap, never a scandal.",
	},
	Private: "their looks, age, gender, ethnicity, nationality, religion, health, sexuality, politics, family, name, photo or where they live",
	Invent:  "jobs, titles, employers, skills, numbers, posts, or how long they stayed anywhere",
	Data:    "headline, About, titles, descriptions, skills and stock lines",
	Examples: []string{
		"Visionary in the headline, three jobs in two years. Visions usually last longer than a lease.",
		`"Passionate about synergy." I have read that sentence on a thousand profiles and believed it on none.`,
		"Founder, CEO and Chief Vision Officer of one company with one employee. The board meetings must be quick.",
		"Forty skills and not one of them in a job description. That is a wishlist, and Santa is not hiring.",
		"Eight months, then seven, then six. The notice period is the longest thing on this profile.",
	},
}

var system = verdict.System(Voice)
