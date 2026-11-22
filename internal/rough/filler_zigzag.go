// Copied from github.com/d2lang/rough-go, file filler_zigzag.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.6.6, Copyright (c) 2019 Preet Shihn.

package rough

import "math"

type zigZagFiller struct {
	hachureFiller
}

func (f *zigZagFiller) fillPolygon(points []Point, o *ResolvedOptions) OpSet {
	gap := o.HachureGap
	if gap < 0 {
		gap = o.StrokeWidth * 4
	}
	gap = math.Max(gap, 0.1)
	// Object.assign in Rough.js keeps the mutable randomizer shared.
	o2 := *o
	o2.HachureGap = gap
	lines := polygonHachureLines(points, &o2)
	angle := math.Pi / 180 * o.HachureAngle
	dgx := gap * 0.5 * math.Cos(angle)
	dgy := gap * 0.5 * math.Sin(angle)
	zigzag := make([]Line, 0, len(lines)*2)
	for _, line := range lines {
		p1, p2 := line[0], line[1]
		if lineLength(line) == 0 {
			continue
		}
		zigzag = append(zigzag,
			Line{{p1[0] - dgx, p1[1] + dgy}, p2},
			Line{{p1[0] + dgx, p1[1] - dgy}, p2},
		)
	}
	return OpSet{Type: OpSetFillSketch, Ops: f.renderLines(zigzag, o, false)}
}
