// SPDX-License-Identifier: Apache-2.0

//go:build windows

package recui

import (
	"image"
	"math"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"snippingtoolenhanced/internal/imaging"
	"snippingtoolenhanced/internal/record"
	"snippingtoolenhanced/internal/render"
	"snippingtoolenhanced/internal/win"
)

// Options for one recording.
type Options struct {
	Rect   image.Rectangle // virtual-screen coordinates
	Path   string
	Mic    bool
	System bool
	Dark   bool
}

// Result of the recording UI.
type Result struct {
	Path   string // empty if cancelled or deleted
	Mic    bool   // final toggle states, to remember as defaults
	System bool
	Err    error
}

const (
	barClass   = "SnippingToolEnhancedRecordBar"
	frameClass = "SnippingToolEnhancedRecordFrame"
	timerTick  = 1
)

type ui struct {
	o      Options
	v      View
	th     render.Theme
	scale  float64
	hwnd   windows.HWND
	frames [4]windows.HWND
	dib    *win.DIB
	sess   *record.Session
	done   bool
	res    Result
}

var cur *ui

var barProc = syscall.NewCallback(func(h, msg, wp, lp uintptr) uintptr {
	if cur == nil || windows.HWND(h) != cur.hwnd {
		return win.DefWindowProc(windows.HWND(h), uint32(msg), wp, lp)
	}
	return cur.proc(uint32(msg), wp, lp)
})

var frameProc = syscall.NewCallback(func(h, msg, wp, lp uintptr) uintptr {
	if uint32(msg) == win.WM_PAINT {
		var ps win.PaintStruct
		dc := win.BeginPaint(windows.HWND(h), &ps)
		win.FillRect(dc, win.Rect{Right: 10000, Bottom: 10000}, win.RGB(0xE8, 0x11, 0x23))
		win.EndPaint(windows.HWND(h), &ps)
		return 0
	}
	return win.DefWindowProc(windows.HWND(h), uint32(msg), wp, lp)
})

// Run shows the recording bar for o.Rect and blocks until the recording is
// stopped, deleted or cancelled. The calling goroutine must be locked to its thread.
func Run(o Options) Result {
	u := &ui{o: o, th: render.Light, scale: float64(win.SystemDPI()) / 96,
		v: View{Phase: Ready, Mic: o.Mic, System: o.System, MicOK: true, SystemOK: true}}
	if o.Dark {
		u.th = render.Dark
	}
	cur = u
	defer func() { cur = nil }()
	_ = win.RegisterClassStyle(barClass, barProc, win.IDC_ARROW, 0, win.CS_DROPSHADOW)
	_ = win.RegisterClass(frameClass, frameProc, win.IDC_ARROW, 0)

	// Red frame just outside the recorded area; excluded from the recording.
	t := int32(math.Max(2, math.Round(2*u.scale)))
	r := o.Rect
	rects := [4]win.Rect{
		{Left: int32(r.Min.X) - t, Top: int32(r.Min.Y) - t, Right: int32(r.Max.X) + t, Bottom: int32(r.Min.Y)},
		{Left: int32(r.Min.X) - t, Top: int32(r.Max.Y), Right: int32(r.Max.X) + t, Bottom: int32(r.Max.Y) + t},
		{Left: int32(r.Min.X) - t, Top: int32(r.Min.Y), Right: int32(r.Min.X), Bottom: int32(r.Max.Y)},
		{Left: int32(r.Max.X), Top: int32(r.Min.Y), Right: int32(r.Max.X) + t, Bottom: int32(r.Max.Y)},
	}
	for i, fr := range rects {
		h, err := win.CreateWindow(win.WS_EX_TOPMOST|win.WS_EX_TOOLWINDOW|win.WS_EX_NOACTIVATE|0x20 /*TRANSPARENT*/, frameClass, "", win.WS_POPUP,
			fr.Left, fr.Top, fr.Right-fr.Left, fr.Bottom-fr.Top)
		if err == nil {
			win.ExcludeFromCapture(h)
			win.SetTopmost(h, fr.Left, fr.Top, fr.Right-fr.Left, fr.Bottom-fr.Top, false)
			u.frames[i] = h
		}
	}
	defer func() {
		for _, h := range u.frames {
			if h != 0 {
				win.DestroyWindow(h)
			}
		}
	}()

	var err error
	u.hwnd, err = win.CreateWindow(win.WS_EX_TOPMOST|win.WS_EX_TOOLWINDOW, barClass, "Snipping Tool Enhanced recording", win.WS_POPUP, 0, 0, 10, 10)
	if err != nil {
		return Result{Err: err}
	}
	win.ExcludeFromCapture(u.hwnd)
	win.RoundCorners(u.hwnd, false)
	u.layout()
	win.SetForegroundWindow(u.hwnd)
	win.SetTimer(u.hwnd, timerTick, 250)

	var m win.Msg
	quit := false
	for !u.done {
		if !win.GetMessage(&m) {
			quit = true
			break
		}
		win.TranslateMessage(&m)
		win.DispatchMessage(&m)
	}
	if u.sess != nil { // window closed some other way: keep what was recorded
		p, err := u.sess.Stop()
		u.res = Result{Path: p, Err: err}
	}
	win.KillTimer(u.hwnd, timerTick)
	win.DestroyWindow(u.hwnd)
	if u.dib != nil {
		u.dib.Close()
	}
	if quit {
		win.PostQuitMessage(int(m.WParam))
	}
	u.res.Mic, u.res.System = u.v.Mic, u.v.System
	return u.res
}

