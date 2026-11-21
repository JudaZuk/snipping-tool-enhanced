// SPDX-License-Identifier: Apache-2.0

package naming

import (
	"testing"
	"time"
)

func TestExpand(t *testing.T) {
	tm := time.Date(2026, 11, 21, 9, 5, 7, 0, time.UTC)
	got := Expand("Screenshot {date} {time} {app} {title} #{n}", Info{Time: tm, App: "notepad", Title: `a/b:c?`, N: 4})
	if got != "Screenshot 2026-11-21 090507 notepad a_b_c_ #4" {
		t.Fatal(got)
	}
	if Expand("  ...  ", Info{Time: tm}) != "Screenshot 2026-11-21 090507" {
		t.Fatal("empty fallback")
	}
}

func TestSanitize(t *testing.T) {
	cases := map[string]string{"CON": "_CON", "nul.txt": "_nul.txt", "ok name. ": "ok name", "tab\there": "tab_here"}
	for in, want := range cases {
		if got := Sanitize(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestUnique(t *testing.T) {
	taken := map[string]bool{`C:\x\a.png`: true, `C:\x\a (2).png`: true}
	got := Unique(`C:\x\`, "a", ".png", func(p string) bool { return taken[p] })
	if got != `C:\x\a (3).png` {
		t.Fatal(got)
	}
}
