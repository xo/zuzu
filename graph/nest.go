package graph

import (
	"fmt"
	"math"
	"slices"
)

// Lay lays out a graph in ranks (D31). A cluster with a direction of its
// own, or a cluster that an edge names, lays out on its own when no edge
// links a node inside it to a node outside it, as in Mermaid. Its layout
// then takes the place of one node, of its size, in the layout that holds
// it. An edge to any other cluster ends at the box of the cluster: the
// layout takes it to the first node of the cluster, and the route stops at
// the box.
func Lay(in LayoutInput) (*Layout, error) {
	n, err := newNest(&in)
	if err != nil {
		return nil, err
	}
	if n == nil {
		return lay(in)
	}
	return n.lay()
}

// FollowedDirections returns the ids of the clusters of a graph whose
// direction the layout does not keep, because an edge links a node inside
// the cluster to a node outside it. Each such cluster follows the direction
// of the layout that holds it, and the map gives that direction. A
// direction that is the same as the one it follows is not in the map.
func FollowedDirections(g *Graph) map[string]string {
	in := LayoutInput{Direction: g.Direction}
	for _, n := range g.Nodes {
		in.Nodes = append(in.Nodes, LayoutNode{ID: n.ID, Cluster: n.Cluster})
	}
	for _, e := range g.Edges {
		in.Edges = append(in.Edges, LayoutEdge{Source: e.Source, Target: e.Target})
	}
	for _, c := range g.Clusters {
		in.Clusters = append(in.Clusters, LayoutCluster{ID: c.ID, Parent: c.Parent, Direction: c.Direction})
	}
	n, err := newNest(&in)
	if err != nil || n == nil {
		return nil
	}
	out := map[string]string{}
	for c, cl := range in.Clusters {
		if cl.Direction == "" || n.boxed[c] {
			continue
		}
		if d := n.dirOf(n.cscope[c]); d != cl.Direction {
			out[cl.ID] = d
		}
	}
	return out
}

// nest holds the clusters that lay out on their own, and the ends of the
// edges, for one call of Lay.
type nest struct {
	in     *LayoutInput
	parent []int // the parent of each cluster, or -1
	ncl    []int // the cluster of each node, or -1
	depth  []int
	boxed  []bool
	dir    []string // the direction of each boxed cluster
	// nscope and cscope are the innermost boxed cluster that holds each
	// node, and each cluster but itself, or -1 for none.
	nscope, cscope []int
	// ends holds the two ends of each edge in the layout: the index of a
	// node, or -1-c for a boxed cluster c.
	ends [][2]int
	// clip holds, for each end of each edge, the cluster at whose box the
	// route stops, or -1.
	clip [][2]int
}

