package graph

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// The lane layout is the layout of a swimlane diagram of Mermaid 12.3. Each
// cluster at the top of the graph is a lane, a band that runs along the
// ranks, beside the other lanes in the order of the input. A node is in the
// lane of the cluster at the top that holds it. The ranks run as the
// swimlane layout of Mermaid ranks them: an edge between two lanes does not
// push its target to the next rank, and a lane holds one node in each rank.
// The edges route at right angles between the nodes.

// noLane is the lane of a dummy node of a long edge, and of a cluster in
// the graph that ranks.
const noLane = -1

// laneLayout holds the state of one call of layLanes.
type laneLayout struct {
	in       *LayoutInput
	sideways bool
	ids      map[string]int
	cids     map[string]int
	cparent  []int
	cdepth   []int
	// lanes holds the cluster of each lane, in order, or -1 for the lane
	// of the nodes outside every cluster. lane holds the index in lanes of
	// the lane of each node.
	lanes []int
	lane  []int
	// w and h are the sizes of the nodes in the frame where the ranks run
	// down, so w is across the ranks.
	w, h []float64
	// ends holds the two ends of each edge: the index of a node, or -1-c
	// for a cluster c.
	ends [][2]int
	rank []int
	// rg is the graph that ranks, with its edges that close a cycle turned
	// around.
	rg *rgraph
	// layers holds the nodes of each rank, in order across the ranks.
	layers [][]int
	x, y   []float64
	pad    float64
}

// layLanes lays out a graph in lanes. It reads no rank group, no minlen and
// no weight of an edge, and no direction of a cluster, as Mermaid does not.
func layLanes(in LayoutInput) (*Layout, error) {
	l := &laneLayout{in: &in, ids: map[string]int{}, cids: map[string]int{}}
	l.sideways = in.Direction == LR || in.Direction == RL
	l.pad = in.ClusterPadding
	if l.pad <= 0 {
		l.pad = 12
	}
	for i, n := range in.Nodes {
		if _, ok := l.ids[n.ID]; ok {
			return nil, fmt.Errorf("the node %q twice: %w", n.ID, ErrGraph)
		}
		l.ids[n.ID] = i
	}
	for i, c := range in.Clusters {
		l.cids[c.ID] = i
	}
	l.cparent = make([]int, len(in.Clusters))
	for i, c := range in.Clusters {
		l.cparent[i] = -1
		if p, ok := l.cids[c.Parent]; ok && c.Parent != "" && p != i {
			l.cparent[i] = p
		}
	}
	l.cdepth = make([]int, len(in.Clusters))
	for i := range in.Clusters {
		for p := l.cparent[i]; p >= 0 && l.cdepth[i] < len(in.Clusters); p = l.cparent[p] {
			l.cdepth[i]++
		}
	}
	for _, e := range in.Edges {
		var end [2]int
		for k, id := range []string{e.Source, e.Target} {
			if i, ok := l.ids[id]; ok {
				end[k] = i
			} else if c, ok := l.cids[id]; ok {
				end[k] = -1 - c
			} else {
				return nil, fmt.Errorf("an edge to %q, which is not a node: %w", id, ErrGraph)
			}
		}
		l.ends = append(l.ends, end)
	}
	l.findLanes()
	for _, n := range in.Nodes {
		w, h := n.Width, n.Height
		if l.sideways {
			w, h = h, w
		}
		l.w, l.h = append(l.w, w), append(l.h, h)
	}
	l.rankNodes()
	l.orderNodes()
	g := l.place()
	out := l.result(g)
	placeLabels(l.in, out)
	l.normalize(out)
	return out, nil
}

// top returns the cluster at the top that holds a cluster, or -1 for a
// cluster in a cycle of parents.
func (l *laneLayout) top(c int) int {
	for range len(l.cparent) {
		if c < 0 || l.cparent[c] < 0 {
			return c
		}
		c = l.cparent[c]
	}
	return -1
}

