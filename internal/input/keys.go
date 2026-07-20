//go:build windows

package input

import "strings"

// Windows virtual-key codes used by the app's hotkeys.
const (
	vkShift   = 0x10
	vkControl = 0x11
	vkMenu    = 0x12 // Alt
	vkTab     = 0x09
	vkReturn  = 0x0D
	vkPrior   = 0x21 // Page Up
	vkNext    = 0x22 // Page Down
	vkDelete  = 0x2E
	vkCapital = 0x14 // Caps Lock
	vkF1      = 0x70
	vkF2      = 0x71
	vkPeriod  = 0xBE // OEM_PERIOD
	vk1       = 0x31
	vkZ       = 0x5A
	vkC       = 0x43
)

// tokenVK maps a hotkey token to its virtual-key code.
var tokenVK = map[string]int{
	"shift":     vkShift,
	"ctrl":      vkControl,
	"control":   vkControl,
	"alt":       vkMenu,
	"tab":       vkTab,
	"enter":     vkReturn,
	"page up":   vkPrior,
	"pageup":    vkPrior,
	"page down": vkNext,
	"pagedown":  vkNext,
	"delete":    vkDelete,
	"caps lock": vkCapital,
	"capslock":  vkCapital,
	"f1":        vkF1,
	"f2":        vkF2,
	".":         vkPeriod,
	"1":         vk1,
	"z":         vkZ,
	"c":         vkC,
}

// parseSpec turns a hotkey spec like "ctrl+f2" or "shift" into its main
// virtual-key code and required modifier flags.
func parseSpec(spec string) (vk int, ctrl, alt, shift bool, ok bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(spec)), "+")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		// The last token is the main key; earlier tokens are modifiers, unless
		// there is only one token.
		isModifier := i < len(parts)-1
		switch p {
		case "ctrl", "control":
			if isModifier {
				ctrl = true
				continue
			}
		case "alt":
			if isModifier {
				alt = true
				continue
			}
		case "shift":
			if isModifier {
				shift = true
				continue
			}
		}
		code, found := tokenVK[p]
		if !found {
			return 0, false, false, false, false
		}
		vk = code
	}
	return vk, ctrl, alt, shift, true
}
