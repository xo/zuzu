package graph

import (
	"math"
	"strings"
)

// pen writes path data. Each call adds one command.
type pen struct{ b strings.Builder }

func (p *pen) String() string { return p.b.String() }

func (p *pen) m(x, y float64) *pen { p.b.WriteString("M" + num(x) + "," + num(y)); return p }
func (p *pen) l(x, y float64) *pen { p.b.WriteString("L" + num(x) + "," + num(y)); return p }
func (p *pen) h(x float64) *pen    { p.b.WriteString("H" + num(x)); return p }
func (p *pen) v(y float64) *pen    { p.b.WriteString("V" + num(y)); return p }
func (p *pen) z() *pen             { p.b.WriteString("Z"); return p }

func (p *pen) c(x1, y1, x2, y2, x, y float64) *pen {
	p.b.WriteString("C" + num(x1) + "," + num(y1) + " " + num(x2) + "," + num(y2) + " " + num(x) + "," + num(y))
	return p
}

// a writes an arc of the radii rx and ry to x, y. A sweep of 1 turns
// clockwise on the screen.
func (p *pen) a(rx, ry float64, sweep int, x, y float64) *pen {
	s := "0"
	if sweep == 1 {
		s = "1"
	}
	p.b.WriteString("A" + num(rx) + "," + num(ry) + " 0 0," + s + " " + num(x) + "," + num(y))
	return p
}

// stroke returns the path of a line through points that goes there and
// back. It closes on itself, so it holds no area, and a fill leaves it
// alone: the outline draws it and the fill does not.
func stroke(pts ...[2]float64) string {
	var p pen
	p.m(pts[0][0], pts[0][1])
	for _, q := range pts[1:] {
		p.l(q[0], q[1])
	}
	for i := len(pts) - 2; i >= 0; i-- {
		p.l(pts[i][0], pts[i][1])
	}
	return p.String()
}

// circlePath returns the path of a circle at cx, cy, which turns
// clockwise.
func circlePath(cx, cy, r float64) string {
	var p pen
	return p.m(cx+r, cy).a(r, r, 1, cx-r, cy).a(r, r, 1, cx+r, cy).z().String()
}

// wave is the bottom of a document in the box from x0, y0 to x1, y1: a
// curve from the right corner, a above the bottom, to the left corner. Its
// control points are as those of the shape doc.
type wave struct{ x0, y0, x1, y1, a float64 }

// points returns the four points of the curve, from right to left.
func (d wave) points() [4][2]float64 {
	q := (d.x1 - d.x0) / 4
	mid := (d.x0 + d.x1) / 2
	return [4][2]float64{{d.x1, d.y1 - d.a}, {mid + q, d.y1 + d.a}, {mid - q, d.y1 - 3*d.a}, {d.x0, d.y1 - d.a}}
}

// path returns the path of the document.
func (d wave) path() string {
	c := d.points()
	var p pen
	return p.m(d.x0, d.y0).h(d.x1).v(c[0][1]).c(c[1][0], c[1][1], c[2][0], c[2][1], c[3][0], c[3][1]).z().String()
}

// at returns the y of the curve at x.
func (d wave) at(x float64) float64 {
	c := d.points()
	bez := func(t float64, i int) float64 {
		u := 1 - t
		return u*u*u*c[0][i] + 3*u*u*t*c[1][i] + 3*u*t*t*c[2][i] + t*t*t*c[3][i]
	}
	lo, hi := 0.0, 1.0
	for range 40 {
		mid := (lo + hi) / 2
		// x falls from right to left along the curve.
		if bez(mid, 0) > x {
			lo = mid
		} else {
			hi = mid
		}
	}
	return bez((lo+hi)/2, 1)
}

// brace returns the path of a curly brace from -y to y, with its tip at
// x0, 0. A brace of side 1 opens to the right, as {, and of side -1 to the
// left, as }. Its curves have the radius r, and it is 2r wide. It goes
// there and back, as a stroke does.
func brace(x0, y, r, side float64) string {
	sw := func(s int) int {
		if side < 0 {
			return 1 - s
		}
		return s
	}
	end, mid := x0+2*r*side, x0+r*side
	var p pen
	p.m(end, -y).a(r, r, sw(0), mid, -y+r).v(-r).a(r, r, sw(1), x0, 0).a(r, r, sw(1), mid, r).v(y-r).a(r, r, sw(0), end, y)
	p.a(r, r, sw(1), mid, y-r).v(r).a(r, r, sw(0), x0, 0).a(r, r, sw(0), mid, -r).v(-y+r).a(r, r, sw(1), end, -y)
	return p.String()
}

