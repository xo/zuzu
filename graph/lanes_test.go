package graph

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
)

func TestCollate(t *testing.T) {
	t.Parallel()
	// The order of localeCompare of JavaScript in the root locale.
	want := []string{"_a", "-a", "1", "10", "2", "a", "A", "a1", "ab", "B", "b", "c"}
	got := slices.Clone(want)
	slices.Reverse(got)
	slices.SortStableFunc(got, collate)
	// b and B differ by case only, so their order is the order of case.
	if !slices.Equal(got, []string{"_a", "-a", "1", "10", "2", "a", "A", "a1", "ab", "b", "B", "c"}) {
		t.Errorf("unexpected order %q", got)
	}
	if collate("abc", "abc") != 0 {
		t.Error("expected equal names to compare equal")
	}
}

// lanesInput builds an input in lanes: each node is id:lane, a lane of ""
// for none, and each edge is a pair of ids.
func lanesInput(dir string, nodes []string, edges ...[2]string) LayoutInput {
	in := LayoutInput{Direction: dir, Lanes: true}
	seen := map[string]bool{}
	for _, n := range nodes {
		id, lane, _ := strings.Cut(n, ":")
		if lane != "" && !seen[lane] {
			seen[lane] = true
			in.Clusters = append(in.Clusters, LayoutCluster{ID: lane, TitleWidth: 30, TitleHeight: 16})
		}
		in.Nodes = append(in.Nodes, LayoutNode{ID: id, Cluster: lane, Width: 60, Height: 30})
	}
	for _, e := range edges {
		in.Edges = append(in.Edges, LayoutEdge{Source: e[0], Target: e[1]})
	}
	return in
}

// ranksOf returns the rank of each node of a layout by its id.
func ranksOf(in LayoutInput, l *Layout) map[string]int {
	out := map[string]int{}
	for i, n := range in.Nodes {
		out[n.ID] = l.Nodes[i].Rank
	}
	return out
}

func TestLaneRanks(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		in    LayoutInput
		ranks map[string]int
	}{
		{
			// The example of the docs of Mermaid, as mmdc ranks it: an edge
			// between two lanes keeps the rank, and an edge that closes a
			// cycle turns around.
			"decisions",
			lanesInput(TB, []string{"collect:Intake", "validate:Intake", "review:Review", "decide:Review", "schedule:Delivery", "complete:Delivery"},
				[2]string{"collect", "validate"}, [2]string{"validate", "review"}, [2]string{"review", "decide"},
				[2]string{"decide", "schedule"}, [2]string{"schedule", "complete"}, [2]string{"decide", "collect"}),
			map[string]int{"collect": 0, "validate": 1, "review": 1, "decide": 2, "schedule": 2, "complete": 3},
		},
		{
			// A lane holds one node in each rank, so the second node with
			// no edge takes the next rank.
			"one node in a rank",
			lanesInput(LR, []string{"a:L", "b:L", "c:M"}, [2]string{"a", "c"}),
			map[string]int{"a": 0, "b": 1, "c": 0},
		},
		{
			// With LaneEdges, each edge moves its target to the next rank,
			// and two nodes of a lane can share a rank.
			"lane edges",
			func() LayoutInput {
				in := lanesInput(LR, []string{"a:One", "b:One", "c:One", "d:Two", "e:Two"},
					[2]string{"a", "b"}, [2]string{"a", "c"}, [2]string{"b", "d"}, [2]string{"c", "d"}, [2]string{"d", "e"})
				in.LaneEdges = true
				return in
			}(),
			map[string]int{"a": 0, "b": 1, "c": 1, "d": 2, "e": 3},
		},
	} {
		l, err := Lay(tt.in)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if got := ranksOf(tt.in, l); fmt.Sprint(got) != fmt.Sprint(tt.ranks) {
			t.Errorf("%s: expected the ranks %v, got %v", tt.name, tt.ranks, got)
		}
	}
}

