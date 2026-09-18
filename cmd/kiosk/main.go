// Command kiosk is the double-clickable launcher for the stand. It opens the
// roast web app in Microsoft Edge, locked to one full-screen tab.
//
// It launches real Edge rather than embedding a WebView2 control on purpose.
// Printing runs over Web Serial, and Edge supports that where an embedded
// webview does not reliably. Real Edge also picks up the enterprise policy that
// grants the printer without a prompt.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/upgaming/roaster/internal/config"
)

func main() {
	configPath := flag.String("config", "roaster.json", "settings file, read for kioskUrl")
	url := flag.String("url", "", "address to open, overriding the settings file")
	flag.Parse()

	target := *url
	if target == "" {
		path := *configPath
		if !filepath.IsAbs(path) {
			if exe, err := os.Executable(); err == nil {
				path = filepath.Join(filepath.Dir(exe), path)
			}
		}
		cfg, err := config.Load(path)
		if err != nil {
			fatal("could not read %s: %v", path, err)
		}
		target = cfg.KioskURL
	}

	browser, err := findEdge()
	if err != nil {
		fatal("%v", err)
	}

	cmd := exec.Command(browser,
		"--kiosk", target,
		"--edge-kiosk-type=fullscreen",
		"--no-first-run",
		"--no-default-browser-check",
		"--kiosk-idle-timeout-minutes=0",

		// A Surface is a touchscreen. Without these, a swipe navigates the page
		// away and a pinch zooms the receipt, and a kiosk has no way back.
		"--overscroll-history-navigation=0",
		"--disable-pinch",
	)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr

	log.Printf("opening %s in %s", target, filepath.Base(browser))
	if err := cmd.Run(); err != nil {
		fatal("browser exited: %v", err)
	}
}

func findEdge() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("the kiosk launcher is for Windows; open %s in a browser instead", "the URL")
	}
	candidates := []string{
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
	}
	for _, c := range candidates {
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
			fmt.Fprintf(f, "%s\n", msg)
			f.Close()
		}
	}
	log.Fatal(msg)
}
