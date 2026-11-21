// SPDX-License-Identifier: Apache-2.0

// Package emoji holds the emoji offered by Quick markup. The pictures are
// Microsoft's Fluent Emoji (Color style, MIT license, see png/LICENSE-fluentui-emoji.txt),
// rendered to 128×128 PNG. The list matches the editor's emoji picker.
package emoji

import (
	"bytes"
	"embed"
	"image"
	"image/draw"
	"image/png"
	"sync"

	xdraw "golang.org/x/image/draw"
)

//go:embed png/*.png
var files embed.FS

// Emoji is one entry of the picker.
type Emoji struct {
	Char string // the emoji as text
	File string // code points, e.g. "1f600"
}

// List is the picker order.
var List = []Emoji{
	{"😀", "1f600"},
	{"😂", "1f602"},
	{"😊", "1f60a"},
	{"😍", "1f60d"},
	{"🤔", "1f914"},
	{"😮", "1f62e"},
	{"😢", "1f622"},
	{"😡", "1f621"},
	{"👍", "1f44d"},
	{"👎", "1f44e"},
	{"👏", "1f44f"},
	{"🙏", "1f64f"},
	{"💪", "1f4aa"},
	{"👀", "1f440"},
	{"👉", "1f449"},
	{"👈", "1f448"},
	{"👆", "1f446"},
	{"👇", "1f447"},
	{"✅", "2705"},
	{"❌", "274c"},
	{"⚠️", "26a0"},
	{"❗", "2757"},
	{"❓", "2753"},
	{"💡", "1f4a1"},
	{"⭐", "2b50"},
	{"🔥", "1f525"},
	{"🎉", "1f389"},
	{"❤️", "2764"},
	{"💯", "1f4af"},
	{"📌", "1f4cc"},
	{"📎", "1f4ce"},
	{"🔍", "1f50d"},
	{"🖱️", "1f5b1"},
	{"⌨️", "2328"},
	{"💻", "1f4bb"},
	{"📷", "1f4f7"},
	{"📝", "1f4dd"},
	{"📅", "1f4c5"},
	{"⏰", "23f0"},
	{"🚀", "1f680"},
	{"🐛", "1f41b"},
	{"🔒", "1f512"},
	{"🔑", "1f511"},
	{"📧", "1f4e7"},
	{"☎️", "260e"},
	{"🏠", "1f3e0"},
	{"➡️", "27a1"},
	{"⬅️", "2b05"},
}

var (
	mu     sync.Mutex
	full   = map[int]*image.NRGBA{}
	scaled = map[[2]int]*image.NRGBA{}
)

// Image returns emoji i at its full 128×128 size, or nil.
func Image(i int) *image.NRGBA {
	if i < 0 || i >= len(List) {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	return load(i)
}

func load(i int) *image.NRGBA {
	if img, ok := full[i]; ok {
		return img
	}
	data, err := files.ReadFile("png/" + List[i].File + ".png")
	if err != nil {
		return nil
	}
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	img := image.NewNRGBA(src.Bounds())
	draw.Draw(img, img.Bounds(), src, src.Bounds().Min, draw.Src)
	full[i] = img
	return img
}

// Scaled returns emoji i resized to size×size pixels (cached), or nil.
func Scaled(i, size int) *image.NRGBA {
	if i < 0 || i >= len(List) || size <= 0 {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	k := [2]int{i, size}
	if img, ok := scaled[k]; ok {
		return img
	}
	src := load(i)
	if src == nil {
		return nil
	}
	if len(scaled) > 256 {
		scaled = map[[2]int]*image.NRGBA{}
	}
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)
	scaled[k] = dst
	return dst
}
