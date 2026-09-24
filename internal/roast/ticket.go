package roast

import (
	"fmt"

	"github.com/upgaming/roaster/internal/event"
	"github.com/upgaming/roaster/internal/receipt"
)

// Doc lays out the printed audit.
func (r Roast) Doc(ev event.Event, terminal string) *receipt.Doc {
	var (
		center = receipt.Style{Align: receipt.AlignCenter}
		heavy  = receipt.Rule{Char: ev.HeavyRuleChar()}
		gap    = receipt.Feed{Lines: 1}
		brk    = receipt.Feed{Lines: 2}

		// Double size keeps the letterforms in proportion.
		head = receipt.Style{Align: receipt.AlignCenter, Bold: true, Double: true}
	)

	// A masthead that outruns the 21 double-width columns falls back to single
	// width rather than wrapping across two lines.
	if len([]rune(ev.Receipt.Title)) > receipt.Width()/2 {
		head = receipt.Style{Align: receipt.AlignCenter, Bold: true, Tall: true}
	}

	d := &receipt.Doc{}
	d.Add(
		gap,
		receipt.Image{Name: ev.Receipt.Logo},
		brk,
		receipt.Text{Value: ev.Receipt.Title, Style: head, Bleed: true},
		gap,
		receipt.Text{Value: fmt.Sprintf("Terminal #%s  |  %s", terminal, ev.Name), Style: center},
		brk,

		heavy,
		receipt.KV{Label: "Candidate", Value: "@" + r.Handle},
		receipt.KV{Label: "Audited", Value: r.At.Format("2 Jan 2006, 15:04")},
		heavy,
	)

	if r.Score != "" {
		d.Add(brk, receipt.Hero{Caption: "Roast severity: " + r.ScoreTag, Value: r.Score})
	}
	if r.Archetype != "" {
		d.Add(gap, receipt.Text{Value: r.Archetype, Style: receipt.Style{Align: receipt.AlignCenter, Bold: true}})
	}

	d.Add(
		brk,
		receipt.Section{Label: "Damage report"},
		receipt.Text{Value: "Read straight off your account. Sorry."},
		gap,
	)
	for _, m := range r.Metrics {
		if m.Percent != nil {
			d.Add(receipt.Bar{Label: m.Label, Value: m.Value, Percent: *m.Percent, Tag: m.Tag})
			continue
		}
		v := m.Value
		if m.Tag != "" {
			v = fmt.Sprintf("%s [%s]", m.Value, m.Tag)
		}
		d.Add(receipt.KV{Label: m.Label, Value: v})
	}

	pack := PackFor(r.Pack)
	r.heat(d, pack)
	r.exhibit(d, pack)

	d.Add(
		brk,
		receipt.Section{Label: "The verdict"},
		gap,
		receipt.Para{Value: "“" + r.Verdict + "”"},

		brk,
	)
	if len(r.Strengths) > 0 {
		d.Add(receipt.Section{Label: "Strengths"}, gap)
		for _, s := range r.Strengths {
			d.Add(receipt.Para{Value: s, Mark: "+ "})
		}
		d.Add(brk)
	}
	d.Add(
		receipt.Section{Label: "Action items"},
		receipt.Text{Value: "Agreed in this review. Due by the next one."},
		gap,
	)
	for _, a := range r.Actions {
		d.Add(receipt.Para{Value: a, Mark: "[ ] "})
	}

	share := ev.ShareURL(r.Code)
	d.Add(
		brk,
		receipt.QR{Data: share, Size: 5},
		gap,
		receipt.Text{Value: ev.Receipt.CTA, Style: center},
		receipt.Text{Value: trimScheme(share), Style: center},
		brk,
		receipt.Tear{},
		gap,
	)
	if ev.Receipt.Stub != "" {
		d.Add(receipt.Text{Value: ev.Receipt.Stub, Style: receipt.Style{Align: receipt.AlignCenter, Bold: true}})
	}
	// The count belongs to the event, not the person. Everyone standing at the
	// same stand is looking at the same open roles.
	hiring := r.Hiring
	if hiring == "" {
		hiring = ev.Receipt.Hiring
	}
	if hiring != "" {
		d.Add(receipt.Text{Value: hiring, Style: center})
	}
	d.Add(receipt.Cut{})
	return d
}

// An empty calendar still prints its grid. The page of blank days is the joke.
func (r Roast) heat(d *receipt.Doc, pack Pack) {
	if r.Heat == nil {
		return
	}
	total, idle, days := 0, 0, 0
	for _, w := range r.Heat {
		for _, n := range w {
			if n < 0 {
				continue
			}
			days++
			total += n
			if n == 0 {
				idle++
			}
		}
	}
	d.Add(
		receipt.Feed{Lines: 2},
		receipt.Section{Label: pack.Calendar},
		receipt.Text{Value: fmt.Sprintf("%d weeks, a column each. Dark is busy.", len(r.Heat))},
		receipt.Feed{Lines: 1},
		receipt.Heatmap{Weeks: r.Heat},
		receipt.Feed{Lines: 1},
		receipt.KV{Label: "Contributions", Value: fmt.Sprint(total)},
		receipt.KV{Label: "Days with none", Value: fmt.Sprintf("%d of %d", idle, days)},
	)
	if total == 0 {
		d.Add(receipt.Feed{Lines: 1}, receipt.Text{Value: pack.Spotless})
	}
}

// exhibit prints where and when, so anyone holding the receipt can check it.
func (r Roast) exhibit(d *receipt.Doc, pack Pack) {
	d.Add(receipt.Feed{Lines: 2}, receipt.Section{Label: "Your worst commit"})
	w := r.Exhibit
	if w == nil {
		d.Add(
			receipt.Text{Value: pack.NoExhibit[0]},
			receipt.Feed{Lines: 1},
			receipt.Text{Value: pack.NoExhibit[1]},
		)
		return
	}
	msg := w.Text
	if msg == "" {
		msg = "(an empty message)"
	}
	where := w.Where
	if w.Ref != "" {
		where = w.Ref + " in " + w.Where
	}
	d.Add(
		receipt.Text{Value: pack.Exhibit},
		receipt.Feed{Lines: 1},
		receipt.Para{Value: "“" + msg + "”", Indent: 2},
		receipt.Feed{Lines: 1},
		receipt.Text{Value: where},
		receipt.Text{Value: w.At.Format("Mon 2 Jan 2006, 15:04")},
	)
}

func trimScheme(u string) string {
	for _, p := range []string{"https://", "http://"} {
		if len(u) > len(p) && u[:len(p)] == p {
			return u[len(p):]
		}
	}
	return u
}
