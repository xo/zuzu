package graph

import (
	"math"
	"strings"

	"github.com/xo/zuzu/internal/shape"
)

// shapes are the shapes of a node that zuzu draws, by their names in DOT
// and in Mermaid. A name maps to the shape that draws it.
var shapes = map[string]string{
	"rect": "rect", "box": "rect", "square": "rect", "rectangle": "rect", "record": "rect", "process": "rect", "proc": "rect",
	"rounded": "rounded", "Mrecord": "rounded", "event": "rounded",
	"stadium": "stadium", "pill": "stadium", "terminal": "stadium",
	"subroutine": "subroutine", "subproc": "subroutine", "fr-rect": "subroutine", "framed-rectangle": "subroutine",
	"cylinder": "cylinder", "cyl": "cylinder", "database": "cylinder", "db": "cylinder",
	"circle": "circle", "circ": "circle", "sm-circ": "circle", "small-circle": "circle", "start": "circle",
	"doublecircle": "doublecircle", "dbl-circ": "doublecircle", "double-circle": "doublecircle",
	"ellipse": "ellipse", "oval": "ellipse",
	"diamond": "diamond", "diam": "diamond", "decision": "diamond", "rhombus": "diamond", "question": "diamond",
	"hexagon": "hexagon", "hex": "hexagon", "prepare": "hexagon",
	"lean-r": "lean-r", "lean-right": "lean-r", "parallelogram": "lean-r", "in-out": "lean-r",
	"lean-l": "lean-l", "lean-left": "lean-l", "out-in": "lean-l",
	"trap-b": "trap-b", "trapezoid": "trap-b", "trapezoid-bottom": "trap-b", "priority": "trap-b",
	"trap-t": "trap-t", "inv-trapezoid": "trap-t", "trapezoid-top": "trap-t", "manual": "trap-t", "invtrapezium": "trap-t", "trapezium": "trap-b",
	"odd": "odd", "asymmetric": "odd",
	"triangle": "triangle", "tri": "triangle", "extract": "triangle",
	"invtriangle": "invtriangle", "flip-tri": "invtriangle", "flipped-triangle": "invtriangle", "manual-file": "invtriangle",
	"octagon": "octagon", "pentagon": "pentagon", "house": "house", "invhouse": "invhouse",
	"note": "note", "doc": "doc", "document": "doc", "folder": "folder", "tab": "folder",
	"point": "point", "f-circ": "point", "filled-circle": "point", "junction": "point",
	"bullseye": "bullseye", "bar": "bar",
	"plaintext": "none", "plain": "none", "none": "none", "text": "none", "underline": "none",
}

// ShapeOf returns the shape that draws a name of a shape, and false for a
// name that zuzu does not draw. An empty name is a rect.
func ShapeOf(name string) (string, bool) {
	if name == "" {
		return "rect", true
	}
	s, ok := shapes[name]
	return s, ok
}

// NodeSize returns the size of a node of a shape around a label of a size,
// with a padding.
func NodeSize(kind string, tw, th, pad float64) (float64, float64) {
	w, h := tw+2*pad, th+2*pad*1.2
	w = math.Max(w, 40)
	h = math.Max(h, 30)
	switch kind {
	case "circle":
		d := math.Max(w, h)
		return d, d
	case "doublecircle":
		d := math.Max(w, h) + 8
		return d, d
	case "ellipse":
		return w * 1.25, h * 1.3
	case "diamond":
		d := tw + th + 2*pad
		return math.Max(d, 40), math.Max(d, 40)
	case "hexagon", "lean-r", "lean-l", "trap-b", "trap-t", "stadium", "odd", "octagon":
		return w + h/2, h
	case "triangle", "invtriangle", "pentagon", "house", "invhouse":
		return w * 1.5, h * 1.5
	case "cylinder":
		return w, h + 12
	case "doc":
		return w, h + 6
	case "point":
		return 8, 8
	case "bullseye":
		return 14, 14
	case "bar":
		return 70, 10
	case "none":
		return tw + 4, th + 4
	}
	return w, h
}

