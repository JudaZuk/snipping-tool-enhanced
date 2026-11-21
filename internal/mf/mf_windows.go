// SPDX-License-Identifier: Apache-2.0

//go:build windows

// Package mf wraps the parts of Windows Media Foundation used for screen
// recording (white paper §4): an H.264/AAC MP4 sink writer and a source
// reader for trimming. Vtable indices and GUIDs come from the Windows SDK
// metadata (Windows.Win32.winmd).
package mf

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"snippingtoolenhanced/internal/win"
)

var (
	mfplat      = windows.NewLazySystemDLL("mfplat.dll")
	mfreadwrite = windows.NewLazySystemDLL("mfreadwrite.dll")

	pMFStartup                   = mfplat.NewProc("MFStartup")
	pMFShutdown                  = mfplat.NewProc("MFShutdown")
	pMFCreateAttributes          = mfplat.NewProc("MFCreateAttributes")
	pMFCreateMediaType           = mfplat.NewProc("MFCreateMediaType")
	pMFCreateSample              = mfplat.NewProc("MFCreateSample")
	pMFCreateMemoryBuffer        = mfplat.NewProc("MFCreateMemoryBuffer")
	pMFCreateSinkWriterFromURL   = mfreadwrite.NewProc("MFCreateSinkWriterFromURL")
	pMFCreateSourceReaderFromURL = mfreadwrite.NewProc("MFCreateSourceReaderFromURL")
)

const mfVersion = 0x00020070 // MF_VERSION (MF_SDK_VERSION 2 << 16 | MF_API_VERSION 0x70)

// GUIDs (Windows.Win32.winmd).
var (
	mtMajorType       = win.MustGUID("48eba18e-f8c9-4687-bf11-0a74c9f96a8f")
	mtSubtype         = win.MustGUID("f7e34c9a-42e8-4714-b74b-cb29d72c35e5")
	mtAvgBitrate      = win.MustGUID("20332624-fb0d-4d9e-bd0d-cbf6786c102e")
	mtInterlaceMode   = win.MustGUID("e2724bb8-e676-4806-b4b2-a8d6efb44ccd")
	mtFrameSize       = win.MustGUID("1652c33d-d6b2-4012-b834-72030849a37d")
	mtFrameRate       = win.MustGUID("c459a2e8-3d2c-4e44-b132-fee5156c7bb0")
	mtPixelAspect     = win.MustGUID("c6376a1e-8d0a-4027-be45-6d9a0ad39bb6")
	mtDefaultStride   = win.MustGUID("644b4e48-1e02-4516-b0eb-c01ca9d49ac6")
	mtMinAperture     = win.MustGUID("d7388766-18fe-48c6-a177-ee894867c8c4") // MF_MT_MINIMUM_DISPLAY_APERTURE
	mtAudioChannels   = win.MustGUID("37e48bf5-645e-4c5b-89de-ada9e29b696a")
	mtAudioRate       = win.MustGUID("5faeeae7-0290-4c31-9e8a-c534f68d9dba")
	mtAudioBits       = win.MustGUID("f2deb57f-40fa-4764-aa33-ed4f2d1ff669")
	mtAudioAvgBytes   = win.MustGUID("1aab75c8-cfef-451c-ab95-ac034b8e1731")
	mtAudioBlockAlign = win.MustGUID("322de230-9eeb-43bd-ab7a-ff412251541d")

	typeVideo   = win.MustGUID("73646976-0000-0010-8000-00aa00389b71")
	typeAudio   = win.MustGUID("73647561-0000-0010-8000-00aa00389b71")
	fmtH264     = win.MustGUID("34363248-0000-0010-8000-00aa00389b71")
	fmtRGB32    = win.MustGUID("00000016-0000-0010-8000-00aa00389b71")
	fmtAAC      = win.MustGUID("00001610-0000-0010-8000-00aa00389b71")
	fmtPCM      = win.MustGUID("00000001-0000-0010-8000-00aa00389b71")
	hwTransform = win.MustGUID("a634a91c-822b-41b9-a494-4de4643612b0")
	container   = win.MustGUID("150ff23f-4abc-478b-ac4f-e1916fba1cca")
	mpeg4       = win.MustGUID("dc6cd05d-b9d0-40ef-bd35-fa622c1ab28a")
	videoProc   = win.MustGUID("fb394f3d-ccf1-42ee-bbb3-f9b845d5681d")
	pdDuration  = win.MustGUID("6c990d33-bb8e-477a-8598-0d5d96fcd88a")
)

