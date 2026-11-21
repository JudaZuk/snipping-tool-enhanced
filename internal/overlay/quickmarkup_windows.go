// SPDX-License-Identifier: Apache-2.0

//go:build windows

package overlay

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"snippingtoolenhanced/internal/markup"
	"snippingtoolenhanced/internal/render"
	"snippingtoolenhanced/internal/win"
)

// Quick markup: after the area is selected the overlay stays open; the area
// can be marked up, re-cropped with grabbers, then copied, saved or shared.

type mkState struct {
	sel    image.Rectangle
	poly   []image.Point // freeform outline (no grabbers)
	square bool          // Square mode keeps the area square
	doc    markup.Doc
	st     MarkupState
	bar    MarkupBar
	barImg *image.RGBA
	fly    Flyout
	flyImg *image.RGBA
	dib    *win.DIB // the screen with the committed marks
	cur    *markup.Object
	drag   string // "draw", "erase", "slider", "grab:<handle>"
	start  image.Point
	from   image.Rectangle // selection when a grab started
}

// quickOn reports whether a finished selection goes to Quick markup.
func (s *state) quickOn() bool {
	return s.quick && s.kind == KindSnip && !s.textActive && !s.colorActive
}

func (s *state) setQuick(on bool) {
	s.quick = on
	if on {
		s.textActive, s.colorActive = false, false
		s.invalidate(s.loupeRect, 2)
		s.loupeRect = image.Rectangle{}
	}
	s.renderBar()
}

// enterMarkup switches the overlay from selecting to marking up sel.
func (s *state) enterMarkup(sel image.Rectangle, poly []image.Point) {
	d, err := win.NewDIB(int(s.w), int(s.h))
	if err != nil {
		s.result = Result{Outcome: Captured, Kind: s.kind, Mode: s.mode, Rect: sel, Poly: poly}
		s.done = true
		return
	}
	win.BitBlt(d.DC, 0, 0, s.w, s.h, s.bright.DC, 0, 0)
	win.GdiFlush()
	s.mk = &mkState{sel: sel, poly: poly, square: s.mode == Square, dib: d, st: MarkupState{Ink: DefaultInk()}}
	s.dragging = false
	win.ReleaseCapture()
	s.layoutMarkup()
	win.InvalidateRect(s.hwnd, win.Rect{Right: s.w, Bottom: s.h})
}

func (s *state) closeMarkup() {
	if s.mk != nil && s.mk.dib != nil {
		s.mk.dib.Close()
	}
}

func (s *state) monitorFor(r image.Rectangle) image.Rectangle {
	c := image.Pt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2)
	if m, ok := s.opts.Frame.MonitorAt(c); ok {
		return m.Rect
	}
	return s.opts.Frame.PrimaryMonitor().Rect
}

// layoutMarkup places the toolbar (and open flyout) for the current selection.
func (s *state) layoutMarkup() {
	m := s.mk
	if m.barImg != nil {
		s.invalidate(m.barImg.Bounds(), 0)
	}
	m.bar = LayoutMarkupBar(m.sel, s.monitorFor(m.sel), s.scale)
	s.renderMarkupBar()
	s.renderFlyout()
}

func (s *state) renderMarkupBar() {
	m := s.mk
	if m.barImg != nil {
		s.invalidate(m.barImg.Bounds(), 0)
	}
	m.st.CanUndo = len(m.doc.Objects) > 0
	m.barImg = m.bar.Render(s.th, m.st)
	s.invalidate(m.barImg.Bounds(), 0)
}

func (s *state) renderFlyout() {
	m := s.mk
	if m.flyImg != nil {
		s.invalidate(m.flyImg.Bounds(), 0)
	}
	m.flyImg = nil
	m.fly = Flyout{}
	if m.st.Flyout == "" {
		return
	}
	m.fly = m.bar.LayoutFlyout(m.st.Flyout)
	m.flyImg = m.fly.Render(s.th, m.st.Ink, m.st.Hover)
	s.invalidate(m.flyImg.Bounds(), 0)
}

func (s *state) setFlyout(tool string) {
	s.mk.st.Flyout = tool
	s.renderFlyout()
	s.renderMarkupBar() // tooltips hide while a flyout is open
}

// ---------- grabbers ----------

var handleNames = []string{"nw", "n", "ne", "e", "se", "s", "sw", "w"}