// checkLaneRules checks a layout in lanes: no two nodes overlap, each
// cluster holds its nodes and the clusters inside it, the lanes lie side by
// side in order along the same ranks, and each route goes at right angles
// and crosses no node but its ends.
func checkLaneRules(t *testing.T, name string, in LayoutInput, l *Layout) {
	t.Helper()
	cidx := map[string]int{}
	for i, c := range in.Clusters {
		cidx[c.ID] = i
	}
	nidx := map[string]int{}
	for i, n := range in.Nodes {
		nidx[n.ID] = i
	}
	box := func(p NodePlace) rect { return rectAt([2]float64{p.X, p.Y}, p.Width, p.Height) }
	holds := func(c ClusterPlace, b rect) bool {
		return b.x1 >= c.X-0.01 && b.y1 >= c.Y-0.01 && b.x2 <= c.X+c.Width+0.01 && b.y2 <= c.Y+c.Height+0.01
	}
	for i, a := range l.Nodes {
		for j := range i {
			if box(a).overlaps(box(l.Nodes[j])) {
				t.Errorf("%s: the nodes %s and %s overlap", name, in.Nodes[i].ID, in.Nodes[j].ID)
			}
		}
		for c := in.Nodes[i].Cluster; c != ""; c = in.Clusters[cidx[c]].Parent {
			if !holds(l.Clusters[cidx[c]], box(a)) {
				t.Errorf("%s: the cluster %s does not hold the node %s", name, c, in.Nodes[i].ID)
			}
		}
	}
	for i, c := range in.Clusters {
		b := l.Clusters[i]
		if c.Parent != "" && b.Width > 0 && !holds(l.Clusters[cidx[c.Parent]], rect{b.X, b.Y, b.X + b.Width, b.Y + b.Height}) {
			t.Errorf("%s: the cluster %s does not hold the cluster %s", name, c.Parent, c.ID)
		}
	}
	var lanes []ClusterPlace
	for i, c := range in.Clusters {
		if c.Parent == "" && l.Clusters[i].Width > 0 {
			lanes = append(lanes, l.Clusters[i])
		}
	}
	sideways := in.Direction == LR || in.Direction == RL
	for k := 1; k < len(lanes); k++ {
		a, b := lanes[k-1], lanes[k]
		ok := a.X+a.Width <= b.X && a.Y == b.Y && a.Height == b.Height
		if sideways {
			ok = a.Y+a.Height <= b.Y && a.X == b.X && a.Width == b.Width
		}
		if !ok {
			t.Errorf("%s: the lane %d does not lie after the lane %d along the same ranks", name, k, k-1)
		}
	}
	for i, e := range l.Edges {
		le := in.Edges[i]
		if _, ok := nidx[le.Target]; ok && len(e.Points) < 2 {
			t.Errorf("%s: the edge %s -> %s has no route", name, le.Source, le.Target)
		}
		for k := 1; k < len(e.Points); k++ {
			p, q := e.Points[k-1], e.Points[k]
			if math.Abs(p[0]-q[0]) > 1e-6 && math.Abs(p[1]-q[1]) > 1e-6 {
				t.Errorf("%s: the edge %s -> %s bends at an angle at %v %v", name, le.Source, le.Target, p, q)
			}
			for j, n := range l.Nodes {
				if in.Nodes[j].ID == le.Source || in.Nodes[j].ID == le.Target {
					continue
				}
				if b := box(n); b.cuts(p, q) {
					t.Errorf("%s: the edge %s -> %s crosses the node %s", name, le.Source, le.Target, in.Nodes[j].ID)
				}
			}
		}
	}
}

func TestLaneRules(t *testing.T) {
	t.Parallel()
	for seed := range uint64(30) {
		for _, dir := range []string{TB, BT, LR, RL} {
			for _, cross := range []bool{false, true} {
				in := randomGraph(seed, 12, 16, true)
				in.Direction, in.Lanes, in.LaneEdges = dir, true, cross
				l, err := Lay(in)
				if err != nil {
					t.Fatal(err)
				}
				checkLaneRules(t, fmt.Sprintf("seed %d %s %v", seed, dir, cross), in, l)
			}
		}
	}
}

func TestLaneLayout(t *testing.T) {
	t.Parallel()
	in := lanesInput(LR, []string{"a:L", "b:L", "c:M", "loose"}, [2]string{"a", "b"}, [2]string{"b", "b"}, [2]string{"a", "M"}, [2]string{"loose", "a"})
	in.Edges[1].LabelWidth, in.Edges[1].LabelHeight = 30, 14
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	checkLaneRules(t, "layout", in, l)
	// A loop goes out of the side of its node and back.
	if p := l.Edges[1].Points; len(p) != 4 {
		t.Errorf("expected a loop of 4 points, got %v", p)
	}
	// An edge to a lane stops at the box of the lane.
	m := l.Clusters[1]
	end := l.Edges[2].Points[len(l.Edges[2].Points)-1]
	if math.Abs(end[1]-m.Y) > 0.01 && math.Abs(end[1]-m.Y-m.Height) > 0.01 && math.Abs(end[0]-m.X) > 0.01 {
		t.Errorf("expected the edge to the lane M to stop at its box %v, got %v", m, end)
	}
	// The node outside every cluster is in a lane before the others, and
	// the lanes run down when the ranks run across.
	if l.Nodes[3].Y >= l.Nodes[0].Y {
		t.Errorf("expected the loose node above the lane L, got %v and %v", l.Nodes[3], l.Nodes[0])
	}
	if _, err := Lay(lanesInput(TB, []string{"a:L"}, [2]string{"a", "x"})); !errors.Is(err, ErrGraph) {
		t.Errorf("expected ErrGraph for an edge to no node, got %v", err)
	}
	if l, err := Lay(LayoutInput{Lanes: true}); err != nil || l.Width != 0 {
		t.Errorf("expected an empty layout, got %v %v", l, err)
	}
}
