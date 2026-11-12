package mermaid_test

import (
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/scene"
)

// xyLine matches a bar or a line of an xychart: its kind and its list.
var xyLine = regexp.MustCompile(`^\s*(bar|line)\b.*(\[.*\])\s*$`)

// xyCats matches the categories of the x axis.
var xyCats = regexp.MustCompile(`^\s*x-axis\b.*\[(.*)\]\s*$`)

// xyLabel matches a value of a line with its label in quotes.
var xyLabel = regexp.MustCompile(`^\s*\S+\s+"([^"]*)"\s*$`)

func TestXYFixtures(t *testing.T) {
	t.Parallel()
	for name, src := range mmdFiles(t, "xychart") {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			items := renderMermaid(t, src)
			want := map[string][]float64{}
			for l := range strings.SplitSeq(src, "\n") {
				if m := xyCats.FindStringSubmatch(l); m != nil {
					for c := range strings.SplitSeq(m[1], ",") {
						c = strings.Trim(strings.TrimSpace(c), `"`)
						if n := textCount(items, c); n != 1 {
							t.Errorf("the category %q draws %d times", c, n)
						}
					}
				}
				m := xyLine.FindStringSubmatch(l)
				if m == nil {
					continue
				}
				for v := range strings.SplitSeq(strings.Trim(m[2], "[]"), ",") {
					if lm := xyLabel.FindStringSubmatch(v); lm != nil {
						if n := textCount(items, lm[1]); n != 1 {
							t.Errorf("the label %q draws %d times", lm[1], n)
						}
					}
					f, err := strconv.ParseFloat(strings.Fields(v)[0], 64)
					if err != nil {
						t.Fatalf("the value %q: %v", v, err)
					}
					want[m[1]] = append(want[m[1]], f)
				}
			}
			for kind, mark := range map[string]string{"bar": scene.Rect, "line": scene.Line} {
				var got []float64
				for _, it := range items {
					if it.mark == mark && it.datum != nil && it.datum["kind"] == kind {
						y, _ := it.datum["y"].(float64)
						got = append(got, y)
					}
				}
				slices.Sort(got)
				exp := slices.Sorted(slices.Values(want[kind]))
				if !slices.Equal(got, exp) {
					t.Errorf("the values of the %ss: expected %v, got %v", kind, exp, got)
				}
			}
		})
	}
}

func TestXYParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src  string
		rows []string
	}{
		// With no x axis, the values spread over 1 to the number of values
		// of the first plot.
		{"xychart\nbar [3, -2, 5]\nline [1, 2, 3, 4, 5]", []string{
			"bar:1:3", "bar:2:-2", "bar:3:5", "line:1:1", "line:1.5:2", "line:2:3", "line:2.5:4", "line:3:5",
		}},
		{"xychart\nx-axis [a, \"b c\"]\nline \"name\" [+1.5, .5 \"half\"]", []string{"line:a:1.5", "line:b c:0.5:half"}},
		{"xychart-beta\nx-axis t 10 --> 20\nbar [1, 2, 3]", []string{"bar:10:1", "bar:15:2", "bar:20:3"}},
		{"xychart\nx-axis 0-->1\nbar [1, 2]", []string{"bar:0:1", "bar:1:2"}},
	}
	for _, test := range tests {
		items := renderMermaid(t, test.src)
		var got []string
		for _, it := range items {
			if it.datum == nil || (it.mark != scene.Rect && it.mark != scene.Line) {
				continue
			}
			if k, _ := it.datum["kind"].(string); k == "bar" || k == "line" {
				row := k + ":" + strconv.FormatFloat(num(it.datum["x"]), 'f', -1, 64)
				if s, ok := it.datum["x"].(string); ok {
					row = k + ":" + s
				}
				row += ":" + strconv.FormatFloat(num(it.datum["y"]), 'f', -1, 64)
				if l, _ := it.datum["label"].(string); l != "" {
					row += ":" + l
				}
				got = append(got, row)
			}
		}
		if !slices.Equal(got, test.rows) {
			t.Errorf("%q: expected %v, got %v", test.src, test.rows, got)
		}
	}
}

