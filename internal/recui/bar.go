// SPDX-License-Identifier: Apache-2.0

// Package recui is the recording control bar (white paper §4: 3-second
// countdown, Start, Pause/Resume, Stop, Delete, microphone and system audio).
package recui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"snippingtoolenhanced/internal/render"
)

// Phase of the recording UI.
type Phase int

const (
	Ready Phase = iota
	Countdown
	Recording
	Paused
)

// Button IDs.
const (
	BtnStart  = "start"
	BtnStop   = "stop"
	BtnPause  = "pause"
	BtnDelete = "delete"
	BtnMic    = "mic"
	BtnSystem = "system"
	BtnClose  = "close"
)

// View is everything the bar shows.
type View struct {
	Phase     Phase
	Count     int // countdown seconds left
	Elapsed   time.Duration
	Mic       bool
	System    bool
	MicOK     bool // source available / capturing
	SystemOK  bool
	Hover     string
	LockAudio bool // audio sources cannot be added after start
}

type item struct {
	id string
	r  image.Rectangle
}

// Layout returns the bar size and button rectangles (bar-local coordinates).
func Layout(v View, scale float64) (image.Point, []item) {
	px := func(f float64) int { return int(math.Round(f * scale)) }
	pad, h, gap := px(6), px(40), px(4)
	var specs []struct {
		id string
		w  int
	}
	add := func(id string, w float64) {
		specs = append(specs, struct {
			id string
			w  int
		}{id, px(w)})
	}
	switch v.Phase {
	case Ready, Countdown:
		add(BtnStart, 96)
		add("|", 13)
		add(BtnMic, 40)
		add(BtnSystem, 40)
		add("|", 13)
		add(BtnClose, 40)
	default:
		add(BtnStop, 40)
		add(BtnPause, 40)
		add("timer", 76)
		add(BtnDelete, 40)
		add("|", 13)
		add(BtnMic, 40)
		add(BtnSystem, 40)
	}
	x := pad
	var items []item
	for i, s := range specs {
		if i > 0 {
			x += gap
		}
		items = append(items, item{s.id, image.Rect(x, pad, x+s.w, pad+h)})
		x += s.w
	}
	return image.Pt(x+pad, h+2*pad), items
}

// Hit returns the button under p (bar-local).
func Hit(v View, scale float64, p image.Point) string {
	_, items := Layout(v, scale)
	for _, it := range items {
		if p.In(it.r) && it.id != "|" && it.id != "timer" {
			return it.id
		}
	}
	return ""
}

var tips = map[string]string{BtnStart: "Start recording", BtnStop: "Stop", BtnPause: "Pause", BtnDelete: "Delete recording",
	BtnMic: "Microphone", BtnSystem: "System audio", BtnClose: "Close"}

// Render draws the bar.
func Render(v View, scale float64, th render.Theme) *image.RGBA {
	size, items := Layout(v, scale)
	img := image.NewRGBA(image.Rect(0, 0, size.X, size.Y))
	f := func(i int) float64 { return float64(i) }
	px := func(x float64) float64 { return math.Round(x * scale) }
	render.FillRoundRect(img, 0, 0, f(size.X), f(size.Y), 0, th.Surface)
	red := color.RGBA{0xC4, 0x2B, 0x1C, 0xFF}
	isz := px(20)
	muted := th.TextMuted
	muted.A = 0x80
	for _, it := range items {
		r := it.r
		x, y, w, h := f(r.Min.X), f(r.Min.Y), f(r.Dx()), f(r.Dy())
		if it.id == "|" {
			render.FillRoundRect(img, x+w/2, y+px(8), 1, h-px(16), 0, th.Border)
			continue
		}
		if it.id == v.Hover && it.id != BtnStart {
			render.FillRoundRect(img, x, y, w, h, px(4), th.Selected)
		}
		icon := func(name string, c color.RGBA) { render.DrawIcon(img, name, x+(w-isz)/2, y+(h-isz)/2, isz, c) }
		switch it.id {
		case BtnStart:
			bg := th.Accent
			if v.Hover == BtnStart {
				bg.A = 0xE0
			}
			render.FillRoundRect(img, x, y+px(2), w, h-px(4), px(4), bg)
			label := "Start"
			if v.Phase == Countdown {
				label = fmt.Sprintf("Starting %d", v.Count)
			}
			render.TextCentered(img, label, x, y, w, h, px(14), th.OnAccent)
		case BtnStop:
			render.FillRoundRect(img, x+px(8), y+px(8), w-px(16), h-px(16), px(4), red)
			render.DrawIcon(img, render.IconStop, x+(w-px(14))/2, y+(h-px(14))/2, px(14), color.RGBA{255, 255, 255, 255})
		case BtnPause:
			if v.Phase == Paused {
				icon(render.IconPlay, th.Text)
			} else {
				icon(render.IconPause, th.Text)
			}
		case "timer":
			el := v.Elapsed.Truncate(time.Second)
			t := fmt.Sprintf("%02d:%02d:%02d", int(el.Hours()), int(el.Minutes())%60, int(el.Seconds())%60)
			c := th.Text
			if v.Phase == Paused {
				c = th.TextMuted
			}
			render.TextCentered(img, t, x, y, w, h, px(14), c)
		case BtnDelete:
			icon(render.IconDelete, th.Text)
		case BtnMic:
			name, c := render.IconMic, th.Text
			if !v.Mic {
				name = render.IconMicOff
			}
			if !v.MicOK {
				c = muted
			}
			icon(name, c)
		case BtnSystem:
			name, c := render.IconSpeaker, th.Text
			if !v.System {
				name = render.IconSpeakerOff
			}
			if !v.SystemOK {
				c = muted
			}
			icon(name, c)
		case BtnClose:
			icon(render.IconClose, th.Text)
		}
	}
	return img
}

// Tip returns the tooltip text for a button.
func Tip(id string, v View) string {
	switch id {
	case BtnPause:
		if v.Phase == Paused {
			return "Resume"
		}
	case BtnMic:
		if v.Mic {
			return "Microphone on"
		}
		return "Microphone off"
	case BtnSystem:
		if v.System {
			return "System audio on"
		}
		return "System audio off"
	}
	return tips[id]
}
