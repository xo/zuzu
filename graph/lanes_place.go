package graph

import (
	"math"
	"slices"
	"strings"
)

// The spaces of a lane layout, in pixels.
const (
	// lanePad is the space inside a lane around its nodes. Mermaid keeps 20.
	lanePad = 16.0
	// laneGap is the space between two lanes.
	laneGap = 8.0
	// laneLoop is how far a loop reaches out of the side of its node.
	laneLoop = 16.0
	// laneBend is the cost of a bend of a route, in pixels of length.
	laneBend = 16.0
	// laneShare is the cost of a part of a route that another route, or
	// the label of another edge, already takes.
	laneShare = 30.0
	// laneCorner is the radius of a corner of a route.
	laneCorner = 5.0
)

// laneGeom is the geometry of a lane layout in the frame where the ranks
// run down: the places of the ranks, the gaps between them and the lanes.
type laneGeom struct {
	rankSep, nodeSep float64
	center           []float64 // the center of each rank
	gapMid           []float64 // the middle of the gap after each rank
	flowLen          float64   // the length of the ranks and their gaps
	flowPad          float64   // the space of the lanes before and after the ranks
	band             float64   // the title of the lanes across the ranks
	x0, x1           []float64 // the sides of each lane
	low, high        []float64 // the space inside each lane at its sides
	laneX            []float64 // the center of the nodes of each lane
	// spots holds the place that each label of an edge wants, and has
	// whether it wants one. gapOf holds the gap of the label of each edge
	// across the ranks, or -1.
	spots [][2]float64
	has   []bool
	gapOf []int
	// boxes holds the box of each cluster, or a box of no width for a
	// cluster with no node.
	boxes []rect
}

// labelSize returns the size of the label of an edge in the frame where
// the ranks run down.
func (l *laneLayout) labelSize(i int) (float64, float64) {
	e := l.in.Edges[i]
	if l.sideways {
		return e.LabelHeight, e.LabelWidth
	}
	return e.LabelWidth, e.LabelHeight
}

