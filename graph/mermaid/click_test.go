package mermaid

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/vega"
)

// TestClickForms reads each form of a click of a flowchart: a URL, href and
// a URL, a callback, call and a callback, with a tooltip and a target.
func TestClickForms(t *testing.T) {
	t.Parallel()
	src := `flowchart LR
  A --> B & C & D & E & F & G & H
  click A "https://example.com/a" "tip of A"
  click B href "https://example.com/b" "tip of B" _blank
  click C "#part"
  click D callback "tip of D"
  click E call fn(1, "two") "tip of E"
  click F "javascript:alert('x')" "tip of F"
  click G,H href "page.html"
  click Z "https://example.com/z"`
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	g, report, err := Graph(d, false)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]string{
		"A": {"https://example.com/a", "tip of A"},
		"B": {"https://example.com/b", "tip of B"},
		"C": {"#part", ""},
		"D": {"", "tip of D"},
		"E": {"", "tip of E"},
		"F": {"", "tip of F"},
		"G": {"page.html", ""},
		"H": {"page.html", ""},
	}
	for _, n := range g.Nodes {
		if got := [2]string{n.Href, n.Tooltip}; got != want[n.ID] {
			t.Errorf("%s: got %q, want %q", n.ID, got, want[n.ID])
		}
	}
	var msgs []string
	for _, is := range report {
		msgs = append(msgs, is.Message)
	}
	for _, m := range []string{
		"the target _blank of a link",
		"the callback callback, since no script runs",
		`the callback fn(1, "two"), since no script runs`,
		"the link javascript:alert('x'), whose scheme zuzu does not write",
		"a click on Z, which is not a node",
	} {
		if !slices.Contains(msgs, m) {
			t.Errorf("the report does not hold %q: %q", m, msgs)
		}
	}
	if len(msgs) != 5 {
		t.Errorf("the report holds %d issues, want 5: %q", len(msgs), msgs)
	}
	if _, _, err := Graph(d, true); !errors.Is(err, vega.ErrUnsupported) {
		t.Errorf("strict: got %v, want %v", err, vega.ErrUnsupported)
	}
}

// TestClickSyntax checks the statements click that do not read.
func TestClickSyntax(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		`click A`,
		`click A href`,
		`click A "url" "tip" _blank extra`,
		`click "A" "url"`,
		`click A "url`,
	} {
		d, err := Read("flowchart LR\n  A --> B\n  " + src)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := Graph(d, false); !errors.Is(err, ErrSyntax) {
			t.Errorf("%s: got %v, want %v", src, err, ErrSyntax)
		}
	}
}

// TestClassAndStateClicks reads link and callback of a class diagram and a
// click of a state diagram.
func TestClassAndStateClicks(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		src, id, href, tooltip, issue string
	}{
		{"classDiagram\n  class A\n  link A \"https://example.com\" \"a tip\"", "A", "https://example.com", "a tip", ""},
		{"classDiagram\n  class A\n  callback A \"fn\" \"a tip\"", "A", "", "a tip", "the callback fn, since no script runs"},
		{"classDiagram\n  class A\n  click A call fn() \"a tip\"", "A", "", "a tip", "the callback fn(), since no script runs"},
		{"stateDiagram-v2\n  A: Google\n  click A \"https://google.com\" \"Visit Google\"", "A", "https://google.com", "Visit Google", ""},
	} {
		d, err := Read(c.src)
		if err != nil {
			t.Fatal(err)
		}
		g, report, err := Graph(d, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range g.Nodes {
			if n.ID == c.id && (n.Href != c.href || n.Tooltip != c.tooltip) {
				t.Errorf("%s: got %q and %q, want %q and %q", c.src, n.Href, n.Tooltip, c.href, c.tooltip)
			}
		}
		var msgs []string
		for _, is := range report {
			msgs = append(msgs, is.Message)
		}
		if got := strings.Join(msgs, ", "); got != c.issue {
			t.Errorf("%s: got the report %q, want %q", c.src, got, c.issue)
		}
	}
}