// poly returns the path of a polygon.
func poly(pts ...[2]float64) string {
	var b strings.Builder
	for i, p := range pts {
		if i == 0 {
			b.WriteString("M")
		} else {
			b.WriteString("L")
		}
		b.WriteString(num(p[0]) + "," + num(p[1]))
	}
	b.WriteString("Z")
	return b.String()
}

// ShapePath returns the path of a node of a shape and a size, around 0, 0.
func ShapePath(kind string, w, h float64) string {
	return shapePath(kind, w, h)
}

// RecordPath returns the path of a record: its box, of the shape rect or
// rounded, and a line between each two fields. The fields lie across the
// box when across is true, and down it otherwise, and sizes holds the size
// of each field on that axis.
func RecordPath(kind string, w, h float64, across bool, sizes []float64) string {
	var b strings.Builder
	b.WriteString(shapePath(kind, w, h))
	at := -w / 2
	if !across {
		at = -h / 2
	}
	for _, s := range sizes[:max(len(sizes)-1, 0)] {
		at += s
		if across {
			b.WriteString("M" + num(at) + "," + num(-h/2) + "L" + num(at) + "," + num(h/2))
		} else {
			b.WriteString("M" + num(-w/2) + "," + num(at) + "L" + num(w/2) + "," + num(at))
		}
	}
	return b.String()
}

