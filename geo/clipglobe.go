package geo

import "math"

// antimeridian is the clip of a whole-world projection: it cuts each line
// where it crosses the meridian at 180°, and where it passes over a pole.
var antimeridian = &clipper{
	visible:     func(float64, float64) bool { return true },
	newLine:     func(s stream) lineClipper { return newAntimeridianLine(s) },
	interpolate: antimeridianInterpolate,
	start:       [2]float64{-math.Pi, -halfPi},
}

type antimeridianLine struct {
	s                  stream
	lambda0, phi0, sg0 float64
	cleanFlag          int
}

func newAntimeridianLine(s stream) *antimeridianLine {
	return &antimeridianLine{s: s, lambda0: math.NaN(), phi0: math.NaN(), sg0: math.NaN()}
}

func (l *antimeridianLine) lineStart() {
	l.s.lineStart()
	l.cleanFlag = 1
}

func (l *antimeridianLine) point(lambda1, phi1 float64) {
	sg1 := -math.Pi
	if lambda1 > 0 {
		sg1 = math.Pi
	}
	delta := math.Abs(lambda1 - l.lambda0)
	switch {
	case math.Abs(delta-math.Pi) < epsilon:
		// The line passes over a pole.
		if (l.phi0+phi1)/2 > 0 {
			l.phi0 = halfPi
		} else {
			l.phi0 = -halfPi
		}
		l.s.point(l.lambda0, l.phi0)
		l.s.point(l.sg0, l.phi0)
		l.s.lineEnd()
		l.s.lineStart()
		l.s.point(sg1, l.phi0)
		l.s.point(lambda1, l.phi0)
		l.cleanFlag = 0
	case l.sg0 != sg1 && delta >= math.Pi:
		// The line crosses the antimeridian.
		if math.Abs(l.lambda0-l.sg0) < epsilon {
			l.lambda0 -= l.sg0 * epsilon
		}
		if math.Abs(lambda1-sg1) < epsilon {
			lambda1 -= sg1 * epsilon
		}
		l.phi0 = antimeridianIntersect(l.lambda0, l.phi0, lambda1, phi1)
		l.s.point(l.sg0, l.phi0)
		l.s.lineEnd()
		l.s.lineStart()
		l.s.point(sg1, l.phi0)
		l.cleanFlag = 0
	}
	l.lambda0, l.phi0 = lambda1, phi1
	l.s.point(lambda1, phi1)
	l.sg0 = sg1
}

func (l *antimeridianLine) lineEnd() {
	l.s.lineEnd()
	l.lambda0, l.phi0 = math.NaN(), math.NaN()
}

func (l *antimeridianLine) polygonStart() {}
func (l *antimeridianLine) polygonEnd()   {}
func (l *antimeridianLine) sphere()       {}

func (l *antimeridianLine) clean() int { return 2 - l.cleanFlag }

// antimeridianIntersect returns the latitude where the great arc between
// two points crosses the antimeridian.
func antimeridianIntersect(lambda0, phi0, lambda1, phi1 float64) float64 {
	s := math.Sin(lambda0 - lambda1)
	if math.Abs(s) <= epsilon {
		return (phi0 + phi1) / 2
	}
	c0, c1 := math.Cos(phi0), math.Cos(phi1)
	return math.Atan((math.Sin(phi0)*c1*math.Sin(lambda1) - math.Sin(phi1)*c0*math.Sin(lambda0)) / (c0 * c1 * s))
}

func antimeridianInterpolate(from, to *pt, direction int, s stream) {
	switch {
	case from == nil:
		phi := float64(direction) * halfPi
		s.point(-math.Pi, phi)
		s.point(0, phi)
		s.point(math.Pi, phi)
		s.point(math.Pi, 0)
		s.point(math.Pi, -phi)
		s.point(0, -phi)
		s.point(-math.Pi, -phi)
		s.point(-math.Pi, 0)
		s.point(-math.Pi, phi)
	case math.Abs(from.x-to.x) > epsilon:
		lambda := -math.Pi
		if from.x < to.x {
			lambda = math.Pi
		}
		phi := float64(direction) * lambda / 2
		s.point(-lambda, phi)
		s.point(0, phi)
		s.point(lambda, phi)
	default:
		s.point(to.x, to.y)
	}
}

