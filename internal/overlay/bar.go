// SPDX-License-Identifier: Apache-2.0

// Package overlay is the capture overlay (white paper §2 "capture overlay",
// §3 snip modes and capture-bar tools).
package overlay

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"

	"snippingtoolenhanced/internal/render"
)

// Mode is a snip mode.
type Mode string

// The snip modes of white paper §3.
const (
	Square     Mode = "square"
	Rectangle  Mode = "rectangle"
	Freeform   Mode = "freeform"
	Window     Mode = "window"
	Fullscreen Mode = "fullscreen"
)

// Kind is what the overlay captures.
type Kind string

const (
	KindSnip   Kind = "snip"
	KindRecord Kind = "record"
)

// SnipModes and RecordModes are listed in the mode dropdown (white paper §3, §4).
var (
	SnipModes   = []Mode{Rectangle, Square, Window, Fullscreen, Freeform}
	RecordModes = []Mode{Rectangle, Window}
)

// ModesFor returns the modes available for a kind.
func ModesFor(k Kind) []Mode {
	if k == KindRecord {
		return RecordModes
	}
	return SnipModes
}

// Bar item IDs.
const (
	ItemSnip   = "snip"
	ItemRecord = "record"
	ItemMode   = "mode"
	ItemMarkup = "markup"
	ItemColor  = "color"
	ItemText   = "text"
	ItemClose  = "close"
	menuPrefix = "menu:"
)

var labels = map[Mode]string{Rectangle: "Rectangle", Window: "Window", Fullscreen: "Full screen", Freeform: "Freeform", Square: "Square"}

var modeIcons = map[Mode]string{
	Rectangle: render.IconRectangle, Window: render.IconWindow, Fullscreen: render.IconFullscreen,
	Freeform: render.IconFreeform, Square: render.IconSquare,
}

// BarState is everything that changes how the bar looks.
type BarState struct {
	Kind        Kind
	Mode        Mode
	ColorActive bool
	TextActive  bool // Text Extractor
	Hover       string
	MenuOpen    bool
}

// BarItem is one control on the capture bar.
type BarItem struct {
	ID   string
	Rect image.Rectangle // frame coordinates
}

// Bar is the floating capture bar at the top of the active monitor.
type Bar struct {
	Items   []BarItem
	Seg     image.Rectangle // snip/record segmented control
	Surface image.Rectangle
	Area    image.Rectangle // everything the bar may paint (incl. shadow, tooltip, menu)
	Scale   float64
	mon     image.Rectangle
}

func (b Bar) px(v float64) int { return int(math.Round(v * b.Scale)) }

// LayoutBar places the bar centered at the top of monitor mon.
func LayoutBar(mon image.Rectangle, scale float64) Bar {
	b := Bar{Scale: scale, mon: mon}
	pad, h, gap, group := b.px(6), b.px(40), b.px(4), b.px(14)
	type spec struct {
		id string
		w  int
		g  int // extra space before
	}
	specs := []spec{{ItemSnip, b.px(44), 0}, {ItemRecord, b.px(44), 0}, {ItemMode, b.px(56), group},
		{ItemMarkup, b.px(44), group}, {ItemColor, b.px(44), 0}, {ItemText, b.px(44), 0}, {ItemClose, b.px(44), group}}
	w := pad * 2
	for i, sp := range specs {
		w += sp.w + sp.g
		if i > 0 {
			w += gap
		}
	}
	x0 := mon.Min.X + (mon.Dx()-w)/2
	y0 := mon.Min.Y + b.px(12)
	b.Surface = image.Rect(x0, y0, x0+w, y0+h+pad*2)
	x := x0 + pad
	for i, sp := range specs {
		if i > 0 {
			x += gap
		}
		x += sp.g
		b.Items = append(b.Items, BarItem{ID: sp.id, Rect: image.Rect(x, y0+pad, x+sp.w, y0+pad+h)})
		x += sp.w
	}
	b.Seg = b.Items[0].Rect.Union(b.Items[1].Rect).Inset(-b.px(2))
	m := b.px(24)
	menuH := len(SnipModes)*b.px(36) + b.px(16)
	b.Area = image.Rect(b.Surface.Min.X-m, b.Surface.Min.Y-m, b.Surface.Max.X+m, b.Surface.Max.Y+menuH+b.px(24)).Intersect(mon)
	return b
}

