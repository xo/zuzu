package vegalite

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// compile compiles a spec of JSON.
func compile(t *testing.T, src string) *vega.Spec {
	t.Helper()
	s, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, report, err := Compile(s, WithStrict(true))
	if err != nil {
		t.Fatalf("Compile: %v (report %v)", err, report)
	}
	return out
}

// names returns the names of the datasets, the scales and the marks.
func names(s *vega.Spec) (data, scales, marks []string) {
	for _, d := range s.Data {
		data = append(data, d.Name)
	}
	for _, sc := range s.Scales {
		scales = append(scales, sc.Name+":"+sc.Type)
	}
	for _, m := range s.Marks {
		marks = append(marks, m.Name+":"+m.Type)
	}
	return data, scales, marks
}

func TestCompileBar(t *testing.T) {
	t.Parallel()
	s := compile(t, `{
		"data": {"values": [{"a": "A", "b": 28}, {"a": "B", "b": 55}]},
		"mark": "bar",
		"encoding": {
			"x": {"field": "a", "type": "nominal"},
			"y": {"field": "b", "type": "quantitative"}
		}
	}`)
	data, scales, marks := names(s)
	if !slices.Equal(data, []string{"source_0", "data_0"}) {
		t.Errorf("data = %v", data)
	}
	if !slices.Equal(scales, []string{"x:band", "y:linear"}) {
		t.Errorf("scales = %v", scales)
	}
	if !slices.Equal(marks, []string{"marks:rect"}) {
		t.Errorf("marks = %v", marks)
	}
	// The bars stack, as Vega-Lite stacks a bar of raw data.
	if ts := s.Data[1].Transform; len(ts) < 1 || ts[0].Type != "stack" {
		t.Errorf("the transforms of data_0 = %v, want a stack first", ts)
	}
	var steps []string
	for _, sig := range s.Signals {
		steps = append(steps, sig.Name)
	}
	if !slices.Equal(steps, []string{"x_step", "width"}) {
		t.Errorf("signals = %v, want x_step and width", steps)
	}
}

func TestCompileAggregate(t *testing.T) {
	t.Parallel()
	s := compile(t, `{
		"data": {"url": "data/cars.json"},
		"mark": "bar",
		"encoding": {
			"x": {"bin": true, "field": "Horsepower"},
			"y": {"aggregate": "count"},
			"color": {"field": "Origin"}
		}
	}`)
	if len(s.Data) != 1 || s.Data[0].Format == nil || s.Data[0].Format.Type != "json" {
		t.Fatalf("data = %+v, want one source of json", s.Data)
	}
	var types []string
	for _, tr := range s.Data[0].Transform {
		types = append(types, tr.Type)
	}
	if want := []string{"extent", "bin", "aggregate", "stack", "filter"}; !slices.Equal(types, want) {
		t.Errorf("transforms = %v, want %v", types, want)
	}
	var as []string
	const count = "__count"
	if err := json.Unmarshal(s.Data[0].Transform[2].Params["as"], &as); err != nil || as[0] != count {
		t.Errorf("the aggregate is named %v, want %s", as, count)
	}
	if len(s.Legends) != 1 || s.Legends[0].Fill != "color" {
		t.Errorf("legends = %+v, want one of fill", s.Legends)
	}
}

func TestCompileLayer(t *testing.T) {
	t.Parallel()
	s := compile(t, `{
		"data": {"values": [{"a": "A", "b": 28}]},
		"encoding": {"x": {"field": "a", "type": "ordinal"}},
		"layer": [
			{"mark": "bar", "encoding": {"y": {"field": "b", "type": "quantitative"}}},
			{"mark": "line", "encoding": {"y": {"field": "b", "type": "quantitative"}}}
		]
	}`)
	_, scales, marks := names(s)
	if !slices.Equal(marks, []string{"layer_0_marks:rect", "layer_1_marks:line"}) {
		t.Errorf("marks = %v", marks)
	}
	if !slices.Equal(scales, []string{"x:band", "y:linear"}) {
		t.Errorf("scales = %v, want the layers to share x and y", scales)
	}
}

