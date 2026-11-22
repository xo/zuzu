package graph

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/zuzu/theme"
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

func TestSubroutineFitsLabel(t *testing.T) {
	t.Parallel()
	// The label of a subroutine, with its padding, sits between the two
	// inner lines, also when the label is short.
	for _, tw := range []float64{6, 20, 80} {
		pad := 8.0
		w, _ := NodeSize("subroutine", tw, 14, pad)
		box, _ := NodeSize("rect", tw, 14, pad)
		if inner := w - 2*subroutineFrame; inner < tw+2*pad || inner < box {
			t.Errorf("a label %v wide has %v between the inner lines, want at least %v", tw, inner, math.Max(tw+2*pad, box))
		}
	}
}

func TestStrokeHoldsNoArea(t *testing.T) {
	t.Parallel()
	got := stroke([2]float64{0, 0}, [2]float64{10, 0}, [2]float64{10, 5})
	if want := "M0,0L10,0L10,5L10,0L0,0"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFramedCircleDot(t *testing.T) {
	t.Parallel()
	// The dot of fr-circ is the circles inside the ring. The outline of a
	// node fills each one as a band, and the bands cover the dot from the
	// middle out to a radius of 2.5, so the dot is 5 pixels wide, as in
	// Mermaid 12.3. A circle smaller than half the outline leaves a hole
	// in the middle when okage draws it.
	path := ShapePath("fr-circ", 14, 14)
	var radii []float64
	for _, m := range regexp.MustCompile(`A([0-9.]+),`).FindAllStringSubmatch(path, -1) {
		r, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			t.Fatal(err)
		}
		if r < 7 && !slices.Contains(radii, r) {
			radii = append(radii, r)
		}
	}
	slices.Sort(radii)
	half := theme.NodeOutline / 2
	if len(radii) == 0 || radii[0] < half {
		t.Fatalf("the circles of the dot are %v, want the smallest at least %v", radii, half)
	}
	covered := 0.0
	for _, r := range radii {
		if r-half > covered {
			t.Errorf("the dot has a gap from %v to %v", covered, r-half)
		}
		covered = math.Max(covered, r+half)
	}
	if math.Abs(covered-2.5) > 1e-9 {
		t.Errorf("the dot has a radius of %v, want 2.5", covered)
	}
}

// TestCylinder checks that the front edge of the top of a cylinder runs
// from right to left with the sweep of the outline, so that the nonzero
// fill covers the top and the top does not show the background, and that a
// cylinder of DOT takes the size of a box, as Graphviz draws it, where a
// cylinder of Mermaid grows by its ends.
func TestCylinder(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"cylinder", DOTCylinder} {
		path := ShapePath(kind, 60, 40)
		_, front, ok := strings.Cut(path, "Z")
		if !ok || !strings.HasPrefix(front, "M30,") || !strings.Contains(front, " 0 0,1 -30,") {
			t.Errorf("%s: the front edge of the top is %q, want it from the right to the left with the sweep 1", kind, front)
		}
	}
	bw, bh := NodeSize("rect", 20, 12, 8)
	if w, h := NodeSize(DOTCylinder, 20, 12, 8); w != bw || h != bh {
		t.Errorf("a cylinder of DOT is %v by %v, want the size %v by %v of a box", w, h, bw, bh)
	}
	if _, h := NodeSize("cylinder", 20, 12, 8); h <= bh {
		t.Errorf("a cylinder of Mermaid is %v high, want more than the %v of a box", h, bh)
	}
	// The label sits in the middle of the body, one radius of the top
	// below the center of the node.
	for _, kind := range []string{"cylinder", DOTCylinder} {
		if _, got := TextOffset(kind, 80, 50); got != cylinderRadius(50) || got <= 0 {
			t.Errorf("%s: the label is %v below the center, want %v", kind, got, cylinderRadius(50))
		}
	}
	if _, got := TextOffset("rect", 80, 50); got != 0 {
		t.Errorf("rect: the label is %v below the center, want 0", got)
	}
}

// TestTextOffsetOfDecoratedShapes checks that the label of a shape with a
// band, a strip, a tab, a cap or a stack sits in the middle of the body of
// the shape: right and down past a band or a strip, left past the end of
// a cylinder on its side, and up and left onto the front sheet of a stack.
func TestTextOffsetOfDecoratedShapes(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		kind   string
		dx, dy float64
	}{
		{"div-rect", 0, paneBand / 2},
		{"win-pane", paneBand / 2, paneBand / 2},
		{"st-rect", -stackOffset, stackOffset},
		{"lin-cyl", 0, cylinderRadius(60) + linCylGap/2},
		{"h-cyl", -hcylRadius(60), 0},
		{"bucket", 0, bucketRadius(60)},
		{"browser", 0, browserBar / 2},
		{"folder", 0, folderTab(60) / 2},
	} {
		dx, dy := TextOffset(tt.kind, 120, 60)
		if dx != tt.dx || dy != tt.dy {
			t.Errorf("%s: the label moves %v, %v, want %v, %v", tt.kind, dx, dy, tt.dx, tt.dy)
		}
	}
}
