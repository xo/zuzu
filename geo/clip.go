package geo

import (
	"math"
	"slices"
)

// pt is a point of a line that a clip keeps, with the flag that marks a
// point where the line crosses the edge.
type pt struct {
	x, y float64
	m    int
}

// buffer is a stream that keeps the lines it receives.
type buffer struct {
	lines [][]pt
}

func (b *buffer) point(x, y float64) { b.pointFlag(x, y, 0) }

func (b *buffer) pointFlag(x, y float64, m int) {
	n := len(b.lines) - 1
	b.lines[n] = append(b.lines[n], pt{x, y, m})
}

func (b *buffer) lineStart()    { b.lines = append(b.lines, nil) }
func (b *buffer) lineEnd()      {}
func (b *buffer) polygonStart() {}
func (b *buffer) polygonEnd()   {}
func (b *buffer) sphere()       {}

// rejoin joins the last line to the first.
func (b *buffer) rejoin() {
	if len(b.lines) > 1 {
		b.lines = joinEnds(b.lines)
	}
}

// result returns the lines and empties the buffer.
func (b *buffer) result() [][]pt {
	l := b.lines
	b.lines = nil
	return l
}

// lineClipper cuts a line where it crosses the edge of a clip.
type lineClipper interface {
	stream
	// clean returns 1 if the line did not cross the edge, and 2 if the
	// first and the last pieces join.
	clean() int
}

// interpolator walks along the edge of a clip from one point to another,
// or around the whole edge if from is nil.
type interpolator func(from, to *pt, direction int, s stream)

// clipper clips a geometry on the globe: it keeps the points that are
// visible, cuts the lines at the edge, and closes the polygons along the
// edge. visible, newLine and interpolate describe the edge, and start is a
// point that lies outside it.
type clipper struct {
	visible     func(lambda, phi float64) bool
	newLine     func(s stream) lineClipper
	interpolate interpolator
	start       [2]float64
}

// clipStream is the stream of a clipper.
type clipStream struct {
	c        *clipper
	sink     stream
	line     lineClipper
	ringBuf  *buffer
	ringSink lineClipper

	mode           int // 0 point, 1 line, 2 polygon, 3 ring
	polygonStarted bool
	polygon        [][][2]float64
	ring           [][2]float64
	segments       [][]pt
}

func (c *clipper) stream(sink stream) stream {
	b := &buffer{}
	return &clipStream{c: c, sink: sink, line: c.newLine(sink), ringBuf: b, ringSink: c.newLine(b)}
}

func (s *clipStream) point(lambda, phi float64) {
	switch s.mode {
	case 0, 2:
		if s.c.visible(lambda, phi) {
			s.sink.point(lambda, phi)
		}
	case 1:
		s.line.point(lambda, phi)
	case 3:
		s.ring = append(s.ring, [2]float64{lambda, phi})
		s.ringSink.point(lambda, phi)
	}
}

func (s *clipStream) lineStart() {
	if s.mode == 2 {
		s.mode = 3
		s.ringSink.lineStart()
		s.ring = s.ring[:0:0]
		return
	}
	s.mode = 1
	s.line.lineStart()
}

func (s *clipStream) lineEnd() {
	if s.mode == 3 {
		s.ringEnd()
		s.mode = 2
		return
	}
	s.mode = 0
	s.line.lineEnd()
}

func (s *clipStream) polygonStart() {
	s.mode = 2
	s.segments = nil
	s.polygon = nil
}

func (s *clipStream) startPolygon() {
	if !s.polygonStarted {
		s.sink.polygonStart()
		s.polygonStarted = true
	}
}

func (s *clipStream) polygonEnd() {
	s.mode = 0
	startInside := polygonContains(s.polygon, s.c.start)
	if len(s.segments) > 0 {
		s.startPolygon()
		rejoin(s.segments, compareAntimeridian, startInside, s.c.interpolate, s.sink)
	} else if startInside {
		s.startPolygon()
		s.sink.lineStart()
		s.c.interpolate(nil, nil, 1, s.sink)
		s.sink.lineEnd()
	}
	if s.polygonStarted {
		s.sink.polygonEnd()
		s.polygonStarted = false
	}
	s.segments, s.polygon = nil, nil
}

