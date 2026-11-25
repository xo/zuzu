// SPDX-License-Identifier: EPL-2.0

package elk_test

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
)

// TestLayoutPorts checks that an edge leaves the port of its source and
// enters the port of its target on the border of each node, at the place
// of the port along the side that faces the route, and that a port at a
// cluster is unsupported.
func TestLayoutPorts(t *testing.T) {
	t.Parallel()
	in := graph.LayoutInput{
		Nodes: []graph.LayoutNode{{ID: "a", Width: 120, Height: 30}, {ID: "b", Width: 120, Height: 30}},
		Edges: []graph.LayoutEdge{
			{Source: "a", Target: "b", SourcePort: []float64{40, 0}, TargetPort: []float64{-40, 0}},
		},
	}
	l := lay(t, in)
	a, b, pts := l.Nodes[0], l.Nodes[1], l.Edges[0].Points
	start, end := pts[0], pts[len(pts)-1]
	if math.Abs(start[0]-(a.X+40)) > 0.5 || math.Abs(start[1]-(a.Y+a.Height/2)) > 0.5 {
		t.Errorf("expected the start at the bottom of a, 40 to the right, got %v with a at %g, %g", start, a.X, a.Y)
	}
	if math.Abs(end[0]-(b.X-40)) > 0.5 || math.Abs(end[1]-(b.Y-b.Height/2)) > 0.5 {
		t.Errorf("expected the end at the top of b, 40 to the left, got %v with b at %g, %g", end, b.X, b.Y)
	}
	if len(l.Unsupported) > 0 {
		t.Errorf("expected nothing unsupported, got %q", l.Unsupported)
	}
	// A port on a side, such as the east side, leaves that side.
	in.Edges[0].SourcePort, in.Edges[0].SourceSide = []float64{60, 0}, []float64{1, 0}
	l = lay(t, in)
	a, start = l.Nodes[0], l.Edges[0].Points[0]
	if math.Abs(start[0]-(a.X+a.Width/2)) > 0.5 || math.Abs(start[1]-a.Y) > 0.5 {
		t.Errorf("expected the start at the east of a, got %v with a at %g, %g", start, a.X, a.Y)
	}
	in = graph.LayoutInput{
		Nodes:    nodes("a@k", "b"),
		Clusters: []graph.LayoutCluster{{ID: "k"}},
		Edges:    []graph.LayoutEdge{{Source: "k", Target: "b", SourcePort: []float64{0, 0}}},
	}
	if l := lay(t, in); !slices.Equal(l.Unsupported, []string{"the port of the edge k -> b at k, which zuzu.elk leaves out"}) {
		t.Errorf("expected the port at the cluster unsupported, got %q", l.Unsupported)
	}
}

// TestLayoutEndLabels checks that ELK places the labels at the ends of an
// edge near their ends, and that graph.Finish keeps those places.
func TestLayoutEndLabels(t *testing.T) {
	t.Parallel()
	in := graph.LayoutInput{
		Nodes: nodes("a", "b"),
		Edges: []graph.LayoutEdge{{
			Source: "a", Target: "b", HeadMarker: true,
			TailLabelWidth: 10, TailLabelHeight: 12, HeadLabelWidth: 10, HeadLabelHeight: 12,
		}},
	}
	l := lay(t, in)
	a, b, ep := l.Nodes[0], l.Nodes[1], l.Edges[0]
	if !ep.HeadPlaced || !ep.TailPlaced {
		t.Fatalf("expected ELK to place both labels, got %+v", ep)
	}
	if !(ep.TailLabelY > a.Y && ep.TailLabelY < ep.HeadLabelY && ep.HeadLabelY < b.Y) {
		t.Errorf("expected the tail label below a and the head label above b, got %g and %g, a at %g, b at %g",
			ep.TailLabelY, ep.HeadLabelY, a.Y, b.Y)
	}
	// A label at a fixed place of DOT, and a label of an algorithm that
	// places none, take the place of graph.Finish.
	in.Edges[0].LabelDistance = 2
	if ep := lay(t, in).Edges[0]; ep.HeadPlaced || ep.TailPlaced {
		t.Errorf("expected graph.Finish to place the labels at a fixed place, got %+v", ep)
	}
	in.Edges[0].LabelDistance, in.Algorithm = 0, "elk.stress"
	if l := lay(t, in); l.Edges[0].HeadPlaced || len(l.Unsupported) > 0 {
		t.Errorf("expected graph.Finish to place the labels of elk.stress with no issue, got %+v and %q", l.Edges[0], l.Unsupported)
	}
}

// TestLayoutSettings checks that the engine reads the settings of the
// section elk of the config of Mermaid, and names the ones that it does
// not read.
func TestLayoutSettings(t *testing.T) {
	t.Parallel()
	// The cycle a -> b -> c -> a has no source. ELK breaks it at b, the
	// first node of the model, and keepEntryNodeOnTop pins a, the first
	// node that no edge enters once c -> a, which closes the cycle in the
	// order of the edges, is left out.
	in := graph.LayoutInput{
		Nodes: nodes("b", "c", "a"),
		Edges: []graph.LayoutEdge{{Source: "a", Target: "b"}, {Source: "b", Target: "c"}, {Source: "c", Target: "a"}},
	}
	top := func(l *graph.Layout) string {
		ids := []string{"b", "c", "a"}
		least := 0
		for i := range ids {
			if l.Nodes[i].Y < l.Nodes[least].Y {
				least = i
			}
		}
		return ids[least]
	}
	if got := top(lay(t, in)); got == "a" {
		t.Errorf("expected ELK to break the cycle elsewhere with no setting, got %s on top", got)
	}
	in.ELK = map[string]string{"keepEntryNodeOnTop": "true"}
	if got := top(lay(t, in)); got != "a" {
		t.Errorf("expected a on top with keepEntryNodeOnTop, got %s", got)
	}
	in.ELK = map[string]string{
		"mergeEdges": "true", "orientFeedbackEdges": "false", "preset": "legacy", "layeringLayerBound": "3",
		"lineHops": "true", "straightenEdges": "false", "forceNodeModelOrder": "maybe",
	}
	l := lay(t, in)
	want := []string{
		"the setting elk.forceNodeModelOrder: maybe, which zuzu.elk leaves out",
		"the setting elk.lineHops, which zuzu.elk leaves out",
		"the setting elk.straightenEdges, which zuzu.elk leaves out",
	}
	if !slices.Equal(l.Unsupported, want) {
		t.Errorf("expected the settings\n%s\ngot\n%s", strings.Join(want, "\n"), strings.Join(l.Unsupported, "\n"))
	}
}