func (s *state) handles() map[string]image.Point {
	m := s.mk
	if m.poly != nil {
		return nil
	}
	r := m.sel
	cx, cy := (r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2
	all := map[string]image.Point{"nw": r.Min, "n": {cx, r.Min.Y}, "ne": {r.Max.X, r.Min.Y}, "e": {r.Max.X, cy},
		"se": r.Max, "s": {cx, r.Max.Y}, "sw": {r.Min.X, r.Max.Y}, "w": {r.Min.X, cy}}
	if m.square { // a square stays square: corners only
		for _, k := range []string{"n", "e", "s", "w"} {
			delete(all, k)
		}
	}
	return all
}

func (s *state) handleAt(p image.Point) string {
	rad := int(10 * s.scale)
	for _, name := range handleNames {
		if h, ok := s.handles()[name]; ok && abs(p.X-h.X) <= rad && abs(p.Y-h.Y) <= rad {
			return name
		}
	}
	return ""
}

// cropTo moves the grabbed edges to p.
func (s *state) cropTo(p image.Point, keepSquare bool) {
	m := s.mk
	r := m.from
	h := strings.TrimPrefix(m.drag, "grab:")
	minSize := int(16 * s.scale)
	b := s.opts.Frame.Image.Bounds()
	p.X, p.Y = max(b.Min.X, min(p.X, b.Max.X)), max(b.Min.Y, min(p.Y, b.Max.Y))
	if strings.Contains(h, "w") {
		r.Min.X = min(p.X, r.Max.X-minSize)
	}
	if strings.Contains(h, "e") {
		r.Max.X = max(p.X, r.Min.X+minSize)
	}
	if strings.Contains(h, "n") {
		r.Min.Y = min(p.Y, r.Max.Y-minSize)
	}
	if strings.Contains(h, "s") {
		r.Max.Y = max(p.Y, r.Min.Y+minSize)
	}
	if keepSquare && len(h) == 2 {
		side := max(r.Dx(), r.Dy())
		if strings.Contains(h, "w") {
			r.Min.X = r.Max.X - side
		} else {
			r.Max.X = r.Min.X + side
		}
		if strings.Contains(h, "n") {
			r.Min.Y = r.Max.Y - side
		} else {
			r.Max.Y = r.Min.Y + side
		}
		r = r.Intersect(b)
	}
	if r == m.sel {
		return
	}
	pad := int(12*s.scale) + 2
	s.invalidate(m.sel, pad)
	m.sel = r
	s.invalidate(m.sel, pad)
	s.layoutMarkup()
}

// ---------- marks ----------

// region returns the marked-up screen pixels in r (from the DIB).
func (s *state) region(r image.Rectangle) *image.RGBA {
	img := image.NewRGBA(r)
	bits, w := s.mk.dib.Bits, int(s.w)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		si := (y*w + r.Min.X) * 4
		di := img.PixOffset(r.Min.X, y)
		for x := r.Min.X; x < r.Max.X; x, si, di = x+1, si+4, di+4 {
			img.Pix[di], img.Pix[di+1], img.Pix[di+2], img.Pix[di+3] = bits[si+2], bits[si+1], bits[si], 255
		}
	}
	return img
}

func (s *state) store(img *image.RGBA) {
	r := img.Bounds()
	bits, w := s.mk.dib.Bits, int(s.w)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		di := (y*w + r.Min.X) * 4
		si := img.PixOffset(r.Min.X, y)
		for x := r.Min.X; x < r.Max.X; x, si, di = x+1, si+4, di+4 {
			bits[di], bits[di+1], bits[di+2] = img.Pix[si+2], img.Pix[si+1], img.Pix[si]
		}
	}
}

// commit adds the object being drawn.
func (s *state) commit() {
	m := s.mk
	o := m.cur
	m.cur = nil
	if o == nil {
		return
	}
	m.doc.Add(*o)
	r := o.Bounds().Intersect(s.opts.Frame.Image.Bounds())
	if !r.Empty() {
		win.GdiFlush()
		img := s.region(r)
		markup.Draw(img, *o)
		s.store(img)
	}
	s.invalidate(r, 2)
	s.renderMarkupBar()
}

