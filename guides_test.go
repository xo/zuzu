package zuzu

import (
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu/scene"
)

// roleTexts renders a spec and returns the lines of the text items of a
// role, such as axis-label, with the report of the render.
func roleTexts(t *testing.T, src, role string, strict bool) ([]string, Report, error) {
	t.Helper()
	c, _, err := Parse([]byte(src), WithDialect(Vega), WithStrict(strict))
	if err != nil {
		t.Fatal(err)
	}
	s, report, err := c.Render(t.Context(), WithLocation(time.UTC))
	if err != nil {
		return nil, report, err
	}
	var out []string
	var walk func(m *scene.Mark)
	walk = func(m *scene.Mark) {
		for i := range m.Items {
			if m.Role == role {
				out = append(out, m.Items[i].Text...)
			}
			for j := range m.Items[i].Items {
				walk(&m.Items[i].Items[j])
			}
		}
	}
	walk(&s.Root)
	return out, report, nil
}

// TestAxisSignalOfTheDatum checks that a property of an axis whose signal
// reads the datum takes the datum of each label, tick and grid line, as in
// vega.js. Before, the signal of flight-passengers ran once with no datum,
// so that no label showed and every grid line was dashed.
func TestAxisSignalOfTheDatum(t *testing.T) {
	t.Parallel()
	axis := `{"scale": "y", "orient": "left", "grid": true, "values": [0, 5, 10],
		"labelOpacity": {"signal": "datum.value ? 1 : 0"},
		"gridDash": {"signal": "datum.value ? [] : [5, 4]"},
		"tickWidth": {"signal": "datum.index == 0.5 ? 3 : 1"}}`
	for _, it := range axisItems(t, ``, axis, "axis-label") {
		v, _ := it.Datum.(map[string]any)["value"].(float64)
		if want := map[bool]float64{true: 1, false: 0}[v != 0]; it.Opacity == nil || *it.Opacity != want {
			t.Errorf("the label of %v has the opacity %v, want %v", v, it.Opacity, want)
		}
	}
	for _, it := range axisItems(t, ``, axis, "axis-grid") {
		v, _ := it.Datum.(map[string]any)["value"].(float64)
		if dashed := len(it.StrokeDash) > 0; dashed != (v == 0) {
			t.Errorf("the grid line of %v has the dash %v", v, it.StrokeDash)
		}
	}
	for _, it := range axisItems(t, ``, axis, "axis-tick") {
		v, _ := it.Datum.(map[string]any)["value"].(float64)
		if want := map[bool]float64{true: 3, false: 1}[v == 5]; it.StrokeWidth == nil || *it.StrokeWidth != want {
			t.Errorf("the tick of %v has the width %v, want %v", v, it.StrokeWidth, want)
		}
	}
}

// TestAxisEndTicks checks that an axis with no tickCount puts a tick and a
// label on both ends of a nice domain, with the precision that its step
// needs, and that a tickCount of the spec wins (D46).
func TestAxisEndTicks(t *testing.T) {
	t.Parallel()
	spec := func(domain, axis string) string {
		return `{"width": 240, "height": 100,
			"scales": [{"name": "x", "type": "linear", "domain": ` + domain + `, "range": "width"}],
			"axes": [` + axis + `]}`
	}
	for _, test := range []struct {
		domain, axis string
		want         []string
	}{
		{`[0, 45000]`, `{"scale": "x", "orient": "bottom"}`, []string{"0", "15,000", "30,000", "45,000"}},
		{`[0, 7.5]`, `{"scale": "x", "orient": "bottom"}`, []string{"0.0", "1.5", "3.0", "4.5", "6.0", "7.5"}},
		{`[0, 45000]`, `{"scale": "x", "orient": "bottom", "tickCount": 5}`, []string{"0", "10,000", "20,000", "30,000", "40,000"}},
	} {
		got, _, err := roleTexts(t, spec(test.domain, test.axis), "axis-label", true)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, test.want) {
			t.Errorf("%s %s: the labels are %q, want %q", test.domain, test.axis, got, test.want)
		}
	}
}

