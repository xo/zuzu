// SPDX-License-Identifier: EPL-2.0

package adapter

import (
	"slices"
	"testing"
)

// treeOf returns the tree of a graph and the index of each node.
func treeOf(g *Graph) (tree, map[string]int) {
	byID := map[string]int{}
	for i := range g.Nodes {
		byID[g.Nodes[i].ID] = i
	}
	return newTree(g, byID), byID
}

// endsOf returns the ends of the edges of a graph.
func endsOf(g *Graph, byID map[string]int) []ends {
	var es []ends
	for _, e := range g.Edges {
		es = append(es, ends{byID[e.From], byID[e.To]})
	}
	return es
}

// TestFeedbackEdges checks the edges that close a cycle only through a
// cluster, as the tests of subgraphFeedbackEdges.ts of Mermaid do: A -> B
// inside G, then B -> X -> C with C inside G. X -> C goes to ELK reversed.
// A cycle between plain nodes stays for ELK.
func TestFeedbackEdges(t *testing.T) {
	t.Parallel()
	g := &Graph{
		Nodes: []Node{
			{ID: "G", Cluster: true}, in("G", "A", 10, 10), in("G", "B", 10, 10), in("G", "C", 10, 10),
			box("X", 10, 10), box("P", 10, 10), box("Q", 10, 10),
		},
		Edges: []Edge{
			edge("ab", "A", "B"), edge("bx", "B", "X"), edge("xc", "X", "C"),
			edge("pq", "P", "Q"), edge("qp", "Q", "P"), edge("gc", "G", "C"),
		},
	}
	tr, byID := treeOf(g)
	got := feedbackEdges(endsOf(g, byID), tr)
	if want := []bool{false, false, true, false, false, false}; !slices.Equal(got, want) {
		t.Errorf("expected the feedback edges %v, got %v", want, got)
	}
	// The layout routes the reversed edge from X to C, and downstream.
	r, err := Layout(g)
	if err != nil {
		t.Fatal(err)
	}
	pts := r.Edges[2].Points
	x, c := r.Nodes[byID["X"]], r.Nodes[byID["C"]]
	if len(pts) < 2 || pts[0].Y < x.Y || pts[len(pts)-1].Y > c.Y+c.Height {
		t.Errorf("expected the route of X -> C from X to C, got %v, X at %v, C at %v", pts, x.Rect, c.Rect)
	}
	g.Config.KeepFeedbackEdges = true
	b, err := build(g, false)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(b.reversed, true) {
		t.Errorf("expected no edge reversed with KeepFeedbackEdges, got %v", b.reversed)
	}
}

// TestNestedOrder checks the order of the edges by nesting, as the tests
// of nestedEdgeOrder.ts of Mermaid do: outer -> a, then da -> a with da
// inside outer, and a -> outer, then a -> da.
func TestNestedOrder(t *testing.T) {
	t.Parallel()
	g := &Graph{
		Nodes: []Node{{ID: "outer", Cluster: true}, in("outer", "da", 10, 10), box("a", 10, 10), box("b", 10, 10)},
		Edges: []Edge{
			edge("e0", "outer", "a"), edge("e1", "b", "a"), edge("e2", "da", "a"),
			edge("e3", "b", "outer"), edge("e4", "b", "da"),
		},
	}
	tr, byID := treeOf(g)
	if got, want := nestedOrder(endsOf(g, byID), tr), []int{2, 0, 1, 4, 3}; !slices.Equal(got, want) {
		t.Errorf("expected the order %v, got %v", want, got)
	}
	b, err := build(g, false)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, e := range b.root.Edges {
		ids = append(ids, e.ID.String())
	}
	if want := []string{"e2", "e0", "e1", "e4", "e3"}; !slices.Equal(ids, want) {
		t.Errorf("expected ELK to take the edges %v, got %v", want, ids)
	}
}

