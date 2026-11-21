// SPDX-License-Identifier: Apache-2.0

// Package imaging holds pixel operations shared by capture, overlay and output.
package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strconv"
	"strings"

	"golang.org/x/image/vector"
)

// Crop copies r (clipped to src bounds) into a new image whose origin is (0,0).
func Crop(src *image.RGBA, r image.Rectangle) *image.RGBA {
	r = r.Intersect(src.Bounds())
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Bounds(), src, r.Min, draw.Src)
	return dst
}

// AddBorder returns a new image with a solid frame of the given thickness
// drawn around img (the image grows by 2×thickness in each dimension).
func AddBorder(img *image.RGBA, c color.RGBA, thickness int) *image.RGBA {
	if thickness <= 0 {
		return img
	}
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()+2*thickness, b.Dy()+2*thickness))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)
	draw.Draw(dst, image.Rect(thickness, thickness, thickness+b.Dx(), thickness+b.Dy()), img, b.Min, draw.Src)
	return dst
}

// ParseHexColor parses "#RRGGBB" or "RRGGBB".
func ParseHexColor(s string) (color.RGBA, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return color.RGBA{}, fmt.Errorf("color %q: want #RRGGBB", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("color %q: %w", s, err)
	}
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, nil
}

// BGRAToRGBA converts a top-down BGRA buffer (as produced by GDI DIB sections)
// into an opaque RGBA image.
func BGRAToRGBA(bgra []byte, w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	p := img.Pix
	n := w * h * 4
	for i := 0; i < n; i += 4 {
		p[i], p[i+1], p[i+2], p[i+3] = bgra[i+2], bgra[i+1], bgra[i], 255
	}
	return img
}

// RGBAToBGRA converts an RGBA image into a top-down BGRA buffer. If dim is in
// (0,1) every channel is multiplied by it (used for the overlay backdrop).
func RGBAToBGRA(img *image.RGBA, dim float64) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]byte, w*h*4)
	f := uint32(256)
	if dim > 0 && dim < 1 {
		f = uint32(dim * 256)
	}
	for y := 0; y < h; y++ {
		row := img.Pix[(y)*img.Stride:]
		o := out[y*w*4:]
		for x := 0; x < w; x++ {
			i := x * 4
			o[i] = uint8(uint32(row[i+2]) * f >> 8)
			o[i+1] = uint8(uint32(row[i+1]) * f >> 8)
			o[i+2] = uint8(uint32(row[i]) * f >> 8)
			o[i+3] = 255
		}
	}
	return out
}

// Encode writes img as PNG (default), JPEG ("jpeg"/"jpg") or GIF.
func Encode(img image.Image, format string, jpegQuality int) ([]byte, error) {
	var buf bytes.Buffer
	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		if jpegQuality <= 0 || jpegQuality > 100 {
			jpegQuality = 90
		}
		// JPEG has no alpha: flatten onto white.
		b := img.Bounds()
		flat := image.NewRGBA(b)
		draw.Draw(flat, b, image.White, image.Point{}, draw.Src)
		draw.Draw(flat, b, img, b.Min, draw.Over)
		err := jpeg.Encode(&buf, flat, &jpeg.Options{Quality: jpegQuality})
		return buf.Bytes(), err
	case "gif":
		// 256-color palette with Floyd–Steinberg dithering; transparent areas
		// become white, as GIF has only 1-bit transparency.
		b := img.Bounds()
		flat := image.NewRGBA(b)
		draw.Draw(flat, b, image.White, image.Point{}, draw.Src)
		draw.Draw(flat, b, img, b.Min, draw.Over)
		err := gif.Encode(&buf, flat, &gif.Options{NumColors: 256, Drawer: draw.FloydSteinberg})
		return buf.Bytes(), err
	default:
		enc := png.Encoder{CompressionLevel: png.DefaultCompression}
		err := enc.Encode(&buf, img)
		return buf.Bytes(), err
	}
}

// ToRGBA returns img as *image.RGBA with origin (0,0), copying if needed.
func ToRGBA(img image.Image) *image.RGBA {
	if r, ok := img.(*image.RGBA); ok && r.Bounds().Min == (image.Point{}) {
		return r
	}
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}

// OpaqueToNRGBA converts an opaque RGBA image to NRGBA (identical bytes when
// every pixel is opaque).
func OpaqueToNRGBA(src *image.RGBA) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		copy(dst.Pix[y*dst.Stride:y*dst.Stride+b.Dx()*4], src.Pix[(y+b.Min.Y-src.Rect.Min.Y)*src.Stride+(b.Min.X-src.Rect.Min.X)*4:])
	}
	return dst
}

// Freeform crops src to the bounding box of poly; pixels outside the shape
// become transparent white (white where alpha is ignored, e.g. CF_DIB),
// with an anti-aliased edge (white paper §3: "area outside is transparent/white").
func Freeform(src *image.RGBA, poly []image.Point) *image.NRGBA {
	if len(poly) < 3 {
		return nil
	}
	bb := image.Rect(poly[0].X, poly[0].Y, poly[0].X+1, poly[0].Y+1)
	for _, p := range poly[1:] {
		bb = bb.Union(image.Rectangle{Min: p, Max: p.Add(image.Pt(1, 1))})
	}
	bb = bb.Intersect(src.Bounds())
	if bb.Dx() < 2 || bb.Dy() < 2 {
		return nil
	}
	mask := image.NewAlpha(image.Rect(0, 0, bb.Dx(), bb.Dy()))
	z := vector.NewRasterizer(bb.Dx(), bb.Dy())
	z.MoveTo(float32(poly[0].X-bb.Min.X), float32(poly[0].Y-bb.Min.Y))
	for _, p := range poly[1:] {
		z.LineTo(float32(p.X-bb.Min.X), float32(p.Y-bb.Min.Y))
	}
	z.ClosePath()
	z.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
	out := OpaqueToNRGBA(Crop(src, bb))
	for y := 0; y < bb.Dy(); y++ {
		for x := 0; x < bb.Dx(); x++ {
			a := mask.AlphaAt(x, y).A
			i := out.PixOffset(x, y)
			if a == 0 {
				out.Pix[i], out.Pix[i+1], out.Pix[i+2] = 255, 255, 255
			}
			out.Pix[i+3] = a
		}
	}
	return out
}

// BorderN adds a solid frame of thickness t around img.
func BorderN(img *image.NRGBA, c color.RGBA, t int) *image.NRGBA {
	if t <= 0 {
		return img
	}
	b := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx()+2*t, b.Dy()+2*t))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)
	draw.Draw(dst, image.Rect(t, t, t+b.Dx(), t+b.Dy()), img, b.Min, draw.Over)
	return dst
}
