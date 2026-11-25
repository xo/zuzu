package graph

import (
	"cmp"
	"math"
	"slices"
)

// endGap is the least space along a side of a node between the ends of two
// edges that join the node on that side, so that the arrowhead of one edge
// does not cover the line of the other.
const endGap = 8.0

// edgeEnd is an end of an edge at a node, for spreadEnds: the edge, which
// end, and where the end is along the side of the node that it joins, and
// where the next point of the route is along that side.
type edgeEnd struct {
	edge   int
	target bool
	at     float64
	next   float64
}

// spreadEnds moves apart the ends of the edges that join a node on the same
// side of its box closer than endGap, along that side, and keeps their
// order, so that two edges that come to a node from about the same
// direction do not meet at their ends and cross there. An end moves back
// onto the outline of the shape along the line from the center of the
// node. An end at a port and an edge from a node to itself stay.
func spreadEnds(in *LayoutInput, out *Layout, ids map[string]int) {
	// sides holds the ends at each side of each node: 0 to 3 for the top,
	// the bottom, the left and the right.
	sides := map[[2]int][]edgeEnd{}
	for i, e := range in.Edges {
		a, b := ids[e.Source], ids[e.Target]
		pts := out.Edges[i].Points
		if a == b || len(pts) < 2 {
			continue
		}
		last := len(pts) - 1
		if e.SourcePort == nil {
			side, at, next := sideOf(out.Nodes[a], pts[0], pts[1])
			sides[[2]int{a, side}] = append(sides[[2]int{a, side}], edgeEnd{edge: i, at: at, next: next})
		}
		if e.TargetPort == nil {
			side, at, next := sideOf(out.Nodes[b], pts[last], pts[last-1])
			sides[[2]int{b, side}] = append(sides[[2]int{b, side}], edgeEnd{edge: i, target: true, at: at, next: next})
		}
	}
	keys := make([][2]int, 0, len(sides))
	for k := range sides {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(p, q [2]int) int { return cmp.Or(p[0]-q[0], p[1]-q[1]) })
	for _, k := range keys {
		ends := sides[k]
		if len(ends) < 2 {
			continue
		}
		slices.SortFunc(ends, func(p, q edgeEnd) int {
			return cmp.Or(cmp.Compare(p.at, q.at), cmp.Compare(p.next, q.next), p.edge-q.edge)
		})
		// The coordinate along the top and the bottom is x, and along
		// the left and the right y.
		n := out.Nodes[k[0]]
		half := n.Width / 2
		if k[1] >= 2 {
			half = n.Height / 2
		}
		ats := make([]float64, len(ends))
		for j, en := range ends {
			ats[j] = en.at
		}
		if !spreadRun(ats, half) {
			continue
		}
		kind := in.Nodes[k[0]].Shape
		for j, en := range ends {
			if ats[j] == en.at {
				continue
			}
			pts := out.Edges[en.edge].Points
			p := &pts[0]
			if en.target {
				p = &pts[len(pts)-1]
			}
			var x, y float64
			if k[1] < 2 {
				x, y = n.X+ats[j], p[1]
			} else {
				x, y = p[0], n.Y+ats[j]
			}
			p[0], p[1] = clip(n, kind, x, y)
			setEndAngles(&out.Edges[en.edge])
		}
	}
}

// sideOf returns the side of the box of a node that an end of an edge
// joins, 0 to 3 for the top, the bottom, the left and the right, and where
// the end and the next point of the route are along that side, from the
// center of the node.
func sideOf(n NodePlace, p, next [2]float64) (int, float64, float64) {
	dx, dy := p[0]-n.X, p[1]-n.Y
	if math.Abs(dy)*n.Width >= math.Abs(dx)*n.Height {
		side := 0
		if dy > 0 {
			side = 1
		}
		return side, dx, next[0] - n.X
	}
	side := 2
	if dx > 0 {
		side = 3
	}
	return side, dy, next[1] - n.Y
}

// spreadRun moves the values of a sorted list apart, so that each two
// next to each other are at least endGap apart, inside -half to half, less
// a margin at each corner. Each run of values that are too close spreads
// around the mean of its values, and a run that comes too close to the
// next joins it. It reports whether it moved a value.
func spreadRun(ats []float64, half float64) bool {
	lo, hi := -half+math.Min(endGap/2, half/2), half-math.Min(endGap/2, half/2)
	// A run holds the values from first to first+n-1, with the sum of
	// their first places.
	type run struct {
		first, n int
		sum      float64
	}
	gap := func(n int) float64 {
		if n > 1 && float64(n-1)*endGap > hi-lo {
			return (hi - lo) / float64(n-1)
		}
		return endGap
	}
	start := func(r run) float64 {
		span := float64(r.n-1) * gap(r.n)
		return math.Min(math.Max(r.sum/float64(r.n)-span/2, lo), hi-span)
	}
	runs := make([]run, len(ats))
	for i, v := range ats {
		runs[i] = run{first: i, n: 1, sum: v}
	}
	for joined := true; joined; {
		joined = false
		for i := 0; i+1 < len(runs); i++ {
			a, b := runs[i], runs[i+1]
			end := start(a) + float64(a.n-1)*gap(a.n)
			if a.n == 1 && b.n == 1 {
				end = a.sum
			}
			next := start(b)
			if b.n == 1 {
				next = b.sum
			}
			if next-end < endGap-1e-9 {
				runs[i] = run{first: a.first, n: a.n + b.n, sum: a.sum + b.sum}
				runs = append(runs[:i+1], runs[i+2:]...)
				joined = true
				break
			}
		}
	}
	moved := false
	for _, r := range runs {
		if r.n == 1 {
			continue
		}
		s, g := start(r), gap(r.n)
		for k := range r.n {
			if v := s + float64(k)*g; math.Abs(v-ats[r.first+k]) > 1e-9 {
				ats[r.first+k], moved = v, true
			}
		}
	}
	return moved
}

// setEndAngles sets the ends of a route and their angles from its points.
func setEndAngles(ep *EdgePlace) {
	pts := ep.Points
	last := len(pts) - 1
	ep.EndX, ep.EndY = pts[last][0], pts[last][1]
	ep.EndAngle = math.Atan2(pts[last][1]-pts[last-1][1], pts[last][0]-pts[last-1][0]) * 180 / math.Pi
	ep.StartX, ep.StartY = pts[0][0], pts[0][1]
	ep.StartAngle = math.Atan2(pts[0][1]-pts[1][1], pts[0][0]-pts[1][0]) * 180 / math.Pi
}
