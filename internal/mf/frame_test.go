// SPDX-License-Identifier: Apache-2.0

package mf

import "testing"

// frame builds a w×h test image with rows padded to pitch bytes; pixel (x,y)
// has bytes {x, y, 7, 255}.
func frame(w, h, pitch int, bottomUp bool) []byte {
	ap := pitch
	b := make([]byte, ap*h)
	for y := 0; y < h; y++ {
		row := y
		if bottomUp {
			row = h - 1 - y
		}
		for x := 0; x < w; x++ {
			o := row*ap + x*4
			b[o], b[o+1], b[o+2], b[o+3] = byte(x), byte(y), 7, 255
		}
	}
	return b
}

func check(t *testing.T, dst []byte, w, h int) {
	t.Helper()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := (y*w + x) * 4
			if dst[o] != byte(x) || dst[o+1] != byte(y) {
				t.Fatalf("pixel %d,%d = %v", x, y, dst[o:o+4])
			}
		}
	}
}

func TestCopyFramePaddedRows(t *testing.T) {
	// The 1.3.0 bug: an 898-pixel-wide frame decoded with 912-pixel rows.
	w, h, pitch := 898, 20, 912*4
	src := frame(w, h, pitch, false)
	dst := make([]byte, w*h*4)
	if !copyFrame(dst, w, h, src, pitch) {
		t.Fatal("copy failed")
	}
	check(t, dst, w, h)
}

func TestCopyFrameBottomUp(t *testing.T) {
	w, h, pitch := 30, 10, 32*4
	src := frame(w, h, pitch, true)
	dst := make([]byte, w*h*4)
	if !copyFrame(dst, w, h, src, -pitch) {
		t.Fatal("copy failed")
	}
	check(t, dst, w, h)
}

func TestCopyFrameRejectsShortBuffers(t *testing.T) {
	if copyFrame(make([]byte, 40), 10, 2, make([]byte, 50), 40) {
		t.Fatal("must reject a buffer that is too small")
	}
}

func TestGuessPitch(t *testing.T) {
	cases := []struct{ n, w, h, want int }{
		{912 * 4 * 354, 898, 354, 912 * 4}, // rows padded to 16
		{912 * 4 * 368, 898, 354, 912 * 4}, // rows and height padded to 16
		{898 * 4 * 354, 898, 354, 898 * 4}, // tight
		{960 * 4 * 354, 898, 354, 960 * 4}, // padded to 64
		{1920 * 4 * 1088, 1920, 1080, 1920 * 4},
	}
	for _, c := range cases {
		if got := guessPitch(c.n, c.w, c.h); got != c.want {
			t.Errorf("guessPitch(%d, %d, %d) = %d, want %d", c.n, c.w, c.h, got, c.want)
		}
	}
}

func TestCopyRegionAperture(t *testing.T) {
	// 898×354 visible inside a decoded 912×368 frame, top-down and bottom-up.
	for _, bottomUp := range []bool{false, true} {
		w, h, pitch := 912, 368, 912*4
		src := frame(w, h, pitch, bottomUp)
		p := pitch
		if bottomUp {
			p = -pitch
		}
		dst := make([]byte, 898*354*4)
		if !copyRegion(dst, 898, 354, src, p, h, 0, 0) {
			t.Fatal("copy failed")
		}
		check(t, dst, 898, 354)
		// With an offset the first copied pixel is (ox, oy).
		dst2 := make([]byte, 4*3*4)
		if !copyRegion(dst2, 4, 3, src, p, h, 5, 7) || dst2[0] != 5 || dst2[1] != 7 {
			t.Fatalf("offset copy: %v", dst2[:4])
		}
	}
	if copyRegion(make([]byte, 64), 4, 4, make([]byte, 64), 16, 4, 1, 0) {
		t.Fatal("area past the row end must be rejected")
	}
}

func TestAperture(t *testing.T) {
	b := []byte{0, 0, 2, 0, 0, 0, 3, 0, 0x82, 3, 0, 0, 0x62, 1, 0, 0}
	x, y, w, h, ok := aperture(b)
	if !ok || x != 2 || y != 3 || w != 898 || h != 354 {
		t.Fatalf("got %d %d %d %d %v", x, y, w, h, ok)
	}
	if _, _, _, _, ok := aperture(b[:8]); ok {
		t.Fatal("short blob")
	}
}
