// SPDX-License-Identifier: Apache-2.0

//go:build windows

package mf

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"snippingtoolenhanced/internal/win"
)

// propVariant is a PROPVARIANT holding a VT_I8 (24 bytes on 64-bit Windows).
type propVariant struct {
	vt       uint16
	_        [3]uint16
	val      int64
	_padding uintptr
}

type reader struct{ *win.COM }

func openReader(path string) (reader, error) {
	a, err := newAttributes(1)
	if err != nil {
		return reader{}, err
	}
	defer a.Release()
	_ = a.setU32(&videoProc, 1)
	var r *win.COM
	p, _ := windows.UTF16PtrFromString(path)
	hr, _, _ := pMFCreateSourceReaderFromURL.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(a.COM)), uintptr(unsafe.Pointer(&r)))
	if err := win.HRErr("MFCreateSourceReaderFromURL", hr); err != nil {
		return reader{}, err
	}
	return reader{r}, nil
}

func (r reader) currentType(stream uint32) (attrs, error) {
	var t *win.COM
	err := r.HR("GetCurrentMediaType", srGetCurrentMediaType, uintptr(stream), uintptr(unsafe.Pointer(&t)))
	return attrs{t}, err
}

// Duration returns the length of a media file in 100 ns units.
func Duration(path string) (int64, error) {
	r, err := openReader(path)
	if err != nil {
		return 0, err
	}
	defer r.Release()
	var pv propVariant
	if err := r.HR("GetPresentationAttribute", srGetPresentationAttr, mediaSource, uintptr(unsafe.Pointer(&pdDuration)), uintptr(unsafe.Pointer(&pv))); err != nil {
		return 0, err
	}
	return pv.val, nil
}

// Trim re-encodes the part of src between start and end (100 ns units) into dst.
func Trim(src, dst string, start, end int64) error {
	r, err := openReader(src)
	if err != nil {
		return err
	}
	defer r.Release()
	r.Call(srSetStreamSelection, allStreams, 0)
	r.Call(srSetStreamSelection, firstVideoStream, 1)

	vt, err := newMediaType()
	if err != nil {
		return err
	}
	defer vt.Release()
	vt.setGUID(&mtMajorType, &typeVideo)
	vt.setGUID(&mtSubtype, &fmtRGB32)
	if err := r.HR("SetCurrentMediaType(video)", srSetCurrentMediaType, firstVideoStream, 0, uintptr(unsafe.Pointer(vt.COM))); err != nil {
		return err
	}
	hasAudio := false
	if r.Call(srSetStreamSelection, firstAudioStream, 1) == 0 {
		at, err := newMediaType()
		if err == nil {
			at.setGUID(&mtMajorType, &typeAudio)
			at.setGUID(&mtSubtype, &fmtPCM)
			at.setU32(&mtAudioBits, 16)
			at.setU32(&mtAudioRate, AudioRate)
			at.setU32(&mtAudioChannels, AudioChannels)
			hasAudio = int32(r.Call(srSetCurrentMediaType, firstAudioStream, 0, uintptr(unsafe.Pointer(at.COM)))) >= 0
			at.Release()
		}
		if !hasAudio {
			r.Call(srSetStreamSelection, firstAudioStream, 0)
		}
	}

	lay, fps, err := r.videoLayout()
	if err != nil {
		return err
	}
	ow, oh := lay.vw&^1, lay.vh&^1
	wr, err := NewWriter(dst, ow, oh, fps, hasAudio)
	if err != nil {
		return err
	}

	// Which stream index is video?
	videoIndex := uint32(0)
	if t, err := r.currentType(0); err == nil {
		if g, _ := t.getGUID(&mtMajorType); g != typeVideo {
			videoIndex = 1
		}
		t.Release()
	}

	pv := propVariant{vt: 20 /*VT_I8*/, val: start}
	var guidNull windows.GUID
	r.Call(srSetCurrentPosition, uintptr(unsafe.Pointer(&guidNull)), uintptr(unsafe.Pointer(&pv)))

	frame := make([]byte, ow*oh*4)
	doneVideo, doneAudio := false, !hasAudio
	for !(doneVideo && doneAudio) {
		var idx, flags uint32
		var ts int64
		var smp *win.COM
		if err := r.HR("ReadSample", srReadSample, allStreams, 0, uintptr(unsafe.Pointer(&idx)), uintptr(unsafe.Pointer(&flags)),
			uintptr(unsafe.Pointer(&ts)), uintptr(unsafe.Pointer(&smp))); err != nil {
			wr.Close()
			return err
		}
		isVideo := idx == videoIndex
		if isVideo && flags&flagTypeChanged != 0 {
			// The decoder settled on its real output format (row padding,
			// frame size): read it again before using this sample.
			if l, _, err := r.videoLayout(); err == nil {
				lay = l
			}
		}
		if flags&flagEndOfStream != 0 {
			if isVideo {
				doneVideo = true
			} else {
				doneAudio = true
			}
		}
		if smp == nil {
			continue
		}
		var dur int64
		smp.Call(smpGetSampleDuration, uintptr(unsafe.Pointer(&dur)))
		if ts >= end {
			if isVideo {
				doneVideo = true
			} else {
				doneAudio = true
			}
			smp.Release()
			continue
		}
		if ts+dur <= start {
			smp.Release()
			continue
		}
		t := ts - start
		if t < 0 {
			t = 0
		}
		if isVideo {
			ok, err := videoFrame(smp, frame, ow, oh, lay)
			smp.Release()
			if err != nil {
				wr.Close()
				return err
			}
			if !ok {
				continue
			}
			err = wr.WriteVideo(frame, t, dur)
		} else {
			data, err2 := sampleBytes(smp)
			smp.Release()
			if err2 != nil {
				wr.Close()
				return err2
			}
			err = wr.WriteAudio(data, t, dur)
		}
		if err != nil {
			wr.Close()
			return err
		}
	}
	return wr.Close()
}

