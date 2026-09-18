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

type Roast struct {
	Code     string    `json:"code"` // short share code, resolved against the event
	Handle   string    `json:"handle"`
	At       time.Time `json:"at"`
	Score    string    `json:"score"`    // the one number people photograph
	ScoreTag string    `json:"scoreTag"` // severity word under it
	Metrics  []Metric  `json:"metrics"`
	Verdict  string    `json:"verdict"`
	Odds     []Odd     `json:"odds"`
	Hiring   string    `json:"hiring"` // stub line, filled from open vacancies
}
