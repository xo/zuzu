package graph

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"

	dagro "github.com/xo/zuzu/internal/dagre"
)

// The spaces of zuzu.dagre. Mermaid 12.3 gives dagre its spaces in the
// pixels of its look, whose text is 16 pixels, and the text of zuzu is 11
// pixels. So each space of Mermaid takes the ratio dagreScale, and keeps
// the ratio to the nodes that it has in Mermaid.
const (
	dagreScale = 11.0 / 16
	// dagreNodeSep and dagreRankSep turn NodeSep and RankSep into the
	// nodesep and the ranksep of dagre. The reader of Mermaid gives a
	// nodeSpacing of n the NodeSep n*24/50, and a rankSpacing of n the
	// RankSep n*40/50, so these turn them back into the spaces of Mermaid,
	// with the ratio dagreScale.
	dagreNodeSep = 50.0 / 24 * dagreScale
	dagreRankSep = 50.0 / 40 * dagreScale
	// dagreEdgeSep is the edgesep of dagre, 20, which Mermaid does not set.
	dagreEdgeSep = 20 * dagreScale
	// dagreClusterRankSep is what Mermaid adds to the ranksep of a cluster
	// that lays out on its own, 25.
	dagreClusterRankSep = 25 * dagreScale
	// dagreLoopNode is the size of each of the two nodes that Mermaid puts
	// on a loop, 10.
	dagreLoopNode = 10 * dagreScale
)

// Dagre returns the engine of zuzu.dagre: the layout of dagre 3.1.1, from
// the copy of dagro in internal/dagre, with the wrap of Mermaid 12.3 around
// it (D39 part 12, D44).
//
// The wrap does what Mermaid does with dagre. A cluster with no edge
// across its border lays out on its own, as a graph of its own, in its own
// direction. A cluster with no direction then takes
// LayoutInput.ClusterDirection, or else LR inside a graph of TB, and TB
// inside a graph of any other direction, as Mermaid does for a subgraph of
// a flowchart when flowchart.inheritDir is false, its default. The graph of the cluster has
// the ranksep of the graph around it plus 25 pixels of Mermaid, and its box
// is a node of the graph around it. An edge to a cluster with an edge
// across its border goes to a node in the cluster, and its route stops at
// the box of the cluster. A loop goes through two small nodes, and draws on
// the side of its node where they lie. With LayoutInput.ReverseClusters,
// the sibling clusters set in the reverse of their order, as Mermaid sets
// the subgraphs of a flowchart.
//
// The engine names in Layout.Unsupported the ports, the lanes, the groups
// of the same rank, the edges that take no part in the ranks, and a loop on
// a cluster, which dagre does not have. It lays out each such part as
// though the input did not ask for it. It gives no rank and no order.
func Dagre() Engine {
	return dagreEngine{}
}

// dagreEngine is the engine of Dagre.
type dagreEngine struct{}

// Name satisfies Engine.
func (dagreEngine) Name() string {
	return EngineDagre
}

// noEnd is an end that is neither a node nor a cluster.
const noEnd = math.MinInt32

// dagreRun holds the state of one call of the Lay of Dagre.
type dagreRun struct {
	in *LayoutInput
	// parent is the parent of each cluster, ncl the cluster of each node,
	// both -1 for none, and depth the depth of each cluster.
	parent, ncl, depth []int
	// order holds the clusters from the outer to the inner, as Mermaid
	// lists the subgraphs, with each list of siblings reversed when
	// LayoutInput.ReverseClusters says so.
	order []int
	// empty says that a cluster holds no node.
	empty []bool
	// ends holds the ends of each edge: a node i, or a cluster -1-c. It
	// holds the ends after the wrap moves an end at a cluster to a node.
	ends [][2]int
	// clip holds, for each end of each edge, the cluster at whose box the
	// route stops, or -1.
	clip [][2]int
	// ext says that an edge crosses the border of a cluster, and boxed
	// that the cluster lays out on its own.
	ext, boxed []bool
	// home is the boxed cluster whose graph holds each cluster, or -1 for
	// the graph of the whole layout.
	home []int
	// edgeOrder is the order of the edges in dagre: the edges with no end
	// at a cluster, then the others, as Mermaid sets them again.
	edgeOrder []int
	// ids holds the id in dagre of each node, and cids of each cluster.
	ids, cids []string
	used      map[string]bool
	// dirs and seps hold the direction and the nodesep and the ranksep of
	// the graph of each boxed cluster, by its index plus one, with the
	// graph of the whole layout at 0.
	dirs []string
	seps [][2]float64
	// scopes holds the layout of each graph, by the same index.
	scopes      []*dagreScope
	unsupported []string
}

