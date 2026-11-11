package graph

import (
	"math"
	"slices"
)

// redge is an edge of the graph of the ranks: from u to v, at least minlen
// ranks long, of a weight.
type redge struct {
	u, v   int
	minlen int
	weight float64
}

// acyclic returns the edges with each edge that closes a cycle turned
// around, by the greedy heuristic of Eades, Lin and Smyth, 1993: it peels
// the sinks to the end of an order and the sources to its start, and else
// takes the node whose weight out most exceeds its weight in. An edge that
// goes back in the order turns around. The ties go to the node that comes
// first, so that the result is the same each time. reversed says which.
func acyclic(n int, edges []redge) ([]redge, []bool) {
	out := make([]float64, n)
	in := make([]float64, n)
	adjOut := make([][]int, n)
	adjIn := make([][]int, n)
	for i, e := range edges {
		if e.u == e.v {
			continue
		}
		out[e.u] += e.weight
		in[e.v] += e.weight
		adjOut[e.u] = append(adjOut[e.u], i)
		adjIn[e.v] = append(adjIn[e.v], i)
	}
	removed := make([]bool, n)
	var head, tail []int
	remove := func(x int) {
		removed[x] = true
		for _, i := range adjOut[x] {
			if !removed[edges[i].v] {
				in[edges[i].v] -= edges[i].weight
			}
		}
		for _, i := range adjIn[x] {
			if !removed[edges[i].u] {
				out[edges[i].u] -= edges[i].weight
			}
		}
	}
	left := n
	for left > 0 {
		progress := true
		for progress {
			progress = false
			for x := range n {
				if !removed[x] && out[x] <= 1e-9 {
					tail = append(tail, x)
					remove(x)
					left--
					progress = true
				}
			}
			for x := range n {
				if !removed[x] && in[x] <= 1e-9 {
					head = append(head, x)
					remove(x)
					left--
					progress = true
				}
			}
		}
		if left == 0 {
			break
		}
		best, bd := -1, math.Inf(-1)
		for x := range n {
			if !removed[x] && out[x]-in[x] > bd {
				best, bd = x, out[x]-in[x]
			}
		}
		head = append(head, best)
		remove(best)
		left--
	}
	slices.Reverse(tail)
	order := slices.Concat(head, tail)
	pos := make([]int, n)
	for i, x := range order {
		pos[x] = i
	}
	res := make([]redge, len(edges))
	rev := make([]bool, len(edges))
	for i, e := range edges {
		res[i] = e
		if e.u != e.v && pos[e.u] > pos[e.v] {
			res[i].u, res[i].v = e.v, e.u
			rev[i] = true
		}
	}
	return res, rev
}

// rankNodes gives each node a rank by network simplex, as Gansner,
// Koutsofios, North and Vo, 1993, describe it: each edge spans at least its
// minlen, and the sum of the spans times the weights is the least. The
// edges must hold no cycle. Each part of the graph that no edge joins ranks
// on its own, from 0.
func rankNodes(n int, edges []redge) []int {
	// Join the edges between the same two nodes.
	type pair struct{ u, v int }
	merged := map[pair]int{}
	var es []redge
	for _, e := range edges {
		if e.u == e.v {
			continue
		}
		k := pair{e.u, e.v}
		if i, ok := merged[k]; ok {
			es[i].weight += e.weight
			es[i].minlen = max(es[i].minlen, e.minlen)
			continue
		}
		merged[k] = len(es)
		es = append(es, e)
	}
	rank := make([]int, n)
	// The parts of the graph, by a union of the ends of each edge.
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	find := func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	for _, e := range es {
		parent[find(e.u)] = find(e.v)
	}
	parts := map[int][]int{}
	var roots []int
	for x := range n {
		r := find(x)
		if _, ok := parts[r]; !ok {
			roots = append(roots, r)
		}
		parts[r] = append(parts[r], x)
	}
	for _, r := range roots {
		nodes := parts[r]
		in := map[int]bool{}
		for _, x := range nodes {
			in[x] = true
		}
		var pe []redge
		for _, e := range es {
			if in[e.u] {
				pe = append(pe, e)
			}
		}
		simplex(nodes, pe, rank)
	}
	return rank
}

