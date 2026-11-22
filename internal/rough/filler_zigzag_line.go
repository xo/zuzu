// Copied from github.com/d2lang/rough-go, file filler_zigzag_line.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.0.4, Copyright (c) 2019 Preet Shihn.

package rough

import "math"

type zigZagLineFiller struct {
	helper renderHelper
}

func (f *zigZagLineFiller) fillPolygon(points []Point, o *ResolvedOptions) OpSet {
	gap := o.HachureGap
	if gap < 0 {
		gap = o.StrokeWidth * 4
	}
	zo := o.ZigzagOffset
	if zo < 0 {
		zo = gap
	}
	o2 := *o
	o2.HachureGap = gap + zo
	lines := polygonHachureLines(points, &o2)
	return OpSet{Type: OpSetFillSketch, Ops: f.zigzagLines(lines, zo, &o2)}
}

func (f *zigZagLineFiller) zigzagLines(lines []Line, zo float64, o *ResolvedOptions) []Op {
	ops := make([]Op, 0)
	for _, line := range lines {
		length := lineLength(line)
		count := jsRound(length / (2 * zo))
		p1, p2 := line[0], line[1]
		if p1[0] > p2[0] {
			p1, p2 = line[1], line[0]
		}
		alpha := math.Atan((p2[1] - p1[1]) / (p2[0] - p1[0]))
		for i := 0.0; i < count; i++ {
			lstart := i * 2 * zo
			lend := (i + 1) * 2 * zo
			dz := math.Sqrt(2 * math.Pow(zo, 2))
			start := Point{p1[0] + lstart*math.Cos(alpha), p1[1] + lstart*math.Sin(alpha)}
			end := Point{p1[0] + lend*math.Cos(alpha), p1[1] + lend*math.Sin(alpha)}
			middle := Point{
				start[0] + dz*math.Cos(alpha+math.Pi/4),
				start[1] + dz*math.Sin(alpha+math.Pi/4),
			}
			ops = append(ops, f.helper.doubleLineOps(start[0], start[1], middle[0], middle[1], o)...)
			ops = append(ops, f.helper.doubleLineOps(middle[0], middle[1], end[0], end[1], o)...)
		}
	}
	return ops
}
