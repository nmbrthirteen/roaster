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
	"strconv"
	"strings"
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

	// The page beats every five seconds. Four missed beats is a page that has
	// stopped running, whatever the reason.
	beatGap    = 20 * time.Second
	retryEvery = 5 * time.Second

	// No page for this long while the server is answering means the view itself
	// is gone, and only a new window brings it back.
	viewGone = 90 * time.Second

	// A screen nobody has touched for this long is between visitors, which is
	// when a page that has been up for days can be reloaded unnoticed.
	idleEnough   = 5 * time.Minute
	freshenAfter = 12 * time.Hour
)

// live is how the window procedure and the keyboard hook, both called by
// Windows, find the session they belong to. There is one window.
var live *host

// Windows holds on to both of these, so they are made once rather than per
// window: a process only ever gets so many callbacks.
var (
	onMessage = syscall.NewCallback(wndProc)
	onKey     = syscall.NewCallback(hookProc)

	registered bool
)

type host struct {
	s    session
	hwnd uintptr
	view *edge.Chromium
	hook uintptr
	done chan struct{}

	up   atomic.Bool  // the server answers
	beat atomic.Int64 // when the page last reported in
	idle atomic.Int64 // seconds since the screen was last touched
	page atomic.Value // where the beat came from

	// Touched from the window procedure only, which is one thread.
	navAt   time.Time
	shownAt time.Time
	blind   time.Time

	again   bool
	leaving bool

	// managed means Windows is holding the screen for us, which it does for an
	// app assigned to kiosk mode.
	managed bool
}

// show opens the window and holds it until an operator leaves through the
// hidden menu. A view that dies is replaced rather than mourned.
func show(s session) error {
	runtime.LockOSThread()

	if !onlyOne(instanceName, className) {
		log.Printf("already running; brought the open one to the front")
		return nil
	}

	// A stand that has gone dark is a stand nobody walks up to, and this holds
	// whatever the machine's power settings say.
	awake()
	defer letSleep()

	for {
		h := &host{s: s, done: make(chan struct{})}
		again, err := h.run()
		if err != nil {
			// Windows starts the shell again the moment it exits, so failing
			// fast here would be a loop nobody can read. Fail slowly instead.
			if s.shell {
				log.Printf("%v", err)
				time.Sleep(30 * time.Second)
			}
			return err
		}
		if !again {
			return nil
		}
		// As the Windows shell this hands the rebuild to Windows, which starts
		// the shell again: a fresh process rather than a second view beside a
		// dead one.
		if s.shell {
			log.Printf("the view stopped answering; leaving so Windows starts it again")
			return nil
		}
		log.Printf("the view stopped answering; opening it again")
		time.Sleep(2 * time.Second)
	}
}

func (h *host) run() (bool, error) {
	live = h
	defer func() { live = nil }()

	h.managed = packaged()
	if h.managed {
		log.Printf("started from a package; leaving the foreground to Windows")
	}

	instance, _, _ := getModuleHandle.Call(0)
	if err := h.register(instance); err != nil {
		return false, err
	}
	if err := h.open(instance); err != nil {
		return false, err
	}
	defer h.tidy()

	h.view = edge.NewChromium()
	h.view.DataPath = state.Path("kiosk-data")
	h.view.MessageCallback = h.heard
	h.view.SetGlobalPermission(edge.CoreWebView2PermissionStateDeny)
	if !h.s.windowed {
		h.view.AcceleratorKeyCallback = blockedKey
	}

	if !h.view.Embed(h.hwnd) {
		return false, fmt.Errorf("the view did not start; install the Microsoft Edge WebView2 Runtime on this device")
	}
	h.view.Resize()
	h.tighten()
	h.view.Init(boot)
	h.navigate()

	if !h.s.windowed {
		h.hook, _, _ = setWindowsHookEx.Call(whKeyboardLL, onKey, instance, 0)
		if h.hook == 0 {
			// Worth running without: the window still has no way out, it just
			// no longer swallows Alt+Tab.
			log.Printf("could not take the keyboard; system shortcuts stay live")
		}
	}

	setTimer.Call(h.hwnd, tickID, tickEvery, 0)
	go h.watchServer()

	h.pump()
	close(h.done)
	return h.again, nil
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
	style := uintptr(wsPopup | wsVisible | wsClipChildren)
	exStyle := uintptr(wsExTopmost | wsExAppWindow)
	at := screen()

	x, y := at.left, at.top
	w, height := at.right-at.left, at.bottom-at.top

	if h.s.windowed {
		style = wsOverlappedWin | wsVisible | wsClipChildren
		exStyle = wsExAppWindow
		x, y, w, height = x+(w-1360)/2, y+(height-900)/2, 1360, 900
	}

	hwnd, _, err := createWindowEx.Call(
		exStyle,
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
	setForegroundWindow.Call(hwnd)

	if !h.s.windowed && !h.s.cursor {
		showCursor.Call(0)
	}
	return nil
}

// tighten removes everything a browser offers that a stand must not: the
// context menu, developer tools, zoom, and the error page that would tell a
// visitor the server is down in Microsoft's words rather than ours.
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
	if h.hook != 0 {
		unhookWindowsHookEx.Call(h.hook)
		h.hook = 0
	}
	if h.hwnd != 0 {
		killTimer.Call(h.hwnd, tickID)
		destroyWindow.Call(h.hwnd)
		h.hwnd = 0
	}
}

