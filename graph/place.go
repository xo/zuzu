package graph

import (
	"math"
	"slices"
	"strconv"

	"github.com/xo/zuzu/internal/shape"
)

// separation returns the least distance between the centers of two
// vertices next to each other in a rank: half of each width, the space
// between nodes or between edges, and the padding of each cluster border
// between them.
func (l *layout) separation(u, v int) float64 {
	a, b := l.vs[u], l.vs[v]
	gap := l.sep.node
	switch {
	case a.dummy && b.dummy:
		gap = l.sep.edge
	case a.dummy || b.dummy:
		gap = (l.sep.node + l.sep.edge) / 2
	}
	if a.border != 0 || b.border != 0 {
		// A border keeps the padding of its cluster from what is inside,
		// and the space between nodes from what is outside.
		inner := a.border == -1 || b.border == 1
		if a.border != 0 && b.border != 0 {
			inner = a.border == -1 && b.border == 1
		}
		if inner {
			return (a.w+b.w)/2 + l.sep.pad
		}
		return (a.w+b.w)/2 + l.sep.edge
	}
	return (a.w+b.w)/2 + gap
}

// place sets x and y of each vertex: x by Brandes and Köpf, the balance of
// four alignments, and y from the heights of the ranks.
func (l *layout) place() {
	n := len(l.vs)
	conflict := l.conflicts()
	var xs [4][]float64
	for d := range 4 {
		downward, leftward := d < 2, d%2 == 0
		layers := l.copyLayers()
		if !leftward {
			for _, ly := range layers {
				slices.Reverse(ly)
			}
		}
		pred := l.up
		if !downward {
			slices.Reverse(layers)
			pred = l.down
		}
		pos := make([]int, n)
		for _, ly := range layers {
			for i, v := range ly {
				pos[v] = i
			}
		}
		root, align := l.align(layers, pred, pos, conflict)

		x := l.compact(layers, root, align, pos, leftward)
		if !leftward {
			for i := range x {
				x[i] = -x[i]
			}
		}
		xs[d] = x
	}
	// Shift the four to the one of the least width, the left ones to its
	// left and the right ones to its right, and take the mean of the four.
	// The four keep the same order in each rank, so their mean keeps every
	// separation, and a block that is straight in each, such as the border
	// of a cluster, stays straight.
	smallest, sw := 0, math.Inf(1)
	lo, hi := make([]float64, 4), make([]float64, 4)
	for d := range 4 {
		lo[d], hi[d] = math.Inf(1), math.Inf(-1)
		for v := range n {
			lo[d] = math.Min(lo[d], xs[d][v]-l.vs[v].w/2)
			hi[d] = math.Max(hi[d], xs[d][v]+l.vs[v].w/2)
		}
		if hi[d]-lo[d] < sw {
			smallest, sw = d, hi[d]-lo[d]
		}
	}
	for d := range 4 {
		shift := lo[smallest] - lo[d]
		if d%2 == 1 {
			shift = hi[smallest] - hi[d]
		}
		for v := range n {
			xs[d][v] += shift
		}
	}
	l.x = make([]float64, n)
	for v := range n {
		l.x[v] = (xs[0][v] + xs[1][v] + xs[2][v] + xs[3][v]) / 4
	}
	l.placeRanks()
}

// conflicts marks the edges of the first type: an edge of a node that
// crosses an inner segment, an edge between two dummies, so that the long
// edges stay straight.
func (l *layout) conflicts() map[[2]int]bool {
	marked := map[[2]int]bool{}
	inner := func(v int) int {
		if !l.vs[v].dummy {
			return -1
		}
		for _, u := range l.up[v] {
			if l.vs[u].dummy {
				return u
			}
		}
		return -1
	}
	for r := 1; r+1 <= len(l.layers)-1; r++ {
		upper, lower := l.layers[r], l.layers[r+1]
		k0, li := 0, 0
		for l1, v := range lower {
			iu := inner(v)
			if l1 != len(lower)-1 && iu < 0 {
				continue
			}
			k1 := len(upper) - 1
			if iu >= 0 {
				k1 = l.vs[iu].order
			}
			for ; li <= l1; li++ {
				w := lower[li]
				for _, u := range l.up[w] {
					k := l.vs[u].order
					if (k < k0 || k > k1) && (!l.vs[u].dummy || !l.vs[w].dummy) {
						marked[[2]int{u, w}] = true
					}
				}
			}
			k0 = k1
		}
	}
	return marked
}

