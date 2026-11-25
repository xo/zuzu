package graph

import (
	"cmp"
	"maps"
	"math"
	"slices"
)

// laneRouter routes the edges at right angles on a grid of lines: the
// centers of the nodes, the middles of the gaps between the ranks, and the
// sides inside each lane and between two lanes, with more lines beside and
// between them for the routes that run side by side and for the ports of a
// node.
// A route does not cross a node, except the nodes at its ends. It shares
// no part of a line with another route, and it does not bend where
// another route passes, nor pass where another route bends, so that two
// routes only cross. Each bend and each crossing costs more.
type laneRouter struct {
	xs, ys []float64
	// block holds the nodes that each step of the grid crosses: the step
	// from a point to the next point across, then to the next point
	// along, by the index of the point.
	block [2][][]int
	// extra holds the cost of each step that other routes and labels take.
	extra [2][]float64
	// used holds the steps that a route takes, as block, and mark holds
	// how the routes pass each point.
	used [2][]bool
	mark []uint8
	// inside holds the node whose box holds each point, or -1.
	inside []int
	// fillX and fillY say which lines fillLines added. A route does not
	// come into its target node on such a line, so that the arrowheads on
	// a side of a node stay laneTrack apart.
	fillX, fillY []bool
	// loops holds the route of each loop, by the index of its edge.
	loops map[int]laneRoute
}

// The marks of a point of the grid.
const (
	// markAcross and markAlong mark a point that a route passes straight,
	// across or along the ranks.
	markAcross uint8 = 1 << iota
	markAlong
	// markBend marks a point where a route bends.
	markBend
)

// The spaces of the lines beside the main lines of the grid, in pixels.
const (
	// laneTrack is the space between two routes side by side, and between
	// two ports of a node, as the space EDGE_GAP of Mermaid 12.3.
	laneTrack = 12.0
	// laneTrackClear is the least space between a line beside and any
	// other line, and between two lines that fillLines adds.
	laneTrackClear = 6.0
	// lanePortClear is the least space between a port and a corner of its
	// node, and between a line beside the middle of a gap and the ranks on
	// each side of the gap.
	lanePortClear = 3.0
	// laneStartCost is the cost of each pixel from the center of a node to
	// the port where a route leaves it or comes into it.
	laneStartCost = 1.5
	// laneNear is the cost of a step beside a step that another route
	// takes, closer than laneTrack.
	laneNear = 8.0
	// laneMargin is the space around a node where a route does not run,
	// as the boxes of the nodes in block hold it.
	laneMargin = 2.0
)

// snapLoop moves a loop onto the lines of the grid closest to it, when
// they keep the shape of the loop: its two sides across inside its node,
// apart, on lines that no loop of the node before it takes, in taken, and
// its side along out of its node. Otherwise the loop keeps its place.
func (l *laneLayout) snapLoop(r *laneRouter, i int, rt laneRoute, taken []float64) laneRoute {
	v := l.ends[i][0]
	b := l.nodeBox(v)
	// near returns the line inside the node closest to a place, on its
	// side of the center, that no loop takes, or NaN for none.
	near := func(at float64, before bool) float64 {
		best := math.NaN()
		for _, y := range r.ys {
			if y <= b.y1 || y >= b.y2 || slices.Contains(taken, y) || before && y > l.y[v] || !before && y < l.y[v] {
				continue
			}
			if math.IsNaN(best) || math.Abs(y-at) < math.Abs(best-at) {
				best = y
			}
		}
		return best
	}
	y1, y2 := near(rt.pts[0][1], true), near(rt.pts[2][1], false)
	x1 := r.xs[nearLine(r.xs, rt.pts[1][0])]
	if math.IsNaN(y1) || math.IsNaN(y2) || y1 >= y2 || x1 > b.x1-laneMargin && x1 < b.x2+laneMargin || (x1 > l.x[v]) != (rt.pts[1][0] > l.x[v]) {
		return rt
	}
	x := rt.pts[0][0]
	lx := rt.label[0] + x1 - rt.pts[1][0]
	return laneRoute{pts: [][2]float64{{x, y1}, {x1, y1}, {x1, y2}, {x, y2}}, label: [2]float64{lx, rt.label[1]}}
}