// newNest reads the clusters and the edges of an input. It returns nil
// when no cluster has a direction and no edge names a cluster, so that the
// layout of such a graph does not change.
func newNest(in *LayoutInput) (*nest, error) {
	nodes := map[string]int{}
	for i, nd := range in.Nodes {
		if _, ok := nodes[nd.ID]; ok {
			return nil, fmt.Errorf("the node %q twice: %w", nd.ID, ErrGraph)
		}
		nodes[nd.ID] = i
	}
	nc := len(in.Clusters)
	cids := map[string]int{}
	for i, c := range in.Clusters {
		cids[c.ID] = i
	}
	need := false
	for _, c := range in.Clusters {
		switch c.Direction {
		case "":
		case TB, BT, LR, RL:
			need = true
		default:
			return nil, fmt.Errorf("the direction %q of the cluster %q: %w", c.Direction, c.ID, ErrGraph)
		}
	}
	// A node wins over a cluster of the same id.
	named := make([]bool, nc)
	for _, e := range in.Edges {
		for _, id := range []string{e.Source, e.Target} {
			if _, ok := nodes[id]; ok {
				continue
			}
			c, ok := cids[id]
			if !ok {
				// lay returns the error of an end that is not a node.
				return nil, nil
			}
			named[c], need = true, true
		}
	}
	if !need {
		return nil, nil
	}
	n := &nest{in: in, parent: make([]int, nc), depth: make([]int, nc)}
	for i, c := range in.Clusters {
		n.parent[i] = -1
		if p, ok := cids[c.Parent]; ok && c.Parent != "" && p != i {
			n.parent[i] = p
		}
	}
	for i := range nc {
		for p := n.parent[i]; p >= 0; p = n.parent[p] {
			if n.depth[i]++; n.depth[i] > nc {
				return nil, fmt.Errorf("the cluster %q is inside itself: %w", in.Clusters[i].ID, ErrGraph)
			}
		}
	}
	// within says whether a cluster is c or inside it.
	within := func(x, c int) bool {
		for ; x >= 0; x = n.parent[x] {
			if x == c {
				return true
			}
		}
		return false
	}
	n.ncl = make([]int, len(in.Nodes))
	ncl := n.ncl
	for i, nd := range in.Nodes {
		ncl[i] = -1
		if c, ok := cids[nd.Cluster]; ok && nd.Cluster != "" {
			ncl[i] = c
		}
	}
	// anchor is the first node of each cluster, or -1 for a cluster that
	// holds no node.
	anchor := make([]int, nc)
	for c := range nc {
		anchor[c] = -1
		for i := range in.Nodes {
			if within(ncl[i], c) {
				anchor[c] = i
				break
			}
		}
		if named[c] && anchor[c] < 0 {
			return nil, fmt.Errorf("an edge to the cluster %q, which holds no node: %w", in.Clusters[c].ID, ErrGraph)
		}
	}
	// An end is a node, i, or a cluster, -1-c.
	end := func(id string) int {
		if i, ok := nodes[id]; ok {
			return i
		}
		return -1 - cids[id]
	}
	// inside says whether an end is inside a cluster. A cluster is not
	// inside itself.
	inside := func(e, c int) bool {
		if e >= 0 {
			return within(ncl[e], c)
		}
		return within(n.parent[-1-e], c)
	}
	n.ends = make([][2]int, len(in.Edges))
	n.clip = make([][2]int, len(in.Edges))
	external := make([]bool, nc)
	n.boxed = make([]bool, nc)
	for {
		for c := range nc {
			n.boxed[c] = !external[c] && anchor[c] >= 0 && (in.Clusters[c].Direction != "" || named[c])
		}
		for i, e := range in.Edges {
			for k, id := range []string{e.Source, e.Target} {
				x := end(id)
				n.ends[i][k], n.clip[i][k] = x, -1
				if x < 0 && !n.boxed[-1-x] {
					n.ends[i][k], n.clip[i][k] = anchor[-1-x], -1-x
				}
			}
		}
		changed := false
		for c := range nc {
			if external[c] {
				continue
			}
			for _, ee := range n.ends {
				if inside(ee[0], c) != inside(ee[1], c) {
					external[c], changed = true, true
					break
				}
			}
		}
		if !changed {
			break
		}
	}
	n.nscope, n.cscope = make([]int, len(in.Nodes)), make([]int, nc)
	boxedAt := func(x int) int {
		for ; x >= 0 && !n.boxed[x]; x = n.parent[x] {
		}
		return x
	}
	for i := range in.Nodes {
		n.nscope[i] = boxedAt(ncl[i])
	}
	for c := range nc {
		n.cscope[c] = boxedAt(n.parent[c])
	}
	n.dir = make([]string, nc)
	// The clusters go from the outer to the inner, so that a cluster finds
	// the direction of the one that holds it.
	order := make([]int, nc)
	for c := range nc {
		order[c] = c
	}
	slices.SortStableFunc(order, func(a, b int) int { return n.depth[a] - n.depth[b] })
	for _, c := range order {
		if n.boxed[c] {
			n.dir[c] = in.Clusters[c].Direction
			if n.dir[c] == "" {
				n.dir[c] = n.dirOf(n.cscope[c])
			}
		}
	}
	return n, nil
}

