package vegalite

import (
	"encoding/json/v2"
	"reflect"
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