// nearLine returns the index of the line closest to a place.
func nearLine(lines []float64, v float64) int {
	i, _ := slices.BinarySearch(lines, v)
	switch {
	case i == len(lines):
		return i - 1
	case i > 0 && v-lines[i-1] < lines[i]-v:
		return i - 1
	}
	return i
}

// nodeBox returns the box of a node in the frame where the ranks run down.
func (l *laneLayout) nodeBox(v int) rect {
	return rectAt([2]float64{l.x[v], l.y[v]}, l.w[v], l.h[v])
}

// newRouter builds the grid of a layout. Its main lines come in this
// order, and each one that is closer than laneTrackClear to a line before
// it takes no place: the centers of the nodes, the lines of the loops, the
// ports of each node, the middles of the gaps between two ranks, the sides
// of the lanes, the middles of the gaps between two nodes of a lane, the
// lines around the ranks, and the lines beside the middle of each gap
// between two ranks. A port is lanePortClear from a corner of its node at
// least, and a line beside the middle of a gap is that far from the ranks.
// A node then takes the ports that ports gives it, and fillLines adds the
// lines between the main lines. Each loop then takes the lines of the grid
// closest to it, where they keep its shape, so that the routes keep apart
// from it.
func (l *laneLayout) newRouter(g *laneGeom) *laneRouter {
	var xs, ys []float64
	add := func(lines []float64, vs ...float64) []float64 {
		for _, v := range vs {
			if clearOf(lines, v) {
				lines = append(lines, v)
			}
		}
		return lines
	}
	for v := range l.in.Nodes {
		xs, ys = add(xs, l.x[v]), add(ys, l.y[v])
	}
	loops := map[int]laneRoute{}
	for i, e := range l.ends {
		if a := e[0]; a >= 0 && a == e[1] {
			lw, _ := l.labelSize(i)
			loops[i] = l.loop(i, lw)
			xs = add(xs, loops[i].pts[1][0])
			ys = add(ys, loops[i].pts[0][1], loops[i].pts[2][1])
		}
	}
	// beside adds the lines at each space of laneTrack on both sides of a
	// line, up to a reach. A reach shorter than laneTrack and at least
	// laneTrackClear takes one line on each side at its end, so that a
	// small node has ports too.
	beside := func(lines []float64, at, reach float64) []float64 {
		step := laneTrack
		if reach < laneTrack && reach >= laneTrackClear {
			step = reach
		}
		for k := 1; float64(k)*step <= reach; k++ {
			lines = add(lines, at-float64(k)*step, at+float64(k)*step)
		}
		return lines
	}
	for v := range l.in.Nodes {
		xs = beside(xs, l.x[v], l.w[v]/2-lanePortClear)
		ys = beside(ys, l.y[v], l.h[v]/2-lanePortClear)
	}
	ys = add(ys, g.gapMid...)
	for li := range l.lanes {
		xs = add(xs, g.x0[li]+g.low[li]-g.high[li]+lanePad/2, g.x1[li]-lanePad/2)
		if li > 0 {
			xs = add(xs, g.x0[li]-laneGap/2)
		}
	}
	for _, ly := range l.layers {
		for k := 1; k < len(ly); k++ {
			a, b := ly[k-1], ly[k]
			if l.lane[a] == l.lane[b] {
				xs = add(xs, (l.x[a]+l.w[a]/2+l.x[b]-l.w[b]/2)/2)
			}
		}
	}
	ys = add(ys, -g.flowPad+lanePad/2, g.flowLen+g.flowPad-lanePad/2)
	for r, mid := range g.gapMid {
		ys = beside(ys, mid, g.gapLen[r]/2-lanePortClear)
	}
	for v := range l.in.Nodes {
		b := l.nodeBox(v)
		xs = l.ports(xs, l.x[v], b.x1, b.x2)
		ys = l.ports(ys, l.y[v], b.y1, b.y2)
	}
	var sidesX, sidesY [][2]float64
	for v := range l.in.Nodes {
		b := l.nodeBox(v)
		sidesX, sidesY = append(sidesX, [2]float64{b.x1, b.x2}), append(sidesY, [2]float64{b.y1, b.y2})
	}
	r := &laneRouter{loops: map[int]laneRoute{}}
	r.xs, r.fillX = fillLines(xs, sidesX)
	r.ys, r.fillY = fillLines(ys, sidesY)
	taken := map[int][]float64{}
	for _, i := range slices.Sorted(maps.Keys(loops)) {
		v := l.ends[i][0]
		rt := l.snapLoop(r, i, loops[i], taken[v])
		r.loops[i] = rt
		taken[v] = append(taken[v], rt.pts[0][1], rt.pts[2][1])
	}
	nx, ny := len(r.xs), len(r.ys)
	for k := range 2 {
		r.block[k] = make([][]int, nx*ny)
		r.extra[k] = make([]float64, nx*ny)
		r.used[k] = make([]bool, nx*ny)
	}
	r.mark = make([]uint8, nx*ny)
	r.inside = make([]int, nx*ny)
	for i := range r.inside {
		r.inside[i] = -1
	}
	for v := range l.in.Nodes {
		b := l.nodeBox(v)
		for j, y := range r.ys {
			for i, x := range r.xs {
				if x > b.x1 && x < b.x2 && y > b.y1 && y < b.y2 {
					r.inside[j*nx+i] = v
				}
			}
		}
		b = rect{b.x1 - laneMargin, b.y1 - laneMargin, b.x2 + laneMargin, b.y2 + laneMargin}
		i1, i2 := nearLine(r.xs, b.x1)-1, nearLine(r.xs, b.x2)+1
		j1, j2 := nearLine(r.ys, b.y1)-1, nearLine(r.ys, b.y2)+1
		for j := max(j1, 0); j <= min(j2, ny-1); j++ {
			for i := max(i1, 0); i <= min(i2, nx-1); i++ {
				p := [2]float64{r.xs[i], r.ys[j]}
				if i+1 < nx && b.cuts(p, [2]float64{r.xs[i+1], r.ys[j]}) {
					r.block[0][j*nx+i] = append(r.block[0][j*nx+i], v)
				}
				if j+1 < ny && b.cuts(p, [2]float64{r.xs[i], r.ys[j+1]}) {
					r.block[1][j*nx+i] = append(r.block[1][j*nx+i], v)
				}
			}
		}
	}
	return r
}

