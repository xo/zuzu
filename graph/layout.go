package graph

import (
	"cmp"
	"fmt"
	"math"
	"slices"
)

// LayoutInput is what Lay lays out: nodes with their sizes, edges, clusters
// and the options.
type LayoutInput struct {
	Nodes    []LayoutNode
	Edges    []LayoutEdge
	Clusters []LayoutCluster
	// Direction is TB, BT, LR or RL. Empty is TB.
	Direction string
	// RankSep, NodeSep and ClusterPadding are the space between the ranks,
	// between the nodes of a rank, and inside a cluster. 0 is 40, 24 and 12.
	RankSep, NodeSep, ClusterPadding float64
}

// LayoutNode is a node to lay out.
type LayoutNode struct {
	ID string
	// Cluster is the id of the cluster that holds the node, or empty, and
	// Rank names a group of nodes of the same rank, or empty.
	Cluster, Rank string
	// Shape decides where an edge meets the node: ellipse, circle,
	// diamond, or else a rect.
	Shape         string
	Width, Height float64
	// XLabelWidth and XLabelHeight are the size of the xlabel of the node,
	// or 0 for none. The xlabel takes no space in the ranks.
	XLabelWidth, XLabelHeight float64
}

// LayoutEdge is an edge to lay out.
type LayoutEdge struct {
	// Source and Target are the ids of the ends: each a node, or else a
	// cluster. An edge to a cluster ends at the box of the cluster.
	Source, Target string
	// MinLen is 1 or more, and Weight is 1 for 0.
	MinLen int
	Weight float64
	// NoConstraint takes the edge out of the ranks.
	NoConstraint bool
	// LabelWidth and LabelHeight are the size of the label of the edge, or
	// 0 for none.
	LabelWidth, LabelHeight float64
	// HeadLabelWidth, HeadLabelHeight, TailLabelWidth and TailLabelHeight
	// are the sizes of the labels at the ends of the edge, and XLabelWidth
	// and XLabelHeight the size of its xlabel, or 0 for none. These labels
	// take no space in the ranks.
	HeadLabelWidth, HeadLabelHeight float64
	TailLabelWidth, TailLabelHeight float64
	XLabelWidth, XLabelHeight       float64
	// LabelDistance and LabelAngle fix the places of the labels at the
	// ends, as Edge.LabelDistance and Edge.LabelAngle say, or the layout
	// places them beside the edge when LabelDistance is 0.
	LabelDistance, LabelAngle float64
	// Curve is the interpolation of the route: basis, the default, or
	// another curve of Vega, such as linear or step.
	Curve string
	// SourcePort and TargetPort are the places of the ports at the ends,
	// as x and y from the center of the node, or nil for none. An edge
	// leaves a port on the side of the node that faces its route.
	SourcePort, TargetPort []float64
	// SourceSide and TargetSide are the directions out of the node at a
	// port on a side, such as 1, 0 for the east side, or nil. An edge at
	// such a port ends on the port and leaves it in that direction.
	SourceSide, TargetSide []float64
}

// LayoutCluster is a cluster to lay out, with the size of its title.
type LayoutCluster struct {
	ID, Parent              string
	TitleWidth, TitleHeight float64
	// Direction is the direction of the ranks inside the cluster, TB, BT,
	// LR or RL, or empty for the direction of the layout that holds it.
	// The cluster keeps it only when no edge links a node inside the
	// cluster to a node outside it, as in Mermaid. Otherwise the cluster
	// follows the layout that holds it.
	Direction string
}

// Layout is the result of Lay, with the places in the order of the input.
type Layout struct {
	Width, Height float64
	Nodes         []NodePlace
	Edges         []EdgePlace
	Clusters      []ClusterPlace
}

// NodePlace is the place of a node: its center and its size, its rank and
// its order in the rank. A node in a cluster that lays out on its own has
// the rank and the order of the layout of that cluster.
type NodePlace struct {
	X, Y, Width, Height float64
	Rank, Order         int
	// XLabelX and XLabelY are the center of the xlabel of the node.
	XLabelX, XLabelY float64
}

