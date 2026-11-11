package dot

import (
	"testing"
)

func TestColors(t *testing.T) {
	t.Parallel()
	g, report, err := Parse(`digraph { a [style=filled, fillcolor=salmon2, color=Grey28, fontcolor=gray95] }`, true)
	if err != nil {
		t.Fatal(err)
	}
	st := g.Nodes[0].Style
	if st["fill"] != "#ee8262" || st["stroke"] != "#474747" || st["color"] != "#f2f2f2" || len(report) != 0 {
		t.Errorf("style %v, report %v", st, report)
	}
	// An unknown color is an error in the strict mode.
	if _, _, err := Parse(`digraph { a [color=nocolor] }`, true); err == nil {
		t.Error("expected an error for an unknown color in the strict mode")
	}
}

func TestGradients(t *testing.T) {
	t.Parallel()
	src := `digraph {
	a [style=filled, fillcolor="red;0.3:blue", gradientangle=90]
	b [style=radial, fillcolor="red:blue"]
	c [style=radial]
	d [style=filled, fillcolor="red:blue:green"]
	e [style=filled, fillcolor="red:blue", gradientangle=up]
	subgraph cluster_x { style=filled; fillcolor="yellow:green"; gradientangle=270; x }
	subgraph cluster_y { style=radial; bgcolor="white:lightblue2"; y }
	t [shape=none, label=<<table bgcolor="a1:blue" gradientangle="315"><tr><td style="radial" bgcolor="red:blue">x</td></tr></table>>]
}`
	g, report, err := Parse(src, false)
	if err != nil {
		t.Fatal(err)
	}
	style := func(st map[string]string) string {
		return st["fill"] + " " + st["gradient"] + " " + st["gradientAngle"] + " " + st["gradientType"]
	}
	want := []string{"red red;0.3:blue 90 ", "red red:blue  radial", "lightgrey   ", "red red:blue  ", "red red:blue  "}
	for i, w := range want {
		if got := style(g.Nodes[i].Style); got != w {
			t.Errorf("node %s: got %q, want %q", g.Nodes[i].ID, got, w)
		}
	}
	if got := style(g.Clusters[0].Style); got != "yellow yellow:green 270 " {
		t.Errorf("cluster x: %q", got)
	}
	if got := style(g.Clusters[1].Style); got != "white white:#b2dfee  radial" {
		t.Errorf("cluster y: %q", got)
	}
	// The third color, the angle up and the color a1. The gradientangle
	// and the style radial of the label of HTML are not issues.
	var msgs []string
	for _, is := range report {
		msgs = append(msgs, is.Message)
	}
	if len(report) != 3 {
		t.Errorf("expected 3 issues, got %q", msgs)
	}
}

func TestHTMLRecord(t *testing.T) {
	t.Parallel()
	src := `digraph {
	node [shape=record]
	a [label=<{<b>I/O class</b> | + property<br align="left"/>...<br align="left"/>|<i>+ method</i><br align="left"/>}>]
	b [label=<x \| y|<u>z</u>>]
	c [label=<<table><tr><td port="p">t</td></tr></table>>]
	d [label="{a|b}"]
	e [label=<{<b>x</b> y|<font color="red">z</font>}>]
	c:p -> a
}`
	g, report, err := Parse(src, false)
	if err != nil {
		t.Fatal(err)
	}
	a, b, c, d := g.Nodes[0], g.Nodes[1], g.Nodes[2], g.Nodes[3]
	if a.HTML != "" || !a.Stack || len(a.Fields) != 3 {
		t.Fatalf("node a: %q %v %v", a.HTML, a.Stack, a.Fields)
	}
	if f := a.Fields[0]; f.Text != "I/O class" || f.Align != "" || len(f.Fonts) != 1 || !f.Fonts[0].Bold {
		t.Errorf("field 0: %+v", f)
	}
	if f := a.Fields[1]; f.Text != "+ property\n..." || f.Align != "l" || f.Fonts != nil {
		t.Errorf("field 1: %+v", f)
	}
	if f := a.Fields[2]; f.Text != "+ method" || f.Align != "l" || len(f.Fonts) != 1 || !f.Fonts[0].Italic {
		t.Errorf("field 2: %+v", f)
	}
	if b.Stack || len(b.Fields) != 2 || b.Fields[0].Text != "x | y" || len(b.Fields[1].Fonts) != 1 || !b.Fields[1].Fonts[0].Underline {
		t.Errorf("node b: %v %+v", b.Stack, b.Fields)
	}
	// A table with no bar is a label of HTML in the shape of the record.
	if c.HTML == "" || c.Fields != nil {
		t.Errorf("node c: %q %v", c.HTML, c.Fields)
	}
	if !d.Stack || len(d.Fields) != 2 {
		t.Errorf("node d: %v %v", d.Stack, d.Fields)
	}
	// The line in two fonts and the font color of e.
	if len(report) != 2 {
		t.Errorf("expected 2 issues, got %v", report)
	}
	lr, _, err := Parse(`digraph { rankdir=LR; a [shape=record, label="{a|b}"] }`, true)
	if err != nil {
		t.Fatal(err)
	}
	if lr.Nodes[0].Stack {
		t.Error("a group in braces in LR is stacked")
	}
}

func TestColorSchemes(t *testing.T) {
	t.Parallel()
	// The colors follow the SVG of dot 16.1.1 for the same source.
	src := `digraph {
	colorscheme=blues9
	subgraph cluster_x { style=filled; fillcolor=3; label=<<font color="5">T</font>>; p }
	t [shape=none, colorscheme=reds9, label=<<table bgcolor="2"><tr><td bgcolor="4:/greens9/6" color="9"><font color="8">x</font></td></tr></table>>]
	node [colorscheme=greens9]
	u [style=filled, fillcolor="6:/x11/gray", gradientangle=90]
	u -> t [colorscheme=purples9, color=7, fontcolor=9, label=e, labelfontcolor=2, headlabel=h]
}`
	g, report, err := Parse(src, true)
	if err != nil {
		t.Fatalf("%v (%v)", err, report)
	}
	c := g.Clusters[0]
	if c.Style["fill"] != "#c6dbef" || c.HTML != `<font color="/blues9/5">T</font>` {
		t.Errorf("cluster: %v %q", c.Style, c.HTML)
	}
	if n := g.Nodes[1]; n.HTML != `<table bgcolor="/reds9/2"><tr><td bgcolor="/reds9/4:/greens9/6" color="/reds9/9"><font color="/reds9/8">x</font></td></tr></table>` {
		t.Errorf("node t: %q", n.HTML)
	}
	if st := g.Nodes[2].Style; st["fill"] != "#41ab5d" || st["gradient"] != "#41ab5d:#c0c0c0" {
		t.Errorf("node u: %v", st)
	}
	e := g.Edges[0]
	if e.Style["stroke"] != "#6a51a3" || e.Style["color"] != "#3f007d" || e.LabelStyle["color"] != "#efedf5" {
		t.Errorf("edge: %v %v", e.Style, e.LabelStyle)
	}
	// A colorscheme of a graph does not reach its nodes, and that of a
	// cluster does not reach the nodes in it, as in dot.
	for _, src := range []string{
		`digraph { colorscheme=blues9; a [color=3] }`,
		`digraph { subgraph cluster_a { colorscheme=blues9; a [color=3] } }`,
		`digraph { a [colorscheme=set13, color=4] }`,
	} {
		if _, _, err := Parse(src, true); err == nil {
			t.Errorf("%s: expected an error in the strict mode", src)
		}
	}
}
