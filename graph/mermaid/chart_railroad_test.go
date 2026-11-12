package mermaid_test

import (
	"encoding/json/v2"
	"errors"
	"maps"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
)

// rrWant is what a fixture of a railroad diagram draws: the name of each
// rule, and the text of each box, in the order of the source.
type rrWant struct {
	rules  []string
	labels []string
}

func TestRailroadFixtures(t *testing.T) {
	t.Parallel()
	cases := map[string]rrWant{
		"01-ir.mmd": {
			rules: []string{"expression", "term", "factor", "number", "digit"},
			labels: []string{
				"term", "+", "-", "term", "factor", "^", "term", "number", "(", "expression", ")", "digit",
				"0", "1", "? any other digit ?",
			},
		},
		"02-ebnf.mmd": {
			rules: []string{"json", "element", "object", "array", "member", "string", "character", "number"},
			labels: []string{
				"element", "object", "array", "string", "number", "true", "false", "null",
				"{", "member", ",", "member", "}", "[", "element", ",", "element", "]", "string", ":", "element",
				`"`, "character", "-", `"`, `"`, "? any character ?", "digit",
			},
		},
		"03-abnf.mmd": {
			rules: []string{"address", "local-part", "domain", "label", "code", "letter"},
			labels: []string{
				"local-part", "@", "domain", "ALPHA", "DIGIT", ".", "-", "label", ".", "label",
				"ALPHA", "DIGIT", "-", ".", "DIGIT", "%x2D", "%x41-5A", "%x61-7A",
			},
		},
		"04-peg.mmd": {
			rules: []string{"Identifier", "Keyword", "Letter", "Digit"},
			labels: []string{
				"? !Keyword ?", "Letter", "Letter", "Digit", "if", "else", "while", "? !Letter ?",
				"a", "b", "_", `? &"0" ?`, "? . ?", "1", "2",
			},
		},
		"05-settings.mmd": {
			rules:  []string{"sign", "number", "list"},
			labels: []string{"+", "-", "sign", "digit", "[", "number", ",", "number", "]"},
		},
		"06-lexer.mmd": {
			rules:  []string{"terminals", "greeting", "quote"},
			labels: []string{"b", "? c ?", "Hello", "", "World", `"`, "it", "s"},
		},
	}
	files := fixtures(t, "railroad")
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
			checkRailroad(t, items, want)
		})
	}
}

// rrBox is a box of a railroad diagram that drew: its kind, its box, its
// color and its label.
type rrBox struct {
	kind, color, label string
	b                  box
}

// rrBoxes returns the boxes of a railroad diagram that drew, in the order
// of the source.
func rrBoxes(t *testing.T, items []drawnItem) []rrBox {
	t.Helper()
	byID := map[string]*rrBox{}
	order := map[string]int{}
	for _, kind := range []string{"terminals", "nonterminals", "specials"} {
		for _, it := range drawnOf(items, kind) {
			d := it.datum
			id := datumID(it)
			if _, ok := byID[id]; ok {
				t.Errorf("the box %s draws twice", id)
			}
			byID[id] = &rrBox{kind: kind, color: it.item.Stroke.Color, b: box{num(d["x"]), num(d["y"]), num(d["x2"]), num(d["y2"])}}
			order[id] = int(num(d["n"]))
		}
	}
	for _, it := range drawnOf(items, "labels") {
		b, ok := byID[datumID(it)]
		if !ok {
			t.Fatalf("a label of no box: %q", it.text)
		}
		if b.label != "" {
			t.Errorf("the label of %s draws twice", datumID(it))
		}
		b.label = it.text
		if x, y := num(it.datum["x"]), num(it.datum["y"]); x < b.b.x0 || x > b.b.x1 || y < b.b.y0 || y > b.b.y1 {
			t.Errorf("the label %q is outside its box", it.text)
		}
	}
	ids := slices.Collect(maps.Keys(byID))
	slices.SortFunc(ids, func(a, b string) int { return order[a] - order[b] })
	out := make([]rrBox, 0, len(ids))
	for _, id := range ids {
		out = append(out, *byID[id])
	}
	return out
}