// findLanes sets the lanes and the lane of each node. A lane is a cluster at
// the top, in the order of the input, and the nodes outside every cluster,
// or in a cycle of clusters, make a lane of their own before them. A lane that holds no node takes no
// place.
func (l *laneLayout) findLanes() {
	in := l.in
	held := map[int]bool{}
	loose := false
	tops := make([]int, len(in.Nodes))
	for i, n := range in.Nodes {
		tops[i] = -1
		if c, ok := l.cids[n.Cluster]; ok && n.Cluster != "" {
			tops[i] = l.top(c)
		}
		if tops[i] < 0 {
			loose = true
			continue
		}
		held[tops[i]] = true
	}
	if loose {
		l.lanes = append(l.lanes, -1)
	}
	for c := range in.Clusters {
		if l.cparent[c] < 0 && held[c] {
			l.lanes = append(l.lanes, c)
		}
	}
	l.lane = make([]int, len(in.Nodes))
	for i, t := range tops {
		l.lane[i] = slices.Index(l.lanes, t)
	}
}

// rgraph is the graph that ranks: the clusters, then the nodes, as
// Mermaid holds them, and the edges between two nodes that are not loops.
type rgraph struct {
	names []string
	// node is the index of each item among the nodes, or -1 for a cluster.
	node []int
	// lane is the lane of each item, or noLane.
	lane  []int
	edges []redgeID
}

// redgeID is an edge of the graph that ranks, with its id, which breaks a
// tie between two edges.
type redgeID struct {
	u, v int
	id   string
}

// rankGraph builds the graph that ranks. Mermaid holds the clusters in the
// order that they end in the source, which is the order of the input with
// each cluster after the clusters inside it, and the nodes after them.
func (l *laneLayout) rankGraph() *rgraph {
	g := &rgraph{}
	var post func(c int)
	post = func(c int) {
		for d := range l.in.Clusters {
			if l.cparent[d] == c {
				post(d)
			}
		}
		g.names = append(g.names, l.in.Clusters[c].ID)
		g.node = append(g.node, -1)
		g.lane = append(g.lane, noLane)
	}
	for c := range l.in.Clusters {
		if l.cparent[c] < 0 {
			post(c)
		}
	}
	first := len(g.names)
	for i, n := range l.in.Nodes {
		g.names = append(g.names, n.ID)
		g.node = append(g.node, i)
		g.lane = append(g.lane, l.lane[i])
	}
	for i, e := range l.in.Edges {
		a, b := l.ends[i][0], l.ends[i][1]
		if a < 0 || b < 0 || a == b || e.NoConstraint {
			continue
		}
		id := "L_" + e.Source + "_" + e.Target + "_" + strconv.Itoa(i)
		g.edges = append(g.edges, redgeID{first + a, first + b, id})
	}
	return g
}

// rankNodes gives each node its rank, as the swimlane layout of Mermaid
// does: the edges that close a cycle turn around, and then each node in the
// order of a topological sort takes the first rank after its sources. In
// the default mode, an edge from another lane does not move its target to
// the next rank, and a node takes a rank after the last node of its lane.
// With LaneEdges, each edge moves its target to the next rank, and the
// ranks then move toward the mean of the ranks of their neighbors.
func (l *laneLayout) rankNodes() {
	g := l.rankGraph()
	g.edges = acyclicLanes(g)
	l.rg = g
	var rank []int
	if l.in.LaneEdges {
		rank = gravityRanks(g)
	} else {
		rank = compactRanks(g, l.in.Direction == LR)
	}
	l.rank = make([]int, len(l.in.Nodes))
	top := 0
	for i, n := range g.node {
		if n >= 0 {
			l.rank[n] = max(0, rank[i])
			top = max(top, l.rank[n])
		}
	}
	// layers holds the nodes of each rank in the order of the topological
	// sort, as Mermaid builds them.
	l.layers = make([][]int, top+1)
	for _, i := range topoKahn(g) {
		if n := g.node[i]; n >= 0 {
			l.layers[l.rank[n]] = append(l.layers[l.rank[n]], n)
		}
	}
}

