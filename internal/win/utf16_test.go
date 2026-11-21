// SPDX-License-Identifier: Apache-2.0

package win

import "testing"

func TestMultiString(t *testing.T) {
	got := MultiString("PNG (*.png)\x00*.png\x00JPEG\x00*.jpg\x00")
	want := []uint16{}
	for _, r := range "PNG (*.png)\x00*.png\x00JPEG\x00*.jpg" {
		want = append(want, uint16(r))
	}
	want = append(want, 0, 0)
	if len(got) != len(want) {
		t.Fatalf("len %d want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("at %d: %d != %d", i, got[i], want[i])
		}
	}
	if m := MultiString(""); len(m) != 2 || m[0] != 0 {
		t.Fatal("empty list")
	}
}
