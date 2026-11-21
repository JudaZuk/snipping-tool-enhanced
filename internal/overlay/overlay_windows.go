// SPDX-License-Identifier: Apache-2.0

//go:build windows

package overlay

import (
	"fmt"
	"image"
	"image/color"
	"syscall"

	"golang.org/x/sys/windows"

	"snippingtoolenhanced/internal/capture"
	"snippingtoolenhanced/internal/imaging"
	"snippingtoolenhanced/internal/render"
	"snippingtoolenhanced/internal/win"
)

// Outcome of an overlay session.
type Outcome int

const (
	Captured Outcome = iota
	Cancelled
	ColorPicked
	TextSelected // Text Extractor: Rect holds the area to read
)

// Options configures one overlay session.
type Options struct {
	Frame *capture.Frame
	Kind  Kind // snip or record
	Mode  Mode
	Text  bool // start in Text Extractor
	Dark  bool
}

// Result is what the user picked, in frame coordinates.
type Result struct {
	Outcome   Outcome
	Kind      Kind
	Mode      Mode
	Rect      image.Rectangle // rectangle, square, window, full screen
	Poly      []image.Point   // freeform outline
	Color     color.RGBA      // color picker
	ColorText string
}

const className = "SnippingToolEnhancedOverlay"

type state struct {
	opts        Options
	th          render.Theme
	hwnd        windows.HWND
	bright, dim *win.DIB
	dimImg      *image.RGBA
	back        *win.DIB
	w, h        int32
	scale       float64
	kind        Kind
	mode        Mode
	menuOpen    bool
	colorActive bool
	textActive  bool
	loupe       Loupe
	loupeRect   image.Rectangle
	bar         Bar
	barImg      *win.DIB
	hover       string
	dragging    bool
	start       image.Point
	sel         image.Rectangle
	poly        []image.Point
	done        bool
	result      Result
}

var st *state

var wndProc = syscall.NewCallback(func(h, msg, wp, lp uintptr) uintptr {
	if st == nil || windows.HWND(h) != st.hwnd {
		return win.DefWindowProc(windows.HWND(h), uint32(msg), wp, lp)
	}
	return st.proc(uint32(msg), wp, lp)
})

