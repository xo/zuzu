package mermaid_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/vega"
)

// treemapNode is a node that a fixture of a treemap draws: its name, its
// value on a leaf or 0 on a section, and the text of its value or total.
type treemapNode struct {
	name    string
	value   float64
	text    string
	section bool
}

func TestTreemapFixtures(t *testing.T) {
	t.Parallel()
	sec := func(name, text string) treemapNode { return treemapNode{name: name, text: text, section: true} }
	leaf := func(name string, v float64, text string) treemapNode {
		return treemapNode{name: name, value: v, text: text}
	}
	cases := map[string][]treemapNode{
		"01-basic.mmd": {
			sec("Category A", "30"), leaf("Item A1", 10, "10"), leaf("Item A2", 20, "20"),
			sec("Category B", "40"), leaf("Item B1", 15, "15"), leaf("Item B2", 25, "25"),
		},
		"02-nested.mmd": {
			sec("Products", "180"), sec("Electronics", "100"), leaf("Phones", 50, "50"), leaf("Computers", 30, "30"),
			leaf("Accessories", 20, "20"), sec("Clothing", "80"), leaf("Men's", 40, "40"), leaf("Women's", 40, "40"),
		},
		"03-styled.mmd": {
			sec("Section 1", "24"), leaf("Leaf 1.1", 12, "12"), sec("Section 1.2", "12"), leaf("Leaf 1.2.1", 12, "12"),
			sec("Section 2", "57"), leaf("Leaf 2.1", 20, "20"), leaf("Leaf 2.2", 25, "25"), leaf("Leaf 2.3", 12, "12"),
		},
		"04-budget.mmd": {
			sec("Budget", "$1,500,000"), sec("Operations", "$1,000,000"), leaf("Salaries", 700000, "$700,000"),
			leaf("Equipment", 200000, "$200,000"), leaf("Supplies", 100000, "$100,000"), sec("Marketing", "$500,000"),
			leaf("Advertising", 400000, "$400,000"), leaf("Events", 100000, "$100,000"),
		},
		"05-tabs.mmd": {
			sec("Main", "50"), leaf("A", 20, "20"), sec("B", "25"), leaf("B1", 10, "10"), leaf("B2", 15, "15"),
			leaf("C", 5, "5"), leaf("Side", 30, "30"),
		},
		"06-fit.mmd": {
			sec("Source files", "113"), leaf("internal/render/okage.go", 60, "60"),
			leaf("internal/render/svg_writer.go", 30, "30"), leaf("layout/axis_x.go", 14, "14"), leaf("cmd/main.go", 9, "9"),
		},
		"07-dashes.mmd": {
			sec("Plans", "80"), leaf("Basic", 30, "30"), leaf("Pro", 50, "50"),
			sec("Add-ons", "25"), leaf("Storage", 15, "15"), leaf("Support", 10, "10"),
		},
	}
	files := fixtures(t, "treemap")
	if len(files) != len(cases) {
		t.Fatalf("%d fixtures and %d cases", len(files), len(cases))
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			nodes, ok := cases[filepath.Base(path)]
			if !ok {
				t.Fatal("no case")
			}
			items, report := drawFixture(t, path)
			if len(report) != 0 {
				t.Errorf("report: %v", report)
			}
			boxes := countDrawn(items, scene.Rect, func(it drawnItem) bool { d, _ := it.datum["depth"].(float64); return d > 0 })
			if boxes != len(nodes) {
				t.Errorf("%d boxes for %d nodes", boxes, len(nodes))
			}
			for _, n := range nodes {
				box := countDrawn(items, scene.Rect, func(it drawnItem) bool {
					children, _ := it.datum["children"].(float64)
					if it.datum["name"] != n.name || (children > 0) != n.section {
						return false
					}
					return n.section || it.datum["size"] == n.value
				})
				label := countDrawn(items, scene.Text, func(it drawnItem) bool { return it.datum["name"] == n.name && it.text == n.name })
				value := countDrawn(items, scene.Text, func(it drawnItem) bool { return it.datum["name"] == n.name && it.text == n.text })
				if box != 1 || label != 1 || value != 1 {
					t.Errorf("the node %q draws %d boxes, %d labels and %d values", n.name, box, label, value)
				}
			}
		})
	}
}

func TestTreemapDashes(t *testing.T) {
	t.Parallel()
	items, _ := drawFixture(t, "testdata/treemap/07-dashes.mmd")
	for name, want := range map[string][]float64{"Basic": {6, 3}, "Add-ons": {2}, "Pro": nil} {
		n := countDrawn(items, scene.Rect, func(it drawnItem) bool {
			return it.datum["name"] == name && slices.Equal(it.item.StrokeDash, want)
		})
		if n != 1 {
			t.Errorf("the box of %q does not draw the dashes %v", name, want)
		}
	}
	// A dash that is not a length is an error.
	src := "treemap-beta\n\"A\": 1:::c\nclassDef c stroke-dasharray:a b;"
	if _, err := chartOf(src, false); !errors.Is(err, mermaid.ErrSyntax) {
		t.Errorf("%q: the error %v", src, err)
	}
}