func (s *clipStream) sphere() {
	s.sink.polygonStart()
	s.sink.lineStart()
	s.c.interpolate(nil, nil, 1, s.sink)
	s.sink.lineEnd()
	s.sink.polygonEnd()
}

// ringEnd closes a ring, and keeps its pieces for the end of the polygon.
func (s *clipStream) ringEnd() {
	if len(s.ring) > 0 {
		first := s.ring[0]
		s.ring = append(s.ring, first)
		s.ringSink.point(first[0], first[1])
	}
	s.ringSink.lineEnd()
	clean := s.ringSink.clean()
	pieces := s.ringBuf.result()
	if len(s.ring) > 0 {
		s.ring = s.ring[:len(s.ring)-1]
	}
	s.polygon = append(s.polygon, s.ring)
	s.ring = nil
	n := len(pieces)
	if n == 0 {
		return
	}
	if clean&1 != 0 {
		// The ring does not cross the edge.
		seg := pieces[0]
		if m := len(seg) - 1; m > 0 {
			s.startPolygon()
			s.sink.lineStart()
			for _, p := range seg[:m] {
				s.sink.point(p.x, p.y)
			}
			s.sink.lineEnd()
		}
		return
	}
	if n > 1 && clean&2 != 0 {
		pieces = joinEnds(pieces)
	}
	for _, p := range pieces {
		if len(p) > 1 {
			s.segments = append(s.segments, p)
		}
	}
}

// joinEnds joins the last line of a list to its first, as one line at the
// end.
func joinEnds(lines [][]pt) [][]pt {
	n := len(lines)
	joined := append(append([]pt{}, lines[n-1]...), lines[0]...)
	return append(lines[1:n-1:n-1], joined)
}

// compareAntimeridian orders the points where lines cross the edge of a
// clip of the globe, along the edge.
func compareAntimeridian(a, b *crossing) float64 {
	return edgeOrder(a.p) - edgeOrder(b.p)
}

func edgeOrder(p pt) float64 {
	if p.x < 0 {
		return p.y - halfPi - epsilon
	}
	return halfPi - p.y
}

// crossing is a point where a line crosses the edge of a clip, linked to
// its neighbors along the line and along the edge.
type crossing struct {
	p       pt
	points  []pt // the piece of the line that starts or ends here
	other   *crossing
	entry   bool
	visited bool
	next    *crossing
	prev    *crossing
}

// rejoin joins the pieces of a clipped polygon into rings: from each piece
// it walks along the edge of the clip to the next piece, in the direction
// that keeps the inside of the polygon on the same side.
func rejoin(segments [][]pt, compare func(a, b *crossing) float64, startInside bool, interpolate interpolator, s stream) {
	var subject, clip []*crossing
	for _, seg := range segments {
		n := len(seg) - 1
		if n <= 0 {
			continue
		}
		p0, p1 := seg[0], seg[n]
		if pointEqual(p0, p1) {
			if p0.m == 0 && p1.m == 0 {
				s.lineStart()
				for _, p := range seg[:n] {
					s.point(p.x, p.y)
				}
				s.lineEnd()
				continue
			}
			// Move the point, so that the piece is not a closed ring.
			seg[n].x += 2 * epsilon
			p1 = seg[n]
		}
		x := &crossing{p: p0, points: seg, entry: true}
		x.other = &crossing{p: p0, other: x}
		subject = append(subject, x)
		clip = append(clip, x.other)
		x = &crossing{p: p1, points: seg}
		x.other = &crossing{p: p1, other: x, entry: true}
		subject = append(subject, x)
		clip = append(clip, x.other)
	}
	if len(subject) == 0 {
		return
	}
	slices.SortStableFunc(clip, func(a, b *crossing) int {
		d := compare(a, b)
		switch {
		case d < 0:
			return -1
		case d > 0:
			return 1
		}
		return 0
	})
	link(subject)
	link(clip)
	for _, c := range clip {
		startInside = !startInside
		c.entry = startInside
	}
	start := subject[0]
	for {
		current := start
		isSubject := true
		for current.visited {
			current = current.next
			if current == start {
				return
			}
		}
		points := current.points
		s.lineStart()
		for {
			current.visited = true
			current.other.visited = true
			if current.entry {
				if isSubject {
					for _, p := range points {
						s.point(p.x, p.y)
					}
				} else {
					interpolate(&current.p, &current.next.p, 1, s)
				}
				current = current.next
			} else {
				if isSubject {
					points = current.prev.points
					for _, p := range slices.Backward(points) {
						s.point(p.x, p.y)
					}
				} else {
					interpolate(&current.p, &current.prev.p, -1, s)
				}
				current = current.prev
			}
			current = current.other
			points = current.points
			isSubject = !isSubject
			if current.visited {
				break
			}
		}
		s.lineEnd()
	}
}