// acyclicLanes turns around each edge that closes a cycle: a depth-first
// search from the items with no edge in, then from the others, follows the
// edges out of an item in the order of the names of their targets.
func acyclicLanes(g *rgraph) []redgeID {
	n := len(g.names)
	adj := make([][]int, n)
	indeg := make([]int, n)
	for i, e := range g.edges {
		adj[e.u] = append(adj[e.u], i)
		indeg[e.v]++
	}
	for _, a := range adj {
		slices.SortStableFunc(a, func(i, j int) int {
			ei, ej := g.edges[i], g.edges[j]
			if ei.v == ej.v {
				return collate(ei.id, ej.id)
			}
			if c := collate(g.names[ei.v], g.names[ej.v]); c != 0 {
				return c
			}
			return cmp.Compare(ei.v, ej.v)
		})
	}
	state := make([]int8, n)
	out := slices.Clone(g.edges)
	var visit func(u int)
	visit = func(u int) {
		state[u] = 1
		for _, i := range adj[u] {
			switch v := g.edges[i].v; state[v] {
			case 0:
				visit(v)
			case 1:
				out[i].u, out[i].v = v, u
			}
		}
		state[u] = 2
	}
	var seeds []int
	for _, zero := range []bool{true, false} {
		for v := range n {
			if (indeg[v] == 0) == zero {
				seeds = append(seeds, v)
			}
		}
	}
	for _, v := range seeds {
		if state[v] == 0 {
			visit(v)
		}
	}
	return out
}

// successors returns the targets of the edges out of each item, sorted by
// their names.
func successors(g *rgraph) [][]int {
	succ := make([][]int, len(g.names))
	for _, e := range g.edges {
		succ[e.u] = append(succ[e.u], e.v)
	}
	for _, s := range succ {
		slices.SortStableFunc(s, func(a, b int) int { return collate(g.names[a], g.names[b]) })
	}
	return succ
}

// sources returns the items with no edge in, sorted by their names, except
// that the nodes with no edge at all keep the order of the input among
// themselves.
func sources(g *rgraph, indeg []int) []int {
	linked := make([]bool, len(g.names))
	for _, e := range g.edges {
		linked[e.u], linked[e.v] = true, true
	}
	alone := func(i int) bool { return !linked[i] && g.node[i] >= 0 }
	var sorted, lone []int
	for i := range g.names {
		if indeg[i] == 0 {
			sorted = append(sorted, i)
			if alone(i) {
				lone = append(lone, i)
			}
		}
	}
	slices.SortStableFunc(sorted, func(a, b int) int { return collate(g.names[a], g.names[b]) })
	next := 0
	for k, i := range sorted {
		if alone(i) {
			sorted[k] = lone[next]
			next++
		}
	}
	return sorted
}

// indegrees returns the number of edges into each item.
func indegrees(g *rgraph) []int {
	indeg := make([]int, len(g.names))
	for _, e := range g.edges {
		indeg[e.v]++
	}
	return indeg
}

// topoKahn returns the items in a topological order: a queue of the
// items with no edge in, where each item that its last edge in frees goes
// before the first item of the queue whose name is not before its own, by
// the order of the code units of JavaScript.
func topoKahn(g *rgraph) []int {
	indeg := indegrees(g)
	queue := sources(g, indeg)
	succ := successors(g)
	var order []int
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		order = append(order, u)
		for _, v := range succ[u] {
			indeg[v]--
			if indeg[v] == 0 {
				i := 0
				for i < len(queue) && g.names[queue[i]] < g.names[v] {
					i++
				}
				queue = slices.Insert(queue, i, v)
			}
		}
	}
	return order
}

// topoGenerations returns the items in a topological order by generations:
// the items with no edge in, then the items that they free, sorted by their
// names, and so on.
func topoGenerations(g *rgraph) []int {
	indeg := indegrees(g)
	front := sources(g, indeg)
	succ := successors(g)
	var order []int
	for len(front) > 0 {
		var next []int
		for _, u := range front {
			order = append(order, u)
			for _, v := range succ[u] {
				indeg[v]--
				if indeg[v] == 0 {
					next = append(next, v)
				}
			}
		}
		slices.SortStableFunc(next, func(a, b int) int { return collate(g.names[a], g.names[b]) })
		front = next
	}
	return order
}

