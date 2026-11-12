package graph

import (
	"math"
	"slices"
)

// The gaps of the labels that take no space in the ranks, in pixels: a
// label at an end starts endAlong past the end along the edge and keeps
// endBeside from its line, so that it clears the arrowhead, labels at the
// same end that move apart keep endApart from each other, and an xlabel
// keeps xGap from its edge.
const (
	endAlong  = 3.0
	endBeside = 4.0
	endApart  = 2.0
	xGap      = 3.0
)

// endSteps is the number of steps that a label at an end can move away from
// its first places, along its edge and away from it.
const endSteps = 6

// rect is a box from x1, y1 to x2, y2.
type rect struct{ x1, y1, x2, y2 float64 }

// rectAt returns the box of a size around a center.
func rectAt(c [2]float64, w, h float64) rect {
	return rect{c[0] - w/2, c[1] - h/2, c[0] + w/2, c[1] + h/2}
}

// overlaps reports whether two boxes share more than a border.
func (a rect) overlaps(b rect) bool {
	return a.x1 < b.x2 && b.x1 < a.x2 && a.y1 < b.y2 && b.y1 < a.y2
}

// inside returns the part of the box that lies deeper in it than a point,
// the box cut on each side by the distance from the point to its nearest
// side. It returns a box that overlaps nothing when no part lies deeper,
// and the whole box when the point is outside it.
func (a rect) inside(p [2]float64) rect {
	d := min(p[0]-a.x1, a.x2-p[0], p[1]-a.y1, a.y2-p[1])
	if d <= 0 {
		return a
	}
	if 2*d >= a.x2-a.x1 || 2*d >= a.y2-a.y1 {
		inf := math.Inf(1)
		return rect{inf, inf, inf, inf}
	}
	return rect{a.x1 + d, a.y1 + d, a.x2 - d, a.y2 - d}
}

// cuts reports whether the segment from p to q passes through the box, by
// clipping the segment to the box.
func (a rect) cuts(p, q [2]float64) bool {
	t0, t1 := 0.0, 1.0
	d := [2]float64{q[0] - p[0], q[1] - p[1]}
	for _, s := range []struct{ d, lo, hi, at float64 }{
		{d[0], a.x1, a.x2, p[0]}, {d[1], a.y1, a.y2, p[1]},
	} {
		if s.d == 0 {
			if s.at <= s.lo || s.at >= s.hi {
				return false
			}
			continue
		}
		u, v := (s.lo-s.at)/s.d, (s.hi-s.at)/s.d
		if u > v {
			u, v = v, u
		}
		t0, t1 = math.Max(t0, u), math.Min(t1, v)
		if t0 >= t1 {
			return false
		}
	}
	return true
}

// reach returns how far a box of a size reaches from its center in a
// direction of length 1.
func reach(d [2]float64, w, h float64) float64 {
	return math.Abs(d[0])*w/2 + math.Abs(d[1])*h/2
}

// smooth returns the points that a B-spline through the points of a route
// passes near: each point between the ends pulled toward its neighbors.
func smooth(pts [][2]float64) [][2]float64 {
	out := make([][2]float64, len(pts))
	for j, p := range pts {
		if j == 0 || j == len(pts)-1 {
			out[j] = p
			continue
		}
		out[j] = [2]float64{(pts[j-1][0] + 4*p[0] + pts[j+1][0]) / 6, (pts[j-1][1] + 4*p[1] + pts[j+1][1]) / 6}
	}
	return out
}

// middle returns the point halfway along a route, by its length, and the
// direction of the route there.
func middle(pts [][2]float64) ([2]float64, [2]float64) {
	total := 0.0
	for j := 1; j < len(pts); j++ {
		total += math.Hypot(pts[j][0]-pts[j-1][0], pts[j][1]-pts[j-1][1])
	}
	left := total / 2
	for j := 1; j < len(pts); j++ {
		p, q := pts[j-1], pts[j]
		n := math.Hypot(q[0]-p[0], q[1]-p[1])
		if n == 0 {
			continue
		}
		d := [2]float64{(q[0] - p[0]) / n, (q[1] - p[1]) / n}
		if left <= n || j == len(pts)-1 {
			t := math.Min(left, n)
			return [2]float64{p[0] + d[0]*t, p[1] + d[1]*t}, d
		}
		left -= n
	}
	return pts[0], [2]float64{1, 0}
}

