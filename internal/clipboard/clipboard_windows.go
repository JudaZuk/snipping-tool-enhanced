// SPDX-License-Identifier: Apache-2.0

//go:build windows

// Package clipboard puts images and text on the Windows clipboard (§8).
package clipboard

import (
	"encoding/binary"
	"errors"
	"image"
	"image/draw"
	"sync"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"

	"snippingtoolenhanced/internal/imaging"
	"snippingtoolenhanced/internal/win"
)

// owner is a message-only window that owns our clipboard data. OpenClipboard
// with a NULL owner would make SetClipboardData fail after EmptyClipboard.
var (
	ownerOnce sync.Once
	owner     windows.HWND
)

func open() error {
	ownerOnce.Do(func() { owner, _ = win.CreateMessageWindow() })
	for i := 0; i < 20; i++ {
		if win.OpenClipboard(owner) {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return errors.New("clipboard is busy")
}

// SetImage places img as CF_DIB (for every app) and PNG (keeps transparency,
// used by browsers, Office and chat apps).
func SetImage(img image.Image) error {
	pngBytes, err := imaging.Encode(img, "png", 0)
	if err != nil {
		return err
	}
	dib := DIB(toNRGBA(img))
	if err := open(); err != nil {
		return err
	}
	defer win.CloseClipboard()
	win.EmptyClipboard()
	if err := win.SetClipboardBytes(win.CF_DIB, dib); err != nil {
		return err
	}
	if f := win.RegisterClipboardFormat("PNG"); f != 0 {
		_ = win.SetClipboardBytes(f, pngBytes)
	}
	return nil
}

// SetText places Unicode text on the clipboard.
func SetText(s string) error {
	u := utf16.Encode([]rune(s + "\x00"))
	b := make([]byte, len(u)*2)
	for i, c := range u {
		binary.LittleEndian.PutUint16(b[i*2:], c)
	}
	if err := open(); err != nil {
		return err
	}
	defer win.CloseClipboard()
	win.EmptyClipboard()
	return win.SetClipboardBytes(win.CF_UNICODETEXT, b)
}

func toNRGBA(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n
	}
	b := img.Bounds()
	n := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(n, n.Bounds(), img, b.Min, draw.Src)
	return n
}

// DIB encodes img as a packed bottom-up 32-bpp BITMAPINFOHEADER + pixels.
func DIB(img *image.NRGBA) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]byte, 40+w*h*4)
	le := binary.LittleEndian
	le.PutUint32(out[0:], 40)
	le.PutUint32(out[4:], uint32(w))
	le.PutUint32(out[8:], uint32(h)) // positive = bottom-up
	le.PutUint16(out[12:], 1)
	le.PutUint16(out[14:], 32)
	le.PutUint32(out[16:], 0) // BI_RGB
	le.PutUint32(out[20:], uint32(w*h*4))
	px := out[40:]
	for y := 0; y < h; y++ {
		src := img.Pix[y*img.Stride:]
		dst := px[(h-1-y)*w*4:]
		for x := 0; x < w; x++ {
			i := x * 4
			dst[i], dst[i+1], dst[i+2], dst[i+3] = src[i+2], src[i+1], src[i], src[i+3]
		}
	}
	return out
}

// SetGIF puts an animated GIF on the clipboard as a file (CF_HDROP, which
// chat and mail apps accept) plus the registered "GIF" format.
func SetGIF(path string, data []byte) error {
	// DROPFILES header (20 bytes) followed by a double-NUL-terminated UTF-16 list.
	u := utf16.Encode([]rune(path + "\x00\x00"))
	b := make([]byte, 20+len(u)*2)
	binary.LittleEndian.PutUint32(b[0:], 20) // pFiles
	binary.LittleEndian.PutUint32(b[16:], 1) // fWide
	for i, c := range u {
		binary.LittleEndian.PutUint16(b[20+i*2:], c)
	}
	if err := open(); err != nil {
		return err
	}
	defer win.CloseClipboard()
	win.EmptyClipboard()
	if err := win.SetClipboardBytes(15 /*CF_HDROP*/, b); err != nil {
		return err
	}
	if f := win.RegisterClipboardFormat("GIF"); f != 0 {
		_ = win.SetClipboardBytes(f, data)
	}
	return nil
}
