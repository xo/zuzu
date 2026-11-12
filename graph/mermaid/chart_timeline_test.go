package mermaid_test

import (
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/vega"
)

// tlWant is what a fixture of a timeline draws: the header of each
// section in order, each period with its events, the color of each
// period, and the direction.
type tlWant struct {
	sections []string
	periods  [][]string
	// colors holds the number of the color of each period. Two periods
	// with the same number share a color, and two with other numbers do
	// not.
	colors   []int
	vertical bool
}

func TestTimelineFixtures(t *testing.T) {
	t.Parallel()
	cases := map[string]tlWant{
		"01-periods.mmd": {
			periods: [][]string{
				{"2002", "LinkedIn"}, {"2004", "Facebook", "Google", "YouTube Music"}, {"2005", "YouTube"}, {"2006", "Twitter"},
			},
			colors: []int{0, 1, 2, 3},
		},
		"02-sections.mmd": {
			sections: []string{"17th-20th century", "21st century"},
			periods: [][]string{
				{"Industry 1.0", "Machinery, Water power, Steam power"},
				{"Industry 2.0", "Electricity, Internal combustion engine, Mass production"},
				{"Industry 3.0", "Electronics, Computers, Automation"},
				{"Industry 4.0", "Internet, Robotics, Internet of Things"},
				{"Industry 5.0", "Artificial intelligence, Big data, 3D printing"},
			},
			colors: []int{0, 0, 0, 1, 1},
		},
		"03-vertical.mmd": {
			sections: []string{"2023 Q1 Release Personal Tier", "2023 Q2 Release XYZ Tier"},
			periods: [][]string{
				{"Bullet 1", "sub-point 1a", "sub-point 1b", "sub-point 1c"}, {"Bullet 2", "sub-point 2a"},
				{"Bullet 3", "sub-point 3a", "sub-point 3b"}, {"Bullet 4"},
			},
			colors:   []int{0, 0, 1, 1},
			vertical: true,
		},
		"04-one-color.mmd": {
			periods:  [][]string{{"2002", "LinkedIn"}, {"2004", "Facebook", "Google"}, {"2005", "YouTube"}},
			colors:   []int{0, 0, 0},
			vertical: true,
		},
		"05-grammar.mmd": {
			sections: []string{"Early", "Late"},
			periods: [][]string{
				{"1999", "the first event", "an event of 1999"}, {"C"},
				{"2020", "see https://mermaid.js.org", "a :", "b"}, {"2021", `"quoted" and #1`}, {"2022", "on the next line"},
			},
			colors: []int{0, 1, 1, 1, 1},
		},
		"06-groups.mmd": {
			sections: []string{"A", "Empty", "A"},
			periods:  [][]string{{"first", "before the first section"}, {"2020", "a"}, {"2021", "b"}},
			colors:   []int{0, 1, 3},
		},
	}
	files := fixtures(t, "timeline")
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
			checkTimeline(t, items, want)
		})
	}
}

// labelsByID returns the text of each label of a mark by the id of its
// box.
func labelsByID(t *testing.T, items []drawnItem, mark string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, it := range drawnOf(items, mark) {
		id := datumID(it)
		if _, ok := out[id]; ok {
			t.Errorf("the label of %s draws twice", id)
		}
		out[id] = it.text
	}
	return out
}

// boxesByID returns the box of each item of a mark of rects by its id.
func boxesByID(t *testing.T, items []drawnItem, mark string) (map[string]box, map[string]string) {
	t.Helper()
	boxes, colors := map[string]box{}, map[string]string{}
	for _, it := range drawnOf(items, mark) {
		id := datumID(it)
		if _, ok := boxes[id]; ok {
			t.Errorf("the box %s draws twice", id)
		}
		boxes[id] = boxOf(it)
		colors[id] = it.item.Stroke.Color
	}
	return boxes, colors
}