// dagreScope is the layout of one graph in dagre: the graph of the whole
// layout, or of a boxed cluster.
type dagreScope struct {
	nodes    map[int]dagro.Attrs
	clusters map[int]dagro.Attrs
	edges    map[int]dagro.Attrs
	// loops holds the two nodes of each loop.
	loops map[int][2]dagro.Attrs
	// x, y, width and height are the box of the boxed cluster in its own
	// graph, its center and its size, with room for its title.
	x, y, width, height float64
}

// Lay satisfies Engine.
func (dagreEngine) Lay(in LayoutInput) (*Layout, error) {
	r := &dagreRun{in: &in, used: map[string]bool{}}
	if err := r.read(); err != nil {
		return nil, err
	}
	r.anchors()
	r.extract()
	// The inner graphs lay out first, so that the graphs around them know
	// the size of their boxes.
	for _, c := range slices.Backward(r.order) {
		if r.boxed[c] {
			if err := r.layScope(c); err != nil {
				return nil, err
			}
		}
	}
	if err := r.layScope(-1); err != nil {
		return nil, err
	}
	out := &Layout{
		Nodes: make([]NodePlace, len(in.Nodes)), Edges: make([]EdgePlace, len(in.Edges)), Clusters: make([]ClusterPlace, len(in.Clusters)),
	}
	r.place(-1, 0, 0, out)
	r.routes(out)
	Finish(r.in, out)
	out.Unsupported = r.unsupported
	return out, nil
}

// unsupportedf names a part of the input that dagre does not lay out.
func (r *dagreRun) unsupportedf(format string, args ...any) {
	r.unsupported = append(r.unsupported, fmt.Sprintf(format, args...)+", which zuzu.dagre leaves out")
}

