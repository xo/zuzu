package svg

import (
	"fmt"
	"math"
	"strings"
)

// curve returns the path of a line through points, with the interpolation of
// Vega. The curves follow the documented curves of d3-shape (D6). A path
// that does not move first continues the path before it with a line.
func curve(interpolate string, pts [][2]float64, move bool) (string, error) {
	if len(pts) == 0 {
		return "", nil
	}
	var b strings.Builder
	if move {
		b.WriteString("M" + pt(pts[0][0], pts[0][1]))
	} else {
		b.WriteString("L" + pt(pts[0][0], pts[0][1]))
	}
	switch interpolate {
	case "", "linear":
		for _, p := range pts[1:] {
			b.WriteString("L" + pt(p[0], p[1]))
		}
	case "linear-closed":
		for _, p := range pts[1:] {
			b.WriteString("L" + pt(p[0], p[1]))
		}
		b.WriteString("Z")
	case "step", "step-before", "step-after":
		step(&b, interpolate, pts)
	case "monotone":
		monotone(&b, pts)
	case "basis":
		basis(&b, pts)
	default:
		return "", fmt.Errorf("drawing a curve %q: %w", interpolate, ErrUnsupportedCurve)
	}
	return b.String(), nil
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
		for _, p := range pts[1:] {
			b.WriteString("L" + pt(p[0], p[1]))
		}
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
		b.WriteString("C" + pt(x0+dx, y0+dx*t[i-1]) + " " + pt(x1-dx, y1-dx*t[i]) + " " + pt(x1, y1))
	}
}

// basis draws a cubic B-spline through the points, which starts at the
// first point and ends at the last, as curveBasis of d3-shape draws it.
func basis(b *strings.Builder, pts [][2]float64) {
	n := len(pts)
	if n < 3 {
		for _, p := range pts[1:] {
			b.WriteString("L" + pt(p[0], p[1]))
		}
		return
	}
	x0, y0, x1, y1 := pts[0][0], pts[0][1], pts[1][0], pts[1][1]
	b.WriteString("L" + pt((5*x0+x1)/6, (5*y0+y1)/6))
	bezier := func(x, y float64) {
		b.WriteString("C" + pt((2*x0+x1)/3, (2*y0+y1)/3) + " " + pt((x0+2*x1)/3, (y0+2*y1)/3) + " " +
			pt((x0+4*x1+x)/6, (y0+4*y1+y)/6))
		x0, y0, x1, y1 = x1, y1, x, y
	}
	for _, p := range pts[2:] {
		bezier(p[0], p[1])
	}
	bezier(x1, y1)
	b.WriteString("L" + pt(x1, y1))
}

// shape returns the path of a symbol of a size, which is the area of the
// symbol in square pixels, with the shapes of Vega.
func shape(name string, size float64) (string, error) {
	r := math.Sqrt(size) / 2
	h := math.Sqrt(3) / 2 * r
	switch name {
	case "", "circle":
		return "M" + pt(r, 0) + "A" + num(r) + "," + num(r) + " 0 1,1 " + pt(-r, 0) +
			"A" + num(r) + "," + num(r) + " 0 1,1 " + pt(r, 0) + "Z", nil
	case "square":
		return "M" + pt(-r, -r) + "H" + num(r) + "V" + num(r) + "H" + num(-r) + "Z", nil
	case "diamond":
		return "M" + pt(-r, 0) + "L" + pt(0, -r) + "L" + pt(r, 0) + "L" + pt(0, r) + "Z", nil
	case "cross":
		s := r / 2.5
		return "M" + pt(-r, -s) + "L" + pt(-r, s) + "L" + pt(-s, s) + "L" + pt(-s, r) +
			"L" + pt(s, r) + "L" + pt(s, s) + "L" + pt(r, s) + "L" + pt(r, -s) +
			"L" + pt(s, -s) + "L" + pt(s, -r) + "L" + pt(-s, -r) + "L" + pt(-s, -s) + "Z", nil
	case "triangle", "triangle-up":
		return "M" + pt(0, -h) + "L" + pt(-r, h) + "L" + pt(r, h) + "Z", nil
	case "triangle-down":
		return "M" + pt(0, h) + "L" + pt(-r, -h) + "L" + pt(r, -h) + "Z", nil
	case "triangle-right":
		return "M" + pt(h, 0) + "L" + pt(-h, -r) + "L" + pt(-h, r) + "Z", nil
	case "triangle-left":
		return "M" + pt(-h, 0) + "L" + pt(h, -r) + "L" + pt(h, r) + "Z", nil
	}
	return "", fmt.Errorf("drawing a symbol %q: %w", name, ErrUnsupportedShape)
}

// arcPath returns the path of a slice of a ring, centered on the origin, as
// the arc of d3-shape draws it without round corners. Angles are in radians,
// clockwise from twelve o'clock, and the pad angle opens a gap between two
// slices.
func arcPath(r0, r1, start, end, pad float64) string {
	const eps = 1e-9
	if r1 < r0 {
		r0, r1 = r1, r0
	}
	if r1 <= eps {
		return "M0,0Z"
	}
	a0, a1 := start-math.Pi/2, end-math.Pi/2
	da := math.Abs(a1 - a0)
	cw := a1 > a0
	sweep := func(clockwise bool) string {
		if clockwise {
			return "1"
		}
		return "0"
	}
	at := func(r, a float64) string { return pt(r*math.Cos(a), r*math.Sin(a)) }
	arcTo := func(r, from, to float64, clockwise bool) string {
		large := "0"
		if math.Abs(to-from) > math.Pi {
			large = "1"
		}
		return "A" + num(r) + "," + num(r) + " 0 " + large + "," + sweep(clockwise) + " " + at(r, to)
	}
	if da >= 2*math.Pi-eps {
		mid := a0 + math.Copysign(math.Pi, a1-a0)
		d := "M" + at(r1, a0) + arcTo(r1, a0, mid, cw) + arcTo(r1, mid, a1, cw) + "Z"
		if r0 > eps {
			d += "M" + at(r0, a1) + arcTo(r0, a1, mid, !cw) + arcTo(r0, mid, a0, !cw) + "Z"
		}
		return d
	}
	a01, a11, a00, a10 := a0, a1, a0, a1
	if pad > eps {
		ap := pad / 2
		rp := math.Sqrt(r0*r0 + r1*r1)
		dir := 1.0
		if !cw {
			dir = -1
		}
		p1 := math.Asin(math.Min(1, rp/r1*math.Sin(ap)))
		if da-2*p1 > eps {
			a01, a11 = a0+p1*dir, a1-p1*dir
		} else {
			a01 = (a0 + a1) / 2
			a11 = a01
		}
		if r0 > eps {
			p0 := math.Asin(math.Min(1, rp/r0*math.Sin(ap)))
			if da-2*p0 > eps {
				a00, a10 = a0+p0*dir, a1-p0*dir
			} else {
				a00 = (a0 + a1) / 2
				a10 = a00
			}
		}
	}
	d := "M" + at(r1, a01) + arcTo(r1, a01, a11, cw)
	if r0 > eps {
		d += "L" + at(r0, a10) + arcTo(r0, a10, a00, !cw)
	} else {
		d += "L0,0"
	}
	return d + "Z"
}
