package tools

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	procSetCursorPos    = user32.NewProc("SetCursorPos")
	procMouseEvent      = user32.NewProc("mouse_event")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procSendInput       = user32.NewProc("SendInput")
	procGetCursorPos    = user32.NewProc("GetCursorPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procShowWindow      = user32.NewProc("ShowWindow")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")

	// targetWindow stores the window handle that should receive automation input
	targetWindow uintptr
	// appWindow stores VisionChat's own window handle
	appWindow uintptr
)

const (
	swMinimize = 6
	swRestore  = 9
)

// SetAppWindow records VisionChat's window handle so we can minimize it during automation.
func SetAppWindow(hwnd uintptr) {
	appWindow = hwnd
}

// FocusTarget minimizes VisionChat so the target app gets focus.
// Call this ONCE before a batch of tool executions.
func FocusTarget() {
	if appWindow != 0 {
		// Minimize VisionChat — the previous window automatically gets focus
		procShowWindow.Call(appWindow, uintptr(swMinimize))
		time.Sleep(400 * time.Millisecond)
	}
}

// RestoreApp brings VisionChat back after automation.
func RestoreApp() {
	if appWindow != 0 {
		procShowWindow.Call(appWindow, uintptr(swRestore))
		time.Sleep(200 * time.Millisecond)
		procSetForegroundWindow.Call(appWindow)
	}
}

// CaptureTargetWindow saves the current foreground window as the automation target.
func CaptureTargetWindow() {
	fg, _, _ := procGetForegroundWindow.Call()
	if fg != appWindow {
		targetWindow = fg
	}
}

// CaptureAppWindow records the current foreground window as VisionChat's own window.
func CaptureAppWindow() {
	fg, _, _ := procGetForegroundWindow.Call()
	appWindow = fg
}

const (
	mouseEventFLeftDown  = 0x0002
	mouseEventFLeftUp    = 0x0004
	mouseEventFRightDown = 0x0008
	mouseEventFRightUp   = 0x0010

	inputKeyboard = 1
	keyEventFUp   = 0x0002
	keyEventFUnicode = 0x0004

	smCxScreen = 0
	smCyScreen = 1
)

type point struct {
	X, Y int32
}

type keyboardInput struct {
	Type uint32
	Ki   keybdInput
	_    [8]byte // padding
}

type keybdInput struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

// GetScreenSize returns the primary screen resolution.
func GetScreenSize() (int, int) {
	cx, _, _ := procGetSystemMetrics.Call(uintptr(smCxScreen))
	cy, _, _ := procGetSystemMetrics.Call(uintptr(smCyScreen))
	return int(cx), int(cy)
}

// GetCursorPosition returns current mouse position.
func GetCursorPosition() (int, int) {
	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	return int(pt.X), int(pt.Y)
}

// MoveMouse moves the cursor to absolute screen coordinates.
func MoveMouse(x, y int) {
	procSetCursorPos.Call(uintptr(x), uintptr(y))
}

// Click performs a left click at the given screen coordinates.
func Click(x, y int) {
	MoveMouse(x, y)
	time.Sleep(50 * time.Millisecond)
	procMouseEvent.Call(mouseEventFLeftDown, 0, 0, 0, 0)
	time.Sleep(30 * time.Millisecond)
	procMouseEvent.Call(mouseEventFLeftUp, 0, 0, 0, 0)
}

// DoubleClick performs a double left click.
func DoubleClick(x, y int) {
	Click(x, y)
	time.Sleep(80 * time.Millisecond)
	Click(x, y)
}

// RightClick performs a right click.
func RightClick(x, y int) {
	MoveMouse(x, y)
	time.Sleep(50 * time.Millisecond)
	procMouseEvent.Call(mouseEventFRightDown, 0, 0, 0, 0)
	time.Sleep(30 * time.Millisecond)
	procMouseEvent.Call(mouseEventFRightUp, 0, 0, 0, 0)
}

// TypeText types a string character by character using Unicode input.
func TypeText(text string) {
	for _, r := range text {
		typeRune(r)
		time.Sleep(20 * time.Millisecond)
	}
}

func typeRune(r rune) {
	encoded := utf16.Encode([]rune{r})
	for _, code := range encoded {
		var inputs [2]keyboardInput

		// Key down
		inputs[0] = keyboardInput{
			Type: inputKeyboard,
			Ki: keybdInput{
				WScan:   code,
				DwFlags: keyEventFUnicode,
			},
		}
		// Key up
		inputs[1] = keyboardInput{
			Type: inputKeyboard,
			Ki: keybdInput{
				WScan:   code,
				DwFlags: keyEventFUnicode | keyEventFUp,
			},
		}

		procSendInput.Call(
			2,
			uintptr(unsafe.Pointer(&inputs[0])),
			uintptr(unsafe.Sizeof(inputs[0])),
		)
	}
}