// ports returns the lines of a grid with the ports that a node needs
// across one of its sides, from lo to hi with its center at c: its center
// when no line is inside it, and a port on each side of its center when
// fewer than 3 lines are inside it. A port that the lines outside the node
// kept out takes its place all the same, when it is laneTrackClear from
// each line inside the node, so that each side of a small node has more
// than one port.
func (l *laneLayout) ports(lines []float64, c, lo, hi float64) []float64 {
	var in []float64
	for _, v := range lines {
		if v > lo && v < hi {
			in = append(in, v)
		}
	}
	if len(in) == 0 {
		in = append(in, c)
		lines = append(lines, c)
	}
	reach := math.Min(laneTrack, (hi-lo)/2-lanePortClear)
	if len(in) >= 3 || reach < laneTrackClear {
		return lines
	}
	for _, v := range []float64{c - reach, c + reach} {
		if clearOf(in, v) {
			in, lines = append(in, v), append(lines, v)
		}
	}
	return lines
}

// fillLines sorts the lines of a grid and adds lines between each two
// lines or sides of nodes, in sides, that are 2 times laneTrackClear apart
// or more, as many as fit laneTrackClear apart, at even spaces, so that
// routes can run side by side wherever there is room. A line outside every
// node and closer than laneMargin to a side of a node takes no place, as a
// route cannot run along it. It returns the lines, and which of them it
// added.
func fillLines(lines []float64, sides [][2]float64) ([]float64, []bool) {
	marks := slices.Clone(lines)
	for _, s := range sides {
		marks = append(marks, s[0], s[1])
	}
	slices.Sort(marks)
	slices.Sort(lines)
	var all []float64
	var fill []bool
	for i := 1; i < len(marks); i++ {
		a, b := marks[i-1], marks[i]
		k := int(math.Floor((b-a)/laneTrackClear)) - 1
		for j := 1; j <= k; j++ {
			all, fill = append(all, a+(b-a)*float64(j)/float64(k+1)), append(fill, true)
		}
	}
	for _, v := range lines {
		all, fill = append(all, v), append(fill, false)
	}
	idx := make([]int, len(all))
	for i := range idx {
		idx[i] = i
	}
	slices.SortStableFunc(idx, func(a, b int) int { return cmp.Compare(all[a], all[b]) })
	var out []float64
	var outFill []bool
	for _, i := range idx {
		v := all[i]
		inside, near := false, false
		for _, s := range sides {
			inside = inside || v > s[0] && v < s[1]
			near = near || v > s[0]-laneMargin && v <= s[0] || v >= s[1] && v < s[1]+laneMargin
		}
		if near && !inside || len(out) > 0 && v-out[len(out)-1] < 0.01 {
			continue
		}
		out, outFill = append(out, v), append(outFill, fill[i])
	}
	return out, outFill
}

