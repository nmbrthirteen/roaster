// Package server is the kiosk's HTTP surface. One binary serves three things:
// the stand a visitor walks up to, the designer an operator lays receipts out
// in, and the hidden menu that keeps a locked device serviceable.
//
// Everything a visitor touches has to be quick and has to work with the venue
// wifi down, so the stand holds no state a restart would lose and asks nothing
// of the network that is not on the machine already.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/upgaming/roaster/internal/config"
	"github.com/upgaming/roaster/internal/event"
	"github.com/upgaming/roaster/internal/receipt"
	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/ui"
)

// Options is what a server cannot work out for itself.
type Options struct {
	Config   config.Config  // what is running, flags included
	Saved    config.Config  // what the settings file says, which is what gets saved back
	Path     string         // the settings file
	Provider roast.Provider // set by tests; a stand builds its own from the settings
	Assets   receipt.Assets
}

type Server struct {
	st       *station
	provider roast.Provider
	assets   receipt.Assets
	started  time.Time
	netState atomic.Pointer[string]
	queued   atomic.Int64 // when the print queue was first seen holding a job, in Unix nanoseconds

	designer *template.Template
	kiosk    *template.Template
}

// New builds a server. A printer that will not open is reported and stepped
// over: a stand with no paper still has to come up, because the hidden menu is
// how an operator fixes it.
func New(o Options) (*Server, error) {
	s := &Server{
		st: &station{
			cfg:    o.Config,
			saved:  o.Saved,
			path:   o.Path,
			roasts: map[string]roast.Roast{},
		},
		provider: o.Provider,
		assets:   o.Assets,
		started:  time.Now(),
		designer: template.Must(template.ParseFS(ui.FS, "preview.html")),
		kiosk:    template.Must(template.ParseFS(ui.FS, "kiosk.html")),
	}

	if o.Config.Columns != 0 && !receipt.SetWidth(o.Config.Columns) {
		log.Printf("columns %d is not a printer width; using %d", o.Config.Columns, receipt.Width())
	}
	receipt.SetTextAsImage(o.Config.Text != "font")
	if err := s.st.reloadEvents(); err != nil {
		return nil, fmt.Errorf("events: %w", err)
	}
	if spec := o.Config.Printer; spec != "" {
		if err := s.st.setPrinter(spec); err != nil {
			log.Printf("printer %q unavailable: %v", spec, err)
		} else {
			log.Printf("printer: %s", spec)
		}
	}
	return s, nil
}

// Handler wires every route the device answers on.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.stand(mux)
	s.designerRoutes(mux)
	s.adminRoutes(mux)
	return mux
}

// Terminal is the number printed on every receipt, for the start-up line.
func (s *Server) Terminal() string { return s.st.config().Terminal }

// pick resolves which event a request is about: the one it asked for, the one
// the device is set to, or the first there is.
func (s *Server) pick(r *http.Request) event.Event {
	set := s.st.eventSet()
	if ev, ok := set.Get(r.URL.Query().Get("event")); ok {
		return ev
	}
	if ev, ok := set.Get(s.st.config().Event); ok {
		return ev
	}
	return set.First()
}

// send prints a document and says what happened, in words meant for the person
// standing at the stand rather than for a log. It reports whether it printed.
func (s *Server) send(w http.ResponseWriter, doc *receipt.Doc, what string) bool {
	prn, _ := s.st.printer()
	if prn == nil {
		s.st.note("No printer selected.")
		http.Error(w, "Pick a printer first.", http.StatusPreconditionFailed)
		return false
	}
	job := doc.ESCPOS(s.assets)
	if err := prn.Print(job); err != nil {
		log.Printf("print: %v", err)
		s.st.printFailure(err.Error())
		http.Error(w, err.Error(), http.StatusBadGateway)
		return false
	}
	s.st.counted()
	fmt.Fprintf(w, "Sent %s, %d bytes to %s.", what, len(job), prn.Name())
	return true
}

// printed records that a roast's receipt came out. Only the roast service
// keeps records; demo roasts have none to update.
func (s *Server) printed(code string) {
	src, err := s.source()
	if err != nil {
		return
	}
	remote, ok := src.(roast.Remote)
	if !ok {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := remote.Printed(ctx, code); err != nil {
			log.Printf("record print of %s: %v", code, err)
		}
	}()
}

// guard is the code check. Everything an operator can reach goes through it,
// because between them these routes can choose a printer, join a network and
// reboot the machine, and the stand sits in a room full of strangers.
func (s *Server) guard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pin := s.st.config().AdminPIN
		if pin != "" && r.Header.Get("X-Admin-Pin") != pin {
			http.Error(w, "Wrong code.", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

func (s *Server) post(h http.HandlerFunc) http.HandlerFunc {
	return s.guard(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "post only", http.StatusMethodNotAllowed)
			return
		}
		h(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func all(s *event.Set) []event.Event {
	var out []event.Event
	for _, c := range s.Codes() {
		if ev, ok := s.Get(c); ok {
			out = append(out, ev)
		}
	}
	return out
}

// sample is the roast the designer lays out against. It is fixed rather than
// random so two people comparing screens see the same receipt.
func sample() roast.Roast {
	r := roast.Sample("nmbrthirteen")
	r.Code = "7k2f9"
	return r
}

func textMode() string {
	if receipt.TextAsImage() {
		return "image"
	}
	return "font"
}

// testSlip exercises the three things a printer can silently fail at: raster
// printing, which carries every line of text unless the stand is set to the
// printer's own font, the drawn fonts, and the native QR command.
func testSlip(spec string, ev event.Event) *receipt.Doc {
	center := receipt.Style{Align: receipt.AlignCenter}
	d := &receipt.Doc{}
	d.Add(
		receipt.Feed{Lines: 1},
		receipt.Image{Name: ev.Receipt.Logo},
		receipt.Feed{Lines: 2},
		receipt.Text{
			Value: "Printer test",
			Style: receipt.Style{Align: receipt.AlignCenter, Bold: true, Double: true},
			Bleed: true,
		},
		receipt.Feed{Lines: 2},
		receipt.KV{Label: "Target", Value: spec},
		receipt.KV{Label: "Columns", Value: strconv.Itoa(receipt.Width())},
		receipt.KV{Label: "Letters", Value: textMode()},
		receipt.Feed{Lines: 2},
		receipt.Section{Label: "This line should be reversed"},
		receipt.Feed{Lines: 1},
		receipt.Bar{Label: "Gauge", Value: "60%", Percent: 60},
		receipt.Text{Value: "José Müller — გამარჯობა …"},
		receipt.Text{Value: "Every letter above as written, no question marks."},
		receipt.Feed{Lines: 2},
		receipt.QR{Data: "https://lifeat.upgaming.com", Size: 5},
		receipt.Feed{Lines: 1},
		receipt.Text{Value: "A scannable code above, not a blank.", Style: center},
		receipt.Cut{},
	)
	return d
}
