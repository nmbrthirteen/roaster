//go:build windows

package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/jchv/go-webview2/pkg/edge"

	"github.com/upgaming/roaster/internal/state"
)

// The application icon, in the window, on the taskbar and in Alt+Tab. It is
// embedded so a device holding one file still looks like an application.
//
//go:embed icon.ico
var appIcon []byte

const (
	className    = "RoasterKiosk"
	instanceName = `Local\RoasterKiosk`

	tickID    = 1
	tickEvery = 1000 // ms
)

// An ordinary window: it can be moved, minimised, alt-tabbed and closed. Kiosk
// mode is Windows' job, and it does it to whatever application it is given.
// Nothing here tries to hold the screen.
var (
	live       *host
	onMessage  = syscall.NewCallback(wndProc)
	registered bool
)

// showing is what the view has been pointed at, so a state that has not changed
// is not navigated again.
type showing int

const (
	nothing showing = iota
	waitingPage
	targetPage
)

type host struct {
	s    session
	hwnd uintptr
	view *edge.Chromium
	done chan struct{}

	up   atomic.Bool // the server answers
	page showing     // written from the window thread only
}

// show opens the window and returns when it closes.
func show(s session) error {
	runtime.LockOSThread()

	if !onlyOne(instanceName, className) {
		log.Printf("already running; brought the open one to the front")
		return nil
	}

	h := &host{s: s, done: make(chan struct{})}
	return h.run()
}

func (h *host) run() error {
	live = h
	defer func() { live = nil }()

	instance, _, _ := getModuleHandle.Call(0)
	if err := h.register(instance); err != nil {
		return err
	}
	if err := h.open(instance); err != nil {
		return err
	}
	defer h.tidy()

	h.view = edge.NewChromium()
	h.view.DataPath = state.Path("kiosk-data")
	h.view.SetGlobalPermission(edge.CoreWebView2PermissionStateDeny)

	if !h.view.Embed(h.hwnd) {
		return fmt.Errorf("the view did not start; install the Microsoft Edge WebView2 Runtime on this device")
	}
	h.view.Resize()
	h.tighten()

	// Whichever is true right now, rather than a blank window for a second.
	if alive(health(h.s.url), time.Second) {
		h.up.Store(true)
		h.navigate()
	} else {
		h.hold()
	}

	setTimer.Call(h.hwnd, tickID, tickEvery, 0)
	go h.watchServer()

	h.pump()
	close(h.done)
	return nil
}

func (h *host) register(instance uintptr) error {
	if registered {
		return nil
	}
	cursor, _, _ := loadCursor.Call(0, 32512) // IDC_ARROW
	brush, _, _ := createSolidBrush.Call(0x00070707)

	class := wndClassEx{
		size:       uint32(unsafe.Sizeof(wndClassEx{})),
		wndProc:    onMessage,
		instance:   instance,
		icon:       appIcons(256),
		iconSm:     appIcons(32),
		cursor:     cursor,
		background: brush,
		className:  utf16(className),
	}
	if r, _, err := registerClassEx.Call(uintptr(unsafe.Pointer(&class))); r == 0 {
		return fmt.Errorf("could not register the window class: %w", err)
	}
	registered = true
	return nil
}

func (h *host) open(instance uintptr) error {
	// The stand fills the screen and has no title bar, because a visitor has no
	// use for one. It is still an ordinary window underneath: Alt+Tab reaches
	// it, Alt+F4 closes it, and nothing holds it in front of anything else.
	at := screen()
	style := uintptr(wsPopup | wsClipChildren)
	x, y := at.monitor.left, at.monitor.top
	w := at.monitor.right - at.monitor.left
	height := at.monitor.bottom - at.monitor.top

	if h.s.window {
		style = wsOverlappedWin | wsClipChildren
		w, height = 1360, 900
		x = at.work.left + (at.work.right-at.work.left-w)/2
		y = at.work.top + (at.work.bottom-at.work.top-height)/2
	}

	hwnd, _, err := createWindowEx.Call(
		wsExAppWindow,
		uintptr(unsafe.Pointer(utf16(className))),
		uintptr(unsafe.Pointer(utf16(h.s.title))),
		style,
		uintptr(x), uintptr(y), uintptr(w), uintptr(height),
		0, 0, instance, 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("could not open the window: %w", err)
	}
	h.hwnd = hwnd

	sendMessage.Call(hwnd, wmSetIcon, iconBig, appIcons(256))
	sendMessage.Call(hwnd, wmSetIcon, iconSmall, appIcons(32))

	showWindow.Call(hwnd, swShow)
	updateWindow.Call(hwnd)
	return nil
}

