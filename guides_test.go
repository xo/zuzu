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
				{"name": "size", "type": "linear", "domain": [0, 25], "range": [0, 400]}],
			` + guides + `}`
	}
	tests := []struct {
		name, guides, role string
		want               []string
	}{
		{"axis s", `"axes": [{"scale": "x", "orient": "bottom", "format": "s"}]`, "axis-label",
			[]string{"0M", "2M", "4M", "6M", "8M", "10M", "12M"}},
		{"axis default", `"axes": [{"scale": "x", "orient": "bottom", "labelOverlap": false}]`, "axis-label",
			[]string{"0", "2,000,000", "4,000,000", "6,000,000", "8,000,000", "10,000,000", "12,000,000"}},
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
// group of an axis, or of a part of a legend, goes in the report, or is an
// error in the strict mode. The layout of Vega places the group of an axis
// itself, and the layout of a legend of zuzu does not place its parts as
// Vega does.
func TestGuideEncodePlaceReport(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		guides, want string
	}{
		{`"axes": [{"scale": "x", "orient": "bottom", "encode": {"axis": {"update": {"x": {"value": 5}}}}}]`, "the encode of x of the axis of an axis"},
		{`"legends": [{"fill": "x", "encode": {"labels": {"update": {"y": {"value": 5}}}}}]`, "the encode of y of the labels of a legend"},
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

// TestAxisSpecProps checks that the runtime reads the labelAlign, the
// titleX, the titleY, the titleAngle, the titleAlign and the titleBaseline
// of an axis, from the axis or from the config, with the axis over the
// config, as in Vega. Before, it dropped them, so the year of each row of
// calendar-view covered the label "Jan".
func TestAxisSpecProps(t *testing.T) {
	t.Parallel()
	items := func(config, axis, role string) []scene.Item {
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