// redraw rebuilds the marked-up pixels in r from the screen and the marks.
func (s *state) redraw(r image.Rectangle) {
	r = r.Intersect(s.opts.Frame.Image.Bounds())
	if r.Empty() {
		return
	}
	img := image.NewRGBA(r)
	copy8(img, s.opts.Frame.Image, r)
	for _, o := range s.mk.doc.Objects {
		if o.Bounds().Overlaps(r) {
			markup.Draw(img, o)
		}
	}
	win.GdiFlush()
	s.store(img)
	s.invalidate(r, 2)
	s.renderMarkupBar()
}

func copy8(dst, src *image.RGBA, r image.Rectangle) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		copy(dst.Pix[dst.PixOffset(r.Min.X, y):dst.PixOffset(r.Max.X, y)], src.Pix[src.PixOffset(r.Min.X, y):src.PixOffset(r.Max.X, y)])
	}
}

func fpt(p image.Point) render.Pt { return render.Pt{X: float64(p.X) + 0.5, Y: float64(p.Y) + 0.5} }

// inArea reports whether p is inside the area being marked up (for a
// freeform area, its bounding box: marks are clipped to the shape).
func (s *state) inArea(p image.Point) bool { return p.In(s.mk.sel) }

func pointInPoly(p image.Point, poly []image.Point) bool {
	in := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		a, b := poly[i], poly[j]
		if (a.Y > p.Y) != (b.Y > p.Y) && float64(p.X) < float64(b.X-a.X)*float64(p.Y-a.Y)/float64(b.Y-a.Y)+float64(a.X) {
			in = !in
		}
	}
	return in
}

// markResult is the outcome with the current marks.
func (s *state) markResult(action string) Result {
	m := s.mk
	return Result{Outcome: Captured, Kind: KindSnip, Mode: s.mode, Rect: m.sel, Poly: m.poly,
		Marks: append([]markup.Object(nil), m.doc.Objects...), Action: action}
}

func (s *state) act(id string) {
	switch id {
	case ActCopy, ActShare:
		s.result = s.markResult(id)
		s.done = true
	case ActSave:
		if s.opts.Save == nil {
			return
		}
		s.mk.st.Hover = ""
		s.renderMarkupBar()
		if s.opts.Save(s.hwnd, s.markResult(ActSave)) {
			s.result = s.markResult(ActSave)
			s.done = true
		}
	case ActClose:
		s.cancel()
	}
}

func (s *state) undo(redo bool) {
	m := s.mk
	var r image.Rectangle
	if redo {
		r = m.doc.Redo()
	} else {
		r = m.doc.Undo()
	}
	s.redraw(r)
}

// ---------- input ----------

func (s *state) clickTool(id string) {
	m := s.mk
	switch id {
	case ToolEmoji:
		if m.st.Ink.Emoji >= 0 {
			m.st.Ink.Tool = ToolEmoji
		}
		if m.st.Flyout == ToolEmoji {
			s.setFlyout("")
		} else {
			s.setFlyout(ToolEmoji)
		}
	case ToolPen, ToolHighlight, ToolEraser, ToolShapes:
		if m.st.Ink.Tool == id {
			if m.st.Flyout == id {
				s.setFlyout("")
			} else {
				s.setFlyout(id)
			}
			return
		}
		m.st.Ink.Tool = id
		s.setFlyout("")
	default:
		s.setFlyout("")
		s.act(id)
	}
}

func (s *state) clickFlyout(hit string, p image.Point) {
	m := s.mk
	ink := &m.st.Ink
	var i int
	switch {
	case strings.HasPrefix(hit, "c:"):
		fmt.Sscanf(hit, "c:%d", &i)
		switch m.st.Flyout {
		case ToolPen:
			ink.PenColor = PenColors[i]
		case ToolHighlight:
			ink.HLColor = HighlightColors[i]
		case ToolShapes:
			ink.ShapeColor = ShapeColors[i]
		}
	case strings.HasPrefix(hit, "k:"):
		fmt.Sscanf(hit, "k:%d", &i)
		ink.ShapeKind = ShapeKinds[i]
	case strings.HasPrefix(hit, "e:"):
		fmt.Sscanf(hit, "e:%d", &i)
		ink.Emoji, ink.Tool = i, ToolEmoji
		s.setFlyout("")
		return
	case hit == "eraseall":
		s.redraw(m.doc.Clear())
		s.setFlyout("")
		return
	case hit == "slider":
		m.drag = "slider"
		win.SetCapture(s.hwnd)
		s.slide(p.X)
		return
	default:
		return
	}
	s.renderFlyout()
	s.renderMarkupBar()
}

