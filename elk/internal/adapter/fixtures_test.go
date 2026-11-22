// SPDX-License-Identifier: EPL-2.0

package adapter

import (
	"fmt"
	"strconv"
	"strings"
)

// fixture is a graph of the tests, with a name.
type fixture struct {
	name string
	g    Graph
}

// box returns a node of the given size.
func box(id string, w, h float64) Node {
	return Node{ID: id, Width: w, Height: h}
}

// in returns a node of the given size inside a cluster.
func in(parent, id string, w, h float64) Node {
	return Node{ID: id, Parent: parent, Width: w, Height: h}
}

// edge returns an edge from one node to another.
func edge(id, from, to string) Edge {
	return Edge{ID: id, From: from, To: to}
}

// title returns the label of a cluster or an edge.
func title(text string, w, h float64) *Label {
	return &Label{Text: text, Width: w, Height: h}
}

// small returns a graph of four nodes and four edges, with a cycle, which
// each algorithm can lay out.
func small(algorithm string) Graph {
	return Graph{
		Algorithm: algorithm,
		Nodes: []Node{
			box("a", 80, 40), box("b", 60, 40), box("c", 100, 50), box("d", 70, 30),
		},
		Edges: []Edge{
			edge("ab", "a", "b"), edge("ac", "a", "c"), edge("bd", "b", "d"), edge("da", "d", "a"),
		},
	}
}

// fixtures returns the graphs of the golden test and of the test of
// determinism. They cover each algorithm, each direction, the clusters, the
// ports, the labels and the settings of Mermaid.
func fixtures() []fixture {
	out := []fixture{{
		name: "chain-tb",
		g: Graph{
			Nodes: []Node{box("A", 90, 42), box("B", 120, 42), box("C", 70, 42), box("D", 70, 42)},
			Edges: []Edge{
				{ID: "L_A_B_0", From: "A", To: "B", Label: title("yes", 26, 24)},
				edge("L_B_C_0", "B", "C"),
				edge("L_B_D_0", "B", "D"),
				edge("L_C_D_0", "C", "D"),
			},
		},
	}, {
		name: "cluster-lr",
		g: Graph{
			Direction: "LR",
			Nodes: []Node{
				box("start", 80, 42),
				{ID: "S", Cluster: true, Label: title("Service", 60, 24), Padding: 8},
				in("S", "x", 70, 42),
				in("S", "y", 70, 42),
				box("end", 80, 42),
			},
			Edges: []Edge{
				edge("e1", "start", "x"),
				edge("e2", "x", "y"),
				{ID: "e3", From: "y", To: "end", Label: title("done", 34, 24)},
				edge("e4", "start", "end"),
			},
		},
	}, {
		name: "cluster-own-direction",
		g: Graph{
			Nodes: []Node{
				box("top", 80, 42),
				{ID: "row", Cluster: true, Label: title("Row", 30, 24), Padding: 8, Direction: "LR"},
				in("row", "r1", 50, 42),
				in("row", "r2", 50, 42),
				in("row", "r3", 50, 42),
				box("bottom", 80, 42),
			},
			Edges: []Edge{
				edge("e1", "top", "row"),
				edge("e2", "r1", "r2"),
				edge("e3", "r2", "r3"),
				edge("e4", "row", "bottom"),
			},
		},
	}, {
		name: "nested-clusters-bt",
		g: Graph{
			Direction: "BT",
			Nodes: []Node{
				{ID: "outer", Cluster: true, Label: title("Outer", 40, 24), Padding: 8},
				{ID: "inner", Parent: "outer", Cluster: true, Label: title("Inner", 40, 24), Padding: 8},
				in("inner", "i1", 60, 40),
				in("outer", "o1", 60, 40),
				box("free", 60, 40),
			},
			Edges: []Edge{edge("e1", "i1", "o1"), edge("e2", "o1", "free"), edge("e3", "free", "i1")},
		},
	}, {
		name: "cycle-legacy-merged-rl",
		g: Graph{
			Direction: "RL",
			Config:    Config{Preset: "legacy", MergeEdges: true},
			Nodes:     []Node{box("a", 50, 40), box("b", 50, 40), box("c", 50, 40), box("d", 50, 40)},
			Edges: []Edge{
				edge("ab", "a", "b"), edge("bc", "b", "c"), edge("ca", "c", "a"),
				edge("ad", "a", "d"), edge("bd", "b", "d"), edge("cd", "c", "d"),
			},
		},
	}, {
		name: "placement-network-simplex",
		g: Graph{
			Config: Config{
				NodePlacementStrategy: "NETWORK_SIMPLEX",
				CycleBreakingStrategy: "GREEDY",
			},
			Nodes: []Node{box("a", 50, 40), box("b", 90, 40), box("c", 50, 40), box("d", 50, 40)},
			Edges: []Edge{edge("ab", "a", "b"), edge("ac", "a", "c"), edge("bd", "b", "d"), edge("cd", "c", "d")},
		},
	}, {
		name: "ports",
		g: Graph{
			Direction: "LR",
			Nodes: []Node{
				{ID: "t", Width: 120, Height: 80, Ports: []Port{
					{ID: "t.in", Side: West, Width: 8, Height: 8},
					{ID: "t.out1", Side: East, Width: 8, Height: 8},
					{ID: "t.out2", Side: East, Width: 8, Height: 8},
				}},
				box("u", 60, 40),
				box("v", 60, 40),
				{ID: "w", Width: 60, Height: 40, SpreadPorts: true},
			},
			Edges: []Edge{
				{ID: "e1", From: "w", To: "t", ToPort: "t.in"},
				{ID: "e2", From: "t", FromPort: "t.out1", To: "u"},
				{ID: "e3", From: "t", FromPort: "t.out2", To: "v"},
				edge("e4", "w", "u"),
			},
		},
	}, {
		name: "wide-titles",
		g: Graph{
			Direction: "LR",
			Nodes: []Node{
				{ID: "c", Cluster: true, Label: title("A long title", 140, 24), Padding: 8},
				in("c", "a", 40, 30),
				{ID: "d", Cluster: true, Label: title("Another long title", 180, 24), Padding: 8, Direction: "TB"},
				in("d", "b", 40, 30),
				in("d", "e", 40, 30),
				{ID: "f", Cluster: true, Label: title("", 0, 0), Padding: 8},
				in("f", "g", 40, 30),
				{ID: "n", Cluster: true, Label: title("A note that the frame does not draw", 220, 24), Padding: 8, NoTitle: true},
				in("n", "h", 40, 30),
			},
			Edges: []Edge{edge("ab", "a", "b"), edge("be", "b", "e"), edge("ag", "a", "g"), edge("gh", "g", "h")},
		},
	}, {
		name: "end-labels",
		g: Graph{
			Nodes: []Node{box("CUSTOMER", 110, 60), box("ORDER", 90, 60)},
			Edges: []Edge{{
				ID: "rel", From: "CUSTOMER", To: "ORDER",
				Label: title("places", 44, 24),
				Tail:  []Label{{Text: "1", Width: 20, Height: 24}},
				Head:  []Label{{Text: "0..*", Width: 36, Height: 24}},
			}},
		},
	}}
	for _, f := range out {
		if f.name == "wide-titles" {
			f.name, f.g.Direction = "wide-titles-tb", "TB"
			out = append(out, f)
			break
		}
	}
	for _, a := range Algorithms {
		out = append(out, fixture{name: "small-" + strings.TrimPrefix(a, "elk."), g: small(a)})
	}
	for _, a := range []string{Box, RectPacking, Radial, Stress} {
		out = append(out, fixture{name: "cluster-algorithm-" + strings.TrimPrefix(a, "elk."), g: Graph{
			Nodes: []Node{
				box("head", 80, 40),
				{ID: "group", Cluster: true, Label: title("Group", 40, 24), Padding: 8, Algorithm: a},
				in("group", "g1", 50, 30),
				in("group", "g2", 70, 30),
				in("group", "g3", 40, 30),
				in("group", "g4", 60, 30),
			},
			Edges: []Edge{edge("e1", "head", "group"), edge("e2", "g1", "g2"), edge("e3", "g1", "g3")},
		}})
	}
	return out
}

