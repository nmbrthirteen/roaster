//go:build !windows

package main

import (
	"log"
	"os/exec"
	"runtime"
)

// show has no window to open away from Windows. The server is running by now,
// so a development machine gets the page in whatever browser it has and the
// process stays up to keep supervising.
func show(s session) error {
	open := "xdg-open"
	if runtime.GOOS == "darwin" {
		open = "open"
	}
	if err := exec.Command(open, s.url).Start(); err != nil {
		log.Printf("open %s by hand: %v", s.url, err)
	}
	log.Printf("the kiosk window is Windows only; showing %s in a browser", s.url)
	<-s.stop
	return nil
}
