// Package shape draws the geometry of the items of a scene as the path data
// of SVG: the curves of lines and areas, the symbols, the arcs, the trails
// and the rects with round corners. The SVG writer writes the paths, and the
// direct renderer parses them, so that both draw the same shapes.
package shape

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// Curve returns the path of a line through points, with an interpolation of
// Vega, and false for an interpolation that it does not know. The curves follow the documented curves of d3-shape (D6). A path that
// does not move first continues the path before it with a line. tension is
// the tension of the item, which Vega gives to cardinal as its tension, to
// bundle as its beta and to catmull-rom as its alpha. Zero means the default.
func Curve(interpolate string, pts [][2]float64, move bool, tension float64) (string, bool) {
	if len(pts) == 0 {
		return "", true
	}
	var b strings.Builder
	if move {
		b.WriteString("M" + pt(pts[0][0], pts[0][1]))
	} else {
		b.WriteString("L" + pt(pts[0][0], pts[0][1]))
	}
	switch interpolate {
	case "", "linear":
		lines(&b, pts)
	case "linear-closed":
		lines(&b, pts)
		b.WriteString("Z")
	case "step", "step-before", "step-after":
		step(&b, interpolate, pts)
	case "monotone":
		monotone(&b, pts)
	case "basis":
		basis(&b, pts)
	case "bundle":
		beta := 0.85
		if tension != 0 {
			beta = tension
		}
		basis(&b, bundle(pts, beta))
	case "cardinal":
		cardinal(&b, pts, tension)
	case "catmull-rom":
		alpha := 0.5
		if tension != 0 {
			alpha = tension
		}
		catmullRom(&b, pts, alpha)
	case "natural":
		natural(&b, pts)
	default:
		return "", false
	}
	return b.String(), true
}

// lines draws straight lines through the points after the first.
func lines(b *strings.Builder, pts [][2]float64) {
	for _, p := range pts[1:] {
		b.WriteString("L" + pt(p[0], p[1]))
	}
}

// cubic writes one cubic Bézier segment.
func cubic(b *strings.Builder, x1, y1, x2, y2, x, y float64) {
	b.WriteString("C" + pt(x1, y1) + " " + pt(x2, y2) + " " + pt(x, y))
}

// step draws a step curve: the step falls at the middle between two points,
// at the second point for step-before, or at the first for step-after.
func step(b *strings.Builder, kind string, pts [][2]float64) {
	t := 0.5
	switch kind {
	case "step-before":
		t = 0
	case "step-after":
		t = 1
	}
	for i := 1; i < len(pts); i++ {
		x0, y0, x1, y1 := pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1]
		switch t {
		case 0:
			b.WriteString("L" + pt(x0, y1))
		case 1:
			b.WriteString("L" + pt(x1, y0))
		default:
			xm := x0 + (x1-x0)*t
			b.WriteString("L" + pt(xm, y0) + "L" + pt(xm, y1))
		}
		b.WriteString("L" + pt(x1, y1))
	}
}

