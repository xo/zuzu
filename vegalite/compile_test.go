package vegalite

import (
	"encoding/json/v2"
	"slices"
	"testing"

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
