// SPDX-License-Identifier: EPL-2.0

package elk

// This file is a file of zuzu, not of elk-go. It holds the checks of the
// tests layout_test.go and inside_self_loop_test.go of upstream that do not
// need the JSON form of a graph, written with the types of Go (change 4). The
// expected values are the values of upstream, which match ELK.js 0.12.0.

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func TestZuzuLayoutEmptyGraph(t *testing.T) {
	t.Parallel()
	g := &Graph{ID: StringID("root"), Children: []*Node{}, Edges: []*Edge{}}
	if err := Layout(g); err != nil {
		t.Fatal(err)
	}
	if g.X == nil || g.Y == nil || g.Width == nil || g.Height == nil {
		t.Fatal("the layout did not set the place and the size of the root")
	}
}

func TestZuzuLayoutRejectsAMissingReference(t *testing.T) {
	t.Parallel()
	g := &Graph{
		ID:       StringID("root"),
		Children: []*Node{{ID: StringID("a")}},
		Edges: []*Edge{{
			ID:      StringID("e"),
			Sources: []ID{StringID("missing")},
			Targets: []ID{StringID("a")},
		}},
	}
	if err := Layout(g); err == nil {
		t.Fatal("an edge from a missing node gave no error")
	}
}

// cycleGraph returns the graph of TestLayoutJSONConcurrentDeterminism of
// upstream.
func cycleGraph() *Graph {
	return &Graph{
		ID: StringID("root"),
		LayoutOptions: LayoutOptions{
			"elk.algorithm":                           "layered",
			"elk.direction":                           "DOWN",
			"elk.hierarchyHandling":                   "INCLUDE_CHILDREN",
			"elk.layered.considerModelOrder.strategy": "NODES_AND_EDGES",
			"elk.layered.cycleBreaking.strategy":      "GREEDY_MODEL_ORDER",
		},
		Children: []*Node{
			{ID: StringID("a"), Width: Float64(60), Height: Float64(30)},
			{ID: StringID("b"), Width: Float64(70), Height: Float64(35)},
			{ID: StringID("c"), Width: Float64(50), Height: Float64(25)},
		},
		Edges: []*Edge{
			{ID: StringID("e1"), Sources: []ID{StringID("a")}, Targets: []ID{StringID("b")}},
			{ID: StringID("e2"), Sources: []ID{StringID("a")}, Targets: []ID{StringID("c")}},
			{ID: StringID("e3"), Sources: []ID{StringID("c")}, Targets: []ID{StringID("a")}},
		},
	}
}

