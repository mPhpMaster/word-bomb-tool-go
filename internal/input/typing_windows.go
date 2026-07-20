//go:build windows

package input

import (
	"unicode/utf16"
	"unsafe"
)

const (
	inputKeyboard   = 1
	keyEventUnicode = 0x0004
	keyEventKeyUp   = 0x0002
)

var procSendInput = user32.NewProc("SendInput")

// Compile-time assertion that INPUT is the correct 40-byte Win32 layout on
// amd64. If the size is ever wrong this indexes out of range and fails to build.
var _ = [1]struct{}{}[unsafe.Sizeof(inputStruct{})-40]

type keyboardInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

// input matches the Win32 INPUT structure (keyboard variant) with the union
// padded to the size of MOUSEINPUT on amd64.
type inputStruct struct {
	inputType uint32
	_         uint32 // alignment padding before the union
	ki        keyboardInput
	_         [8]byte // pad union up to MOUSEINPUT size
}

func sendInputs(in []inputStruct) {
	if len(in) == 0 {
		return
	}
	procSendInput.Call(
		uintptr(len(in)),
		uintptr(unsafe.Pointer(&in[0])),
		unsafe.Sizeof(in[0]),
	)
}

// TypeRune sends a single Unicode rune as a key down + key up pair. Runes
// outside the BMP are sent as UTF-16 surrogate pairs.
func TypeRune(r rune) {
	units := utf16.Encode([]rune{r})
	in := make([]inputStruct, 0, len(units)*2)
	for _, u := range units {
		in = append(in,
			unicodeInput(u, keyEventUnicode),
			unicodeInput(u, keyEventUnicode|keyEventKeyUp),
		)
	}
	sendInputs(in)
}

func unicodeInput(unit uint16, flags uint32) inputStruct {
	return inputStruct{
		inputType: inputKeyboard,
		ki: keyboardInput{
			wVk:     0,
			wScan:   unit,
			dwFlags: flags,
		},
	}
}

// TypeString types each rune of s in order with no inter-key delay.
func TypeString(s string) {
	for _, r := range s {
		TypeRune(r)
	}
}

// PressEnter presses and releases the Enter key.
func PressEnter() {
	down := inputStruct{inputType: inputKeyboard, ki: keyboardInput{wVk: vkReturn}}
	up := inputStruct{inputType: inputKeyboard, ki: keyboardInput{wVk: vkReturn, dwFlags: keyEventKeyUp}}
	sendInputs([]inputStruct{down, up})
}