// align joins each vertex to a median neighbor in the rank before, as a
// block, when no conflict forbids it and the join crosses no join made
// before it in the rank. The borders of the clusters join first, so that
// each border is straight where it can be.
func (l *layout) align(layers [][]int, pred [][]int, pos []int, conflict map[[2]int]bool) ([]int, []int) {
	n := len(l.vs)
	root := make([]int, n)
	align := make([]int, n)
	for v := range n {
		root[v], align[v] = v, v
	}
	marked := func(u, v int) bool {
		return conflict[[2]int{u, v}] || conflict[[2]int{v, u}]
	}
	for _, ly := range layers {
		var made [][2]int
		crosses := func(u, v int) bool {
			for _, p := range made {
				if (pos[u] < p[0]) != (pos[v] < p[1]) || pos[u] == p[0] || pos[v] == p[1] {
					return true
				}
			}
			return false
		}
		join := func(u, v int) {
			align[u] = v
			root[v] = root[u]
			align[v] = root[v]
			made = append(made, [2]int{pos[u], pos[v]})
		}
		for _, v := range ly {
			if l.vs[v].border == 0 {
				continue
			}
			for _, u := range pred[v] {
				if l.vs[u].border == l.vs[v].border && l.vs[u].cluster == l.vs[v].cluster && align[v] == v && !crosses(u, v) {
					join(u, v)
				}
			}
		}
		for _, v := range ly {
			if l.vs[v].border != 0 {
				continue
			}
			nb := slices.Clone(pred[v])
			if len(nb) == 0 {
				continue
			}
			slices.SortFunc(nb, func(a, b int) int { return pos[a] - pos[b] })
			d := len(nb)
			for _, m := range []int{(d - 1) / 2, d / 2} {
				if align[v] != v {
					break
				}
				u := nb[m]
				if !marked(u, v) && !crosses(u, v) {
					join(u, v)
				}
				if d%2 == 1 {
					break
				}
			}
		}
	}
	return root, align
}

// compact places the blocks: a graph of the blocks with an edge from the
// block of each vertex to the block of the next one in its rank, each as
// long as their separation, places each block as far left as the edges
// allow, and then as far right as the blocks after it allow.
func (l *layout) compact(layers [][]int, root, _ []int, _ []int, _ bool) []float64 {
	n := len(l.vs)
	type bedge struct {
		to  int
		gap float64
	}
	out := map[int][]bedge{}
	in := map[int][]bedge{}
	for _, ly := range layers {
		for i := 1; i < len(ly); i++ {
			a, b := root[ly[i-1]], root[ly[i]]
			g := l.separation(ly[i-1], ly[i])
			out[a] = append(out[a], bedge{b, g})
			in[b] = append(in[b], bedge{a, g})
		}
	}
	var blocks []int
	for v := range n {
		if root[v] == v {
			blocks = append(blocks, v)
		}
	}
	// A topological order of the blocks.
	indeg := map[int]int{}
	for _, b := range blocks {
		for _, e := range out[b] {
			indeg[e.to]++
		}
	}
	var queue, topo []int
	for _, b := range blocks {
		if indeg[b] == 0 {
			queue = append(queue, b)
		}
	}
	for len(queue) > 0 {
		b := queue[0]
		queue = queue[1:]
		topo = append(topo, b)
		for _, e := range out[b] {
			indeg[e.to]--
			if indeg[e.to] == 0 {
				queue = append(queue, e.to)
			}
		}
	}
	bx := map[int]float64{}
	for _, b := range topo {
		x := 0.0
		for _, e := range in[b] {
			x = math.Max(x, bx[e.to]+e.gap)
		}
		bx[b] = x
	}
	for _, b := range slices.Backward(topo) {
		if len(out[b]) == 0 {
			continue
		}
		m := math.Inf(1)
		for _, e := range out[b] {
			m = math.Min(m, bx[e.to]-e.gap)
		}
		bx[b] = math.Max(bx[b], m)
	}
	x := make([]float64, n)
	for v := range n {
		x[v] = bx[root[v]]
	}
	return x
}

