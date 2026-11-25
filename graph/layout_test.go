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

func TestLayNestedClusterDirection(t *testing.T) {
	t.Parallel()
	// In a graph LR, the cluster outer lays out TB, and the cluster inner
	// in it, with no direction, lays out LR, as ELK lays out a subgraph of
	// Mermaid with no direction.
	in := LayoutInput{
		Direction: LR,
		Nodes: []LayoutNode{
			{ID: "x", Cluster: "outer", Width: 20, Height: 20},
			{ID: "a", Cluster: "inner", Width: 20, Height: 20}, {ID: "b", Cluster: "inner", Width: 20, Height: 20},
		},
		Edges: []LayoutEdge{{Source: "a", Target: "b"}, {Source: "x", Target: "inner"}},
		Clusters: []LayoutCluster{
			{ID: "outer", Direction: TB, TitleWidth: 10, TitleHeight: 12},
			{ID: "inner", Parent: "outer", TitleWidth: 10, TitleHeight: 12},
		},
	}
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	checkRules(t, "nested", in, l)
	x, a, b := l.Nodes[0], l.Nodes[1], l.Nodes[2]
	if a.Y != b.Y || a.X >= b.X {
		t.Errorf("expected a left of b, got %v and %v", a, b)
	}
	if x.Y >= l.Clusters[1].Y {
		t.Errorf("expected x above inner, got %v and %v", x, l.Clusters[1])
	}
	// An edge from x to a, inside outer, crosses the box of inner, so
	// outer and inner lay out with the graph, LR.
	in.Edges[1].Target = "a"
	if l, err = Lay(in); err != nil {
		t.Fatal(err)
	}
	checkRules(t, "crossed", in, l)
	x, a, b = l.Nodes[0], l.Nodes[1], l.Nodes[2]
	if x.X >= a.X || a.X >= b.X {
		t.Errorf("expected x, a and b from the left to the right, got %v, %v and %v", x, a, b)
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
		Edges: []LayoutEdge{{Source: "a", Target: "b", Weight: new(100.0)}, {Source: "c", Target: "b"}},
	}
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := l.Nodes[0], l.Nodes[2]; math.Abs(a.X-b.X) > 0.01 {
		t.Errorf("a at x %v and b at x %v, want the same x", a.X, b.X)
	}
}

func TestLayZeroWeight(t *testing.T) {
	t.Parallel()
	// c has the neighbors b and x above it. With the weight 1 on the edge
	// from x, c goes between them, as in dot. With the weight 0, that edge
	// does not pull, and c goes under b, as in dot.
	lay := func(bc, xc *float64) *Layout {
		t.Helper()
		in := LayoutInput{
			Nodes: []LayoutNode{{ID: "a", Width: 20, Height: 20}, {ID: "b", Width: 20, Height: 20}, {ID: "c", Width: 20, Height: 20}, {ID: "x", Width: 20, Height: 20}},
			Edges: []LayoutEdge{{Source: "a", Target: "b"}, {Source: "b", Target: "c", Weight: bc}, {Source: "x", Target: "c", Weight: xc}},
		}
		l, err := Lay(in)
		if err != nil {
			t.Fatal(err)
		}
		checkRules(t, TB, in, l)
		return l
	}
	l := lay(nil, new(1.0))
	if b, c, x := l.Nodes[1], l.Nodes[2], l.Nodes[3]; !(c.X > math.Min(b.X, x.X)+0.01 && c.X < math.Max(b.X, x.X)-0.01) {
		t.Errorf("the weight 1: c at x %v, want it between b at %v and x at %v", c.X, b.X, x.X)
	}
	l = lay(nil, new(0.0))
	if b, c := l.Nodes[1], l.Nodes[2]; math.Abs(b.X-c.X) > 0.01 {
		t.Errorf("the weight 0: c at x %v, want it under b at %v", c.X, b.X)
	}
	// When every edge into c weighs 0, c has no median, and the layout
	// still places it.
	lay(new(0.0), new(0.0))
}