// circleClip is the clip of a small circle about the center of the view,
// with a radius in radians: an orthographic projection shows only the near
// half of the globe.
func circleClip(radius float64) *clipper {
	cr := math.Cos(radius)
	small := cr > 0
	start := [2]float64{-math.Pi, radius - math.Pi}
	if small {
		start = [2]float64{0, -radius}
	}
	c := &circle{radius: radius, cr: cr, small: small, notHemisphere: math.Abs(cr) > epsilon}
	return &clipper{
		visible: c.visible,
		newLine: func(s stream) lineClipper { return &circleLine{c: c, s: s} },
		interpolate: func(from, to *pt, direction int, s stream) {
			circleStream(s, radius, 2*radians, direction, from, to)
		},
		start: start,
	}
}

type circle struct {
	radius, cr           float64
	small, notHemisphere bool
}

func (c *circle) visible(lambda, phi float64) bool {
	return math.Cos(lambda)*math.Cos(phi) > c.cr
}

// code returns where a point lies about the square that holds the circle,
// as bits.
func (c *circle) code(lambda, phi float64) int {
	r := c.radius
	if !c.small {
		r = math.Pi - c.radius
	}
	code := 0
	if lambda < -r {
		code |= 1
	} else if lambda > r {
		code |= 2
	}
	if phi < -r {
		code |= 4
	} else if phi > r {
		code |= 8
	}
	return code
}

// intersect returns the point where the great arc from a to b crosses the
// circle. With two, it returns both points where the arc crosses, and ok is
// false if it crosses fewer than twice.
func (c *circle) intersect(a, b pt, two bool) (p, q pt, ok bool) {
	pa, pb := cartesian(a.x, a.y), cartesian(b.x, b.y)
	n1 := vec{1, 0, 0}
	n2 := pa.cross(pb)
	n2n2 := n2.dot(n2)
	n1n2 := n2[0]
	det := n2n2 - n1n2*n1n2
	if det == 0 {
		if two {
			return pt{}, pt{}, false
		}
		return a, pt{}, true
	}
	c1 := c.cr * n2n2 / det
	c2 := -c.cr * n1n2 / det
	u := n1.cross(n2)
	A := n1.scale(c1).add(n2.scale(c2))
	w := A.dot(u)
	uu := u.dot(u)
	t2 := w*w - uu*(A.dot(A)-1)
	if t2 < 0 {
		return pt{}, pt{}, false
	}
	t := math.Sqrt(t2)
	qv := u.scale((-w - t) / uu).add(A)
	p.x, p.y = spherical(qv)
	if !two {
		return p, pt{}, true
	}
	l0, l1, phi0, phi1 := a.x, b.x, a.y, b.y
	if l1 < l0 {
		l0, l1 = l1, l0
	}
	delta := l1 - l0
	polar := math.Abs(delta-math.Pi) < epsilon
	meridian := polar || delta < epsilon
	if !polar && phi1 < phi0 {
		phi0, phi1 = phi1, phi0
	}
	var in bool
	switch {
	case meridian && polar:
		ref := phi1
		if math.Abs(p.x-l0) < epsilon {
			ref = phi0
		}
		in = (phi0+phi1 > 0) != (p.y < ref)
	case meridian:
		in = phi0 <= p.y && p.y <= phi1
	default:
		in = (delta > math.Pi) != (l0 <= p.x && p.x <= l1)
	}
	if !in {
		return pt{}, pt{}, false
	}
	q1 := u.scale((-w + t) / uu).add(A)
	q.x, q.y = spherical(q1)
	return p, q, true
}