// monotone draws a cubic curve that keeps the monotonicity of y in x, as
// described by Steffen and as monotoneX of d3-shape draws it.
func monotone(b *strings.Builder, pts [][2]float64) {
	n := len(pts)
	if n < 3 {
		lines(b, pts)
		return
	}
	sign := func(v float64) float64 {
		if v < 0 {
			return -1
		}
		return 1
	}
	slope := func(h, dy float64) float64 {
		if h == 0 {
			return 0
		}
		return dy / h
	}
	t := make([]float64, n)
	for i := 1; i < n-1; i++ {
		h0, h1 := pts[i][0]-pts[i-1][0], pts[i+1][0]-pts[i][0]
		s0, s1 := slope(h0, pts[i][1]-pts[i-1][1]), slope(h1, pts[i+1][1]-pts[i][1])
		p := 0.0
		if h0+h1 != 0 {
			p = (s0*h1 + s1*h0) / (h0 + h1)
		}
		v := (sign(s0) + sign(s1)) * math.Min(math.Abs(s0), math.Min(math.Abs(s1), 0.5*math.Abs(p)))
		if math.IsNaN(v) {
			v = 0
		}
		t[i] = v
	}
	end := func(x0, y0, x1, y1, tt float64) float64 {
		h := x1 - x0
		if h == 0 {
			return tt
		}
		return (3*(y1-y0)/h - tt) / 2
	}
	t[0] = end(pts[0][0], pts[0][1], pts[1][0], pts[1][1], t[1])
	t[n-1] = end(pts[n-2][0], pts[n-2][1], pts[n-1][0], pts[n-1][1], t[n-2])
	for i := 1; i < n; i++ {
		x0, y0, x1, y1 := pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1]
		dx := (x1 - x0) / 3
		cubic(b, x0+dx, y0+dx*t[i-1], x1-dx, y1-dx*t[i], x1, y1)
	}
}

// basis draws a cubic B-spline through the points, which starts at the
// first point and ends at the last, as curveBasis of d3-shape draws it.
func basis(b *strings.Builder, pts [][2]float64) {
	n := len(pts)
	if n < 3 {
		lines(b, pts)
		return
	}
	x0, y0, x1, y1 := pts[0][0], pts[0][1], pts[1][0], pts[1][1]
	b.WriteString("L" + pt((5*x0+x1)/6, (5*y0+y1)/6))
	bezier := func(x, y float64) {
		cubic(b, (2*x0+x1)/3, (2*y0+y1)/3, (x0+2*x1)/3, (y0+2*y1)/3, (x0+4*x1+x)/6, (y0+4*y1+y)/6)
		x0, y0, x1, y1 = x1, y1, x, y
	}
	for _, p := range pts[2:] {
		bezier(p[0], p[1])
	}
	bezier(x1, y1)
	b.WriteString("L" + pt(x1, y1))
}

// bundle straightens the points toward the line from the first to the last
// by beta, as curveBundle of d3-shape does before it draws a B-spline.
func bundle(pts [][2]float64, beta float64) [][2]float64 {
	n := len(pts)
	if n < 3 {
		return pts
	}
	x0, y0 := pts[0][0], pts[0][1]
	dx, dy := pts[n-1][0]-x0, pts[n-1][1]-y0
	out := make([][2]float64, n)
	for i, p := range pts {
		t := float64(i) / float64(n-1)
		out[i] = [2]float64{beta*p[0] + (1-beta)*(x0+t*dx), beta*p[1] + (1-beta)*(y0+t*dy)}
	}
	return out
}

// padded returns the point at i, with the first and the last point repeated
// beyond the ends.
func padded(pts [][2]float64, i int) [2]float64 {
	return pts[max(0, min(len(pts)-1, i))]
}

// cardinal draws a cardinal spline with a tension from 0 to 1, as
// curveCardinal of d3-shape draws it.
func cardinal(b *strings.Builder, pts [][2]float64, tension float64) {
	if len(pts) < 3 {
		lines(b, pts)
		return
	}
	k := (1 - tension) / 6
	for i := 0; i+1 < len(pts); i++ {
		// At an end, d3-shape puts the control point on the end point, which
		// is what the point beyond the end gives when it is the other point
		// of the segment.
		p0, p1, p2, p3 := pts[i+1], pts[i], pts[i+1], pts[i]
		if i > 0 {
			p0 = pts[i-1]
		}
		if i+2 < len(pts) {
			p3 = pts[i+2]
		}
		cubic(b, p1[0]+k*(p2[0]-p0[0]), p1[1]+k*(p2[1]-p0[1]), p2[0]+k*(p1[0]-p3[0]), p2[1]+k*(p1[1]-p3[1]), p2[0], p2[1])
	}
}

