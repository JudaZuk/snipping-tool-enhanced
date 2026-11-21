// SPDX-License-Identifier: Apache-2.0

package render

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func TestFillAndText(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 60))
	FillRoundRect(img, 10, 10, 180, 40, 8, Light.Surface)
	if c := img.RGBAAt(100, 30); c != Light.Surface {
		t.Fatalf("center %v", c)
	}
	if c := img.RGBAAt(10, 10); c.A > 40 {
		t.Fatalf("corner should be (mostly) transparent: %v", c)
	}
	Text(img, "Rectangle", 20, 30, 14, Light.Text)
	dark := 0
	for x := 20; x < 100; x++ {
		for y := 20; y < 40; y++ {
			if img.RGBAAt(x, y).R < 128 {
				dark++
			}
		}
	}
	if dark < 20 {
		t.Fatal("text not drawn")
	}
	if MeasureText("abc", 14) <= 0 {
		t.Fatal("measure")
	}
}

func TestIconsDraw(t *testing.T) {
	for _, n := range []string{IconCamera, IconVideo, IconRectangle, IconSquare, IconFreeform, IconWindow, IconFullscreen, IconMarkup, IconColor, IconText, IconClose, IconChevron, IconCheck, IconRecord, IconStop, IconPause, IconPlay, IconDelete, IconMic, IconMicOff, IconSpeaker, IconSpeakerOff} {
		img := image.NewRGBA(image.Rect(0, 0, 40, 40))
		DrawIcon(img, n, 0, 0, 40, color.RGBA{0, 0, 0, 255})
		ink := 0
		for i := 3; i < len(img.Pix); i += 4 {
			if img.Pix[i] > 0 {
				ink++
			}
		}
		if ink < 30 {
			t.Errorf("%s: only %d pixels", n, ink)
		}
	}
}

// Writes a preview sheet when RENDER_PREVIEW is set (used during design review).
func TestPreview(t *testing.T) {
	out := os.Getenv("RENDER_PREVIEW")
	if out == "" {
		t.Skip()
	}
	names := []string{IconCamera, IconVideo, IconRectangle, IconSquare, IconFreeform, IconWindow, IconFullscreen, IconMarkup, IconColor, IconText, IconClose,
		IconChevron, IconCheck, IconRecord, IconStop, IconPause, IconPlay, IconDelete, IconMic, IconMicOff, IconSpeaker, IconSpeakerOff}
	img := image.NewRGBA(image.Rect(0, 0, 40*len(names)+20, 130))
	FillRoundRect(img, 0, 0, float64(img.Rect.Dx()), 130, 0, Light.Surface)
	for i, n := range names {
		DrawIcon(img, n, float64(10+i*40), 10, 20, Light.Text)
		DrawIcon(img, n, float64(10+i*40), 50, 30, Light.Text)
		DrawIcon(img, n, float64(10+i*40), 90, 20*1.25, Light.Text)
	}
	f, _ := os.Create(out)
	defer f.Close()
	png.Encode(f, img)
}
