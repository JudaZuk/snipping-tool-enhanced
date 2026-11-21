// SPDX-License-Identifier: Apache-2.0

//go:build windows

// Package handoff passes a capture from the Print Screen host to an open
// editor window when "Multiple windows" is off (white paper §3).
package handoff

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"

	"snippingtoolenhanced/internal/output"
	"snippingtoolenhanced/internal/win"
)

const (
	editorMutex = `Local\SnippingToolEnhanced.Editor`
	eventName   = `Local\SnippingToolEnhanced.Handoff`
)

func file() string { return filepath.Join(output.DataDir(), "handoff.txt") }

// MarkEditorOpen records that an editor window exists; call the returned
// function when it closes.
func MarkEditorOpen() func() {
	h, _ := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(editorMutex))
	return func() {
		if h != 0 {
			windows.CloseHandle(h)
		}
	}
}

// EditorOpen reports whether any editor window is open in this session.
func EditorOpen() bool {
	h, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(editorMutex))
	if h != 0 {
		windows.CloseHandle(h)
	}
	return err == windows.ERROR_ALREADY_EXISTS
}

func event() windows.Handle {
	h, _ := windows.CreateEvent(nil, 0 /*auto-reset*/, 0, windows.StringToUTF16Ptr(eventName))
	return h
}

// Send hands path to an open editor. Returns false if no editor is open.
func Send(path string) bool {
	if !EditorOpen() {
		return false
	}
	if err := os.WriteFile(file(), []byte(path), 0o644); err != nil {
		return false
	}
	h := event()
	if h == 0 {
		return false
	}
	defer windows.CloseHandle(h)
	win.AllowAnyForeground()
	return windows.SetEvent(h) == nil
}

// Wait blocks until a capture is handed over and returns its path.
func Wait() (string, bool) {
	h := event()
	if h == 0 {
		return "", false
	}
	defer windows.CloseHandle(h)
	for {
		if ev, err := windows.WaitForSingleObject(h, windows.INFINITE); err != nil || ev != windows.WAIT_OBJECT_0 {
			return "", false
		}
		b, err := os.ReadFile(file())
		if err == nil && strings.TrimSpace(string(b)) != "" {
			return strings.TrimSpace(string(b)), true
		}
	}
}
