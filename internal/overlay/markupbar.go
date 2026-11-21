// SPDX-License-Identifier: Apache-2.0

package overlay

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"

	"snippingtoolenhanced/internal/emoji"
	"snippingtoolenhanced/internal/markup"
	"snippingtoolenhanced/internal/render"
)

// Quick markup toolbar items (white paper §3: "after selecting a region you
// annotate inside the selection, re-crop with handles, then copy, save or
// share without opening the editor").
const (
	ToolPen       = "pen"
	ToolHighlight = "highlighter"
	ToolEraser    = "eraser"
	ToolShapes    = "shapes"
	ToolEmoji     = "emoji"
	ActCopy       = "copy"
	ActSave       = "save"
	ActShare      = "share"
	ActSearch     = "search" // Visual Search
	ActClose      = "mclose"
)

// Ink colors, the same as the editor's.
var (
	PenColors = hexColors("#000000", "#ffffff", "#d1d3d4", "#a7a9ac", "#808285", "#58595b", "#b31564", "#e61b1b", "#ff5500", "#ffaa00",
		"#ffce00", "#ffe600", "#a2e61b", "#26e600", "#008055", "#00aacc", "#004de6", "#3d00b8", "#6600cc", "#600080",
		"#f7d7c4", "#bb9167", "#8e562e", "#613d30", "#ff80ff", "#ffc680", "#ffff80", "#80ff9e", "#80d6ff", "#bcb3ff")
	HighlightColors = hexColors("#ffe600", "#26e600", "#44c8f5", "#ff80ff", "#ff8000", "#bcb3ff")
	ShapeColors     = hexColors("#000000", "#ffffff", "#e61b1b", "#ff5500", "#ffce00", "#26e600", "#00aacc", "#004de6", "#6600cc", "#b31564", "#808285", "#8e562e")
	ShapeKinds      = []markup.Kind{markup.Line, markup.Arrow, markup.Rectangle, markup.Oval}
)

func hexColors(hs ...string) []color.RGBA {
	out := make([]color.RGBA, len(hs))
	for i, h := range hs {
		var r, g, b uint8
		fmt.Sscanf(strings.TrimPrefix(h, "#"), "%02x%02x%02x", &r, &g, &b)
		out[i] = color.RGBA{r, g, b, 255}
	}
	return out
}

// Ink is the current tool settings.
type Ink struct {
	Tool       string
	PenColor   color.RGBA
	PenSize    int
	HLColor    color.RGBA
	HLSize     int
	ShapeKind  markup.Kind
	ShapeColor color.RGBA
	ShapeSize  int
	Emoji      int // -1 until one is picked
}

// DefaultInk matches the editor's defaults.
func DefaultInk() Ink {
	return Ink{Tool: ToolPen, PenColor: PenColors[7], PenSize: 4, HLColor: HighlightColors[0], HLSize: 16,
		ShapeKind: markup.Rectangle, ShapeColor: ShapeColors[2], ShapeSize: 4, Emoji: -1}
}

// MarkupState is everything that changes how the toolbar looks.
type MarkupState struct {
	Ink     Ink
	Hover   string
	Flyout  string // tool whose options are open, or ""
	CanUndo bool
	// SearchName is the Visual Search engine shown in the tooltip ("Bing").
	SearchName string
}

// MarkupBar is the Quick markup toolbar next to the selection.
type MarkupBar struct {
	Items   []BarItem
	Surface image.Rectangle
	Scale   float64
	mon     image.Rectangle
	below   bool // the bar is under the selection (flyouts open downwards)
}

func (b MarkupBar) px(v float64) int { return int(math.Round(v * b.Scale)) }