// Run shows the overlay and blocks, pumping messages, until the user picks
// something or cancels. The calling goroutine must be locked to its thread.
func Run(o Options) (Result, error) {
	if st != nil {
		return Result{}, fmt.Errorf("overlay already open")
	}
	if o.Frame == nil {
		return Result{}, fmt.Errorf("overlay: no frame")
	}
	if o.Kind == "" {
		o.Kind = KindSnip
	}
	if o.Mode == "" || !hasMode(ModesFor(o.Kind), o.Mode) {
		o.Mode = Rectangle
	}
	if err := win.RegisterClass(className, wndProc, win.IDC_CROSS, 0); err != nil {
		return Result{}, err
	}
	s := &state{opts: o, kind: o.Kind, mode: o.Mode, textActive: o.Text && o.Kind == KindSnip, th: render.Light, loupe: Loupe{Sample: 11}, result: Result{Outcome: Cancelled}}
	if o.Dark {
		s.th = render.Dark
	}
	b := o.Frame.Image.Bounds()
	s.w, s.h = int32(b.Dx()), int32(b.Dy())

	// Dimmed backdrop (55%), as an image for compositing and a DIB for blitting.
	s.dimImg = image.NewRGBA(b)
	for i := 0; i < len(o.Frame.Image.Pix); i += 4 {
		px := o.Frame.Image.Pix
		s.dimImg.Pix[i] = uint8(uint32(px[i]) * 141 >> 8)
		s.dimImg.Pix[i+1] = uint8(uint32(px[i+1]) * 141 >> 8)
		s.dimImg.Pix[i+2] = uint8(uint32(px[i+2]) * 141 >> 8)
		s.dimImg.Pix[i+3] = 255
	}
	var err error
	if s.bright, err = dibFrom(o.Frame.Image); err != nil {
		return Result{}, err
	}
	defer s.bright.Close()
	if s.dim, err = dibFrom(s.dimImg); err != nil {
		return Result{}, err
	}
	defer s.dim.Close()
	if s.back, err = win.NewDIB(int(s.w), int(s.h)); err != nil {
		return Result{}, err
	}
	defer s.back.Close()

	st = s
	defer func() { st = nil }()
	s.hwnd, err = win.CreateWindow(win.WS_EX_TOPMOST|win.WS_EX_TOOLWINDOW, className, "Snipping Tool Enhanced",
		win.WS_POPUP, int32(o.Frame.Origin.X), int32(o.Frame.Origin.Y), s.w, s.h)
	if err != nil {
		return Result{}, err
	}
	s.scale = float64(win.DpiForWindow(s.hwnd)) / 96

	// Capture bar on the monitor under the pointer.
	cur := s.cursor()
	mon := o.Frame.PrimaryMonitor().Rect
	if m, ok := o.Frame.MonitorAt(cur); ok {
		mon = m.Rect
	}
	s.bar = LayoutBar(mon, s.scale)
	s.renderBar()
	defer s.barImg.Close()

	win.ShowWindow(s.hwnd, win.SW_SHOW)
	win.UpdateWindow(s.hwnd)
	win.SetForegroundWindow(s.hwnd)
	win.SetFocus(s.hwnd)
	s.hoverAt(cur)

	var m win.Msg
	quit := false
	for !s.done {
		if !win.GetMessage(&m) {
			quit = true
			break
		}
		win.TranslateMessage(&m)
		win.DispatchMessage(&m)
	}
	win.DestroyWindow(s.hwnd)
	if quit {
		win.PostQuitMessage(int(m.WParam))
	}
	return s.result, nil
}

func dibFrom(img *image.RGBA) (*win.DIB, error) {
	b := img.Bounds()
	d, err := win.NewDIB(b.Dx(), b.Dy())
	if err != nil {
		return nil, err
	}
	copy(d.Bits, imaging.RGBAToBGRA(img, 0))
	return d, nil
}

func (s *state) cursor() image.Point {
	p := win.CursorPos()
	return image.Pt(int(p.X)-s.opts.Frame.Origin.X, int(p.Y)-s.opts.Frame.Origin.Y)
}

func (s *state) renderBar() {
	img := s.bar.Render(s.dimImg, s.th, s.barState())
	if s.barImg != nil {
		s.barImg.Close()
	}
	s.barImg, _ = dibFrom(img)
	win.InvalidateRect(s.hwnd, toWin(s.bar.Area))
}

func (s *state) invalidate(r image.Rectangle, pad int) {
	if !r.Empty() {
		win.InvalidateRect(s.hwnd, toWin(r.Inset(-pad)))
	}
}

func (s *state) setSel(r image.Rectangle) {
	if r == s.sel {
		return
	}
	s.invalidate(s.sel, 4)
	s.sel = r
	s.invalidate(s.sel, 4)
	s.invalidate(s.bar.Area, 0)
}

func (s *state) barState() BarState {
	return BarState{Kind: s.kind, Mode: s.mode, ColorActive: s.colorActive, TextActive: s.textActive, Hover: s.hover, MenuOpen: s.menuOpen}
}

func hasMode(list []Mode, m Mode) bool {
	for _, x := range list {
		if x == m {
			return true
		}
	}
	return false
}

func (s *state) setKind(k Kind) {
	s.kind = k
	if !hasMode(ModesFor(k), s.mode) {
		s.mode = Rectangle
	}
	s.setMode(s.mode)
}

func (s *state) setMode(m Mode) {
	s.colorActive = false
	s.textActive = false
	s.menuOpen = false
	s.mode = m
	s.dragging = false
	win.ReleaseCapture()
	s.invalidate(polyBounds(s.poly), 4)
	s.poly = nil
	s.invalidate(s.loupeRect, 2)
	s.loupeRect = image.Rectangle{}
	s.setSel(image.Rectangle{})
	s.renderBar()
	s.hoverAt(s.cursor())
}

