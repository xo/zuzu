package graph

import (
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"slices"
	"strconv"
	"testing"
)

// randomGraph makes a graph of n nodes and m edges, in clusters, from a
// seed.
func randomGraph(seed uint64, n, m int, clusters bool) LayoutInput {
	r := rand.New(rand.NewPCG(seed, 1))
	in := LayoutInput{}
	if clusters {
		in.Clusters = []LayoutCluster{{ID: "c0", TitleWidth: 20, TitleHeight: 12}, {ID: "c1", Parent: "c0", TitleWidth: 20, TitleHeight: 12}, {ID: "c2", TitleWidth: 20, TitleHeight: 12}}
	}
	for i := range n {
		c := ""
		if clusters {
			c = []string{"", "c0", "c1", "c2"}[r.IntN(4)]
		}
		in.Nodes = append(in.Nodes, LayoutNode{ID: strconv.Itoa(i), Cluster: c, Width: 20 + float64(r.IntN(40)), Height: 20 + float64(r.IntN(20))})
	}
	for range m {
		a, b := r.IntN(n), r.IntN(n)
		e := LayoutEdge{Source: strconv.Itoa(a), Target: strconv.Itoa(b), MinLen: 1 + r.IntN(2)}
		if r.IntN(4) == 0 {
			e.LabelWidth, e.LabelHeight = 30, 12
		}
		in.Edges = append(in.Edges, e)
	}
	return in
}

// checkRules checks the hard rules of D31 on a layout: no two nodes
// overlap, and each cluster holds its nodes and the clusters inside it.
func checkRules(t *testing.T, name string, in LayoutInput, l *Layout) {
	t.Helper()
	for i, a := range l.Nodes {
		for j := range i {
			b := l.Nodes[j]
			if math.Abs(a.X-b.X) < (a.Width+b.Width)/2-0.01 && math.Abs(a.Y-b.Y) < (a.Height+b.Height)/2-0.01 {
				t.Errorf("%s: the nodes %s and %s overlap", name, in.Nodes[i].ID, in.Nodes[j].ID)
			}
		}
	}
	cidx := map[string]int{}
	for i, c := range in.Clusters {
		cidx[c.ID] = i
	}
	inside := func(c ClusterPlace, x1, y1, x2, y2 float64) bool {
		return x1 >= c.X-0.01 && y1 >= c.Y-0.01 && x2 <= c.X+c.Width+0.01 && y2 <= c.Y+c.Height+0.01
	}
	for i, n := range in.Nodes {
		if n.Cluster == "" {
			continue
		}
		p := l.Nodes[i]
		for c := n.Cluster; c != ""; c = in.Clusters[cidx[c]].Parent {
			if !inside(l.Clusters[cidx[c]], p.X-p.Width/2, p.Y-p.Height/2, p.X+p.Width/2, p.Y+p.Height/2) {
				t.Errorf("%s: the cluster %s does not hold the node %s", name, c, n.ID)
			}
		}
	}
	for i, c := range in.Clusters {
		// A cluster that holds no node has no box.
		if c.Parent == "" || l.Clusters[i].Width == 0 {
			continue
		}
		b := l.Clusters[i]
		if !inside(l.Clusters[cidx[c.Parent]], b.X, b.Y, b.X+b.Width, b.Y+b.Height) {
			t.Errorf("%s: the cluster %s does not hold the cluster %s", name, c.Parent, c.ID)
		}
	}
	// No node of another cluster lies in a cluster.
	for i, c := range in.Clusters {
		b := l.Clusters[i]
		for j, n := range in.Nodes {
			member := false
			for x := n.Cluster; x != ""; x = in.Clusters[cidx[x]].Parent {
				if x == c.ID {
					member = true
				}
			}
			p := l.Nodes[j]
			if !member && p.X > b.X && p.X < b.X+b.Width && p.Y > b.Y && p.Y < b.Y+b.Height {
				t.Errorf("%s: the node %s lies in the cluster %s", name, n.ID, c.ID)
			}
		}
	}
}