// LayoutMarkupBar places the toolbar centered under the selection, above it
// if there is no room, or inside its bottom edge (full-screen selections).
func LayoutMarkupBar(sel, mon image.Rectangle, scale float64) MarkupBar {
	b := MarkupBar{Scale: scale, mon: mon}
	pad, h, gap, group := b.px(6), b.px(40), b.px(4), b.px(14)
	ids := []string{ToolPen, ToolHighlight, ToolEraser, ToolShapes, ToolEmoji, ActCopy, ActSave, ActShare, ActSearch, ActClose}
	groupBefore := map[string]bool{ActCopy: true, ActClose: true}
	w := pad * 2
	for i, id := range ids {
		w += b.px(44)
		if i > 0 {
			w += gap
		}
		if groupBefore[id] {
			w += group
		}
	}
	bh := h + pad*2
	margin := b.px(10)
	x0 := sel.Min.X + (sel.Dx()-w)/2
	x0 = max(mon.Min.X+margin, min(x0, mon.Max.X-margin-w))
	var y0 int
	switch {
	case sel.Max.Y+margin+bh <= mon.Max.Y-margin:
		y0, b.below = sel.Max.Y+margin, true
	case sel.Min.Y-margin-bh >= mon.Min.Y+margin:
		y0 = sel.Min.Y - margin - bh
	default:
		y0, b.below = max(mon.Min.Y+margin, min(sel.Max.Y, mon.Max.Y)-margin-bh), false
	}
	b.Surface = image.Rect(x0, y0, x0+w, y0+bh)
	x := x0 + pad
	for i, id := range ids {
		if i > 0 {
			x += gap
		}
		if groupBefore[id] {
			x += group
		}
		b.Items = append(b.Items, BarItem{ID: id, Rect: image.Rect(x, y0+pad, x+b.px(44), y0+pad+h)})
		x += b.px(44)
	}
	return b
}

func (b MarkupBar) item(id string) image.Rectangle {
	for _, it := range b.Items {
		if it.ID == id {
			return it.Rect
		}
	}
	return image.Rectangle{}
}

// Area is everything the toolbar paints, including its shadow and tooltip.
func (b MarkupBar) Area() image.Rectangle {
	m := b.px(24)
	r := b.Surface.Inset(-m)
	if b.below {
		r.Max.Y += b.px(32)
	} else {
		r.Min.Y -= b.px(32)
	}
	return r.Intersect(b.mon)
}

