package mermaid_test

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/icon"
	"github.com/xo/zuzu/icons/carbon"
	"github.com/xo/zuzu/icons/fa"
	"github.com/xo/zuzu/icons/k8s"
	"github.com/xo/zuzu/icons/mdi"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/vega"
)

// drawDiagram renders a source of Mermaid with the bundled packs of icons,
// and returns the items of its scene, with the name of the mark of each,
// and the report.
func drawDiagram(t *testing.T, src string, strict bool) ([]drawnItem, zuzu.Report, error) {
	t.Helper()
	c, report, err := zuzu.Parse([]byte(src), zuzu.WithStrict(strict))
	if err != nil {
		return nil, report, err
	}
	s, more, err := c.Render(context.Background(), zuzu.WithLocation(time.UTC),
		zuzu.WithIcons("fa", fa.Pack()), zuzu.WithIcons("mdi", mdi.Pack()),
		zuzu.WithIcons("carbon", carbon.Pack()), zuzu.WithIcons("k8s", k8s.Pack()))
	report = append(report, more...)
	if err != nil {
		return nil, report, err
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
			out = append(out, drawnItem{mark: m.Type, role: m.Name, text: strings.Join(it.Text, " "), datum: d, item: it})
			for j := range it.Items {
				walk(&it.Items[j])
			}
		}
	}
	walk(&s.Root)
	return out, report, nil
}

// drawnOf returns the items of a mark by its name.
func drawnOf(items []drawnItem, name string) []drawnItem {
	var out []drawnItem
	for _, it := range items {
		if it.role == name {
			out = append(out, it)
		}
	}
	return out
}

// mindmapWant is a node that a fixture of a mindmap draws: its label, its
// shape, and the label of its parent, or empty for the root.
type mindmapWant struct {
	label, shape, parent string
}

func TestMindmapFixtures(t *testing.T) {
	t.Parallel()
	n := func(label, shape, parent string) mindmapWant { return mindmapWant{label, shape, parent} }
	cases := map[string][]mindmapWant{
		"01-basic.mmd": {
			n("mindmap", "circle", ""), n("Origins", "default", "mindmap"), n("Long history", "default", "Origins"),
			n("Research", "default", "mindmap"),
		},
		"02-docs.mmd": {
			n("mindmap", "circle", ""), n("Origins", "default", "mindmap"), n("Long history", "default", "Origins"),
			n("Popularisation", "default", "Origins"),
			n("British popular psychology author Tony Buzan", "default", "Popularisation"),
			n("Research", "default", "mindmap"), n("On effectiveness and features", "default", "Research"),
			n("On Automatic creation", "default", "Research"), n("Uses", "default", "On Automatic creation"),
			n("Creative techniques", "default", "Uses"), n("Strategic planning", "default", "Uses"),
			n("Argument mapping", "default", "Uses"), n("Tools", "default", "mindmap"),
			n("Pen and paper", "default", "Tools"), n("Mermaid", "default", "Tools"),
		},
		"03-shapes.mmd": {
			n("Square root", "rect", ""), n("Rounded square", "rounded", "Square root"), n("Circle", "circle", "Square root"),
			n("Bang", "bang", "Square root"), n("Cloud", "cloud", "Square root"), n("Hexagon", "hexagon", "Square root"),
			n("A plain node", "default", "Square root"), n("A square in quotes (with brackets)", "rect", "Square root"),
		},
		"04-markdown.mmd": {
			n("Root with a second line", "rect", ""),
			n("A bold word, an italic word, and a long text that wraps to a second line because it is long", "rect", "Root with a second line"),
			n("Regular bold and italic text", "rect", "Root with a second line"),
			n("snake_case_names stay plain", "default", "Root with a second line"),
		},
		"05-config.mmd": {
			n("Plan", "default", ""), n("Design", "default", "Plan"), n("Sketches", "default", "Design"),
			n("Review with the team", "default", "Design"), n("Build", "default", "Plan"), n("Code", "default", "Build"),
			n("Tests", "default", "Build"), n("Unit", "default", "Tests"), n("Visual", "default", "Tests"),
			n("Ship", "default", "Plan"),
		},
		"06-wide.mmd": {
			n("Topics", "default", ""), n("One", "default", "Topics"), n("Two", "default", "Topics"),
			n("Three", "default", "Topics"), n("Four", "default", "Topics"), n("Five", "default", "Topics"),
			n("Five A", "default", "Five"), n("Five B", "default", "Five"), n("Five C", "default", "Five"),
			n("Six", "default", "Topics"), n("Seven", "default", "Topics"), n("Eight", "default", "Topics"),
			n("Nine", "default", "Topics"), n("Ten", "default", "Topics"), n("Eleven", "default", "Topics"),
			n("Twelve", "default", "Topics"),
		},
	}
	files := fixtures(t, "mindmap")
	if len(files) != len(cases) {
		t.Fatalf("%d fixtures and %d cases", len(files), len(cases))
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			want, ok := cases[filepath.Base(path)]
			if !ok {
				t.Fatal("no case")
			}
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			items, _, err := drawDiagram(t, string(src), true)
			if err != nil {
				t.Fatalf("draw in the strict mode: %v", err)
			}
			checkMindmap(t, items, want, strings.Contains(string(src), "colorBy: depth"))
		})
	}
}

