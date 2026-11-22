package graph

import (
	"cmp"
	"errors"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// layDagre lays out a graph with the engine of zuzu.dagre, and checks the
// rules that every layout keeps: no two nodes overlap, each node lies in
// the box of its cluster and each cluster in the box of its parent, and the
// layout starts at 0, 0.
func layDagre(t *testing.T, in LayoutInput) *Layout {
	t.Helper()
	l, err := Dagre().Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Dagre().Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(l, again) {
		t.Error("expected the same layout from two calls")
	}
	for i := range l.Nodes {
		for j := range i {
			a, b := l.Nodes[i], l.Nodes[j]
			if math.Abs(a.X-b.X) < (a.Width+b.Width)/2-0.5 && math.Abs(a.Y-b.Y) < (a.Height+b.Height)/2-0.5 {
				t.Errorf("the nodes %s and %s overlap", in.Nodes[i].ID, in.Nodes[j].ID)
			}
		}
	}
	box := map[string]ClusterPlace{}
	for i, c := range in.Clusters {
		box[c.ID] = l.Clusters[i]
	}
	inside := func(c ClusterPlace, x0, y0, x1, y1 float64) bool {
		return x0 >= c.X-0.5 && y0 >= c.Y-0.5 && x1 <= c.X+c.Width+0.5 && y1 <= c.Y+c.Height+0.5
	}
	minX, minY := math.Inf(1), math.Inf(1)
	for i, n := range in.Nodes {
		p := l.Nodes[i]
		minX, minY = math.Min(minX, p.X-p.Width/2), math.Min(minY, p.Y-p.Height/2)
		if n.Cluster != "" && !inside(box[n.Cluster], p.X-p.Width/2, p.Y-p.Height/2, p.X+p.Width/2, p.Y+p.Height/2) {
			t.Errorf("the node %s is outside its cluster %s", n.ID, n.Cluster)
		}
	}
	for _, c := range in.Clusters {
		if inner, outer := box[c.ID], box[c.Parent]; c.Parent != "" && !inside(outer, inner.X, inner.Y, inner.X+inner.Width, inner.Y+inner.Height) {
			t.Errorf("the cluster %s is outside its cluster %s", c.ID, c.Parent)
		}
	}
	if minX < -1e-9 || minY < -1e-9 {
		t.Errorf("expected the layout to start at 0, 0, got a node at %g, %g", minX, minY)
	}
	return l
}

// nodes returns nodes of a size, each in a cluster or none.
func nodes(ids ...string) []LayoutNode {
	var out []LayoutNode
	for _, id := range ids {
		name, cluster, _ := strings.Cut(id, "@")
		out = append(out, LayoutNode{ID: name, Cluster: cluster, Width: 40, Height: 20})
	}
	return out
}

func TestDagreDirections(t *testing.T) {
	t.Parallel()
	// along returns the place of a node along the ranks, which grows from
	// the first rank to the last.
	along := map[string]func(NodePlace) float64{
		TB: func(n NodePlace) float64 { return n.Y },
		BT: func(n NodePlace) float64 { return -n.Y },
		LR: func(n NodePlace) float64 { return n.X },
		RL: func(n NodePlace) float64 { return -n.X },
	}
	for dir, f := range along {
		in := LayoutInput{Direction: dir, Nodes: nodes("a", "b", "c"), Edges: []LayoutEdge{{Source: "a", Target: "b"}, {Source: "b", Target: "c"}}}
		l := layDagre(t, in)
		if !(f(l.Nodes[0]) < f(l.Nodes[1]) && f(l.Nodes[1]) < f(l.Nodes[2])) {
			t.Errorf("%s: expected the ranks a, b, c in the direction, got %v", dir, l.Nodes)
		}
		for i, ep := range l.Edges {
			if len(ep.Points) < 2 || ep.Path == "" {
				t.Errorf("%s: expected a route of the edge %d, got %v", dir, i, ep)
			}
		}
		if len(l.Unsupported) > 0 {
			t.Errorf("%s: expected nothing unsupported, got %q", dir, l.Unsupported)
		}
	}
}

func TestDagreLabels(t *testing.T) {
	t.Parallel()
	in := LayoutInput{
		Nodes: nodes("a", "b"),
		Edges: []LayoutEdge{
			{Source: "a", Target: "b", LabelWidth: 30, LabelHeight: 12, HeadLabelWidth: 10, HeadLabelHeight: 8},
			{Source: "b", Target: "b", LabelWidth: 20, LabelHeight: 10},
		},
	}
	l := layDagre(t, in)
	a, b, ep := l.Nodes[0], l.Nodes[1], l.Edges[0]
	if !(ep.LabelY > a.Y+a.Height/2 && ep.LabelY < b.Y-b.Height/2) {
		t.Errorf("expected the label between the nodes, got %g, from %g to %g", ep.LabelY, a.Y, b.Y)
	}
	if ep.HeadLabelX == 0 && ep.HeadLabelY == 0 {
		t.Error("expected a place for the label at the head")
	}
	if ep.EndY > b.Y-b.Height/2+0.5 {
		t.Errorf("expected the end on the top of b, got %g", ep.EndY)
	}
	loop := l.Edges[1]
	if len(loop.Points) != 4 {
		t.Fatalf("expected the four points of a loop, got %v", loop.Points)
	}
	lb := rectAt([2]float64{loop.LabelX, loop.LabelY}, 20, 10)
	if lb.overlaps(rectAt([2]float64{b.X, b.Y}, b.Width, b.Height)) {
		t.Errorf("expected the label of the loop off its node, got %v", lb)
	}
}

func TestDagreNestedClusters(t *testing.T) {
	t.Parallel()
	// The edge from x to b crosses the borders of both clusters, so
	// neither lays out on its own.
	in := LayoutInput{
		Nodes:    nodes("a@inner", "b@inner", "c@outer", "x"),
		Edges:    []LayoutEdge{{Source: "a", Target: "b"}, {Source: "b", Target: "c"}, {Source: "x", Target: "b"}},
		Clusters: []LayoutCluster{{ID: "outer", TitleWidth: 30, TitleHeight: 12}, {ID: "inner", Parent: "outer", TitleWidth: 30, TitleHeight: 12}},
	}
	l := layDagre(t, in)
	if l.Clusters[0].Depth != 0 || l.Clusters[1].Depth != 1 {
		t.Errorf("expected the depths 0 and 1, got %d and %d", l.Clusters[0].Depth, l.Clusters[1].Depth)
	}
	if l.Clusters[0].Width < 30+2*12 {
		t.Errorf("expected the cluster to fit its title, got the width %g", l.Clusters[0].Width)
	}
}

func TestDagreBoxedCluster(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dir  string
		edge bool
		// row says that a and b lie side by side, and else one above the
		// other.
		row bool
	}{
		// A cluster with no edge across its border lays out on its own,
		// in LR in a graph of TB, as Mermaid does.
		{"alone", "", false, true},
		// Its own direction wins.
		{"own direction", TB, false, false},
		// A cluster with an edge across its border follows the graph.
		{"linked", "", true, false},
	}
	for _, tt := range tests {
		in := LayoutInput{
			Nodes:    nodes("a@c", "b@c", "x"),
			Edges:    []LayoutEdge{{Source: "a", Target: "b"}},
			Clusters: []LayoutCluster{{ID: "c", Direction: tt.dir}},
		}
		if tt.edge {
			in.Edges = append(in.Edges, LayoutEdge{Source: "x", Target: "a"})
		}
		l := layDagre(t, in)
		a, b := l.Nodes[0], l.Nodes[1]
		if row := math.Abs(a.Y-b.Y) < 1 && b.X > a.X; row != tt.row {
			t.Errorf("%s: expected side by side %v, got a at %g, %g and b at %g, %g", tt.name, tt.row, a.X, a.Y, b.X, b.Y)
		}
	}
}

