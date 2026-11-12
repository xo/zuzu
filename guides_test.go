package zuzu

import (
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
// labels, and that an encode of a place goes in the report, or is an error
// in the strict mode.
func TestAxisEncode(t *testing.T) {
	t.Parallel()
	spec := func(props string) string {
		return `{"width": 200, "height": 100,
			"scales": [{"name": "x", "type": "band", "domain": [1, 2, 3], "range": "width"}],
			"axes": [{"scale": "x", "orient": "bottom",
				"encode": {"labels": {"update": {` + props + `}}}}]}`
	}
	got, _, err := roleTexts(t, spec(`"text": {"signal": "datum.value + ':00 am'"}`), "axis-label", true)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"1:00 am", "2:00 am", "3:00 am"}; !slices.Equal(got, want) {
		t.Errorf("the labels are %q, want %q", got, want)
	}
	_, report, err := roleTexts(t, spec(`"x": {"value": 5}`), "axis-label", false)
	if err != nil || len(report) != 1 || !strings.Contains(report[0].Error(), "the encode of x of the labels") {
		t.Errorf("an encode of x gives the report %v and the error %v", report, err)
	}
	if _, _, err := roleTexts(t, spec(`"x": {"value": 5}`), "axis-label", true); err == nil {
		t.Errorf("an encode of x in the strict mode gives no error")
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
