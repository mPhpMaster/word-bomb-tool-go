//go:build windows

package input

import "testing"

func TestArabicRunesAreSentAsUnicodeKeyEvents(t *testing.T) {
	for _, r := range "يزال" {
		in := runeInputs(r)
		if len(in) != 2 {
			t.Fatalf("%q: %d events, want down+up", r, len(in))
		}
		for i, e := range in {
			if e.inputType != inputKeyboard || e.ki.wVk != 0 || e.ki.wScan != uint16(r) {
				t.Fatalf("%q event %d = %+v", r, i, e)
			}
			if e.ki.dwFlags&keyEventUnicode == 0 {
				t.Fatalf("%q event %d lacks KEYEVENTF_UNICODE", r, i)
			}
		}
		if in[0].ki.dwFlags&keyEventKeyUp != 0 || in[1].ki.dwFlags&keyEventKeyUp == 0 {
			t.Fatalf("%q: want key down then key up", r)
		}
	}
	// Runes outside the BMP become a surrogate pair (two downs, two ups).
	if n := len(runeInputs('😀')); n != 4 {
		t.Fatalf("surrogate pair: %d events, want 4", n)
	}
}
