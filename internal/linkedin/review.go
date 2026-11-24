package linkedin

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/upgaming/roaster/internal/metric"
	"github.com/upgaming/roaster/internal/roast"
)

// Findings are what the profile gives away, each with a stock line the model
// may beat. The value is the fact; the line is the joke.
func Findings(p Profile, now time.Time) []roast.Finding {
	var out []roast.Finding
	if start := careerStart(p); !start.IsZero() {
		years := months(start, now) / 12
		line := fmt.Sprintf("%s in and %s to show for it.", count(years, "year"), count(len(p.Positions), "job title"))
		if years == 0 {
			line = "Under a year in. The buzzwords arrived before the experience."
		}
		out = append(out, roast.Finding{Title: "Career so far", Value: "since " + start.Format("Jan 2006"), Line: line})
	}

	if longest, ok := extreme(p, now, func(a, b int) bool { return a > b }); ok && len(p.Positions) > 1 {
		out = append(out, roast.Finding{
			Title: "Longest stay",
			Value: duration(stay(longest, now)) + " at " + shown(longest.Company, 28),
			Line:  "The one place that kept you. Or the one you never told you were leaving.",
		})
	}
	if shortest, ok := extreme(p, now, func(a, b int) bool { return a < b }); ok && finished(p) > 1 && stay(shortest, now) < shortStay {
		out = append(out, roast.Finding{
			Title: "Shortest stay",
			Value: duration(stay(shortest, now)) + " at " + shown(shortest.Company, 28),
			Line:  "Long enough to update the headline. Not long enough to learn the wifi password.",
		})
	}
	if gap := longestGap(p, now); gap >= 3 {
		out = append(out, roast.Finding{Title: "Longest gap", Value: duration(gap), Line: "Officially a sabbatical. The profile is keeping quiet about it."})
	}
	if n := atOnce(p, now); n >= 2 {
		out = append(out, roast.Finding{Title: "Jobs at once", Value: count(n, "role"), Line: "Several full-time jobs, one person. The calendar must be fiction."})
	}
	if n := len(p.Skills); n > 0 {
		out = append(out, roast.Finding{Title: "Skills listed", Value: fmt.Sprint(n), Line: "A skills section long enough to need its own skills section."})
	}
	if p.Followers > 0 {
		out = append(out, roast.Finding{Title: "Followers", Value: fmt.Sprint(p.Followers), Line: "An audience. Mostly recruiters and former classmates."})
	}
	return out
}

// Habits are how the profile is written: the claims it stacks and the words it
// leans on.
func Habits(p Profile) []roast.Finding {
	var out []roast.Finding
	if parts := headlineParts.Split(strings.TrimSpace(p.Headline), -1); len(parts) >= 2 {
		out = append(out, roast.Finding{Title: "Headline sections", Value: fmt.Sprint(len(parts)), Line: "A headline built like a train timetable. Every stop a different job."})
	}
	if word, n := favourite(Buzzwords(p.Headline + " " + p.About)); n > 0 {
		out = append(out, roast.Finding{Title: "Favourite buzzword", Value: fmt.Sprintf("%q, %s", word, count(n, "time")), Line: "Said often enough that it stopped meaning anything."})
	}
	if n := founded(p); n > 0 {
		out = append(out, roast.Finding{Title: "Companies founded", Value: fmt.Sprint(n), Line: "Founder is the title you give yourself when nobody else will."})
	}
	if n := len(p.Certifications); n >= 3 {
		out = append(out, roast.Finding{Title: "Certificates", Value: fmt.Sprint(n), Line: "Proof you finished the videos. The work is still pending."})
	}
	about := len(strings.FieldsFunc(p.About, notWord))
	switch {
	case about == 0:
		out = append(out, roast.Finding{Title: "About section", Value: "empty", Line: "The one box that asks who you are, left blank."})
	case about > 250:
		out = append(out, roast.Finding{Title: "About section", Value: count(about, "word"), Line: "An autobiography where a paragraph would do."})
	}
	return out
}