// num returns a number of a datum, or 0.
func num(v any) float64 {
	f, _ := v.(float64)
	return f
}

func TestXYErrors(t *testing.T) {
	t.Parallel()
	tests := []string{
		"xychart",
		"xychart sideways\nbar [1]",
		"xychart\nbar 1, 2",
		"xychart\nbar [1, x]",
		"xychart\narea [1, 2]",
		"xychart\ny-axis [a, b]\nbar [1]",
		"xychart\nx-axis t a --> b\nbar [1]",
		"xychart\nx-axis \"t\nbar [1]",
		"xychart\nbar [1, 2",
	}
	for _, src := range tests {
		if _, err := compileMermaid(t, src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: expected a syntax error, got %v", src, err)
		}
	}
}

func TestXYIssues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src, part string
	}{
		{"---\nconfig:\n  xyChart:\n    xAxis:\n      showTick: true\n---\nxychart\nbar [1]", "xyChart.xAxis.showTick"},
		{"---\nconfig:\n  xyChart:\n    yAxis:\n      showAxisLine: true\n---\nxychart\nbar [1]", "xyChart.yAxis.showAxisLine"},
		{"---\nconfig:\n  xyChart:\n    xAxis:\n      showAxisLine: true\n---\nxychart\nx-axis 1 --> 5\nbar [1]", "xyChart.xAxis.showAxisLine"},
		{"---\nconfig:\n  themeVariables:\n    xyChart:\n      titleColor: red\n---\nxychart\nbar [1]", "themeVariables.xyChart.titleColor"},
		// zuzu draws no ticks, puts the labels 8 from the plot and the title
		// of an axis at its end, and draws no axis line along numbers.
		{"---\nconfig:\n  xyChart:\n    plotReservedSpacePercent: 60\n---\nxychart\nbar [1]", "xyChart.plotReservedSpacePercent"},
		{"---\nconfig:\n  xyChart:\n    xAxis:\n      tickLength: 10\n---\nxychart\nbar [1]", "xyChart.xAxis.tickLength"},
		{"---\nconfig:\n  xyChart:\n    xAxis:\n      tickWidth: 5\n---\nxychart\nbar [1]", "xyChart.xAxis.tickWidth"},
		{"---\nconfig:\n  xyChart:\n    yAxis:\n      labelPadding: 10\n---\nxychart\nbar [1]", "xyChart.yAxis.labelPadding"},
		{"---\nconfig:\n  xyChart:\n    yAxis:\n      titlePadding: 10\n---\nxychart\nbar [1]", "xyChart.yAxis.titlePadding"},
		{"---\nconfig:\n  xyChart:\n    yAxis:\n      axisLineWidth: 5\n---\nxychart\nx-axis [a]\nbar [1]", "xyChart.yAxis.axisLineWidth"},
		{"---\nconfig:\n  xyChart:\n    xAxis:\n      axisLineWidth: 5\n---\nxychart\nx-axis 1 --> 5\nbar [1]", "xyChart.xAxis.axisLineWidth"},
	}
	for _, test := range tests {
		report, err := compileMermaid(t, test.src, false)
		if err != nil {
			t.Errorf("%q: %v", test.src, err)
			continue
		}
		if !reportHolds(report, test.part) {
			t.Errorf("%q: expected an issue of %q, got %v", test.src, test.part, report)
		}
		if _, err := compileMermaid(t, test.src, true); err == nil {
			t.Errorf("%q: expected an error in the strict mode", test.src)
		}
	}
	// Mermaid draws no value past the last category and no label of a bar,
	// so neither is an issue (D32).
	for _, src := range []string{"xychart\nx-axis [a]\nbar [1, 2]", "xychart\nbar [1 \"one\"]"} {
		if report, err := compileMermaid(t, src, true); err != nil || len(report) != 0 {
			t.Errorf("%q: the report %v and the error %v", src, report, err)
		}
	}
}