// endPlaces returns the places to try for the center of a label of a size
// at an end of an edge: e is the end, in the direction into the edge, and
// right the side at the right of the edge as it runs from its tail to its
// head. A fixed distance puts the label 10 times dist from the end, at
// angle degrees counterclockwise from the edge, as dot does. Otherwise the
// label sits beside the edge just past the end, at its right and then at
// its left.
func endPlaces(e, in, right [2]float64, w, h, dist, angle float64) [][2]float64 {
	if dist > 0 {
		a := angle * math.Pi / 180
		// A turn counterclockwise on the screen, where y runs down.
		d := [2]float64{in[0]*math.Cos(a) + in[1]*math.Sin(a), -in[0]*math.Sin(a) + in[1]*math.Cos(a)}
		return [][2]float64{{e[0] + 10*dist*d[0], e[1] + 10*dist*d[1]}}
	}
	along := endAlong + reach(in, w, h)
	var out [][2]float64
	for _, s := range []float64{1, -1} {
		side := [2]float64{s * right[0], s * right[1]}
		beside := endBeside + reach(side, w, h)
		out = append(out, [2]float64{e[0] + in[0]*along + side[0]*beside, e[1] + in[1]*along + side[1]*beside})
	}
	return out
}

// spot is a place to try for the center of a label at an end of an edge,
// and the number of steps that it lies from the first places.
type spot struct {
	c     [2]float64
	steps int
}

// endSpread returns more places for the center of a label of a size at an
// end of an edge, for when the places of endPlaces cover a node or another
// label, as when the heads of many edges end at the same port. Each place
// keeps to a side of the edge, and lies up to endSteps steps farther from
// the edge and farther along it than the place of endPlaces on that side. A
// step is the height or the width of the label, the smaller one, and
// endApart. The nearest places come first.
func endSpread(e, in, right [2]float64, w, h float64) []spot {
	type place struct {
		s spot
		d int
	}
	along, step := endAlong+reach(in, w, h), math.Min(w, h)+endApart
	var out []place
	for a := range endSteps + 1 {
		for b := range endSteps + 1 {
			if a == 0 && b == 0 {
				continue
			}
			for _, s := range []float64{1, -1} {
				side := [2]float64{s * right[0], s * right[1]}
				k, m := along+float64(a)*step, endBeside+reach(side, w, h)+float64(b)*step
				c := [2]float64{e[0] + in[0]*k + side[0]*m, e[1] + in[1]*k + side[1]*m}
				out = append(out, place{spot{c, a + b}, a*a + b*b})
			}
		}
	}
	slices.SortStableFunc(out, func(p, q place) int { return p.d - q.d })
	spots := make([]spot, len(out))
	for j, p := range out {
		spots[j] = p.s
	}
	return spots
}

// ends returns the head and the tail of an edge: each end, the direction
// into the edge there, and the side at the right of the edge.
func ends(ep *EdgePlace) (head, headIn, headRight, tail, tailIn, tailRight [2]float64) {
	ha, ta := ep.EndAngle*math.Pi/180, ep.StartAngle*math.Pi/180
	head, tail = [2]float64{ep.EndX, ep.EndY}, [2]float64{ep.StartX, ep.StartY}
	// The angle at the head is the direction of the edge into the node,
	// and the one at the tail the direction out of the edge into the
	// node, against the run of the edge.
	run := [2]float64{math.Cos(ha), math.Sin(ha)}
	headIn, headRight = [2]float64{-run[0], -run[1]}, [2]float64{-run[1], run[0]}
	run = [2]float64{-math.Cos(ta), -math.Sin(ta)}
	tailIn, tailRight = run, [2]float64{-run[1], run[0]}
	return
}

// placeEnds sets the places of the labels at the ends of an edge to the
// first place that endPlaces gives, with no check of what they cover.
func placeEnds(e LayoutEdge, ep *EdgePlace) {
	if len(ep.Points) < 2 {
		return
	}
	head, headIn, headRight, tail, tailIn, tailRight := ends(ep)
	if e.HeadLabelWidth > 0 {
		c := endPlaces(head, headIn, headRight, e.HeadLabelWidth, e.HeadLabelHeight, e.LabelDistance, e.LabelAngle)[0]
		ep.HeadLabelX, ep.HeadLabelY = c[0], c[1]
	}
	if e.TailLabelWidth > 0 {
		c := endPlaces(tail, tailIn, tailRight, e.TailLabelWidth, e.TailLabelHeight, e.LabelDistance, e.LabelAngle)[0]
		ep.TailLabelX, ep.TailLabelY = c[0], c[1]
	}
}