// EdgePlace is the route of an edge: its points, its path of SVG, the
// place of its label, and the place and the angle in degrees of the end at
// its target and at its source. HeadLabelX, HeadLabelY, TailLabelX,
// TailLabelY, XLabelX and XLabelY are the centers of the labels at its
// head and its tail and of its xlabel.
type EdgePlace struct {
	Points                     [][2]float64
	Path                       string
	LabelX, LabelY             float64
	EndX, EndY, EndAngle       float64
	StartX, StartY, StartAngle float64
	HeadLabelX, HeadLabelY     float64
	TailLabelX, TailLabelY     float64
	XLabelX, XLabelY           float64
}

// ClusterPlace is the box of a cluster, its depth from 0 for an outer one,
// and the place of the top left of its title.
type ClusterPlace struct {
	X, Y, Width, Height float64
	Depth               int
	TitleX, TitleY      float64
}

// vertex is a node of the layered graph: a node of the input, a dummy of a
// long edge, or the dummy that holds the label of an edge.
type vertex struct {
	w, h    float64
	node    int // the index of the input node, or -1
	cluster int // the index of the cluster, or -1
	rank    int
	order   int
	dummy   bool
	// border is -1 for the left border of a cluster in a rank, 1 for its
	// right border, and 0 for any other vertex.
	border int
}

// layout holds the state of one call of Lay.
type layout struct {
	in      *LayoutInput
	vs      []vertex
	up      [][]int // the neighbors of each vertex in the rank above
	down    [][]int // and in the rank below
	layers  [][]int
	cparent []int
	cdepth  []int
	// chains holds the vertices of each edge from its upper end to its
	// lower end, and flip whether that is the reverse of the edge.
	chains [][]int
	flip   []bool
	flat   []bool
	label  []int // the vertex of the label of each edge, or -1
	x, y   []float64
	sep    struct{ rank, node, edge, pad float64 }
	// corder is the order of each cluster among the clusters beside it,
	// the same in every rank, so that the blocks of the clusters keep
	// their order from rank to rank.
	corder []float64
}

// lay lays out a graph of nodes in ranks: the method of Gansner,
// Koutsofios, North and Vo, 1993, with the order of the ranks by the
// weighted median and the places across the ranks by Brandes and Köpf,
// 2001 (D31). It reads no direction of a cluster, and each edge joins two
// nodes.
func lay(in LayoutInput) (*Layout, error) {
	l := &layout{in: &in}
	l.sep.rank, l.sep.node, l.sep.pad = in.RankSep, in.NodeSep, in.ClusterPadding
	if l.sep.rank <= 0 {
		l.sep.rank = 40
	}
	if l.sep.node <= 0 {
		l.sep.node = 24
	}
	if l.sep.pad <= 0 {
		l.sep.pad = 12
	}
	l.sep.edge = l.sep.node / 2
	sideways := in.Direction == LR || in.Direction == RL
	ids := map[string]int{}
	for i, n := range in.Nodes {
		if _, ok := ids[n.ID]; ok {
			return nil, fmt.Errorf("the node %q twice: %w", n.ID, ErrGraph)
		}
		ids[n.ID] = i
	}
	cids := map[string]int{}
	for i, c := range in.Clusters {
		cids[c.ID] = i
	}
	l.cparent = make([]int, len(in.Clusters))
	for i, c := range in.Clusters {
		l.cparent[i] = -1
		if p, ok := cids[c.Parent]; ok && c.Parent != "" {
			l.cparent[i] = p
		}
	}
	l.cdepth = make([]int, len(in.Clusters))
	for i := range in.Clusters {
		for p := l.cparent[i]; p >= 0 && l.cdepth[i] < len(in.Clusters); p = l.cparent[p] {
			l.cdepth[i]++
		}
	}
	titleH := 0.0
	for _, c := range in.Clusters {
		titleH = math.Max(titleH, c.TitleHeight)
	}
	if sideways {
		// The title of a cluster sits at its top, which is across the
		// ranks, so it widens the space between a cluster and its neighbor.
		l.sep.pad += titleH
	}
	for _, n := range in.Nodes {
		w, h := n.Width, n.Height
		if sideways {
			w, h = h, w
		}
		c := -1
		if i, ok := cids[n.Cluster]; ok && n.Cluster != "" {
			c = i
		}
		l.vs = append(l.vs, vertex{w: w, h: h, node: len(l.vs), cluster: c})
	}
	hasLabels := false
	for _, e := range in.Edges {
		if _, ok := ids[e.Source]; !ok {
			return nil, fmt.Errorf("an edge from %q, which is not a node: %w", e.Source, ErrGraph)
		}
		if _, ok := ids[e.Target]; !ok {
			return nil, fmt.Errorf("an edge to %q, which is not a node: %w", e.Target, ErrGraph)
		}
		if e.LabelWidth > 0 && e.Source != e.Target {
			hasLabels = true
		}
	}
	if hasLabels {
		// A label sits in a rank of its own between the ends of its edge,
		// so each edge spans two ranks at least and the ranks are closer.
		l.sep.rank /= 2
	}
	l.rank(ids, hasLabels)
	l.chainEdges(ids, sideways)
	l.addBorders()
	l.order()
	l.place()
	return l.result(ids, sideways), nil
}