// braceWidth is the width of a brace and the space between it and the
// label.
const braceWidth = 10.0

// stackOffset is the step between the sheets of a stack of documents or
// of processes.
const stackOffset = 5.0

// moreSize returns the size of a node of a shape of Mermaid around a label
// of a size w, h, with its padding, and false for a shape that it does
// not know.
func moreSize(kind string, w, h float64) (float64, float64, bool) {
	switch kind {
	case "lin-doc":
		return w + 6, h + 6, true
	case "tag-doc":
		return w, h + 6, true
	case "docs":
		return w + 2*stackOffset, h + 6 + 2*stackOffset, true
	case "st-rect":
		return w + 2*stackOffset, h + 2*stackOffset, true
	case "brace-l", "brace-r", "bracket":
		return w + braceWidth, h, true
	case "braces":
		return w + 2*braceWidth, h, true
	case "card", "notch-pent", "tag-rect", "datastore":
		return w, h, true
	case "lin-rect":
		return w + 6, h, true
	case "div-rect":
		return w, h + 8, true
	case "win-pane":
		return w + 8, h + 8, true
	case "delay", "curv-trap":
		return w + h/2, h, true
	case "bow-rect":
		return w + h/3, h, true
	case "h-cyl":
		return w + 3*hcylRadius(h), h, true
	case "lin-cyl":
		return w, h + 36, true
	case "sl-rect":
		return w, h * 1.5, true
	case "paper-tape":
		return w, h + 8, true
	case "bucket":
		return w * 1.25, h + 12, true
	case "console", "browser":
		return w, h + 16, true
	case "cloud", "bang":
		return w * 1.3, h * 1.5, true
	case "hourglass":
		return math.Max(w, h), math.Max(w, h), true
	case "bolt":
		return w, h * 2, true
	case "cross-circ":
		d := math.Max(w, h)
		return d, d, true
	case "fr-circ":
		return 14, 14, true
	}
	return 0, 0, false
}

// hcylRadius is the radius across of an end of a cylinder on its side of
// a height.
func hcylRadius(h float64) float64 {
	return math.Min(h*0.18, 12)
}

