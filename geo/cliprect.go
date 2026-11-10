package geo

import "math"

// clipMax bounds the coordinates that the clip of a rectangle reads, so
// that a point far away does not lose precision.
const clipMax = 1e9

// rectClip clips the projected geometry to a rectangle of the plane.
type rectClip struct {
	x0, y0, x1, y1 float64
}

func (r rectClip) visible(x, y float64) bool {
	return r.x0 <= x && x <= r.x1 && r.y0 <= y && y <= r.y1
}

// corner returns the side of the rectangle that a point on its edge lies
// on, counted in the direction of the walk.
func (r rectClip) corner(p pt, direction int) int {
	switch {
	case math.Abs(p.x-r.x0) < epsilon:
		if direction > 0 {
			return 0
		}
		return 3
	case math.Abs(p.x-r.x1) < epsilon:
		if direction > 0 {
			return 2
		}
		return 1
	case math.Abs(p.y-r.y0) < epsilon:
		if direction > 0 {
			return 1
		}
		return 0
	}
	if direction > 0 {
		return 3
	}
	return 2
}

// comparePoint orders two points on the edge of the rectangle.
func (r rectClip) comparePoint(a, b pt) float64 {
	ca, cb := r.corner(a, 1), r.corner(b, 1)
	switch {
	case ca != cb:
		return float64(ca - cb)
	case ca == 0:
		return b.y - a.y
	case ca == 1:
		return a.x - b.x
	case ca == 2:
		return a.y - b.y
	}
	return b.x - a.x
}

func (r rectClip) interpolate(from, to *pt, direction int, s stream) {
	a, a1 := 0, 0
	if from == nil || func() bool {
		a, a1 = r.corner(*from, direction), r.corner(*to, direction)
		return a != a1 || (r.comparePoint(*from, *to) < 0) != (direction > 0)
	}() {
		for {
			x, y := r.x1, r.y0
			if a == 0 || a == 3 {
				x = r.x0
			}
			if a > 1 {
				y = r.y1
			}
			s.point(x, y)
			a = (a + direction + 4) % 4
			if a == a1 {
				break
			}
		}
		return
	}
	s.point(to.x, to.y)
}

// clipSegment cuts the segment from a to b to the rectangle, by the method
// of Liang and Barsky. It reports false if no part of it lies inside.
func (r rectClip) clipSegment(a, b *pt) bool {
	ax, ay := a.x, a.y
	dx, dy := b.x-ax, b.y-ay
	t0, t1 := 0.0, 1.0
	edge := func(q, d float64, low bool) bool {
		// q is the distance to the edge, and d the change along the axis.
		if d == 0 {
			if low {
				return q <= 0
			}
			return q >= 0
		}
		t := q / d
		if (d < 0) == low {
			if t < t0 {
				return false
			}
			if t < t1 {
				t1 = t
			}
		} else {
			if t > t1 {
				return false
			}
			if t > t0 {
				t0 = t
			}
		}
		return true
	}
	if !edge(r.x0-ax, dx, true) || !edge(r.x1-ax, dx, false) || !edge(r.y0-ay, dy, true) || !edge(r.y1-ay, dy, false) {
		return false
	}
	if t0 > 0 {
		a.x, a.y = ax+t0*dx, ay+t0*dy
	}
	if t1 < 1 {
		b.x, b.y = ax+t1*dx, ay+t1*dy
	}
	return true
}

// rectStream is the stream of a clip to a rectangle.
type rectStream struct {
	r      rectClip
	sink   stream
	active stream
	buf    *buffer

	inLine   bool
	segments [][]pt
	polygon  [][]pt
	ring     []pt
	inPoly   bool

	xFirst, yFirst float64
	vFirst         bool
	xPrev, yPrev   float64
	vPrev          bool
	first          bool
	clean          bool
}

func (r rectClip) stream(sink stream) stream {
	return &rectStream{r: r, sink: sink, active: sink, buf: &buffer{}}
}

