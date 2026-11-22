// Copied from github.com/d2lang/dagro, file rank.go, at commit
// 30476a97eb11 (D39, D44, docs/UPSTREAM.md). Its MIT license is in
// internal/dagre/LICENSE, and its notice in internal/dagre/NOTICE.

package dagro

// rank selects one of Dagre's three rankers. Unknown and absent values
// intentionally fall back to network simplex.
func rank(g *Graph) {
	ranker := ""
	if graphLabel := asAttrs(g.Graph()); graphLabel != nil {
		ranker = stringValue(graphLabel, "ranker")
	}

	switch ranker {
	case "longest-path":
		longestPath(g)
	case "tight-tree":
		longestPath(g)
		feasibleTree(g)
	case "network-simplex":
		networkSimplex(g)
	default:
		networkSimplex(g)
	}
}
