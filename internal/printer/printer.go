// Package printer sends a finished ESC/POS job to hardware.
package printer

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Printer interface {
	Print(job []byte) error
	Name() string
}

// Open builds a printer from a spec: tcp:host:9100, win:queue, lp:queue,
// usb:/dev/usb/lp0 or file:path.
func Open(spec string) (Printer, error) {
	kind, target, ok := strings.Cut(spec, ":")
	if !ok || target == "" {
		return nil, fmt.Errorf("printer spec %q: want tcp:host:port, lp:queue or file:path", spec)
	}
	switch kind {
	case "tcp":
		return tcpPrinter{addr: target}, nil
	case "usb":
		return usbPrinter{dev: target}, nil
	case "win":
		return openWindows(target)
	case "lp":
		return lpPrinter{queue: target}, nil
	case "file":
		return filePrinter{path: target}, nil
	default:
		return nil, fmt.Errorf("unknown printer transport %q", kind)
	}
}

// tcpPrinter opens a fresh connection per job, so a printer unplugged between
// receipts recovers on the next one instead of wedging the queue.
type tcpPrinter struct{ addr string }

func (p tcpPrinter) Name() string { return "tcp " + p.addr }

func (p tcpPrinter) Print(job []byte) error {
	conn, err := net.DialTimeout("tcp", p.addr, 3*time.Second)
	if err != nil {
		return fmt.Errorf("connect %s: %w", p.addr, err)
	}
	defer conn.Close()

	if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	if _, err := conn.Write(job); err != nil {
		return fmt.Errorf("write to %s: %w", p.addr, err)
	}
	return nil
}

// lpPrinter passes -o raw, which stops the driver reinterpreting ESC/POS as text.
type usbPrinter struct{ dev string }

func (p usbPrinter) Name() string { return "usb " + p.dev }

func (p usbPrinter) Print(job []byte) error {
	// A device node, not a file: the usual create and truncate flags do the
	// wrong thing to it.
	f, err := os.OpenFile(p.dev, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", p.dev, err)
	}
	defer f.Close()

	if _, err := f.Write(job); err != nil {
		return fmt.Errorf("write to %s: %w", p.dev, err)
	}
	return nil
}

type lpPrinter struct{ queue string }

func (p lpPrinter) Name() string { return "queue " + p.queue }

func (p lpPrinter) Print(job []byte) error {
	cmd := exec.Command("lp", "-d", p.queue, "-o", "raw", "-")
	cmd.Stdin = strings.NewReader(string(job))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lp: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

type filePrinter struct{ path string }

func (p filePrinter) Name() string { return "file " + p.path }

func (p filePrinter) Print(job []byte) error {
	return os.WriteFile(p.path, job, 0o644)
}
