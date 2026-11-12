package mermaid_test

import (
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/vega"
)

// kbWantCard is a card that a fixture of a kanban draws: its text, its
// ticket, its assignee, the color of its priority and its link.
type kbWantCard struct {
	label, ticket, assigned, priority, href string
}

// kbWantColumn is a column that a fixture of a kanban draws, with its
// cards.
type kbWantColumn struct {
	label string
	cards []kbWantCard
}

func TestKanbanFixtures(t *testing.T) {
	t.Parallel()
	cases := map[string][]kbWantColumn{
		"01-columns.mmd": {
			{"Todo", []kbWantCard{{label: "Write the grammar"}, {label: "Write the tests"}}},
			{"In progress", []kbWantCard{{label: "Lay out the board"}}},
			{"Done", []kbWantCard{{label: "Read the source"}, {label: "Measure the labels"}, {label: "Draw the cards"}}},
		},
		"02-metadata.mmd": {
			{"Todo", []kbWantCard{
				{"Create parsing tests", "MC-2038", "K.Sveidqvist", "orange", "https://tickets.example.com/browse/MC-2038"},
				{label: "Last item", assigned: "knsv", priority: "lightblue"},
				{label: "Plain card"},
			}},
			{"Done", []kbWantCard{
				{"Update DB function", "MC-2037", "knsv", "red", "https://tickets.example.com/browse/MC-2037"},
				{label: "Medium card"},
				{label: "Low card", priority: "blue"},
			}},
		},
		"03-wrap.mmd": {
			{"Todo", []kbWantCard{{label: "Create renderer so that it works in all cases. We also add some extra text here for testing purposes. And some more just for the extra flare."}}},
			{"Title of diagram is more than 100 chars when user duplicates diagram with 100 char, wrapping", []kbWantCard{{label: "Short"}}},
		},
		"04-grammar.mmd": {
			{"Todo", []kbWantCard{
				{label: "A card with no id"}, {label: "A string"}, {label: "A bold card"}, {label: "A text over two lines"},
				{label: "A label from the data"}, {label: "A card", ticket: "12", assigned: "me"}, {label: "accTitle: a card too"},
			}},
			{"Empty", nil},
			{"Last", []kbWantCard{{label: "A deeper card"}}},
		},
		"05-flat.mmd": {
			{"Todo", nil}, {"Create Documentation", nil}, {"id2", nil}, {"Create Blog about the new diagram", nil},
		},
	}
	files := fixtures(t, "kanban")
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
			checkKanban(t, items, want)
		})
	}
}

// checkKanban checks the columns and the cards of a kanban that drew:
// their texts, their order, their colors, and that each card is inside its
// column and under the card before it.
func checkKanban(t *testing.T, items []drawnItem, want []kbWantColumn) {
	t.Helper()
	columns, colColors := boxesByID(t, items, "columns")
	columnLabels := labelsByID(t, items, "columnLabels")
	cards, cardColors := boxesByID(t, items, "cards")
	// The runs of the text of a card join into its text.
	cardText := map[string]string{}
	for _, it := range drawnOf(items, "cardLabels") {
		id := datumID(it)
		cardText[id] = strings.TrimSpace(cardText[id] + " " + it.text)
	}
	tickets, assignees, priorities, hrefs := map[string]string{}, map[string]string{}, map[string]string{}, map[string]string{}
	for _, it := range drawnOf(items, "tickets") {
		tickets[datumID(it)], hrefs[datumID(it)] = it.text, it.item.Href
	}
	for _, it := range drawnOf(items, "assignees") {
		assignees[datumID(it)] = it.text
	}
	for _, it := range drawnOf(items, "priorities") {
		priorities[datumID(it)] = it.item.Stroke.Color
	}
	if len(columns) != len(want) {
		t.Fatalf("expected %d columns, got %d", len(want), len(columns))
	}
	nCards := 0
	var prev box
	for i, wc := range want {
		id := "c" + strconv.Itoa(i)
		col, ok := columns[id]
		if !ok {
			t.Fatalf("no column %s", id)
		}
		if got := strings.Join(strings.Fields(columnLabels[id]), " "); got != wc.label {
			t.Errorf("%s: expected the label %q, got %q", id, wc.label, got)
		}
		if i > 0 && col.x0 <= prev.x1 {
			t.Errorf("%s is not at the right of the column before it", id)
		}
		if i > 0 && colColors[id] == colColors["c"+strconv.Itoa(i-1)] {
			t.Errorf("%s has the color of the column before it", id)
		}
		prev = col
		last := box{y1: col.y0}
		for j, w := range wc.cards {
			cid := id + "i" + strconv.Itoa(j)
			card, ok := cards[cid]
			if !ok {
				t.Fatalf("no card %s", cid)
			}
			nCards++
			if cardText[cid] != w.label {
				t.Errorf("%s: expected the text %q, got %q", cid, w.label, cardText[cid])
			}
			if tickets[cid] != w.ticket || assignees[cid] != w.assigned || hrefs[cid] != w.href {
				t.Errorf("%s: expected the ticket %q, the assignee %q and the link %q, got %q, %q and %q",
					cid, w.ticket, w.assigned, w.href, tickets[cid], assignees[cid], hrefs[cid])
			}
			if priorities[cid] != w.priority {
				t.Errorf("%s: expected the priority %q, got %q", cid, w.priority, priorities[cid])
			}
			if cardColors[cid] != colColors[id] {
				t.Errorf("%s: expected the color of its column %s, got %s", cid, colColors[id], cardColors[cid])
			}
			if card.x0 < col.x0 || card.x1 > col.x1 || card.y1 > col.y1 || card.y0 < last.y1 {
				t.Errorf("%s is not inside its column under the card before it", cid)
			}
			last = card
		}
	}
	if len(cards) != nCards {
		t.Errorf("expected %d cards, got %d", nCards, len(cards))
	}
	if len(priorities) != countPriorities(want) {
		t.Errorf("expected %d lines of priority, got %d", countPriorities(want), len(priorities))
	}
}

