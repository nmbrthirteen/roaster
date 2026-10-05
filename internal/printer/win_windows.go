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
	procGetPrinter       = winspool.NewProc("GetPrinterW")
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

// printerInfo2 is PRINTER_INFO_2W: thirteen pointers, then the numbers.
type printerInfo2 struct {
	_          [13]uintptr
	Attributes uint32
	_          [4]uint32 // priority, default priority, start and until time
	Status     uint32
	Jobs       uint32
	_          uint32 // average pages a minute
}

const workOffline = 0x400

// The spooler's status flags, worst first. A driver reports only what the
// printer tells it, so a cheap thermal printer may never say it is out of
// paper; its jobs stay in the queue instead, which Waiting shows.
var spoolerProblems = []struct {
	flag uint32
	say  string
}{
	{0x10, "Out of paper"},
	{0x8, "Paper jam"},
	{0x40, "Paper problem"},
	{0x400000, "Cover open"},
	{0x80, "Printer offline"},
	{0x1000, "Printer not available"},
	{0x1, "Printer paused"},
	{0x100000, "Printer needs attention"},
	{0x2, "Printer error"},
}

func (p winPrinter) Check() (Status, error) {
	name, err := syscall.UTF16PtrFromString(p.queue)
	if err != nil {
		return Status{}, err
	}
	var h syscall.Handle
	if r, _, _ := procOpenPrinter.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&h)), 0); r == 0 {
		return Status{Problem: "Printer not found"}, nil
	}
	defer procClosePrinter.Call(uintptr(h))

	var need uint32
	procGetPrinter.Call(uintptr(h), 2, 0, 0, uintptr(unsafe.Pointer(&need)))
	if need < uint32(unsafe.Sizeof(printerInfo2{})) {
		return Status{}, fmt.Errorf("printer %q gave no status", p.queue)
	}
	buf := make([]byte, need)
	if r, _, err := procGetPrinter.Call(uintptr(h), 2,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(need), uintptr(unsafe.Pointer(&need))); r == 0 {
		return Status{}, fmt.Errorf("printer status: %w", err)
	}
	info := (*printerInfo2)(unsafe.Pointer(&buf[0]))

	st := Status{Waiting: int(info.Jobs)}
	for _, pr := range spoolerProblems {
		if info.Status&pr.flag != 0 {
			st.Problem = pr.say
			return st, nil
		}
	}
	if info.Attributes&workOffline != 0 {
		st.Problem = "Printer set to work offline"
	}
	return st, nil
}

func openWindows(queue string) (Printer, error) { return winPrinter{queue: queue}, nil }
