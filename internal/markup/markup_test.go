// SPDX-License-Identifier: Apache-2.0

package markup

import (
	"image"
	"image/color"
	"image/draw"
	"testing"

	"snippingtoolenhanced/internal/render"
)

func canvas(r image.Rectangle, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(r)
	draw.Draw(img, r, image.NewUniform(c), image.Point{}, draw.Src)
	return img
}

var white = color.RGBA{255, 255, 255, 255}
var red = color.RGBA{230, 27, 27, 255}

func TestPenDrawsOnSubImage(t *testing.T) {
	// The canvas covers screen pixels 100..200; the stroke is in screen coordinates.
	img := canvas(image.Rect(100, 100, 200, 200), white)
	o := Object{Kind: Pen, Color: red, Width: 6, Points: []render.Pt{{X: 110, Y: 150}, {X: 150, Y: 150}, {X: 190, Y: 150}}}
	Draw(img, o)
	if c := img.RGBAAt(150, 150); c != red {
		t.Fatalf("stroke center = %v", c)
	}
	if c := img.RGBAAt(150, 160); c != white {
		t.Fatalf("outside the stroke = %v", c)
	}
}

func TestHighlighterMultiplies(t *testing.T) {
	img := canvas(image.Rect(0, 0, 50, 50), white)
	img.SetRGBA(25, 10, color.RGBA{0, 0, 0, 255}) // black text pixel
	yellow := color.RGBA{255, 230, 0, 255}
	Draw(img, Object{Kind: Highlighter, Color: yellow, Width: 30, Points: []render.Pt{{X: 5, Y: 10}, {X: 45, Y: 10}}})
	c := img.RGBAAt(20, 10)
	if c.R != 255 || c.B >= 255 || c.B < 100 {
		t.Fatalf("highlighted white = %v", c)
	}
	if k := img.RGBAAt(25, 10); k.R != 0 || k.G != 0 {
		t.Fatalf("black under the highlighter must stay black: %v", k)
	}
}

func TestShapesAndEmoji(t *testing.T) {
	img := canvas(image.Rect(0, 0, 200, 200), white)
	Draw(img, Object{Kind: Rectangle, Color: red, Width: 4, Points: []render.Pt{{X: 20, Y: 20}, {X: 80, Y: 60}}})
	if img.RGBAAt(20, 40) != red || img.RGBAAt(50, 40) != white {
		t.Fatal("rectangle outline")
	}
	Draw(img, Object{Kind: Arrow, Color: red, Width: 4, Points: []render.Pt{{X: 100, Y: 150}, {X: 190, Y: 150}}})
	if img.RGBAAt(185, 150) != red || img.RGBAAt(120, 150) != red {
		t.Fatal("arrow")
	}
	Draw(img, Object{Kind: Emoji, Emoji: 0, Box: image.Rect(100, 20, 148, 68)})
	if img.RGBAAt(124, 44) == white {
		t.Fatal("emoji not drawn")
	}
}

func TestEraseUndoRedo(t *testing.T) {
	var d Doc
	a := Object{Kind: Pen, Color: red, Width: 4, Points: []render.Pt{{X: 0, Y: 0}, {X: 100, Y: 0}}}
	b := Object{Kind: Oval, Color: red, Width: 4, Points: []render.Pt{{X: 0, Y: 50}, {X: 100, Y: 150}}}
	d.Add(a)
	d.Add(b)
	if r := d.Erase(render.Pt{X: 50, Y: 1}, 4); r.Empty() || len(d.Objects) != 1 || d.Objects[0].Kind != Oval {
		t.Fatalf("erase: %v %d", r, len(d.Objects))
	}
	if !d.Erase(render.Pt{X: 50, Y: 100}, 4).Empty() {
		t.Fatal("the inside of an oval is not part of it")
	}
	d.Undo()
	if len(d.Objects) != 2 {
		t.Fatal("undo erase")
	}
	d.Undo()
	d.Undo()
	if len(d.Objects) != 0 || !d.Undo().Empty() {
		t.Fatal("undo to empty")
	}
	d.Redo()
	if len(d.Objects) != 1 {
		t.Fatal("redo")
	}
	d.Add(b) // drops the redo branch
	if !d.Redo().Empty() || len(d.Objects) != 2 {
		t.Fatal("new action clears redo")
	}
	if d.Clear().Empty() || len(d.Objects) != 0 {
		t.Fatal("clear")
	}
}

func TestSmoothKeepsEnds(t *testing.T) {
	in := []render.Pt{{X: 0, Y: 0}, {X: 10, Y: 10}, {X: 20, Y: 0}, {X: 30, Y: 10}}
	out := Smooth(in)
	if out[0] != in[0] || out[len(out)-1] != in[3] || len(out) <= len(in) {
		t.Fatalf("smooth: %v", out)
	}
}

// A straight stroke made of many short segments must have an even edge
// (no darker beads where the pieces meet).
func TestStrokeEdgeIsEven(t *testing.T) {
	img := canvas(image.Rect(0, 0, 200, 40), white)
	var pts []render.Pt
	for x := 10.0; x <= 190; x += 7 {
		pts = append(pts, render.Pt{X: x, Y: 20.3})
	}
	Draw(img, Object{Kind: Pen, Color: color.RGBA{0, 0, 0, 255}, Width: 4, Points: pts})
	ref := img.RGBAAt(40, 22)
	for x := 30; x < 170; x++ {
		if c := img.RGBAAt(x, 22); c != ref {
			t.Fatalf("edge pixel %d = %v, want %v", x, c, ref)
		}
	}
}
