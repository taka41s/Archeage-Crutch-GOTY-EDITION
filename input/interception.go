// interception.go
package input

import (
	"fmt"
	"sync"
	"time"
	"unsafe"
	"strings"

	"golang.org/x/sys/windows"
)

// ============================================
// Interception Driver Integration
// https://github.com/oblitum/Interception
// ============================================

var (
	interceptionDLL              *windows.LazyDLL
	procInterceptionCreateContext = (*windows.LazyProc)(nil)
	procInterceptionDestroyContext = (*windows.LazyProc)(nil)
	procInterceptionGetPrecedence  = (*windows.LazyProc)(nil)
	procInterceptionSetPrecedence  = (*windows.LazyProc)(nil)
	procInterceptionGetFilter      = (*windows.LazyProc)(nil)
	procInterceptionSetFilter      = (*windows.LazyProc)(nil)
	procInterceptionWait           = (*windows.LazyProc)(nil)
	procInterceptionWaitWithTimeout = (*windows.LazyProc)(nil)
	procInterceptionSend           = (*windows.LazyProc)(nil)
	procInterceptionReceive        = (*windows.LazyProc)(nil)
	procInterceptionIsKeyboard     = (*windows.LazyProc)(nil)
	procInterceptionIsMouse        = (*windows.LazyProc)(nil)
	procInterceptionIsInvalid      = (*windows.LazyProc)(nil)
	procInterceptionGetHardwareId  = (*windows.LazyProc)(nil)

	interceptionLoaded bool
	interceptionMu     sync.Mutex
)

// Interception types
type InterceptionContext uintptr
type InterceptionDevice int32
type InterceptionPrecedence int32
type InterceptionFilter uint16

// Interception constants
const (
	INTERCEPTION_MAX_KEYBOARD = 10
	INTERCEPTION_MAX_MOUSE    = 10
	INTERCEPTION_MAX_DEVICE   = INTERCEPTION_MAX_KEYBOARD + INTERCEPTION_MAX_MOUSE
)

// Keyboard filter flags
const (
	INTERCEPTION_FILTER_KEY_NONE             InterceptionFilter = 0x0000
	INTERCEPTION_FILTER_KEY_ALL              InterceptionFilter = 0xFFFF
	INTERCEPTION_FILTER_KEY_DOWN             InterceptionFilter = 0x0001
	INTERCEPTION_FILTER_KEY_UP               InterceptionFilter = 0x0002
	INTERCEPTION_FILTER_KEY_E0               InterceptionFilter = 0x0004
	INTERCEPTION_FILTER_KEY_E1               InterceptionFilter = 0x0008
	INTERCEPTION_FILTER_KEY_TERMSRV_SET_LED  InterceptionFilter = 0x0010
	INTERCEPTION_FILTER_KEY_TERMSRV_SHADOW   InterceptionFilter = 0x0020
	INTERCEPTION_FILTER_KEY_TERMSRV_VKPACKET InterceptionFilter = 0x0040
)

// Key state flags
const (
	INTERCEPTION_KEY_DOWN             = 0x00
	INTERCEPTION_KEY_UP               = 0x01
	INTERCEPTION_KEY_E0               = 0x02
	INTERCEPTION_KEY_E1               = 0x04
	INTERCEPTION_KEY_TERMSRV_SET_LED  = 0x08
	INTERCEPTION_KEY_TERMSRV_SHADOW   = 0x10
	INTERCEPTION_KEY_TERMSRV_VKPACKET = 0x20
)

// InterceptionKeyStroke represents a keyboard event
type InterceptionKeyStroke struct {
	Code        uint16
	State       uint16
	Information uint32
}

// InterceptionMouseStroke represents a mouse event
type InterceptionMouseStroke struct {
	State       uint16
	Flags       uint16
	Rolling     int16
	X           int32
	Y           int32
	Information uint32
}

// VirtualKeyboard gerencia um teclado virtual via Interception
type VirtualKeyboard struct {
	context    InterceptionContext
	device     InterceptionDevice
	available  bool
	mu         sync.Mutex
}

var virtualKeyboard *VirtualKeyboard