func TestXYAxes(t *testing.T) {
	t.Parallel()
	items := renderMermaid(t, mmdFiles(t, "xychart")["07-axes.mmd"])
	// The labels of the x axis turn by 45 degrees, and its title does not
	// draw. The y axis is at the left, so its labels do not turn, and they
	// do not draw.
	for _, s := range []string{"January", "May"} {
		n := 0
		for _, it := range items {
			if it.mark == scene.Text && it.text == s && it.item.Angle == 45 {
				n++
			}
		}
		if n != 1 {
			t.Errorf("the label %q draws %d times at 45 degrees", s, n)
		}
	}
	for s, want := range map[string]int{"Month": 0, "Visits": 1, "20": 0, "100": 0} {
		if n := textCount(items, s); n != want {
			t.Errorf("%q draws %d times, not %d", s, n, want)
		}
	}
	drawn, _ := drawFixture(t, "testdata/xychart/07-axes.mmd")
	if n := countDrawn(drawn, scene.Rule, func(it drawnItem) bool { return it.role == "axis-domain" }); n != 0 {
		t.Errorf("%d axis lines draw", n)
	}
	drawn, _ = drawFixture(t, "testdata/xychart/01-sales.mmd")
	if n := countDrawn(drawn, scene.Rule, func(it drawnItem) bool { return it.role == "axis-domain" }); n != 1 {
		t.Errorf("%d axis lines draw in a chart of categories, not 1", n)
	}
	// A rotation outside -90 to 90 degrees is 0, as in Mermaid.
	items = renderMermaid(t, "---\nconfig:\n  xyChart:\n    xAxis:\n      labelRotation: 120\n---\nxychart\nx-axis [a, b]\nbar [1, 2]")
	for _, it := range items {
		if it.mark == scene.Text && it.text == "a" && it.item.Angle != 0 {
			t.Errorf("the label a turns by %v", it.item.Angle)
		}
	}
}

func TestXYConfig(t *testing.T) {
	t.Parallel()
	items := renderMermaid(t, mmdFiles(t, "xychart")["06-config.mmd"])
	size := func(s string) float64 {
		for _, it := range items {
			if it.mark == scene.Text && it.text == s {
				return it.item.FontSize
			}
		}
		return 0
	}
	// The x axis of Mermaid holds the categories, on the left of a
	// horizontal chart.
	for s, want := range map[string]float64{"Sales by month": 22, "jan": 14, "Months": 13, "4,000": 12, "Revenue (in $)": 15} {
		if got := size(s); got != want {
			t.Errorf("%q draws at %v, not %v", s, got, want)
		}
	}
	line := false
	for _, it := range items {
		if it.mark == scene.Rule && it.item.StrokeWidth != nil && *it.item.StrokeWidth == 3 {
			line = true
		}
	}
	if !line {
		t.Error("the axis line of the categories is not 3 wide")
	}
	// A value sits inside the end of its bar, 10 from the end, and in the
	// middle of the bar across it.
	for _, bar := range items {
		if bar.mark != scene.Rect || bar.datum["kind"] != "bar" {
			continue
		}
		v := num(bar.datum["y"])
		found := false
		for _, it := range items {
			if it.mark != scene.Text || it.text != strconv.FormatFloat(v, 'f', -1, 64) {
				continue
			}
			found = true
			x, y := it.item.X+it.item.Dx, it.item.Y+it.item.Dy
			if end := bar.item.X + bar.item.Width; x != end-10 || it.item.Align != "right" {
				t.Errorf("the value %v ends at %v, %s aligned, and its bar at %v", v, x, it.item.Align, end)
			}
			if mid := bar.item.Y + bar.item.Height/2; y != mid || it.item.Baseline != "middle" {
				t.Errorf("the value %v is at %v with the baseline %s, and the middle of its bar at %v", v, y, it.item.Baseline, mid)
			}
		}
		if !found {
			t.Errorf("the value %v of a bar draws no label", v)
		}
	}
}
