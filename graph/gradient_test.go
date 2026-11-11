package graph

import (
	"encoding/json/v2"
	"slices"
	"strings"
	"testing"
)

func TestColorX11(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, exp string
	}{
		{"salmon2", "#ee8262"},
		{"gray95", "#f2f2f2"},
		{"Grey28", "#474747"},
		{"gray0", "#000000"},
		{"grey100", "#ffffff"},
		{"lightblue2", "#b2dfee"},
		{"darkolivegreen3", "#a2cd5a"},
		{"lightyellow1", "#ffffe0"},
		{"navyblue", "#000080"},
		{"lightgoldenrod", "#eedd82"},
		// A name that CSS has keeps its color of CSS.
		{"gray", "gray"},
		{"green", "green"},
		{"goldenrod2:red", "#eeb422"},
	}
	for _, test := range tests {
		if got, ok := Color(test.in); !ok || got != test.exp {
			t.Errorf("Color(%q) = %q, %v, want %q", test.in, got, ok, test.exp)
		}
	}
	if _, ok := Color("gray101"); ok {
		t.Error("gray101 is not a color")
	}
}

func TestGradient(t *testing.T) {
	t.Parallel()
	tests := []struct {
		list   string
		angle  float64
		radial bool
		exp    string
	}{
		{"red:blue", 0, false, `{"gradient":"linear","stops":[{"color":"red","offset":0},{"color":"blue","offset":1}],"x1":0,"x2":1,"y1":0.5,"y2":0.5}`},
		{"red:blue", 90, false, `{"gradient":"linear","stops":[{"color":"red","offset":0},{"color":"blue","offset":1}],"x1":0.5,"x2":0.5,"y1":1,"y2":0}`},
		{"red;0.3:blue", 0, false, `{"gradient":"linear","stops":[{"color":"red","offset":0.299},{"color":"blue","offset":0.3}],"x1":0,"x2":1,"y1":0.5,"y2":0.5}`},
		{"red:blue;0.3", 0, false, `{"gradient":"linear","stops":[{"color":"red","offset":0.699},{"color":"blue","offset":0.7}],"x1":0,"x2":1,"y1":0.5,"y2":0.5}`},
		{"red:salmon2", 0, true, `{"gradient":"radial","r1":0,"r2":0.75,"stops":[{"color":"red","offset":0},{"color":"#ee8262","offset":1}],"x1":0.5,"x2":0.5,"y1":0.5,"y2":0.5}`},
		{"red:blue", 45, true, `{"gradient":"radial","r1":0,"r2":0.75,"stops":[{"color":"red","offset":0},{"color":"blue","offset":1}],"x1":0.8536,"x2":0.5,"y1":0.1464,"y2":0.5}`},
		{"red:blue", 180, true, `{"gradient":"radial","r1":0,"r2":0.75,"stops":[{"color":"red","offset":0},{"color":"blue","offset":1}],"x1":0.001,"x2":0.5,"y1":0.5,"y2":0.5}`},
	}
	for _, test := range tests {
		g, ok := Gradient(test.list, test.angle, test.radial)
		if !ok {
			t.Errorf("Gradient(%q) is false", test.list)
			continue
		}
		b, err := json.Marshal(g, json.Deterministic(true))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != test.exp {
			t.Errorf("Gradient(%q, %v, %v)\n got %s\nwant %s", test.list, test.angle, test.radial, b, test.exp)
		}
	}
	for _, list := range []string{"red", "red:nocolor", ""} {
		if _, ok := Gradient(list, 0, false); ok {
			t.Errorf("Gradient(%q) is true", list)
		}
	}
	if _, _, ok := ColorList("red;2:blue"); ok {
		t.Error("a weight of 2 reads")
	}
}

func TestHTMLGradients(t *testing.T) {
	t.Parallel()
	src := `<table gradientangle="315" bgcolor="a:b" style="rounded"><!-- <td gradientangle="1"> -->
<tr><td bgcolor="a:b">x</td><td STYLE='radial,rounded' gradientangle = "90">y</td></tr>
<tr><td><table style="radial"><tr><td gradientangle="x">z</td></tr></table></td></tr></table>`
	out, tables, cells := HTMLGradients(src)
	if len(out) != len(src) || strings.Count(out, "\n") != strings.Count(src, "\n") {
		t.Errorf("the places changed: %q", out)
	}
	if strings.Contains(out, `gradientangle="315"`) || strings.Contains(out, "radial") || !strings.Contains(out, `gradientangle="x"`) {
		t.Errorf("the source is %q", out)
	}
	expTables := []HTMLGradient{{Angle: 315}, {Radial: true}}
	expCells := []HTMLGradient{{}, {Angle: 90, Radial: true}, {}, {}}
	if !slices.Equal(tables, expTables) || !slices.Equal(cells, expCells) {
		t.Errorf("tables %v, cells %v", tables, cells)
	}
}

func TestColorScheme(t *testing.T) {
	t.Parallel()
	// The values and the colors that do not read follow dot 16.1.1.
	tests := []struct {
		in, scheme, exp string
	}{
		{"/rdylgn11/1", "", "#a50026"},
		{"/RdYlGn11/2", "", "#d73027"},
		{"/rdylgn11/11", "", "#006837"},
		{"/set312/12", "", "#ffed6f"},
		{"/blues9/2:/reds9/8", "", "#deebf7"},
		{"3", "reds9", "#fcbba1"},
		{"red", "reds9", "red"},
		{"salmon2", "blues9", "#ee8262"},
		{"/x11/green", "", "#00ff00"},
		{"/x11/salmon2", "", "#ee8262"},
		{"/svg/green", "", "green"},
		{"green", "x11", "green"},
		{"//red", "", "red"},
		{"/blues9/3", "reds9", "#c6dbef"},
		{"4", "nosuchscheme", ""},
		{"3", "", ""},
		{"10", "reds9", ""},
		{"0", "reds9", ""},
		{"/greens3/red", "", ""},
		{"/accent3/x", "", ""},
		{"salmon2", "svg", ""},
		{"/svg/salmon2", "", ""},
		{"/nosuchscheme/1", "", ""},
	}
	for _, test := range tests {
		got, ok := ColorIn(test.in, test.scheme)
		if got != test.exp || ok != (test.exp != "") {
			t.Errorf("ColorIn(%q, %q) = %q, %v, want %q", test.in, test.scheme, got, ok, test.exp)
		}
	}
	if n := len(brewer); n != 265 {
		t.Errorf("expected the 265 schemes of Graphviz, got %d", n)
	}
}

func TestHTMLInScheme(t *testing.T) {
	t.Parallel()
	src := `<table bgcolor="2" color='red'><tr><td bgcolor="1;0.3:/blues9/9"><font color="9">x</font></td></tr></table>`
	exp := `<table bgcolor="/reds9/2" color='red'><tr><td bgcolor="/reds9/1;0.3:/blues9/9"><font color="/reds9/9">x</font></td></tr></table>`
	if got := HTMLInScheme(src, "reds9"); got != exp {
		t.Errorf("got  %s\nwant %s", got, exp)
	}
	if got := HTMLInScheme(src, ""); got != src {
		t.Errorf("no scheme changed the source: %s", got)
	}
}