// dirOf returns the direction of a boxed cluster, or of the graph for -1.
func (n *nest) dirOf(s int) string {
	if s >= 0 {
		return n.dir[s]
	}
	if n.in.Direction == "" {
		return TB
	}
	return n.in.Direction
}

// scoped is the layout of the graph or of a boxed cluster: the layout of
// its input, with what each node, cluster and edge of that input is.
type scoped struct {
	l *Layout
	// node is the node of each node of the input, or -1 for a boxed
	// cluster, which box names.
	node, box []int
	cluster   []int
	edge      []int
	inner     map[int]*scoped
	// width and height are the size of the layout inside its box.
	width, height float64
}

// boxID is the id of the node that stands for a boxed cluster. No id of a
// source starts with a byte 0.
func boxID(c int) string {
	return fmt.Sprintf("\x00%d", c)
}

// pad returns the padding of a cluster.
func (n *nest) pad() float64 {
	if n.in.ClusterPadding <= 0 {
		return 12
	}
	return n.in.ClusterPadding
}

// boxSize returns the size of the box of a boxed cluster, with its padding
// and its title.
func (n *nest) boxSize(c int, s *scoped) (float64, float64) {
	cl := n.in.Clusters[c]
	p := n.pad()
	return math.Max(s.width, cl.TitleWidth) + 2*p, s.height + 2*p + cl.TitleHeight
}

// layScope lays out the graph, for -1, or a boxed cluster: its nodes, its
// clusters, and a node for each boxed cluster in it, which it lays out
// first.
func (n *nest) layScope(s int) (*scoped, error) {
	in := n.in
	out := &scoped{inner: map[int]*scoped{}}
	sub := LayoutInput{Direction: n.dirOf(s), RankSep: in.RankSep, NodeSep: in.NodeSep, ClusterPadding: in.ClusterPadding}
	clusterID := func(c int) string {
		if c == s || c < 0 {
			return ""
		}
		return in.Clusters[c].ID
	}
	// child returns the boxed cluster in s that holds a boxed cluster x,
	// or x itself, or -1 when x is not inside s.
	child := func(x int) int {
		for ; x >= 0; x = n.cscope[x] {
			if n.cscope[x] == s {
				return x
			}
		}
		return -1
	}
	for i, nd := range in.Nodes {
		if n.nscope[i] == s {
			nd.Cluster = clusterID(n.ncl[i])
			sub.Nodes = append(sub.Nodes, nd)
			out.node, out.box = append(out.node, i), append(out.box, -1)
			continue
		}
		b := child(n.nscope[i])
		if b < 0 || slices.Contains(out.box, b) {
			continue
		}
		inner, err := n.layScope(b)
		if err != nil {
			return nil, err
		}
		out.inner[b] = inner
		w, h := n.boxSize(b, inner)
		sub.Nodes = append(sub.Nodes, LayoutNode{ID: boxID(b), Cluster: clusterID(n.parent[b]), Shape: "rect", Width: w, Height: h})
		out.node, out.box = append(out.node, -1), append(out.box, b)
	}
	for c, cl := range in.Clusters {
		if n.boxed[c] || n.cscope[c] != s {
			continue
		}
		sub.Clusters = append(sub.Clusters, LayoutCluster{ID: cl.ID, Parent: clusterID(n.parent[c]), TitleWidth: cl.TitleWidth, TitleHeight: cl.TitleHeight, TitleTop: cl.TitleTop})
		out.cluster = append(out.cluster, c)
	}
	// rep returns the id in s of an end, or false when the end is not in s.
	rep := func(e int) (string, bool) {
		if e >= 0 {
			if n.nscope[e] == s {
				return in.Nodes[e].ID, true
			}
			b := child(n.nscope[e])
			return boxID(b), b >= 0
		}
		b := child(-1 - e)
		return boxID(b), b >= 0
	}
	for i, e := range in.Edges {
		a, aok := rep(n.ends[i][0])
		b, bok := rep(n.ends[i][1])
		if !aok || !bok || n.inScope(n.ends[i], s) {
			continue
		}
		if n.ends[i][0] < 0 || a != in.Nodes[n.ends[i][0]].ID {
			e.SourcePort, e.SourceSide = nil, nil
		}
		if n.ends[i][1] < 0 || b != in.Nodes[n.ends[i][1]].ID {
			e.TargetPort, e.TargetSide = nil, nil
		}
		e.Source, e.Target = a, b
		sub.Edges = append(sub.Edges, e)
		out.edge = append(out.edge, i)
	}
	l, err := lay(sub)
	if err != nil {
		return nil, err
	}
	out.l, out.width, out.height = l, l.Width, l.Height
	return out, nil
}

