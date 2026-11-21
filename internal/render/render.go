// SPDX-License-Identifier: Apache-2.0

// Package render draws anti-aliased Windows 11-style UI pieces (rounded
// surfaces, outline icons, Segoe UI text) into RGBA images. The capture
// overlay and the capture notification use it, so they look like the rest of
// Windows 11 instead of classic GDI controls.
package render

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// Theme holds the Windows 11 colors for light or dark mode.
type Theme struct {
	Dark       bool
	Surface    color.RGBA // flyout / bar background
	Border     color.RGBA
	Text       color.RGBA
	TextMuted  color.RGBA
	Hover      color.RGBA // subtle fill
	Selected   color.RGBA
	Card       color.RGBA // raised control fill (selected segment)
	Accent     color.RGBA
	OnAccent   color.RGBA
	ShadowBase color.RGBA
}

// Light and Dark follow the WinUI 3 default palette.
var (
	Light = Theme{
		Surface: rgba(0xF9, 0xF9, 0xF9, 0xFF), Border: rgba(0xE5, 0xE5, 0xE5, 0xFF),
		Text: rgba(0x1A, 0x1A, 0x1A, 0xFF), TextMuted: rgba(0x5D, 0x5D, 0x5D, 0xFF),
		Hover: rgba(0, 0, 0, 0x0F), Selected: rgba(0, 0, 0, 0x0A), Card: rgba(0xFF, 0xFF, 0xFF, 0xFF),
		Accent: rgba(0x00, 0x5F, 0xB8, 0xFF), OnAccent: rgba(0xFF, 0xFF, 0xFF, 0xFF),
		ShadowBase: rgba(0, 0, 0, 0xFF),
	}
	Dark = Theme{
		Dark:    true,
		Surface: rgba(0x2C, 0x2C, 0x2C, 0xFF), Border: rgba(0x3D, 0x3D, 0x3D, 0xFF),
		Text: rgba(0xFF, 0xFF, 0xFF, 0xFF), TextMuted: rgba(0xC5, 0xC5, 0xC5, 0xFF),
		Hover: rgba(0xFF, 0xFF, 0xFF, 0x0F), Selected: rgba(0xFF, 0xFF, 0xFF, 0x0B), Card: rgba(0x3A, 0x3A, 0x3A, 0xFF),
		Accent: rgba(0x60, 0xCD, 0xFF, 0xFF), OnAccent: rgba(0x00, 0x00, 0x00, 0xFF),
		ShadowBase: rgba(0, 0, 0, 0xFF),
	}
)

func rgba(r, g, b, a uint8) color.RGBA { return color.RGBA{r, g, b, a} }

// Premul converts a straight-alpha color into the premultiplied form
// image/draw expects.
func Premul(c color.RGBA) color.RGBA {
	a := uint32(c.A)
	return color.RGBA{uint8(uint32(c.R) * a / 255), uint8(uint32(c.G) * a / 255), uint8(uint32(c.B) * a / 255), c.A}
}

// ---------- paths ----------

// Path accumulates closed polygons (float coordinates) for filling.
type Path struct{ polys [][]Pt }

// Pt is a float point.
type Pt struct{ X, Y float64 }

// Poly adds a polygon, normalized to counter-clockwise so overlapping
// parts union under the non-zero rule.
func (p *Path) Poly(pts []Pt) {
	if len(pts) < 3 {
		return
	}
	if area(pts) < 0 {
		rev := make([]Pt, len(pts))
		for i := range pts {
			rev[i] = pts[len(pts)-1-i]
		}
		pts = rev
	}
	p.polys = append(p.polys, pts)
}

// Hole adds a polygon with clockwise orientation (cuts out of CCW shapes).
func (p *Path) Hole(pts []Pt) {
	if len(pts) < 3 {
		return
	}
	if area(pts) > 0 {
		rev := make([]Pt, len(pts))
		for i := range pts {
			rev[i] = pts[len(pts)-1-i]
		}
		pts = rev
	}
	p.polys = append(p.polys, pts)
}

func area(pts []Pt) float64 {
	a := 0.0
	for i := range pts {
		j := (i + 1) % len(pts)
		a += pts[i].X*pts[j].Y - pts[j].X*pts[i].Y
	}
	return a / 2
}

// Fill rasterizes the path onto dst with color c (straight alpha), blending over.
func (p *Path) Fill(dst *image.RGBA, c color.RGBA) {
	b := dst.Bounds()
	z := vector.NewRasterizer(b.Dx(), b.Dy())
	z.DrawOp = draw.Over
	for _, poly := range p.polys {
		z.MoveTo(float32(poly[0].X-float64(b.Min.X)), float32(poly[0].Y-float64(b.Min.Y)))
		for _, q := range poly[1:] {
			z.LineTo(float32(q.X-float64(b.Min.X)), float32(q.Y-float64(b.Min.Y)))
		}
		z.ClosePath()
	}
	z.Draw(dst, b, image.NewUniform(Premul(c)), image.Point{})
}

// Mask rasterizes the path's coverage over the area b (absolute coordinates).
func (p *Path) Mask(b image.Rectangle) *image.Alpha {
	m := image.NewAlpha(b)
	if b.Empty() {
		return m
	}
	z := vector.NewRasterizer(b.Dx(), b.Dy())
	for _, poly := range p.polys {
		z.MoveTo(float32(poly[0].X-float64(b.Min.X)), float32(poly[0].Y-float64(b.Min.Y)))
		for _, q := range poly[1:] {
			z.LineTo(float32(q.X-float64(b.Min.X)), float32(q.Y-float64(b.Min.Y)))
		}
		z.ClosePath()
	}
	z.Draw(m, b, image.Opaque, image.Point{})
	return m
}