// countPriorities returns the number of cards of a kanban with a line of
// priority.
func countPriorities(want []kbWantColumn) int {
	n := 0
	for _, c := range want {
		for _, card := range c.cards {
			if card.priority != "" {
				n++
			}
		}
	}
	return n
}

func TestKanbanWrap(t *testing.T) {
	t.Parallel()
	// The text of a card wraps at 175 pixels of Mermaid, and the name of a
	// column at 200, scaled by 11/16, because Mermaid draws the labels of
	// a kanban at 16 pixels (D32).
	src := "kanban\n  a[Title of diagram is more than 100 chars when user duplicates diagram with 100 char]\n" +
		"    b[Create renderer so that it works in all cases. We also add some extra text here.]\n"
	items, _, err := drawDiagram(t, src, true)
	if err != nil {
		t.Fatal(err)
	}
	m := text.NewEstimate()
	heads := drawnOf(items, "columnLabels")
	if len(heads) != 1 || len(heads[0].item.Text) < 2 {
		t.Fatalf("expected a name of more lines, got %v", heads)
	}
	for _, line := range heads[0].item.Text {
		if w := m.Width(text.Font{Family: text.Family, Size: 11, Weight: 600}, line); w > 200*11.0/16 {
			t.Errorf("the line %q of the name is %.1f wide", line, w)
		}
	}
	lines := map[float64]float64{}
	for _, it := range drawnOf(items, "cardLabels") {
		lines[it.item.Y] = max(lines[it.item.Y], it.item.X+m.Width(text.Font{Family: text.Family, Size: 11, Weight: 400}, it.text))
	}
	if len(lines) < 2 {
		t.Fatalf("expected a text of more lines, got %d", len(lines))
	}
	card := drawnOf(items, "cards")[0]
	for _, x := range lines {
		if w := x - card.item.X - 7; w > 175*11.0/16+0.01 {
			t.Errorf("a line of the card is %.1f wide", w)
		}
	}
}

func TestKanbanSpec(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile(filepath.Join("testdata", "kanban", "05-flat.mmd"))
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
	}
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Description != "Each node has the same indent, so each is a column." {
		t.Errorf("expected the description, got %q", spec.Description)
	}
}

func TestKanbanErrors(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"two nodes on a line":                  "kanban\n  a[A]\n    b[B] c[C]\n",
		"a card after a node with less indent": "kanban\n  a[A]\n b[B]\n  c[C]\n",
		"a shape with a capital letter":        "kanban\n  a[A]\n    b[B]@{ shape: Rect }\n",
		"a shape with an underscore":           "kanban\n  a[A]\n    b[B]@{ shape: kanban_item }\n",
		"a text after a node":                  "kanban\n  a[A]\n    c[C ]x\n",
		"the keyword on a later line":          "kanban\n  Kanban board\n",
		"a node with no end":                   "kanban\n  a[A\n",
		"a node with no text":                  "kanban\n  a[]\n",
		"a space after a string":               "kanban\n  a[\"A\" ]\n",
		"an icon before the first node":        "kanban\n  ::icon(fa fa-book)\n",
		"a class before the first node":        "kanban\n  :::urgent\n",
		"data with no end":                     "kanban\n  a[A]@{ label: x\n",
		"a caret in the data":                  "kanban\n  a[A]@{ label: x^y }\n",
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

func TestKanbanReads(t *testing.T) {
	t.Parallel()
	// Each source draws with no issue, as it does in Mermaid.
	tests := map[string]string{
		"no node":                            "kanban\n",
		"a node after the word":              "kanban Todo\n    b[B]\n",
		"a node with less indent at the end": "kanban\n  a[A]\n b[B]\n",
		"a priority of no color":             "kanban\n  a[A]\n    b[B]@{ priority: Urgent, foo: bar }\n",
		"a title is a column":                "kanban\n  title My board\n  a[A]\n",
		"other shapes":                       "kanban\n  a(A)\n    b((B))\n    c{{C}}\n    d))D((\n    e)E(\n",
		"a space after a node":               "kanban\n  a[A]\n    b[B] \n",
		"padding and a width of a node":      "---\nconfig:\n  kanban:\n    padding: 40\n  mindmap:\n    maxNodeWidth: 50\n---\nkanban\n  a[A]\n",
		"a section width":                    "---\nconfig:\n  kanban:\n    sectionWidth: 300\n---\nkanban\n  a[A]\n",
	}
	for name, src := range tests {
		report, err := chartOf(src, true)
		if err != nil || len(report) != 0 {
			t.Errorf("%s: the report %v and the error %v", name, report, err)
		}
	}
}

func TestKanbanSettings(t *testing.T) {
	t.Parallel()
	const body = "kanban\n  a[A]\n    b[B]@{ ticket: X-1 }\n"
	tests := map[string]string{
		"a width that is not a number": "---\nconfig:\n  kanban:\n    sectionWidth: wide\n---\n" + body,
		"the padding of a mindmap":     "---\nconfig:\n  mindmap:\n    padding: 40\n---\n" + body,
		"a link of a script":           "---\nconfig:\n  kanban:\n    ticketBaseUrl: 'javascript:alert(#TICKET#)'\n---\n" + body,
		"a base that is not a text":    "---\nconfig:\n  kanban:\n    ticketBaseUrl: 3\n---\n" + body,
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
