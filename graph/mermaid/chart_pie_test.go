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

func TestPieHighlight(t *testing.T) {
	t.Parallel()
	items := renderMermaid(t, mmdFiles(t, "pie")["06-highlight.mmd"])
	// The slice that highlightSlice names reaches 5 percent past the rim.
	for label, want := range map[string]float64{"Rent": 157.5, "Food": 150, "Fun": 150} {
		n := itemCount(items, scene.Arc, "slice", func(d map[string]any) bool { return d["label"] == label })
		if n != 1 {
			t.Fatalf("the slice %s draws %d times", label, n)
		}
		for _, it := range items {
			if it.mark == scene.Arc && it.datum["label"] == label && it.item.OuterRadius != want {
				t.Errorf("the slice %s has the radius %v, not %v", label, it.item.OuterRadius, want)
			}
		}
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
		{"---\nconfig:\n  pie:\n    highlightSlice: hover\n---\npie\n\"A\" : 1", "pie.highlightSlice"},
		{"---\nconfig:\n  pie:\n    legendPosition: center\n---\npie\n\"A\" : 1", "legend position center"},
		{"---\nconfig:\n  theme: sepia\n---\npie\n\"A\" : 1", "theme"},
		{"---\nconfig:\n  themeVariables:\n    pie2: red\n---\npie\n\"A\" : 1", "themeVariables"},
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
		// The label 3% does not fit in its slice at these places, so it goes
		// outside the pie. A distance of 0 stands for outside.
		{"", map[string]float64{"79%": 112.5, "17%": 112.5, "3%": 0}},
		{"    textPosition: 0.5\n", map[string]float64{"79%": 75, "17%": 75, "3%": 0}},
		{"    zuzu.textPlacement: bisector\n", map[string]float64{"79%": 112.5, "17%": 112.5, "3%": 0}},
		{"    zuzu.textPlacement: centroid\n", map[string]float64{"79%": 24.14, "17%": 95.04, "3%": 0}},
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
				got := math.Hypot(it.item.X-cx, it.item.Y-cy)
				if want == 0 && got > 150 {
					continue
				}
				if math.Abs(got-want) > 0.01 {
					t.Errorf("%q: the label %s is %.2f from the center, expected %.2f", test.config, text, got, want)
				}
			}
			if !found {
				t.Errorf("%q: no label %s", test.config, text)
			}
		}
	}
}

func TestPieOutsideLabels(t *testing.T) {
	t.Parallel()
	// The 1% slices are too thin for their labels, and 90% is not.
	const src = "pie\n\"Big\" : 90\n\"A\" : 1\n\"B\" : 5\n\"C\" : 2\n\"D\" : 1\n\"E\" : 1"
	items := renderMermaid(t, src)
	var cx, cy, ro float64
	for _, it := range items {
		if it.mark == scene.Arc {
			cx, cy, ro = it.item.X, it.item.Y, it.item.OuterRadius
		}
	}
	type label struct {
		text  string
		box   [4]float64
		left  bool
		rules int
	}
	var outside []label
	for _, it := range items {
		if it.mark != scene.Text || !strings.HasSuffix(it.text, "%") {
			continue
		}
		d := it.datum
		in := math.Hypot(it.item.X-cx, it.item.Y-cy) < ro
		if d["label"] == "Big" {
			if !in || d["outside"] == true {
				t.Errorf("the label %s of the large slice is outside the pie", it.text)
			}
			continue
		}
		if d["label"] == "A" || d["label"] == "D" || d["label"] == "E" {
			if in || d["outside"] != true {
				t.Errorf("the label %s of the slice %v is inside the pie", it.text, d["label"])
				continue
			}
		}
		if in {
			continue
		}
		w := 6.0 * float64(len(it.text))
		l := label{text: it.text, box: [4]float64{it.item.X - w/2, it.item.Y - 5.5, it.item.X + w/2, it.item.Y + 5.5}}
		l.left = it.item.X < cx
		// The label sits in a column beside the pie, on the side of the
		// middle angle of its slice.
		if l.box[2] > cx-ro && l.box[0] < cx+ro {
			t.Errorf("the label %s of %v is not beside the pie", it.text, d["label"])
		}
		px, _ := d["px"].(float64)
		py, _ := d["py"].(float64)
		if r := math.Hypot(px-cx, py-cy); math.Abs(r-ro) > 0.01 {
			t.Errorf("the leader of %v starts %.2f from the center, not on the rim", d["label"], r)
		}
		if (px < cx) != l.left {
			t.Errorf("the label of %v is on the other side of its slice", d["label"])
		}
		for _, r := range items {
			if r.mark == scene.Rule && r.datum != nil && r.datum["label"] == d["label"] {
				l.rules++
			}
		}
		if l.rules != 2 {
			t.Errorf("the leader of %v has %d parts, not 2", d["label"], l.rules)
		}
		outside = append(outside, l)
	}
	if len(outside) < 3 {
		t.Fatalf("%d labels outside the pie, not 3 or more", len(outside))
	}
	for i, a := range outside {
		for _, b := range outside[i+1:] {
			if a.box[0] < b.box[2] && b.box[0] < a.box[2] && a.box[1] < b.box[3] && b.box[1] < a.box[3] {
				t.Errorf("the labels %s and %s overlap", a.text, b.text)
			}
		}
	}
}

