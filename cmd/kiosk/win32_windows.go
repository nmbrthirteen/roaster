//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"
)

// Just enough of the Win32 API to own a window. The WebView2 binding brings its
// own copy, but it keeps it internal, so this is the kiosk's.
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
	setWindowPos        = user32.NewProc("SetWindowPos")
	getClientRect       = user32.NewProc("GetClientRect")
	getMessage          = user32.NewProc("GetMessageW")
	translateMessage    = user32.NewProc("TranslateMessage")
	dispatchMessage     = user32.NewProc("DispatchMessageW")
	postQuitMessage     = user32.NewProc("PostQuitMessage")
	postMessage         = user32.NewProc("PostMessageW")
	sendMessage         = user32.NewProc("SendMessageW")
	loadCursor          = user32.NewProc("LoadCursorW")
	showCursor          = user32.NewProc("ShowCursor")
	setTimer            = user32.NewProc("SetTimer")
	killTimer           = user32.NewProc("KillTimer")
	monitorFromPoint    = user32.NewProc("MonitorFromPoint")
	getMonitorInfo      = user32.NewProc("GetMonitorInfoW")
	setWindowsHookEx    = user32.NewProc("SetWindowsHookExW")
	unhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	callNextHookEx      = user32.NewProc("CallNextHookEx")
	getAsyncKeyState    = user32.NewProc("GetAsyncKeyState")
	findWindow          = user32.NewProc("FindWindowW")
	foregroundWindow    = user32.NewProc("GetForegroundWindow")
	windowThreadProcess = user32.NewProc("GetWindowThreadProcessId")
	currentProcessID    = kernel32.NewProc("GetCurrentProcessId")
	createIconFromRes   = user32.NewProc("CreateIconFromResourceEx")
	getModuleHandle     = kernel32.NewProc("GetModuleHandleW")
	setThreadExecState  = kernel32.NewProc("SetThreadExecutionState")
	createMutex         = kernel32.NewProc("CreateMutexW")
	copyMemory          = kernel32.NewProc("RtlMoveMemory")
	createSolidBrush    = gdi32.NewProc("CreateSolidBrush")
)

const (
	wsPopup         = 0x80000000
	wsVisible       = 0x10000000
	wsClipChildren  = 0x02000000
	wsOverlappedWin = 0x00CF0000
	wsExTopmost     = 0x00000008
	wsExAppWindow   = 0x00040000
	cwUseDefault    = 0x80000000
	swShow          = 5
	swHide          = 0

	wmDestroy    = 0x0002
	wmSize       = 0x0005
	wmSetFocus   = 0x0007
	wmClose      = 0x0010
	wmSetIcon    = 0x0080
	wmSysCommand = 0x0112
	wmTimer      = 0x0113
	wmMove       = 0x0003
	wmActivate   = 0x0006

	scScreenSave = 0xF140
	scMonitorOff = 0xF170

	hwndTopmost   = ^uintptr(0) // (HWND)-1
	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoActivate = 0x0010
	swpShow       = 0x0040

	iconSmall = 0
	iconBig   = 1

	monitorPrimary = 0x00000001

	whKeyboardLL = 13
	wmKeyDown    = 0x0100
	wmSysKeyDown = 0x0104
	llAltDown    = 0x20

	vkTab     = 0x09
	vkEscape  = 0x1B
	vkLWin    = 0x5B
	vkRWin    = 0x5C
	vkApps    = 0x5D
	vkF4      = 0x73
	vkControl = 0x11
	vkShift   = 0x10

	esContinuous      = 0x80000000
	esSystemRequired  = 0x00000001
	esDisplayRequired = 0x00000002

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

type kbdLLHook struct {
	vkCode    uint32
	scanCode  uint32
	flags     uint32
	time      uint32
	extraInfo uintptr
}

func utf16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}
	return p
}

// screen returns the primary monitor in pixels. GetSystemMetrics would answer
// in the thread's DPI, and this window is per-monitor aware.
func screen() rect {
	h, _, _ := monitorFromPoint.Call(0, 0, monitorPrimary)
	info := monitorInfo{size: uint32(unsafe.Sizeof(monitorInfo{}))}
	if r, _, _ := getMonitorInfo.Call(h, uintptr(unsafe.Pointer(&info))); r == 0 {
		return rect{0, 0, 1920, 1080}
	}
	return info.monitor
}

// held reports a key being down right now, which is how the hook sees modifiers
// it was not handed.
func held(vk uintptr) bool {
	state, _, _ := getAsyncKeyState.Call(vk)
	return state&0x8000 != 0
}

// awake keeps the screen on for as long as this thread lives. A stand that has
// gone dark is a stand nobody walks up to.
func awake() {
	setThreadExecState.Call(esContinuous | esSystemRequired | esDisplayRequired)
}

func letSleep() { setThreadExecState.Call(esContinuous) }

// onlyOne holds a named mutex for the life of the process, so opening the app
// twice brings the first copy forward instead of stacking a second screen.
func onlyOne(name, class string) bool {
	_, _, err := createMutex.Call(0, 1, uintptr(unsafe.Pointer(utf16(name))))
	if errno, ok := err.(syscall.Errno); ok && errno == errorAlreadyExists {
		if h, _, _ := findWindow.Call(uintptr(unsafe.Pointer(utf16(class))), 0); h != 0 {
			setForegroundWindow.Call(h)
		}
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
