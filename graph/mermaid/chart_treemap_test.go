package mermaid_test

import (
	"errors"
	"path/filepath"
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
