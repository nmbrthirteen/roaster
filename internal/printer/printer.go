// Package printer sends a finished ESC/POS job to hardware. The transport is
// chosen by a spec string so the same binary drives a networked printer at an
// event, a USB queue on a desk, and a file during development.
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

// Open builds a printer from a spec:
//
//	tcp:192.168.1.50:9100   a networked ESC/POS printer, the event setup
//	usb:/dev/usb/lp0        a USB printer on Linux, via the usblp character device
//	win:Receipt             a Windows spooler queue, raw datatype
//	lp:Receipt              a CUPS queue on macOS or Linux
//	file:/tmp/job.bin       write the bytes to disk, for development
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

// tcpPrinter talks raw ESC/POS on port 9100. A fresh connection per job means a
// printer that was unplugged between receipts recovers on the next one instead
// of wedging the queue.
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

// lpPrinter hands the bytes to the OS spooler untouched. -o raw is what stops
// the driver reinterpreting ESC/POS as text.
// usbPrinter writes to the character device the Linux usblp driver creates for
// a USB printer. No spooler, no driver, no permission dialog: the kernel has
// already claimed the device and hands over a file you can write ESC/POS to.
// Android has no equivalent, which is the whole reason a small Linux host
// earns its place in the stand.
type usbPrinter struct{ dev string }

func (p usbPrinter) Name() string { return "usb " + p.dev }

func (p usbPrinter) Print(job []byte) error {
	// Opened write-only with no create and no truncate: this is a device node,
	// not a file, and the usual file flags do the wrong thing to it.
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
