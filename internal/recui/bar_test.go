// SPDX-License-Identifier: Apache-2.0

package recui

import (
	"image"
	"image/draw"
	"image/png"
	"os"
	"testing"
	"time"

	"snippingtoolenhanced/internal/render"
)

func TestLayoutHit(t *testing.T) {
	v := View{Phase: Ready, Mic: true, System: true, MicOK: true, SystemOK: true}
	size, items := Layout(v, 1)
	if size.X < 200 || len(items) != 6 {
		t.Fatal(size, len(items))
	}
	if Hit(v, 1, items[0].r.Min.Add(image.Pt(2, 2))) != BtnStart || Hit(v, 1, image.Pt(-5, -5)) != "" {
		t.Fatal("hit")
	}
	v.Phase = Recording
	_, items = Layout(v, 1.5)
	if items[0].id != BtnStop || items[2].id != "timer" || Hit(v, 1.5, items[2].r.Min.Add(image.Pt(2, 2))) != "" {
		t.Fatal("recording layout")
	}
}

func TestPreview(t *testing.T) {
	out := os.Getenv("RECBAR_PREVIEW")
	if out == "" {
		t.Skip()
	}
	views := []View{
		{Phase: Ready, Mic: false, System: true, MicOK: true, SystemOK: true, Hover: BtnStart},
		{Phase: Countdown, Count: 2, Mic: true, System: true, MicOK: true, SystemOK: true},
		{Phase: Recording, Elapsed: 83 * time.Second, Mic: true, System: true, MicOK: true, SystemOK: true, Hover: BtnPause},
		{Phase: Paused, Elapsed: 83 * time.Second, Mic: false, System: true, MicOK: false, SystemOK: true},
	}
	sheet := image.NewRGBA(image.Rect(0, 0, 420, 260))
	draw.Draw(sheet, sheet.Rect, image.NewUniform(render.Premul(render.Light.Border)), image.Point{}, draw.Src)
	y := 10
	for _, v := range views {
		img := Render(v, 1, render.Light)
		draw.Draw(sheet, img.Rect.Add(image.Pt(10, y)), img, image.Point{}, draw.Src)
		y += img.Rect.Dy() + 10
	}
	f, _ := os.Create(out)
	png.Encode(f, sheet)
	f.Close()
}
