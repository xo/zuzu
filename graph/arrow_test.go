package graph

import (
	"strings"
	"testing"
)

func TestArrowOf(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{
		"", "normal", "inv", "box", "crow", "vee", "tee", "diamond", "dot", "curve", "icurve", "none",
		"onormal", "lnormal", "rnormal", "olbox", "odot", "empty", "invempty", "open", "halfopen", "ediamond",
		"invdot", "invodot", "crowodot", "crowtee", "teeodot", "teetee", "lteeoldiamond", "nonenormal",
		"circle", "cross", "boxboxboxbox",
	} {
		if !ArrowOf(kind) {
			t.Errorf("%q is not an arrowhead", kind)
		}
	}
	for _, kind := range []string{"nope", "boxboxboxboxbox", "o", "normalx", "circlecircle"} {
		if ArrowOf(kind) {
			t.Errorf("%q is an arrowhead", kind)
		}
	}
}

func TestArrowPaths(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ kind, filled, open string }{
		{"normal", "M0,0L-2,-0.8L-2,0.8Z", ""},
		{"", "M0,0L-2,-0.8L-2,0.8Z", ""},
		{"onormal", "", "M0,0L-2,-0.8L-2,0.8Z"},
		{"empty", "", "M0,0L-2,-0.8L-2,0.8Z"},
		{"lnormal", "M0,0L-2,-0.8L-2,0Z", ""},
		{"rnormal", "M0,0L-2,0L-2,0.8Z", ""},
		{"inv", "M-2,0L0,-0.8L0,0.8Z", ""},
		{"none", "", ""},
		{"nonenormal", "M-1,0L-3,-0.8L-3,0.8Z", ""},
		{"teetee", "", "M-0.4,-1.1L-0.4,1.1M-1.4,-1.1L-1.4,1.1"},
		{"crow", "", "M-2,0L0,-1.1M-2,0L0,0M-2,0L0,1.1"},
		{"lcrow", "", "M-2,0L0,-1.1M-2,0L0,0"},
		{"rcrow", "", "M-2,0L0,0M-2,0L0,1.1"},
		{"crowodot", "", "M-2,0L0,-1.1M-2,0L0,0M-2,0L0,1.1M-2,0A0.8,0.8 0 1,0 -3.6,0A0.8,0.8 0 1,0 -2,0Z"},
		{"dotodot", "M0,0A0.8,0.8 0 1,0 -1.6,0A0.8,0.8 0 1,0 0,0Z", "M-1.6,0A0.8,0.8 0 1,0 -3.2,0A0.8,0.8 0 1,0 -1.6,0Z"},
		{"odiamond", "", "M0,0L-1.2,-0.8L-2.4,0L-1.2,0.8Z"},
		{"vee", "M0,0L-2,-0.9L-1.4,0L-2,0.9Z", ""},
		{"lvee", "M0,0L-2,-0.9L-1.4,0Z", ""},
		// A name that is not an arrowhead draws as normal.
		{"nope", "M0,0L-2,-0.8L-2,0.8Z", ""},
	} {
		filled, open := ArrowPaths(c.kind)
		if filled != c.filled || open != c.open {
			t.Errorf("%q: got %q and %q, want %q and %q", c.kind, filled, open, c.filled, c.open)
		}
	}
	// A curve draws the line of the edge through its arc.
	for _, kind := range []string{"curve", "icurve", "lcurve", "ricurve"} {
		if _, open := ArrowPaths(kind); !strings.Contains(open, "A1.4,1.4") || !strings.HasSuffix(open, "M-2,0L0,0") {
			t.Errorf("%q: %q", kind, open)
		}
	}
}