func TestDagreReverseClusters(t *testing.T) {
	t.Parallel()
	// Three sibling clusters, the last of which holds two more, as in a
	// flowchart with the subgraphs one, two and top, which holds b1 and b2.
	// No edge crosses a border, so top lays out on its own, from left to
	// right, and so do b1 and b2.
	in := LayoutInput{
		Nodes: nodes("a@one", "b@two", "c@b1", "d@b2"),
		Clusters: []LayoutCluster{
			{ID: "one"}, {ID: "two"}, {ID: "top"}, {ID: "b1", Parent: "top"}, {ID: "b2", Parent: "top"},
		},
	}
	// order returns the ids of the clusters one, two and top from the left,
	// and of b1 and b2 from the top.
	order := func(l *Layout) string {
		ids := []string{"one", "two", "top", "b1", "b2"}
		outer, inner := slices.Clone(ids[:3]), slices.Clone(ids[3:])
		slices.SortFunc(outer, func(a, b string) int {
			return cmp.Compare(l.Clusters[slices.Index(ids, a)].X, l.Clusters[slices.Index(ids, b)].X)
		})
		slices.SortFunc(inner, func(a, b string) int {
			return cmp.Compare(l.Clusters[slices.Index(ids, a)].Y, l.Clusters[slices.Index(ids, b)].Y)
		})
		return strings.Join(slices.Concat(outer, inner), " ")
	}
	if got, want := order(layDagre(t, in)), "one two top b1 b2"; got != want {
		t.Errorf("expected the clusters in the order %s, got %s", want, got)
	}
	in.ReverseClusters = true
	if got, want := order(layDagre(t, in)), "top two one b2 b1"; got != want {
		t.Errorf("expected the clusters in the order %s, got %s", want, got)
	}
}

