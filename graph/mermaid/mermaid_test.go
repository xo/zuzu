package mermaid

import (
	"encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
)

func TestFrontMatter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, fm string
		key      string
		exp      any
	}{
		{"folded strip", "description: >-\n  One line,\n  and: more\n", "description", "One line, and: more"},
		{"folded clip", "description: >\n  a\n  b\n", "description", "a b\n"},
		{"folded keep", "description: >+\n  a\n\n\n", "description", "a\n\n\n"},
		{"folded blank line", "description: >-\n  a\n\n  b\n  c\n", "description", "a\nb c"},
		{"folded more indent", "description: >-\n  a\n    b\n  c\n", "description", "a\n  b\nc"},
		{"literal", "description: |\n  a: 1\n  b\n", "description", "a: 1\nb\n"},
		{"literal strip", "description: |-\n  a\n    b\n", "description", "a\n  b"},
		{"literal indent first", "description: |2-\n  a\n", "description", "a"},
		{"plain under key", "description:\n  Icons such as fa:fa-cogs,\n  and colons.\n", "description", "Icons such as fa:fa-cogs, and colons."},
		{"plain goes on", "description: Starts here\n  and goes on\n", "description", "Starts here and goes on"},
		{"quoted goes on", "description: \"Starts here,\n  goes on\"\n", "description", "Starts here, goes on"},
		{"title block", "title: >-\n  A long\n  title\n", "title", "A long title"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			src := "---\n" + test.fm + "config:\n  theme: dark\n  pie:\n    textPosition: 0.5\n---\npie\n\"a\": 1\n"
			d, err := Read(src)
			if err != nil {
				t.Fatal(err)
			}
			got := any(d.Title)
			if test.key == "description" {
				got = d.Description
			}
			if got != test.exp {
				t.Errorf("expected %q, got %q", test.exp, got)
			}
			// The config after the block still reads, and the block adds no
			// key of its own.
			if len(d.Config) != 2 || str(d.Config, "theme") != "dark" {
				t.Errorf("config %v", d.Config)
			}
			if v, _ := configNumber(d.Config, "pie", "textPosition"); v != 0.5 {
				t.Errorf("pie.textPosition %v", d.Config["pie"])
			}
		})
	}
}

func TestFrontMatterOneIssue(t *testing.T) {
	t.Parallel()
	src := "---\ndescription: >-\n  A pie with `x: 1` and\n  more: text\nowner: me\n---\npie\n\"a\": 1\n"
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	_, _, report, err := Chart(d, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 1 || !strings.Contains(report[0].Message, "the key owner of the front matter") {
		t.Errorf("expected one issue of owner, got %v", report)
	}
}

func TestDescription(t *testing.T) {
	t.Parallel()
	fm := "---\ndescription: From the front\n---\n"
	acc := "accDescr: the description\naccTitle: the title\n"
	tests := []struct {
		name, src, exp string
	}{
		{"flowchart", fm + "flowchart LR\n" + acc + "A --> B\n", "From the front the description the title"},
		{"state", fm + "stateDiagram-v2\n" + acc + "A --> B\n", "From the front the description the title"},
		{"class", fm + "classDiagram\n" + acc + "A --> B\n", "From the front the description the title"},
		{"er", fm + "erDiagram\n" + acc + "A ||--o{ B : has\n", "From the front the description the title"},
		{"front matter only", fm + "flowchart LR\nA --> B\n", "From the front"},
		{"no front matter", "flowchart LR\n" + acc + "A --> B\n", "the description the title"},
		{"pie", fm + "pie\n" + acc + "\"a\": 1\n", "From the front the description the title"},
		{"sequence", fm + "sequenceDiagram\n" + acc + "A->>B: hi\n", "From the front the description the title"},
		{"gantt", fm + "gantt\n" + acc + "dateFormat YYYY-MM-DD\nT : 2024-01-01, 1d\n", "From the front the description the title"},
		{"empty gantt", fm + "gantt\n" + acc, "From the front the description the title"},
		{"sankey", fm + "sankey\nA,B,1\n", "From the front"},
		{"number", "---\ndescription: 42\n---\npie\n\"a\": 1\n", "42"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			d, err := Read(test.src)
			if err != nil {
				t.Fatal(err)
			}
			var got string
			if IsChart(d.Kind) {
				b, _, _, err := Chart(d, true)
				if err != nil {
					t.Fatal(err)
				}
				var spec struct {
					Description string `json:"description"`
				}
				if err := json.Unmarshal(b, &spec); err != nil {
					t.Fatal(err)
				}
				got = spec.Description
			} else {
				g, _, err := Graph(d, true)
				if err != nil {
					t.Fatal(err)
				}
				got = g.Description
			}
			if got != test.exp {
				t.Errorf("expected %q, got %q", test.exp, got)
			}
		})
	}
}