func (s *state) slide(x int) {
	m := s.mk
	v := m.fly.SliderValue(x)
	switch m.st.Flyout {
	case ToolPen:
		m.st.Ink.PenSize = v
	case ToolHighlight:
		m.st.Ink.HLSize = v
	case ToolShapes:
		m.st.Ink.ShapeSize = v
	}
	s.renderFlyout()
}

func (s *state) beginMark(p image.Point) {
	m := s.mk
	ink := m.st.Ink
	switch ink.Tool {
	case ToolPen:
		m.cur = &markup.Object{Kind: markup.Pen, Color: ink.PenColor, Width: float64(ink.PenSize), Points: []render.Pt{fpt(p)}}
	case ToolHighlight:
		m.cur = &markup.Object{Kind: markup.Highlighter, Color: ink.HLColor, Width: float64(ink.HLSize), Points: []render.Pt{fpt(p)}}
	case ToolShapes:
		m.cur = &markup.Object{Kind: ink.ShapeKind, Color: ink.ShapeColor, Width: float64(ink.ShapeSize), Points: []render.Pt{fpt(p), fpt(p)}}
	case ToolEmoji:
		if ink.Emoji < 0 {
			s.setFlyout(ToolEmoji)
			return
		}
		m.cur = &markup.Object{Kind: markup.Emoji, Emoji: ink.Emoji, Box: s.emojiBox(p, p)}
	case ToolEraser:
		m.drag = "erase"
		m.start = p
		win.SetCapture(s.hwnd)
		s.redraw(m.doc.Erase(fpt(p), 4*s.scale))
		return
	}
	m.drag, m.start = "draw", p
	win.SetCapture(s.hwnd)
	s.invalidate(m.cur.Bounds(), 2)
}

// emojiBox is a click (default size, centered) or a dragged square.
func (s *state) emojiBox(a, b image.Point) image.Rectangle {
	side := max(abs(b.X-a.X), abs(b.Y-a.Y))
	if side < int(12*s.scale) {
		d := int(math.Round(48 * s.scale))
		return image.Rect(a.X-d/2, a.Y-d/2, a.X-d/2+d, a.Y-d/2+d)
	}
	x0, y0 := a.X, a.Y
	if b.X < a.X {
		x0 = a.X - side
	}
	if b.Y < a.Y {
		y0 = a.Y - side
	}
	return image.Rect(x0, y0, x0+side, y0+side)
}

func (s *state) moveMark(p image.Point) {
	m := s.mk
	switch m.drag {
	case "erase":
		// Erase along the path, so fast movements don't skip over marks.
		a, b := fpt(m.start), fpt(p)
		n := int(math.Max(1, math.Hypot(b.X-a.X, b.Y-a.Y)/(3*s.scale)))
		var changed image.Rectangle
		for i := 1; i <= n; i++ {
			t := float64(i) / float64(n)
			changed = changed.Union(m.doc.Erase(render.Pt{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t}, 4*s.scale))
		}
		m.start = p
		s.redraw(changed)
		return
	case "slider":
		s.slide(p.X)
		return
	}
	if strings.HasPrefix(m.drag, "grab:") {
		s.cropTo(p, m.square || win.KeyDown(win.VK_SHIFT))
		return
	}
	o := m.cur
	if o == nil {
		return
	}
	before := o.Bounds()
	switch o.Kind {
	case markup.Pen, markup.Highlighter:
		last := o.Points[len(o.Points)-1]
		if math.Abs(fpt(p).X-last.X)+math.Abs(fpt(p).Y-last.Y) < 1.5 {
			return
		}
		o.Points = append(o.Points, fpt(p))
		// Only the newest segments change.
		n := len(o.Points)
		seg := markup.Object{Kind: o.Kind, Width: o.Width, Points: o.Points[max(0, n-4):]}
		s.invalidate(seg.Bounds(), 2)
		return
	case markup.Emoji:
		o.Box = s.emojiBox(m.start, p)
	default:
		end := p
		if win.KeyDown(win.VK_SHIFT) { // straight angles / square shapes
			end = constrain(m.start, p, o.Kind)
		}
		o.Points[1] = fpt(end)
	}
	s.invalidate(before.Union(o.Bounds()), 2)
}

