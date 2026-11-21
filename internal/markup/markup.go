// SPDX-License-Identifier: Apache-2.0

// Package markup is the ink model of Quick markup: pen and highlighter
// strokes, shapes and emoji drawn over a capture, with undo and redo.
// Coordinates are pixels of the captured screen.
package markup

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"snippingtoolenhanced/internal/emoji"
	"snippingtoolenhanced/internal/render"
)

// Kind of object.
type Kind int

const (
	Pen Kind = iota
	Highlighter
	Line
	Arrow
	Rectangle
	Oval
	Emoji
)

// Object is one mark. Objects are never changed after they are added.
type Object struct {
	Kind   Kind
	Points []render.Pt // Pen, Highlighter: the stroke; shapes: start and end
	Color  color.RGBA
	Width  float64
	Emoji  int             // index into emoji.List
	Box    image.Rectangle // Emoji: where it is drawn
}

// HighlightAlpha is how strongly the highlighter tints (multiplied in, as in the editor).
const HighlightAlpha = 0.45

func (o Object) arrowHead() float64 { return math.Max(12, o.Width*3.5) }

// Bounds is the area the object can paint.
func (o Object) Bounds() image.Rectangle {
	if o.Kind == Emoji {
		return o.Box
	}
	if len(o.Points) == 0 {
		return image.Rectangle{}
	}
	x0, y0, x1, y1 := o.Points[0].X, o.Points[0].Y, o.Points[0].X, o.Points[0].Y
	for _, p := range o.Points[1:] {
		x0, y0 = math.Min(x0, p.X), math.Min(y0, p.Y)
		x1, y1 = math.Max(x1, p.X), math.Max(y1, p.Y)
	}
	pad := o.Width/2 + 2
	if o.Kind == Arrow {
		pad = math.Max(pad, o.arrowHead()+2)
	}
	return image.Rect(int(math.Floor(x0-pad)), int(math.Floor(y0-pad)), int(math.Ceil(x1+pad)), int(math.Ceil(y1+pad)))
}

// Hit reports whether p (with tolerance tol) touches the object; the eraser uses it.
func (o Object) Hit(p render.Pt, tol float64) bool {
	switch o.Kind {
	case Emoji:
		return image.Pt(int(p.X), int(p.Y)).In(o.Box.Inset(-int(tol)))
	case Pen, Highlighter, Line, Arrow:
		r := o.Width/2 + tol
		if len(o.Points) == 1 {
			return math.Hypot(p.X-o.Points[0].X, p.Y-o.Points[0].Y) <= r
		}
		for i := 1; i < len(o.Points); i++ {
			if segDist(p, o.Points[i-1], o.Points[i]) <= r {
				return true
			}
		}
	case Rectangle, Oval:
		if len(o.Points) < 2 {
			return false
		}
		pts := o.outline()
		for i := range pts {
			if segDist(p, pts[i], pts[(i+1)%len(pts)]) <= o.Width/2+tol {
				return true
			}
		}
	}
	return false
}

func segDist(p, a, b render.Pt) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	l := dx*dx + dy*dy
	t := 0.0
	if l > 0 {
		t = math.Max(0, math.Min(1, ((p.X-a.X)*dx+(p.Y-a.Y)*dy)/l))
	}
	return math.Hypot(p.X-(a.X+t*dx), p.Y-(a.Y+t*dy))
}

