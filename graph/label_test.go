package graph

import (
	"math"
	"testing"
)

// labelBox returns the box of a label of a size at a center.
func labelBox(x, y, w, h float64) rect {
	return rectAt([2]float64{x, y}, w, h)
}

func TestEndLabels(t *testing.T) {
	t.Parallel()
	in := LayoutInput{
		Nodes: []LayoutNode{{ID: "a", Width: 60, Height: 30}, {ID: "b", Width: 60, Height: 30}},
		Edges: []LayoutEdge{{Source: "a", Target: "b", HeadLabelWidth: 20, HeadLabelHeight: 14, TailLabelWidth: 10, TailLabelHeight: 14}},
	}
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	a, b, e := l.Nodes[0], l.Nodes[1], l.Edges[0]
	head, tail := labelBox(e.HeadLabelX, e.HeadLabelY, 20, 14), labelBox(e.TailLabelX, e.TailLabelY, 10, 14)
	// The edge runs down, so the labels sit at its right, which is the
	// left of the page, past the ends.
	if head.x2 > e.EndX-endBeside+0.01 || head.y2 > e.EndY-endAlong+0.01 {
		t.Errorf("the head label %v is not left of and above the end %v, %v", head, e.EndX, e.EndY)
	}
	if tail.x2 > e.StartX-endBeside+0.01 || tail.y1 < e.StartY+endAlong-0.01 {
		t.Errorf("the tail label %v is not left of and below the start %v, %v", tail, e.StartX, e.StartY)
	}
	for _, n := range []NodePlace{a, b} {
		box := labelBox(n.X, n.Y, n.Width, n.Height)
		if box.overlaps(head) || box.overlaps(tail) {
			t.Errorf("a label covers a node: %v %v %v", box, head, tail)
		}
	}
	if head.overlaps(tail) {
		t.Errorf("the labels overlap: %v %v", head, tail)
	}
	// The rank between the ends holds both labels.
	if gap := (b.Y - b.Height/2) - (a.Y + a.Height/2); gap < 14+14+4*endAlong {
		t.Errorf("the space between the ranks is %v", gap)
	}
	for _, v := range []float64{head.x1, head.y1, tail.x1, tail.y1} {
		if v < -0.01 {
			t.Errorf("a label is outside the layout: %v %v", head, tail)
		}
	}
}

func TestFixedEndLabels(t *testing.T) {
	t.Parallel()
	in := LayoutInput{
		Direction: LR,
		Nodes:     []LayoutNode{{ID: "a", Width: 60, Height: 30}, {ID: "b", Width: 60, Height: 30}},
		Edges:     []LayoutEdge{{Source: "a", Target: "b", HeadLabelWidth: 10, HeadLabelHeight: 10, TailLabelWidth: 10, TailLabelHeight: 10, LabelDistance: 2, LabelAngle: 90}},
	}
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	e := l.Edges[0]
	// Into the edge at the head is to the left, and 90 degrees
	// counterclockwise from it is down. At the tail it is up.
	near := func(x, y, wx, wy float64) bool { return math.Abs(x-wx) < 0.01 && math.Abs(y-wy) < 0.01 }
	if !near(e.HeadLabelX, e.HeadLabelY, e.EndX, e.EndY+20) {
		t.Errorf("the head label at %v, %v, the end at %v, %v", e.HeadLabelX, e.HeadLabelY, e.EndX, e.EndY)
	}
	if !near(e.TailLabelX, e.TailLabelY, e.StartX, e.StartY-20) {
		t.Errorf("the tail label at %v, %v, the start at %v, %v", e.TailLabelX, e.TailLabelY, e.StartX, e.StartY)
	}
}

func TestXLabels(t *testing.T) {
	t.Parallel()
	in := LayoutInput{
		Nodes: []LayoutNode{{ID: "a", Width: 60, Height: 30, XLabelWidth: 30, XLabelHeight: 14}, {ID: "b", Width: 60, Height: 30}},
		Edges: []LayoutEdge{{Source: "a", Target: "b", XLabelWidth: 24, XLabelHeight: 14}},
	}
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	a, e := l.Nodes[0], l.Edges[0]
	// The xlabel of a node sits at its top left corner.
	if math.Abs(a.XLabelX-(a.X-a.Width/2-15)) > 0.01 || math.Abs(a.XLabelY-(a.Y-a.Height/2-7)) > 0.01 {
		t.Errorf("the xlabel of a at %v, %v, the node at %+v", a.XLabelX, a.XLabelY, a)
	}
	if a.XLabelX-15 < -0.01 || a.XLabelY-7 < -0.01 {
		t.Errorf("the xlabel of a is outside the layout: %v, %v", a.XLabelX, a.XLabelY)
	}
	// The xlabel of an edge sits beside its middle, at its right, and
	// takes no rank.
	mid := (e.StartY + e.EndY) / 2
	if math.Abs(e.XLabelY-mid) > 0.01 || math.Abs(e.XLabelX-(e.StartX-xGap-12)) > 0.01 {
		t.Errorf("the xlabel of the edge at %v, %v, the edge from %v, %v to %v, %v", e.XLabelX, e.XLabelY, e.StartX, e.StartY, e.EndX, e.EndY)
	}
	if l.Nodes[1].Rank-a.Rank != 1 {
		t.Errorf("the edge spans %d ranks", l.Nodes[1].Rank-a.Rank)
	}
}