func TestPieLeaderColor(t *testing.T) {
	t.Parallel()
	items := renderMermaid(t, "pie\n\"Dogs\" : 386\n\"Cats\" : 85\n\"Rats\" : 15")
	fill := ""
	for _, it := range items {
		if it.mark == scene.Arc && it.datum != nil && it.datum["label"] == "Rats" {
			fill = it.item.Fill.Color
		}
	}
	n := 0
	for _, it := range items {
		if it.mark != scene.Rule || it.datum == nil || it.datum["label"] != "Rats" {
			continue
		}
		n++
		if it.item.Stroke.Color != fill {
			t.Errorf("expected the leader line in the color %s of its slice, got %s", fill, it.item.Stroke.Color)
		}
	}
	if n != 2 || fill == "" {
		t.Errorf("expected two parts of a leader line and a slice, got %d and %q", n, fill)
	}
	if textCount(items, "Dogs") != 1 {
		t.Error("expected the legend to name Dogs once")
	}
}

func TestChartAccessibility(t *testing.T) {
	t.Parallel()
	acc := "accTitle: The title\naccDescr {\n  It has\n  two lines\n}\n"
	for _, src := range []string{
		"pie\n" + acc + "\"A\" : 1",
		"journey\n" + acc + "section S\nTask: 3: Me",
		"architecture-beta\n" + acc + "service a(server)[A]",
		"quadrantChart\n" + acc + "A: [0.5, 0.5]",
		"xychart\n" + acc + "bar [1]",
		"treemap-beta\n" + acc + "\"A\"\n    \"B\": 1",
		"radar-beta\n" + acc + "axis A, B, C\ncurve c{1, 2, 3}",
		"wardley-beta\n" + acc + "component A [0.5, 0.5]",
	} {
		d, err := mermaid.Read(src)
		if err != nil {
			t.Fatal(err)
		}
		b, _, report, err := mermaid.Chart(d, true)
		if err != nil || len(report) != 0 {
			t.Errorf("%q: the report %v and the error %v", src, report, err)
			continue
		}
		if want := `"description":"The title It has two lines"`; !strings.Contains(string(b), want) {
			t.Errorf("%q: the spec has no %s", src, want)
		}
	}
	d, err := mermaid.Read("pie\naccDescr {\nnever ends\n\"A\" : 1")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := mermaid.Chart(d, false); !errors.Is(err, mermaid.ErrSyntax) {
		t.Errorf("an accDescr with no end: the error %v", err)
	}
}
