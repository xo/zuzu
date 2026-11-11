package mermaid_test

import (
	"context"
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/layout"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/vega"
)

// drawnItem is an item of a scene that a fixture draws: the type and the
// role of its mark, its text and its datum.
type drawnItem struct {
	mark, role, text string
	datum            map[string]any
	item             *scene.Item
}

// drawFixture renders a fixture of Mermaid and returns the items of its
// scene, except those of the title, and the report.
func drawFixture(t *testing.T, path string) ([]drawnItem, zuzu.Report) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c, report, err := zuzu.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	s, more, err := c.Render(context.Background(), zuzu.WithLocation(time.UTC))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var out []drawnItem
	var walk func(m *scene.Mark)
	walk = func(m *scene.Mark) {
		if strings.HasPrefix(m.Role, "title") {
			return
		}
		for i := range m.Items {
			it := &m.Items[i]
			d, _ := it.Datum.(map[string]any)
			out = append(out, drawnItem{mark: m.Type, role: m.Role, text: strings.Join(it.Text, " "), datum: d, item: it})
			for j := range it.Items {
				walk(&it.Items[j])
			}
		}
	}
	walk(&s.Root)
	return out, append(report, more...)
}

// countDrawn counts the items of a type of mark that match.
func countDrawn(items []drawnItem, mark string, match func(drawnItem) bool) int {
	n := 0
	for _, it := range items {
		if it.mark == mark && match(it) {
			n++
		}
	}
	return n
}

// textDrawn counts the text items that write a text.
func textDrawn(items []drawnItem, s string) int {
	return countDrawn(items, scene.Text, func(it drawnItem) bool { return it.text == s })
}

// fixtures returns the fixtures of a kind of chart.
func fixtures(t *testing.T, kind string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", kind, "*.mmd"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures of %s: %v", kind, err)
	}
	return files
}

// chartOf compiles a source of a chart and returns its report.
func chartOf(src string, strict bool) (vega.Report, error) {
	d, err := mermaid.Read(src)
	if err != nil {
		return nil, err
	}
	_, _, report, err := mermaid.Chart(d, strict)
	return report, err
}

// sankeyLinks reads the links of a fixture with encoding/csv, apart from
// the compiler.
func sankeyLinks(t *testing.T, path string) [][3]string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var links [][3]string
	body := false
	for l := range strings.SplitSeq(string(b), "\n") {
		l = strings.TrimSpace(l)
		switch {
		case l == "sankey" || l == "sankey-beta":
			body = true
		case !body || l == "" || strings.HasPrefix(l, "%%"):
		default:
			r := csv.NewReader(strings.NewReader(l))
			f, err := r.Read()
			if err != nil || len(f) != 3 {
				t.Fatalf("%s: the line %q: %v", path, l, err)
			}
			links = append(links, [3]string{strings.TrimSpace(f[0]), strings.TrimSpace(f[1]), strings.TrimSpace(f[2])})
		}
	}
	return links
}

func TestSankeyFixtures(t *testing.T) {
	t.Parallel()
	for _, path := range fixtures(t, "sankey") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			items, report := drawFixture(t, path)
			if len(report) != 0 {
				t.Errorf("report: %v", report)
			}
			links := sankeyLinks(t, path)
			var nodes []string
			seen := map[string]bool{}
			for _, l := range links {
				v, _ := strconv.ParseFloat(l[2], 64)
				n := countDrawn(items, scene.Path, func(it drawnItem) bool {
					return it.datum["source"] == l[0] && it.datum["target"] == l[1] && it.datum["value"] == v
				})
				if n != 1 {
					t.Errorf("the link %v draws %d times", l, n)
				}
				for _, name := range l[:2] {
					if !seen[name] {
						seen[name] = true
						nodes = append(nodes, name)
					}
				}
			}
			for _, name := range nodes {
				rects := countDrawn(items, scene.Rect, func(it drawnItem) bool { return it.datum["key"] == name })
				// A label that has no room loses its end to an ellipsis.
				labels := countDrawn(items, scene.Text, func(it drawnItem) bool {
					short, cut := strings.CutSuffix(it.text, "…")
					return it.datum["key"] == name && (strings.HasPrefix(it.text, name) || cut && strings.HasPrefix(name, short))
				})
				if rects != 1 || labels != 1 {
					t.Errorf("the node %q draws %d boxes and %d labels", name, rects, labels)
				}
			}
		})
	}
}

func TestSankeyLabels(t *testing.T) {
	t.Parallel()
	items, _ := drawFixture(t, "testdata/sankey/05-config.mmd")
	for _, s := range []string{"Electricity grid $125 bn", "Coal $40 bn", "Losses $10 bn"} {
		if textDrawn(items, s) != 1 {
			t.Errorf("no label %q", s)
		}
	}
	items, _ = drawFixture(t, "testdata/sankey/03-quotes.mmd")
	if textDrawn(items, `Heating and cooling, "homes" 212.29`) != 1 {
		t.Error("the label of a node in quotes is not drawn")
	}
}

