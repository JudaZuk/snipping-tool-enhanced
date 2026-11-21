// SPDX-License-Identifier: Apache-2.0

package render

import (
	"image"
	"image/color"
	"math"
)

// Icon names.
const (
	IconCamera     = "camera"
	IconVideo      = "video"
	IconRectangle  = "rectangle"
	IconSquare     = "square"
	IconFreeform   = "freeform"
	IconWindow     = "window"
	IconFullscreen = "fullscreen"
	IconMarkup     = "markup"
	IconColor      = "colorpicker"
	IconText       = "textextract"
	IconClose      = "close"
	IconChevron    = "chevron"
	IconCheck      = "check"
	IconRecord     = "record"
	IconStop       = "stop"
	IconPause      = "pause"
	IconPlay       = "play"
	IconDelete     = "delete"
	IconMic        = "mic"
	IconMicOff     = "micoff"
	IconSpeaker    = "speaker"
	IconSpeakerOff = "speakeroff"
)

// pen draws pixel-snapped strokes: design coordinates are on a 20×20 grid and
// whole-number coordinates land on pixel centers, so straight lines are
// exactly one device pixel (or a whole number of pixels) wide and sharp.
type pen struct {
	x, y, k, w float64
	p          Path
}

func newPen(x, y, size float64) *pen {
	k := size / 20
	w := math.Max(1, math.Round(k))
	return &pen{x: math.Round(x), y: math.Round(y), k: k, w: w}
}

// pt maps a grid coordinate to device space, snapped to pixel centers.
func (q *pen) pt(gx, gy float64) Pt {
	off := 0.0
	if int(q.w)%2 == 1 {
		off = 0.5
	}
	return Pt{q.x + math.Round(gx*q.k) + off, q.y + math.Round(gy*q.k) + off}
}

func (q *pen) line(pts ...float64) {
	var ps []Pt
	for i := 0; i+1 < len(pts); i += 2 {
		ps = append(ps, q.pt(pts[i], pts[i+1]))
	}
	q.p.Stroke(ps, q.w, false)
}

// rrect strokes a rounded rectangle from (x0,y0) to (x1,y1) in grid units.
func (q *pen) rrect(x0, y0, x1, y1, r float64) {
	a, b := q.pt(x0, y0), q.pt(x1, y1)
	pts := RoundRect(a.X, a.Y, b.X-a.X, b.Y-a.Y, q.pt(x0+r, y0).X-a.X)
	pts = append(pts, pts[0])
	q.p.Stroke(pts, q.w, false)
}

// arc strokes a circular arc (degrees, 0 = east, clockwise on screen).
func (q *pen) arc(cx, cy, r, from, to float64) {
	c := q.pt(cx, cy)
	// Radius measured in snapped space, so arc ends meet snapped lines exactly.
	rx, ry := q.pt(cx+r, cy).X-c.X, q.pt(cx, cy+r).Y-c.Y
	if to-from >= 360 { // full circles stay round
		rx, ry = r*q.k, r*q.k
	}
	n := int(math.Max(12, rx*1.5))
	var pts []Pt
	for i := 0; i <= n; i++ {
		a := (from + (to-from)*float64(i)/float64(n)) * math.Pi / 180
		pts = append(pts, Pt{c.X + rx*math.Cos(a), c.Y + ry*math.Sin(a)})
	}
	q.p.Stroke(pts, q.w, false)
}

// rrectOpen strokes a rounded rectangle with the corner region beyond
// (gx, gy) (bottom-right) left open, as in the snip and markup icons.
func (q *pen) rrectOpen(x0, y0, x1, y1, r, gx, gy float64) {
	a, b := q.pt(x0, y0), q.pt(x1, y1)
	g := q.pt(gx, gy)
	pts := RoundRect(a.X, a.Y, b.X-a.X, b.Y-a.Y, q.pt(x0+r, y0).X-a.X)
	// RoundRect starts at the top-right corner and runs clockwise; rotate so the
	// polyline starts right after the gap and ends right before it.
	in := func(p Pt) bool { return p.X > g.X+0.01 && p.Y > g.Y+0.01 }
	start := -1
	for i := range pts {
		if in(pts[i]) && !in(pts[(i+1)%len(pts)]) {
			start = (i + 1) % len(pts)
		}
	}
	if start < 0 {
		q.rrect(x0, y0, x1, y1, r)
		return
	}
	var out []Pt
	for i := 0; i < len(pts); i++ {
		p := pts[(start+i)%len(pts)]
		if in(p) {
			break
		}
		out = append(out, p)
	}
	q.p.Stroke(out, q.w, false)
}

func (q *pen) fillPoly(pts ...float64) {
	var ps []Pt
	for i := 0; i+1 < len(pts); i += 2 {
		ps = append(ps, q.pt(pts[i], pts[i+1]))
	}
	q.p.Poly(ps)
}

