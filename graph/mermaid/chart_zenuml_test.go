package mermaid_test

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// zenWant is what a fixture of ZenUML draws: the labels of its
// participants from left to right, its messages down the page, and the
// words of the tabs of its fragments.
type zenWant struct {
	heads []string
	// messages holds each message as its number, its sender, its arrow,
	// -> for a solid line or --> for a dashed one, its receiver and its
	// text.
	messages []string
	frames   []string
}

func TestZenUMLFixtures(t *testing.T) {
	t.Parallel()
	cases := map[string]zenWant{
		"01-messages.mmd": {
			heads: []string{"Client", "OrderService", "Stock", "Payment", "Mail"},
			messages: []string{
				"1 Client->OrderService create(order)",
				"1.1 OrderService->Stock reserve(items)",
				"1.1.1 Stock-->OrderService inventory",
				"1.2 OrderService->Payment charge(card)",
				"1.2.1 Payment->Payment audit()",
				"1.2.2 Payment-->OrderService receipt",
				"1.3 OrderService->Mail send confirmation",
				"1.4 OrderService-->Client created",
				"2 Mail->Client confirmation sent",
			},
		},
		"02-participants.mmd": {
			heads: []string{"Alice", "Order store", "«service» API", "Gate", "Ctl", "Thing"},
			messages: []string{
				"1 Alice->API get()", "1.1 API->DB query()", "1.2 API->Ctl check()",
				"1.3 API->Thing load()", "1.4 API->Gate open()",
			},
		},
		"03-fragments.mmd": {
			heads: []string{"", "A", "B", "C"},
			messages: []string{
				"1 _STARTER_->A start()", "1.1.1 A->B go()", "1.1.2 A->B wait()", "1.1.3 A->B stop()",
				"1.2.1 A->B next()", "1.3.1 A->B extra()", "1.4.1 B->C one", "1.4.2 B->C two",
				"1.5.1 A->C write()", "1.6.1 A->C init()", "1.7.1 A->C risky()", "1.7.2 A->C recover()",
				"1.7.3 A->C close()",
			},
			frames: []string{"alt", "loop", "opt", "par", "critical", "section", "try", "ref"},
		},
		"04-creation.mmd": {
			heads: []string{"", "s:Session", "Cache", "Log"},
			messages: []string{
				"1 _STARTER_-->s:Session «user»", "1.1 s:Session-->_STARTER_ s", "3 _STARTER_->s:Session touch()",
				"3.1 s:Session-->Cache «create»", "4 s:Session->Log done",
			},
		},
		"05-grammar.mmd": {
			heads: []string{"", "Front end", "Back"},
			messages: []string{
				"1 _STARTER_->Front end load(a,\"b c\").then(done)", "1.1 Front end-->_STARTER_ x",
				"2 Back-->Front end result", "3 Back-->Front end again", "4 Back->Front end",
				"5 _STARTER_->Front end note to self", " _STARTER_-->_STARTER_ early",
			},
		},
	}
	files := mmdFiles(t, "zenuml")
	if len(files) != len(cases) {
		t.Fatalf("%d fixtures and %d cases", len(files), len(cases))
	}
	for name, src := range files {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			want, ok := cases[name]
			if !ok {
				t.Fatal("no case")
			}
			items := seqRender(t, src, theme.Light)
			seqRender(t, src, theme.Dark)
			checkZenUML(t, items, want)
		})
	}
}

// checkZenUML checks the heads, the messages and the fragments of a ZenUML
// diagram that drew.
func checkZenUML(t *testing.T, items []seqItem, want zenWant) {
	t.Helper()
	heads := seqOf(items, "shapes", "icons")
	slices.SortStableFunc(heads, func(a, b seqItem) int { return cmp.Compare(seqNum(a.datum, "x"), seqNum(b.datum, "x")) })
	labels := map[float64]string{}
	for _, it := range seqOf(items, "actorLabels") {
		labels[seqNum(it.datum, "x")] = it.text
	}
	var gotHeads []string
	for _, h := range heads {
		gotHeads = append(gotHeads, labels[seqNum(h.datum, "x")])
	}
	if !slices.Equal(gotHeads, want.heads) {
		t.Errorf("expected the heads %q, got %q", want.heads, gotHeads)
	}
	messages := seqOf(items, "messages", "messagesDotted")
	slices.SortStableFunc(messages, func(a, b seqItem) int { return cmp.Compare(seqNum(a.datum, "y"), seqNum(b.datum, "y")) })
	// The number of each message is at its left, on the line of the
	// message.
	numbers := map[float64]string{}
	for _, it := range seqOf(items, "numberLabels") {
		numbers[seqNum(it.datum, "y")] = it.text
	}
	var got []string
	for _, m := range messages {
		arrow := "->"
		if m.name == "messagesDotted" {
			arrow = "-->"
		}
		got = append(got, strings.TrimRight(fmt.Sprintf("%s %s%s%s %s", numbers[seqNum(m.datum, "y")-2], m.datum["from"], arrow, m.datum["to"], m.datum["text"]), " "))
	}
	if !slices.Equal(got, want.messages) {
		t.Errorf("expected the messages\n%q, got\n%q", want.messages, got)
	}
	frames := seqOf(items, "frames")
	slices.SortStableFunc(frames, func(a, b seqItem) int { return cmp.Compare(seqNum(a.datum, "y"), seqNum(b.datum, "y")) })
	var gotFrames []string
	for _, f := range frames {
		block, _ := f.datum["block"].(string)
		gotFrames = append(gotFrames, block)
	}
	if !slices.Equal(gotFrames, want.frames) {
		t.Errorf("expected the frames %q, got %q", want.frames, gotFrames)
	}
}

