// SPDX-License-Identifier: Apache-2.0

//go:build windows

// Package snip is the capture pipeline shared by the Print Screen host and the
// editor's New button: freeze → overlay → crop → border → clipboard/auto-save.
package snip

import (
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"

	"snippingtoolenhanced/internal/capture"
	"snippingtoolenhanced/internal/clipboard"
	"snippingtoolenhanced/internal/imaging"
	"snippingtoolenhanced/internal/markup"
	"snippingtoolenhanced/internal/ocr"
	"snippingtoolenhanced/internal/output"
	"snippingtoolenhanced/internal/overlay"
	"snippingtoolenhanced/internal/recui"
	"snippingtoolenhanced/internal/settings"
	"snippingtoolenhanced/internal/win"
)

// Result of one capture.
type Result struct {
	Outcome   overlay.Outcome
	Kind      overlay.Kind
	Image     *image.NRGBA
	Mode      overlay.Mode
	ColorText string
	VideoPath string // recordings
	Text      string // Text Extractor
	Copied    bool   // Text Extractor copied Text to the clipboard
	// Action is the Quick markup button that finished the snip:
	// overlay.ActCopy, overlay.ActSave (already saved) or overlay.ActShare.
	Action string
	Area   image.Rectangle // the snipped area, in virtual-screen coordinates
	Err    error
}

// Text Extractor outcome (re-exported for callers).
const TextExtracted = overlay.TextSelected

// Dark reports whether the UI should use the dark theme for these settings.
func Dark(s settings.Settings) bool {
	switch s.Theme {
	case "dark":
		return true
	case "light":
		return false
	}
	return win.AppsUseDarkTheme()
}

// Interactive waits delay, freezes the screen and runs the overlay. The
// calling goroutine must be locked to its OS thread. Only one capture can
// run at a time across processes.
func Interactive(kind overlay.Kind, mode overlay.Mode, delay time.Duration, s settings.Settings) Result {
	return run(kind, mode, false, delay, s)
}

// TextExtractor opens the overlay with Text Extractor selected.
func TextExtractor(delay time.Duration, s settings.Settings) Result {
	return run(overlay.KindSnip, overlay.Rectangle, true, delay, s)
}

func run(kind overlay.Kind, mode overlay.Mode, text bool, delay time.Duration, s settings.Settings) Result {
	h, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(`Local\SnippingToolEnhanced.Capture`))
	if err == windows.ERROR_ALREADY_EXISTS {
		windows.CloseHandle(h)
		return Result{Outcome: overlay.Cancelled}
	}
	if err == nil {
		defer func() {
			if h != 0 {
				windows.CloseHandle(h)
			}
		}()
	}
	if delay > 0 {
		time.Sleep(delay)
	}
	frame, err := capture.Freeze(false, uint32(os.Getpid()))
	if err != nil {
		return Result{Outcome: overlay.Cancelled, Err: err}
	}
	save := func(owner windows.HWND, r overlay.Result) bool {
		img := Compose(frame, r, s)
		if img == nil {
			return false
		}
		p, err := SaveAs(owner, img, s)
		if err != nil {
			win.MessageBox(owner, "The snip could not be saved.\n\n"+err.Error(), "Snipping Tool Enhanced", win.MB_ICONERROR)
			return false
		}
		return p != ""
	}
	res, err := overlay.Run(overlay.Options{Frame: frame, Kind: kind, Mode: mode, Text: text, Dark: Dark(s), QuickMarkup: s.QuickMarkup, Save: save})
	if err != nil {
		return Result{Outcome: overlay.Cancelled, Err: err}
	}
	if res.QuickMarkup != s.QuickMarkup { // remember the toggle
		cur := settings.Load(output.SettingsPath())
		cur.QuickMarkup = res.QuickMarkup
		_ = settings.Save(output.SettingsPath(), cur)
	}
	switch res.Outcome {
	case overlay.ColorPicked:
		if err := clipboard.SetText(res.ColorText); err != nil {
			return Result{Outcome: overlay.Cancelled, Err: err}
		}
		return Result{Outcome: overlay.ColorPicked, ColorText: res.ColorText, Mode: res.Mode}
	case overlay.Cancelled:
		return Result{Outcome: overlay.Cancelled}
	case overlay.TextSelected:
		img := imaging.OpaqueToNRGBA(imaging.Crop(frame.Image, res.Rect))
		r, err := ocr.Recognize(img)
		if err != nil {
			return Result{Outcome: overlay.Cancelled, Err: err}
		}
		text := r.Text(s.TextRemoveLineBreaks)
		out := Result{Outcome: overlay.TextSelected, Text: text, Image: img}
		if s.TextAutoCopy && text != "" {
			out.Copied = clipboard.SetText(text) == nil
		}
		return out
	}
	if res.Kind == overlay.KindRecord {
		windows.CloseHandle(h) // allow other captures while recording
		h = 0
		return recordArea(res.Rect.Add(frame.Origin), res.Mode, s)
	}
	img := Compose(frame, res, s)
	if img == nil {
		return Result{Outcome: overlay.Cancelled}
	}
	return Result{Outcome: overlay.Captured, Kind: overlay.KindSnip, Image: img, Mode: res.Mode, Action: res.Action,
		Area: res.Rect.Add(frame.Origin)}
}

