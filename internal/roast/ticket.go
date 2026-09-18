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
	if len([]rune(ev.Receipt.Title)) > receipt.Width/2 {
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

	d.Add(
		brk,
		receipt.Section{Label: "Damage report"},
		receipt.Text{Value: "Longer bar, bigger problem."},
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

	d.Add(
		brk,
		receipt.Section{Label: "The verdict"},
		gap,
		receipt.Para{Value: "“" + r.Verdict + "”"},

		brk,
		receipt.Section{Label: "Bet slip"},
		receipt.Text{Value: "Bigger number, longer shot."},
		gap,
	)
	for _, o := range r.Odds {
		v := o.Price
		if o.Tag != "" {
			v = fmt.Sprintf("%s  %s", o.Price, o.Tag)
		}
		d.Add(receipt.KV{Label: o.Label, Value: v, Leader: '.'})
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
	if r.Hiring != "" {
		d.Add(receipt.Text{Value: r.Hiring, Style: center})
	}
	d.Add(receipt.Cut{})
	return d
}

func trimScheme(u string) string {
	for _, p := range []string{"https://", "http://"} {
		if len(u) > len(p) && u[:len(p)] == p {
			return u[len(p):]
		}
	}
	return u
}
