package zuzu

import (
	"strings"
	"testing"
)

func TestDOTPaint(t *testing.T) {
	t.Parallel()
	src := `digraph {
	node [shape=record, style=filled, fillcolor=gray95]
	a [shape=box, fillcolor="salmon2:goldenrod2", gradientangle=90]
	b [shape=ellipse, style=radial, fillcolor="white:lightblue2"]
	c [label=<{<b>I/O class</b>|+ property<br align="left"/>}>]
	subgraph cluster_x { style=filled; fillcolor="lightyellow1:darkolivegreen3"; d }
	t [shape=none, label=<<table><tr><td bgcolor="red;0.3:blue" gradientangle="270">x</td></tr></table>>]
	a -> b -> c -> d
}`
	s, report, err := svgOf(t, src, true)
	if err != nil {
		t.Fatalf("%v (%v)", err, report)
	}
	for _, want := range []string{
		`x1="0.5" y1="1" x2="0.5" y2="0"><stop offset="0" stop-color="#ee8262"/><stop offset="1" stop-color="#eeb422"/>`,
		`<radialGradient`, `stop-color="#b2dfee"`,
		`x1="0.5" y1="0" x2="0.5" y2="1"><stop offset="0.299" stop-color="red"/><stop offset="0.3" stop-color="blue"/>`,
		`stop-color="#ffffe0"`, `stop-color="#a2cd5a"`, `fill="#f2f2f2"`,
		`>I/O class<`, `>+ property<`, `font-weight="600"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("the SVG has no %s", want)
		}
	}
	if strings.Contains(s, "{") || strings.Contains(s, "&lt;b&gt;") {
		t.Error("the record draws its braces or its tags as text")
	}
}

func TestDOTColorSchemes(t *testing.T) {
	t.Parallel()
	src := `digraph {
	a [shape=none, colorscheme=reds9, label=<<table bgcolor="/rdylgn11/1:/rdylgn11/11" gradientangle="315"><tr><td bgcolor="2" style="radial">x</td><td bgcolor="3:/blues9/4">y</td></tr></table>>]
	b [style=filled, colorscheme=greens9, fillcolor=6]
	a -> b
}`
	s, report, err := svgOf(t, src, true)
	if err != nil {
		t.Fatalf("%v (%v)", err, report)
	}
	for _, want := range []string{
		`stop-color="#a50026"`, `stop-color="#006837"`, `fill="#fee0d2"`,
		`stop-color="#fcbba1"`, `stop-color="#9ecae1"`, `fill="#41ab5d"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("the SVG has no %s", want)
		}
	}
}
