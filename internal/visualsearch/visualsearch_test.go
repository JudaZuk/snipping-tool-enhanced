// SPDX-License-Identifier: Apache-2.0

package visualsearch

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"
)

func TestNormalizeAndName(t *testing.T) {
	if Normalize("google") != Google || Normalize("") != Bing || Normalize("yahoo") != Bing {
		t.Fatal("normalize")
	}
	if Name("google") != "Google Lens" || Name("x") != "Bing" {
		t.Fatal("name")
	}
}

func TestEncodeFlattensAndScales(t *testing.T) {
	img := image.NewNRGBA(image.Rect(10, 10, 6010, 1010)) // transparent, 6000×1000
	img.SetNRGBA(10, 10, color.NRGBA{255, 0, 0, 255})
	b, err := Encode(img)
	if err != nil {
		t.Fatal(err)
	}
	out, err := jpeg.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if out.Bounds().Dx() != MaxSide || out.Bounds().Dy() != 500 {
		t.Fatalf("size %v", out.Bounds())
	}
	if r, g, bl, _ := out.At(1500, 250).RGBA(); r>>8 < 245 || g>>8 < 245 || bl>>8 < 245 {
		t.Fatal("transparent must become white")
	}
	if _, err := Encode(image.NewRGBA(image.Rect(0, 0, 0, 0))); err == nil {
		t.Fatal("empty")
	}
}

func TestPages(t *testing.T) {
	jpg := []byte{0xff, 0xd8, 1, 2, 3}
	b64 := base64.StdEncoding.EncodeToString(jpg)
	bing := string(Page(Bing, jpg))
	if !strings.Contains(bing, `name="imageBin"`) || !strings.Contains(bing, "www.bing.com/images/search?view=detailv2&amp;iss=sbiupload") || !strings.Contains(bing, b64) {
		t.Fatal("bing page")
	}
	g := string(Page(Google, jpg))
	if !strings.Contains(g, `name="encoded_image"`) || !strings.Contains(g, "lens.google.com/v3/upload") || !strings.Contains(g, "DataTransfer") {
		t.Fatal("google page")
	}
}