// checkMindmap checks the nodes, the labels, the edges and the layout of a
// mindmap that drew.
func checkMindmap(t *testing.T, items []drawnItem, want []mindmapWant, byDepth bool) {
	t.Helper()
	nodes := append(drawnOf(items, "shapes"), drawnOf(items, "plainShapes")...)
	if len(nodes) != len(want) {
		t.Fatalf("%d nodes, expected %d", len(nodes), len(want))
	}
	byID := map[float64]map[string]any{}
	for _, it := range nodes {
		byID[num(it.datum["id"])] = it.datum
	}
	// The runs of the label of a node, in order, make its label, apart
	// from the spaces between the runs.
	runs := map[float64]string{}
	for _, it := range drawnOf(items, "labels") {
		runs[num(it.datum["node"])] += it.text
	}
	for _, w := range want {
		found := 0
		for id, d := range byID {
			if d["label"] != w.label {
				continue
			}
			parent := ""
			if p, ok := d["parent"].(float64); ok {
				parent, _ = byID[p]["label"].(string)
			}
			if d["shape"] == w.shape && parent == w.parent {
				found++
			}
			if got := runs[id]; strings.ReplaceAll(got, " ", "") != strings.ReplaceAll(w.label, " ", "") {
				t.Errorf("the node %q draws the text %q", w.label, got)
			}
		}
		if found != 1 {
			t.Errorf("the node %q, %s, under %q, draws %d times", w.label, w.shape, w.parent, found)
		}
	}
	edges := 0
	for _, it := range drawnOf(items, "edges") {
		if it.datum["source"] != it.datum["target"] {
			edges++
		}
	}
	if edges != len(want)-1 {
		t.Errorf("%d edges for %d nodes", edges, len(want))
	}
	// No two nodes overlap, each node is on the side of its branch, away
	// from its parent, and a branch keeps its color.
	box := func(d map[string]any) (float64, float64, float64, float64) {
		x, y, w, h := num(d["x"]), num(d["y"]), num(d["width"]), num(d["height"])
		return x - w/2, y - h/2, x + w/2, y + h/2
	}
	ids := slices.Sorted(func(yield func(float64) bool) {
		for id := range byID {
			if !yield(id) {
				return
			}
		}
	})
	for i, a := range ids {
		ax0, ay0, ax1, ay1 := box(byID[a])
		for _, b := range ids[i+1:] {
			bx0, by0, bx1, by1 := box(byID[b])
			if ax0 < bx1-0.5 && bx0 < ax1-0.5 && ay0 < by1-0.5 && by0 < ay1-0.5 {
				t.Errorf("the nodes %q and %q overlap", byID[a]["label"], byID[b]["label"])
			}
		}
		d := byID[a]
		p, ok := d["parent"].(float64)
		if !ok {
			continue
		}
		pd := byID[p]
		side := num(d["side"])
		if dx := num(d["x"]) - num(pd["x"]); dx*side <= 0 || math.Abs(side) != 1 {
			t.Errorf("the node %q is not on the side %v of its parent", d["label"], side)
		}
		if num(pd["depth"]) > 0 && !byDepth && pd["color"] != d["color"] {
			t.Errorf("the node %q has the color %v, and its parent %v", d["label"], d["color"], pd["color"])
		}
	}
}

