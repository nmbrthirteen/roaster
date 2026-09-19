package server

import (
	"fmt"
	"sync"

	"github.com/upgaming/roaster/internal/config"
	"github.com/upgaming/roaster/internal/event"
	"github.com/upgaming/roaster/internal/printer"
	"github.com/upgaming/roaster/internal/roast"
)

// station is everything one stand knows about itself. Handlers read it through
// these methods and never touch the fields, so the lock is in one place.
type station struct {
	mu     sync.RWMutex
	cfg    config.Config // what is running, flags included
	saved  config.Config // what the file says; flags must not leak into it
	path   string
	prn    printer.Printer
	events *event.Set

	// Finished roasts, kept so the kiosk can ask for the printable bytes after
	// the audit without sending the whole document back and forth.
	roasts map[string]roast.Roast

	printed   int
	lastError string
	lastRoast *roast.Roast
}

// note records the last thing that went wrong, so the hidden menu can show it
// without anyone reading a log file on a locked machine.
func (s *station) note(msg string) {
	s.mu.Lock()
	s.lastError = msg
	s.mu.Unlock()
}

func (s *station) counted() {
	s.mu.Lock()
	s.printed++
	s.lastError = ""
	s.mu.Unlock()
}

func (s *station) stats() (int, string, *roast.Roast) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.printed, s.lastError, s.lastRoast
}

func (s *station) setTerminal(v string) error {
	s.mu.Lock()
	s.cfg.Terminal, s.saved.Terminal = v, v
	saved, path := s.saved, s.path
	s.mu.Unlock()
	return config.Save(path, saved)
}

func (s *station) setProvider(v string) error {
	if v != "demo" && v != "remote" {
		return fmt.Errorf("provider must be demo or remote")
	}
	s.mu.Lock()
	s.cfg.Provider, s.saved.Provider = v, v
	saved, path := s.saved, s.path
	s.mu.Unlock()
	return config.Save(path, saved)
}

func (s *station) keep(r roast.Roast) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.roasts == nil {
		s.roasts = map[string]roast.Roast{}
	}
	s.roasts[r.Code] = r
	s.lastRoast = &r
}

func (s *station) recall(code string) (roast.Roast, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.roasts[code]
	return r, ok
}

func (s *station) config() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *station) eventSet() *event.Set {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.events
}

func (s *station) reloadEvents() error {
	set, err := event.Load(s.config().EventsDir)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.events = set
	s.mu.Unlock()
	return nil
}

func (s *station) setEvent(code string) error {
	if _, ok := s.eventSet().Get(code); !ok {
		return fmt.Errorf("no event %q", code)
	}
	s.mu.Lock()
	s.cfg.Event, s.saved.Event = code, code
	saved, path := s.saved, s.path
	s.mu.Unlock()
	return config.Save(path, saved)
}

func (s *station) setEventsDir(dir string) {
	s.mu.Lock()
	s.cfg.EventsDir, s.saved.EventsDir = dir, dir
	s.mu.Unlock()
}

func (s *station) printer() (printer.Printer, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.prn, s.cfg.Printer
}

func (s *station) setPrinter(spec string) error {
	var p printer.Printer
	if spec != "" {
		opened, err := printer.Open(spec)
		if err != nil {
			return err
		}
		p = opened
	}

	s.mu.Lock()
	s.prn = p
	s.cfg.Printer, s.saved.Printer = spec, spec
	saved, path := s.saved, s.path
	s.mu.Unlock()

	return config.Save(path, saved)
}
