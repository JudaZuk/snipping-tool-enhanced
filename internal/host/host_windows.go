// SPDX-License-Identifier: Apache-2.0

//go:build windows

// Package host keeps Print Screen working while no editor window is open
// (white paper §2 "System entry points"). It has no window of its own.
package host

import (
	"encoding/json"
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"snippingtoolenhanced/internal/clipboard"
	"snippingtoolenhanced/internal/handoff"
	"snippingtoolenhanced/internal/imaging"
	"snippingtoolenhanced/internal/notify"
	"snippingtoolenhanced/internal/output"
	"snippingtoolenhanced/internal/overlay"
	"snippingtoolenhanced/internal/settings"
	"snippingtoolenhanced/internal/share"
	"snippingtoolenhanced/internal/snip"
	"snippingtoolenhanced/internal/win"
)

const (
	className = "SnippingToolEnhancedHost"
	mutexName = `Local\SnippingToolEnhanced.Host`
	wmSnip    = win.WM_APP + 1
	modAlt    = 0x0001
	modWin    = 0x0008
	modNoRep  = 0x4000
	vkSnap    = 0x2C

	hkPrintScreen = 1 // Print Screen: snip
	hkSnip        = 2 // Win + Alt + S: snip
	hkRecord      = 3 // Win + Alt + V: record
)

// Shortcuts records which global shortcuts the background process holds.
type Shortcuts struct {
	PrintScreen bool `json:"printScreen"`
	Snip        bool `json:"snip"`
	Record      bool `json:"record"`
}

func shortcutsPath() string { return filepath.Join(output.DataDir(), "shortcuts.json") }

// ReadShortcuts returns what the background process registered. If it has
// not written its status yet, the shortcuts are assumed to work.
func ReadShortcuts() Shortcuts {
	s := Shortcuts{PrintScreen: true, Snip: true, Record: true}
	if b, err := os.ReadFile(shortcutsPath()); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

var (
	hostHwnd windows.HWND
	busy     bool
)

var proc = syscall.NewCallback(func(h, msg, wp, lp uintptr) uintptr {
	switch uint32(msg) {
	case win.WM_HOTKEY:
		win.PostMessage(windows.HWND(h), wmSnip, wp, 0)
		return 0
	case wmSnip:
		kind := overlay.KindSnip
		if wp == hkRecord {
			kind = overlay.KindRecord
		}
		doSnip(kind)
		return 0
	}
	return win.DefWindowProc(windows.HWND(h), uint32(msg), wp, lp)
})

// Running reports whether the host is already running in this session.
func Running() bool {
	h, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(mutexName))
	if h != 0 {
		windows.CloseHandle(h)
	}
	return err == windows.ERROR_ALREADY_EXISTS
}

// Start launches the host in the background if it is not running.
func Start() {
	if Running() {
		return
	}
	if exe, err := os.Executable(); err == nil {
		_ = exec.Command(exe, "--background").Start()
	}
}