// Compose cuts the snip out of the frozen screen, with any Quick markup marks
// and the border setting.
func Compose(frame *capture.Frame, res overlay.Result, s settings.Settings) *image.NRGBA {
	src := frame.Image
	area := res.Rect
	if res.Mode == overlay.Freeform {
		area = image.Rectangle{}
		for _, p := range res.Poly {
			area = area.Union(image.Rectangle{Min: p, Max: p.Add(image.Pt(1, 1))})
		}
	}
	if len(res.Marks) > 0 {
		area = area.Intersect(src.Bounds())
		work := image.NewRGBA(area)
		draw.Draw(work, area, src, area.Min, draw.Src)
		markup.DrawAll(work, res.Marks)
		src = work
	}
	var img *image.NRGBA
	if res.Mode == overlay.Freeform {
		img = imaging.Freeform(src, res.Poly)
	} else {
		img = imaging.OpaqueToNRGBA(imaging.Crop(src, res.Rect))
	}
	if img == nil {
		return nil
	}
	if s.BorderEnabled {
		if c, err := imaging.ParseHexColor(s.BorderColor); err == nil {
			img = imaging.BorderN(img, c, s.BorderThickness)
		}
	}
	return img
}

// SaveAs asks where to save img (PNG, JPEG or GIF) and saves it. It returns
// "" when the dialog was cancelled.
func SaveAs(owner windows.HWND, img image.Image, s settings.Settings) (string, error) {
	dir := output.ScreenshotsFolder(s.SaveFolder)
	_ = os.MkdirAll(dir, 0o755)
	name := "Screenshot " + time.Now().Format("2006-01-02 150405")
	p, idx := win.SaveFileDialog(owner, "Save as", dir, name,
		"PNG (*.png)\x00*.png\x00JPEG (*.jpg)\x00*.jpg;*.jpeg\x00GIF (*.gif)\x00*.gif\x00", "png")
	if p == "" {
		return "", nil
	}
	want := map[int]string{1: ".png", 2: ".jpg", 3: ".gif"}[idx]
	ext := strings.ToLower(filepath.Ext(p))
	if want != "" && ext != want && !(want == ".jpg" && ext == ".jpeg") {
		p = strings.TrimSuffix(p, filepath.Ext(p)) + want
	}
	return p, output.Write(img, p)
}

// ShareFile writes img to a temporary PNG for the share sheet (deleted after
// 7 days, like the editor's working copies).
func ShareFile(img image.Image) (string, error) {
	dir := filepath.Join(output.DataDir(), "Share")
	go output.Cleanup(dir, 7*24*time.Hour)
	return output.SaveNew(img, dir)
}

// recordArea shows the recording controls for rect (virtual-screen coordinates).
func recordArea(rect image.Rectangle, mode overlay.Mode, s settings.Settings) Result {
	path := output.NewRecordingPath(s.AutoSaveRecordings, s.RecordingsFolder)
	r := recui.Run(recui.Options{Rect: rect, Path: path, Mic: s.RecordMic, System: s.RecordSystemAudio, Dark: Dark(s)})
	// Remember the audio choices as the new defaults.
	cur := settings.Load(output.SettingsPath())
	cur.RecordMic, cur.RecordSystemAudio = r.Mic, r.System
	_ = settings.Save(output.SettingsPath(), cur)
	if r.Err != nil {
		return Result{Outcome: overlay.Cancelled, Err: r.Err}
	}
	if r.Path == "" {
		return Result{Outcome: overlay.Cancelled}
	}
	return Result{Outcome: overlay.Captured, Kind: overlay.KindRecord, Mode: mode, VideoPath: r.Path}
}

// Delivered says where a capture went.
type Delivered struct {
	Copied    bool
	SavedPath string // Pictures\Screenshots copy, if auto-save is on
	WorkPath  string // working copy the editor opens
}

// Deliver copies the capture to the clipboard, auto-saves it if enabled and
// writes the working copy for the editor.
func Deliver(img *image.NRGBA, s settings.Settings) (Delivered, error) {
	var d Delivered
	var firstErr error
	if p, err := output.SaveNew(img, output.CapturesDir()); err == nil {
		d.WorkPath = p
	} else {
		firstErr = fmt.Errorf("working copy: %w", err)
	}
	go output.Cleanup(output.CapturesDir(), 7*24*time.Hour)
	if s.AutoCopyChanges {
		if err := clipboard.SetImage(img); err == nil {
			d.Copied = true
		} else if firstErr == nil {
			firstErr = fmt.Errorf("clipboard: %w", err)
		}
	}
	if s.AutoSaveScreenshots {
		if p, err := output.SaveNew(img, output.ScreenshotsFolder(s.SaveFolder)); err == nil {
			d.SavedPath = p
		} else if firstErr == nil {
			firstErr = fmt.Errorf("auto-save: %w", err)
		}
	}
	return d, firstErr
}
