// Command roaster runs the kiosk.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/upgaming/roaster/internal/config"
	"github.com/upgaming/roaster/internal/event"
	"github.com/upgaming/roaster/internal/printer"
	"github.com/upgaming/roaster/internal/receipt"
	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/secret"
	"github.com/upgaming/roaster/internal/ui"
)

type station struct {
	mu     sync.RWMutex
	cfg    config.Config // what is running, flags included
	saved  config.Config // what the file says; flags must not leak into it
	path   string
	prn    printer.Printer
	events *event.Set

	// Finished roasts, kept so the kiosk can ask for the printable bytes after the
	// audit without sending the whole document back and forth.
	roasts map[string]roast.Roast
}

func (s *station) keep(r roast.Roast) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.roasts == nil {
		s.roasts = map[string]roast.Roast{}
	}
	s.roasts[r.Code] = r
}

func (s *station) recall(code string) (roast.Roast, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.roasts[code]
	return r, ok
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

// startLog mirrors the log to a file beside the executable.
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
	setToken := flag.Bool("set-token", false, "store this terminal's token, read from standard input")
	flag.Parse()

	if *setToken {
		storeToken()
		return
	}

	beside(configPath)

	saved, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	cfg := saved
	config.ApplyFlags(&cfg, flag.CommandLine)

	st := &station{cfg: cfg, saved: saved, path: *configPath, roasts: map[string]roast.Roast{}}

	provider, err := pickProvider(cfg)
	if err != nil {
		log.Fatalf("provider: %v", err)
	}
	log.Printf("roasts: %s", cfg.Provider)
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
	kioskTpl := template.Must(template.ParseFS(ui.FS, "kiosk.html"))

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
	mux.Handle("/fonts/", http.FileServer(http.FS(ui.FS)))

	// The manifest is what lets Edge install this as a real application with its
	// own icon and window, rather than a browser tab.
	mux.HandleFunc("/manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		raw, _ := ui.FS.ReadFile("manifest.webmanifest")
		w.Write(raw)
	})
	mux.HandleFunc("/qr", handleQR)

	mux.HandleFunc("/preview", func(w http.ResponseWriter, r *http.Request) {
		ev := pick(r)
		doc := demoRoast().Doc(ev, st.config().Terminal)
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
		w.Write([]byte(demoRoast().Doc(pick(r), st.config().Terminal).Plain()))
	})

	mux.HandleFunc("/preview.bin", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(demoRoast().Doc(pick(r), st.config().Terminal).ESCPOS(assets))
	})

	// Raw bytes for the browser to write over Web Serial.
	mux.HandleFunc("/test.bin", func(w http.ResponseWriter, r *http.Request) {
		_, spec := st.printer()
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(testSlip(spec, pick(r)).ESCPOS(assets))
	})

	// Choosing a printer persists to the settings file, so a device is set up once
	// by hand and never again.
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
		send(w, demoRoast().Doc(pick(r), st.config().Terminal), "receipt")
	})

	mux.HandleFunc("/print/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "post only", http.StatusMethodNotAllowed)
			return
		}
		_, spec := st.printer()
		send(w, testSlip(spec, pick(r)), "test slip")
	})

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

	mux.HandleFunc("/kiosk", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := kioskTpl.Execute(w, pick(r)); err != nil {
			log.Printf("kiosk: %v", err)
		}
	})

	mux.HandleFunc("/api/roast", func(w http.ResponseWriter, r *http.Request) {
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

		ev := pick(r)
		enc := json.NewEncoder(w)
		emit := func(u roast.Update) {
			fmt.Fprint(w, "data: ")
			enc.Encode(u)
			fmt.Fprint(w, "\n")
			flusher.Flush()
		}

		result, err := provider.Roast(r.Context(), roast.Request{
			Handle: handle,
			Event:  ev.Code,
		}, emit)
		if err != nil {
			emit(roast.Update{Phase: roast.PhaseError, Error: err.Error()})
			return
		}
		st.keep(result)
	})

	mux.HandleFunc("/receipt.bin", func(w http.ResponseWriter, r *http.Request) {
		rst, ok := st.recall(r.URL.Query().Get("code"))
		if !ok {
			http.Error(w, "no such roast", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(rst.Doc(pick(r), st.config().Terminal).ESCPOS(assets))
	})

	mux.HandleFunc("/receipt/print", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "post only", http.StatusMethodNotAllowed)
			return
		}
		rst, ok := st.recall(r.FormValue("code"))
		if !ok {
			http.Error(w, "no such roast", http.StatusNotFound)
			return
		}
		send(w, rst.Doc(pick(r), st.config().Terminal), "receipt")
	})

	mux.HandleFunc("/api/example", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(demoRoast())
	})

	// Health is what a kiosk browser polls to decide the app is alive.
	started := time.Now()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_, spec := st.printer()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"uptimeSeconds":%d,"printer":%q,"event":%q,"terminal":%q}`+"\n",
			int(time.Since(started).Seconds()), spec, st.config().Event, st.config().Terminal)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/kiosk", http.StatusFound)
	})

	log.Printf("roaster listening on %s, terminal #%s", cfg.Addr, st.config().Terminal)
	log.Fatal(http.ListenAndServe(cfg.Addr, mux))
}

// testSlip exercises the three features a printer can silently fail at:
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

// beside falls back to the executable's folder when a relative path is not found
// from the working directory.
func beside(path *string) {
	if *path == "" || filepath.IsAbs(*path) {
		return
	}
	if _, err := os.Stat(*path); err == nil {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	*path = filepath.Join(filepath.Dir(exe), *path)
}

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

func pickProvider(cfg config.Config) (roast.Provider, error) {
	switch cfg.Provider {
	case "", "demo":
		return roast.Demo{}, nil
	case "remote":
		if cfg.RemoteURL == "" {
			return nil, fmt.Errorf("provider is remote but remoteUrl is empty")
		}
		token, err := secret.Load()
		if err != nil {
			return nil, err
		}
		if token == "" {
			return nil, fmt.Errorf("no terminal token stored; run roaster -set-token")
		}
		return roast.Remote{URL: cfg.RemoteURL, Token: token}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q, want demo or remote", cfg.Provider)
	}
}

// storeToken keeps the token off the command line, where it would land in
// shell history and in the process list.
func storeToken() {
	fmt.Print("Terminal token: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		log.Fatalf("could not read the token: %v", err)
	}
	if err := secret.Store(line); err != nil {
		log.Fatalf("could not store the token: %v", err)
	}
	fmt.Println("Stored. It is encrypted to this machine and is not in any settings file.")
}

func demoRoast() roast.Roast {
	r := roast.Sample("nmbrthirteen")
	r.Code = "7k2f9"
	return r
}
