package dagro

// This file is a file of zuzu, not of dagro. It holds the test of change 4
// of README.md: the crossing sweep of dagre 0.8, which Mermaid 12.3 bundles
// as dagre-d3-es 7.0.14.

import (
	"strconv"
	"testing"
)

// TestOrderOfDagre08 lays out four graphs and compares the center of each
// node with the center that dagre-d3-es 7.0.14 gives for the same graph.
// The sweep of dagre 3.1.1 places the nodes of each graph in another order.
func TestOrderOfDagre08(t *testing.T) {
	t.Parallel()
	type place struct {
		v    string
		x, y float64
	}
	tests := []struct {
		name  string
		edges [][2]string
		want  []place
	}{{
		name:  "fan",
		edges: [][2]string{{"a", "b"}, {"a", "c"}, {"b", "d"}, {"c", "d"}, {"a", "e"}, {"e", "f"}, {"c", "f"}},
		want:  []place{{"a", 110, 10}, {"b", 20, 80}, {"c", 110, 80}, {"d", 30, 150}, {"e", 200, 80}, {"f", 190, 150}},
	}, {
		name:  "cross",
		edges: [][2]string{{"a", "c"}, {"a", "d"}, {"b", "c"}, {"b", "d"}},
		want:  []place{{"a", 30, 10}, {"b", 120, 10}, {"c", 20, 80}, {"d", 130, 80}},
	}, {
		name:  "join",
		edges: [][2]string{{"a", "d"}, {"b", "d"}, {"c", "e"}, {"a", "f"}, {"b", "g"}, {"c", "g"}, {"d", "g"}, {"e", "f"}},
		want: []place{
			{"a", 130, 10}, {"b", 30, 10}, {"c", 220, 10}, {"d", 20, 80}, {"e", 210, 80},
			{"f", 192.5, 150}, {"g", 75, 150},
		},
	}, {
		name:  "tree",
		edges: [][2]string{{"r", "a"}, {"r", "b"}, {"r", "c"}, {"a", "y"}, {"b", "x"}, {"c", "x"}, {"a", "z"}, {"c", "z"}},
		want: []place{
			{"r", 165, 10}, {"a", 30, 80}, {"b", 140, 80}, {"c", 230, 80},
			{"x", 220, 150}, {"y", 20, 150}, {"z", 120, 150},
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewGraph(GraphOptions{Compound: true, Multigraph: true}).SetGraph(Attrs{
				"nodesep": 50.0, "ranksep": 50.0, "rankdir": "TB",
			})
			for _, p := range test.want {
				g.SetNode(p.v, Attrs{"width": 40.0, "height": 20.0})
			}
			for i, e := range test.edges {
				g.SetEdge(e[0], e[1], Attrs{}, strconv.Itoa(i))
			}
			layoutMustSucceed(t, g)
			for _, p := range test.want {
				assertLayoutPoint(t, asAttrs(g.Node(p.v)), p.x, p.y)
			}
		})
	}
}