func (b Bar) item(id string) image.Rectangle {
	for _, it := range b.Items {
		if it.ID == id {
			return it.Rect
		}
	}
	return image.Rectangle{}
}

// Enabled reports whether a bar item works in this version and kind.
func Enabled(id string, k Kind) bool {
	switch id {
	case ItemMarkup:
		return false // Quick markup is not built yet
	case ItemColor, ItemText:
		return k == KindSnip
	}
	return true
}

func (b Bar) menuRect(k Kind) image.Rectangle {
	mr := b.item(ItemMode)
	n := len(ModesFor(k))
	w := b.px(196)
	return image.Rect(mr.Min.X, b.Surface.Max.Y+b.px(4), mr.Min.X+w, b.Surface.Max.Y+b.px(4)+n*b.px(36)+b.px(8))
}

func (b Bar) menuItems(k Kind) []BarItem {
	r := b.menuRect(k)
	var out []BarItem
	for i, m := range ModesFor(k) {
		y := r.Min.Y + b.px(4) + i*b.px(36)
		out = append(out, BarItem{ID: menuPrefix + string(m), Rect: image.Rect(r.Min.X+b.px(4), y, r.Max.X-b.px(4), y+b.px(36))})
	}
	return out
}

// Hit returns the control under p: an item ID, "menu:<mode>", "menu", "bar" or "".
func (b Bar) Hit(p image.Point, st BarState) string {
	if st.MenuOpen {
		for _, it := range b.menuItems(st.Kind) {
			if p.In(it.Rect) {
				return it.ID
			}
		}
		if p.In(b.menuRect(st.Kind)) {
			return "menu"
		}
	}
	for _, it := range b.Items {
		if p.In(it.Rect) {
			return it.ID
		}
	}
	if p.In(b.Surface) {
		return "bar"
	}
	return ""
}

// MenuMode extracts the mode from a "menu:<mode>" hit.
func MenuMode(hit string) (Mode, bool) {
	if len(hit) > len(menuPrefix) && hit[:len(menuPrefix)] == menuPrefix {
		return Mode(hit[len(menuPrefix):]), true
	}
	return "", false
}

var tips = map[string]string{
	ItemSnip: "Snip", ItemRecord: "Record", ItemMode: "Snipping area",
	ItemMarkup: "Quick markup – not available in this version yet", ItemColor: "Color picker",
	ItemText: "Text extractor", ItemClose: "Close (Esc)",
}

