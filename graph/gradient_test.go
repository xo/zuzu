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