// TestCyclicEntries checks the entry of a cycle that keepEntryNodeOnTop
// pins to the first layer: the first node that no edge enters once the
// edges that close a cycle are left out. A group with a source, and a
// cycle inside a cluster that an edge enters, nominate nothing.
func TestCyclicEntries(t *testing.T) {
	t.Parallel()
	g := &Graph{
		Nodes: []Node{
			box("a", 10, 10), box("b", 10, 10), box("c", 10, 10), box("s", 10, 10), box("t", 10, 10),
			{ID: "K", Cluster: true}, in("K", "k1", 10, 10), in("K", "k2", 10, 10),
		},
		Edges: []Edge{
			// c -> a enters a first, and b -> c -> a closes the cycle
			// back to b.
			edge("ca", "c", "a"), edge("ab", "a", "b"), edge("bc", "b", "c"),
			edge("st", "s", "t"), edge("k1k2", "k1", "k2"), edge("k2k1", "k2", "k1"),
		},
	}
	tr, byID := treeOf(g)
	got := cyclicEntries(len(g.Nodes), endsOf(g, byID), tr)
	var ids []string
	for _, i := range got {
		ids = append(ids, g.Nodes[i].ID)
	}
	if want := []string{"c", "k1"}; !slices.Equal(ids, want) {
		t.Errorf("expected the entries %v, got %v", want, ids)
	}
	g.Config.KeepEntryNodeOnTop = true
	b, err := build(g, false)
	if err != nil {
		t.Fatal(err)
	}
	for i, n := range b.nodes {
		_, pinned := n.LayoutOptions["elk.layered.layering.layerConstraint"]
		if want := slices.Contains(got, i); pinned != want {
			t.Errorf("the node %s is pinned %t, want %t", g.Nodes[i].ID, pinned, want)
		}
	}
	g.Config.KeepEntryNodeOnTop = false
	if b, err = build(g, false); err != nil {
		t.Fatal(err)
	}
	for _, n := range b.nodes {
		if _, ok := n.LayoutOptions["elk.layered.layering.layerConstraint"]; ok {
			t.Errorf("expected no node pinned without KeepEntryNodeOnTop, got %s", n.ID.String())
		}
	}
}

// TestEndLabelsKeepRoomForMarkers checks that a label at an end with a
// marker keeps 6 pixels around it in ELK, and that the result gives the
// box of the label without that room.
func TestEndLabelsKeepRoomForMarkers(t *testing.T) {
	t.Parallel()
	g := &Graph{
		Nodes: []Node{box("a", 60, 40), box("b", 60, 40)},
		Edges: []Edge{{
			ID: "e", From: "a", To: "b", HeadMarker: true,
			Tail: []Label{{Text: "1", Width: 10, Height: 12}}, Head: []Label{{Text: "n", Width: 10, Height: 12}},
		}},
	}
	b, err := build(g, false)
	if err != nil {
		t.Fatal(err)
	}
	labels := b.edges[0].Labels
	if w, h := *labels[1].Width, *labels[2].Width; w != 10 || h != 22 {
		t.Errorf("expected the tail 10 wide and the head 22 wide, got %g and %g", w, h)
	}
	r, err := Layout(g)
	if err != nil {
		t.Fatal(err)
	}
	if h := r.Edges[0].Head[0]; h == nil || h.Width != 10 || h.Height != 12 {
		t.Errorf("expected the head label 10 by 12, got %+v", h)
	}
}

// TestPortsWithAPlace checks that a port with a place keeps it, and that
// the edge ends at it.
func TestPortsWithAPlace(t *testing.T) {
	t.Parallel()
	g := &Graph{
		Nodes: []Node{
			{ID: "a", Width: 100, Height: 40, Ports: []Port{{ID: "ap", Side: South, At: &Point{X: 80, Y: 40}}}},
			box("b", 60, 40),
		},
		Edges: []Edge{{ID: "e", From: "a", To: "b", FromPort: "ap"}},
	}
	r, err := Layout(g)
	if err != nil {
		t.Fatal(err)
	}
	a, p := r.Nodes[0], r.Nodes[0].Ports[0]
	if p.X-a.X != 80 || p.Y-a.Y != 40 {
		t.Errorf("expected the port at 80, 40 of its node, got %g, %g", p.X-a.X, p.Y-a.Y)
	}
	if start := r.Edges[0].Points[0]; start.X != p.X || start.Y != p.Y {
		t.Errorf("expected the edge to start at the port %v, got %v", p, start)
	}
}