func TestLayRules(t *testing.T) {
	t.Parallel()
	for seed := range uint64(40) {
		for _, dir := range []string{TB, LR} {
			in := randomGraph(seed, 4+int(seed%20), 3+int(seed%30), seed%2 == 0)
			in.Direction = dir
			l, err := Lay(in)
			if err != nil {
				t.Fatal(err)
			}
			name := fmt.Sprintf("seed %d %s", seed, dir)
			checkRules(t, name, in, l)
			// The edges that keep their direction go down the ranks by at
			// least their minlen, when no edge reverses them.
			again, err := Lay(in)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(l, again) {
				t.Errorf("%s: two layouts differ", name)
			}
			// The rules hold too when the clusters have directions of their
			// own and edges end at clusters.
			if len(in.Clusters) == 0 {
				continue
			}
			for i := range in.Clusters {
				in.Clusters[i].Direction = []string{LR, BT, RL}[(int(seed)+i)%3]
			}
			in.Edges = append(in.Edges, LayoutEdge{Source: in.Nodes[0].ID, Target: "c1"}, LayoutEdge{Source: "c2", Target: in.Nodes[len(in.Nodes)-1].ID})
			holds := map[string]bool{}
			for _, n := range in.Nodes {
				holds[n.Cluster] = true
			}
			if !holds["c1"] || !holds["c2"] {
				continue
			}
			l, err = Lay(in)
			if err != nil {
				t.Fatal(err)
			}
			checkRules(t, name+" with directions", in, l)
		}
	}
}

func TestLayClusterDirection(t *testing.T) {
	t.Parallel()
	in := LayoutInput{
		Nodes: []LayoutNode{
			{ID: "x", Width: 20, Height: 20},
			{ID: "a", Cluster: "c", Width: 20, Height: 20}, {ID: "b", Cluster: "c", Width: 20, Height: 20},
		},
		Edges:    []LayoutEdge{{Source: "a", Target: "b"}, {Source: "x", Target: "c"}},
		Clusters: []LayoutCluster{{ID: "c", Direction: LR, TitleWidth: 10, TitleHeight: 12}},
	}
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	checkRules(t, "kept", in, l)
	x, a, b, c := l.Nodes[0], l.Nodes[1], l.Nodes[2], l.Clusters[0]
	if a.Y != b.Y || a.X >= b.X {
		t.Errorf("expected a left of b, got %v and %v", a, b)
	}
	if x.Y >= c.Y {
		t.Errorf("expected x above the cluster, got %v and %v", x, c)
	}
	// The edge to the cluster ends at the top of its box.
	if e := l.Edges[1]; math.Abs(e.EndY-c.Y) > 0.01 || math.Abs(e.EndAngle-90) > 0.01 {
		t.Errorf("expected the edge to end at the top of %v, going down, got %v at %v", c, e.EndY, e.EndAngle)
	}
	// An edge from x to a crosses the cluster, which then follows the
	// graph, and the edge to the cluster stops at its box.
	in.Edges = append(in.Edges, LayoutEdge{Source: "x", Target: "a"})
	if l, err = Lay(in); err != nil {
		t.Fatal(err)
	}
	checkRules(t, "followed", in, l)
	a, b, c = l.Nodes[1], l.Nodes[2], l.Clusters[0]
	if a.Y >= b.Y {
		t.Errorf("expected a above b, got %v and %v", a, b)
	}
	if e := l.Edges[1]; math.Abs(e.EndY-c.Y) > 0.01 {
		t.Errorf("expected the edge to stop at the top of %v, got %v", c, e.EndY)
	}
	g := &Graph{
		Direction: TB,
		Nodes:     []*Node{{ID: "x"}, {ID: "a", Cluster: "c"}, {ID: "b", Cluster: "c"}},
		Edges:     []*Edge{{Source: "x", Target: "a"}, {Source: "a", Target: "b"}},
		Clusters:  []*Cluster{{ID: "c", Direction: LR}},
	}
	if got := FollowedDirections(g); !reflect.DeepEqual(got, map[string]string{"c": TB}) {
		t.Errorf("expected the cluster c to follow TB, got %v", got)
	}
	g.Edges[0].Target = "c"
	if got := FollowedDirections(g); len(got) != 0 {
		t.Errorf("expected the cluster c to keep LR, got %v", got)
	}
}

