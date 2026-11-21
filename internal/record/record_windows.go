// SPDX-License-Identifier: Apache-2.0

//go:build windows

// Package record records a fixed screen region to MP4 (white paper §4):
// about 30 fps H.264 with optional system audio and microphone (AAC).
package record

import (
	"errors"
	"image"
	"os"
	"runtime"
	"sync"
	"time"

	"snippingtoolenhanced/internal/audio"
	"snippingtoolenhanced/internal/mf"
	"snippingtoolenhanced/internal/win"
)

// FPS is the recording frame rate (not user-configurable, per the white paper).
const FPS = 30

// Options for a recording.
type Options struct {
	Rect        image.Rectangle // virtual-screen coordinates
	Path        string
	SystemAudio bool
	Mic         bool
}

// Session is a running recording.
type Session struct {
	opts    Options
	w, h    int
	mu      sync.Mutex
	paused  bool
	pausedT time.Duration
	pauseAt time.Time
	start   time.Time
	frames  chan frame
	pcm     chan frame
	stop    chan struct{}
	done    chan error
	sys     *audio.Source
	mic     *audio.Source
	sysBuf  *audio.Buffer
	micBuf  *audio.Buffer
	muteSys bool
	muteMic bool
}

type frame struct {
	data   []byte
	t, dur int64
}

// Start begins recording. The region is rounded down to even dimensions.
func Start(o Options) (*Session, error) {
	r := o.Rect
	w, h := r.Dx()&^1, r.Dy()&^1
	if w < 16 || h < 16 {
		return nil, errors.New("the recording area is too small")
	}
	s := &Session{opts: o, w: w, h: h, frames: make(chan frame, 8), pcm: make(chan frame, 64),
		stop: make(chan struct{}), done: make(chan error, 1)}

	ready := make(chan error, 1)
	go s.writer(ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	isPaused := func() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.paused }
	if o.SystemAudio {
		s.sysBuf = audio.NewBuffer(audio.Rate / 2)
		if src, err := audio.StartSystem(s.sysBuf, isPaused); err == nil {
			s.sys = src
		}
	}
	if o.Mic {
		s.micBuf = audio.NewBuffer(audio.Rate / 2)
		if src, err := audio.StartMic(s.micBuf, isPaused); err == nil {
			s.mic = src
		}
	}
	s.start = time.Now()
	go s.video()
	go s.mixer()
	return s, nil
}

// elapsed is recording time excluding pauses.
func (s *Session) elapsed() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if s.paused {
		now = s.pauseAt
	}
	return now.Sub(s.start) - s.pausedT
}

// Elapsed returns the recorded duration so far.
func (s *Session) Elapsed() time.Duration { return s.elapsed() }

// Pause and Resume.
func (s *Session) Pause() {
	s.mu.Lock()
	if !s.paused {
		s.paused, s.pauseAt = true, time.Now()
	}
	s.mu.Unlock()
}

func (s *Session) Resume() {
	s.mu.Lock()
	if s.paused {
		s.pausedT += time.Since(s.pauseAt)
		s.paused = false
	}
	s.mu.Unlock()
	if s.sysBuf != nil {
		s.sysBuf.Clear()
	}
	if s.micBuf != nil {
		s.micBuf.Clear()
	}
}

// Paused reports whether the session is paused.
func (s *Session) Paused() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.paused }

// SetMuted mutes the system audio or microphone track contribution.
func (s *Session) SetMuted(system, mic bool) {
	s.mu.Lock()
	s.muteSys, s.muteMic = system, mic
	s.mu.Unlock()
}

// HasAudio reports which sources are actually capturing.
func (s *Session) HasAudio() (system, mic bool) { return s.sys != nil, s.mic != nil }

// Stop finishes the file and returns its path.
func (s *Session) Stop() (string, error) {
	close(s.stop)
	if s.sys != nil {
		s.sys.Stop()
	}
	if s.mic != nil {
		s.mic.Stop()
	}
	err := <-s.done
	return s.opts.Path, err
}

// Discard stops and deletes the recording.
func (s *Session) Discard() {
	_, _ = s.Stop()
	_ = os.Remove(s.opts.Path)
}

func (s *Session) video() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	dib, err := win.NewDIB(s.w, s.h)
	if err != nil {
		close(s.frames)
		return
	}
	defer dib.Close()
	period := time.Second / FPS
	tick := time.NewTicker(period)
	defer tick.Stop()
	defer close(s.frames)
	next := int64(0) // next frame number
	for {
		select {
		case <-s.stop:
			return
		case <-tick.C:
		}
		if s.Paused() {
			continue
		}
		el := s.elapsed()
		n := int64(el / period)
		if n < next { // already have a frame for this slot
			continue
		}
		next = n + 1
		_ = dib.BlitFromScreen(int32(s.opts.Rect.Min.X), int32(s.opts.Rect.Min.Y))
		win.DrawCursor(dib.DC, int32(s.opts.Rect.Min.X), int32(s.opts.Rect.Min.Y))
		data := make([]byte, len(dib.Bits))
		copy(data, dib.Bits)
		for i := 3; i < len(data); i += 4 {
			data[i] = 255
		}
		f := frame{data: data, t: n * int64(period/100), dur: int64(period / 100)}
		select {
		case s.frames <- f:
		default: // encoder is behind: drop this frame
		}
	}
}

func (s *Session) mixer() {
	defer close(s.pcm)
	if s.sys == nil && s.mic == nil {
		<-s.stop
		return
	}
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	var written int64 // frames
	for {
		select {
		case <-s.stop:
			return
		case <-tick.C:
		}
		if s.Paused() {
			continue
		}
		target := int64(s.elapsed().Seconds() * audio.Rate)
		n := int(target - written)
		if n <= 0 {
			continue
		}
		s.mu.Lock()
		var srcs []*audio.Buffer
		if s.sys != nil && !s.muteSys {
			srcs = append(srcs, s.sysBuf)
		} else if s.sysBuf != nil {
			s.sysBuf.Clear()
		}
		if s.mic != nil && !s.muteMic {
			srcs = append(srcs, s.micBuf)
		} else if s.micBuf != nil {
			s.micBuf.Clear()
		}
		s.mu.Unlock()
		pcm := audio.Mix(n, srcs...)
		t := written * 10_000_000 / audio.Rate
		dur := int64(n) * 10_000_000 / audio.Rate
		written += int64(n)
		s.pcm <- frame{data: pcm, t: t, dur: dur}
	}
}

func (s *Session) writer(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	shutdown, err := mf.Startup()
	if err != nil {
		ready <- err
		return
	}
	defer shutdown()
	wr, err := mf.NewWriter(s.opts.Path, s.w, s.h, FPS, s.opts.SystemAudio || s.opts.Mic)
	if err != nil {
		ready <- err
		return
	}
	ready <- nil
	var firstErr error
	frames, pcm := s.frames, s.pcm
	for frames != nil || pcm != nil {
		select {
		case f, ok := <-frames:
			if !ok {
				frames = nil
				continue
			}
			if err := wr.WriteVideo(f.data, f.t, f.dur); err != nil && firstErr == nil {
				firstErr = err
			}
		case f, ok := <-pcm:
			if !ok {
				pcm = nil
				continue
			}
			if err := wr.WriteAudio(f.data, f.t, f.dur); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	if err := wr.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	s.done <- firstErr
}