// morePath returns the path of a shape of Mermaid, and false for a shape
// that it does not know.
func morePath(kind string, w, h float64) (string, bool) {
	x, y := w/2, h/2
	var p pen
	switch kind {
	case "lin-doc":
		d := wave{-x, -y, x, y, math.Min(6, h/5)}
		return d.path() + stroke([2]float64{-x + 6, -y}, [2]float64{-x + 6, d.at(-x + 6)}), true
	case "tag-doc":
		d := wave{-x, -y, x, y, math.Min(6, h/5)}
		t := math.Min(10, h/4)
		return d.path() + stroke([2]float64{x, y - d.a - t}, [2]float64{x - 1.5*t, d.at(x - 1.5*t)}), true
	case "docs":
		o := stackOffset
		a := math.Min(6, (h-2*o)/5)
		d := wave{-x, -y + 2*o, x - 2*o, y, a}
		return sheet(-x+o, -y+o, x-o, y-a-o) + sheet(-x+2*o, -y, x, y-a-2*o) + d.path(), true
	case "st-rect":
		o := stackOffset
		return sheet(-x+o, -y+o, x-o, y-o) + sheet(-x+2*o, -y, x, y-2*o) +
			poly([2]float64{-x, -y + 2*o}, [2]float64{x - 2*o, -y + 2*o}, [2]float64{x - 2*o, y}, [2]float64{-x, y}), true
	case "brace-l":
		r := math.Min(4, y/2)
		return brace(-x, y, r, 1), true
	case "brace-r":
		r := math.Min(4, y/2)
		return brace(x, y, r, -1), true
	case "braces":
		r := math.Min(4, y/2)
		return brace(-x, y, r, 1) + brace(x, y, r, -1), true
	case "bracket":
		return stroke([2]float64{-x + 6, -y}, [2]float64{-x, -y}, [2]float64{-x, y}, [2]float64{-x + 6, y}), true
	case "card":
		c := math.Min(12, h/3)
		return poly([2]float64{-x + c, -y}, [2]float64{x, -y}, [2]float64{x, y}, [2]float64{-x, y}, [2]float64{-x, -y + c}), true
	case "notch-pent":
		c := math.Min(12, h/3)
		return poly([2]float64{-x + c, -y}, [2]float64{x - c, -y}, [2]float64{x, -y + c}, [2]float64{x, y}, [2]float64{-x, y}, [2]float64{-x, -y + c}), true
	case "lin-rect":
		return rectPath(x, y) + stroke([2]float64{-x + 6, -y}, [2]float64{-x + 6, y}), true
	case "div-rect":
		return rectPath(x, y) + stroke([2]float64{-x, -y + 8}, [2]float64{x, -y + 8}), true
	case "win-pane":
		return rectPath(x, y) + stroke([2]float64{-x, -y + 8}, [2]float64{x, -y + 8}) + stroke([2]float64{-x + 8, -y}, [2]float64{-x + 8, y}), true
	case "tag-rect":
		t := math.Min(10, h/3)
		return rectPath(x, y) + stroke([2]float64{x, y - t}, [2]float64{x - t, y}), true
	case "datastore":
		return stroke([2]float64{-x, -y}, [2]float64{x, -y}) + stroke([2]float64{-x, y}, [2]float64{x, y}), true
	case "sl-rect":
		return poly([2]float64{-x, -y + h/4}, [2]float64{x, -y}, [2]float64{x, y}, [2]float64{-x, y}), true
	case "delay":
		r := math.Min(y, x)
		return p.m(-x, -y).h(x-r).a(r, y, 1, x-r, y).h(-x).z().String(), true
	case "curv-trap":
		r, i := math.Min(y*0.6, w/4), h/4
		return p.m(-x+i, -y).h(x-r).a(r, y, 1, x-r, y).h(-x+i).l(-x, 0).z().String(), true
	case "bow-rect":
		r := h / 6
		return p.m(-x+r, -y).h(x).a(r, y, 0, x, y).h(-x+r).a(r, y, 1, -x+r, -y).z().String(), true
	case "h-cyl":
		r := hcylRadius(h)
		p.m(-x+r, -y).h(x-r).a(r, y, 1, x-r, y).h(-x+r).a(r, y, 1, -x+r, -y).z()
		p.m(x-r, -y).a(r, y, 0, x-r, y).a(r, y, 1, x-r, -y)
		return p.String(), true
	case "lin-cyl":
		ry := math.Min(h*0.12, 10)
		at := -y + 2*ry + 5
		return shapePath("cylinder", w, h) + p.m(-x, at).a(x, ry, 0, x, at).a(x, ry, 1, -x, at).String(), true
	case "paper-tape":
		k := math.Min(h*0.3, 14)
		a := 0.3 * k
		return p.m(-x, -y+a).c(-x/3, -y+a-k, x/3, -y+a+k, x, -y+a).v(y-a).c(x/3, y-a+k, -x/3, y-a-k, -x, y-a).z().String(), true
	case "bucket":
		ry, i := math.Min(h*0.1, 8), w/10
		top := -y + ry
		p.m(-x, top).a(x, ry, 1, x, top).l(x-i, y-ry).a(x-i, ry, 1, -x+i, y-ry).z()
		p.m(-x, top).a(x, ry, 0, x, top).a(x, ry, 1, -x, top)
		return p.String(), true
	case "console":
		return shapePath("rounded", w, h) +
			stroke([2]float64{-x + 8, -y + 6}, [2]float64{-x + 12, -y + 9}, [2]float64{-x + 8, -y + 12}) +
			stroke([2]float64{-x + 14, -y + 12}, [2]float64{-x + 19, -y + 12}), true
	case "browser":
		var b strings.Builder
		b.WriteString(shapePath("rounded", w, h) + stroke([2]float64{-x, -y + 12}, [2]float64{x, -y + 12}))
		for i := range 3 {
			b.WriteString(circlePath(-x+7+float64(i)*5, -y+6, 1.5))
		}
		if w > 48 {
			b.WriteString(shapeRoundRect(-x+24, -y+3, w-32, 6, 3))
		}
		return b.String(), true
	case "cloud":
		return scallops(x, y, 1), true
	case "bang":
		return scallops(x, y, 0), true
	case "hourglass":
		return poly([2]float64{-x, -y}, [2]float64{x, -y}, [2]float64{-x, y}, [2]float64{x, y}), true
	case "bolt":
		g := 7 * w / 35
		return poly([2]float64{x, -y}, [2]float64{-x, g / 2}, [2]float64{x - 2*g, g / 2}, [2]float64{-x, y},
			[2]float64{x, -g / 2}, [2]float64{-x + 2*g, -g / 2}), true
	case "cross-circ":
		c := x * math.Sqrt2 / 2
		return circlePath(0, 0, x) + stroke([2]float64{-c, -c}, [2]float64{c, c}) + stroke([2]float64{c, -c}, [2]float64{-c, c}), true
	case "fr-circ":
		// A ring and a dot. The dot is two small circles that the outline
		// fills.
		return circlePath(0, 0, x) + circlePath(0, 0, 0.7) + circlePath(0, 0, 1.9), true
	}
	return "", false
}

