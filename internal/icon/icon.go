// SPDX-License-Identifier: Apache-2.0

// Package icon draws the app icon at runtime and for the .exe resources: a
// teal rounded tile with a dashed selection and a mouse pointer.
package icon

import (
	"image"
	"image/color"
	"math"

	"snippingtoolenhanced/internal/render"
)

var (
	teal  = color.RGBA{0x0F, 0x76, 0x6E, 0xFF}
	white = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
)

// Draw renders the icon at size×size pixels. The design is on an 88-unit
// grid; small sizes use thicker relative strokes and fewer dashes so the
// shape stays readable at 16 px.
func Draw(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	s := float64(size)
	k := s / 88
	render.FillRoundRect(img, 0, 0, s, s, 20*k, teal)

	// Dashed selection rectangle.
	w := math.Max(1.5, 5*k)
	var p render.Path
	rect := render.RoundRect(17*k, 17*k, 44*k, 40*k, 6*k)
	rect = append(rect, rect[0])
	dash, gap := 9*k, 6*k
	if size <= 24 { // at tiny sizes a dashed line turns to mush: use 4 dashes per side
		dash, gap = 4.5*k*2, 5*k
		w = math.Max(1.25, 6*k)
	}
	dashed(&p, rect, w, dash, gap)
	p.Fill(img, white)

	// Mouse pointer with a teal outline so it separates from the dashes.
	ptr := []render.Pt{{X: 48, Y: 44}, {X: 48, Y: 74}, {X: 56, Y: 66}, {X: 62, Y: 79}, {X: 69, Y: 76}, {X: 63, Y: 63}, {X: 74, Y: 63}}
	for i := range ptr {
		ptr[i].X *= k
		ptr[i].Y *= k
	}
	var outline render.Path
	closed := append(append([]render.Pt{}, ptr...), ptr[0])
	outline.Stroke(closed, math.Max(1, 6*k), false)
	outline.Fill(img, teal)
	var fill render.Path
	fill.Poly(ptr)
	fill.Fill(img, white)
	return img
}

// dashed adds dash segments along a polyline.
func dashed(p *render.Path, pts []render.Pt, width, dash, gap float64) {
	on, left := true, dash
	var cur []render.Pt
	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1], pts[i]
		segLen := math.Hypot(b.X-a.X, b.Y-a.Y)
		pos := 0.0
		for segLen-pos > 1e-9 {
			step := math.Min(left, segLen-pos)
			t0, t1 := pos/segLen, (pos+step)/segLen
			q0 := render.Pt{X: a.X + (b.X-a.X)*t0, Y: a.Y + (b.Y-a.Y)*t0}
			q1 := render.Pt{X: a.X + (b.X-a.X)*t1, Y: a.Y + (b.Y-a.Y)*t1}
			if on {
				if len(cur) == 0 {
					cur = append(cur, q0)
				}
				cur = append(cur, q1)
			}
			pos += step
			left -= step
			if left <= 1e-9 {
				if on && len(cur) > 1 {
					p.Stroke(cur, width, false)
				}
				cur = nil
				on = !on
				if on {
					left = dash
				} else {
					left = gap
				}
			}
		}
	}
	if on && len(cur) > 1 {
		p.Stroke(cur, width, false)
	}
}

// BGRA returns the icon as top-down BGRA with straight alpha.
func BGRA(size int) []byte {
	img := Draw(size)
	out := make([]byte, len(img.Pix))
	for i := 0; i < len(out); i += 4 {
		a := img.Pix[i+3]
		un := func(v uint8) uint8 {
			if a == 0 {
				return 0
			}
			return uint8(uint32(v) * 255 / uint32(a))
		}
		out[i], out[i+1], out[i+2], out[i+3] = un(img.Pix[i+2]), un(img.Pix[i+1]), un(img.Pix[i]), a
	}
	return out
}

// NRGBA returns the icon as a straight-alpha image (for .ico/.png files).
func NRGBA(size int) *image.NRGBA {
	b := BGRA(size)
	n := image.NewNRGBA(image.Rect(0, 0, size, size))
	for i := 0; i < len(b); i += 4 {
		n.Pix[i], n.Pix[i+1], n.Pix[i+2], n.Pix[i+3] = b[i+2], b[i+1], b[i], b[i+3]
	}
	return n
}
