// Package config keeps the per-device settings in a file beside the binary.
package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	// Addr binds to loopback by default. A stand sits on venue wifi, and
	// anything reachable there can drive the printer. Widen it only for a
	// deliberately hosted setup.
	Addr      string `json:"addr"`
	Terminal  string `json:"terminal"`  // printed on every receipt, one per stand
	Printer   string `json:"printer"`   // tcp:host:9100, lp:queue or file:path
	Event     string `json:"event"`     // event code the kiosk opens on
	EventsDir string `json:"eventsDir"` // event files that override the built-in ones
	KioskURL  string `json:"kioskUrl"`  // what the kiosk launcher opens

	// Provider is "demo" for a rehearsal with no credentials, or "remote" to
	// call a service that holds them.
	Provider string `json:"provider"`

	// AdminPIN guards the hidden menu. Change it before an event: the menu can
	// reboot the machine.
	AdminPIN string `json:"adminPin"`

	// RemoteURL is the roast endpoint. The terminal's token lives beside the
	// binary rather than in here, so a settings file carries nothing secret.
	RemoteURL string `json:"remoteUrl"`
}

func Defaults() Config {
	return Config{
		Addr:      "127.0.0.1:3000",
		Terminal:  "001",
		EventsDir: "events",
		KioskURL:  "http://localhost:3000/kiosk",
		Provider:  "demo",
		AdminPIN:  "1379",
	}
}

// Load reads path over the defaults.
func Load(path string) (Config, error) {
	c := Defaults()
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, fmt.Errorf("%s: %w", path, err)
	}
	// A settings file written before the kiosk had its own path would open the
	// designer instead.
	if u, err := url.Parse(c.KioskURL); err == nil && (u.Path == "" || u.Path == "/") {
		c.KioskURL = strings.TrimSuffix(c.KioskURL, "/") + "/kiosk"
	}
	return c, nil
}

// ApplyFlags lets an explicit flag win over the file, so a single odd run does
// not need the file edited and put back.
func ApplyFlags(c *Config, fs *flag.FlagSet) {
	fs.Visit(func(f *flag.Flag) {
		v := f.Value.String()
		switch f.Name {
		case "addr":
			c.Addr = v
		case "terminal":
			c.Terminal = v
		case "printer":
			c.Printer = v
		case "event":
			c.Event = v
		case "events":
			c.EventsDir = v
		}
	})
}

// Save writes the settings back, so a printer chosen in the UI survives a
// restart without anyone editing the file by hand.
func Save(path string, c Config) error {
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}