// Render draws the bar over backdrop (the dimmed screen under b.Area).
func (b Bar) Render(backdrop *image.RGBA, th render.Theme, st BarState) *image.RGBA {
	out := image.NewRGBA(b.Area)
	draw.Draw(out, b.Area, backdrop, b.Area.Min, draw.Src)
	f := func(v int) float64 { return float64(v) }
	s := b.Surface
	r := f(b.px(8))
	render.Shadow(out, f(s.Min.X), f(s.Min.Y), f(s.Dx()), f(s.Dy()), r, f(b.px(16)), th.ShadowBase, 0.3)
	render.FillRoundRect(out, f(s.Min.X), f(s.Min.Y), f(s.Dx()), f(s.Dy()), r, th.Border)
	render.FillRoundRect(out, f(s.Min.X)+1, f(s.Min.Y)+1, f(s.Dx())-2, f(s.Dy())-2, r-1, th.Surface)

	// Segmented snip / record control.
	sg := b.Seg
	render.FillRoundRect(out, f(sg.Min.X), f(sg.Min.Y), f(sg.Dx()), f(sg.Dy()), f(b.px(6)), th.Selected)
	isz := f(b.px(20))
	disabled := th.TextMuted
	disabled.A = 0x70
	for _, it := range b.Items {
		ir := it.Rect
		on := (it.ID == ItemSnip && st.Kind == KindSnip) || (it.ID == ItemRecord && st.Kind == KindRecord) ||
			(it.ID == ItemColor && st.ColorActive) || (it.ID == ItemText && st.TextActive) || (it.ID == ItemMode && st.MenuOpen)
		en := Enabled(it.ID, st.Kind)
		rad := f(b.px(4))
		switch {
		case on && (it.ID == ItemSnip || it.ID == ItemRecord):
			render.FillRoundRect(out, f(ir.Min.X), f(ir.Min.Y), f(ir.Dx()), f(ir.Dy()), rad, th.Border)
			render.FillRoundRect(out, f(ir.Min.X)+1, f(ir.Min.Y)+1, f(ir.Dx())-2, f(ir.Dy())-2, rad, th.Card)
			pw, ph := f(b.px(16)), f(b.px(3))
			render.FillRoundRect(out, f(ir.Min.X)+(f(ir.Dx())-pw)/2, f(ir.Max.Y)-ph-f(b.px(2)), pw, ph, ph/2, th.Accent)
		case on:
			render.FillRoundRect(out, f(ir.Min.X), f(ir.Min.Y), f(ir.Dx()), f(ir.Dy()), rad, th.Hover)
		case it.ID == st.Hover && en:
			render.FillRoundRect(out, f(ir.Min.X), f(ir.Min.Y), f(ir.Dx()), f(ir.Dy()), rad, th.Selected)
		}
		col := th.Text
		if !en {
			col = disabled
		}
		cy := f(ir.Min.Y) + (f(ir.Dy())-isz)/2
		switch it.ID {
		case ItemMode:
			render.DrawIcon(out, modeIcons[st.Mode], f(ir.Min.X+b.px(6)), cy, isz, col)
			cs := f(b.px(12))
			render.DrawIcon(out, render.IconChevron, f(ir.Max.X-b.px(18)), f(ir.Min.Y)+(f(ir.Dy())-cs)/2, cs, th.TextMuted)
		default:
			icon := map[string]string{ItemSnip: render.IconCamera, ItemRecord: render.IconVideo, ItemMarkup: render.IconMarkup,
				ItemColor: render.IconColor, ItemText: render.IconText, ItemClose: render.IconClose}[it.ID]
			render.DrawIcon(out, icon, f(ir.Min.X)+(f(ir.Dx())-isz)/2, cy, isz, col)
		}
	}
	if st.MenuOpen {
		b.menu(out, th, st)
	} else if t, ok := tips[st.Hover]; ok {
		b.tooltip(out, th, t, b.item(st.Hover))
	}
	return out
}

func (b Bar) menu(out *image.RGBA, th render.Theme, st BarState) {
	f := func(v int) float64 { return float64(v) }
	mr := b.menuRect(st.Kind)
	r := f(b.px(8))
	render.Shadow(out, f(mr.Min.X), f(mr.Min.Y), f(mr.Dx()), f(mr.Dy()), r, f(b.px(12)), th.ShadowBase, 0.28)
	render.FillRoundRect(out, f(mr.Min.X), f(mr.Min.Y), f(mr.Dx()), f(mr.Dy()), r, th.Border)
	render.FillRoundRect(out, f(mr.Min.X)+1, f(mr.Min.Y)+1, f(mr.Dx())-2, f(mr.Dy())-2, r-1, th.Surface)
	isz := f(b.px(16))
	for _, it := range b.menuItems(st.Kind) {
		m, _ := MenuMode(it.ID)
		ir := it.Rect
		if it.ID == st.Hover {
			render.FillRoundRect(out, f(ir.Min.X), f(ir.Min.Y), f(ir.Dx()), f(ir.Dy()), f(b.px(4)), th.Selected)
		}
		if m == st.Mode {
			render.FillRoundRect(out, f(ir.Min.X), f(ir.Min.Y)+f(ir.Dy())/2-f(b.px(8)), f(b.px(3)), f(b.px(16)), f(b.px(2)), th.Accent)
		}
		render.DrawIcon(out, modeIcons[m], f(ir.Min.X+b.px(12)), f(ir.Min.Y)+(f(ir.Dy())-isz)/2, isz, th.Text)
		render.Text(out, labels[m], f(ir.Min.X+b.px(40)), f(ir.Min.Y)+f(ir.Dy())/2, f(b.px(14)), th.Text)
	}
}

