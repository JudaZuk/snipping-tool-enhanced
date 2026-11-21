// SPDX-License-Identifier: Apache-2.0

package icon

import (
	"image"
	"image/draw"
	"image/png"
	"os"
	"testing"
)

func TestDraw(t *testing.T) {
	for _, s := range []int{16, 24, 32, 48, 256} {
		img := NRGBA(s)
		c := img.NRGBAAt(s/2, 2)
		if c.A != 255 || c.G < c.R {
			t.Fatalf("size %d: tile color %v", s, c)
		}
		if img.NRGBAAt(0, 0).A > 80 {
			t.Fatalf("size %d: corner should be transparent", s)
		}
		white := 0
		for i := 0; i < len(img.Pix); i += 4 {
			if img.Pix[i] > 230 && img.Pix[i+1] > 230 && img.Pix[i+3] == 255 {
				white++
			}
		}
		if white < s*s/30 {
			t.Fatalf("size %d: too little white (%d)", s, white)
		}
	}
}

// ICON_PREVIEW=file writes the icon at several sizes.
func TestPreview(t *testing.T) {
	out := os.Getenv("ICON_PREVIEW")
	if out == "" {
		t.Skip()
	}
	sizes := []int{256, 64, 48, 32, 24, 16}
	sheet := image.NewNRGBA(image.Rect(0, 0, 256+64+48+32+24+16+70, 256))
	x := 0
	for _, s := range sizes {
		draw.Draw(sheet, image.Rect(x, 0, x+s, s), NRGBA(s), image.Point{}, draw.Over)
		x += s + 10
	}
	f, _ := os.Create(out)
	png.Encode(f, sheet)
	f.Close()
}