// inScope says whether both ends of an edge are in one boxed cluster
// inside s, so that the edge belongs to the layout of that cluster.
func (n *nest) inScope(ends [2]int, s int) bool {
	// scopes returns the boxed clusters inside s that hold an end. A
	// boxed cluster at an end is a node of the one that holds it.
	scopes := func(e int) []int {
		var out []int
		var x int
		if e >= 0 {
			x = n.nscope[e]
		} else {
			x = n.cscope[-1-e]
		}
		for ; x >= 0 && x != s; x = n.cscope[x] {
			out = append(out, x)
		}
		return out
	}
	a, b := scopes(ends[0]), scopes(ends[1])
	for _, x := range a {
		if slices.Contains(b, x) {
			return true
		}
	}
	return false
}

// lay lays out the graph and each boxed cluster in it, places each boxed
// cluster in the node that stands for it, and stops each edge to a cluster
// that is not boxed at its box.
func (n *nest) lay() (*Layout, error) {
	root, err := n.layScope(-1)
	if err != nil {
		return nil, err
	}
	in := n.in
	out := &Layout{
		Width: root.l.Width, Height: root.l.Height,
		Nodes: make([]NodePlace, len(in.Nodes)), Edges: make([]EdgePlace, len(in.Edges)), Clusters: make([]ClusterPlace, len(in.Clusters)),
	}
	n.place(root, 0, 0, out)
	for i := range in.Edges {
		n.clipEdge(i, out)
	}
	return out, nil
}

// place moves the places of a layout by dx and dy into out, and the
// layouts of its boxed clusters into their boxes.
func (n *nest) place(s *scoped, dx, dy float64, out *Layout) {
	in := n.in
	for k, np := range s.l.Nodes {
		np.X, np.Y = np.X+dx, np.Y+dy
		np.shiftLabels(dx, dy)
		if i := s.node[k]; i >= 0 {
			out.Nodes[i] = np
			continue
		}
		c := s.box[k]
		cl := in.Clusters[c]
		x1, y1 := np.X-np.Width/2, np.Y-np.Height/2
		p := n.pad()
		out.Clusters[c] = ClusterPlace{X: x1, Y: y1, Width: np.Width, Height: np.Height, Depth: n.depth[c], TitleX: x1 + p, TitleY: y1 + p/2 + cl.TitleTop}
		inner := s.inner[c]
		n.place(inner, x1+(np.Width-inner.width)/2, y1+p+cl.TitleHeight, out)
	}
	for k, cp := range s.l.Clusters {
		c := s.cluster[k]
		if cp.Width > 0 {
			cp.X, cp.Y, cp.TitleX, cp.TitleY = cp.X+dx, cp.Y+dy, cp.TitleX+dx, cp.TitleY+dy
		}
		cp.Depth = n.depth[c]
		out.Clusters[c] = cp
	}
	for k, ep := range s.l.Edges {
		i := s.edge[k]
		pts := make([][2]float64, len(ep.Points))
		for j, p := range ep.Points {
			pts[j] = [2]float64{p[0] + dx, p[1] + dy}
		}
		ep.Points = pts
		ep.LabelX, ep.LabelY = ep.LabelX+dx, ep.LabelY+dy
		ep.EndX, ep.EndY, ep.StartX, ep.StartY = ep.EndX+dx, ep.EndY+dy, ep.StartX+dx, ep.StartY+dy
		ep.shiftLabels(dx, dy)
		ep.Path = n.path(i, pts)
		out.Edges[i] = ep
	}
}

