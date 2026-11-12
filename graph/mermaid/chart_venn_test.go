package mermaid_test

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/vega"
)

// vennWant is what a fixture of a venn diagram draws: the label of each
// set, in the order of the source, the labels of the unions, the texts,
// and the count of the overlaps that a style fills.
type vennWant struct {
	sets, unions, texts []string
	fills               int
}

func TestVennFixtures(t *testing.T) {
	t.Parallel()
	cases := map[string]vennWant{
		"01-two.mmd": {sets: []string{"Frontend", "Backend"}, unions: []string{"APIs"}},
		"02-three.mmd": {
			sets:   []string{"Desirable", "Feasible", "Viable"},
			unions: []string{"Buildable", "Sustainable", "Marketable", "Ship it"},
		},
		// A set of size 0 draws nothing, nor its union.
		"03-sizes.mmd": {sets: []string{"Alpha", "Beta", "Gamma"}, unions: []string{"AB"}},
		"04-texts.mmd": {
			sets:  []string{"Fruits", "Greens"},
			texts: []string{"Apple", "Banana", "Orange", "Carrot", "2", "Tomato", "Pepper and more"},
		},
		"05-styles.mmd":  {sets: []string{"Alpha", "Beta", "Gamma"}, unions: []string{"Both"}, texts: []string{"Red text"}, fills: 1},
		"06-apart.mmd":   {sets: []string{"A", "B", "C", "D", "E"}},
		"07-grammar.mmd": {sets: []string{"One", "Two", "Three"}},
	}
	files := fixtures(t, "venn")
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
			items, report, err := drawDiagram(t, string(src), true)
			if err != nil || len(report) != 0 {
				t.Fatalf("the report %v and the error %v", report, err)
			}
			checkVenn(t, items, want)
		})
	}
}

// vennCircle is a circle that a venn diagram draws.
type vennCircle struct {
	x, y, r float64
	key     string
}

// checkVenn checks that each set draws one circle, in a color of its own,
// with its label inside it, and that each label of a union and each text
// draws once.
func checkVenn(t *testing.T, items []drawnItem, want vennWant) {
	t.Helper()
	var circles []vennCircle
	for _, it := range drawnOf(items, "circles") {
		// The path of a circle starts at its right end, the radius right of
		// its center.
		r, err := strconv.ParseFloat(strings.Split(strings.TrimPrefix(it.item.Path, "M"), ",")[0], 64)
		if err != nil {
			t.Fatalf("the path %q: %v", it.item.Path, err)
		}
		key, _ := it.datum["key"].(string)
		circles = append(circles, vennCircle{it.item.X, it.item.Y, r, key})
	}
	if len(circles) != len(want.sets) {
		t.Fatalf("%d circles, want %d", len(circles), len(want.sets))
	}
	if n := len(drawnOf(items, "outlines")); n != len(circles) {
		t.Errorf("%d outlines for %d circles", n, len(circles))
	}
	var keys []string
	for _, c := range circles {
		if slices.Contains(keys, c.key) {
			t.Errorf("two circles take the color %s", c.key)
		}
		keys = append(keys, c.key)
	}
	labels := drawnOf(items, "setLabels")
	if len(labels) != len(want.sets) {
		t.Fatalf("%d labels of sets, want %d", len(labels), len(want.sets))
	}
	for i, it := range labels {
		if it.text != want.sets[i] {
			t.Errorf("the label %d is %q, want %q", i, it.text, want.sets[i])
		}
		c := circles[i]
		if d := math.Hypot(it.item.X-c.x, it.item.Y-c.y); d >= c.r {
			t.Errorf("the label %q is %.1f from the center of its circle of radius %.1f", it.text, d, c.r)
		}
	}
	texts := func(mark string) []string {
		var out []string
		for _, it := range drawnOf(items, mark) {
			out = append(out, it.text)
		}
		return out
	}
	if got := texts("overlapLabels"); !slices.Equal(got, want.unions) {
		t.Errorf("the labels of the unions %q, want %q", got, want.unions)
	}
	if got := texts("texts"); !slices.Equal(got, want.texts) {
		t.Errorf("the texts %q, want %q", got, want.texts)
	}
	if n := len(drawnOf(items, "overlaps")); n != want.fills {
		t.Errorf("%d overlaps filled, want %d", n, want.fills)
	}
}