func TestDagreEdgeToCluster(t *testing.T) {
	t.Parallel()
	near := func(v, w float64) bool { return math.Abs(v-w) < 0.5 }
	onBox := func(c ClusterPlace, x, y float64) bool {
		inX, inY := x >= c.X-0.5 && x <= c.X+c.Width+0.5, y >= c.Y-0.5 && y <= c.Y+c.Height+0.5
		return inX && inY && (near(x, c.X) || near(x, c.X+c.Width) || near(y, c.Y) || near(y, c.Y+c.Height))
	}
	// The cluster c has an edge across its border, from b to x, so the
	// edge from x to c goes to a node in c and stops at its box.
	linked := LayoutInput{
		Nodes:    nodes("a@c", "b@c", "x", "y"),
		Edges:    []LayoutEdge{{Source: "a", Target: "b"}, {Source: "b", Target: "y"}, {Source: "x", Target: "c"}},
		Clusters: []LayoutCluster{{ID: "c"}},
	}
	l := layDagre(t, linked)
	if ep := l.Edges[2]; !onBox(l.Clusters[0], ep.EndX, ep.EndY) {
		t.Errorf("expected the edge to the linked cluster to stop at its box %v, got %g, %g", l.Clusters[0], ep.EndX, ep.EndY)
	}
	// The cluster c has no other edge, so it lays out on its own, and the
	// edge goes to its box.
	alone := LayoutInput{
		Nodes:    nodes("a@c", "b@c", "x"),
		Edges:    []LayoutEdge{{Source: "a", Target: "b"}, {Source: "x", Target: "c"}},
		Clusters: []LayoutCluster{{ID: "c"}},
	}
	l = layDagre(t, alone)
	if ep := l.Edges[1]; !onBox(l.Clusters[0], ep.EndX, ep.EndY) {
		t.Errorf("expected the edge to the boxed cluster to stop at its box %v, got %g, %g", l.Clusters[0], ep.EndX, ep.EndY)
	}
	if len(l.Unsupported) > 0 {
		t.Errorf("expected nothing unsupported, got %q", l.Unsupported)
	}
}

func TestDagreUnsupported(t *testing.T) {
	t.Parallel()
	in := LayoutInput{
		Lanes: true,
		Nodes: []LayoutNode{
			{ID: "a", Cluster: "c", Rank: "same", Width: 40, Height: 20},
			{ID: "b", Cluster: "c", Rank: "same", Width: 40, Height: 20},
			{ID: "x", Width: 40, Height: 20},
		},
		Edges: []LayoutEdge{
			{Source: "a", Target: "x", SourcePort: []float64{5, 0}},
			{Source: "x", Target: "b", NoConstraint: true},
			{Source: "c", Target: "c"},
		},
		Clusters: []LayoutCluster{{ID: "c"}},
	}
	l := layDagre(t, in)
	want := []string{
		`the group of the same rank "same", which zuzu.dagre leaves out`,
		"the lanes of the layout, which zuzu.dagre leaves out",
		"the port of the edge a -> x, which zuzu.dagre leaves out",
		"constraint false on the edge x -> b, which zuzu.dagre leaves out",
		`the loop on the cluster "c", which zuzu.dagre leaves out`,
	}
	got := slices.Clone(l.Unsupported)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
	if len(l.Edges[2].Points) != 0 {
		t.Errorf("expected no route for the loop on a cluster, got %v", l.Edges[2].Points)
	}
}

func TestDagreErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   LayoutInput
		want string
	}{
		{"twice", LayoutInput{Nodes: nodes("a", "a")}, `the node "a" twice`},
		{"no end", LayoutInput{Nodes: nodes("a"), Edges: []LayoutEdge{{Source: "a", Target: "z"}}}, `"z", which is neither`},
		{"empty cluster", LayoutInput{Nodes: nodes("a"), Edges: []LayoutEdge{{Source: "a", Target: "c"}}, Clusters: []LayoutCluster{{ID: "c"}}}, "holds no node"},
		{"direction", LayoutInput{Nodes: nodes("a@c"), Clusters: []LayoutCluster{{ID: "c", Direction: "up"}}}, `the direction "up"`},
	}
	for _, tt := range tests {
		_, err := Dagre().Lay(tt.in)
		if !errors.Is(err, ErrGraph) || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: expected an error with %q, got %v", tt.name, tt.want, err)
		}
	}
}
