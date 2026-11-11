package mermaid_test

import (
	"context"
	"errors"
	"html"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/theme"
)

// seqItem is an item of the scene of a sequence diagram: the name and the
// type of its mark, its text and its datum.
type seqItem struct {
	name, mark, text string
	datum            map[string]any
}

// seqRender renders a sequence diagram in the strict mode, in a mode, so
// that a part that zuzu does not draw fails the test, and returns the items
// of its scene.
func seqRender(t *testing.T, src string, mode theme.Mode) []seqItem {
	t.Helper()
	c, report, err := zuzu.Parse([]byte(src), zuzu.WithStrict(true))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(report) > 0 {
		t.Fatalf("a report in the strict mode: %v", report)
	}
	s, _, err := c.Render(context.Background(), zuzu.WithLocation(time.UTC), zuzu.WithMode(mode))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var out []seqItem
	var walk func(m *scene.Mark)
	walk = func(m *scene.Mark) {
		for i := range m.Items {
			it := &m.Items[i]
			d, _ := it.Datum.(map[string]any)
			out = append(out, seqItem{name: m.Name, mark: m.Type, text: strings.Join(it.Text, " "), datum: d})
			for j := range it.Items {
				walk(&it.Items[j])
			}
		}
	}
	walk(&s.Root)
	return out
}

// seqOf returns the items of the marks of some names.
func seqOf(items []seqItem, names ...string) []seqItem {
	var out []seqItem
	for _, it := range items {
		if slices.Contains(names, it.name) {
			out = append(out, it)
		}
	}
	return out
}

// seqNum returns a number of a datum.
func seqNum(d map[string]any, key string) float64 {
	f, _ := d[key].(float64)
	return f
}

var (
	// seqMessageLine matches a message of a sequence diagram: its sender,
	// its arrow, its receiver and its text. The test reads the source
	// apart from the compiler.
	seqMessageLine = regexp.MustCompile(`^\s*([^:]+?)\s*(<<-->>|<<->>|-->>|--\|\\|--\|/|--\\\\|--//|/\|--|\\\|--|//--|\\\\--|->>|-->|--x|--\)|-\|\\|-\|/|-\\\\|-//|/\|-|\\\|-|//-|\\\\-|->|-x|-\))\s*([+-]|\(\))?\s*([^:]+?)\s*:(.*)$`)
	// seqNoteLine matches a note and its text.
	seqNoteLine = regexp.MustCompile(`(?i)^\s*note\s+(?:left of|right of|over)\s+[^:]+:(.*)$`)
	// seqStartLine matches the start of a block with a frame.
	seqStartLine = regexp.MustCompile(`(?i)^\s*(loop|alt|opt|par|critical|break)\b`)
	// seqRectLine matches the start of a rect.
	seqRectLine = regexp.MustCompile(`(?i)^\s*rect\b`)
	// seqSectionLine matches a section of a block.
	seqSectionLine = regexp.MustCompile(`(?i)^\s*(else|and|option)\b`)
	// seqEntityCode matches an entity code of Mermaid.
	seqEntityCode = regexp.MustCompile(`#(\w+);`)
	// seqHeld matches an entity code that the test holds apart while it
	// splits a line.
	seqHeld = regexp.MustCompile("\x00(\\w+)\x01")
	// seqBreakTag matches a line break.
	seqBreakTag = regexp.MustCompile(`(?i)<br\s*/?>`)
)

// seqStatements splits a source into its statements, with the entity codes
// decoded after the split, as Mermaid reads them.
func seqStatements(src string) []string {
	var out []string
	for line := range strings.SplitSeq(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "%%") {
			continue
		}
		line = seqEntityCode.ReplaceAllString(line, "\x00$1\x01")
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		for st := range strings.SplitSeq(line, ";") {
			st = seqHeld.ReplaceAllStringFunc(st, func(m string) string {
				name := m[1 : len(m)-1]
				if name[0] >= '0' && name[0] <= '9' {
					return html.UnescapeString("&#" + name + ";")
				}
				return html.UnescapeString("&" + name + ";")
			})
			out = append(out, st)
		}
	}
	return out
}

// seqLabel returns a text of the source as the compiler writes it in a
// datum: the line breaks and the spaces as one space, and no prefix of
// wrap.
func seqLabel(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "nowrap:"), "wrap:")
	return strings.Join(strings.Fields(seqBreakTag.ReplaceAllString(s, " ")), " ")
}