func TestTreemapClasses(t *testing.T) {
	t.Parallel()
	items, _ := drawFixture(t, "testdata/treemap/03-styled.mmd")
	for _, name := range []string{"Section 1.2", "Leaf 2.1"} {
		box := countDrawn(items, scene.Rect, func(it drawnItem) bool {
			return it.datum["name"] == name && it.item.Fill.Color == "red" && it.item.Stroke.Color == "#FFD600"
		})
		label := countDrawn(items, scene.Text, func(it drawnItem) bool {
			return it.datum["name"] == name && it.text == name && it.item.Fill.Color == "blue"
		})
		if box != 1 || label != 1 {
			t.Errorf("the class of %q draws %d boxes and %d labels", name, box, label)
		}
	}
	plain := countDrawn(items, scene.Rect, func(it drawnItem) bool {
		return it.datum["name"] == "Leaf 2.2" && it.item.Fill.Color != "red" && it.item.Stroke.Color == ""
	})
	if plain != 1 {
		t.Error("a node with no class takes the style of a class")
	}
}

// leafText returns the size of the name and of the value of a leaf, or 0
// for one that does not draw, and whether its name ends with an ellipsis.
// A name is bold and a value is not.
func leafText(items []drawnItem, name string) (float64, float64, bool) {
	var label, value float64
	cut := false
	for _, it := range items {
		if children, _ := it.datum["children"].(float64); it.mark != scene.Text || it.datum["name"] != name || children > 0 {
			continue
		}
		if it.item.FontWeight == "600" {
			label = it.item.FontSize
			cut = strings.HasSuffix(it.text, "…")
		} else {
			value = it.item.FontSize
		}
	}
	return label, value, cut
}

// drawTreemap draws the source of a treemap.
func drawTreemap(t *testing.T, src string) []drawnItem {
	t.Helper()
	path := filepath.Join(t.TempDir(), "treemap.mmd")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	items, report := drawFixture(t, path)
	if len(report) != 0 {
		t.Errorf("report: %v", report)
	}
	return items
}

func TestTreemapFit(t *testing.T) {
	t.Parallel()
	items, _ := drawFixture(t, "testdata/treemap/06-fit.mmd")
	// A name that fits draws at labelFontSize, and a name in a narrow
	// leaf gets smaller, with its value.
	if label, value, _ := leafText(items, "cmd/main.go"); label != 11 || value != 10 {
		t.Errorf("a name that fits draws at %v with its value at %v", label, value)
	}
	label, value, cut := leafText(items, "layout/axis_x.go")
	if label < 8 || label >= 11 || value >= 10 || value == 0 || cut {
		t.Errorf("a name in a narrow leaf draws at %v with its value at %v, cut %v", label, value, cut)
	}
	// In a small treemap, a name that does not fit at 8 does not draw, and
	// its value does not draw either.
	items = drawTreemap(t, "treemap-beta\n\"Files\"\n    \"a\": 60\n    \"a much longer name of a file.go\": 3\n")
	if label, value, _ := leafText(items, "a much longer name of a file.go"); label != 0 || value != 0 {
		t.Errorf("a name that does not fit draws at %v with its value at %v", label, value)
	}
	// In a treemap of more than 20 leaves, a name gets as small as 4 and
	// is cut with no ellipsis.
	var b strings.Builder
	b.WriteString("treemap-beta\n\"Files\"\n    \"a\": 40\n")
	for i := range 21 {
		fmt.Fprintf(&b, "    \"packages/app/tests/file%d.go\": 1\n", i)
	}
	items = drawTreemap(t, b.String())
	small := 0
	for i := range 21 {
		label, _, cut := leafText(items, fmt.Sprintf("packages/app/tests/file%d.go", i))
		if cut {
			t.Errorf("the name of leaf %d ends with an ellipsis", i)
		}
		if label == 4 {
			small++
		}
	}
	if small == 0 {
		t.Error("no name draws at 4")
	}
}

func TestTreemapErrors(t *testing.T) {
	t.Parallel()
	tests := []string{
		"treemap\nLeaf: 1\n",
		"treemap\n\"Leaf\": x\n",
		"treemap\n\"Leaf\": -1\n",
		"treemap\n",
		"treemap\n\"A\"\n  \"B\": 1\nclassDef c fill\n",
		"treemap\n\"A\"\n  \"B\": 1\nclassDef c stroke-width:wide\n",
	}
	for _, src := range tests {
		if _, err := chartOf(src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: the error %v", src, err)
		}
	}
}

func TestTreemapIssues(t *testing.T) {
	t.Parallel()
	body := "treemap-beta\n\"A\"\n    \"B\": 1\n"
	tests := []struct {
		src, issue string
	}{
		{"treemap-beta\n\"A\": 5\n    \"B\": 1\n", "the value 5 of the section"},
		{"treemap-beta\n\"A\"\n    \"B\"\n", "the leaf \"B\" with no value"},
		{"treemap-beta\n\"A\"\n    \"B\": 1:::nope\n", "the class nope"},
		{body + "classDef c fill:red,font-weight:bold\n", "the style font-weight"},
		{"---\nconfig:\n  treemap:\n    nodeWidth: 50\n---\n" + body, "treemap.nodeWidth"},
		{"---\nconfig:\n  treemap:\n    valueFormat: 'zz'\n---\n" + body, "valueFormat zz"},
		{body + "accDescr: A tree\n", "accessibility"},
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
