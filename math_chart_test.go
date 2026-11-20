package zuzu

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/zuzu/text/math"
)

// TestMathOfChartsSVG checks that a sequence diagram, a mindmap and a block
// diagram parsed with the font of math draw each formula as a path of the
// role math, with its outline and its TeX as its label, and with no $$ in a
// text (D36).
func TestMathOfChartsSVG(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, src string
		tex       []string
	}{
		{"sequence", "sequenceDiagram\n  participant A as $$\\alpha$$\n  A->>B: send $$\\frac{a}{b}$$\n  Note over B: $$x^2$$\n", []string{`\alpha`, `\frac{a}{b}`, `x^2`}},
		{"mindmap", "mindmap\n  root((Root))\n    a[\"$$\\frac{a}{b}$$\"]\n", []string{`\frac{a}{b}`}},
		{"block", "block-beta\n  a[\"area $$\\pi r^2$$\"] b\n  a -- \"$$e$$\" --> b\n", []string{`\pi r^2`, `e`}},
	}
	path := regexp.MustCompile(`<path [^>]*d="([^"]*)" role="img" aria-label="([^"]*)"`)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			c, report, err := Parse([]byte(test.src), WithMath(math.Font()), WithStrict(true))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if len(report) > 0 {
				t.Fatalf("the report %v", report)
			}
			var b bytes.Buffer
			if _, err := c.SVG(context.Background(), &b); err != nil {
				t.Fatalf("svg: %v", err)
			}
			out := b.String()
			labels := map[string]bool{}
			for _, m := range path.FindAllStringSubmatch(out, -1) {
				if !strings.HasPrefix(m[1], "M") || len(m[1]) < 20 {
					t.Errorf("the path of math %q has the outline %q", m[2], m[1])
				}
				labels[m[2]] = true
			}
			for _, w := range test.tex {
				if !labels[w] {
					t.Errorf("no path of math with the label %q in %v", w, labels)
				}
			}
			if strings.Contains(out, "$$") {
				t.Errorf("the SVG holds $$")
			}
		})
	}
}