// read reads the nodes, the clusters and the edges of the input, and names
// the parts that dagre does not lay out.
func (r *dagreRun) read() error {
	in := r.in
	nodes := map[string]int{}
	for i, n := range in.Nodes {
		if _, ok := nodes[n.ID]; ok {
			return fmt.Errorf("the node %q twice: %w", n.ID, ErrGraph)
		}
		nodes[n.ID] = i
	}
	nc := len(in.Clusters)
	cids := map[string]int{}
	for i, c := range in.Clusters {
		cids[c.ID] = i
	}
	r.parent, r.depth = make([]int, nc), make([]int, nc)
	for i, c := range in.Clusters {
		r.parent[i] = -1
		if p, ok := cids[c.Parent]; ok && c.Parent != "" && p != i {
			r.parent[i] = p
		}
		switch c.Direction {
		case "", TB, BT, LR, RL:
		default:
			return fmt.Errorf("the direction %q of the cluster %q: %w", c.Direction, c.ID, ErrGraph)
		}
	}
	for i := range nc {
		for p := r.parent[i]; p >= 0; p = r.parent[p] {
			if r.depth[i]++; r.depth[i] > nc {
				return fmt.Errorf("the cluster %q is inside itself: %w", in.Clusters[i].ID, ErrGraph)
			}
		}
	}
	r.order = make([]int, nc)
	for c := range nc {
		r.order[c] = c
	}
	slices.SortStableFunc(r.order, func(a, b int) int { return cmp.Compare(r.depth[a], r.depth[b]) })
	if in.ReverseClusters {
		r.order = r.reversedOrder()
	}
	r.ncl = make([]int, len(in.Nodes))
	r.empty = make([]bool, nc)
	for c := range nc {
		r.empty[c] = true
	}
	ranks := map[string]bool{}
	for i, n := range in.Nodes {
		r.ncl[i] = -1
		if c, ok := cids[n.Cluster]; ok && n.Cluster != "" {
			r.ncl[i] = c
			for x := c; x >= 0; x = r.parent[x] {
				r.empty[x] = false
			}
		}
		if n.Rank != "" && !ranks[n.Rank] {
			ranks[n.Rank] = true
			r.unsupportedf("the group of the same rank %q", n.Rank)
		}
	}
	if in.Lanes {
		r.unsupportedf("the lanes of the layout")
	}
	r.ends = make([][2]int, len(in.Edges))
	r.clip = make([][2]int, len(in.Edges))
	for i, e := range in.Edges {
		for k, id := range []string{e.Source, e.Target} {
			r.clip[i][k] = -1
			if n, ok := nodes[id]; ok {
				r.ends[i][k] = n
				continue
			}
			c, ok := cids[id]
			if !ok {
				return fmt.Errorf("an edge from or to %q, which is neither a node nor a cluster: %w", id, ErrGraph)
			}
			if r.empty[c] {
				return fmt.Errorf("an edge to the cluster %q, which holds no node: %w", id, ErrGraph)
			}
			r.ends[i][k] = -1 - c
		}
		if r.ends[i][0] < 0 && r.ends[i][0] == r.ends[i][1] {
			r.unsupportedf("the loop on the cluster %q", e.Source)
		}
		if e.SourcePort != nil || e.TargetPort != nil {
			r.unsupportedf("the port of the edge %s -> %s", e.Source, e.Target)
		}
		if e.NoConstraint {
			r.unsupportedf("constraint false on the edge %s -> %s", e.Source, e.Target)
		}
	}
	r.ids = make([]string, len(in.Nodes))
	for i, n := range in.Nodes {
		r.ids[i] = n.ID
		r.used[n.ID] = true
	}
	r.cids = make([]string, nc)
	for c, cl := range in.Clusters {
		r.cids[c] = r.newID(cl.ID)
	}
	return nil
}

// reversedOrder returns the clusters in the order of the wrap of dagre in
// Mermaid 12.3 for a flowchart: each cluster before the clusters inside it,
// and each list of siblings in the reverse of the order of the input.
// Mermaid lists a subgraph when it ends, so an inner subgraph comes before
// the subgraph around it, and it sets the subgraphs from the last of that
// list to the first (D45 part 5).
func (r *dagreRun) reversedOrder() []int {
	kids := map[int][]int{}
	for c := range r.in.Clusters {
		kids[r.parent[c]] = append(kids[r.parent[c]], c)
	}
	out := make([]int, 0, len(r.in.Clusters))
	var walk func(p int)
	walk = func(p int) {
		for _, c := range slices.Backward(kids[p]) {
			out = append(out, c)
			walk(c)
		}
	}
	walk(-1)
	return out
}

// newID returns an id for dagre that no other node has: the id itself, or
// the id with bytes 0 after it. No id of a source holds a byte 0.
func (r *dagreRun) newID(id string) string {
	for r.used[id] {
		id += "\x00"
	}
	r.used[id] = true
	return id
}

// within says whether a cluster x is c or inside it.
func (r *dagreRun) within(x, c int) bool {
	for ; x >= 0; x = r.parent[x] {
		if x == c {
			return true
		}
	}
	return false
}

// inside says whether an end, a node or a cluster, is inside a cluster c.
// A cluster is not inside itself.
func (r *dagreRun) inside(end, c int) bool {
	if end >= 0 {
		return r.within(r.ncl[end], c)
	}
	return -1-end != c && r.within(-1-end, c)
}

// parentOf returns the cluster that holds an end, or -1.
func (r *dagreRun) parentOf(end int) int {
	if end >= 0 {
		return r.ncl[end]
	}
	return r.parent[-1-end]
}

