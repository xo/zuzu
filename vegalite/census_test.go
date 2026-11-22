package vegalite

import (
	"strings"
	"testing"
)

// The tests in this file come from specs of the corpus that the compiler
// drew wrongly beside vega-lite.js. Each test names its spec.

// A default title shows the field as the spec writes it, with no escape
// that the compiler adds to a nested field (test_aggregate_nested,
// bar_argmax_transform).
func TestTitleOfNestedField(t *testing.T) {
	t.Parallel()
	s := compileMap(t, `{
		"data": {"values": [{"p": {"v": "a", "y": 1}, "m": {"b": 2}}]},
		"mark": "bar",
		"encoding": {
			"y": {"field": "p.v", "type": "nominal"},
			"x": {"field": "p.y", "aggregate": "sum", "type": "quantitative"},
			"color": {"field": "m['b']", "type": "quantitative"}
		}
	}`)
	got := asJSON(s["axes"]) + asJSON(s["legends"])
	for _, want := range []string{`"title":"p.v"`, `"title":"Sum of p.y"`, `"title":"m['b']"`} {
		if !strings.Contains(got, want) {
			t.Errorf("the guides = %s, want %s", got, want)
		}
	}
	if strings.Contains(got, `\\.`) {
		t.Errorf("the guides = %s, want no escaped dot in a title", got)
	}
}

// A rect whose x and x2 each have a time unit runs from x to x2, not over
// the interval of x (layer_falkensee).
func TestTimeUnitWithEnd(t *testing.T) {
	t.Parallel()
	s := compileMap(t, `{
		"data": {"values": [{"start": "1933", "end": "1945"}], "format": {"parse": {"start": "date:'%Y'", "end": "date:'%Y'"}}},
		"mark": "rect",
		"encoding": {
			"x": {"field": "start", "timeUnit": "year"},
			"x2": {"field": "end", "timeUnit": "year"}
		}
	}`)
	u := update(find(s, "marks", "marks"))
	if got := asJSON(u["x"]); got != `{"field":"year_start","scale":"x"}` {
		t.Errorf("x = %s, want the start of the year of start", got)
	}
	if got := asJSON(u["x2"]); got != `{"field":"year_end","scale":"x"}` {
		t.Errorf("x2 = %s, want the start of the year of end", got)
	}
}

// A datum that is a date time object is a date: the rule goes through the
// scale of time at that date, and the domain holds it
// (layer_line_datum_rule_datetime). A number datum joins the domain too
// (layer_line_datum_rule).
func TestDateTimeDatum(t *testing.T) {
	t.Parallel()
	s := compileMap(t, `{
		"layer": [
			{"data": {"values": [{"d": "2005-01-01", "p": 1}]}, "mark": "line",
				"encoding": {"x": {"field": "d", "type": "temporal"}, "y": {"field": "p", "type": "quantitative"}}},
			{"data": {"values": [{}]}, "mark": {"type": "rule", "size": 2},
				"encoding": {"x": {"datum": {"year": 2006}}, "y": {"datum": 300}}}
		]
	}`)
	u := update(find(s, "marks", "layer_1_marks"))
	if got := asJSON(u["x"]); got != `{"scale":"x","signal":"time(datetime(2006, 0, 1, 0, 0, 0, 0))"}` {
		t.Errorf("x = %s, want the date through the scale", got)
	}
	if got := asJSON(u["strokeWidth"]); got != `{"value":2}` {
		t.Errorf("strokeWidth = %s, want the size 2 of the rule", got)
	}
	x, y := find(s, "scales", "x"), find(s, "scales", "y")
	if got := asJSON(x["type"]); got != `"time"` {
		t.Errorf("the type of x = %s, want time", got)
	}
	if got := asJSON(x["domain"]); !strings.Contains(got, `{"signal":"[{data: time(datetime(2006, 0, 1, 0, 0, 0, 0))}]"}`) {
		t.Errorf("the domain of x = %s, want the date", got)
	}
	if got := asJSON(y["domain"]); !strings.Contains(got, `[300]`) {
		t.Errorf("the domain of y = %s, want 300", got)
	}
}

// A range {"field": ...} is the values of the field, in the order of the
// domain (point_scale_range_field).
func TestRangeField(t *testing.T) {
	t.Parallel()
	s := compileMap(t, `{
		"data": {"values": [{"l": "A", "c": "red"}, {"l": "B", "c": "blue"}]},
		"mark": "circle",
		"encoding": {"color": {"field": "l", "type": "nominal", "scale": {"range": {"field": "c"}}}}
	}`)
	got := asJSON(find(s, "scales", "color")["range"])
	if want := `{"data":"source_0","field":"c","sort":{"field":"l","op":"min"}}`; got != want {
		t.Errorf("the range = %s, want %s", got, want)
	}
}