// incoming returns the sources of the edges into each item, one for each
// edge.
func incoming(g *rgraph) [][]int {
	in := make([][]int, len(g.names))
	for _, e := range g.edges {
		in[e.v] = append(in[e.v], e.u)
	}
	return in
}

// compactRanks ranks the items as the default mode of Mermaid does: an item
// takes the greatest rank of its sources, plus 1 for a source in its own
// lane, and at least the rank after the last item of its lane. A graph from
// left to right takes the items by generations.
func compactRanks(g *rgraph, lr bool) []int {
	order := topoKahn(g)
	if lr {
		order = topoGenerations(g)
	}
	in := incoming(g)
	rank := make([]int, len(g.names))
	next := map[int]int{}
	for _, v := range order {
		if g.node[v] < 0 {
			continue
		}
		base := 0
		for _, u := range in[v] {
			w := 0
			if g.lane[u] == g.lane[v] {
				w = 1
			}
			base = max(base, rank[u]+w)
		}
		r := max(base, next[g.lane[v]])
		rank[v] = r
		next[g.lane[v]] = r + 1
	}
	return rank
}

// gravityRanks ranks the items as Mermaid does when an edge between two
// lanes counts: the longest path, where a node with one source in another
// lane shares its rank, then 8 passes that move each item toward the mean
// of the ranks around it, and then each edge spans one rank at least.
func gravityRanks(g *rgraph) []int {
	order := topoKahn(g)
	in := incoming(g)
	out := make([][]int, len(g.names))
	for _, e := range g.edges {
		out[e.u] = append(out[e.u], e.v)
	}
	rank := make([]int, len(g.names))
	for _, v := range order {
		switch len(in[v]) {
		case 0:
			rank[v] = 0
		case 1:
			u := in[v][0]
			rank[v] = rank[u]
			if g.lane[u] == g.lane[v] {
				rank[v]++
			}
		default:
			r := math.MinInt
			for _, u := range in[v] {
				r = max(r, rank[u]+1)
			}
			rank[v] = r
		}
	}
	bounds := func(v int) (int, int, bool) {
		lb := 0
		for _, u := range in[v] {
			lb = max(lb, rank[u]+1)
		}
		ub := math.MaxInt
		for _, w := range out[v] {
			ub = min(ub, rank[w]-1)
		}
		return lb, ub, len(out[v]) > 0
	}
	relax := func(vs []int) bool {
		changed := false
		for _, v := range vs {
			if len(in[v]) == 0 && len(out[v]) == 0 {
				continue
			}
			pred, succ := float64(rank[v]), float64(rank[v])
			if len(in[v]) > 0 {
				s := 0.0
				for _, u := range in[v] {
					s += float64(rank[u] + 1)
				}
				pred = s / float64(len(in[v]))
			}
			if len(out[v]) > 0 {
				s := 0.0
				for _, w := range out[v] {
					s += float64(rank[w] - 1)
				}
				succ = s / float64(len(out[v]))
			}
			want := int(math.Floor((pred+succ)/2 + 0.5))
			lb, ub, ok := bounds(v)
			if !ok {
				ub = max(lb, want)
			}
			if r := min(max(want, lb), ub); r != rank[v] {
				rank[v], changed = r, true
			}
		}
		return changed
	}
	rev := slices.Clone(order)
	slices.Reverse(rev)
	for range 8 {
		fwd := relax(order)
		back := relax(rev)
		if !fwd && !back {
			break
		}
	}
	for _, v := range order {
		if lb, _, _ := bounds(v); rank[v] < lb {
			rank[v] = lb
		}
	}
	for _, v := range rev {
		if _, ub, ok := bounds(v); ok && rank[v] > ub {
			rank[v] = ub
		}
	}
	return rank
}

