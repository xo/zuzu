// SPDX-License-Identifier: EPL-2.0

package elk_test

import (
	"errors"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/elk"
	"github.com/xo/zuzu/graph"
)

// nodes returns nodes of a size, each in a cluster after its @ or none.
func nodes(ids ...string) []graph.LayoutNode {
	var out []graph.LayoutNode
	for _, id := range ids {
		name, cluster, _ := strings.Cut(id, "@")
		out = append(out, graph.LayoutNode{ID: name, Cluster: cluster, Width: 40, Height: 20})
	}
	return out
}

// lay lays out an input with the engine of ELK twice, and checks that both
// layouts are the same, that the layout starts at 0, 0, and that each node
// lies in the box of its cluster.
func lay(t *testing.T, in graph.LayoutInput) *graph.Layout {
	t.Helper()
	l, err := elk.Layout().Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := elk.Layout().Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(l, again) {
		t.Error("expected the same layout from two calls")
	}
	box := map[string]graph.ClusterPlace{}
	for i, c := range in.Clusters {
		box[c.ID] = l.Clusters[i]
	}
	minX, minY := math.Inf(1), math.Inf(1)
	for i, n := range in.Nodes {
		p := l.Nodes[i]
		minX, minY = math.Min(minX, p.X-p.Width/2), math.Min(minY, p.Y-p.Height/2)
		if p.Width != n.Width || p.Height != n.Height {
			t.Errorf("expected the node %s to keep its size, got %g by %g", n.ID, p.Width, p.Height)
		}
		if c, ok := box[n.Cluster]; ok && (p.X-p.Width/2 < c.X-0.5 || p.X+p.Width/2 > c.X+c.Width+0.5 ||
			p.Y-p.Height/2 < c.Y-0.5 || p.Y+p.Height/2 > c.Y+c.Height+0.5) {
			t.Errorf("the node %s at %g, %g is outside its cluster %s at %+v", n.ID, p.X, p.Y, n.Cluster, c)
		}
	}
	if minX < -1e-9 || minY < -1e-9 {
		t.Errorf("expected the layout to start at 0, 0, got a node at %g, %g", minX, minY)
	}
	return l
}

func TestLayout(t *testing.T) {
	t.Parallel()
	e := elk.Layout()
	if got := e.Name(); got != graph.EngineELK {
		t.Errorf("the name is %q, want %s", got, graph.EngineELK)
	}
	in := graph.LayoutInput{
		Direction: graph.LR,
		Nodes:     nodes("a", "b@c", "x"),
		Clusters:  []graph.LayoutCluster{{ID: "c", TitleWidth: 30, TitleHeight: 12}},
		Edges: []graph.LayoutEdge{
			{Source: "a", Target: "b", LabelWidth: 20, LabelHeight: 10},
			{Source: "b", Target: "x"},
		},
	}
	l := lay(t, in)
	a, b, x := l.Nodes[0], l.Nodes[1], l.Nodes[2]
	if !(a.X < b.X && b.X < x.X) {
		t.Errorf("expected a, b and x from the left to the right, got %g, %g and %g", a.X, b.X, x.X)
	}
	for i, ep := range l.Edges {
		if len(ep.Points) < 2 || ep.Path == "" {
			t.Errorf("expected a route of the edge %d, got %+v", i, ep)
		}
	}
	if ep := l.Edges[0]; !(ep.LabelX > a.X && ep.LabelX < b.X) {
		t.Errorf("expected the label between a and b, got %g", ep.LabelX)
	}
	if c := l.Clusters[0]; c.Width < 30 || c.TitleX <= c.X || c.TitleY <= c.Y {
		t.Errorf("expected the cluster to fit its title, got %+v", c)
	}
	if len(l.Unsupported) > 0 {
		t.Errorf("expected nothing unsupported, got %q", l.Unsupported)
	}
}

func TestLayoutAlgorithms(t *testing.T) {
	t.Parallel()
	in := graph.LayoutInput{
		Nodes: nodes("a", "b", "c", "d"),
		Edges: []graph.LayoutEdge{{Source: "a", Target: "b"}, {Source: "a", Target: "c"}, {Source: "c", Target: "d"}},
	}
	layered := lay(t, in)
	for _, name := range []string{"", "elk", "elk.layered", "elk.stress", "elk.force", "elk.mrtree", "elk.sporeOverlap", "elk.box", "elk.rectpacking"} {
		in.Algorithm = name
		l := lay(t, in)
		if len(l.Unsupported) > 0 {
			t.Errorf("%q: expected nothing unsupported, got %q", name, l.Unsupported)
		}
		same := reflect.DeepEqual(l.Nodes, layered.Nodes)
		if want := name == "" || name == "elk" || name == "elk.layered"; same != want {
			t.Errorf("%q: expected the places of elk.layered %v, got %v", name, want, same)
		}
		for i, ep := range l.Edges {
			if len(ep.Points) < 2 {
				t.Errorf("%q: expected a route of the edge %d, got %v", name, i, ep.Points)
			}
		}
	}
	in.Algorithm = "elk.radial"
	l := lay(t, in)
	if !reflect.DeepEqual(l.Nodes, layered.Nodes) || len(l.Unsupported) != 1 || !strings.Contains(l.Unsupported[0], `the algorithm "elk.radial"`) {
		t.Errorf("expected elk.radial to lay out with elk.layered and to be unsupported, got %q", l.Unsupported)
	}
}