// format writes a layout as text, one line for each place, with each number
// in full, so that a golden file shows each change.
func format(name string, r *Result, g *Graph) string {
	var sb strings.Builder
	n := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
	rect := func(r Rect) string { return n(r.X) + " " + n(r.Y) + " " + n(r.Width) + " " + n(r.Height) }
	opt := func(r *Rect) string {
		if r == nil {
			return "none"
		}
		return rect(*r)
	}
	fmt.Fprintf(&sb, "graph %s %s %s\n", name, n(r.Width), n(r.Height))
	for i, p := range r.Nodes {
		fmt.Fprintf(&sb, "node %s %s", g.Nodes[i].ID, rect(p.Rect))
		if g.Nodes[i].Cluster {
			fmt.Fprintf(&sb, " label %s", opt(p.Label))
		}
		sb.WriteString("\n")
		for j, pr := range p.Ports {
			fmt.Fprintf(&sb, "port %s %s\n", g.Nodes[i].Ports[j].ID, rect(pr))
		}
	}
	for i, p := range r.Edges {
		fmt.Fprintf(&sb, "edge %s", edgeID(g, i))
		for _, pt := range p.Points {
			fmt.Fprintf(&sb, " %s,%s", n(pt.X), n(pt.Y))
		}
		sb.WriteString("\n")
		if g.Edges[i].Label != nil {
			fmt.Fprintf(&sb, "label %s %s\n", edgeID(g, i), opt(p.Label))
		}
		for _, t := range p.Tail {
			fmt.Fprintf(&sb, "tail %s %s\n", edgeID(g, i), opt(t))
		}
		for _, h := range p.Head {
			fmt.Fprintf(&sb, "head %s %s\n", edgeID(g, i), opt(h))
		}
	}
	for _, issue := range r.Issues {
		fmt.Fprintf(&sb, "issue %s\n", issue)
	}
	return sb.String()
}