// catmullRom draws a Catmull-Rom spline with a parameter alpha, which is the
// centripetal spline at one half, as curveCatmullRom of d3-shape draws it.
func catmullRom(b *strings.Builder, pts [][2]float64, alpha float64) {
	if len(pts) < 3 {
		lines(b, pts)
		return
	}
	const eps = 1e-12
	dist := func(p, q [2]float64) float64 { return math.Hypot(p[0]-q[0], p[1]-q[1]) }
	for i := 0; i+1 < len(pts); i++ {
		p0, p1, p2, p3 := padded(pts, i-1), pts[i], pts[i+1], padded(pts, i+2)
		l01a, l12a, l23a := math.Pow(dist(p0, p1), alpha), math.Pow(dist(p1, p2), alpha), math.Pow(dist(p2, p3), alpha)
		l01, l12, l23 := l01a*l01a, l12a*l12a, l23a*l23a
		c1, c2 := p1, p2
		if l01a > eps {
			a, n := 2*l01+3*l01a*l12a+l12, 3*l01a*(l01a+l12a)
			c1 = [2]float64{(p1[0]*a - p0[0]*l12 + p2[0]*l01) / n, (p1[1]*a - p0[1]*l12 + p2[1]*l01) / n}
		}
		if l23a > eps {
			bb, m := 2*l23+3*l23a*l12a+l12, 3*l23a*(l23a+l12a)
			c2 = [2]float64{(p2[0]*bb + p1[0]*l23 - p3[0]*l12) / m, (p2[1]*bb + p1[1]*l23 - p3[1]*l12) / m}
		}
		cubic(b, c1[0], c1[1], c2[0], c2[1], p2[0], p2[1])
	}
}

// natural draws a natural cubic spline, whose second derivative is zero at
// both ends, as curveNatural of d3-shape draws it.
func natural(b *strings.Builder, pts [][2]float64) {
	n := len(pts) - 1
	if n < 2 {
		lines(b, pts)
		return
	}
	xs, ys := make([]float64, n+1), make([]float64, n+1)
	for i, p := range pts {
		xs[i], ys[i] = p[0], p[1]
	}
	ax, bx := naturalControls(xs)
	ay, by := naturalControls(ys)
	for i := range n {
		cubic(b, ax[i], ay[i], bx[i], by[i], xs[i+1], ys[i+1])
	}
}

// naturalControls returns the two control values of each segment of a
// natural cubic spline through the values of x.
func naturalControls(x []float64) (a, b []float64) {
	n := len(x) - 1
	a, b, r := make([]float64, n), make([]float64, n), make([]float64, n)
	a[0], b[0], r[0] = 0, 2, x[0]+2*x[1]
	for i := 1; i < n-1; i++ {
		a[i], b[i], r[i] = 1, 4, 4*x[i]+2*x[i+1]
	}
	a[n-1], b[n-1], r[n-1] = 2, 7, 8*x[n-1]+x[n]
	for i := 1; i < n; i++ {
		m := a[i] / b[i-1]
		b[i] -= m
		r[i] -= m * r[i-1]
	}
	a[n-1] = r[n-1] / b[n-1]
	for i := n - 2; i >= 0; i-- {
		a[i] = (r[i] - a[i+1]) / b[i]
	}
	b[n-1] = (x[n] + a[n-1]) / 2
	for i := range n - 1 {
		b[i] = 2*x[i+1] - a[i+1]
	}
	return a, b
}

