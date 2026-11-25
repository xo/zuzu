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
	// laneCross is the cost of a crossing of two routes. As in Mermaid
	// 12.3, a crossing costs as much as 20 bends.
	laneCross = 40 * laneBend
	// laneBack is the cost of each pixel of a step against the way to the
	// goal, along the ranks. As in Mermaid 12.3, it costs 2 bends.
	laneBack = 2 * laneBend
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
	gapLen           []float64 // the length of the gap after each rank
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
		reach := laneLoop + float64(l.loopIndex(i)/2)*laneTrack
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
			if !l.sideways || l.turned() {
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
			lo += d * nestedTitle
			if !l.turned() {
				lo += laneTitle[li]
			}
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
			g.gapMid, g.gapLen = append(g.gapMid, cursor+gap/2), append(g.gapLen, gap)
			cursor += gap
		}
	}
	g.flowLen = cursor
	g.flowPad = lanePad + float64(deepest)*l.pad
	if !l.sideways {
		g.flowPad += float64(deepest) * nestedTitle
	}
	// A title of a lane runs along the ranks when they run from the right
	// to the left, so the lanes are at least as long as their titles.
	for _, c := range l.lanes {
		if c >= 0 && l.sideways && !l.turned() {
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

// turned reports whether the titles of the lanes turn, so that they read
// from the bottom to the top in a band at the left of each lane, as in
// Mermaid 12.3: when the ranks run from the left to the right.
func (l *laneLayout) turned() bool {
	return l.in.Direction == LR
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
		switch {
		case l.turned():
			// The band of the title is before the first rank, at the left
			// of the image.
			b.y1 -= g.band
		case !l.sideways:
			b = l.titleBox(b, g.band)
		}
		g.boxes[c] = b
	}
}

// loop routes an edge from a node to itself out of the side of the node,
// at right angles, with its label beside it. The loops of a node go out of
// its right side and its left side in turn, and each loop of a side
// reaches farther and wider than the loop of that side before it, so that
// no two loops share a line.
func (l *laneLayout) loop(i int, lw float64) laneRoute {
	v := l.ends[i][0]
	k := l.loopIndex(i)
	side, ring := 1.0, float64(k/2)
	if k%2 == 1 {
		side = -1
	}
	x, y, w, h := l.x[v], l.y[v], l.w[v], l.h[v]
	x1 := x + side*(w/2+laneLoop+ring*laneTrack)
	dy := math.Min(math.Max(h/4, laneTrackClear)+ring*laneTrackClear, h/2-1)
	pts := [][2]float64{{x, y - dy}, {x1, y - dy}, {x1, y + dy}, {x, y + dy}}
	return laneRoute{pts: pts, label: [2]float64{x1 + side*(4+lw/2), y}}
}

// loopIndex returns the number of the loops of the node of a loop that
// come before it in the input.
func (l *laneLayout) loopIndex(i int) int {
	v, k := l.ends[i][0], 0
	for j := range i {
		if l.ends[j][0] == v && l.ends[j][1] == v {
			k++
		}
	}
	return k
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
		cp := ClusterPlace{
			X: x1, Y: y1, Width: x2 - x1, Height: y2 - y1, Depth: l.cdepth[c],
			TitleX: x1 + l.pad, TitleY: y1 + l.pad/2 + in.Clusters[c].TitleTop,
		}
		if l.turned() && l.cparent[c] < 0 {
			// The title reads from the bottom to the top, in the middle of
			// the band at the left of the lane.
			cp.TitleX, cp.TitleY = x1+l.pad/2+in.Clusters[c].TitleTop, (y1+y2)/2+in.Clusters[c].TitleWidth/2
			cp.TitleAngle = -90
		}
		out.Clusters[c] = cp
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

// paths sets the path of each route, with a hop where it crosses another
// route when the input asks for hops.
func (l *laneLayout) paths(out *Layout) {
	curves := make([]string, len(l.in.Edges))
	for i, e := range l.in.Edges {
		curves[i] = e.Curve
	}
	hops := make([][]laneHop, len(out.Edges))
	if l.in.LaneHops != "" {
		hops = laneHops(out.Edges, curves)
	}
	for i := range out.Edges {
		if e := &out.Edges[i]; len(e.Points) > 0 {
			e.Path = hopPath(curves[i], l.in.LaneHops, e.Points, hops[i])
		}
	}
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
	}
	l.paths(out)
	out.Width, out.Height = x2-x1, y2-y1
}