// IMFAttributes / IMFMediaType vtable indices.
const (
	attrGetUINT32 = 7
	attrGetUINT64 = 8
	attrGetGUID   = 10
	attrGetBlob   = 15
	attrSetUINT32 = 21
	attrSetUINT64 = 22
	attrSetGUID   = 24
)

// IMFSinkWriter.
const (
	swAddStream         = 3
	swSetInputMediaType = 4
	swBeginWriting      = 5
	swWriteSample       = 6
	swFinalize          = 11
)

// IMFSample (after the 30 IMFAttributes methods).
const (
	smpSetSampleTime             = 36
	smpGetSampleDuration         = 37
	smpSetSampleDuration         = 38
	smpConvertToContiguousBuffer = 41
	smpAddBuffer                 = 42
)

// IMFMediaBuffer.
const (
	bufLock             = 3
	bufUnlock           = 4
	bufGetCurrentLength = 5
	bufSetCurrentLength = 6
)

// IMFSourceReader.
const (
	srSetStreamSelection  = 4
	srGetCurrentMediaType = 6
	srSetCurrentMediaType = 7
	srSetCurrentPosition  = 8
	srReadSample          = 9
	srGetPresentationAttr = 12
)

const (
	firstVideoStream = 0xFFFFFFFC // MF_SOURCE_READER_FIRST_VIDEO_STREAM
	firstAudioStream = 0xFFFFFFFD // MF_SOURCE_READER_FIRST_AUDIO_STREAM
	allStreams       = 0xFFFFFFFE // MF_SOURCE_READER_ALL_STREAMS / ANY_STREAM
	mediaSource      = 0xFFFFFFFF // MF_SOURCE_READER_MEDIASOURCE
	flagEndOfStream  = 0x2
	flagTypeChanged  = 0x20 // MF_SOURCE_READERF_CURRENTMEDIATYPECHANGED
)

// Startup initializes COM (multithreaded) and Media Foundation for the
// calling OS thread. Call the returned function when done.
func Startup() (func(), error) {
	// An error here means COM is already initialized on this thread, which is fine.
	_ = windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED)
	r, _, _ := pMFStartup.Call(mfVersion, 0)
	if err := win.HRErr("MFStartup", r); err != nil {
		return nil, err
	}
	return func() { pMFShutdown.Call() }, nil
}

type attrs struct{ *win.COM }

func newAttributes(n uint32) (attrs, error) {
	var p *win.COM
	r, _, _ := pMFCreateAttributes.Call(uintptr(unsafe.Pointer(&p)), uintptr(n))
	return attrs{p}, win.HRErr("MFCreateAttributes", r)
}

func newMediaType() (attrs, error) {
	var p *win.COM
	r, _, _ := pMFCreateMediaType.Call(uintptr(unsafe.Pointer(&p)))
	return attrs{p}, win.HRErr("MFCreateMediaType", r)
}

func (a attrs) setGUID(k, v *windows.GUID) error {
	return a.HR("SetGUID", attrSetGUID, uintptr(unsafe.Pointer(k)), uintptr(unsafe.Pointer(v)))
}
func (a attrs) setU32(k *windows.GUID, v uint32) error {
	return a.HR("SetUINT32", attrSetUINT32, uintptr(unsafe.Pointer(k)), uintptr(v))
}
func (a attrs) setU64(k *windows.GUID, v uint64) error {
	return a.HR("SetUINT64", attrSetUINT64, uintptr(unsafe.Pointer(k)), uintptr(v))
}
func (a attrs) getU32(k *windows.GUID) (uint32, error) {
	var v uint32
	err := a.HR("GetUINT32", attrGetUINT32, uintptr(unsafe.Pointer(k)), uintptr(unsafe.Pointer(&v)))
	return v, err
}
func (a attrs) getU64(k *windows.GUID) (uint64, error) {
	var v uint64
	err := a.HR("GetUINT64", attrGetUINT64, uintptr(unsafe.Pointer(k)), uintptr(unsafe.Pointer(&v)))
	return v, err
}
func (a attrs) getGUID(k *windows.GUID) (windows.GUID, error) {
	var v windows.GUID
	err := a.HR("GetGUID", attrGetGUID, uintptr(unsafe.Pointer(k)), uintptr(unsafe.Pointer(&v)))
	return v, err
}

