// SPDX-License-Identifier: Apache-2.0

package overlay

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"testing"

	"snippingtoolenhanced/internal/render"
)

var mon = image.Rect(0, 0, 1920, 1080)

func TestMarkupBarPlacement(t *testing.T) {
	// Under a small selection.
	b := LayoutMarkupBar(image.Rect(800, 300, 1100, 500), mon, 1)
	if !b.below || b.Surface.Min.Y <= 500 || !b.Surface.In(mon) {
		t.Fatalf("below: %v", b.Surface)
	}
	// Selection at the bottom: the bar goes above it.
	b = LayoutMarkupBar(image.Rect(800, 900, 1100, 1075), mon, 1)
	if b.below || b.Surface.Max.Y > 900 {
		t.Fatalf("above: %v", b.Surface)
	}
	// Full screen: inside the bottom edge.
	b = LayoutMarkupBar(mon, mon, 1.5)
	if !b.Surface.In(mon) {
		t.Fatalf("full screen: %v", b.Surface)
	}
	// Near the right edge, still on screen.
	b = LayoutMarkupBar(image.Rect(1880, 100, 1920, 140), mon, 2)
	if !b.Surface.In(mon) {
		t.Fatalf("right edge: %v", b.Surface)
	}
	if b.Hit(b.item(ActShare).Min.Add(image.Pt(2, 2))) != ActShare {
		t.Fatal("hit share")
	}
}

func TestFlyouts(t *testing.T) {
	b := LayoutMarkupBar(image.Rect(600, 200, 1300, 600), mon, 1)
	for _, tool := range []string{ToolPen, ToolHighlight, ToolEraser, ToolShapes, ToolEmoji} {
		fl := b.LayoutFlyout(tool)
		if fl.Rect.Empty() || !fl.Rect.In(mon) {
			t.Fatalf("%s flyout at %v", tool, fl.Rect)
		}
		for _, it := range fl.Items {
			if !it.Rect.In(fl.Rect) {
				t.Fatalf("%s item %s outside the flyout", tool, it.ID)
			}
			if got := fl.Hit(it.Rect.Min.Add(image.Pt(3, 3))); got != it.ID {
				t.Fatalf("%s: hit %s = %s", tool, it.ID, got)
			}
		}
	}
	fl := b.LayoutFlyout(ToolPen)
	if fl.SliderValue(fl.Slider.Min.X-50) != 1 || fl.SliderValue(fl.Slider.Max.X+50) != 24 {
		t.Fatal("slider range")
	}
	if b.LayoutFlyout(ActCopy).Tool != "" {
		t.Fatal("actions have no flyout")
	}
}

// TestMarkupPreview writes the toolbar and flyouts to $MARKUP_PREVIEW for a visual check.
func TestMarkupPreview(t *testing.T) {
	out := os.Getenv("MARKUP_PREVIEW")
	if out == "" {
		t.Skip()
	}
	for _, dark := range []bool{false, true} {
		th := render.Light
		if dark {
			th = render.Dark
		}
		img := image.NewRGBA(image.Rect(0, 0, 1500, 820))
		draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{90, 110, 130, 255}), image.Point{}, draw.Src)
		ink := DefaultInk()
		ink.Emoji = 3
		paste := func(src *image.RGBA, dx, dy int) {
			draw.Draw(img, src.Bounds().Add(image.Pt(dx, dy)), src, src.Bounds().Min, draw.Over)
		}
		b := LayoutMarkupBar(image.Rect(300, 0, 900, 40), image.Rect(0, 0, 1500, 820), 1)
		paste(b.Render(th, MarkupState{Ink: ink, Hover: ActShare}), 0, 0)
		x := 0
		for _, tool := range []string{ToolPen, ToolHighlight, ToolShapes, ToolEraser, ToolEmoji} {
			fl := b.LayoutFlyout(tool)
			im := fl.Render(th, ink, "")
			paste(im, x-fl.Area().Min.X+20, 130-fl.Area().Min.Y+20)
			x += fl.Area().Dx()
			if x > 1100 {
				x = 0
			}
		}
		name := out + "-light.png"
		if dark {
			name = out + "-dark.png"
		}
		fh, _ := os.Create(name)
		png.Encode(fh, img)
		fh.Close()
	}
}
