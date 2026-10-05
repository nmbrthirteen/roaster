package printer

import (
	"net"
	"testing"
)

func TestESCPOSStatusBytes(t *testing.T) {
	// Bit 1 and bit 4 are always set in these replies, so a ready printer is
	// not all zeros.
	const ready = 0x12
	for _, c := range []struct {
		printer, offline, paper byte
		want                    string
	}{
		{ready, ready, ready, ""},
		{ready, ready, ready | 0x60, "Out of paper"},
		{ready, ready | 0x20, ready, "Out of paper"},
		{ready, ready | 0x04, ready, "Cover open"},
		{ready, ready | 0x40, ready, "Printer error"},
		{ready | 0x08, ready, ready, "Printer offline"},
		{ready, ready, ready | 0x0C, ""}, // paper running low still prints
	} {
		if got := escposProblem(c.printer, c.offline, c.paper); got != c.want {
			t.Errorf("escposProblem(%#x, %#x, %#x) = %q, want %q", c.printer, c.offline, c.paper, got, c.want)
		}
	}
}

func TestANetworkPrinterReportsNoPaper(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		ask := make([]byte, 3)
		for _, reply := range []byte{0x12, 0x12, 0x72} {
			if _, err := conn.Read(ask); err != nil {
				return
			}
			conn.Write([]byte{reply})
		}
	}()

	st, err := tcpPrinter{addr: ln.Addr().String()}.Check()
	if err != nil {
		t.Fatal(err)
	}
	if st.Problem != "Out of paper" {
		t.Errorf("problem = %q", st.Problem)
	}
}

func TestANetworkPrinterThatIsGoneSaysSo(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	st, err := tcpPrinter{addr: addr}.Check()
	if err != nil || st.Problem != "Printer not reachable" {
		t.Errorf("got %+v, %v", st, err)
	}
}
