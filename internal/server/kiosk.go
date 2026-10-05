package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/upgaming/roaster/internal/event"
	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/ui"
)

// stand is what a visitor reaches: the page, the audit behind it, and the
// receipt that comes out. Nothing here asks for the code, because nothing here
// can change the device.
func (s *Server) stand(mux *http.ServeMux) {
	mux.Handle("/assets/", http.FileServer(http.FS(ui.FS)))
	mux.Handle("/fonts/", http.FileServer(http.FS(ui.FS)))

	// The manifest is what lets a browser install this as an application with
	// its own icon and window rather than a tab.
	mux.HandleFunc("/manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		raw, _ := ui.FS.ReadFile("manifest.webmanifest")
		w.Write(raw)
	})

	mux.HandleFunc("/qr", qr)
	mux.HandleFunc("/kiosk", s.page)
	mux.HandleFunc("/api/roast", s.audit)
	mux.HandleFunc("/receipt.bin", s.receiptBytes)
	mux.HandleFunc("/receipt/print", s.printReceipt)
	mux.HandleFunc("/health", s.health)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/kiosk", http.StatusFound)
	})
}

type kioskPage struct {
	event.Event
	Pack     roast.Pack
	Headline string
	TimeZone string
}

// stockHeadline is the event default. It names GitHub, so a stand set to
// another social swaps in that pack's headline.
const stockHeadline = "Roast your GitHub"

func (s *Server) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	ev := s.pick(r)
	pack := roast.PackFor(s.st.config().Pack)
	headline := ev.Kiosk.Headline
	if headline == stockHeadline {
		headline = pack.Headline
	}
	if err := s.kiosk.Execute(w, kioskPage{Event: ev, Pack: pack, Headline: headline, TimeZone: s.st.config().Zone().String()}); err != nil {
		log.Printf("kiosk: %v", err)
	}
}

// audit streams the roast as it happens. Every stage is sent the moment it
// lands, so the screen fills while the work is still going on rather than
// holding a queue in front of a spinner.
func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	handle := strings.TrimSpace(r.URL.Query().Get("handle"))
	if handle == "" {
		http.Error(w, "missing handle", http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ev := s.pick(r)
	enc := json.NewEncoder(w)
	emit := func(u roast.Update) {
		fmt.Fprint(w, "data: ")
		enc.Encode(u)
		fmt.Fprint(w, "\n")
		flusher.Flush()
	}

	provider, err := s.source()
	if err != nil {
		s.st.note("Roast source: " + err.Error())
		emit(roast.Update{Phase: roast.PhaseError, Error: errNotSetUp.Error()})
		return
	}
	result, err := provider.Roast(r.Context(), roast.Request{
		Handle:        handle,
		Pack:          roast.PackFor(s.st.config().Pack).Key,
		Event:         ev.Code,
		Terminal:      s.st.config().Terminal,
		Offset:        standOffset(s.st.config().Zone()),
		ModelProvider: s.st.config().ModelProvider,
	}, emit)
	if err != nil {
		s.st.note("Last audit: " + err.Error())
		emit(roast.Update{Phase: roast.PhaseError, Error: err.Error()})
		return
	}
	// The stand's own clock, so the receipt prints the time in the room rather
	// than the roast server's zone.
	result.At = time.Now().In(s.st.config().Zone())
	s.st.keep(result)
}

// receiptBytes hands the raw job to a page printing over Web Serial.
func (s *Server) receiptBytes(w http.ResponseWriter, r *http.Request) {
	rst, ok := s.st.recall(r.URL.Query().Get("code"))
	if !ok {
		http.Error(w, "no such roast", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(rst.Doc(s.pick(r), s.st.config().Terminal).ESCPOS(s.assets))
}

// printReceipt is the visitor's own receipt, so it is the one printing route
// with no code on it.
func (s *Server) printReceipt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "post only", http.StatusMethodNotAllowed)
		return
	}
	rst, ok := s.st.recall(r.FormValue("code"))
	if !ok {
		http.Error(w, "no such roast", http.StatusNotFound)
		return
	}
	s.send(w, rst.Doc(s.pick(r), s.st.config().Terminal), "receipt")
}

// health is what the page and the launcher both poll to decide this is alive.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	_, spec := s.st.printer()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"ok":true,"uptimeSeconds":%d,"printer":%q,"event":%q,"terminal":%q}`+"\n",
		int(time.Since(s.started).Seconds()), spec, s.st.config().Event, s.st.config().Terminal)
}

func qr(w http.ResponseWriter, r *http.Request) {
	data := r.URL.Query().Get("d")
	if data == "" {
		http.Error(w, "missing d", http.StatusBadRequest)
		return
	}
	mod, _ := strconv.Atoi(r.URL.Query().Get("m"))
	if mod < 1 {
		mod = 6
	}
	png, err := qrcode.Encode(data, qrcode.Medium, mod*37)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(png)
}

func standOffset(zone *time.Location) int {
	_, off := time.Now().In(zone).Zone()
	return off
}