// Scancode map (diferente de VK codes)
var scancodeMap = map[string]uint16{
	"ESC": 0x01, "ESCAPE": 0x01,
	"1": 0x02, "2": 0x03, "3": 0x04, "4": 0x05, "5": 0x06,
	"6": 0x07, "7": 0x08, "8": 0x09, "9": 0x0A, "0": 0x0B,
	"-": 0x0C, "=": 0x0D,
	"BACKSPACE": 0x0E, "TAB": 0x0F,
	"Q": 0x10, "W": 0x11, "E": 0x12, "R": 0x13, "T": 0x14,
	"Y": 0x15, "U": 0x16, "I": 0x17, "O": 0x18, "P": 0x19,
	"[": 0x1A, "]": 0x1B, "ENTER": 0x1C,
	"CTRL": 0x1D, "CONTROL": 0x1D, "LCTRL": 0x1D, "LCONTROL": 0x1D,
	"A": 0x1E, "S": 0x1F, "D": 0x20, "F": 0x21, "G": 0x22,
	"H": 0x23, "J": 0x24, "K": 0x25, "L": 0x26,
	";": 0x27, "'": 0x28, "`": 0x29, "TILDE": 0x29,
	"LSHIFT": 0x2A, "SHIFT": 0x2A,
	"\\": 0x2B,
	"Z": 0x2C, "X": 0x2D, "C": 0x2E, "V": 0x2F,
	"B": 0x30, "N": 0x31, "M": 0x32,
	",": 0x33, ".": 0x34, "/": 0x35,
	"RSHIFT": 0x36,
	"NUMPAD*": 0x37, "NUM*": 0x37,
	"ALT": 0x38, "LALT": 0x38,
	"SPACE": 0x39,
	"CAPSLOCK": 0x3A,
	"F1": 0x3B, "F2": 0x3C, "F3": 0x3D, "F4": 0x3E, "F5": 0x3F,
	"F6": 0x40, "F7": 0x41, "F8": 0x42, "F9": 0x43, "F10": 0x44,
	"NUMLOCK": 0x45, "SCROLLLOCK": 0x46,
	"NUMPAD7": 0x47, "NUMPAD8": 0x48, "NUMPAD9": 0x49, "NUMPAD-": 0x4A,
	"NUMPAD4": 0x4B, "NUMPAD5": 0x4C, "NUMPAD6": 0x4D, "NUMPAD+": 0x4E,
	"NUMPAD1": 0x4F, "NUMPAD2": 0x50, "NUMPAD3": 0x51,
	"NUMPAD0": 0x52, "NUMPAD.": 0x53,
	"NUM7": 0x47, "NUM8": 0x48, "NUM9": 0x49,
	"NUM4": 0x4B, "NUM5": 0x4C, "NUM6": 0x4D,
	"NUM1": 0x4F, "NUM2": 0x50, "NUM3": 0x51,
	"NUM0": 0x52,
	"F11": 0x57, "F12": 0x58,
}

// Extended keys (precisam do flag E0)
var extendedKeys = map[uint16]bool{
	0x1C: false, // Enter normal não é extended, mas Numpad Enter é
	0x1D: false, // LCtrl normal, RCtrl é extended
	0x35: false, // / normal, Numpad / é extended
	0x37: false, // * normal, PrintScreen é extended
	0x38: false, // LAlt normal, RAlt é extended
	0x45: false, // NumLock
	0x47: false, // Home (extended)
	0x48: false, // Up (extended)
	0x49: false, // PageUp (extended)
	0x4B: false, // Left (extended)
	0x4D: false, // Right (extended)
	0x4F: false, // End (extended)
	0x50: false, // Down (extended)
	0x51: false, // PageDown (extended)
	0x52: false, // Insert (extended)
	0x53: false, // Delete (extended)
}

// Extended scancodes (navegação, etc)
var extendedScancodes = map[string]uint16{
	"RCTRL": 0x1D, "RCONTROL": 0x1D,
	"RALT": 0x38,
	"HOME": 0x47, "UP": 0x48, "PAGEUP": 0x49,
	"LEFT": 0x4B, "RIGHT": 0x4D,
	"END": 0x4F, "DOWN": 0x50, "PAGEDOWN": 0x51,
	"INSERT": 0x52, "DELETE": 0x53,
	"NUMPADENTER": 0x1C,
	"NUMPAD/": 0x35,
}

