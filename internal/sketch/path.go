package sketch

import (
	"math"
	"strconv"
	"strings"

	"github.com/xo/zuzu/internal/rough"
	"github.com/xo/zuzu/internal/shape"
)

// pathData writes the operations of a set of rough.js as path data, with
// at most two decimals, so that the SVG stays short and both renderers
// read the same numbers.
func pathData(ops []rough.Op) string {
	var b strings.Builder
	for _, op := range ops {
		var cmd byte
		switch op.Op {
		case rough.OpMove:
			cmd = 'M'
		case rough.OpLineTo:
			cmd = 'L'
		case rough.OpBCurveTo:
			cmd = 'C'
		case rough.OpQCurveTo:
			cmd = 'Q'
		default:
			continue
		}
		b.WriteByte(cmd)
		for i, v := range op.Data {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(num(v))
		}
	}
	return b.String()
}

// num writes a number with at most two decimals and no trailing zeros.
func num(v float64) string {
	v = math.Round(v*100) / 100
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// absolute returns path data in absolute moves, lines, cubic curves and
// closes, which rough.js reads. An arc and a quadratic curve become cubic
// curves, as shape.Parse gives them.
func absolute(d string) string {
	var w writer
	_ = shape.Parse(d, &w)
	return w.b.String()
}

// writer is a sink of shape.Parse that writes absolute path data.
type writer struct {
	b      strings.Builder
	x0, y0 float64
}

func (w *writer) MoveTo(x, y float64) {
	w.point('M', x, y)
	w.x0, w.y0 = x, y
}

func (w *writer) LineTo(x, y float64) {
	w.point('L', x, y)
	w.x0, w.y0 = x, y
}

func (w *writer) QuadTo(x1, y1, x, y float64) {
	// The cubic curve with the control points two thirds of the way to
	// the control point of the quadratic curve is the same curve.
	w.point('C', w.x0+2.0/3*(x1-w.x0), w.y0+2.0/3*(y1-w.y0), x+2.0/3*(x1-x), y+2.0/3*(y1-y), x, y)
	w.x0, w.y0 = x, y
}

func (w *writer) CubicTo(x1, y1, x2, y2, x, y float64) {
	w.point('C', x1, y1, x2, y2, x, y)
	w.x0, w.y0 = x, y
}

func (w *writer) Close() {
	w.b.WriteByte('Z')
}

func (w *writer) point(cmd byte, pts ...float64) {
	w.b.WriteByte(cmd)
	for i, v := range pts {
		if i > 0 {
			w.b.WriteByte(' ')
		}
		w.b.WriteString(strconv.FormatFloat(math.Round(v*1000)/1000, 'f', -1, 64))
	}
}

// polygon returns the points of path data as one polygon, for a hachure.
// Each curve becomes lines. The subpaths join through bridges that go out
// and come back along the same line, which add no area to a fill of the
// rule evenodd, as the hachure of rough.js reads a polygon. So a hole,
// such as the inside of a ring, stays empty.
func polygon(d string) []rough.Point {
	var f flattener
	_ = shape.Parse(d, &f)
	f.end()
	var out []rough.Point
	for _, ring := range nonzero(f.rings) {
		if len(out) > 0 {
			// Close the ring before, and go to the start of this one.
			out = append(out, out[0])
		}
		out = append(out, ring...)
		if len(out) > len(ring) {
			// Come back to the start of this ring, then to the first
			// ring, along the bridge.
			out = append(out, ring[0])
		}
	}
	return out
}

// rings returns the number of rings of path data with at least three
// points, the parts of its fill.
func rings(d string) int {
	var f flattener
	_ = shape.Parse(d, &f)
	f.end()
	n := 0
	for _, ring := range f.rings {
		if len(ring) >= 3 {
			n++
		}
	}
	return n
}

// nonzero returns the rings of at least three points that the fill of
// the rule nonzero, the rule of SVG, needs, for the hachure of rough.js,
// which reads its rings with the rule evenodd. A ring stays when the fill
// changes across it: when the winding number just outside it, the sum of
// the directions of the rings that hold it, is zero on one side of it and
// not on the other. So the inner circle of a double circle, inside a ring
// that runs the same way, adds no area and goes, and the hachure does not
// leave it empty. A ring inside a ring that runs the other way stays a
// hole, and the dot of a bullseye, inside that hole, stays filled.
func nonzero(rings [][]rough.Point) [][]rough.Point {
	var keep [][]rough.Point
	for _, r := range rings {
		if len(r) >= 3 {
			keep = append(keep, r)
		}
	}
	dir := func(r []rough.Point) int {
		if signedArea(r) > 0 {
			return 1
		}
		return -1
	}
	var out [][]rough.Point
	for i, r := range keep {
		outside := 0
		for j, o := range keep {
			if i != j && math.Abs(signedArea(o)) > math.Abs(signedArea(r)) && inside(r[0], o) {
				outside += dir(o)
			}
		}
		if (outside == 0) != (outside+dir(r) == 0) {
			out = append(out, r)
		}
	}
	return out
}

// signedArea returns the area of a ring, positive when it runs one way
// and negative when it runs the other.
func signedArea(r []rough.Point) float64 {
	a := 0.0
	for i := range r {
		p, q := r[i], r[(i+1)%len(r)]
		a += p[0]*q[1] - q[0]*p[1]
	}
	return a / 2
}

// inside reports whether a point is inside a ring, by the rule evenodd.
func inside(p rough.Point, r []rough.Point) bool {
	in := false
	for i, j := 0, len(r)-1; i < len(r); j, i = i, i+1 {
		a, b := r[i], r[j]
		if (a[1] > p[1]) != (b[1] > p[1]) && p[0] < (b[0]-a[0])*(p[1]-a[1])/(b[1]-a[1])+a[0] {
			in = !in
		}
	}
	return in
}

// flattener is a sink of shape.Parse that turns path data into rings of
// points.
type flattener struct {
	rings  [][]rough.Point
	ring   []rough.Point
	x0, y0 float64
}

func (f *flattener) MoveTo(x, y float64) {
	f.end()
	f.add(x, y)
}

func (f *flattener) LineTo(x, y float64) {
	f.add(x, y)
}

func (f *flattener) QuadTo(x1, y1, x, y float64) {
	f.CubicTo(f.x0+2.0/3*(x1-f.x0), f.y0+2.0/3*(y1-f.y0), x+2.0/3*(x1-x), y+2.0/3*(y1-y), x, y)
}

func (f *flattener) CubicTo(x1, y1, x2, y2, x, y float64) {
	x0, y0 := f.x0, f.y0
	// The length of the hull of the control points bounds the length of
	// the curve. A step of about 3 pixels draws it smooth enough for a
	// hachure.
	hull := math.Hypot(x1-x0, y1-y0) + math.Hypot(x2-x1, y2-y1) + math.Hypot(x-x2, y-y2)
	n := int(math.Min(24, math.Max(2, math.Ceil(hull/3))))
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		u := 1 - t
		a, b, c, e := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
		f.add(a*x0+b*x1+c*x2+e*x, a*y0+b*y1+c*y2+e*y)
	}
}

func (f *flattener) Close() {
	if len(f.ring) > 0 {
		x, y := f.ring[0][0], f.ring[0][1]
		f.end()
		f.x0, f.y0 = x, y
	}
}

// end ends the ring that is open.
func (f *flattener) end() {
	if len(f.ring) > 0 {
		f.rings = append(f.rings, f.ring)
	}
	f.ring = nil
}

func (f *flattener) add(x, y float64) {
	f.ring = append(f.ring, rough.Point{x, y})
	f.x0, f.y0 = x, y
}