// PressKey presses a virtual key by name (enter, tab, escape, backspace, etc.)
func PressKey(key string) {
	vk, ok := keyMap[strings.ToLower(key)]
	if !ok {
		return
	}

	var inputs [2]keyboardInput
	inputs[0] = keyboardInput{
		Type: inputKeyboard,
		Ki:   keybdInput{WVk: vk},
	}
	inputs[1] = keyboardInput{
		Type: inputKeyboard,
		Ki:   keybdInput{WVk: vk, DwFlags: keyEventFUp},
	}

	procSendInput.Call(
		2,
		uintptr(unsafe.Pointer(&inputs[0])),
		uintptr(unsafe.Sizeof(inputs[0])),
	)
}

// HotKey presses a key combination (e.g., ctrl+a, ctrl+l).
func HotKey(modifier, key string) {
	modVk, ok1 := keyMap[strings.ToLower(modifier)]
	keyVk, ok2 := keyMap[strings.ToLower(key)]
	if !ok1 || !ok2 {
		return
	}

	var inputs [4]keyboardInput
	// Modifier down
	inputs[0] = keyboardInput{Type: inputKeyboard, Ki: keybdInput{WVk: modVk}}
	// Key down
	inputs[1] = keyboardInput{Type: inputKeyboard, Ki: keybdInput{WVk: keyVk}}
	// Key up
	inputs[2] = keyboardInput{Type: inputKeyboard, Ki: keybdInput{WVk: keyVk, DwFlags: keyEventFUp}}
	// Modifier up
	inputs[3] = keyboardInput{Type: inputKeyboard, Ki: keybdInput{WVk: modVk, DwFlags: keyEventFUp}}

	procSendInput.Call(
		4,
		uintptr(unsafe.Pointer(&inputs[0])),
		uintptr(unsafe.Sizeof(inputs[0])),
	)
}

var keyMap = map[string]uint16{
	"enter":     0x0D,
	"tab":       0x09,
	"escape":    0x1B,
	"esc":       0x1B,
	"backspace": 0x08,
	"delete":    0x2E,
	"space":     0x20,
	"up":        0x26,
	"down":      0x28,
	"left":      0x25,
	"right":     0x27,
	"home":      0x24,
	"end":       0x23,
	"pageup":    0x21,
	"pagedown":  0x22,
	"ctrl":      0x11,
	"alt":       0x12,
	"shift":     0x10,
	"f1":        0x70,
	"f2":        0x71,
	"f3":        0x72,
	"f4":        0x73,
	"f5":        0x74,
	"f11":       0x7A,
	"f12":       0x7B,
	"a": 0x41, "b": 0x42, "c": 0x43, "d": 0x44, "e": 0x45,
	"f": 0x46, "g": 0x47, "h": 0x48, "i": 0x49, "j": 0x4A,
	"k": 0x4B, "l": 0x4C, "m": 0x4D, "n": 0x4E, "o": 0x4F,
	"p": 0x50, "q": 0x51, "r": 0x52, "s": 0x53, "t": 0x54,
	"u": 0x55, "v": 0x56, "w": 0x57, "x": 0x58, "y": 0x59,
	"z": 0x5A,
}

// === High-level tools ===

func toolWebSearch(args map[string]any) ToolResult {
	site := strings.ToLower(getArg(args, "site"))
	query := getArg(args, "query")
	if query == "" {
		return ToolResult{Success: false, Error: "missing 'query' argument"}
	}

	encodedQuery := strings.ReplaceAll(query, " ", "+")
	var url string
	switch {
	case site == "youtube" || strings.Contains(site, "youtube.com"):
		url = "https://www.youtube.com/results?search_query=" + encodedQuery
	case site == "google" || strings.Contains(site, "google.com"):
		url = "https://www.google.com/search?q=" + encodedQuery
	case site == "twitter" || site == "x" || strings.Contains(site, "x.com"):
		url = "https://x.com/search?q=" + encodedQuery
	case site == "github" || strings.Contains(site, "github.com"):
		url = "https://github.com/search?q=" + encodedQuery
	case site == "amazon" || strings.Contains(site, "amazon"):
		url = "https://www.amazon.com.br/s?k=" + encodedQuery
	default:
		url = "https://www.google.com/search?q=" + encodedQuery + "+site:" + site
	}

	// Open URL directly in the default browser — no focus tricks needed
	cmd := exec.Command("cmd", "/c", "start", "", url)
	if err := cmd.Start(); err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("failed to open URL: %v", err)}
	}

	return ToolResult{Success: true, Output: fmt.Sprintf("Opened in browser: %s (query: %s)", site, query)}
}