// tick is the whole supervision of the page, once a second on the thread that
// owns the window.
func (h *host) tick() {
	if askedToQuit() || stopped(h.s.stop) {
		h.leave()
		return
	}
	if !h.s.windowed && !h.managed {
		h.keepFront()
	}

	now := time.Now()
	beating := now.Sub(time.Unix(0, h.beat.Load())) < beatGap
	where, _ := h.page.Load().(string)
	showing := beating && !strings.HasPrefix(where, "about:")

	switch {
	case showing:
		h.blind = time.Time{}
		if h.shownAt.IsZero() {
			h.shownAt = now
			log.Printf("the page is up")
		}
		// Days of uptime and a page nobody is looking at: reload it now rather
		// than in front of someone.
		if now.Sub(h.shownAt) > freshenAfter && h.idle.Load() > int64(idleEnough/time.Second) {
			log.Printf("reloading after %s up", now.Sub(h.shownAt).Round(time.Hour))
			h.navigate()
		}

	case h.up.Load():
		if h.blind.IsZero() {
			h.blind = now
		}
		if now.Sub(h.blind) > viewGone {
			log.Printf("no page for %s with the server answering; opening a new window", viewGone)
			h.again = true
			h.close()
			return
		}
		if now.Sub(h.navAt) > retryEvery {
			h.navigate()
		}

	default:
		// The server is not answering yet. Say so on brand instead of leaving
		// the screen black.
		if now.Sub(h.navAt) > retryEvery && !strings.HasPrefix(where, "about:") {
			h.view.NavigateToString(waiting)
			h.navAt = now
		}
	}
}

func (h *host) navigate() {
	h.navAt = time.Now()
	h.shownAt = time.Time{}
	h.view.Navigate(h.s.url)
}

// heard reads the page's heartbeat: how long the screen has been untouched, and
// which page is beating.
func (h *host) heard(message string) {
	rest, ok := strings.CutPrefix(message, "roaster ")
	if !ok {
		return
	}
	idle, where, _ := strings.Cut(rest, " ")
	seconds, err := strconv.ParseInt(idle, 10, 64)
	if err != nil {
		return
	}
	h.beat.Store(time.Now().UnixNano())
	h.idle.Store(seconds)
	h.page.Store(where)
}

func (h *host) watchServer() {
	url := health(h.s.url)
	for {
		h.up.Store(alive(url, 2*time.Second))
		select {
		case <-h.done:
			return
		case <-h.s.stop:
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// keepFront puts the stand back on top when something else takes the screen: an
// update prompt, a driver notice, anything with a window of its own. Windows
// belonging to this application are left alone, because one of them is the
// password box the hidden menu opens.
func (h *host) keepFront() {
	front, _, _ := foregroundWindow.Call()
	if front == 0 || front == h.hwnd {
		return
	}
	var pid uint32
	windowThreadProcess.Call(front, uintptr(unsafe.Pointer(&pid)))
	mine, _, _ := currentProcessID.Call()
	if uintptr(pid) == mine {
		return
	}
	setWindowPos.Call(h.hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpShow)
	setForegroundWindow.Call(h.hwnd)
}

// leave closes the stand for good. When Windows starts this instead of the
// desktop, leaving has to put the desktop back or there is nothing there.
func (h *host) leave() {
	h.leaving = true
	if h.s.shell {
		log.Printf("starting the desktop")
		if err := exec.Command(filepath.Join(os.Getenv("WINDIR"), "explorer.exe")).Start(); err != nil {
			log.Printf("could not start the desktop: %v", err)
		}
	}
	h.close()
}

func (h *host) close() { destroyWindow.Call(h.hwnd) }

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

	case wmSysCommand:
		// The screen saver and the monitor going to sleep both arrive here, and
		// a stand does neither.
		if h != nil && !h.s.windowed {
			switch wParam & 0xFFF0 {
			case scScreenSave, scMonitorOff:
				return 0
			}
		}

	case wmClose:
		// Alt+F4 and anything else asking politely gets nothing. Leaving is a
		// decision made in the hidden menu.
		if h != nil && !h.s.windowed && !h.leaving {
			return 0
		}
		destroyWindow.Call(hwnd)
		return 0

	case wmDestroy:
		postQuitMessage.Call(0)
		return 0
	}

	r, _, _ := defWindowProc.Call(hwnd, message, wParam, lParam)
	return r
}

// hookProc swallows the shortcuts that would put a visitor somewhere else:
// Alt+Tab, Alt+F4, Alt+Esc, Ctrl+Esc, Ctrl+Shift+Esc and the Windows key.
// Ctrl+Alt+Delete is not one a program can take; the lockdown script turns off
// what that screen offers instead.
func hookProc(code int32, wParam, lParam uintptr) uintptr {
	if code == 0 && live != nil && (wParam == wmKeyDown || wParam == wmSysKeyDown) {
		// The address belongs to Windows, so the event is copied out rather
		// than pointed at.
		var key kbdLLHook
		copyMemory.Call(uintptr(unsafe.Pointer(&key)), lParam, unsafe.Sizeof(key))
		if swallow(key) {
			return 1
		}
	}
	r, _, _ := callNextHookEx.Call(0, uintptr(code), wParam, lParam)
	return r
}

func swallow(k kbdLLHook) bool {
	alt := k.flags&llAltDown != 0
	switch k.vkCode {
	case vkLWin, vkRWin, vkApps:
		return true
	case vkTab, vkEscape:
		return alt || held(vkControl)
	case vkF4:
		return alt
	}
	return false
}

// blockedKey answers the view's own shortcuts. Typing has to keep working: a
// visitor types a GitHub handle, and an operator types a code and a wireless
// password, so only the browser combinations go.
func blockedKey(key uint) bool {
	if key >= 0x70 && key <= 0x7B { // F1 to F12
		return true
	}
	if !held(vkControl) {
		return false
	}
	switch key {
	case 'A', 'C', 'V', 'X', 'Z':
		return false
	}
	return true
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