// placeLabels places the labels that take no space in the ranks: the
// labels at the ends of each edge, the xlabel of each edge, and the
// xlabel of each node, as dot does after its layout. Each label goes at
// the first of a few places where it covers no node, no other label and no
// edge, or else at the place that covers the least.
func placeLabels(in *LayoutInput, out *Layout) {
	var boxes []rect
	for _, n := range out.Nodes {
		boxes = append(boxes, rectAt([2]float64{n.X, n.Y}, n.Width, n.Height))
	}
	for i, e := range in.Edges {
		if e.LabelWidth > 0 {
			boxes = append(boxes, rectAt([2]float64{out.Edges[i].LabelX, out.Edges[i].LabelY}, e.LabelWidth, e.LabelHeight))
		}
	}
	routes := make([][][2]float64, len(out.Edges))
	for i, ep := range out.Edges {
		routes[i] = smooth(ep.Points)
	}
	index := make(map[string]int, len(in.Nodes))
	for i, n := range in.Nodes {
		index[n.ID] = i
	}
	// cost returns the number of nodes and labels that a box covers, and
	// the number of edges that it cuts. The node at index own counts only
	// where the box covers its part inner.
	cost := func(b rect, own int, inner rect) (int, int) {
		covers, cuts := 0, 0
		for j, o := range boxes {
			if j == own {
				o = inner
			}
			if b.overlaps(o) {
				covers++
			}
		}
		for _, r := range routes {
			for j := 1; j < len(r); j++ {
				if b.cuts(r[j-1], r[j]) {
					cuts++
					break
				}
			}
		}
		return covers, cuts
	}
	// choose returns the place that covers and cuts the least, where a
	// node or a label that it covers counts as two edges that it cuts.
	choose := func(w, h float64, places [][2]float64) [2]float64 {
		best, least := places[0], math.MaxInt
		for _, p := range places {
			if covers, cuts := cost(rectAt(p, w, h), -1, rect{}); 2*covers+cuts < least {
				best, least = p, 2*covers+cuts
				if least == 0 {
					break
				}
			}
		}
		return best
	}
	pick := func(w, h float64, places [][2]float64) [2]float64 {
		best := choose(w, h, places)
		boxes = append(boxes, rectAt(best, w, h))
		return best
	}
	// pickEnd places a label at an end of an edge, at the node own. When
	// the place that choose gives covers another node or label, as when
	// many edges end at the same port, the label moves to the place of
	// endPlaces or endSpread that covers the fewest of them. Of those, it
	// takes the one where the edges that it cuts and its steps away from
	// the end add up to the least, and of those the nearest. Here a label
	// covers a box only where it reaches more than endApart into it, so
	// that a label that only touches a box stays where it is. An edge can
	// end inside the margin of its node, so the label can cover as much of
	// its own node as lies outside the end.
	pickEnd := func(e, into, right [2]float64, w, h, dist, angle float64, own string) [2]float64 {
		skip, ok := index[own]
		if !ok {
			skip = -1
		}
		var inner rect
		if skip >= 0 {
			inner = boxes[skip].inside(e)
		}
		core := func(c [2]float64) rect {
			return rectAt(c, max(w-2*endApart, 0), max(h-2*endApart, 0))
		}
		places := endPlaces(e, into, right, w, h, dist, angle)
		best := choose(w, h, places)
		if covers, cuts := cost(core(best), skip, inner); covers > 0 && dist <= 0 {
			spots := make([]spot, 0, len(places))
			for _, p := range places {
				spots = append(spots, spot{p, 0})
			}
			for _, p := range append(spots, endSpread(e, into, right, w, h)...) {
				c, k := cost(core(p.c), skip, inner)
				if c < covers || c == covers && k+p.steps < cuts {
					best, covers, cuts = p.c, c, k+p.steps
				}
			}
		}
		boxes = append(boxes, rectAt(best, w, h))
		return best
	}
	for i, e := range in.Edges {
		ep := &out.Edges[i]
		if len(ep.Points) < 2 {
			continue
		}
		head, headIn, headRight, tail, tailIn, tailRight := ends(ep)
		if w, h := e.HeadLabelWidth, e.HeadLabelHeight; w > 0 {
			c := pickEnd(head, headIn, headRight, w, h, e.LabelDistance, e.LabelAngle, e.Target)
			ep.HeadLabelX, ep.HeadLabelY = c[0], c[1]
		}
		if w, h := e.TailLabelWidth, e.TailLabelHeight; w > 0 {
			c := pickEnd(tail, tailIn, tailRight, w, h, e.LabelDistance, e.LabelAngle, e.Source)
			ep.TailLabelX, ep.TailLabelY = c[0], c[1]
		}
		if w, h := e.XLabelWidth, e.XLabelHeight; w > 0 {
			m, d := middle(routes[i])
			// Beside the middle on each side, and then farther out, past
			// what is beside the edge.
			var places [][2]float64
			for step := range 3 {
				for _, s := range []float64{1, -1} {
					side := [2]float64{-s * d[1], s * d[0]}
					k := xGap + reach(side, w, h) + float64(step)*(reach(side, w, h)+xGap)
					places = append(places, [2]float64{m[0] + side[0]*k, m[1] + side[1]*k})
				}
			}
			if e.LabelWidth > 0 {
				// Past the label of the edge, on each side.
				l := [2]float64{ep.LabelX, ep.LabelY}
				for _, s := range []float64{1, -1} {
					side := [2]float64{-s * d[1], s * d[0]}
					k := reach(side, e.LabelWidth, e.LabelHeight) + xGap + reach(side, w, h)
					places = append(places, [2]float64{l[0] + side[0]*k, l[1] + side[1]*k})
				}
			}
			c := pick(w, h, places)
			ep.XLabelX, ep.XLabelY = c[0], c[1]
		}
	}
	for i, n := range in.Nodes {
		w, h := n.XLabelWidth, n.XLabelHeight
		if !(w > 0) {
			continue
		}
		np := &out.Nodes[i]
		x1, y1, x2, y2 := np.X-np.Width/2-w/2, np.Y-np.Height/2-h/2, np.X+np.Width/2+w/2, np.Y+np.Height/2+h/2
		c := pick(w, h, [][2]float64{{x1, y1}, {x2, y1}, {x1, y2}, {x2, y2}, {x1, np.Y}, {x2, np.Y}})
		np.XLabelX, np.XLabelY = c[0], c[1]
	}
}

