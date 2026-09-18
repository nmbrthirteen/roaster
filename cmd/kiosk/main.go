// Command kiosk is the double-clickable launcher for the stand.
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
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/upgaming/roaster/internal/config"
)

func main() {
	configPath := flag.String("config", "roaster.json", "settings file, read for kioskUrl")
	target := flag.String("url", "", "address to open, overriding the settings file")
	locked := flag.Bool("locked", false, "use Edge kiosk mode instead of an app window")
	flag.Parse()

	if *target == "" {
		*target = readURL(*configPath)
	}

	// Whoever double clicked this expects a working stand, not an error about a
	// server they did not know they had to start.
	if err := ensureServing(*target); err != nil {
		fatal("%v", err)
	}

	browser, err := findEdge()
	if err != nil {
		fatal("%v", err)
	}

	// Its own profile gives the stand a separate window, taskbar identity and
	// Web Serial grant from the user's browser.
	profile := filepath.Join(filepath.Dir(mustExe()), "kiosk-profile")

	args := []string{
		"--user-data-dir=" + profile,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-features=msEdgeIdentityFeatures,msSmartScreenProtection",

		// On a touchscreen a swipe would navigate away and a pinch would zoom,
		// and a kiosk has no way back.
		"--overscroll-history-navigation=0",
		"--disable-pinch",
	}
	if *locked {
		// Kiosk mode: no window controls at all, for the stand itself.
		args = append(args, "--kiosk", *target, "--edge-kiosk-type=fullscreen",
			"--kiosk-idle-timeout-minutes=0")
	} else {
		// A standalone window with no browser UI.
		args = append(args, "--app="+*target, "--start-fullscreen")
	}

	cmd := exec.Command(browser, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr

	log.Printf("opening %s in %s", *target, filepath.Base(browser))
	if err := cmd.Run(); err != nil {
		fatal("browser exited: %v", err)
	}
}

func mustExe() string {
	exe, err := os.Executable()
	if err != nil {
		fatal("could not locate the launcher: %v", err)
	}
	return exe
}

func readURL(path string) string {
	if !filepath.IsAbs(path) {
		if _, err := os.Stat(path); err != nil {
			if exe, err := os.Executable(); err == nil {
				path = filepath.Join(filepath.Dir(exe), path)
			}
		}
	}
	cfg, err := config.Load(path)
	if err != nil {
		fatal("could not read %s: %v", path, err)
	}
	return cfg.KioskURL
}

// ensureServing waits for the app to answer, starting it if it is local and
// nothing is listening.
func ensureServing(target string) error {
	u, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("kioskUrl %q is not a valid address: %w", target, err)
	}
	health := u.Scheme + "://" + u.Host + "/health"

	if alive(health, 800*time.Millisecond) {
		return nil
	}
	if !isLocal(u.Hostname()) {
		return fmt.Errorf("%s is not answering, and it is not a local address this can start", target)
	}

	server, err := serverPath()
	if err != nil {
		return err
	}
	log.Printf("server is not running, starting %s", filepath.Base(server))

	cmd := exec.Command(server)
	cmd.Dir = filepath.Dir(server)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start %s: %w", server, err)
	}

	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if alive(health, time.Second) {
			log.Printf("server is up")
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("started %s but it never answered on %s; see roaster.log",
		filepath.Base(server), health)
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

func isLocal(host string) bool {
	if host == "localhost" || strings.EqualFold(host, "localhost.") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func serverPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	name := "roaster"
	if runtime.GOOS == "windows" {
		name = "roaster.exe"
	}
	p := filepath.Join(filepath.Dir(exe), name)
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("could not find %s beside the launcher", name)
	}
	return p, nil
}

func findEdge() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("the kiosk launcher is for Windows; open the address in a browser instead")
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

// fatal reports to a log file as well as the console, because a launcher run by
// double click has no console anyone will read.
func fatal(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if exe, err := os.Executable(); err == nil {
		path := filepath.Join(filepath.Dir(exe), "kiosk.log")
		if f, ferr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); ferr == nil {
			fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), msg)
			f.Close()
		}
	}
	log.Fatal(msg)
}
