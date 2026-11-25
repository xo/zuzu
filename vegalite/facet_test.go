package vegalite

import (
	"strings"
	"testing"
)

// bullet is a facet of rows of layers of bars and ticks, each row with an
// x scale of its own, as the bullet charts of Vega-Lite draw them.
const bullet = `{"$schema": "https://vega.github.io/schema/vega-lite/v5.json",
	"data": {"values": [
		{"title": "Revenue", "ranges": [150, 300], "measures": [220], "markers": [250]},
		{"title": "Profit", "ranges": [20, 30], "measures": [21], "markers": [26]}
	]},
	"facet": {"row": {"field": "title", "type": "ordinal", "header": {"labelAngle": 0, "title": ""}}},
	"spec": {
		"encoding": {"x": {"type": "quantitative", "scale": {"nice": false}, "title": null}},
		"layer": [
			{"mark": {"type": "bar", "color": "#eee"}, "encoding": {"x": {"field": "ranges[1]"}}},
			{"mark": {"type": "bar", "color": "#ccc"}, "encoding": {"x": {"field": "ranges[0]"}}},
			{"mark": {"type": "bar", "color": "steelblue", "size": 10}, "encoding": {"x": {"field": "measures[0]"}}},
			{"mark": {"type": "tick", "color": "black"}, "encoding": {"x": {"field": "markers[0]"}}}
		]
	},
	"resolve": {"scale": {"x": "independent"}},
	"config": {"tick": {"thickness": 2}, "scale": {"barBandPaddingInner": 0}}
}`

// TestFacetIndependentLayers checks that each cell of a facet with an
// independent x scale over layers reads the datasets of the cell for the
// domain of its scale, and not those of the top level, which hold the rows
// of every cell.
func TestFacetIndependentLayers(t *testing.T) {
	t.Parallel()
	m := compileMap(t, bullet)
	cell := find(m, "marks", "cell")
	if cell == nil {
		t.Fatal("no cell")
	}
	inCell := map[string]bool{}
	data, _ := cell["data"].([]any)
	for _, d := range data {
		if d, ok := d.(map[string]any); ok {
			name, _ := d["name"].(string)
			inCell[name] = true
		}
	}
	scales, _ := cell["scales"].([]any)
	if len(scales) != 1 {
		t.Fatalf("the cell has the scales %s, want child_x", asJSON(scales))
	}
	sc, _ := scales[0].(map[string]any)
	domain, _ := sc["domain"].(map[string]any)
	fields, _ := domain["fields"].([]any)
	if len(fields) == 0 {
		t.Fatalf("the domain is %s, want the fields of the layers", asJSON(domain))
	}
	for _, f := range fields {
		ref, _ := f.(map[string]any)
		if d, _ := ref["data"].(string); !inCell[d] {
			t.Errorf("the domain reads %s, which the cell does not hold: %s", d, asJSON(domain))
		}
	}
}

// TestFacetFlattensOnce checks that the fields inside objects of the layers
// of a facet over inline values flatten once, in a dataset of their own
// that the facet reads, as vega-lite.js compiles it, and not in each
// dataset of a layer or of a cell.
func TestFacetFlattensOnce(t *testing.T) {
	t.Parallel()
	m := compileMap(t, bullet)
	flattens := func(d map[string]any) []string {
		var out []string
		ts, _ := d["transform"].([]any)
		for _, tr := range ts {
			tr, _ := tr.(map[string]any)
			if e, _ := tr["expr"].(string); tr["type"] == "formula" && strings.Contains(e, "] && datum[") {
				as, _ := tr["as"].(string)
				out = append(out, as)
			}
		}
		return out
	}
	parse := find(m, "data", "data_0")
	if parse == nil {
		t.Fatalf("no data_0: %s", asJSON(m["data"]))
	}
	if got := strings.Join(flattens(parse), ","); got != "ranges.1,ranges.0,measures.0,markers.0" {
		t.Errorf("data_0 flattens %s, want ranges.1,ranges.0,measures.0,markers.0", got)
	}
	data, _ := m["data"].([]any)
	for _, d := range data {
		d, _ := d.(map[string]any)
		if d["name"] != "data_0" && len(flattens(d)) > 0 {
			t.Errorf("the dataset %s flattens again: %s", d["name"], asJSON(d))
		}
	}
	cell := find(m, "marks", "cell")
	if cell == nil {
		t.Fatal("no cell")
	}
	facet, _ := cell["from"].(map[string]any)["facet"].(map[string]any)
	if facet["data"] != "data_0" {
		t.Errorf("the facet reads %v, want data_0", facet["data"])
	}
	inCell, _ := cell["data"].([]any)
	for _, d := range inCell {
		d, _ := d.(map[string]any)
		if len(flattens(d)) > 0 {
			t.Errorf("the dataset %s of the cell flattens again: %s", d["name"], asJSON(d))
		}
	}
}