func (b Bar) tooltip(out *image.RGBA, th render.Theme, text string, anchor image.Rectangle) {
	fs := float64(b.px(12))
	tw := render.MeasureText(text, fs) + float64(b.px(16))
	h := float64(b.px(28))
	x := float64(anchor.Min.X) + float64(anchor.Dx())/2 - tw/2
	x = math.Max(float64(b.Area.Min.X+2), math.Min(x, float64(b.Area.Max.X)-tw-2))
	y := float64(b.Surface.Max.Y + b.px(8))
	r := float64(b.px(4))
	render.Shadow(out, x, y, tw, h, r, float64(b.px(6)), th.ShadowBase, 0.2)
	render.FillRoundRect(out, x, y, tw, h, r, th.Border)
	render.FillRoundRect(out, x+1, y+1, tw-2, h-2, r, th.Surface)
	render.TextCentered(out, text, x, y, tw, h, fs, th.Text)
}

// ---------- color picker ----------

// ColorFormats are cycled with Tab in the color picker.
var ColorFormats = []string{"HEX", "RGB", "HSL"}

// FormatColor renders c in format f (index into ColorFormats).
func FormatColor(c color.RGBA, f int) string {
	switch f % len(ColorFormats) {
	case 1:
		return fmt.Sprintf("rgb(%d, %d, %d)", c.R, c.G, c.B)
	case 2:
		h, s, l := hsl(c)
		return fmt.Sprintf("hsl(%d, %d%%, %d%%)", h, s, l)
	default:
		return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
	}
}

func hsl(c color.RGBA) (int, int, int) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l := (mx + mn) / 2
	if mx == mn {
		return 0, 0, int(math.Round(l * 100))
	}
	d := mx - mn
	s := d / (1 - math.Abs(2*l-1))
	var h float64
	switch mx {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return int(math.Round(h)) % 360, int(math.Round(s * 100)), int(math.Round(l * 100))
}

// Loupe is the magnifier shown in color-picker mode.
type Loupe struct {
	Sample int // odd number of screen pixels per side
	Format int
}

// LoupeRect returns where the loupe is drawn for cursor p inside bounds.
func (l Loupe) LoupeRect(p image.Point, bounds image.Rectangle, scale float64) image.Rectangle {
	cell := int(math.Max(6, math.Round(132*scale/float64(l.Sample))))
	w := cell * l.Sample
	h := w + int(math.Round(48*scale))
	off := int(24 * scale)
	x, y := p.X+off, p.Y+off
	if x+w > bounds.Max.X {
		x = p.X - off - w
	}
	if y+h > bounds.Max.Y {
		y = p.Y - off - h
	}
	return image.Rect(x, y, x+w, y+h)
}