func TestHeaderLine(t *testing.T) {
	t.Parallel()
	fm := "---\ntitle: T\ndescription: >-\n  a\n  b\n---\n%% a comment\n\n"
	// The keyword is on line 9 of each source.
	tests := []string{
		fm + "pie bogus\n\"a\": 1",
		fm + "quadrantChart bogus\nquadrant-1 A",
		fm + "journey bogus\nsection A",
		fm + "xychart bogus\nbar [1]",
		fm + "sankey bogus\na,b,1",
		fm + "treemap bogus\n\"a\": 1",
		fm + "radar-beta bogus\naxis a",
		fm + "mindmap bogus\nroot",
		fm + "wardley-beta bogus\ncomponent a [0.5, 0.5]",
		fm + "architecture-beta bogus\nservice a(server)[A]",
		fm + "sequenceDiagram bogus\nA->>B: hi",
		fm + "gitGraph bogus\ncommit",
	}
	for _, src := range tests {
		d, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		if d.N != 9 {
			t.Errorf("%s: expected the keyword on line 9, got %d", d.Kind, d.N)
		}
		_, _, _, err = Chart(d, false)
		if !errors.Is(err, ErrSyntax) || !strings.HasPrefix(err.Error(), "line 9: ") {
			t.Errorf("%s: expected an error at line 9, got %v", d.Kind, err)
		}
	}
	for _, src := range []string{fm + "flowchart XY\na", fm + "stateDiagram-v2 bogus\n[*] --> a", fm + "erDiagram bogus\nA ||--o{ B : has"} {
		d, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = Graph(d, false)
		if !errors.Is(err, ErrSyntax) || !strings.HasPrefix(err.Error(), "line 9: ") {
			t.Errorf("%s: expected an error at line 9, got %v", d.Kind, err)
		}
	}
}

// nodeLabels returns the label of each node of a flowchart by its id.
func nodeLabels(t *testing.T, src string) (map[string]string, *graph.Graph) {
	t.Helper()
	d, err := Read(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	g, _, err := Graph(d, true)
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	out := map[string]string{}
	for _, n := range g.Nodes {
		out[n.ID] = n.Label
	}
	return out, g
}

func TestMarkdownLines(t *testing.T) {
	t.Parallel()
	src := "flowchart LR\n" +
		"  subgraph \"`**Two**\n  titles`\"\n" +
		"  a(\"`The **cat**\n  in the hat`\") -- \"`1o **ipa**\n  more`\" --> b{{\"`The **dog** in the hog`\"}}\n" +
		"  b -->|\"`a\n  b`\"| c[\"plain\n  text\"]\n" +
		"  end\n" +
		"  d[after]"
	labels, g := nodeLabels(t, src)
	exp := map[string]string{"a": "The cat\nin the hat", "b": "The dog in the hog", "c": "plain\ntext", "d": "after"}
	for k, v := range exp {
		if labels[k] != v {
			t.Errorf("node %s: expected %q, got %q", k, v, labels[k])
		}
	}
	if len(g.Edges) != 2 || g.Edges[0].Label != "1o ipa\nmore" || g.Edges[1].Label != "a\nb" {
		t.Errorf("edges %+v", g.Edges)
	}
	if len(g.Clusters) != 1 || g.Clusters[0].Label != "Two\ntitles" {
		t.Errorf("clusters %+v", g.Clusters)
	}
}

func TestPropertyQuotes(t *testing.T) {
	t.Parallel()
	src := "flowchart TB\n" +
		"  a@{ shape: rect, label: 'Single, quoted' }\n" +
		"  b@{ shape: rect, label: \"Double, quoted\" }\n" +
		"  c@{ shape: rect, label: 'It''s \"here\"' }\n" +
		"  d@{ shape: rect, label: It's plain }"
	labels, _ := nodeLabels(t, src)
	exp := map[string]string{"a": "Single, quoted", "b": "Double, quoted", "c": `It's "here"`, "d": "It's plain"}
	for k, v := range exp {
		if labels[k] != v {
			t.Errorf("node %s: expected %q, got %q", k, v, labels[k])
		}
	}
}
