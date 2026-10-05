package printer

import (
	"fmt"
	"net"
	"time"
)

// Status is what a printer says about itself. Problem is empty when it is
// ready, and Waiting counts jobs it has been sent and not yet printed.
type Status struct {
	Problem string
	Waiting int
}

// Checker is a printer that can be asked how it is. Not every transport can
// answer: a device node and lp only take bytes.
type Checker interface {
	Check() (Status, error)
}

// ESC/POS real time status, DLE EOT n. Each answers one byte.
var (
	askPrinter = []byte{0x10, 0x04, 1}
	askOffline = []byte{0x10, 0x04, 2}
	askPaper   = []byte{0x10, 0x04, 4}
)

func (p tcpPrinter) Check() (Status, error) {
	conn, err := net.DialTimeout("tcp", p.addr, 2*time.Second)
	if err != nil {
		return Status{Problem: "Printer not reachable"}, nil
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return Status{}, err
	}

	var got [3]byte
	for i, ask := range [][]byte{askPrinter, askOffline, askPaper} {
		if _, err := conn.Write(ask); err != nil {
			return Status{}, err
		}
		if _, err := conn.Read(got[i : i+1]); err != nil {
			// A printer that takes jobs but keeps quiet about itself.
			return Status{}, fmt.Errorf("no status from %s: %w", p.addr, err)
		}
	}
	return Status{Problem: escposProblem(got[0], got[1], got[2])}, nil
}

func escposProblem(printer, offline, paper byte) string {
	switch {
	case paper&0x60 != 0 || offline&0x20 != 0:
		return "Out of paper"
	case offline&0x04 != 0:
		return "Cover open"
	case offline&0x40 != 0:
		return "Printer error"
	case printer&0x08 != 0:
		return "Printer offline"
	}
	return ""
}