// children returns what a cluster holds directly, or the graph for -1: the
// clusters that hold a node, then the nodes, in the order of the input, as
// Mermaid sets them.
func (r *dagreRun) children(c int) []int {
	var out []int
	for _, d := range r.order {
		if r.parent[d] == c && !r.empty[d] {
			out = append(out, -1-d)
		}
	}
	for i := range r.in.Nodes {
		if r.ncl[i] == c {
			out = append(out, i)
		}
	}
	return out
}

// leaf returns the node that stands for an end inside the cluster c, as
// findNonClusterChild of Mermaid finds it: the end itself for a node, and
// else the first node down the clusters that shares no edge with c, or
// else the last that does.
func (r *dagreRun) leaf(end, c int) int {
	if end >= 0 {
		return end
	}
	reserve := noEnd
	for _, k := range r.children(-1 - end) {
		x := r.leaf(k, c)
		if x == noEnd {
			continue
		}
		if !r.shares(c, x) {
			return x
		}
		reserve = x
	}
	return reserve
}

// shares says whether a node x has an edge to its cluster c, or an edge to
// a node that c has an edge to, as findCommonEdges of Mermaid finds it.
func (r *dagreRun) shares(c, x int) bool {
	for _, ee := range r.ends {
		if ee[0] == x && ee[1] == -1-c {
			return true
		}
		if ee[0] != -1-c {
			continue
		}
		for _, ff := range r.ends {
			if ff[0] == x && ff[1] == ee[1] {
				return true
			}
		}
	}
	return false
}

// inBoxed says whether a cluster between an end and the cluster c, not c
// itself, has no edge across its border, as isNodeInExtractableCluster of
// Mermaid finds it.
func (r *dagreRun) inBoxed(end, c int) bool {
	for p := r.parentOf(end); p >= 0 && p != c; p = r.parent[p] {
		if !r.ext[p] {
			return true
		}
	}
	return false
}

// safeAnchor returns a node of the cluster c outside the cluster excluded
// that lies in no cluster with no edge across its border, or noEnd, as
// findSafeAnchorNode of Mermaid finds it.
func (r *dagreRun) safeAnchor(c, excluded int) int {
	for _, k := range r.children(c) {
		if excluded >= 0 && (k == -1-excluded || r.inside(k, excluded)) {
			continue
		}
		x := r.leaf(k, c)
		if x != noEnd && !r.inBoxed(x, c) {
			return x
		}
	}
	return noEnd
}

// anchors finds which clusters an edge crosses the border of, and moves
// each end at a cluster with such an edge to a node inside it, as
// adjustClustersAndEdges of Mermaid does. The route of such an edge stops
// at the box of the cluster.
func (r *dagreRun) anchors() {
	nc := len(r.in.Clusters)
	r.ext = make([]bool, nc)
	anchor := make([]int, nc)
	for c := range nc {
		anchor[c] = noEnd
		if r.empty[c] {
			continue
		}
		anchor[c] = r.leaf(-1-c, c)
		for _, ee := range r.ends {
			if r.inside(ee[0], c) != r.inside(ee[1], c) {
				r.ext[c] = true
				break
			}
		}
	}
	for _, c := range r.order {
		first := anchor[c]
		if first == noEnd {
			continue
		}
		if p := r.parentOf(first); p != c && p >= 0 && !r.ext[p] {
			anchor[c] = -1 - p
		}
		out := false
		for _, ee := range r.ends {
			out = out || ee[0] == -1-c
		}
		if r.ext[c] && out && r.inBoxed(first, c) {
			if s := r.safeAnchor(c, r.parentOf(first)); s != noEnd {
				anchor[c] = s
			}
		}
	}
	var kept, moved []int
	for i := range r.ends {
		ee := &r.ends[i]
		if ee[0] >= 0 && ee[1] >= 0 || ee[0] == ee[1] {
			// A loop on a cluster stays, and draws nothing, as read
			// reports.
			kept = append(kept, i)
			continue
		}
		for k := range 2 {
			x := ee[k]
			if x >= 0 || !r.ext[-1-x] || anchor[-1-x] == noEnd {
				continue
			}
			ee[k], r.clip[i][k] = anchor[-1-x], -1-x
			if p := r.parentOf(ee[k]); p >= 0 {
				r.ext[p] = true
			}
		}
		moved = append(moved, i)
	}
	r.edgeOrder = slices.Concat(kept, moved)
	// An end can stay at a cluster that a later edge gave an edge across
	// its border. It moves to the node of the cluster too.
	for i := range r.ends {
		if r.ends[i][0] == r.ends[i][1] {
			continue
		}
		for k := range 2 {
			if x := r.ends[i][k]; x < 0 && r.ext[-1-x] && anchor[-1-x] != noEnd && anchor[-1-x] != x {
				r.ends[i][k], r.clip[i][k] = anchor[-1-x], -1-x
			}
		}
	}
}