func (q *pen) fillCircle(cx, cy, r float64) {
	c := q.pt(cx, cy)
	q.p.Poly(Circle(c.X, c.Y, r*q.k))
}

// DrawIcon draws a Fluent-style icon of the given size with its top-left at (x, y).
func DrawIcon(dst *image.RGBA, name string, x, y, size float64, c color.RGBA) {
	q := newPen(x, y, size)
	switch name {
	case IconCamera:
		q.rrect(2, 6, 17, 16, 2)
		q.line(6, 6, 7, 4, 12, 4, 13, 6)
		q.arc(9.5, 11, 3, 0, 360)
	case IconVideo:
		q.rrect(2, 5, 13, 15, 2)
		q.line(13, 9, 17, 6.5, 17, 13.5, 13, 11)
	case IconRectangle:
		q.rrectOpen(2, 5, 16, 15, 2, 11, 11)
		q.line(15, 12, 15, 18)
		q.line(12, 15, 18, 15)
	case IconSquare:
		q.rrect(4, 4, 16, 16, 2)
	case IconFreeform:
		var pts []float64
		for i := 0; i <= 40; i++ {
			t := float64(i) / 40 * 2 * math.Pi
			r := 6.5 + 1.1*math.Sin(3*t+0.6)
			pts = append(pts, 10+r*math.Cos(t)*1.1, 10+r*math.Sin(t)*0.85)
		}
		var ps []Pt
		for i := 0; i+1 < len(pts); i += 2 {
			ps = append(ps, Pt{q.x + pts[i]*q.k, q.y + pts[i+1]*q.k})
		}
		q.p.Stroke(ps, q.w, false)
	case IconWindow:
		q.rrect(2, 4, 17, 16, 2)
		q.line(2, 7, 17, 7)
	case IconFullscreen:
		q.rrect(2, 3, 17, 13, 2)
		q.line(7, 16, 12, 16)
		q.line(9.5, 13, 9.5, 16)
	case IconMarkup:
		q.rrectOpen(2, 3, 16, 16, 2, 11, 9)
		q.line(2, 14, 7, 9, 10, 12)
		q.arc(12, 7, 1.2, 0, 360)
		q.fillPoly(12, 18.5, 11.5, 15.5, 17, 10, 19.5, 12.5, 14, 18)
	case IconColor:
		q.line(3, 17, 4, 16, 11, 9)
		q.line(4, 16, 4, 14.5, 10, 8.5)
		q.line(8.5, 6.5, 13.5, 11.5)
		q.fillPoly(11, 6.5, 14, 3.5, 15.5, 3.2, 16.8, 4.5, 16.5, 6, 13.5, 9)
	case IconText:
		q.line(2, 6, 2, 4, 3, 3, 6, 3)
		q.line(14, 3, 17, 3, 18, 4, 18, 6)
		q.line(18, 14, 18, 16, 17, 17, 14, 17)
		q.line(6, 17, 3, 17, 2, 16, 2, 14)
		q.line(6, 7, 14, 7)
		q.line(6, 10, 14, 10)
		q.line(6, 13, 11, 13)
	case IconClose:
		q.line(4, 4, 16, 16)
		q.line(16, 4, 4, 16)
	case IconChevron:
		q.line(6, 8, 10, 12, 14, 8)
	case IconCheck:
		q.line(4, 10, 8, 14, 16, 6)
	case IconRecord:
		q.fillCircle(10, 10, 6)
	case IconStop:
		q.fillPoly(5, 5, 15, 5, 15, 15, 5, 15)
	case IconPause:
		q.fillPoly(5, 4, 8, 4, 8, 16, 5, 16)
		q.fillPoly(12, 4, 15, 4, 15, 16, 12, 16)
	case IconPlay:
		q.fillPoly(6, 4, 16, 10, 6, 16)
	case IconDelete:
		q.line(3, 5, 17, 5)
		q.line(8, 5, 8, 3, 12, 3, 12, 5)
		q.line(5, 5, 6, 17, 14, 17, 15, 5)
		q.line(8.5, 8, 8.5, 14)
		q.line(11.5, 8, 11.5, 14)
	case IconMic, IconMicOff:
		q.rrect(7, 2, 13, 12, 3)
		q.arc(10, 9, 6, 0, 180)
		q.line(10, 15, 10, 18)
		q.line(7, 18, 13, 18)
		if name == IconMicOff {
			q.line(3, 3, 17, 17)
		}
	case IconSpeaker, IconSpeakerOff:
		q.line(2, 8, 2, 12, 5, 12, 10, 16, 10, 4, 5, 8, 2, 8)
		if name == IconSpeakerOff {
			q.line(13, 7, 18, 13)
			q.line(18, 7, 13, 13)
		} else {
			q.arc(10, 10, 4, -50, 50)
			q.arc(10, 10, 7, -50, 50)
		}
	}
	q.p.Fill(dst, c)
}