// rank gives each vertex a rank, with the nodes of a group of the same
// rank as one.
func (l *layout) rank(ids map[string]int, double bool) {
	in := l.in
	rep := make([]int, len(in.Nodes))
	group := map[string]int{}
	for i, n := range in.Nodes {
		rep[i] = i
		if n.Rank == "" {
			continue
		}
		if r, ok := group[n.Rank]; ok {
			rep[i] = r
		} else {
			group[n.Rank] = i
		}
	}
	var es []redge
	for _, e := range in.Edges {
		if e.NoConstraint {
			continue
		}
		u, v := rep[ids[e.Source]], rep[ids[e.Target]]
		if u == v {
			continue
		}
		m := max(1, e.MinLen)
		if double {
			m *= 2
		}
		w := e.Weight
		if w <= 0 {
			w = 1
		}
		es = append(es, redge{u: u, v: v, minlen: m, weight: w})
	}
	es, _ = acyclic(len(in.Nodes), es)
	ranks := rankNodes(len(in.Nodes), es)
	for i := range in.Nodes {
		l.vs[i].rank = ranks[rep[i]]
	}
}

// lca returns the innermost cluster that holds two clusters, or -1.
func (l *layout) lca(a, b int) int {
	for a >= 0 && b >= 0 && a != b {
		if l.cdepth[a] >= l.cdepth[b] {
			a = l.cparent[a]
		} else {
			b = l.cparent[b]
		}
	}
	if a < 0 || b < 0 {
		return -1
	}
	return a
}

