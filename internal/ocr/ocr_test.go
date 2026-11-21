// SPDX-License-Identifier: Apache-2.0

package ocr

import "testing"

func TestTextAndScale(t *testing.T) {
	r := Result{Lines: []Line{{Text: "Hello world", Words: []Word{{Text: "Hello", X: 10, Y: 20, W: 40, H: 12}}}, {Text: "  "}, {Text: "Second"}}}
	if r.Text(false) != "Hello world\r\nSecond" || r.Text(true) != "Hello world Second" {
		t.Fatal(r.Text(false), r.Text(true))
	}
	r.Scale(2)
	if w := r.Lines[0].Words[0]; w.X != 5 || w.Y != 10 || w.W != 20 || w.H != 6 {
		t.Fatal(w)
	}
}
