//go:build amd64 && !amd64.v3

package dagro

// This file is a file of zuzu, not of dagro. It holds the golden test of
// the copy: the layout of a fixed set of graphs must be the same bytes as
// testdata/golden.txt. The file holds the bytes of amd64 below the level v3,
// so the test runs only there. On another machine the compiler can fuse a
// multiply and an add into one instruction, so the bytes can differ. zuzu
// promises the same bytes on one architecture, not across machines (D41).

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "write the golden files again")

// goldenGraph builds one graph of the test.
type goldenGraph struct {
	name  string
	build func() *Graph
}

// newGoldenGraph returns a compound multigraph with the attributes of a
// graph, as the engine of zuzu.dagre makes it.
func newGoldenGraph(attrs Attrs) *Graph {
	return NewGraph(GraphOptions{Compound: true, Multigraph: true}).SetGraph(attrs)
}

var goldenGraphs = []goldenGraph{{
	name: "chain",
	build: func() *Graph {
		g := newGoldenGraph(Attrs{"rankdir": "TB"})
		for _, v := range []string{"a", "b", "c", "d"} {
			g.SetNode(v, Attrs{"width": 60.0, "height": 30.0})
		}
		g.SetEdge("a", "b", Attrs{}, "0")
		g.SetEdge("b", "c", Attrs{}, "1")
		g.SetEdge("a", "c", Attrs{"minlen": 2.0}, "2")
		g.SetEdge("c", "d", Attrs{}, "3")
		g.SetEdge("d", "a", Attrs{}, "4")
		return g
	},
}, {
	name: "labels",
	build: func() *Graph {
		g := newGoldenGraph(Attrs{"rankdir": "LR", "nodesep": 34.375, "ranksep": 34.375, "edgesep": 13.75})
		for i, v := range []string{"start", "left", "right", "end"} {
			g.SetNode(v, Attrs{"width": 40.0 + 10*float64(i), "height": 24.0})
		}
		g.SetEdge("start", "left", Attrs{"width": 30.0, "height": 14.0, "labelpos": "c"}, "0")
		g.SetEdge("start", "right", Attrs{"width": 50.0, "height": 14.0, "labelpos": "c"}, "1")
		g.SetEdge("left", "end", Attrs{"weight": 2.0}, "2")
		g.SetEdge("right", "end", Attrs{"width": 20.0, "height": 28.0, "labelpos": "r"}, "3")
		g.SetEdge("start", "end", Attrs{}, "4")
		return g
	},
}, {
	name: "clusters",
	build: func() *Graph {
		g := newGoldenGraph(Attrs{"rankdir": "BT", "nodesep": 20.0, "ranksep": 30.0})
		for _, v := range []string{"x", "y", "z", "w", "v"} {
			g.SetNode(v, Attrs{"width": 50.0, "height": 20.0})
		}
		g.SetNode("outer", Attrs{})
		g.SetNode("inner", Attrs{})
		for _, p := range [][2]string{{"inner", "outer"}, {"x", "outer"}, {"y", "inner"}, {"z", "inner"}} {
			if err := g.SetParent(p[0], p[1]); err != nil {
				panic(err)
			}
		}
		g.SetEdge("x", "y", Attrs{}, "0")
		g.SetEdge("y", "z", Attrs{"width": 24.0, "height": 12.0, "labelpos": "c"}, "1")
		g.SetEdge("z", "w", Attrs{}, "2")
		g.SetEdge("w", "v", Attrs{}, "3")
		g.SetEdge("v", "x", Attrs{}, "4")
		g.SetEdge("w", "w", Attrs{"width": 10.0, "height": 10.0}, "5")
		return g
	},
}}

// TestGoldenLayouts compares the places of each node and the points and
// the label of each edge with the golden file, byte for byte.
func TestGoldenLayouts(t *testing.T) {
	var b strings.Builder
	f := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
	for _, gg := range goldenGraphs {
		g := gg.build()
		if err := Layout(g); err != nil {
			t.Fatalf("%s: %v", gg.name, err)
		}
		fmt.Fprintf(&b, "graph %s %s %s\n", gg.name, f(num(asAttrs(g.Graph()), "width")), f(num(asAttrs(g.Graph()), "height")))
		for _, v := range g.Nodes() {
			n := asAttrs(g.Node(v))
			fmt.Fprintf(&b, "node %s %s %s %s %s\n", v, f(num(n, "x")), f(num(n, "y")), f(num(n, "width")), f(num(n, "height")))
		}
		for _, e := range g.Edges() {
			a := asAttrs(g.Edge(e))
			fmt.Fprintf(&b, "edge %s %s %s", e.V, e.W, e.Name)
			if has(a, "x") {
				fmt.Fprintf(&b, " label %s %s", f(num(a, "x")), f(num(a, "y")))
			}
			for _, p := range a["points"].([]Point) {
				fmt.Fprintf(&b, " %s,%s", f(p.X), f(p.Y))
			}
			b.WriteString("\n")
		}
	}
	path := filepath.Join("testdata", "golden.txt")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := b.String(); got != string(want) {
		t.Errorf("the layouts differ from %s. Run go test -run TestGoldenLayouts -update on amd64 below the level v3 to write it again, if the change is on purpose.\ngot:\n%s", path, got)
	}
}