// Render draws the loupe for cursor p over backdrop (covering the returned rect).
func (l Loupe) Render(frame, backdrop *image.RGBA, p image.Point, scale float64, th render.Theme) (*image.RGBA, image.Rectangle, color.RGBA) {
	area := l.LoupeRect(p, frame.Bounds(), scale)
	pad := int(math.Round(12 * scale))
	full := area.Inset(-pad).Intersect(frame.Bounds())
	out := image.NewRGBA(full)
	draw.Draw(out, full, backdrop, full.Min, draw.Src)
	cell := area.Dx() / l.Sample
	grid := cell * l.Sample
	f := func(v int) float64 { return float64(v) }
	r := 8 * scale
	render.Shadow(out, f(area.Min.X), f(area.Min.Y), f(area.Dx()), f(area.Dy()), r, 10*scale, th.ShadowBase, 0.3)
	render.FillRoundRect(out, f(area.Min.X), f(area.Min.Y), f(area.Dx()), f(area.Dy()), r, th.Border)
	render.FillRoundRect(out, f(area.Min.X)+1, f(area.Min.Y)+1, f(area.Dx())-2, f(area.Dy())-2, r-1, th.Surface)
	half := l.Sample / 2
	var center color.RGBA
	gx, gy := area.Min.X, area.Min.Y
	// Clip the magnified grid to the rounded top of the loupe.
	clip := image.NewAlpha(image.Rect(0, 0, grid, grid))
	tmp := image.NewRGBA(clip.Rect)
	render.FillRoundRect(tmp, 1, 1, f(grid)-2, f(grid)+r, r-1, color.RGBA{0, 0, 0, 255})
	for i := 3; i < len(tmp.Pix); i += 4 {
		clip.Pix[i/4] = tmp.Pix[i]
	}
	mag := image.NewRGBA(clip.Rect)
	for sy := 0; sy < l.Sample; sy++ {
		for sx := 0; sx < l.Sample; sx++ {
			q := image.Pt(p.X-half+sx, p.Y-half+sy)
			c := color.RGBA{0, 0, 0, 255}
			if q.In(frame.Bounds()) {
				c = frame.RGBAAt(q.X, q.Y)
			}
			if sx == half && sy == half {
				center = c
			}
			draw.Draw(mag, image.Rect(sx*cell, sy*cell, sx*cell+cell, sy*cell+cell), image.NewUniform(c), image.Point{}, draw.Src)
		}
	}
	gridLine := color.RGBA{0x80, 0x80, 0x80, 0x30}
	for i := 1; i < l.Sample; i++ {
		draw.Draw(mag, image.Rect(i*cell, 0, i*cell+1, grid), image.NewUniform(render.Premul(gridLine)), image.Point{}, draw.Over)
		draw.Draw(mag, image.Rect(0, i*cell, grid, i*cell+1), image.NewUniform(render.Premul(gridLine)), image.Point{}, draw.Over)
	}
	cx, cy := half*cell, half*cell
	render.StrokeRoundRect(mag, f(cx)-1, f(cy)-1, f(cell)+2, f(cell)+2, 0, 1, color.RGBA{0, 0, 0, 255})
	render.StrokeRoundRect(mag, f(cx), f(cy), f(cell), f(cell), 0, 1, color.RGBA{255, 255, 255, 255})
	draw.DrawMask(out, image.Rect(gx, gy, gx+grid, gy+grid), mag, image.Point{}, clip, image.Point{}, draw.Over)
	// Label: swatch + value + format.
	ly := f(gy + grid)
	lh := f(area.Max.Y) - ly
	sw := 16 * scale
	render.FillRoundRect(out, f(gx)+10*scale, ly+(lh-sw)/2, sw, sw, 4*scale, th.Border)
	render.FillRoundRect(out, f(gx)+10*scale+1, ly+(lh-sw)/2+1, sw-2, sw-2, 3*scale, center)
	render.Text(out, FormatColor(center, l.Format), f(gx)+34*scale, ly+lh/2-7*scale, 13*scale, th.Text)
	render.Text(out, "Click to copy", f(gx)+34*scale, ly+lh/2+9*scale, 11*scale, th.TextMuted)
	return out, full, center
}
