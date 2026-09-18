package printer

import (
	"os/exec"
	"runtime"
	"strings"
)

// Candidate is one printer the UI can offer.
type Candidate struct {
	Spec  string `json:"spec"`  // what to pass to Open
	Label string `json:"label"` // what to show in the selector
}

// Discover lists the print queues the operating system already knows about.
// Networked printers are not discoverable this way and are entered by address,
// which is the better option anyway: tcp:host:9100 is the same string on every
// machine, so moving the kiosk to another device changes nothing.
func Discover() []Candidate {
	var names []string
	scheme := "lp:"
	switch runtime.GOOS {
	case "windows":
		scheme = "win:"
		names = run("powershell", "-NoProfile", "-Command", "Get-Printer | Select-Object -ExpandProperty Name")
	default:
		// lpstat -a prints "QUEUE accepting requests since ...".
		for _, line := range run("lpstat", "-a") {
			if f := strings.Fields(line); len(f) > 0 {
				names = append(names, f[0])
			}
		}
	}

	out := make([]Candidate, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		out = append(out, Candidate{Spec: scheme + n, Label: strings.ReplaceAll(n, "_", " ")})
	}
	return out
}

func run(name string, args ...string) []string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n")
}
