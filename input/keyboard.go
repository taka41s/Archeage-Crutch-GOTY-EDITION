package input

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procKeybd_event         = user32.NewProc("keybd_event")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procSendMessageW        = user32.NewProc("SendMessageW")
	procFindWindowW         = user32.NewProc("FindWindowW")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procMapVirtualKeyW      = user32.NewProc("MapVirtualKeyW")
	procGetAsyncKeyState    = user32.NewProc("GetAsyncKeyState")
	procSendInput           = user32.NewProc("SendInput")
)

const (
	KEYEVENTF_KEYDOWN     = 0x0000
	KEYEVENTF_KEYUP       = 0x0002
	KEYEVENTF_EXTENDEDKEY = 0x0001
	KEYEVENTF_SCANCODE    = 0x0008

	WM_KEYDOWN    = 0x0100
	WM_KEYUP      = 0x0101
	WM_CHAR       = 0x0102
	WM_SYSKEYDOWN = 0x0104
	WM_SYSKEYUP   = 0x0105

	MAPVK_VK_TO_VSC = 0

	INPUT_KEYBOARD = 1
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

// KEYBDINPUT estrutura para SendInput
type KEYBDINPUT struct {
	Vk        uint16
	Scan      uint16
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

// INPUT estrutura para SendInput
type INPUT struct {
	Type uint32
	Ki   KEYBDINPUT
	_    [8]byte // padding para union
}

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

// Todos os modificadores para verificar estado
var allModifiers = []uint8{
	VK_SHIFT, VK_CONTROL, VK_ALT,
	VK_LSHIFT, VK_RSHIFT,
	VK_LCONTROL, VK_RCONTROL,
	VK_LALT, VK_RALT,
}

// GameWindow gerencia a janela do jogo
type GameWindow struct {
	hwnd       windows.HWND
	className  string
	windowName string
	mu         sync.RWMutex
}

var gameWindow = &GameWindow{
	windowName: "ArcheAge",
}

// Mutex para evitar conflitos de input
var inputMutex sync.Mutex

// KeyCombo representa uma combinação de teclas
type KeyCombo struct {
	Modifiers []uint8
	MainKey   uint8
	RawString string
}

// SetGameWindow configura a janela alvo
func SetGameWindow(className, windowName string) {
	gameWindow.mu.Lock()
	defer gameWindow.mu.Unlock()
	gameWindow.className = className
	gameWindow.windowName = windowName
	gameWindow.hwnd = 0
}

func FindGameWindow() windows.HWND {
	gameWindow.mu.Lock()
	defer gameWindow.mu.Unlock()

	if gameWindow.hwnd != 0 {
		return gameWindow.hwnd
	}

	var hwnd uintptr

	if gameWindow.className != "" {
		classNamePtr, _ := windows.UTF16PtrFromString(gameWindow.className)
		hwnd, _, _ = procFindWindowW.Call(uintptr(unsafe.Pointer(classNamePtr)), 0)
	}

	if hwnd == 0 && gameWindow.windowName != "" {
		windowNamePtr, _ := windows.UTF16PtrFromString(gameWindow.windowName)
		hwnd, _, _ = procFindWindowW.Call(0, uintptr(unsafe.Pointer(windowNamePtr)))
	}

	gameWindow.hwnd = windows.HWND(hwnd)
	return gameWindow.hwnd
}

func GetGameHWND() windows.HWND {
	hwnd := FindGameWindow()
	if hwnd == 0 {
		alternativeNames := []string{"ArcheAge", "ARCHEAGE", "archeage"}
		for _, name := range alternativeNames {
			windowNamePtr, _ := windows.UTF16PtrFromString(name)
			h, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(windowNamePtr)))
			if h != 0 {
				gameWindow.mu.Lock()
				gameWindow.hwnd = windows.HWND(h)
				gameWindow.mu.Unlock()
				return windows.HWND(h)
			}
		}
	}
	return hwnd
}

func IsKeyPressed(vkCode uint8) bool {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vkCode))
	return ret&0x8000 != 0
}

func GetPressedModifiers() []uint8 {
	pressed := make([]uint8, 0, 4)
	
	if IsKeyPressed(VK_LSHIFT) {
		pressed = append(pressed, VK_LSHIFT)
	}
	if IsKeyPressed(VK_RSHIFT) {
		pressed = append(pressed, VK_RSHIFT)
	}
	if IsKeyPressed(VK_LCONTROL) {
		pressed = append(pressed, VK_LCONTROL)
	}
	if IsKeyPressed(VK_RCONTROL) {
		pressed = append(pressed, VK_RCONTROL)
	}
	if IsKeyPressed(VK_LALT) {
		pressed = append(pressed, VK_LALT)
	}
	if IsKeyPressed(VK_RALT) {
		pressed = append(pressed, VK_RALT)
	}
	
	return pressed
}

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

func sendInputKey(vkCode uint8, isKeyUp bool) {
	var input INPUT
	input.Type = INPUT_KEYBOARD
	input.Ki.Vk = uint16(vkCode)
	
	scanCode, _, _ := procMapVirtualKeyW.Call(uintptr(vkCode), MAPVK_VK_TO_VSC)
	input.Ki.Scan = uint16(scanCode)
	
	if isKeyUp {
		input.Ki.Flags = KEYEVENTF_KEYUP
	} else {
		input.Ki.Flags = 0
	}

	procSendInput.Call(1, uintptr(unsafe.Pointer(&input)), unsafe.Sizeof(input))
}