// TestAxisConfigLabels checks that labels: false in the config of the axes
// hides the labels of an axis, as in rect_mosaic_labelled, and that the
// labels of the axis win over the config.
func TestAxisConfigLabels(t *testing.T) {
	t.Parallel()
	spec := func(axis string) string {
		return `{"width": 240, "height": 100, "config": {"axis": {"labels": false}},
			"scales": [{"name": "x", "type": "linear", "domain": [0, 10], "range": "width"}],
			"axes": [` + axis + `]}`
	}
	for _, test := range []struct {
		axis string
		want int
	}{
		{`{"scale": "x", "orient": "bottom"}`, 0},
		{`{"scale": "x", "orient": "bottom", "labels": true}`, 6},
	} {
		got, _, err := roleTexts(t, spec(test.axis), "axis-label", true)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != test.want {
			t.Errorf("%s: %d labels %q, want %d", test.axis, len(got), got, test.want)
		}
	}
}

// TestGuideNumberFormat checks that a format of numbers without a
// precision, such as "s", takes the precision that tells two ticks apart,
// as in Vega, and that the type s takes the SI prefix of the largest end.
// Before, "s" wrote 6 digits, such as 0.00000 and 10.0000.
func TestGuideNumberFormat(t *testing.T) {
	t.Parallel()
	spec := func(guides string) string {
		return `{"width": 240, "height": 100,
			"scales": [
				{"name": "x", "type": "linear", "domain": [0, 12000000], "range": "width"},
				{"name": "size", "type": "linear", "domain": [0, 25], "range": [4, 400]}],
			` + guides + `}`
	}
	tests := []struct {
		name, guides, role string
		want               []string
	}{
		{"axis s", `"axes": [{"scale": "x", "orient": "bottom", "format": "s"}]`, "axis-label",
			[]string{"0M", "3M", "6M", "9M", "12M"}},
		{"axis default", `"axes": [{"scale": "x", "orient": "bottom", "labelOverlap": false}]`, "axis-label",
			[]string{"0", "3,000,000", "6,000,000", "9,000,000", "12,000,000"}},
		{"axis %", `"axes": [{"scale": "x", "orient": "bottom", "format": "%", "values": [0, 6000000]}]`, "axis-label",
			[]string{"0%", "600000000%"}},
		{"legend s", `"legends": [{"size": "size", "format": "s"}]`, "legend-label",
			[]string{"0", "5", "10", "15", "20", "25"}},
	}
	for _, test := range tests {
		got, _, err := roleTexts(t, spec(test.guides), test.role, true)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if !slices.Equal(got, test.want) {
			t.Errorf("%s: the labels are %q, want %q", test.name, got, test.want)
		}
	}
}

// TestLogAxisLabels checks that the labels of a log axis write the small
// powers in full, such as 0.001, and not as 0, and that only some ticks
// between two powers take a label.
func TestLogAxisLabels(t *testing.T) {
	t.Parallel()
	got, _, err := roleTexts(t, `{"width": 100, "height": 400,
		"scales": [{"name": "y", "type": "log", "domain": [0.001, 10000], "range": "height"}],
		"axes": [{"scale": "y", "orient": "left", "labelOverlap": false}]}`, "axis-label", true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"0.001", "0.01", "0.1", "1", "10", "100", "1,000", "10,000"}
	if !slices.Equal(got, want) {
		t.Errorf("the labels are %q, want %q", got, want)
	}
	got, _, err = roleTexts(t, `{"width": 100, "height": 400,
		"scales": [{"name": "y", "type": "log", "domain": [0.1, 100], "range": "height"}],
		"axes": [{"scale": "y", "orient": "left", "labelOverlap": false}]}`, "axis-label", true)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"0.1", "0.2", "1", "2", "10", "20", "100"}
	if !slices.Equal(got, want) {
		t.Errorf("the labels between the powers are %q, want %q", got, want)
	}
}

