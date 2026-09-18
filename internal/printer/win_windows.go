//go:build windows

package printer

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Windows has no lp. The RAW datatype is what tells the spooler to hand the
// bytes over untouched.
var (
	winspool             = syscall.NewLazyDLL("winspool.drv")
	procOpenPrinter      = winspool.NewProc("OpenPrinterW")
	procClosePrinter     = winspool.NewProc("ClosePrinter")
	procStartDocPrinter  = winspool.NewProc("StartDocPrinterW")
	procEndDocPrinter    = winspool.NewProc("EndDocPrinter")
	procStartPagePrinter = winspool.NewProc("StartPagePrinter")
	procEndPagePrinter   = winspool.NewProc("EndPagePrinter")
	procWritePrinter     = winspool.NewProc("WritePrinter")
)

type docInfo1 struct {
	DocName    *uint16
	OutputFile *uint16
	Datatype   *uint16
}

type winPrinter struct{ queue string }

func (p winPrinter) Name() string { return "windows " + p.queue }

func (p winPrinter) Print(job []byte) error {
	name, err := syscall.UTF16PtrFromString(p.queue)
	if err != nil {
		return err
	}
	docName, _ := syscall.UTF16PtrFromString("Upgaming receipt")
	datatype, _ := syscall.UTF16PtrFromString("RAW")

	var h syscall.Handle
	if r, _, err := procOpenPrinter.Call(
		uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&h)), 0); r == 0 {
		return fmt.Errorf("open printer %q: %w", p.queue, err)
	}
	defer procClosePrinter.Call(uintptr(h))

	info := docInfo1{DocName: docName, Datatype: datatype}
	if r, _, err := procStartDocPrinter.Call(
		uintptr(h), 1, uintptr(unsafe.Pointer(&info))); r == 0 {
		return fmt.Errorf("start document: %w", err)
	}
	defer procEndDocPrinter.Call(uintptr(h))

	if r, _, err := procStartPagePrinter.Call(uintptr(h)); r == 0 {
		return fmt.Errorf("start page: %w", err)
	}
	defer procEndPagePrinter.Call(uintptr(h))

	var written uint32
	if r, _, err := procWritePrinter.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&job[0])),
		uintptr(len(job)),
		uintptr(unsafe.Pointer(&written)),
	); r == 0 {
		return fmt.Errorf("write to printer: %w", err)
	}
	if int(written) != len(job) {
		return fmt.Errorf("printer took %d of %d bytes", written, len(job))
	}
	return nil
}

func openWindows(queue string) (Printer, error) { return winPrinter{queue: queue}, nil }
