// SPDX-License-Identifier: Apache-2.0

// Package visualsearch sends a snip to a web image search (white paper §6:
// "Visual Search with Bing … sends the image off-device for a web search and
// does not copy or auto-save it"). It writes a small local page that uploads
// the image to the chosen engine with an ordinary form post, the way the
// engines' own upload forms do, and opens it in the default browser. No
// account, key or third-party image host is involved.
package visualsearch

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"

	xdraw "golang.org/x/image/draw"
)

// Engine IDs (settings.VisualSearch).
const (
	Bing   = "bing"
	Google = "google"
)

// Engine is a search engine that accepts an uploaded image.
type Engine struct {
	ID   string
	Name string // shown in the tooltip and settings
}

// Engines lists the choices, Bing first (the Windows 11 default).
var Engines = []Engine{{Bing, "Bing"}, {Google, "Google Lens"}}

// Normalize returns a known engine ID (Bing for anything else).
func Normalize(id string) string {
	for _, e := range Engines {
		if e.ID == id {
			return id
		}
	}
	return Bing
}

// Name is the engine's display name.
func Name(id string) string {
	id = Normalize(id)
	for _, e := range Engines {
		if e.ID == id {
			return e.Name
		}
	}
	return ""
}

// MaxSide limits the uploaded picture; larger snips are scaled down.
const MaxSide = 3000

// Encode prepares img for upload: transparent parts (freeform snips) become
// white, large pictures are scaled down, and the result is a JPEG.
func Encode(img image.Image) ([]byte, error) {
	b := img.Bounds()
	if b.Empty() {
		return nil, fmt.Errorf("the snip is empty")
	}
	w, h := b.Dx(), b.Dy()
	if s := max(w, h); s > MaxSide {
		w, h = max(1, w*MaxSide/s), max(1, h*MaxSide/s)
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	if w == b.Dx() && h == b.Dy() {
		draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Over)
	} else {
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Over, nil)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Upload targets.
const (
	// Bing's image upload: the picture as base64 in the "imageBin" field.
	BingAction = "https://www.bing.com/images/search?view=detailv2&iss=sbiupload&FORM=SBIHMP&sbifnm=snip.jpg"
	// Google Lens upload: the picture as a file in the "encoded_image" field.
	GoogleAction = "https://lens.google.com/v3/upload"
)

// Page returns the local HTML page that uploads the JPEG data to engine id.
func Page(id string, jpg []byte) []byte {
	id = Normalize(id)
	data := base64.StdEncoding.EncodeToString(jpg)
	action, field, fill := BingAction, `<input type="hidden" name="imageBin" id="field">`, `document.getElementById("field").value = data;`
	if id == Google {
		action, field = GoogleAction, `<input type="file" name="encoded_image" id="field" accept="image/*" hidden>`
		fill = `const bin = atob(data), bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  const dt = new DataTransfer();
  dt.items.add(new File([bytes], "snip.jpg", { type: "image/jpeg" }));
  document.getElementById("field").files = dt.files;`
	}
	name := html.EscapeString(Name(id))
	return []byte(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="referrer" content="no-referrer">
<title>Visual Search with ` + name + `</title>
<style>body{font:15px "Segoe UI",system-ui,sans-serif;margin:48px;color:#1b1b1b;background:#f3f3f3}@media(prefers-color-scheme:dark){body{color:#fff;background:#202020}}</style>
</head>
<body>
<p id="msg">Searching with ` + name + `…</p>
<form id="form" method="post" enctype="multipart/form-data" action="` + html.EscapeString(action) + `">` + field + `</form>
<script>
const data = "` + data + `";
try {
  ` + fill + `
  document.getElementById("form").submit();
} catch (e) {
  document.getElementById("msg").textContent = "The search could not be started in this browser: " + e.message;
}
</script>
</body>
</html>
`)
}
