package verdict

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/upgaming/roaster/internal/roast"
)

// Page is every line of the roast that is written rather than measured. The
// numbers are never in it: the model only ever words what the audit found.
type Page struct {
	Archetype string   `json:"archetype"`
	Verdict   string   `json:"verdict"`
	Strengths []string `json:"strengths"`
	Actions   []string `json:"actions"`
	Findings  []string `json:"findings"` // a line for each Brief.Findings, in order
	Habits    []string `json:"habits"`   // a line for each Brief.Habits, in order
}

// maxLine is one written line other than the verdict: a receipt line or two.
// maxArchetype is a label, which has to fit on one line of paper.
const (
	maxLine      = 140
	maxArchetype = 32
)

// schema is the shape both models are held to. Lengths are not in it, since
// strict schemas cannot say "as many as there were"; Merge checks them.
var schema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"archetype": map[string]any{"type": "string"},
		"verdict":   map[string]any{"type": "string"},
		"strengths": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"actions":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"findings":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"habits":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
	},
	"required":             []string{"archetype", "verdict", "strengths", "actions", "findings", "habits"},
	"additionalProperties": false,
}

// parse reads what a model sent back. Anything that is not the shape asked
// for is unusable as a whole; a bad line inside it is Merge's problem.
func parse(text string) (Page, error) {
	var p Page
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &p); err != nil {
		return Page{}, ErrUnusable
	}
	return p, nil
}

// Merge lays what the model wrote over what the numbers wrote, a line at a
// time. A line that is missing, unprintable or too long keeps the stock one,
// and a list that came back the wrong length is kept whole, because a line
// out of place would sit under the wrong number.
func Merge(b Brief, p Page) Page {
	out := Written(b)
	if v, ok := Clean(p.Verdict); ok {
		out.Verdict = v
	}
	if a, ok := label(p.Archetype); ok {
		out.Archetype = a
	}
	out.Strengths = overlay(out.Strengths, p.Strengths)
	out.Actions = overlay(out.Actions, p.Actions)
	out.Findings = overlay(out.Findings, p.Findings)
	out.Habits = overlay(out.Habits, p.Habits)
	return out
}

// Written is the page from the numbers alone: what prints when there is no
// model, or the model had nothing usable to say.
func Written(b Brief) Page {
	return Page{
		Archetype: Archetype(b),
		Verdict:   Fallback(b),
		Strengths: b.Strengths,
		Actions:   b.Actions,
		Findings:  linesOf(b.Findings),
		Habits:    linesOf(b.Habits),
	}
}

func overlay(stock, written []string) []string {
	if len(written) != len(stock) {
		return stock
	}
	out := make([]string, len(stock))
	for i := range stock {
		out[i] = stock[i]
		if l, ok := Clean(written[i]); ok && utf8.RuneCountInString(l) <= maxLine {
			out[i] = l
		}
	}
	return out
}

// label is an archetype made printable: short, one line, with no full stop
// trailing off it.
func label(s string) (string, bool) {
	s, ok := Clean(s)
	s = strings.TrimRight(s, ".!")
	if !ok || s == "" || utf8.RuneCountInString(s) > maxArchetype {
		return "", false
	}
	return s, true
}

func linesOf(fs []roast.Finding) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Line
	}
	return out
}