// LoadInterception carrega a DLL do Interception
func LoadInterception() error {
	interceptionMu.Lock()
	defer interceptionMu.Unlock()

	if interceptionLoaded {
		return nil
	}

	// Tenta carregar a DLL (precisa estar no PATH ou no mesmo diretório)
	interceptionDLL = windows.NewLazyDLL("interception.dll")
	
	if err := interceptionDLL.Load(); err != nil {
		return fmt.Errorf("failed to load interception.dll: %v (driver installed?)", err)
	}

	// Carrega todas as funções
	procInterceptionCreateContext = interceptionDLL.NewProc("interception_create_context")
	procInterceptionDestroyContext = interceptionDLL.NewProc("interception_destroy_context")
	procInterceptionGetPrecedence = interceptionDLL.NewProc("interception_get_precedence")
	procInterceptionSetPrecedence = interceptionDLL.NewProc("interception_set_precedence")
	procInterceptionGetFilter = interceptionDLL.NewProc("interception_get_filter")
	procInterceptionSetFilter = interceptionDLL.NewProc("interception_set_filter")
	procInterceptionWait = interceptionDLL.NewProc("interception_wait")
	procInterceptionWaitWithTimeout = interceptionDLL.NewProc("interception_wait_with_timeout")
	procInterceptionSend = interceptionDLL.NewProc("interception_send")
	procInterceptionReceive = interceptionDLL.NewProc("interception_receive")
	procInterceptionIsKeyboard = interceptionDLL.NewProc("interception_is_keyboard")
	procInterceptionIsMouse = interceptionDLL.NewProc("interception_is_mouse")
	procInterceptionIsInvalid = interceptionDLL.NewProc("interception_is_invalid")
	procInterceptionGetHardwareId = interceptionDLL.NewProc("interception_get_hardware_id")

	interceptionLoaded = true
	return nil
}

// InitVirtualKeyboard inicializa o teclado virtual
func InitVirtualKeyboard() error {
	if err := LoadInterception(); err != nil {
		return err
	}

	virtualKeyboard = &VirtualKeyboard{}
	
	// Cria contexto
	ret, _, _ := procInterceptionCreateContext.Call()
	if ret == 0 {
		return fmt.Errorf("failed to create interception context")
	}
	virtualKeyboard.context = InterceptionContext(ret)

	// Encontra o primeiro dispositivo de teclado disponível
	for device := InterceptionDevice(1); device <= INTERCEPTION_MAX_KEYBOARD; device++ {
		ret, _, _ := procInterceptionIsKeyboard.Call(uintptr(device))
		if ret != 0 {
			virtualKeyboard.device = device
			virtualKeyboard.available = true
			fmt.Printf("[Interception] Using keyboard device: %d\n", device)
			break
		}
	}

	if !virtualKeyboard.available {
		return fmt.Errorf("no keyboard device found")
	}

	return nil
}

// CloseVirtualKeyboard fecha o teclado virtual
func CloseVirtualKeyboard() {
	if virtualKeyboard != nil && virtualKeyboard.context != 0 {
		procInterceptionDestroyContext.Call(uintptr(virtualKeyboard.context))
		virtualKeyboard.context = 0
		virtualKeyboard.available = false
	}
}

// IsInterceptionAvailable verifica se o Interception está disponível
func IsInterceptionAvailable() bool {
	return virtualKeyboard != nil && virtualKeyboard.available
}

// sendInterceptionKey envia uma tecla via Interception
func sendInterceptionKey(scancode uint16, isExtended bool, isKeyUp bool) error {
	if !IsInterceptionAvailable() {
		return fmt.Errorf("interception not available")
	}

	virtualKeyboard.mu.Lock()
	defer virtualKeyboard.mu.Unlock()

	stroke := InterceptionKeyStroke{
		Code:        scancode,
		State:       INTERCEPTION_KEY_DOWN,
		Information: 0,
	}

	if isKeyUp {
		stroke.State |= INTERCEPTION_KEY_UP
	}

	if isExtended {
		stroke.State |= INTERCEPTION_KEY_E0
	}

	ret, _, _ := procInterceptionSend.Call(
		uintptr(virtualKeyboard.context),
		uintptr(virtualKeyboard.device),
		uintptr(unsafe.Pointer(&stroke)),
		1,
	)

	if ret == 0 {
		return fmt.Errorf("failed to send key")
	}

	return nil
}

// getScancode retorna o scancode e se é extended
func getScancode(keyName string) (uint16, bool) {
	keyName = strings.ToUpper(keyName)
	
	// Verifica primeiro os extended
	if sc, ok := extendedScancodes[keyName]; ok {
		return sc, true
	}
	
	// Depois os normais
	if sc, ok := scancodeMap[keyName]; ok {
		return sc, false
	}
	
	return 0, false
}

// KeyComboInterception representa um combo para Interception
type KeyComboInterception struct {
	Modifiers []struct {
		Scancode   uint16
		IsExtended bool
	}
	MainKey struct {
		Scancode   uint16
		IsExtended bool
	}
	RawString string
}