// placeRanks sets y of each vertex at the middle of its rank. The space
// between two ranks holds the padding and the titles of the clusters that
// end above it and start below it.
func (l *layout) placeRanks() {
	in := l.in
	h := make([]float64, len(l.layers))
	for r, ly := range l.layers {
		for _, v := range ly {
			h[r] = math.Max(h[r], l.vs[v].h)
		}
	}
	nc := len(in.Clusters)
	minR, maxR := make([]int, nc), make([]int, nc)
	for c := range nc {
		minR[c], maxR[c] = math.MaxInt, -1
	}
	for _, v := range l.vs {
		if v.dummy {
			continue
		}
		for c := v.cluster; c >= 0; c = l.cparent[c] {
			minR[c] = min(minR[c], v.rank)
			maxR[c] = max(maxR[c], v.rank)
		}
	}
	title := func(c int) float64 {
		if in.Direction == LR || in.Direction == RL {
			return 0
		}
		return in.Clusters[c].TitleHeight
	}
	// need returns the space that a cluster and the clusters inside it
	// that start, or end, in the same rank need at that side.
	var need func(c int, start bool) float64
	need = func(c int, start bool) float64 {
		inner := 0.0
		for d := range nc {
			if l.cparent[d] != c {
				continue
			}
			if start && minR[d] == minR[c] || !start && maxR[d] == maxR[c] {
				inner = math.Max(inner, need(d, start))
			}
		}
		top := l.sep.pad
		if start == (in.Direction != BT) {
			top += title(c)
		}
		return top + inner
	}
	l.y = make([]float64, len(l.vs))
	ys := make([]float64, len(l.layers))
	for r := range l.layers {
		if r == 0 {
			ys[r] = h[0] / 2
			continue
		}
		above, below := 0.0, 0.0
		for c := range nc {
			p := l.cparent[c]
			outer := p < 0 || minR[p] != minR[c]
			if outer && minR[c] == r {
				below = math.Max(below, need(c, true))
			}
			outerEnd := p < 0 || maxR[p] != maxR[c]
			if outerEnd && maxR[c] == r-1 {
				above = math.Max(above, need(c, false))
			}
		}
		gap := math.Max(l.sep.rank, above+below+l.sep.rank/2)
		ys[r] = ys[r-1] + h[r-1]/2 + gap + h[r]/2
	}
	for v := range l.vs {
		l.y[v] = ys[l.vs[v].rank]
	}
}

// result maps the places to the direction of the layout and routes the
// edges.
func (l *layout) result(ids map[string]int, _ bool) *Layout {
	in := l.in
	tf := func(x, y float64) (float64, float64) {
		switch in.Direction {
		case BT:
			return x, -y
		case LR:
			return y, x
		case RL:
			return -y, x
		}
		return x, y
	}
	size := func(w, h float64) (float64, float64) {
		if in.Direction == LR || in.Direction == RL {
			return h, w
		}
		return w, h
	}
	out := &Layout{Nodes: make([]NodePlace, len(in.Nodes)), Edges: make([]EdgePlace, len(in.Edges)), Clusters: make([]ClusterPlace, len(in.Clusters))}
	for i := range in.Nodes {
		v := l.vs[i]
		x, y := tf(l.x[i], l.y[i])
		w, h := size(v.w, v.h)
		out.Nodes[i] = NodePlace{X: x, Y: y, Width: w, Height: h, Rank: v.rank, Order: v.order}
	}
	l.clusterBoxes(out, tf)
	l.routes(out, ids, tf)
	l.normalize(out)
	return out
}

