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

	cur, err := r.currentType(firstVideoStream)
	if err != nil {
		return err
	}
	size, _ := cur.getU64(&mtFrameSize)
	rate, _ := cur.getU64(&mtFrameRate)
	stride32, strideErr := cur.getU32(&mtDefaultStride)
	cur.Release()
	w, h := int(size>>32), int(size&0xffffffff)
	fps := 30
	if d := rate & 0xffffffff; d != 0 && rate>>32 != 0 {
		fps = int((rate >> 32) / d)
	}
	stride := -w * 4 // RGB without an explicit stride is bottom-up
	if strideErr == nil {
		stride = int(int32(stride32))
	}
	ow, oh := w&^1, h&^1
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
		data, err := sampleBytes(smp)
		smp.Release()
		if err != nil {
			wr.Close()
			return err
		}
		t := ts - start
		if t < 0 {
			t = 0
		}
		if isVideo {
			if len(data) < abs(stride)*h {
				continue
			}
			for y := 0; y < oh; y++ {
				row := y
				if stride < 0 {
					row = h - 1 - y
				}
				copy(frame[y*ow*4:(y+1)*ow*4], data[row*abs(stride):row*abs(stride)+ow*4])
			}
			err = wr.WriteVideo(frame, t, dur)
		} else {
			err = wr.WriteAudio(data, t, dur)
		}
		if err != nil {
			wr.Close()
			return err
		}
	}
	return wr.Close()
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
