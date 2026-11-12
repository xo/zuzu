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

func TestConfigAxisDomain(t *testing.T) {
	t.Parallel()
	const bars = `"mark": "bar",
		"encoding": {"x": {"field": "c", "type": "nominal"}, "y": {"field": "b", "type": "quantitative"}}`
	if d := marksOf(renderVL(t, bars), "axis-domain"); len(d) == 0 {
		t.Fatal("the band axis draws no domain")
	}
	s := renderVL(t, bars+`, "config": {"axis": {"domain": false}}`)
	if d := marksOf(s, "axis-domain"); len(d) != 0 {
		t.Errorf("the config hides the domain, but %d axes draw it", len(d))
	}
	s = renderVL(t, `"mark": "bar",
		"encoding": {"x": {"field": "c", "type": "nominal", "axis": {"domain": true}}, "y": {"field": "b", "type": "quantitative"}},
		"config": {"axis": {"domain": false}}`)
	if d := marksOf(s, "axis-domain"); len(d) != 1 {
		t.Errorf("the axis shows its domain over the config, but %d axes draw it", len(d))
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

// renderVega renders a Vega spec.
func renderVega(t *testing.T, src string) *scene.Scene {
	t.Helper()
	c, _, err := Parse([]byte(`{"$schema": "https://vega.github.io/schema/vega/v5.json",` + src + `}`))
	if err != nil {
		t.Fatal(err)
	}
	s, _, err := c.Render(t.Context(), WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestHierarchyLook(t *testing.T) {
	t.Parallel()
	s := renderVega(t, `"width": 300, "height": 200,
		"data": [{"name": "tree", "values": [
			{"id": "root", "p": null}, {"id": "groupA", "p": "root"}, {"id": "a1", "p": "groupA", "v": 5},
			{"id": "a2", "p": "groupA", "v": 3}, {"id": "b1", "p": "root", "v": 4}],
			"transform": [{"type": "stratify", "key": "id", "parentKey": "p"},
				{"type": "treemap", "field": "v", "method": "squarify", "size": [300, 200]}]}],
		"marks": [{"type": "rect", "from": {"data": "tree"}, "encode": {"enter": {
			"x": {"field": "x0"}, "y": {"field": "y0"}, "x2": {"field": "x1"}, "y2": {"field": "y1"}}}}]`)
	rects := marksOf(s, "mark")[0].Items
	for _, it := range rects {
		d, ok := it.Datum.(map[string]any)
		if !ok {
			t.Fatalf("the datum of a node is %T", it.Datum)
		}
		children, _ := d["children"].(float64)
		group := children > 0
		want, r := 0.85, 3.0
		if group {
			want, r = 0.14, 6
		}
		if it.FillOpacity == nil || *it.FillOpacity != want || it.CornerRadius != r {
			t.Errorf("%v: the opacity %v and the corner %v, want %v and %v", d["id"], it.FillOpacity, it.CornerRadius, want, r)
		}
	}
	names := marksOf(s, "zuzu-hierarchy-label")
	if len(names) != 1 {
		t.Fatalf("%d marks of names, want 1", len(names))
	}
	found := map[string]bool{}
	for _, it := range names[0].Items {
		found[it.Text[0]] = true
	}
	if !found["groupA"] || found["root"] {
		t.Errorf("the names are %v, want groupA and not root", found)
	}
}

func TestDonutTotal(t *testing.T) {
	t.Parallel()
	s := renderVL(t, `"mark": {"type": "arc", "innerRadius": 50},
		"encoding": {"theta": {"field": "b", "type": "quantitative"}, "color": {"field": "a", "type": "nominal"}}`)
	total := marksOf(s, "zuzu-donut-total")
	if len(total) != 1 || total[0].Items[0].Text[0] != "17" {
		t.Fatalf("the total of the donut: %v", total)
	}
	// A pie has no middle, so it has no total.
	if p := marksOf(renderVL(t, `"mark": "arc", "encoding": {"theta": {"field": "b", "type": "quantitative"}}`), "zuzu-donut-total"); len(p) != 0 {
		t.Error("a pie writes a total")
	}
}

func TestSankeyLook(t *testing.T) {
	t.Parallel()
	s := renderVega(t, `"width": 300, "height": 200,
		"data": [
			{"name": "links", "values": [{"source": "a", "target": "b", "value": 2}, {"source": "a", "target": "c", "value": 1}]},
			{"name": "nodes", "source": "links", "transform": [{"type": "zuzu.sankey", "size": [300, 200]}]},
			{"name": "bands", "source": "links", "transform": [{"type": "zuzu.sankeylinks", "nodes": "nodes"}]}],
		"marks": [
			{"type": "path", "from": {"data": "bands"}, "encode": {"enter": {"path": {"field": "path"}, "fill": {"value": "#888"}}}},
			{"type": "rect", "from": {"data": "nodes"}, "encode": {"enter": {
				"x": {"field": "x0"}, "x2": {"field": "x1"}, "y": {"field": "y0"}, "y2": {"field": "y1"}}}}]`)
	marks := marksOf(s, "mark")
	if it := marks[0].Items[0]; it.FillOpacity == nil || *it.FillOpacity != 0.32 {
		t.Errorf("a link has the opacity %v, want 0.32", it.FillOpacity)
	}
	if it := marks[1].Items[0]; it.CornerRadius != 3 {
		t.Errorf("a node has the corner %v, want 3", it.CornerRadius)
	}
}