// clusterBoxes sets the box of each cluster: across the ranks from its
// left border to its right border, and along them from the top of its
// first rank to the bottom of its last, with its padding and room for its
// title at its top.
func (l *layout) clusterBoxes(out *Layout, tf func(x, y float64) (float64, float64)) {
	in := l.in
	nc := len(in.Clusters)
	x1, x2 := make([]float64, nc), make([]float64, nc)
	minR, maxR := make([]int, nc), make([]int, nc)
	for c := range nc {
		x1[c], x2[c] = math.Inf(1), math.Inf(-1)
		minR[c], maxR[c] = math.MaxInt, -1
	}
	for v, vt := range l.vs {
		if vt.border == 0 {
			continue
		}
		c := vt.cluster
		x1[c], x2[c] = math.Min(x1[c], l.x[v]), math.Max(x2[c], l.x[v])
		minR[c], maxR[c] = min(minR[c], vt.rank), max(maxR[c], vt.rank)
	}
	rankTop := map[int]float64{}
	rankBottom := map[int]float64{}
	for v, vt := range l.vs {
		t, b := l.y[v]-vt.h/2, l.y[v]+vt.h/2
		if old, ok := rankTop[vt.rank]; !ok || t < old {
			rankTop[vt.rank] = t
		}
		if old, ok := rankBottom[vt.rank]; !ok || b > old {
			rankBottom[vt.rank] = b
		}
	}
	// The depth of the clusters inside a cluster that share its first or
	// its last rank, whose padding and titles it holds too.
	var stack func(c int, first bool) float64
	stack = func(c int, first bool) float64 {
		inner := 0.0
		for d := range nc {
			if l.cparent[d] == c && (first && minR[d] == minR[c] || !first && maxR[d] == maxR[c]) {
				inner = math.Max(inner, stack(d, first))
			}
		}
		s := l.sep.pad + inner
		if first == (in.Direction != BT) && in.Direction != LR && in.Direction != RL {
			s += in.Clusters[c].TitleHeight
		}
		return s
	}
	for c := range nc {
		if maxR[c] < 0 {
			// A cluster that holds no node has no box.
			continue
		}
		bx1, bx2 := x1[c], x2[c]
		by1 := rankTop[minR[c]] - stack(c, true)
		by2 := rankBottom[maxR[c]] + stack(c, false)
		if in.Direction == LR || in.Direction == RL {
			// The title is at the top of the cluster, which is its left
			// across the ranks.
			bx1 -= in.Clusters[c].TitleHeight
		}
		ax, ay := tf(bx1, by1)
		bx, by := tf(bx2, by2)
		X1, Y1, X2, Y2 := math.Min(ax, bx), math.Min(ay, by), math.Max(ax, bx), math.Max(ay, by)
		pad := in.ClusterPadding
		if pad <= 0 {
			pad = 12
		}
		out.Clusters[c] = ClusterPlace{
			X: X1, Y: Y1, Width: X2 - X1, Height: Y2 - Y1, Depth: l.cdepth[c],
			TitleX: X1 + pad, TitleY: Y1 + pad/2,
		}
	}
}

// clip returns the point where the line from the center of a node toward
// a point meets the border of the node.
func clip(n NodePlace, kind string, px, py float64) (float64, float64) {
	dx, dy := px-n.X, py-n.Y
	if dx == 0 && dy == 0 {
		return n.X, n.Y
	}
	hw, hh := n.Width/2, n.Height/2
	var t float64
	switch kind {
	case "ellipse", "circle", "doublecircle", "oval", "point":
		t = 1 / math.Sqrt(dx*dx/(hw*hw)+dy*dy/(hh*hh))
	case "diamond", "rhombus":
		t = 1 / (math.Abs(dx)/hw + math.Abs(dy)/hh)
	default:
		t = math.Inf(1)
		if dx != 0 {
			t = hw / math.Abs(dx)
		}
		if dy != 0 {
			t = math.Min(t, hh/math.Abs(dy))
		}
	}
	if t >= 1 || math.IsNaN(t) {
		return px, py
	}
	return n.X + t*dx, n.Y + t*dy
}

// end returns the end of an edge at a node: the border of its shape on
// the line to a point, or, at a port, the border of its box on the line
// from the port to the point.
func end(n NodePlace, kind string, port []float64, to [2]float64) (float64, float64) {
	if len(port) != 2 {
		return clip(n, kind, to[0], to[1])
	}
	px, py := n.X+port[0], n.Y+port[1]
	dx, dy := to[0]-px, to[1]-py
	t := 1.0
	if dx != 0 {
		bx := n.X + math.Copysign(n.Width/2, dx)
		t = math.Min(t, (bx-px)/dx)
	}
	if dy != 0 {
		by := n.Y + math.Copysign(n.Height/2, dy)
		t = math.Min(t, (by-py)/dy)
	}
	return px + max(t, 0)*dx, py + max(t, 0)*dy
}