// tighten turns off what a page in a browser gets and an application does not:
// the context menu, developer tools, the status bar, zoom by keys, wheel or
// pinch, swipe back and forward, browser shortcuts such as F5 and Ctrl+P, and
// the error page that would explain the server being down in Microsoft's words
// rather than ours.
func (h *host) tighten() {
	settings, err := h.view.GetSettings()
	if err != nil {
		log.Printf("settings: %v", err)
		return
	}
	for what, err := range map[string]error{
		"context menus": settings.PutAreDefaultContextMenusEnabled(false),
		"dev tools":     settings.PutAreDevToolsEnabled(false),
		"status bar":    settings.PutIsStatusBarEnabled(false),
		"zoom":          settings.PutIsZoomControlEnabled(false),
		"pinch zoom":    settings.PutIsPinchZoomEnabled(false),
		"swipe back":    settings.PutIsSwipeNavigationEnabled(false),
		"browser keys":  settings.PutAreBrowserAcceleratorKeysEnabled(false),
		"error page":    settings.PutIsBuiltInErrorPageEnabled(false),
	} {
		if err != nil {
			log.Printf("could not turn off %s: %v", what, err)
		}
	}
}

func (h *host) pump() {
	var m msg
	for {
		r, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if r == 0 || int32(r) == -1 {
			return
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func (h *host) tidy() {
	if h.hwnd != 0 {
		killTimer.Call(h.hwnd, tickID)
		destroyWindow.Call(h.hwnd)
		h.hwnd = 0
	}
}

// tick watches two things and nothing else: an operator leaving through the
// hidden menu, and the server coming or going. The page looks after itself once
// it is up; it polls the server and reloads on its own.
func (h *host) tick() {
	if askedToQuit() || stopped(h.s.stop) {
		h.leave()
		return
	}
	switch {
	case h.up.Load() && h.page != targetPage:
		h.navigate()
	case !h.up.Load() && h.page != waitingPage:
		h.hold()
	}
}

func (h *host) navigate() {
	log.Printf("opening %s", h.s.url)
	h.page = targetPage
	h.view.Navigate(h.s.url)
}

// hold says the server is not answering yet, on brand, instead of leaving a
// window empty.
func (h *host) hold() {
	h.page = waitingPage
	h.view.NavigateToString(waiting)
}

// watchServer keeps up to date with whether there is anything to show. Three
// misses rather than one, so a restart from the hidden menu does not flash the
// waiting page over a page that is about to come back.
func (h *host) watchServer() {
	url := health(h.s.url)
	misses := 0

	for {
		if alive(url, 2*time.Second) {
			misses = 0
			h.up.Store(true)
		} else if misses++; misses >= 3 {
			h.up.Store(false)
		}

		select {
		case <-h.done:
			return
		case <-h.s.stop:
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// leave closes the app. When Windows starts this instead of the desktop,
// leaving has to put the desktop back or there is nothing there.
func (h *host) leave() {
	if h.s.shell {
		log.Printf("starting the desktop")
		if err := exec.Command(filepath.Join(os.Getenv("WINDIR"), "explorer.exe")).Start(); err != nil {
			log.Printf("could not start the desktop: %v", err)
		}
	}
	destroyWindow.Call(h.hwnd)
}

func stopped(c <-chan struct{}) bool {
	select {
	case <-c:
		return true
	default:
		return false
	}
}

func wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	h := live
	switch message {
	case wmSize:
		if h != nil && h.view != nil {
			h.view.Resize()
		}
		return 0

	case wmMove:
		if h != nil && h.view != nil {
			h.view.NotifyParentWindowPositionChanged()
		}
		return 0

	case wmSetFocus:
		if h != nil && h.view != nil {
			h.view.Focus()
		}
		return 0

	case wmTimer:
		if h != nil && wParam == tickID {
			h.tick()
		}
		return 0

	case wmDestroy:
		postQuitMessage.Call(0)
		return 0
	}

	r, _, _ := defWindowProc.Call(hwnd, message, wParam, lParam)
	return r
}

// appIcons hands out the window's icons once each. Windows keeps them for the
// life of the process.
var icons = map[int]uintptr{}

func appIcons(size int) uintptr {
	if h, ok := icons[size]; ok {
		return h
	}
	h, err := icon(appIcon, size)
	if err != nil {
		log.Printf("icon at %dpx: %v", size, err)
	}
	icons[size] = h
	return h
}
