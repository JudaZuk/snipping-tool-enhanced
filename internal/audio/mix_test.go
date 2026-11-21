// SPDX-License-Identifier: Apache-2.0

package audio

import (
	"encoding/binary"
	"math"
	"testing"
)

func f32(vals ...float32) []byte {
	b := make([]byte, len(vals)*4)
	for i, v := range vals {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(v))
	}
	return b
}

func TestConvertSameRate(t *testing.T) {
	c := NewConverter(Format{Rate: 48000, Channels: 2, Bits: 32, Float: true})
	total := 0
	for k := 0; k < 10; k++ {
		in := make([]float32, 960) // 480 frames = 10 ms
		for i := range in {
			in[i] = 0.5
		}
		out := c.Convert(f32(in...))
		total += len(out) / 2
		for _, v := range out {
			if math.Abs(float64(v-0.5)) > 1e-6 {
				t.Fatal(v)
			}
		}
	}
	if total < 4790 || total > 4800 {
		t.Fatalf("frames %d", total)
	}
}

func TestConvertResampleMono16(t *testing.T) {
	c := NewConverter(Format{Rate: 44100, Channels: 1, Bits: 16})
	total := 0
	for k := 0; k < 100; k++ {
		in := make([]byte, 441*2)
		for i := 0; i < 441; i++ {
			binary.LittleEndian.PutUint16(in[i*2:], uint16(int16(16384)))
		}
		out := c.Convert(in)
		total += len(out) / 2
		if len(out) > 0 && (math.Abs(float64(out[0]-0.5)) > 1e-3 || out[0] != out[1]) {
			t.Fatal(out[:2])
		}
	}
	if total < 47900 || total > 48100 { // 1 s of audio
		t.Fatalf("frames %d", total)
	}
}

func TestBufferAndMix(t *testing.T) {
	a, b := NewBuffer(100), NewBuffer(100)
	a.Push([]float32{0.5, 0.5, 0.5, 0.5})
	b.Push([]float32{0.75, -0.25})
	pcm := Mix(3, a, b)
	if len(pcm) != 12 {
		t.Fatal(len(pcm))
	}
	s := func(i int) int16 { return int16(binary.LittleEndian.Uint16(pcm[i*2:])) }
	if s(0) != 32767 || s(1) < 8000 || s(2) < 16000 || s(4) != 0 {
		t.Fatal(s(0), s(1), s(2), s(4))
	}
	big := make([]float32, 1000)
	a.Push(big)
	if len(a.Take(1000)) != 2000 {
		t.Fatal("take pads with silence")
	}
}