// shapePath returns the path of a shape.
func shapePath(kind string, w, h float64) string {
	x, y := w/2, h/2
	i := h / 4
	ellipse := func(rx, ry float64) string {
		return "M" + num(rx) + ",0A" + num(rx) + "," + num(ry) + " 0 1,1 " + num(-rx) + ",0A" + num(rx) + "," + num(ry) + " 0 1,1 " + num(rx) + ",0Z"
	}
	switch kind {
	case "rect":
		return poly([2]float64{-x, -y}, [2]float64{x, -y}, [2]float64{x, y}, [2]float64{-x, y})
	case "rounded":
		return shape.RoundRect(-x, -y, w, h, 6, 6, 6, 6)
	case "stadium":
		r := y
		return "M" + num(-x+r) + "," + num(-y) + "H" + num(x-r) + "A" + num(r) + "," + num(r) + " 0 0,1 " + num(x-r) + "," + num(y) +
			"H" + num(-x+r) + "A" + num(r) + "," + num(r) + " 0 0,1 " + num(-x+r) + "," + num(-y) + "Z"
	case "subroutine":
		return poly([2]float64{-x, -y}, [2]float64{x, -y}, [2]float64{x, y}, [2]float64{-x, y}) +
			"M" + num(-x+8) + "," + num(-y) + "V" + num(y) + "M" + num(x-8) + "," + num(-y) + "V" + num(y)
	case "cylinder":
		ry := math.Min(h*0.12, 10)
		return "M" + num(-x) + "," + num(-y+ry) + "A" + num(x) + "," + num(ry) + " 0 0,1 " + num(x) + "," + num(-y+ry) +
			"V" + num(y-ry) + "A" + num(x) + "," + num(ry) + " 0 0,1 " + num(-x) + "," + num(y-ry) + "Z" +
			"M" + num(-x) + "," + num(-y+ry) + "A" + num(x) + "," + num(ry) + " 0 0,0 " + num(x) + "," + num(-y+ry)
	case "circle":
		return ellipse(x, x)
	case "doublecircle":
		return ellipse(x, x) + ellipse(x-4, x-4)
	case "ellipse":
		return ellipse(x, y)
	case "point":
		return ellipse(x, y)
	case "bullseye":
		// A ring and a dot: the hole turns the other way, so that it stays
		// empty with either rule of fill.
		ring := func(r float64, sweep string) string {
			return "M" + num(r) + ",0A" + num(r) + "," + num(r) + " 0 1," + sweep + " " + num(-r) + ",0A" + num(r) + "," + num(r) + " 0 1," + sweep + " " + num(r) + ",0Z"
		}
		return ring(x, "1") + ring(x*0.72, "0") + ring(x*0.46, "1")
	case "bar":
		r := math.Min(2, math.Min(w, h)/2)
		return shape.RoundRect(-x, -y, w, h, r, r, r, r)
	case "diamond":
		return poly([2]float64{0, -y}, [2]float64{x, 0}, [2]float64{0, y}, [2]float64{-x, 0})
	case "hexagon":
		return poly([2]float64{-x + i, -y}, [2]float64{x - i, -y}, [2]float64{x, 0}, [2]float64{x - i, y}, [2]float64{-x + i, y}, [2]float64{-x, 0})
	case "lean-r":
		return poly([2]float64{-x + i, -y}, [2]float64{x, -y}, [2]float64{x - i, y}, [2]float64{-x, y})
	case "lean-l":
		return poly([2]float64{-x, -y}, [2]float64{x - i, -y}, [2]float64{x, y}, [2]float64{-x + i, y})
	case "trap-b":
		return poly([2]float64{-x + i, -y}, [2]float64{x - i, -y}, [2]float64{x, y}, [2]float64{-x, y})
	case "trap-t":
		return poly([2]float64{-x, -y}, [2]float64{x, -y}, [2]float64{x - i, y}, [2]float64{-x + i, y})
	case "odd":
		return poly([2]float64{-x, -y}, [2]float64{x, -y}, [2]float64{x, y}, [2]float64{-x, y}, [2]float64{-x + i, 0})
	case "triangle":
		return poly([2]float64{0, -y}, [2]float64{x, y}, [2]float64{-x, y})
	case "invtriangle":
		return poly([2]float64{-x, -y}, [2]float64{x, -y}, [2]float64{0, y})
	case "octagon":
		return poly([2]float64{-x + i, -y}, [2]float64{x - i, -y}, [2]float64{x, -y + i}, [2]float64{x, y - i},
			[2]float64{x - i, y}, [2]float64{-x + i, y}, [2]float64{-x, y - i}, [2]float64{-x, -y + i})
	case "pentagon":
		return poly([2]float64{0, -y}, [2]float64{x, -y * 0.25}, [2]float64{x * 0.62, y}, [2]float64{-x * 0.62, y}, [2]float64{-x, -y * 0.25})
	case "house":
		return poly([2]float64{0, -y}, [2]float64{x, -y * 0.2}, [2]float64{x, y}, [2]float64{-x, y}, [2]float64{-x, -y * 0.2})
	case "invhouse":
		return poly([2]float64{-x, -y}, [2]float64{x, -y}, [2]float64{x, y * 0.2}, [2]float64{0, y}, [2]float64{-x, y * 0.2})
	case "note":
		f := math.Min(10, h/3)
		return poly([2]float64{-x, -y}, [2]float64{x - f, -y}, [2]float64{x, -y + f}, [2]float64{x, y}, [2]float64{-x, y}) +
			"M" + num(x-f) + "," + num(-y) + "V" + num(-y+f) + "H" + num(x)
	case "doc":
		a := math.Min(6, h/5)
		return "M" + num(-x) + "," + num(-y) + "H" + num(x) + "V" + num(y-a) +
			"C" + num(x/2) + "," + num(y+a) + " " + num(-x/2) + "," + num(y-3*a) + " " + num(-x) + "," + num(y-a) + "Z"
	case "folder":
		t := math.Min(8, h/4)
		return poly([2]float64{-x, -y}, [2]float64{-x + w*0.4, -y}, [2]float64{-x + w*0.4 + t, -y + t}, [2]float64{x, -y + t}, [2]float64{x, y}, [2]float64{-x, y})
	case "none":
		return ""
	}
	return shape.RoundRect(-x, -y, w, h, 6, 6, 6, 6)
}