func (a attrs) getBlob(k *windows.GUID, buf []byte) (int, error) {
	var n uint32
	err := a.HR("GetBlob", attrGetBlob, uintptr(unsafe.Pointer(k)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&n)))
	return int(n), err
}

func pack(hi, lo uint32) uint64 { return uint64(hi)<<32 | uint64(lo) }

// ---------------------------------------------------------------- writer

// Writer encodes RGB32 frames to H.264 and 16-bit PCM to AAC in an MP4 file.
type Writer struct {
	sw           *win.COM
	video, audio uint32
	hasAudio     bool
	W, H         int
}

// Audio input format of the writer.
const (
	AudioRate     = 48000
	AudioChannels = 2
)

// NewWriter creates path. width and height must be even.
func NewWriter(path string, width, height, fps int, withAudio bool) (*Writer, error) {
	if width%2 != 0 || height%2 != 0 || width < 2 || height < 2 {
		return nil, fmt.Errorf("mf: frame size %dx%d must be even", width, height)
	}
	a, err := newAttributes(2)
	if err != nil {
		return nil, err
	}
	defer a.Release()
	_ = a.setU32(&hwTransform, 1)
	_ = a.setGUID(&container, &mpeg4)

	var sw *win.COM
	p, _ := windows.UTF16PtrFromString(path)
	r, _, _ := pMFCreateSinkWriterFromURL.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(a.COM)), uintptr(unsafe.Pointer(&sw)))
	if err := win.HRErr("MFCreateSinkWriterFromURL", r); err != nil {
		return nil, err
	}
	w := &Writer{sw: sw, W: width, H: height}
	fail := func(e error) (*Writer, error) { sw.Release(); return nil, e }

	// Bitrate ~ 0.12 bits per pixel per frame, at least 2 Mbit/s.
	bitrate := uint32(float64(width*height*fps) * 0.12)
	if bitrate < 2_000_000 {
		bitrate = 2_000_000
	}
	out, err := newMediaType()
	if err != nil {
		return fail(err)
	}
	defer out.Release()
	for _, e := range []error{
		out.setGUID(&mtMajorType, &typeVideo), out.setGUID(&mtSubtype, &fmtH264), out.setU32(&mtAvgBitrate, bitrate),
		out.setU32(&mtInterlaceMode, 2), out.setU64(&mtFrameSize, pack(uint32(width), uint32(height))),
		out.setU64(&mtFrameRate, pack(uint32(fps), 1)), out.setU64(&mtPixelAspect, pack(1, 1)),
	} {
		if e != nil {
			return fail(e)
		}
	}
	if err := sw.HR("AddStream(video)", swAddStream, uintptr(unsafe.Pointer(out.COM)), uintptr(unsafe.Pointer(&w.video))); err != nil {
		return fail(err)
	}
	in, err := newMediaType()
	if err != nil {
		return fail(err)
	}
	defer in.Release()
	for _, e := range []error{
		in.setGUID(&mtMajorType, &typeVideo), in.setGUID(&mtSubtype, &fmtRGB32), in.setU32(&mtInterlaceMode, 2),
		in.setU64(&mtFrameSize, pack(uint32(width), uint32(height))), in.setU64(&mtFrameRate, pack(uint32(fps), 1)),
		in.setU64(&mtPixelAspect, pack(1, 1)), in.setU32(&mtDefaultStride, uint32(width*4)), // positive stride = top-down rows
	} {
		if e != nil {
			return fail(e)
		}
	}
	if err := sw.HR("SetInputMediaType(video)", swSetInputMediaType, uintptr(w.video), uintptr(unsafe.Pointer(in.COM)), 0); err != nil {
		return fail(err)
	}

	if withAudio {
		if err := w.addAudio(); err == nil {
			w.hasAudio = true
		}
	}
	if err := sw.HR("BeginWriting", swBeginWriting); err != nil {
		return fail(err)
	}
	return w, nil
}