// Hit returns the toolbar item under p, "bar" for the toolbar's background, or "".
func (b MarkupBar) Hit(p image.Point) string {
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

var markupTips = map[string]string{
	ToolPen: "Ballpoint pen", ToolHighlight: "Highlighter", ToolEraser: "Eraser", ToolShapes: "Shapes", ToolEmoji: "Emoji",
	ActCopy: "Copy (Ctrl+C)", ActSave: "Save as (Ctrl+S)", ActShare: "Share", ActClose: "Close (Esc)",
}

var markupIcons = map[string]string{
	ToolPen: render.IconPen, ToolHighlight: render.IconHighlight, ToolEraser: render.IconEraser, ToolShapes: render.IconShapes,
	ToolEmoji: render.IconEmoji, ActCopy: render.IconCopy, ActSave: render.IconSave, ActShare: render.IconShare, ActSearch: render.IconVisualSearch,
	ActClose: render.IconClose,
}

// Render draws the toolbar on a transparent image covering Area().
func (b MarkupBar) Render(th render.Theme, st MarkupState) *image.RGBA {
	area := b.Area()
	out := image.NewRGBA(area)
	f := func(v int) float64 { return float64(v) }
	s := b.Surface
	r := f(b.px(8))
	render.Shadow(out, f(s.Min.X), f(s.Min.Y), f(s.Dx()), f(s.Dy()), r, f(b.px(16)), th.ShadowBase, 0.3)
	render.FillRoundRect(out, f(s.Min.X), f(s.Min.Y), f(s.Dx()), f(s.Dy()), r, th.Border)
	render.FillRoundRect(out, f(s.Min.X)+1, f(s.Min.Y)+1, f(s.Dx())-2, f(s.Dy())-2, r-1, th.Surface)
	isz := f(b.px(20))
	for _, it := range b.Items {
		ir := it.Rect
		rad := f(b.px(4))
		on := it.ID == st.Ink.Tool
		switch {
		case on:
			render.FillRoundRect(out, f(ir.Min.X), f(ir.Min.Y), f(ir.Dx()), f(ir.Dy()), rad, th.Selected)
			pw, ph := f(b.px(16)), f(b.px(3))
			render.FillRoundRect(out, f(ir.Min.X)+(f(ir.Dx())-pw)/2, f(ir.Max.Y)-ph-f(b.px(2)), pw, ph, ph/2, th.Accent)
		case it.ID == st.Hover:
			render.FillRoundRect(out, f(ir.Min.X), f(ir.Min.Y), f(ir.Dx()), f(ir.Dy()), rad, th.Selected)
		}
		x, y := f(ir.Min.X)+(f(ir.Dx())-isz)/2, f(ir.Min.Y)+(f(ir.Dy())-isz)/2
		switch it.ID {
		case ToolPen:
			render.DrawToolTip(out, render.IconPen, x, y, isz, st.Ink.PenColor)
		case ToolHighlight:
			render.DrawToolTip(out, render.IconHighlight, x, y, isz, st.Ink.HLColor)
		}
		render.DrawIcon(out, markupIcons[it.ID], x, y, isz, th.Text)
	}
	if st.Hover == ActSearch && st.Flyout == "" {
		name := st.SearchName
		if name == "" {
			name = "Bing"
		}
		b.tooltip(out, th, "Visual Search with "+name, b.item(ActSearch))
	} else if t, ok := markupTips[st.Hover]; ok && st.Flyout == "" {
		b.tooltip(out, th, t, b.item(st.Hover))
	}
	return out
}

func (b MarkupBar) tooltip(out *image.RGBA, th render.Theme, text string, anchor image.Rectangle) {
	fs := float64(b.px(12))
	tw := render.MeasureText(text, fs) + float64(b.px(16))
	h := float64(b.px(28))
	x := float64(anchor.Min.X) + float64(anchor.Dx())/2 - tw/2
	area := out.Bounds()
	x = math.Max(float64(area.Min.X+2), math.Min(x, float64(area.Max.X)-tw-2))
	y := float64(b.Surface.Max.Y + b.px(6))
	if !b.below {
		y = float64(b.Surface.Min.Y-b.px(6)) - h
	}
	r := float64(b.px(4))
	render.Shadow(out, x, y, tw, h, r, float64(b.px(6)), th.ShadowBase, 0.2)
	render.FillRoundRect(out, x, y, tw, h, r, th.Border)
	render.FillRoundRect(out, x+1, y+1, tw-2, h-2, r, th.Surface)
	render.TextCentered(out, text, x, y, tw, h, fs, th.Text)
}

// ---------- flyouts (tool options) ----------

// Flyout is the options panel of a tool.
type Flyout struct {
	Tool   string
	Rect   image.Rectangle
	Items  []BarItem       // "c:<index>", "k:<index>", "e:<index>", "eraseall"
	Slider image.Rectangle // size slider track area (pen, highlighter, shapes)
	Max    int             // slider maximum
	Scale  float64
	title  string
	sizeY  int // baseline row of the size slider
}

func (fl Flyout) px(v float64) int { return int(math.Round(v * fl.Scale)) }

// LayoutFlyout places tool's options next to the toolbar.
func (b MarkupBar) LayoutFlyout(tool string) Flyout {
	fl := Flyout{Tool: tool, Scale: b.Scale}
	pad, sw, cols := fl.px(12), fl.px(28), 10
	var colors []color.RGBA
	switch tool {
	case ToolPen:
		colors, fl.Max, fl.title = PenColors, 24, "Ballpoint pen"
	case ToolHighlight:
		colors, fl.Max, fl.title, cols = HighlightColors, 40, "Highlighter", 6
	case ToolShapes:
		colors, fl.Max, fl.title, cols = ShapeColors, 24, "Shapes", 6
	case ToolEraser:
		fl.title = "Eraser"
	case ToolEmoji:
		fl.title = "Emoji"
	default:
		return Flyout{}
	}
	w := pad*2 + cols*sw
	if tool == ToolEraser {
		w = fl.px(220)
	}
	if tool == ToolEmoji {
		w = pad*2 + 8*fl.px(36)
	}
	w = max(w, fl.px(220))
	// Content, relative to (0,0); moved into place below.
	y := pad + fl.px(28) // under the title
	var items []BarItem
	switch tool {
	case ToolEraser:
		items = append(items, BarItem{ID: "eraseall", Rect: image.Rect(pad, y, w-pad, y+fl.px(36))})
		y += fl.px(36)
	case ToolEmoji:
		cell := fl.px(36)
		for i := range emoji.List {
			cx, cy := pad+(i%8)*cell, y+(i/8)*cell
			items = append(items, BarItem{ID: fmt.Sprintf("e:%d", i), Rect: image.Rect(cx, cy, cx+cell, cy+cell)})
		}
		y += (len(emoji.List) + 7) / 8 * cell
	default:
		if tool == ToolShapes {
			kw := fl.px(44)
			for i := range ShapeKinds {
				items = append(items, BarItem{ID: fmt.Sprintf("k:%d", i), Rect: image.Rect(pad+i*kw, y, pad+i*kw+fl.px(40), y+fl.px(36))})
			}
			y += fl.px(44)
		}
		for i := range colors {
			cx, cy := pad+(i%cols)*sw, y+(i/cols)*sw
			items = append(items, BarItem{ID: fmt.Sprintf("c:%d", i), Rect: image.Rect(cx, cy, cx+sw, cy+sw)})
		}
		y += (len(colors) + cols - 1) / cols * sw
		y += fl.px(10)
		fl.sizeY = y + fl.px(16)
		fl.Slider = image.Rect(pad+fl.px(44), y, w-pad-fl.px(36), y+fl.px(32))
		y += fl.px(32)
	}
	h := y + pad
	// Under (or above) the toolbar, centered on the tool's button.
	anchor := b.item(tool)
	x0 := anchor.Min.X + anchor.Dx()/2 - w/2
	x0 = max(b.mon.Min.X+fl.px(8), min(x0, b.mon.Max.X-fl.px(8)-w))
	y0 := b.Surface.Max.Y + fl.px(6)
	if !b.below || y0+h > b.mon.Max.Y {
		y0 = b.Surface.Min.Y - fl.px(6) - h
	}
	off := image.Pt(x0, y0)
	fl.Rect = image.Rect(0, 0, w, h).Add(off)
	for i := range items {
		items[i].Rect = items[i].Rect.Add(off)
	}
	fl.Items = items
	if !fl.Slider.Empty() {
		fl.Slider = fl.Slider.Add(off)
		fl.sizeY += y0
	}
	return fl
}

// Area is everything the flyout paints (with its shadow).
func (fl Flyout) Area() image.Rectangle { return fl.Rect.Inset(-fl.px(16)) }

// Hit returns the flyout item under p: an item ID, "slider", "fly" (background) or "".
func (fl Flyout) Hit(p image.Point) string {
	if fl.Tool == "" || !p.In(fl.Rect) {
		return ""
	}
	for _, it := range fl.Items {
		if p.In(it.Rect) {
			return it.ID
		}
	}
	if p.In(fl.Slider) {
		return "slider"
	}
	return "fly"
}

// SliderValue converts a pointer x position to a size (1..Max).
func (fl Flyout) SliderValue(x int) int {
	t := fl.track()
	v := 1 + int(math.Round(float64(x-t.Min.X)/float64(max(1, t.Dx()))*float64(fl.Max-1)))
	return max(1, min(fl.Max, v))
}

func (fl Flyout) track() image.Rectangle {
	k := fl.px(8)
	return image.Rect(fl.Slider.Min.X+k, fl.sizeY, fl.Slider.Max.X-k, fl.sizeY)
}

// Render draws the flyout on a transparent image covering Area().
func (fl Flyout) Render(th render.Theme, ink Ink, hover string) *image.RGBA {
	area := fl.Area()
	out := image.NewRGBA(area)
	f := func(v int) float64 { return float64(v) }
	r := fl.Rect
	rad := f(fl.px(8))
	render.Shadow(out, f(r.Min.X), f(r.Min.Y), f(r.Dx()), f(r.Dy()), rad, f(fl.px(12)), th.ShadowBase, 0.28)
	render.FillRoundRect(out, f(r.Min.X), f(r.Min.Y), f(r.Dx()), f(r.Dy()), rad, th.Border)
	render.FillRoundRect(out, f(r.Min.X)+1, f(r.Min.Y)+1, f(r.Dx())-2, f(r.Dy())-2, rad-1, th.Surface)
	pad := f(fl.px(12))
	render.Text(out, fl.title, f(r.Min.X)+pad, f(r.Min.Y)+pad+f(fl.px(10)), f(fl.px(14)), th.Text)

	var colors []color.RGBA
	var cur color.RGBA
	size := 0
	switch fl.Tool {
	case ToolPen:
		colors, cur, size = PenColors, ink.PenColor, ink.PenSize
	case ToolHighlight:
		colors, cur, size = HighlightColors, ink.HLColor, ink.HLSize
	case ToolShapes:
		colors, cur, size = ShapeColors, ink.ShapeColor, ink.ShapeSize
	}
	for _, it := range fl.Items {
		ir := it.Rect
		var idx int
		switch {
		case strings.HasPrefix(it.ID, "c:"):
			fmt.Sscanf(it.ID, "c:%d", &idx)
			c := colors[idx]
			d := f(fl.px(20))
			x, y := f(ir.Min.X)+(f(ir.Dx())-d)/2, f(ir.Min.Y)+(f(ir.Dy())-d)/2
			if c == cur {
				render.FillRoundRect(out, x-3, y-3, d+6, d+6, (d+6)/2, th.Accent)
				render.FillRoundRect(out, x-1, y-1, d+2, d+2, (d+2)/2, th.Surface)
			} else if it.ID == hover {
				render.FillRoundRect(out, x-3, y-3, d+6, d+6, (d+6)/2, th.Selected)
			}
			render.FillRoundRect(out, x, y, d, d, d/2, th.Border)
			render.FillRoundRect(out, x+1, y+1, d-2, d-2, d/2-1, c)
		case strings.HasPrefix(it.ID, "k:"):
			fmt.Sscanf(it.ID, "k:%d", &idx)
			if ShapeKinds[idx] == ink.ShapeKind {
				render.FillRoundRect(out, f(ir.Min.X), f(ir.Min.Y), f(ir.Dx()), f(ir.Dy()), f(fl.px(4)), th.Selected)
				render.FillRoundRect(out, f(ir.Min.X)+(f(ir.Dx())-f(fl.px(14)))/2, f(ir.Max.Y)-f(fl.px(5)), f(fl.px(14)), f(fl.px(3)), f(fl.px(1.5)), th.Accent)
			} else if it.ID == hover {
				render.FillRoundRect(out, f(ir.Min.X), f(ir.Min.Y), f(ir.Dx()), f(ir.Dy()), f(fl.px(4)), th.Selected)
			}
			icon := map[markup.Kind]string{markup.Line: render.IconLine, markup.Arrow: render.IconArrow, markup.Rectangle: render.IconRect, markup.Oval: render.IconOval}[ShapeKinds[idx]]
			isz := f(fl.px(20))
			render.DrawIcon(out, icon, f(ir.Min.X)+(f(ir.Dx())-isz)/2, f(ir.Min.Y)+(f(ir.Dy())-isz)/2, isz, th.Text)
		case strings.HasPrefix(it.ID, "e:"):
			fmt.Sscanf(it.ID, "e:%d", &idx)
			if idx == ink.Emoji || it.ID == hover {
				render.FillRoundRect(out, f(ir.Min.X)+1, f(ir.Min.Y)+1, f(ir.Dx())-2, f(ir.Dy())-2, f(fl.px(4)), th.Selected)
			}
			es := fl.px(24)
			if img := emoji.Scaled(idx, es); img != nil {
				p := image.Pt(ir.Min.X+(ir.Dx()-es)/2, ir.Min.Y+(ir.Dy()-es)/2)
				draw.Draw(out, image.Rectangle{Min: p, Max: p.Add(image.Pt(es, es))}, img, image.Point{}, draw.Over)
			}
		case it.ID == "eraseall":
			bg := th.Card
			if hover == it.ID {
				bg = th.Selected
			}
			render.FillRoundRect(out, f(ir.Min.X), f(ir.Min.Y), f(ir.Dx()), f(ir.Dy()), f(fl.px(4)), th.Border)
			render.FillRoundRect(out, f(ir.Min.X)+1, f(ir.Min.Y)+1, f(ir.Dx())-2, f(ir.Dy())-2, f(fl.px(4)), bg)
			isz := f(fl.px(16))
			render.DrawIcon(out, render.IconEraser, f(ir.Min.X+fl.px(10)), f(ir.Min.Y)+(f(ir.Dy())-isz)/2, isz, th.Text)
			render.Text(out, "Erase all ink", f(ir.Min.X+fl.px(34)), f(ir.Min.Y)+f(ir.Dy())/2, f(fl.px(13)), th.Text)
		}
	}
	if !fl.Slider.Empty() {
		label := "Size"
		if fl.Tool == ToolShapes {
			label = "Width"
		}
		cy := f(fl.sizeY)
		render.Text(out, label, f(fl.Slider.Min.X-fl.px(44)), cy, f(fl.px(13)), th.Text)
		t := fl.track()
		th2 := f(fl.px(4))
		render.FillRoundRect(out, f(t.Min.X), cy-th2/2, f(t.Dx()), th2, th2/2, th.Border)
		k := float64(size-1) / float64(max(1, fl.Max-1))
		kx := f(t.Min.X) + k*f(t.Dx())
		render.FillRoundRect(out, f(t.Min.X), cy-th2/2, kx-f(t.Min.X), th2, th2/2, th.Accent)
		d := f(fl.px(18))
		render.FillRoundRect(out, kx-d/2, cy-d/2, d, d, d/2, th.Border)
		render.FillRoundRect(out, kx-d/2+1, cy-d/2+1, d-2, d-2, d/2-1, th.Surface)
		render.FillRoundRect(out, kx-d/4, cy-d/4, d/2, d/2, d/4, th.Accent)
		render.Text(out, fmt.Sprint(size), f(fl.Slider.Max.X+fl.px(8)), cy, f(fl.px(13)), th.Text)
	}
	return out
}