func (s *state) setTextExtractor(on bool) {
	s.textActive = on
	s.colorActive = false
	s.menuOpen = false
	s.dragging = false
	s.invalidate(s.loupeRect, 2)
	s.loupeRect = image.Rectangle{}
	s.setSel(image.Rectangle{})
	s.renderBar()
}

func (s *state) setColorPicker(on bool) {
	s.textActive = false
	s.colorActive = on
	s.setSel(image.Rectangle{})
	s.invalidate(s.loupeRect, 2)
	s.loupeRect = image.Rectangle{}
	s.renderBar()
	s.hoverAt(s.cursor())
}

func (s *state) cycleMode(dir int) {
	modes := ModesFor(s.kind)
	for i, m := range modes {
		if m == s.mode {
			s.setMode(modes[(i+dir+len(modes))%len(modes)])
			return
		}
	}
}

// hoverAt updates bar hover, window/monitor highlight and the loupe.
func (s *state) hoverAt(p image.Point) {
	h := s.bar.Hit(p, s.barState())
	if h != s.hover {
		s.hover = h
		s.renderBar()
	}
	if s.colorActive {
		s.invalidate(s.loupeRect, 2)
		if h == "" {
			s.loupeRect = s.loupe.LoupeRect(p, s.opts.Frame.Image.Bounds(), s.scale).Inset(-int(12 * s.scale))
			s.invalidate(s.loupeRect, 2)
		} else {
			s.loupeRect = image.Rectangle{}
		}
		return
	}
	if s.textActive {
		return
	}
	if s.dragging || h != "" {
		if !s.dragging && (s.mode == Window || s.mode == Fullscreen) {
			s.setSel(image.Rectangle{})
		}
		return
	}
	switch s.mode {
	case Window:
		if w, ok := s.opts.Frame.WindowAt(p); ok {
			s.setSel(w.Rect)
		} else {
			s.setSel(image.Rectangle{})
		}
	case Fullscreen:
		if m, ok := s.opts.Frame.MonitorAt(p); ok {
			s.setSel(m.Rect)
		}
	}
}

func (s *state) finishRect(r image.Rectangle) {
	r = r.Intersect(s.opts.Frame.Image.Bounds())
	if r.Dx() < 2 || r.Dy() < 2 {
		return
	}
	s.result = Result{Outcome: Captured, Kind: s.kind, Mode: s.mode, Rect: r}
	s.done = true
}

func (s *state) cancel() {
	s.result = Result{Outcome: Cancelled}
	s.done = true
}

func pt(lp uintptr) image.Point { return image.Pt(int(win.LoInt(lp)), int(win.HiInt(lp))) }

