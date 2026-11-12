package mermaid_test

import (
	"encoding/json/v2"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
)

// cyWant is what a fixture of a Cynefin diagram draws: the labels of the
// items of each domain in order, the transitions as from>to:label, the
// size of the diagram and whether the descriptions of the domains draw.
type cyWant struct {
	items       map[string][]string
	transitions []string
	w, h        float64
	notes       bool
}

func TestCynefinFixtures(t *testing.T) {
	t.Parallel()
	cases := map[string]cyWant{
		"01-domains.mmd": {
			items: map[string][]string{
				"complex":     {"Investigate root cause", "Run chaos experiment"},
				"complicated": {"Analyze performance data", "Expert review needed"},
				"clear":       {"Restart service", "Apply known fix"},
				"chaotic":     {"Page on-call immediately"},
				"confusion":   {"Unknown failure mode"},
			},
			w: 800, h: 600, notes: true,
		},
		"02-transitions.mmd": {
			items: map[string][]string{
				"complex": {"Market research"}, "complicated": {"Competitive analysis"},
				"clear": {"Standard pricing"}, "chaotic": {"Crisis management"},
			},
			transitions: []string{
				"complex>complicated:Pattern identified", "complicated>clear:Best practice codified",
				"clear>chaotic:", "chaotic>complex:Stabilized",
			},
			w: 800, h: 600, notes: true,
		},
		"03-confusion.mmd": {
			items: map[string][]string{"confusion": {"Unknown A", "Unknown B", "Unknown C", "+2 more"}},
			w:     800, h: 600, notes: true,
		},
		"04-settings.mmd": {
			items: map[string][]string{"complex": {"Adaptive work"}, "clear": {"Standard work"}},
			w:     1000, h: 700,
		},
		"05-grammar.mmd": {
			items: map[string][]string{
				"complex": {"Feature discovery"}, "complicated": {"Refactoring", `Code "review"`},
				"clear": {"Hotfix"}, "chaotic": {"Incident"},
			},
			transitions: []string{"confusion>clear:"},
			w:           800, h: 600, notes: true,
		},
	}
	files := fixtures(t, "cynefin")
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
			checkCynefin(t, items, want)
		})
	}
}

// checkCynefin checks the domains, the items, their colors and the
// transitions of a Cynefin diagram that drew.
func checkCynefin(t *testing.T, items []drawnItem, want cyWant) {
	t.Helper()
	// The four domains fill the quarters of the diagram, each in a color
	// of its own.
	quarters := map[string]box{}
	colors := map[string]string{}
	for _, it := range drawnOf(items, "domains") {
		d, _ := it.datum["domain"].(string)
		quarters[d] = boxOf(it)
		colors[d] = it.item.Fill.Color
	}
	if len(quarters) != 4 {
		t.Fatalf("%d domains, expected 4", len(quarters))
	}
	w, h := want.w, want.h
	x0, y0 := quarters["complex"].x0, quarters["complex"].y0
	for d, b := range map[string]box{
		"complex": {0, 0, w / 2, h / 2}, "complicated": {w / 2, 0, w, h / 2},
		"chaotic": {0, h / 2, w / 2, h}, "clear": {w / 2, h / 2, w, h},
	} {
		got := quarters[d]
		if got.x0-x0 != b.x0 || got.y0-y0 != b.y0 || got.x1-x0 != b.x1 || got.y1-y0 != b.y1 {
			t.Errorf("%s: the box %v, expected %v", d, got, b)
		}
	}
	if len(slices.Compact(slices.Sorted(maps.Values(colors)))) != 4 {
		t.Errorf("the domains share colors: %v", colors)
	}
	// Each item draws once, as a badge inside its domain, under the
	// badge before it, with its label in the middle.
	got := map[string][]string{}
	badges := append(drawnOf(items, "items"), drawnOf(items, "itemsMore")...)
	byID := map[string]drawnItem{}
	for _, it := range badges {
		byID[datumID(it)] = it
	}
	last := map[string]float64{}
	for _, it := range drawnOf(items, "itemLabels") {
		d, _ := it.datum["domain"].(string)
		got[d] = append(got[d], it.text)
		b, ok := byID[datumID(it)]
		if !ok {
			t.Errorf("no badge for the item %q", it.text)
			continue
		}
		bb := boxOf(b)
		if q, ok := quarters[d]; ok && !bb.inside(q) {
			t.Errorf("the item %q is out of its domain %s", it.text, d)
		}
		if bb.y0 < last[d] {
			t.Errorf("the item %q is not under the item before it", it.text)
		}
		last[d] = bb.y1
		if it.item.X < bb.x0 || it.item.X > bb.x1 || it.item.Y < bb.y0 || it.item.Y > bb.y1 {
			t.Errorf("the label %q is out of its badge", it.text)
		}
		if d != "confusion" && b.item.Stroke.Color != colors[d] {
			t.Errorf("the item %q is %s, expected the color %s of %s", it.text, b.item.Stroke.Color, colors[d], d)
		}
	}
	for d := range want.items {
		if !slices.Equal(got[d], want.items[d]) {
			t.Errorf("%s: the items %q, expected %q", d, got[d], want.items[d])
		}
	}
	if len(got) != len(want.items) {
		t.Errorf("the items %q, expected %q", got, want.items)
	}
	// Each transition draws once with an arrowhead, and its label when
	// it has one.
	labels := map[string]string{}
	for _, it := range drawnOf(items, "transitionLabels") {
		labels[datumID(it)] = it.text
	}
	var trans []string
	for _, it := range drawnOf(items, "transitions") {
		from, _ := it.datum["from"].(string)
		to, _ := it.datum["to"].(string)
		trans = append(trans, from+">"+to+":"+labels[datumID(it)])
	}
	if !slices.Equal(trans, want.transitions) {
		t.Errorf("the transitions %q, expected %q", trans, want.transitions)
	}
	if n := len(drawnOf(items, "arrows")); n != len(want.transitions) {
		t.Errorf("%d arrowheads, expected %d", n, len(want.transitions))
	}
	notes := len(drawnOf(items, "notes")) > 0
	if notes != want.notes {
		t.Errorf("the descriptions draw: %v, expected %v", notes, want.notes)
	}
	if n := len(drawnOf(items, "names")); n != 5 {
		t.Errorf("%d names of domains, expected 5", n)
	}
	if len(drawnOf(items, "boundaries")) != 2 || len(drawnOf(items, "cliff")) != 1 || len(drawnOf(items, "confusion")) != 1 {
		t.Error("the boundaries, the cliff and the ellipse of confusion do not each draw")
	}
}