// constrain snaps lines to 45° steps and rectangles/ovals to squares/circles.
func constrain(a, b image.Point, k markup.Kind) image.Point {
	dx, dy := b.X-a.X, b.Y-a.Y
	if k == markup.Line || k == markup.Arrow {
		ang := math.Round(math.Atan2(float64(dy), float64(dx))/(math.Pi/4)) * (math.Pi / 4)
		l := math.Hypot(float64(dx), float64(dy))
		return image.Pt(a.X+int(math.Round(l*math.Cos(ang))), a.Y+int(math.Round(l*math.Sin(ang))))
	}
	side := max(abs(dx), abs(dy))
	sx, sy := 1, 1
	if dx < 0 {
		sx = -1
	}
	if dy < 0 {
		sy = -1
	}
	return image.Pt(a.X+sx*side, a.Y+sy*side)
}

func (s *state) endMark() {
	m := s.mk
	drag := m.drag
	m.drag = ""
	win.ReleaseCapture()
	switch {
	case drag == "draw":
		s.commit()
	case strings.HasPrefix(drag, "grab:"):
		s.layoutMarkup()
	}
}

func (s *state) markupHover(p image.Point) {
	m := s.mk
	h := m.fly.Hit(p)
	if h == "" || h == "fly" {
		h = m.bar.Hit(p)
	}
	if h != m.st.Hover {
		m.st.Hover = h
		s.renderMarkupBar()
		if m.st.Flyout != "" {
			s.renderFlyout()
		}
	}
}

func (s *state) markupCursor() uintptr {
	m := s.mk
	p := s.cursor()
	if m.drag != "" && strings.HasPrefix(m.drag, "grab:") {
		return cursorFor(strings.TrimPrefix(m.drag, "grab:"))
	}
	if m.fly.Hit(p) != "" || m.bar.Hit(p) != "" {
		return win.IDC_ARROW
	}
	if h := s.handleAt(p); h != "" {
		return cursorFor(h)
	}
	if s.inArea(p) {
		return win.IDC_CROSS
	}
	return win.IDC_ARROW
}

func cursorFor(h string) uintptr {
	switch h {
	case "nw", "se":
		return win.IDC_SIZENWSE
	case "ne", "sw":
		return win.IDC_SIZENESW
	case "e", "w":
		return win.IDC_SIZEWE
	}
	return win.IDC_SIZENS
}

// markupProc handles input while marking up.
func (s *state) markupProc(msg uint32, wp, lp uintptr) (uintptr, bool) {
	m := s.mk
	switch msg {
	case win.WM_SETCURSOR:
		win.SetCursor(win.LoadCursor(s.markupCursor()))
		return 1, true
	case win.WM_KEYDOWN, win.WM_SYSKEYDOWN:
		ctrl := win.KeyDown(win.VK_CONTROL)
		switch {
		case wp == win.VK_ESCAPE:
			if m.st.Flyout != "" {
				s.setFlyout("")
			} else {
				s.cancel()
			}
		case ctrl && wp == 'Z':
			s.undo(false)
		case ctrl && wp == 'Y':
			s.undo(true)
		case ctrl && wp == 'C':
			s.act(ActCopy)
		case ctrl && wp == 'S':
			s.act(ActSave)
		}
		return 0, true
	case win.WM_RBUTTONDOWN, win.WM_MOUSEWHEEL:
		return 0, true // keep the marks: only Esc or Close cancels
	case win.WM_LBUTTONDOWN:
		p := pt(lp)
		if m.st.Flyout != "" {
			if h := m.fly.Hit(p); h != "" {
				s.clickFlyout(h, p)
				return 0, true
			}
			if m.bar.Hit(p) == "" { // a click elsewhere only closes the flyout
				s.setFlyout("")
				return 0, true
			}
		}
		if h := m.bar.Hit(p); h != "" {
			if h != "bar" {
				s.clickTool(h)
			}
			return 0, true
		}
		if h := s.handleAt(p); h != "" {
			m.drag, m.from = "grab:"+h, m.sel
			win.SetCapture(s.hwnd)
			return 0, true
		}
		if s.inArea(p) {
			s.beginMark(p)
		}
		return 0, true
	case win.WM_MOUSEMOVE:
		p := pt(lp)
		if m.drag != "" {
			s.moveMark(p)
		} else {
			s.markupHover(p)
		}
		return 0, true
	case win.WM_LBUTTONUP:
		if m.drag != "" {
			s.endMark()
		}
		return 0, true
	case win.WM_CAPTURECHANGED:
		if m.drag != "" {
			m.drag = ""
			s.commit()
			s.layoutMarkup()
		}
		return 0, true
	}
	return 0, false
}

