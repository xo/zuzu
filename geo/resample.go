package geo

import "math"

const (
	// maxDepth bounds how many times resampling splits a segment.
	maxDepth = 16
)

// cosMinDistance is the cosine of 30°: resampling splits any segment that
// spans a wider angle on the globe.
var cosMinDistance = math.Cos(30 * radians)

// projectFunc maps a point in radians to the plane.
type projectFunc func(lambda, phi float64) (float64, float64)

// resampler projects each point, and adds points along each segment of a
// line where the straight segment in the plane strays from the great arc on
// the globe by more than the precision. A precision of 0 adds no points.
type resampler struct {
	pass

	project projectFunc
	delta2  float64

	inPolygon, inLine, ring bool
	first                   bool
	// The first point of a ring.
	l00, x00, y00 float64
	c00           vec
	// The previous point.
	l0, x0, y0 float64
	c0         vec
}

func newResampler(project projectFunc, delta2 float64, next stream) *resampler {
	return &resampler{next: next, project: project, delta2: delta2}
}

func (r *resampler) point(lambda, phi float64) {
	if !r.inLine || r.delta2 == 0 {
		r.next.point(r.project(lambda, phi))
		return
	}
	c := cartesian(lambda, phi)
	x, y := r.project(lambda, phi)
	r.lineTo(r.x0, r.y0, r.l0, r.c0, x, y, lambda, c, maxDepth)
	r.x0, r.y0, r.l0, r.c0 = x, y, lambda, c
	r.next.point(x, y)
	if r.ring && r.first {
		r.first = false
		r.l00, r.x00, r.y00, r.c00 = lambda, x, y, c
	}
}

func (r *resampler) lineStart() {
	r.inLine = true
	r.ring = r.inPolygon
	r.first = true
	r.x0 = math.NaN()
	r.next.lineStart()
}

func (r *resampler) lineEnd() {
	if r.ring && r.delta2 != 0 && !r.first {
		r.lineTo(r.x0, r.y0, r.l0, r.c0, r.x00, r.y00, r.l00, r.c00, maxDepth)
	}
	r.inLine = false
	r.ring = false
	r.next.lineEnd()
}

func (r *resampler) polygonStart() {
	r.inPolygon = true
	r.next.polygonStart()
}

func (r *resampler) polygonEnd() {
	r.inPolygon = false
	r.next.polygonEnd()
}

// lineTo adds the points between two projected points, by splitting the
// great arc between them at its middle while the middle strays from the
// straight segment.
func (r *resampler) lineTo(x0, y0, l0 float64, c0 vec, x1, y1, l1 float64, c1 vec, depth int) {
	dx, dy := x1-x0, y1-y0
	d2 := dx*dx + dy*dy
	if !(d2 > 4*r.delta2) || depth <= 0 {
		return
	}
	depth--
	s := c0.add(c1)
	m := math.Sqrt(s.dot(s))
	c := s[2] / m
	phi2 := asin(c)
	var l2 float64
	if math.Abs(math.Abs(c)-1) < epsilon || math.Abs(l0-l1) < epsilon {
		l2 = (l0 + l1) / 2
	} else {
		l2 = math.Atan2(s[1], s[0])
	}
	x2, y2 := r.project(l2, phi2)
	dx2, dy2 := x2-x0, y2-y0
	dz := dy*dx2 - dx*dy2
	if dz*dz/d2 > r.delta2 || math.Abs((dx*dx2+dy*dy2)/d2-0.5) > 0.3 || c0.dot(c1) < cosMinDistance {
		mid := vec{s[0] / m, s[1] / m, c}
		r.lineTo(x0, y0, l0, c0, x2, y2, l2, mid, depth)
		r.next.point(x2, y2)
		r.lineTo(x2, y2, l2, mid, x1, y1, l1, c1, depth)
	}
}
