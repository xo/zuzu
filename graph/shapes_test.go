package graph

import (
	"math"
	"strings"
	"testing"
)

func TestShapeOfMermaid(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"lined-document": "lin-doc", "tagged-document": "tag-doc", "brace-l": "brace-l", "comment": "brace-l",
		"brace": "brace-l", "braces": "braces", "brace-r": "brace-r", "notch-rect": "card", "lin-proc": "lin-rect",
		"fork": "bar", "join": "bar", "collate": "hourglass", "com-link": "bolt", "das": "h-cyl", "disk": "lin-cyl",
		"display": "curv-trap", "div-proc": "div-rect", "internal-storage": "win-pane", "loop-limit": "notch-pent",
		"manual-input": "sl-rect", "documents": "docs", "processes": "st-rect", "stored-data": "bow-rect",
		"summary": "cross-circ", "tag-proc": "tag-rect", "paper-tape": "paper-tape", "flag": "paper-tape",
		"data-store": "datastore", "stop": "fr-circ", "subprocess": "subroutine", "directory": "folder",
		"bucket": "bucket", "console": "console", "browser": "browser", "cloud": "cloud", "bang": "bang",
		"bpmn-annotation": "bracket", "bpmn-activity": "rounded",
		"Mdiamond": "Mdiamond", "Msquare": "Msquare", "Mcircle": "Mcircle",
	}
	for name, want := range tests {
		got, ok := ShapeOf(name)
		if !ok || got != want {
			t.Errorf("ShapeOf(%q) = %q, %v, want %q", name, got, ok, want)
		}
		if p := ShapePath(got, 120, 50); p == "" && got != "none" {
			t.Errorf("the shape %s draws nothing", got)
		}
	}
}

func TestPolygon(t *testing.T) {
	t.Parallel()
	name := Polygon(3, 0, 0, 0, false)
	if got, ok := ShapeOf(name); !ok || got != name {
		t.Fatalf("ShapeOf(%q) = %q, %v", name, got, ok)
	}
	if got := Polygon(2, 0, 0, 0, false); got != "ellipse" {
		t.Errorf("a polygon of 2 sides is %q, want ellipse", got)
	}
	if _, ok := ShapeOf("polygon x"); ok {
		t.Error("expected no shape for polygon x")
	}
	// A triangle has its tip at the top, and its base a quarter of the
	// height below the center, as in Graphviz.
	p, _ := readPolygon(name)
	pts := p.scaled(144, 144)
	want := [][2]float64{{72, 36}, {0, -72}, {-72, 36}}
	for i, q := range want {
		if math.Abs(pts[i][0]-q[0]) > 0.01 || math.Abs(pts[i][1]-q[1]) > 0.01 {
			t.Errorf("corner %d is %v, want %v", i, pts[i], q)
		}
	}
	// A skew of 0.5 moves the top to the right, and a distortion of 0.5
	// widens the top, by as much as dot draws them.
	for _, c := range []struct {
		name string
		want [][2]float64
	}{
		{Polygon(4, 0.5, 0, 0, false), [][2]float64{{45.68, 72}, {72, -72}, {-45.68, -72}, {-72, 72}}},
		{Polygon(4, 0, 0.5, 0, false), [][2]float64{{38.92, 72}, {74.92, -72}, {-74.92, -72}, {-38.92, 72}}},
		// An orientation of 10 turns the triangle 10 degrees the other way
		// from the clock.
		{Polygon(3, 0, 0, 10, false), [][2]float64{{72, 25.01}, {-13.31, -72}, {-58.69, 46.99}}},
	} {
		p, _ := readPolygon(c.name)
		w := 144.0
		if p.distortion != 0 {
			w = 149.84
		}
		pts := p.scaled(w, 144)
		for i, q := range c.want {
			if math.Abs(pts[i][0]-q[0]) > 0.01 || math.Abs(pts[i][1]-q[1]) > 0.01 {
				t.Errorf("%s: corner %d is %v, want %v", c.name, i, pts[i], q)
			}
		}
	}
	// A square of 4 sides fits its label as a box does.
	w, h := NodeSize(Polygon(4, 0, 0, 0, false), 60, 20, 8)
	bw, bh := NodeSize("box", 60, 20, 8)
	if math.Abs(w-bw) > 0.5 || math.Abs(h-bh) > 0.5 {
		t.Errorf("the square is %v by %v, want %v by %v", w, h, bw, bh)
	}
	// A regular polygon is as wide as it is high.
	if w, h := NodeSize(Polygon(6, 0, 0, 0, true), 60, 20, 8); w != h {
		t.Errorf("the regular hexagon is %v by %v", w, h)
	}
	if !strings.HasPrefix(ShapePath(name, 100, 80), "M") {
		t.Error("expected a path of the triangle")
	}
}

func TestClipPolygon(t *testing.T) {
	t.Parallel()
	// An edge from below ends on the base of a triangle, a quarter of the
	// height below its center.
	n := NodePlace{X: 100, Y: 100, Width: 80, Height: 80}
	x, y := clip(n, Polygon(3, 0, 0, 0, false), 100, 300)
	if math.Abs(x-100) > 0.01 || math.Abs(y-120) > 0.01 {
		t.Errorf("the end is %v, %v, want 100, 120", x, y)
	}
}

func TestStrokeHoldsNoArea(t *testing.T) {
	t.Parallel()
	got := stroke([2]float64{0, 0}, [2]float64{10, 0}, [2]float64{10, 5})
	if want := "M0,0L10,0L10,5L10,0L0,0"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