// extract sets which clusters lay out on their own, each cluster with no
// edge across its border, and the direction and the spaces of each graph,
// as the extractor of Mermaid does.
func (r *dagreRun) extract() {
	in := r.in
	nc := len(in.Clusters)
	r.boxed, r.home = make([]bool, nc), make([]int, nc)
	for c := range nc {
		r.boxed[c] = !r.ext[c] && !r.empty[c]
	}
	for c := range nc {
		r.home[c] = r.boxedAt(r.parent[c])
	}
	nodeSep, rankSep := in.NodeSep, in.RankSep
	if nodeSep <= 0 {
		nodeSep = 24
	}
	if rankSep <= 0 {
		rankSep = 40
	}
	r.dirs, r.seps = make([]string, nc+1), make([][2]float64, nc+1)
	r.scopes = make([]*dagreScope, nc+1)
	r.dirs[0] = cmp.Or(in.Direction, TB)
	r.seps[0] = [2]float64{nodeSep * dagreNodeSep, rankSep * dagreRankSep}
	for _, c := range r.order {
		if !r.boxed[c] {
			continue
		}
		h := r.home[c] + 1
		dir := cmp.Or(in.Clusters[c].Direction, in.ClusterDirection)
		if dir == "" {
			dir = TB
			if r.dirs[h] == TB {
				dir = LR
			}
		}
		r.dirs[c+1] = dir
		r.seps[c+1] = [2]float64{r.seps[h][0], r.seps[h][1] + dagreClusterRankSep}
	}
}

// boxedAt returns the innermost boxed cluster that is c or holds it, or -1.
func (r *dagreRun) boxedAt(c int) int {
	for ; c >= 0 && !r.boxed[c]; c = r.parent[c] {
	}
	return c
}

// rep returns the id in the graph of s of an end: the end itself when the
// graph holds it, the box of the boxed cluster in s that holds it, or
// false when the end is not in the graph of s.
func (r *dagreRun) rep(end, s int) (string, bool) {
	var x int
	if end >= 0 {
		x = r.boxedAt(r.ncl[end])
		if x == s {
			return r.ids[end], true
		}
	} else {
		x = -1 - end
	}
	for ; x >= 0; x = r.home[x] {
		if r.home[x] == s && r.boxed[x] {
			return r.cids[x], true
		}
	}
	return "", false
}

