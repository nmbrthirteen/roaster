package server

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/upgaming/roaster/internal/event"
	"github.com/upgaming/roaster/internal/printer"
	"github.com/upgaming/roaster/internal/receipt"
	"github.com/upgaming/roaster/internal/state"
)

// designerRoutes are the ones an operator uses to lay out a receipt, choose a
// printer and write an event. Everything that changes the device asks for the
// code first.
func (s *Server) designerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/preview", s.preview)
	mux.HandleFunc("/preview.txt", s.previewText)
	mux.HandleFunc("/preview.bin", s.previewBytes)
	mux.HandleFunc("/test.bin", s.testBytes)
	mux.HandleFunc("/api/example", s.example)

	mux.HandleFunc("/printer", s.post(s.choosePrinter))
	mux.HandleFunc("/print", s.post(s.printSample))
	mux.HandleFunc("/print/test", s.post(s.printTest))
	mux.HandleFunc("/event", s.post(s.chooseEvent))
	mux.HandleFunc("/events", s.post(s.createEvent))
}

func (s *Server) preview(w http.ResponseWriter, r *http.Request) {
	ev := s.pick(r)
	doc := sample().Doc(ev, s.st.config().Terminal)
	_, spec := s.st.printer()

	data := struct {
		Receipt     template.HTML
		Width       int
		LineCount   int
		Millimetres string
		Bytes       int
		Event       event.Event
		Events      []event.Event
		Printer     string
		Printers    []printer.Candidate
	}{
		Receipt:     template.HTML(doc.HTML(s.assets)),
		Width:       receipt.Width(),
		LineCount:   len(doc.Lines()),
		Millimetres: strconv.Itoa(int(doc.EstimateHeightMM(s.assets))),
		Bytes:       len(doc.ESCPOS(s.assets)),
		Event:       ev,
		Events:      all(s.st.eventSet()),
		Printer:     spec,
		Printers:    printer.Discover(),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.designer.Execute(w, data); err != nil {
		log.Printf("preview: %v", err)
	}
}

func (s *Server) previewText(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(sample().Doc(s.pick(r), s.st.config().Terminal).Plain()))
}

func (s *Server) previewBytes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(sample().Doc(s.pick(r), s.st.config().Terminal).ESCPOS(s.assets))
}

// testBytes is the test slip as raw bytes, for a page printing it itself.
func (s *Server) testBytes(w http.ResponseWriter, r *http.Request) {
	_, spec := s.st.printer()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(testSlip(spec, s.pick(r)).ESCPOS(s.assets))
}

func (s *Server) example(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, sample())
}

// choosePrinter persists, so a device is set up once by hand and never again.
func (s *Server) choosePrinter(w http.ResponseWriter, r *http.Request) {
	spec := r.FormValue("spec")
	if err := s.st.setPrinter(spec); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if spec == "" {
		fmt.Fprint(w, "No printer selected.")
		return
	}
	fmt.Fprintf(w, "Saved. Printing to %s.", spec)
}

func (s *Server) printSample(w http.ResponseWriter, r *http.Request) {
	s.send(w, sample().Doc(s.pick(r), s.st.config().Terminal), "receipt")
}

func (s *Server) printTest(w http.ResponseWriter, r *http.Request) {
	_, spec := s.st.printer()
	s.send(w, testSlip(spec, s.pick(r)), "test slip")
}

func (s *Server) chooseEvent(w http.ResponseWriter, r *http.Request) {
	if err := s.st.setEvent(r.FormValue("code")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Fprintf(w, "Kiosk now opens on %s.", r.FormValue("code"))
}

func (s *Server) createEvent(w http.ResponseWriter, r *http.Request) {
	// Clone whatever is running when no source is named, so an event made from
	// the menu never comes out missing its branding.
	base, ok := s.st.eventSet().Get(r.FormValue("from"))
	if !ok {
		base = s.pick(r)
	}
	ev := base
	ev.Name = r.FormValue("name")
	ev.Code = event.Slug(ev.Name)
	if ev.Code == "" {
		http.Error(w, "the event needs a name", http.StatusBadRequest)
		return
	}
	if v := r.FormValue("title"); v != "" {
		ev.Receipt.Title = v
	}
	if v := r.FormValue("stub"); v != "" {
		ev.Receipt.Stub = v
	}

	if err := event.Write(s.st.config().EventsDir, ev); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.st.setEventsDir(state.Resolve(s.st.config().EventsDir))

	if err := s.st.reloadEvents(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "Created %s.json", ev.Code)
}