func TestLayoutReverseClusters(t *testing.T) {
	t.Parallel()
	in := graph.LayoutInput{
		Nodes:    nodes("a@one", "b@two", "c@three"),
		Clusters: []graph.LayoutCluster{{ID: "one"}, {ID: "two"}, {ID: "three"}},
	}
	// left returns the ids of the clusters from the left.
	left := func(l *graph.Layout) string {
		ids := []string{"one", "two", "three"}
		order := slices.Clone(ids)
		slices.SortStableFunc(order, func(a, b string) int {
			pa, pb := l.Clusters[slices.Index(ids, a)], l.Clusters[slices.Index(ids, b)]
			switch {
			case pa.X < pb.X:
				return -1
			case pa.X > pb.X:
				return 1
			}
			return 0
		})
		return strings.Join(order, " ")
	}
	plain := left(lay(t, in))
	in.ReverseClusters = true
	reversed := left(lay(t, in))
	if plain == reversed {
		t.Errorf("expected ReverseClusters to change the order of the clusters, got %s both times", plain)
	}
}

func TestLayoutUnsupported(t *testing.T) {
	t.Parallel()
	weight := 2.0
	in := graph.LayoutInput{
		Lanes: true,
		Nodes: []graph.LayoutNode{{ID: "a", Rank: "same", Width: 40, Height: 20}, {ID: "b", Rank: "same", Width: 40, Height: 20}, {ID: "c", Width: 40, Height: 20}},
		Edges: []graph.LayoutEdge{
			{Source: "a", Target: "b", SourcePort: []float64{0, 10}},
			{Source: "b", Target: "c", NoConstraint: true, MinLen: 2, Weight: &weight},
		},
	}
	l := lay(t, in)
	want := []string{
		"the lanes of the layout, which zuzu.elk leaves out",
		`the group of the same rank "same", which zuzu.elk leaves out`,
		"the port of the edge a -> b, which zuzu.elk leaves out",
		"constraint false on the edge b -> c, which zuzu.elk leaves out",
		"the length 2 of the edge b -> c, which zuzu.elk leaves out",
		"the weight 2 of the edge b -> c, which zuzu.elk leaves out",
	}
	if !slices.Equal(l.Unsupported, want) {
		t.Errorf("expected the parts\n%s\ngot\n%s", strings.Join(want, "\n"), strings.Join(l.Unsupported, "\n"))
	}
}

func TestLayoutShapes(t *testing.T) {
	t.Parallel()
	in := graph.LayoutInput{
		Nodes: []graph.LayoutNode{{ID: "a", Shape: "diamond", Width: 60, Height: 40}, {ID: "b", Shape: "ellipse", Width: 60, Height: 40}},
		Edges: []graph.LayoutEdge{{Source: "a", Target: "b"}},
	}
	l := lay(t, in)
	a, b, ep := l.Nodes[0], l.Nodes[1], l.Edges[0]
	start, end := ep.Points[0], ep.Points[len(ep.Points)-1]
	if d := math.Abs(start[0]-a.X)/(a.Width/2) + math.Abs(start[1]-a.Y)/(a.Height/2); math.Abs(d-1) > 1e-6 {
		t.Errorf("expected the start on the outline of the diamond, got %v, at %g", start, d)
	}
	ex, ey := (end[0]-b.X)/(b.Width/2), (end[1]-b.Y)/(b.Height/2)
	if d := ex*ex + ey*ey; math.Abs(d-1) > 1e-6 {
		t.Errorf("expected the end on the outline of the ellipse, got %v, at %g", end, d)
	}
}

func TestLayoutErrors(t *testing.T) {
	t.Parallel()
	for name, in := range map[string]graph.LayoutInput{
		"twice":     {Nodes: nodes("a", "a")},
		"end":       {Nodes: nodes("a"), Edges: []graph.LayoutEdge{{Source: "a", Target: "z"}}},
		"direction": {Nodes: nodes("a"), Direction: "up"},
		"loop":      {Clusters: []graph.LayoutCluster{{ID: "a", Parent: "b"}, {ID: "b", Parent: "a"}}},
	} {
		if _, err := elk.Layout().Lay(in); !errors.Is(err, graph.ErrGraph) {
			t.Errorf("%s: expected an error that wraps graph.ErrGraph, got %v", name, err)
		}
	}
}
