package mermaid_test

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/vega"
)

// chartItem is an item of the scene of a chart: the type of its mark, its
// text, its datum and the item.
type chartItem struct {
	mark, text string
	datum      map[string]any
	item       *scene.Item
}

// renderMermaid renders a source of Mermaid in the strict mode, so that a
// part that zuzu does not draw fails the test, and returns the items of
// its scene.
func renderMermaid(t *testing.T, src string) []chartItem {
	t.Helper()
	c, _, err := zuzu.Parse([]byte(src), zuzu.WithStrict(true))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	s, _, err := c.Render(context.Background(), zuzu.WithLocation(time.UTC))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var out []chartItem
	var walk func(m *scene.Mark)
	walk = func(m *scene.Mark) {
		for i := range m.Items {
			it := &m.Items[i]
			d, _ := it.Datum.(map[string]any)
			out = append(out, chartItem{mark: m.Type, text: strings.Join(it.Text, " "), datum: d, item: it})
			for j := range it.Items {
				walk(&it.Items[j])
			}
		}
	}
	walk(&s.Root)
	return out
}

// mmdFiles returns the fixtures of a kind of chart and their sources.
func mmdFiles(t *testing.T, kind string) map[string]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", kind, "*.mmd"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures of %s: %v", kind, err)
	}
	out := map[string]string{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(p)] = string(b)
	}
	return out
}

// itemCount counts the items of a type of mark whose datum has a kind and
// matches.
func itemCount(items []chartItem, mark, kind string, match func(d map[string]any) bool) int {
	n := 0
	for _, it := range items {
		if it.mark == mark && it.datum != nil && it.datum["kind"] == kind && match(it.datum) {
			n++
		}
	}
	return n
}

// textCount counts the text items that write a text.
func textCount(items []chartItem, s string) int {
	n := 0
	for _, it := range items {
		if it.mark == scene.Text && it.text == s {
			n++
		}
	}
	return n
}

// compileMermaid compiles a source of Mermaid into the spec of a chart,
// and returns its report.
func compileMermaid(t *testing.T, src string, strict bool) (vega.Report, error) {
	t.Helper()
	d, err := mermaid.Read(src)
	if err != nil {
		return nil, err
	}
	_, _, report, err := mermaid.Chart(d, strict)
	return report, err
}

// reportHolds reports whether a report names a part.
func reportHolds(r vega.Report, part string) bool {
	return slices.ContainsFunc(r, func(i vega.Issue) bool { return strings.Contains(i.Message, part) })
}

// pieSlice matches a slice of a pie: its label and its value.
var pieSlice = regexp.MustCompile(`^\s*"([^"]+)"\s*:\s*([0-9.]+)\s*$`)

func TestPieFixtures(t *testing.T) {
	t.Parallel()
	for name, src := range mmdFiles(t, "pie") {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			items := renderMermaid(t, src)
			showData := strings.Contains(src, "pie showData")
			n := 0
			for l := range strings.SplitSeq(src, "\n") {
				m := pieSlice.FindStringSubmatch(l)
				if m == nil {
					continue
				}
				n++
				v, _ := strconv.ParseFloat(m[2], 64)
				if c := itemCount(items, scene.Arc, "slice", func(d map[string]any) bool {
					return d["label"] == m[1] && d["value"] == v
				}); c != 1 {
					t.Errorf("the slice %s of %v draws %d times", m[1], v, c)
				}
				legend := m[1]
				if showData {
					legend += " [" + m[2] + "]"
				}
				if c := textCount(items, legend); c != 1 {
					t.Errorf("the label %q draws %d times", legend, c)
				}
			}
			if c := itemCount(items, scene.Arc, "slice", func(map[string]any) bool { return true }); c != n || n == 0 {
				t.Errorf("expected %d slices, got %d", n, c)
			}
		})
	}
}

