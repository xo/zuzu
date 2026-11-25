// SPDX-License-Identifier: EPL-2.0

package adapter

// This file holds the passes of render.ts of Mermaid 12.3.0 that change
// the graph of ELK after it is built: the edges that turn back into a
// cluster (subgraphFeedbackEdges.ts), the order of the edges by nesting
// (nestedEdgeOrder.ts), and the entry of a cycle that keepEntryNodeOnTop
// pins to the first layer (findCyclicEntryNodes).

// tree holds the cluster of each node, by the index of the node, or -1 for
// the root.
type tree []int

// newTree returns the tree of a graph.
func newTree(g *Graph, byID map[string]int) tree {
	t := make(tree, len(g.Nodes))
	for i := range g.Nodes {
		t[i] = -1
		if p, ok := byID[g.Nodes[i].Parent]; ok && g.Nodes[i].Parent != "" {
			t[i] = p
		}
	}
	return t
}

// ancestor returns the first cluster that holds both nodes or is one of
// them, or -1 for the root, as findCommonAncestor does. An edge from a node
// to itself has the cluster that holds the node.
func (t tree) ancestor(a, b int) int {
	if a == b {
		return t[a]
	}
	seen := map[int]bool{}
	for id, depth := a, 0; id >= 0 && depth <= len(t); id, depth = t[id], depth+1 {
		seen[id] = true
		if id == b {
			return id
		}
	}
	for id, depth := b, 0; id >= 0 && depth <= len(t); id, depth = t[id], depth+1 {
		if seen[id] {
			return id
		}
	}
	return -1
}

// childOn returns the node on the way from a node up to an ancestor that
// the ancestor holds directly, or the node itself.
func (t tree) childOn(id, ancestor int) int {
	for depth := 0; t[id] != ancestor && t[id] >= 0 && depth <= len(t); depth++ {
		id = t[id]
	}
	return id
}

// inside reports whether a node lies inside a cluster, at any depth.
func (t tree) inside(id, cluster int) bool {
	for p, depth := t[id], 0; p >= 0 && depth <= len(t); p, depth = t[p], depth+1 {
		if p == cluster {
			return true
		}
	}
	return false
}

// ends are the indexes of the two ends of an edge.
type ends struct {
	from, to int
}

// feedbackEdges returns, for each edge, whether ELK lays it out reversed,
// as findSubgraphFeedbackEdges of Mermaid does. ELK lays out a cluster as
// one node of the graph that holds it. A node outside a cluster that takes
// an edge from it and sends one back into it then closes a cycle, and ELK
// routes one edge of the cycle around the cluster. Each edge is looked at
// in the graph of its lowest common cluster, with each cluster below it as
// one node. A search in depth from the sources, in the order of the edges,
// marks an edge to a node on its stack. Of those, an edge with an end
// inside such a cluster goes to ELK reversed, so that ELK sees no cycle.
// A cycle between plain nodes stays for ELK to break.
func feedbackEdges(es []ends, t tree) []bool {
	flags := make([]bool, len(es))
	type levelEdge struct {
		index, from, to int
		collapsed       bool
	}
	levels := map[int][]levelEdge{}
	var order []int
	for i, e := range es {
		if e.from == e.to {
			continue
		}
		a := t.ancestor(e.from, e.to)
		if a == e.from || a == e.to {
			// An edge between a cluster and a node inside it crosses no
			// border of the graph that holds the cluster.
			continue
		}
		from, to := t.childOn(e.from, a), t.childOn(e.to, a)
		if _, ok := levels[a]; !ok {
			order = append(order, a)
		}
		levels[a] = append(levels[a], levelEdge{i, from, to, from != e.from || to != e.to})
	}
	for _, a := range order {
		level := levels[a]
		successors := map[int][]levelEdge{}
		var nodes []int
		inDegree := map[int]int{}
		for _, e := range level {
			for _, n := range []int{e.from, e.to} {
				if _, ok := successors[n]; !ok {
					successors[n] = nil
					nodes = append(nodes, n)
					inDegree[n] = 0
				}
			}
			successors[e.from] = append(successors[e.from], e)
			inDegree[e.to]++
		}
		const onStack, done = 1, 2
		state := map[int]int{}
		var roots []int
		for _, n := range nodes {
			if inDegree[n] == 0 {
				roots = append(roots, n)
			}
		}
		roots = append(roots, nodes...)
		type frame struct{ node, next int }
		for _, root := range roots {
			if _, ok := state[root]; ok {
				continue
			}
			state[root] = onStack
			stack := []frame{{root, 0}}
			for len(stack) > 0 {
				f := &stack[len(stack)-1]
				out := successors[f.node]
				if f.next >= len(out) {
					state[f.node] = done
					stack = stack[:len(stack)-1]
					continue
				}
				e := out[f.next]
				f.next++
				switch s, ok := state[e.to]; {
				case ok && s == onStack:
					flags[e.index] = e.collapsed
				case !ok:
					state[e.to] = onStack
					stack = append(stack, frame{e.to, 0})
				}
			}
		}
	}
	return flags
}