// checkTimeline checks the sections, the periods, the events and their
// colors of a timeline that drew.
func checkTimeline(t *testing.T, items []drawnItem, want tlWant) {
	t.Helper()
	sections, _ := boxesByID(t, items, "sections")
	sectionLabels := labelsByID(t, items, "sectionLabels")
	var ids []string
	for id := range sections {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b string) int {
		if want.vertical {
			return int(sections[a].y0 - sections[b].y0)
		}
		return int(sections[a].x0 - sections[b].x0)
	})
	var got []string
	for _, id := range ids {
		got = append(got, sectionLabels[id])
	}
	if !slices.Equal(got, want.sections) {
		t.Errorf("expected the sections %q, got %q", want.sections, got)
	}
	periods, colors := boxesByID(t, items, "periods")
	periodLabels := labelsByID(t, items, "periodLabels")
	events, eventColors := boxesByID(t, items, "events")
	eventLabels := labelsByID(t, items, "eventLabels")
	if len(periods) != len(want.periods) {
		t.Fatalf("expected %d periods, got %d", len(want.periods), len(periods))
	}
	nEvents := 0
	byColor := map[int]string{}
	var prev box
	for n, wp := range want.periods {
		id := "p" + strconv.Itoa(n)
		p, ok := periods[id]
		if !ok {
			t.Fatalf("no period %s", id)
		}
		if periodLabels[id] != wp[0] {
			t.Errorf("%s: expected the label %q, got %q", id, wp[0], periodLabels[id])
		}
		// The periods go from left to right, or from top to bottom.
		if n > 0 && (!want.vertical && p.x0 < prev.x1 || want.vertical && p.y0 < prev.y1) {
			t.Errorf("%s is not after the period before it", id)
		}
		prev = p
		c := colors[id]
		if other, ok := byColor[want.colors[n]]; ok && other != c {
			t.Errorf("%s: expected the color %s, got %s", id, other, c)
		}
		for k, other := range byColor {
			if k != want.colors[n] && other == c {
				t.Errorf("%s: the color %s of another group", id, c)
			}
		}
		byColor[want.colors[n]] = c
		last := p
		for k, we := range wp[1:] {
			eid := id + "e" + strconv.Itoa(k)
			e, ok := events[eid]
			if !ok {
				t.Fatalf("no event %s", eid)
			}
			nEvents++
			if eventLabels[eid] != we {
				t.Errorf("%s: expected the label %q, got %q", eid, we, eventLabels[eid])
			}
			if eventColors[eid] != c {
				t.Errorf("%s: expected the color of its period %s, got %s", eid, c, eventColors[eid])
			}
			// An event is under its period, or at its right, and under
			// the event before it.
			switch {
			case !want.vertical && (e.x0 != p.x0 || e.y0 < last.y1):
				t.Errorf("%s is not under %s", eid, id)
			case want.vertical && k == 0 && (e.x0 < p.x1 || e.y0 != p.y0):
				t.Errorf("%s is not at the right of %s", eid, id)
			case want.vertical && k > 0 && e.y0 < last.y1:
				t.Errorf("%s is not under the event before it", eid)
			}
			last = e
		}
	}
	if len(events) != nEvents {
		t.Errorf("expected %d events, got %d", nEvents, len(events))
	}
	// No two boxes overlap.
	backs := drawnOf(items, "backs")
	for i := range backs {
		for j := range i {
			a, b := boxOf(backs[i]), boxOf(backs[j])
			if a.overlaps(b) {
				t.Errorf("two boxes overlap: %v and %v", a, b)
			}
		}
	}
	if n := len(drawnOf(items, "dots")); n != len(want.periods) {
		t.Errorf("expected a dot for each period, got %d", n)
	}
	if n := len(drawnOf(items, "axis")); n != 1 {
		t.Errorf("expected one axis, got %d", n)
	}
}

func TestTimelineColors(t *testing.T) {
	t.Parallel()
	// With no section, the periods take the colors of the scheme in their
	// order, and the eleventh period takes the first color again.
	var src strings.Builder
	src.WriteString("timeline\n")
	for i := range 11 {
		src.WriteString("  " + strconv.Itoa(2000+i) + "\n")
	}
	items, _, err := drawDiagram(t, src.String(), true)
	if err != nil {
		t.Fatal(err)
	}
	var colors []string
	for _, it := range drawnOf(items, "periods") {
		colors = append(colors, it.item.Stroke.Color)
	}
	if len(colors) != 11 || colors[10] != colors[0] || len(slices.Compact(slices.Clone(colors[:10]))) != 10 {
		t.Errorf("expected ten colors, then the first again, got %v", colors)
	}
}