// place places the ranks, the lanes and the nodes, in the frame where the
// ranks run down.
func (l *laneLayout) place() *laneGeom {
	in := l.in
	g := &laneGeom{rankSep: in.RankSep, nodeSep: in.NodeSep}
	if g.rankSep <= 0 {
		g.rankSep = 40
	}
	if g.nodeSep <= 0 {
		g.nodeSep = 24
	}
	nr, nl := len(l.layers), len(l.lanes)
	// The labels of the edges. A label of an edge in one rank goes beside
	// its target, on the side of its source, and widens the lane of its
	// target. A label of an edge across the ranks goes in the gap in the
	// middle of its ranks, and makes the gap longer.
	type rowLabel struct {
		edge, rank, lane int
		before           bool
	}
	var rows []rowLabel
	need := make([]float64, nr)
	gapOf := make([]int, len(in.Edges))
	heights := make([]float64, nr)
	content := make([]float64, nl)
	for i := range in.Edges {
		gapOf[i] = -1
		a, b := l.ends[i][0], l.ends[i][1]
		lw, lh := l.labelSize(i)
		if a < 0 || b < 0 || lw <= 0 {
			continue
		}
		if a == b {
			continue
		}
		ra, rb := l.rank[a], l.rank[b]
		if ra == rb {
			rows = append(rows, rowLabel{i, rb, l.lane[b], l.lane[a] < l.lane[b]})
			heights[rb] = math.Max(heights[rb], lh)
			continue
		}
		lo, hi := min(ra, rb), max(ra, rb)
		gapOf[i] = lo + (hi-lo-1)/2
		need[gapOf[i]] = math.Max(need[gapOf[i]], lh)
		content[l.lane[b]] = math.Max(content[l.lane[b]], lw)
	}
	for r, ly := range l.layers {
		for _, v := range ly {
			heights[r] = math.Max(heights[r], l.h[v])
		}
	}
	// The width of the nodes of each lane in each rank.
	rowWidth := func(r, lane int) float64 {
		w, k := 0.0, 0
		for _, v := range l.layers[r] {
			if l.lane[v] == lane {
				w += l.w[v]
				k++
			}
		}
		return w + g.nodeSep*float64(max(k-1, 0))
	}
	for r := range l.layers {
		for li := range l.lanes {
			content[li] = math.Max(content[li], rowWidth(r, li))
		}
	}
	type side struct{ rank, lane int }
	before, after := map[side]float64{}, map[side]float64{}
	for _, rl := range rows {
		lw, _ := l.labelSize(rl.edge)
		if rl.before {
			before[side{rl.rank, rl.lane}] += g.nodeSep + lw
		} else {
			after[side{rl.rank, rl.lane}] += g.nodeSep + lw
		}
	}
	for _, rl := range rows {
		k := side{rl.rank, rl.lane}
		content[rl.lane] = math.Max(content[rl.lane], rowWidth(rl.rank, rl.lane)+2*math.Max(before[k], after[k]))
	}
	// A loop goes out of the side of its node, with its label beside it.
	for i := range in.Edges {
		a, b := l.ends[i][0], l.ends[i][1]
		if a < 0 || a != b {
			continue
		}
		lw, _ := l.labelSize(i)
		reach := laneLoop
		if lw > 0 {
			reach += 4 + lw
		}
		content[l.lane[a]] = math.Max(content[l.lane[a]], rowWidth(l.rank[a], l.lane[a])+2*reach)
	}
	// The nested clusters of each lane, and their titles.
	depth := make([]int, nl)
	nestedTitle, laneTitle := 0.0, make([]float64, nl)
	deepest := 0
	for c, cl := range in.Clusters {
		t := l.top(c)
		li := slices.Index(l.lanes, t)
		if li < 0 {
			continue
		}
		if c == t {
			laneTitle[li] = cl.TitleHeight
			if !l.sideways {
				g.band = math.Max(g.band, cl.TitleHeight)
				content[li] = math.Max(content[li], cl.TitleWidth+2*l.pad-2*lanePad)
			}
			continue
		}
		depth[li] = max(depth[li], l.cdepth[c])
		deepest = max(deepest, l.cdepth[c])
		nestedTitle = math.Max(nestedTitle, cl.TitleHeight)
		w := cl.TitleWidth + 2*l.pad*float64(l.cdepth[c])
		if l.sideways {
			w = 2 * l.pad * float64(l.cdepth[c])
		}
		content[li] = math.Max(content[li], w)
	}
	// The lanes, side by side.
	at := 0.0
	for li := range l.lanes {
		d := float64(depth[li])
		lo, hi := lanePad+d*l.pad, lanePad+d*l.pad
		if l.sideways {
			lo += laneTitle[li] + d*nestedTitle
		}
		g.x0 = append(g.x0, at)
		g.low, g.high = append(g.low, lo), append(g.high, hi)
		g.laneX = append(g.laneX, at+lo+content[li]/2)
		at += lo + content[li] + hi
		g.x1 = append(g.x1, at)
		at += laneGap
	}
	// The ranks, one after another.
	l.x, l.y = make([]float64, len(in.Nodes)), make([]float64, len(in.Nodes))
	cursor := 0.0
	for r := range l.layers {
		g.center = append(g.center, cursor+heights[r]/2)
		cursor += heights[r]
		if r < nr-1 {
			gap := g.rankSep + need[r]
			g.gapMid = append(g.gapMid, cursor+gap/2)
			cursor += gap
		}
	}
	g.flowLen = cursor
	g.flowPad = lanePad + float64(deepest)*l.pad
	if !l.sideways {
		g.flowPad += float64(deepest) * nestedTitle
	}
	// A title of a lane runs along the ranks when they run across, so the
	// lanes are at least as long as their titles.
	for _, c := range l.lanes {
		if c >= 0 && l.sideways {
			g.flowLen = math.Max(g.flowLen, in.Clusters[c].TitleWidth+2*l.pad-2*g.flowPad)
		}
	}
	for r, ly := range l.layers {
		for li := range l.lanes {
			start := g.laneX[li] - rowWidth(r, li)/2
			for _, v := range ly {
				if l.lane[v] != li {
					continue
				}
				l.x[v], l.y[v] = start+l.w[v]/2, g.center[r]
				start += l.w[v] + g.nodeSep
			}
		}
	}
	// The places that the labels want: beside the target in its rank, or
	// in the gap, on the line into the target.
	g.spots, g.has, g.gapOf = make([][2]float64, len(in.Edges)), make([]bool, len(in.Edges)), gapOf
	edge := map[side]float64{}
	for _, rl := range rows {
		k := side{rl.rank, rl.lane}
		lw, _ := l.labelSize(rl.edge)
		if _, ok := edge[k]; !ok {
			e := math.Inf(1)
			if !rl.before {
				e = math.Inf(-1)
			}
			for _, v := range l.layers[rl.rank] {
				if l.lane[v] != rl.lane {
					continue
				}
				if rl.before {
					e = math.Min(e, l.x[v]-l.w[v]/2)
				} else {
					e = math.Max(e, l.x[v]+l.w[v]/2)
				}
			}
			edge[k] = e
		}
		sign := 1.0
		if rl.before {
			sign = -1
		}
		c := edge[k] + sign*(g.nodeSep+lw/2)
		g.spots[rl.edge], g.has[rl.edge] = [2]float64{c, g.center[rl.rank]}, true
		edge[k] = c + sign*lw/2
	}
	for i, gp := range gapOf {
		if gp < 0 {
			continue
		}
		a, b := l.ends[i][0], l.ends[i][1]
		x := l.x[b]
		if l.lane[a] == l.lane[b] {
			x = (l.x[a] + l.x[b]) / 2
		}
		g.spots[i], g.has[i] = [2]float64{x, g.gapMid[gp]}, true
	}
	l.clusterBoxes(g)
	return g
}

