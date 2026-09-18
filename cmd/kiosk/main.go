// Command kiosk runs the stand. It starts the server, keeps it alive, and holds
// the app open, so it is the only thing anyone has to launch.
//
// It drives real Edge rather than embedding a WebView2 control on purpose.
// Printing runs over Web Serial, which Edge supports and an embedded webview
// does not reliably, and real Edge picks up the enterprise policy that grants
// the printer without a prompt.
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
)

const (
	restartDelay = 3 * time.Second
	startupWait  = 30 * time.Second
)

func main() {
	configPath := flag.String("config", "roaster.json", "settings file, read for kioskUrl")
	rawURL := flag.String("url", "", "address to open, overriding the settings file")
	windowed := flag.Bool("windowed", false, "open an app window instead of locking the screen")
	once := flag.Bool("once", false, "exit when the app is closed instead of reopening it")
	flag.Parse()

	logTo("kiosk.log")

	target := *rawURL
	if target == "" {
		target = forceKiosk(readURL(*configPath))
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

	if err := serve(target, stop); err != nil {
		fail("%v", err)
	}

	browser, err := findEdge()
	if err != nil {
		fail("%v", err)
	}

	// Closing the app must not end the stand. Somebody will do it by accident.
	for {
		run(browser, browserArgs(target, *windowed))
		if *once {
			return
		}
		select {
		case <-stop:
			return
		case <-time.After(2 * time.Second):
			log.Printf("app closed, reopening")
		}
	}
}

// serve makes the app answer and keeps it answering. A server someone else is
// already running is left alone, and a remote address has nothing to start.
func serve(target string, stop <-chan struct{}) error {
	u, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("%q is not a valid address: %w", target, err)
	}
	health := u.Scheme + "://" + u.Host + "/health"

	if alive(health, time.Second) {
		log.Printf("server already running")
		return nil
	}
	if !isLoopback(u.Hostname()) {
		return fmt.Errorf("%s is not answering and is not an address this can start", target)
	}

	server, err := serverPath()
	if err != nil {
		return err
	}
	go supervise(server, stop)

	deadline := time.Now().Add(startupWait)
	for time.Now().Before(deadline) {
		if alive(health, time.Second) {
			log.Printf("server is up")
			return nil
		}
		time.Sleep(400 * time.Millisecond)
	}
	return fmt.Errorf("%s never answered on %s; see roaster.log", filepath.Base(server), health)
}

func supervise(server string, stop <-chan struct{}) {
	for {
		log.Printf("starting %s", filepath.Base(server))
		run(server, nil)

		select {
		case <-stop:
			return
		case <-time.After(restartDelay):
			log.Printf("server exited, restarting")
		}
	}
}

func run(name string, args []string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = filepath.Dir(name)
	if err := cmd.Run(); err != nil {
		log.Printf("%s: %v", filepath.Base(name), err)
	}
}

func browserArgs(target string, windowed bool) []string {
	// Its own profile gives the stand a separate window, taskbar identity and
	// Web Serial grant from the user's browser.
	args := []string{
		"--user-data-dir=" + filepath.Join(filepath.Dir(mustExe()), "kiosk-profile"),
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-session-crashed-bubble",
		"--hide-crash-restore-bubble",

		// On a touchscreen a swipe would navigate away and a pinch would zoom,
		// and a locked kiosk has no way back.
		"--overscroll-history-navigation=0",
		"--disable-pinch",
	}
	if windowed {
		return append(args, "--app="+target, "--start-fullscreen")
	}
	return append(args,
		"--kiosk", target,
		"--edge-kiosk-type=fullscreen",
		"--kiosk-idle-timeout-minutes=0",
	)
}

// forceKiosk pins the path. The designer is a tool, and the stand must never
// open it, whatever a stale settings file says.
func forceKiosk(target string) string {
	u, err := url.Parse(target)
	if err != nil || u.Host == "" {
		return target
	}
	u.Path, u.RawQuery, u.Fragment = "/kiosk", "", ""
	return u.String()
}

func readURL(path string) string {
	if !filepath.IsAbs(path) {
		if _, err := os.Stat(path); err != nil {
			path = filepath.Join(filepath.Dir(mustExe()), path)
		}
	}
	cfg, err := config.Load(path)
	if err != nil {
		fail("could not read %s: %v", path, err)
	}
	return cfg.KioskURL
}

func alive(health string, timeout time.Duration) bool {
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

func findEdge() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("the launcher is for Windows; open the address in a browser instead")
	}
	for _, c := range []string{
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
	} {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	if p, err := exec.LookPath("msedge.exe"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("could not find Microsoft Edge in the usual places")
}

func mustExe() string {
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("could not locate the launcher: %v", err)
	}
	return exe
}

// logTo writes beside the executable, because a launcher started by double
// click has no console anyone will read.
func logTo(name string) {
	path := filepath.Join(filepath.Dir(mustExe()), name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(f)
	log.Printf("--- kiosk starting, %s/%s ---", runtime.GOOS, runtime.GOARCH)
}

func fail(format string, args ...any) {
	log.Printf(format, args...)
	os.Exit(1)
}
