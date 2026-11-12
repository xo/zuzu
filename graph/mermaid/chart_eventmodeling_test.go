package mermaid_test

import (
	"encoding/json/v2"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/vega"
)

// emWantFrame is a frame that a fixture of an event model draws: its id,
// its kind, the name in its box, the label of its swimlane, and the lines
// of its data.
type emWantFrame struct {
	id, kind, name, lane string
	data                 []string
}

// emWant is what a fixture of an event model draws: the swimlanes from
// top to bottom, the frames in the order of the source, and the arrows as
// the ids of their ends.
type emWant struct {
	lanes  []string
	frames []emWantFrame
	rels   []string
}

func TestEventModelFixtures(t *testing.T) {
	t.Parallel()
	f := func(id, kind, name, lane string, data ...string) emWantFrame {
		return emWantFrame{id, kind, name, lane, data}
	}
	const ui, crm, evts = "UI/Automation", "Command/Read Model", "Events"
	cases := map[string]emWant{
		"01-state-change.mmd": {
			lanes: []string{ui, crm, evts},
			frames: []emWantFrame{
				f("01", "ui", "CartScreen", ui), f("02", "command", "AddItem", crm), f("03", "event", "ItemAdded", evts),
				f("04", "command", "RemoveItem", crm), f("05", "event", "ItemRemoved", evts),
			},
			rels: []string{"01-02", "02-03", "03-04", "04-05"},
		},
		"02-namespaces.mmd": {
			lanes: []string{ui, crm, "C/RM: Inventory", evts, "Stream: External", "Stream: Cart"},
			frames: []emWantFrame{
				f("01", "ui", "CartUI", ui), f("02", "command", "AddItem", crm), f("03", "event", "ItemAdded", evts),
				f("04", "event", "InventoryChanged", "Stream: External"), f("05", "processor", "InventoryProcessor", ui),
				f("06", "command", "ChangeInventory", crm), f("07", "event", "InventoryChanged", "Stream: Cart"),
				f("08", "readmodel", "Stock", "C/RM: Inventory"), f("09", "ui", "a.b.c", ui),
			},
			rels: []string{"01-02", "02-03", "04-05", "05-06", "06-07", "07-08", "08-09"},
		},
		"03-data.mmd": {
			lanes: []string{ui, crm, evts},
			frames: []emWantFrame{
				f("01", "ui", "CartUI", ui),
				f("02", "command", "AddItem", crm, "description: 'john'", "image: 'avatar_john'", "price: 20.4"),
				f("03", "event", "ItemAdded", evts, "description: string"),
				f("04", "command", "AddItem", crm), f("05", "event", "ItemAdded", evts),
			},
			rels: []string{"01-02", "02-03", "03-04", "04-05"},
		},
		"04-sources.mmd": {
			lanes: []string{ui, crm, evts},
			frames: []emWantFrame{
				f("01", "readmodel", "CartUI", crm), f("02", "event", "CartCreated", evts), f("03", "event", "ItemAdded", evts),
				f("04", "readmodel", "CartItems", crm), f("05", "processor", "Notifier", ui), f("06", "command", "Notify", crm),
				f("07", "ui", "Screen", ui), f("08", "ui", "OtherScreen", ui),
			},
			rels: []string{"01-02", "01-03", "02-04", "03-04", "04-05", "05-06", "06-06", "06-07", "06-08"},
		},
		"05-grammar.mmd": {
			lanes: []string{ui, crm, evts},
			frames: []emWantFrame{
				f("01", "ui", "CartUI", ui), f("02", "command", "AddItem", crm), f("03", "event", "ItemAdded", evts),
				f("004", "event", "ItemAddedAgain", evts),
			},
			rels: []string{"01-02", "02-03", "02-004"},
		},
		"06-wrap.mmd": {
			lanes: []string{ui, crm},
			frames: []emWantFrame{
				f("01", "ui", "AnExtremelyLongNameOfAScreenThatIsWiderThanTheWidthOfABox", ui),
				f("02", "command", "AddItem", crm, "a long line of inline data that runs on and on and on and on and on past the width of a box"),
			},
			rels: []string{"01-02"},
		},
	}
	files := fixtures(t, "eventmodeling")
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
			checkEventModel(t, items, want)
		})
	}
}