func TestLayRanks(t *testing.T) {
	t.Parallel()
	in := LayoutInput{
		Nodes: []LayoutNode{{ID: "a", Width: 10, Height: 10}, {ID: "b", Width: 10, Height: 10}, {ID: "c", Width: 10, Height: 10}, {ID: "d", Width: 10, Height: 10, Rank: "same"}, {ID: "e", Width: 10, Height: 10, Rank: "same"}},
		Edges: []LayoutEdge{{Source: "a", Target: "b"}, {Source: "b", Target: "c", MinLen: 2}, {Source: "a", Target: "d"}, {Source: "c", Target: "e"}},
	}
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	r := func(i int) int { return l.Nodes[i].Rank }
	if r(1)-r(0) < 1 || r(2)-r(1) < 2 {
		t.Errorf("the ranks %d %d %d do not keep the minlen", r(0), r(1), r(2))
	}
	if r(3) != r(4) {
		t.Errorf("the nodes of one rank group have the ranks %d and %d", r(3), r(4))
	}
}

func TestLayCycleSource(t *testing.T) {
	t.Parallel()
	// The search from s turns c -> a around, so that s, the only node with
	// no edge in, keeps the first rank, as in dot.
	in := LayoutInput{}
	for _, id := range []string{"s", "a", "b", "c", "d"} {
		in.Nodes = append(in.Nodes, LayoutNode{ID: id, Width: 20, Height: 20})
	}
	for _, e := range [][2]string{{"s", "a"}, {"a", "b"}, {"b", "c"}, {"c", "a"}, {"d", "a"}, {"c", "d"}} {
		in.Edges = append(in.Edges, LayoutEdge{Source: e[0], Target: e[1]})
	}
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{0, 1, 2, 3, 4} {
		if got := l.Nodes[i].Rank; got != want {
			t.Errorf("the node %s has the rank %d, want %d", in.Nodes[i].ID, got, want)
		}
	}
}

func TestLayFlatOrder(t *testing.T) {
	t.Parallel()
	// Four columns, each a chain from a node of the first rank. The edges in
	// the first rank give the order of the columns, against the order of
	// the input.
	in := LayoutInput{}
	cols := []string{"w", "x", "y", "z"}
	for _, c := range cols {
		in.Nodes = append(in.Nodes, LayoutNode{ID: c, Width: 20, Height: 20, Rank: "top"})
		for r := 1; r <= 3; r++ {
			in.Nodes = append(in.Nodes, LayoutNode{ID: c + strconv.Itoa(r), Width: 20, Height: 20})
			in.Edges = append(in.Edges, LayoutEdge{Source: c + strconv.Itoa(r-1), Target: c + strconv.Itoa(r)})
		}
		in.Edges[len(in.Edges)-3].Source = c
	}
	for _, e := range [][2]string{{"z", "y"}, {"y", "x"}, {"x", "w"}} {
		in.Edges = append(in.Edges, LayoutEdge{Source: e[0], Target: e[1]})
	}
	ids := map[string]int{}
	for i, n := range in.Nodes {
		ids[n.ID] = i
	}
	for _, dir := range []string{TB, LR} {
		in.Direction = dir
		l, err := Lay(in)
		if err != nil {
			t.Fatal(err)
		}
		for r := range 4 {
			var order []int
			for _, id := range slices.Backward(cols) {
				if r > 0 {
					id += strconv.Itoa(r)
				}
				order = append(order, l.Nodes[ids[id]].Order)
			}
			if !slices.IsSorted(order) {
				t.Errorf("%s: the rank %d has the orders %v of z, y, x and w", dir, r, order)
			}
		}
		checkRules(t, dir, in, l)
	}
}

func TestLayWeight(t *testing.T) {
	t.Parallel()
	// b has two neighbors above it. The heavy edge from a stays straight.
	in := LayoutInput{
		Nodes: []LayoutNode{{ID: "a", Width: 20, Height: 20}, {ID: "c", Width: 20, Height: 20}, {ID: "b", Width: 20, Height: 20}},
		Edges: []LayoutEdge{{Source: "a", Target: "b", Weight: 100}, {Source: "c", Target: "b"}},
	}
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := l.Nodes[0], l.Nodes[2]; math.Abs(a.X-b.X) > 0.01 {
		t.Errorf("a at x %v and b at x %v, want the same x", a.X, b.X)
	}
}

