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
	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// c4Want is what a fixture of a C4 diagram draws: the shape and the
// boundary of each element, the boundary of each boundary, the rows of
// elements or boundaries that line up left to right, the number of
// relationships and arrowheads, the texts of the relationships, and the
// links of some parts.
type c4Want struct {
	shapes   map[string]string
	parents  map[string]string
	rows     [][]string
	rels     int
	arrows   int
	relTexts []string
	links    map[string]string
}

func TestC4Fixtures(t *testing.T) {
	t.Parallel()
	cases := map[string]c4Want{
		"01-context.mmd": {
			shapes: map[string]string{
				"customer": "person", "banking": "rounded", "mainframe": "cylinder", "events": "h-cyl",
				"visitor": "person", "mail": "rounded", "archive": "cylinder", "feed": "h-cyl",
			},
			parents: map[string]string{
				"customer": "bank", "banking": "bank", "mainframe": "core", "events": "core", "core": "bank",
				"bank": "", "visitor": "", "mail": "", "archive": "", "feed": "",
			},
			rows:     [][]string{{"visitor", "mail"}, {"archive", "feed"}, {"customer", "banking"}},
			rels:     4,
			arrows:   5,
			relTexts: []string{"Uses", "Reads", "[JDBC]", "Sends e-mail with", "Mails"},
		},
		"02-container.mmd": {
			shapes: map[string]string{
				"customer": "person", "spa": "rounded", "mobile": "rounded", "db": "cylinder", "api": "cylinder",
				"queue": "h-cyl", "bus": "h-cyl",
			},
			parents: map[string]string{
				"customer": "", "app": "", "spa": "app", "mobile": "app", "db": "app", "api": "app", "queue": "app", "bus": "app",
			},
			rows:     [][]string{{"spa", "mobile"}, {"db", "api"}, {"queue", "bus"}},
			rels:     3,
			arrows:   3,
			relTexts: []string{"Uses", "[HTTPS]", "Reads from and writes to", "[JDBC]", "Calls"},
			links:    map[string]string{"customer": "https://example.com/customer", "spa-api": "https://example.com/api"},
		},
		"03-component.mmd": {
			shapes: map[string]string{
				"spa": "rounded", "sign": "rounded", "accounts": "rounded", "cache": "cylinder", "store": "cylinder",
				"jobs": "h-cyl", "audit": "h-cyl", "worker": "subroutine", "legacy": "cylinder",
			},
			parents: map[string]string{
				"spa": "", "worker": "", "legacy": "", "api": "", "sign": "api", "accounts": "api", "cache": "api",
				"store": "api", "jobs": "api", "audit": "api",
			},
			rows:     [][]string{{"spa", "worker"}, {"sign", "accounts"}, {"cache", "store"}, {"jobs", "audit"}},
			rels:     2,
			arrows:   2,
			relTexts: []string{"Reads", "Calls", "[JSON/HTTPS]"},
		},
		"04-dynamic.mmd": {
			shapes:   map[string]string{"db": "cylinder", "spa": "rounded", "sign": "rounded", "security": "rounded"},
			parents:  map[string]string{"db": "", "spa": "", "b": "", "sign": "b", "security": "b"},
			rows:     [][]string{{"db", "spa"}, {"sign", "security"}},
			rels:     3,
			arrows:   3,
			relTexts: []string{"1: Submits credentials to", "[JSON/HTTPS]", "2: Checks with", "3: Reads", "[JDBC]"},
		},
		"05-deployment.mmd": {
			shapes:   map[string]string{"mobile": "rounded", "app": "rounded", "db": "cylinder"},
			parents:  map[string]string{"phone": "", "dc": "", "web": "dc", "dbs": "dc", "mobile": "phone", "app": "web", "db": "dbs"},
			rows:     [][]string{{"phone", "dc"}, {"web", "dbs"}, {"app", "db"}},
			rels:     2,
			arrows:   2,
			relTexts: []string{"Calls", "[HTTPS]", "Reads", "[JDBC]"},
		},
	}
	files := fixtures(t, "c4")
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
			checkC4(t, items, want)
		})
	}
}

