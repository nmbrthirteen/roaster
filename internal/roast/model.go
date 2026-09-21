// Package roast holds the result of one audit and turns it into a printable
// document.
package roast

import "time"

type Metric struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Tag   string `json:"tag,omitempty"` // e.g. "critical", "doomed"

	// Percent draws a gauge under the value.
	Percent *int `json:"percent,omitempty"`
}

// Finding is one more thing the account gives away. The share page has room
// for these; the receipt does not.
type Finding struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Line  string `json:"line"`
}

// Item is one thing a person published: a commit, a post, an answer.
type Item struct {
	Ref   string    `json:"ref,omitempty"` // a commit hash, a post id
	Where string    `json:"where"`         // the repository, community or tag
	Text  string    `json:"text"`
	At    time.Time `json:"at"`
}

// Section is one step of the audit screen's reading: a command, and what it
// printed.
type Section struct {
	Cmd   string   `json:"cmd"`
	Lines []string `json:"lines"`
}

type Roast struct {
	Code     string    `json:"code"`           // short share code, resolved against the event
	Pack     string    `json:"pack,omitempty"` // the platform read; empty is GitHub
	Handle   string    `json:"handle"`
	At       time.Time `json:"at"`
	Score    string    `json:"score"`    // the one number people photograph
	ScoreTag string    `json:"scoreTag"` // severity word under it

	// Archetype is the kind of developer the account makes: a short label
	// under the score. Empty on a roast from a server that predates it.
	Archetype string   `json:"archetype,omitempty"`
	Metrics   []Metric `json:"metrics"`
	Verdict   string   `json:"verdict"`

	// Actions are the to-do list a performance review ends on, each one
	// earned by a number above it.
	Actions []string `json:"actions"`

	// Strengths come first in any review worth the name: real credit, with
	// the sting left in.
	Strengths []string `json:"strengths,omitempty"`

	// Findings, Habits and Story are for the share page: more of the roast,
	// and the terminal reading the stand played.
	Findings []Finding `json:"findings,omitempty"`
	Habits   []Finding `json:"habits,omitempty"`
	Story    []Section `json:"story,omitempty"`

	// Exhibit is the worst item, quoted on the receipt. Nil is an account with
	// nothing public.
	Exhibit *Item `json:"exhibit,omitempty"`

	// Heat is the activity calendar, a week to an entry, Sunday first. -1 is
	// a day still to come. Nil prints no calendar at all, which is different
	// from an empty one.
	Heat [][7]int `json:"heat,omitempty"`

	Hiring string `json:"hiring"` // stub line, filled from open vacancies
}
