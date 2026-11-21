// SPDX-License-Identifier: Apache-2.0

// Package audio captures system sound and the microphone for screen
// recordings (white paper §4) and mixes them to 48 kHz stereo 16-bit PCM.
package audio

import (
	"encoding/binary"
	"math"
	"sync"
)

// Rate and channel count of the mixed output.
const (
	Rate     = 48000
	Channels = 2
)

// Format describes a source's native sample format.
type Format struct {
	Rate     int
	Channels int
	Bits     int
	Float    bool
}

// Converter turns native interleaved samples into 48 kHz stereo float32,
// resampling linearly and keeping state between packets.
type Converter struct {
	f    Format
	pos  float64 // fractional read position into the pending input
	prev [2]float32
	have bool
}

// NewConverter returns a converter for format f.
func NewConverter(f Format) *Converter { return &Converter{f: f} }

func (c *Converter) frame(data []byte, i int) [2]float32 {
	bps := c.f.Bits / 8
	at := func(ch int) float32 {
		o := (i*c.f.Channels + ch) * bps
		switch {
		case c.f.Float && bps == 4:
			return math.Float32frombits(binary.LittleEndian.Uint32(data[o:]))
		case bps == 2:
			return float32(int16(binary.LittleEndian.Uint16(data[o:]))) / 32768
		case bps == 4:
			return float32(int32(binary.LittleEndian.Uint32(data[o:]))) / 2147483648
		case bps == 3:
			v := int32(data[o]) | int32(data[o+1])<<8 | int32(int8(data[o+2]))<<16
			return float32(v) / 8388608
		}
		return 0
	}
	if c.f.Channels == 1 {
		v := at(0)
		return [2]float32{v, v}
	}
	return [2]float32{at(0), at(1)}
}

// Convert consumes one packet of native frames and returns stereo float32 at 48 kHz.
func (c *Converter) Convert(data []byte) []float32 {
	bpf := c.f.Channels * c.f.Bits / 8
	if bpf == 0 || c.f.Rate == 0 {
		return nil
	}
	n := len(data) / bpf
	if n == 0 {
		return nil
	}
	step := float64(c.f.Rate) / Rate
	var out []float32
	// Virtual input: prev (index -1) followed by frames 0..n-1.
	get := func(i int) [2]float32 {
		if i < 0 {
			return c.prev
		}
		return c.frame(data, i)
	}
	if !c.have {
		c.prev = c.frame(data, 0)
		c.have = true
	}
	for c.pos < float64(n-1) {
		i := int(math.Floor(c.pos))
		t := float32(c.pos - float64(i))
		a, b := get(i), get(i+1)
		out = append(out, a[0]+(b[0]-a[0])*t, a[1]+(b[1]-a[1])*t)
		c.pos += step
	}
	c.pos -= float64(n)
	c.prev = c.frame(data, n-1)
	return out
}

// Buffer is a thread-safe FIFO of stereo float samples with a maximum length.
type Buffer struct {
	mu   sync.Mutex
	data []float32
	max  int
}

// NewBuffer holds at most maxFrames stereo frames (older audio is dropped).
func NewBuffer(maxFrames int) *Buffer { return &Buffer{max: maxFrames * 2} }

// Push appends samples.
func (b *Buffer) Push(s []float32) {
	b.mu.Lock()
	b.data = append(b.data, s...)
	if over := len(b.data) - b.max; over > 0 {
		b.data = b.data[over&^1:]
	}
	b.mu.Unlock()
}

// Clear drops everything buffered.
func (b *Buffer) Clear() {
	b.mu.Lock()
	b.data = b.data[:0]
	b.mu.Unlock()
}

// Take removes up to frames stereo frames; missing frames are silence.
func (b *Buffer) Take(frames int) []float32 {
	out := make([]float32, frames*2)
	b.mu.Lock()
	n := copy(out, b.data)
	b.data = b.data[n:]
	b.mu.Unlock()
	return out
}

// Mix sums sources and converts to interleaved little-endian int16 PCM.
func Mix(frames int, sources ...*Buffer) []byte {
	acc := make([]float32, frames*2)
	for _, s := range sources {
		if s == nil {
			continue
		}
		for i, v := range s.Take(frames) {
			acc[i] += v
		}
	}
	out := make([]byte, len(acc)*2)
	for i, v := range acc {
		if v > 1 {
			v = 1
		} else if v < -1 {
			v = -1
		}
		binary.LittleEndian.PutUint16(out[i*2:], uint16(int16(v*32767)))
	}
	return out
}
