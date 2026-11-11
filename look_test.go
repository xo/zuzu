package zuzu

import (
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu/scene"
)

// renderVL renders a Vega-Lite spec of rows a, b and c.
func renderVL(t *testing.T, rest string) *scene.Scene {
	t.Helper()
	src := []byte(`{"$schema": "https://vega.github.io/schema/vega-lite/v5.json",
		"data": {"values": [{"a": 1, "b": 3, "c": "p"}, {"a": 2, "b": 5, "c": "p"}, {"a": 3, "b": 4, "c": "p"},
			{"a": 1, "b": 1, "c": "q"}, {"a": 2, "b": 2, "c": "q"}, {"a": 3, "b": 2, "c": "q"}]},` + rest + `}`)
	c, _, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	s, _, err := c.Render(t.Context(), WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// marksOf returns the marks of a role in a scene, in order.
func marksOf(s *scene.Scene, role string) []*scene.Mark {
	var out []*scene.Mark
	var walk func(m *scene.Mark)
	walk = func(m *scene.Mark) {
		if m.Role == role {
			out = append(out, m)
		}
		for i := range m.Items {
			for j := range m.Items[i].Items {
				walk(&m.Items[i].Items[j])
			}
		}
	}
	walk(&s.Root)
	return out
}

func TestSpecConfig(t *testing.T) {
	t.Parallel()
	s := renderVL(t, `"mark": "point",
		"encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "b", "type": "quantitative"}, "color": {"field": "c"}},
		"title": "T",
		"config": {"axis": {"labelColor": "#123456", "gridDash": [2, 2]}, "legend": {"labelColor": "#654321", "orient": "bottom"},
			"title": {"color": "#abcdef"}, "range": {"category": ["#111111", "#222222"]}, "background": "#fafafa"}`)
	if s.Background.Color != "#fafafa" {
		t.Errorf("the background is %q", s.Background.Color)
	}
	if l := marksOf(s, "axis-label"); len(l) == 0 || l[0].Items[0].Fill.Color != "#123456" {
		t.Error("the config of the axis does not color its labels")
	}
	if g := marksOf(s, "axis-grid"); len(g) == 0 || len(g[0].Items[0].StrokeDash) != 2 {
		t.Error("the config of the axis does not dash its grid")
	}
	if l := marksOf(s, "legend-label"); len(l) == 0 || l[0].Items[0].Fill.Color != "#654321" {
		t.Error("the config of the legend does not color its labels")
	}
	if l := marksOf(s, "title-text"); len(l) == 0 || l[0].Items[0].Fill.Color != "#abcdef" {
		t.Error("the config of the title does not color it")
	}
	if p := marksOf(s, "mark"); len(p) == 0 || p[0].Items[0].Fill.Color != "#111111" && p[0].Items[0].Fill.Color != "#222222" {
		t.Errorf("the range of the config does not color the points: %v", p[0].Items[0].Fill)
	}
}

func TestAreaLook(t *testing.T) {
	t.Parallel()
	s := renderVL(t, `"transform": [{"filter": "datum.c == 'p'"}], "mark": "area",
		"encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "b", "type": "quantitative"}}`)
	area := marksOf(s, "mark")[0]
	if g := area.Items[0].Fill.Gradient; g == nil || len(g.Stops) != 2 || !strings.HasPrefix(g.Stops[0].Color, "rgba(") {
		t.Errorf("the area has the fill %+v, want a gradient", area.Items[0].Fill)
	}
	if l := marksOf(s, "zuzu-area-line"); len(l) != 1 || len(l[0].Items) != 3 || l[0].Items[0].Interpolate != "monotone" {
		t.Errorf("the line on top of the area: %v", l)
	}
	if h := marksOf(s, "zuzu-high-label"); len(h) != 1 || h[0].Items[0].Text[0] != "High 5" {
		t.Errorf("the high of the area: %v", h)
	}
}

func TestStackLook(t *testing.T) {
	t.Parallel()
	s := renderVL(t, `"mark": "bar",
		"encoding": {"x": {"field": "a", "type": "ordinal"}, "y": {"field": "b", "type": "quantitative"}, "color": {"field": "c"}}`)
	bars := marksOf(s, "mark")[0].Items
	// The two bars at a = 1 meet with a gap of 1.5 between them.
	var at []scene.Item
	for _, b := range bars {
		if b.X == bars[0].X {
			at = append(at, b)
		}
	}
	if len(at) != 2 {
		t.Fatalf("%d bars at one place, want 2", len(at))
	}
	top, low := at[0], at[1]
	if top.Y > low.Y {
		top, low = low, top
	}
	if gap := low.Y - (top.Y + top.Height); gap < 1.49 || gap > 1.51 {
		t.Errorf("the gap of the stack is %v, want 1.5", gap)
	}
}

func TestBoxLook(t *testing.T) {
	t.Parallel()
	s := renderVL(t, `"mark": "boxplot",
		"encoding": {"x": {"field": "c", "type": "nominal"}, "y": {"field": "b", "type": "quantitative"}}`)
	var box *scene.Item
	for _, m := range marksOf(s, "mark") {
		if m.Type == scene.Rect && len(m.Items) > 0 && m.Items[0].CornerRadius == 5 {
			box = &m.Items[0]
		}
	}
	if box == nil || box.FillOpacity == nil || *box.FillOpacity != 0.2 || box.Stroke.Color != box.Fill.Color {
		t.Fatalf("the box is %+v", box)
	}
}

func TestBubbleOrder(t *testing.T) {
	t.Parallel()
	s := renderVL(t, `"mark": "circle",
		"encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "c", "type": "nominal"}, "size": {"field": "b", "type": "quantitative"}}`)
	items := marksOf(s, "mark")[0].Items
	for i := 1; i < len(items); i++ {
		if *items[i].Size > *items[i-1].Size {
			t.Fatalf("a bubble of %v draws after one of %v", *items[i].Size, *items[i-1].Size)
		}
	}
}

func TestLineEnds(t *testing.T) {
	t.Parallel()
	s := renderVL(t, `"mark": "line",
		"encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "b", "type": "quantitative"}, "color": {"field": "c"}}`)
	labels := marksOf(s, "zuzu-end-label")
	if len(labels) != 2 {
		t.Fatalf("%d names at the ends of the lines, want 2", len(labels))
	}
	if len(marksOf(s, "legend")) != 0 {
		t.Error("the legend draws although the lines name each series")
	}
}

func TestReferenceLine(t *testing.T) {
	t.Parallel()
	s := renderVL(t, `"layer": [
		{"mark": "point", "encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "b", "type": "quantitative"}}},
		{"mark": "rule", "encoding": {"y": {"aggregate": "mean", "field": "b", "type": "quantitative"}}}]`)
	var rule *scene.Item
	for _, m := range marksOf(s, "mark") {
		if m.Type == scene.Rule {
			rule = &m.Items[0]
		}
	}
	if rule == nil || len(rule.StrokeDash) == 0 {
		t.Fatalf("the reference line is %+v, want a dash", rule)
	}
	if l := marksOf(s, "zuzu-reference-label"); len(l) != 1 || l[0].Items[0].Text[0] != "2.83" {
		t.Errorf("the name of the reference line: %v", l)
	}
}
