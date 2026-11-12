package mermaid

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/xo/zuzu/vega"
)

// TestMath checks that each label with math of KaTeX is one issue in the
// loose mode and an error in the strict mode, in the kinds where Mermaid
// 12.3 draws the math, and that zuzu draws no math.
func TestMath(t *testing.T) {
	t.Parallel()
	issue := func(path, label string) string {
		return path + ": " + fmt.Sprintf(mathIssue, label)
	}
	tests := []struct {
		name, src string
		want      []string
	}{
		{"flowchart", "flowchart LR\n  A[\"$$a$$\"] -->|\"$$b$$\"| B\n  subgraph S[\"$$c$$\"]\n    B\n  end\n", []string{
			issue("node A", "$$a$$"), issue("edge A to B", "$$b$$"), issue("cluster S", "$$c$$"),
		}},
		{"state", "stateDiagram-v2\n  A: x $$a$$ y\n  A --> B: $$b$$\n", []string{
			issue("node A", "x $$a$$ y"), issue("edge A to B", "$$b$$"),
		}},
		{"er", "erDiagram\n  C ||--o{ D : \"$$a$$\"\n  C {\n    string name \"$$b$$\"\n  }\n", []string{
			issue("node C", "$$b$$"), issue("edge C to D", "$$a$$"),
		}},
		{"usecase", "usecase-beta\n  actor U(\"$$a$$\")\n  L(\"$$b$$\")\n  U --> L\n", []string{
			issue("node U", "$$a$$"), issue("node L", "$$b$$"),
		}},
		{"sequence", "sequenceDiagram\n  box Aqua $$z$$\n    participant A as $$a$$\n  end\n  A->>B: $$b$$\n  Note over A: $$c$$\n  loop $$d$$\n    A->>B: hi\n  end\n", []string{
			issue("line 3", "$$a$$"), issue("line 5", "$$b$$"), issue("line 6", "$$c$$"), issue("line 7", "$$d$$"),
		}},
		{"mindmap", "mindmap\n  root\n    a[\"$$a$$\"]\n", []string{issue("line 3", "$$a$$")}},
		{"block", "block-beta\n  a[\"$$a$$\"] b\n  a -- \"$$b$$\" --> b\n", []string{issue("line 2", "$$a$$"), issue("line 3", "$$b$$")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := reportOf(t, test.src)
			for _, w := range test.want {
				if !slices.Contains(got, w) {
					t.Errorf("the report %q does not hold %q", got, w)
				}
			}
			if len(got) != len(test.want) {
				t.Errorf("the report %q, want %d issues", got, len(test.want))
			}
			if err := strictCheck(t, test.src); !errors.Is(err, vega.ErrUnsupported) {
				t.Errorf("in the strict mode, the error %v", err)
			}
		})
	}
	// Mermaid draws the source of the math as text in a label of SVG, and
	// in the kinds that draw no math, as zuzu does. That is no issue.
	for _, src := range []string{
		"---\nconfig:\n  htmlLabels: false\n---\nflowchart LR\n  A[\"$$a$$\"] --> B\n",
		"---\nconfig:\n  flowchart:\n    htmlLabels: false\n---\nclassDiagram\n  class A[\"$$a$$\"]\n",
		"---\nconfig:\n  htmlLabels: false\n---\nmindmap\n  root[\"$$a$$\"]\n",
		"---\nconfig:\n  htmlLabels: false\n---\nblock-beta\n  a[\"$$a$$\"]\n",
		"timeline\n  title $$a$$\n  2020 : $$b$$\n",
		"pie title $$a$$\n  \"$$b$$\" : 1\n",
		"gantt\n  dateFormat YYYY-MM-DD\n  $$a$$ : 2024-01-01, 1d\n",
		"C4Context\n  Person(a, \"$$a$$\")\n",
		"flowchart LR\n  A[\"$a$ and $5\"] --> B\n",
		"ishikawa-beta\n  $$a$$\n  $$b$$\n",
	} {
		if err := strictCheck(t, src); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
}
