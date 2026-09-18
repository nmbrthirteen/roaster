// Command kiosk is the stand. It is a Windows application that owns the screen
// and draws the kiosk page in its own window, so there is no browser to close,
// no address bar to reach and one icon to open.
//
// It also starts the server it shows and keeps it running, which is why this is
// the only thing anyone has to launch.
//
// Hosting the page rather than driving Edge costs Web Serial: WebView2 has no
// navigator.serial, so a printer is reached through the server's own transports
// instead. On a device running its own server that is the shorter path anyway.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/upgaming/roaster/internal/config"
	"github.com/upgaming/roaster/internal/state"
)

const (
	restartDelay = 3 * time.Second

	// A server that dies this fast died on startup, and it will do it again.
	briefRun = 15 * time.Second

	// Written by the server when an operator leaves through the hidden menu.
	quitFile = ".quit"
)

// session is what the window is asked to show.
type session struct {
	url   string
	title string

	// windowed opens a normal window that closes. The stand runs without it:
	// full screen, always on top, and no way out but the hidden menu.
	windowed bool

	// shell means Windows starts this instead of the desktop, so leaving has to
	// put the desktop back.
	shell bool

	// cursor keeps the mouse pointer. A touchscreen has nothing to point with.
	cursor bool

	stop <-chan struct{}
}

func main() {
	configPath := flag.String("config", "roaster.json", "settings file, read for kioskUrl")
	rawURL := flag.String("url", "", "address to open, overriding the settings file")
	windowed := flag.Bool("windowed", false, "open a window that closes instead of locking the screen")
	preview := flag.Bool("preview", false, "open the receipt designer, in a window, unlocked")
	shell := flag.Bool("shell", false, "this is running as the Windows shell, so leaving starts the desktop")
	cursor := flag.Bool("cursor", false, "keep the mouse pointer on the locked screen")
	flag.Parse()

	logTo("kiosk.log")

	page, title := "/kiosk", "Roaster"
	if *preview {
		page, title = "/preview", "Roaster receipt designer"
		*windowed = true
	}

	target := *rawURL
	if target == "" {
		target = at(readURL(*configPath), page)
	}
	log.Printf("target %s", target)

	stop := make(chan struct{})
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		log.Printf("shutting down")
		close(stop)
	}()

	// The window opens before the server answers on purpose. Waiting first would
	// leave a locked screen black for as long as the wait, and the window has a
	// page of its own to show meanwhile.
	if err := serve(target, stop); err != nil {
		fail("%v", err)
	}

	// Closing the app is a decision an operator makes in the hidden menu, so a
	// leftover marker from last time must not end this run early.
	clearQuit()

	if err := show(session{
		url:      target,
		title:    title,
		windowed: *windowed,
		shell:    *shell,
		cursor:   *cursor,
		stop:     stop,
	}); err != nil {
		fail("%v", err)
	}
}

// serve makes the app answer and keeps it answering. A server someone else is
// already running is left alone, and a remote address has nothing to start.
//
// Only an address that cannot be parsed stops the stand here. Everything else
// is something that may yet come good: a server still booting, venue wifi that
// is not up. The window opens and waits rather than quitting on a locked device
// with nobody in front of it.
func serve(target string, stop <-chan struct{}) error {
	u, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("%q is not a valid address: %w", target, err)
	}
	if alive(health(target), time.Second) {
		log.Printf("server already running")
		return nil
	}
	if !isLoopback(u.Hostname()) {
		log.Printf("%s is not answering yet; waiting for it", target)
		return nil
	}

	server, err := serverPath()
	if err != nil {
		log.Printf("%v; waiting for something else to answer on %s", err, target)
		return nil
	}
	go supervise(server, stop)
	return nil
}

func supervise(server string, stop <-chan struct{}) {
	name := filepath.Base(server)
	brief := 0

	for {
		log.Printf("starting %s", name)
		started := time.Now()
		run(server, nil)
		lasted := time.Since(started)

		// A server that dies on startup will do it again, so the retry slows
		// down. It never stops: a stand has to come back by itself from a
		// failure nobody is standing there to fix.
		wait := restartDelay
		if lasted < briefRun {
			brief++
			if brief > 3 {
				wait = time.Minute
			}
		} else {
			brief = 0
		}

		if askedToQuit() {
			return
		}
		log.Printf("%s exited after %s, starting it again in %s",
			name, lasted.Round(time.Second), wait)
		select {
		case <-stop:
			return
		case <-time.After(wait):
		}
	}
}

func quitPath() string { return state.Path(quitFile) }

func askedToQuit() bool {
	_, err := os.Stat(quitPath())
	return err == nil
}

func clearQuit() { _ = os.Remove(quitPath()) }

func run(name string, args []string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = filepath.Dir(name)
	quiet(cmd)
	if err := cmd.Run(); err != nil {
		log.Printf("%s: %v", filepath.Base(name), err)
	}
}

// at pins the path. The stand must land on the kiosk whatever a stale settings
// file says, and the designer is only ever reached on purpose.
func at(target, page string) string {
	u, err := url.Parse(target)
	if err != nil || u.Host == "" {
		return target
	}
	u.Path, u.RawQuery, u.Fragment = page, "", ""
	return u.String()
}

func health(target string) string {
	u, err := url.Parse(target)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host + "/health"
}

func readURL(path string) string {
	state.Seed(path)
	path = state.Resolve(path)

	cfg, err := config.Load(path)
	if err != nil {
		fail("could not read %s: %v", path, err)
	}
	return cfg.KioskURL
}

func alive(health string, timeout time.Duration) bool {
	if health == "" {
		return false
	}
	client := http.Client{Timeout: timeout}
	res, err := client.Get(health)
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode == http.StatusOK
}

func isLoopback(host string) bool {
	if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func serverPath() (string, error) {
	name := "roaster"
	if runtime.GOOS == "windows" {
		name = "roaster.exe"
	}
	p := filepath.Join(filepath.Dir(mustExe()), name)
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("%s is not beside the launcher", name)
	}
	return p, nil
}

func mustExe() string {
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("could not locate the launcher: %v", err)
	}
	return exe
}

// logTo writes to the folder the app keeps its state in, because an application
// started by double click has no console anyone will read.
func logTo(name string) {
	path := state.Path(name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(f)
	log.Printf("--- kiosk starting, %s/%s ---", runtime.GOOS, runtime.GOARCH)
	log.Printf("state: %s", state.Dir())
}

func fail(format string, args ...any) {
	log.Printf(format, args...)
	os.Exit(1)
}