// rectPath returns the path of a rectangle of the half size x, y.
func rectPath(x, y float64) string {
	return poly([2]float64{-x, -y}, [2]float64{x, -y}, [2]float64{x, y}, [2]float64{-x, y})
}

// shapeRoundRect returns the path of a rectangle with round corners of a
// radius, which turns clockwise.
func shapeRoundRect(x, y, w, h, r float64) string {
	r = math.Min(r, math.Min(w, h)/2)
	var p pen
	return p.m(x+r, y).h(x+w-r).a(r, r, 1, x+w, y+r).v(y+h-r).a(r, r, 1, x+w-r, y+h).h(x+r).a(r, r, 1, x, y+h-r).v(y+r).a(r, r, 1, x+r, y).z().String()
}

// sheet returns the part of a sheet behind a stack that shows: its top
// and its right side, a band of the width stackOffset, from x0, y0 to
// x1, y1.
func sheet(x0, y0, x1, y1 float64) string {
	o := stackOffset
	return poly([2]float64{x0, y0 + o}, [2]float64{x0, y0}, [2]float64{x1, y0}, [2]float64{x1, y1}, [2]float64{x1 - o, y1}, [2]float64{x1 - o, y0 + o})
}

// scallops returns the path of a cloud, with bumps out of an ellipse of
// the half size x, y when out is 1, or of a bang, with bumps into it when
// out is 0. The bumps are of the same length along the ellipse.
func scallops(x, y float64, out int) string {
	// The bumps reach out of the ellipse by a quarter of their length.
	n := int(math.Round(math.Pi * (x + y) / 22))
	n = min(max(n, 8), 28)
	step := math.Pi * (x + y) / float64(n)
	bump := 0.27 * step
	rx, ry := x-bump, y-bump
	if out == 0 {
		rx, ry = x, y
	}
	// Walk the ellipse in fine steps, and put a corner each step of the
	// length.
	const fine = 720
	var lens []float64
	var pts [][2]float64
	total := 0.0
	prev := [2]float64{rx, 0}
	for i := range fine + 1 {
		a := 2 * math.Pi * float64(i) / fine
		q := [2]float64{rx * math.Cos(a), ry * math.Sin(a)}
		total += math.Hypot(q[0]-prev[0], q[1]-prev[1])
		lens = append(lens, total)
		pts = append(pts, q)
		prev = q
	}
	var corners [][2]float64
	j := 0
	for k := range n {
		want := total * float64(k) / float64(n)
		for j < fine && lens[j] < want {
			j++
		}
		corners = append(corners, pts[j])
	}
	var p pen
	p.m(corners[0][0], corners[0][1])
	for k := range corners {
		q := corners[(k+1)%n]
		c := corners[k]
		r := 0.6 * math.Hypot(q[0]-c[0], q[1]-c[1])
		p.a(r, r, out, q[0], q[1])
	}
	return p.z().String()
}
