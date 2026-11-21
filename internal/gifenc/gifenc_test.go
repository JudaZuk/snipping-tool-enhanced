// SPDX-License-Identifier: Apache-2.0

package gifenc

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"
)

func frame(shift int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 64, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 64; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8((x + shift) * 4), uint8(y * 6), 128, 255})
		}
	}
	return img
}

func TestEncode(t *testing.T) {
	for _, q := range []Quality{Low, High} {
		e := New(q)
		for i := 0; i < 5; i++ {
			e.Add(frame(i))
		}
		b, err := e.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		g, err := gif.DecodeAll(bytes.NewReader(b))
		if err != nil || len(g.Image) != 5 || g.Delay[0] != 100/q.FPS {
			t.Fatal(err, len(g.Image))
		}
		if len(g.Image[0].Palette) > q.Colors {
			t.Fatal("palette too large")
		}
	}
}

func TestMedianCutSmallImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(1, 0, color.RGBA{0, 0, 255, 255})
	if p := MedianCut(img, 256); len(p) != 2 {
		t.Fatal(len(p))
	}
}