// simplex ranks one connected part of a graph by network simplex.
func simplex(nodes []int, es []redge, rank []int) {
	if len(nodes) == 1 {
		rank[nodes[0]] = 0
		return
	}
	// The first ranks: the longest path from the sources.
	indeg := map[int]int{}
	out := map[int][]int{}
	inE := map[int][]int{}
	for i, e := range es {
		indeg[e.v]++
		out[e.u] = append(out[e.u], i)
		inE[e.v] = append(inE[e.v], i)
	}
	var queue []int
	for _, x := range nodes {
		rank[x] = 0
		if indeg[x] == 0 {
			queue = append(queue, x)
		}
	}
	for len(queue) > 0 {
		x := queue[0]
		queue = queue[1:]
		for _, i := range out[x] {
			e := es[i]
			rank[e.v] = max(rank[e.v], rank[x]+e.minlen)
			indeg[e.v]--
			if indeg[e.v] == 0 {
				queue = append(queue, e.v)
			}
		}
	}
	slack := func(i int) int { return rank[es[i].v] - rank[es[i].u] - es[i].minlen }
	// A tight tree that spans the part.
	inTree := map[int]bool{}
	treeEdge := make([]bool, len(es))
	adj := map[int][]int{}
	for i, e := range es {
		adj[e.u] = append(adj[e.u], i)
		adj[e.v] = append(adj[e.v], i)
	}
	var grow func(x int)
	grow = func(x int) {
		inTree[x] = true
		for _, i := range adj[x] {
			e := es[i]
			y := e.u + e.v - x
			if !inTree[y] && slack(i) == 0 {
				treeEdge[i] = true
				grow(y)
			}
		}
	}
	grow(nodes[0])
	for len(inTree) < len(nodes) {
		// The edge of the least slack with one end in the tree.
		best, bs := -1, math.MaxInt
		for i, e := range es {
			if inTree[e.u] != inTree[e.v] && slack(i) < bs {
				best, bs = i, slack(i)
			}
		}
		if best < 0 {
			break
		}
		d := bs
		if inTree[es[best].v] {
			d = -d
		}
		for x := range inTree {
			rank[x] += d
		}
		clear(inTree)
		clear(treeEdge)
		grow(nodes[0])
	}
	// Exchange tree edges of a negative cut value for the edges of the
	// least slack across, until none is left.
	low := map[int]int{}
	lim := map[int]int{}
	parentEdge := map[int]int{}
	order := func() {
		clear(low)
		clear(lim)
		clear(parentEdge)
		n := 0
		var dfs func(x, from int)
		dfs = func(x, from int) {
			l := n + 1
			for _, i := range adj[x] {
				if !treeEdge[i] || i == from {
					continue
				}
				e := es[i]
				y := e.u + e.v - x
				parentEdge[y] = i
				dfs(y, i)
			}
			n++
			low[x], lim[x] = l, n
		}
		dfs(nodes[0], -1)
	}
	under := func(x, c int) bool { return low[c] <= lim[x] && lim[x] <= lim[c] }
	// cut returns the cut value of the tree edge i, whose lower end is c.
	cut := func(i int) float64 {
		e := es[i]
		c := e.v
		if parentEdge[e.u] == i {
			c = e.u
		}
		tailBelow := c == e.u
		v := 0.0
		for _, f := range es {
			fu, fv := under(f.u, c), under(f.v, c)
			if fu == fv {
				continue
			}
			// The tail component holds e.u.
			fromTail := fu == tailBelow
			if fromTail {
				v += f.weight
			} else {
				v -= f.weight
			}
		}
		return v
	}
	for range 4*len(es) + 100 {
		order()
		leave := -1
		for i := range es {
			if treeEdge[i] && cut(i) < 0 {
				leave = i
				break
			}
		}
		if leave < 0 {
			break
		}
		e := es[leave]
		c := e.v
		if parentEdge[e.u] == leave {
			c = e.u
		}
		tailBelow := c == e.u
		// The entering edge goes from the head component to the tail
		// component, with the least slack.
		enter, bs := -1, math.MaxInt
		for i, f := range es {
			if treeEdge[i] {
				continue
			}
			fu, fv := under(f.u, c), under(f.v, c)
			if fu == fv {
				continue
			}
			uInTail := fu == tailBelow
			if !uInTail && slack(i) < bs {
				enter, bs = i, slack(i)
			}
		}
		if enter < 0 {
			break
		}
		treeEdge[leave] = false
		treeEdge[enter] = true
		// The ranks follow the tree, each tree edge tight.
		seen := map[int]bool{nodes[0]: true}
		stack := []int{nodes[0]}
		for len(stack) > 0 {
			x := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, i := range adj[x] {
				if !treeEdge[i] {
					continue
				}
				f := es[i]
				y := f.u + f.v - x
				if seen[y] {
					continue
				}
				seen[y] = true
				if y == f.v {
					rank[y] = rank[x] + f.minlen
				} else {
					rank[y] = rank[x] - f.minlen
				}
				stack = append(stack, y)
			}
		}
	}
	lo := math.MaxInt
	for _, x := range nodes {
		lo = min(lo, rank[x])
	}
	for _, x := range nodes {
		rank[x] -= lo
	}
}
