// Copied from github.com/d2lang/dagro, file position.go, at commit
// 30476a97eb11 (D39, D44, docs/UPSTREAM.md). Its MIT license is in
// internal/dagre/LICENSE, and its notice in internal/dagre/NOTICE.

package dagro

// position assigns coordinates to the leaf nodes in g. Compound container
// nodes are positioned later from their border nodes, as in Dagre 3.1.1.
func position(g *Graph) {
	g = asNonCompoundGraph(g)

	positionY(g)
	for v, x := range positionX(g) {
		asAttrs(g.Node(v))["x"] = x
	}
}

func positionY(g *Graph) {
	layering := buildLayerMatrix(g)
	rankSep := num(asAttrs(g.Graph()), "ranksep")
	prevY := float64(0)

	for _, layer := range layering {
		maxHeight := float64(0)
		for _, v := range layer {
			node := asAttrs(g.Node(v))
			height := float64(0)
			if has(node, "height") {
				height = num(node, "height")
			}
			if !(maxHeight > height) {
				maxHeight = height
			}
		}
		for _, v := range layer {
			asAttrs(g.Node(v))["y"] = prevY + maxHeight/2
		}
		prevY += maxHeight + rankSep
	}
}