// clearOf reports whether a place is at least laneTrackClear from each of
// some lines.
func clearOf(lines []float64, v float64) bool {
	for _, w := range lines {
		if math.Abs(v-w) < laneTrackClear {
			return false
		}
	}
	return true
}

// routeState is a place of the search of a route: its cost, its cost plus
// the least cost from it to a goal, and a point of the grid with the
// direction that the route came in, 0 to 3, or 4 at the start, as at.
type routeState struct {
	cost, guess float64
	at          int
}

// routeHeap is a binary heap of the places of the search, the one of the
// least guess first, and of two that guess the same, the one of the lower
// index.
type routeHeap []routeState

func (h routeHeap) less(i, j int) bool {
	if h[i].guess != h[j].guess {
		return h[i].guess < h[j].guess
	}
	return h[i].at < h[j].at
}

func (h *routeHeap) push(s routeState) {
	*h = append(*h, s)
	for i := len(*h) - 1; i > 0; {
		p := (i - 1) / 2
		if !h.less(i, p) {
			break
		}
		(*h)[i], (*h)[p] = (*h)[p], (*h)[i]
		i = p
	}
}

func (h *routeHeap) pop() routeState {
	old := *h
	top, n := old[0], len(old)-1
	old[0] = old[n]
	*h = old[:n]
	for i := 0; ; {
		c := 2*i + 1
		if c >= n {
			break
		}
		if c+1 < n && h.less(c+1, c) {
			c++
		}
		if !h.less(c, i) {
			break
		}
		(*h)[i], (*h)[c] = (*h)[c], (*h)[i]
		i = c
	}
	return top
}

