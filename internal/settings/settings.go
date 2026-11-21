// SPDX-License-Identifier: Apache-2.0

// Package settings stores the in-app settings from white paper §8
// (More > Settings) plus the last snip mode and delay chosen in the editor.
package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Version of the settings file; Load migrates older files.
const Version = 3

// Settings is persisted as JSON in %LOCALAPPDATA%\SnippingToolEnhanced\settings.json.
type Settings struct {
	Version int `json:"version"`

	// Snipping
	AutoCopyChanges     bool   `json:"autoCopyChanges"`     // Automatically copy changes to clipboard
	AutoSaveScreenshots bool   `json:"autoSaveScreenshots"` // Automatically save original screenshots
	SaveFolder          string `json:"saveFolder"`          // empty = Pictures\Screenshots
	AskToSaveEdited     bool   `json:"askToSaveEdited"`     // Ask to save edited screenshots
	MultipleWindows     bool   `json:"multipleWindows"`     // Multiple windows
	BorderEnabled       bool   `json:"borderEnabled"`       // Add border to each screenshot
	BorderColor         string `json:"borderColor"`
	BorderThickness     int    `json:"borderThickness"`
	OpenEditorAfterSnip bool   `json:"openEditorAfterSnip"` // open the editor directly after every capture (§3)

	// Text Extractor
	TextAutoCopy         bool `json:"textAutoCopy"`         // Automatically copy text
	TextRemoveLineBreaks bool `json:"textRemoveLineBreaks"` // Remove line breaks

	// Screen recording
	AutoSaveRecordings bool   `json:"autoSaveRecordings"` // Automatically save original screen recordings
	RecordingsFolder   string `json:"recordingsFolder"`   // empty = Videos\Screen Recordings
	RecordMic          bool   `json:"recordMic"`          // Include microphone input by default
	RecordSystemAudio  bool   `json:"recordSystemAudio"`  // Include system audio by default

	// App
	Theme string `json:"theme"` // system | light | dark

	// Remembered editor choices
	Kind  string `json:"kind"`  // snip | record
	Mode  string `json:"mode"`  // square | rectangle | freeform | window | fullscreen
	Delay int    `json:"delay"` // 0, 3, 5, 10
	// QuickMarkup is the capture bar's Quick markup toggle (Ctrl+E).
	QuickMarkup bool `json:"quickMarkup"`
	// VisualSearch is the engine for Quick markup's Visual Search: "bing" or "google".
	VisualSearch string `json:"visualSearch"`
}

// Default returns the factory settings.
func Default() Settings {
	return Settings{
		AutoCopyChanges:     true,
		AutoSaveScreenshots: true,
		AskToSaveEdited:     true,
		MultipleWindows:     false,
		BorderColor:         "#000000",
		BorderThickness:     1,
		TextAutoCopy:        true,
		VisualSearch:        "bing",
		AutoSaveRecordings:  false, // like Windows 11: recordings are saved with Save
		Version:             Version,
		RecordMic:           false,
		RecordSystemAudio:   true,
		Theme:               "system",
		Kind:                "snip",
		Mode:                "rectangle",
	}
}

// Load reads settings; a missing or unreadable file yields defaults.
func Load(path string) Settings {
	s := Default()
	b, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	s.Version = 0 // files without a version are from 1.1.x
	if json.Unmarshal(b, &s) != nil {
		return Default()
	}
	if s.Version < 2 {
		s.AutoSaveRecordings = false // 1.1.x saved recordings automatically by default
	}
	if s.Version < 3 {
		s.TextAutoCopy = true // new in 1.3
	}
	s.Normalize()
	return s
}

// Save writes settings atomically.
func Save(path string, s Settings) error {
	s.Normalize()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Normalize fixes out-of-range values.
func (s *Settings) Normalize() {
	s.Version = Version
	switch s.Mode {
	case "square", "rectangle", "freeform", "window", "fullscreen":
	default:
		s.Mode = "rectangle"
	}
	if s.Kind != "record" {
		s.Kind = "snip"
	}
	if s.Kind == "record" && s.Mode != "rectangle" && s.Mode != "window" {
		s.Mode = "rectangle"
	}
	switch s.Delay {
	case 0, 3, 5, 10:
	default:
		s.Delay = 0
	}
	switch s.Theme {
	case "system", "light", "dark":
	default:
		s.Theme = "system"
	}
	if s.BorderThickness < 1 {
		s.BorderThickness = 1
	}
	if s.BorderThickness > 20 {
		s.BorderThickness = 20
	}
	if len(s.BorderColor) != 7 || s.BorderColor[0] != '#' {
		s.BorderColor = "#000000"
	}
}

// FromJSON merges a JSON document from the editor's settings page.
func FromJSON(b []byte) (Settings, error) {
	s := Default()
	if err := json.Unmarshal(b, &s); err != nil {
		return s, errors.New("invalid settings")
	}
	s.Normalize()
	return s, nil
}