// checkRailroad checks the rules, the boxes, their labels and their colors,
// and the markers of a railroad diagram that drew.
func checkRailroad(t *testing.T, items []drawnItem, want rrWant) {
	t.Helper()
	var names []string
	tracks := map[string]float64{}
	for _, it := range drawnOf(items, "names") {
		names = append(names, strings.TrimSuffix(it.text, " ="))
		tracks[datumID(it)] = num(it.datum["y"])
	}
	if !slices.Equal(names, want.rules) {
		t.Errorf("expected the rules %q, got %q", want.rules, names)
	}
	// Each rule has a start marker and an end marker on its track.
	markers := map[string]int{}
	for _, it := range drawnOf(items, "markers") {
		id := datumID(it)
		markers[id]++
		if y, ok := tracks[id]; !ok || math.Abs(num(it.datum["y"])-y) > 0.01 {
			t.Errorf("a marker of %s is off the track of its rule", id)
		}
	}
	for id := range tracks {
		if markers[id] != 2 {
			t.Errorf("the rule %s has %d markers", id, markers[id])
		}
	}
	boxes := rrBoxes(t, items)
	labels := make([]string, len(boxes))
	colors := map[string]string{}
	for i, b := range boxes {
		labels[i] = b.label
		if c, ok := colors[b.kind]; ok && c != b.color {
			t.Errorf("the %s have the colors %s and %s", b.kind, c, b.color)
		}
		colors[b.kind] = b.color
		for _, o := range boxes[:i] {
			if b.b.overlaps(o.b) {
				t.Errorf("the box %q overlaps the box %q", b.label, o.label)
			}
		}
	}
	if !slices.Equal(labels, want.labels) {
		t.Errorf("expected the boxes %q, got %q", want.labels, labels)
	}
	seen := map[string]string{}
	for kind, c := range colors {
		if other, ok := seen[c]; ok {
			t.Errorf("the %s and the %s have one color", kind, other)
		}
		seen[c] = kind
	}
}

// rrPointRE matches a command of a path and its numbers.
var rrPointRE = regexp.MustCompile(`([MLA])([^MLA]*)`)

// rrTrackPoints returns the points of the tracks of a railroad diagram that
// drew: the end of each move, line and arc.
func rrTrackPoints(items []drawnItem) [][2]float64 {
	var out [][2]float64
	for _, it := range drawnOf(items, "tracks") {
		path, _ := it.datum["path"].(string)
		dx, dy := num(it.datum["x"]), num(it.datum["y"])
		for _, m := range rrPointRE.FindAllStringSubmatch(path, -1) {
			f := strings.FieldsFunc(m[2], func(r rune) bool { return r == ',' || r == ' ' })
			x, _ := strconv.ParseFloat(f[len(f)-2], 64)
			y, _ := strconv.ParseFloat(f[len(f)-1], 64)
			out = append(out, [2]float64{x + dx, y + dy})
		}
	}
	return out
}

// TestRailroadLayout checks the tracks around the parts of a rule: a
// choice stacks its alternatives on one center, an optional has a bypass
// over its box, and a repeat a loop under it, with a bypass when it can
// be empty.
func TestRailroadLayout(t *testing.T) {
	t.Parallel()
	draw := func(def string) ([]rrBox, [][2]float64, float64) {
		t.Helper()
		items, report, err := drawDiagram(t, "railroad-beta\na = "+def+" ;\n", true)
		if err != nil || len(report) != 0 {
			t.Fatalf("%s: the report %v and the error %v", def, report, err)
		}
		return rrBoxes(t, items), rrTrackPoints(items), num(drawnOf(items, "names")[0].datum["y"])
	}
	bounds := func(pts [][2]float64) (float64, float64) {
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, p := range pts {
			lo, hi = math.Min(lo, p[1]), math.Max(hi, p[1])
		}
		return lo, hi
	}
	mid := func(b box) (float64, float64) { return (b.x0 + b.x1) / 2, (b.y0 + b.y1) / 2 }

	boxes, _, track := draw(`choice(terminal("a"), terminal("bbbbbb"), terminal("c"))`)
	if len(boxes) != 3 {
		t.Fatalf("expected 3 boxes, got %d", len(boxes))
	}
	x0, y0 := mid(boxes[0].b)
	x1, y1 := mid(boxes[1].b)
	x2, y2 := mid(boxes[2].b)
	if math.Abs(y1-track) > 0.01 || y0 >= y1 || y2 <= y1 {
		t.Errorf("a choice stacks its alternatives at %v, %v and %v around the track %v", y0, y1, y2, track)
	}
	if math.Abs(x0-x1) > 0.01 || math.Abs(x2-x1) > 0.01 {
		t.Errorf("a choice centers its alternatives at %v, %v and %v", x0, x1, x2)
	}

	for _, tc := range []struct {
		def         string
		over, under bool
	}{
		{`optional(terminal("x"))`, true, false},
		{`oneOrMore(terminal("x"))`, false, true},
		{`zeroOrMore(terminal("x"))`, true, true},
	} {
		boxes, pts, track := draw(tc.def)
		b := boxes[0].b
		if _, y := mid(b); math.Abs(y-track) > 0.01 {
			t.Errorf("%s: the box is at %v, off the track %v", tc.def, y, track)
		}
		lo, hi := bounds(pts)
		if over := lo < b.y0-1; over != tc.over {
			t.Errorf("%s: a track over the box is %v, expected %v", tc.def, over, tc.over)
		}
		if under := hi > b.y1+1; under != tc.under {
			t.Errorf("%s: a track under the box is %v, expected %v", tc.def, under, tc.under)
		}
	}
}