// layScope lays out the graph of a boxed cluster, or of the whole layout
// for -1, with dagre.
func (r *dagreRun) layScope(s int) error {
	in := r.in
	sc := &dagreScope{
		nodes: map[int]dagro.Attrs{}, clusters: map[int]dagro.Attrs{}, edges: map[int]dagro.Attrs{}, loops: map[int][2]dagro.Attrs{},
	}
	r.scopes[s+1] = sc
	g := dagro.NewGraph(dagro.GraphOptions{Compound: true, Multigraph: true}).SetGraph(dagro.Attrs{
		"rankdir": r.dirs[s+1], "nodesep": r.seps[s+1][0], "ranksep": r.seps[s+1][1], "edgesep": dagreEdgeSep,
	})
	// parentID returns the id of the cluster that holds a node or a
	// cluster in the graph, or empty for none.
	parentID := func(c int) string {
		if c < 0 {
			return ""
		}
		return r.cids[c]
	}
	type member struct {
		id, parent string
	}
	var members []member
	// add adds what a boxed cluster holds, as the copy of Mermaid sets it,
	// each cluster after what it holds.
	var add func(c int)
	add = func(c int) {
		for _, k := range r.children(c) {
			if k >= 0 {
				a := dagro.Attrs{"width": in.Nodes[k].Width, "height": in.Nodes[k].Height}
				g.SetNode(r.ids[k], a)
				sc.nodes[k] = a
				members = append(members, member{r.ids[k], parentID(c)})
				continue
			}
			d := -1 - k
			if r.boxed[d] {
				inner := r.scopes[d+1]
				a := dagro.Attrs{"width": inner.width, "height": inner.height}
				g.SetNode(r.cids[d], a)
				sc.clusters[d] = a
				members = append(members, member{r.cids[d], parentID(c)})
				continue
			}
			add(d)
			a := dagro.Attrs{}
			g.SetNode(r.cids[d], a)
			sc.clusters[d] = a
			members = append(members, member{r.cids[d], parentID(c)})
		}
	}
	if s < 0 {
		// Mermaid sets each subgraph, from the outer to the inner, then
		// each node.
		for _, c := range r.order {
			if !r.empty[c] && r.home[c] == -1 {
				a := dagro.Attrs{}
				if r.boxed[c] {
					inner := r.scopes[c+1]
					a = dagro.Attrs{"width": inner.width, "height": inner.height}
				}
				g.SetNode(r.cids[c], a)
				sc.clusters[c] = a
				members = append(members, member{r.cids[c], parentID(r.parent[c])})
			}
		}
		for i := range in.Nodes {
			if r.boxedAt(r.ncl[i]) == -1 {
				a := dagro.Attrs{"width": in.Nodes[i].Width, "height": in.Nodes[i].Height}
				g.SetNode(r.ids[i], a)
				sc.nodes[i] = a
				members = append(members, member{r.ids[i], parentID(r.ncl[i])})
			}
		}
	} else {
		add(s)
		a := dagro.Attrs{}
		g.SetNode(r.cids[s], a)
		sc.clusters[s] = a
	}
	for _, m := range members {
		if m.parent != "" {
			if err := g.SetParent(m.id, m.parent); err != nil {
				return fmt.Errorf("laying out with dagre: %w: %w", ErrGraph, err)
			}
		}
	}
	for _, i := range r.edgeOrder {
		e := in.Edges[i]
		a, aok := r.rep(r.ends[i][0], s)
		b, bok := r.rep(r.ends[i][1], s)
		if !aok || !bok || r.ends[i][0] < 0 && r.ends[i][0] == r.ends[i][1] {
			continue
		}
		if a == b && r.ends[i][0] != r.ends[i][1] {
			// Both ends lie in one box, and the graph of the box holds
			// the edge.
			continue
		}
		attrs := func(label bool) dagro.Attrs {
			at := dagro.Attrs{"minlen": float64(max(e.MinLen, 1)), "labelpos": "c"}
			if e.Weight != nil {
				at["weight"] = *e.Weight
			}
			if label && e.LabelWidth > 0 {
				at["width"], at["height"] = e.LabelWidth, e.LabelHeight
			}
			return at
		}
		name := strconv.Itoa(i)
		if a != b {
			at := attrs(true)
			g.SetEdge(a, b, at, name)
			sc.edges[i] = at
			continue
		}
		// A loop goes through two small nodes in the cluster of its node,
		// and its label sits on the edge between them.
		n := r.ends[i][0]
		var dummies [2]dagro.Attrs
		var dids [2]string
		for k := range 2 {
			dids[k] = r.newID(a + "---" + a + "---" + strconv.Itoa(k+1))
			dummies[k] = dagro.Attrs{"width": dagreLoopNode, "height": dagreLoopNode}
			g.SetNode(dids[k], dummies[k])
			if p := r.ncl[n]; p >= 0 {
				if err := g.SetParent(dids[k], r.cids[p]); err != nil {
					return fmt.Errorf("laying out with dagre: %w: %w", ErrGraph, err)
				}
			}
		}
		g.SetEdge(a, dids[0], attrs(false), name+"-0")
		mid := attrs(true)
		g.SetEdge(dids[0], dids[1], mid, name+"-1")
		g.SetEdge(dids[1], a, attrs(false), name+"-2")
		sc.edges[i] = mid
		sc.loops[i] = dummies
	}
	if err := dagro.Layout(g); err != nil {
		return fmt.Errorf("laying out with dagre: %w: %w", ErrGraph, err)
	}
	if s >= 0 {
		box := sc.clusters[s]
		pad := r.pad()
		sc.x, sc.y = attrNum(box, "x"), attrNum(box, "y")
		sc.width = math.Max(attrNum(box, "width"), in.Clusters[s].TitleWidth+2*pad)
		sc.height = attrNum(box, "height")
	}
	return nil
}

