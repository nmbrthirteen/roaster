package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/upgaming/roaster/internal/event"
	"github.com/upgaming/roaster/internal/netconf"
	"github.com/upgaming/roaster/internal/printer"
	"github.com/upgaming/roaster/internal/receipt"
	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/state"
)

// quitFile tells the launcher an operator asked to leave, so the supervisor
// stands down instead of reopening the stand.
const quitFile = ".quit"

// admin is the hidden menu. A device locked to this one app still has to be
// serviceable, so everything an operator would otherwise open Windows for lives
// here: the printer, the network, the event, a reprint, a reboot.
type admin struct {
	st      *station
	started time.Time
	pick    func(*http.Request) event.Event
	send    func(http.ResponseWriter, *receipt.Doc, string)
}

type adminState struct {
	Terminal  string              `json:"terminal"`
	EventCode string              `json:"event"`
	Events    []event.Event       `json:"events"`
	Printer   string              `json:"printer"`
	Printers  []printer.Candidate `json:"printers"`
	Provider  string              `json:"provider"`
	Wifi      netconf.Status      `json:"wifi"`
	Uptime    int                 `json:"uptimeSeconds"`
	Printed   int                 `json:"printed"`
	LastError string              `json:"lastError,omitempty"`
	Reprint   bool                `json:"canReprint"`
	Platform  string              `json:"platform"`
}

func (a admin) routes(mux *http.ServeMux) {
	// Every route checks the code. The menu can reboot the machine, and the
	// kiosk stands in a room full of strangers.
	guard := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			pin := a.st.config().AdminPIN
			if pin != "" && r.Header.Get("X-Admin-Pin") != pin {
				http.Error(w, "Wrong code.", http.StatusForbidden)
				return
			}
			h(w, r)
		}
	}
	post := func(h http.HandlerFunc) http.HandlerFunc {
		return guard(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "post only", http.StatusMethodNotAllowed)
				return
			}
			h(w, r)
		})
	}

	mux.HandleFunc("/admin/state", guard(a.state))
	mux.HandleFunc("/admin/settings", post(a.settings))
	mux.HandleFunc("/admin/reprint", post(a.reprint))
	mux.HandleFunc("/admin/wifi/scan", guard(a.wifiScan))
	mux.HandleFunc("/admin/wifi/connect", post(a.wifiConnect))
	mux.HandleFunc("/admin/restart", post(a.restart))
	mux.HandleFunc("/admin/quit", post(a.quit))
	mux.HandleFunc("/admin/reboot", post(a.reboot))
	mux.HandleFunc("/admin/shutdown", post(a.shutdown))
}

func (a admin) state(w http.ResponseWriter, r *http.Request) {
	cfg := a.st.config()
	_, spec := a.st.printer()
	printed, lastErr, last := a.st.stats()
	wifi, _ := netconf.Current()

	writeJSON(w, adminState{
		Terminal:  cfg.Terminal,
		EventCode: a.pick(r).Code,
		Events:    all(a.st.eventSet()),
		Printer:   spec,
		Printers:  printer.Discover(),
		Provider:  cfg.Provider,
		Wifi:      wifi,
		Uptime:    int(time.Since(a.started).Seconds()),
		Printed:   printed,
		LastError: lastErr,
		Reprint:   last != nil,
		Platform:  runtime.GOOS,
	})
}

func (a admin) settings(w http.ResponseWriter, r *http.Request) {
	if v := r.FormValue("terminal"); v != "" {
		if err := a.st.setTerminal(v); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if v := r.FormValue("provider"); v != "" {
		if err := a.st.setProvider(v); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	fmt.Fprint(w, "Saved.")
}

// reprint matters more than it sounds. Paper jams, and the person whose roast
// it was is still standing there.
func (a admin) reprint(w http.ResponseWriter, r *http.Request) {
	_, _, last := a.st.stats()
	if last == nil {
		http.Error(w, "Nothing has been printed yet.", http.StatusNotFound)
		return
	}
	a.send(w, last.Doc(a.pick(r), a.st.config().Terminal), "reprint")
}

func (a admin) wifiScan(w http.ResponseWriter, r *http.Request) {
	networks, err := netconf.Scan()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, networks)
}

func (a admin) wifiConnect(w http.ResponseWriter, r *http.Request) {
	if err := netconf.Connect(r.FormValue("ssid"), r.FormValue("password")); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	fmt.Fprintf(w, "Joining %s.", r.FormValue("ssid"))
}

func (a admin) restart(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Restarting.")
	go exitSoon()
}

func (a admin) quit(w http.ResponseWriter, r *http.Request) {
	if err := touchQuit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, "Closing.")
	go exitSoon()
}

func (a admin) reboot(w http.ResponseWriter, r *http.Request) {
	a.power(w, "/r", "Rebooting.")
}

func (a admin) shutdown(w http.ResponseWriter, r *http.Request) {
	a.power(w, "/s", "Shutting down.")
}

func (a admin) power(w http.ResponseWriter, flag, message string) {
	if runtime.GOOS != "windows" {
		http.Error(w, "Only wired up for Windows.", http.StatusNotImplemented)
		return
	}
	if err := exec.Command("shutdown", flag, "/t", "0").Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, message)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// exitSoon lets the reply reach the browser before the process goes away. The
// launcher restarts it, which is the point.
func exitSoon() {
	time.Sleep(400 * time.Millisecond)
	os.Exit(0)
}

func touchQuit() error {
	return os.WriteFile(state.Path(quitFile), []byte("quit\n"), 0o644)
}

var _ = roast.Roast{}