// checkC4 checks the elements, the boundaries and the relationships of a
// C4 diagram that drew.
func checkC4(t *testing.T, items []drawnItem, want c4Want) {
	t.Helper()
	boxes := map[string]box{}
	parents := map[string]string{}
	links := map[string]string{}
	for _, it := range drawnPrefix(items, "elements") {
		id := datumID(it)
		if _, ok := boxes[id]; ok {
			t.Errorf("the element %s draws twice", id)
		}
		x, y, w, h := num(it.datum["x"]), num(it.datum["y"]), num(it.datum["width"]), num(it.datum["height"])
		boxes[id] = box{x - w/2, y - h/2, x + w/2, y + h/2}
		parents[id], _ = it.datum["parent"].(string)
		if s := want.shapes[id]; it.datum["shape"] != s {
			t.Errorf("the element %s has the shape %v, expected %s", id, it.datum["shape"], s)
		}
		if it.item.Href != "" {
			links[id] = it.item.Href
		}
	}
	if len(boxes) != len(want.shapes) {
		t.Errorf("%d elements, expected %d", len(boxes), len(want.shapes))
	}
	groups := map[string]box{}
	for _, it := range drawnPrefix(items, "boundaries") {
		id := datumID(it)
		groups[id] = boxOf(it)
		parents[id], _ = it.datum["parent"].(string)
	}
	for id, p := range want.parents {
		if got, ok := parents[id]; !ok || got != p {
			t.Errorf("%s is in %q, expected %q", id, got, p)
		}
	}
	// A part is inside the boundary that holds it, and no two parts of a
	// boundary overlap.
	all := maps.Clone(boxes)
	maps.Copy(all, groups)
	for id, b := range all {
		if p := parents[id]; p != "" && !b.inside(groups[p]) {
			t.Errorf("%s is not inside its boundary %s", id, p)
		}
		for other, o := range all {
			if id < other && parents[id] == parents[other] && b.overlaps(o) {
				t.Errorf("%s and %s overlap", id, other)
			}
		}
	}
	// The parts of a row share their top, from left to right.
	for _, row := range want.rows {
		y0 := all[row[0]].y0
		for i := 1; i < len(row); i++ {
			p, b := all[row[i-1]], all[row[i]]
			if math.Abs(b.y0-y0) > 0.5 || b.x0 <= p.x1 {
				t.Errorf("the row %v: %s is at %v, %v", row, row[i], b.x0, b.y0)
			}
		}
	}
	rels := drawnPrefix(items, "rels")
	for _, it := range rels {
		if it.item.Href != "" {
			links[datumID(it)] = it.item.Href
		}
	}
	if len(rels) != want.rels {
		t.Errorf("%d relationships, expected %d", len(rels), want.rels)
	}
	if n := len(drawnPrefix(items, "arrows")); n != want.arrows {
		t.Errorf("%d arrowheads, expected %d", n, want.arrows)
	}
	var texts []string
	for _, it := range drawnPrefix(items, "relTexts") {
		texts = append(texts, it.text)
	}
	slices.Sort(texts)
	wantTexts := slices.Clone(want.relTexts)
	slices.Sort(wantTexts)
	if !slices.Equal(texts, wantTexts) {
		t.Errorf("the texts of the relationships %q, expected %q", texts, wantTexts)
	}
	for id, l := range want.links {
		if links[id] != l {
			t.Errorf("the link of %s is %q, expected %q", id, links[id], l)
		}
	}
}

// c4Rows compiles a C4 diagram and returns the rows of a dataset of its
// spec by the prefix of the name of its marks.
func c4Rows(t *testing.T, src, prefix string) []map[string]any {
	t.Helper()
	items, report, err := drawDiagram(t, src, true)
	if err != nil || len(report) != 0 {
		t.Fatalf("%q: the report %v and the error %v", src, report, err)
	}
	var out []map[string]any
	for _, it := range drawnPrefix(items, prefix) {
		out = append(out, it.datum)
	}
	return out
}

func TestC4Layout(t *testing.T) {
	t.Parallel()
	tops := func(rows []map[string]any) []float64 {
		var out []float64
		for _, r := range rows {
			out = append(out, num(r["y"])-num(r["height"])/2)
		}
		return out
	}
	// The width of the screen of mmdc holds two elements in a row, and
	// c4ShapeInRow holds fewer.
	four := "C4Context\nSystem(a, \"A\")\nSystem(b, \"B\")\nSystem(c, \"C\")\nSystem(d, \"D\")\n"
	y := tops(c4Rows(t, four, "elements"))
	if y[0] != y[1] || y[1] == y[2] || y[2] != y[3] {
		t.Errorf("the tops %v, not two rows of two", y)
	}
	y = tops(c4Rows(t, four+"UpdateLayoutConfig($c4ShapeInRow=\"1\")\n", "elements"))
	if y[0] == y[1] || y[1] == y[2] {
		t.Errorf("the tops %v, not one element in a row", y)
	}
	// c4BoundaryInRow puts a boundary in a row of its own.
	two := "C4Context\nBoundary(a, \"A\") {\nSystem(x, \"X\")\n}\nBoundary(b, \"B\") {\nSystem(y, \"Y\")\n}\n"
	rows := c4Rows(t, two, "boundaries")
	if num(rows[0]["y"]) != num(rows[1]["y"]) {
		t.Errorf("the boundaries are at %v and %v, not in one row", rows[0]["y"], rows[1]["y"])
	}
	rows = c4Rows(t, two+"UpdateLayoutConfig($c4ShapeInRow=\"4\", $c4BoundaryInRow=\"1\")\n", "boundaries")
	if num(rows[0]["y"]) >= num(rows[1]["y"]) {
		t.Errorf("the boundaries are at %v and %v, not one under the other", rows[0]["y"], rows[1]["y"])
	}
	// A second statement of an element updates it in its place, and a
	// second relationship between two ends replaces the first.
	src := "C4Context\nPerson(a, \"A\")\nSystem(b, \"B\")\nSystem(a, \"New\")\nRel(a, b, \"one\")\nRel(a, b, \"two\")\n"
	elems := c4Rows(t, src, "elements")
	if len(elems) != 2 || elems[0]["label"] != "New" || elems[0]["shape"] != "rounded" {
		t.Errorf("the elements %v", elems)
	}
	if rels := c4Rows(t, src, "rels"); len(rels) != 1 || rels[0]["label"] != "two" {
		t.Errorf("the relationships %v", rels)
	}
	// Mermaid draws the first relationship straight, and the others as
	// curves.
	src = "C4Context\nSystem(a, \"A\")\nSystem(b, \"B\")\nRel(a, b, \"x\")\nRel(b, a, \"y\")\n"
	rels := c4Rows(t, src, "rels")
	slices.SortFunc(rels, func(x, y map[string]any) int { return int(num(x["key"]) - num(y["key"])) })
	if p0, _ := rels[0]["path"].(string); strings.Contains(p0, "Q") {
		t.Errorf("the first path %s is a curve", p0)
	}
	if p1, _ := rels[1]["path"].(string); !strings.Contains(p1, "Q") {
		t.Errorf("the second path %s is not a curve", p1)
	}
	// A value not in quotes runs to the next comma, even past its line, as
	// in Mermaid.
	src = "C4Context\nPerson(a, Some Label, \"A\")\nSystem(b, \"B\", ,)\n"
	if elems := c4Rows(t, src, "elements"); elems[0]["label"] != "Some Label" {
		t.Errorf("the elements %v", elems)
	}
	// accTitle sets the title, as in Mermaid 12.3.
	d, err := mermaid.Read("C4Context\ntitle One\naccTitle: Two\nPerson(a, \"A\")\n")
	if err != nil {
		t.Fatal(err)
	}
	spec, _, _, err := mermaid.Chart(d, true)
	if err != nil || !strings.Contains(string(spec), `"text":"Two"`) {
		t.Errorf("the spec %s and the error %v", spec, err)
	}
}

// TestC4External checks that an external element draws in the neutral
// color muted, at the tint of the other elements, and that each other
// element takes the color of its kind (D32).
func TestC4External(t *testing.T) {
	t.Parallel()
	src := "C4Container\nPerson_Ext(a, \"A\")\nSystem(b, \"B\")\nSystemDb_Ext(c, \"C\")\nContainer_Ext(d, \"D\")\n" +
		"Container(e, \"E\")\nUpdateElementStyle(d, $bgColor=\"red\")\n"
	items, report, err := drawDiagram(t, src, true)
	if err != nil || len(report) != 0 {
		t.Fatalf("the report %v and the error %v", report, err)
	}
	th := theme.New(theme.Light, theme.Zuzu)
	want := map[string]string{"a": th.Muted, "b": th.Category[1], "c": th.Muted, "d": "red", "e": th.Category[2]}
	got := map[string]string{}
	for _, it := range drawnPrefix(items, "elements") {
		id := datumID(it)
		got[id] = it.item.Fill.Color
		if id == "a" || id == "c" {
			if it.item.Stroke.Color != th.Muted || it.item.FillOpacity == nil || *it.item.FillOpacity != 0.14 {
				t.Errorf("%s: the stroke %s and the opacity %v", id, it.item.Stroke.Color, it.item.FillOpacity)
			}
		}
	}
	if !maps.Equal(got, want) {
		t.Errorf("the fills %v, want %v", got, want)
	}
}

func TestC4Errors(t *testing.T) {
	t.Parallel()
	tests := []string{
		"C4Context",
		"C4Context foo\nPerson(a, \"A\")",
		"C4Context\nPerson (a, \"A\")",
		"C4Context\nPerson(a)\n",
		"C4Context\nPerson()\n",
		"C4Context\nPerson(a, \"A\" )\n",
		"C4Context\nPerson(a, \"A)\n",
		"C4Context\nPerson(a, $descr=)\n",
		"C4Context\nPerson(a, $descr=\"\")\n",
		"C4Context\nPerson(a, \"A\"); Person(b, \"B\")\n",
		"C4Context\nPerson(a, \"A\") Person(b, \"B\")\n",
		"C4Context\nfoo\n",
		"C4Context\nPerson(a, \"direction LR\")\n",
		"C4Context\nPerson(a, \"A\")\nRel(a, z, \"uses\")\n",
		"C4Context\nPerson(a, \"A\")\nRel(a, global, \"uses\")\n",
		"C4Context\nSystem_Boundary(b, \"B\") {\n}\n",
		"C4Context\nSystem_Boundary(b, \"B\") { Person(a, \"A\") }\n",
		"C4Context\nSystem_Boundary(b, \"B\")\nPerson(a, \"A\")\n",
		"C4Context\nSystem_Boundary(b, \"B\") {\nPerson(a, \"A\")\n",
		"C4Context\nSystem_Boundary(b, \"B\") {\ntitle T\nPerson(a, \"A\")\n}\n",
		"C4Context\nPerson(a, \"A\")\n}\n",
		"C4Context\nSystem_Boundary(b, \"B\") {\nSystem_Boundary(b, \"C\") {\nPerson(a, \"A\")\n}\n}\n",
		"C4Context\nBoundary(global, \"G\") {\nPerson(a, \"A\")\n}\n",
		"C4Context\nPerson($alias=\"a\", \"A\")\n",
		"C4Context\nC4Container\n",
		"C4Context\ntitle T; x\n",
	}
	for _, src := range tests {
		if _, err := chartOf(src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: the error %v", src, err)
		}
	}
	// An error names the line of the source.
	_, err := chartOf("---\ntitle: T\n---\nC4Context\n  Person(a, \"A\")\n\n  foo\n", false)
	if err == nil || !strings.HasPrefix(err.Error(), "line 7: ") {
		t.Errorf("the error %v, not at line 7", err)
	}
}

func TestC4Issues(t *testing.T) {
	t.Parallel()
	body := "C4Context\nPerson(a, \"A\")\nSystem(b, \"B\")\n"
	tests := []struct {
		src, issue string
	}{
		{"---\nconfig:\n  c4:\n    c4ShapeInRow: 2\n---\n" + body, "c4.c4ShapeInRow"},
		{"---\nconfig:\n  c4:\n    width: wide\n---\n" + body, "c4.width"},
		{"---\nconfig:\n  wrap: 1\n---\n" + body, "wrap"},
		{body + "Rel(a, \"b\")\n", "with no label"},
		{body + "Person(c, \"C\", $foo=\"x\")\n", "the argument $foo"},
		{body + "Person(c, $label=\"C\")\n", "draws as no label"},
		{body + "UpdateElementStyle(b, $shadowing=\"true\")\n", "the argument $shadowing"},
		{body + "Rel(a, b, \"x\")\nUpdateRelStyle(a, b, $offsetX=\"far\")\n", `the offsetX "far"`},
		{body + "Person(c, \"C\", $link=\"javascript:alert(1)\")\n", "the link javascript:alert(1)"},
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
		"---\nconfig:\n  wrap: true\n  c4:\n    wrap: false\n    c4ShapeMargin: 30\n    width: 180\n    useMaxWidth: false\n---\n" + body,
		body + "Rel(a, b, \"x\", \"HTTPS\", $link=\"https://example.com\")\nUpdateRelStyle(a, b, $lineColor=\"red\", $offsetY=\"-10\")\n",
		body + "UpdateElementStyle(a, $bgColor=\"grey\", $fontColor=\"red\", $borderColor=\"red\", $shape=\"queue\")\n",
		body + "Person(c, \"C\", $tags=\"db, queue\")\nSystem(d, \"D\", $sprite=\"person\")\n",
		"%%{init: {'config': {'wrap': false}}}%%\n" + body,
		body + "accDescr {\n  A diagram\n}\nRelIndex(1, a, b, \"x\")\nBiRel(b, a, \"y\")\n",
		// Mermaid 12.3 reads each of these and draws nothing for it, so
		// zuzu accepts it with no issue (D32).
		body + "Rel_U(a, b, \"x\")\nRel_D(a, b, \"x\")\nRel_L(b, a, \"x\")\nRel_R(b, a, \"x\")\n",
		body + "Rel(a, b, \"x\", $descr=\"d\", $tags=\"t\", $sprite=\"s\")\n",
		body + "Person(c, \"C\", $tags=\"v1\")\nSystem(d, \"D\", $sprite=\"user\")\n",
		body + "Person(c, \"C\", \"d\", \"person\", \"db\", \"l\", \"x\")\n",
		body + "UpdateElementStyle(b, $shape=\"folder\")\nUpdateElementStyle(z, $bgColor=\"red\")\nUpdateRelStyle(a, b, $textColor=\"red\")\n",
		body + "Boundary(x, \"X\", \"t\", $tags=\"v1\") {\nSystem(e, \"E\")\n}\n",
		"C4Deployment\nNode_L(n, \"N\", $sprite=\"s\") {\nContainer(c, \"C\")\n}\nNode_R(m, \"M\") {\nContainer(d, \"D\")\n}\n",
	} {
		if report, err := chartOf(src, true); err != nil || len(report) != 0 {
			t.Errorf("%q: the report %v and the error %v", src, report, err)
		}
	}
}