// gridSteps are the moves on the grid: across forward and back, then along
// forward and back.
var gridSteps = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// route finds the cheapest route from any of the starts to any of the
// goals, by the search A* with the distance to the box of the goals. Each
// start has a cost of its own. goal holds the cost of each point that is a
// goal, and -1 for each other point. from and to are the centers of the
// ends. The route can cross the nodes in ok. It comes into the node
// target, or -1 for a cluster, on a line that fillLines did not add.
//
// A step across the ranks costs a little more the farther it is from the
// rank of the start, so that a route crosses the lanes early. A step of
// more than 5 pixels against the way from the start to the goal, when the
// goal is more than 10 pixels away that way, costs laneBack for each pixel
// along the ranks and half of that across them, as in Mermaid 12.3. A
// route that leaves its start against the way along the ranks, or comes
// into its goal that way, costs a little more too, so that a route leaves
// down and comes in from above when its goal is below.
//
// Outside the nodes in ok, a route of apartSteps shares no step with
// another route, and a route of apartPoints also does not bend where
// another route passes, nor pass where another route bends. It returns the
// points of the route, or nil for none.
func (r *laneRouter) route(starts []int, startCost, goal []float64, ok []int, target int, from, to [2]float64, apart int) [][2]float64 {
	nx, ny := len(r.xs), len(r.ys)
	n := nx * ny * 5
	dist := make([]float64, n)
	prev := make([]int, n)
	for i := range dist {
		dist[i], prev[i] = math.Inf(1), -1
	}
	// least is the least cost from a point to a goal: its distance to the
	// box of the goals, as each step costs its length at least.
	gx1, gy1, gx2, gy2 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for p, c := range goal {
		if c >= 0 {
			x, y := r.xs[p%nx], r.ys[p/nx]
			gx1, gy1, gx2, gy2 = math.Min(gx1, x), math.Min(gy1, y), math.Max(gx2, x), math.Max(gy2, y)
		}
	}
	least := func(p int) float64 {
		x, y := r.xs[p%nx], r.ys[p/nx]
		return math.Max(0, math.Max(gx1-x, x-gx2)) + math.Max(0, math.Max(gy1-y, y-gy2))
	}
	h := &routeHeap{}
	for k, p := range starts {
		s := p*5 + 4
		if startCost[k] < dist[s] {
			dist[s] = startCost[k]
			h.push(routeState{startCost[k], startCost[k] + least(p), s})
		}
	}
	free := func(bs []int) bool {
		for _, b := range bs {
			if !slices.Contains(ok, b) {
				return false
			}
		}
		return true
	}
	// hidden reports whether a point is inside a node at an end of the
	// route, where the route does not draw.
	hidden := func(p int) bool {
		return r.inside[p] >= 0 && slices.Contains(ok, r.inside[p])
	}
	// back is the step against the way to the goal.
	back := 3
	if to[1] < from[1] {
		back = 2
	}
	// against returns the cost of a step against the way to the goal.
	against := func(dx, dy float64) float64 {
		c := 0.0
		if way := to[1] - from[1]; way > 10 && dy < -5 || way < -10 && dy > 5 {
			c += math.Abs(dy) * laneBack
		}
		if way := to[0] - from[0]; way > 10 && dx < -5 || way < -10 && dx > 5 {
			c += math.Abs(dx) * laneBack / 2
		}
		return c
	}
	end := -1
	for len(*h) > 0 {
		s := h.pop()
		if s.cost > dist[s.at] {
			continue
		}
		p, d := s.at/5, s.at%5
		if goal[p] >= 0 && d < 4 {
			end = s.at
			break
		}
		i, j := p%nx, p/nx
		for k, st := range gridSteps {
			if d < 4 && k == d^1 {
				continue
			}
			ni, nj := i+st[0], j+st[1]
			if ni < 0 || nj < 0 || ni >= nx || nj >= ny {
				continue
			}
			// The step between two points is held by the lower one.
			a := min(i, ni) + min(j, nj)*nx
			axis := k / 2
			if !free(r.block[axis][a]) {
				continue
			}
			q := nj*nx + ni
			if target >= 0 && (r.inside[p] == target) != (r.inside[q] == target) && (axis == 0 && r.fillY[j] || axis == 1 && r.fillX[i]) {
				continue
			}
			if apart >= apartSteps && r.used[axis][a] && (!hidden(p) || !hidden(q)) {
				continue
			}
			if apart >= apartPoints {
				if d < 4 && !hidden(p) {
					bend := d != k
					if bend && r.mark[p] != 0 || !bend && r.mark[p]&markBend != 0 {
						continue
					}
				}
			}
			dx, dy := r.xs[ni]-r.xs[i], r.ys[nj]-r.ys[j]
			length := math.Abs(dx) + math.Abs(dy)
			c := s.cost + length + r.extra[axis][a] + against(dx, dy)
			if axis == 0 {
				c += 0.0002 * math.Abs(r.ys[j]-from[1]) * length
			}
			if d < 4 && d != k {
				c += laneBend
			}
			if d == k && !hidden(p) && r.mark[p]&(markAcross|markAlong) != 0 {
				// The route crosses another route.
				c += laneCross
			}
			if k == back && d == 4 {
				c += laneBend / 2
			}
			if goal[q] >= 0 {
				c += goal[q]
				if k == back {
					c += laneBend
				}
			}
			t := q*5 + k
			if c < dist[t] {
				dist[t], prev[t] = c, s.at
				h.push(routeState{c, c + least(q), t})
			}
		}
	}
	if end < 0 {
		return nil
	}
	var pts [][2]float64
	for s := end; s >= 0; s = prev[s] {
		p := s / 5
		pts = append(pts, [2]float64{r.xs[p%nx], r.ys[p/nx]})
	}
	slices.Reverse(pts)
	return straighten(pts)
}

