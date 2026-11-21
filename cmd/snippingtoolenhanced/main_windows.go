// SPDX-License-Identifier: Apache-2.0

//go:build windows

// Command SnippingToolEnhanced is the whole application in one executable:
// the editor window, the capture overlay and the background Print Screen
// listener. Build with -ldflags "-H windowsgui".
package main

import (
	"os"

	"golang.org/x/sys/windows/registry"

	"snippingtoolenhanced/internal/crash"
	"snippingtoolenhanced/internal/editor"
	"snippingtoolenhanced/internal/host"
	"snippingtoolenhanced/internal/win"
)

func main() {
	if blockedByPolicy() {
		win.MessageBox(0, "Snipping Tool Enhanced has been turned off by your organization.", "Snipping Tool Enhanced", win.MB_ICONWARNING)
		os.Exit(3)
	}
	args := os.Args[1:]
	role := "editor"
	if len(args) >= 1 && args[0] == "--background" {
		role = "background"
	}
	crash.Install(role)
	switch {
	case len(args) >= 1 && args[0] == "--background":
		os.Exit(host.Run())
	case len(args) >= 2 && args[0] == "--editor":
		os.Exit(editor.Run(args[1], shortcutStatus))
	default:
		// Start menu, taskbar or double-click: open the editor window (§2);
		// make sure Print Screen keeps working after it closes.
		host.Start()
		path := ""
		if len(args) == 1 {
			path = args[0] // "Open with" an image
		}
		os.Exit(editor.Run(path, shortcutStatus))
	}
}

// shortcutStatus tells the editor which shortcuts work.
func shortcutStatus() editor.Shortcuts {
	s := host.ReadShortcuts()
	return editor.Shortcuts{PrintScreen: s.PrintScreen, PrintScreenWindows: host.PrintScreenStatus() == "windows", Snip: s.Snip, Record: s.Record}
}

// blockedByPolicy honors "Do not allow Snipping Tool to run" (white paper §8,
// registry value DisableSnippingTool) for the machine or the user.
func blockedByPolicy() bool {
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		k, err := registry.OpenKey(root, `SOFTWARE\Policies\Microsoft\TabletPC`, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := k.GetIntegerValue("DisableSnippingTool")
		k.Close()
		if err == nil && v != 0 {
			return true
		}
	}
	return false
}