// chainEdges splits each edge that spans more than one rank into a chain
// of dummy vertices, one in each rank, with the label of the edge in the
// middle one.
func (l *layout) chainEdges(ids map[string]int, sideways bool) {
	in := l.in
	n := len(in.Edges)
	l.chains = make([][]int, n)
	l.flip = make([]bool, n)
	l.flat = make([]bool, n)
	l.label = make([]int, n)
	for i, e := range in.Edges {
		l.label[i] = -1
		a, b := ids[e.Source], ids[e.Target]
		if a == b {
			continue
		}
		if l.vs[a].rank > l.vs[b].rank {
			a, b = b, a
			l.flip[i] = true
		}
		if l.vs[a].rank == l.vs[b].rank {
			l.flat[i] = true
			l.chains[i] = []int{a, b}
			continue
		}
		c := l.lca(l.vs[a].cluster, l.vs[b].cluster)
		chain := []int{a}
		mid := (l.vs[a].rank + l.vs[b].rank) / 2
		for r := l.vs[a].rank + 1; r < l.vs[b].rank; r++ {
			v := vertex{node: -1, cluster: c, rank: r, dummy: true}
			if r == mid && e.LabelWidth > 0 {
				v.w, v.h = e.LabelWidth, e.LabelHeight
				if sideways {
					v.w, v.h = v.h, v.w
				}
				l.label[i] = len(l.vs)
			}
			chain = append(chain, len(l.vs))
			l.vs = append(l.vs, v)
		}
		chain = append(chain, b)
		l.chains[i] = chain
	}
	l.up = make([][]int, len(l.vs))
	l.down = make([][]int, len(l.vs))
	for i, ch := range l.chains {
		if l.flat[i] || len(ch) < 2 {
			continue
		}
		for k := 1; k < len(ch); k++ {
			l.down[ch[k-1]] = append(l.down[ch[k-1]], ch[k])
			l.up[ch[k]] = append(l.up[ch[k]], ch[k-1])
		}
	}
}

// addBorders adds a left and a right border to each cluster in each rank
// that it spans, chained from rank to rank. The order keeps them at the
// ends of the block of the cluster, and the places keep each chain
// straight, so that a cluster is a box that no other node enters.
func (l *layout) addBorders() {
	nc := len(l.in.Clusters)
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
	for c := range nc {
		for _, side := range []int{-1, 1} {
			prev := -1
			for r := minR[c]; r <= maxR[c]; r++ {
				v := len(l.vs)
				l.vs = append(l.vs, vertex{node: -1, cluster: c, rank: r, dummy: true, border: side})
				l.up = append(l.up, nil)
				l.down = append(l.down, nil)
				if prev >= 0 {
					l.down[prev] = append(l.down[prev], v)
					l.up[v] = append(l.up[v], prev)
				}
				prev = v
			}
		}
	}
}

// order sets the order of the vertices in each rank: a first order by a
// search from the nodes in the order of the input, then 24 sweeps of the
// weighted median down and up, each with the swap of neighbors, keeping the
// order with the fewest crossings. The vertices of a cluster stay together
// in each rank.
func (l *layout) order() {
	top := 0
	for _, v := range l.vs {
		top = max(top, v.rank)
	}
	l.layers = make([][]int, top+1)
	seen := make([]bool, len(l.vs))
	var visit func(v int)
	visit = func(v int) {
		if seen[v] {
			return
		}
		seen[v] = true
		l.layers[l.vs[v].rank] = append(l.layers[l.vs[v].rank], v)
		for _, w := range l.down[v] {
			visit(w)
		}
	}
	for v := range l.vs {
		if len(l.up[v]) == 0 {
			visit(v)
		}
	}
	for v := range l.vs {
		visit(v)
	}
	for r := range l.layers {
		l.layers[r] = l.arrange(l.layers[r], nil)
	}
	l.setOrder()
	l.clusterOrder()
	for r := range l.layers {
		l.layers[r] = l.arrange(l.layers[r], nil)
	}
	l.setOrder()
	best := l.copyLayers()
	bestCross := l.crossings()
	for iter := range 24 {
		if bestCross == 0 {
			break
		}
		l.clusterOrder()
		if iter%2 == 0 {
			for r := 1; r < len(l.layers); r++ {
				l.sweep(r, l.up)
			}
		} else {
			for r := len(l.layers) - 2; r >= 0; r-- {
				l.sweep(r, l.down)
			}
		}
		l.transpose()
		if c := l.crossings(); c < bestCross {
			bestCross, best = c, l.copyLayers()
		}
	}
	l.layers = best
	l.setOrder()
}