func TestSankeyLabelsApart(t *testing.T) {
	t.Parallel()
	style := layout.Style{Measure: text.NewEstimate()}
	for _, path := range fixtures(t, "sankey") {
		items, _ := drawFixture(t, path)
		var labels, nodes []layout.Bounds
		var names []string
		for _, it := range items {
			switch {
			case it.mark == scene.Text:
				labels = append(labels, style.ItemBounds(scene.Text, it.item))
				names = append(names, it.text)
			case it.mark == scene.Rect && it.datum["key"] != nil:
				nodes = append(nodes, style.ItemBounds(scene.Rect, it.item))
			}
		}
		overlap := func(a, b layout.Bounds) bool {
			return a.X1 < b.X2 && b.X1 < a.X2 && a.Y1 < b.Y2 && b.Y1 < a.Y2
		}
		for i, a := range labels {
			for j := i + 1; j < len(labels); j++ {
				if overlap(a, labels[j]) {
					t.Errorf("%s: the label %q overlaps the label %q", path, names[i], names[j])
				}
			}
			for _, n := range nodes {
				if overlap(a, n) {
					t.Errorf("%s: the label %q overlaps a node", path, names[i])
				}
			}
		}
	}
	// A plain sankey keeps each label whole, on the inner side of its
	// node, at the middle of the node.
	items, _ := drawFixture(t, "testdata/sankey/01-basic.mmd")
	for _, it := range items {
		if it.mark != scene.Text {
			continue
		}
		num := func(k string) float64 {
			f, _ := it.datum[k].(float64)
			return f
		}
		x0, x1, mid := num("x0"), num("x1"), (num("y0")+num("y1"))/2
		wantX, wantAlign := x1+6, "left"
		if x0 >= 300 {
			wantX, wantAlign = x0-6, "right"
		}
		if strings.HasSuffix(it.text, "…") || it.item.X != wantX || it.item.Y != mid || it.item.Align != wantAlign {
			t.Errorf("the label %q is at %v, %v, %s, not at %v, %v, %s", it.text, it.item.X, it.item.Y, it.item.Align, wantX, mid, wantAlign)
		}
	}
}

func TestSankeyGradient(t *testing.T) {
	t.Parallel()
	items, _ := drawFixture(t, "testdata/sankey/07-gradient.mmd")
	// colors holds the color of each node, from nodeColors or the theme.
	colors := map[string]string{}
	for _, it := range items {
		if key, ok := it.datum["key"].(string); ok && it.mark == scene.Rect {
			colors[key] = it.item.Fill.Color
		}
	}
	if colors["Grid"] != "#4e79a7" || colors["Homes"] != "#e15759" || colors["Coal"] == "" {
		t.Fatalf("the colors of the nodes: %v", colors)
	}
	links := 0
	for _, it := range items {
		if it.mark != scene.Path {
			continue
		}
		links++
		g := it.item.Fill.Gradient
		src, _ := it.datum["source"].(string)
		dst, _ := it.datum["target"].(string)
		// The gradient runs from left to right across the bounds of the
		// band, from the color of the source to the color of the target.
		if g == nil || g.Type != scene.Linear || g.X1 != 0 || g.Y1 != 0 || g.X2 != 1 || g.Y2 != 0 || len(g.Stops) != 2 ||
			g.Stops[0].Color != colors[src] || g.Stops[1].Color != colors[dst] || g.Stops[0].Offset != 0 || g.Stops[1].Offset != 1 {
			t.Errorf("the link from %s to %s has the fill %+v", src, dst, it.item.Fill)
		}
	}
	// An outlined label has a halo. The label of Plant, in a layer before
	// the layer of Grid, the largest node, is at the left of its node. In
	// the legacy style, it is at the right, since Plant is in the left half.
	for _, it := range items {
		if it.mark != scene.Text {
			continue
		}
		if it.item.Stroke.IsZero() || it.item.StrokeWidth == nil || *it.item.StrokeWidth != 3 {
			t.Errorf("the label %q has no halo: %+v, %v", it.text, it.item.Stroke, it.item.StrokeWidth)
		}
		x0, _ := it.datum["x0"].(float64)
		if key := it.datum["key"]; key == "Plant" && (it.item.Align != "right" || it.item.X != x0-6 || x0 >= 300) {
			t.Errorf("the label of Plant is at %v, %s, and its node at %v", it.item.X, it.item.Align, x0)
		}
	}
	if links != 7 {
		t.Errorf("%d links, want 7", links)
	}
}

func TestSankeyErrors(t *testing.T) {
	t.Parallel()
	tests := []string{
		"sankey\nA,B\n",
		"sankey\nA,B,x\n",
		"sankey\nA,B,-1\n",
		"sankey\n\"A,B,1\n",
		"sankey\n\"A\"x,B,1\n",
		"sankey\n,B,1\n",
		"sankey\n",
	}
	for _, src := range tests {
		if _, err := chartOf(src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: the error %v", src, err)
		}
	}
}

func TestSankeyIssues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src, issue string
	}{
		{"---\nconfig:\n  sankey:\n    labelStyle: boxed\n---\nsankey\nA,B,1\n", "labelStyle boxed"},
		{"---\nconfig:\n  sankey:\n    useWidth: 500\n---\nsankey\nA,B,1\n", "sankey.useWidth"},
		{"---\nconfig:\n  sankey:\n    nodeAlignment: top\n---\nsankey\nA,B,1\n", "nodeAlignment top"},
		{"---\nconfig:\n  theme: forest\n---\nsankey\nA,B,1\n", "theme of Mermaid"},
		{"---\nconfig:\n  fontFamily: serif\n---\nsankey\nA,B,1\n", "setting fontFamily"},
	}
	for _, test := range tests {
		report, err := chartOf(test.src, false)
		if err != nil {
			t.Fatalf("%q: %v", test.src, err)
		}
		if len(report) != 1 || !strings.Contains(report[0].Error(), test.issue) {
			t.Errorf("%q: the report %v, not %q", test.src, report, test.issue)
		}
		if _, err := chartOf(test.src, true); !errors.Is(err, vega.ErrUnsupported) {
			t.Errorf("%q: in the strict mode, the error %v", test.src, err)
		}
	}
}