// checkEventModel checks the swimlanes, the boxes, their texts and colors,
// and the arrows of an event model that drew.
func checkEventModel(t *testing.T, items []drawnItem, want emWant) {
	t.Helper()
	// The swimlanes, from top to bottom.
	lanes := drawnOf(items, "lanes")
	slices.SortFunc(lanes, func(a, b drawnItem) int { return int(a.item.Y - b.item.Y) })
	laneBox, laneLabel := map[string]box{}, map[string]string{}
	var labels []string
	for _, it := range lanes {
		laneBox[datumID(it)] = boxOf(it)
	}
	for _, it := range drawnOf(items, "laneLabels") {
		laneLabel[datumID(it)] = it.text
	}
	for _, it := range lanes {
		labels = append(labels, laneLabel[datumID(it)])
	}
	if !slices.Equal(labels, want.lanes) {
		t.Errorf("expected the swimlanes %q, got %q", want.lanes, labels)
	}
	// Each box is in its swimlane, in the color of its kind, after the box
	// before it on the time axis.
	boxes := map[string]drawnItem{}
	for _, mark := range []string{"boxes", "uiBoxes"} {
		for _, it := range drawnOf(items, mark) {
			id := datumID(it)
			if _, ok := boxes[id]; ok {
				t.Errorf("the box %s draws twice", id)
			}
			if kind, _ := it.datum["kind"].(string); (kind == "ui") != (mark == "uiBoxes") {
				t.Errorf("the box %s of the kind %s draws in %s", id, kind, mark)
			}
			boxes[id] = it
		}
	}
	if len(boxes) != len(want.frames) {
		t.Fatalf("expected %d boxes, got %d", len(want.frames), len(boxes))
	}
	names, datas := emTexts(items, "names"), emTexts(items, "data")
	colors := map[string]string{}
	var prev box
	for i, wf := range want.frames {
		it, ok := boxes[wf.id]
		if !ok {
			t.Fatalf("no box %s", wf.id)
		}
		b := boxOf(it)
		if kind, _ := it.datum["kind"].(string); kind != wf.kind {
			t.Errorf("%s: expected the kind %s, got %s", wf.id, wf.kind, kind)
		}
		lane, _ := it.datum["lane"].(string)
		if laneLabel[lane] != wf.lane || !b.inside(laneBox[lane]) {
			t.Errorf("%s: expected it inside the swimlane %q, got %q", wf.id, wf.lane, laneLabel[lane])
		}
		if i > 0 && b.x0 <= prev.x0 {
			t.Errorf("%s does not start after the box before it", wf.id)
		}
		prev = b
		var name strings.Builder
		for _, s := range names[wf.id] {
			name.WriteString(strings.TrimSuffix(s, "-"))
		}
		if name.String() != wf.name {
			t.Errorf("%s: expected the name %q, got %q", wf.id, wf.name, names[wf.id])
		}
		if got := strings.Join(datas[wf.id], " "); got != strings.Join(wf.data, " ") {
			t.Errorf("%s: expected the data %q, got %q", wf.id, wf.data, datas[wf.id])
		}
		c := it.item.Stroke.Color
		if other, ok := colors[wf.kind]; ok && other != c {
			t.Errorf("%s: expected the color %s of its kind, got %s", wf.id, other, c)
		}
		for k, other := range colors {
			if k != wf.kind && other == c && k != "ui" && wf.kind != "ui" {
				t.Errorf("%s: the color %s of the kind %s", wf.id, c, k)
			}
		}
		colors[wf.kind] = c
	}
	// Each arrow ends with its head on the edge of its target.
	var rels []string
	for _, it := range drawnOf(items, "relations") {
		rels = append(rels, datumID(it))
	}
	if !slices.Equal(rels, want.rels) {
		t.Errorf("expected the arrows %q, got %q", want.rels, rels)
	}
	for _, it := range drawnOf(items, "arrows") {
		_, to, _ := strings.Cut(datumID(it), "-")
		b := boxOf(boxes[to])
		x, y := it.item.X, it.item.Y
		if x < b.x0 || x > b.x1 || math.Abs(y-b.y0) > 0.01 && math.Abs(y-b.y1) > 0.01 {
			t.Errorf("the arrow %s ends at %g, %g, off the edge of %v", datumID(it), x, y, b)
		}
	}
	if n := len(drawnOf(items, "arrows")); n != len(want.rels) {
		t.Errorf("expected %d arrowheads, got %d", len(want.rels), n)
	}
}

