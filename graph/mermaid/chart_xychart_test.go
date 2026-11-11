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
		{"xychart\nx-axis [a]\nbar [1, 2]", "which have no category"},
		{"xychart\nbar [1 \"one\"]", `the label "one" of a bar`},
		{"xychart\naccTitle: T\nbar [1]", "accTitle"},
		{"---\nconfig:\n  xyChart:\n    xAxis:\n      showTick: false\n---\nxychart\nbar [1]", "xyChart.xAxis"},
		{"---\nconfig:\n  themeVariables:\n    xyChart:\n      titleColor: red\n---\nxychart\nbar [1]", "themeVariables.xyChart.titleColor"},
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
}