// Symbol returns the path of a symbol of a size, which is the area of the
// symbol in square pixels, with the shapes of Vega. A shape that is not a
// name is the path of an SVG, drawn in a square from -1 to 1 and scaled to
// the size, as Vega draws a custom shape. It returns false for a shape that
// is neither.
func Symbol(name string, size float64) (string, bool) {
	r := math.Sqrt(size) / 2
	h := math.Sqrt(3) / 2 * r
	o := h - r*math.Tan(math.Pi/6)
	switch name {
	case "", "circle":
		return "M" + pt(r, 0) + "A" + num(r) + "," + num(r) + " 0 1,1 " + pt(-r, 0) +
			"A" + num(r) + "," + num(r) + " 0 1,1 " + pt(r, 0) + "Z", true
	case "square":
		return "M" + pt(-r, -r) + "H" + num(r) + "V" + num(r) + "H" + num(-r) + "Z", true
	case "diamond":
		return "M" + pt(-r, 0) + "L" + pt(0, -r) + "L" + pt(r, 0) + "L" + pt(0, r) + "Z", true
	case "cross":
		s := r / 2.5
		return "M" + pt(-r, -s) + "L" + pt(-r, s) + "L" + pt(-s, s) + "L" + pt(-s, r) +
			"L" + pt(s, r) + "L" + pt(s, s) + "L" + pt(r, s) + "L" + pt(r, -s) +
			"L" + pt(s, -s) + "L" + pt(s, -r) + "L" + pt(-s, -r) + "L" + pt(-s, -s) + "Z", true
	case "arrow":
		s, t, v := r/7, r/2.5, r/8
		return "M" + pt(-s, r) + "L" + pt(s, r) + "L" + pt(s, -v) + "L" + pt(t, -v) +
			"L" + pt(0, -r) + "L" + pt(-t, -v) + "L" + pt(-s, -v) + "Z", true
	case "wedge":
		w := r / 4
		return "M" + pt(0, -h-o) + "L" + pt(-w, h-o) + "L" + pt(w, h-o) + "Z", true
	case "triangle":
		return "M" + pt(0, -h-o) + "L" + pt(-r, h-o) + "L" + pt(r, h-o) + "Z", true
	case "triangle-up":
		return "M" + pt(0, -h) + "L" + pt(-r, h) + "L" + pt(r, h) + "Z", true
	case "triangle-down":
		return "M" + pt(0, h) + "L" + pt(-r, -h) + "L" + pt(r, -h) + "Z", true
	case "triangle-right":
		return "M" + pt(h, 0) + "L" + pt(-h, -r) + "L" + pt(-h, r) + "Z", true
	case "triangle-left":
		return "M" + pt(-h, 0) + "L" + pt(h, -r) + "L" + pt(h, r) + "Z", true
	case "stroke":
		return "M" + pt(-r, 0) + "L" + pt(r, 0), true
	}
	d, err := scalePath(name, r)
	if err != nil {
		return "", false
	}
	return d, true
}

// scalePath multiplies each coordinate of an SVG path by k. The flags and the
// rotation of an arc do not scale.
func scalePath(d string, k float64) (string, error) {
	errPath := errors.New("bad path")
	var out strings.Builder
	var cmd rune
	arg := 0
	i := 0
	for i < len(d) {
		c := rune(d[i])
		switch {
		case unicode.IsSpace(c) || c == ',':
			i++
		case strings.ContainsRune("MmLlHhVvCcSsQqTtAaZz", c):
			cmd, arg = c, 0
			out.WriteRune(c)
			i++
		default:
			j := i + 1
			for j < len(d) && (unicode.IsDigit(rune(d[j])) || d[j] == '.' || d[j] == 'e' || d[j] == 'E' ||
				((d[j] == '-' || d[j] == '+') && (d[j-1] == 'e' || d[j-1] == 'E'))) {
				j++
			}
			v, err := strconv.ParseFloat(d[i:j], 64)
			if err != nil || cmd == 0 {
				return "", errPath
			}
			if arg > 0 {
				out.WriteByte(' ')
			}
			pos := arg % 7
			if (cmd == 'A' || cmd == 'a') && pos >= 2 && pos <= 4 {
				out.WriteString(num(v))
			} else {
				out.WriteString(num(v * k))
			}
			arg++
			i = j
		}
	}
	return out.String(), nil
}

