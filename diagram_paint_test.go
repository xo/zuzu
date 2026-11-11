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
