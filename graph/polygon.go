package graph

import (
	"math"
	"strconv"
	"strings"
)

// polygon is the shape polygon of DOT: a polygon of a number of sides with
// a flat base, which skew slants, distortion widens at the top, and
// orientation turns, in degrees, as Graphviz documents them. A regular
// polygon is as wide as it is high.
type polygon struct {
	sides                         int
	skew, distortion, orientation float64
	regular                       bool
}

// Polygon returns the name of the shape polygon of DOT with its sides,
// skew, distortion, orientation and regular. ShapeOf reads it. A polygon
// of fewer than 3 sides is an ellipse.
func Polygon(sides int, skew, distortion, orientation float64, regular bool) string {
	if sides < 3 {
		return "ellipse"
	}
	s := "polygon " + strconv.Itoa(min(sides, 100)) + " " + num(skew) + " " + num(distortion) + " " + num(orientation)
	if regular {
		s += " regular"
	}
	return s
}

// readPolygon reads the name of a polygon that Polygon writes. The name
// polygon alone is a polygon of 4 sides.
func readPolygon(name string) (polygon, bool) {
	f := strings.Fields(name)
	if len(f) == 0 || f[0] != "polygon" {
		return polygon{}, false
	}
	p := polygon{sides: 4}
	if len(f) > 1 {
		n, err := strconv.Atoi(f[1])
		if err != nil || n < 3 || n > 100 {
			return polygon{}, false
		}
		p.sides = n
	}
	for i, v := range []*float64{&p.skew, &p.distortion, &p.orientation} {
		if len(f) > i+2 {
			x, err := strconv.ParseFloat(f[i+2], 64)
			if err != nil || math.IsNaN(x) || math.IsInf(x, 0) {
				return polygon{}, false
			}
			*v = x
		}
	}
	switch {
	case len(f) == 6 && f[5] == "regular":
		p.regular = true
	case len(f) > 5:
		return polygon{}, false
	}
	return p, true
}

// name returns the name of the polygon, as Polygon writes it.
func (p polygon) name() string {
	return Polygon(p.sides, p.skew, p.distortion, p.orientation, p.regular)
}

// unit returns the corners of the polygon, from x and y -1 to 1, with y
// down. The corners of the regular polygon lie on a circle of radius one
// half, with a flat base. Skew moves each corner across by half the skew
// times its height, and distortion scales it across by its height, so that
// a positive distortion widens the top. Both stretch the polygon across by
// the length of the diagonal of 1 and their size. Then orientation turns
// it, and the corners scale to fill the square.
func (p polygon) unit() [][2]float64 {
	n := float64(p.sides)
	stretch := math.Hypot(math.Abs(p.distortion)+math.Abs(p.skew), 1)
	widen := p.distortion * math.Sqrt2 / math.Cos(math.Pi/n)
	turn := p.orientation * math.Pi / 180
	pts := make([][2]float64, p.sides)
	var mx, my float64
	for i := range pts {
		// Up is positive y here, as in Graphviz.
		a := -math.Pi/2 + math.Pi/n + 2*math.Pi*float64(i)/n
		rx, ry := math.Cos(a)/2, math.Sin(a)/2
		x := rx*(stretch+ry*widen) + ry*p.skew/2
		y := ry
		r, b := math.Hypot(x, y), math.Atan2(y, x)+turn
		x, y = r*math.Cos(b), -r*math.Sin(b)
		pts[i] = [2]float64{x, y}
		mx, my = math.Max(mx, math.Abs(x)), math.Max(my, math.Abs(y))
	}
	for i := range pts {
		if mx > 0 {
			pts[i][0] /= mx
		}
		if my > 0 {
			pts[i][1] /= my
		}
	}
	return pts
}

// size returns the size of the polygon around a box of a size: the
// polygon of the least area that holds the box at its center.
func (p polygon) size(w, h float64) (float64, float64) {
	pts := p.unit()
	// fits reports whether the corners of the box of the half size cx, cy
	// lie in the unit polygon.
	fits := func(cx, cy float64) bool {
		for _, q := range [][2]float64{{-cx, -cy}, {cx, -cy}, {cx, cy}, {-cx, cy}} {
			if !inside(pts, q) {
				return false
			}
		}
		return true
	}
	// Try the corners of the box in each direction from the center, one
	// degree apart, and keep the box of the most area, which makes the
	// polygon of the least. The polygon grows across at least as much as
	// it grows down, as a label is wider than it is high.
	kx, ky := 2.0, 2.0
	best := 0.0
	for d := 45; d < 90; d++ {
		a := float64(d) * math.Pi / 180
		cx, cy := math.Cos(a), math.Sin(a)
		lo, hi := 0.0, 1.5
		for range 30 {
			mid := (lo + hi) / 2
			if fits(mid*cx, mid*cy) {
				lo = mid
			} else {
				hi = mid
			}
		}
		if area := lo * lo * cx * cy; area > best*(1+1e-9) {
			best, kx, ky = area, 1/(lo*cx), 1/(lo*cy)
		}
	}
	w, h = w*math.Min(kx, 4), h*math.Min(ky, 4)
	if p.regular {
		w = math.Max(w, h)
		h = w
	}
	return w, h
}

// path returns the path of the polygon of a size around 0, 0.
func (p polygon) path(w, h float64) string {
	pts := p.scaled(w, h)
	return poly(pts...)
}

// scaled returns the corners of the polygon of a size around 0, 0.
func (p polygon) scaled(w, h float64) [][2]float64 {
	pts := p.unit()
	for i := range pts {
		pts[i][0] *= w / 2
		pts[i][1] *= h / 2
	}
	return pts
}

// inside reports whether a point is inside a polygon, by the rule of even
// and odd.
func inside(pts [][2]float64, q [2]float64) bool {
	in := false
	for i := range pts {
		a, b := pts[i], pts[(i+1)%len(pts)]
		if (a[1] > q[1]) != (b[1] > q[1]) {
			x := a[0] + (q[1]-a[1])/(b[1]-a[1])*(b[0]-a[0])
			if q[0] < x {
				in = !in
			}
		}
	}
	return in
}

// cross returns the part of the way, t, along the line from 0, 0 to dx, dy
// where the line first meets a side of a polygon around 0, 0, or +Inf.
func cross(pts [][2]float64, dx, dy float64) float64 {
	t := math.Inf(1)
	for i := range pts {
		a, b := pts[i], pts[(i+1)%len(pts)]
		ex, ey := b[0]-a[0], b[1]-a[1]
		den := dx*ey - dy*ex
		if den == 0 {
			continue
		}
		// The line meets the side at s along the line and u along the side.
		s := (a[0]*ey - a[1]*ex) / den
		u := (a[0]*dy - a[1]*dx) / den
		if s > 0 && u >= 0 && u <= 1 {
			t = math.Min(t, s)
		}
	}
	return t
}
