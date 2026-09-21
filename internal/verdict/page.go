package verdict

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/upgaming/roaster/internal/roast"
)

type Page struct {
	Archetype string   `json:"archetype"`
	Verdict   string   `json:"verdict"`
	Strengths []string `json:"strengths"`
	Actions   []string `json:"actions"`
	Findings  []string `json:"findings"`
	Habits    []string `json:"habits"`
}

const (
	maxLine      = 100
	maxArchetype = 32
)

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

func parse(text string) (Page, error) {
	var p Page
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &p); err != nil {
		return Page{}, ErrUnusable
	}
	return p, nil
}

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
	out.Findings = overlay(out.Findings, unprefixed(b.Findings, p.Findings))
	out.Habits = overlay(out.Habits, unprefixed(b.Habits, p.Habits))
	return out
}

func unprefixed(fs []roast.Finding, lines []string) []string {
	if len(lines) != len(fs) {
		return lines
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		l = strings.TrimSpace(l)
		if rest, ok := strings.CutPrefix(l, "["+fs[i].Title+", "+fs[i].Value+"]"); ok {
			l = rest
		}
		if len(l) >= len(fs[i].Title) && strings.EqualFold(l[:len(fs[i].Title)], fs[i].Title) {
			rest := strings.TrimLeft(l[len(fs[i].Title):], ": ")
			rest = strings.TrimPrefix(rest, fs[i].Value)
			l = strings.TrimLeft(rest, ".,: ")
		}
		out[i] = strings.TrimSpace(l)
	}
	return out
}

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