// pad returns the padding of a cluster.
func (r *dagreRun) pad() float64 {
	if r.in.ClusterPadding <= 0 {
		return 12
	}
	return r.in.ClusterPadding
}

// attrNum returns a number of the attributes of dagre, or 0.
func attrNum(a dagro.Attrs, key string) float64 {
	v, _ := a[key].(float64)
	return v
}

// place moves the places of the graph of s by dx and dy into out, and the
// graphs of its boxed clusters into their boxes.
func (r *dagreRun) place(s int, dx, dy float64, out *Layout) {
	in := r.in
	sc := r.scopes[s+1]
	for i, a := range sc.nodes {
		out.Nodes[i] = NodePlace{X: attrNum(a, "x") + dx, Y: attrNum(a, "y") + dy, Width: in.Nodes[i].Width, Height: in.Nodes[i].Height}
	}
	pad := r.pad()
	for c, a := range sc.clusters {
		if c == s {
			continue
		}
		x, y := attrNum(a, "x")+dx, attrNum(a, "y")+dy
		w, h := attrNum(a, "width"), attrNum(a, "height")
		if r.boxed[c] {
			inner := r.scopes[c+1]
			w, h = inner.width, inner.height
			r.place(c, x-inner.x, y-inner.y, out)
		}
		w = math.Max(w, in.Clusters[c].TitleWidth+2*pad)
		x1, y1 := x-w/2, y-h/2
		out.Clusters[c] = ClusterPlace{X: x1, Y: y1, Width: w, Height: h, Depth: r.depth[c], TitleX: x1 + pad, TitleY: y1 + pad/2 + in.Clusters[c].TitleTop}
	}
	for i, a := range sc.edges {
		ep := &out.Edges[i]
		pts, _ := a["points"].([]dagro.Point)
		ep.Points = make([][2]float64, len(pts))
		for j, p := range pts {
			ep.Points[j] = [2]float64{p.X + dx, p.Y + dy}
		}
		if _, ok := a["x"]; ok {
			ep.LabelX, ep.LabelY = attrNum(a, "x")+dx, attrNum(a, "y")+dy
		}
		if d, ok := sc.loops[i]; ok {
			ep.Points = [][2]float64{{attrNum(d[0], "x") + dx, attrNum(d[0], "y") + dy}, {attrNum(d[1], "x") + dx, attrNum(d[1], "y") + dy}}
		}
	}
}

