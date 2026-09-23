package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/upgaming/roaster/internal/event"
	"github.com/upgaming/roaster/internal/netconf"
	"github.com/upgaming/roaster/internal/printer"
	"github.com/upgaming/roaster/internal/receipt"
	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/secret"
	"github.com/upgaming/roaster/internal/state"
	"github.com/upgaming/roaster/internal/update"
	"github.com/upgaming/roaster/internal/version"
)

// quitFile tells the launcher an operator asked to leave, so the supervisor
// stands down instead of reopening the stand.
const quitFile = ".quit"

// The hidden menu. A device locked to this one app still has to be
// serviceable, so everything an operator would otherwise open Windows for lives
// here: the printer, the network, the event, a reprint, a reboot.

type adminState struct {
	Terminal  string              `json:"terminal"`
	Pack      string              `json:"pack"`
	Packs     []roast.Pack        `json:"packs"`
	EventCode string              `json:"event"`
	Events    []event.Event       `json:"events"`
	Printer   string              `json:"printer"`
	Columns   int                 `json:"columns"`
	Printers  []printer.Candidate `json:"printers"`
	Provider  string              `json:"provider"`
	RemoteURL string              `json:"remoteUrl"`
	TokenSet  bool                `json:"tokenSet"`
	Problems  []string            `json:"problems"`
	Wifi      netconf.Status      `json:"wifi"`
	Uptime    int                 `json:"uptimeSeconds"`
	Printed   int                 `json:"printed"`
	LastError string              `json:"lastError,omitempty"`
	Reprint   bool                `json:"canReprint"`
	Platform  string              `json:"platform"`
	Version   string              `json:"version"`
}

func (s *Server) adminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/admin/state", s.guard(s.adminState))
	mux.HandleFunc("/admin/settings", s.post(s.adminSettings))
	mux.HandleFunc("/admin/service", s.guard(s.adminService))
	mux.HandleFunc("/admin/reprint", s.post(s.adminReprint))
	mux.HandleFunc("/admin/wifi/scan", s.guard(s.wifiScan))
	mux.HandleFunc("/admin/wifi/connect", s.post(s.wifiConnect))
	mux.HandleFunc("/admin/wifi/forget", s.post(s.wifiForget))
	mux.HandleFunc("/admin/restart", s.post(s.adminRestart))
	mux.HandleFunc("/admin/quit", s.post(s.adminQuit))
	mux.HandleFunc("/admin/reboot", s.post(s.adminReboot))
	mux.HandleFunc("/admin/shutdown", s.post(s.adminShutdown))
	mux.HandleFunc("/admin/signout", s.post(s.adminSignOut))
	mux.HandleFunc("/admin/update", s.guard(s.adminUpdate))
}

func (s *Server) adminState(w http.ResponseWriter, r *http.Request) {
	cfg := s.st.config()
	_, spec := s.st.printer()
	printed, lastErr, last := s.st.stats()
	wifi, _ := netconf.Current()

	writeJSON(w, adminState{
		Terminal:  cfg.Terminal,
		Pack:      roast.PackFor(cfg.Pack).Key,
		Packs:     roast.All(),
		EventCode: s.pick(r).Code,
		Events:    all(s.st.eventSet()),
		Printer:   spec,
		Columns:   receipt.Width(),
		Printers:  printer.Discover(),
		Provider:  cfg.Provider,
		RemoteURL: cfg.RemoteURL,
		TokenSet:  tokenSet(),
		Problems:  s.problemList(),
		Wifi:      wifi,
		Uptime:    int(time.Since(s.started).Seconds()),
		Printed:   printed,
		LastError: lastErr,
		Reprint:   last != nil,
		Platform:  runtime.GOOS,
		Version:   version.Version,
	})
}

