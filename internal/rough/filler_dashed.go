// Copied from github.com/d2lang/rough-go, file filler_dashed.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.0.4, Copyright (c) 2019 Preet Shihn.

package rough

import "math"

type dashedFiller struct {
	helper renderHelper
}

func (f *dashedFiller) fillPolygon(points []Point, o *ResolvedOptions) OpSet {
	lines := polygonHachureLines(points, o)
	return OpSet{Type: OpSetFillSketch, Ops: f.dashedLine(lines, o)}
}

func (f *dashedFiller) dashedLine(lines []Line, o *ResolvedOptions) []Op {
	offset := o.DashOffset
	if offset < 0 {
		if o.HachureGap < 0 {
			offset = o.StrokeWidth * 4
		} else {
			offset = o.HachureGap
		}
	}
	gap := o.DashGap
	if gap < 0 {
		if o.HachureGap < 0 {
			gap = o.StrokeWidth * 4
		} else {
			gap = o.HachureGap
		}
	}
	ops := make([]Op, 0)
	for _, line := range lines {
		length := lineLength(line)
		// Hachure endpoints are integer scan-line intersections before they are
		// rotated back. Go and V8's trigonometric implementations can leave the
		// rotation-preserved length on opposite sides of an exact dash boundary
		// by one ULP (for example, 1.9999999999999998 versus 2). Snap only that
		// immediately adjacent representation so the structural Math.floor
		// decision remains stable while all non-boundary arithmetic is unchanged.
		count := math.Floor(snapOneULPToInteger(length / (offset + gap)))
		startOffset := (length + gap - count*(offset+gap)) / 2
		p1, p2 := line[0], line[1]
		if p1[0] > p2[0] {
			p1, p2 = line[1], line[0]
		}
		alpha := math.Atan((p2[1] - p1[1]) / (p2[0] - p1[0]))
		for i := 0.0; i < count; i++ {
			lstart := i * (offset + gap)
			lend := lstart + offset
			start := Point{
				p1[0] + lstart*math.Cos(alpha) + startOffset*math.Cos(alpha),
				p1[1] + lstart*math.Sin(alpha) + startOffset*math.Sin(alpha),
			}
			end := Point{
				p1[0] + lend*math.Cos(alpha) + startOffset*math.Cos(alpha),
				p1[1] + lend*math.Sin(alpha) + startOffset*math.Sin(alpha),
			}
			ops = append(ops, f.helper.doubleLineOps(start[0], start[1], end[0], end[1], o)...)
		}
	}
	return ops
}

func snapOneULPToInteger(v float64) float64 {
	nearest := math.Round(v)
	if v == nearest || math.Nextafter(v, nearest) == nearest {
		return nearest
	}
	return v
}