func (s *state) proc(msg uint32, wp, lp uintptr) uintptr {
	switch msg {
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_PAINT:
		s.paint()
		return 0
	case win.WM_SETCURSOR:
		if s.hover != "" && s.hover != "bar" {
			win.SetCursor(win.LoadCursor(win.IDC_ARROW))
		} else {
			win.SetCursor(win.LoadCursor(win.IDC_CROSS))
		}
		return 1
	case win.WM_KEYDOWN, win.WM_SYSKEYDOWN:
		shift := win.KeyDown(win.VK_SHIFT)
		switch wp {
		case win.VK_ESCAPE:
			if s.menuOpen {
				s.menuOpen = false
				s.renderBar()
			} else if s.colorActive {
				s.setColorPicker(false)
			} else if s.textActive {
				s.setTextExtractor(false)
			} else {
				s.cancel()
			}
		case win.VK_LEFT:
			if shift {
				s.cycleMode(-1)
			}
		case win.VK_RIGHT:
			if shift {
				s.cycleMode(1)
			}
		case win.VK_TAB:
			if s.colorActive {
				s.loupe.Format = (s.loupe.Format + 1) % len(ColorFormats)
				s.invalidate(s.loupeRect, 2)
			}
		case win.VK_OEM_PLUS, win.VK_ADD:
			s.zoomLoupe(-2)
		case win.VK_OEM_MINUS, win.VK_SUBTRACT:
			s.zoomLoupe(2)
		}
		return 0
	case win.WM_MOUSEWHEEL:
		if s.colorActive {
			if int16(uint16(wp>>16)) > 0 {
				s.zoomLoupe(-2)
			} else {
				s.zoomLoupe(2)
			}
		}
		return 0
	case win.WM_RBUTTONDOWN:
		s.cancel()
		return 0
	case win.WM_LBUTTONDOWN:
		p := pt(lp)
		hit := s.bar.Hit(p, s.barState())
		if m, ok := MenuMode(hit); ok {
			s.setMode(m)
			return 0
		}
		if s.menuOpen && hit != ItemMode {
			s.menuOpen = false
			s.renderBar()
			if hit == "" || hit == "menu" {
				return 0
			}
		}
		switch hit {
		case "bar", "menu":
			return 0
		case ItemClose:
			s.cancel()
			return 0
		case ItemSnip:
			s.setKind(KindSnip)
			return 0
		case ItemRecord:
			s.setKind(KindRecord)
			return 0
		case ItemMode:
			s.menuOpen = !s.menuOpen
			s.renderBar()
			return 0
		case ItemColor:
			if Enabled(ItemColor, s.kind) {
				s.setColorPicker(!s.colorActive)
			}
			return 0
		case ItemText:
			if Enabled(ItemText, s.kind) {
				s.setTextExtractor(!s.textActive)
			}
			return 0
		case ItemMarkup:
			return 0 // not available yet
		}
		if s.colorActive {
			c := s.opts.Frame.Image.RGBAAt(p.X, p.Y)
			s.result = Result{Outcome: ColorPicked, Color: c, ColorText: FormatColor(c, s.loupe.Format)}
			s.done = true
			return 0
		}
		if s.textActive { // Text Extractor: drag the area to read
			s.dragging, s.start = true, p
			win.SetCapture(s.hwnd)
			return 0
		}
		switch s.mode {
		case Square, Rectangle:
			s.dragging, s.start = true, p
			win.SetCapture(s.hwnd)
		case Freeform:
			s.dragging = true
			s.poly = []image.Point{p}
			win.SetCapture(s.hwnd)
		case Window, Fullscreen:
			s.hoverAt(p)
			if !s.sel.Empty() {
				s.finishRect(s.sel)
			}
		}
		return 0
	case win.WM_MOUSEMOVE:
		p := pt(lp)
		if s.dragging && s.textActive {
			s.setSel(dragRect(s.start, p, false))
			return 0
		}
		if s.dragging {
			switch s.mode {
			case Freeform:
				last := s.poly[len(s.poly)-1]
				if abs(p.X-last.X)+abs(p.Y-last.Y) >= 2 {
					s.poly = append(s.poly, p)
					s.invalidate(segmentRect(last, p), int(6*s.scale)+4)
				}
			default:
				s.setSel(dragRect(s.start, p, s.mode == Square || win.KeyDown(win.VK_SHIFT)))
			}
			return 0
		}
		s.hoverAt(p)
		return 0
	case win.WM_LBUTTONUP:
		if !s.dragging {
			return 0
		}
		s.dragging = false
		win.ReleaseCapture()
		p := pt(lp)
		if s.textActive {
			r := dragRect(s.start, p, false).Intersect(s.opts.Frame.Image.Bounds())
			if r.Dx() >= 4 && r.Dy() >= 4 {
				s.result = Result{Outcome: TextSelected, Kind: KindSnip, Rect: r}
				s.done = true
			} else {
				s.setSel(image.Rectangle{})
			}
			return 0
		}
		if s.mode == Freeform {
			s.poly = append(s.poly, p)
			if b := polyBounds(s.poly); len(s.poly) >= 3 && b.Dx() >= 4 && b.Dy() >= 4 {
				s.result = Result{Outcome: Captured, Kind: s.kind, Mode: Freeform, Poly: s.poly, Rect: b}
				s.done = true
			} else {
				s.invalidate(polyBounds(s.poly), 6)
				s.poly = nil
			}
			return 0
		}
		r := dragRect(s.start, p, s.mode == Square || win.KeyDown(win.VK_SHIFT))
		if r.Dx() >= 2 && r.Dy() >= 2 {
			s.finishRect(r)
		} else {
			s.setSel(image.Rectangle{})
		}
		return 0
	case win.WM_CAPTURECHANGED:
		s.dragging = false
		return 0
	case win.WM_CLOSE:
		s.cancel()
		return 0
	}
	return win.DefWindowProc(s.hwnd, msg, wp, lp)
}

