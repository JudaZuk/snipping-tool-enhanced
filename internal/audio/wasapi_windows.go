// SPDX-License-Identifier: Apache-2.0

//go:build windows

package audio

import (
	"encoding/binary"
	"errors"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"snippingtoolenhanced/internal/win"
)

var (
	ole32             = windows.NewLazySystemDLL("ole32.dll")
	pCoCreateInstance = ole32.NewProc("CoCreateInstance")
	pCoTaskMemFree    = ole32.NewProc("CoTaskMemFree")

	clsidMMDeviceEnumerator = win.MustGUID("bcde0395-e52f-467c-8e3d-c4579291692e")
	iidIMMDeviceEnumerator  = win.MustGUID("a95664d2-9614-4f35-a746-de8db63617e6")
	iidIAudioClient         = win.MustGUID("1cb9ad4c-dbfa-4c32-b178-c2f568a703b2")
	iidIAudioCaptureClient  = win.MustGUID("c8adbd64-e71e-48a0-a4de-185c395cd317")
	subtypeFloat            = win.MustGUID("00000003-0000-0010-8000-00aa00389b71")
)

// Vtable indices (Windows SDK metadata).
const (
	enumGetDefaultAudioEndpoint = 4
	devActivate                 = 3
	acInitialize                = 3
	acGetMixFormat              = 8
	acStart                     = 10
	acStop                      = 11
	acGetService                = 14
	ccGetBuffer                 = 3
	ccReleaseBuffer             = 4
	ccGetNextPacketSize         = 5

	eRender  = 0
	eCapture = 1
	eConsole = 0

	clsctxAll         = 0x17
	streamLoopback    = 0x00020000 // AUDCLNT_STREAMFLAGS_LOOPBACK
	bufferFlagsSilent = 0x2
)

// Source captures one endpoint (loopback of the default speakers, or the
// default microphone) into a Buffer at 48 kHz stereo.
type Source struct {
	Out    *Buffer
	stop   chan struct{}
	done   chan struct{}
	paused func() bool
}

// StartSystem captures what is playing on the default output device.
func StartSystem(out *Buffer, paused func() bool) (*Source, error) {
	return start(eRender, streamLoopback, out, paused)
}

// StartMic captures the default microphone.
func StartMic(out *Buffer, paused func() bool) (*Source, error) {
	return start(eCapture, 0, out, paused)
}

func start(flow, flags uint32, out *Buffer, paused func() bool) (*Source, error) {
	s := &Source{Out: out, stop: make(chan struct{}), done: make(chan struct{}), paused: paused}
	ready := make(chan error, 1)
	go s.run(flow, flags, ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return s, nil
}

// Stop ends capture and waits for the capture thread.
func (s *Source) Stop() {
	close(s.stop)
	<-s.done
}

func (s *Source) run(flow, flags uint32, ready chan<- error) {
	defer close(s.done)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	_ = windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED)
	defer windows.CoUninitialize()

	var enum *win.COM
	r, _, _ := pCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)), 0, clsctxAll,
		uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)), uintptr(unsafe.Pointer(&enum)))
	if err := win.HRErr("CoCreateInstance(MMDeviceEnumerator)", r); err != nil {
		ready <- err
		return
	}
	defer enum.Release()
	var dev *win.COM
	if err := enum.HR("GetDefaultAudioEndpoint", enumGetDefaultAudioEndpoint, uintptr(flow), eConsole, uintptr(unsafe.Pointer(&dev))); err != nil {
		ready <- err
		return
	}
	defer dev.Release()
	var client *win.COM
	if err := dev.HR("Activate", devActivate, uintptr(unsafe.Pointer(&iidIAudioClient)), clsctxAll, 0, uintptr(unsafe.Pointer(&client))); err != nil {
		ready <- err
		return
	}
	defer client.Release()
	var wfx uintptr
	if err := client.HR("GetMixFormat", acGetMixFormat, uintptr(unsafe.Pointer(&wfx))); err != nil {
		ready <- err
		return
	}
	defer pCoTaskMemFree.Call(wfx)
	// Read the 18-byte WAVEFORMATEX first; only WAVEFORMATEXTENSIBLE is 40 bytes.
	hdr := unsafe.Slice((*byte)(win.PtrFromUintptr(wfx)), 18)
	n := 18
	if binary.LittleEndian.Uint16(hdr[0:]) == 0xFFFE && binary.LittleEndian.Uint16(hdr[16:]) >= 22 {
		n = 40
	}
	f, err := parseFormat(unsafe.Slice((*byte)(win.PtrFromUintptr(wfx)), n))
	if err != nil {
		ready <- err
		return
	}
	const bufferHns = 2_000_000 // 200 ms
	if err := client.HR("Initialize", acInitialize, 0 /*shared*/, uintptr(flags), bufferHns, 0, wfx, 0); err != nil {
		ready <- err
		return
	}
	var cap *win.COM
	if err := client.HR("GetService", acGetService, uintptr(unsafe.Pointer(&iidIAudioCaptureClient)), uintptr(unsafe.Pointer(&cap))); err != nil {
		ready <- err
		return
	}
	defer cap.Release()
	if err := client.HR("Start", acStart); err != nil {
		ready <- err
		return
	}
	defer client.Call(acStop)
	ready <- nil

	conv := NewConverter(f)
	bpf := f.Channels * f.Bits / 8
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-tick.C:
		}
		for {
			var packet uint32
			if cap.HR("GetNextPacketSize", ccGetNextPacketSize, uintptr(unsafe.Pointer(&packet))) != nil || packet == 0 {
				break
			}
			var data uintptr
			var frames, bflags uint32
			if cap.HR("GetBuffer", ccGetBuffer, uintptr(unsafe.Pointer(&data)), uintptr(unsafe.Pointer(&frames)), uintptr(unsafe.Pointer(&bflags)), 0, 0) != nil {
				break
			}
			n := int(frames) * bpf
			chunk := make([]byte, n)
			if bflags&bufferFlagsSilent == 0 && data != 0 {
				copy(chunk, unsafe.Slice((*byte)(win.PtrFromUintptr(data)), n))
			}
			cap.Call(ccReleaseBuffer, uintptr(frames))
			out := conv.Convert(chunk)
			if s.paused == nil || !s.paused() {
				s.Out.Push(out)
			}
		}
	}
}

// parseFormat reads a WAVEFORMATEX / WAVEFORMATEXTENSIBLE.
func parseFormat(b []byte) (Format, error) {
	le := binary.LittleEndian
	tag := le.Uint16(b[0:])
	f := Format{Channels: int(le.Uint16(b[2:])), Rate: int(le.Uint32(b[4:])), Bits: int(le.Uint16(b[14:]))}
	switch tag {
	case 3: // WAVE_FORMAT_IEEE_FLOAT
		f.Float = true
	case 1: // WAVE_FORMAT_PCM
	case 0xFFFE: // WAVE_FORMAT_EXTENSIBLE
		if len(b) < 40 {
			return f, errors.New("unsupported audio format")
		}
		var g windows.GUID
		copy((*[16]byte)(unsafe.Pointer(&g))[:], b[24:40])
		f.Float = g == subtypeFloat
	default:
		return f, errors.New("unsupported audio format")
	}
	if f.Channels < 1 || f.Bits%8 != 0 || f.Bits == 0 {
		return f, errors.New("unsupported audio format")
	}
	return f, nil
}
