// SPDX-License-Identifier: Apache-2.0

//go:build windows

// Package output resolves folders and writes capture files (white paper §2, §3, §6).
package output

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"

	"snippingtoolenhanced/internal/imaging"
	"snippingtoolenhanced/internal/naming"
)

// DataDir is %LOCALAPPDATA%\SnippingToolEnhanced.
func DataDir() string {
	base, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
	if err != nil {
		base = os.Getenv("LOCALAPPDATA")
	}
	return filepath.Join(base, "SnippingToolEnhanced")
}

// SettingsPath is the settings file.
func SettingsPath() string { return filepath.Join(DataDir(), "settings.json") }

// CapturesDir holds the working copy of each capture that the editor opens.
func CapturesDir() string { return filepath.Join(DataDir(), "Captures") }

// DefaultScreenshotsFolder is Pictures\Screenshots (follows Known Folder redirection).
func DefaultScreenshotsFolder() string {
	if p, err := windows.KnownFolderPath(windows.FOLDERID_Screenshots, 0); err == nil && p != "" {
		return p
	}
	p, err := windows.KnownFolderPath(windows.FOLDERID_Pictures, 0)
	if err != nil {
		p = filepath.Join(os.Getenv("USERPROFILE"), "Pictures")
	}
	return filepath.Join(p, "Screenshots")
}

// ScreenshotsFolder returns the configured folder or the default.
func ScreenshotsFolder(configured string) string {
	if strings.TrimSpace(configured) != "" {
		return configured
	}
	return DefaultScreenshotsFolder()
}

// DefaultRecordingsFolder is Videos\Screen Recordings.
func DefaultRecordingsFolder() string {
	p, err := windows.KnownFolderPath(windows.FOLDERID_Videos, 0)
	if err != nil {
		p = filepath.Join(os.Getenv("USERPROFILE"), "Videos")
	}
	return filepath.Join(p, "Screen Recordings")
}

// RecordingsWorkDir holds recordings that have not been saved yet.
func RecordingsWorkDir() string { return filepath.Join(DataDir(), "Recordings") }

// IsUnsavedRecording reports whether path is in the working folder.
func IsUnsavedRecording(path string) bool {
	rel, err := filepath.Rel(RecordingsWorkDir(), path)
	return err == nil && !strings.HasPrefix(rel, "..")
}

// NewRecordingPath returns where a new recording is written: the recordings
// folder when auto-save is on, otherwise the app's working folder.
func NewRecordingPath(autoSave bool, folder string) string {
	dir := RecordingsWorkDir()
	go Cleanup(dir, 7*24*time.Hour)
	if autoSave {
		dir = folder
		if strings.TrimSpace(dir) == "" {
			dir = DefaultRecordingsFolder()
		}
	}
	_ = os.MkdirAll(dir, 0o755)
	name := naming.Expand("Screen Recording {date} {time}", naming.Info{Time: time.Now()})
	return naming.Unique(dir, name, ".mp4", exists)
}

// SaveNew writes img as PNG into dir as "Screenshot YYYY-MM-DD HHMMSS.png".
func SaveNew(img image.Image, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := naming.Expand("Screenshot {date} {time}", naming.Info{Time: time.Now()})
	path := naming.Unique(dir, name, ".png", exists)
	return path, Write(img, path)
}

// Write encodes by extension (.png, .jpg/.jpeg, .gif) and writes atomically.
func Write(img image.Image, path string) error {
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	data, err := imaging.Encode(img, format, 92)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".partial"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Cleanup deletes files older than maxAge in dir.
func Cleanup(dir string, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cut := time.Now().Add(-maxAge)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && !e.IsDir() && info.ModTime().Before(cut) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }
