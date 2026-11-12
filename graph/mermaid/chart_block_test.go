package mermaid_test

import (
	"errors"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/vega"
)

// blockWant is what a fixture of a block diagram draws: the label of each
// block, the shape of some blocks, the title of each composite block, the
// rows of blocks that line up left to right, and the number of edges,
// arrowheads and labels of edges.
type blockWant struct {
	labels             map[string]string
	shapes             map[string]string
	composites         map[string]string
	rows               [][]string
	edges, arrows, eLb int
}

func TestBlockFixtures(t *testing.T) {
	t.Parallel()
	cases := map[string]blockWant{
		"01-grid.mmd": {
			labels: map[string]string{
				"a": "A wide label", "b": "b", "c": "c", "d": "d", "e": "e", "f": "f", "g": "g", "h": "h", "i": "i", "overflow": "overflow",
			},
			rows: [][]string{{"a", "b"}, {"c", "d"}, {"e"}, {"f", "g"}, {"h"}, {"i", "overflow"}},
		},
		"02-shapes.mmd": {
			labels: map[string]string{
				"sq": "Square", "rd": "Rounded", "ci": "Circle", "dc": "Double", "di": "Diamond", "hx": "Hexagon",
				"st": "Stadium", "sr": "Subroutine", "cy": "Database", "od": "Odd", "lr": "Lean right", "ll": "Lean left",
				"tb": "Trapezoid", "tt": "Inverted", "br": "Two lines", "am": "<tag>",
			},
			shapes: map[string]string{
				"sq": "rect", "rd": "rounded", "ci": "circle", "dc": "doublecircle", "di": "diamond", "hx": "hexagon",
				"st": "stadium", "sr": "subroutine", "cy": "cylinder", "od": "odd", "lr": "lean-r", "ll": "lean-l",
				"tb": "trap-b", "tt": "trap-t", "br": "rect",
			},
			rows: [][]string{{"sq", "rd", "ci", "dc"}, {"di", "hx", "st", "sr"}, {"cy", "od", "lr", "ll"}, {"tb", "tt", "br", "am"}},
		},
		"03-composite.mmd": {
			labels: map[string]string{
				"top": "top", "h": "h", "i": "i", "j": "j", "k": "k", "g": "g", "l": "l", "m": "m", "n": "n",
				"inner1": "inner1", "inner2": "inner2",
			},
			composites: map[string]string{"group1": "Group one", "group2": "", "block-1": ""},
			rows:       [][]string{{"top"}, {"group1", "g"}, {"h", "i"}, {"j", "k"}, {"l", "m", "n", "block-1"}},
		},
		"04-arrows.mmd": {
			labels: map[string]string{"down": "down", "right": "right", "both": "both", "left": "left", "up": "up", "wide": "wide", "tall": "tall"},
			shapes: map[string]string{"down": "blockArrow", "wide": "blockArrow"},
			rows:   [][]string{{"down"}, {"right", "both", "left"}, {"up"}, {"wide", "tall"}},
		},
		"05-edges.mmd": {
			labels:     map[string]string{"A": "A", "B": "B", "C": "C", "D": "D", "E": "E", "F": "F", "G1": "G1", "H": "H"},
			composites: map[string]string{"G": ""},
			rows:       [][]string{{"A", "B"}, {"C", "D"}, {"E", "F"}, {"G", "H"}},
			edges:      8, arrows: 7, eLb: 1,
		},
		"06-styles.mmd": {
			labels:     map[string]string{"a": "Default", "b": "Blue", "c": "Styled", "d": "Boxed"},
			composites: map[string]string{"box": ""},
			rows:       [][]string{{"a", "b", "c"}},
		},
	}
	files := fixtures(t, "block")
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
			checkBlocks(t, items, want)
		})
	}
}

// drawnPrefix returns the items of the marks whose names start with a
// prefix, as the marks of a block diagram that split by their style.
func drawnPrefix(items []drawnItem, prefix string) []drawnItem {
	var out []drawnItem
	for _, it := range items {
		if strings.HasPrefix(it.role, prefix) && !strings.HasPrefix(it.role, prefix+"Label") {
			out = append(out, it)
		}
	}
	return out
}

// checkBlocks checks the blocks, the composite blocks and the edges of a
// block diagram that drew.
func checkBlocks(t *testing.T, items []drawnItem, want blockWant) {
	t.Helper()
	boxes := map[string]box{}
	parents := map[string]string{}
	for _, it := range drawnPrefix(items, "blocks") {
		id := datumID(it)
		if _, ok := boxes[id]; ok {
			t.Errorf("the block %s draws twice", id)
		}
		x, y, w, h := num(it.datum["x"]), num(it.datum["y"]), num(it.datum["width"]), num(it.datum["height"])
		boxes[id] = box{x - w/2, y - h/2, x + w/2, y + h/2}
		parents[id], _ = it.datum["parent"].(string)
		if s, ok := want.shapes[id]; ok && it.datum["shape"] != s {
			t.Errorf("the block %s has the shape %v, expected %s", id, it.datum["shape"], s)
		}
	}
	if len(boxes) != len(want.labels) {
		t.Errorf("%d blocks, expected %d", len(boxes), len(want.labels))
	}
	labels := map[string][]string{}
	for _, it := range drawnPrefix(items, "labels") {
		labels[datumID(it)] = append(labels[datumID(it)], it.text)
	}
	for id, label := range want.labels {
		if got := strings.Join(labels[id], " "); got != label {
			t.Errorf("the block %s draws the label %q, expected %q", id, got, label)
		}
	}
	groups := map[string]box{}
	for _, it := range drawnPrefix(items, "composites") {
		id := datumID(it)
		groups[id] = boxOf(it)
		parents[id], _ = it.datum["parent"].(string)
	}
	titles := map[string]string{}
	for _, it := range drawnPrefix(items, "titles") {
		titles[datumID(it)] = it.text
	}
	if len(groups) != len(want.composites) {
		t.Errorf("%d composite blocks, expected %d", len(groups), len(want.composites))
	}
	for id, title := range want.composites {
		if titles[id] != title {
			t.Errorf("the composite block %s draws the title %q, expected %q", id, titles[id], title)
		}
	}
	// A block is inside the composite block that holds it, and no two
	// blocks of a grid overlap.
	all := maps.Clone(boxes)
	maps.Copy(all, groups)
	for id, b := range all {
		if p := parents[id]; p != "" && !b.inside(groups[p]) {
			t.Errorf("%s is not inside its composite block %s", id, p)
		}
		for other, o := range all {
			if id < other && parents[id] == parents[other] && b.overlaps(o) {
				t.Errorf("%s and %s overlap", id, other)
			}
		}
	}
	// The blocks of a row share their middle, from left to right, and each
	// row is below the one before.
	center := func(id string) (float64, float64) {
		b := all[id]
		return (b.x0 + b.x1) / 2, (b.y0 + b.y1) / 2
	}
	for _, row := range want.rows {
		_, y0 := center(row[0])
		for i := 1; i < len(row); i++ {
			px, _ := center(row[i-1])
			x, y := center(row[i])
			if math.Abs(y-y0) > 0.5 || x <= px {
				t.Errorf("the row %v: %s is at %v, %v", row, row[i], x, y)
			}
		}
	}
	counts := []struct {
		name      string
		got, want int
	}{
		{"edges", len(drawnPrefix(items, "edges")), want.edges},
		{"arrowheads", len(drawnOf(items, "arrows")) + len(drawnOf(items, "openArrows")), want.arrows},
		{"labels of edges", len(drawnOf(items, "edgeLabels")), want.eLb},
	}
	for _, c := range counts {
		if c.got != c.want {
			t.Errorf("%d %s, expected %d", c.got, c.name, c.want)
		}
	}
}

// blockSpec compiles a block diagram and returns the rows of the datasets
// of its spec by name.
func blockSpec(t *testing.T, src string) map[string][]map[string]any {
	t.Helper()
	items, report, err := drawDiagram(t, src, true)
	if err != nil || len(report) != 0 {
		t.Fatalf("%q: the report %v and the error %v", src, report, err)
	}
	out := map[string][]map[string]any{}
	for _, it := range items {
		switch {
		case strings.HasPrefix(it.role, "blocks"):
			out["blocks"] = append(out["blocks"], it.datum)
		case strings.HasPrefix(it.role, "composites"):
			out["composites"] = append(out["composites"], it.datum)
		case strings.HasPrefix(it.role, "edges"):
			out["edges"] = append(out["edges"], it.datum)
		}
	}
	return out
}

func TestBlockReading(t *testing.T) {
	t.Parallel()
	ids := func(rows []map[string]any) string {
		var s []string
		for _, r := range rows {
			id, _ := r["id"].(string)
			s = append(s, id)
		}
		return strings.Join(s, " ")
	}
	// Statements part at white space and new lines, as in Mermaid, so a
	// line holds more than one statement, and columns goes on to the next
	// line.
	got := blockSpec(t, "block-beta columns\n2 a b c")
	if ids(got["blocks"]) != "a b c" {
		t.Errorf("the blocks %s", ids(got["blocks"]))
	}
	if x0, x2 := num(got["blocks"][0]["x"]), num(got["blocks"][2]["x"]); x0 != x2 {
		t.Errorf("columns 2 puts c at %v and a at %v", x2, x0)
	}
	// A link adds the blocks it names that no statement declares, in the
	// grid where it is, and a chain of links draws each link.
	got = blockSpec(t, "block-beta\n  a\n  block\n    a --> b --> c\n  end\n")
	if ids(got["blocks"]) != "a b c" || len(got["edges"]) != 2 {
		t.Errorf("the blocks %s and %d edges", ids(got["blocks"]), len(got["edges"]))
	}
	for _, r := range got["blocks"][1:] {
		if r["parent"] != "block-1" {
			t.Errorf("%s is in %v", r["id"], r["parent"])
		}
	}
	// A second statement of a block sets its shape and its label, and its
	// width does not count, as in Mermaid. A block before a link keeps a
	// width of one column.
	got = blockSpec(t, "block-beta\n  columns 3\n  a\n  a(\"again\")\n  a:2\n  b:2 --> c\n")
	if len(got["blocks"]) != 3 || got["blocks"][0]["label"] != "again" || got["blocks"][0]["shape"] != "rounded" {
		t.Fatalf("the blocks %v", got["blocks"])
	}
	if w0, w1 := num(got["blocks"][0]["width"]), num(got["blocks"][1]["width"]); w0 != w1 {
		t.Errorf("a is %v wide and b %v", w0, w1)
	}
	// ~~~ is the id of a block, since an id of Mermaid can hold ~, and an
	// id can hold a dot.
	got = blockSpec(t, "block-beta\n  a ~~~ id2.1\n")
	if ids(got["blocks"]) != "a ~~~ id2.1" {
		t.Errorf("the blocks %s", ids(got["blocks"]))
	}
	// block BL is a composite block with no id that holds the block BL.
	got = blockSpec(t, "block\n  B\n  block BL\n    D\n  end\n  BL --> B\n")
	if ids(got["blocks"]) != "B BL D" || ids(got["composites"]) != "block-1" || len(got["edges"]) != 1 {
		t.Errorf("the blocks %s, the composite blocks %s and %d edges", ids(got["blocks"]), ids(got["composites"]), len(got["edges"]))
	}
	// Each kind of link. An x or an o before a link is a part of the id
	// before it, as in Mermaid, so only < marks the start of a link.
	got = blockSpec(t, "block-beta\n  a b\n  a --- b\n  a ==> b\n  a -.-> b\n  a <--x b\n  a --o b\n  a == \"t\" ==> b\n")
	want := []struct {
		width float64
		dash  bool
	}{{0, false}, {2.5, false}, {0, true}, {0, false}, {0, false}, {2.5, false}}
	edges := got["edges"]
	slices.SortFunc(edges, func(x, y map[string]any) int { return int(num(x["key"]) - num(y["key"])) })
	for i, w := range want {
		e := edges[i]
		if num(e["strokeWidth"]) != w.width || (e["strokeDash"] != nil) != w.dash {
			t.Errorf("the edge %d: %v", i, e)
		}
	}
	items, _, err := drawDiagram(t, "block-beta\n  a b\n  a <--x b\n  a --o b\n  a --- b\n", true)
	if err != nil {
		t.Fatal(err)
	}
	ends := map[string]int{}
	for _, it := range items {
		if it.role == "arrows" || it.role == "openArrows" {
			kind, _ := it.datum["kind"].(string)
			end, _ := it.datum["end"].(string)
			ends[kind+" at "+end]++
		}
	}
	if len(ends) != 3 || ends["normal at a"] != 1 || ends["cross at b"] != 1 || ends["circle at b"] != 1 {
		t.Errorf("the arrowheads %v", ends)
	}
}

func TestBlockErrors(t *testing.T) {
	t.Parallel()
	tests := []string{
		"block-beta",
		"block-beta\n  a[label]\n",
		"block-beta\n  a[\"\"]\n",
		"block-beta\n  a[\"`**md**`\"]\n",
		"block-beta\n  a[\"x\"\n",
		"block-beta\n  a[ \"x\" ]\n",
		"block-beta\n  a<[\"x\"]>\n",
		"block-beta\n  a<[\"x\"]>()\n",
		"block-beta\n  a<[\"x\"]>(north)\n",
		"block-beta\n  accTitle: A title\n  a\n",
		"block-beta\n  accDescr: A description\n  a\n",
		"block-beta\n  a b\n  linkStyle 0 stroke:red\n",
		"block-beta\n  \"text\"\n",
		"block-beta\n  block\n    a\n",
		"block-beta\n  block\n  end\n",
		"block-beta\n  a\n  end\n",
		"block-beta\n  block:g\n    a\n  end\n  block:g\n    b\n  end\n",
		"block-beta\n  a\n  block:a\n    b\n  end\n",
		"block-beta\n  block:\n  end\n",
		"block-beta\n  a --> \n",
		"block-beta\n  --> a\n",
		"block-beta\n  a -- \"x\" b\n",
		"block-beta\n  a --> b:2\n",
		"block-beta\n  :2\n",
		"block-beta\n  a b\n  style a,b fill:#f00\n",
		"block-beta\n  a\n  style z fill:#f00\n",
		"block-beta\n  columns 0\n  a\n",
		"block-beta\n  a:0\n",
		"block-beta\n  a = b\n",
	}
	for _, src := range tests {
		if _, err := chartOf(src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: the error %v", src, err)
		}
	}
	// An error names the line of the source.
	_, err := chartOf("---\ntitle: T\n---\nblock-beta\n  a\n\n  b[x]\n", false)
	if err == nil || !strings.HasPrefix(err.Error(), "line 7: ") {
		t.Errorf("the error %v, not at line 7", err)
	}
}

func TestBlockIssues(t *testing.T) {
	t.Parallel()
	body := "block-beta\n  a b\n"
	tests := []struct {
		src, issue string
	}{
		{"---\nconfig:\n  block:\n    spacing: 4\n---\n" + body, "block.spacing"},
		{"---\nconfig:\n  block:\n    padding: wide\n---\n" + body, "block.padding"},
		{body + "  style a fill#969;\n", `the style "fill#969"`},
		{body + "  style a fill:#969,filter:blur(2px)\n", "the style filter"},
		{body + "  class a blue\n", "the class blue"},
		{body + "  classDef blue fill:#00f\n  class z blue\n", "the class of z"},
		{body + "  a --> a\n", "the edge from a to itself"},
		{body + "  block:g[(\"G\")]\n    c\n  end\n", "the shape of the composite block g"},
		{body + "  block:g\n    c\n  end\n  g((\"G\"))\n", "the shape of g"},
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
		"---\nconfig:\n  block:\n    padding: 4\n    useMaxWidth: false\n---\n" + body,
		body + "  classDef default fill:#eee\n  classDef blue fill:#00f;\n  class a blue\n  style b fill:#f00;\n",
		"block\n  columns auto\n  a:2 space:2 b\n",
	} {
		if report, err := chartOf(src, true); err != nil || len(report) != 0 {
			t.Errorf("%q: the report %v and the error %v", src, report, err)
		}
	}
}