type circleLine struct {
	c         *circle
	s         stream
	p0        pt
	has0      bool
	c0        int
	v0, v00   bool
	cleanFlag int
}

func (l *circleLine) lineStart() {
	l.v00, l.v0 = false, false
	l.cleanFlag = 1
}

func (l *circleLine) point(lambda, phi float64) {
	c := l.c
	p1 := pt{x: lambda, y: phi}
	v := c.visible(lambda, phi)
	var code int
	switch {
	case c.small && !v:
		code = c.code(lambda, phi)
	case !c.small && v:
		d := -math.Pi
		if lambda < 0 {
			d = math.Pi
		}
		code = c.code(lambda+d, phi)
	}
	if !l.has0 {
		l.v00, l.v0 = v, v
		if v {
			l.s.lineStart()
		}
	}
	if v != l.v0 {
		p2, _, ok := c.intersect(l.p0, p1, false)
		if !ok || pointEqual(l.p0, p2) || pointEqual(p1, p2) {
			p1.m = 1
		}
	}
	if v != l.v0 {
		l.cleanFlag = 0
		var p2 pt
		if v {
			l.s.lineStart()
			p2, _, _ = c.intersect(p1, l.p0, false)
			l.s.point(p2.x, p2.y)
		} else {
			p2, _, _ = c.intersect(l.p0, p1, false)
			pointFlag(l.s, p2.x, p2.y, 2)
			l.s.lineEnd()
		}
		l.p0 = p2
	} else if c.notHemisphere && l.has0 && c.small != v {
		if code&l.c0 == 0 {
			if t0, t1, ok := c.intersect(p1, l.p0, true); ok {
				l.cleanFlag = 0
				if c.small {
					l.s.lineStart()
					l.s.point(t0.x, t0.y)
					l.s.point(t1.x, t1.y)
					l.s.lineEnd()
				} else {
					l.s.point(t1.x, t1.y)
					l.s.lineEnd()
					l.s.lineStart()
					pointFlag(l.s, t0.x, t0.y, 3)
				}
			}
		}
	}
	if v && (!l.has0 || !pointEqual(l.p0, p1)) {
		pointFlag(l.s, p1.x, p1.y, p1.m)
	}
	l.p0, l.has0 = p1, true
	l.v0, l.c0 = v, code
}

func (l *circleLine) lineEnd() {
	if l.v0 {
		l.s.lineEnd()
	}
	l.has0 = false
}

func (l *circleLine) polygonStart() {}
func (l *circleLine) polygonEnd()   {}
func (l *circleLine) sphere()       {}

func (l *circleLine) clean() int {
	k := l.cleanFlag
	if l.v00 && l.v0 {
		k |= 2
	}
	return k
}

// circleStream draws a circle about the center of the view, or the arc of
// it from one point to another, with a step of delta radians. A clip walks
// its edge in steps of 2 degrees.
func circleStream(s stream, radius, delta float64, direction int, from, to *pt) {
	cr, sr := math.Cos(radius), math.Sin(radius)
	step := float64(direction) * delta
	var t0, t1 float64
	if from == nil {
		t0 = radius + float64(direction)*tau
		t1 = radius - step/2
	} else {
		t0 = circleRadius(cr, *from)
		t1 = circleRadius(cr, *to)
		if (direction > 0 && t0 < t1) || (direction <= 0 && t0 > t1) {
			t0 += float64(direction) * tau
		}
	}
	for t := t0; (direction > 0 && t > t1) || (direction <= 0 && t < t1); t -= step {
		x, y := spherical(vec{cr, -sr * math.Cos(t), -sr * math.Sin(t)})
		s.point(x, y)
	}
}

// circleRadius returns the angle about the center of the view of a point
// on the circle.
func circleRadius(cr float64, p pt) float64 {
	v := cartesian(p.x, p.y)
	v[0] -= cr
	v = v.normalize()
	r := acos(-v[1])
	if -v[2] < 0 {
		r = -r
	}
	return math.Mod(r+tau-epsilon, tau)
}
