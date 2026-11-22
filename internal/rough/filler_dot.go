// Copied from github.com/d2lang/rough-go, file filler_dot.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.0.4, Copyright (c) 2019 Preet Shihn.

package rough

import "math"

type dotFiller struct {
	helper renderHelper
}

func (f *dotFiller) fillPolygon(points []Point, o *ResolvedOptions) OpSet {
	o2 := *o
	o2.CurveStepCount = 4
	o2.HachureAngle = 0
	o2.Roughness = 1
	lines := polygonHachureLines(points, &o2)
	return f.dotsOnLines(lines, &o2)
}

func (f *dotFiller) dotsOnLines(lines []Line, o *ResolvedOptions) OpSet {
	ops := make([]Op, 0)
	gap := o.HachureGap
	if gap < 0 {
		gap = o.StrokeWidth * 4
	}
	gap = math.Max(gap, 0.1)
	fweight := o.FillWeight
	if fweight < 0 {
		fweight = o.StrokeWidth / 2
	}
	for _, line := range lines {
		length := lineLength(line)
		dl := length / gap
		count := math.Ceil(dl) - 1
		alpha := math.Atan((line[1][1] - line[0][1]) / (line[1][0] - line[0][0]))
		for i := 0.0; i < count; i++ {
			l := gap * (i + 1)
			dy := l * math.Sin(alpha)
			dx := l * math.Cos(alpha)
			c := Point{line[0][0] - dx, line[0][1] + dy}
			cx := f.helper.randOffsetWithRange(c[0]-gap/4, c[0]+gap/4, o)
			cy := f.helper.randOffsetWithRange(c[1]-gap/4, c[1]+gap/4, o)
			el := f.helper.ellipse(cx, cy, fweight, fweight, o)
			ops = append(ops, el.Ops...)
		}
	}
	return OpSet{Type: OpSetFillSketch, Ops: ops}
}