// take adds the cost of the steps that a route and its label take to the
// routes after it. It marks the steps and the points that the route takes,
// except those inside the nodes at its ends, in ends.
func (r *laneRouter) take(pts [][2]float64, ends []int, label rect, hasLabel bool) {
	nx, ny := len(r.xs), len(r.ys)
	hidden := func(p int) bool {
		return r.inside[p] >= 0 && slices.Contains(ends, r.inside[p])
	}
	for k := 1; k < len(pts); k++ {
		a, b := pts[k-1], pts[k]
		i1, i2 := nearLine(r.xs, a[0]), nearLine(r.xs, b[0])
		j1, j2 := nearLine(r.ys, a[1]), nearLine(r.ys, b[1])
		if k > 1 && !hidden(j1*nx+i1) {
			r.mark[j1*nx+i1] |= markBend
		}
		axis, flag := 1, markAlong
		lo, hi := min(j1, j2), max(j1, j2)
		step := func(t int) (int, int) { return t*nx + i1, (t+1)*nx + i1 }
		if j1 == j2 {
			axis, flag = 0, markAcross
			lo, hi = min(i1, i2), max(i1, i2)
			step = func(t int) (int, int) { return j1*nx + t, j1*nx + t + 1 }
		}
		for t := lo; t < hi; t++ {
			p, q := step(t)
			r.extra[axis][p] += laneShare
			if hidden(p) && hidden(q) {
				continue
			}
			r.used[axis][p] = true
			r.beside(axis, p)
			if t > lo && !hidden(p) {
				r.mark[p] |= flag
			}
		}
	}
	if !hasLabel {
		return
	}
	for j := range ny {
		for i := range nx {
			p := [2]float64{r.xs[i], r.ys[j]}
			if i+1 < nx && label.cuts(p, [2]float64{r.xs[i+1], r.ys[j]}) {
				r.extra[0][j*nx+i] += laneShare
			}
			if j+1 < ny && label.cuts(p, [2]float64{r.xs[i], r.ys[j+1]}) {
				r.extra[1][j*nx+i] += laneShare
			}
		}
	}
}

// beside adds laneNear to the cost of each step on a line beside the line
// of a step that a route takes, closer than laneTrack, so that routes side
// by side keep laneTrack apart where they can.
func (r *laneRouter) beside(axis, p int) {
	nx := len(r.xs)
	i, j := p%nx, p/nx
	if axis == 0 {
		for _, nj := range []int{j - 1, j + 1} {
			if nj >= 0 && nj < len(r.ys) && math.Abs(r.ys[nj]-r.ys[j]) < laneTrack-0.5 {
				r.extra[0][nj*nx+i] += laneNear
			}
		}
		return
	}
	for _, ni := range []int{i - 1, i + 1} {
		if ni >= 0 && ni < nx && math.Abs(r.xs[ni]-r.xs[i]) < laneTrack-0.5 {
			r.extra[1][j*nx+ni] += laneNear
		}
	}
}