// emTexts returns the lines of a mark of text by the id of their box, from
// top to bottom.
func emTexts(items []drawnItem, mark string) map[string][]string {
	out := map[string][]string{}
	for _, it := range drawnOf(items, mark) {
		id := datumID(it)
		out[id] = append(out[id], it.text)
	}
	return out
}

func TestEventModelWrap(t *testing.T) {
	t.Parallel()
	// A name and a line of data wrap at 430 pixels of Mermaid, scaled by
	// 11/16, and a name wider than that breaks with a hyphen after the
	// letter that passes the width, as in Mermaid (D32).
	items, _, err := drawDiagram(t, "eventmodeling\n  tf 01 cmd "+strings.Repeat("Word", 30)+" { "+strings.Repeat("data ", 40)+"}\n", true)
	if err != nil {
		t.Fatal(err)
	}
	names, datas := emTexts(items, "names")["01"], emTexts(items, "data")["01"]
	if len(names) < 2 || !strings.HasSuffix(names[0], "-") || len(datas) < 2 {
		t.Fatalf("expected a name and data of more lines, got %q and %q", names, datas)
	}
	m := text.NewEstimate()
	for _, mark := range []string{"names", "data"} {
		f := text.Font{Family: text.Family, Size: 11, Weight: 400}
		if mark == "names" {
			f.Weight = 600
		}
		for _, it := range drawnOf(items, mark) {
			line := it.text
			if cut, ok := strings.CutSuffix(line, "-"); ok {
				line = cut[:len(cut)-1]
			}
			if w := m.Width(f, line); w > 430.0*11/16 {
				t.Errorf("the line %q is %.1f wide", it.text, w)
			}
		}
	}
}

func TestEventModelSpec(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		file, title, desc string
	}{
		{"01-state-change.mmd", "A cart", "Two commands, each with the event that it makes"},
		{"05-grammar.mmd", "The title of the body", "The grammar An event model that uses each part of the grammar"},
	} {
		src, err := os.ReadFile(filepath.Join("testdata", "eventmodeling", tt.file))
		if err != nil {
			t.Fatal(err)
		}
		d, err := mermaid.Read(string(src))
		if err != nil {
			t.Fatal(err)
		}
		b, lite, report, err := mermaid.Chart(d, true)
		if err != nil || len(report) != 0 || lite {
			t.Fatalf("the report %v and the error %v", report, err)
		}
		var spec struct {
			Description string `json:"description"`
			Title       struct {
				Text string `json:"text"`
			} `json:"title"`
		}
		if err := json.Unmarshal(b, &spec); err != nil {
			t.Fatal(err)
		}
		if spec.Title.Text != tt.title || spec.Description != tt.desc {
			t.Errorf("%s: expected the title %q and the description %q, got %q and %q",
				tt.file, tt.title, tt.desc, spec.Title.Text, spec.Description)
		}
	}
}

func TestEventModelErrors(t *testing.T) {
	t.Parallel()
	// Each source is an error in Mermaid 12.3 too.
	tests := map[string]string{
		"a title at the start of a line":    "eventmodeling\n  title Hello\n  tf 01 ui A\n",
		"accTitle at the start of a line":   "eventmodeling\n  accTitle: Hello\n  tf 01 ui A\n",
		"a frame id twice":                  "eventmodeling\n  tf 01 ui A\n  tf 01 cmd B\n",
		"a frame id of four digits":         "eventmodeling\n  tf 1234 ui A\n",
		"a kind that Mermaid does not have": "eventmodeling\n  tf 01 foo A\n",
		"a kind in upper case":              "eventmodeling\n  tf 01 UI A\n",
		"a keyword as a name":               "eventmodeling\n  tf 01 ui data\n",
		"a data block in one line":          "eventmodeling\n  tf 01 ui A\n  data X { a: 1 }\n",
		"a data block in a frame":           "eventmodeling\n  tf 01 cmd A {\n  a: 1\n}\n",
		"a data block before a reference":   "eventmodeling\n  tf 02 cmd B { x: 1 } [[D]]\n",
		"text after the end of a block":     "eventmodeling\n  tf 01 ui A\n  data D {\n  a\n}tf 02 cmd B\n",
		"an indented end of a block":        "eventmodeling\n  tf 01 ui A\n  data D {\n  a\n  }\n",
		"a namespace in a gwt":              "eventmodeling\n  tf 01 ui A\n  gwt 01 given evt Foo.Bar then evt Baz\n",
		"a note in one line":                "eventmodeling\n  tf 01 ui A\n  note 01 { x }\n",
		"a dot at the end of a name":        "eventmodeling\n  tf 01 ui A.\n",
		"a hyphen in a name":                "eventmodeling\n  tf 01 ui A-B\n",
		"an arrow to nothing":               "eventmodeling\n  tf 01 ui A ->>\n",
		"a type of data that is not known":  "eventmodeling\n  tf 01 ui A `foo` { a }\n",
		"a namespace of a data block":       "eventmodeling\n  data D.E {\n  a\n}\n",
		"text after inline data":            "eventmodeling\n  tf 01 cmd B { a } x\n",
		"the keyword on a later line":       "eventmodeling\n  tf 01 ui A\n  eventmodeling\n",
		"a comment with no end":             "eventmodeling\n  tf 01 ui A /* x\n",
	}
	for name, src := range tests {
		d, err := mermaid.Read(src)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, _, _, err := mermaid.Chart(d, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%s: expected an error of the syntax, got %v", name, err)
		}
	}
}

