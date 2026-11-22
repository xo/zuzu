package graph

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

// engineInputs are graphs that use each part of LayoutInput that the
// layered layout reads.
func engineInputs() map[string]LayoutInput {
	w := 0.0
	return map[string]LayoutInput{
		"chain": {Nodes: []LayoutNode{{ID: "a", Width: 40, Height: 20}, {ID: "b", Width: 40, Height: 20}, {ID: "c", Width: 40, Height: 20}},
			Edges: []LayoutEdge{{Source: "a", Target: "b", LabelWidth: 20, LabelHeight: 10}, {Source: "b", Target: "c"}, {Source: "c", Target: "a", Weight: &w}}},
		"nested": {Direction: LR,
			Nodes:    []LayoutNode{{ID: "a", Cluster: "in", Width: 30, Height: 20}, {ID: "b", Cluster: "out", Width: 30, Height: 20}, {ID: "c", Width: 30, Height: 20, XLabelWidth: 10, XLabelHeight: 8}},
			Edges:    []LayoutEdge{{Source: "a", Target: "b"}, {Source: "c", Target: "in", HeadLabelWidth: 12, HeadLabelHeight: 8}, {Source: "c", Target: "c"}},
			Clusters: []LayoutCluster{{ID: "out", TitleWidth: 20, TitleHeight: 12}, {ID: "in", Parent: "out", Direction: TB}}},
		"lanes": {Lanes: true,
			Nodes:    []LayoutNode{{ID: "a", Cluster: "l1", Width: 30, Height: 20}, {ID: "b", Cluster: "l2", Width: 30, Height: 20}},
			Edges:    []LayoutEdge{{Source: "a", Target: "b"}},
			Clusters: []LayoutCluster{{ID: "l1"}, {ID: "l2"}}},
	}
}

func TestLayeredEngineIsLay(t *testing.T) {
	t.Parallel()
	e := Layered()
	if e.Name() != EngineLayered {
		t.Errorf("expected the name %s, got %s", EngineLayered, e.Name())
	}
	for name, in := range engineInputs() {
		want, err := Lay(in)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got, err := e.Lay(in)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: the engine of zuzu.layered lays out other than Lay", name)
		}
		if got.Unsupported != nil {
			t.Errorf("%s: expected nothing unsupported, got %q", name, got.Unsupported)
		}
	}
}

func TestBuiltinEngine(t *testing.T) {
	t.Parallel()
	for _, name := range []string{EngineLayered, EngineDagre} {
		if e := BuiltinEngine(name); e == nil || e.Name() != name {
			t.Errorf("expected the engine %s, got %v", name, e)
		}
	}
	for _, name := range []string{"", "elk", "Dagre"} {
		if e := BuiltinEngine(name); e != nil {
			t.Errorf("expected no engine for %q, got %s", name, e.Name())
		}
	}
}

func TestFinish(t *testing.T) {
	t.Parallel()
	in := &LayoutInput{
		Nodes: []LayoutNode{{ID: "a", Width: 20, Height: 10}, {ID: "b", Width: 20, Height: 10, XLabelWidth: 8, XLabelHeight: 6}},
		Edges: []LayoutEdge{{Source: "a", Target: "b", Curve: "linear", HeadLabelWidth: 6, HeadLabelHeight: 4}, {Source: "a", Target: "a"}},
	}
	out := &Layout{
		Nodes: []NodePlace{{X: 110, Y: 105, Width: 20, Height: 10}, {X: 110, Y: 145, Width: 20, Height: 10}},
		Edges: []EdgePlace{
			{Points: [][2]float64{{110, 110}, {110, 140}}},
			{Points: [][2]float64{{120, 103}, {130, 103}, {130, 107}, {120, 107}}},
		},
	}
	Finish(in, out)
	if out.Nodes[0].X-out.Nodes[0].Width/2 > 1e-9 && out.Nodes[0].Y-out.Nodes[0].Height/2 > 1e-9 {
		t.Errorf("expected the layout to start at 0, 0, got the node a at %g, %g", out.Nodes[0].X, out.Nodes[0].Y)
	}
	if out.Width <= 0 || out.Height <= 0 {
		t.Errorf("expected a size, got %g by %g", out.Width, out.Height)
	}
	ep := out.Edges[0]
	if ep.EndAngle != 90 || ep.StartAngle != -90 {
		t.Errorf("expected the angles 90 at the end and -90 at the start, got %g and %g", ep.EndAngle, ep.StartAngle)
	}
	if ep.EndX != ep.Points[1][0] || ep.EndY != ep.Points[1][1] {
		t.Errorf("expected the end at the last point, got %g, %g", ep.EndX, ep.EndY)
	}
	if !strings.HasPrefix(ep.Path, "M") || strings.Contains(ep.Path, "C") {
		t.Errorf("expected a linear path, got %q", ep.Path)
	}
	if ep.HeadLabelX == 0 && ep.HeadLabelY == 0 {
		t.Error("expected a place for the label at the head")
	}
	if np := out.Nodes[1]; np.XLabelX == 0 && np.XLabelY == 0 {
		t.Error("expected a place for the xlabel of the node b")
	}
	if p := out.Edges[1].Path; !strings.Contains(p, "C") {
		t.Errorf("expected the cubic path of a loop, got %q", p)
	}
	if math.IsNaN(out.Width) {
		t.Error("expected a finite size")
	}
}