func TestCompileUnsupported(t *testing.T) {
	t.Parallel()
	s, err := Parse([]byte(`{"data": {"values": []}, "mark": "boxplot", "encoding": {}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Compile(s, WithStrict(true)); err == nil {
		t.Error("strict Compile of a boxplot succeeded, want an issue")
	}
	if _, report, err := Compile(s); err != nil || len(report) == 0 {
		t.Errorf("loose Compile = %v, %v, want a report", report, err)
	}
}

func TestTimeUnits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		units []string
		utc   bool
	}{
		{"yearmonthdate", []string{"year", "month", "date"}, false},
		{"utcmonth", []string{"month"}, true},
		{"dayofyear", []string{"dayofyear"}, false},
		{"hoursminutes", []string{"hours", "minutes"}, false},
	}
	for _, test := range tests {
		units, utc := timeUnits(test.name)
		if !slices.Equal(units, test.units) || utc != test.utc {
			t.Errorf("timeUnits(%q) = %v, %t, want %v, %t", test.name, units, utc, test.units, test.utc)
		}
	}
}

func TestCompileComposite(t *testing.T) {
	t.Parallel()
	s := compile(t, `{
		"data": {"url": "data/penguins.json"},
		"mark": {"type": "boxplot", "extent": "min-max"},
		"encoding": {
			"x": {"field": "Species", "type": "nominal"},
			"y": {"field": "Body Mass (g)", "type": "quantitative"}
		}
	}`)
	_, scales, marks := names(s)
	want := []string{"layer_0_marks:rule", "layer_1_marks:rule", "layer_2_marks:rect", "layer_3_marks:rect"}
	if !slices.Equal(marks, want) {
		t.Errorf("marks = %v, want %v", marks, want)
	}
	if !slices.Equal(scales, []string{"x:band", "y:linear"}) {
		t.Errorf("scales = %v", scales)
	}
	var as []string
	if err := json.Unmarshal(s.Data[0].Transform[0].Params["as"], &as); err != nil {
		t.Fatal(err)
	}
	if want := []string{"lower_box_Body Mass (g)", "upper_box_Body Mass (g)", "mid_box_Body Mass (g)", "lower_whisker_Body Mass (g)", "upper_whisker_Body Mass (g)"}; !slices.Equal(as, want) {
		t.Errorf("the aggregate gives %v, want %v", as, want)
	}
	e := compile(t, `{
		"data": {"url": "data/barley.json"},
		"mark": {"type": "errorbar", "ticks": true},
		"encoding": {"x": {"field": "yield", "type": "quantitative"}, "y": {"field": "variety", "type": "ordinal"}}
	}`)
	if _, _, marks := names(e); !slices.Equal(marks, []string{"layer_0_marks:rect", "layer_1_marks:rect", "layer_2_marks:rule"}) {
		t.Errorf("errorbar marks = %v, want two ticks and a rule", marks)
	}
}

// TestCompositeParts checks that the parts of a composite mark take the
// properties of the spec and of its config, and that each property the spec
// gives adds a style that tells the look to keep it. Before, the config of a
// box plot was dropped, and the look painted over a median that the spec
// colored. The ticks of an error bar take the color of the theme, as its
// rule does, rather than black.
func TestCompositeParts(t *testing.T) {
	t.Parallel()
	m := compileMap(t, `{
		"data": {"values": [{"s": "a", "v": 1}, {"s": "a", "v": 3}]},
		"mark": {"type": "boxplot", "extent": "min-max", "rule": {"strokeWidth": 3}},
		"encoding": {"x": {"field": "s", "type": "nominal"}, "y": {"field": "v", "type": "quantitative"}},
		"config": {"boxplot": {"median": {"color": "orange"}}}
	}`)
	median := find(m, "marks", "layer_3_marks")
	if got := asJSON(update(median)["fill"]); got != `{"value":"orange"}` {
		t.Errorf("the fill of the median is %s, want orange", got)
	}
	if st := asJSON(median["style"]); !strings.Contains(st, `"zuzu.own.color"`) {
		t.Errorf("the styles of the median are %s, want zuzu.own.color", st)
	}
	rule := find(m, "marks", "layer_0_marks")
	if got := asJSON(update(rule)["strokeWidth"]); got != `{"value":3}` {
		t.Errorf("the width of the rule is %s, want 3", got)
	}
	if st := asJSON(rule["style"]); !strings.Contains(st, `"zuzu.own.strokeWidth"`) {
		t.Errorf("the styles of the rule are %s, want zuzu.own.strokeWidth", st)
	}
	e := compileMap(t, `{
		"data": {"values": [{"s": "a", "v": 1}, {"s": "a", "v": 3}]},
		"mark": {"type": "errorbar", "ticks": true},
		"encoding": {"x": {"field": "v", "type": "quantitative"}, "y": {"field": "s", "type": "ordinal"}}
	}`)
	for _, name := range []string{"layer_0_marks", "layer_2_marks"} {
		u := update(find(e, "marks", name))
		if u["fill"] != nil || u["stroke"] != nil {
			t.Errorf("%s has a color of its own, %s, want the color of the theme", name, asJSON(u))
		}
	}
}

// TestReferenceRule checks that a rule across the plot takes the look of a
// reference line, unless the spec gives it a color, a width or a dash.
func TestReferenceRule(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		mark string
		want bool
	}{
		{`"rule"`, true},
		{`{"type": "rule", "stroke": "firebrick"}`, false},
		{`{"type": "rule", "strokeDash": [2, 2]}`, false},
	} {
		m := compileMap(t, `{"data": {"values": [{"a": 1}]}, "mark": `+c.mark+`, "encoding": {"x": {"field": "a", "type": "quantitative"}}}`)
		got := strings.Contains(asJSON(find(m, "marks", "marks")["style"]), "zuzu.reference")
		if got != c.want {
			t.Errorf("the rule %s is a reference: %v, want %v", c.mark, got, c.want)
		}
	}
}

// TestOverlayLine checks that the line over an area takes the properties
// that the spec gives it as an object.
func TestOverlayLine(t *testing.T) {
	t.Parallel()
	m := compileMap(t, `{"data": {"values": [{"a": 1, "b": 2}]},
		"mark": {"type": "area", "line": {"color": "darkgreen"}},
		"encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "b", "type": "quantitative"}}}`)
	if got := asJSON(update(find(m, "marks", "layer_1_marks"))["stroke"]); got != `{"value":"darkgreen"}` {
		t.Errorf("the stroke of the line is %s, want darkgreen", got)
	}
}

// TestCompileAxisProps checks that the axis of a channel passes on the
// properties that place its labels and its title, and those that style its
// parts, as vega-lite.js does: the grid ones to the axis of the grid, and
// zindex, bandPosition and tickBand to both axes. Before, each was an
// issue, so the labels of bar_axis_space_saving ran into the plot. A
// condition on a property stays an issue.
func TestCompileAxisProps(t *testing.T) {
	t.Parallel()
	s := compile(t, `{
		"data": {"values": [{"a": "A", "b": 28}]},
		"mark": "bar",
		"encoding": {
			"y": {"field": "a", "type": "nominal", "axis": {
				"grid": true, "bandPosition": 0, "zindex": 1, "labelAlign": "left",
				"labelBaseline": "middle", "labelPadding": -5, "labelOffset": -15,
				"labelExpr": "upper(datum.label)", "labelFontSize": 12, "labelFontWeight": "bold",
				"titleX": 5, "titleY": -5, "titleAngle": 0, "titleAlign": "left", "titleBaseline": "bottom",
				"titleAnchor": "start", "titleFontSize": 9, "tickSize": 4, "domainWidth": 2,
				"gridColor": "red", "gridDash": [2, 2], "titlePadding": {"expr": "1 + 2"},
				"tickBand": "extent", "tickRound": false
			}},
			"x": {"field": "b", "type": "quantitative"}
		}
	}`)
	var grid, main *vega.Axis
	for i := range s.Axes {
		if a := &s.Axes[i]; a.Scale == "y" && a.Grid != nil && *a.Grid {
			grid = a
		} else if a.Scale == "y" {
			main = a
		}
	}
	if grid == nil || main == nil {
		t.Fatalf("axes = %+v, want a grid and a main axis of y", s.Axes)
	}
	str := func(v any) string {
		t.Helper()
		b, err := json.Marshal(v, json.Deterministic(true))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	var m map[string]jsontext.Value
	if err := json.Unmarshal([]byte(str(main)), &m); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{
		"labelBaseline": `"middle"`, "labelPadding": `-5`, "bandPosition": `0`, "zindex": `1`,
		"labelOffset": `-15`, "labelFontSize": `12`, "labelFontWeight": `"bold"`, "titleX": `5`, "titleY": `-5`,
		"titleAngle": `0`, "titleAlign": `"left"`, "titleBaseline": `"bottom"`, "titleAnchor": `"start"`,
		"titlePadding": `{"signal":"1 + 2"}`, "titleFontSize": `9`, "tickSize": `4`, "domainWidth": `2`,
		"tickBand": `"extent"`, "tickRound": `false`,
	} {
		if got := string(m[k]); got != want {
			t.Errorf("the %s of the axis = %s, want %s", k, got, want)
		}
	}
	if _, ok := m["gridColor"]; ok {
		t.Error("the axis of the labels has the gridColor, want it on the axis of the grid only")
	}
	if got := str(grid); !strings.Contains(got, `"gridColor":"red"`) || !strings.Contains(got, `"gridDash":[2,2]`) ||
		!strings.Contains(got, `"zindex":1`) || !strings.Contains(got, `"bandPosition":0`) ||
		!strings.Contains(got, `"tickBand":"extent"`) {
		t.Errorf("the axis of the grid = %s, want its gridColor, gridDash, zindex, bandPosition and tickBand", got)
	}
	if got := str(main.Encode); got != `{"labels":{"update":{"text":{"signal":"upper(datum.label)"}}}}` {
		t.Errorf("the encode of the axis = %s, want the labelExpr as the text of the labels", got)
	}
	cond, err := Parse([]byte(`{
		"data": {"values": [{"b": 28}]},
		"mark": "point",
		"encoding": {"x": {"field": "b", "type": "quantitative", "axis": {
			"gridDash": {"condition": {"test": "datum.value > 0", "value": [2, 2]}, "value": []}
		}}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	// A condition on the gridDash of each tick is a list of rules in the
	// encoding of the grid, as vega-lite.js writes it.
	out, _, err := Compile(cond, WithStrict(true))
	if err != nil {
		t.Fatalf("strict Compile of a condition on the gridDash of an axis: %v", err)
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"strokeDash":[{"value":[2,2],"test":"datum.value > 0"},{"value":[]}]`; !strings.Contains(string(b), want) {
		t.Errorf("expected %s in the grid of the axis, got %s", want, b)
	}
	// A condition on a parameter needs a selection, and is an issue.
	param, err := Parse([]byte(`{
		"data": {"values": [{"b": 28}]},
		"mark": "point",
		"encoding": {"x": {"field": "b", "type": "quantitative", "axis": {
			"gridDash": {"condition": {"param": "p", "value": [2, 2]}, "value": []}
		}}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Compile(param, WithStrict(true)); err == nil {
		t.Error("strict Compile of a condition on a parameter succeeded, want an issue")
	}
}

// TestCompileOffsetSizes checks the sizes of a group of bars, as vega-lite.js
// writes them: an offset of a band takes the step of the view, and the band
// holds its steps with the paddings of 0.2, unless the step is "for" the
// position. An offset of a unit of time on a scale of time spans 80 percent
// of one unit, around the band position of the channel.
func TestCompileOffsetSizes(t *testing.T) {
	t.Parallel()
	const data = `"data": {"values": [{"c": "A", "g": "x", "v": 1, "d": "2005-01-01"}]},`
	scaleJSON := func(s *vega.Spec, name string) string {
		t.Helper()
		for _, sc := range s.Scales {
			if sc.Name == name {
				b, err := json.Marshal(sc, json.Deterministic(true))
				if err != nil {
					t.Fatal(err)
				}
				return string(b)
			}
		}
		t.Fatalf("no scale %s", name)
		return ""
	}
	signalJSON := func(s *vega.Spec) string {
		b, err := json.Marshal(s.Signals, json.Deterministic(true))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	tests := []struct {
		name, spec string
		want       []string
	}{
		{"grouped", `{` + data + `"mark": "bar", "encoding": {"x": {"field": "c"}, "xOffset": {"field": "g"}, "y": {"field": "v", "type": "quantitative"}}}`,
			[]string{`20 * bandspace(domain('xOffset').length, 0, 0) / (1-0.2)`, `"paddingInner":0.2`, `"paddingOuter":0.2`, `"range":{"step":20}`}},
		{"step for position", `{` + data + `"width": {"step": 50, "for": "position"}, "mark": "bar", "encoding": {"x": {"field": "c"}, "xOffset": {"field": "g"}, "y": {"field": "v", "type": "quantitative"}}}`,
			[]string{`"name":"x_step","value":50`, `"range":[0,{"signal":"bandwidth('x')"}]`}},
		{"unit of time", `{` + data + `"mark": "bar", "encoding": {"x": {"timeUnit": "yearmonth", "field": "d", "type": "temporal"}, "xOffset": {"field": "g"}, "y": {"field": "v", "type": "quantitative"}}}`,
			[]string{`0.1 * (scale('x', datetime(2001, 1, 1, 0, 0, 0, 0)) - scale('x', datetime(2001, 0, 1, 0, 0, 0, 0)))`, `0.9 * (scale('x'`}},
		{"unit of time at its start", `{` + data + `"mark": "bar", "encoding": {"x": {"timeUnit": "yearmonth", "field": "d", "type": "temporal", "bandPosition": 0}, "xOffset": {"field": "g"}, "y": {"field": "v", "type": "quantitative"}}}`,
			[]string{`-0.4 * (scale('x'`, `0.4 * (scale('x'`}},
	}
	for _, tt := range tests {
		s := compile(t, tt.spec)
		got := signalJSON(s) + scaleJSON(s, "x") + scaleJSON(s, "xOffset")
		for _, w := range tt.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: no %s in\n%s", tt.name, w, got)
			}
		}
	}
}

// TestCompileDescendingContinuous checks that a continuous scale sorted
// descending runs backward, as vega-lite.js writes it, so that the left half
// of a population pyramid grows from the middle, and that a reverse of the
// scale wins.
func TestCompileDescendingContinuous(t *testing.T) {
	t.Parallel()
	const data = `"data": {"values": [{"a": "x", "v": 1}]},`
	for _, tt := range []struct {
		scale string
		want  bool
	}{{"", true}, {`, "scale": {"reverse": false}`, false}} {
		s := compile(t, `{`+data+`"mark": "bar", "encoding": {"y": {"field": "a"}, "x": {"field": "v", "type": "quantitative", "sort": "descending"`+tt.scale+`}}}`)
		for _, sc := range s.Scales {
			if sc.Name == "x" && sc.Reverse.Bool != tt.want {
				t.Errorf("scale %q: reverse %v, want %v", tt.scale, sc.Reverse.Bool, tt.want)
			}
		}
	}
}

// TestCompileDiscreteBarWithEnd checks that a bar with an x2 field on a
// discrete scale runs from x to x2, as vega-lite.js draws a histogram of
// uneven bins on a point scale, and does not take the width of a band.
func TestCompileDiscreteBarWithEnd(t *testing.T) {
	t.Parallel()
	s := compile(t, `{"data": {"values": [{"a": "1", "b": "2", "v": 3}]}, "mark": "bar", "encoding": {
		"x": {"field": "a", "type": "ordinal", "scale": {"type": "point"}}, "x2": {"field": "b"},
		"y": {"field": "v", "type": "quantitative"}}}`)
	b, err := json.Marshal(s.Marks[0].Encode["update"], json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, w := range []string{`"x":{"field":"a","scale":"x"}`, `"x2":{"field":"b","scale":"x"}`} {
		if !strings.Contains(got, w) {
			t.Errorf("no %s in %s", w, got)
		}
	}
	if strings.Contains(got, "bandwidth") {
		t.Errorf("the bar takes the width of a band: %s", got)
	}
}

// TestCompileMarkPosition checks that a position that the mark gives, such
// as "x": 5, places a mark with no channel of x there, as vega-lite.js
// writes it, and not in the middle of the view.
func TestCompileMarkPosition(t *testing.T) {
	t.Parallel()
	s := compile(t, `{"data": {"values": [{"a": "x"}]}, "mark": {"type": "text", "align": "left", "x": 5},
		"encoding": {"y": {"field": "a", "type": "nominal"}, "text": {"field": "a"}}}`)
	b, err := json.Marshal(s.Marks[0].Encode["update"]["x"], json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != `{"value":5}` {
		t.Errorf("x is %s, want {\"value\":5}", got)
	}
}

// TestAxisConfigAndTimeUnit checks that a labelAngle of the config of an
// axis of x wins over the turn of the labels of a discrete axis, as in
// parallel_coordinate, and that the axis of a time unit has a tickMinStep
// of one unit, as vega-lite.js writes it, so that a label of a year does
// not repeat.
func TestAxisConfigAndTimeUnit(t *testing.T) {
	t.Parallel()
	out := compile(t, `{"data": {"values": [{"a": "x", "b": 1}]}, "mark": "bar",
		"encoding": {"x": {"field": "a", "type": "nominal"}, "y": {"field": "b", "type": "quantitative"}},
		"config": {"axisX": {"labelAngle": 0}}}`)
	for _, a := range out.Axes {
		if a.Scale == "x" && !a.LabelAngle.IsZero() {
			t.Errorf("the axis of x has the labelAngle %s, want none", a.LabelAngle.JSON)
		}
	}
	out = compile(t, `{"data": {"values": [{"a": "2006-01-01", "b": 1}]}, "mark": "line",
		"encoding": {"x": {"field": "a", "timeUnit": {"unit": "year", "step": 2}, "type": "temporal"},
			"y": {"field": "b", "type": "quantitative"}}}`)
	want := "datetime(2003, 0, 1, 0, 0, 0, 0) - datetime(2001, 0, 1, 0, 0, 0, 0)"
	n := 0
	for _, a := range out.Axes {
		if a.Scale == "x" {
			n++
			if a.TickMinStep.Signal != want {
				t.Errorf("the axis of x has the tickMinStep %q, want %q", a.TickMinStep.Signal, want)
			}
		}
	}
	if n == 0 {
		t.Error("no axis of x")
	}
}

// TestLabelTurnOfADiscreteAxis checks that a discrete axis of x takes the
// key zuzu.labelTurn in place of the labelAngle of 270 of vega-lite.js,
// and that a labelAngle of the axis or of its config wins over it. The
// angle of a config that the Vega config does not have, such as
// axisDiscrete, goes on the axis, as vega-lite.js puts it (D58 part 4).
func TestLabelTurnOfADiscreteAxis(t *testing.T) {
	t.Parallel()
	spec := func(x, config string) string {
		return `{"data": {"values": [{"a": "x", "b": 1}]}, "mark": "bar",
			"encoding": {"x": {"field": "a", "type": "nominal"` + x + `}, "y": {"field": "b", "type": "quantitative"}},
			"config": {` + config + `}}`
	}
	tests := []struct {
		name, spec string
		turn       bool
		angle      string
	}{
		{"no angle", spec("", ""), true, ""},
		{"the angle of the axis", spec(`, "axis": {"labelAngle": 45}`, ""), false, "45"},
		{"the angle of axisX", spec("", `"axisX": {"labelAngle": 0}`), false, ""},
		{"the angle of axisBand", spec("", `"axisBand": {"labelAngle": 0}`), false, "0"},
		{"the angle of axisDiscrete", spec("", `"axisDiscrete": {"labelAngle": 0}`), false, "0"},
		{"the angle of axisXDiscrete over axisDiscrete", spec("", `"axisDiscrete": {"labelAngle": 0}, "axisXDiscrete": {"labelAngle": 30}`), false, "30"},
		{"the angle of axisPoint on a band", spec("", `"axisPoint": {"labelAngle": 0}`), true, ""},
		{"a time unit", `{"data": {"values": [{"a": "2006-01-01", "b": 1}]}, "mark": "bar",
			"encoding": {"x": {"field": "a", "timeUnit": "month", "type": "ordinal"}, "y": {"field": "b", "type": "quantitative"}}}`, false, ""},
	}
	for _, tt := range tests {
		out := compile(t, tt.spec)
		n := 0
		for _, a := range out.Axes {
			if a.Scale != "x" || (a.Grid != nil && *a.Grid) {
				continue
			}
			n++
			turn, ok := a.Extra[theme.LabelTurn]
			if ok != tt.turn || (ok && string(turn) != "270") {
				t.Errorf("%s: the axis of x has zuzu.labelTurn %s, want it %v", tt.name, turn, tt.turn)
			}
			if got := string(a.LabelAngle.JSON); got != tt.angle {
				t.Errorf("%s: the axis of x has the labelAngle %q, want %q", tt.name, got, tt.angle)
			}
		}
		if n != 1 {
			t.Errorf("%s: %d axes of x with labels, want 1", tt.name, n)
		}
	}
}

// TestCompileOffsetMarks checks the causes of the charts that zuzu drew
// wrongly beside vega.js: a continuous offset that jitters points, the
// offsets of a bar over a unit of time, the size and the axes of the views
// that share a scale of position, the dates of a legend of a discrete
// scale, a box plot with an offset, a bar from one band to another, the
// smallest size of a bar, a stack normalized to one with its labels, and a
// bar sized by a field.
func TestCompileOffsetMarks(t *testing.T) {
	t.Parallel()
	const data = `"data": {"values": [{"c": "A", "g": "x", "v": 1, "w": 2, "d": "2005-01-01"}]},`
	tests := []struct {
		name, spec string
		want, not  []string
	}{
		{"jitter", `{` + data + `"mark": "point", "encoding": {"x": {"field": "v", "type": "quantitative"}, "y": {"field": "c", "type": "ordinal"}, "yOffset": {"field": "w", "type": "quantitative"}}}`,
			[]string{`"name":"yOffset","range":[{"signal":"bandwidth('y')"},0],"type":"linear","zero":false`, `"offset":{"field":"w","scale":"yOffset"}`, `isValid(datum[\"w\"])`},
			[]string{`"step":20`}},
		{"offsets of a bar over a unit of time", `{` + data + `"mark": {"type": "bar", "xOffset": -2, "x2Offset": 2}, "encoding": {"x": {"timeUnit": "month", "field": "d", "type": "temporal"}, "y": {"field": "v", "type": "quantitative"}}}`,
			[]string{`"x":{"field":"month_d","offset":2.5,"scale":"x"}`, `"x2":{"field":"month_d_end","offset":-2.5,"scale":"x"}`}, nil},
		{"shared scale of x", `{` + data + `"vconcat": [{"mark": "text", "encoding": {"x": {"field": "v", "type": "quantitative"}, "text": {"field": "c"}}}, {"mark": "point", "encoding": {"x": {"field": "w", "type": "quantitative"}}}], "resolve": {"scale": {"x": "shared"}}}`,
			[]string{`"width":{"signal":"width"}`, `"width":300`, `"tickCount":{"signal":"ceil(width/40)"},"title":"v"`, `"tickCount":{"signal":"ceil(width/40)"},"title":"w"`},
			[]string{`concat_0_width`, `concat_1_width`}},
		{"dates of a discrete legend", `{` + data + `"mark": "bar", "encoding": {"x": {"field": "c"}, "y": {"field": "v", "type": "quantitative"}, "color": {"timeUnit": "year", "field": "d", "type": "nominal"}}}`,
			[]string{`"format":"%Y","formatType":"time"`}, nil},
		{"box plot with an offset", `{` + data + `"mark": "boxplot", "encoding": {"x": {"field": "c", "type": "nominal"}, "xOffset": {"field": "g", "type": "nominal"}, "y": {"field": "v", "type": "quantitative"}}}`,
			[]string{`"width":{"value":14},"xc":{"field":"c","offset":{"band":0.5,"field":"g","scale":"xOffset"},"scale":"x"}`},
			[]string{`"offset":{"field":"g","scale":"xOffset"}`, `bandwidth('xOffset')`}},
		{"bar from band to band", `{` + data + `"mark": "bar", "encoding": {"x": {"field": "v", "type": "quantitative"}, "y": {"field": "c"}, "y2": {"field": "g"}}}`,
			[]string{`"y":{"band":0.5,"field":"c","scale":"y"}`, `"y2":{"band":0.5,"field":"g","scale":"y"}`}, []string{`bandwidth('y')`}},
		{"smallest size of a bar", `{` + data + `"mark": "bar", "config": {"bar": {"minBandSize": 4}}, "encoding": {"x": {"field": "c"}, "xOffset": {"field": "g"}, "y": {"field": "v", "type": "quantitative"}}}`,
			[]string{`max(4, bandwidth('xOffset'))`}, nil},
		{"normalized stack", `{` + data + `"layer": [{"mark": "bar"}, {"mark": "text", "encoding": {"x": {"aggregate": "sum", "field": "v", "stack": "normalize", "bandPosition": 0.5}, "text": {"field": "v"}}}], "encoding": {"y": {"field": "c"}, "x": {"aggregate": "sum", "field": "v", "stack": "normalize"}, "color": {"field": "g"}}}`,
			[]string{`"format":".0%"`, `"x":{"signal":"scale(\"x\", 0.5 * datum[\"sum_v_start\"] + 0.5 * datum[\"sum_v_end\"])"}`}, nil},
		{"size of a bar", `{` + data + `"mark": "bar", "encoding": {"x": {"field": "c"}, "y": {"field": "v", "type": "quantitative"}, "size": {"field": "g", "type": "ordinal"}}}`,
			[]string{`"width":{"field":"g","scale":"size"},"xc":{"band":0.5,"field":"c","scale":"x"}`, `"name":"size","range":[2,19],"type":"point"`}, nil},
	}
	for _, tt := range tests {
		// The JSON goes through a map, so that its keys are in order.
		b, err := vega.Write(compile(t, tt.spec))
		if err != nil {
			t.Fatal(err)
		}
		var v any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		if b, err = json.Marshal(v, json.Deterministic(true)); err != nil {
			t.Fatal(err)
		}
		got := string(b)
		for _, w := range tt.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: no %s in\n%s", tt.name, w, got)
			}
		}
		for _, w := range tt.not {
			if strings.Contains(got, w) {
				t.Errorf("%s: %s in\n%s", tt.name, w, got)
			}
		}
	}
}

// TestCompileConfigGrid checks that grid: false in the config of the axes
// leaves out the axis of grid lines, as vega-lite.js does, and that the
// grid of the encoding wins over the config.
func TestCompileConfigGrid(t *testing.T) {
	t.Parallel()
	const data = `"data": {"values": [{"a": 1, "b": 2}]},`
	for _, tt := range []struct {
		name, config, axis string
		want               int
	}{
		{"no config", ``, ``, 2},
		{"axis", `"config": {"axis": {"grid": false}},`, ``, 0},
		{"axisX", `"config": {"axisX": {"grid": false}},`, ``, 1},
		{"axisX wins", `"config": {"axis": {"grid": false}, "axisX": {"grid": true}},`, ``, 1},
		{"encoding wins", `"config": {"axis": {"grid": false}},`, `, "axis": {"grid": true}`, 1},
	} {
		s := compile(t, `{`+data+tt.config+`"mark": "point", "encoding": {
			"x": {"field": "a", "type": "quantitative"`+tt.axis+`}, "y": {"field": "b", "type": "quantitative"}}}`)
		n := 0
		for _, a := range s.Axes {
			if a.Grid != nil && *a.Grid {
				n++
			}
		}
		if n != tt.want {
			t.Errorf("%s: %d axes of grid lines, want %d", tt.name, n, tt.want)
		}
	}
}

// TestCompileVarianceChart checks the causes of deneb/variance-chart, whose
// left chart did not draw. A layer that gives a color as a datum keeps the
// domain and the range of the color of its parent, as vega-lite.js merges
// the encoding of a layer. The group of a stack of bars with round corners
// takes the place and the size of a bar across its band, its band of 0.5
// and its offset of y. A rule on a scale of numbers beside a discrete
// position runs from zero, and an opacity with a condition alone keeps the
// condition.
func TestCompileVarianceChart(t *testing.T) {
	t.Parallel()
	s := compile(t, `{"data": {"values": [{"b": "A", "f": 3, "a": 2, "p": 0.5}]},
		"hconcat": [
			{"encoding": {"color": {"type": "nominal", "scale": {"domain": ["Actual", "Forecast"], "range": ["#404040", "silver"]}},
				"y": {"field": "b", "type": "nominal"}, "x": {"type": "quantitative"}},
			 "layer": [{"mark": {"type": "bar", "cornerRadius": 3, "yOffset": 12, "height": {"band": 0.5}},
				"encoding": {"x": {"field": "f"}, "color": {"datum": "Forecast"}}}]},
			{"mark": "rule", "encoding": {"y": {"field": "b", "type": "nominal"}, "x": {"field": "p", "type": "quantitative"},
				"opacity": {"condition": {"test": "datum.p > 1", "value": 0.3}}}}]}`)
	b, err := vega.Write(s)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	if b, err = json.Marshal(v, json.Deterministic(true)); err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{
		`"domain":["Actual","Forecast"]`, `"range":["#404040","silver"]`,
		`"height":{"signal":"max(0.25, 0.5 * bandwidth('concat_0_y'))"}`,
		`"y":{"band":0.25,"field":"b","offset":12,"scale":"concat_0_y"}`,
		`"x2":{"scale":"concat_1_x","value":0}`,
		`"opacity":[{"test":"datum.p > 1","value":0.3}]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %s in\n%s", want, got)
		}
	}
}

// TestCompileEmptyTimeFormat checks that an empty format of an axis and of
// a legend of a time unit is no format, as vega-lite.js reads it, so that
// the labels of deneb/calendar-heatmap read Mon and Tue, not whole dates.
func TestCompileEmptyTimeFormat(t *testing.T) {
	t.Parallel()
	s := compile(t, `{"data": {"values": [{"d": "2024-01-01", "v": 1}]}, "mark": "rect", "encoding": {
		"y": {"field": "d", "timeUnit": "day", "type": "ordinal", "axis": {"format": ""}},
		"color": {"field": "d", "timeUnit": "day", "type": "ordinal", "legend": {"format": ""}}}}`)
	for _, a := range s.Axes {
		if a.Scale == "y" && string(a.Format.JSON) != `"%a"` {
			t.Errorf("the axis has the format %s, want \"%%a\"", a.Format.JSON)
		}
	}
	if len(s.Legends) == 0 {
		t.Fatal("no legend")
	}
	for _, l := range s.Legends {
		if string(l.Format.JSON) != `"%a"` {
			t.Errorf("the legend has the format %s, want \"%%a\"", l.Format.JSON)
		}
	}
}

// TestCompileFacetSpacing checks that the spacing of the channel row or
// column of a facet sets that part of the padding of the layout, as
// vega-lite.js writes it, such as the spacing of 10 between the months of
// deneb/calendar-heatmap.
func TestCompileFacetSpacing(t *testing.T) {
	t.Parallel()
	s := compile(t, `{"data": {"values": [{"m": "Jan", "y": "2020", "v": 1}]}, "mark": "rect", "encoding": {
		"column": {"field": "m", "spacing": 10}, "row": {"field": "y"}, "color": {"field": "v", "type": "quantitative"}}}`)
	if got := string(s.Layout.JSON); !strings.Contains(got, `"padding":{"column":10,"row":20}`) {
		t.Errorf("the layout is %s, want the padding {\"column\":10,\"row\":20}", got)
	}
}

// TestCompileBinnedBarSpacing checks that a bar over bins that the data
// already holds, with bin.binned and an x2 field, leaves the gap of its
// binSpacing between bars, as a bar over computed bins does, so that the
// bars of bar_binned_data do not touch.
func TestCompileBinnedBarSpacing(t *testing.T) {
	t.Parallel()
	s := compile(t, `{"data": {"values": [{"a": 8, "b": 10, "c": 7}]}, "mark": "bar", "encoding": {
		"x": {"field": "a", "bin": {"binned": true, "step": 2}}, "x2": {"field": "b"},
		"y": {"field": "c", "type": "quantitative"}}}`)
	u := s.Marks[0].Encode["update"]
	for ch, want := range map[string]float64{"x": 0.5, "x2": -0.5} {
		b, err := json.Marshal(u[ch])
		if err != nil {
			t.Fatal(err)
		}
		var ref map[string]any
		if err := json.Unmarshal(b, &ref); err != nil {
			t.Fatal(err)
		}
		if ref["offset"] != want {
			t.Errorf("%s is %s, want the offset %v", ch, b, want)
		}
	}
}

// TestConfigAxisTitle checks that the title of the config of the axes, such
// as null for no title, gives the title of each axis that sets none, as
// vega-lite.js reads it, and that the title of an axis wins over it.
func TestConfigAxisTitle(t *testing.T) {
	t.Parallel()
	s, err := Parse([]byte(`{
		"data": {"values": [{"a": 1, "b": 2}]},
		"mark": "point",
		"encoding": {
			"x": {"field": "a", "type": "quantitative"},
			"y": {"field": "b", "type": "quantitative", "axis": {"title": "B"}}
		},
		"config": {"axis": {"title": null}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := Compile(s, WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	titles := map[string]string{}
	for _, a := range out.Axes {
		if a.Title.JSON != nil {
			titles[a.Scale] = string(a.Title.JSON)
		}
	}
	if want := map[string]string{"y": `"B"`}; !maps.Equal(titles, want) {
		t.Errorf("the titles %v, want %v", titles, want)
	}
}

// TestLayerTitleExplicitWins checks that a title that a layer gives wins
// over the title of the field of another layer, as vega-lite.js merges the
// titles of an axis, so the axis reads x and not "x, Median of m".
func TestLayerTitleExplicitWins(t *testing.T) {
	t.Parallel()
	s, err := Parse([]byte(`{
		"data": {"values": [{"x": 1, "m": 2}]},
		"layer": [
			{"mark": "point", "encoding": {"x": {"field": "x", "type": "quantitative", "title": "x"}}},
			{"mark": "rule", "encoding": {"x": {"aggregate": "median", "field": "m"}}}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := Compile(s, WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range out.Axes {
		if a.Title.JSON != nil && string(a.Title.JSON) != `"x"` {
			t.Errorf("the title %s, want x", a.Title.JSON)
		}
	}
}

// TestStrokeWidthLegend checks that a legend of strokeWidth alone names its
// scale, so that it draws, and that the scale runs from 1 to 4, the
// minStrokeWidth and maxStrokeWidth of Vega-Lite.
func TestStrokeWidthLegend(t *testing.T) {
	t.Parallel()
	s, err := Parse([]byte(`{
		"data": {"values": [{"a": 1, "b": 2, "c": 1}]},
		"mark": "line",
		"encoding": {
			"x": {"field": "a", "type": "quantitative"},
			"y": {"field": "b", "type": "quantitative"},
			"strokeWidth": {"field": "c", "type": "quantitative"}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := Compile(s, WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Legends) != 1 || out.Legends[0].StrokeWidth != "strokeWidth" {
		t.Errorf("the legends %+v, want one of the scale strokeWidth", out.Legends)
	}
	b, err := json.Marshal(out.Scales)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"name":"strokeWidth","type":"linear"`) || !strings.Contains(string(b), `"range":[1,4]`) {
		t.Errorf("expected a scale strokeWidth from 1 to 4, got %s", b)
	}
}

// TestRangeEndsReported checks that rangeMin and rangeMax are an issue on a
// range that is not two values, such as the named range of a color, and
// replace the end of a range of two values, such as the range of a size.
func TestRangeEndsReported(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		enc   string
		issue bool
	}{
		{`"color": {"field": "c", "type": "quantitative", "scale": {"rangeMin": "red"}}`, true},
		{`"x": {"field": "c", "type": "nominal", "scale": {"rangeMax": 200}}`, true},
		{`"size": {"field": "c", "type": "quantitative", "scale": {"rangeMax": 500}}`, false},
	} {
		s, err := Parse([]byte(`{"data": {"values": [{"c": 3}]}, "mark": "point", "encoding": {` + test.enc + `}}`))
		if err != nil {
			t.Fatal(err)
		}
		_, report, err := Compile(s)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(report) > 0; got != test.issue {
			t.Errorf("%s: the report %v, want an issue %v", test.enc, report, test.issue)
		}
	}
}

// TestTimeUnitLegendIsSymbol checks that a legend of colors over a unit of
// time draws a symbol for each unit, as vega-lite.js writes it, and not a
// gradient.
func TestTimeUnitLegendIsSymbol(t *testing.T) {
	t.Parallel()
	s, err := Parse([]byte(`{
		"data": {"values": [{"d": "2020-01-01", "v": 1}]},
		"mark": "line",
		"encoding": {
			"x": {"field": "d", "type": "temporal", "timeUnit": "month"},
			"y": {"field": "v", "type": "quantitative"},
			"color": {"field": "d", "type": "temporal", "timeUnit": "quarter"}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Legends) != 1 || out.Legends[0].Type != "symbol" {
		t.Errorf("the legends %+v, want one of the type symbol", out.Legends)
	}
}

// TestMarkConfigStyles checks that config.mark and the config of a kind of
// mark, such as config.text, pass on as a style that the mark takes after
// the styles of the look, without the properties that only Vega-Lite has,
// and that their color is the color of the mark, as vega-lite.js writes it.
func TestMarkConfigStyles(t *testing.T) {
	t.Parallel()
	s, err := Parse([]byte(`{
		"data": {"values": [{"a": 1}]},
		"mark": "text",
		"encoding": {"x": {"field": "a", "type": "quantitative"}, "text": {"field": "a", "type": "quantitative"}},
		"config": {"mark": {"color": "red", "opacity": 0.5}, "text": {"dx": 3, "dy": 1, "filled": true}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := Compile(s, WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(out, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"zuzu.spec.text":{"dx":3,"dy":1,"opacity":0.5}`,
		`"fill":{"value":"red"}`,
		`"style":["text","zuzu.spec.text"]`,
	} {
		if !strings.Contains(string(b), want) {
			t.Errorf("expected %s in %s", want, b)
		}
	}
}

// TestConfigNumberFormat checks that config.numberFormat formats each
// quantitative field, in its axis, its legend, its text and its tooltip, as
// vega-lite.js writes it, and that customFormatTypes is an issue.
func TestConfigNumberFormat(t *testing.T) {
	t.Parallel()
	src := `{
		"data": {"values": [{"a": 1, "b": 2}]},
		"mark": "text",
		"encoding": {
			"x": {"field": "a", "type": "quantitative"},
			"color": {"field": "b", "type": "quantitative"},
			"text": {"field": "a", "type": "quantitative"},
			"tooltip": {"field": "b", "type": "quantitative"}
		},
		"config": {"numberFormat": ".2f"%s}
	}`
	s, err := Parse([]byte(fmt.Sprintf(src, "")))
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := Compile(s, WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(out, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"format":".2f"`, `"text":{"signal":"format(datum[\"a\"], \".2f\")"}`,
		`"tooltip":{"signal":"format(datum[\"b\"], \".2f\")"}`,
	} {
		if !strings.Contains(string(b), want) {
			t.Errorf("expected %s in %s", want, b)
		}
	}
	if n := strings.Count(string(b), `"format":".2f"`); n != 2 {
		t.Errorf("%d formats .2f, want the axis and the legend", n)
	}
	s, err = Parse([]byte(fmt.Sprintf(src, `, "customFormatTypes": true`)))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Compile(s, WithStrict(true)); err == nil {
		t.Error("strict Compile with customFormatTypes succeeded, want an issue")
	}
}

// TestRectIntervalKeepsZero checks that the scale of a rect with an end on
// its axis, as in a mosaic, keeps zero, as vega-lite.js writes it.
func TestRectIntervalKeepsZero(t *testing.T) {
	t.Parallel()
	s, err := Parse([]byte(`{
		"data": {"values": [{"a": 1, "b": 3, "c": 2, "d": 5}]},
		"mark": "rect",
		"encoding": {
			"x": {"field": "a", "type": "quantitative"}, "x2": {"field": "b"},
			"y": {"field": "c", "type": "quantitative"}, "y2": {"field": "d"}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := Compile(s, WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	for _, sc := range out.Scales {
		if (sc.Name == "x" || sc.Name == "y") && (sc.Zero.Kind != vega.Literal || !sc.Zero.Bool) {
			t.Errorf("the scale %s has zero %+v, want true", sc.Name, sc.Zero)
		}
	}
}

// TestGuidePropsPassOn checks that the properties of an axis and of a legend
// that Vega has pass on to the Vega guide, in its fields, so that the
// runtime draws them or says that it does not.
func TestGuidePropsPassOn(t *testing.T) {
	t.Parallel()
	s, err := Parse([]byte(`{
		"data": {"values": [{"a": 1, "b": "x"}]},
		"mark": "point",
		"encoding": {
			"x": {"field": "a", "type": "quantitative", "axis": {"minExtent": 30, "labelFlush": false, "tickBand": "extent"}},
			"color": {"field": "b", "type": "nominal", "legend": {"titleOrient": "left", "labelOverlap": true, "padding": 4}}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	out, report, err := Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 0 {
		t.Errorf("the report %v, want none", report)
	}
	b, err := json.Marshal(out, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"minExtent":30`, `"labelFlush":false`, `"tickBand":"extent"`, `"titleOrient":"left"`, `"labelOverlap":true`, `"padding":4`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("expected %s in %s", want, b)
		}
	}
}
