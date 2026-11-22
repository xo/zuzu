// Copied from github.com/d2lang/rough-go, file filler_scan_line_hachure.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.6.6 and hachure-fill 0.5.2.
// Copyright (c) 2019, 2023 Preet Shihn.

package rough

import (
	"math"
	"sort"
)

type hachureEdgeEntry struct {
	ymin   float64
	ymax   float64
	x      float64
	islope float64
}

type activeHachureEdgeEntry struct {
	s    float64
	edge *hachureEdgeEntry
}

const collapsedEndpointMaxULPs = 8

func polygonHachureLines(points []Point, o *ResolvedOptions) []Line {
	rotationCenter := Point{0, 0}
	angle := o.HachureAngle + 90
	gap := o.HachureGap
	if gap < 0 {
		gap = o.StrokeWidth * 4
	}
	gap = jsRound(math.Max(gap, 0.1))
	skipOffset := 1.0
	if o.Roughness >= 1 && random(o) > 0.7 {
		skipOffset = gap
	}
	scanPoints := points
	var unrotated []Point
	if angle != 0 {
		unrotated = append([]Point(nil), points...)
		rotatePoints(points, rotationCenter, angle)
		// Keep the source-observable point rotation/round trip on the caller's
		// slice, but stabilize scan-line event ordering on a private copy. V8
		// and Go trig can collapse two near-coincident rotated ordinates onto
		// the same float on only one runtime, which otherwise changes edge
		// activation and the entire filler topology.
		scanPoints = append([]Point(nil), points...)
		stabilizeCollapsedRotatedOrdinates(unrotated, scanPoints, angle)
	}
	lines := straightHachureLinesModern(scanPoints, gap, skipOffset)
	if angle != 0 {
		rotatePoints(points, rotationCenter, -angle)
		rotateLines(lines, rotationCenter, -angle)
	}
	return lines
}

func straightHachureLinesModern(points []Point, gap, stepOffset float64) []Line {
	vertices := append([]Point(nil), points...)
	if len(vertices) == 0 {
		return nil
	}
	if !pointsJoinEqual(vertices[0], vertices[len(vertices)-1]) {
		vertices = append(vertices, vertices[0])
	}
	lines := make([]Line, 0)
	if len(vertices) <= 2 {
		return lines
	}

	edges := make([]*hachureEdgeEntry, 0, len(vertices)-1)
	for i := 0; i < len(vertices)-1; i++ {
		p1, p2 := vertices[i], vertices[i+1]
		if p1[1] == p2[1] {
			continue
		}
		ymin := math.Min(p1[1], p2[1])
		edges = append(edges, &hachureEdgeEntry{
			ymin: ymin, ymax: math.Max(p1[1], p2[1]),
			x:      chooseFloat(ymin == p1[1], p1[0], p2[0]),
			islope: (p2[0] - p1[0]) / (p2[1] - p1[1]),
		})
	}
	sort.SliceStable(edges, func(i, j int) bool {
		a, b := edges[i], edges[j]
		if a.ymin != b.ymin {
			return a.ymin < b.ymin
		}
		if a.x != b.x {
			return a.x < b.x
		}
		return a.ymax < b.ymax
	})
	if len(edges) == 0 {
		return lines
	}

	active := make([]activeHachureEdgeEntry, 0)
	y := edges[0].ymin
	iteration := 0.0
	for len(active) != 0 || len(edges) != 0 {
		ix := -1
		for i := range edges {
			if edges[i].ymin > y {
				break
			}
			ix = i
		}
		if ix >= 0 {
			for _, edge := range edges[:ix+1] {
				active = append(active, activeHachureEdgeEntry{s: y, edge: edge})
			}
			edges = edges[ix+1:]
		}
		kept := active[:0]
		for _, ae := range active {
			if ae.edge.ymax > y {
				kept = append(kept, ae)
			}
		}
		active = kept
		sort.SliceStable(active, func(i, j int) bool { return active[i].edge.x < active[j].edge.x })
		if stepOffset != 1 || math.Mod(iteration, gap) == 0 {
			for i := 0; i+1 < len(active); i += 2 {
				lines = append(lines, Line{{jsRound(active[i].edge.x), y}, {jsRound(active[i+1].edge.x), y}})
			}
		}
		y += stepOffset
		for i := range active {
			active[i].edge.x += stepOffset * active[i].edge.islope
		}
		iteration++
	}
	return lines
}