func TestMindmapMarkdown(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("testdata/mindmap/04-markdown.mmd")
	if err != nil {
		t.Fatal(err)
	}
	items, _, err := drawDiagram(t, string(src), true)
	if err != nil {
		t.Fatal(err)
	}
	style := map[string][2]any{}
	for _, it := range drawnOf(items, "labels") {
		style[it.text] = [2]any{it.datum["fontWeight"], it.item.FontStyle}
	}
	tests := []struct {
		text   string
		weight float64
		italic bool
	}{
		{"bold", 600, false},
		{"italic", 400, true},
		{"Regular", 400, false},
		{"snake_case_names stay plain", 400, false},
		{"Root", 600, false},
	}
	for _, test := range tests {
		s, ok := style[test.text]
		if !ok {
			t.Errorf("no run %q in %v", test.text, style)
			continue
		}
		if s[0] != test.weight || (s[1] == "italic") != test.italic {
			t.Errorf("the run %q has the weight %v and the style %v", test.text, s[0], s[1])
		}
	}
}

func TestMindmapIcons(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("testdata/mindmap/05-config.mmd")
	if err != nil {
		t.Fatal(err)
	}
	items, _, err := drawDiagram(t, string(src), true)
	if err != nil {
		t.Fatal(err)
	}
	var icons []string
	for _, it := range items {
		if name, ok := it.datum["icon"].(string); ok && it.mark == scene.Path {
			icons = append(icons, name)
		}
	}
	slices.Sort(icons)
	if !slices.Equal(icons, []string{"fa:hammer", "mdi:palette"}) {
		t.Errorf("the icons %v", icons)
	}
}

func TestMindmapIconNames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src, icon string
	}{
		{"fa fa-book", "fa:book"},
		{"fas fa-book", "fas:book"},
		{"fa-solid fa-book", "fas:book"},
		{"fab fa-github", "fab:github"},
		{"mdi mdi-skull-outline", "mdi:skull-outline"},
		{"mdi:home", "mdi:home"},
	}
	for _, test := range tests {
		items, report, err := drawDiagram(t, "mindmap\n  a\n  ::icon("+test.src+")\n", false)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, it := range items {
			found = found || it.datum["icon"] == test.icon
		}
		if !found {
			t.Errorf("%q: no icon %s, the report %v", test.src, test.icon, report)
		}
	}
}

func TestMindmapErrors(t *testing.T) {
	t.Parallel()
	tests := []string{
		"mindmap\n",
		"mindmap\n  a\nb\n",
		"mindmap\n  a[b\n",
		"mindmap\n  a[b]x\n",
		"mindmap\n  a(\"b)\n",
		"mindmap\n  a{b}\n",
		"mindmap\n  ::icon(fa fa-book)\n  a\n",
		"mindmap\n  :::big\n  a\n",
		"mindmap extra\n  a\n",
		"mindmap\n  a[\"`b\n",
	}
	for _, src := range tests {
		if _, err := chartOf(src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: the error %v", src, err)
		}
	}
}

func TestMindmapIssues(t *testing.T) {
	t.Parallel()
	body := "mindmap\n  root\n    child\n"
	tests := []struct {
		src, issue string
	}{
		{body + "    :::urgent\n", "the class urgent"},
		{body + "    ::icon(book)\n", "names no pack"},
		{"---\nconfig:\n  mindmap:\n    useMaxWidth: false\n---\n" + body, "mindmap.useMaxWidth"},
		{"---\nconfig:\n  mindmap:\n    colorBy: section\n---\n" + body, "mindmap.colorBy section"},
		{"---\nconfig:\n  layout: elk\n---\n" + body, "layout elk"},
		{"---\nconfig:\n  theme: dark\n---\n" + body, "the theme of Mermaid"},
		{"---\nconfig:\n  look: handDrawn\n---\n" + body, "handDrawn"},
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
	for _, src := range []string{
		"---\nconfig:\n  layout: tidy-tree\n  mindmap:\n    layoutAlgorithm: cose-bilkent\n    padding: 4\n    maxNodeWidth: 90\n---\n" + body,
	} {
		if report, err := chartOf(src, true); err != nil || len(report) != 0 {
			t.Errorf("%q: the report %v and the error %v", src, report, err)
		}
	}
	// An icon that no pack holds draws as a box with a question mark, and
	// the report names it.
	src := body + "    ::icon(fa fa-no-such-icon)\n"
	_, report, err := drawDiagram(t, src, false)
	if err != nil || len(report) != 1 || !strings.Contains(report[0].Error(), "fa:no-such-icon") {
		t.Errorf("an icon that does not load: the report %v and the error %v", report, err)
	}
	if _, _, err := drawDiagram(t, src, true); !errors.Is(err, icon.ErrMissing) {
		t.Errorf("an icon that does not load, in the strict mode: the error %v", err)
	}
}