func TestSequenceFixtures(t *testing.T) {
	t.Parallel()
	for name, src := range mmdFiles(t, "sequence") {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			items := seqRender(t, src, theme.Light)
			seqRender(t, src, theme.Dark)
			messages := seqOf(items, "messages", "messagesDotted")
			labels := seqOf(items, "messageLabels")
			var ys []float64
			n, frames, rects, sections := 0, 0, 0, 0
			for _, st := range seqStatements(src) {
				switch {
				case seqNoteLine.MatchString(st):
					text := seqLabel(seqNoteLine.FindStringSubmatch(st)[1])
					if c := len(slices.DeleteFunc(seqOf(items, "noteLabels"), func(it seqItem) bool { return it.datum["text"] != text })); c != 1 {
						t.Errorf("the note %q draws %d times", text, c)
					}
					continue
				case seqStartLine.MatchString(st):
					frames++
				case seqRectLine.MatchString(st):
					rects++
				case seqSectionLine.MatchString(st):
					sections++
				}
				m := seqMessageLine.FindStringSubmatch(st)
				if m == nil {
					continue
				}
				from, to := strings.TrimSuffix(m[1], "()"), m[4]
				text := seqLabel(m[5])
				dotted := strings.Contains(m[2], "--")
				var found []seqItem
				for _, it := range messages {
					if it.datum["from"] == from && it.datum["to"] == to && it.datum["text"] == text {
						found = append(found, it)
					}
				}
				if len(found) != 1 {
					t.Errorf("the message %s %s %s %q draws %d times", from, m[2], to, text, len(found))
					continue
				}
				if (found[0].name == "messagesDotted") != dotted {
					t.Errorf("the message %q draws in %s", text, found[0].name)
				}
				if c := len(slices.DeleteFunc(slices.Clone(labels), func(it seqItem) bool { return it.datum["text"] != text })); c != 1 {
					t.Errorf("the label %q draws %d times", text, c)
				}
				ys = append(ys, seqNum(found[0].datum, "y"))
				n++
			}
			if len(messages) != n || n == 0 {
				t.Errorf("expected %d messages, got %d", n, len(messages))
			}
			if !slices.IsSorted(ys) || len(slices.Compact(slices.Clone(ys))) != len(ys) {
				t.Errorf("the messages do not go down the page in order: %v", ys)
			}
			if c := len(seqOf(items, "frames")); c != frames {
				t.Errorf("expected %d frames, got %d", frames, c)
			}
			if c := len(seqOf(items, "rects", "rectsFilled")); c != rects {
				t.Errorf("expected %d rects, got %d", rects, c)
			}
			if c := len(seqOf(items, "dividers")); c != sections {
				t.Errorf("expected %d sections, got %d", sections, c)
			}
		})
	}
}

// seqFixture renders a fixture of the sequence diagrams.
func seqFixture(t *testing.T, name string) []seqItem {
	t.Helper()
	return seqRender(t, mmdFiles(t, "sequence")[name], theme.Light)
}