// titleBox grows a box by the height of a title on the side that is the
// top of the image.
func (l *laneLayout) titleBox(b rect, h float64) rect {
	switch l.in.Direction {
	case BT:
		b.y2 += h
	case LR, RL:
		b.x1 -= h
	default:
		b.y1 -= h
	}
	return b
}

// clusterBoxes sets the box of each cluster in the frame where the ranks
// run down. A lane runs along all the ranks. A cluster inside a lane holds
// its nodes and its clusters, with its padding and its title.
func (l *laneLayout) clusterBoxes(g *laneGeom) {
	in := l.in
	g.boxes = make([]rect, len(in.Clusters))
	order := make([]int, len(in.Clusters))
	for c := range order {
		order[c] = c
	}
	slices.SortStableFunc(order, func(a, b int) int { return l.cdepth[b] - l.cdepth[a] })
	for _, c := range order {
		if l.cparent[c] < 0 {
			continue
		}
		b := rect{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
		grow := func(o rect) {
			b = rect{math.Min(b.x1, o.x1), math.Min(b.y1, o.y1), math.Max(b.x2, o.x2), math.Max(b.y2, o.y2)}
		}
		for v, n := range in.Nodes {
			if n.Cluster == in.Clusters[c].ID {
				grow(rectAt([2]float64{l.x[v], l.y[v]}, l.w[v], l.h[v]))
			}
		}
		for d := range in.Clusters {
			if l.cparent[d] == c && g.boxes[d].x2 > g.boxes[d].x1 {
				grow(g.boxes[d])
			}
		}
		if math.IsInf(b.x1, 1) {
			continue
		}
		b = rect{b.x1 - l.pad, b.y1 - l.pad, b.x2 + l.pad, b.y2 + l.pad}
		if w := in.Clusters[c].TitleWidth + 2*l.pad; !l.sideways && b.x2-b.x1 < w {
			grow := (w - (b.x2 - b.x1)) / 2
			b.x1, b.x2 = b.x1-grow, b.x2+grow
		}
		g.boxes[c] = l.titleBox(b, in.Clusters[c].TitleHeight)
	}
	for li, c := range l.lanes {
		if c < 0 {
			continue
		}
		b := rect{g.x0[li], -g.flowPad, g.x1[li], g.flowLen + g.flowPad}
		if !l.sideways {
			b = l.titleBox(b, g.band)
		}
		g.boxes[c] = b
	}
}

// laneRouter routes the edges at right angles on a grid of lines: the
// centers of the nodes, the middles of the gaps between the ranks, and the
// sides inside each lane and between two lanes. A route does not cross a
// node, except the nodes at its ends, and each bend and each part that
// another route takes cost more.
type laneRouter struct {
	xs, ys []float64
	// block holds the nodes that each step of the grid crosses: the step
	// from a point to the next point across, then to the next point
	// along, by the index of the point.
	block [2][][]int
	// extra holds the cost of each step that other routes and labels take.
	extra [2][]float64
}

// gridLines sorts the places of the lines of a grid and keeps one of each
// two places closer than half a pixel.
func gridLines(vs []float64) []float64 {
	slices.Sort(vs)
	var out []float64
	for _, v := range vs {
		if len(out) == 0 || v-out[len(out)-1] > 0.5 {
			out = append(out, v)
		}
	}
	return out
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

// newRouter builds the grid of a layout.
func (l *laneLayout) newRouter(g *laneGeom) *laneRouter {
	var xs, ys []float64
	for v := range l.in.Nodes {
		xs, ys = append(xs, l.x[v]), append(ys, l.y[v])
	}
	for li := range l.lanes {
		xs = append(xs, g.x0[li]+g.low[li]-g.high[li]+lanePad/2, g.x1[li]-lanePad/2)
		if li > 0 {
			xs = append(xs, g.x0[li]-laneGap/2)
		}
	}
	for _, ly := range l.layers {
		for k := 1; k < len(ly); k++ {
			a, b := ly[k-1], ly[k]
			if l.lane[a] == l.lane[b] {
				xs = append(xs, (l.x[a]+l.w[a]/2+l.x[b]-l.w[b]/2)/2)
			}
		}
	}
	ys = append(ys, g.center...)
	ys = append(ys, g.gapMid...)
	ys = append(ys, -g.flowPad+lanePad/2, g.flowLen+g.flowPad-lanePad/2)
	r := &laneRouter{xs: gridLines(xs), ys: gridLines(ys)}
	nx, ny := len(r.xs), len(r.ys)
	for k := range 2 {
		r.block[k] = make([][]int, nx*ny)
		r.extra[k] = make([]float64, nx*ny)
	}
	for v := range l.in.Nodes {
		b := l.nodeBox(v)
		b = rect{b.x1 - 2, b.y1 - 2, b.x2 + 2, b.y2 + 2}
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

// routeState is a place of the search of a route: its cost, and a point
// of the grid with the direction that the route came in, 0 to 3, or 4 at
// the start, as at.
type routeState struct {
	cost float64
	at   int
}

// routeHeap is a binary heap of the places of the search, the cheapest
// first, and of two that cost the same, the one of the lower index.
type routeHeap []routeState

func (h routeHeap) less(i, j int) bool {
	if h[i].cost != h[j].cost {
		return h[i].cost < h[j].cost
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
// goals. The route can cross the nodes in ok. A step across the ranks costs a
// little more the farther it is from the rank of the start, so that a
// route crosses the lanes early. A route that leaves its start against the
// way from the start to the goal, or comes into its goal that way, costs a
// little more too, so that a route leaves down and comes in from above
// when its goal is below. It returns the points of the route.
func (r *laneRouter) route(starts []int, goal []bool, ok []int, fromY, toY float64) [][2]float64 {
	nx, ny := len(r.xs), len(r.ys)
	n := nx * ny * 5
	dist := make([]float64, n)
	prev := make([]int, n)
	for i := range dist {
		dist[i], prev[i] = math.Inf(1), -1
	}
	h := &routeHeap{}
	for _, p := range starts {
		s := p*5 + 4
		dist[s] = 0
		h.push(routeState{0, s})
	}
	free := func(bs []int) bool {
		for _, b := range bs {
			if !slices.Contains(ok, b) {
				return false
			}
		}
		return true
	}
	// back is the step against the way to the goal.
	back := 3
	if toY < fromY {
		back = 2
	}
	end := -1
	for len(*h) > 0 {
		s := h.pop()
		if s.cost > dist[s.at] {
			continue
		}
		p, d := s.at/5, s.at%5
		if goal[p] {
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
			length := math.Abs(r.xs[ni]-r.xs[i]) + math.Abs(r.ys[nj]-r.ys[j])
			c := s.cost + length + r.extra[axis][a]
			if axis == 0 {
				c += 0.0002 * math.Abs(r.ys[j]-fromY) * length
			}
			if d < 4 && d != k {
				c += laneBend
			}
			if k == back && d == 4 {
				c += laneBend / 2
			}
			if k == back && goal[nj*nx+ni] {
				c += laneBend
			}
			t := (nj*nx+ni)*5 + k
			if c < dist[t] {
				dist[t], prev[t] = c, s.at
				h.push(routeState{c, t})
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

// take adds the cost of the gridSteps that a route and its label take to the
// routes after it.
func (r *laneRouter) take(pts [][2]float64, label rect, hasLabel bool) {
	nx, ny := len(r.xs), len(r.ys)
	for k := 1; k < len(pts); k++ {
		a, b := pts[k-1], pts[k]
		i1, i2 := nearLine(r.xs, a[0]), nearLine(r.xs, b[0])
		j1, j2 := nearLine(r.ys, a[1]), nearLine(r.ys, b[1])
		if j1 == j2 {
			for i := min(i1, i2); i < max(i1, i2); i++ {
				r.extra[0][j1*nx+i] += laneShare
			}
		} else {
			for j := min(j1, j2); j < max(j1, j2); j++ {
				r.extra[1][j*nx+i1] += laneShare
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

// routes routes each edge in the order of the input, in the frame where
// the ranks run down.
func (l *laneLayout) routes(g *laneGeom) []laneRoute {
	in := l.in
	r := l.newRouter(g)
	nx := len(r.xs)
	out := make([]laneRoute, len(in.Edges))
	// region returns the points of the grid where a route can start or
	// end at an end of an edge: the center of a node, or each point inside
	// the box of a cluster and outside its nodes.
	region := func(e int) ([]int, float64) {
		if e >= 0 {
			return []int{nearLine(r.ys, l.y[e])*nx + nearLine(r.xs, l.x[e])}, l.y[e]
		}
		b := g.boxes[-1-e]
		var ps []int
		for j, y := range r.ys {
			for i, x := range r.xs {
				p := [2]float64{x, y}
				if p[0] <= b.x1 || p[0] >= b.x2 || p[1] <= b.y1 || p[1] >= b.y2 {
					continue
				}
				inside := false
				for v := range in.Nodes {
					nb := l.nodeBox(v)
					if p[0] > nb.x1 && p[0] < nb.x2 && p[1] > nb.y1 && p[1] < nb.y2 {
						inside = true
						break
					}
				}
				if !inside {
					ps = append(ps, j*nx+i)
				}
			}
		}
		return ps, (b.y1 + b.y2) / 2
	}
	for i := range in.Edges {
		a, b := l.ends[i][0], l.ends[i][1]
		lw, lh := l.labelSize(i)
		if a >= 0 && a == b {
			out[i] = l.loop(i, lw)
			continue
		}
		starts, fromY := region(a)
		goals, toY := region(b)
		goal := make([]bool, len(r.xs)*len(r.ys))
		for _, p := range goals {
			goal[p] = true
		}
		var ok []int
		for _, e := range []int{a, b} {
			if e >= 0 {
				ok = append(ok, e)
			}
		}
		pts := r.route(starts, goal, ok, fromY, toY)
		if len(pts) < 2 {
			continue
		}
		rt := laneRoute{pts: pts}
		if lw > 0 {
			rt.label = labelOn(pts, g, i)
		}
		r.take(pts, rectAt(rt.label, lw, lh), lw > 0)
		out[i] = rt
	}
	return out
}

// loop routes an edge from a node to itself out of the side of the node,
// at right angles, with its label beside it.
func (l *laneLayout) loop(i int, lw float64) laneRoute {
	v := l.ends[i][0]
	x, y, w, h := l.x[v], l.y[v], l.w[v], l.h[v]
	x1 := x + w/2 + laneLoop
	pts := [][2]float64{{x, y - h/4}, {x1, y - h/4}, {x1, y + h/4}, {x, y + h/4}}
	return laneRoute{pts: pts, label: [2]float64{x1 + 4 + lw/2, y}}
}

// result writes the layout in the frame of its direction.
func (l *laneLayout) result(g *laneGeom) *Layout {
	in := l.in
	tf := func(p [2]float64) [2]float64 {
		switch in.Direction {
		case BT:
			return [2]float64{p[0], -p[1]}
		case LR:
			return [2]float64{p[1], p[0]}
		case RL:
			return [2]float64{-p[1], p[0]}
		}
		return p
	}
	out := &Layout{Nodes: make([]NodePlace, len(in.Nodes)), Edges: make([]EdgePlace, len(in.Edges)), Clusters: make([]ClusterPlace, len(in.Clusters))}
	for v := range in.Nodes {
		p := tf([2]float64{l.x[v], l.y[v]})
		out.Nodes[v] = NodePlace{X: p[0], Y: p[1], Width: in.Nodes[v].Width, Height: in.Nodes[v].Height, Rank: l.rank[v]}
	}
	for r, ly := range l.layers {
		for k, v := range ly {
			out.Nodes[v].Rank, out.Nodes[v].Order = r, k
		}
	}
	for c, b := range g.boxes {
		if b.x2 <= b.x1 {
			continue
		}
		p, q := tf([2]float64{b.x1, b.y1}), tf([2]float64{b.x2, b.y2})
		x1, y1 := math.Min(p[0], q[0]), math.Min(p[1], q[1])
		x2, y2 := math.Max(p[0], q[0]), math.Max(p[1], q[1])
		out.Clusters[c] = ClusterPlace{
			X: x1, Y: y1, Width: x2 - x1, Height: y2 - y1, Depth: l.cdepth[c],
			TitleX: x1 + l.pad, TitleY: y1 + l.pad/2 + in.Clusters[c].TitleTop,
		}
	}
	for i, rt := range l.routes(g) {
		if len(rt.pts) < 2 {
			continue
		}
		pts := make([][2]float64, len(rt.pts))
		for k, p := range rt.pts {
			pts[k] = tf(p)
		}
		a, b := l.ends[i][0], l.ends[i][1]
		if pts = l.clipEnd(pts, b, out); len(pts) < 2 {
			continue
		}
		slices.Reverse(pts)
		if pts = l.clipEnd(pts, a, out); len(pts) < 2 {
			continue
		}
		slices.Reverse(pts)
		ep := &out.Edges[i]
		ep.Points = pts
		last := len(pts) - 1
		ep.EndX, ep.EndY = pts[last][0], pts[last][1]
		ep.EndAngle = math.Atan2(pts[last][1]-pts[last-1][1], pts[last][0]-pts[last-1][0]) * 180 / math.Pi
		ep.StartX, ep.StartY = pts[0][0], pts[0][1]
		ep.StartAngle = math.Atan2(pts[0][1]-pts[1][1], pts[0][0]-pts[1][0]) * 180 / math.Pi
		lp := tf(rt.label)
		ep.LabelX, ep.LabelY = lp[0], lp[1]
		ep.Path = lanePath(in.Edges[i].Curve, pts)
	}
	return out
}

// clipEnd stops a route at the border of the end of an edge that its last
// point is at: the border of the shape of a node, or the box of a cluster.
// It returns nil when no part of the route is outside the end.
func (l *laneLayout) clipEnd(pts [][2]float64, e int, out *Layout) [][2]float64 {
	if e < 0 {
		if cut, ok := cutAtBox(pts, out.Clusters[-1-e]); ok {
			return cut
		}
		return pts
	}
	n := out.Nodes[e]
	kind := l.in.Nodes[e].Shape
	inside := func(p [2]float64) bool {
		bx, by := clip(n, kind, p[0], p[1])
		return bx == p[0] && by == p[1]
	}
	k := len(pts) - 1
	for k > 0 && inside(pts[k-1]) {
		k--
	}
	if k == 0 {
		return nil
	}
	return append(slices.Clone(pts[:k]), border(n, kind, pts[k], pts[k-1]))
}

// lanePath returns the path of a route at right angles: with round
// corners for the curve basis, the default, and with the curve of Vega for
// any other curve.
func lanePath(curve string, pts [][2]float64) string {
	if curve != "" && curve != "basis" {
		return curvePath(curve, pts)
	}
	var b strings.Builder
	b.WriteString("M" + num(pts[0][0]) + "," + num(pts[0][1]))
	for k := 1; k < len(pts)-1; k++ {
		p, c, n := pts[k-1], pts[k], pts[k+1]
		d1, d2 := math.Hypot(c[0]-p[0], c[1]-p[1]), math.Hypot(n[0]-c[0], n[1]-c[1])
		if d1 == 0 || d2 == 0 {
			continue
		}
		r := math.Min(laneCorner, math.Min(d1, d2)/2)
		a := [2]float64{c[0] - (c[0]-p[0])/d1*r, c[1] - (c[1]-p[1])/d1*r}
		e := [2]float64{c[0] + (n[0]-c[0])/d2*r, c[1] + (n[1]-c[1])/d2*r}
		b.WriteString("L" + num(a[0]) + "," + num(a[1]) + "Q" + num(c[0]) + "," + num(c[1]) + " " + num(e[0]) + "," + num(e[1]))
	}
	last := pts[len(pts)-1]
	b.WriteString("L" + num(last[0]) + "," + num(last[1]))
	return b.String()
}

// normalize moves every place so that the layout starts at 0, 0, and sets
// its size.
func (l *laneLayout) normalize(out *Layout) {
	x1, y1, x2, y2 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	add := func(ax, ay, bx, by float64) {
		x1, y1 = math.Min(x1, ax), math.Min(y1, ay)
		x2, y2 = math.Max(x2, bx), math.Max(y2, by)
	}
	for _, n := range out.Nodes {
		add(n.X-n.Width/2, n.Y-n.Height/2, n.X+n.Width/2, n.Y+n.Height/2)
	}
	for _, c := range out.Clusters {
		if c.Width > 0 {
			add(c.X, c.Y, c.X+c.Width, c.Y+c.Height)
		}
	}
	for i, e := range out.Edges {
		for _, p := range e.Points {
			add(p[0], p[1], p[0], p[1])
		}
		if w := l.in.Edges[i].LabelWidth; w > 0 && len(e.Points) > 0 {
			h := l.in.Edges[i].LabelHeight
			add(e.LabelX-w/2, e.LabelY-h/2, e.LabelX+w/2, e.LabelY+h/2)
		}
	}
	labelBounds(l.in, out, add)
	if math.IsInf(x1, 1) {
		return
	}
	for i := range out.Nodes {
		out.Nodes[i].X -= x1
		out.Nodes[i].Y -= y1
		out.Nodes[i].shiftLabels(-x1, -y1)
	}
	for i := range out.Clusters {
		c := &out.Clusters[i]
		if c.Width > 0 {
			c.X, c.Y, c.TitleX, c.TitleY = c.X-x1, c.Y-y1, c.TitleX-x1, c.TitleY-y1
		}
	}
	for i := range out.Edges {
		e := &out.Edges[i]
		if len(e.Points) == 0 {
			continue
		}
		for j := range e.Points {
			e.Points[j][0] -= x1
			e.Points[j][1] -= y1
		}
		e.LabelX, e.LabelY = e.LabelX-x1, e.LabelY-y1
		e.EndX, e.EndY, e.StartX, e.StartY = e.EndX-x1, e.EndY-y1, e.StartX-x1, e.StartY-y1
		e.shiftLabels(-x1, -y1)
		e.Path = lanePath(l.in.Edges[i].Curve, e.Points)
	}
	out.Width, out.Height = x2-x1, y2-y1
}
