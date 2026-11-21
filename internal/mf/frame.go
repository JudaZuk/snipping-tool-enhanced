// SPDX-License-Identifier: Apache-2.0

package mf

// Decoded video frames often have padded rows: an 898-pixel-wide frame may be
// stored with room for 912 pixels per row, and RGB frames may be stored
// bottom-up. These helpers copy such frames into a tight top-down buffer.

// copyFrame copies oh rows of ow BGRA pixels from src into dst (ow*oh*4 bytes).
// pitch is the distance in bytes from one row to the next; negative means the
// image is stored bottom-up, in which case src starts at the last row.
func copyFrame(dst []byte, ow, oh int, src []byte, pitch int) bool {
	return copyRegion(dst, ow, oh, src, pitch, oh, 0, 0)
}

// copyRegion copies the ow×oh area at (ox, oy) of a decoded frame into dst
// (top-down, ow*4 bytes per row). src holds rows stored rows of |pitch| bytes;
// a negative pitch means they are stored bottom-up (src starts at the last
// row). The offset is the display aperture: decoders pad H.264 frames to a
// multiple of 16 rows, so the visible picture can sit inside a larger frame.
func copyRegion(dst []byte, ow, oh int, src []byte, pitch, rows, ox, oy int) bool {
	rowBytes := ow * 4
	ap := pitch
	if ap < 0 {
		ap = -ap
	}
	if ow <= 0 || oh <= 0 || ox < 0 || oy < 0 || oy+oh > rows || ox*4+rowBytes > ap ||
		len(dst) < rowBytes*oh || len(src) < ap*(rows-1)+ox*4+rowBytes {
		return false
	}
	for y := 0; y < oh; y++ {
		r := oy + y
		if pitch < 0 {
			r = rows - 1 - r
		}
		off := r*ap + ox*4
		copy(dst[y*rowBytes:(y+1)*rowBytes], src[off:off+rowBytes])
	}
	return true
}

// aperture decodes an MFVideoArea blob (MF_MT_MINIMUM_DISPLAY_APERTURE):
// OffsetX {fract uint16, value int16}, OffsetY {…}, Area {cx int32, cy int32}.
func aperture(b []byte) (x, y, w, h int, ok bool) {
	if len(b) < 16 {
		return 0, 0, 0, 0, false
	}
	i16 := func(o int) int { return int(int16(uint16(b[o]) | uint16(b[o+1])<<8)) }
	i32 := func(o int) int {
		return int(int32(uint32(b[o]) | uint32(b[o+1])<<8 | uint32(b[o+2])<<16 | uint32(b[o+3])<<24))
	}
	x, y, w, h = i16(2), i16(6), i32(8), i32(12)
	return x, y, w, h, x >= 0 && y >= 0 && w > 0 && h > 0
}

// guessPitch finds the row length of a contiguous frame buffer of n bytes for
// a w×h image when the decoder does not report it: decoders pad rows to a
// multiple of 16, 32 or 64 pixels, and may pad the height to a multiple of 16.
func guessPitch(n, w, h int) int {
	for _, align := range []int{1, 16, 32, 64, 128} {
		pw := (w + align - 1) / align * align
		pitch := pw * 4
		for _, ph := range []int{h, (h + 15) / 16 * 16} {
			if pitch*ph == n {
				return pitch
			}
		}
	}
	if h > 0 && n%h == 0 && n/h >= w*4 {
		return n / h
	}
	return w * 4
}