func isModifierInCombo(mod uint8, combo KeyCombo) bool {
	for _, m := range combo.Modifiers {
		if m == mod {
			return true
		}
		if m == VK_SHIFT && (mod == VK_LSHIFT || mod == VK_RSHIFT) {
			return true
		}
		if m == VK_CONTROL && (mod == VK_LCONTROL || mod == VK_RCONTROL) {
			return true
		}
		if m == VK_ALT && (mod == VK_LALT || mod == VK_RALT) {
			return true
		}
	}
	return false
}

func SendKeyComboClean(combo KeyCombo) {
	inputMutex.Lock()
	defer inputMutex.Unlock()

	userModifiers := GetPressedModifiers()
	
	modifiersToRelease := make([]uint8, 0)
	for _, mod := range userModifiers {
		if !isModifierInCombo(mod, combo) {
			modifiersToRelease = append(modifiersToRelease, mod)
		}
	}

	modifiersToPress := make([]uint8, 0)
	for _, mod := range combo.Modifiers {
		alreadyPressed := false
		for _, userMod := range userModifiers {
			if mod == userMod {
				alreadyPressed = true
				break
			}
			if mod == VK_SHIFT && (userMod == VK_LSHIFT || userMod == VK_RSHIFT) {
				alreadyPressed = true
				break
			}
			if mod == VK_CONTROL && (userMod == VK_LCONTROL || userMod == VK_RCONTROL) {
				alreadyPressed = true
				break
			}
			if mod == VK_ALT && (userMod == VK_LALT || userMod == VK_RALT) {
				alreadyPressed = true
				break
			}
		}
		if !alreadyPressed {
			modifiersToPress = append(modifiersToPress, mod)
		}
	}

	for _, mod := range modifiersToRelease {
		sendInputKey(mod, true) // key up
	}
	
	if len(modifiersToRelease) > 0 {
		time.Sleep(5 * time.Millisecond)
	}

	for _, mod := range modifiersToPress {
		sendInputKey(mod, false)
	}
	
	if len(modifiersToPress) > 0 {
		time.Sleep(5 * time.Millisecond)
	}

	sendInputKey(combo.MainKey, false) // key down
	time.Sleep(15 * time.Millisecond)
	sendInputKey(combo.MainKey, true) // key up
	
	time.Sleep(5 * time.Millisecond)

	for i := len(modifiersToPress) - 1; i >= 0; i-- {
		sendInputKey(modifiersToPress[i], true) // key up
	}
	
	if len(modifiersToPress) > 0 {
		time.Sleep(5 * time.Millisecond)
	}

	for _, mod := range modifiersToRelease {
		if IsKeyPressed(mod) {
			sendInputKey(mod, false)
		}
	}
}

func SendKeyCombo(combo KeyCombo) {
	SendKeyComboClean(combo)
}

func SpamKeyCombo(combo KeyCombo, count int, interval time.Duration) {
	for i := 0; i < count; i++ {
		SendKeyComboClean(combo)
		if i < count-1 && interval > 0 {
			time.Sleep(interval)
		}
	}
}

func SpamKeyComboFast(combo KeyCombo, count int) {
	inputMutex.Lock()
	defer inputMutex.Unlock()

	userModifiers := GetPressedModifiers()
	modifiersToRelease := make([]uint8, 0)
	for _, mod := range userModifiers {
		if !isModifierInCombo(mod, combo) {
			modifiersToRelease = append(modifiersToRelease, mod)
		}
	}

	for _, mod := range modifiersToRelease {
		sendInputKey(mod, true)
	}
	
	if len(modifiersToRelease) > 0 {
		time.Sleep(3 * time.Millisecond)
	}

	for _, mod := range combo.Modifiers {
		sendInputKey(mod, false)
	}
	
	if len(combo.Modifiers) > 0 {
		time.Sleep(3 * time.Millisecond)
	}

	for i := 0; i < count; i++ {
		sendInputKey(combo.MainKey, false)
		time.Sleep(8 * time.Millisecond)
		sendInputKey(combo.MainKey, true)
		time.Sleep(8 * time.Millisecond)
	}

	for i := len(combo.Modifiers) - 1; i >= 0; i-- {
		sendInputKey(combo.Modifiers[i], true)
	}
	
	time.Sleep(3 * time.Millisecond)

	for _, mod := range modifiersToRelease {
		if IsKeyPressed(mod) {
			sendInputKey(mod, false)
		}
	}
}

func IsGameFocused() bool {
	hwnd := GetGameHWND()
	if hwnd == 0 {
		return false
	}
	foreground, _, _ := procGetForegroundWindow.Call()
	return windows.HWND(foreground) == hwnd
}

func FocusGame() bool {
	hwnd := GetGameHWND()
	if hwnd == 0 {
		return false
	}
	ret, _, _ := procSetForegroundWindow.Call(uintptr(hwnd))
	return ret != 0
}