// TestLayBalance checks the balance of the ranks of dot: a node whose edges
// in and out weigh the same moves to the rank with the fewest nodes that
// its edges allow, in a graph with no cluster.
func TestLayBalance(t *testing.T) {
	t.Parallel()
	in := LayoutInput{
		Nodes: []LayoutNode{{ID: "a", Width: 20, Height: 20}, {ID: "b", Width: 20, Height: 20}, {ID: "c", Width: 20, Height: 20}, {ID: "x", Width: 20, Height: 20}},
		Edges: []LayoutEdge{{Source: "a", Target: "b"}, {Source: "b", Target: "c"}, {Source: "x", Target: "c", Weight: new(0.0)}},
	}
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	if got := []int{l.Nodes[0].Rank, l.Nodes[1].Rank, l.Nodes[2].Rank, l.Nodes[3].Rank}; !slices.Equal(got, []int{0, 1, 2, 0}) {
		t.Errorf("expected the ranks 0, 1, 2 and 0, as in dot, got %v", got)
	}
	checkRules(t, TB, in, l)
	// A node with a heavier edge out stays where network simplex puts it.
	in.Edges[2].Weight = nil
	if l, err = Lay(in); err != nil {
		t.Fatal(err)
	}
	if r := l.Nodes[3].Rank; r != 1 {
		t.Errorf("expected x in the rank 1, got %d", r)
	}
	// dot does not balance a graph with a cluster.
	in.Edges[2].Weight = new(0.0)
	in.Nodes[0].Cluster = "k"
	in.Clusters = []LayoutCluster{{ID: "k"}}
	if l, err = Lay(in); err != nil {
		t.Fatal(err)
	}
	if r := l.Nodes[3].Rank; r != 1 {
		t.Errorf("expected x in the rank 1 in a graph with a cluster, got %d", r)
	}
	// The balance keeps the minlen of each edge.
	ranks := rankNodes(3, []redge{{u: 0, v: 1, minlen: 2, weight: 1}, {u: 2, v: 1, minlen: 1, weight: 0}}, true)
	if !slices.Equal(ranks, []int{0, 2, 0}) {
		t.Errorf("expected the ranks 0, 2 and 0, got %v", ranks)
	}
}

func TestLayLoopSide(t *testing.T) {
	t.Parallel()
	// A loop goes at the right of its node when the ranks run down or up,
	// and above it when they run across, as in dot. Its label goes beyond
	// the loop on the same side.
	in := LayoutInput{
		Nodes: []LayoutNode{{ID: "a", Width: 40, Height: 20}, {ID: "b", Width: 40, Height: 20}},
		Edges: []LayoutEdge{{Source: "a", Target: "a"}, {Source: "a", Target: "b"}, {Source: "b", Target: "b", LabelWidth: 30, LabelHeight: 14}},
	}
	for _, dir := range []string{TB, BT, LR, RL} {
		in.Direction = dir
		l, err := Lay(in)
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range []int{0, 2} {
			n, e := l.Nodes[k/2], l.Edges[k]
			for _, p := range e.Points[1:3] {
				out := p[0] > n.X+n.Width/2
				if dir == LR || dir == RL {
					out = p[1] < n.Y-n.Height/2
				}
				if !out {
					t.Errorf("%s: the loop of %s has the point %v, beside the node at %v,%v", dir, in.Nodes[k/2].ID, p, n.X, n.Y)
				}
			}
			if k == 2 && (dir == LR || dir == RL) && !(e.LabelY < e.Points[1][1]) {
				t.Errorf("%s: the label of the loop at y %v, want it above the loop at %v", dir, e.LabelY, e.Points[1][1])
			}
		}
		checkRules(t, dir, in, l)
	}
}

// TestLayLoopArrows checks that the arrowhead at each end of a loop lies
// along the curve where the curve meets the node, in each direction, so
// that the arrowhead sits on the line and does not point off it.
func TestLayLoopArrows(t *testing.T) {
	t.Parallel()
	in := LayoutInput{
		Nodes: []LayoutNode{{ID: "a", Width: 40, Height: 20}},
		Edges: []LayoutEdge{{Source: "a", Target: "a"}},
	}
	angle := func(to, from [2]float64) float64 {
		return math.Atan2(to[1]-from[1], to[0]-from[0]) * 180 / math.Pi
	}
	near := func(a, b float64) bool {
		d := math.Mod(math.Abs(a-b), 360)
		return math.Min(d, 360-d) < 1e-6
	}
	for _, dir := range []string{TB, BT, LR, RL} {
		in.Direction = dir
		l, err := Lay(in)
		if err != nil {
			t.Fatal(err)
		}
		e := l.Edges[0]
		p := e.Points
		if want := angle(p[3], p[2]); !near(e.EndAngle, want) {
			t.Errorf("%s: the arrowhead at the end turns to %v, want %v along the curve", dir, e.EndAngle, want)
		}
		if want := angle(p[0], p[1]); !near(e.StartAngle, want) {
			t.Errorf("%s: the arrowhead at the start turns to %v, want %v along the curve", dir, e.StartAngle, want)
		}
	}
}