// orderNodes orders the nodes of each rank: by lane, and in a lane that
// holds more than one node of a rank, by the median of the places of their
// neighbors, with 3 sweeps down and up and the swap of neighbors in a lane,
// as Mermaid does. A long edge has a dummy node in each rank that it
// crosses, which belongs to no lane.
func (l *laneLayout) orderNodes() {
	crowded := false
	for _, ly := range l.layers {
		seen := map[int]bool{}
		for _, v := range ly {
			if seen[l.lane[v]] {
				crowded = true
			}
			seen[l.lane[v]] = true
		}
	}
	if !crowded {
		for r := range l.layers {
			slices.SortStableFunc(l.layers[r], func(a, b int) int { return cmp.Compare(l.lane[a], l.lane[b]) })
		}
		return
	}
	// The items are the nodes, then the dummies, each with its lane and
	// its name.
	n := len(l.in.Nodes)
	laneOf := slices.Clone(l.lane)
	names := make([]string, 0, n)
	for _, nd := range l.in.Nodes {
		names = append(names, nd.ID)
	}
	layers := make([][]int, len(l.layers))
	for r := range l.layers {
		layers[r] = slices.Clone(l.layers[r])
	}
	type pedge struct {
		u, v int
		id   string
	}
	var edges []pedge
	g := l.rg
	sorted := slices.Clone(g.edges)
	slices.SortStableFunc(sorted, func(a, b redgeID) int { return collate(a.id, b.id) })
	for _, e := range sorted {
		u, v := g.node[e.u], g.node[e.v]
		ru, rv := l.rank[u], l.rank[v]
		if rv-ru <= 1 {
			edges = append(edges, pedge{u, v, e.id})
			continue
		}
		prev := u
		for r := ru + 1; r < rv; r++ {
			d := len(laneOf)
			laneOf = append(laneOf, noLane)
			names = append(names, fmt.Sprintf("placeholder-%d", d-n))
			layers[r] = append(layers[r], d)
			edges = append(edges, pedge{prev, d, e.id})
			prev = d
		}
		edges = append(edges, pedge{prev, v, e.id})
	}
	index := func(ly []int) map[int]int {
		m := make(map[int]int, len(ly))
		for i, v := range ly {
			m[v] = i
		}
		return m
	}
	// cross counts the crossings of the edges between two ranks.
	cross := func(upper, lower []int) int {
		ui, li := index(upper), index(lower)
		type pair struct{ u, v int }
		var ps []pair
		for _, e := range edges {
			a, ok1 := ui[e.u]
			b, ok2 := li[e.v]
			if ok1 && ok2 {
				ps = append(ps, pair{a, b})
			}
		}
		slices.SortFunc(ps, func(x, y pair) int {
			if c := cmp.Compare(x.u, y.u); c != 0 {
				return c
			}
			return cmp.Compare(x.v, y.v)
		})
		vs := make([]int, len(ps))
		for i, p := range ps {
			vs[i] = p.v
		}
		return inversions(vs)
	}
	median := func(ps []float64) float64 {
		if len(ps) == 0 {
			return math.Inf(1)
		}
		a := slices.Clone(ps)
		slices.Sort(a)
		if len(a)%2 == 1 {
			return a[len(a)/2]
		}
		return (a[len(a)/2-1] + a[len(a)/2]) / 2
	}
	mean := func(ps []float64) float64 {
		if len(ps) == 0 {
			return math.Inf(1)
		}
		s := 0.0
		for _, p := range ps {
			s += p
		}
		return s / float64(len(ps))
	}
	reorder := func(fixed, target []int, down bool) []int {
		fi, ci := index(fixed), index(target)
		nb := map[int][]float64{}
		for _, e := range edges {
			if down {
				if i, ok := fi[e.u]; ok {
					if _, ok := ci[e.v]; ok {
						nb[e.v] = append(nb[e.v], float64(i))
					}
				}
			} else if i, ok := fi[e.v]; ok {
				if _, ok := ci[e.u]; ok {
					nb[e.u] = append(nb[e.u], float64(i))
				}
			}
		}
		sortBy := func(vs []int) []int {
			out := slices.Clone(vs)
			slices.SortStableFunc(out, func(a, b int) int {
				sa, sb := median(nb[a]), median(nb[b])
				switch {
				case sa == sb:
					if c := cmp.Compare(ci[a], ci[b]); c != 0 {
						return c
					}
					return collate(names[a], names[b])
				case math.IsInf(sa, 1):
					return 1
				case math.IsInf(sb, 1):
					return -1
				}
				return cmp.Compare(sa, sb)
			})
			return out
		}
		byLane := map[int][]int{}
		for _, v := range target {
			byLane[laneOf[v]] = append(byLane[laneOf[v]], v)
		}
		var out []int
		for li := range l.lanes {
			out = append(out, sortBy(byLane[li])...)
		}
		for _, v := range sortBy(byLane[noLane]) {
			bc := mean(nb[v])
			at := len(out)
			if !math.IsInf(bc, 1) {
				for i, w := range out {
					if bc < mean(nb[w]) {
						at = i
						break
					}
				}
			}
			out = slices.Insert(out, at, v)
		}
		return out
	}
	transpose := func(upper, cur, next []int) []int {
		best := slices.Clone(cur)
		score := func(o []int) int {
			s := cross(upper, o)
			if next != nil {
				s += cross(o, next)
			}
			return s
		}
		bestScore := score(best)
		for improved := true; improved; {
			improved = false
			for i := 0; i+1 < len(best); i++ {
				if laneOf[best[i]] != laneOf[best[i+1]] {
					continue
				}
				best[i], best[i+1] = best[i+1], best[i]
				if s := score(best); s < bestScore {
					bestScore, improved = s, true
				} else {
					best[i], best[i+1] = best[i+1], best[i]
				}
			}
		}
		return best
	}
	at := func(i int) []int {
		if i < 0 || i >= len(layers) {
			return nil
		}
		return layers[i]
	}
	for range 3 {
		for i := 1; i < len(layers); i++ {
			layers[i] = reorder(layers[i-1], layers[i], true)
			layers[i] = transpose(layers[i-1], layers[i], at(i+1))
		}
		for i := len(layers) - 2; i >= 0; i-- {
			layers[i] = reorder(layers[i+1], layers[i], false)
			layers[i] = transpose(layers[i+1], layers[i], at(i-1))
		}
	}
	for r, ly := range layers {
		l.layers[r] = l.layers[r][:0]
		for _, v := range ly {
			if v < n {
				l.layers[r] = append(l.layers[r], v)
			}
		}
		slices.SortStableFunc(l.layers[r], func(a, b int) int { return cmp.Compare(l.lane[a], l.lane[b]) })
	}
}

