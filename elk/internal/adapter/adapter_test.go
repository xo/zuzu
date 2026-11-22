// SPDX-License-Identifier: EPL-2.0

package adapter

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestEachAlgorithmRuns(t *testing.T) {
	t.Parallel()
	for _, a := range Algorithms {
		g := small(a)
		r, err := Layout(&g)
		if err != nil {
			t.Fatalf("%s: %v", a, err)
		}
		if !(r.Width > 0 && r.Height > 0) {
			t.Errorf("%s: the graph is %vx%v", a, r.Width, r.Height)
		}
		for i, p := range r.Nodes {
			// RectPacking expands the nodes to fill their rows, as Mermaid asks.
			if !finite(p.X, p.Y, p.Width, p.Height) || p.Width < g.Nodes[i].Width || p.Height < g.Nodes[i].Height ||
				a != RectPacking && (p.Width != g.Nodes[i].Width || p.Height != g.Nodes[i].Height) {
				t.Errorf("%s: node %s is at %+v, of the size %vx%v", a, g.Nodes[i].ID, p.Rect, g.Nodes[i].Width, g.Nodes[i].Height)
			}
			if p.X < 0 || p.Y < 0 || p.X+p.Width > r.Width || p.Y+p.Height > r.Height {
				t.Errorf("%s: node %s at %+v is not inside the graph of %vx%v", a, g.Nodes[i].ID, p.Rect, r.Width, r.Height)
			}
		}
		routes := a != Box && a != RectPacking
		for i, p := range r.Edges {
			if routes && len(p.Points) < 2 {
				t.Errorf("%s: edge %s has the points %v", a, g.Edges[i].ID, p.Points)
			}
			for _, pt := range p.Points {
				if !finite(pt.X, pt.Y) {
					t.Errorf("%s: edge %s has the point %v", a, g.Edges[i].ID, pt)
				}
			}
		}
	}
}

// finite reports whether each number is finite.
func finite(vs ...float64) bool {
	for _, v := range vs {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

func TestLayoutIsDeterministic(t *testing.T) {
	t.Parallel()
	for _, f := range fixtures() {
		want, err := Layout(&f.g)
		if err != nil {
			t.Fatalf("%s: %v", f.name, err)
		}
		var wg sync.WaitGroup
		results := make([]*Result, 8)
		errs := make([]error, len(results))
		for i := range results {
			wg.Go(func() {
				g := f.g
				results[i], errs[i] = Layout(&g)
			})
		}
		wg.Wait()
		for i, got := range results {
			if errs[i] != nil {
				t.Fatalf("%s: %v", f.name, errs[i])
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s: run %d gives another layout", f.name, i)
			}
		}
	}
}

func TestSeedsAreFixed(t *testing.T) {
	t.Parallel()
	for _, a := range []string{Layered, Force, SporeOverlap, "org.eclipse.elk.random"} {
		var want *Result
		for _, seed := range []string{"0", "1", ""} {
			g := small(Layered)
			g.Options = map[string]string{"elk.algorithm": a}
			if seed != "" {
				g.Options["elk.randomSeed"] = seed
			}
			got, err := Layout(&g)
			if err != nil {
				t.Fatalf("%s: %v", a, err)
			}
			if want == nil {
				want = got
				continue
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: the seed %q gives another layout than the seed 0", a, seed)
			}
		}
	}
}

func TestTitlesOfClusters(t *testing.T) {
	t.Parallel()
	// ELK.js widens a cluster for its title when the direction is LR or RL.
	g := Graph{
		Direction: "LR",
		Nodes: []Node{
			{ID: "c", Cluster: true, Label: title("A long title", 140, 24), Padding: 8},
			in("c", "a", 40, 30),
		},
	}
	r, err := Layout(&g)
	if err != nil {
		t.Fatal(err)
	}
	c, a := r.Nodes[0], r.Nodes[1]
	if c.Label == nil {
		t.Fatal("the title of the cluster has no place")
	}
	if l := *c.Label; l.Y != c.Y+labelPadding || l.X+l.Width/2 != c.X+c.Width/2 {
		t.Errorf("the title is at %+v, want the middle of the top of %+v", l, c.Rect)
	}
	if c.Width != 140+2*labelPadding {
		t.Errorf("the cluster is %v wide, want its title and the padding of the label", c.Width)
	}
	if a.Y < c.Label.Y+c.Label.Height {
		t.Errorf("node a at %+v is under the title at %+v", a.Rect, *c.Label)
	}
}

func TestEndLabelsNeedLayered(t *testing.T) {
	t.Parallel()
	g := Graph{
		Algorithm: Stress,
		Nodes:     []Node{box("a", 40, 30), box("b", 40, 30)},
		Edges:     []Edge{{ID: "e", From: "a", To: "b", Head: []Label{{Text: "1", Width: 10, Height: 20}}}},
	}
	r, err := Layout(&g)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Issues) != 1 || !strings.Contains(r.Issues[0], "end labels") {
		t.Errorf("the issues are %q, want one on the end labels", r.Issues)
	}
	if len(r.Edges[0].Head) != 0 {
		t.Errorf("the head labels have places %v, want none", r.Edges[0].Head)
	}
}

func TestAClusterLosesItsAlgorithmToACrossingEdge(t *testing.T) {
	t.Parallel()
	g := Graph{
		Nodes: []Node{
			box("out", 40, 30),
			{ID: "c", Cluster: true, Label: title("C", 10, 20), Algorithm: Box},
			in("c", "a", 40, 30),
			{ID: "d", Cluster: true, Label: title("D", 10, 20), Algorithm: "elk.nothing"},
			in("d", "b", 40, 30),
		},
		Edges: []Edge{edge("e", "out", "a")},
	}
	r, err := Layout(&g)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`the cluster "d" asks for the algorithm "elk.nothing", which ELK does not have, so it takes the algorithm of the graph`,
		`an edge crosses the cluster "c", so it takes the algorithm of the graph`,
	}
	if !reflect.DeepEqual(r.Issues, want) {
		t.Errorf("the issues are %q, want %q", r.Issues, want)
	}
}