// TestCynefinTitle checks that the title of the body wins over the title
// of the front matter, that an empty title draws none, and that the
// description joins the front matter, accTitle and accDescr.
func TestCynefinTitle(t *testing.T) {
	t.Parallel()
	for src, want := range map[string]string{
		"---\ntitle: Front\n---\ncynefin-beta\n  title  Two   words\n": "Two words",
		"---\ntitle: Front\n---\ncynefin-beta\n  title\n":              "",
		"---\ntitle: Front\n---\ncynefin-beta\n":                       "Front",
	} {
		got, _ := specOf(t, src)
		if got != want {
			t.Errorf("%q: the title %q, expected %q", src, got, want)
		}
	}
	src := "---\ndescription: A map.\n---\ncynefin-beta\n  accTitle: The title\n  accDescr {\n    The description\n  }\n"
	if _, desc := specOf(t, src); desc != "A map. The title The description" {
		t.Errorf("the description %q", desc)
	}
}

// specOf compiles a source of a chart in the strict mode, and returns the
// title and the description of its spec.
func specOf(t *testing.T, src string) (string, string) {
	t.Helper()
	d, err := mermaid.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	b, _, _, err := mermaid.Chart(d, true)
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	var spec struct {
		Title struct {
			Text string `json:"text"`
		} `json:"title"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatal(err)
	}
	return spec.Title.Text, spec.Description
}

// TestCynefinErrors checks the sources that Mermaid 12.3.0 does not read,
// as mmdc draws them, and the settings that zuzu reports.
func TestCynefinErrors(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"cynefin-beta\n  \"orphan\"\n",
		"cynefin-beta\n  complex --> clear : unquoted\n",
		"cynefin-beta\n  complex\n  item\n",
		"cynefin-beta\n  complex --> clear \"x\"\n",
		"cynefin-beta\n  complex\n  --> clear\n",
		"cynefin-beta\n  complexity\n",
		"cynefin-beta\n  complex --> clear : \"x\" complex\n",
		"cynefin-beta\n  complex \"open\n",
		"cynefin-beta\n  complex --> : \"x\"\n",
	} {
		if _, err := chartOf(src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: expected an error of syntax, got %v", src, err)
		}
	}
	for src, part := range map[string]string{
		"---\nconfig:\n  cynefin:\n    width: wide\n---\ncynefin-beta\n":                          "cynefin.width",
		"---\nconfig:\n  cynefin:\n    showDomainDescriptions: 1\n---\ncynefin-beta\n":            "cynefin.showDomainDescriptions",
		"---\nconfig:\n  cynefin:\n    zoom: 2\n---\ncynefin-beta\n":                              "cynefin.zoom",
		"---\nconfig:\n  themeVariables:\n    cynefin:\n      cliffWidth: 9\n---\ncynefin-beta\n": "themeVariables.cynefin.cliffWidth",
	} {
		report, err := chartOf(src, false)
		if err != nil || !reportHolds(report, part) {
			t.Errorf("%q: expected %s in the report %v, got the error %v", src, part, report, err)
		}
	}
}

// TestInfoFixtures checks that an info diagram draws the version of
// Mermaid that zuzu reads, as mmdc draws it, and no title.
func TestInfoFixtures(t *testing.T) {
	t.Parallel()
	for _, path := range fixtures(t, "info") {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		items, report, err := drawDiagram(t, string(src), true)
		if err != nil || len(report) != 0 {
			t.Fatalf("%s: the report %v and the error %v", path, report, err)
		}
		var texts []string
		for _, it := range items {
			if it.mark == "text" {
				texts = append(texts, it.text)
			}
		}
		if !slices.Equal(texts, []string{"v12.3.0"}) {
			t.Errorf("%s: the texts %q", path, texts)
		}
		title, desc := specOf(t, string(src))
		if title != "" {
			t.Errorf("%s: the title %q", path, title)
		}
		if strings.Contains(string(src), "accTitle") && desc != "The version The version of Mermaid" {
			t.Errorf("%s: the description %q", path, desc)
		}
	}
	for _, src := range []string{"info foo\n", "info\nshowInfo\nshowInfo\n", "info\n  complex\n"} {
		if _, err := chartOf(src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: expected an error of syntax, got %v", src, err)
		}
	}
}