// straighten drops each point of a route that lies on the line of the
// points before and after it.
func straighten(pts [][2]float64) [][2]float64 {
	if len(pts) < 3 {
		return pts
	}
	out := [][2]float64{pts[0]}
	for k := 1; k < len(pts)-1; k++ {
		a, b, c := out[len(out)-1], pts[k], pts[k+1]
		if a[0] == b[0] && b[0] == c[0] || a[1] == b[1] && b[1] == c[1] {
			continue
		}
		out = append(out, b)
	}
	return append(out, pts[len(pts)-1])
}

// closestOn returns the point of a route closest to a place.
func closestOn(pts [][2]float64, at [2]float64) [2]float64 {
	best, bd := at, math.Inf(1)
	for k := 1; k < len(pts); k++ {
		a, b := pts[k-1], pts[k]
		dx, dy := b[0]-a[0], b[1]-a[1]
		t := 0.0
		if d := dx*dx + dy*dy; d > 0 {
			t = math.Max(0, math.Min(1, ((at[0]-a[0])*dx+(at[1]-a[1])*dy)/d))
		}
		p := [2]float64{a[0] + t*dx, a[1] + t*dy}
		if d := math.Hypot(p[0]-at[0], p[1]-at[1]); d < bd {
			best, bd = p, d
		}
	}
	return best
}

// labelOn returns the place of the label of an edge on its route: the
// place that the label wants, when the route passes it. A label across the
// ranks goes where the route crosses the middle of its gap, as near to that
// place as it can. Any other label goes in the middle of the longest part
// of the route.
func labelOn(pts [][2]float64, g *laneGeom, i int) [2]float64 {
	if !g.has[i] {
		return middleOf(pts)
	}
	at := g.spots[i]
	if p := closestOn(pts, at); math.Hypot(p[0]-at[0], p[1]-at[1]) < 0.5 {
		return p
	}
	if gp := g.gapOf[i]; gp >= 0 {
		y := g.gapMid[gp]
		best, bd := [2]float64{}, math.Inf(1)
		for k := 1; k < len(pts); k++ {
			a, b := pts[k-1], pts[k]
			if a[0] != b[0] || y <= math.Min(a[1], b[1]) || y >= math.Max(a[1], b[1]) {
				continue
			}
			if d := math.Abs(a[0] - at[0]); d < bd {
				best, bd = [2]float64{a[0], y}, d
			}
		}
		if !math.IsInf(bd, 1) {
			return best
		}
	}
	return middleOf(pts)
}

// middleOf returns the middle of the longest part of a route.
func middleOf(pts [][2]float64) [2]float64 {
	best, bl := [2]float64{}, -1.0
	for k := 1; k < len(pts); k++ {
		a, b := pts[k-1], pts[k]
		if d := math.Hypot(b[0]-a[0], b[1]-a[1]); d > bl {
			best, bl = [2]float64{(a[0] + b[0]) / 2, (a[1] + b[1]) / 2}, d
		}
	}
	return best
}

// laneRoute is the route of an edge in the frame where the ranks run down,
// before it stops at the borders of its ends, with the place of its label.
type laneRoute struct {
	pts   [][2]float64
	label [2]float64
}

// routeOrder returns the edges that are not loops in the order that they
// route, as Mermaid 12.3 routes them: the edges between two lanes first,
// then the shorter edges first, by the distance from the center of one end
// to the center of the other, and then in the order of the input.
func (l *laneLayout) routeOrder(g *laneGeom) []int {
	type item struct {
		i, cross int
		dist     float64
	}
	center := func(e int) ([2]float64, int) {
		if e >= 0 {
			return [2]float64{l.x[e], l.y[e]}, l.lanes[l.lane[e]]
		}
		b := g.boxes[-1-e]
		return [2]float64{(b.x1 + b.x2) / 2, (b.y1 + b.y2) / 2}, l.top(-1 - e)
	}
	var items []item
	for i, e := range l.ends {
		if e[0] >= 0 && e[0] == e[1] {
			continue
		}
		pa, la := center(e[0])
		pb, lb := center(e[1])
		it := item{i: i, dist: math.Abs(pb[0]-pa[0]) + math.Abs(pb[1]-pa[1])}
		if la != lb {
			it.cross = 1
		}
		items = append(items, it)
	}
	slices.SortStableFunc(items, func(a, b item) int {
		if a.cross != b.cross {
			return b.cross - a.cross
		}
		if math.Abs(a.dist-b.dist) > 1 {
			return cmp.Compare(a.dist, b.dist)
		}
		return a.i - b.i
	})
	out := make([]int, len(items))
	for k, it := range items {
		out[k] = it.i
	}
	return out
}