// outline is the closed outline of a rectangle or oval.
func (o Object) outline() []render.Pt {
	a, b := o.Points[0], o.Points[len(o.Points)-1]
	x0, y0, x1, y1 := math.Min(a.X, b.X), math.Min(a.Y, b.Y), math.Max(a.X, b.X), math.Max(a.Y, b.Y)
	if o.Kind == Rectangle {
		return []render.Pt{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
	}
	cx, cy, rx, ry := (x0+x1)/2, (y0+y1)/2, math.Max((x1-x0)/2, 0.1), math.Max((y1-y0)/2, 0.1)
	n := int(math.Max(24, math.Min(256, (rx+ry)/2)))
	pts := make([]render.Pt, n)
	for i := range pts {
		t := float64(i) / float64(n) * 2 * math.Pi
		pts[i] = render.Pt{X: cx + rx*math.Cos(t), Y: cy + ry*math.Sin(t)}
	}
	return pts
}

// Smooth turns raw pointer samples into a smooth curve: quadratic segments
// through the midpoints, as the editor draws strokes.
func Smooth(p []render.Pt) []render.Pt {
	if len(p) < 3 {
		return p
	}
	out := []render.Pt{p[0]}
	from := p[0]
	for i := 1; i < len(p)-1; i++ {
		ctl := p[i]
		to := render.Pt{X: (p[i].X + p[i+1].X) / 2, Y: (p[i].Y + p[i+1].Y) / 2}
		n := int(math.Max(2, math.Min(12, (math.Hypot(ctl.X-from.X, ctl.Y-from.Y)+math.Hypot(to.X-ctl.X, to.Y-ctl.Y))/3)))
		for k := 1; k <= n; k++ {
			t := float64(k) / float64(n)
			u := 1 - t
			out = append(out, render.Pt{X: u*u*from.X + 2*u*t*ctl.X + t*t*to.X, Y: u*u*from.Y + 2*u*t*ctl.Y + t*t*to.Y})
		}
		from = to
	}
	return append(out, p[len(p)-1])
}

// mask renders the object's coverage over area. Strokes are measured by
// distance to the line, so each stroke has one smooth outline (overlapping
// pieces would darken the anti-aliased edge where they meet).
func (o Object) mask(area image.Rectangle) *image.Alpha {
	m := image.NewAlpha(area)
	switch o.Kind {
	case Pen, Highlighter:
		strokeMask(m, Smooth(o.Points), o.Width, false)
	case Line:
		strokeMask(m, []render.Pt{o.Points[0], o.Points[len(o.Points)-1]}, o.Width, false)
	case Arrow:
		a, b := o.Points[0], o.Points[len(o.Points)-1]
		ang := math.Atan2(b.Y-a.Y, b.X-a.X)
		head := o.arrowHead()
		end := render.Pt{X: b.X - math.Cos(ang)*head*0.55, Y: b.Y - math.Sin(ang)*head*0.55}
		strokeMask(m, []render.Pt{a, end}, o.Width, false)
		var p render.Path
		p.Poly([]render.Pt{b,
			{X: b.X - head*math.Cos(ang-0.42), Y: b.Y - head*math.Sin(ang-0.42)},
			{X: b.X - head*math.Cos(ang+0.42), Y: b.Y - head*math.Sin(ang+0.42)}})
		hm := p.Mask(area)
		for i, v := range hm.Pix {
			if v > m.Pix[i] {
				m.Pix[i] = v
			}
		}
	case Rectangle, Oval:
		strokeMask(m, o.outline(), o.Width, true)
	}
	return m
}

// strokeMask adds a round-capped, round-joined polyline of width w to m.
func strokeMask(m *image.Alpha, pts []render.Pt, w float64, closed bool) {
	if len(pts) == 0 {
		return
	}
	r := math.Max(w/2, 0.5)
	n := len(pts) - 1
	if closed {
		n = len(pts)
	}
	if n == 0 {
		n = 1 // a dot
	}
	for i := 0; i < n; i++ {
		a, b := pts[i], pts[(i+1)%len(pts)]
		if len(pts) == 1 {
			b = a
		}
		box := image.Rect(int(math.Floor(math.Min(a.X, b.X)-r-1)), int(math.Floor(math.Min(a.Y, b.Y)-r-1)),
			int(math.Ceil(math.Max(a.X, b.X)+r+1)), int(math.Ceil(math.Max(a.Y, b.Y)+r+1))).Intersect(m.Rect)
		for y := box.Min.Y; y < box.Max.Y; y++ {
			o := m.PixOffset(box.Min.X, y)
			for x := box.Min.X; x < box.Max.X; x, o = x+1, o+1 {
				cov := r + 0.5 - segDist(render.Pt{X: float64(x) + 0.5, Y: float64(y) + 0.5}, a, b)
				if cov <= 0 {
					continue
				}
				v := uint8(255)
				if cov < 1 {
					v = uint8(cov * 255)
				}
				if v > m.Pix[o] {
					m.Pix[o] = v
				}
			}
		}
	}
}

// Draw paints o onto dst. dst may be any part of the screen (its bounds are
// in screen coordinates); only that part is rendered.
func Draw(dst *image.RGBA, o Object) {
	area := o.Bounds().Intersect(dst.Bounds())
	if area.Empty() {
		return
	}
	switch o.Kind {
	case Emoji:
		img := emoji.Scaled(o.Emoji, o.Box.Dx())
		if img != nil {
			draw.Draw(dst, area, img, area.Min.Sub(o.Box.Min), draw.Over)
		}
	case Highlighter:
		multiply(dst, o.mask(area), o.Color)
	default:
		draw.DrawMask(dst, area, image.NewUniform(o.Color), image.Point{}, o.mask(area), area.Min, draw.Over)
	}
}

// multiply tints dst under mask with c, like a highlighter (multiply blend
// at HighlightAlpha), so text under it stays readable.
func multiply(dst *image.RGBA, m *image.Alpha, c color.RGBA) {
	b := m.Rect
	for y := b.Min.Y; y < b.Max.Y; y++ {
		mi := m.PixOffset(b.Min.X, y)
		di := dst.PixOffset(b.Min.X, y)
		for x := b.Min.X; x < b.Max.X; x, mi, di = x+1, mi+1, di+4 {
			a := m.Pix[mi]
			if a == 0 {
				continue
			}
			k := uint32(float64(a) * HighlightAlpha)
			for ch, cv := range [3]uint8{c.R, c.G, c.B} {
				d := uint32(dst.Pix[di+ch])
				mul := d * uint32(cv) / 255
				dst.Pix[di+ch] = uint8((d*(255-k) + mul*k) / 255)
			}
		}
	}
}

// DrawAll paints every object onto dst in order.
func DrawAll(dst *image.RGBA, objs []Object) {
	for _, o := range objs {
		Draw(dst, o)
	}
}

// Doc is the list of objects with undo history.
type Doc struct {
	Objects []Object
	hist    [][]Object
	pos     int
}

func (d *Doc) commit(next []Object) {
	if d.hist == nil {
		d.hist = [][]Object{nil}
	}
	d.hist = append(d.hist[:d.pos+1], next)
	d.pos = len(d.hist) - 1
	d.Objects = next
}

func (d *Doc) clone() []Object { return append([]Object(nil), d.Objects...) }

// Add appends an object.
func (d *Doc) Add(o Object) { d.commit(append(d.clone(), o)) }

// Erase removes the objects touching p and reports what changed (the union
// of their bounds).
func (d *Doc) Erase(p render.Pt, tol float64) image.Rectangle {
	var keep []Object
	var gone image.Rectangle
	for _, o := range d.Objects {
		if o.Hit(p, tol) {
			gone = gone.Union(o.Bounds())
		} else {
			keep = append(keep, o)
		}
	}
	if !gone.Empty() {
		d.commit(keep)
	}
	return gone
}

// Clear removes everything ("Erase all ink").
func (d *Doc) Clear() image.Rectangle {
	if len(d.Objects) == 0 {
		return image.Rectangle{}
	}
	r := d.bounds(d.Objects)
	d.commit(nil)
	return r
}

func (d *Doc) bounds(objs []Object) image.Rectangle {
	var r image.Rectangle
	for _, o := range objs {
		r = r.Union(o.Bounds())
	}
	return r
}

// Undo steps back; it returns the area that changed.
func (d *Doc) Undo() image.Rectangle {
	if d.pos == 0 {
		return image.Rectangle{}
	}
	before := d.Objects
	d.pos--
	d.Objects = d.hist[d.pos]
	return d.bounds(before).Union(d.bounds(d.Objects))
}

// Redo steps forward; it returns the area that changed.
func (d *Doc) Redo() image.Rectangle {
	if d.pos >= len(d.hist)-1 {
		return image.Rectangle{}
	}
	before := d.Objects
	d.pos++
	d.Objects = d.hist[d.pos]
	return d.bounds(before).Union(d.bounds(d.Objects))
}
