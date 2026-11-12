package zuzu

import (
	"math"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/theme"
)

// edgeItem is the line of an edge or an arrowhead of a diagram, as a scene
// draws it.
type edgeItem struct {
	x, y   float64
	stroke string
	fill   string
	width  float64
	size   float64
}

// edgeItems renders a diagram and returns the lines of its edges, the path
// items that have a stroke and no fill, with the last point of each, and
// its arrowheads, the symbols with a path.
func edgeItems(t *testing.T, src string, opts ...RenderOption) ([]edgeItem, []edgeItem) {
	t.Helper()
	c, _, err := Parse([]byte(src), WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	s, _, err := c.Render(t.Context(), append([]RenderOption{WithLocation(time.UTC)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	num := regexp.MustCompile(`-?[0-9.]+(?:e-?[0-9]+)?`)
	var lines, arrows []edgeItem
	var walk func(m *scene.Mark)
	walk = func(m *scene.Mark) {
		for i := range m.Items {
			it := m.Items[i]
			e := edgeItem{x: it.X, y: it.Y, stroke: it.Stroke.Color, fill: it.Fill.Color}
			if it.StrokeWidth != nil {
				e.width = *it.StrokeWidth
			}
			switch {
			case m.Type == scene.Symbol && len(it.Shape) > 0 && it.Shape[0] == 'M':
				if it.Size != nil {
					e.size = *it.Size
				}
				arrows = append(arrows, e)
			case m.Type == scene.Path && it.Stroke.Color != "" && it.Fill.Color == "" && it.Path != "":
				ns := num.FindAllString(it.Path, -1)
				x, _ := strconv.ParseFloat(ns[len(ns)-2], 64)
				y, _ := strconv.ParseFloat(ns[len(ns)-1], 64)
				e.x, e.y = it.X+x, it.Y+y
				lines = append(lines, e)
			}
			for j := range it.Items {
				walk(&it.Items[j])
			}
		}
	}
	walk(&s.Root)
	return lines, arrows
}

// TestEdgesTakeTheTokensOfAnEdge draws the edges of DOT in the color edge
// and the width edgeWidth of docs/STYLE.md, with arrowheads of
// arrowLength. It checks that a color and a width of the source win over
// the tokens, that arrowsize scales the arrowhead, and that the tip of each
// arrowhead is the end of its line, on the border of the node.
func TestEdgesTakeTheTokensOfAnEdge(t *testing.T) {
	t.Parallel()
	src := `digraph {
	rankdir=LR
	node [shape=box]
	a -> b
	c -> d [arrowsize=2]
	edge [color=black]
	e -> f
	g -> h [color="#ff0000", penwidth=3]
}`
	for _, m := range []theme.Mode{theme.Light, theme.Dark} {
		th := theme.New(m, theme.Zuzu)
		lines, arrows := edgeItems(t, src, WithMode(m))
		if len(lines) != 4 || len(arrows) != 4 {
			t.Fatalf("%s: %d lines and %d arrowheads, want 4 and 4", m, len(lines), len(arrows))
		}
		size := theme.ArrowLength * theme.ArrowLength
		want := map[string]struct {
			width, size float64
			n           int
		}{
			th.Edge:   {theme.EdgeWidth, size, 2},
			"black":   {theme.EdgeWidth, size, 1},
			"#ff0000": {3, size, 1},
		}
		got := map[string]int{}
		for _, l := range lines {
			w, ok := want[l.stroke]
			if !ok {
				t.Errorf("%s: a line in %q", m, l.stroke)
				continue
			}
			got[l.stroke]++
			if l.width != w.width {
				t.Errorf("%s: the line in %s is %v wide, want %v", m, l.stroke, l.width, w.width)
			}
			// The tip of an arrowhead is the end of the line, at each size.
			var a *edgeItem
			for i := range arrows {
				if math.Abs(arrows[i].x-l.x) < 1e-6 && math.Abs(arrows[i].y-l.y) < 1e-6 {
					a = &arrows[i]
				}
			}
			if a == nil {
				t.Errorf("%s: no arrowhead at the end %v,%v of the line in %s", m, l.x, l.y, l.stroke)
				continue
			}
			if a.fill != l.stroke || a.stroke != l.stroke {
				t.Errorf("%s: the arrowhead of the line in %s is %s and %s", m, l.stroke, a.fill, a.stroke)
			}
			if a.size != w.size && a.size != 4*w.size {
				t.Errorf("%s: the arrowhead of the line in %s has the size %v", m, l.stroke, a.size)
			}
		}
		for c, w := range want {
			if got[c] != w.n {
				t.Errorf("%s: %d lines in %s, want %d", m, got[c], c, w.n)
			}
		}
		// The arrowsize 2 makes the arrowhead twice as long, four times
		// the size of the symbol.
		var big int
		for _, a := range arrows {
			if a.size == 4*size {
				big++
			}
		}
		if big != 1 {
			t.Errorf("%s: %d arrowheads of the arrowsize 2, want 1", m, big)
		}
	}
}

// TestLinkStyleWinsOverTheEdge checks that the stroke of linkStyle of
// Mermaid paints a link and its arrowhead in place of the color edge.
func TestLinkStyleWinsOverTheEdge(t *testing.T) {
	t.Parallel()
	lines, arrows := edgeItems(t, "flowchart LR\n  a --> b --> c\n  linkStyle 1 stroke:#00ff00,stroke-width:4px")
	th := theme.New(theme.Light, theme.Zuzu)
	if len(lines) != 2 || len(arrows) != 2 {
		t.Fatalf("%d lines and %d arrowheads, want 2 and 2", len(lines), len(arrows))
	}
	var plain, styled int
	for _, l := range append(lines, arrows...) {
		switch l.stroke {
		case th.Edge:
			plain++
		case "#00ff00":
			styled++
		default:
			t.Errorf("an edge in %q", l.stroke)
		}
	}
	if plain != 2 || styled != 2 {
		t.Errorf("%d items in the color edge and %d in the color of linkStyle, want 2 and 2", plain, styled)
	}
	for _, l := range lines {
		if want := map[bool]float64{true: 4, false: theme.EdgeWidth}[l.stroke == "#00ff00"]; l.width != want {
			t.Errorf("the line in %s is %v wide, want %v", l.stroke, l.width, want)
		}
	}
}
