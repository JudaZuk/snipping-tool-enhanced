// SPDX-License-Identifier: Apache-2.0

//go:build windows

// Package editor is the Snipping Tool Enhanced window (white paper §2 "full
// editor"): the New / mode / delay command bar, markup tools, save, copy and
// settings. The UI is ui/index.html, hosted in WebView2.
package editor

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/jchv/go-webview2"
	_ "golang.org/x/image/bmp"
	"golang.org/x/sys/windows"

	"snippingtoolenhanced/internal/clipboard"
	"snippingtoolenhanced/internal/crash"
	"snippingtoolenhanced/internal/handoff"
	"snippingtoolenhanced/internal/icon"
	"snippingtoolenhanced/internal/imaging"
	"snippingtoolenhanced/internal/ocr"
	"snippingtoolenhanced/internal/output"
	"snippingtoolenhanced/internal/overlay"
	"snippingtoolenhanced/internal/settings"
	"snippingtoolenhanced/internal/snip"
	"snippingtoolenhanced/internal/win"
)

//go:embed ui/index.html
var page string

// Version is set at build time.
var Version = "1.0.0-dev"

const webView2URL = "https://go.microsoft.com/fwlink/p/?LinkId=2124703"

// ImageInfo is sent to the page for every image it shows.
type ImageInfo struct {
	DataURL string `json:"dataUrl"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

type loadResult struct {
	Video           *VideoInfo        `json:"video"`
	Settings        settings.Settings `json:"settings"`
	Dark            bool              `json:"dark"`
	Shortcuts       Shortcuts         `json:"shortcuts"`
	Image           *ImageInfo        `json:"image"`
	Version         string            `json:"version"`
	DefaultDir      string            `json:"defaultDir"`
	DefaultVideoDir string            `json:"defaultVideoDir"`
}

type snipResult struct {
	Image     *ImageInfo `json:"image,omitempty"`
	Video     *VideoInfo `json:"video,omitempty"`
	ColorText string     `json:"color,omitempty"`
	Text      *textInfo  `json:"text,omitempty"`
	NewWindow bool       `json:"newWindow,omitempty"`
}

type textInfo struct {
	Text   string `json:"text"`
	Copied bool   `json:"copied"`
}

type editor struct {
	w         webview2.WebView
	hwnd      windows.HWND
	s         settings.Settings
	dirty     bool
	force     bool
	prevProc  uintptr
	hasImage  bool
	snipping  bool
	shortcuts func() Shortcuts
}

var ed *editor

var closeProc = syscall.NewCallback(func(h, msg, wp, lp uintptr) uintptr {
	if ed != nil && uint32(msg) == win.WM_CLOSE && !ed.force && ed.dirty && ed.s.AskToSaveEdited {
		ed.w.Eval("window.stRequestClose && stRequestClose()")
		return 0
	}
	if ed == nil {
		return win.DefWindowProc(windows.HWND(h), uint32(msg), wp, lp)
	}
	return win.CallWindowProc(ed.prevProc, windows.HWND(h), uint32(msg), wp, lp)
})

// Shortcuts reports which global shortcuts work (see the host package).
type Shortcuts struct {
	PrintScreen        bool `json:"printScreen"`        // Print Screen opens our overlay
	PrintScreenWindows bool `json:"printScreenWindows"` // Windows keeps Print Screen for itself
	Snip               bool `json:"snip"`               // Win + Alt + S
	Record             bool `json:"record"`             // Win + Alt + V
}

// Run opens an editor window, optionally showing the image or recording at
// path, and blocks until it closes.
func Run(path string, shortcuts func() Shortcuts) int {
	runtime.LockOSThread()
	win.SetDpiAwarenessPerMonitorV2()
	unlock := handoff.MarkEditorOpen()
	defer unlock()

	e := &editor{s: settings.Load(output.SettingsPath()), shortcuts: shortcuts}
	ed = e
	scale := float64(win.SystemDPI()) / 96
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     os.Getenv("SNIPPING_TOOL_DEVTOOLS") == "1",
		AutoFocus: true,
		DataPath:  filepath.Join(output.DataDir(), "WebView2"),
		WindowOptions: webview2.WindowOptions{
			Title:  "Snipping Tool Enhanced",
			IconId: 1, // from the .exe resources (tools/genres)
			Width:  uint(720 * scale),
			Height: uint(380 * scale),
			Center: true,
		},
	})
	if w == nil {
		if win.MessageBox(0, "Snipping Tool Enhanced needs the Microsoft Edge WebView2 Runtime, which was not found.\n\nOpen the download page now?",
			"Snipping Tool Enhanced", win.MB_YESNO|win.MB_ICONWARNING) == win.IDYES {
			_ = win.ShellOpen(webView2URL)
		}
		return 10
	}
	e.w = w
	e.hwnd = windows.HWND(uintptr(w.Window()))
	// Small (title bar) and big (Alt+Tab, taskbar) icons drawn for this DPI.
	if hi := win.IconFromBGRA(icon.BGRA(int(16*scale)), int(16*scale)); hi != 0 {
		win.SendMessage(e.hwnd, 0x0080 /*WM_SETICON*/, 0 /*ICON_SMALL*/, uintptr(hi))
	}
	if hi := win.IconFromBGRA(icon.BGRA(int(32*scale)), int(32*scale)); hi != 0 {
		win.SendMessage(e.hwnd, 0x0080, 1 /*ICON_BIG*/, uintptr(hi))
	}
	win.SetDarkTitleBar(e.hwnd, snip.Dark(e.s))
	e.prevProc = win.SubclassWindow(e.hwnd, closeProc)

	var initial *ImageInfo
	var initialVideo *VideoInfo
	if path != "" {
		var err error
		if isVideo(path) {
			initialVideo, err = loadVideo(path)
		} else {
			initial, err = loadImage(path)
		}
		if err != nil {
			win.MessageBox(e.hwnd, "Could not open the file:\n"+err.Error(), "Snipping Tool Enhanced", win.MB_ICONERROR)
		}
	}
	e.bind(initial, initialVideo)
	go e.listenHandoff()
	if initial != nil {
		e.fitTo(initial.Width, initial.Height)
	}
	if initialVideo != nil {
		e.fitTo(1280, 720)
	}
	w.SetHtml(page)
	w.Run()
	return 0
}

func (e *editor) bindSync(name string, f interface{}) {
	if err := e.w.Bind(name, f); err != nil {
		win.MessageBox(0, "Internal error binding "+name+": "+err.Error(), "Snipping Tool Enhanced", win.MB_ICONERROR)
	}
}

// bindAsync runs f outside the WebView2 event handler (needed for anything
// that opens a dialog or the capture overlay) and resolves the page's
// promise through window.stDone(id, result, error).
func (e *editor) bindAsync(name string, f func(args []json.RawMessage) (any, error)) {
	e.bindSync(name, func(id int, args ...json.RawMessage) {
		e.w.Dispatch(func() {
			var res any
			var err error
			func() {
				defer crash.Recover(name, &err)
				res, err = f(args)
			}()
			msg := ""
			if err != nil {
				msg = err.Error()
			}
			b, _ := json.Marshal(res)
			e.w.Eval(fmt.Sprintf("window.stDone(%d, %s, %s)", id, b, jsString(msg)))
		})
	})
}

func (e *editor) bind(initial *ImageInfo, initialVideo *VideoInfo) {
	e.bindSync("stLoad", func() loadResult {
		e.hasImage = initial != nil || initialVideo != nil
		return loadResult{Settings: e.s, Dark: snip.Dark(e.s), Shortcuts: e.shortcuts(), Image: initial, Video: initialVideo,
			Version: Version, DefaultDir: output.DefaultScreenshotsFolder(), DefaultVideoDir: output.DefaultRecordingsFolder()}
	})
	e.bindSync("stSaveSettings", func(raw string) (settings.Settings, error) {
		s, err := settings.FromJSON([]byte(raw))
		if err != nil {
			return e.s, err
		}
		e.s = s
		win.SetDarkTitleBar(e.hwnd, snip.Dark(s))
		return s, settings.Save(output.SettingsPath(), s)
	})
	e.bindSync("stIsDark", func() bool { return snip.Dark(e.s) })
	e.bindSync("stCopy", func(dataURL string) (err error) {
		defer crash.Recover("copy", &err)
		img, err := decodeDataURL(dataURL)
		if err != nil {
			return err
		}
		return clipboard.SetImage(img)
	})
	e.bindSync("stEditInPaint", func(dataURL string) (err error) {
		defer crash.Recover("edit in Paint", &err)
		img, err := decodeDataURL(dataURL)
		if err != nil {
			return err
		}
		p, err := output.SaveNew(img, output.CapturesDir())
		if err != nil {
			return err
		}
		return exec.Command("mspaint.exe", p).Start()
	})
	e.bindSync("stOpenWindowsSettings", func() {
		_ = win.ShellOpen("ms-settings:easeofaccess-keyboard")
	})
	e.bindSync("stShowInFolder", func(p string) { _ = exec.Command("explorer.exe", "/select,", p).Start() })
	e.bindSync("stSetDirty", func(d bool) {
		e.dirty = d
		t := "Snipping Tool Enhanced"
		if d && e.hasImage {
			t = "*" + t
		}
		e.w.SetTitle(t)
	})
	e.bindSync("stHasImage", func(v bool) { e.hasImage = v })
	e.bindSync("stClose", func() {
		e.force = true
		win.PostMessage(e.hwnd, win.WM_CLOSE, 0, 0)
	})

	e.bindAsync("stNewSnip", func(a []json.RawMessage) (any, error) {
		var kind, mode string
		var delay int
		if len(a) >= 3 {
			_ = json.Unmarshal(a[0], &kind)
			_ = json.Unmarshal(a[1], &mode)
			_ = json.Unmarshal(a[2], &delay)
		}
		return e.newSnip(overlay.Kind(kind), overlay.Mode(mode), delay)
	})
	e.bindAsync("stSaveVideo", func(a []json.RawMessage) (any, error) {
		var p string
		var start, end, dur float64
		if len(a) >= 4 {
			_ = json.Unmarshal(a[0], &p)
			_ = json.Unmarshal(a[1], &start)
			_ = json.Unmarshal(a[2], &end)
			_ = json.Unmarshal(a[3], &dur)
		}
		return e.saveVideo(p, start, end, dur)
	})
	e.bindAsync("stOcr", func(a []json.RawMessage) (any, error) {
		var dataURL string
		if len(a) > 0 {
			_ = json.Unmarshal(a[0], &dataURL)
		}
		img, err := decodeDataURL(dataURL)
		if err != nil {
			return nil, err
		}
		return ocr.Recognize(img)
	})
	e.bindSync("stCopyText", func(t string) (err error) {
		defer crash.Recover("copy text", &err)
		return clipboard.SetText(t)
	})
	e.bindSync("stGifBegin", func(quality string) string { return gifBegin(quality) })
	e.bindSync("stGifFrame", func(id, dataURL string) (err error) {
		defer crash.Recover("GIF export", &err)
		return gifFrame(id, dataURL)
	})
	e.bindAsync("stGifFinish", func(a []json.RawMessage) (any, error) {
		var id, action, name string
		if len(a) >= 3 {
			_ = json.Unmarshal(a[0], &id)
			_ = json.Unmarshal(a[1], &action)
			_ = json.Unmarshal(a[2], &name)
		}
		return e.gifFinish(id, action, name)
	})
	e.bindAsync("stSaveAs", func(a []json.RawMessage) (any, error) {
		var dataURL string
		if len(a) > 0 {
			_ = json.Unmarshal(a[0], &dataURL)
		}
		return e.saveAs(dataURL)
	})
	e.bindAsync("stOpenFile", func([]json.RawMessage) (any, error) {
		p := win.OpenFileDialog(e.hwnd, "Open",
			"Images and recordings (*.png;*.jpg;*.jpeg;*.gif;*.bmp;*.mp4)\x00*.png;*.jpg;*.jpeg;*.gif;*.bmp;*.mp4\x00All files\x00*.*\x00")
		if p == "" {
			return nil, nil
		}
		if isVideo(p) {
			v, err := loadVideo(p)
			if err != nil {
				return nil, err
			}
			e.hasImage = true
			e.fitTo(1280, 720)
			return snipResult{Video: v}, nil
		}
		info, err := loadImage(p)
		if err != nil {
			return nil, err
		}
		e.hasImage = true
		e.fitTo(info.Width, info.Height)
		return snipResult{Image: info}, nil
	})
	e.bindAsync("stPickFolder", func([]json.RawMessage) (any, error) {
		return win.PickFolder(e.hwnd, "Choose the screenshots folder", output.ScreenshotsFolder(e.s.SaveFolder)), nil
	})
}

// newSnip hides the window, runs the capture overlay in this process and
// returns the capture to the page (or opens it in a new window).
func (e *editor) newSnip(kind overlay.Kind, mode overlay.Mode, delay int) (any, error) {
	if e.snipping {
		return snipResult{}, nil
	}
	e.snipping = true
	defer func() { e.snipping = false }()
	if mode == "" {
		mode = overlay.Mode(e.s.Mode)
	}
	win.ShowWindow(e.hwnd, win.SW_HIDE)
	time.Sleep(250 * time.Millisecond) // let the desktop repaint without this window
	var res snip.Result
	switch kind {
	case "text":
		res = snip.TextExtractor(time.Duration(delay)*time.Second, e.s)
	case "":
		kind = overlay.KindSnip
		fallthrough
	default:
		res = snip.Interactive(kind, mode, time.Duration(delay)*time.Second, e.s)
	}
	defer win.BringToFront(e.hwnd)
	if res.Err != nil {
		return nil, res.Err
	}
	switch res.Outcome {
	case snip.TextExtracted:
		return snipResult{Text: &textInfo{Text: res.Text, Copied: res.Copied}}, nil
	case overlay.ColorPicked:
		return snipResult{ColorText: res.ColorText}, nil
	case overlay.Cancelled:
		return snipResult{}, nil
	}
	e.s = settings.Load(output.SettingsPath()) // recording may have updated audio defaults
	if res.Kind == overlay.KindRecord {
		if e.s.MultipleWindows && e.hasImage {
			exe, _ := os.Executable()
			_ = exec.Command(exe, "--editor", res.VideoPath).Start()
			return snipResult{NewWindow: true}, nil
		}
		v, err := loadVideo(res.VideoPath)
		if err != nil {
			return nil, err
		}
		e.hasImage = true
		e.fitTo(1280, 720)
		return snipResult{Video: v}, nil
	}
	d, err := snip.Deliver(res.Image, e.s)
	if d.WorkPath == "" {
		return nil, err
	}
	if e.s.MultipleWindows && e.hasImage {
		exe, _ := os.Executable()
		_ = exec.Command(exe, "--editor", d.WorkPath).Start()
		return snipResult{NewWindow: true}, nil
	}
	info, lerr := loadImage(d.WorkPath)
	if lerr != nil {
		return nil, lerr
	}
	e.hasImage = true
	e.fitTo(info.Width, info.Height)
	return snipResult{Image: info}, err
}

func (e *editor) saveAs(dataURL string) (string, error) {
	img, err := decodeDataURL(dataURL)
	if err != nil {
		return "", err
	}
	dir := output.ScreenshotsFolder(e.s.SaveFolder)
	_ = os.MkdirAll(dir, 0o755)
	name := "Screenshot " + time.Now().Format("2006-01-02 150405")
	p, idx := win.SaveFileDialog(e.hwnd, "Save as", dir, name,
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

// fitTo resizes the window so an image of w×h pixels fits (within 85% of
// the work area), and centers it.
func (e *editor) fitTo(w, h int) {
	scale := float64(win.DpiForWindow(e.hwnd)) / 96
	work := win.PrimaryWorkArea()
	ww, wh := int(work.Right-work.Left), int(work.Bottom-work.Top)
	cw := clamp(w+int(80*scale), int(1180*scale), ww*85/100)
	ch := clamp(h+int(150*scale), int(560*scale), wh*85/100)
	win.MoveWindow(e.hwnd, work.Left+int32((ww-cw)/2), work.Top+int32((wh-ch)/2), int32(cw), int32(ch))
}

// listenHandoff receives captures from the Print Screen host when
// "Multiple windows" is off.
func (e *editor) listenHandoff() {
	for {
		p, ok := handoff.Wait()
		if !ok {
			return
		}
		e.w.Dispatch(func() {
			var res snipResult
			if isVideo(p) {
				v, err := loadVideo(p)
				if err != nil {
					return
				}
				res.Video = v
				e.fitTo(1280, 720)
			} else {
				info, err := loadImage(p)
				if err != nil {
					return
				}
				res.Image = info
				e.fitTo(info.Width, info.Height)
			}
			e.hasImage = true
			b, _ := json.Marshal(res)
			e.w.Eval("window.stReceive && stReceive(" + string(b) + ")")
			win.BringToFront(e.hwnd)
		})
	}
}

func loadImage(path string) (*ImageInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("this file is not a supported image")
	}
	mime := map[string]string{"png": "image/png", "jpeg": "image/jpeg", "gif": "image/gif"}[format]
	if mime == "" { // e.g. BMP: re-encode for the page
		data, err = imaging.Encode(img, "png", 0)
		if err != nil {
			return nil, err
		}
		mime = "image/png"
	}
	b := img.Bounds()
	return &ImageInfo{
		DataURL: "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data),
		Name:    strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Path: path,
		Width: b.Dx(), Height: b.Dy(),
	}, nil
}

func decodeDataURL(u string) (*image.NRGBA, error) {
	const prefix = "base64,"
	i := strings.Index(u, prefix)
	if !strings.HasPrefix(u, "data:image/") || i < 0 {
		return nil, errors.New("not an image")
	}
	raw, err := base64.StdEncoding.DecodeString(u[i+len(prefix):])
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	n := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(n, n.Bounds(), img, b.Min, draw.Src)
	return n, nil
}

func jsString(s string) string { b, _ := json.Marshal(s); return string(b) }

func clamp(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