// labelBounds calls add with the box of each label of a layout that takes
// no space in the ranks.
func labelBounds(in *LayoutInput, out *Layout, add func(x1, y1, x2, y2 float64)) {
	box := func(x, y, w, h float64) {
		if w > 0 {
			add(x-w/2, y-h/2, x+w/2, y+h/2)
		}
	}
	for i, n := range in.Nodes {
		box(out.Nodes[i].XLabelX, out.Nodes[i].XLabelY, n.XLabelWidth, n.XLabelHeight)
	}
	for i, e := range in.Edges {
		ep := out.Edges[i]
		box(ep.HeadLabelX, ep.HeadLabelY, e.HeadLabelWidth, e.HeadLabelHeight)
		box(ep.TailLabelX, ep.TailLabelY, e.TailLabelWidth, e.TailLabelHeight)
		box(ep.XLabelX, ep.XLabelY, e.XLabelWidth, e.XLabelHeight)
	}
}

// shiftLabels moves the xlabel of a node.
func (n *NodePlace) shiftLabels(dx, dy float64) {
	n.XLabelX, n.XLabelY = n.XLabelX+dx, n.XLabelY+dy
}

// shiftLabels moves the labels of an edge that take no space in the ranks.
func (e *EdgePlace) shiftLabels(dx, dy float64) {
	e.HeadLabelX, e.HeadLabelY = e.HeadLabelX+dx, e.HeadLabelY+dy
	e.TailLabelX, e.TailLabelY = e.TailLabelX+dx, e.TailLabelY+dy
	e.XLabelX, e.XLabelY = e.XLabelX+dx, e.XLabelY+dy
}
