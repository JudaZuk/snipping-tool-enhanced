// SPDX-License-Identifier: Apache-2.0

// Package naming builds file names from the user's pattern (§4.3).
// Tokens: {date} {time} {app} {title} {n}.
package naming

import (
	"fmt"
	"strings"
	"time"
)

// Info supplies values for the pattern tokens.
type Info struct {
	Time  time.Time
	App   string
	Title string
	N     int
}

// Expand fills the pattern and returns a safe Windows file name (no extension).
func Expand(pattern string, in Info) string {
	r := strings.NewReplacer(
		"{date}", in.Time.Format("2006-01-02"),
		"{time}", in.Time.Format("150405"),
		"{app}", in.App,
		"{title}", in.Title,
		"{n}", fmt.Sprint(in.N),
	)
	name := Sanitize(r.Replace(pattern))
	if name == "" {
		name = "Screenshot " + in.Time.Format("2006-01-02 150405")
	}
	return name
}

var reserved = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// Sanitize removes characters Windows forbids in file names, trims trailing
// dots/spaces, avoids reserved device names and caps the length.
func Sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 0x20, strings.ContainsRune(`<>:"/\|?*`, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimRight(strings.TrimSpace(b.String()), ". ")
	if rs := []rune(out); len(rs) > 150 {
		out = strings.TrimRight(string(rs[:150]), ". ")
	}
	base := strings.ToUpper(out)
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	if reserved[base] {
		out = "_" + out
	}
	return out
}

// Unique returns dir\name.ext, adding " (2)", " (3)"… while exists reports true.
func Unique(dir, name, ext string, exists func(string) bool) string {
	join := func(n string) string { return strings.TrimRight(dir, `\/`) + `\` + n + ext }
	p := join(name)
	for i := 2; exists(p) && i < 10000; i++ {
		p = join(fmt.Sprintf("%s (%d)", name, i))
	}
	return p
}
