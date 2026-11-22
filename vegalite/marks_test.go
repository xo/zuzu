package vegalite

import (
	"encoding/json/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/xo/zuzu/vega"
)

// findMark returns the mark of a name, in the marks or in their groups.
func findMark(marks []vega.Mark, name string) *vega.Mark {
	for i := range marks {
		if marks[i].Name == name {
			return &marks[i]
		}
		if m := findMark(marks[i].Marks, name); m != nil {
			return m
		}
	}
	return nil
}

// TestCompilePositions tests the positions and the sizes of the marks that
// the compiler writes.
func TestCompilePositions(t *testing.T) {
	t.Parallel()
	const ohlc = `"data": {"values": [
		{"date": "2009-06-01", "open": 28.7, "close": 30, "low": 28.3, "high": 30.1},
		{"date": "2009-06-02", "open": 30, "close": 29.6, "low": 29.2, "high": 30.4}
	]}`
	tests := []struct {
		name string
		spec string
		mark string
		// want holds the JSON of each property of the update of the mark,
		// in any order of keys, and an empty string for a property that
		// must not be set.
		want map[string]string
	}{
		{
			// A bar on a continuous x has no band, so it is centered on its
			// value, 5 pixels wide by default.
			"bar on a time x", `{` + ohlc + `,
				"mark": "bar",
				"encoding": {
					"x": {"field": "date", "type": "temporal"},
					"y": {"field": "open", "type": "quantitative"},
					"y2": {"field": "close"}
				}
			}`, "marks",
			map[string]string{
				"xc":    `{"scale":"x","field":"date"}`,
				"width": `{"value":5}`,
				"x":     "",
			},
		},
		{
			"bar on a time x with a width", `{` + ohlc + `,
				"mark": {"type": "bar", "width": 7},
				"encoding": {
					"x": {"field": "date", "type": "temporal"},
					"y": {"field": "open", "type": "quantitative"},
					"y2": {"field": "close"}
				}
			}`, "marks",
			map[string]string{"xc": `{"scale":"x","field":"date"}`, "width": `{"value":7}`},
		},
		{
			"bar on a time x with a size", `{` + ohlc + `,
				"mark": {"type": "bar", "size": 4},
				"encoding": {
					"x": {"field": "date", "type": "temporal"},
					"y": {"field": "open", "type": "quantitative"},
					"y2": {"field": "close"}
				}
			}`, "marks",
			map[string]string{"width": `{"value":4}`},
		},
		{
			"bar on a time x with the config", `{` + ohlc + `,
				"mark": "bar",
				"encoding": {
					"x": {"field": "date", "type": "temporal"},
					"y": {"field": "open", "type": "quantitative"}
				},
				"config": {"bar": {"continuousBandSize": 3}}
			}`, "marks",
			map[string]string{"width": `{"value":3}`},
		},
		{
			// A stacked bar on a quantitative x is centered on its value too.
			"stacked bar on a quantitative x", `{
				"data": {"values": [{"a": 1, "b": 28}, {"a": 1, "b": 55}, {"a": 5, "b": 43}]},
				"mark": "bar",
				"encoding": {
					"x": {"field": "a", "type": "quantitative"},
					"y": {"field": "b", "type": "quantitative"}
				}
			}`, "marks",
			map[string]string{
				"xc":    `{"scale":"x","field":"a"}`,
				"width": `{"value":5}`,
				"y":     `{"scale":"y","field":"b_end"}`,
				"y2":    `{"scale":"y","field":"b_start"}`,
			},
		},
		{
			"horizontal bar on a quantitative y", `{
				"data": {"values": [{"a": 1, "b": 28}, {"a": 5, "b": 43}]},
				"mark": "bar",
				"encoding": {
					"y": {"field": "a", "type": "quantitative"},
					"x": {"field": "b", "type": "quantitative", "aggregate": "sum"}
				}
			}`, "marks",
			map[string]string{"yc": `{"scale":"y","field":"a"}`, "height": `{"value":5}`},
		},
		{
			// A height relative to the band takes that share of the band,
			// around its middle.
			"bar with a height relative to the band", `{
				"data": {"values": [{"a": "A", "b": 28}, {"a": "B", "b": 55}]},
				"mark": {"type": "bar", "height": {"band": 0.5}},
				"encoding": {
					"y": {"field": "a", "type": "nominal"},
					"x": {"field": "b", "type": "quantitative"}
				}
			}`, "marks",
			map[string]string{
				"y":      `{"scale":"y","field":"a","band":0.25}`,
				"height": `{"signal":"max(0.25, 0.5 * bandwidth('y'))"}`,
			},
		},
		{
			// The outliers of a boxplot come before the box, which turns the
			// scale into bands. They sit in the middle of the band.
			"outliers of a boxplot", `{
				"data": {"values": [
					{"s": "A", "v": 1}, {"s": "A", "v": 2}, {"s": "A", "v": 3}, {"s": "A", "v": 30},
					{"s": "B", "v": 2}, {"s": "B", "v": 3}, {"s": "B", "v": 4}
				]},
				"mark": "boxplot",
				"encoding": {
					"x": {"field": "s", "type": "nominal"},
					"y": {"field": "v", "type": "quantitative"}
				}
			}`, "layer_0_layer_0_marks",
			map[string]string{"x": `{"scale":"x","field":"s","band":0.5}`},
		},
		{
			// A point on a scale that stays a point scale has no band.
			"point on a point scale", `{
				"data": {"values": [{"s": "A", "v": 1}]},
				"mark": "point",
				"encoding": {
					"x": {"field": "s", "type": "nominal"},
					"y": {"field": "v", "type": "quantitative"}
				}
			}`, "marks",
			map[string]string{"x": `{"scale":"x","field":"s"}`},
		},
		{
			// A rule from x to x2 with no y is in the middle of the height.
			// It does not run from the top to the bottom.
			"rule with x and x2", `{
				"data": {"values": [{"lo": 1, "hi": 3}]},
				"mark": "rule",
				"encoding": {
					"x": {"field": "lo", "type": "quantitative"},
					"x2": {"field": "hi"}
				}
			}`, "marks",
			map[string]string{
				"x2": `{"scale":"x","field":"hi"}`,
				"y":  `{"signal":"height","mult":0.5}`,
				"y2": "",
			},
		},
		{
			"rule with y and y2", `{
				"data": {"values": [{"lo": 1, "hi": 3}]},
				"mark": "rule",
				"encoding": {
					"y": {"field": "lo", "type": "quantitative"},
					"y2": {"field": "hi"}
				}
			}`, "marks",
			map[string]string{
				"y2": `{"scale":"y","field":"hi"}`,
				"x":  `{"signal":"width","mult":0.5}`,
				"x2": "",
			},
		},
		{
			// Each condition of a list is a rule, in the order of the spec.
			"list of conditions", `{
				"data": {"values": [{"a": "A", "b": 28}]},
				"mark": "bar",
				"encoding": {
					"x": {"field": "a", "type": "nominal"},
					"y": {"field": "b", "type": "quantitative"},
					"color": {
						"condition": [
							{"test": "datum.a === 'A'", "value": "#f7e0b6"},
							{"test": "datum.b < 0", "value": "#f78a64"}
						],
						"value": "#93c4aa"
					}
				}
			}`, "marks",
			map[string]string{
				"fill": `[{"test":"datum.a === 'A'","value":"#f7e0b6"},{"test":"datum.b < 0","value":"#f78a64"},{"value":"#93c4aa"}]`,
			},
		},
		{
			// A color with conditions alone falls back to the color of a
			// mark of the theme, through a scale of one value, so a bar
			// whose condition does not hold keeps its color, as in
			// bar_grouped_thin.vl.json.
			"conditions alone", `{
				"data": {"values": [{"a": "A", "b": 28}]},
				"mark": "bar",
				"encoding": {
					"x": {"field": "a", "type": "nominal"},
					"y": {"field": "b", "type": "quantitative"},
					"color": {"condition": {"test": "datum.b < 0", "value": "#aaa"}}
				}
			}`, "marks",
			map[string]string{
				"fill": `[{"test":"datum.b < 0","value":"#aaa"},{"scale":"zuzu_mark_color","value":"zuzu_mark_color"}]`,
			},
		},
		{
			// The color of the mark, then of the config of the mark, wins
			// over the color of the theme as the fallback.
			"conditions alone with the color of the mark", `{
				"data": {"values": [{"a": "A", "b": 28}]},
				"mark": {"type": "bar", "color": "red"},
				"encoding": {
					"x": {"field": "a", "type": "nominal"},
					"y": {"field": "b", "type": "quantitative"},
					"color": {"condition": {"test": "datum.b < 0", "value": "#aaa"}}
				}
			}`, "marks",
			map[string]string{
				"fill": `[{"test":"datum.b < 0","value":"#aaa"},{"value":"red"}]`,
			},
		},
		{
			"conditions alone with the color of the config", `{
				"data": {"values": [{"a": "A", "b": 28}]},
				"mark": "bar",
				"encoding": {
					"x": {"field": "a", "type": "nominal"},
					"y": {"field": "b", "type": "quantitative"},
					"color": {"condition": {"test": "datum.b < 0", "value": "#aaa"}}
				},
				"config": {"bar": {"color": "green"}}
			}`, "marks",
			map[string]string{
				"fill": `[{"test":"datum.b < 0","value":"#aaa"},{"value":"green"}]`,
			},
		},
		{
			// A view that keeps invalid values puts an invalid value at
			// zero of its scale, as vega-lite.js writes it, so a bar of a
			// mean of no values has no height.
			"invalid values kept", `{
				"data": {"values": [{"a": "A", "b": 28}]},
				"mark": "bar",
				"encoding": {
					"x": {"field": "a", "type": "nominal"},
					"y": {"field": "b", "type": "quantitative", "stack": null}
				},
				"config": {"mark": {"invalid": null}}
			}`, "marks",
			map[string]string{
				"y": `[{"test":"!isValid(datum[\"b\"]) || !isFinite(+datum[\"b\"])","scale":"y","value":0},{"scale":"y","field":"b"}]`,
			},
		},
		{
			// When the domain can leave out zero, an invalid value goes
			// to the bottom of the view.
			"invalid values kept with no zero", `{
				"data": {"values": [{"a": "A", "b": 28}]},
				"mark": {"type": "point", "invalid": null},
				"encoding": {
					"x": {"field": "a", "type": "nominal"},
					"y": {"field": "b", "type": "quantitative", "scale": {"zero": false}}
				}
			}`, "marks",
			map[string]string{
				"y": `[{"test":"!isValid(datum[\"b\"]) || !isFinite(+datum[\"b\"])","field":{"group":"height"}},{"scale":"y","field":"b"}]`,
			},
		},
		{
			// A rule from one band to another runs between their middles,
			// and the offsets of the mark move its ends.
			"rule between bands with offsets", `{
				"data": {"values": [{"a": "A", "b": "B", "v": 1}]},
				"mark": {"type": "rule", "xOffset": -2, "x2Offset": 2},
				"encoding": {
					"x": {"field": "a", "type": "nominal"},
					"x2": {"field": "b"},
					"y": {"field": "v", "type": "quantitative"}
				}
			}`, "marks",
			map[string]string{
				"x":       `{"scale":"x","field":"a","band":0.5,"offset":-2}`,
				"x2":      `{"scale":"x","field":"b","band":0.5,"offset":2}`,
				"xOffset": "",
			},
		},
		{
			// A bar from one band to another, such as a lane of a heat
			// lane, runs between the middles of the bands.
			"bar from band to band", `{"data": {"values": [{"a": 1, "b": 2, "c": "p", "d": "q"}]},
				"mark": "bar",
				"encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "c"}, "y2": {"field": "d"}}
			}`, "marks",
			map[string]string{
				"y":      `{"scale":"y","field":"c","band":0.5}`,
				"y2":     `{"scale":"y","field":"d","band":0.5}`,
				"height": "",
			},
		},
		{
			// On a point scale the bar runs between the points.
			"bar from point to point", `{"data": {"values": [{"a": 1, "b": 2, "c": "p", "d": "q"}]},
				"mark": "bar",
				"encoding": {"x": {"field": "c", "type": "ordinal", "scale": {"type": "point"}}, "x2": {"field": "d"}, "y": {"field": "a", "type": "quantitative"}}
			}`, "marks",
			map[string]string{
				"x":     `{"scale":"x","field":"c"}`,
				"x2":    `{"scale":"x","field":"d"}`,
				"width": "",
			},
		},
		{
			// A binned bar leaves a gap of 1 pixel, its binSpacing.
			"binned bar", `{"data": {"values": [{"a": 1, "b": 2, "c": "p", "d": "q"}]},
				"mark": "bar",
				"encoding": {"x": {"field": "a", "bin": true}, "y": {"aggregate": "count"}}
			}`, "marks",
			map[string]string{
				"x":  `{"scale":"x","field":"bin_maxbins_10_a","offset":0.5}`,
				"x2": `{"scale":"x","field":"bin_maxbins_10_a_end","offset":-0.5}`,
			},
		},
		{
			// The range of y runs up, so the gap of a binned y turns
			// around, and the binSpacing of the spec sets it.
			"binned y bar with a binSpacing", `{"data": {"values": [{"a": 1, "b": 2, "c": "p", "d": "q"}]},
				"mark": {"type": "bar", "binSpacing": 3},
				"encoding": {"y": {"field": "a", "bin": true}, "x": {"aggregate": "count"}}
			}`, "marks",
			map[string]string{
				"y":  `{"scale":"y","field":"bin_maxbins_10_a","offset":-1.5}`,
				"y2": `{"scale":"y","field":"bin_maxbins_10_a_end","offset":1.5}`,
			},
		},
		{
			// A reversed scale turns the gap around.
			"binned bar on a reversed scale", `{"data": {"values": [{"a": 1, "b": 2, "c": "p", "d": "q"}]},
				"mark": "bar",
				"encoding": {"x": {"field": "a", "bin": true, "scale": {"reverse": true}}, "y": {"aggregate": "count"}}
			}`, "marks",
			map[string]string{
				"x":  `{"scale":"x","field":"bin_maxbins_10_a","offset":-0.5}`,
				"x2": `{"scale":"x","field":"bin_maxbins_10_a_end","offset":0.5}`,
			},
		},
		{
			// A binSpacing of 0, and a rect, leave no gap.
			"binned bar with no spacing", `{"data": {"values": [{"a": 1, "b": 2, "c": "p", "d": "q"}]},
				"mark": {"type": "bar", "binSpacing": 0},
				"encoding": {"x": {"field": "a", "bin": true}, "y": {"aggregate": "count"}}
			}`, "marks",
			map[string]string{
				"x":  `{"scale":"x","field":"bin_maxbins_10_a"}`,
				"x2": `{"scale":"x","field":"bin_maxbins_10_a_end"}`,
			},
		},
		{
			"bar over a time unit on y", `{"data": {"values": [{"a": 1, "b": 2, "c": "p", "d": "q"}]},
				"mark": "bar",
				"encoding": {"y": {"field": "a", "type": "temporal", "timeUnit": "binnedutcyearmonthdate"}, "x": {"field": "b", "type": "quantitative"}}
			}`, "marks",
			map[string]string{
				"y":  `{"scale":"y","field":"a","offset":-0.5}`,
				"y2": `{"scale":"y","field":"a_end","offset":0.5}`,
			},
		},
		{
			// A value that an expression gives is a signal.
			"rule at an expression", `{"data": {"values": [{"a": 1, "b": 2, "c": "p", "d": "q"}]},
				"mark": "rule",
				"encoding": {"x": {"value": {"expr": "width / 2"}}}
			}`, "marks",
			map[string]string{
				"x": `{"signal":"width / 2"}`,
			},
		},
		{
			// A tick with no y sits in the middle of the view, three
			// quarters of its height.
			"tick with no y", `{"data": {"values": [{"a": 1, "b": 2, "c": "p", "d": "q"}]},
				"mark": "tick",
				"encoding": {"x": {"field": "a", "type": "quantitative"}}
			}`, "marks",
			map[string]string{
				"yc":     `{"signal":"height","mult":0.5}`,
				"height": `{"signal":"0.75 * height"}`,
				"y":      "",
			},
		},
		{
			// A tick on a band fills it, as a bar does.
			"tick on a band", `{"data": {"values": [{"a": 1, "b": 2, "c": "p", "d": "q"}]},
				"mark": "tick",
				"encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "c", "type": "ordinal"}}
			}`, "marks",
			map[string]string{
				"y":      `{"scale":"y","field":"c"}`,
				"height": `{"signal":"max(0.25, bandwidth('y'))"}`,
			},
		},
		{
			// A tick across a continuous x is centered on its value, 5
			// pixels wide.
			"tick on a continuous x", `{"data": {"values": [{"a": 1, "b": 2, "c": "p", "d": "q"}]},
				"mark": "tick",
				"encoding": {"x": {"field": "a", "type": "temporal"}, "y": {"field": "b", "type": "quantitative"}}
			}`, "marks",
			map[string]string{
				"xc":    `{"scale":"x","field":"a"}`,
				"width": `{"value":5}`,
			},
		},
		{
			// A bar with a size and no y sits in the middle of the view.
			"bar with a size and no y", `{"data": {"values": [{"a": 1, "b": 2, "c": "p", "d": "q"}]},
				"mark": {"type": "bar", "size": 14},
				"encoding": {"x": {"field": "a", "type": "quantitative"}}
			}`, "marks",
			map[string]string{
				"yc":     `{"signal":"height","mult":0.5}`,
				"height": `{"value":14}`,
			},
		},
		{
			// A cornerRadiusEnd of 0 reaches the encode, so the ends are
			// square.
			"bar with a cornerRadiusEnd", `{"data": {"values": [{"a": 1, "b": 2, "c": "p", "d": "q"}]},
				"mark": {"type": "bar", "cornerRadiusEnd": 0},
				"encoding": {"x": {"field": "c", "type": "nominal"}, "y": {"field": "a", "type": "quantitative", "stack": null}}
			}`, "marks",
			map[string]string{
				"cornerRadiusTopLeft":     `[{"test":"scale(\"y\", datum[\"a\"]) < scale(\"y\", 0)","value":0},{"value":0}]`,
				"cornerRadiusBottomRight": `[{"test":"scale(\"y\", datum[\"a\"]) > scale(\"y\", 0)","value":0},{"value":0}]`,
			},
		},
		{
			// A gradient is the color of a mark.
			"area with a gradient", `{"data": {"values": [{"a": 1, "b": 2, "c": "p", "d": "q"}]},
				"mark": {"type": "area", "color": {"gradient": "linear", "stops": [{"offset": 0, "color": "white"}, {"offset": 1, "color": "red"}]}},
				"encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "b", "type": "quantitative"}}
			}`, "marks",
			map[string]string{
				"fill": `{"value":{"gradient":"linear","stops":[{"offset":0,"color":"white"},{"offset":1,"color":"red"}]}}`,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			s := compile(t, test.spec)
			m := findMark(s.Marks, test.mark)
			if m == nil {
				t.Fatalf("no mark %s", test.mark)
			}
			update := m.Encode["update"]
			for k, want := range test.want {
				p, ok := update[k]
				if want == "" {
					if ok {
						t.Errorf("%s is set, want it unset", k)
					}
					continue
				}
				if !ok {
					t.Errorf("%s is not set, want %s", k, want)
					continue
				}
				got, err := json.Marshal(p)
				if err != nil {
					t.Fatal(err)
				}
				var g, w any
				if err := json.Unmarshal(got, &g); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(want), &w); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(g, w) {
					t.Errorf("%s = %s, want %s", k, got, want)
				}
			}
		})
	}
}

