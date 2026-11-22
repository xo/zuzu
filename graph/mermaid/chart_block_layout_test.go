package mermaid

import (
	"strings"
	"testing"
)

// TestArrowTextOffset checks that the label of a block arrow to one side
// centers on its shaft, which ends where its head starts, and that an
// arrow to both sides or on both axes keeps its label in its middle. An
// arrow 100 by 40 to one side has a head of 20, and an arrow 60 by 80 up
// or down a head of 20.
func TestArrowTextOffset(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		dirs   string
		w, h   float64
		dx, dy float64
	}{
		{"right", 100, 40, -10, 0},
		{"left", 100, 40, 10, 0},
		{"down", 60, 80, 0, -10},
		{"up", 60, 80, 0, 10},
		{"left right", 100, 40, 0, 0},
		{"up down", 60, 80, 0, 0},
		{"right down", 80, 80, 0, 0},
	} {
		dirs := map[string]bool{}
		for d := range strings.FieldsSeq(tt.dirs) {
			dirs[d] = true
		}
		if dx, dy := arrowTextOffset(dirs, tt.w, tt.h); dx != tt.dx || dy != tt.dy {
			t.Errorf("%s: the label moves %v, %v, want %v, %v", tt.dirs, dx, dy, tt.dx, tt.dy)
		}
	}
}