// Trail returns the outline of a trail: a line through points whose width is
// the size of each point, with round ends, as the trail of Vega draws it.
func Trail(pts [][3]float64) string {
	var b strings.Builder
	for i := 1; i < len(pts); i++ {
		x1, y1, r1 := pts[i-1][0], pts[i-1][1], pts[i-1][2]/2
		x2, y2, r2 := pts[i][0], pts[i][1], pts[i][2]/2
		ux, uy := y1-y2, x2-x1
		d := math.Hypot(ux, uy)
		if d == 0 {
			b.WriteString(circle(x2, y2, r2))
			continue
		}
		ux, uy = ux/d, uy/d
		b.WriteString("M" + pt(x1-ux*r1, y1-uy*r1) + "L" + pt(x2-ux*r2, y2-uy*r2) +
			"A" + num(r2) + "," + num(r2) + " 0 0,1 " + pt(x2+ux*r2, y2+uy*r2) +
			"L" + pt(x1+ux*r1, y1+uy*r1) +
			"A" + num(r1) + "," + num(r1) + " 0 0,1 " + pt(x1-ux*r1, y1-uy*r1) + "Z")
	}
	return b.String()
}

// circle returns the path of a full circle.
func circle(x, y, r float64) string {
	return "M" + pt(x+r, y) + "A" + num(r) + "," + num(r) + " 0 1,1 " + pt(x-r, y) +
		"A" + num(r) + "," + num(r) + " 0 1,1 " + pt(x+r, y) + "Z"
}

// pathBuilder writes a path and keeps its current point, so that an arc
// draws a line to its start only when the path is elsewhere, as the arc of
// d3-path does.
type pathBuilder struct {
	b     strings.Builder
	x, y  float64
	moved bool
}

// moveTo starts a new part of the path.
func (p *pathBuilder) moveTo(x, y float64) {
	p.b.WriteString("M" + pt(x, y))
	p.x, p.y, p.moved = x, y, true
}

// lineTo draws a line.
func (p *pathBuilder) lineTo(x, y float64) {
	p.b.WriteString("L" + pt(x, y))
	p.x, p.y = x, y
}

// arc draws a circular arc around (cx, cy) from angle a0 to angle a1, in
// radians, anticlockwise when ccw is true, as the arc of a canvas draws it.
func (p *pathBuilder) arc(cx, cy, r, a0, a1 float64, ccw bool) {
	x0, y0 := cx+r*math.Cos(a0), cy+r*math.Sin(a0)
	switch {
	case !p.moved:
		p.moveTo(x0, y0)
	case math.Abs(p.x-x0) > 1e-6 || math.Abs(p.y-y0) > 1e-6:
		p.lineTo(x0, y0)
	}
	if r <= 0 {
		return
	}
	da := a1 - a0
	if ccw {
		da = a0 - a1
	}
	if da < 0 {
		da = math.Mod(da, 2*math.Pi) + 2*math.Pi
	}
	sweep := "1"
	if ccw {
		sweep = "0"
	}
	if da > 2*math.Pi-1e-6 {
		xm, ym := cx-r*math.Cos(a0), cy-r*math.Sin(a0)
		p.b.WriteString("A" + num(r) + "," + num(r) + " 0 1," + sweep + " " + pt(xm, ym) +
			"A" + num(r) + "," + num(r) + " 0 1," + sweep + " " + pt(x0, y0))
		p.x, p.y = x0, y0
		return
	}
	large := "0"
	if da >= math.Pi {
		large = "1"
	}
	x1, y1 := cx+r*math.Cos(a1), cy+r*math.Sin(a1)
	p.b.WriteString("A" + num(r) + "," + num(r) + " 0 " + large + "," + sweep + " " + pt(x1, y1))
	p.x, p.y = x1, y1
}