// routes sets the route of each edge: the points of its chain, clipped at
// the borders of its ends, as a cubic B-spline. A loop goes around the
// right of its node, and edges between the same two nodes spread apart.
func (l *layout) routes(out *Layout, ids map[string]int, tf func(x, y float64) (float64, float64)) {
	in := l.in
	type pair struct{ a, b int }
	seen := map[pair]int{}
	for i, e := range in.Edges {
		a, b := ids[e.Source], ids[e.Target]
		na, nb := out.Nodes[a], out.Nodes[b]
		ep := &out.Edges[i]
		if a == b {
			l.loop(ep, na, e)
			continue
		}
		var pts [][2]float64
		for _, v := range l.chains[i] {
			x, y := tf(l.x[v], l.y[v])
			pts = append(pts, [2]float64{x, y})
		}
		if l.flip[i] {
			slices.Reverse(pts)
		}
		if l.flat[i] && absInt(l.vs[a].order-l.vs[b].order) > 1 {
			// An edge in one rank that passes other nodes bends around
			// them.
			mx, my := (pts[0][0]+pts[1][0])/2, (pts[0][1]+pts[1][1])/2
			ox, oy := tf(0, -(math.Max(l.vs[a].h, l.vs[b].h)/2 + l.sep.rank/2))
			pts = [][2]float64{pts[0], {mx + ox, my + oy}, pts[1]}
		}
		// Edges between the same two nodes spread apart.
		k := pair{min(a, b), max(a, b)}
		if c := seen[k]; c > 0 {
			off := float64((c+1)/2) * 14
			if c%2 == 0 {
				off = -off
			}
			if len(pts) == 2 {
				pts = [][2]float64{pts[0], {(pts[0][0] + pts[1][0]) / 2, (pts[0][1] + pts[1][1]) / 2}, pts[1]}
			}
			dx, dy := pts[len(pts)-1][0]-pts[0][0], pts[len(pts)-1][1]-pts[0][1]
			d := math.Hypot(dx, dy)
			if d > 0 {
				for j := 1; j < len(pts)-1; j++ {
					pts[j][0] += -dy / d * off
					pts[j][1] += dx / d * off
				}
			}
		}
		seen[k]++
		pts[0][0], pts[0][1] = end(na, in.Nodes[a].Shape, e.SourcePort, pts[1])
		last := len(pts) - 1
		pts[last][0], pts[last][1] = end(nb, in.Nodes[b].Shape, e.TargetPort, pts[last-1])
		ep.Points = pts
		ep.EndX, ep.EndY = pts[last][0], pts[last][1]
		ep.EndAngle = math.Atan2(pts[last][1]-pts[last-1][1], pts[last][0]-pts[last-1][0]) * 180 / math.Pi
		ep.StartX, ep.StartY = pts[0][0], pts[0][1]
		ep.StartAngle = math.Atan2(pts[0][1]-pts[1][1], pts[0][0]-pts[1][0]) * 180 / math.Pi
		if lv := l.label[i]; lv >= 0 {
			// A B-spline passes near its middle points, not through them,
			// so the label sits on the curve where its point pulls it.
			j := slices.Index(l.chains[i], lv)
			if l.flip[i] {
				j = len(pts) - 1 - j
			}
			if j > 0 && j < len(pts)-1 {
				ep.LabelX = (pts[j-1][0] + 4*pts[j][0] + pts[j+1][0]) / 6
				ep.LabelY = (pts[j-1][1] + 4*pts[j][1] + pts[j+1][1]) / 6
			} else {
				ep.LabelX, ep.LabelY = tf(l.x[lv], l.y[lv])
			}
		} else {
			m := len(pts) / 2
			ep.LabelX, ep.LabelY = (pts[m-1][0]+pts[m][0])/2, (pts[m-1][1]+pts[m][1])/2
		}
	}
}

