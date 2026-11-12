package graph

import (
	"math"
	"strings"
)

// arrowAliases are the old names of the arrowheads of DOT, by the names
// that draw them.
var arrowAliases = map[string]string{
	"empty": "onormal", "invempty": "oinv", "open": "vee", "halfopen": "lvee", "ediamond": "odiamond",
}

// arrowBases are the shapes that the name of an arrowhead joins, as the
// arrowType of Graphviz names them. Mermaid adds circle, cross and
// circleplus, which stand alone.
var arrowBases = []string{"normal", "inv", "box", "crow", "vee", "tee", "diamond", "dot", "curve", "icurve", "none"}

// arrowPart is one shape of an arrowhead with its modifiers: open draws it
// with no fill, and side keeps only its left half, l, or its right half,
// r, as seen along the edge toward the node.
type arrowPart struct {
	base string
	open bool
	side byte
}

// arrowParts reads the name of an arrowhead: one to four shapes, each with
// the modifiers o, l and r before it, such as crowodot or lteeoldiamond.
// The first shape is at the node. An empty name is normal. It returns
// false for a name that is not an arrowhead.
func arrowParts(kind string) ([]arrowPart, bool) {
	if kind == "" {
		kind = "normal"
	}
	if a, ok := arrowAliases[kind]; ok {
		kind = a
	}
	if kind == "circle" || kind == "cross" || kind == "circleplus" {
		return []arrowPart{{base: kind}}, true
	}
	var out []arrowPart
	for s := kind; s != ""; {
		if len(out) == 4 {
			return nil, false
		}
		var p arrowPart
		for s != "" && strings.IndexByte("olr", s[0]) >= 0 {
			if s[0] == 'o' {
				p.open = true
			} else {
				p.side = s[0]
			}
			s = s[1:]
		}
		for _, b := range arrowBases {
			if strings.HasPrefix(s, b) {
				p.base = b
				break
			}
		}
		if p.base == "" {
			return nil, false
		}
		s = s[len(p.base):]
		out = append(out, p)
	}
	return out, true
}

// ArrowOf reports whether zuzu draws a kind of arrowhead: a name of the
// arrowType of Graphviz, or circle, cross or circleplus of Mermaid. An
// empty kind is normal.
func ArrowOf(kind string) bool {
	_, ok := arrowParts(kind)
	return ok
}

// ArrowPaths returns the paths of an arrowhead of a kind, for a symbol of
// Vega that points along x to its tip at 0, 0, scaled by the size of the
// symbol. A shape of normal is 2 long, and each shape of the name follows
// the one before it back along the edge. The first path holds the shapes
// that are filled with the color of the edge. The second holds the open
// shapes, which are filled with the background, and the shapes that are
// only lines, such as crow and tee. A kind that is not an arrowhead draws
// as normal, and none draws nothing.
func ArrowPaths(kind string) (string, string) {
	parts, ok := arrowParts(kind)
	if !ok {
		parts = []arrowPart{{base: "normal"}}
	}
	var filled, open strings.Builder
	at := 0.0
	for _, p := range parts {
		path, length, isOpen := p.path(at)
		if isOpen {
			open.WriteString(path)
		} else {
			filled.WriteString(path)
		}
		at -= length
	}
	return filled.String(), open.String()
}

// path returns the path of a shape with its tip at x, its length, and
// whether it goes with the open shapes.
func (p arrowPart) path(x float64) (string, float64, bool) {
	pt := func(px, py float64) [2]float64 { return [2]float64{x + px, py} }
	at := func(px, py float64) string { return num(x+px) + "," + num(py) }
	// lo and hi are the sides of a shape of a half width w, with the
	// half that the side leaves out at 0.
	half := func(w float64) (float64, float64) {
		switch p.side {
		case 'l':
			return -w, 0
		case 'r':
			return 0, w
		}
		return -w, w
	}
	// arc draws an arc of a circle of a radius around cx, 0, from the
	// angle a to b in degrees, cut to the half of the side. It draws the
	// line of the edge through the arc again, because the fill of the arc
	// hides it.
	arc := func(cx, r, a, b float64) string {
		switch p.side {
		case 'l':
			b = (a + b) / 2
		case 'r':
			a = (a + b) / 2
		}
		ra, rb := a*math.Pi/180, b*math.Pi/180
		return "M" + at(cx+r*math.Cos(ra), r*math.Sin(ra)) + "A" + num(r) + "," + num(r) + " 0 0,1 " + at(cx+r*math.Cos(rb), r*math.Sin(rb)) +
			"M" + at(-2, 0) + "L" + at(0, 0)
	}
	switch p.base {
	case "inv":
		lo, hi := half(0.8)
		return poly(pt(-2, 0), pt(0, lo), pt(0, hi)), 2, p.open
	case "box":
		lo, hi := half(0.8)
		return poly(pt(0, lo), pt(-1.6, lo), pt(-1.6, hi), pt(0, hi)), 2, p.open
	case "diamond":
		lo, hi := half(0.8)
		return poly(pt(0, 0), pt(-1.2, lo), pt(-2.4, 0), pt(-1.2, hi)), 2.4, p.open
	case "dot":
		return "M" + at(0, 0) + "A0.8,0.8 0 1,0 " + at(-1.6, 0) + "A0.8,0.8 0 1,0 " + at(0, 0) + "Z", 1.6, p.open
	case "vee":
		lo, hi := half(0.9)
		pts := [][2]float64{pt(0, 0)}
		if lo != 0 {
			pts = append(pts, pt(-2, lo))
		}
		pts = append(pts, pt(-1.4, 0))
		if hi != 0 {
			pts = append(pts, pt(-2, hi))
		}
		return poly(pts...), 2, false
	case "crow":
		lo, hi := half(1.1)
		ys := []float64{lo, 0}
		if hi != 0 {
			ys = append(ys, hi)
		}
		if lo == 0 {
			ys = ys[1:]
		}
		var b strings.Builder
		for _, y := range ys {
			b.WriteString("M" + at(-2, 0) + "L" + at(0, y))
		}
		return b.String(), 2, true
	case "tee":
		lo, hi := half(1.1)
		return "M" + at(-0.4, lo) + "L" + at(-0.4, hi), 1, true
	case "curve":
		return arc(-1.4, 1.4, -50, 50), 2, true
	case "icurve":
		return arc(0.9, 1.4, 130, 230), 2, true
	case "none":
		return "", 1, false
	case "circle":
		return "M0,0A0.6,0.6 0 1,0 " + at(-1.2, 0) + "A0.6,0.6 0 1,0 0,0Z", 1.2, true
	case "cross":
		return "M-0.2,-0.8L-1.8,0.8M-0.2,0.8L-1.8,-0.8", 2, true
	case "circleplus":
		// The circle of the relation contains of a requirement diagram,
		// with a cross of two lines in it, one along the edge.
		return "M0,0A1,1 0 1,0 -2,0A1,1 0 1,0 0,0ZM0,0L-2,0M-1,-1L-1,1", 2, true
	}
	lo, hi := half(0.8)
	return poly(pt(0, 0), pt(-2, lo), pt(-2, hi)), 2, p.open
}
