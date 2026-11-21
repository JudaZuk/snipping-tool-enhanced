// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func checker(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), uint8(x + y), 255})
		}
	}
	return img
}

func TestCrop(t *testing.T) {
	src := checker(50, 40)
	c := Crop(src, image.Rect(10, 5, 30, 25))
	if c.Bounds() != image.Rect(0, 0, 20, 20) || c.RGBAAt(0, 0) != src.RGBAAt(10, 5) || c.RGBAAt(19, 19) != src.RGBAAt(29, 24) {
		t.Fatal("crop mismatch")
	}
	if Crop(src, image.Rect(40, 30, 100, 100)).Bounds().Dx() != 10 {
		t.Fatal("crop must clip to source")
	}
}

func TestBorder(t *testing.T) {
	red := color.RGBA{255, 0, 0, 255}
	b := AddBorder(checker(4, 3), red, 2)
	if b.Bounds().Dx() != 8 || b.Bounds().Dy() != 7 || b.RGBAAt(0, 0) != red || b.RGBAAt(7, 6) != red || b.RGBAAt(2, 2) != (color.RGBA{0, 0, 0, 255}) {
		t.Fatal("border mismatch")
	}
}

func TestHex(t *testing.T) {
	c, err := ParseHexColor("#0A80fF")
	if err != nil || c != (color.RGBA{10, 128, 255, 255}) {
		t.Fatal(c, err)
	}
	if _, err := ParseHexColor("#12"); err == nil {
		t.Fatal("want error")
	}
}

func TestBGRARoundTrip(t *testing.T) {
	src := checker(7, 5)
	back := BGRAToRGBA(RGBAToBGRA(src, 0), 7, 5)
	if !bytes.Equal(back.Pix, src.Pix) {
		t.Fatal("round trip")
	}
	dim := RGBAToBGRA(src, 0.5)
	if dim[2] != src.Pix[0]/2 { // R channel of pixel 0
		t.Fatalf("dim %d vs %d", dim[2], src.Pix[0])
	}
}

func TestEncode(t *testing.T) {
	b, err := Encode(checker(8, 8), "png", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
	if b, err := Encode(checker(8, 8), "jpeg", 80); err != nil || len(b) < 10 || b[0] != 0xFF {
		t.Fatal("jpeg", err)
	}
}

func TestFreeformAndBorderN(t *testing.T) {
	src := checker(100, 100)
	tri := []image.Point{{10, 10}, {90, 10}, {10, 90}}
	f := Freeform(src, tri)
	if f.Bounds().Dx() < 79 || f.Bounds().Dy() < 79 {
		t.Fatal(f.Bounds())
	}
	if f.NRGBAAt(5, 5).A != 255 || f.NRGBAAt(75, 75).A != 0 || f.NRGBAAt(75, 75).R != 255 {
		t.Fatalf("mask: %v %v", f.NRGBAAt(5, 5), f.NRGBAAt(75, 75))
	}
	b := BorderN(f, color.RGBA{255, 0, 0, 255}, 2)
	if b.Bounds().Dx() != f.Bounds().Dx()+4 || b.NRGBAAt(0, 0).R != 255 {
		t.Fatal("border")
	}
	// The first point must be inside the result (it was dropped before 1.2.2).
	if g := Freeform(src, []image.Point{{5, 5}, {60, 20}, {30, 60}}); g.Bounds().Dx() != 56 || g.Bounds().Dy() != 56 {
		t.Fatal("bounds", g.Bounds())
	}
	if Freeform(src, tri[:2]) != nil {
		t.Fatal("need 3 points")
	}
	if g, err := Encode(f, "gif", 0); err != nil || string(g[:3]) != "GIF" {
		t.Fatal("gif", err)
	}
}
