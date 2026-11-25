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
// Each curve becomes lines, and a ring of fewer than three points goes. The
// rings join through bridges that go out and come back along the same
// line. A bridge adds no area to a fill by the rule evenodd, and does not
// change the winding number of the rule nonzero, which the hachure of a
// path follows. So a hole, such as the inside of a ring that runs the
// other way, stays empty, and the overlap of two rings that run the same
// way is filled.
func polygon(d string) []rough.Point {
	var f flattener
	_ = shape.Parse(d, &f)
	f.end()
	var out []rough.Point
	for _, ring := range f.rings {
		if len(ring) < 3 {
			continue
		}
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