// seqFind returns the datum of the only item of some marks that matches.
func seqFind(t *testing.T, items []seqItem, match func(d map[string]any) bool, names ...string) map[string]any {
	t.Helper()
	var found []map[string]any
	for _, it := range seqOf(items, names...) {
		if it.datum != nil && match(it.datum) {
			found = append(found, it.datum)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected one item of %v, got %d", names, len(found))
	}
	return found[0]
}

// seqText matches a datum by its text.
func seqText(text string) func(d map[string]any) bool {
	return func(d map[string]any) bool { return d["text"] == text }
}

func TestSequenceLifecycle(t *testing.T) {
	t.Parallel()
	items := seqFixture(t, "06-lifecycle.mmd")
	hi := seqFind(t, items, seqText("Hi Carl!"), "messages")
	head := seqFind(t, items, func(d map[string]any) bool { return d["key"] == "Carl" && d["kind"] == "head" }, "shapes")
	if seqNum(head, "y") != seqNum(hi, "y") {
		t.Errorf("Carl draws at %v, and the message that creates it at %v", head["y"], hi["y"])
	}
	bye := seqFind(t, items, seqText("We are too many"), "messages")
	foot := seqFind(t, items, func(d map[string]any) bool { return d["key"] == "Carl" && d["kind"] == "foot" }, "shapes")
	if seqNum(foot, "y") != seqNum(bye, "y") {
		t.Errorf("Carl ends at %v, and the message that destroys it at %v", foot["y"], bye["y"])
	}
	life := seqFind(t, items, func(d map[string]any) bool { return d["key"] == "Carl" }, "lifelines")
	if seqNum(life, "y") <= seqNum(hi, "y") || seqNum(life, "y2") >= seqNum(bye, "y") {
		t.Errorf("the lifeline of Carl runs from %v to %v", life["y"], life["y2"])
	}
	d := seqFind(t, items, func(d map[string]any) bool { return d["key"] == "D" && d["kind"] == "head" }, "icons")
	if seqNum(d, "y") > seqNum(seqFind(t, items, seqText("Hi!"), "messages"), "y") {
		t.Errorf("Donald draws below the message that creates it")
	}
	// Alice has two activations, one inside the other, and D has one.
	var alice []map[string]any
	for _, it := range seqOf(items, "activations") {
		if it.datum["key"] == "Alice" {
			alice = append(alice, it.datum)
		}
	}
	if len(alice) != 2 {
		t.Fatalf("expected 2 activations of Alice, got %d", len(alice))
	}
	inner, outer := alice[0], alice[1]
	if seqNum(inner, "y") < seqNum(outer, "y") || seqNum(inner, "y2") > seqNum(outer, "y2") || seqNum(inner, "x") <= seqNum(outer, "x") {
		t.Errorf("the inner activation %v is not inside %v and to its right", inner, outer)
	}
}

func TestSequenceActivation(t *testing.T) {
	t.Parallel()
	items := seqFixture(t, "03-arrows.mmd")
	on := seqFind(t, items, seqText("Activate"), "messages")
	off := seqFind(t, items, seqText("Deactivate"), "messagesDotted")
	a := seqFind(t, items, func(d map[string]any) bool { return d["key"] == "B" }, "activations")
	if seqNum(a, "y") != seqNum(on, "y") || seqNum(a, "y2") != seqNum(off, "y") {
		t.Errorf("the activation runs from %v to %v, and the messages are at %v and %v", a["y"], a["y2"], on["y"], off["y"])
	}
	// The message that activates B ends at the side of the activation.
	end := seqNum(on, "x") + mustPathEnd(t, on["path"])
	if end != seqNum(a, "x") {
		t.Errorf("the message ends at %v, and the activation starts at %v", end, a["x"])
	}
	if c := len(seqOf(items, "dots")); c != 4 {
		t.Errorf("expected 4 dots of central connections, got %d", c)
	}
}

// mustPathEnd returns the length of the path of a message, M0,0H<n>.
func mustPathEnd(t *testing.T, p any) float64 {
	t.Helper()
	s, _ := p.(string)
	rest, ok := strings.CutPrefix(s, "M0,0H")
	if !ok {
		t.Fatalf("the path %q", s)
	}
	f, err := strconv.ParseFloat(rest, 64)
	if err != nil {
		t.Fatalf("the path %q: %v", s, err)
	}
	return f
}

func TestSequenceAutonumber(t *testing.T) {
	t.Parallel()
	items := seqFixture(t, "08-autonumber.mmd")
	var got []string
	for _, it := range seqOf(items, "numbers") {
		got = append(got, it.text)
	}
	want := []string{"1", "2", "10", "15", "1.5", "1.75"}
	if !slices.Equal(got, want) {
		t.Errorf("expected the numbers %v, got %v", want, got)
	}
}

func TestSequenceConfig(t *testing.T) {
	t.Parallel()
	items := seqFixture(t, "09-config.mmd")
	for _, it := range seqOf(items, "actorLabels") {
		if it.text == "Ghost" {
			t.Errorf("hideUnusedParticipants draws Ghost")
		}
		if it.datum["kind"] == "foot" {
			t.Errorf("mirrorActors false draws a foot of %s", it.text)
		}
	}
	if c := len(seqOf(items, "numbers")); c != 3 {
		t.Errorf("showSequenceNumbers draws %d numbers, and 3 messages", c)
	}
	for _, it := range seqOf(items, "messageLabels") {
		if it.datum["align"] != "left" {
			t.Errorf("messageAlign left draws %q at %v", it.text, it.datum["align"])
		}
	}
	long := seqFind(t, items, func(d map[string]any) bool { s, _ := d["text"].(string); return strings.HasPrefix(s, "A long") }, "messageLabels")
	if lines, _ := long["lines"].([]any); len(lines) < 2 {
		t.Errorf("wrap draws the long message on %d lines", len(lines))
	}
	short := seqFind(t, items, seqText("This text stays on one line"), "messageLabels")
	if lines, _ := short["lines"].([]any); len(lines) != 1 {
		t.Errorf("nowrap: draws the message on %d lines", len(lines))
	}
	self := seqFind(t, items, seqText("Think about it"), "messages")
	if p, _ := self["path"].(string); strings.Contains(p, "C") {
		t.Errorf("rightAngles draws a curve: %s", p)
	}
}

func TestSequenceBoxes(t *testing.T) {
	t.Parallel()
	items := seqFixture(t, "07-boxes.mmd")
	boxes := seqOf(items, "boxes", "boxesFilled")
	if len(boxes) != 4 {
		t.Fatalf("expected 4 boxes, got %d", len(boxes))
	}
	slices.SortFunc(boxes, func(a, b seqItem) int { return int(seqNum(a.datum, "x") - seqNum(b.datum, "x")) })
	for i := 1; i < len(boxes); i++ {
		if seqNum(boxes[i].datum, "x") <= seqNum(boxes[i-1].datum, "x2") {
			t.Errorf("the boxes %d and %d overlap", i-1, i)
		}
	}
	members := [][]string{{"A", "B"}, {"C"}, {"D"}, {"E"}}
	for i, keys := range members {
		for _, key := range keys {
			life := seqFind(t, items, func(d map[string]any) bool { return d["key"] == key }, "lifelines")
			if x := seqNum(life, "x"); x <= seqNum(boxes[i].datum, "x") || x >= seqNum(boxes[i].datum, "x2") {
				t.Errorf("%s at %v is outside its box", key, x)
			}
		}
	}
	if c := len(seqOf(items, "boxesFilled")); c != 2 {
		t.Errorf("expected 2 boxes with a color, got %d", c)
	}
}

func TestSequenceErrors(t *testing.T) {
	t.Parallel()
	tests := []string{
		"sequenceDiagram",
		"sequenceDiagram LR\nA->>B: hi",
		"sequenceDiagram\nA->>B",
		"sequenceDiagram\nhello there",
		"sequenceDiagram\nloop x\nA->>B: hi",
		"sequenceDiagram\nend",
		"sequenceDiagram\nelse x",
		"sequenceDiagram\nloop\nelse\nend",
		"sequenceDiagram\nalt a\nand b\nend",
		"sequenceDiagram\ndeactivate A",
		"sequenceDiagram\nA->>B: hi\nB-->>-A: no",
		"sequenceDiagram\ncreate participant B\nA->>C: hi",
		"sequenceDiagram\ncreate participant B",
		"sequenceDiagram\nA->>B: hi\ncreate participant B\nA->>B: again",
		"sequenceDiagram\nA->>B: hi\ndestroy B\nA->>C: no",
		"sequenceDiagram\nA->>B: hi\ndestroy B\nA->>B: bye\nA->>B: again",
		"sequenceDiagram\nbox Blue\nA->>B: hi\nend",
		"sequenceDiagram\nbox Blue\nparticipant A",
		"sequenceDiagram\nbox Blue\nparticipant A\nend\nbox Red\nparticipant A\nend",
		"sequenceDiagram\nNote over A,B,C: three",
		"sequenceDiagram\nNote left of A,B: two",
		"sequenceDiagram\nautonumber x",
		"sequenceDiagram\nparticipant A@{ \"type\": \"queue\"\nA->>B: hi",
		"sequenceDiagram\nparticipant A@{ \"type\": \"queue\" } then\nA->>B: hi",
	}
	for _, src := range tests {
		if _, err := compileMermaid(t, src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: expected a syntax error, got %v", src, err)
		}
	}
}

func TestSequenceIssues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src, part string
	}{
		{"sequenceDiagram\nA->>B: hi\nlink A: Dashboard @ https://example.com", "link"},
		{"sequenceDiagram\nA->>B: hi\nlinks A: {\"Wiki\": \"https://example.com\"}", "links"},
		{"sequenceDiagram\nA->>B: hi\nproperties A: {\"class\": \"x\"}", "properties"},
		{"sequenceDiagram\nA->>B: hi\ndetails A: id", "details"},
		{"sequenceDiagram\naccTitle: Hi\nA->>B: hi", "accTitle"},
		{"sequenceDiagram\naccDescr {\nA talk\n}\nA->>B: hi", "accDescr"},
		{"sequenceDiagram\nparticipant A@{ \"type\": \"robot\" }\nA->>B: hi", "robot"},
		{"sequenceDiagram\nparticipant A@{ \"type\": \"queue\", \"icon\": \"x\" }\nA->>B: hi", "icon"},
		{"sequenceDiagram\nrect nocolor\nA->>B: hi\nend", "nocolor"},
		{"---\nconfig:\n  sequence:\n    actorFontFamily: Arial\n---\nsequenceDiagram\nA->>B: hi", "sequence.actorFontFamily"},
		{"---\nconfig:\n  sequence:\n    messageAlign: middle\n---\nsequenceDiagram\nA->>B: hi", "sequence.messageAlign"},
		{"---\nconfig:\n  theme: dark\n---\nsequenceDiagram\nA->>B: hi", "theme"},
	}
	for _, test := range tests {
		report, err := compileMermaid(t, test.src, false)
		if err != nil {
			t.Errorf("%q: %v", test.src, err)
			continue
		}
		if !reportHolds(report, test.part) {
			t.Errorf("%q: expected an issue of %q, got %v", test.src, test.part, report)
		}
		if _, err := compileMermaid(t, test.src, true); err == nil {
			t.Errorf("%q: expected an error in the strict mode", test.src)
		}
	}
}

func TestSequenceUserExample(t *testing.T) {
	t.Parallel()
	report, err := compileMermaid(t, mmdFiles(t, "sequence")["01-basic.mmd"], false)
	if err != nil || len(report) != 0 {
		t.Errorf("the example of the user: %v, %v", err, report)
	}
}