// TestRailroadSyntax compares the parse of zuzu with mmdc: each source of
// ok draws in Mermaid 12.3.0, and each other source is an error there.
func TestRailroadSyntax(t *testing.T) {
	t.Parallel()
	ok := []string{
		"railroad-beta\n",
		"railroad-beta\na = choice(terminal(\"a\")) ;\n",
		"railroad-beta\na = terminal('x') ;\n",
		"railroad-beta\ntitle A\naccTitle: t\naccDescr {\n d\n e }\na = terminal(\"x\") ;\n",
		"railroad-beta\na = terminal(\"x\") ; /* c */ b = nonterminal(\"y\") ;\n",
		"railroad-beta\nterminals = terminal(\"x\") ;\n",
		"railroad-beta\na-b = terminal(\"x\") ;\n",
		"railroad-beta\na = terminal(\"x\\\"y\") ;\n",
		// title matches the start of titled, so the rule is d.
		"railroad-beta\ntitled = terminal(\"x\") ;\n",
		"railroad-ebnf-beta\na ::= b , c | d ;\n",
		"railroad-ebnf-beta\na = b - c ;\n",
		"railroad-ebnf-beta\na = (* comment *) b ;\n",
		"railroad-ebnf-beta\na = ? letters ? ;\n",
		"railroad-ebnf-beta\na = [ b ] { c } ( d ) ;\n",
		"railroad-ebnf-beta\na = b*+? ;\n",
		"railroad-abnf-beta\na = 1*DIGIT / \"x\" ;\n",
		"railroad-abnf-beta\na = 3DIGIT ;\n",
		"railroad-abnf-beta\na = %x41-5A ;\n",
		"railroad-abnf-beta\na = *1( b ) [ c ] ;\n",
		"railroad-peg-beta\nA <- !B . &\"x\" ;\n",
		"railroad-peg-beta\nA <- B ; # comment\n",
		"railroad-peg-beta\nA <- (B C)? D* E+ ;\n",
		"railroad-peg-beta\nA <- !(B C) ;\n",
		"railroad-peg-beta title X\nA <- B ;\n",
	}
	bad := []string{
		"railroad-beta\na = terminal(\"x\")\n",
		"railroad-beta\na = sequence() ;\n",
		"railroad-beta\na = optional(terminal(\"a\"), terminal(\"b\")) ;\n",
		"railroad-beta\na = terminal(x) ;\n",
		"railroad-beta\na = terminal(\"x\") ;\ntitle B\n",
		"railroad-beta\nterminal = terminal(\"x\") ;\n",
		"railroad-beta\n1a = terminal(\"x\") ;\n",
		"railroad-beta\na = sequence(terminal(\"x\"),) ;\n",
		"railroad-ebnf-beta\na = b c\n",
		"railroad-ebnf-beta\na = ;\n",
		"railroad-ebnf-beta\na = b - ;\n",
		"railroad-ebnf-beta\na = b ;;\n",
		"railroad-ebnf-beta\na = ?   ? ;\n",
		"railroad-ebnf-beta\na = b,, c ;\n",
		// A semicolon with text after it on its line is a comment of ABNF,
		// so the rule has no end.
		"railroad-abnf-beta\na = b ; c\n",
		"railroad-abnf-beta\na =/ b ;\n",
		"railroad-abnf-beta\na = 'x' ;\n",
		"railroad-abnf-beta\na_b = c ;\n",
		"railroad-abnf-beta\na = 2*3 ;\n",
		"railroad-peg-beta\nA <- [a-z] ;\n",
		"railroad-peg-beta\nA = B ;\n",
		"railroad-peg-beta\nA <- !!B ;\n",
		"railroad-peg-beta\nA <- B?? ;\n",
		"railroad-peg-beta\naccTitle T\nA <- B ;\n",
	}
	for _, src := range ok {
		if _, err := chartOf(src, true); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
	for _, src := range bad {
		if _, err := chartOf(src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: expected an error of the syntax, got %v", src, err)
		}
	}
}

// TestRailroadSpec checks the title and the description of the spec of a
// railroad diagram.
func TestRailroadSpec(t *testing.T) {
	t.Parallel()
	spec := func(src string) map[string]any {
		t.Helper()
		d, err := mermaid.Read(src)
		if err != nil {
			t.Fatal(err)
		}
		b, _, _, err := mermaid.Chart(d, true)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	title := func(m map[string]any) string {
		tm, _ := m["title"].(map[string]any)
		s, _ := tm["text"].(string)
		return s
	}
	tests := []struct {
		src, title, desc string
	}{
		{"railroad-beta\ntitle \"A \\\"B\\\"\"\na = terminal(\"x\") ;\n", `A "B"`, ""},
		{"railroad-abnf-beta\ntitle \"A \\\"B\"\na = \"x\" ;\n", `A \"B`, ""},
		{"---\ntitle: Front\n---\nrailroad-peg-beta\nA <- B ;\n", "Front", ""},
		{"---\ntitle: Front\n---\nrailroad-peg-beta\ntitle Own\nA <- B ;\n", "Own", ""},
		{"railroad-beta\ntitle A\n", "A", ""},
		{
			"---\ndescription: First\n---\nrailroad-ebnf-beta\naccDescr: Second\naccTitle: Third\na = b ;\n",
			"", "First Second Third",
		},
		{"railroad-ebnf-beta\naccTitle: Old\naccTitle: New\naccDescr {\n  one\n  two\n}\na = b ;\n", "", "New one\ntwo"},
	}
	for _, tc := range tests {
		m := spec(tc.src)
		if got := title(m); got != tc.title {
			t.Errorf("%q: expected the title %q, got %q", tc.src, tc.title, got)
		}
		if got, _ := m["description"].(string); got != tc.desc {
			t.Errorf("%q: expected the description %q, got %q", tc.src, tc.desc, got)
		}
	}
}

// TestRailroadSettings checks the settings of the section railroad.
func TestRailroadSettings(t *testing.T) {
	t.Parallel()
	const body = "railroad-beta\na = terminal(\"x\") ;\n"
	front := func(cfg string) string { return "---\nconfig:\n  railroad:\n" + cfg + "---\n" + body }
	for _, src := range []string{
		front("    compactMode: true\n    showMarkers: false\n    padding: 4\n    arcRadius: '12'\n"),
		front("    fontFamily: Courier\n    useMaxWidth: false\n"),
		"%%{init: {'config': {'padding': 4}}}%%\n" + body,
	} {
		if report, err := chartOf(src, true); err != nil || len(report) != 0 {
			t.Errorf("%q: the report %v and the error %v", src, report, err)
		}
	}
	for _, src := range []string{
		front("    padding: -1\n"),
		front("    verticalSeparation: wide\n"),
		front("    terminalFill: '#ff0'\n"),
		front("    fontSize: 20\n"),
		front("    fontFamily: Courier-Bold\n"),
		// Mermaid reads the key config of an EBNF diagram as
		// railroadEbnf, which it does not read.
		"%%{init: {'config': {'padding': 4}}}%%\nrailroad-ebnf-beta\na = b ;\n",
	} {
		report, err := chartOf(src, false)
		if err != nil || len(report) != 1 {
			t.Errorf("%q: expected one issue, got the report %v and the error %v", src, report, err)
		}
		if _, err := chartOf(src, true); err == nil {
			t.Errorf("%q: no error in the strict mode", src)
		}
	}
	// The padding sets the size of a box.
	height := func(src string) float64 {
		items, _, err := drawDiagram(t, src, true)
		if err != nil {
			t.Fatal(err)
		}
		b := rrBoxes(t, items)[0].b
		return b.y1 - b.y0
	}
	if small, big := height(front("    padding: 2\n")), height(front("    padding: 20\n")); big-small < 24 {
		t.Errorf("a padding of 20 draws a box of %v and a padding of 2 a box of %v", big, small)
	}
}