func TestTimelineWrap(t *testing.T) {
	t.Parallel()
	// An event wraps at 150 pixels of Mermaid, scaled by 11/14, and a period
	// of a timeline TD at 200.
	for _, tt := range []struct {
		src   string
		mark  string
		width float64
	}{
		{"timeline\n  2020 : Electricity, Internal combustion engine, Mass production\n", "eventLabels", 150},
		{"timeline TD\n  Electricity, Internal combustion engine, Mass production and more : a\n", "periodLabels", 200},
	} {
		items, _, err := drawDiagram(t, tt.src, true)
		if err != nil {
			t.Fatal(err)
		}
		labels := drawnOf(items, tt.mark)
		if len(labels) != 1 || len(labels[0].item.Text) < 2 {
			t.Fatalf("%s: expected a label of more lines, got %v", tt.mark, labels)
		}
		m := text.NewEstimate()
		for _, line := range labels[0].item.Text {
			f := text.Font{Family: text.Family, Size: 11, Weight: 400}
			if tt.mark == "periodLabels" {
				f.Weight = 600
			}
			if w := m.Width(f, line); w > tt.width*11/14 {
				t.Errorf("%s: the line %q is %.1f wide", tt.mark, line, w)
			}
		}
	}
}

func TestTimelineSpec(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile(filepath.Join("testdata", "timeline", "05-grammar.mmd"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := mermaid.Read(string(src))
	if err != nil {
		t.Fatal(err)
	}
	b, _, report, err := mermaid.Chart(d, true)
	if err != nil || len(report) != 0 {
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
	if spec.Title.Text != "The grammar of a timeline" {
		t.Errorf("expected the title, got %q", spec.Title.Text)
	}
	if spec.Description != "Grammar A timeline that uses each part of the grammar" {
		t.Errorf("expected the description, got %q", spec.Description)
	}
}

func TestTimelineErrors(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"an event before a period":        "timeline\n  : orphan\n",
		"an event after a first section":  "timeline\n  section A : x\n",
		"a colon with no space":           "timeline\n  10:30 : x\n",
		"a colon at the end":              "timeline\n  2020 :\n",
		"a colon in a section":            "timeline\n  section A:B\n  2020 : a\n",
		"an event with no text":           "timeline\n  2020 : : x\n",
		"the keyword on a later line":     "timeline\n  timeline\n",
		"an accDescr block with no end":   "timeline\n  accDescr {\n  2020 : a\n",
		"a colon with no space in events": "timeline\n  2020 : a :b\n  2021 :x\n",
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

func TestTimelineReads(t *testing.T) {
	t.Parallel()
	// Each source draws with no issue, as it does in Mermaid.
	tests := map[string]string{
		"a period after the keyword":   "timeline 2019 : z\n",
		"TB is a period":               "timeline TB\n",
		"a direction in lower case":    "timeline td\n  2020 : a\n",
		"no period":                    "timeline\n  title Empty\n",
		"a section with no period":     "timeline\n  section A\n",
		"a title of one word":          "timeline\n  title\n",
		"accTitle in lower case":       "timeline\n  acctitle: a\n  2020 : b\n",
		"a colon inside an event":      "timeline\n  2020 : a:b\n",
		"a comment after a period":     "timeline\n  2020 # : not an event\n",
		"a hash inside an event":       "timeline\n  2020 : C# and F#\n",
		"a hash inside a section name": "timeline\n  2019\n  section C# : x\n",
	}
	for name, src := range tests {
		report, err := chartOf(src, true)
		if err != nil || len(report) != 0 {
			t.Errorf("%s: the report %v and the error %v", name, report, err)
		}
	}
}

func TestTimelineSettings(t *testing.T) {
	t.Parallel()
	const body = "timeline\n  2020 : a\n"
	tests := map[string]string{
		"disableMulticolor that is not a boolean": "---\nconfig:\n  timeline:\n    disableMulticolor: 1\n---\n" + body,
		"the padding of Mermaid":                  "---\nconfig:\n  timeline:\n    padding: 10\n---\n" + body,
		"the colors of the sections":              "---\nconfig:\n  themeVariables:\n    cScale0: '#ff0000'\n---\n" + body,
		"an unknown theme":                        "---\nconfig:\n  theme: sepia\n---\n" + body,
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
	for _, src := range []string{
		"---\nconfig:\n  timeline:\n    disableMulticolor: false\n    useMaxWidth: false\n  look: neo\n---\n" + body,
		"---\nconfig:\n  theme: forest\n---\n" + body,
		"%%{init: {'timeline': {'disableMulticolor': true}}}%%\n" + body,
	} {
		if report, err := chartOf(src, true); err != nil || len(report) != 0 {
			t.Errorf("%q: the report %v and the error %v", src, report, err)
		}
	}
}
