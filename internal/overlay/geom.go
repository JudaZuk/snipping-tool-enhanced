// SPDX-License-Identifier: Apache-2.0

package overlay

import "image"

// dragRect returns the selection for a drag from a to b; with square set the
// result is 1:1 in the drag direction.
func dragRect(a, b image.Point, square bool) image.Rectangle {
	if square {
		dx, dy := b.X-a.X, b.Y-a.Y
		side := abs(dx)
		if abs(dy) > side {
			side = abs(dy)
		}
		b = image.Pt(a.X+sign(dx)*side, a.Y+sign(dy)*side)
	}
	return image.Rectangle{Min: a, Max: b}.Canon()
}

// segmentRect is the bounding box of the line from a to b, at least 1×1.
// (image.Rectangle.Union ignores empty rectangles, so two points can't be
// combined with Union.)
func segmentRect(a, b image.Point) image.Rectangle {
	return image.Rect(a.X, a.Y, a.X+1, a.Y+1).Union(image.Rect(b.X, b.Y, b.X+1, b.Y+1))
}

func polyBounds(pts []image.Point) image.Rectangle {
	if len(pts) == 0 {
		return image.Rectangle{}
	}
	r := image.Rect(pts[0].X, pts[0].Y, pts[0].X+1, pts[0].Y+1)
	for _, p := range pts[1:] {
		r = r.Union(image.Rectangle{Min: p, Max: p.Add(image.Pt(1, 1))})
	}
	return r
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sign(v int) int {
	if v < 0 {
		return -1
	}
	return 1
}
