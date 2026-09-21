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

type Odd struct {
	Label string `json:"label"`
	Price string `json:"price"`         // American odds, e.g. "+650"
	Tag   string `json:"tag,omitempty"` // e.g. "lock"
}

// Item is one thing a person published: a commit, a post, an answer.
type Item struct {
	Ref   string    `json:"ref,omitempty"` // a commit hash, a post id
	Where string    `json:"where"`         // the repository, community or tag
	Text  string    `json:"text"`
	At    time.Time `json:"at"`
}

type Roast struct {
	Code     string    `json:"code"`           // short share code, resolved against the event
	Pack     string    `json:"pack,omitempty"` // the platform read; empty is GitHub
	Handle   string    `json:"handle"`
	At       time.Time `json:"at"`
	Score    string    `json:"score"`    // the one number people photograph
	ScoreTag string    `json:"scoreTag"` // severity word under it
	Metrics  []Metric  `json:"metrics"`
	Verdict  string    `json:"verdict"`
	Odds     []Odd     `json:"odds"`

	// Exhibit is the worst item, quoted on the receipt. Nil is an account with
	// nothing public.
	Exhibit *Item `json:"exhibit,omitempty"`

	// Heat is the activity calendar, a week to an entry, Sunday first. -1 is
	// a day still to come. Nil prints no calendar at all, which is different
	// from an empty one.
	Heat [][7]int `json:"heat,omitempty"`

	Hiring string `json:"hiring"` // stub line, filled from open vacancies
}