func stabilizeCollapsedRotatedOrdinates(unrotated, rotated []Point, degrees float64) {
	if len(unrotated) != len(rotated) || len(rotated) < 2 {
		return
	}
	angle := (math.Pi / 180) * degrees
	sin, cos := math.Sin(angle), math.Cos(angle)
	for i := range rotated {
		j := i + 1
		if j == len(rotated) {
			j = 0
		}
		if rotated[i][1] != rotated[j][1] ||
			pointsJoinEqual(unrotated[i], unrotated[j]) ||
			!pointsWithinULPs(unrotated[i], unrotated[j], collapsedEndpointMaxULPs) {
			continue
		}
		// Compute the transformed difference directly instead of subtracting
		// the already-rounded absolute coordinates. This retains the sign that
		// was lost to cancellation in the ordinary rotation.
		deltaY := (unrotated[i][0]-unrotated[j][0])*sin +
			(unrotated[i][1]-unrotated[j][1])*cos
		switch {
		case deltaY > 0:
			rotated[i][1] = math.Nextafter(rotated[i][1], math.Inf(1))
		case deltaY < 0:
			rotated[i][1] = math.Nextafter(rotated[i][1], math.Inf(-1))
		}
	}
}

func pointsWithinULPs(a, b Point, maxULPs float64) bool {
	scale := 1.0
	for axis := range a {
		for _, coordinate := range [...]float64{a[axis], b[axis]} {
			if math.IsNaN(coordinate) || math.IsInf(coordinate, 0) {
				return false
			}
			scale = math.Max(scale, math.Abs(coordinate))
		}
	}
	tolerance := maxULPs * (math.Nextafter(scale, math.Inf(1)) - scale)
	for axis := range a {
		if math.Abs(a[axis]-b[axis]) > tolerance {
			return false
		}
	}
	return true
}

func straightHachureLines(points []Point, o *ResolvedOptions) []Line {
	vertices := append([]Point(nil), points...)
	if len(vertices) == 0 {
		// Rough.js would fail while indexing an empty polygon. Returning the
		// empty source-shaped result keeps the unexported helper total without
		// changing any generator-visible nonempty behavior.
		return nil
	}
	if !pointsJoinEqual(vertices[0], vertices[len(vertices)-1]) {
		vertices = append(vertices, Point{vertices[0][0], vertices[0][1]})
	}
	lines := make([]Line, 0)
	if len(vertices) <= 2 {
		return lines
	}

	gap := o.HachureGap
	if gap < 0 {
		gap = o.StrokeWidth * 4
	}
	gap = math.Max(gap, 0.1)

	edges := make([]*hachureEdgeEntry, 0, len(vertices)-1)
	for i := 0; i < len(vertices)-1; i++ {
		p1 := vertices[i]
		p2 := vertices[i+1]
		if p1[1] != p2[1] {
			ymin := math.Min(p1[1], p2[1])
			edges = append(edges, &hachureEdgeEntry{
				ymin:   ymin,
				ymax:   math.Max(p1[1], p2[1]),
				x:      chooseFloat(ymin == p1[1], p1[0], p2[0]),
				islope: (p2[0] - p1[0]) / (p2[1] - p1[1]),
			})
		}
	}
	// ECMAScript's Array.prototype.sort is stable. Stable ties are observable
	// for coincident polygon edges, so do not use sort.Slice here.
	sort.SliceStable(edges, func(i, j int) bool {
		e1, e2 := edges[i], edges[j]
		if e1.ymin != e2.ymin {
			return e1.ymin < e2.ymin
		}
		if e1.x != e2.x {
			return e1.x < e2.x
		}
		return e1.ymax < e2.ymax
	})
	if len(edges) == 0 {
		return lines
	}

	activeEdges := make([]activeHachureEdgeEntry, 0)
	y := edges[0].ymin
	for len(activeEdges) > 0 || len(edges) > 0 {
		if len(edges) > 0 {
			ix := -1
			for i := range edges {
				if edges[i].ymin > y {
					break
				}
				ix = i
			}
			if ix >= 0 {
				removed := edges[:ix+1]
				edges = edges[ix+1:]
				for _, edge := range removed {
					activeEdges = append(activeEdges, activeHachureEdgeEntry{s: y, edge: edge})
				}
			}
		}

		kept := activeEdges[:0]
		for _, ae := range activeEdges {
			if ae.edge.ymax > y {
				kept = append(kept, ae)
			}
		}
		activeEdges = kept
		sort.SliceStable(activeEdges, func(i, j int) bool {
			return activeEdges[i].edge.x < activeEdges[j].edge.x
		})

		if len(activeEdges) > 1 {
			for i := 0; i < len(activeEdges); i += 2 {
				next := i + 1
				if next >= len(activeEdges) {
					break
				}
				ce := activeEdges[i].edge
				ne := activeEdges[next].edge
				lines = append(lines, Line{{jsRound(ce.x), y}, {jsRound(ne.x), y}})
			}
		}

		y += gap
		for i := range activeEdges {
			ae := &activeEdges[i]
			ae.edge.x += gap * ae.edge.islope
		}
	}
	return lines
}

func chooseFloat(condition bool, ifTrue, ifFalse float64) float64 {
	if condition {
		return ifTrue
	}
	return ifFalse
}

func pointsJoinEqual(a, b Point) bool {
	for i := 0; i < 2; i++ {
		if a[i] == b[i] {
			continue
		}
		// JavaScript's Array#join renders every NaN as "NaN".
		if math.IsNaN(a[i]) && math.IsNaN(b[i]) {
			continue
		}
		return false
	}
	return true
}
