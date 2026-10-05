package server

import (
	"log"
	"time"

	"github.com/upgaming/roaster/internal/printer"
)

// A receipt takes a second or two to leave the queue. One still there after
// this has stuck behind a printer that is not taking it.
const stuckAfter = 20 * time.Second

// printerProblem is the one line the menu shows when the printer needs a hand,
// and an empty one when it does not.
func (s *Server) printerProblem(now time.Time) string {
	prn, spec := s.st.printer()
	if spec == "" {
		return "No printer selected"
	}
	if prn == nil {
		return "Printer not available"
	}

	if c, ok := prn.(printer.Checker); ok {
		st, err := c.Check()
		if err != nil {
			log.Printf("printer status: %v", err)
		}
		if st.Problem != "" {
			return st.Problem
		}
		if s.stuck(st.Waiting, now) {
			return "Receipt stuck in the print queue"
		}
	}

	if s.st.lastPrintFailed() {
		return "Last receipt did not print"
	}
	return ""
}

func (s *Server) stuck(waiting int, now time.Time) bool {
	if waiting == 0 {
		s.queued.Store(0)
		return false
	}
	s.queued.CompareAndSwap(0, now.UnixNano())
	return now.Sub(time.Unix(0, s.queued.Load())) >= stuckAfter
}