// layout renders the bar and places it above the recorded area (or inside its
// top edge when there is no room above).
func (u *ui) layout() {
	img := Render(u.v, u.scale, u.th)
	if u.dib != nil {
		u.dib.Close()
	}
	u.dib, _ = win.NewDIB(img.Rect.Dx(), img.Rect.Dy())
	if u.dib != nil {
		copy(u.dib.Bits, imaging.RGBAToBGRA(img, 0))
	}
	w, h := int32(img.Rect.Dx()), int32(img.Rect.Dy())
	r := u.o.Rect
	x := int32(r.Min.X+r.Dx()/2) - w/2
	y := int32(r.Min.Y) - h - int32(12*u.scale)
	mon := win.Rect{Top: -1 << 30}
	for _, m := range win.Monitors() {
		if int32(r.Min.X) >= m.Rect.Left && int32(r.Min.X) < m.Rect.Right && int32(r.Min.Y) >= m.Rect.Top && int32(r.Min.Y) < m.Rect.Bottom {
			mon = m.Rect
		}
	}
	if y < mon.Top {
		y = int32(r.Min.Y) + int32(12*u.scale)
	}
	win.SetTopmost(u.hwnd, x, y, w, h, false)
	win.Invalidate(u.hwnd)
}

func (u *ui) proc(msg uint32, wp, lp uintptr) uintptr {
	switch msg {
	case win.WM_PAINT:
		var ps win.PaintStruct
		dc := win.BeginPaint(u.hwnd, &ps)
		if u.dib != nil {
			win.BitBlt(dc, 0, 0, int32(u.dib.W), int32(u.dib.H), u.dib.DC, 0, 0)
		}
		win.EndPaint(u.hwnd, &ps)
		return 0
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_MOUSEMOVE:
		h := Hit(u.v, u.scale, image.Pt(int(win.LoInt(lp)), int(win.HiInt(lp))))
		if h != u.v.Hover {
			u.v.Hover = h
			u.layout()
		}
		return 0
	case win.WM_LBUTTONUP:
		u.click(Hit(u.v, u.scale, image.Pt(int(win.LoInt(lp)), int(win.HiInt(lp)))))
		return 0
	case win.WM_TIMER:
		u.tick()
		return 0
	case win.WM_KEYDOWN:
		if wp == win.VK_ESCAPE && u.v.Phase <= Countdown {
			u.finish(Result{})
		}
		return 0
	case win.WM_CLOSE:
		return 0
	}
	return win.DefWindowProc(u.hwnd, msg, wp, lp)
}

var countdownStart time.Time

func (u *ui) tick() {
	switch u.v.Phase {
	case Countdown:
		left := 3 - int(time.Since(countdownStart)/time.Second)
		if left <= 0 {
			u.begin()
			return
		}
		if left != u.v.Count {
			u.v.Count = left
			u.layout()
		}
	case Recording, Paused:
		el := u.sess.Elapsed().Truncate(time.Second)
		if el != u.v.Elapsed {
			u.v.Elapsed = el
			u.layout()
		}
	}
}

func (u *ui) begin() {
	s, err := record.Start(record.Options{Rect: u.o.Rect, Path: u.o.Path, SystemAudio: u.v.System, Mic: u.v.Mic})
	if err != nil {
		u.finish(Result{Err: err})
		return
	}
	u.sess = s
	sys, mic := s.HasAudio()
	u.v.SystemOK = sys || !u.v.System
	u.v.MicOK = mic || !u.v.Mic
	if !sys {
		u.v.System = false
	}
	if !mic {
		u.v.Mic = false
	}
	u.v.LockAudio = true
	u.v.Phase = Recording
	u.layout()
}

func (u *ui) click(id string) {
	switch id {
	case BtnStart:
		if u.v.Phase == Ready {
			u.v.Phase, u.v.Count = Countdown, 3
			countdownStart = time.Now()
			u.layout()
		}
	case BtnClose:
		u.finish(Result{})
	case BtnMic, BtnSystem:
		if u.v.LockAudio {
			// After start, toggles mute sources that are capturing.
			sys, mic := u.sess.HasAudio()
			if id == BtnMic && mic {
				u.v.Mic = !u.v.Mic
			}
			if id == BtnSystem && sys {
				u.v.System = !u.v.System
			}
			u.sess.SetMuted(!u.v.System, !u.v.Mic)
		} else if id == BtnMic {
			u.v.Mic = !u.v.Mic
		} else {
			u.v.System = !u.v.System
		}
		u.layout()
	case BtnPause:
		if u.v.Phase == Recording {
			u.sess.Pause()
			u.v.Phase = Paused
		} else if u.v.Phase == Paused {
			u.sess.Resume()
			u.v.Phase = Recording
		}
		u.layout()
	case BtnStop:
		if u.sess != nil {
			p, err := u.sess.Stop()
			u.sess = nil
			u.finish(Result{Path: p, Err: err})
		}
	case BtnDelete:
		if u.sess != nil {
			u.sess.Discard()
			u.sess = nil
		}
		u.finish(Result{})
	}
}

func (u *ui) finish(r Result) {
	u.res = r
	u.done = true
}