func (s *state) zoomLoupe(d int) {
	if !s.colorActive {
		return
	}
	n := s.loupe.Sample + d
	if n < 5 || n > 25 {
		return
	}
	s.invalidate(s.loupeRect, 2)
	s.loupe.Sample = n
	s.hoverAt(s.cursor())
}

func (s *state) paint() {
	var ps win.PaintStruct
	hdc := win.BeginPaint(s.hwnd, &ps)
	defer win.EndPaint(s.hwnd, &ps)
	pr := ps.Paint
	bdc := s.back.DC
	win.BitBlt(bdc, pr.Left, pr.Top, pr.Right-pr.Left, pr.Bottom-pr.Top, s.dim.DC, pr.Left, pr.Top)

	if len(s.poly) > 1 {
		pts := make([]win.Point, len(s.poly))
		for i, p := range s.poly {
			pts[i] = win.Point{X: int32(p.X), Y: int32(p.Y)}
		}
		// White line with a dark edge, so it is visible on light and dark content.
		w := int32(2*s.scale + 0.5)
		for _, pn := range []struct {
			w int32
			c uint32
		}{{w + 2, win.RGB(0x20, 0x20, 0x20)}, {w, win.RGB(255, 255, 255)}} {
			pen := win.CreatePen(pn.w, pn.c)
			old := win.SelectObject(bdc, pen)
			win.Polyline(bdc, pts)
			win.SelectObject(bdc, old)
			win.DeleteObject(pen)
		}
	}
	if !s.sel.Empty() {
		r := s.sel
		win.BitBlt(bdc, int32(r.Min.X), int32(r.Min.Y), int32(r.Dx()), int32(r.Dy()), s.bright.DC, int32(r.Min.X), int32(r.Min.Y))
		win.FrameRect(bdc, toWin(r), win.RGB(255, 255, 255), 1)
	}
	if s.colorActive && !s.loupeRect.Empty() {
		img, area, _ := s.loupe.Render(s.opts.Frame.Image, s.dimImg, s.cursor(), s.scale, s.th)
		if d, err := dibFrom(img); err == nil {
			win.BitBlt(bdc, int32(area.Min.X), int32(area.Min.Y), int32(area.Dx()), int32(area.Dy()), d.DC, 0, 0)
			d.Close()
		}
	}
	if s.barImg != nil {
		a := s.bar.Area
		win.BitBlt(bdc, int32(a.Min.X), int32(a.Min.Y), int32(a.Dx()), int32(a.Dy()), s.barImg.DC, 0, 0)
	}
	win.BitBlt(hdc, pr.Left, pr.Top, pr.Right-pr.Left, pr.Bottom-pr.Top, bdc, pr.Left, pr.Top)
}

func toWin(r image.Rectangle) win.Rect {
	return win.Rect{Left: int32(r.Min.X), Top: int32(r.Min.Y), Right: int32(r.Max.X), Bottom: int32(r.Max.Y)}
}
