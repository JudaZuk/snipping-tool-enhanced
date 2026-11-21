// SPDX-License-Identifier: Apache-2.0

// Package gifenc builds animated GIFs from video frames (white paper §4,
// "GIF export"), with a per-frame median-cut palette.
package gifenc

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"sort"
)

// Quality presets.
type Quality struct {
	FPS      int
	MaxWidth int
	Colors   int
	Dither   bool
}

var (
	Low  = Quality{FPS: 10, MaxWidth: 480, Colors: 128, Dither: false}
	High = Quality{FPS: 15, MaxWidth: 960, Colors: 256, Dither: true}
)

// MaxSeconds is how much of a recording is exported.
const MaxSeconds = 30

// Encoder collects frames.
type Encoder struct {
	q   Quality
	out gif.GIF
}

// New starts a GIF with quality q.
func New(q Quality) *Encoder { return &Encoder{q: q} }

// Add appends a frame shown for 1/FPS seconds.
func (e *Encoder) Add(img image.Image) {
	b := img.Bounds()
	pal := MedianCut(img, e.q.Colors)
	p := image.NewPaletted(image.Rect(0, 0, b.Dx(), b.Dy()), pal)
	var d draw.Drawer = draw.Src
	if e.q.Dither {
		d = draw.FloydSteinberg
	}
	d.Draw(p, p.Bounds(), img, b.Min)
	e.out.Image = append(e.out.Image, p)
	e.out.Delay = append(e.out.Delay, 100/e.q.FPS)
}

// Len is the number of frames.
func (e *Encoder) Len() int { return len(e.out.Image) }

// Bytes encodes the animation (looping forever).
func (e *Encoder) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	e.out.LoopCount = 0
	err := gif.EncodeAll(&buf, &e.out)
	return buf.Bytes(), err
}

type box struct{ px []color.RGBA }

func (b box) span() (int, int) {
	var lo, hi [3]uint8
	lo = [3]uint8{255, 255, 255}
	for _, p := range b.px {
		for i, v := range [3]uint8{p.R, p.G, p.B} {
			if v < lo[i] {
				lo[i] = v
			}
			if v > hi[i] {
				hi[i] = v
			}
		}
	}
	best, axis := -1, 0
	for i := 0; i < 3; i++ {
		if d := int(hi[i]) - int(lo[i]); d > best {
			best, axis = d, i
		}
	}
	return axis, best
}

// MedianCut returns a palette of at most n colors for img.
func MedianCut(img image.Image, n int) color.Palette {
	b := img.Bounds()
	step := 1
	if total := b.Dx() * b.Dy(); total > 40000 {
		step = total / 40000
	}
	var px []color.RGBA
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if i%step == 0 {
				r, g, bb, _ := img.At(x, y).RGBA()
				px = append(px, color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bb >> 8), 255})
			}
			i++
		}
	}
	if len(px) == 0 {
		return color.Palette{color.Black}
	}
	boxes := []box{{px}}
	for len(boxes) < n {
		bi, bestSpan := -1, 0
		for i, bx := range boxes {
			if len(bx.px) < 2 {
				continue
			}
			if _, s := bx.span(); s > bestSpan {
				bi, bestSpan = i, s
			}
		}
		if bi < 0 || bestSpan == 0 {
			break
		}
		bx := boxes[bi]
		axis, _ := bx.span()
		sort.Slice(bx.px, func(a, c int) bool { return ch(bx.px[a], axis) < ch(bx.px[c], axis) })
		mid := len(bx.px) / 2
		boxes[bi] = box{bx.px[:mid]}
		boxes = append(boxes, box{bx.px[mid:]})
	}
	pal := make(color.Palette, 0, len(boxes))
	for _, bx := range boxes {
		var r, g, bb int
		for _, p := range bx.px {
			r, g, bb = r+int(p.R), g+int(p.G), bb+int(p.B)
		}
		k := len(bx.px)
		pal = append(pal, color.RGBA{uint8(r / k), uint8(g / k), uint8(bb / k), 255})
	}
	return pal
}

func ch(c color.RGBA, axis int) uint8 {
	switch axis {
	case 0:
		return c.R
	case 1:
		return c.G
	}
	return c.B
}