// TestCompileSizeIssues tests that a size that the compiler does not draw
// is an issue.
func TestCompileSizeIssues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		spec string
	}{
		{"band on a continuous scale", `{
			"data": {"values": [{"a": 1, "b": 2}]},
			"mark": {"type": "bar", "width": {"band": 0.5}},
			"encoding": {
				"x": {"field": "a", "type": "quantitative"},
				"y": {"field": "b", "type": "quantitative"}
			}
		}`},
		{"fixed width over a time unit", `{
			"data": {"values": [{"a": "2020-01-01", "b": 2}]},
			"mark": {"type": "bar", "width": 5},
			"encoding": {
				"x": {"field": "a", "type": "temporal", "timeUnit": "month"},
				"y": {"field": "b", "type": "quantitative"}
			}
		}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			s, err := Parse([]byte(test.spec))
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := Compile(s, WithStrict(true)); err == nil {
				t.Error("strict Compile succeeded, want an issue")
			}
			if _, report, err := Compile(s); err != nil || len(report) == 0 {
				t.Errorf("loose Compile = %v, %v, want a report", report, err)
			}
		})
	}
}

// compileMap compiles a spec of JSON and returns the Vega spec as JSON
// values, for a test that reads a few parts of it.
func compileMap(t *testing.T, src string) map[string]any {
	t.Helper()
	b, err := json.Marshal(compile(t, src), json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// find returns the first object of a list in m, at the key list, whose
// name is name, or nil.
func find(m map[string]any, list, name string) map[string]any {
	l, _ := m[list].([]any)
	for _, x := range l {
		if o, ok := x.(map[string]any); ok && o["name"] == name {
			return o
		}
		if o, ok := x.(map[string]any); ok {
			if r := find(o, list, name); r != nil {
				return r
			}
		}
	}
	return nil
}

// update returns the update set of the encoding of a mark.
func update(m map[string]any) map[string]any {
	e, _ := m["encode"].(map[string]any)
	u, _ := e["update"].(map[string]any)
	return u
}

// asJSON writes a value as JSON, for a message.
func asJSON(v any) string {
	b, _ := json.Marshal(v, json.Deterministic(true))
	return string(b)
}

// A discrete y puts its first value at the top, with a fixed height as
// with a step.
func TestDiscreteYTop(t *testing.T) {
	t.Parallel()
	for _, typ := range []string{"nominal", "ordinal"} {
		s := compileMap(t, `{
			"height": 200,
			"data": {"values": [{"a": "A", "b": 1}, {"a": "B", "b": 2}]},
			"mark": "bar",
			"encoding": {"y": {"field": "a", "type": "`+typ+`"}, "x": {"field": "b", "type": "quantitative"}}
		}`)
		y := find(s, "scales", "y")
		if got, want := asJSON(y["range"]), `[0,{"signal":"height"}]`; got != want {
			t.Errorf("%s: the range of y = %s, want %s", typ, got, want)
		}
	}
	// A continuous y keeps its low values at the bottom.
	s := compileMap(t, `{
		"height": 200,
		"data": {"values": [{"a": "A", "b": 1}]},
		"mark": "bar",
		"encoding": {"x": {"field": "a", "type": "nominal"}, "y": {"field": "b", "type": "quantitative"}}
	}`)
	if got, want := asJSON(find(s, "scales", "y")["range"]), `[{"signal":"height"},0]`; got != want {
		t.Errorf("the range of a quantitative y = %s, want %s", got, want)
	}
}

// A binned theta spans each arc from the start of its bin to its end, and
// a text of an arc reads theta and radius at the middle of its interval.
func TestPolarText(t *testing.T) {
	t.Parallel()
	s := compileMap(t, `{
		"data": {"values": [{"r": 1.5}, {"r": 2.5}, {"r": 2.7}]},
		"layer": [
			{"mark": "arc"},
			{"mark": {"type": "text", "radiusOffset": 15}, "encoding": {
				"text": {"bin": true, "field": "r"},
				"angle": {"bin": true, "field": "r", "scale": {"range": [90, 450]}}
			}}
		],
		"encoding": {
			"theta": {"bin": true, "field": "r"},
			"radius": {"aggregate": "count", "scale": {"type": "sqrt", "rangeMin": 20}}
		}
	}`)
	arc := update(find(s, "marks", "layer_0_marks"))
	if arc["startAngle"] == nil || arc["endAngle"] == nil {
		t.Fatalf("the arc of a bin has no start and end angle: %s", asJSON(arc))
	}
	if a, b := asJSON(arc["startAngle"]), asJSON(arc["endAngle"]); a == b {
		t.Errorf("the arc starts and ends at %s", a)
	}
	text := update(find(s, "marks", "layer_1_marks"))
	mid := `scale(\"theta\", 0.5 * datum[\"bin_maxbins_10_r\"] + 0.5 * datum[\"bin_maxbins_10_r_end\"])`
	if got := asJSON(text["theta"]); !strings.Contains(got, mid) {
		t.Errorf("theta of the text = %s, want the middle of the bin", got)
	}
	if got, want := asJSON(text["radius"]), `{"field":"__count","offset":15,"scale":"radius"}`; got != want {
		t.Errorf("radius of the text = %s, want %s", got, want)
	}
	if got := asJSON(text["angle"]); !strings.Contains(got, `scale(\"angle\", 0.5 *`) {
		t.Errorf("angle of the text = %s, want the middle of the bin", got)
	}
	if got := asJSON(text["text"]); !strings.Contains(got, ` – `) {
		t.Errorf("the text of a bin = %s, want its range", got)
	}
	if _, ok := text["radiusOffset"]; ok {
		t.Error("radiusOffset passes on to Vega, which does not have it")
	}
	if got := asJSON(find(s, "scales", "radius")["range"]); !strings.HasPrefix(got, "[20,") {
		t.Errorf("the range of radius = %s, want rangeMin 20 at its start", got)
	}
	if l, ok := s["legends"].([]any); ok && len(l) > 0 {
		t.Errorf("the angle has a legend: %s", asJSON(l))
	}
	// A stack places the text at the middle of its slice, and the property
	// radius places it away from the center.
	p := compileMap(t, `{
		"data": {"values": [{"c": "a", "v": 4}, {"c": "b", "v": 6}]},
		"encoding": {"theta": {"field": "v", "type": "quantitative", "stack": true}},
		"layer": [{"mark": "arc"}, {"mark": {"type": "text", "radius": 90}, "encoding": {"text": {"field": "c"}}}]
	}`)
	pt := update(find(p, "marks", "layer_1_marks"))
	if got := asJSON(pt["theta"]); !strings.Contains(got, `0.5 * datum[\"v_start\"] + 0.5 * datum[\"v_end\"]`) {
		t.Errorf("theta of the label of a slice = %s, want the middle of its stack", got)
	}
	if got := asJSON(pt["radius"]); got != `{"value":90}` {
		t.Errorf("radius of the label of a slice = %s, want 90", got)
	}
}

// A text is centered on its point, unless its mark, the config or a style
// sets its align or its baseline. A style of the config passes on to Vega,
// so the mark writes nothing. The config of the text and of every mark do
// not pass on, so the mark writes their value.
func TestTextAnchor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, mark, config string
		align, baseline    string
	}{
		{"default", `"text"`, `{}`, `{"value":"center"}`, `{"value":"middle"}`},
		{"mark baseline", `{"type": "text", "baseline": "top"}`, `{}`, `{"value":"center"}`, `{"value":"top"}`},
		{"mark align", `{"type": "text", "align": "right"}`, `{}`, `{"value":"right"}`, `{"value":"middle"}`},
		{"mark expr", `{"type": "text", "align": {"expr": "'left'"}}`, `{}`, `{"signal":"'left'"}`, `{"value":"middle"}`},
		{"config text", `"text"`, `{"text": {"align": "left"}}`, `{"value":"left"}`, `{"value":"middle"}`},
		{"config mark", `"text"`, `{"mark": {"baseline": "top"}}`, `{"value":"center"}`, `{"value":"top"}`},
		{"config text over mark", `"text"`, `{"mark": {"align": "right"}, "text": {"align": "left"}}`, `{"value":"left"}`, `{"value":"middle"}`},
		{"style named", `{"type": "text", "style": "label"}`, `{"style": {"label": {"align": "right"}}}`, `null`, `{"value":"middle"}`},
		{"style list", `{"type": "text", "style": ["a", "b"]}`, `{"style": {"b": {"baseline": "top"}}}`, `{"value":"center"}`, `null`},
		{"style text", `"text"`, `{"style": {"text": {"align": "right"}}}`, `null`, `{"value":"middle"}`},
		{"style of another mark", `"text"`, `{"style": {"label": {"align": "right"}}}`, `{"value":"center"}`, `{"value":"middle"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := compileMap(t, `{
				"data": {"values": [{"a": 1}]},
				"mark": `+tt.mark+`,
				"encoding": {"text": {"field": "a", "type": "quantitative"}},
				"config": `+tt.config+`
			}`)
			u := update(find(s, "marks", "marks"))
			if got := asJSON(u["align"]); got != tt.align {
				t.Errorf("align = %s, want %s", got, tt.align)
			}
			if got := asJSON(u["baseline"]); got != tt.baseline {
				t.Errorf("baseline = %s, want %s", got, tt.baseline)
			}
		})
	}
	// A mark that is not a text gets neither.
	u := update(find(compileMap(t, `{
		"data": {"values": [{"a": 1}]},
		"mark": "point",
		"encoding": {"x": {"field": "a", "type": "quantitative"}}
	}`), "marks", "marks"))
	for _, k := range []string{"align", "baseline"} {
		if v, ok := u[k]; ok {
			t.Errorf("a point has %s %s", k, asJSON(v))
		}
	}
}

// The header of a time unit writes the value in the format of the unit, a
// facet in a concatenation keeps its title, and the spacing of the config
// sets the padding of the layout.
func TestConcatFacet(t *testing.T) {
	t.Parallel()
	s := compileMap(t, `{
		"data": {"values": [{"date": "2012-01-01", "p": 1}, {"date": "2013-02-01", "p": 2}]},
		"vconcat": [
			{
				"title": "Annual",
				"mark": "line",
				"encoding": {
					"x": {"field": "date", "timeUnit": "month", "type": "ordinal"},
					"y": {"field": "p", "aggregate": "mean"},
					"row": {"field": "date", "timeUnit": "year"}
				},
				"height": 30
			},
			{"mark": "line", "encoding": {"x": {"field": "date", "timeUnit": "month", "type": "ordinal"}, "y": {"field": "p", "aggregate": "mean"}}}
		],
		"config": {"facet": {"spacing": 5}, "concat": {"spacing": 12}}
	}`)
	group := find(s, "marks", "concat_0_group")
	if group == nil {
		t.Fatal("no group of the facet")
	}
	if got := asJSON(group["title"]); !strings.Contains(got, "Annual") {
		t.Errorf("the title of the facet = %s, want Annual", got)
	}
	if got := asJSON(group["layout"]); !strings.Contains(got, `"padding":5`) {
		t.Errorf("the layout of the facet = %s, want the padding 5 of config.facet.spacing", got)
	}
	if got := asJSON(s["layout"]); !strings.Contains(got, `"padding":12`) {
		t.Errorf("the layout of the concatenation = %s, want the padding 12 of config.concat.spacing", got)
	}
	header := find(s, "marks", "concat_0_row_header")
	if got := asJSON(header["title"]); !strings.Contains(got, `timeFormat(parent[\"year_date\"], \"%Y\")`) {
		t.Errorf("the label of the header = %s, want the year", got)
	}
}