func TestLayLoopRoom(t *testing.T) {
	t.Parallel()
	// The loop of a and its label take room at the right of a, so the label
	// of the long edge from s beside it does not cover them.
	in := LayoutInput{
		Nodes: []LayoutNode{{ID: "s", Width: 20, Height: 20}, {ID: "a", Width: 20, Height: 20}, {ID: "b", Width: 20, Height: 20}},
		Edges: []LayoutEdge{
			{Source: "s", Target: "a"}, {Source: "a", Target: "b"}, {Source: "s", Target: "b", LabelWidth: 40, LabelHeight: 14},
			{Source: "a", Target: "a", LabelWidth: 40, LabelHeight: 14},
		},
	}
	for _, dir := range []string{TB, LR} {
		in.Direction = dir
		l, err := Lay(in)
		if err != nil {
			t.Fatal(err)
		}
		loop, long := l.Edges[3], l.Edges[2]
		if math.Abs(loop.LabelX-long.LabelX) < 40-0.01 && math.Abs(loop.LabelY-long.LabelY) < 14-0.01 {
			t.Errorf("%s: the labels at %v,%v and %v,%v overlap", dir, loop.LabelX, loop.LabelY, long.LabelX, long.LabelY)
		}
	}
}

func TestLayNoCrossings(t *testing.T) {
	t.Parallel()
	// Two chains that a first order crosses.
	in := LayoutInput{
		Nodes: []LayoutNode{{ID: "a", Width: 20, Height: 20}, {ID: "b", Width: 20, Height: 20}, {ID: "c", Width: 20, Height: 20}, {ID: "d", Width: 20, Height: 20}, {ID: "e", Width: 20, Height: 20}, {ID: "f", Width: 20, Height: 20}},
		Edges: []LayoutEdge{{Source: "a", Target: "d"}, {Source: "b", Target: "c"}, {Source: "c", Target: "e"}, {Source: "d", Target: "f"}, {Source: "a", Target: "c"}},
	}
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	if n := Crossings(l); n != 0 {
		t.Errorf("%d crossings, want 0", n)
	}
}

func TestLayErrors(t *testing.T) {
	t.Parallel()
	for _, in := range []LayoutInput{
		{Nodes: []LayoutNode{{ID: "a"}, {ID: "a"}}},
		{Nodes: []LayoutNode{{ID: "a"}}, Edges: []LayoutEdge{{Source: "a", Target: "b"}}},
		{Nodes: []LayoutNode{{ID: "a", Cluster: "c"}}, Clusters: []LayoutCluster{{ID: "c", Direction: "XY"}}},
		{Nodes: []LayoutNode{{ID: "a"}}, Edges: []LayoutEdge{{Source: "a", Target: "c"}}, Clusters: []LayoutCluster{{ID: "c"}}},
	} {
		if _, err := Lay(in); err == nil {
			t.Errorf("%v lays out", in)
		}
	}
}

func TestLaySides(t *testing.T) {
	t.Parallel()
	// An edge from the east side of a to the west side of b, in ranks
	// that run down, leaves a to the right and comes into b from the left.
	in := LayoutInput{
		Nodes: []LayoutNode{{ID: "a", Width: 40, Height: 20}, {ID: "b", Width: 40, Height: 20}},
		Edges: []LayoutEdge{{
			Source: "a", Target: "b",
			SourcePort: []float64{20, 0}, SourceSide: []float64{1, 0},
			TargetPort: []float64{-20, 0}, TargetSide: []float64{-1, 0},
		}},
	}
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	a, b, pts := l.Nodes[0], l.Nodes[1], l.Edges[0].Points
	first, second := pts[0], pts[1]
	last, before := pts[len(pts)-1], pts[len(pts)-2]
	if first != [2]float64{a.X + 20, a.Y} || second[0] <= first[0] || second[1] != first[1] {
		t.Errorf("the start %v %v does not leave the east side of a at %v", first, second, a)
	}
	if last != [2]float64{b.X - 20, b.Y} || before[0] >= last[0] || before[1] != last[1] {
		t.Errorf("the end %v %v does not come into the west side of b at %v", before, last, b)
	}
	if math.Abs(l.Edges[0].EndAngle) > 1e-9 {
		t.Errorf("expected the arrowhead to point east, got %v", l.Edges[0].EndAngle)
	}
}