// Arc returns the path of a slice of a ring, centered on the origin, as
// the arc of d3-shape draws it. Angles are in radians, clockwise from twelve
// o'clock. The pad angle opens a gap between two slices, and the corner
// radius rounds the four corners of the slice.
func Arc(r0, r1, start, end, pad, corner float64) string {
	const eps = 1e-12
	if r1 < r0 {
		r0, r1 = r1, r0
	}
	var p pathBuilder
	if r1 <= eps {
		p.moveTo(0, 0)
		return p.b.String() + "Z"
	}
	a0, a1 := start-math.Pi/2, end-math.Pi/2
	da := math.Abs(a1 - a0)
	cw := a1 > a0
	if da > 2*math.Pi-1e-6 {
		p.moveTo(r1*math.Cos(a0), r1*math.Sin(a0))
		p.arc(0, 0, r1, a0, a1, !cw)
		if r0 > eps {
			p.moveTo(r0*math.Cos(a1), r0*math.Sin(a1))
			p.arc(0, 0, r0, a1, a0, cw)
		}
		return p.b.String() + "Z"
	}
	a01, a11, a00, a10 := a0, a1, a0, a1
	da0, da1 := da, da
	rc := math.Min(math.Abs(r1-r0)/2, corner)
	rc0, rc1 := rc, rc
	if ap := pad / 2; ap > eps {
		rp := math.Sqrt(r0*r0 + r1*r1)
		dir := 1.0
		if !cw {
			dir = -1
		}
		p0 := math.Asin(math.Min(1, rp/r0*math.Sin(ap)))
		if r0 <= eps {
			p0 = 0
		}
		p1 := math.Asin(math.Min(1, rp/r1*math.Sin(ap)))
		if da0 -= p0 * 2; da0 > eps {
			a00, a10 = a00+p0*dir, a10-p0*dir
		} else {
			da0, a00, a10 = 0, (a0+a1)/2, (a0+a1)/2
		}
		if da1 -= p1 * 2; da1 > eps {
			a01, a11 = a01+p1*dir, a11-p1*dir
		} else {
			da1, a01, a11 = 0, (a0+a1)/2, (a0+a1)/2
		}
	}
	x01, y01 := r1*math.Cos(a01), r1*math.Sin(a01)
	x10, y10 := r0*math.Cos(a10), r0*math.Sin(a10)
	x11, y11 := r1*math.Cos(a11), r1*math.Sin(a11)
	x00, y00 := r0*math.Cos(a00), r0*math.Sin(a00)
	if rc > eps && da < math.Pi {
		if ox, oy, ok := intersect(x01, y01, x00, y00, x11, y11, x10, y10); ok {
			ax, ay, bx, by := x01-ox, y01-oy, x11-ox, y11-oy
			kc := 1 / math.Sin(math.Acos((ax*bx+ay*by)/(math.Hypot(ax, ay)*math.Hypot(bx, by)))/2)
			lc := math.Hypot(ox, oy)
			rc0 = math.Min(rc, (r0-lc)/(kc-1))
			rc1 = math.Min(rc, (r1-lc)/(kc+1))
		} else {
			rc0, rc1 = 0, 0
		}
	}
	switch {
	case da1 <= eps:
		p.moveTo(x01, y01)
	case rc1 > eps:
		t0 := cornerTangents(x00, y00, x01, y01, r1, rc1, cw)
		t1 := cornerTangents(x11, y11, x10, y10, r1, rc1, cw)
		p.moveTo(t0.cx+t0.x01, t0.cy+t0.y01)
		if rc1 < rc {
			p.arc(t0.cx, t0.cy, rc1, math.Atan2(t0.y01, t0.x01), math.Atan2(t1.y01, t1.x01), !cw)
		} else {
			p.arc(t0.cx, t0.cy, rc1, math.Atan2(t0.y01, t0.x01), math.Atan2(t0.y11, t0.x11), !cw)
			p.arc(0, 0, r1, math.Atan2(t0.cy+t0.y11, t0.cx+t0.x11), math.Atan2(t1.cy+t1.y11, t1.cx+t1.x11), !cw)
			p.arc(t1.cx, t1.cy, rc1, math.Atan2(t1.y11, t1.x11), math.Atan2(t1.y01, t1.x01), !cw)
		}
	default:
		p.moveTo(x01, y01)
		p.arc(0, 0, r1, a01, a11, !cw)
	}
	switch {
	case r0 <= eps || da0 <= eps:
		p.lineTo(x10, y10)
	case rc0 > eps:
		t0 := cornerTangents(x10, y10, x11, y11, r0, -rc0, cw)
		t1 := cornerTangents(x01, y01, x00, y00, r0, -rc0, cw)
		p.lineTo(t0.cx+t0.x01, t0.cy+t0.y01)
		if rc0 < rc {
			p.arc(t0.cx, t0.cy, rc0, math.Atan2(t0.y01, t0.x01), math.Atan2(t1.y01, t1.x01), !cw)
		} else {
			p.arc(t0.cx, t0.cy, rc0, math.Atan2(t0.y01, t0.x01), math.Atan2(t0.y11, t0.x11), !cw)
			p.arc(0, 0, r0, math.Atan2(t0.cy+t0.y11, t0.cx+t0.x11), math.Atan2(t1.cy+t1.y11, t1.cx+t1.x11), cw)
			p.arc(t1.cx, t1.cy, rc0, math.Atan2(t1.y11, t1.x11), math.Atan2(t1.y01, t1.x01), !cw)
		}
	default:
		p.arc(0, 0, r0, a10, a00, cw)
	}
	return p.b.String() + "Z"
}