// Strengths open the review: real credit, each earned by a number.
func Strengths(p Profile, now time.Time) []string {
	var out []string
	add := func(ok bool, s string) {
		if ok && len(out) < metric.StrengthsMax {
			out = append(out, s)
		}
	}
	if longest, ok := extreme(p, now, func(a, b int) bool { return a > b }); ok {
		add(stay(longest, now) >= 36, fmt.Sprintf("Stayed %s at %s. Loyalty, on LinkedIn of all places.", duration(stay(longest, now)), shown(longest.Company, 28)))
	}
	add(len(p.Positions) > 0 && undescribed(p) < 33, "Most roles have a description. Someone read the form.")
	add(len(p.Skills) > 0 && unbacked(p) < 33, "Most skills are backed by a real job. Rare honesty.")
	add(strings.TrimSpace(p.Headline) != "" && buzzShare(p.Headline) == 0, "A headline with no buzzwords. Suspiciously sincere.")
	add(len(p.Education) > 0, "Education listed. At least one thing here was graded.")
	add(true, "Showed up to a roast stand voluntarily. Brave.")
	add(true, "Has a LinkedIn profile, which is more than the roaster expected.")
	return out
}

// Actions end the review, worst first, each naming what earned it.
func Actions(p Profile, now time.Time) []string {
	var out []string
	add := func(ok bool, s string) {
		if ok && len(out) < metric.ActionsMax {
			out = append(out, s)
		}
	}
	if n := len(p.Positions) * undescribed(p) / 100; n > 0 {
		add(true, fmt.Sprintf("Write one line under each of the %s with no description.", count(n, "role")))
	}
	if words := Buzzwords(p.Headline); len(words) > 0 {
		add(true, fmt.Sprintf("Take %q out of the headline.", words[0]))
	}
	if skills := Unbacked(p); len(skills) > 0 {
		add(true, fmt.Sprintf("Back up %s with a real job, or delete it.", shown(skills[0], 30)))
	}
	add(strings.TrimSpace(p.About) == "", "Write an About section. Three sentences, no adjectives.")
	add(len(headlineParts.Split(strings.TrimSpace(p.Headline), -1)) >= 3, "Cut the headline to the one thing you actually do.")
	add(hopped(p, now) >= 50, "Stay somewhere long enough to finish one project.")
	add(true, "Frame this receipt. It is the most honest thing on your profile.")
	return out
}

// Story is the profile as the screen posts it: one card a section, the card's
// title in Cmd. A line starting "# " is the roaster's note on the card, and
// every value comes from the profile.
func Story(p Profile, now time.Time) []roast.Section {
	profile := roast.Section{Cmd: "Profile", Lines: []string{shown(p.Name, 40), shown(p.Headline, 120), shown(p.Location, 40)}}
	if p.Followers > 0 {
		profile.Lines = append(profile.Lines, metric.Thousands(p.Followers)+" "+plural(p.Followers, "follower"))
	}

	about := roast.Section{Cmd: "About"}
	if text := shown(p.About, 280); text != "" {
		about.Lines = []string{text}
	}
	if len(about.Lines) == 0 {
		about.Lines = []string{"# No About section. A person of few words, or none."}
	} else if words := Buzzwords(p.About); len(words) > 0 {
		about.Lines = append(about.Lines, fmt.Sprintf("# %s in one paragraph.", count(len(words), "buzzword")))
		about.Marks = dedupe(words)
	}

	experience := roast.Section{Cmd: "Experience"}
	for i, pos := range p.Positions {
		if i == 6 {
			experience.Lines = append(experience.Lines, fmt.Sprintf("# And %d more.", len(p.Positions)-i))
			break
		}
		experience.Lines = append(experience.Lines, fmt.Sprintf("%s at %s, %s", shown(pos.Title, 40), shown(pos.Company, 28), span(pos)))
	}
	if len(p.Positions) == 0 {
		experience.Lines = []string{"# No experience listed. The career is in stealth mode."}
	} else {
		experience.Lines = append(experience.Lines, "# Average stay: "+duration(averageStay(p, now))+".")
	}

	skills := roast.Section{Cmd: "Skills"}
	for i, sk := range p.Skills {
		if i == 12 {
			break
		}
		skills.Lines = append(skills.Lines, shown(sk, 30))
	}
	switch unproven := Unbacked(p); {
	case len(p.Skills) == 0:
		skills.Lines = []string{"# No skills listed. Modest, or undecided."}
	case len(unproven) > 0:
		skills.Lines = append(skills.Lines, fmt.Sprintf("# %d of %d appear in no job.", len(unproven), len(p.Skills)))
		skills.Marks = unproven
	}
	return []roast.Section{profile, about, experience, skills}
}