func TestPieErrors(t *testing.T) {
	t.Parallel()
	tests := []string{
		"pie\n\"A\" : -1",
		"pie\n\"A\" : x",
		"pie\n\"A\" 4",
		"pie\nA : 4",
		"pie",
		"pie showData Title\n\"A\" : 1",
	}
	for _, src := range tests {
		if _, err := compileMermaid(t, src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: expected a syntax error, got %v", src, err)
		}
	}
}

func TestPieIssues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src, part string
	}{
		{"pie\n\"A\" : 1\n\"A\" : 2", `the second slice "A"`},
		{"pie\naccTitle: Pets\n\"A\" : 1", "accTitle"},
		{"pie\naccDescr {\nmore\n}\n\"A\" : 1", "accDescr"},
		{"---\nconfig:\n  pie:\n    highlightSlice: A\n---\npie\n\"A\" : 1", "pie.highlightSlice"},
		{"---\nconfig:\n  pie:\n    legendPosition: center\n---\npie\n\"A\" : 1", "legend position center"},
		{"---\nconfig:\n  theme: forest\n---\npie\n\"A\" : 1", "theme"},
		{"---\nconfig:\n  themeVariables:\n    pie2: red\n---\npie\n\"A\" : 1", "themeVariables"},
		{"---\nconfig:\n  look: handDrawn\n---\npie\n\"A\" : 1", "handDrawn"},
		{"---\nconfig:\n  pie:\n    zuzu.textPlacement: middle\n---\npie\n\"A\" : 1", "text placement middle"},
		{"---\nconfig:\n  pie:\n    zuzu.textPlacement: circle\n    textPosition: 0.5\n---\npie\n\"A\" : 1", "pie.textPosition"},
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

func TestPieTextPlacement(t *testing.T) {
	t.Parallel()
	const pets = "pie\n\"Dogs\" : 386\n\"Cats\" : 85\n\"Rats\" : 15"
	tests := []struct {
		config string
		want   map[string]float64
	}{
		{"", map[string]float64{"79%": 112.5, "17%": 112.5, "3%": 112.5}},
		{"    textPosition: 0.5\n", map[string]float64{"79%": 75, "17%": 75, "3%": 75}},
		{"    zuzu.textPlacement: bisector\n", map[string]float64{"79%": 112.5, "17%": 112.5, "3%": 112.5}},
		{"    zuzu.textPlacement: centroid\n", map[string]float64{"79%": 24.14, "17%": 95.04, "3%": 99.84}},
		{"    zuzu.textPlacement: circle\n", map[string]float64{"79%": 75, "17%": 98.54, "3%": 136.76}},
		// In a ring from 75 to 150, a label stays in the ring.
		{"    zuzu.textPlacement: circle\n    donutHole: 0.5\n", map[string]float64{"79%": 112.5, "17%": 112.5, "3%": 136.76}},
		{"    zuzu.textPlacement: centroid\n    donutHole: 0.5\n", map[string]float64{"79%": 112.5}},
	}
	for _, test := range tests {
		src := pets
		if test.config != "" {
			src = "---\nconfig:\n  pie:\n" + test.config + "---\n" + pets
		}
		items := renderMermaid(t, src)
		var cx, cy float64
		for _, it := range items {
			if it.mark == scene.Arc {
				cx, cy = it.item.X, it.item.Y
			}
		}
		for text, want := range test.want {
			found := false
			for _, it := range items {
				if it.mark != scene.Text || it.text != text {
					continue
				}
				found = true
				if it.item.Align != "center" || it.item.Baseline != "middle" {
					t.Errorf("%q: the label %s has the align %q and the baseline %q", test.config, text, it.item.Align, it.item.Baseline)
				}
				if got := math.Hypot(it.item.X-cx, it.item.Y-cy); math.Abs(got-want) > 0.01 {
					t.Errorf("%q: the label %s is %.2f from the center, expected %.2f", test.config, text, got, want)
				}
			}
			if !found {
				t.Errorf("%q: no label %s", test.config, text)
			}
		}
	}
}