// ---------- painting ----------

func (s *state) paintMarkup(pr image.Rectangle) {
	m := s.mk
	bdc := s.back.DC
	sel := m.sel
	if m.poly != nil {
		pts := make([]win.Point, len(m.poly))
		for i, p := range m.poly {
			pts[i] = win.Point{X: int32(p.X), Y: int32(p.Y)}
		}
		win.ClipPolygon(bdc, pts)
	}
	if r := sel.Intersect(pr); !r.Empty() {
		win.BitBlt(bdc, int32(r.Min.X), int32(r.Min.Y), int32(r.Dx()), int32(r.Dy()), m.dib.DC, int32(r.Min.X), int32(r.Min.Y))
	}
	win.ClipPolygon(bdc, nil)
	win.GdiFlush()
	// The mark being drawn.
	if m.cur != nil {
		if r := m.cur.Bounds().Intersect(sel).Intersect(pr); !r.Empty() {
			img := s.region(r)
			markup.Draw(img, *m.cur)
			if m.poly != nil {
				for y := r.Min.Y; y < r.Max.Y; y++ { // keep freeform marks inside the shape
					for x := r.Min.X; x < r.Max.X; x++ {
						if !pointInPoly(image.Pt(x, y), m.poly) {
							i := img.PixOffset(x, y)
							img.Pix[i+3] = 0
						}
					}
				}
			}
			s.blend(img, pr)
		}
	}
	// Outline and grabbers.
	if m.poly != nil {
		pts := make([]win.Point, len(m.poly)+1)
		for i, p := range m.poly {
			pts[i] = win.Point{X: int32(p.X), Y: int32(p.Y)}
		}
		pts[len(m.poly)] = pts[0]
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
		win.GdiFlush()
	} else {
		ui := image.NewRGBA(sel.Inset(-int(12*s.scale) - 2).Intersect(pr))
		f := func(v int) float64 { return float64(v) }
		render.StrokeRoundRect(ui, f(sel.Min.X)-1, f(sel.Min.Y)-1, f(sel.Dx())+2, f(sel.Dy())+2, 0, 1, color.RGBA{255, 255, 255, 255})
		d := 10 * s.scale
		for _, h := range s.handles() {
			x, y := f(h.X)-d/2, f(h.Y)-d/2
			render.FillRoundRect(ui, x-1, y-1, d+2, d+2, (d+2)/2, render.Light.ShadowBase)
			render.FillRoundRect(ui, x, y, d, d, d/2, render.Light.Surface)
		}
		s.blend(ui, pr)
	}
	if m.barImg != nil {
		s.blend(m.barImg, pr)
	}
	if m.flyImg != nil {
		s.blend(m.flyImg, pr)
	}
}

// blend composites img (premultiplied) over the back buffer inside clip.
func (s *state) blend(img *image.RGBA, clip image.Rectangle) {
	r := img.Bounds().Intersect(clip).Intersect(image.Rect(0, 0, int(s.w), int(s.h)))
	bits, w := s.back.Bits, int(s.w)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		si := img.PixOffset(r.Min.X, y)
		di := (y*w + r.Min.X) * 4
		for x := r.Min.X; x < r.Max.X; x, si, di = x+1, si+4, di+4 {
			a := uint32(img.Pix[si+3])
			switch a {
			case 0:
			case 255:
				bits[di], bits[di+1], bits[di+2] = img.Pix[si+2], img.Pix[si+1], img.Pix[si]
			default:
				ia := 255 - a
				bits[di] = uint8(uint32(img.Pix[si+2]) + uint32(bits[di])*ia/255)
				bits[di+1] = uint8(uint32(img.Pix[si+1]) + uint32(bits[di+1])*ia/255)
				bits[di+2] = uint8(uint32(img.Pix[si]) + uint32(bits[di+2])*ia/255)
			}
		}
	}
}