// ParseKeyComboInterception converte string para combo de Interception
func ParseKeyComboInterception(keyStr string) KeyComboInterception {
	combo := KeyComboInterception{
		RawString: keyStr,
	}

	keyStr = strings.ToUpper(strings.TrimSpace(keyStr))
	parts := strings.Split(keyStr, "+")

	modifierNames := map[string]bool{
		"SHIFT": true, "LSHIFT": true, "RSHIFT": true,
		"CTRL": true, "CONTROL": true, "LCTRL": true, "LCONTROL": true, "RCTRL": true, "RCONTROL": true,
		"ALT": true, "LALT": true, "RALT": true,
	}

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		scancode, isExtended := getScancode(part)
		if scancode == 0 {
			fmt.Printf("[Interception] Unknown key: %s\n", part)
			continue
		}

		if i == len(parts)-1 && !modifierNames[part] {
			combo.MainKey.Scancode = scancode
			combo.MainKey.IsExtended = isExtended
		} else {
			combo.Modifiers = append(combo.Modifiers, struct {
				Scancode   uint16
				IsExtended bool
			}{scancode, isExtended})
		}
	}

	return combo
}

// SendKeyVirtual envia uma tecla via teclado virtual (não interfere com input real)
func SendKeyVirtual(keyStr string) error {
	if !IsInterceptionAvailable() {
		return fmt.Errorf("interception not available, falling back to SendInput")
	}

	combo := ParseKeyComboInterception(keyStr)
	
	// Press modifiers
	for _, mod := range combo.Modifiers {
		if err := sendInterceptionKey(mod.Scancode, mod.IsExtended, false); err != nil {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Press main key
	if err := sendInterceptionKey(combo.MainKey.Scancode, combo.MainKey.IsExtended, false); err != nil {
		return err
	}
	time.Sleep(15 * time.Millisecond)

	// Release main key
	if err := sendInterceptionKey(combo.MainKey.Scancode, combo.MainKey.IsExtended, true); err != nil {
		return err
	}
	time.Sleep(5 * time.Millisecond)

	// Release modifiers (reverse order)
	for i := len(combo.Modifiers) - 1; i >= 0; i-- {
		mod := combo.Modifiers[i]
		if err := sendInterceptionKey(mod.Scancode, mod.IsExtended, true); err != nil {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}

	return nil
}

// SendKeyVirtualCombo envia um KeyCombo convertido para Interception
func SendKeyVirtualCombo(combo KeyCombo) error {
	return SendKeyVirtual(combo.RawString)
}

// SpamKeyVirtual envia múltiplas teclas via teclado virtual
func SpamKeyVirtual(keyStr string, count int, interval time.Duration) error {
	for i := 0; i < count; i++ {
		if err := SendKeyVirtual(keyStr); err != nil {
			return err
		}
		if i < count-1 && interval > 0 {
			time.Sleep(interval)
		}
	}
	return nil
}

// SpamKeyVirtualFast spam rápido mantendo modifiers
func SpamKeyVirtualFast(keyStr string, count int) error {
	if !IsInterceptionAvailable() {
		return fmt.Errorf("interception not available")
	}

	combo := ParseKeyComboInterception(keyStr)

	virtualKeyboard.mu.Lock()
	defer virtualKeyboard.mu.Unlock()

	// Press modifiers
	for _, mod := range combo.Modifiers {
		sendInterceptionKey(mod.Scancode, mod.IsExtended, false)
	}
	time.Sleep(5 * time.Millisecond)

	// Spam main key
	for i := 0; i < count; i++ {
		sendInterceptionKey(combo.MainKey.Scancode, combo.MainKey.IsExtended, false)
		time.Sleep(8 * time.Millisecond)
		sendInterceptionKey(combo.MainKey.Scancode, combo.MainKey.IsExtended, true)
		time.Sleep(8 * time.Millisecond)
	}

	// Release modifiers
	for i := len(combo.Modifiers) - 1; i >= 0; i-- {
		mod := combo.Modifiers[i]
		sendInterceptionKey(mod.Scancode, mod.IsExtended, true)
	}

	return nil
}

func SendKey(keyStr string) {
	if IsInterceptionAvailable() {
		if err := SendKeyVirtual(keyStr); err != nil {
			combo := ParseKeyCombo(keyStr)
			SendKeyComboClean(combo)
		}
	} else {
		combo := ParseKeyCombo(keyStr)
		SendKeyComboClean(combo)
	}
}

// SpamKey spam de teclas com fallback automático
func SpamKey(keyStr string, count int, interval time.Duration) {
	if IsInterceptionAvailable() {
		SpamKeyVirtual(keyStr, count, interval)
	} else {
		combo := ParseKeyCombo(keyStr)
		SpamKeyCombo(combo, count, interval)
	}
}

// SpamKeyFast spam rápido com fallback automático
func SpamKeyFast(keyStr string, count int) {
	if IsInterceptionAvailable() {
		SpamKeyVirtualFast(keyStr, count)
		} else {
		combo := ParseKeyCombo(keyStr)
		SpamKeyComboFast(combo, count)
	}
}