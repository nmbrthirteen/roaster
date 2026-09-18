// Package event holds per-event branding.
package event

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed defaults/*.json
var defaults embed.FS

// Receipt carries the print-side branding.
type Receipt struct {
	Title     string `json:"title"`     // headline across the top of the paper
	Logo      string `json:"logo"`      // asset name, without the .png
	Rule      string `json:"rule"`      // light divider character
	Heavy     string `json:"heavy"`     // divider for major breaks
	CTA       string `json:"cta"`       // line under the QR
	Stub      string `json:"stub"`      // headline on the tear-off recruiting stub
	ShareBase string `json:"shareBase"` // share code is appended to this
}

// Kiosk carries the screen-side branding.
type Kiosk struct {
	Headline string `json:"headline"`
	Subhead  string `json:"subhead"`
	Accent   string `json:"accent"`
	Surface  string `json:"surface"`
}

type Event struct {
	Code    string   `json:"code"`
	Name    string   `json:"name"`
	Packs   []string `json:"packs"`
	Receipt Receipt  `json:"receipt"`
	Kiosk   Kiosk    `json:"kiosk"`
}

// Normalise fills anything an event is missing. An event assembled from a form
// or a hand-written file must still print a logo and a QR that points
// somewhere, rather than silently losing them.
func (e *Event) Normalise() {
	if e.Receipt.Logo == "" {
		e.Receipt.Logo = "logo"
	}
	if e.Receipt.Title == "" {
		e.Receipt.Title = "Performance review"
	}
	if e.Receipt.CTA == "" {
		e.Receipt.CTA = "Scan for your digital receipt"
	}
	if e.Receipt.ShareBase == "" {
		e.Receipt.ShareBase = "https://lifeat.upgaming.com/k"
	}
	if e.Name == "" {
		e.Name = e.Code
	}
	if len(e.Packs) == 0 {
		e.Packs = []string{"github"}
	}
	if e.Kiosk.Headline == "" {
		e.Kiosk.Headline = "Roast your GitHub"
	}
	if e.Kiosk.Accent == "" {
		e.Kiosk.Accent = "#0fff50"
	}
	if e.Kiosk.Surface == "" {
		e.Kiosk.Surface = "#070707"
	}
}

// RuleChar and HeavyRuleChar fall back sensibly, so a half-filled event file
// still prints rather than dropping its dividers.
func (e Event) RuleChar() rune { return firstRune(e.Receipt.Rule, '-') }

func (e Event) HeavyRuleChar() rune { return firstRune(e.Receipt.Heavy, '=') }

func firstRune(s string, fallback rune) rune {
	if s == "" {
		return fallback
	}
	return []rune(s)[0]
}

// ShareURL builds the address the QR encodes.
func (e Event) ShareURL(code string) string {
	return strings.TrimSuffix(e.Receipt.ShareBase, "/") + "/" + code
}

type Set struct {
	byCode map[string]Event
	order  []string
}

// Load reads every event file in dir, falling back to the events compiled into
// the binary.
func Load(dir string) (*Set, error) {
	s := &Set{byCode: map[string]Event{}}

	if err := s.read(defaults, "defaults"); err != nil {
		return nil, fmt.Errorf("embedded events: %w", err)
	}
	if dir != "" {
		if _, err := os.Stat(dir); err == nil {
			if err := s.read(os.DirFS(dir), "."); err != nil {
				return nil, fmt.Errorf("events in %s: %w", dir, err)
			}
		}
	}
	if len(s.order) == 0 {
		return nil, fmt.Errorf("no events found")
	}
	sort.Strings(s.order)
	return s, nil
}

func (s *Set) read(fsys fs.FS, dir string) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".json" {
			continue
		}
		raw, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		var ev Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			return fmt.Errorf("%s: %w", e.Name(), err)
		}
		if ev.Code == "" {
			ev.Code = strings.TrimSuffix(e.Name(), ".json")
		}
		ev.Normalise()
		if _, seen := s.byCode[ev.Code]; !seen {
			s.order = append(s.order, ev.Code)
		}
		s.byCode[ev.Code] = ev // a file on disk overrides the embedded default
	}
	return nil
}

func (s *Set) Get(code string) (Event, bool) { ev, ok := s.byCode[code]; return ev, ok }
func (s *Set) Codes() []string               { return s.order }
func (s *Set) First() Event                  { return s.byCode[s.order[0]] }

// Write saves an event into dir, where it overrides any built-in of the same
// code.
func Write(dir string, ev Event) error {
	if ev.Code == "" {
		return fmt.Errorf("event needs a code")
	}
	ev.Normalise()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ev.Code+".json"), append(raw, '\n'), 0o644)
}

// Slug turns a display name into a usable file name and event code.
func Slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