// nestedOrder returns the order in which ELK takes the edges, as
// orderNestedEdgeEnds of Mermaid does. With the order of the model, ELK
// lines up the edges that share an end in their order, even where that
// adds a crossing. So an edge from a node inside a cluster moves before
// the first edge before it from the border of that cluster to the same
// target, and an edge to a node inside a cluster before the first edge
// before it from the same source to the border of that cluster. The other
// edges keep their order. The ends are those that ELK takes, after the
// reverse of the feedback edges.
func nestedOrder(es []ends, t tree) []int {
	order := make([]int, len(es))
	for i := range order {
		order[i] = i
	}
	for i, e := range es {
		from := -1
		for p, j := range order {
			if j == i {
				from = p
				break
			}
		}
		to := -1
		for p, j := range order[:from] {
			earlier := es[j]
			if earlier.to == e.to && t.inside(e.from, earlier.from) || earlier.from == e.from && t.inside(e.to, earlier.to) {
				to = p
				break
			}
		}
		if to < 0 {
			continue
		}
		copy(order[to+1:from+1], order[to:from])
		order[to] = i
	}
	return order
}

// cyclicEntries returns the nodes that keepEntryNodeOnTop pins to the
// first layer, as findCyclicEntryNodes of Mermaid does. It looks at the
// nodes of each cluster, and of the root, apart, with the edges between
// two of them. A group of nodes that these edges join, with no node that
// no edge enters, holds a cycle. Its entry is the first node, in the order
// of the graph, that no edge enters once each edge that closes a cycle, in
// the order of the edges, is left out.
func cyclicEntries(nodes int, es []ends, t tree) []int {
	var entries []int
	groups := map[int][]int{}
	var keys []int
	for i := range nodes {
		p := t[i]
		if _, ok := groups[p]; !ok {
			keys = append(keys, p)
		}
		groups[p] = append(groups[p], i)
	}
	for _, key := range keys {
		ids := groups[key]
		in := map[int]bool{}
		for _, id := range ids {
			in[id] = true
		}
		inDegree := map[int]int{}
		neighbors := map[int][]int{}
		var internal []ends
		for _, e := range es {
			if e.from == e.to || !in[e.from] || !in[e.to] {
				continue
			}
			inDegree[e.to]++
			neighbors[e.from] = append(neighbors[e.from], e.to)
			neighbors[e.to] = append(neighbors[e.to], e.from)
			internal = append(internal, e)
		}
		component := map[int]int{}
		count := 0
		for _, id := range ids {
			if _, ok := component[id]; ok {
				continue
			}
			component[id] = count
			stack := []int{id}
			for len(stack) > 0 {
				cur := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				for _, next := range neighbors[cur] {
					if _, ok := component[next]; !ok {
						component[next] = count
						stack = append(stack, next)
					}
				}
			}
			count++
		}
		hasSource := make([]bool, count)
		for _, id := range ids {
			if inDegree[id] == 0 {
				hasSource[component[id]] = true
			}
		}
		cyclic := false
		for _, s := range hasSource {
			cyclic = cyclic || !s
		}
		if !cyclic {
			continue
		}
		forward := map[int][]int{}
		residual := map[int]int{}
		reaches := func(from, to int) bool {
			seen := map[int]bool{from: true}
			stack := []int{from}
			for len(stack) > 0 {
				cur := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if cur == to {
					return true
				}
				for _, next := range forward[cur] {
					if !seen[next] {
						seen[next] = true
						stack = append(stack, next)
					}
				}
			}
			return false
		}
		for _, e := range internal {
			if reaches(e.to, e.from) {
				continue
			}
			forward[e.from] = append(forward[e.from], e.to)
			residual[e.to]++
		}
		nominated := make([]bool, count)
		for _, id := range ids {
			c := component[id]
			if !hasSource[c] && !nominated[c] && residual[id] == 0 {
				entries = append(entries, id)
				nominated[c] = true
			}
		}
	}
	return entries
}
