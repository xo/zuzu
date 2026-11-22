// Copied from github.com/d2lang/dagro, file order_add_subgraph_constraints.go, at commit
// 30476a97eb11 (D39, D44, docs/UPSTREAM.md). Its MIT license is in
// internal/dagre/LICENSE, and its notice in internal/dagre/NOTICE.

package dagro

// addSubgraphConstraints records ordering constraints between non-contiguous
// sibling subgraphs. The traversal and early return mirror Dagre 3.1.1.
func addSubgraphConstraints(g, cg *Graph, vs []string) {
	prev := map[string]string{}
	rootPrev := ""

	for _, v := range vs {
		child, ok := g.Parent(v)
		for ok && child != "" {
			parent, hasParent := g.Parent(child)
			prevChild := ""
			if hasParent && parent != "" {
				prevChild = prev[parent]
				prev[parent] = child
			} else {
				prevChild = rootPrev
				rootPrev = child
			}
			if prevChild != "" && prevChild != child {
				cg.SetEdge(prevChild, child)
				break
			}
			child, ok = parent, hasParent
		}
	}
}
