// SPDX-License-Identifier: Apache-2.0

//go:build windows

package visualsearch

import (
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"snippingtoolenhanced/internal/output"
	"snippingtoolenhanced/internal/win"
)

// Dir holds the temporary search pages (they contain the snip).
func Dir() string { return filepath.Join(output.DataDir(), "VisualSearch") }

// Cleanup deletes search pages older than 10 minutes; the browser has read
// them long before.
func Cleanup() { output.Cleanup(Dir(), 10*time.Minute) }

// Open searches the web for img with engine id in the default browser.
func Open(id string, img image.Image) error {
	jpg, err := Encode(img)
	if err != nil {
		return err
	}
	Cleanup()
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	p := filepath.Join(Dir(), fmt.Sprintf("search-%d.html", time.Now().UnixNano()))
	if err := os.WriteFile(p, Page(id, jpg), 0o600); err != nil {
		return err
	}
	return openInBrowser(p)
}

// openInBrowser opens the page with the app that handles web links (the
// default browser), not whatever is set to open .html files.
func openInBrowser(path string) error {
	u := fileURL(path)
	cmd := win.AssocCommand("https")
	exe, _ := splitCommand(cmd)
	if exe != "" && strings.HasSuffix(strings.ToLower(exe), ".exe") {
		line := fillCommand(cmd, u)
		c := exec.Command(exe)
		c.SysProcAttr = &syscall.SysProcAttr{CmdLine: line}
		if err := c.Start(); err == nil {
			go c.Wait()
			return nil
		}
	}
	return win.ShellOpen(u)
}
