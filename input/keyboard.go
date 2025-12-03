package input

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

var (
	user32          = windows.NewLazySystemDLL("user32.dll")
	procKeybd_event = user32.NewProc("keybd_event")
)

const (
	KEYEVENTF_KEYDOWN     = 0x0000
	KEYEVENTF_KEYUP       = 0x0002
	KEYEVENTF_EXTENDEDKEY = 0x0001
)

// Modifiers
const (
	VK_SHIFT    = 0x10
	VK_CONTROL  = 0x11
	VK_ALT      = 0x12
	VK_LSHIFT   = 0xA0
	VK_RSHIFT   = 0xA1
	VK_LCONTROL = 0xA2
	VK_RCONTROL = 0xA3
	VK_LALT     = 0xA4
	VK_RALT     = 0xA5
)

var keyCodeMap = map[string]uint8{
	"F1": 0x70, "F2": 0x71, "F3": 0x72, "F4": 0x73,
	"F5": 0x74, "F6": 0x75, "F7": 0x76, "F8": 0x77,
	"F9": 0x78, "F10": 0x79, "F11": 0x7A, "F12": 0x7B,
	"1": 0x31, "2": 0x32, "3": 0x33, "4": 0x34, "5": 0x35,
	"6": 0x36, "7": 0x37, "8": 0x38, "9": 0x39, "0": 0x30,
	"Q": 0x51, "W": 0x57, "E": 0x45, "R": 0x52, "T": 0x54,
	"Y": 0x59, "U": 0x55, "I": 0x49, "O": 0x4F, "P": 0x50,
	"A": 0x41, "S": 0x53, "D": 0x44, "F": 0x46, "G": 0x47,
	"H": 0x48, "J": 0x4A, "K": 0x4B, "L": 0x4C,
	"Z": 0x5A, "X": 0x58, "C": 0x43, "V": 0x56,
	"B": 0x42, "N": 0x4E, "M": 0x4D,
	"SPACE": 0x20, "ENTER": 0x0D, "TAB": 0x09,
	"ESC": 0x1B, "ESCAPE": 0x1B,
	"BACKSPACE": 0x08, "DELETE": 0x2E, "INSERT": 0x2D,
	"HOME": 0x24, "END": 0x23, "PAGEUP": 0x21, "PAGEDOWN": 0x22,
	"UP": 0x26, "DOWN": 0x28, "LEFT": 0x25, "RIGHT": 0x27,
	"NUMPAD0": 0x60, "NUMPAD1": 0x61, "NUMPAD2": 0x62, "NUMPAD3": 0x63,
	"NUMPAD4": 0x64, "NUMPAD5": 0x65, "NUMPAD6": 0x66, "NUMPAD7": 0x67,
	"NUMPAD8": 0x68, "NUMPAD9": 0x69,
	"NUM0": 0x60, "NUM1": 0x61, "NUM2": 0x62, "NUM3": 0x63,
	"NUM4": 0x64, "NUM5": 0x65, "NUM6": 0x66, "NUM7": 0x67,
	"NUM8": 0x68, "NUM9": 0x69,
	"`": 0xC0, "TILDE": 0xC0, "~": 0xC0,
	"-": 0xBD, "=": 0xBB,
	"[": 0xDB, "]": 0xDD, "\\": 0xDC,
	";": 0xBA, "'": 0xDE,
	",": 0xBC, ".": 0xBE, "/": 0xBF,
}

var modifierMap = map[string]uint8{
	"SHIFT":    VK_SHIFT,
	"CTRL":     VK_CONTROL,
	"CONTROL":  VK_CONTROL,
	"ALT":      VK_ALT,
	"LSHIFT":   VK_LSHIFT,
	"RSHIFT":   VK_RSHIFT,
	"LCTRL":    VK_LCONTROL,
	"LCONTROL": VK_LCONTROL,
	"RCTRL":    VK_RCONTROL,
	"RCONTROL": VK_RCONTROL,
	"LALT":     VK_LALT,
	"RALT":     VK_RALT,
}

// KeyCombo representa uma combinação de teclas
type KeyCombo struct {
	Modifiers []uint8
	MainKey   uint8
	RawString string
}

// ParseKeyCombo converte string como "SHIFT+5" em KeyCombo
func ParseKeyCombo(keyStr string) KeyCombo {
	combo := KeyCombo{
		Modifiers: make([]uint8, 0),
		RawString: keyStr,
	}

	keyStr = strings.ToUpper(strings.TrimSpace(keyStr))
	parts := strings.Split(keyStr, "+")

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if i == len(parts)-1 {
			if code, ok := keyCodeMap[part]; ok {
				combo.MainKey = code
			} else {
				fmt.Printf("[KEY] Tecla desconhecida: %s\n", part)
			}
		} else {
			if mod, ok := modifierMap[part]; ok {
				combo.Modifiers = append(combo.Modifiers, mod)
			} else if code, ok := keyCodeMap[part]; ok {
				combo.Modifiers = append(combo.Modifiers, code)
			} else {
				fmt.Printf("[KEY] Modificador desconhecido: %s\n", part)
			}
		}
	}

	return combo
}

// SendKeyCombo envia uma combinação de teclas
func SendKeyCombo(combo KeyCombo) {
	for _, mod := range combo.Modifiers {
		procKeybd_event.Call(uintptr(mod), 0, KEYEVENTF_KEYDOWN, 0)
	}

	if len(combo.Modifiers) > 0 {
		time.Sleep(10 * time.Millisecond)
	}

	procKeybd_event.Call(uintptr(combo.MainKey), 0, KEYEVENTF_KEYDOWN, 0)
	time.Sleep(20 * time.Millisecond)
	procKeybd_event.Call(uintptr(combo.MainKey), 0, KEYEVENTF_KEYUP, 0)

	if len(combo.Modifiers) > 0 {
		time.Sleep(10 * time.Millisecond)
	}

	for i := len(combo.Modifiers) - 1; i >= 0; i-- {
		procKeybd_event.Call(uintptr(combo.Modifiers[i]), 0, KEYEVENTF_KEYUP, 0)
	}
}

// SpamKeyCombo envia a combinação várias vezes
func SpamKeyCombo(combo KeyCombo, count int, interval time.Duration) {
	for i := 0; i < count; i++ {
		SendKeyCombo(combo)
		if i < count-1 && interval > 0 {
			time.Sleep(interval)
		}
	}
}