// routes sets the route of each edge from the points of dagre: the ends
// of an edge at a node on the outline of its shape, the route of an edge
// to a cluster stopped at the box of the cluster, and each loop on the side
// of its node where its two nodes lie.
func (r *dagreRun) routes(out *Layout) {
	in := r.in
	for i, e := range in.Edges {
		ep := &out.Edges[i]
		if len(ep.Points) < 2 {
			ep.Points = nil
			continue
		}
		a, b := r.ends[i][0], r.ends[i][1]
		if a >= 0 && a == b {
			r.loop(i, ep, out.Nodes[a])
			continue
		}
		pts := ep.Points
		last := len(pts) - 1
		if a >= 0 && r.clip[i][0] < 0 {
			pts[0][0], pts[0][1] = clip(out.Nodes[a], in.Nodes[a].Shape, pts[1][0], pts[1][1])
		}
		if b >= 0 && r.clip[i][1] < 0 {
			pts[last][0], pts[last][1] = clip(out.Nodes[b], in.Nodes[b].Shape, pts[last-1][0], pts[last-1][1])
		}
		if c := r.clip[i][1]; c >= 0 {
			if cut, ok := cutAtBox(pts, out.Clusters[c]); ok {
				pts = cut
			}
		}
		if c := r.clip[i][0]; c >= 0 {
			rev := slices.Clone(pts)
			slices.Reverse(rev)
			if cut, ok := cutAtBox(rev, out.Clusters[c]); ok {
				slices.Reverse(cut)
				pts = cut
			}
		}
		ep.Points = pts
		if e.LabelWidth <= 0 {
			m := len(pts) / 2
			ep.LabelX, ep.LabelY = (pts[m-1][0]+pts[m][0])/2, (pts[m-1][1]+pts[m][1])/2
		}
	}
}

// loop routes a loop on a node as Mermaid 12.3 draws it: four points out
// of the side of the node where the two nodes of the loop lie, with the
// label beyond them. Each length of Mermaid takes the ratio dagreScale.
func (r *dagreRun) loop(i int, ep *EdgePlace, n NodePlace) {
	e := r.in.Edges[i]
	// The side: the mean of the two nodes of the loop, from the center of
	// the node.
	cx, cy := (ep.Points[0][0]+ep.Points[1][0])/2, (ep.Points[0][1]+ep.Points[1][1])/2
	dx, dy := cx-n.X, cy-n.Y
	side := map[string]string{TB: "top", BT: "bottom", LR: "right", RL: "left"}[r.dirs[r.boxedAt(r.ncl[r.ends[i][0]])+1]]
	switch {
	case math.Abs(dx) > math.Abs(dy) && dx > 0:
		side = "right"
	case math.Abs(dx) > math.Abs(dy):
		side = "left"
	case dy > 0:
		side = "bottom"
	case dy < 0:
		side = "top"
	}
	clamp := func(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
	s := dagreScale
	maxSpan := math.Max(36*s, math.Min(100*s, n.Width*0.8))
	span := clamp(math.Max(e.LabelWidth, n.Width*0.35), 36*s, maxSpan)
	depth := clamp(math.Min(n.Width, n.Height)*0.45, 24*s, 48*s)
	hw, hh := n.Width/2, n.Height/2
	gap := 4 * s
	var pts [][2]float64
	switch side {
	case "bottom":
		b := n.Y + hh
		pts = [][2]float64{{n.X - span/2, b}, {n.X - span/2, b + depth}, {n.X + span/2, b + depth}, {n.X + span/2, b}}
		ep.LabelX, ep.LabelY = n.X, b+depth+e.LabelHeight/2+gap
	case "right":
		rt := n.X + hw
		pts = [][2]float64{{rt, n.Y - span/2}, {rt + depth, n.Y - span/2}, {rt + depth, n.Y + span/2}, {rt, n.Y + span/2}}
		ep.LabelX, ep.LabelY = rt+depth+e.LabelWidth/2+gap, n.Y
	case "left":
		l := n.X - hw
		pts = [][2]float64{{l, n.Y - span/2}, {l - depth, n.Y - span/2}, {l - depth, n.Y + span/2}, {l, n.Y + span/2}}
		ep.LabelX, ep.LabelY = l-depth-e.LabelWidth/2-gap, n.Y
	default:
		t := n.Y - hh
		pts = [][2]float64{{n.X - span/2, t}, {n.X - span/2, t - depth}, {n.X + span/2, t - depth}, {n.X + span/2, t}}
		ep.LabelX, ep.LabelY = n.X, t-depth-e.LabelHeight/2-gap
	}
	ep.Points = pts
}