func TestXLabelAvoidsNodes(t *testing.T) {
	t.Parallel()
	// A node at the left of a takes the place of its first corner.
	in := LayoutInput{
		Nodes: []LayoutNode{{ID: "a", Width: 40, Height: 30, XLabelWidth: 30, XLabelHeight: 14}, {ID: "b", Width: 40, Height: 60}, {ID: "c", Width: 40, Height: 30}},
		Edges: []LayoutEdge{{Source: "b", Target: "c"}, {Source: "a", Target: "c"}},
	}
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	x := labelBox(l.Nodes[0].XLabelX, l.Nodes[0].XLabelY, 30, 14)
	for i, n := range l.Nodes {
		if labelBox(n.X, n.Y, n.Width, n.Height).overlaps(x) {
			t.Errorf("the xlabel %v covers node %d %+v", x, i, n)
		}
	}
}

func TestRectCuts(t *testing.T) {
	t.Parallel()
	r := rect{0, 0, 10, 10}
	for _, c := range []struct {
		p, q [2]float64
		want bool
	}{
		{[2]float64{-5, 5}, [2]float64{15, 5}, true},
		{[2]float64{-5, -5}, [2]float64{15, -5}, false},
		{[2]float64{5, -5}, [2]float64{5, 15}, true},
		{[2]float64{-5, 5}, [2]float64{-1, 5}, false},
		{[2]float64{-5, 0}, [2]float64{5, 10}, true},
		{[2]float64{-5, 20}, [2]float64{20, 15}, false},
	} {
		if got := r.cuts(c.p, c.q); got != c.want {
			t.Errorf("%v to %v: got %v", c.p, c.q, got)
		}
	}
}

func TestEndLabelsSpread(t *testing.T) {
	t.Parallel()
	// Three edges end at the same port of t, and three leave the same
	// port of s, each with a label at that end.
	const w, h = 120.0, 14.0
	west, east := []float64{-1, 0}, []float64{1, 0}
	in := LayoutInput{Direction: LR}
	for _, id := range []string{"a", "b", "c", "t", "s", "x", "y", "z"} {
		in.Nodes = append(in.Nodes, LayoutNode{ID: id, Width: 80, Height: 40})
	}
	for _, id := range []string{"a", "b", "c"} {
		in.Edges = append(in.Edges, LayoutEdge{Source: id, Target: "t", HeadLabelWidth: w, HeadLabelHeight: h, TargetPort: []float64{-40, -10}, TargetSide: west})
	}
	for _, id := range []string{"x", "y", "z"} {
		in.Edges = append(in.Edges, LayoutEdge{Source: "s", Target: id, TailLabelWidth: w, TailLabelHeight: h, SourcePort: []float64{40, 10}, SourceSide: east})
	}
	l, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	var labels []rect
	for i, e := range l.Edges {
		x, y, ex, ey := e.HeadLabelX, e.HeadLabelY, e.EndX, e.EndY
		if i >= 3 {
			x, y, ex, ey = e.TailLabelX, e.TailLabelY, e.StartX, e.StartY
		}
		b := labelBox(x, y, w, h)
		// Each label stays near its own end.
		if d := math.Hypot(x-ex, y-ey); d > w/2+endAlong+endBeside+float64(endSteps)*(h+endApart)+h {
			t.Errorf("label %d at %v, %v is %v from its end %v, %v", i, x, y, d, ex, ey)
		}
		for j, o := range labels {
			if b.overlaps(o) {
				t.Errorf("label %d %v covers label %d %v", i, b, j, o)
			}
		}
		for j, n := range l.Nodes {
			if b.overlaps(labelBox(n.X, n.Y, n.Width, n.Height)) {
				t.Errorf("label %d %v covers node %s", i, b, in.Nodes[j].ID)
			}
		}
		labels = append(labels, b)
	}
	again, err := Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	for i := range l.Edges {
		if l.Edges[i].HeadLabelX != again.Edges[i].HeadLabelX || l.Edges[i].HeadLabelY != again.Edges[i].HeadLabelY ||
			l.Edges[i].TailLabelX != again.Edges[i].TailLabelX || l.Edges[i].TailLabelY != again.Edges[i].TailLabelY {
			t.Errorf("edge %d: the labels moved between two layouts", i)
		}
	}
}

func TestRectInside(t *testing.T) {
	t.Parallel()
	r := rect{0, 0, 40, 20}
	for _, c := range []struct {
		p    [2]float64
		want rect
	}{
		// On a side, and outside: the whole box.
		{[2]float64{0, 10}, r},
		{[2]float64{-5, 10}, r},
		// 3 inside the left side: the box 3 smaller on each side.
		{[2]float64{3, 10}, rect{3, 3, 37, 17}},
	} {
		if got := r.inside(c.p); got != c.want {
			t.Errorf("%v: got %v, want %v", c.p, got, c.want)
		}
	}
	// At the center, no part lies deeper, and the box covers nothing.
	if got := r.inside([2]float64{20, 10}); got.overlaps(r) {
		t.Errorf("the center: got %v, which overlaps the box", got)
	}
}
