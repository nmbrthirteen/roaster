package server

import (
	"context"
	"log"
	"time"

	"github.com/upgaming/roaster/internal/netconf"
)

// WatchNetwork looks at whether a roast could go through right now, so the
// hidden menu shows a hotel sign-in that lapsed before staff go hunting. It
// asks the roast service's own health route, which costs nothing upstream,
// and looks closer only when that fails.
func (s *Server) WatchNetwork(ctx context.Context) {
	bad := 0
	for {
		state := s.lookAtNetwork(ctx)
		if old := s.netState.Swap(&state); old == nil || *old != state {
			log.Printf("network: %q", state)
		}
		// Nobody stands at the stand. Two misses a minute apart is a network
		// that has stopped, not a blip, so the stand tries to fix it itself.
		if state == "signin" || state == "offline" {
			if bad++; bad >= 2 {
				s.heal(ctx)
			}
		} else {
			bad = 0
		}
		wait := 5 * time.Minute
		if state != "" {
			wait = time.Minute
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// heal rejoins the current network, then tries each network the stand has
// joined before, and keeps the first one roasts can go through on. It never
// joins a network nobody added.
func (s *Server) heal(ctx context.Context) {
	log.Printf("network: rejoining")
	if err := netconf.Rejoin(); err != nil {
		log.Printf("network: rejoin: %v", err)
	} else if s.lookAtNetwork(ctx) == "" {
		log.Printf("network: back after rejoining")
		return
	}
	for _, ssid := range netconf.Known() {
		log.Printf("network: trying %s", ssid)
		if err := netconf.Connect(ssid, ""); err != nil {
			log.Printf("network: %s: %v", ssid, err)
			continue
		}
		if s.lookAtNetwork(ctx) == "" {
			log.Printf("network: back on %s", ssid)
			return
		}
	}
}

// lookAtNetwork is "" when roasts can go through, and otherwise "signin",
// "offline" or "service".
func (s *Server) lookAtNetwork(ctx context.Context) string {
	if s.st.config().Provider != "remote" {
		return ""
	}
	if _, err := s.checkService(ctx); err == nil {
		return ""
	}
	switch netconf.CheckInternet(ctx).State {
	case "signin":
		return "signin"
	case "offline":
		return "offline"
	}
	return "service"
}

// networkNotes is what the hidden menu says about each state. Only staff
// see it; the visitor's screen stays as it is.
var networkNotes = map[string]string{
	"signin":  "No internet: this Wi-Fi wants its sign-in page again. Press Check internet.",
	"offline": "No internet. Join another Wi-Fi.",
	"service": "The roast service is not answering. Press Test the connection.",
}

func (s *Server) network() string {
	if p := s.netState.Load(); p != nil {
		return *p
	}
	return ""
}