// Evidence closes the reading: what the roaster noticed, a fact a line.
func Evidence(habits, findings []roast.Finding) roast.Section {
	s := roast.Section{Cmd: "What the roaster noticed"}
	for _, f := range append(append([]roast.Finding(nil), habits...), findings...) {
		if len(s.Lines) == 8 {
			break
		}
		s.Lines = append(s.Lines, f.Title+": "+shown(f.Value, 40))
	}
	if len(s.Lines) == 0 {
		s.Lines = []string{"# Not much to go on. The verdict will improvise, honestly."}
	}
	return s
}

// Feed is the career as the screen plays it, newest first.
func Feed(p Profile) []roast.Item {
	out := make([]roast.Item, 0, len(p.Positions))
	for _, pos := range p.Positions {
		out = append(out, roast.Item{Where: shown(pos.Company, 40), Text: shown(pos.Title, 60), At: pos.Start})
	}
	return out
}

// Worst is the line printed word for word on the receipt: the headline when it
// leans on buzzwords, otherwise the most buzzword-heavy role description.
func Worst(p Profile) *roast.Item {
	if h := strings.TrimSpace(p.Headline); h != "" && len(Buzzwords(h)) > 0 {
		return &roast.Item{Where: "Headline", Text: shown(h, 160)}
	}
	var best *Position
	most := 0
	for i := range p.Positions {
		if n := len(Buzzwords(p.Positions[i].Description)); n > most {
			best, most = &p.Positions[i], n
		}
	}
	if best != nil {
		return &roast.Item{Where: shown(best.Company, 40), Text: shown(firstSentence(best.Description), 160), At: best.Start}
	}
	if h := strings.TrimSpace(p.Headline); h != "" {
		return &roast.Item{Where: "Headline", Text: shown(h, 160)}
	}
	return nil
}

func extreme(p Profile, now time.Time, better func(a, b int) bool) (Position, bool) {
	var found Position
	ok := false
	for _, pos := range p.Positions {
		if pos.Start.IsZero() {
			continue
		}
		if !ok || better(stay(pos, now), stay(found, now)) {
			found, ok = pos, true
		}
	}
	return found, ok
}

func favourite(words []string) (string, int) {
	counts := map[string]int{}
	for _, w := range words {
		counts[w]++
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) == 0 {
		return "", 0
	}
	return keys[0], counts[keys[0]]
}

func span(p Position) string {
	if p.Start.IsZero() {
		return ""
	}
	end := "now"
	if !p.End.IsZero() {
		end = p.End.Format("2006")
	}
	if end == p.Start.Format("2006") {
		return end
	}
	return p.Start.Format("2006") + " to " + end
}

func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, ".!?\n"); i > 0 {
		return s[:i+1]
	}
	return s
}

// shown is someone else's text made safe for the screen and cut to a column.
func shown(s string, max int) string {
	s = strings.Join(strings.FieldsFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }), " ")
	if r := []rune(s); len(r) > max {
		s = string(r[:max-1]) + "."
	}
	return s
}