func toolOpenURL(args map[string]any) ToolResult {
	url := getArg(args, "url")
	if url == "" {
		return ToolResult{Success: false, Error: "missing 'url' argument"}
	}

	cmd := exec.Command("cmd", "/c", "start", "", url)
	if err := cmd.Start(); err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("failed to open URL: %v", err)}
	}

	return ToolResult{Success: true, Output: fmt.Sprintf("Opened: %s", url)}
}

// === Low-level tool implementations ===

func toolClick(args map[string]any) ToolResult {
	imgX, imgY, err := parseCoords(args)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}
	realX, realY := scaleToScreen(imgX, imgY)
	Click(realX, realY)
	return ToolResult{Success: true, Output: fmt.Sprintf("Clicked at image(%d,%d) → screen(%d,%d)", imgX, imgY, realX, realY)}
}

func toolDoubleClick(args map[string]any) ToolResult {
	imgX, imgY, err := parseCoords(args)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}
	realX, realY := scaleToScreen(imgX, imgY)
	DoubleClick(realX, realY)
	return ToolResult{Success: true, Output: fmt.Sprintf("Double-clicked at image(%d,%d) → screen(%d,%d)", imgX, imgY, realX, realY)}
}

func toolTypeText(args map[string]any) ToolResult {
	text := getArg(args, "text")
	if text == "" {
		return ToolResult{Success: false, Error: "missing 'text' argument"}
	}
	TypeText(text)
	return ToolResult{Success: true, Output: fmt.Sprintf("Typed: %s", text)}
}

func toolPressKey(args map[string]any) ToolResult {
	key := getArg(args, "key")
	if key == "" {
		return ToolResult{Success: false, Error: "missing 'key' argument"}
	}
	PressKey(key)
	return ToolResult{Success: true, Output: fmt.Sprintf("Pressed key: %s", key)}
}

func toolHotKey(args map[string]any) ToolResult {
	modifier := getArg(args, "modifier")
	key := getArg(args, "key")
	if modifier == "" || key == "" {
		return ToolResult{Success: false, Error: "missing 'modifier' and/or 'key' arguments"}
	}
	HotKey(modifier, key)
	return ToolResult{Success: true, Output: fmt.Sprintf("Pressed hotkey: %s+%s", modifier, key)}
}

func toolMoveMouse(args map[string]any) ToolResult {
	imgX, imgY, err := parseCoords(args)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}
	realX, realY := scaleToScreen(imgX, imgY)
	MoveMouse(realX, realY)
	return ToolResult{Success: true, Output: fmt.Sprintf("Moved to image(%d,%d) → screen(%d,%d)", imgX, imgY, realX, realY)}
}

func toolScroll(args map[string]any) ToolResult {
	direction := getArg(args, "direction")
	if direction == "" {
		direction = "down"
	}
	amount := 3
	if a, err := strconv.Atoi(getArg(args, "amount")); err == nil {
		amount = a
	}

	var delta int
	if direction == "up" {
		delta = 120 * amount
	} else {
		delta = -120 * amount
	}

	procMouseEvent.Call(0x0800, 0, 0, uintptr(delta), 0)
	return ToolResult{Success: true, Output: fmt.Sprintf("Scrolled %s %d lines", direction, amount)}
}

func toolGetScreenInfo(args map[string]any) ToolResult {
	_ = args
	w, h := GetScreenSize()
	mx, my := GetCursorPosition()
	return ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Screen: %dx%d | Cursor at: (%d, %d)", w, h, mx, my),
	}
}

// imageWidth and imageHeight track the dimensions of the image the AI sees.
// Used to scale AI coordinates (image space) to real screen coordinates.
var (
	imageWidth  = 512
	imageHeight = 288 // 16:9 default
)

// SetImageDimensions updates the scaling reference from the captured frame.
func SetImageDimensions(w, h int) {
	if w > 0 {
		imageWidth = w
	}
	if h > 0 {
		imageHeight = h
	}
}

// scaleToScreen converts image-space coordinates to real screen coordinates.
func scaleToScreen(imgX, imgY int) (int, int) {
	screenW, screenH := GetScreenSize()
	realX := imgX * screenW / imageWidth
	realY := imgY * screenH / imageHeight
	return realX, realY
}

func parseCoords(args map[string]any) (int, int, error) {
	xStr := getArg(args, "x")
	yStr := getArg(args, "y")
	if xStr == "" || yStr == "" {
		return 0, 0, fmt.Errorf("missing 'x' and/or 'y' arguments")
	}
	x, err := strconv.Atoi(xStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid x coordinate: %s", xStr)
	}
	y, err := strconv.Atoi(yStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid y coordinate: %s", yStr)
	}
	return x, y, nil
}