// clusterOrder sets the order of the clusters from the mean place of their
// vertices, as a share of the width of each rank.
func (l *layout) clusterOrder() {
	nc := len(l.in.Clusters)
	sum := make([]float64, nc)
	cnt := make([]float64, nc)
	for _, ly := range l.layers {
		for i, v := range ly {
			f := (float64(i) + 0.5) / float64(len(ly))
			for c := l.vs[v].cluster; c >= 0; c = l.cparent[c] {
				sum[c] += f
				cnt[c]++
			}
		}
	}
	l.corder = make([]float64, nc)
	for c := range nc {
		if cnt[c] > 0 {
			l.corder[c] = sum[c] / cnt[c]
		}
	}
}

func (l *layout) copyLayers() [][]int {
	out := make([][]int, len(l.layers))
	for i, ly := range l.layers {
		out[i] = slices.Clone(ly)
	}
	return out
}

func (l *layout) setOrder() {
	for _, ly := range l.layers {
		for i, v := range ly {
			l.vs[v].order = i
		}
	}
}

// median returns the weighted median of the orders of the neighbors of a
// vertex, as Gansner and others define it, and false for none.
func (l *layout) median(nb []int) (float64, bool) {
	if len(nb) == 0 {
		return 0, false
	}
	p := make([]float64, len(nb))
	for i, w := range nb {
		p[i] = float64(l.vs[w].order)
	}
	slices.Sort(p)
	m := len(p) / 2
	switch {
	case len(p)%2 == 1:
		return p[m], true
	case len(p) == 2:
		return (p[0] + p[1]) / 2, true
	}
	left, right := p[m-1]-p[0], p[len(p)-1]-p[m]
	if left+right == 0 {
		return (p[m-1] + p[m]) / 2, true
	}
	return (p[m-1]*right + p[m]*left) / (left + right), true
}

// sweep orders a rank by the medians of the neighbors of its vertices in
// the rank before it. A vertex with no neighbor keeps its place.
func (l *layout) sweep(r int, nb [][]int) {
	key := map[int]float64{}
	for _, v := range l.layers[r] {
		if m, ok := l.median(nb[v]); ok {
			key[v] = m
		} else {
			key[v] = float64(l.vs[v].order)
		}
	}
	l.layers[r] = l.arrange(l.layers[r], key)
	for i, v := range l.layers[r] {
		l.vs[v].order = i
	}
}

// arrange sorts a rank by a key, or keeps its order for a nil key, with the
// vertices of each cluster together, a cluster as a block at the mean of
// its keys.
func (l *layout) arrange(rank []int, key map[int]float64) []int {
	k := func(v int) float64 {
		if key == nil {
			return float64(slices.Index(rank, v))
		}
		return key[v]
	}
	// childOf returns the cluster under c on the way down to the cluster
	// of a vertex, or -1 when the vertex is in c itself.
	childOf := func(v, c int) int {
		x := l.vs[v].cluster
		for x >= 0 {
			if l.cparent[x] == c {
				return x
			}
			x = l.cparent[x]
		}
		return -1
	}
	var group func(c int, vs []int) []int
	group = func(c int, vs []int) []int {
		type item struct {
			key   float64
			first int
			verts []int
		}
		var items []item
		blocks := map[int]int{}
		for i, v := range vs {
			ch := childOf(v, c)
			if ch < 0 || l.vs[v].cluster == c {
				key := k(v)
				// The borders of the cluster close its block.
				switch l.vs[v].border {
				case -1:
					key = math.Inf(-1)
				case 1:
					key = math.Inf(1)
				}
				items = append(items, item{key: key, first: i, verts: []int{v}})
				continue
			}
			if j, ok := blocks[ch]; ok {
				items[j].verts = append(items[j].verts, v)
				continue
			}
			blocks[ch] = len(items)
			items = append(items, item{first: i, verts: []int{v}})
		}
		clusterOf := map[int]int{}
		for ch, j := range blocks {
			sum := 0.0
			for _, v := range items[j].verts {
				sum += k(v)
			}
			items[j].key = sum / float64(len(items[j].verts))
			items[j].verts = group(ch, items[j].verts)
			clusterOf[j] = ch
		}
		// Each item keeps its cluster, so that the slots of the clusters can
		// take them in their fixed order.
		type slot struct {
			item

			cluster int
		}
		slots := make([]slot, len(items))
		for j, it := range items {
			c, ok := clusterOf[j]
			if !ok {
				c = -1
			}
			slots[j] = slot{it, c}
		}
		slices.SortStableFunc(slots, func(a, b slot) int {
			if c := cmp.Compare(a.key, b.key); c != 0 {
				return c
			}
			return cmp.Compare(a.first, b.first)
		})
		if l.corder != nil {
			var idx []int
			var cl []slot
			for j, sl := range slots {
				if sl.cluster >= 0 {
					idx = append(idx, j)
					cl = append(cl, sl)
				}
			}
			slices.SortStableFunc(cl, func(a, b slot) int { return cmp.Compare(l.corder[a.cluster], l.corder[b.cluster]) })
			for j, at := range idx {
				slots[at] = cl[j]
			}
		}
		items = items[:0]
		for _, sl := range slots {
			items = append(items, sl.item)
		}
		var out []int
		for _, it := range items {
			out = append(out, it.verts...)
		}
		return out
	}
	return group(-1, rank)
}