func (w *Writer) addAudio() error {
	out, err := newMediaType()
	if err != nil {
		return err
	}
	defer out.Release()
	for _, e := range []error{
		out.setGUID(&mtMajorType, &typeAudio), out.setGUID(&mtSubtype, &fmtAAC), out.setU32(&mtAudioBits, 16),
		out.setU32(&mtAudioRate, AudioRate), out.setU32(&mtAudioChannels, AudioChannels), out.setU32(&mtAudioAvgBytes, 24000),
	} {
		if e != nil {
			return e
		}
	}
	if err := w.sw.HR("AddStream(audio)", swAddStream, uintptr(unsafe.Pointer(out.COM)), uintptr(unsafe.Pointer(&w.audio))); err != nil {
		return err
	}
	in, err := newMediaType()
	if err != nil {
		return err
	}
	defer in.Release()
	for _, e := range []error{
		in.setGUID(&mtMajorType, &typeAudio), in.setGUID(&mtSubtype, &fmtPCM), in.setU32(&mtAudioBits, 16),
		in.setU32(&mtAudioRate, AudioRate), in.setU32(&mtAudioChannels, AudioChannels),
		in.setU32(&mtAudioBlockAlign, 2*AudioChannels), in.setU32(&mtAudioAvgBytes, AudioRate*2*AudioChannels),
	} {
		if e != nil {
			return e
		}
	}
	return w.sw.HR("SetInputMediaType(audio)", swSetInputMediaType, uintptr(w.audio), uintptr(unsafe.Pointer(in.COM)), 0)
}

// HasAudio reports whether the file has an audio track.
func (w *Writer) HasAudio() bool { return w.hasAudio }

func (w *Writer) write(stream uint32, data []byte, t, dur int64) error {
	var buf *win.COM
	r, _, _ := pMFCreateMemoryBuffer.Call(uintptr(len(data)), uintptr(unsafe.Pointer(&buf)))
	if err := win.HRErr("MFCreateMemoryBuffer", r); err != nil {
		return err
	}
	defer buf.Release()
	var p uintptr
	if err := buf.HR("Lock", bufLock, uintptr(unsafe.Pointer(&p)), 0, 0); err != nil {
		return err
	}
	copy(unsafe.Slice((*byte)(win.PtrFromUintptr(p)), len(data)), data)
	buf.Call(bufUnlock)
	buf.Call(bufSetCurrentLength, uintptr(len(data)))
	var smp *win.COM
	r, _, _ = pMFCreateSample.Call(uintptr(unsafe.Pointer(&smp)))
	if err := win.HRErr("MFCreateSample", r); err != nil {
		return err
	}
	defer smp.Release()
	smp.Call(smpAddBuffer, uintptr(unsafe.Pointer(buf)))
	smp.Call(smpSetSampleTime, uintptr(t))
	smp.Call(smpSetSampleDuration, uintptr(dur))
	return w.sw.HR("WriteSample", swWriteSample, uintptr(stream), uintptr(unsafe.Pointer(smp)))
}

// WriteVideo writes one top-down BGRA frame (W*H*4 bytes). Times are in 100 ns units.
func (w *Writer) WriteVideo(bgra []byte, t, dur int64) error { return w.write(w.video, bgra, t, dur) }

// WriteAudio writes interleaved 16-bit stereo PCM at 48 kHz.
func (w *Writer) WriteAudio(pcm []byte, t, dur int64) error {
	if !w.hasAudio || len(pcm) == 0 {
		return nil
	}
	return w.write(w.audio, pcm, t, dur)
}

// Close finalizes the file.
func (w *Writer) Close() error {
	if w.sw == nil {
		return nil
	}
	err := w.sw.HR("Finalize", swFinalize)
	w.sw.Release()
	w.sw = nil
	return err
}