// punctuation is the order of the punctuation and the symbols of ASCII in
// the root collation of Unicode, which sorts them before the digits and
// the letters.
const punctuation = "_-,;:!?.'\"()[]{}@*/\\&#%`^+<=>|~$"

// collate compares two names as localeCompare of JavaScript does in the
// root locale, near enough for the ids of a diagram: white space, then
// punctuation, then digits, then letters without regard to case, and then
// a lower case letter before its upper case.
func collate(a, b string) int {
	key := func(r rune) (int, int) {
		switch {
		case unicode.IsSpace(r):
			return 0, int(r)
		case unicode.IsDigit(r):
			return 3, int(r)
		case unicode.IsLetter(r):
			return 4, int(unicode.ToLower(r))
		}
		if i := strings.IndexRune(punctuation, r); i >= 0 {
			return 1, i
		}
		return 2, int(r)
	}
	ra, rb := []rune(a), []rune(b)
	for i := 0; i < len(ra) && i < len(rb); i++ {
		ca, va := key(ra[i])
		cb, vb := key(rb[i])
		if c := cmp.Compare(ca, cb); c != 0 {
			return c
		}
		if c := cmp.Compare(va, vb); c != 0 {
			return c
		}
	}
	if c := cmp.Compare(len(ra), len(rb)); c != 0 {
		return c
	}
	for i := range ra {
		if ra[i] != rb[i] {
			// A lower case letter comes before its upper case.
			if unicode.IsLower(ra[i]) {
				return -1
			}
			return 1
		}
	}
	return 0
}
