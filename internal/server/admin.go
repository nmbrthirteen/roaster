package server

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/upgaming/roaster/internal/event"
	"github.com/upgaming/roaster/internal/netconf"
	"github.com/upgaming/roaster/internal/printer"
	"github.com/upgaming/roaster/internal/state"
)

// quitFile tells the launcher an operator asked to leave, so the supervisor
// stands down instead of reopening the stand.
const quitFile = ".quit"

// The hidden menu. A device locked to this one app still has to be
// serviceable, so everything an operator would otherwise open Windows for lives
// here: the printer, the network, the event, a reprint, a reboot.

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

func (s *Server) adminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/admin/state", s.guard(s.adminState))
	mux.HandleFunc("/admin/settings", s.post(s.adminSettings))
	mux.HandleFunc("/admin/reprint", s.post(s.adminReprint))
	mux.HandleFunc("/admin/wifi/scan", s.guard(s.wifiScan))
	mux.HandleFunc("/admin/wifi/connect", s.post(s.wifiConnect))
	mux.HandleFunc("/admin/restart", s.post(s.adminRestart))
	mux.HandleFunc("/admin/quit", s.post(s.adminQuit))
	mux.HandleFunc("/admin/reboot", s.post(s.adminReboot))
	mux.HandleFunc("/admin/shutdown", s.post(s.adminShutdown))
}

func (s *Server) adminState(w http.ResponseWriter, r *http.Request) {
	cfg := s.st.config()
	_, spec := s.st.printer()
	printed, lastErr, last := s.st.stats()
	wifi, _ := netconf.Current()

	writeJSON(w, adminState{
		Terminal:  cfg.Terminal,
		EventCode: s.pick(r).Code,
		Events:    all(s.st.eventSet()),
		Printer:   spec,
		Printers:  printer.Discover(),
		Provider:  cfg.Provider,
		Wifi:      wifi,
		Uptime:    int(time.Since(s.started).Seconds()),
		Printed:   printed,
		LastError: lastErr,
		Reprint:   last != nil,
		Platform:  runtime.GOOS,
	})
}

func (s *Server) adminSettings(w http.ResponseWriter, r *http.Request) {
	if v := r.FormValue("terminal"); v != "" {
		if err := s.st.setTerminal(v); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if v := r.FormValue("provider"); v != "" {
		if err := s.st.setProvider(v); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	fmt.Fprint(w, "Saved.")
}

// reprint matters more than it sounds. Paper jams, and the person whose roast
// it was is still standing there.
func (s *Server) adminReprint(w http.ResponseWriter, r *http.Request) {
	_, _, last := s.st.stats()
	if last == nil {
		http.Error(w, "Nothing has been printed yet.", http.StatusNotFound)
		return
	}
	s.send(w, last.Doc(s.pick(r), s.st.config().Terminal), "reprint")
}

func (s *Server) wifiScan(w http.ResponseWriter, r *http.Request) {
	networks, err := netconf.Scan()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, networks)
}

func (s *Server) wifiConnect(w http.ResponseWriter, r *http.Request) {
	if err := netconf.Connect(r.FormValue("ssid"), r.FormValue("password")); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	fmt.Fprintf(w, "Joining %s.", r.FormValue("ssid"))
}

func (s *Server) adminRestart(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Restarting.")
	go exitSoon()
}

func (s *Server) adminQuit(w http.ResponseWriter, r *http.Request) {
	if err := touchQuit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, "Closing.")
	go exitSoon()
}

func (s *Server) adminReboot(w http.ResponseWriter, r *http.Request) {
	s.power(w, "/r", "Rebooting.")
}

func (s *Server) adminShutdown(w http.ResponseWriter, r *http.Request) {
	s.power(w, "/s", "Shutting down.")
}

func (s *Server) power(w http.ResponseWriter, flag, message string) {
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

// exitSoon lets the reply reach the browser before the process goes away. The
// launcher restarts it, which is the point.
func exitSoon() {
	time.Sleep(400 * time.Millisecond)
	os.Exit(0)
}

func touchQuit() error {
	return os.WriteFile(state.Path(quitFile), []byte("quit\n"), 0o644)
}