// tangents is the circle of a round corner of an arc and its two points of
// contact.
type tangents struct {
	cx, cy, x01, y01, x11, y11 float64
}

// cornerTangents returns the circle of radius rc that touches the edge from
// (x0, y0) to (x1, y1) and the circle of radius r1, as d3-shape finds it.
func cornerTangents(x0, y0, x1, y1, r1, rc float64, cw bool) tangents {
	x01, y01 := x0-x1, y0-y1
	lo := rc / math.Hypot(x01, y01)
	if !cw {
		lo = -lo
	}
	ox, oy := lo*y01, -lo*x01
	x11, y11 := x0+ox, y0+oy
	x10, y10 := x1+ox, y1+oy
	x00, y00 := (x11+x10)/2, (y11+y10)/2
	dx, dy := x10-x11, y10-y11
	d2 := dx*dx + dy*dy
	r := r1 - rc
	dd := x11*y10 - x10*y11
	sign := 1.0
	if dy < 0 {
		sign = -1
	}
	d := sign * math.Sqrt(math.Max(0, r*r*d2-dd*dd))
	cx0, cy0 := (dd*dy-dx*d)/d2, (-dd*dx-dy*d)/d2
	cx1, cy1 := (dd*dy+dx*d)/d2, (-dd*dx+dy*d)/d2
	if dx0, dy0, dx1, dy1 := cx0-x00, cy0-y00, cx1-x00, cy1-y00; dx0*dx0+dy0*dy0 > dx1*dx1+dy1*dy1 {
		cx0, cy0 = cx1, cy1
	}
	return tangents{cx: cx0, cy: cy0, x01: -ox, y01: -oy, x11: cx0 * (r1/r - 1), y11: cy0 * (r1/r - 1)}
}

// intersect returns the point where the line through the first two points
// meets the line through the last two.
func intersect(x0, y0, x1, y1, x2, y2, x3, y3 float64) (x, y float64, ok bool) {
	x10, y10, x32, y32 := x1-x0, y1-y0, x3-x2, y3-y2
	t := y32*x10 - x32*y10
	if t*t < 1e-12 {
		return 0, 0, false
	}
	t = (x32*(y0-y2) - y32*(x0-x2)) / t
	return x0 + t*x10, y0 + t*y10, true
}

// num writes a number with at most three decimals and no trailing zeros, as
// the SVG writer writes its numbers.
func num(v float64) string {
	v = math.Round(v*1000) / 1000
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// pt writes a point as x,y.
func pt(x, y float64) string {
	return num(x) + "," + num(y)
}
