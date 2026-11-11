package mermaid_test

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/scene"
)

// quadPoint matches a point of a quadrant chart: its name, x and y.
var quadPoint = regexp.MustCompile(`^\s*"?([^:"\[]+?)"?\s*(?::::[\w-]+)?\s*:\s*\[\s*([0-9.]+)\s*,\s*([0-9.]+)\s*\]`)

// quadNames matches a name of a quadrant or the names of an axis.
var quadNames = regexp.MustCompile(`^\s*(?:quadrant-[1-4]|x-axis|y-axis)\s+(.*)$`)

func TestQuadrantFixtures(t *testing.T) {
	t.Parallel()
	for name, src := range mmdFiles(t, "quadrant") {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			items := renderMermaid(t, src)
			n := 0
			for l := range strings.SplitSeq(src, "\n") {
				if m := quadNames.FindStringSubmatch(l); m != nil {
					for s := range strings.SplitSeq(m[1], "-->") {
						s = strings.Trim(strings.TrimSpace(s), `"`)
						if c := textCount(items, s); c != 1 {
							t.Errorf("the name %q draws %d times", s, c)
						}
					}
				}
				m := quadPoint.FindStringSubmatch(l)
				if m == nil {
					continue
				}
				n++
				x, _ := strconv.ParseFloat(m[2], 64)
				y, _ := strconv.ParseFloat(m[3], 64)
				if c := itemCount(items, scene.Symbol, "point", func(d map[string]any) bool {
					return d["label"] == m[1] && d["x"] == x && d["y"] == y
				}); c != 1 {
					t.Errorf("the point %s draws %d times", m[1], c)
				}
				if c := textCount(items, m[1]); c != 1 {
					t.Errorf("the label %q draws %d times", m[1], c)
				}
			}
			if c := itemCount(items, scene.Symbol, "point", func(map[string]any) bool { return true }); c != n {
				t.Errorf("expected %d points, got %d", n, c)
			}
		})
	}
}

func TestQuadrantStyles(t *testing.T) {
	t.Parallel()
	src := `quadrantChart
  A: [0.1, 0.1]
  B:::big: [0.2, 0.2]
  C:::big: [0.3, 0.3] radius: 4, color: #00f
  D: [0.4, 0.4] stroke-color: #0f0, stroke-width: 3px
  classDef big color: #f00, radius: 10`
	items := renderMermaid(t, src)
	tests := []struct {
		label, fill, stroke string
		size, width         float64
	}{
		{"B", "#f00", "", 400, 0},
		{"C", "#00f", "", 64, 0},
		{"D", "", "#0f0", 100, 3},
	}
	for _, test := range tests {
		found := false
		for _, it := range items {
			if it.mark != scene.Symbol || it.datum == nil || it.datum["label"] != test.label {
				continue
			}
			found = true
			if test.fill != "" && !strings.EqualFold(it.item.Fill.Color, test.fill) {
				t.Errorf("%s: expected the fill %s, got %s", test.label, test.fill, it.item.Fill.Color)
			}
			if test.stroke != "" && !strings.EqualFold(it.item.Stroke.Color, test.stroke) {
				t.Errorf("%s: expected the stroke %s, got %s", test.label, test.stroke, it.item.Stroke.Color)
			}
			if it.item.Size == nil || *it.item.Size != test.size {
				t.Errorf("%s: expected the size %v, got %v", test.label, test.size, it.item.Size)
			}
			if test.width > 0 && (it.item.StrokeWidth == nil || *it.item.StrokeWidth != test.width) {
				t.Errorf("%s: expected the stroke width %v, got %v", test.label, test.width, it.item.StrokeWidth)
			}
		}
		if !found {
			t.Errorf("no point %s", test.label)
		}
	}
}

func TestQuadrantErrors(t *testing.T) {
	t.Parallel()
	tests := []string{
		"quadrantChart\nA: [1.2, 0.5]",
		"quadrantChart\nA: [0.5]",
		"quadrantChart\nA: [0.5, x]",
		"quadrantChart\nA: [0.5, 0.5] size: 4",
		"quadrantChart\nA: [0.5, 0.5] radius: big",
		"quadrantChart\nx-axis a --> b --> c",
		"quadrantChart\nA point with no place",
		"quadrantChart LR\nA: [0.5, 0.5]",
	}
	for _, src := range tests {
		if _, err := compileMermaid(t, src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: expected a syntax error, got %v", src, err)
		}
	}
}

func TestQuadrantIssues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src, part string
	}{
		{"quadrantChart\nA:::nope: [0.5, 0.5]", "the class nope"},
		{"quadrantChart\naccDescr: points\nA: [0.5, 0.5]", "accDescr"},
		{"---\nconfig:\n  quadrantChart:\n    quadrantPadding: 9\n---\nquadrantChart\nA: [0.5, 0.5]", "quadrantChart.quadrantPadding"},
		{"---\nconfig:\n  themeVariables:\n    quadrant1Fill: red\n---\nquadrantChart\nA: [0.5, 0.5]", "themeVariables"},
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
