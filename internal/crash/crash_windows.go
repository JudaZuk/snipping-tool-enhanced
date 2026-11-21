// SPDX-License-Identifier: Apache-2.0

//go:build windows

// Package crash records fatal errors to %LOCALAPPDATA%\SnippingToolEnhanced\crash.log
// so problems on users' machines can be diagnosed, and turns panics in UI
// handlers into ordinary errors instead of closing the app.
package crash

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"golang.org/x/sys/windows"
)

// Path is the crash log location.
func Path() string {
	base, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
	if err != nil {
		base = os.Getenv("LOCALAPPDATA")
	}
	return filepath.Join(base, "SnippingToolEnhanced", "crash.log")
}

// Install sends the Go runtime's fatal-error output (panics, and access
// violations in Windows code) to the crash log.
func Install(role string) {
	p := Path()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if st, err := os.Stat(p); err == nil && st.Size() > 2<<20 {
		_ = os.Remove(p)
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	fmt.Fprintf(f, "--- %s start %s pid %d\n", role, time.Now().Format(time.RFC3339), os.Getpid())
	_ = debug.SetCrashOutput(f, debug.CrashOptions{})
	_ = windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(f.Fd()))
	os.Stderr = f
}

// Log appends a message with a stack trace.
func Log(format string, a ...any) {
	f, err := os.OpenFile(Path(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s "+format+"\n%s\n", append([]any{time.Now().Format(time.RFC3339)}, append(a, debug.Stack())...)...)
}

// Recover converts a panic into an error (use as: defer crash.Recover("name", &err)).
func Recover(name string, err *error) {
	if r := recover(); r != nil {
		Log("panic in %s: %v", name, r)
		if err != nil {
			*err = fmt.Errorf("internal error in %s: %v (details in %s)", name, r, Path())
		}
	}
}