func (s *rectStream) point(x, y float64) {
	if s.inLine {
		s.linePoint(x, y)
		return
	}
	if s.r.visible(x, y) {
		s.active.point(x, y)
	}
}

func (s *rectStream) sphere() {}

func (s *rectStream) polygonInside() int {
	winding := 0
	r := s.r
	for _, ring := range s.polygon {
		if len(ring) == 0 {
			continue
		}
		b0, b1 := ring[0].x, ring[0].y
		for _, p := range ring[1:] {
			a0, a1 := b0, b1
			b0, b1 = p.x, p.y
			if a1 <= r.y1 {
				if b1 > r.y1 && (b0-a0)*(r.y1-a1) > (b1-a1)*(r.x0-a0) {
					winding++
				}
			} else if b1 <= r.y1 && (b0-a0)*(r.y1-a1) < (b1-a1)*(r.x0-a0) {
				winding--
			}
		}
	}
	return winding
}

func (s *rectStream) polygonStart() {
	s.active = s.buf
	s.segments = nil
	s.polygon = nil
	s.inPoly = true
	s.clean = true
}

func (s *rectStream) polygonEnd() {
	startInside := s.polygonInside() != 0
	cleanInside := s.clean && startInside
	if cleanInside || len(s.segments) > 0 {
		s.sink.polygonStart()
		if cleanInside {
			s.sink.lineStart()
			s.r.interpolate(nil, nil, 1, s.sink)
			s.sink.lineEnd()
		}
		if len(s.segments) > 0 {
			rejoin(s.segments, func(a, b *crossing) float64 { return s.r.comparePoint(a.p, b.p) }, startInside, s.r.interpolate, s.sink)
		}
		s.sink.polygonEnd()
	}
	s.active = s.sink
	s.segments, s.polygon, s.ring = nil, nil, nil
	s.inPoly = false
}

func (s *rectStream) lineStart() {
	s.inLine = true
	if s.inPoly {
		s.polygon = append(s.polygon, nil)
		s.ring = nil
	}
	s.first = true
	s.vPrev = false
	s.xPrev, s.yPrev = math.NaN(), math.NaN()
}

func (s *rectStream) lineEnd() {
	if s.inPoly {
		s.linePoint(s.xFirst, s.yFirst)
		if s.vFirst && s.vPrev {
			s.buf.rejoin()
		}
		s.segments = append(s.segments, s.buf.result()...)
	}
	s.inLine = false
	if s.vPrev {
		s.active.lineEnd()
	}
}

func clamp(v float64) float64 {
	return math.Max(-clipMax, math.Min(clipMax, v))
}

func (s *rectStream) linePoint(x, y float64) {
	v := s.r.visible(x, y)
	if s.inPoly {
		n := len(s.polygon) - 1
		s.polygon[n] = append(s.polygon[n], pt{x: x, y: y})
	}
	switch {
	case s.first:
		s.xFirst, s.yFirst, s.vFirst = x, y, v
		s.first = false
		if v {
			s.active.lineStart()
			s.active.point(x, y)
		}
	case v && s.vPrev:
		s.active.point(x, y)
	default:
		s.xPrev, s.yPrev = clamp(s.xPrev), clamp(s.yPrev)
		x, y = clamp(x), clamp(y)
		a, b := pt{x: s.xPrev, y: s.yPrev}, pt{x: x, y: y}
		if s.r.clipSegment(&a, &b) {
			if !s.vPrev {
				s.active.lineStart()
				s.active.point(a.x, a.y)
			}
			s.active.point(b.x, b.y)
			if !v {
				s.active.lineEnd()
			}
			s.clean = false
		} else if v {
			s.active.lineStart()
			s.active.point(x, y)
			s.clean = false
		}
	}
	s.xPrev, s.yPrev, s.vPrev = x, y, v
}
