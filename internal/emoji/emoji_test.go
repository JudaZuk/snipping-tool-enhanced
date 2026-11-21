// SPDX-License-Identifier: Apache-2.0

package emoji

import "testing"

func TestAllEmojiLoad(t *testing.T) {
	if len(List) != 48 {
		t.Fatalf("%d emoji", len(List))
	}
	for i, e := range List {
		img := Image(i)
		if img == nil || img.Bounds().Dx() != 128 {
			t.Fatalf("emoji %d (%s) did not load", i, e.Char)
		}
		// Some pixels must be opaque and the corners transparent.
		if img.NRGBAAt(64, 64).A == 0 && img.NRGBAAt(60, 70).A == 0 {
			t.Errorf("emoji %s looks empty", e.Char)
		}
		if img.NRGBAAt(0, 0).A != 0 {
			t.Errorf("emoji %s has an opaque corner", e.Char)
		}
	}
	if s := Scaled(0, 40); s == nil || s.Bounds().Dx() != 40 {
		t.Fatal("scaled")
	}
	if Image(-1) != nil || Image(len(List)) != nil {
		t.Fatal("out of range")
	}
}
