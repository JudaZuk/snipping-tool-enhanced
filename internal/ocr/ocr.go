// SPDX-License-Identifier: Apache-2.0

// Package ocr recognizes text in images with the Windows on-device OCR
// engine (white paper §3 Text Extractor, §5 Text Actions).
package ocr

import "strings"

// Word is one recognized word with its box in image pixels.
type Word struct {
	Text string  `json:"text"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
}

// Line is one line of text.
type Line struct {
	Text  string `json:"text"`
	Words []Word `json:"words"`
}

// Result is the recognized text of an image.
type Result struct {
	Lines []Line  `json:"lines"`
	Angle float64 `json:"angle"`
}

// Text joins the lines; with removeLineBreaks the lines are joined by spaces.
func (r Result) Text(removeLineBreaks bool) string {
	parts := make([]string, 0, len(r.Lines))
	for _, l := range r.Lines {
		if t := strings.TrimSpace(l.Text); t != "" {
			parts = append(parts, t)
		}
	}
	if removeLineBreaks {
		return strings.Join(parts, " ")
	}
	return strings.Join(parts, "\r\n")
}

// Scale multiplies all word boxes by 1/k (used after recognizing an upscaled image).
func (r *Result) Scale(k float64) {
	if k == 1 || k == 0 {
		return
	}
	for i := range r.Lines {
		for j := range r.Lines[i].Words {
			w := &r.Lines[i].Words[j]
			w.X, w.Y, w.W, w.H = w.X/k, w.Y/k, w.W/k, w.H/k
		}
	}
}