var iid2DBuffer = win.MustGUID("7dc9d5f9-9ed9-44ec-9bbf-0600bb589fbb")

// IMF2DBuffer vtable indices (Windows SDK metadata).
const (
	buf2DLock2D   = 3
	buf2DUnlock2D = 4
)

// layout is how the decoder lays out a video frame.
type layout struct {
	w, h       int // decoded frame size
	vx, vy     int // visible area (display aperture) offset
	vw, vh     int // visible area size
	stride     int // MF_MT_DEFAULT_STRIDE, if reported
	haveStride bool
}

// videoLayout reads the current video output type.
func (r reader) videoLayout() (layout, int, error) {
	cur, err := r.currentType(firstVideoStream)
	if err != nil {
		return layout{}, 0, err
	}
	defer cur.Release()
	size, err := cur.getU64(&mtFrameSize)
	if err != nil {
		return layout{}, 0, err
	}
	l := layout{w: int(size >> 32), h: int(size & 0xffffffff)}
	l.vw, l.vh = l.w, l.h
	if s, err := cur.getU32(&mtDefaultStride); err == nil {
		l.stride, l.haveStride = int(int32(s)), true
	}
	var blob [16]byte
	if n, err := cur.getBlob(&mtMinAperture, blob[:]); err == nil && n >= 16 {
		if x, y, w, h, ok := aperture(blob[:]); ok && x+w <= l.w && y+h <= l.h {
			l.vx, l.vy, l.vw, l.vh = x, y, w, h
		}
	}
	fps := 30
	if rate, err := cur.getU64(&mtFrameRate); err == nil {
		if d := rate & 0xffffffff; d != 0 && rate>>32 != 0 {
			fps = int((rate >> 32) / d)
		}
	}
	return l, fps, nil
}

// videoFrame copies the visible ow×oh area of a decoded RGB32 sample into dst
// (top-down). It uses the decoder's real row pitch (IMF2DBuffer.Lock2D);
// decoders pad rows, so assuming width×4 bytes per row skews the picture.
func videoFrame(smp *win.COM, dst []byte, ow, oh int, l layout) (bool, error) {
	if ow > l.vw || oh > l.vh {
		return false, nil
	}
	var buf *win.COM
	if err := smp.HR("ConvertToContiguousBuffer", smpConvertToContiguousBuffer, uintptr(unsafe.Pointer(&buf))); err != nil {
		return false, err
	}
	defer buf.Release()
	var b2 *win.COM
	if buf.Call(0, uintptr(unsafe.Pointer(&iid2DBuffer)), uintptr(unsafe.Pointer(&b2))) == 0 && b2 != nil {
		defer b2.Release()
		var scan0 uintptr
		var pitch int32
		if b2.Call(buf2DLock2D, uintptr(unsafe.Pointer(&scan0)), uintptr(unsafe.Pointer(&pitch))) == 0 && scan0 != 0 {
			defer b2.Call(buf2DUnlock2D)
			ap := int(pitch)
			if ap < 0 {
				ap = -ap
			}
			if ap < l.w*4 {
				return false, nil
			}
			lo := scan0
			if pitch < 0 { // bottom-up: scan0 is the top row, stored last
				lo = scan0 - uintptr(ap*(l.h-1))
			}
			src := unsafe.Slice((*byte)(win.PtrFromUintptr(lo)), ap*(l.h-1)+l.w*4)
			return copyRegion(dst, ow, oh, src, int(pitch), l.h, l.vx, l.vy), nil
		}
	}
	// No 2D buffer: use the reported stride, or work it out from the size.
	var p uintptr
	var max, cur uint32
	if err := buf.HR("Lock", bufLock, uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&max)), uintptr(unsafe.Pointer(&cur))); err != nil {
		return false, err
	}
	defer buf.Call(bufUnlock)
	data := unsafe.Slice((*byte)(win.PtrFromUintptr(p)), cur)
	pitch := l.stride
	if !l.haveStride || pitch == 0 {
		pitch = -guessPitch(int(cur), l.w, l.h) // RGB without a stride attribute is bottom-up
	}
	ap := pitch
	if ap < 0 {
		ap = -ap
	}
	rows := int(cur) / ap
	if rows < l.h {
		return false, nil
	}
	if pitch < 0 {
		// bottom-up in a contiguous buffer: any height padding comes first
		return copyRegion(dst, ow, oh, data[(rows-l.h)*ap:], pitch, l.h, l.vx, l.vy), nil
	}
	return copyRegion(dst, ow, oh, data, pitch, l.h, l.vx, l.vy), nil
}

func sampleBytes(smp *win.COM) ([]byte, error) {
	var buf *win.COM
	if err := smp.HR("ConvertToContiguousBuffer", smpConvertToContiguousBuffer, uintptr(unsafe.Pointer(&buf))); err != nil {
		return nil, err
	}
	defer buf.Release()
	var p uintptr
	var max, cur uint32
	if err := buf.HR("Lock", bufLock, uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&max)), uintptr(unsafe.Pointer(&cur))); err != nil {
		return nil, err
	}
	defer buf.Call(bufUnlock)
	out := make([]byte, cur)
	copy(out, unsafe.Slice((*byte)(win.PtrFromUintptr(p)), cur))
	return out, nil
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

var _ = fmt.Sprintf