// loop routes an edge from a node to itself, on the side of the node
// across the ranks: at its right, or below it when the ranks run from left
// to right.
func (l *layout) loop(ep *EdgePlace, n NodePlace, e LayoutEdge) {
	r := 18 + e.LabelWidth/4
	if l.in.Direction == LR || l.in.Direction == RL {
		x0, y0 := n.X-n.Width/4, n.Y+n.Height/2
		x1, y1 := n.X+n.Width/4, n.Y+n.Height/2
		ep.Points = [][2]float64{{x0, y0}, {x0 - r/2, y0 + r}, {x1 + r/2, y1 + r}, {x1, y1}}
		ep.EndX, ep.EndY, ep.EndAngle = x1, y1, -90-25
		ep.StartX, ep.StartY, ep.StartAngle = x0, y0, -90+25
		ep.LabelX, ep.LabelY = n.X, y0+r*1.3+e.LabelHeight/2
	} else {
		x0, y0 := n.X+n.Width/2, n.Y-n.Height/4
		x1, y1 := n.X+n.Width/2, n.Y+n.Height/4
		ep.Points = [][2]float64{{x0, y0}, {x0 + r, y0 - r/2}, {x1 + r, y1 + r/2}, {x1, y1}}
		ep.EndX, ep.EndY, ep.EndAngle = x1, y1, 180-25
		ep.StartX, ep.StartY, ep.StartAngle = x0, y0, 180+25
		ep.LabelX, ep.LabelY = x0+r*1.3+e.LabelWidth/2, n.Y
	}
	ep.Path = loopPath(ep.Points)
}

// normalize moves every place so that the layout starts at 0, 0, and sets
// its size.
func (l *layout) normalize(out *Layout) {
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
		if w := l.in.Edges[i].LabelWidth; w > 0 {
			h := l.in.Edges[i].LabelHeight
			add(e.LabelX-w/2, e.LabelY-h/2, e.LabelX+w/2, e.LabelY+h/2)
		}
	}
	if math.IsInf(x1, 1) {
		return
	}
	for i := range out.Nodes {
		out.Nodes[i].X -= x1
		out.Nodes[i].Y -= y1
	}
	for i := range out.Clusters {
		c := &out.Clusters[i]
		if c.Width > 0 {
			c.X, c.Y, c.TitleX, c.TitleY = c.X-x1, c.Y-y1, c.TitleX-x1, c.TitleY-y1
		}
	}
	for i := range out.Edges {
		e := &out.Edges[i]
		for j := range e.Points {
			e.Points[j][0] -= x1
			e.Points[j][1] -= y1
		}
		e.LabelX, e.LabelY = e.LabelX-x1, e.LabelY-y1
		e.EndX, e.EndY, e.StartX, e.StartY = e.EndX-x1, e.EndY-y1, e.StartX-x1, e.StartY-y1
		if len(e.Points) > 0 {
			if l.in.Edges[i].Source == l.in.Edges[i].Target {
				e.Path = loopPath(e.Points)
			} else {
				e.Path = curvePath(l.in.Edges[i].Curve, e.Points)
			}
		}
	}
	out.Width, out.Height = x2-x1, y2-y1
}

// loopPath returns the path of a loop from its four points: a cubic curve
// from the first to the last, which the two between pull out.
func loopPath(p [][2]float64) string {
	c := func(a, b [2]float64) string {
		return num(a[0]+1.6*(b[0]-a[0])) + "," + num(a[1]+1.6*(b[1]-a[1]))
	}
	return "M" + num(p[0][0]) + "," + num(p[0][1]) + "C" + c(p[0], p[1]) + " " + c(p[3], p[2]) + " " + num(p[3][0]) + "," + num(p[3][1])
}

func absInt(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// num writes a number with at most three decimals.
func num(v float64) string {
	v = math.Round(v*1000) / 1000
	if v == 0 {
		return "0"
	}
	return strconvFormat(v)
}

func strconvFormat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// curvePath returns the path of a route with a curve, basis by default and
// for a curve that Vega does not draw.
func curvePath(curve string, pts [][2]float64) string {
	if curve == "" {
		curve = "basis"
	}
	if p, ok := shape.Curve(curve, pts, true, 0); ok {
		return p
	}
	p, _ := shape.Curve("basis", pts, true, 0)
	return p
}