func (s *Server) adminSettings(w http.ResponseWriter, r *http.Request) {
	if v := r.FormValue("terminal"); v != "" {
		if err := s.st.setTerminal(v); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if v := r.FormValue("pack"); v != "" {
		if err := s.st.setPack(v); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if v := r.FormValue("columns"); v != "" {
		n, _ := strconv.Atoi(v)
		if err := s.st.setColumns(n); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if v := r.FormValue("remoteUrl"); v != "" {
		if err := s.st.setRemoteURL(strings.TrimSpace(v)); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if v := r.FormValue("token"); v != "" {
		if err := secret.Store(v); err != nil {
			http.Error(w, "Could not store the token: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if v := r.FormValue("adminPin"); v != "" {
		if err := s.st.setPIN(v); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
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

// adminService tests the roast source as the settings stand now.
func (s *Server) adminService(w http.ResponseWriter, r *http.Request) {
	msg, err := s.checkService(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	fmt.Fprint(w, msg)
}

// problemList is checked fresh each time the menu opens, so a fix clears its
// warning without a restart.
func (s *Server) problemList() []string {
	var out []string
	if !s.st.settingsReadable() {
		out = append(out, "The settings file is damaged, so the stand is running on defaults. Saving any setting here rewrites it.")
	}
	for _, f := range s.st.eventSet().Skipped {
		out = append(out, "Event file skipped: "+f)
	}
	if _, err := s.source(); err != nil {
		out = append(out, err.Error())
	}
	return out
}

func tokenSet() bool {
	t, err := secret.Load()
	return err == nil && t != ""
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
	fmt.Fprintf(w, "Connected to %s.", r.FormValue("ssid"))
}

func (s *Server) wifiForget(w http.ResponseWriter, r *http.Request) {
	if err := netconf.Forget(r.FormValue("ssid")); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	fmt.Fprintf(w, "Forgot %s.", r.FormValue("ssid"))
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

// adminSignOut ends the kiosk account's session, which leaves the device on
// the Windows sign-in screen. An administrator signs in there with no keyboard
// shortcut, and a restart brings the stand back.
func (s *Server) adminSignOut(w http.ResponseWriter, r *http.Request) {
	if runtime.GOOS != "windows" {
		http.Error(w, "Only wired up for Windows.", http.StatusNotImplemented)
		return
	}
	// shutdown /l takes no delay, so the reply goes first.
	fmt.Fprint(w, "Signing out.")
	go func() {
		time.Sleep(400 * time.Millisecond)
		if err := exec.Command("shutdown", "/l").Run(); err != nil {
			s.st.note("Sign out: " + err.Error())
		}
	}()
}

var errNotWindows = errors.New("only wired up for Windows")

func (s *Server) power(w http.ResponseWriter, flag, message string) {
	if err := shutdown(flag); err != nil {
		if errors.Is(err, errNotWindows) {
			http.Error(w, "Only wired up for Windows.", http.StatusNotImplemented)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	fmt.Fprint(w, message)
}

// shutdown is the one place that calls Windows' own shutdown.exe, so a
// reboot asked for through the Power section and one forced by installing
// kiosk.exe run the same command.
func shutdown(flag string) error {
	if runtime.GOOS != "windows" {
		return errNotWindows
	}
	return exec.Command("shutdown", flag, "/t", "0").Start()
}

// adminUpdate answers to both a GET, checking for a release, and a POST,
// installing it. The hidden menu already gates both behind the same code, and
// a check and an install differ in nothing but whether they touch disk.
func (s *Server) adminUpdate(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.adminUpdateCheck(w, r)
	case http.MethodPost:
		s.adminUpdateApply(w, r)
	default:
		http.Error(w, "get or post only", http.StatusMethodNotAllowed)
	}
}

func (s *Server) adminUpdateCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	rel, err := update.Check(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, struct {
		Current   string `json:"current"`
		Latest    string `json:"latest"`
		Available bool   `json:"available"`
	}{
		Current:   version.Version,
		Latest:    rel.Tag,
		Available: rel.Newer,
	})
}

func (s *Server) adminUpdateApply(w http.ResponseWriter, r *http.Request) {
	checkCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	rel, err := update.Check(checkCtx)
	cancel()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if !rel.Newer {
		fmt.Fprintf(w, "Already on %s.", version.Version)
		return
	}

	exe, err := os.Executable()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	applyCtx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	res, err := update.Apply(applyCtx, rel, filepath.Dir(exe))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if res.NeedsReboot {
		if err := shutdown("/r"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "Updated to %s. Rebooting.", res.Version)
		return
	}
	fmt.Fprintf(w, "Updated to %s. Restarting.", res.Version)
	go exitSoon()
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
