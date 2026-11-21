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

func TestLayoutAndHit(t *testing.T) {
	mon := image.Rect(-1920, 0, 0, 1080) // a monitor left of the primary one
	b := LayoutBar(mon, 1.5)
	if !b.Surface.In(mon) || len(b.Items) != 7 {
		t.Fatalf("%+v", b)
	}
	if c := (b.Surface.Min.X + b.Surface.Max.X) / 2; c < -961 || c > -959 {
		t.Fatalf("not centered: %d", c)
	}
	st := BarState{Kind: KindSnip, Mode: Rectangle}
	if b.Hit(b.item(ItemMode).Min.Add(image.Pt(3, 3)), st) != ItemMode || b.Hit(b.Surface.Min.Add(image.Pt(2, 2)), st) != "bar" || b.Hit(image.Pt(-100, 900), st) != "" {
		t.Fatal("hit test")
	}
	st.MenuOpen = true
	items := b.menuItems(KindSnip)
	if len(items) != 5 {
		t.Fatal(len(items))
	}
	if m, ok := MenuMode(b.Hit(items[1].Rect.Min.Add(image.Pt(5, 5)), st)); !ok || m != Square {
		t.Fatal("square must be second", m)
	}
	if m, ok := MenuMode(b.Hit(items[4].Rect.Min.Add(image.Pt(5, 5)), st)); !ok || m != Freeform {
		t.Fatal("menu hit", m)
	}
	if len(b.menuItems(KindRecord)) != 2 || Enabled(ItemColor, KindRecord) || !Enabled(ItemText, KindSnip) || Enabled(ItemText, KindRecord) || !Enabled(ItemMarkup, KindSnip) || Enabled(ItemMarkup, KindRecord) {
		t.Fatal("record mode rules")
	}
	if !b.menuRect(KindSnip).In(b.Area) {
		t.Fatal("menu outside repaint area")
	}
}

func TestFormatColor(t *testing.T) {
	c := color.RGBA{0x33, 0x99, 0xFF, 255}
	if FormatColor(c, 0) != "#3399FF" || FormatColor(c, 1) != "rgb(51, 153, 255)" || FormatColor(c, 2) != "hsl(210, 100%, 60%)" {
		t.Fatal(FormatColor(c, 0), FormatColor(c, 1), FormatColor(c, 2))
	}
	if FormatColor(color.RGBA{128, 128, 128, 255}, 2) != "hsl(0, 0%, 50%)" {
		t.Fatal("grey")
	}
}

func TestPolyBoundsIncludesFirstPoint(t *testing.T) {
	if r := polyBounds([]image.Point{{2, 3}, {10, 10}, {12, 8}}); r != image.Rect(2, 3, 13, 11) {
		t.Fatal(r)
	}
}

func TestSegmentRect(t *testing.T) {
	// The bug in 1.2.1: two points gave an empty repaint area.
	r := segmentRect(image.Pt(10, 20), image.Pt(13, 20))
	if r.Empty() || !image.Pt(10, 20).In(r) || !image.Pt(13, 20).In(r) {
		t.Fatal(r)
	}
	if r := segmentRect(image.Pt(5, 5), image.Pt(5, 5)); r != image.Rect(5, 5, 6, 6) {
		t.Fatal(r)
	}
	if r := segmentRect(image.Pt(30, 40), image.Pt(10, 2)); r != image.Rect(10, 2, 31, 41) {
		t.Fatal(r)
	}
}

func TestDragRect(t *testing.T) {
	if r := dragRect(image.Pt(10, 10), image.Pt(15, 40), true); r != image.Rect(10, 10, 40, 40) {
		t.Fatal(r)
	}
	if r := dragRect(image.Pt(10, 10), image.Pt(4, 30), false); r != image.Rect(4, 10, 10, 30) {
		t.Fatal(r)
	}
}

// OVERLAY_PREVIEW=dir writes mock overlays (bar, menu, loupe) for visual review.
func TestPreview(t *testing.T) {
	out := os.Getenv("OVERLAY_PREVIEW")
	if out == "" {
		t.Skip()
	}
	shots := []struct {
		name  string
		dark  bool
		scale float64
		st    BarState
		loupe bool
	}{
		{"snip", false, 1, BarState{Kind: KindSnip, Mode: Rectangle, Hover: ItemColor}, true},
		{"menu", false, 1, BarState{Kind: KindSnip, Mode: Rectangle, MenuOpen: true, Hover: "menu:window"}, false},
		{"record", false, 1.5, BarState{Kind: KindRecord, Mode: Rectangle}, false},
		{"dark", true, 1, BarState{Kind: KindSnip, Mode: Freeform, Hover: ItemRecord}, false},
	}
	for _, sh := range shots {
		th := render.Light
		if sh.dark {
			th = render.Dark
		}
		frame := image.NewRGBA(image.Rect(0, 0, 1000, 420))
		for y := 0; y < 420; y++ {
			for x := 0; x < 1000; x++ {
				frame.SetRGBA(x, y, color.RGBA{uint8(40 + x/5), uint8(120 + y/6), 210, 255})
			}
		}
		dim := image.NewRGBA(frame.Rect)
		draw.Draw(dim, dim.Rect, frame, image.Point{}, draw.Src)
		for i := 0; i < len(dim.Pix); i += 4 {
			dim.Pix[i], dim.Pix[i+1], dim.Pix[i+2] = uint8(int(dim.Pix[i])*55/100), uint8(int(dim.Pix[i+1])*55/100), uint8(int(dim.Pix[i+2])*55/100)
		}
		b := LayoutBar(frame.Rect, sh.scale)
		bar := b.Render(dim, th, sh.st)
		draw.Draw(dim, b.Area, bar, b.Area.Min, draw.Src)
		if sh.loupe {
			l := Loupe{Sample: 11}
			img, r, _ := l.Render(frame, dim, image.Pt(700, 260), 1, th)
			draw.Draw(dim, r, img, r.Min, draw.Src)
		}
		f, _ := os.Create(out + "/bar-" + sh.name + ".png")
		png.Encode(f, dim)
		f.Close()
	}
}