// The label of a binned facet writes its start and its end
// (config_numberFormatType_test, trellis_scatter_binned_row), and the
// labelExpr, labelAngle, labelAlign and labelPadding of a header set its
// labels (trellis_area_seattle).
func TestHeaderLabels(t *testing.T) {
	t.Parallel()
	s := compileMap(t, `{
		"data": {"values": [{"a": 1, "w": 1500}]},
		"mark": "point",
		"encoding": {
			"x": {"field": "a", "type": "quantitative"},
			"row": {"field": "w", "bin": true}
		}
	}`)
	got := asJSON(find(s, "marks", "row_header")["title"])
	if want := `format(parent[\"bin_maxbins_6_w\"], \"\") + \" – \" + format(parent[\"bin_maxbins_6_w_end\"], \"\")`; !strings.Contains(got, want) {
		t.Errorf("the label = %s, want %s", got, want)
	}
	s = compileMap(t, `{
		"data": {"values": [{"a": 1, "h": 0}]},
		"mark": "point",
		"encoding": {
			"x": {"field": "a", "type": "quantitative"},
			"row": {"field": "h", "type": "nominal", "header": {
				"labelExpr": "datum.value == 0 ? 'Zero' : datum.label",
				"labelAngle": 0, "labelAlign": "left", "labelPadding": 2
			}}
		}
	}`)
	title, ok := find(s, "marks", "row_header")["title"].(map[string]any)
	if !ok {
		t.Fatal("no label of the header")
	}
	want := map[string]string{
		"text":     `{"signal":"parent[\"h\"] == 0 ? 'Zero' : (isValid(parent[\"h\"]) ? parent[\"h\"] : \"\"+parent[\"h\"])"}`,
		"angle":    `0`,
		"align":    `"left"`,
		"baseline": `"middle"`,
		"offset":   `2`,
	}
	for k, w := range want {
		if got := asJSON(title[k]); got != w {
			t.Errorf("the %s of the label = %s, want %s", k, got, w)
		}
	}
}

// A point whose x and y are both binned can fill the step of the bins
// (circle_binned_maxbins_2), in a concatenation too (hconcat_weather). The
// smallest point is 4 square pixels, so that a point of the least value
// shows (config_numberFormatType_test).
func TestBinnedSizeRange(t *testing.T) {
	t.Parallel()
	const unit = `"mark": "circle", "encoding": {
		"x": {"bin": {"maxbins": 2}, "field": "a"},
		"y": {"bin": {"maxbins": 2}, "field": "b"},
		"size": {"aggregate": "count"}
	}`
	want := `pow(0.95 * min(%w / ((bin_maxbins_2_a_bins.stop - bin_maxbins_2_a_bins.start) / bin_maxbins_2_a_bins.step), %h / ((bin_maxbins_2_b_bins.stop - bin_maxbins_2_b_bins.start) / bin_maxbins_2_b_bins.step)), 2)`
	for _, c := range []struct{ name, spec, w, h string }{
		{"unit", `{"data": {"values": [{"a": 1, "b": 2}]}, ` + unit + `}`, "width", "height"},
		{"concat", `{"data": {"values": [{"a": 1, "b": 2}]}, "hconcat": [{"mark": "point", "encoding": {"x": {"field": "a", "type": "quantitative"}}}, {` + unit + `}]}`, "concat_1_width", "concat_1_height"},
	} {
		s := compileMap(t, c.spec)
		got := asJSON(find(s, "scales", "size")["range"])
		w := strings.NewReplacer("%w", c.w, "%h", c.h).Replace(want)
		if got != `[4,{"signal":"`+w+`"}]` {
			t.Errorf("%s: the range of size = %s, want the signal %s", c.name, got, w)
		}
	}
}

// A text with a condition and no field or value writes the text where the
// condition holds, and nothing elsewhere. A value {"expr": ...} of a
// condition is a signal (deneb waffle-charts).
func TestTextCondition(t *testing.T) {
	t.Parallel()
	s := compileMap(t, `{
		"data": {"values": [{"s": 1, "p": 40}]},
		"mark": "text",
		"encoding": {"text": {"condition": {"test": "datum.s == 1", "value": {"expr": "datum.p + '%'"}}}}
	}`)
	got := asJSON(update(find(s, "marks", "marks"))["text"])
	if want := `[{"signal":"datum.p + '%'","test":"datum.s == 1"}]`; got != want {
		t.Errorf("text = %s, want %s", got, want)
	}
}

// A layer that inherits a channel with no field, such as the scale of y,
// drops it, so the y of its mark places it (bar_layered_weather).
func TestInheritedEmptyChannel(t *testing.T) {
	t.Parallel()
	s := compileMap(t, `{
		"data": {"values": [{"id": 1, "lo": 10, "hi": 20}]},
		"encoding": {
			"x": {"field": "id", "type": "ordinal"},
			"y": {"type": "quantitative", "scale": {"domain": [0, 30]}}
		},
		"layer": [
			{"mark": "bar", "encoding": {"y": {"field": "lo"}, "y2": {"field": "hi"}}},
			{"mark": {"type": "text", "y": -5}, "encoding": {"text": {"field": "id"}}}
		]
	}`)
	if got := asJSON(update(find(s, "marks", "layer_1_marks"))["y"]); got != `{"value":-5}` {
		t.Errorf("the y of the text = %s, want the value -5", got)
	}
}

// An order with a sort and no field still sorts the stack
// (stacked_bar_v_ascending).
func TestOrderSortOnly(t *testing.T) {
	t.Parallel()
	s := compileMap(t, `{
		"data": {"values": [{"v": "a", "s": "x", "y": 1}]},
		"mark": "bar",
		"encoding": {
			"x": {"field": "v"},
			"y": {"aggregate": "sum", "field": "y"},
			"color": {"field": "s"},
			"order": {"sort": "ascending"}
		}
	}`)
	if got := asJSON(s["data"]); !strings.Contains(got, `"sort":{"field":["s"],"order":["ascending"]}`) {
		t.Errorf("the data = %s, want a stack sorted ascending", got)
	}
}
