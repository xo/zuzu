package graph

import (
	"slices"
	"testing"
)

func TestShiftDash(t *testing.T) {
	t.Parallel()
	tests := []struct {
		dash []float64
		off  float64
		exp  []float64
	}{
		{[]float64{9, 5}, 0, []float64{9, 5}},
		{[]float64{9, 5}, 4, []float64{5, 5, 4, 0}},
		{[]float64{9, 5}, 900, []float64{5, 5, 4, 0}},
		{[]float64{9, 5}, -10, []float64{5, 5, 4, 0}},
		// In a gap, the dash starts at the next dash.
		{[]float64{9, 5}, 11, []float64{9, 5}},
		{[]float64{4, 2, 1, 3}, 5, []float64{1, 3, 4, 2}},
		// A dash of an odd length repeats, as in SVG.
		{[]float64{3}, 1, []float64{2, 3, 1, 0}},
		{[]float64{9}, 900, []float64{9}},
	}
	for _, test := range tests {
		var in []any
		for _, v := range test.dash {
			in = append(in, v)
		}
		var got []float64
		for _, v := range shiftDash(in, test.off) {
			f, _ := v.(float64)
			got = append(got, f)
		}
		if !slices.Equal(got, test.exp) {
			t.Errorf("%v at %v: expected %v, got %v", test.dash, test.off, test.exp, got)
		}
	}
}

func TestStyleFontStyle(t *testing.T) {
	t.Parallel()
	r := map[string]any{}
	style(r, map[string]string{"fontStyle": "italic", "strokeDash": "9,5", "strokeDashOffset": "4"})
	if r["fontStyle"] != "italic" {
		t.Errorf("expected the style italic, got %v", r["fontStyle"])
	}
	if d, _ := r["strokeDash"].([]any); len(d) != 4 || d[0] != 5.0 {
		t.Errorf("expected the dash 5,5,4,0, got %v", r["strokeDash"])
	}
}
