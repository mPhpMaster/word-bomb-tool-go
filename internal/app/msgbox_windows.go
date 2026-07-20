//go:build windows

package app

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32          = windows.NewLazySystemDLL("user32.dll")
	procMessageBoxW = user32.NewProc("MessageBoxW")
)

const (
	mbYesNo        = 0x00000004
	mbIconWarning  = 0x00000030
	mbIconInfo     = 0x00000040
	mbSystemModal  = 0x00001000
	idYes          = 6
)

// messageYesNo shows a modal Yes/No dialog and reports whether Yes was chosen.
func messageYesNo(title, text string) bool {
	ret, _, _ := procMessageBoxW.Call(
		0,
		strPtr(text),
		strPtr(title),
		uintptr(mbYesNo|mbIconWarning|mbSystemModal),
	)
	return int(ret) == idYes
}

// messageInfo shows a modal information dialog.
func messageInfo(title, text string) {
	procMessageBoxW.Call(
		0,
		strPtr(text),
		strPtr(title),
		uintptr(mbIconInfo|mbSystemModal),
	)
}

func strPtr(s string) uintptr {
	p, _ := windows.UTF16PtrFromString(s)
	return uintptr(unsafe.Pointer(p))
}