// PrintScreenStatus returns "windows" when Windows keeps Print Screen for its
// own screen capture (Settings > Accessibility > Keyboard), otherwise "ours".
func PrintScreenStatus() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Control Panel\Keyboard`, registry.QUERY_VALUE)
	if err != nil {
		return "windows"
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("PrintScreenKeyForSnippingEnabled")
	if err != nil || v != 0 {
		return "windows" // on by default in Windows 11
	}
	return "ours"
}

// Run registers Print Screen and waits for it. Returns when the hotkey
// cannot be registered or another host is running.
func Run() int {
	runtime.LockOSThread()
	win.SetDpiAwarenessPerMonitorV2()
	m, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(mutexName))
	if err == windows.ERROR_ALREADY_EXISTS {
		windows.CloseHandle(m)
		return 0
	}
	defer windows.CloseHandle(m)
	if err := win.RegisterClass(className, proc, win.IDC_ARROW, 0); err != nil {
		return 10
	}
	hostHwnd, err = win.CreateWindow(0, className, "Snipping Tool Enhanced", win.WS_OVERLAPPED, 0, 0, 0, 0)
	if err != nil {
		return 10
	}
	st := Shortcuts{
		PrintScreen: win.RegisterHotKey(hostHwnd, hkPrintScreen, modNoRep, vkSnap) == nil && PrintScreenStatus() == "ours",
		Snip:        win.RegisterHotKey(hostHwnd, hkSnip, modWin|modAlt|modNoRep, 'S') == nil,
		Record:      win.RegisterHotKey(hostHwnd, hkRecord, modWin|modAlt|modNoRep, 'V') == nil,
	}
	if b, err := json.Marshal(st); err == nil {
		_ = os.MkdirAll(output.DataDir(), 0o755)
		_ = os.WriteFile(shortcutsPath(), b, 0o644)
	}
	if !st.PrintScreen && !st.Snip && !st.Record {
		return 0 // nothing to listen for
	}
	var msg win.Msg
	for win.GetMessage(&msg) {
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
	return 0
}

func doSnip(kind overlay.Kind) {
	if busy {
		return
	}
	busy = true
	defer func() { busy = false }()
	s := settings.Load(output.SettingsPath())
	dark := snip.Dark(s)
	// Print Screen and Win+Alt+S open the overlay for a snip, Win+Alt+V for a recording.
	mode := overlay.Mode(s.Mode)
	if kind == overlay.KindRecord && mode != overlay.Window {
		mode = overlay.Rectangle
	} else if kind == overlay.KindSnip && s.Kind == "record" {
		mode = overlay.Rectangle
	}
	res := snip.Interactive(kind, mode, 0, s)
	if res.Outcome == overlay.Captured && res.Kind == overlay.KindRecord {
		openEditor(res.VideoPath, s)
		return
	}
	if res.Err != nil && res.Outcome != overlay.Captured {
		notify.Show(nil, "Snipping Tool Enhanced", res.Err.Error(), dark, nil)
		return
	}
	switch res.Outcome {
	case snip.TextExtracted:
		showText(res, dark)
		return
	case overlay.ColorPicked:
		c, _ := imaging.ParseHexColor(res.ColorText)
		if len(res.ColorText) == 0 || res.ColorText[0] != '#' {
			c = color.RGBA{128, 128, 128, 255}
		}
		sw := image.NewRGBA(image.Rect(0, 0, 64, 64))
		for i := 0; i < len(sw.Pix); i += 4 {
			sw.Pix[i], sw.Pix[i+1], sw.Pix[i+2], sw.Pix[i+3] = c.R, c.G, c.B, 255
		}
		notify.Show(sw, "Color copied", res.ColorText, dark, nil)
		return
	case overlay.Captured:
	default:
		return
	}
	// Quick markup finished with Save (already saved), Share or Copy.
	switch res.Action {
	case overlay.ActSave:
		return
	case overlay.ActShare:
		p, err := snip.ShareFile(res.Image)
		if err == nil {
			err = share.At(res.Area.Min.Add(res.Area.Max).Div(2), p)
		}
		if err != nil {
			notify.Show(nil, "The snip could not be shared", err.Error(), dark, nil)
		}
		return
	case overlay.ActCopy:
		s.AutoCopyChanges = true
	}
	d, err := snip.Deliver(res.Image, s)
	if d.WorkPath == "" {
		if err != nil {
			notify.Show(nil, "The snip could not be saved", err.Error(), dark, nil)
		}
		return
	}
	open := func() { openEditor(d.WorkPath, s) }
	if s.OpenEditorAfterSnip && res.Action == "" {
		open()
		return
	}
	title := "Snip copied to clipboard"
	if !d.Copied {
		title = "Snip captured"
		if d.SavedPath != "" {
			title = "Snip saved"
		}
	}
	notify.Show(res.Image, title, "Select here to mark up and share the image.", dark, open)
}

// showText reports a Text Extractor result. When the text was not copied
// automatically, selecting the notification copies it.
func showText(res snip.Result, dark bool) {
	preview := strings.TrimSpace(strings.SplitN(res.Text, "\n", 2)[0])
	if len([]rune(preview)) > 48 {
		preview = string([]rune(preview)[:47]) + "…"
	}
	switch {
	case res.Text == "":
		notify.Show(res.Image, "No text found", "Try a larger area or clearer text.", dark, nil)
	case res.Copied:
		notify.Show(res.Image, "Text copied", preview, dark, nil)
	default:
		text := res.Text
		notify.Show(res.Image, "Text found – select to copy", preview, dark, func() { _ = clipboard.SetText(text) })
	}
}

// openEditor shows path (image or recording) in the open editor window, or a new one.
func openEditor(path string, s settings.Settings) {
	if !s.MultipleWindows && handoff.Send(path) {
		return
	}
	if exe, err := os.Executable(); err == nil {
		_ = exec.Command(exe, "--editor", path).Start()
	}
}