// link links crossings in a ring, in their order.
func link(l []*crossing) {
	n := len(l)
	if n == 0 {
		return
	}
	for i := range n {
		l[i].next = l[(i+1)%n]
		l[(i+1)%n].prev = l[i]
	}
}

func pointEqual(a, b pt) bool {
	return math.Abs(a.x-b.x) < epsilon && math.Abs(a.y-b.y) < epsilon
}

// longitude returns a longitude in the range from -π to π.
func longitude(lambda float64) float64 {
	if math.Abs(lambda) <= math.Pi {
		return lambda
	}
	return sign(lambda) * (math.Mod(math.Abs(lambda)+math.Pi, tau) - math.Pi)
}

// polygonContains reports whether a polygon on the globe holds a point. It
// sums the angles of the rings to tell the inside from the outside, and
// counts the rings that cross the meridian of the point below it.
func polygonContains(polygon [][][2]float64, point [2]float64) bool {
	lambda, phi := longitude(point[0]), point[1]
	sinPhi := math.Sin(phi)
	normal := vec{math.Sin(lambda), -math.Cos(lambda), 0}
	angle := 0.0
	winding := 0
	var sum adder
	switch sinPhi {
	case 1:
		phi = halfPi + epsilon
	case -1:
		phi = -halfPi - epsilon
	}
	for _, ring := range polygon {
		m := len(ring)
		if m == 0 {
			continue
		}
		p0 := ring[m-1]
		l0 := longitude(p0[0])
		phi0 := p0[1]/2 + quarter
		s0, c0 := math.Sin(phi0), math.Cos(phi0)
		for _, p1 := range ring {
			l1 := longitude(p1[0])
			phi1 := p1[1]/2 + quarter
			s1, c1 := math.Sin(phi1), math.Cos(phi1)
			delta := l1 - l0
			sg := 1.0
			if delta < 0 {
				sg = -1
			}
			absDelta := sg * delta
			anti := absDelta > math.Pi
			k := s0 * s1
			sum.add(math.Atan2(k*sg*math.Sin(absDelta), c0*c1+k*math.Cos(absDelta)))
			if anti {
				angle += delta + sg*tau
			} else {
				angle += delta
			}
			if anti != (l0 >= lambda) != (l1 >= lambda) {
				arc := cartesian(p0[0], p0[1]).cross(cartesian(p1[0], p1[1])).normalize()
				inter := normal.cross(arc).normalize()
				dir := 1.0
				if anti != (delta >= 0) {
					dir = -1
				}
				phiArc := dir * asin(inter[2])
				if phi > phiArc || (phi == phiArc && (arc[0] != 0 || arc[1] != 0)) {
					if anti != (delta >= 0) {
						winding++
					} else {
						winding--
					}
				}
			}
			l0, s0, c0, p0 = l1, s1, c1, p1
		}
	}
	inside := angle < -epsilon || (angle < epsilon && sum.value() < -epsilon2)
	return inside != (winding&1 == 1)
}