// TestAxisEncode checks that the encode of an axis sets the text of its
// labels.
func TestAxisEncode(t *testing.T) {
	t.Parallel()
	src := `{"width": 200, "height": 100,
		"scales": [{"name": "x", "type": "band", "domain": [1, 2, 3], "range": "width"}],
		"axes": [{"scale": "x", "orient": "bottom",
			"encode": {"labels": {"update": {"text": {"signal": "datum.value + ':00 am'"}}}}}]}`
	got, _, err := roleTexts(t, src, "axis-label", true)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"1:00 am", "2:00 am", "3:00 am"}; !slices.Equal(got, want) {
		t.Errorf("the labels are %q, want %q", got, want)
	}
}

// TestAxisEncodePlace checks that an encode of the place of a part of an
// axis applies in the frame of the group of the axis of Vega, which starts
// at the edge of the plot area that the axis runs along, moved out by its
// offset (D35). Before, such an encode went in the report, and the labels
// of the months of the Vega example calendar-view did not move.
func TestAxisEncodePlace(t *testing.T) {
	t.Parallel()
	items := func(axis, role string) ([]scene.Item, Report) {
		t.Helper()
		src := `{"width": 200, "height": 100,
			"scales": [{"name": "x", "type": "band", "domain": [1, 2, 3], "range": "width"},
				{"name": "y", "type": "linear", "domain": [0, 10], "range": "height"}],
			"axes": [` + axis + `]}`
		c, _, err := Parse([]byte(src), WithDialect(Vega), WithStrict(true))
		if err != nil {
			t.Fatal(err)
		}
		s, report, err := c.Render(t.Context(), WithLocation(time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		var out []scene.Item
		var walk func(m *scene.Mark)
		walk = func(m *scene.Mark) {
			for i := range m.Items {
				if m.Role == role {
					out = append(out, m.Items[i])
				}
				for j := range m.Items[i].Items {
					walk(&m.Items[i].Items[j])
				}
			}
		}
		walk(&s.Root)
		return out, report
	}
	for _, c := range []struct {
		name, axis, role string
		check            func(it scene.Item) bool
	}{
		{
			"the x of the labels of a bottom axis, at the start of each band",
			`{"scale": "x", "orient": "bottom", "encode": {"labels": {"update": {"x": {"scale": "x", "field": "value", "band": 0, "offset": 3}}}}}`,
			"axis-label", func(it scene.Item) bool {
				v, _ := it.Datum.(map[string]any)["value"].(float64)
				return math.Abs(it.X-((v-1)*200/3+3)) < 1e-9
			},
		},
		{
			"the y of the labels of a bottom axis, below its edge",
			`{"scale": "x", "orient": "bottom", "encode": {"labels": {"update": {"y": {"value": 4}}}}}`,
			"axis-label", func(it scene.Item) bool { return it.Y == 104 },
		},
		{
			"the y of the labels of a top axis with an offset, above its edge",
			`{"scale": "x", "orient": "top", "offset": 5, "encode": {"labels": {"update": {"y": {"value": -4}}}}}`,
			"axis-label", func(it scene.Item) bool { return it.Y == -9 },
		},
		{
			"the x of the labels of a right axis, at the right of its edge",
			`{"scale": "y", "orient": "right", "encode": {"labels": {"update": {"xc": {"value": 6}}}}}`,
			"axis-label", func(it scene.Item) bool { return it.X == 206 },
		},
		{
			"the x of the title of a left axis",
			`{"scale": "y", "orient": "left", "title": "Count", "encode": {"title": {"update": {"x": {"value": -30}}}}}`,
			"axis-title", func(it scene.Item) bool { return it.X == -30 },
		},
		{
			"the y of a line of the grid of a left axis, with both of its ends",
			`{"scale": "y", "orient": "left", "grid": true, "values": [5], "encode": {"grid": {"update": {"y": {"value": 20}}}}}`,
			"axis-grid", func(it scene.Item) bool { return it.Y == 20 && it.Y2 == 20 && it.X == 0 && it.X2 == 200 },
		},
	} {
		got, report := items(c.axis, c.role)
		if len(got) == 0 || len(report) != 0 {
			t.Errorf("%s: %d items and the report %v", c.name, len(got), report)
			continue
		}
		for _, it := range got {
			if !c.check(it) {
				t.Errorf("%s: the item is at %v %v to %v %v", c.name, it.X, it.Y, it.X2, it.Y2)
			}
		}
	}
}

// TestGuideEncodePlaceReport checks that an encode of the place of the
// group of an axis or of a legend goes in the report, or is an error in the
// strict mode. The layout of Vega places the group of a guide itself.
func TestGuideEncodePlaceReport(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		guides, want string
	}{
		{`"axes": [{"scale": "x", "orient": "bottom", "encode": {"axis": {"update": {"x": {"value": 5}}}}}]`, "the encode of x of the axis of an axis"},
		{`"legends": [{"fill": "x", "encode": {"legend": {"update": {"y": {"value": 5}}}}}]`, "the encode of y of the legend of a legend"},
		{`"legends": [{"fill": "x", "encode": {"legend": {"update": {"x": {"value": 5}}}, "labels": {"update": {"y": {"value": 5}}}}}]`, "the encode of x of the legend of a legend"},
	} {
		src := `{"width": 200, "height": 100,
			"scales": [{"name": "x", "type": "band", "domain": [1, 2, 3], "range": "width"}], ` + c.guides + `}`
		_, report, err := roleTexts(t, src, "axis-label", false)
		if err != nil || len(report) != 1 || !strings.Contains(report[0].Error(), c.want) {
			t.Errorf("%s gives the report %v and the error %v", c.guides, report, err)
		}
		if _, _, err := roleTexts(t, src, "axis-label", true); err == nil {
			t.Errorf("%s in the strict mode gives no error", c.guides)
		}
	}
}

// TestAxisOfAGroupWithNoWidth checks that an axis in a group that sets no
// width runs along the range of its scale, as in Vega. Before, it had one
// label at 0.
func TestAxisOfAGroupWithNoWidth(t *testing.T) {
	t.Parallel()
	got, _, err := roleTexts(t, `{"width": 300, "height": 100,
		"marks": [{"type": "group", "encode": {"update": {"height": {"value": 100}}},
			"scales": [{"name": "x", "type": "linear", "domain": [0, 10], "range": [240, 0]}],
			"axes": [{"scale": "x", "orient": "bottom"}]}]}`, "axis-label", true)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"0", "2", "4", "6", "8", "10"}; !slices.Equal(got, want) {
		t.Errorf("the labels are %q, want %q", got, want)
	}
}