// path returns the path of the route of an edge.
func (n *nest) path(i int, pts [][2]float64) string {
	if len(pts) == 0 {
		return ""
	}
	if n.ends[i][0] == n.ends[i][1] {
		return loopPath(pts)
	}
	return curvePath(n.in.Edges[i].Curve, pts)
}

// clipEdge stops the route of an edge at the box of the cluster at each of
// its ends that names a cluster that is not boxed.
func (n *nest) clipEdge(i int, out *Layout) {
	ep := &out.Edges[i]
	if len(ep.Points) < 2 || n.ends[i][0] == n.ends[i][1] {
		return
	}
	changed := false
	if c := n.clip[i][1]; c >= 0 {
		if pts, ok := cutAtBox(ep.Points, out.Clusters[c]); ok {
			ep.Points, changed = pts, true
		}
	}
	if c := n.clip[i][0]; c >= 0 {
		pts := slices.Clone(ep.Points)
		slices.Reverse(pts)
		if pts, ok := cutAtBox(pts, out.Clusters[c]); ok {
			slices.Reverse(pts)
			ep.Points, changed = pts, true
		}
	}
	if !changed {
		return
	}
	pts := ep.Points
	last := len(pts) - 1
	ep.EndX, ep.EndY = pts[last][0], pts[last][1]
	ep.EndAngle = math.Atan2(pts[last][1]-pts[last-1][1], pts[last][0]-pts[last-1][0]) * 180 / math.Pi
	ep.StartX, ep.StartY = pts[0][0], pts[0][1]
	ep.StartAngle = math.Atan2(pts[0][1]-pts[1][1], pts[0][0]-pts[1][0]) * 180 / math.Pi
	ep.Path = n.path(i, pts)
	placeEnds(n.in.Edges[i], ep)
}

// cutAtBox cuts a route where it first enters a box, and returns false
// when it starts inside the box or never enters it.
func cutAtBox(pts [][2]float64, b ClusterPlace) ([][2]float64, bool) {
	if b.Width <= 0 {
		return nil, false
	}
	in := func(p [2]float64) bool {
		return p[0] > b.X && p[0] < b.X+b.Width && p[1] > b.Y && p[1] < b.Y+b.Height
	}
	if in(pts[0]) {
		return nil, false
	}
	for j := 1; j < len(pts); j++ {
		if !in(pts[j]) {
			continue
		}
		p, q := pts[j-1], pts[j]
		// The least t of the line from p to q on a side of the box that
		// faces p.
		t := 1.0
		dx, dy := q[0]-p[0], q[1]-p[1]
		for _, side := range []struct{ d, from, at float64 }{
			{dx, p[0], b.X}, {dx, p[0], b.X + b.Width}, {dy, p[1], b.Y}, {dy, p[1], b.Y + b.Height},
		} {
			if side.d == 0 {
				continue
			}
			if u := (side.at - side.from) / side.d; u >= 0 && u < t {
				x, y := p[0]+u*dx, p[1]+u*dy
				if x >= b.X-1e-9 && x <= b.X+b.Width+1e-9 && y >= b.Y-1e-9 && y <= b.Y+b.Height+1e-9 {
					t = u
				}
			}
		}
		cut := append(slices.Clone(pts[:j]), [2]float64{p[0] + t*dx, p[1] + t*dy})
		return cut, true
	}
	return nil, false
}