func TestEventModelReads(t *testing.T) {
	t.Parallel()
	// Each source draws with no issue, as it does in Mermaid 12.3.
	tests := map[string]string{
		"no frame":                     "eventmodeling\n",
		"a frame after the keyword":    "eventmodeling tf 01 ui A\n",
		"two frames on a line":         "eventmodeling\n  tf 01 ui A tf 02 cmd B\n",
		"an arrow to a later frame":    "eventmodeling\n  tf 01 ui A ->> 05\n",
		"an arrow to itself":           "eventmodeling\n  tf 01 ui A ->> 01\n",
		"a block that is not there":    "eventmodeling\n  tf 01 ui A [[Missing]]\n",
		"data in quotes":               "eventmodeling\n  tf 01 cmd A 'hello world'\n",
		"two texts in quotes":          "eventmodeling\n  tf 01 cmd A \"a\" \"b\"\n",
		"two texts in braces":          "eventmodeling\n  tf 01 cmd A { a } { b }\n",
		"an end of a block in a block": "eventmodeling\n  tf 01 ui A [[D]]\n  data D {\n  a\n  }\n}\n",
		"a block with spaces after {":  "eventmodeling\n  tf 01 ui A\n  data D {  \n a\n}\n",
		"a statement after a block":    "eventmodeling\n  tf 01 ui A\n  data D {\n a\n} tf 02 cmd B\n",
		"an entity, a note and a gwt":  "eventmodeling\n  entity Foo\n  note 01 {\n hi\n}\n  tf 01 cmd A\n  gwt 01 given evt Foo then evt Foo\n",
		"a description and a title":    "eventmodeling accDescr: Hello\n  tf 01 ui A title My title\n",
		"accDescr in braces":           "eventmodeling accDescr {\n  Hello\n  }\n  tf 01 ui A\n",
		"the settings of Mermaid":      "---\nconfig:\n  eventmodeling:\n    rowHeight: 40\n    useMaxWidth: false\n  theme: forest\n  look: handDrawn\n---\neventmodeling\n  tf 01 ui A\n",
	}
	for name, src := range tests {
		report, err := chartOf(src, true)
		if err != nil || len(report) != 0 {
			t.Errorf("%s: the report %v and the error %v", name, report, err)
		}
	}
}

func TestEventModelSettings(t *testing.T) {
	t.Parallel()
	const body = "eventmodeling\n  tf 01 ui A\n"
	tests := map[string]string{
		"the padding of Mermaid":      "---\nconfig:\n  eventmodeling:\n    padding: 10\n---\n" + body,
		"the colors of the boxes":     "---\nconfig:\n  themeVariables:\n    emUiFill: '#ff0000'\n---\n" + body,
		"a setting that is not known": "---\nconfig:\n  eventmodeling:\n    boxWidth: 10\n---\n" + body,
	}
	for name, src := range tests {
		if _, err := chartOf(src, true); !errors.Is(err, vega.ErrUnsupported) {
			t.Errorf("%s: expected an issue, got %v", name, err)
		}
		report, err := chartOf(src, false)
		if err != nil || len(report) != 1 {
			t.Errorf("%s: expected one line of the report, got %v and %v", name, report, err)
		}
	}
}