// TestRectItemEnds checks that a mark whose data is a mark of rects reads
// x2 and y2 as the larger ends, as in Vega, whatever the order of the
// encoding.
func TestRectItemEnds(t *testing.T) {
	t.Parallel()
	c, _, err := Parse([]byte(`{"width": 200, "height": 100, "marks": [
		{"name": "bars", "type": "rect", "encode": {"enter": {
			"x": {"value": 120}, "x2": {"value": 20}, "y": {"value": 10}, "y2": {"value": 60}}}},
		{"type": "text", "from": {"data": "bars"}, "encode": {"enter": {
			"x": {"field": "x2"}, "y": {"field": "y2"}, "text": {"value": "end"}}}}]}`), WithDialect(Vega), WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	s, _, err := c.Render(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var text *scene.Item
	var walk func(m *scene.Mark)
	walk = func(m *scene.Mark) {
		for i := range m.Items {
			if m.Type == scene.Text && text == nil {
				text = &m.Items[i]
			}
			for j := range m.Items[i].Items {
				walk(&m.Items[i].Items[j])
			}
		}
	}
	walk(&s.Root)
	if text == nil || text.X != 120 || text.Y != 60 {
		t.Errorf("the text at the end of the rect is %+v, want it at 120, 60", text)
	}
}

// axisItems renders an axis of a band scale x of 200 pixels, or of a linear
// scale y of 100 pixels, with a config, and returns the items of a role of
// the axis.
func axisItems(t *testing.T, config, axis, role string) []scene.Item {
	t.Helper()
	src := `{"width": 200, "height": 100, "config": {` + config + `},
		"scales": [{"name": "x", "type": "band", "domain": [1, 2, 3], "range": "width"},
			{"name": "y", "type": "linear", "domain": [0, 10], "range": "height"}],
		"axes": [` + axis + `]}`
	c, _, err := Parse([]byte(src), WithDialect(Vega), WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	s, report, err := c.Render(t.Context(), WithLocation(time.UTC))
	if err != nil || len(report) != 0 {
		t.Fatalf("render: %v, with the report %v", err, report)
	}
	var out []scene.Item
	var walk func(m *scene.Mark)
	walk = func(m *scene.Mark) {
		for i := range m.Items {
			if m.Role == role {
				out = append(out, m.Items[i])
			}
			for j := range m.Items[i].Items {
				walk(&m.Items[i].Items[j])
			}
		}
	}
	walk(&s.Root)
	return out
}

// TestAxisLabelTurn checks the key zuzu.labelTurn of an axis: the labels
// draw flat when each fits the step of its band, and turn when one does
// not, at the labelFontSize of the axis. A labelAngle of the axis or of its
// config wins over it (D58 part 4).
func TestAxisLabelTurn(t *testing.T) {
	t.Parallel()
	const turn = `{"scale": "x", "orient": "bottom", "zuzu.labelTurn": 270`
	tests := []struct {
		name, config, axis string
		want               float64
	}{
		{"labels that fit", ``, turn + `}`, 0},
		{"a label that does not fit", ``, turn + `, "format": ".12f"}`, 270},
		{"a large font", ``, turn + `, "format": ".2f", "labelFontSize": 40}`, 270},
		{"the angle of the axis", ``, turn + `, "format": ".12f", "labelAngle": 0}`, 0},
		{"the angle of the config", `"axisX": {"labelAngle": 0}`, turn + `, "format": ".12f"}`, 0},
	}
	for _, tt := range tests {
		labels := axisItems(t, tt.config, tt.axis, "axis-label")
		if len(labels) != 3 {
			t.Fatalf("%s: %d labels, want 3", tt.name, len(labels))
		}
		for _, it := range labels {
			if it.Angle != tt.want {
				t.Errorf("%s: the label %v turns by %v, want %v", tt.name, it.Text, it.Angle, tt.want)
			}
		}
	}
}

// TestAxisSpecProps checks that the runtime reads the labelAlign, the
// titleX, the titleY, the titleAngle, the titleAlign and the titleBaseline
// of an axis, from the axis or from the config, with the axis over the
// config, as in Vega. Before, it dropped them, so the year of each row of
// calendar-view covered the label "Jan".
func TestAxisSpecProps(t *testing.T) {
	t.Parallel()
	items := func(config, axis, role string) []scene.Item {
		t.Helper()
		return axisItems(t, config, axis, role)
	}
	for _, c := range []struct {
		name, config, axis, role string
		check                    func(it scene.Item) bool
	}{
		{
			"the labelAlign of a top axis",
			``, `{"scale": "x", "orient": "top", "labelAlign": "left"}`,
			"axis-label", func(it scene.Item) bool { return it.Align == "left" },
		},
		{
			"the labelAlign of a bottom axis over its labelAngle",
			``, `{"scale": "x", "orient": "bottom", "labelAngle": 45, "labelAlign": "center"}`,
			"axis-label", func(it scene.Item) bool { return it.Align == "center" && it.Angle == 45 },
		},
		{
			"the labelAlign of the config",
			`"axisX": {"labelAlign": "right"}`, `{"scale": "x", "orient": "bottom"}`,
			"axis-label", func(it scene.Item) bool { return it.Align == "right" },
		},
		{
			"the labelAlign of an axis over the config",
			`"axis": {"labelAlign": "right"}`, `{"scale": "x", "orient": "bottom", "labelAlign": "left"}`,
			"axis-label", func(it scene.Item) bool { return it.Align == "left" },
		},
		{
			"the title of a left axis as calendar-view places it",
			``, `{"scale": "y", "orient": "left", "title": {"signal": "2020"}, "titleAngle": 0, "titleAlign": "right", "titleX": -8, "titleY": -2}`,
			"axis-title", func(it scene.Item) bool {
				return it.X == -8 && it.Y == -2 && it.Angle == 0 && it.Align == "right" && it.Text[0] == "2020"
			},
		},
		{
			"the titleY of a bottom axis with an offset, from the origin of the axis",
			``, `{"scale": "x", "orient": "bottom", "offset": 5, "title": "X", "titleY": 30}`,
			"axis-title", func(it scene.Item) bool { return it.X == 200 && it.Y == 135 },
		},
		{
			"the titleAngle of a signal and the titleBaseline",
			``, `{"scale": "y", "orient": "right", "title": "Y", "titleAngle": {"signal": "45 * 2"}, "titleBaseline": "top"}`,
			"axis-title", func(it scene.Item) bool { return it.Angle == 90 && it.Baseline == "top" },
		},
		{
			"the title of the config of each y axis, as parallel-coordinates places it",
			`"axisY": {"titleX": -2, "titleY": 110, "titleAngle": 0, "titleAlign": "right", "titleBaseline": "top"}`,
			`{"scale": "y", "orient": "left", "title": "Y"}`,
			"axis-title", func(it scene.Item) bool {
				return it.X == -2 && it.Y == 110 && it.Align == "right" && it.Baseline == "top"
			},
		},
	} {
		got := items(c.config, c.axis, c.role)
		if len(got) == 0 {
			t.Errorf("%s: no items", c.name)
			continue
		}
		for _, it := range got {
			if !c.check(it) {
				t.Errorf("%s: the item is at %v, %v, turned %v, aligned %s on %s", c.name, it.X, it.Y, it.Angle, it.Align, it.Baseline)
			}
		}
	}
}

// TestAxisLabelAndTitlePlace checks that the runtime reads the
// labelPadding, the labelOffset, the labelBaseline, the titleAnchor and the
// titlePadding of an axis, and its tickSize and ticks for the distance of
// its labels, from the axis or from the config, as vega.js places them.
// Before, it dropped them with no issue, so the labels of
// bar_axis_space_saving ran into the plot.
func TestAxisLabelAndTitlePlace(t *testing.T) {
	t.Parallel()
	// inBand reports whether a label sits at the middle of a band of x,
	// moved along the axis by off.
	inBand := func(x, off float64) bool {
		f := (x-off)/(200.0/3) - 0.5
		return math.Abs(f-math.Round(f)) < 1e-9
	}
	for _, c := range []struct {
		name, config, axis, role string
		check                    func(it scene.Item) bool
	}{
		{
			"the labelPadding past a tick of the default size",
			``, `{"scale": "y", "orient": "left", "labelPadding": 10}`,
			"axis-label", func(it scene.Item) bool { return it.X == -15 && it.Align == "right" },
		},
		{
			"the labelPadding of an axis with no ticks",
			``, `{"scale": "y", "orient": "left", "labelPadding": 10, "ticks": false}`,
			"axis-label", func(it scene.Item) bool { return it.X == -10 },
		},
		{
			"the tickSize of the config, past which a label keeps the padding of the look",
			`"axisY": {"tickSize": 12}`, `{"scale": "y", "orient": "left"}`,
			"axis-label", func(it scene.Item) bool { return it.X == -15 },
		},
		{
			"a labelPadding below 0, which puts the labels inside the plot",
			`"axis": {"ticks": false}`, `{"scale": "x", "orient": "bottom", "labelPadding": -5}`,
			"axis-label", func(it scene.Item) bool { return it.Y == 95 && inBand(it.X, 0) },
		},
		{
			"the labelOffset and the labelBaseline",
			``, `{"scale": "x", "orient": "bottom", "labelOffset": 7, "labelBaseline": "bottom"}`,
			"axis-label", func(it scene.Item) bool { return it.Baseline == "bottom" && inBand(it.X, 7) },
		},
		{
			"the labelBaseline of the config",
			`"axisLeft": {"labelBaseline": "top"}`, `{"scale": "y", "orient": "left"}`,
			"axis-label", func(it scene.Item) bool { return it.Baseline == "top" },
		},
		{
			"a label in the middle of its band at a bandPosition of 0",
			``, `{"scale": "x", "orient": "bottom", "bandPosition": 0}`,
			"axis-label", func(it scene.Item) bool { return inBand(it.X, 0) },
		},
		{
			"the titleAnchor start of a bottom axis",
			``, `{"scale": "x", "orient": "bottom", "title": "X", "titleAnchor": "start"}`,
			"axis-title", func(it scene.Item) bool { return it.X == 0 && it.Align == "left" && it.Angle == 0 },
		},
		{
			"the titleAnchor middle of a bottom axis",
			``, `{"scale": "x", "orient": "bottom", "title": "X", "titleAnchor": "middle"}`,
			"axis-title", func(it scene.Item) bool { return it.X == 100 && it.Align == "center" },
		},
		{
			"the titleAnchor end of a left axis, at the end of the range, turned as in Vega",
			``, `{"scale": "y", "orient": "left", "title": "Y", "titleAnchor": "end"}`,
			"axis-title", func(it scene.Item) bool {
				return it.Y == 0 && it.X < -8 && it.Angle == -90 && it.Align == "right" && it.Baseline == "bottom"
			},
		},
		{
			"the titleAnchor start of the config of a right axis",
			`"axisY": {"titleAnchor": "start"}`, `{"scale": "y", "orient": "right", "title": "Y"}`,
			"axis-title", func(it scene.Item) bool { return it.Y == 100 && it.Angle == 90 && it.Align == "left" },
		},
		{
			"the titleAnchor end of flight-passengers, with its titleX and titleAngle",
			``, `{"scale": "y", "orient": "left", "title": "Y", "titleAnchor": "end", "titleX": 1, "titleAngle": 0,
				"titleAlign": "left", "titleBaseline": "middle"}`,
			"axis-title", func(it scene.Item) bool {
				return it.X == 1 && it.Y == 0 && it.Angle == 0 && it.Align == "left" && it.Baseline == "middle"
			},
		},
		{
			"a titleAnchor of null, which keeps the place of the look",
			``, `{"scale": "y", "orient": "left", "title": "Y", "titleAnchor": null}`,
			"axis-title", func(it scene.Item) bool { return it.X == 0 && it.Y < 0 && it.Angle == 0 && it.Align == "left" },
		},
	} {
		got := axisItems(t, c.config, c.axis, c.role)
		if len(got) == 0 {
			t.Errorf("%s: no items", c.name)
			continue
		}
		for _, it := range got {
			if !c.check(it) {
				t.Errorf("%s: the item is at %v, %v, turned %v, aligned %s on %s", c.name, it.X, it.Y, it.Angle, it.Align, it.Baseline)
			}
		}
	}
	// A titlePadding is the space between the labels and the title.
	title := func(axis string) scene.Item {
		t.Helper()
		return axisItems(t, ``, axis, "axis-title")[0]
	}
	look := title(`{"scale": "x", "orient": "bottom", "title": "X"}`)
	padded := title(`{"scale": "x", "orient": "bottom", "title": "X", "titlePadding": 20}`)
	if padded.Y-look.Y != 14 {
		t.Errorf("the title with a titlePadding of 20 is at y %v, want 14 below the title of the look at %v", padded.Y, look.Y)
	}
	labels := axisItems(t, ``, `{"scale": "y", "orient": "left"}`, "axis-label")
	near := title(`{"scale": "y", "orient": "left", "title": "Y", "titleAnchor": "middle", "titlePadding": 0}`)
	far := title(`{"scale": "y", "orient": "left", "title": "Y", "titleAnchor": "middle", "titlePadding": 10}`)
	if near.X-far.X != 10 || near.Y != 50 || near.X >= labels[0].X {
		t.Errorf("the turned titles are at %v and %v, %v, want 10 apart at y 50, left of the labels at %v", near.X, far.X, near.Y, labels[0].X)
	}
}

// TestAxisTickBand checks that the runtime draws tickBand, tickExtra,
// tickOffset and tickRound of an axis, and reads bandPosition from the
// config of an axis of bands, as the axis of Vega documents them. Before,
// it reported each property and drew the ticks in the middle of each band,
// and it did not read the config.axisBand of box-plot.
func TestAxisTickBand(t *testing.T) {
	t.Parallel()
	xs := func(items []scene.Item) []float64 {
		var out []float64
		for _, it := range items {
			out = append(out, math.Round(it.X*100)/100)
		}
		slices.Sort(out)
		return out
	}
	for _, c := range []struct {
		name, config, axis, role string
		want                     []float64
	}{
		{"the ticks of the look", ``, `{"scale": "x", "orient": "bottom"}`, "axis-tick", []float64{33.33, 100, 166.67}},
		{"tickBand extent", ``, `{"scale": "x", "orient": "bottom", "tickBand": "extent"}`, "axis-tick", []float64{0, 66.67, 133.33, 200}},
		{"tickBand extent of the grid", ``, `{"scale": "x", "orient": "bottom", "grid": true, "tickBand": "extent"}`, "axis-grid", []float64{0, 66.67, 133.33, 200}},
		{"tickBand center over tickExtra", ``, `{"scale": "x", "orient": "bottom", "tickBand": "center", "tickExtra": true, "bandPosition": 1}`, "axis-tick", []float64{33.33, 100, 166.67}},
		{"the config of box-plot", `"axisBand": {"bandPosition": 1, "tickExtra": true, "tickOffset": 0}`, `{"scale": "x", "orient": "bottom"}`, "axis-tick", []float64{0, 66.67, 133.33, 200}},
		{"bandPosition of the config", `"axisBand": {"bandPosition": 0}`, `{"scale": "x", "orient": "bottom"}`, "axis-tick", []float64{0, 66.67, 133.33}},
		{"tickOffset", ``, `{"scale": "x", "orient": "bottom", "tickOffset": 5}`, "axis-tick", []float64{38.33, 105, 171.67}},
		{"tickOffset moves the labels", ``, `{"scale": "x", "orient": "bottom", "tickOffset": 5}`, "axis-label", []float64{38.33, 105, 171.67}},
		{"tickRound", ``, `{"scale": "x", "orient": "bottom", "tickRound": true}`, "axis-tick", []float64{33, 100, 167}},
		{"tickRound of the labels", ``, `{"scale": "x", "orient": "bottom", "tickRound": true}`, "axis-label", []float64{33, 100, 167}},
	} {
		if got := xs(axisItems(t, c.config, c.axis, c.role)); !slices.Equal(got, c.want) {
			t.Errorf("%s: the %s at %v, want %v", c.name, c.role, got, c.want)
		}
	}
}