func TestZenUMLIssues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		src, issue string
	}{
		{"zenuml\n  A #FF0000\n", "the color #FF0000 of the participant A"},
		{"zenuml\n  [smile] A\n", "the emoji smile of the participant A"},
		{"zenuml\n  @EC2 A\n", "the icon EC2 of the participant A, drawn as a participant"},
		{"zenuml\n  // [red] hot\n  A.m()\n", "the style [red] of a comment"},
		{"zenuml\n  // **bold**\n  A.m()\n", `the Markdown of the comment "**bold**", drawn as plain text`},
		{"zenuml\n  A.m() {\n", "no } where the grammar of ZenUML wants one"},
		{"zenuml\n  A.m()\n  }\n", `the text "}", which does not fit the grammar of ZenUML`},
	} {
		_, _, err := zuzu.Parse([]byte(tc.src), zuzu.WithStrict(true))
		if !errors.Is(err, vega.ErrUnsupported) || !strings.Contains(err.Error(), tc.issue) {
			t.Errorf("%q: expected the issue %q, got %v", tc.src, tc.issue, err)
		}
		_, report, err := zuzu.Parse([]byte(tc.src))
		if err != nil || len(report) != 1 || !strings.Contains(report[0].Error(), tc.issue) {
			t.Errorf("%q: expected the report %q, got %v and %v", tc.src, tc.issue, report, err)
		}
	}
}

func TestZenUMLGrammar(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, src string
		want      zenWant
	}{
		{
			// A name alone at the start is a participant, and the type
			// before an assignment too, as the grammar of ZenUML reads them.
			// A name alone after a statement is a message to the sender
			// itself.
			name: "head",
			src:  "zenuml\n  SomeType a = A.m()\n  Z\n",
			want: zenWant{
				heads:    []string{"", "SomeType", "A"},
				messages: []string{"1 _STARTER_->A m()", "1.1 A-->_STARTER_ a", "2 _STARTER_->_STARTER_ Z"},
			},
		},
		{
			// A type alone is a participant with no name, which ZenUML
			// names so.
			name: "type alone",
			src:  "zenuml\n  @Actor\n  A.m()\n",
			want: zenWant{
				heads:    []string{"", "Missing `Participant`", "A"},
				messages: []string{"1 _STARTER_->A m()"},
			},
		},
		{
			// ZenUML reads accTitle as the name of a participant and the
			// rest of the line as the text of a message.
			name: "accTitle",
			src:  "zenuml\n  accTitle: hello\n",
			want: zenWant{
				heads:    []string{"", "accTitle"},
				messages: []string{"1 _STARTER_->accTitle hello"},
			},
		},
		{
			// A colon starts the text of a message, which runs to the end of
			// the line.
			name: "colon",
			src:  "zenuml\n  A->B: a: b // c\n",
			want: zenWant{heads: []string{"A", "B"}, messages: []string{"1 A->B a: b // c"}},
		},
		{
			// A source with no statement draws the starter alone.
			name: "empty",
			src:  "zenuml\n",
			want: zenWant{heads: []string{""}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			checkZenUML(t, seqRender(t, tc.src, theme.Light), tc.want)
		})
	}
}

func TestZenUMLTitle(t *testing.T) {
	t.Parallel()
	// ZenUML draws its own title, and not the title of the front matter.
	for src, want := range map[string]string{
		"---\ntitle: Front\n---\nzenuml\n  A.m()\n":              "",
		"---\ntitle: Front\n---\nzenuml\n  title Own\n  A.m()\n": "Own",
		"zenuml\n  // a comment\n  title Own\n  A.m()\n":         "Own",
		"zenuml\n  A.m()\n  title Late\n":                        "",
	} {
		c, _, err := zuzu.Parse([]byte(src), zuzu.WithStrict(true))
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		var got string
		if title := c.Spec().Title; title != nil {
			if err := title.Text.Decode(&got); err != nil {
				t.Fatal(err)
			}
		}
		if got != want {
			t.Errorf("%q: expected the title %q, got %q", src, want, got)
		}
	}
}