func TestErrors(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		g    Graph
		err  error
	}{
		{"unknown algorithm", Graph{Algorithm: "elk.nothing"}, ErrAlgorithm},
		{"radial for the graph", Graph{Algorithm: Radial}, ErrAlgorithm},
		{"two nodes with one ID", Graph{Nodes: []Node{box("a", 1, 1), box("a", 1, 1)}}, ErrGraph},
		{"a node named root", Graph{Nodes: []Node{box("root", 1, 1)}}, ErrGraph},
		{"a node with no ID", Graph{Nodes: []Node{box("", 1, 1)}}, ErrGraph},
		{"a port with the ID of a node", Graph{Nodes: []Node{
			{ID: "a", Ports: []Port{{ID: "b"}}}, box("b", 1, 1),
		}}, ErrGraph},
		{"an edge with the ID of a node", Graph{
			Nodes: []Node{box("a", 1, 1)}, Edges: []Edge{edge("a", "a", "a")},
		}, ErrGraph},
		{"a parent that is not a cluster", Graph{Nodes: []Node{box("a", 1, 1), in("a", "b", 1, 1)}}, ErrGraph},
		{"a parent that does not exist", Graph{Nodes: []Node{in("x", "b", 1, 1)}}, ErrGraph},
		{"a loop of clusters", Graph{Nodes: []Node{
			{ID: "c1", Parent: "c2", Cluster: true}, {ID: "c2", Parent: "c1", Cluster: true},
		}}, ErrGraph},
		{"an edge to a missing node", Graph{
			Nodes: []Node{box("a", 1, 1)}, Edges: []Edge{edge("e", "a", "x")},
		}, ErrGraph},
		{"a port of another node", Graph{
			Nodes: []Node{{ID: "a", Ports: []Port{{ID: "p"}}}, box("b", 1, 1)},
			Edges: []Edge{{ID: "e", From: "b", FromPort: "p", To: "a"}},
		}, ErrGraph},
	} {
		if _, err := Layout(&test.g); !errors.Is(err, test.err) {
			t.Errorf("%s: the error is %v, want %v", test.name, err, test.err)
		}
	}
}

func TestRootOptionsFollowMermaid(t *testing.T) {
	t.Parallel()
	opts := rootOptions(&Graph{Direction: "LR"}, Layered)
	want := map[string]any{
		"elk.algorithm":                                        Layered,
		"elk.direction":                                        "RIGHT",
		"elk.layered.nodePlacement.strategy":                   "BRANDES_KOEPF",
		"elk.layered.nodePlacement.bk.fixedAlignment":          "BALANCED",
		"elk.layered.cycleBreaking.strategy":                   "DEPTH_FIRST",
		"elk.layered.layering.strategy":                        "NETWORK_SIMPLEX",
		"elk.layered.mergeEdges":                               "false",
		"elk.layered.crossingMinimization.forceNodeModelOrder": "false",
		"elk.layered.considerModelOrder.strategy":              "NODES_AND_EDGES",
		"elk.layered.layering.coffmanGraham.layerBound":        "4",
		"spacing.baseValue":                                    "40",
	}
	for k, v := range want {
		if opts[k] != v {
			t.Errorf("default: %s is %v, want %v", k, opts[k], v)
		}
	}
	opts = rootOptions(&Graph{
		Direction: "BT",
		Config: Config{
			Preset:                "modelOrder",
			NodePlacementStrategy: "LINEAR_SEGMENTS",
			CycleBreakingStrategy: "GREEDY",
			MergeEdges:            true,
		},
		Options: map[string]string{"spacing.baseValue": "60"},
	}, Stress)
	want = map[string]any{
		"elk.algorithm":                               Stress,
		"elk.direction":                               "UP",
		"elk.layered.nodePlacement.strategy":          "LINEAR_SEGMENTS",
		"elk.layered.nodePlacement.bk.fixedAlignment": "NONE",
		"elk.layered.cycleBreaking.strategy":          "GREEDY",
		"elk.layered.mergeEdges":                      "true",
		"spacing.baseValue":                           "60",
	}
	for k, v := range want {
		if opts[k] != v {
			t.Errorf("set: %s is %v, want %v", k, opts[k], v)
		}
	}
}

func TestDirection(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"": "DOWN", "TB": "DOWN", "TD": "DOWN", "BT": "UP", "LR": "RIGHT", "RL": "LEFT", "XX": "DOWN"} {
		if got := direction(in); got != want {
			t.Errorf("direction(%q) = %q, want %q", in, got, want)
		}
	}
}