func TestLayLoopOutline(t *testing.T) {
	t.Parallel()
	// The ends of a loop sit on the outline of an ellipse or a circle, not
	// on the box around it, and on the box of a box.
	in := LayoutInput{
		Nodes: []LayoutNode{
			{ID: "a", Width: 68, Height: 36, Shape: "ellipse"},
			{ID: "b", Width: 40, Height: 40, Shape: "circle"},
			{ID: "c", Width: 60, Height: 30, Shape: "box"},
		},
		Edges: []LayoutEdge{{Source: "a", Target: "a"}, {Source: "b", Target: "b"}, {Source: "c", Target: "c"}, {Source: "a", Target: "b"}, {Source: "b", Target: "c"}},
	}
	for _, dir := range []string{TB, BT, LR, RL} {
		in.Direction = dir
		l, err := Lay(in)
		if err != nil {
			t.Fatal(err)
		}
		for i := range 3 {
			n, e := l.Nodes[i], l.Edges[i]
			for _, p := range [][2]float64{{e.StartX, e.StartY}, {e.EndX, e.EndY}, e.Points[0], e.Points[3]} {
				dx, dy := (p[0]-n.X)/(n.Width/2), (p[1]-n.Y)/(n.Height/2)
				d := math.Hypot(dx, dy)
				if in.Nodes[i].Shape == "box" {
					d = math.Max(math.Abs(dx), math.Abs(dy))
				}
				if math.Abs(d-1) > 0.01 {
					t.Errorf("%s: the loop of %s ends at %v, at %v of the way out to the outline", dir, in.Nodes[i].ID, p, d)
				}
			}
		}
		checkRules(t, dir, in, l)
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

func TestLayParallel(t *testing.T) {
	t.Parallel()
	// Two edges in opposite directions between two nodes, and two in the
	// same direction, run side by side, with ends of their own on the
	// borders of the nodes.
	for _, dir := range []string{TB, LR} {
		for _, back := range []bool{true, false} {
			in := LayoutInput{
				Direction: dir,
				Nodes:     []LayoutNode{{ID: "a", Width: 60, Height: 30}, {ID: "b", Width: 60, Height: 30}},
				Edges:     []LayoutEdge{{Source: "a", Target: "b"}, {Source: "b", Target: "a"}},
			}
			if !back {
				in.Edges[1] = LayoutEdge{Source: "a", Target: "b"}
			}
			l, err := Lay(in)
			if err != nil {
				t.Fatal(err)
			}
			p, q := l.Edges[0].Points, l.Edges[1].Points
			if len(p) != 2 || len(q) != 2 {
				t.Fatalf("%s: expected two straight edges, got %v and %v", dir, p, q)
			}
			// The ends at each node are a gap apart.
			ends := [2][2][2]float64{{p[0], p[1]}, {q[0], q[1]}}
			if back {
				ends[1] = [2][2]float64{q[1], q[0]}
			}
			for k := range 2 {
				if d := math.Hypot(ends[0][k][0]-ends[1][k][0], ends[0][k][1]-ends[1][k][1]); math.Abs(d-parallelGap) > 0.01 {
					t.Errorf("%s, back %v: the ends at node %d are %v apart, expected %v", dir, back, k, d, parallelGap)
				}
			}
			// Each end is on the border of its node.
			for i := range 2 {
				for k := range 2 {
					end, n := ends[i][k], l.Nodes[k]
					onX := math.Abs(math.Abs(end[0]-n.X)-n.Width/2) < 0.01
					onY := math.Abs(math.Abs(end[1]-n.Y)-n.Height/2) < 0.01
					if !onX && !onY {
						t.Errorf("%s, back %v: the end %v of edge %d is not on the border of %v", dir, back, end, i, n)
					}
				}
			}
		}
	}
}

// TestLayLongEdgeClearsNodes checks that a long edge crosses a rank between
// its ends at the place that the layout keeps for it, and not across a node
// of the rank. A B-spline through one point between the ends cuts the
// corner toward the line between them, so the route takes a point at the
// top and at the bottom of the rank too. In dot/31-weights, the edge from a
// to d passed through c.
func TestLayLongEdgeClearsNodes(t *testing.T) {
	t.Parallel()
	w := func(v float64) *float64 { return &v }
	in := LayoutInput{
		Direction: TB,
		Nodes:     []LayoutNode{{ID: "a", Width: 50, Height: 39}, {ID: "b", Width: 50, Height: 39}, {ID: "c", Width: 50, Height: 39}, {ID: "d", Width: 50, Height: 39}},
		Edges: []LayoutEdge{{Source: "a", Target: "b", Weight: w(8)}, {Source: "a", Target: "c"}, {Source: "b", Target: "d"},
			{Source: "c", Target: "d", Weight: w(0)}, {Source: "a", Target: "d"}},
	}
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	c := l.Nodes[2]
	pts := l.Edges[4].Points
	// The B-spline crosses the rank of c where its point in the rank
	// pulls it, between the points at the top and the bottom of the rank.
	var in3 [][2]float64
	for _, p := range pts {
		if p[1] >= c.Y-c.Height/2-1e-9 && p[1] <= c.Y+c.Height/2+1e-9 {
			in3 = append(in3, p)
		}
	}
	if len(in3) != 3 {
		t.Fatalf("the edge has %d points in the rank of c, want 3: %v", len(in3), pts)
	}
	for _, p := range in3 {
		if p[0] != in3[0][0] {
			t.Errorf("the points of the edge in the rank of c are %v, want one x", in3)
		}
		if math.Abs(p[0]-c.X) <= c.Width/2 {
			t.Errorf("the edge crosses the rank of c at x %v, inside c from %v to %v", p[0], c.X-c.Width/2, c.X+c.Width/2)
		}
	}
}

// TestLayStepEdges checks that an edge with a curve of steps between two
// ranks turns at a bend halfway between them, level with the center of its
// target, so that it comes straight into the middle of the side of its
// target and its arrowhead points along it, as in diagrams/17-init.
func TestLayStepEdges(t *testing.T) {
	t.Parallel()
	for _, dir := range []string{TB, LR} {
		in := LayoutInput{
			Direction: dir,
			Nodes:     []LayoutNode{{ID: "a", Width: 60, Height: 30}, {ID: "b", Width: 60, Height: 30}, {ID: "c", Width: 60, Height: 30}},
			Edges:     []LayoutEdge{{Source: "a", Target: "b", Curve: "stepBefore"}, {Source: "a", Target: "c", Curve: "stepBefore"}},
		}
		l, err := Lay(in)
		if err != nil {
			t.Fatal(err)
		}
		for k, e := range l.Edges {
			n := l.Nodes[k+1]
			last := e.Points[len(e.Points)-1]
			if len(e.Points) != 3 {
				t.Errorf("%s: the edge to %s has %d points, want 3", dir, in.Nodes[k+1].ID, len(e.Points))
				continue
			}
			want, at := 90.0, last[0]-n.X
			if dir == LR {
				want, at = 0, last[1]-n.Y
			}
			if math.Abs(at) > 1e-6 || math.Abs(e.EndAngle-want) > 1e-6 {
				t.Errorf("%s: the edge to %s ends %v off the middle at the angle %v, want 0 off at %v", dir, in.Nodes[k+1].ID, at, e.EndAngle, want)
			}
		}
	}
}

// TestLayClusterTitleRoom checks that the box of a cluster holds its title
// and its padding in each direction, and that in LR the title of a
// cluster, which lies outside its left border across the ranks, does not
// cover the cluster or the node before it.
func TestLayClusterTitleRoom(t *testing.T) {
	t.Parallel()
	const titleW, pad = 300.0, 12.0
	for _, dir := range []string{TB, BT, LR, RL} {
		in := LayoutInput{
			Direction: dir,
			Nodes:     []LayoutNode{{ID: "b", Width: 40, Height: 20}, {ID: "a", Cluster: "c", Width: 40, Height: 20}},
			Edges:     []LayoutEdge{{Source: "b", Target: "a"}},
			Clusters:  []LayoutCluster{{ID: "c", TitleWidth: titleW, TitleHeight: 14}},
		}
		l, err := Lay(in)
		if err != nil {
			t.Fatal(err)
		}
		checkRules(t, dir, in, l)
		if c := l.Clusters[0]; c.Width < titleW+2*pad-0.01 {
			t.Errorf("%s: the box is %g wide, want at least %g for the title", dir, c.Width, titleW+2*pad)
		}
	}
	// Two clusters side by side in LR, and a node beside a nested cluster,
	// keep a gap.
	in := LayoutInput{
		Direction: LR,
		Nodes: []LayoutNode{
			{ID: "a", Cluster: "one", Width: 40, Height: 20}, {ID: "b", Cluster: "one", Width: 40, Height: 20},
			{ID: "c", Cluster: "two", Width: 40, Height: 20}, {ID: "d", Cluster: "two", Width: 40, Height: 20},
		},
		Edges:    []LayoutEdge{{Source: "a", Target: "b"}, {Source: "c", Target: "d"}},
		Clusters: []LayoutCluster{{ID: "one", TitleWidth: 20, TitleHeight: 14}, {ID: "two", TitleWidth: 20, TitleHeight: 14}},
	}
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	one, two := l.Clusters[0], l.Clusters[1]
	if one.Y > two.Y {
		one, two = two, one
	}
	if one.Y+one.Height > two.Y {
		t.Errorf("the boxes overlap: %v and %v", one, two)
	}
}
