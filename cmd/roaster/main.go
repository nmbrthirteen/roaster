// Command roaster runs the kiosk. For now it serves the receipt designer, which
// renders the exact document the printer receives.
package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/upgaming/roaster/internal/config"
	"github.com/upgaming/roaster/internal/event"
	"github.com/upgaming/roaster/internal/printer"
	"github.com/upgaming/roaster/internal/receipt"
	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/ui"
)

// station holds the settings a device can change while running. Choosing a
// printer in the UI has to survive a restart, so every change writes the file
// back rather than living in memory.
type station struct {
	mu     sync.RWMutex
	cfg    config.Config // what is running, flags included
	saved  config.Config // what the file says; flags must not leak into it
	path   string
	prn    printer.Printer
	events *event.Set
}

func (s *station) config() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *station) eventSet() *event.Set {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.events
}

// reloadEvents re-reads the events directory so a file written by the designer
// takes effect without a restart.
func (s *station) reloadEvents() error {
	set, err := event.Load(s.config().EventsDir)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.events = set
	s.mu.Unlock()
	return nil
}

// setEvent chooses which event the kiosk opens on, and remembers it.
func (s *station) setEvent(code string) error {
	if _, ok := s.eventSet().Get(code); !ok {
		return fmt.Errorf("no event %q", code)
	}
	s.mu.Lock()
	s.cfg.Event, s.saved.Event = code, code
	saved, path := s.saved, s.path
	s.mu.Unlock()
	return config.Save(path, saved)
}

func (s *station) printer() (printer.Printer, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.prn, s.cfg.Printer
}

func (s *station) setPrinter(spec string) error {
	var p printer.Printer
	if spec != "" {
		opened, err := printer.Open(spec)
		if err != nil {
			return err
		}
		p = opened
	}

	s.mu.Lock()
	s.prn = p
	s.cfg.Printer, s.saved.Printer = spec, spec
	saved, path := s.saved, s.path
	s.mu.Unlock()

	return config.Save(path, saved)
}

// startLog mirrors the log to a file beside the executable. A kiosk binary is
// launched by double click, so anything written to a console window is gone
// before anyone can read it.
func startLog() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	path := filepath.Join(filepath.Dir(exe), "roaster.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("could not open %s: %v", path, err)
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.Printf("--- roaster starting, %s/%s ---", runtime.GOOS, runtime.GOARCH)
}