// RoundRect returns the outline of a rounded rectangle.
func RoundRect(x, y, w, h, r float64) []Pt {
	r = math.Min(r, math.Min(w, h)/2)
	var pts []Pt
	corner := func(cx, cy, start float64) {
		for i := 0; i <= 8; i++ {
			a := start + float64(i)/8*math.Pi/2
			pts = append(pts, Pt{cx + r*math.Cos(a), cy + r*math.Sin(a)})
		}
	}
	corner(x+w-r, y+r, -math.Pi/2)
	corner(x+w-r, y+h-r, 0)
	corner(x+r, y+h-r, math.Pi/2)
	corner(x+r, y+r, math.Pi)
	return pts
}

// Circle returns a circle outline.
func Circle(cx, cy, r float64) []Pt {
	n := int(math.Max(16, r*2))
	pts := make([]Pt, n)
	for i := range pts {
		a := float64(i) / float64(n) * 2 * math.Pi
		pts[i] = Pt{cx + r*math.Cos(a), cy + r*math.Sin(a)}
	}
	return pts
}

// Stroke adds a polyline of the given width with round joins and caps.
func (p *Path) Stroke(pts []Pt, width float64, closed bool) {
	hw := width / 2
	n := len(pts)
	segs := n - 1
	if closed {
		segs = n
	}
	for i := 0; i < segs; i++ {
		a, b := pts[i], pts[(i+1)%n]
		dx, dy := b.X-a.X, b.Y-a.Y
		l := math.Hypot(dx, dy)
		if l == 0 {
			continue
		}
		nx, ny := -dy/l*hw, dx/l*hw
		p.Poly([]Pt{{a.X + nx, a.Y + ny}, {b.X + nx, b.Y + ny}, {b.X - nx, b.Y - ny}, {a.X - nx, a.Y - ny}})
	}
	for _, q := range pts {
		p.Poly(Circle(q.X, q.Y, hw))
	}
}

// FillRoundRect fills a rounded rectangle.
func FillRoundRect(dst *image.RGBA, x, y, w, h, r float64, c color.RGBA) {
	var p Path
	p.Poly(RoundRect(x, y, w, h, r))
	p.Fill(dst, c)
}

// StrokeRoundRect draws a rounded-rectangle outline of width t inside the bounds.
func StrokeRoundRect(dst *image.RGBA, x, y, w, h, r, t float64, c color.RGBA) {
	var p Path
	p.Poly(RoundRect(x, y, w, h, r))
	p.Hole(RoundRect(x+t, y+t, w-2*t, h-2*t, math.Max(0, r-t)))
	p.Fill(dst, c)
}

// Shadow paints a soft drop shadow under a rounded rectangle.
func Shadow(dst *image.RGBA, x, y, w, h, r, blur float64, base color.RGBA, strength float64) {
	steps := int(math.Max(4, blur))
	for i := steps; i >= 1; i-- {
		f := float64(i) / float64(steps)
		grow := blur * f
		a := strength * (1 - f) * (1 - f) / float64(steps) * 6
		c := base
		c.A = uint8(math.Min(255, a*255))
		FillRoundRect(dst, x-grow, y-grow+blur*0.35, w+2*grow, h+2*grow, r+grow, c)
	}
}

// ---------- text ----------

// FontPaths lists the fonts to try, in order (Windows 11 system fonts).
var FontPaths = []string{
	`C:\Windows\Fonts\segoeui.ttf`,
	`C:\Windows\Fonts\SegUIVar.ttf`,
}

var (
	fontOnce sync.Once
	theFont  *opentype.Font
	faceMu   sync.Mutex
	faces    = map[float64]font.Face{}
)

func loadFont() *opentype.Font {
	fontOnce.Do(func() {
		for _, p := range FontPaths {
			if b, err := os.ReadFile(p); err == nil {
				if f, err := opentype.Parse(b); err == nil {
					theFont = f
					return
				}
			}
		}
		theFont, _ = opentype.Parse(goregular.TTF)
	})
	return theFont
}

// Face returns a cached font face at the given pixel size.
func Face(px float64) font.Face {
	faceMu.Lock()
	defer faceMu.Unlock()
	if f, ok := faces[px]; ok {
		return f
	}
	f, err := opentype.NewFace(loadFont(), &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil
	}
	faces[px] = f
	return f
}

// MeasureText returns the advance width of s.
func MeasureText(s string, px float64) float64 {
	f := Face(px)
	if f == nil {
		return 0
	}
	return float64(font.MeasureString(f, s)) / 64
}

// Text draws s with its baseline-centered box starting at (x, cy): cy is the
// vertical center of the text line.
func Text(dst *image.RGBA, s string, x, cy, px float64, c color.RGBA) {
	f := Face(px)
	if f == nil {
		return
	}
	m := f.Metrics()
	asc, desc := float64(m.Ascent)/64, float64(m.Descent)/64
	baseline := cy + (asc-desc)/2
	d := font.Drawer{Dst: dst, Src: image.NewUniform(Premul(c)), Face: f,
		Dot: fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(baseline * 64)}}
	d.DrawString(s)
}

// TextCentered draws s centered in the box (x, y, w, h).
func TextCentered(dst *image.RGBA, s string, x, y, w, h, px float64, c color.RGBA) {
	tw := MeasureText(s, px)
	Text(dst, s, x+(w-tw)/2, y+h/2, px, c)
}
