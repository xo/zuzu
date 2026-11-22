package vegalite

import (
	"testing"

	"github.com/xo/zuzu/vega"
)

// legendTitles returns the title of each legend of a spec.
func legendTitles(t *testing.T, s *vega.Spec) []string {
	t.Helper()
	var out []string
	for _, l := range s.Legends {
		var title string
		if err := l.Title.Decode(&title); err != nil && !l.Title.IsZero() {
			t.Fatal(err)
		}
		out = append(out, title)
	}
	return out
}

// TestCompileLegendMerge checks that a channel shares the legend of a
// channel of the same field when one of them gives a title of its own, as
// in Vega-Lite, and that a quantile scale of sizes takes four sizes. Before,
// circle_scale_quantile drew two legends, one of them of two entries.
func TestCompileLegendMerge(t *testing.T) {
	t.Parallel()
	s := compile(t, `{"data": {"values": [{"b": 1}, {"b": 2}, {"b": 3}]}, "mark": "circle",
		"encoding": {
			"size": {"field": "b", "type": "quantitative", "scale": {"type": "quantile"}},
			"color": {"field": "b", "type": "quantitative", "scale": {"type": "quantile"}, "legend": {"title": "Quantile"}}}}`)
	if got := legendTitles(t, s); len(got) != 1 || got[0] != "Quantile" {
		t.Fatalf("legends %q, want one named Quantile", got)
	}
	if l := s.Legends[0]; l.Fill == "" || l.Size == "" {
		t.Errorf("legend = %+v, want the fill and the size", l)
	}
	for _, sc := range s.Scales {
		if sc.Name == "size" && sc.Range.Signal != "sequence(4, 361 + (361 - 4) / 3, (361 - 4) / 3)" {
			t.Errorf("the range of the sizes = %+v, want four sizes from 4 to 361", sc.Range)
		}
	}
}

// TestCompileLegendIndependent checks that an independent resolve of a
// legend gives each layer its own legend of the shared scale, with its own
// title and the color of its mark. Before, layer_circle_independent_color
// drew one of its two legends.
func TestCompileLegendIndependent(t *testing.T) {
	t.Parallel()
	s := compile(t, `{"data": {"values": [{"a": 1, "o": "x"}]},
		"layer": [
			{"mark": "circle", "encoding": {"size": {"aggregate": "count", "type": "quantitative", "title": "All"}}},
			{"mark": "circle", "encoding": {"size": {"aggregate": "count", "type": "quantitative", "title": "Some"},
				"color": {"value": "firebrick"}}}],
		"resolve": {"legend": {"size": "independent"}}}`)
	got := legendTitles(t, s)
	if len(got) != 2 || got[0] != "All" || got[1] != "Some" {
		t.Fatalf("legends %q, want All and Some", got)
	}
	if c := string(s.Legends[1].Extra["symbolFillColor"]); c != `"firebrick"` {
		t.Errorf("the color of the second legend = %s, want firebrick", c)
	}
}

// TestCompileLegendProps checks that the properties of a legend that the
// runtime reads pass on to the Vega legend with no issue.
func TestCompileLegendProps(t *testing.T) {
	t.Parallel()
	s := compile(t, `{"data": {"values": [{"a": 1}]}, "mark": "circle",
		"encoding": {"size": {"field": "a", "type": "quantitative",
			"legend": {"clipHeight": 30, "symbolLimit": 3, "labelLimit": 40, "symbolStrokeWidth": 2, "offset": 4}}}}`)
	l := s.Legends[0]
	if l.ClipHeight.IsZero() || l.Offset.IsZero() {
		t.Errorf("legend = %+v, want clipHeight and offset", l)
	}
	for _, k := range []string{"symbolLimit", "labelLimit", "symbolStrokeWidth"} {
		if _, ok := l.Extra[k]; !ok {
			t.Errorf("legend has no %s", k)
		}
	}
}