func main() {
	startLog()

	configPath := flag.String("config", "roaster.json", "per-device settings file")
	flag.String("addr", ":3000", "listen address")
	flag.String("events", "events", "directory of event files, overriding the built-in ones")
	flag.String("terminal", "001", "terminal number printed on every receipt")
	flag.String("printer", "", "printer: tcp:host:9100, lp:queue or file:path")
	flag.String("event", "", "event code the kiosk opens on")
	flag.Parse()

	beside(configPath)

	saved, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	cfg := saved
	config.ApplyFlags(&cfg, flag.CommandLine)

	st := &station{cfg: cfg, saved: saved, path: *configPath}
	if cfg.Printer != "" {
		if err := st.setPrinter(cfg.Printer); err != nil {
			// A printer that has been unplugged must not stop the kiosk booting.
			log.Printf("printer %q unavailable: %v", cfg.Printer, err)
		} else {
			log.Printf("printer: %s", cfg.Printer)
		}
	}

	assets, err := receipt.LoadAssets(ui.FS, "assets")
	if err != nil {
		log.Printf("assets: %v (receipts will print without the logo)", err)
	}

	beside(&st.cfg.EventsDir)
	beside(&st.saved.EventsDir)

	if err := st.reloadEvents(); err != nil {
		log.Fatalf("events: %v", err)
	}

	tpl := template.Must(template.ParseFS(ui.FS, "preview.html"))

	pick := func(r *http.Request) event.Event {
		set := st.eventSet()
		if ev, ok := set.Get(r.URL.Query().Get("event")); ok {
			return ev
		}
		if ev, ok := set.Get(st.config().Event); ok {
			return ev
		}
		return set.First()
	}

	mux := http.NewServeMux()
	mux.Handle("/assets/", http.FileServer(http.FS(ui.FS)))
	mux.HandleFunc("/qr", handleQR)

	mux.HandleFunc("/preview", func(w http.ResponseWriter, r *http.Request) {
		ev := pick(r)
		doc := sample().Doc(ev, st.config().Terminal)
		_, spec := st.printer()

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
			Receipt:     template.HTML(doc.HTML(assets)),
			Width:       receipt.Width,
			LineCount:   len(doc.Lines()),
			Millimetres: strconv.Itoa(int(doc.EstimateHeightMM(assets))),
			Bytes:       len(doc.ESCPOS(assets)),
			Event:       ev,
			Events:      all(st.eventSet()),
			Printer:     spec,
			Printers:    printer.Discover(),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tpl.Execute(w, data); err != nil {
			log.Printf("preview: %v", err)
		}
	})

	mux.HandleFunc("/preview.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(sample().Doc(pick(r), st.config().Terminal).Plain()))
	})

	mux.HandleFunc("/preview.bin", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(sample().Doc(pick(r), st.config().Terminal).ESCPOS(assets))
	})

	// Raw bytes for the browser to write over Web Serial. This is what lets the
	// kiosk be a hosted page with nothing installed on the device.
	mux.HandleFunc("/test.bin", func(w http.ResponseWriter, r *http.Request) {
		_, spec := st.printer()
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(testSlip(spec, pick(r)).ESCPOS(assets))
	})

	// Choosing a printer persists to the settings file, so a device is set up
	// once by hand and never again.
	mux.HandleFunc("/printer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "post only", http.StatusMethodNotAllowed)
			return
		}
		spec := r.FormValue("spec")
		if err := st.setPrinter(spec); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if spec == "" {
			fmt.Fprint(w, "No printer selected.")
			return
		}
		fmt.Fprintf(w, "Saved. Printing to %s.", spec)
	})

	send := func(w http.ResponseWriter, doc *receipt.Doc, what string) {
		prn, _ := st.printer()
		if prn == nil {
			http.Error(w, "Pick a printer first.", http.StatusPreconditionFailed)
			return
		}
		job := doc.ESCPOS(assets)
		if err := prn.Print(job); err != nil {
			log.Printf("print: %v", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		fmt.Fprintf(w, "Sent %s, %d bytes to %s.", what, len(job), prn.Name())
	}

	mux.HandleFunc("/print", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "post only", http.StatusMethodNotAllowed)
			return
		}
		send(w, sample().Doc(pick(r), st.config().Terminal), "receipt")
	})

	mux.HandleFunc("/print/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "post only", http.StatusMethodNotAllowed)
			return
		}
		_, spec := st.printer()
		send(w, testSlip(spec, pick(r)), "test slip")
	})

	// Selecting an event in the designer is what the kiosk opens on next boot.
	mux.HandleFunc("/event", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "post only", http.StatusMethodNotAllowed)
			return
		}
		if err := st.setEvent(r.FormValue("code")); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, "Kiosk now opens on %s.", r.FormValue("code"))
	})

	// Creating an event writes the file and reloads, so a new conference is a
	// form rather than a deploy.
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "post only", http.StatusMethodNotAllowed)
			return
		}
		base, _ := st.eventSet().Get(r.FormValue("from"))
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

		if err := event.Write(st.config().EventsDir, ev); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		beside(&st.cfg.EventsDir)
	beside(&st.saved.EventsDir)

	if err := st.reloadEvents(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "Created %s.json", ev.Code)
	})

	// Health is what a kiosk browser polls to decide the app is alive. Android
	// kills background processes, so something has to notice and reload.
	started := time.Now()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_, spec := st.printer()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"uptimeSeconds":%d,"printer":%q,"event":%q,"terminal":%q}`+"\n",
			int(time.Since(started).Seconds()), spec, st.config().Event, st.config().Terminal)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/preview", http.StatusFound)
	})

	log.Printf("roaster listening on %s, terminal #%s", cfg.Addr, st.config().Terminal)
	log.Fatal(http.ListenAndServe(cfg.Addr, mux))
}

// testSlip exercises the three features a printer can silently fail at: the
// reversed bar, the PC437 block glyphs, and the native QR command. It costs
// about 90mm of paper instead of a full receipt.
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
		receipt.KV{Label: "Columns", Value: strconv.Itoa(receipt.Width)},
		receipt.Feed{Lines: 2},
		receipt.Section{Label: "This line should be reversed"},
		receipt.Feed{Lines: 1},
		receipt.Bar{Label: "Gauge glyphs", Value: "60%", Percent: 60},
		receipt.Text{Value: "Solid blocks above, not question marks."},
		receipt.Feed{Lines: 2},
		receipt.QR{Data: "https://lifeat.upgaming.com", Size: 5},
		receipt.Feed{Lines: 1},
		receipt.Text{Value: "A scannable code above, not a blank.", Style: center},
		receipt.Cut{},
	)
	return d
}

// beside resolves a relative path against the executable's folder. Double
// clicking in Explorer can hand the process any working directory it likes, so
// settings and event files are found next to the binary instead.
func beside(path *string) {
	if *path == "" || filepath.IsAbs(*path) {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	*path = filepath.Join(filepath.Dir(exe), *path)
}

// handleQR renders a QR as a bitmap. The preview uses it for fidelity, and it
// doubles as the fallback for printers that lack the native QR command.
func handleQR(w http.ResponseWriter, r *http.Request) {
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

func all(s *event.Set) []event.Event {
	var out []event.Event
	for _, c := range s.Codes() {
		if ev, ok := s.Get(c); ok {
			out = append(out, ev)
		}
	}
	return out
}

func pct(n int) *int { return &n }

// sample is the fixture the designer renders until the GitHub adapter lands.
func sample() roast.Roast {
	return roast.Roast{
		Code:     "7k2f9",
		Handle:   "nmbrthirteen",
		At:       time.Now(),
		Score:    "89 / 100",
		ScoreTag: "critical",

		// Every label states what was counted, so nobody has to ask what the
		// number means. Each one comes from a single GitHub call: public events
		// give commit timestamps and messages, the repo list gives descriptions
		// and push dates. Higher is always worse, which is what makes one
		// legend line enough for the whole block.
		Metrics: []roast.Metric{
			{Label: "Commits after midnight", Value: "34%", Tag: "owl", Percent: pct(34)},
			{Label: "Friday deploys", Value: "14%", Tag: "reckless", Percent: pct(14)},
			{Label: "Repos with no description", Value: "71%", Percent: pct(71)},
			{Label: "One-word commit messages", Value: "62%", Tag: "terse", Percent: pct(62)},
			{Label: "Longest gap between commits", Value: "214 days"},
		},
		Verdict: "Your architecture diagram looks like a bowl of spaghetti dropped on AWS. " +
			"Upgaming gives you a 12% survival rate in production.",

		// Decimal odds, not American. Upgaming's market reads 7.50, not +650,
		// and a stranger at a booth should not need a glossary.
		Odds: []roast.Odd{
			{Label: "You survive a prod crash", Price: "7.50"},
			{Label: "A Friday ship goes unnoticed", Price: "13.00"},
			{Label: "You blame a junior", Price: "1.25", Tag: "sure thing"},
		},
		Hiring: "6 open roles match your stack",
	}
}