func TestZuzuLayoutConcurrentDeterminism(t *testing.T) {
	t.Parallel()
	want := cycleGraph()
	if err := Layout(want); err != nil {
		t.Fatal(err)
	}
	const workers = 32
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Go(func() {
			for iteration := range 4 {
				got := cycleGraph()
				if err := Layout(got); err != nil {
					errs <- fmt.Errorf("worker %d iteration %d: %w", worker, iteration, err)
					return
				}
				if !reflect.DeepEqual(got, want) {
					errs <- fmt.Errorf("worker %d iteration %d gave another layout", worker, iteration)
					return
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestZuzuLayoutProgrammaticAliasesPreferCanonicalSpelling(t *testing.T) {
	t.Parallel()
	const canonical = "org.eclipse.elk.layered.spacing.nodeNodeBetweenLayers"
	const legacy = "spacing.nodeNodeBetweenLayers"
	g := &Graph{
		ID: StringID("root"),
		LayoutOptions: LayoutOptions{
			"elk.algorithm": "layered",
			legacy:          "100",
			canonical:       "20",
		},
		Children: []*Node{
			{ID: StringID("a"), Width: Float64(10), Height: Float64(10)},
			{ID: StringID("b"), Width: Float64(10), Height: Float64(10)},
		},
		Edges: []*Edge{{ID: StringID("e"), Sources: []ID{StringID("a")}, Targets: []ID{StringID("b")}}},
	}
	if err := Layout(g); err != nil {
		t.Fatal(err)
	}
	if x := g.Children[1].X; x == nil || *x != 42 {
		t.Fatalf("b.x = %v, want 42, the result of the canonical spacing", x)
	}
}

func TestZuzuLayoutPlacesLayeredNodeLabels(t *testing.T) {
	t.Parallel()
	g := &Graph{
		ID:            StringID("root"),
		LayoutOptions: LayoutOptions{"elk.algorithm": "layered"},
		Children: []*Node{{
			ID: StringID("a"), Width: Float64(40), Height: Float64(40),
			LayoutOptions: LayoutOptions{"org.eclipse.elk.nodeLabels.placement": "INSIDE H_CENTER V_CENTER"},
			Labels:        []*Label{{ID: IDPtr(StringID("l")), Text: String("x"), Width: Float64(14), Height: Float64(6)}},
		}},
	}
	if err := Layout(g); err != nil {
		t.Fatal(err)
	}
	l := g.Children[0].Labels[0]
	if l.X == nil || l.Y == nil || *l.X != 13 || *l.Y != 17 {
		t.Fatalf("the label of the node is at (%v, %v), want (13, 17)", l.X, l.Y)
	}
}

func TestZuzuLayoutPlacesLayeredPortLabels(t *testing.T) {
	t.Parallel()
	g := &Graph{
		ID:            StringID("root"),
		LayoutOptions: LayoutOptions{"elk.algorithm": "layered"},
		Children: []*Node{{
			ID: StringID("a"), Width: Float64(100), Height: Float64(100),
			LayoutOptions: LayoutOptions{"elk.portConstraints": "FIXED_SIDE"},
			Ports: []*Port{{
				ID: StringID("p"), Width: Float64(10), Height: Float64(10),
				LayoutOptions: LayoutOptions{"elk.port.side": "EAST"},
				Labels: []*Label{
					{ID: IDPtr(StringID("l1")), Text: String("one"), Width: Float64(20), Height: Float64(10)},
					{ID: IDPtr(StringID("l2")), Text: String("two"), Width: Float64(30), Height: Float64(10)},
				},
			}},
		}},
	}
	if err := Layout(g); err != nil {
		t.Fatal(err)
	}
	p := g.Children[0].Ports[0]
	if p.X == nil || p.Y == nil || *p.X != 100 || *p.Y != 45 {
		t.Fatalf("the port is at (%v, %v), want (100, 45)", p.X, p.Y)
	}
	wantY := []float64{11, 21}
	for i, l := range p.Labels {
		if l.X == nil || l.Y == nil || *l.X != 11 || *l.Y != wantY[i] {
			t.Fatalf("label %d of the port is at (%v, %v), want (11, %v)", i, l.X, l.Y, wantY[i])
		}
	}
}

func TestZuzuLayoutRoutesInsideSelfLoopInNodeContents(t *testing.T) {
	t.Parallel()
	g := &Graph{
		ID:            StringID("root"),
		LayoutOptions: LayoutOptions{"elk.algorithm": "layered"},
		Children: []*Node{{
			ID: StringID("a"), Width: Float64(100), Height: Float64(100),
			LayoutOptions: LayoutOptions{
				"org.eclipse.elk.insideSelfLoops.activate": "true",
				"elk.portConstraints":                      "FIXED_POS",
			},
			Ports: []*Port{
				{
					ID: StringID("p1"), X: Float64(0), Y: Float64(20), Width: Float64(10), Height: Float64(10),
					LayoutOptions: LayoutOptions{"elk.port.side": "WEST"},
				},
				{
					ID: StringID("p2"), X: Float64(90), Y: Float64(70), Width: Float64(10), Height: Float64(10),
					LayoutOptions: LayoutOptions{"elk.port.side": "EAST"},
				},
			},
		}},
		Edges: []*Edge{{
			ID: StringID("e"), Sources: []ID{StringID("p1")}, Targets: []ID{StringID("p2")},
			LayoutOptions: LayoutOptions{"org.eclipse.elk.insideSelfLoops.yo": "true"},
		}},
	}
	if err := Layout(g); err != nil {
		t.Fatal(err)
	}
	n := g.Children[0]
	if *n.X != 12 || *n.Y != 12 || *n.Width != 64 || *n.Height != 80 {
		t.Fatalf("the node is x=%v y=%v w=%v h=%v, want 12, 12, 64 and 80", *n.X, *n.Y, *n.Width, *n.Height)
	}
	if *n.Ports[0].X != 0 || *n.Ports[0].Y != 20 || *n.Ports[1].X != 54 || *n.Ports[1].Y != 70 {
		t.Fatalf("the ports are at (%v, %v) and (%v, %v), want (0, 20) and (54, 70)",
			*n.Ports[0].X, *n.Ports[0].Y, *n.Ports[1].X, *n.Ports[1].Y)
	}
	if *g.Width != 88 || *g.Height != 104 {
		t.Fatalf("the root is %vx%v, want 88x104", *g.Width, *g.Height)
	}
	if len(g.Edges[0].Sections) != 1 {
		t.Fatalf("the loop has %d sections, want 1", len(g.Edges[0].Sections))
	}
	s := g.Edges[0].Sections[0]
	want := []Point{{X: 22, Y: 37}, {X: 44, Y: 37}, {X: 44, Y: 87}, {X: 66, Y: 87}}
	got := append(append([]Point{*s.StartPoint}, s.BendPoints...), *s.EndPoint)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the loop runs through %v, want %v", got, want)
	}
}