// transpose swaps neighbors of the same cluster in each rank while a swap
// lowers the crossings with the ranks on both sides.
func (l *layout) transpose() {
	for range 8 {
		improved := false
		for r, ly := range l.layers {
			for i := 0; i+1 < len(ly); i++ {
				v, w := ly[i], ly[i+1]
				if l.vs[v].cluster != l.vs[w].cluster || l.vs[v].border != 0 || l.vs[w].border != 0 {
					continue
				}
				before := l.pairCross(r, v, w)
				after := l.pairCross(r, w, v)
				if after < before {
					ly[i], ly[i+1] = w, v
					l.vs[v].order, l.vs[w].order = i+1, i
					improved = true
				}
			}
		}
		if !improved {
			return
		}
	}
}

// pairCross returns the crossings of the edges of v and w with v left of
// w, with the ranks above and below.
func (l *layout) pairCross(_ int, v, w int) int {
	n := 0
	for _, nb := range [][][]int{l.up, l.down} {
		for _, a := range nb[v] {
			for _, b := range nb[w] {
				if l.vs[a].order > l.vs[b].order {
					n++
				}
			}
		}
	}
	return n
}

// crossings counts the crossings of the edges between each two ranks, as
// the inversions of the orders of their lower ends.
func (l *layout) crossings() int {
	total := 0
	for r := 0; r+1 < len(l.layers); r++ {
		type seg struct{ a, b int }
		var segs []seg
		for _, u := range l.layers[r] {
			for _, v := range l.down[u] {
				segs = append(segs, seg{l.vs[u].order, l.vs[v].order})
			}
		}
		slices.SortFunc(segs, func(x, y seg) int {
			if c := cmp.Compare(x.a, y.a); c != 0 {
				return c
			}
			return cmp.Compare(x.b, y.b)
		})
		bs := make([]int, len(segs))
		for i, s := range segs {
			bs[i] = s.b
		}
		total += inversions(bs)
	}
	return total
}

// inversions counts the pairs out of order in a list, by a merge sort.
func inversions(a []int) int {
	if len(a) < 2 {
		return 0
	}
	m := len(a) / 2
	left, right := slices.Clone(a[:m]), slices.Clone(a[m:])
	n := inversions(left) + inversions(right)
	i, j, k := 0, 0, 0
	for i < len(left) && j < len(right) {
		if left[i] <= right[j] {
			a[k] = left[i]
			i++
		} else {
			a[k] = right[j]
			n += len(left) - i
			j++
		}
		k++
	}
	for ; i < len(left); i, k = i+1, k+1 {
		a[k] = left[i]
	}
	for ; j < len(right); j, k = j+1, k+1 {
		a[k] = right[j]
	}
	return n
}
