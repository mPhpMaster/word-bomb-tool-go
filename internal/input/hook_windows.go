//go:build windows

// Package input provides a global low-level keyboard hook and synthetic
// keystroke generation on Windows. It replaces the Python `keyboard` library.
package input

import (
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	whKeyboardLL = 13
	wmKeyDown    = 0x0100
	wmSysKeyDown = 0x0104
	wmKeyUp      = 0x0101
	wmSysKeyUp   = 0x0105
	wmQuit       = 0x0012
)

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	procSetWindowsHookEx = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx   = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHook = user32.NewProc("UnhookWindowsHookEx")
	procGetMessage       = user32.NewProc("GetMessageW")
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
	procPostThreadMsg    = user32.NewProc("PostThreadMessageW")
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procGetCurrentThread = kernel32.NewProc("GetCurrentThreadId")
)

type kbdllHookStruct struct {
	vkCode      uint32
	scanCode    uint32
	flags       uint32
	time        uint32
	dwExtraInfo uintptr
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

type hotkey struct {
	vk       int
	ctrl     bool
	alt      bool
	shift    bool
	callback func()
}

// Hook is a global low-level keyboard hook that fires registered callbacks.
type Hook struct {
	mu       sync.Mutex
	hotkeys  []hotkey
	down     map[uint32]bool // currently-pressed main keys (for edge detection)
	hookH    uintptr
	threadID uint32
	started  chan struct{}
}

// NewHook creates an unstarted hook.
func NewHook() *Hook {
	return &Hook{
		down:    make(map[uint32]bool),
		started: make(chan struct{}),
	}
}

// Register adds a hotkey. spec examples: "shift", "alt+1", "ctrl+z", "tab",
// "page up", ".". Unknown specs are ignored (reported by the boolean).
func (h *Hook) Register(spec string, cb func()) bool {
	vk, ctrl, alt, shift, ok := parseSpec(spec)
	if !ok {
		return false
	}
	h.mu.Lock()
	h.hotkeys = append(h.hotkeys, hotkey{vk: vk, ctrl: ctrl, alt: alt, shift: shift, callback: cb})
	h.mu.Unlock()
	return true
}

// Start installs the hook and runs its message loop on a dedicated OS thread.
// It blocks until Stop is called, so callers typically run it in a goroutine.
func (h *Hook) Start() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	tid, _, _ := procGetCurrentThread.Call()
	h.threadID = uint32(tid)

	cb := windows.NewCallback(h.proc)
	handle, _, _ := procSetWindowsHookEx.Call(uintptr(whKeyboardLL), cb, 0, 0)
	h.hookH = handle
	close(h.started)

	var m msg
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 { // WM_QUIT or error
			break
		}
	}
	if h.hookH != 0 {
		procUnhookWindowsHook.Call(h.hookH)
		h.hookH = 0
	}
}

// Started returns a channel closed once the hook is installed.
func (h *Hook) Started() <-chan struct{} { return h.started }

// Stop unhooks and terminates the message loop.
func (h *Hook) Stop() {
	if h.threadID != 0 {
		procPostThreadMsg.Call(uintptr(h.threadID), uintptr(wmQuit), 0, 0)
	}
}

func (h *Hook) proc(nCode int, wParam uintptr, lParam uintptr) uintptr {
	if nCode == 0 {
		switch wParam {
		case wmKeyDown, wmSysKeyDown:
			ks := (*kbdllHookStruct)(unsafe.Pointer(lParam))
			h.handleKeyDown(ks.vkCode)
		case wmKeyUp, wmSysKeyUp:
			ks := (*kbdllHookStruct)(unsafe.Pointer(lParam))
			h.mu.Lock()
			delete(h.down, ks.vkCode)
			h.mu.Unlock()
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

func (h *Hook) handleKeyDown(vk uint32) {
	h.mu.Lock()
	// Edge detection: ignore auto-repeat while the key stays held.
	if h.down[vk] {
		h.mu.Unlock()
		return
	}
	h.down[vk] = true

	ctrl := asyncDown(vkControl)
	alt := asyncDown(vkMenu)
	shift := asyncDown(vkShift)

	var fire []func()
	for _, hk := range h.hotkeys {
		if uint32(hk.vk) != vk {
			continue
		}
		// Modifier requirements must match. For a bare key (no modifiers
		// required) we insist ctrl/alt are not held, so combinations like
		// Ctrl+Tab don't trigger the plain Tab hotkey. Shift is ignored for
		// bare keys unless the key itself is Shift.
		if hk.ctrl != ctrl || hk.alt != alt {
			continue
		}
		if hk.shift && !shift {
			continue
		}
		fire = append(fire, hk.callback)
	}
	h.mu.Unlock()

	for _, cb := range fire {
		go cb()
	}
}

func asyncDown(vk int) bool {
	r, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return r&0x8000 != 0
}
