package graph

import (
	"math"
	"testing"
)

func TestFigureShapes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kind   string
		fw, fh float64
	}{
		{"bpmn-start", 36, 36},
		{"bpmn-intermediate", 36, 36},
		{"bpmn-boundary", 36, 36},
		{"bpmn-end", 36, 36},
		{"bpmn-gateway", 50, 50},
		{"bpmn-data", 36, 50},
		{"bpmn-data-store", 50, 50},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			t.Parallel()
			if got, ok := ShapeOf(tt.kind); !ok || got != tt.kind {
				t.Fatalf("ShapeOf(%q) = %q, %v", tt.kind, got, ok)
			}
			if !LabelBelow(tt.kind) {
				t.Errorf("LabelBelow(%q) = false", tt.kind)
			}
			// A label of 94 by 14 widens the node, and a band of the gap
			// and the label goes above and under the figure.
			w, h := NodeSize(tt.kind, 94, 14, 8)
			if want := math.Max(tt.fw, 94); w != want || h != tt.fh+2*(6+14) {
				t.Errorf("NodeSize = %v, %v, want %v, %v", w, h, want, tt.fh+40)
			}
			// The label is under the figure, 6 below its bottom.
			if _, dy := TextOffset(tt.kind, w, h); dy != tt.fh/2+6+7 {
				t.Errorf("the label is %v below the center, want %v", dy, tt.fh/2+13)
			}
			// A node with no label is its figure.
			if w, h := NodeSize(tt.kind, 0, 0, 8); w != tt.fw || h != tt.fh {
				t.Errorf("NodeSize with no label = %v, %v, want %v, %v", w, h, tt.fw, tt.fh)
			}
			// An edge from the right ends on the figure, not on the box of
			// the label.
			n := NodePlace{X: 0, Y: 0, Width: w, Height: h}
			if x, y := clip(n, tt.kind, 200, 0); math.Abs(x-tt.fw/2) > 1e-9 || y != 0 {
				t.Errorf("an edge from the right ends at %v, %v, want %v, 0", x, y, tt.fw/2)
			}
			if p := ShapePath(tt.kind, w, h); p == "" {
				t.Error("the shape draws nothing")
			}
		})
	}
}

func TestPerson(t *testing.T) {
	t.Parallel()
	if got, ok := ShapeOf("person"); !ok || got != "person" {
		t.Fatalf("ShapeOf(person) = %q, %v", got, ok)
	}
	if LabelBelow("person") {
		t.Error("a person puts its label in its body")
	}
	// The body is at least 100 wide, and the head of 0.23 of the width
	// reaches above it, less its overlap of 0.27 of its radius.
	w, h := NodeSize("person", 40, 14, 8)
	r := 23.0
	if w != 100 || math.Abs(h-(14+2*8*1.2+2*r-0.27*r)) > 1e-9 {
		t.Errorf("NodeSize = %v, %v", w, h)
	}
	// The label is in the middle of the body.
	body := 14 + 2*8*1.2
	if _, dy := TextOffset("person", w, h); math.Abs(dy-(h-body)/2) > 1e-9 {
		t.Errorf("the label is %v below the center, want %v", dy, (h-body)/2)
	}
	// An edge from above ends on the top of the head, and an edge from
	// below on the bottom of the body.
	n := NodePlace{X: 0, Y: 0, Width: w, Height: h}
	if _, y := clip(n, "person", 0, -500); math.Abs(y+h/2) > 0.5 {
		t.Errorf("an edge from above ends at y %v, want %v", y, -h/2)
	}
	if _, y := clip(n, "person", 0, 500); math.Abs(y-h/2) > 1e-6 {
		t.Errorf("an edge from below ends at y %v, want %v", y, h/2)
	}
	// The head is narrower than the body, so an edge from the side at the
	// height of the head ends on the head, inside the box.
	headY := personParts(w, h).headY
	if x, _ := clip(n, "person", 500, headY*500/math.Abs(headY)); x >= w/2-1 {
		t.Errorf("an edge toward the head ends at x %v, on the box of %v", x, w/2)
	}
}
