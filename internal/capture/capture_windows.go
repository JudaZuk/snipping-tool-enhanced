// SPDX-License-Identifier: Apache-2.0

//go:build windows

// Package capture freezes the screen (white paper §5).
//
// This release uses the GDI path (BitBlt with CAPTUREBLT) for the frozen
// still frame. It is the most compatible path, works in RDP sessions and
// honors WDA_EXCLUDEFROMCAPTURE. The Windows.Graphics.Capture path (needed for
// HDR and for recording) is milestone M3.
package capture

import (
	"fmt"
	"image"

	"snippingtoolenhanced/internal/imaging"
	"snippingtoolenhanced/internal/win"
)

// Monitor is a display in frame coordinates (origin = virtual-screen top-left).
type Monitor struct {
	Rect    image.Rectangle
	Primary bool
	Name    string
}

// Window is a capturable top-level window in frame coordinates.
type Window struct {
	Rect  image.Rectangle
	Title string
	PID   uint32
}

// Frame is a frozen copy of the whole virtual screen.
type Frame struct {
	Image    *image.RGBA
	Origin   image.Point // virtual-screen coordinate of pixel (0,0)
	Monitors []Monitor
	Windows  []Window // z-order, topmost first
}

// Freeze grabs every monitor at once, plus the monitor and window layout.
// Must be called with per-monitor-v2 DPI awareness set, so all values are
// physical pixels.
func Freeze(includeCursor bool, excludePID uint32) (*Frame, error) {
	x := win.GetSystemMetrics(win.SM_XVIRTUALSCREEN)
	y := win.GetSystemMetrics(win.SM_YVIRTUALSCREEN)
	w := int(win.GetSystemMetrics(win.SM_CXVIRTUALSCREN))
	h := int(win.GetSystemMetrics(win.SM_CYVIRTUALSCREN))
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("capture: no display")
	}
	// Window layout first: it is cheap and must match the pixels.
	wins := win.TopLevelWindows()
	dib, err := win.NewDIB(w, h)
	if err != nil {
		return nil, fmt.Errorf("capture: %w", err)
	}
	defer dib.Close()
	if err := dib.BlitFromScreen(x, y); err != nil {
		return nil, fmt.Errorf("capture: BitBlt: %w", err)
	}
	if includeCursor {
		win.DrawCursor(dib.DC, x, y)
	}
	f := &Frame{Image: imaging.BGRAToRGBA(dib.Bits, w, h), Origin: image.Pt(int(x), int(y))}
	toFrame := func(r win.Rect) image.Rectangle {
		return image.Rect(int(r.Left-x), int(r.Top-y), int(r.Right-x), int(r.Bottom-y))
	}
	for _, m := range win.Monitors() {
		f.Monitors = append(f.Monitors, Monitor{Rect: toFrame(m.Rect), Primary: m.Primary, Name: m.Name})
	}
	bounds := f.Image.Bounds()
	for _, tw := range wins {
		if tw.PID == excludePID {
			continue
		}
		r := toFrame(tw.Rect).Intersect(bounds)
		if r.Empty() || tw.Class == "Progman" || tw.Class == "WorkerW" {
			continue
		}
		f.Windows = append(f.Windows, Window{Rect: r, Title: tw.Title, PID: tw.PID})
	}
	return f, nil
}

// PrimaryMonitor returns the primary display, or the first one.
func (f *Frame) PrimaryMonitor() Monitor {
	for _, m := range f.Monitors {
		if m.Primary {
			return m
		}
	}
	if len(f.Monitors) > 0 {
		return f.Monitors[0]
	}
	return Monitor{Rect: f.Image.Bounds(), Primary: true}
}

// MonitorAt returns the monitor containing p (frame coordinates).
func (f *Frame) MonitorAt(p image.Point) (Monitor, bool) {
	for _, m := range f.Monitors {
		if p.In(m.Rect) {
			return m, true
		}
	}
	return Monitor{}, false
}

// WindowAt returns the topmost window containing p.
func (f *Frame) WindowAt(p image.Point) (Window, bool) {
	for _, w := range f.Windows {
		if p.In(w.Rect) {
			return w, true
		}
	}
	return Window{}, false
}
