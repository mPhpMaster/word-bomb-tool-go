//go:build windows

// Package ui contains the walk-based GUI: the log window, region overlays, the
// fullscreen region selector, dialogs and the system-tray icon. It is the Go
// port of ui_manager.py and tray_manager.py.
package ui

import (
	"strconv"
	"strings"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

var (
	user32                          = windows.NewLazySystemDLL("user32.dll")
	procSetLayeredWindowAttributes  = user32.NewProc("SetLayeredWindowAttributes")
)

const (
	lwaColorKey = 0x00000001
	lwaAlpha    = 0x00000002
)

// parseHexColor converts "#rrggbb" to a walk.Color.
func parseHexColor(hex string) walk.Color {
	s := strings.TrimPrefix(hex, "#")
	if len(s) != 6 {
		return walk.RGB(0, 0, 0)
	}
	r, _ := strconv.ParseUint(s[0:2], 16, 8)
	g, _ := strconv.ParseUint(s[2:4], 16, 8)
	b, _ := strconv.ParseUint(s[4:6], 16, 8)
	return walk.RGB(byte(r), byte(g), byte(b))
}

// colorRef returns the Win32 COLORREF (0x00BBGGRR) for a hex color.
func colorRef(hex string) uint32 {
	s := strings.TrimPrefix(hex, "#")
	if len(s) != 6 {
		return 0
	}
	r, _ := strconv.ParseUint(s[0:2], 16, 8)
	g, _ := strconv.ParseUint(s[2:4], 16, 8)
	b, _ := strconv.ParseUint(s[4:6], 16, 8)
	return uint32(b)<<16 | uint32(g)<<8 | uint32(r)
}

// setWindowAlpha applies whole-window alpha (0..255) to a layered window.
func setWindowAlpha(hwnd win.HWND, alpha byte) {
	addExStyle(hwnd, win.WS_EX_LAYERED)
	procSetLayeredWindowAttributes.Call(uintptr(hwnd), 0, uintptr(alpha), lwaAlpha)
}

// setWindowColorKey makes every pixel of colorRef fully transparent (used to
// punch out the interior of the region overlays, leaving only the border).
func setWindowColorKey(hwnd win.HWND, key uint32) {
	addExStyle(hwnd, win.WS_EX_LAYERED)
	procSetLayeredWindowAttributes.Call(uintptr(hwnd), uintptr(key), 0, lwaColorKey)
}

func addExStyle(hwnd win.HWND, style int32) {
	cur := win.GetWindowLong(hwnd, win.GWL_EXSTYLE)
	win.SetWindowLong(hwnd, win.GWL_EXSTYLE, cur|style)
}

// setPopupStyle replaces a window's style with WS_POPUP (frameless). WS_POPUP is
// 0x80000000 which does not fit a signed int32 directly, hence the cast.
func setPopupStyle(hwnd win.HWND) {
	var style uint32 = win.WS_POPUP
	win.SetWindowLong(hwnd, win.GWL_STYLE, int32(style))
}

// makeToolOverlay strips the window chrome and turns hwnd into a borderless,
// topmost, non-activating, click-through tool window used for the region
// overlays.
func makeToolOverlay(hwnd win.HWND) {
	cur := win.GetWindowLong(hwnd, win.GWL_EXSTYLE)
	cur |= win.WS_EX_LAYERED | win.WS_EX_TRANSPARENT | win.WS_EX_TOOLWINDOW | win.WS_EX_TOPMOST | win.WS_EX_NOACTIVATE
	win.SetWindowLong(hwnd, win.GWL_EXSTYLE, cur)
}

// setTopmost keeps a window above all others without moving/resizing it.
func setTopmost(hwnd win.HWND) {
	win.SetWindowPos(hwnd, win.HWND_TOPMOST, 0, 0, 0, 0,
		win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOACTIVATE)
}

// virtualScreen returns the bounds of the entire virtual desktop (all monitors).
func virtualScreen() (x, y, w, h int) {
	x = int(win.GetSystemMetrics(win.SM_XVIRTUALSCREEN))
	y = int(win.GetSystemMetrics(win.SM_YVIRTUALSCREEN))
	w = int(win.GetSystemMetrics(win.SM_CXVIRTUALSCREEN))
	h = int(win.GetSystemMetrics(win.SM_CYVIRTUALSCREEN))
	if w == 0 {
		w = int(win.GetSystemMetrics(win.SM_CXSCREEN))
	}
	if h == 0 {
		h = int(win.GetSystemMetrics(win.SM_CYSCREEN))
	}
	return
}
