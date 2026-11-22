// Copied from github.com/d2lang/rough-go, file filler_hachure.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.0.4, Copyright (c) 2019 Preet Shihn.

package rough

type hachureFiller struct {
	helper renderHelper
}

func (f *hachureFiller) fillPolygon(points []Point, o *ResolvedOptions) OpSet {
	return f.fillPolygonWithConnectEnds(points, o, false)
}

func (f *hachureFiller) fillPolygonWithConnectEnds(points []Point, o *ResolvedOptions, connectEnds bool) OpSet {
	lines := polygonHachureLines(points, o)
	ops := f.renderLines(lines, o, connectEnds)
	return OpSet{Type: OpSetFillSketch, Ops: ops}
}

func (f *hachureFiller) renderLines(lines []Line, o *ResolvedOptions, connectEnds bool) []Op {
	ops := make([]Op, 0)
	var prevPoint *Point
	for _, line := range lines {
		ops = append(ops, f.helper.doubleLineOps(
			line[0][0], line[0][1], line[1][0], line[1][1], o,
		)...)
		if connectEnds && prevPoint != nil {
			ops = append(ops, f.helper.doubleLineOps(
				(*prevPoint)[0], (*prevPoint)[1], line[0][0], line[0][1], o,
			)...)
		}
		p := line[1]
		prevPoint = &p
	}
	return ops
}
