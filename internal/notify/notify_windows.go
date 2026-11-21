// SPDX-License-Identifier: Apache-2.0

//go:build windows

// Package notify shows the post-capture notification ("Mark up and share",
// white paper §3) as a Windows 11-style popup in the bottom-right corner.
package notify

import (
	"image"
	"image/draw"
	"math"
	"syscall"

	"golang.org/x/sys/windows"

	"snippingtoolenhanced/internal/imaging"
	"snippingtoolenhanced/internal/render"
	"snippingtoolenhanced/internal/win"
)

const className = "SnippingToolEnhancedNotification"

type popup struct {
	hwnd    windows.HWND
	dib     *win.DIB
	onClick func()
}

var (
	current *popup
	proc    = syscall.NewCallback(func(h, msg, wp, lp uintptr) uintptr {
		p := current
		if p == nil || windows.HWND(h) != p.hwnd {
			return win.DefWindowProc(windows.HWND(h), uint32(msg), wp, lp)
		}
		switch uint32(msg) {
		case win.WM_PAINT:
			var ps win.PaintStruct
			dc := win.BeginPaint(p.hwnd, &ps)
			win.BitBlt(dc, 0, 0, int32(p.dib.W), int32(p.dib.H), p.dib.DC, 0, 0)
			win.EndPaint(p.hwnd, &ps)
			return 0
		case win.WM_ERASEBKGND:
			return 1
		case win.WM_LBUTTONUP:
			cb := p.onClick
			Close()
			if cb != nil {
				cb()
			}
			return 0
		case win.WM_RBUTTONUP, win.WM_TIMER:
			Close()
			return 0
		case win.WM_SETCURSOR:
			win.SetCursor(win.LoadCursor(win.IDC_HAND))
			return 1
		}
		return win.DefWindowProc(windows.HWND(h), uint32(msg), wp, lp)
	})
)

// Show displays a notification with a thumbnail. onClick runs when the user
// selects it. Must be called on a thread with a message loop.
func Show(thumb image.Image, title, body string, dark bool, onClick func()) {
	Close()
	_ = win.RegisterClassStyle(className, proc, win.IDC_HAND, 0, win.CS_DROPSHADOW)
	scale := float64(win.SystemDPI()) / 96
	px := func(v float64) int { return int(math.Round(v * scale)) }
	w, h := px(364), px(104)

	th := render.Light
	if dark {
		th = render.Dark
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(th.Surface), image.Point{}, draw.Src)
	render.StrokeRoundRect(img, 0, 0, float64(w), float64(h), 0, 1, th.Border)

	// Thumbnail (fit into 96×64), with a subtle frame.
	tx, ty, tw, tH := px(16), px(20), px(96), px(64)
	if thumb != nil {
		tb := thumb.Bounds()
		k := math.Min(float64(tw)/float64(tb.Dx()), float64(tH)/float64(tb.Dy()))
		k = math.Min(k, 1*scale*4)
		sw, sh := int(float64(tb.Dx())*k), int(float64(tb.Dy())*k)
		if sw < 1 {
			sw = 1
		}
		if sh < 1 {
			sh = 1
		}
		ox, oy := tx+(tw-sw)/2, ty+(tH-sh)/2
		render.FillRoundRect(img, float64(ox-1), float64(oy-1), float64(sw+2), float64(sh+2), 0, th.Border)
		scaleInto(img, image.Rect(ox, oy, ox+sw, oy+sh), thumb)
	}
	x := float64(tx + tw + px(16))
	render.Text(img, "Snipping Tool Enhanced", x, float64(px(24)), float64(px(12)), th.TextMuted)
	render.Text(img, title, x, float64(px(48)), float64(px(14)), th.Text)
	render.Text(img, body, x, float64(px(70)), float64(px(12)), th.TextMuted)

	d, err := win.NewDIB(w, h)
	if err != nil {
		return
	}
	copy(d.Bits, imaging.RGBAToBGRA(img, 0))

	work := win.Rect{Right: 1920, Bottom: 1040}
	for _, m := range win.Monitors() {
		if m.Primary {
			work = m.Work
		}
	}
	hwnd, err := win.CreateWindow(win.WS_EX_TOPMOST|win.WS_EX_TOOLWINDOW|win.WS_EX_NOACTIVATE, className, "Snipping Tool Enhanced",
		win.WS_POPUP, work.Right-int32(w)-int32(px(12)), work.Bottom-int32(h)-int32(px(12)), int32(w), int32(h))
	if err != nil {
		d.Close()
		return
	}
	current = &popup{hwnd: hwnd, dib: d, onClick: onClick}
	win.RoundCorners(hwnd, false)
	win.SetTopmost(hwnd, work.Right-int32(w)-int32(px(12)), work.Bottom-int32(h)-int32(px(12)), int32(w), int32(h), false)
	win.SetTimer(hwnd, 1, 7000)
}

// Close removes the notification if one is showing.
func Close() {
	if current == nil {
		return
	}
	p := current
	current = nil
	win.KillTimer(p.hwnd, 1)
	win.DestroyWindow(p.hwnd)
	p.dib.Close()
}

// scaleInto draws src into r of dst with box sampling (good enough for thumbnails).
func scaleInto(dst *image.RGBA, r image.Rectangle, src image.Image) {
	sb := src.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		sy := sb.Min.Y + (y-r.Min.Y)*sb.Dy()/r.Dy()
		for x := r.Min.X; x < r.Max.X; x++ {
			sx := sb.Min.X + (x-r.Min.X)*sb.Dx()/r.Dx()
			c := src.At(sx, sy)
			r2, g, b, a := c.RGBA()
			if a < 0xffff { // composite on white (freeform transparency)
				f := 0xffff - a
				r2, g, b = r2+f, g+f, b+f
			}
			i := dst.PixOffset(x, y)
			dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = uint8(r2>>8), uint8(g>>8), uint8(b>>8), 255
		}
	}
}