func TestVennErrors(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		// A venn diagram needs a set.
		"venn-beta\n",
		"venn-beta\n  title X\n",
		// A set takes one id, and a union two or more that a set names.
		"venn-beta\n  set A,B\n",
		"venn-beta\n  set A\n  union A\n",
		"venn-beta\n  set A\n  union A,B\n",
		"venn-beta\n  union A,B\n  set A\n  set B\n",
		// A size is a number of 0 or more.
		"venn-beta\n  set A:-4\n",
		"venn-beta\n  set A:x\n",
		// Mermaid reads no accTitle and no accDescr in a venn diagram.
		"venn-beta\n  accTitle: Sets\n  set A\n",
		"venn-beta\n  accDescr: Sets\n  set A\n",
		// A text needs an id, and a text with no set before it a list of
		// sets. An indented text follows a set or a union only.
		"venn-beta\n  set A\n  text \"x\" \"y\"\n",
		"venn-beta\n  set A\ntext \"x\"\n",
		"venn-beta\n  set A\n  title T\n  text \"x\"\n",
		// A style needs a name, a colon and a value.
		"venn-beta\n  set A\n  style A fill\n",
		"venn-beta\n  set A\n  style A fill:red stroke:blue\n",
		// The keyword comes once, and a word that no rule reads is an
		// error.
		"venn-beta\n  set A\n  venn-beta\n",
		"venn-beta\n  set A\n  set B;\n",
		"venn-beta\n  set A\n  title X # comment\n",
	} {
		_, err := chartOf(src, false)
		if !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: expected a syntax error, got %v", src, err)
		}
	}
}

func TestVennIssues(t *testing.T) {
	t.Parallel()
	const body = "venn-beta\n  set A\n"
	for _, src := range []string{
		// Mermaid draws in the theme of venn.theme, which zuzu does not
		// read.
		"---\nconfig:\n  venn:\n    theme: forest\n---\n" + body,
		"---\nconfig:\n  venn:\n    useDebugLayout: true\n---\n" + body,
		"---\nconfig:\n  venn:\n    width: -5\n---\n" + body,
		"---\nconfig:\n  venn:\n    gap: 5\n---\n" + body,
		"---\nconfig:\n  themeVariables:\n    venn1: '#ff0000'\n---\n" + body,
		body + "  style A fill:nocolor\n",
		body + "  style A color:\"#12\"\n",
	} {
		if _, err := chartOf(src, true); !errors.Is(err, vega.ErrUnsupported) {
			t.Errorf("%q: expected an unsupported feature in the strict mode, got %v", src, err)
		}
		report, err := chartOf(src, false)
		if err != nil || len(report) != 1 {
			t.Errorf("%q: expected one line of the report, got %v and %v", src, report, err)
		}
	}
	// The settings that zuzu reads, a hint of the look, the style of a
	// target that does not exist, and a style that Mermaid does not draw
	// are no issue.
	for _, src := range []string{
		"---\nconfig:\n  venn:\n    width: 600\n    height: 300\n    padding: 0\n    useDebugLayout: false\n    look: handDrawn\n    useMaxWidth: false\n---\nvenn-beta\n  set A\n",
		"---\nconfig:\n  look: handDrawn\n  handDrawnSeed: 1\n  fontFamily: courier\n  theme: redux-dark-color\n---\nvenn-beta\n  set A\n",
		"venn-beta\n  set A\n  style Z fill:red\n  style A font-size:20px, fill-opacity:x\n",
	} {
		if report, err := chartOf(src, true); err != nil || len(report) != 0 {
			t.Errorf("%q: expected no issue, got %v and %v", src, report, err)
		}
	}
}

func TestVennDescription(t *testing.T) {
	t.Parallel()
	d, err := mermaid.Read("---\ntitle: Front\ndescription: Two sets\n---\nvenn-beta\n  set A\n")
	if err != nil {
		t.Fatal(err)
	}
	b, _, _, err := mermaid.Chart(d, true)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"description":"Two sets"`, `"text":"Front"`} {
		if !strings.Contains(s, want) {
			t.Errorf("the spec has no %s", want)
		}
	}
	// The title of the source wins over the title of the front matter.
	d, err = mermaid.Read("---\ntitle: Front\n---\nvenn-beta\n  title Source\n  set A\n")
	if err != nil {
		t.Fatal(err)
	}
	if b, _, _, err = mermaid.Chart(d, true); err != nil || !strings.Contains(string(b), `"text":"Source"`) {
		t.Errorf("the title of the source does not draw: %v", err)
	}
}

// TestVennWideTexts checks that the texts of an area with a word that is
// wider than its cell stack in one column, under the label of the area,
// and do not draw over each other. Mermaid draws REST and GraphQL here on
// top of each other.
func TestVennWideTexts(t *testing.T) {
	t.Parallel()
	src := "venn-beta\n  set A\n  set B\n  union A, B[\"Fullstack\"]\n    text \"REST\"\n    text \"GraphQL\"\n"
	items, report, err := drawDiagram(t, src, true)
	if err != nil || len(report) != 0 {
		t.Fatalf("the report %v and the error %v", report, err)
	}
	at := func(name, text string) *drawnItem {
		for _, it := range drawnOf(items, name) {
			if it.text == text {
				return &it
			}
		}
		t.Fatalf("no %s %q", name, text)
		return nil
	}
	label, rest, gql := at("overlapLabels", "Fullstack"), at("texts", "REST"), at("texts", "GraphQL")
	if rest.item.Y <= label.item.Y {
		t.Errorf("REST at y %g is not under the label at y %g", rest.item.Y, label.item.Y)
	}
	if d := gql.item.Y - rest.item.Y; d < 10 {
		t.Errorf("GraphQL is %g under REST, so the two texts overlap", d)
	}
}
