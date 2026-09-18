//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"
)

// Just enough of the Win32 API to own an ordinary window. The WebView2 binding
// brings its own copy, but it keeps it internal, so this is the kiosk's.
var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")

	registerClassEx     = user32.NewProc("RegisterClassExW")
	createWindowEx      = user32.NewProc("CreateWindowExW")
	destroyWindow       = user32.NewProc("DestroyWindow")
	defWindowProc       = user32.NewProc("DefWindowProcW")
	showWindow          = user32.NewProc("ShowWindow")
	updateWindow        = user32.NewProc("UpdateWindow")
	setForegroundWindow = user32.NewProc("SetForegroundWindow")
	getMessage          = user32.NewProc("GetMessageW")
	translateMessage    = user32.NewProc("TranslateMessage")
	dispatchMessage     = user32.NewProc("DispatchMessageW")
	postQuitMessage     = user32.NewProc("PostQuitMessage")
	sendMessage         = user32.NewProc("SendMessageW")
	loadCursor          = user32.NewProc("LoadCursorW")
	setTimer            = user32.NewProc("SetTimer")
	killTimer           = user32.NewProc("KillTimer")
	monitorFromPoint    = user32.NewProc("MonitorFromPoint")
	getMonitorInfo      = user32.NewProc("GetMonitorInfoW")
	findWindow          = user32.NewProc("FindWindowW")
	createIconFromRes   = user32.NewProc("CreateIconFromResourceEx")
	getModuleHandle     = kernel32.NewProc("GetModuleHandleW")
	createMutex         = kernel32.NewProc("CreateMutexW")
	createSolidBrush    = gdi32.NewProc("CreateSolidBrush")
)

const (
	wsPopup         = 0x80000000
	wsClipChildren  = 0x02000000
	wsOverlappedWin = 0x00CF0000
	wsExAppWindow   = 0x00040000

	swShow = 5

	wmDestroy  = 0x0002
	wmMove     = 0x0003
	wmSize     = 0x0005
	wmSetFocus = 0x0007
	wmSetIcon  = 0x0080
	wmTimer    = 0x0113

	iconSmall = 0
	iconBig   = 1

	monitorPrimary = 0x00000001

	errorAlreadyExists = 183
)

type rect struct{ left, top, right, bottom int32 }

type point struct{ x, y int32 }

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSm     uintptr
}

type monitorInfo struct {
	size    uint32
	monitor rect
	work    rect
	flags   uint32
}

func utf16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}
	return p
}

// screen returns the primary monitor: the whole of it, which is what the stand
// covers, and the part left over by the taskbar, which is where a window with a
// title bar opens. GetSystemMetrics would answer in the thread's DPI, and this
// one is per-monitor aware.
func screen() monitorInfo {
	h, _, _ := monitorFromPoint.Call(0, 0, monitorPrimary)
	info := monitorInfo{size: uint32(unsafe.Sizeof(monitorInfo{}))}
	if r, _, _ := getMonitorInfo.Call(h, uintptr(unsafe.Pointer(&info))); r == 0 {
		full := rect{0, 0, 1920, 1080}
		return monitorInfo{monitor: full, work: full}
	}
	return info
}

// onlyOne holds a named mutex for the life of the process, so opening the app
// twice brings the first copy forward instead of stacking a second window.
func onlyOne(name, class string) bool {
	_, _, err := createMutex.Call(0, 1, uintptr(unsafe.Pointer(utf16(name))))
	if errno, ok := err.(syscall.Errno); ok && errno == errorAlreadyExists {
		h, _, _ := findWindow.Call(uintptr(unsafe.Pointer(utf16(class))), 0)
		if h == 0 {
			// The other copy is on its way out: it still holds the mutex and
			// its window has gone. Carry on, or a relaunch lands on nothing.
			return true
		}
		setForegroundWindow.Call(h)
		return false
	}
	return true
}

// icon builds an HICON from .ico bytes, picking the size closest to what the
// window asked for. The file is embedded, so there is nothing to find on disk.
func icon(ico []byte, size int) (uintptr, error) {
	if len(ico) < 6 || binary.LittleEndian.Uint16(ico[2:]) != 1 {
		return 0, fmt.Errorf("not an icon file")
	}
	count := int(binary.LittleEndian.Uint16(ico[4:]))
	if count == 0 || len(ico) < 6+count*16 {
		return 0, fmt.Errorf("icon file holds no images")
	}

	best, delta := 0, 1<<30
	for i := range count {
		e := ico[6+i*16:]
		w := int(e[0])
		if w == 0 {
			w = 256
		}
		if d := abs(w - size); d < delta {
			best, delta = i, d
		}
	}

	e := ico[6+best*16:]
	length := binary.LittleEndian.Uint32(e[8:])
	offset := binary.LittleEndian.Uint32(e[12:])
	if int(offset)+int(length) > len(ico) {
		return 0, fmt.Errorf("icon image runs past the end of the file")
	}

	h, _, err := createIconFromRes.Call(
		uintptr(unsafe.Pointer(&ico[offset])),
		uintptr(length),
		1,          // an icon rather than a cursor
		0x00030000, // the resource format every Windows since 95 writes
		uintptr(size),
		uintptr(size),
		0,
	)
	if h == 0 {
		return 0, err
	}
	return h, nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