// TestLoneChannelMarks checks the place across the view of a bar and a tick
// that have only x: centered, with the size of the mark, the view less the
// inner padding of the bands of a bar, or three quarters of the view for a
// tick, whose thickness comes from the config.
func TestLoneChannelMarks(t *testing.T) {
	t.Parallel()
	m := compileMap(t, bullet)
	for _, c := range []struct {
		mark string
		want map[string]string
	}{
		{"child_layer_0_marks", map[string]string{"yc": `{"mult":0.5,"signal":"child_height"}`, "height": `{"signal":"1 * child_height"}`}},
		{"child_layer_2_marks", map[string]string{"yc": `{"mult":0.5,"signal":"child_height"}`, "height": `{"value":10}`}},
		{"child_layer_3_marks", map[string]string{"yc": `{"mult":0.5,"signal":"child_height"}`, "height": `{"signal":"0.75 * child_height"}`, "width": `{"value":2}`}},
	} {
		u := update(find(m, "marks", c.mark))
		for k, want := range c.want {
			if got := asJSON(u[k]); got != want {
				t.Errorf("%s: %s = %s, want %s", c.mark, k, got, want)
			}
		}
	}
	// A bar of one binned x runs up the whole view, as Vega-Lite orients
	// it, and a bar of one quantitative x keeps the default padding.
	for _, c := range []struct {
		name, encoding string
		want           map[string]string
	}{
		{"binned", `"x": {"field": "v", "type": "quantitative", "bin": true}`, map[string]string{"y": `{"value":0}`, "height": `{"field":{"group":"height"}}`}},
		{"quantitative", `"x": {"field": "v", "type": "quantitative"}`, map[string]string{"yc": `{"mult":0.5,"signal":"height"}`, "height": `{"signal":"0.9 * height"}`}},
	} {
		m := compileMap(t, `{"data": {"values": [{"v": 1}, {"v": 5}]}, "mark": "bar", "encoding": {`+c.encoding+`}}`)
		u := update(find(m, "marks", "marks"))
		for k, want := range c.want {
			if got := asJSON(u[k]); got != want {
				t.Errorf("%s: %s = %s, want %s", c.name, k, got, want)
			}
		}
	}
}

// TestFacetHeaderLabels checks that the properties of a header style the
// title of Vega that draws its labels, and that a property that the
// compiler does not read is an issue.
func TestFacetHeaderLabels(t *testing.T) {
	t.Parallel()
	m := compileMap(t, bullet)
	title, _ := find(m, "marks", "row_header")["title"].(map[string]any)
	for k, want := range map[string]any{"angle": 0.0, "align": "right", "baseline": "middle", "orient": "left"} {
		if title[k] != want {
			t.Errorf("the title of the header has %s %v, want %v: %s", k, title[k], want, asJSON(title))
		}
	}
	s := strings.Replace(bullet, `"labelAngle": 0`, `"labelAngle": -90, "labelFontSize": 15, "labelExpr": "upper(datum.label)", "format": "d"`, 1)
	spec, err := Parse([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	c, report, err := Compile(spec)
	if err != nil {
		t.Fatal(err)
	}
	b := asJSON(findMark(c.Marks, "row_header").Title)
	for _, want := range []string{`"angle":-90`, `"align":"center"`, `"fontSize":15`, `upper((isValid(parent[\"title\"])`} {
		if !strings.Contains(b, want) {
			t.Errorf("the title of the header is %s, want %s in it", b, want)
		}
	}
	if len(report) != 1 || !strings.Contains(report[0].Error(), "format") {
		t.Errorf("the report is %v, want the property format of the header", report)
	}
}