// How far a route keeps apart from the routes before it.
const (
	// apartNone lets a route share a line with another route, at a cost.
	apartNone = iota
	// apartSteps keeps a route off each part of a line that another route
	// takes.
	apartSteps
	// apartPoints also keeps a route from bending where another route
	// passes, and from passing where another route bends.
	apartPoints
)

// routes routes each edge in the frame where the ranks run down. The loops
// take their places first, and the other edges then route in the order of
// routeOrder. Each route keeps apart from the routes before it, as
// apartPoints says, when it can, else as apartSteps says, and else takes
// the cheapest route, where a part that another route takes costs more.
func (l *laneLayout) routes(g *laneGeom) []laneRoute {
	in := l.in
	r := l.newRouter(g)
	nx := len(r.xs)
	out := make([]laneRoute, len(in.Edges))
	// region returns the points of the grid where a route can start or
	// end at an end of an edge, each with its cost: each point inside a
	// node, which costs more the farther it is from the center of the node,
	// or each point inside the box of a cluster and outside its nodes.
	region := func(e int) ([]int, []float64, [2]float64) {
		var ps []int
		var costs []float64
		if e >= 0 {
			for p, v := range r.inside {
				if v == e {
					ps = append(ps, p)
					costs = append(costs, laneStartCost*(math.Abs(r.xs[p%nx]-l.x[e])+math.Abs(r.ys[p/nx]-l.y[e])))
				}
			}
			return ps, costs, [2]float64{l.x[e], l.y[e]}
		}
		b := g.boxes[-1-e]
		for j, y := range r.ys {
			for i, x := range r.xs {
				if x <= b.x1 || x >= b.x2 || y <= b.y1 || y >= b.y2 || r.inside[j*nx+i] >= 0 {
					continue
				}
				ps, costs = append(ps, j*nx+i), append(costs, 0)
			}
		}
		return ps, costs, [2]float64{(b.x1 + b.x2) / 2, (b.y1 + b.y2) / 2}
	}
	for i := range in.Edges {
		if rt, ok := r.loops[i]; ok {
			lw, lh := l.labelSize(i)
			out[i] = rt
			r.take(rt.pts, nil, rectAt(rt.label, lw, lh), lw > 0)
		}
	}
	for _, i := range l.routeOrder(g) {
		a, b := l.ends[i][0], l.ends[i][1]
		lw, lh := l.labelSize(i)
		starts, startCost, from := region(a)
		goals, goalCost, to := region(b)
		goal := make([]float64, len(r.xs)*len(r.ys))
		for k := range goal {
			goal[k] = -1
		}
		for k, p := range goals {
			goal[p] = goalCost[k]
		}
		var ok []int
		for _, e := range []int{a, b} {
			if e >= 0 {
				ok = append(ok, e)
			}
		}
		var pts [][2]float64
		for apart := apartPoints; apart >= apartNone && len(pts) < 2; apart-- {
			pts = r.route(starts, startCost, goal, ok, b, from, to, apart)
		}
		if len(pts) < 2 {
			continue
		}
		rt := laneRoute{pts: pts}
		if lw > 0 {
			rt.label = labelOn(pts, g, i)
		}
		r.take(pts, ok, rectAt(rt.label, lw, lh), lw > 0)
		out[i] = rt
	}
	return out
}
