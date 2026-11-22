package geo

import (
	"math"
	"slices"
)

// The clip of a polygon on the sphere keeps the part of a geometry that is
// inside a spherical polygon: a ring of points whose edges are great arcs.
// The interrupted and the polyhedral projections clip to the outline of
// their lobes or faces with it.
//
// The parts follow the clip of a small circle in clipglobe.go:
//
//   - A point is visible if the ring holds it. polygonContains sums the
//     angles that the ring turns about the point, and counts the edges that
//     cross the meridian of the point.
//   - A line is cut where one of its great arcs crosses an edge of the ring.
//     The two planes of two great arcs meet in a line through the center of
//     the sphere, which is the cross product of their normals. That line
//     meets the sphere at two opposite points, and a crossing is the one
//     that lies on both arcs.
//   - A polygon is closed along the ring, from the place where one piece
//     leaves the ring to the place where the next piece comes back. The
//     places along the ring are ordered by the index of the edge and the
//     part of the edge before them, and rejoin in clip.go joins the pieces
//     as in the clip of Weiler and Atherton.

// polygonEdge is an edge of the ring of a polygon clip: the great arc from
// a to b, with the unit normal of its plane and its length in radians.
type polygonEdge struct {
	a, b   vec
	normal vec
	length float64
}

// polygonClip is a spherical polygon that a geometry is clipped to. The
// inside of the ring is on its right, as for the exterior ring of a
// polygon of GeoJSON: a ring that runs clockwise in the plane of longitude
// and latitude holds the area that it surrounds.
type polygonClip struct {
	ring    [][2]float64
	polygon [][][2]float64
	edges   []polygonEdge
}

// newPolygonClip returns the clip to a ring of points in radians. The ring
// does not repeat its first point at its end, and each edge is shorter
// than half of a great circle.
func newPolygonClip(ring [][2]float64) *clipper {
	pc := &polygonClip{ring: ring, polygon: [][][2]float64{ring}}
	n := len(ring)
	for i := range n {
		a := cartesian(ring[i][0], ring[i][1])
		b := cartesian(ring[(i+1)%n][0], ring[(i+1)%n][1])
		nrm := a.cross(b)
		if l := math.Sqrt(nrm.dot(nrm)); l > 0 {
			nrm = nrm.scale(1 / l)
		}
		pc.edges = append(pc.edges, polygonEdge{a: a, b: b, normal: nrm, length: arcLength(a, b)})
	}
	return &clipper{
		visible:     pc.visible,
		newLine:     func(s stream) lineClipper { return &polygonLine{c: pc, s: s} },
		interpolate: pc.interpolate,
		compare: func(a, b *crossing) float64 {
			return pc.place(a.p) - pc.place(b.p)
		},
		start: ring[0],
	}
}

// arcLength returns the angle between two unit vectors.
func arcLength(a, b vec) float64 {
	c := a.cross(b)
	return math.Atan2(math.Sqrt(c.dot(c)), a.dot(b))
}

func (pc *polygonClip) visible(lambda, phi float64) bool {
	return polygonContains(pc.polygon, [2]float64{lambda, phi})
}

// onArc reports whether a point of the great circle of an arc lies on the
// arc, from a to b about the normal n.
func onArc(x, a, b, n vec) bool {
	const tol = -1e-12
	return a.cross(x).dot(n) >= tol && x.cross(b).dot(n) >= tol
}

// place returns the place of a point of the ring along the ring: the index
// of its edge plus the part of the edge before it. The first point of the
// ring is at 0.
func (pc *polygonClip) place(p pt) float64 {
	x := cartesian(p.x, p.y)
	best, place := math.Inf(1), 0.0
	for i, e := range pc.edges {
		if e.length == 0 {
			continue
		}
		var d float64
		if onArc(x, e.a, e.b, e.normal) {
			d = math.Abs(x.dot(e.normal))
		} else {
			d = math.Min(arcLength(x, e.a), arcLength(x, e.b))
		}
		if d < best {
			best = d
			f := arcLength(e.a, x) / e.length
			place = float64(i) + math.Min(math.Max(f, 0), 1)
		}
	}
	return place
}

// interpolate walks along the ring from one point to another, in the order
// of place if direction is 1 and back if it is -1, and sends the points of
// the ring between them. With no points, it walks the whole ring. It sends
// a point at least every 2° of each edge, as the clip of a circle does, so
// that the resampling of the outline follows the curve that the projection
// gives each edge.
func (pc *polygonClip) interpolate(from, to *pt, direction int, s stream) {
	n := float64(len(pc.ring))
	var t0, t1 float64
	if from == nil {
		t0, t1 = 0, n
		if direction < 0 {
			t0, t1 = n, 0
		}
		s.point(pc.ring[0][0], pc.ring[0][1])
	} else {
		t0, t1 = pc.place(*from), pc.place(*to)
		if direction > 0 && t1 < t0 {
			t1 += n
		}
		if direction < 0 && t1 > t0 {
			t1 -= n
		}
	}
	if direction > 0 {
		for i := math.Floor(t0); i < t1; i++ {
			m := pc.steps(i)
			for j := range m {
				if t := i + float64(j)/float64(m); t > t0 && t < t1 {
					s.point(pc.at(t))
				}
			}
		}
		return
	}
	for i := math.Ceil(t0) - 1; i+1 > t1; i-- {
		m := pc.steps(i)
		for j := m; j > 0; j-- {
			if t := i + float64(j)/float64(m); t < t0 && t > t1 {
				s.point(pc.at(t))
			}
		}
	}
}

// steps returns the count of steps of no more than 2° along the edge at a
// place.
func (pc *polygonClip) steps(i float64) int {
	n := len(pc.edges)
	e := pc.edges[(int(i)%n+n)%n]
	return max(1, int(math.Ceil(e.length/(2*radians))))
}

// at returns the point of the ring at a place.
func (pc *polygonClip) at(t float64) (float64, float64) {
	n := len(pc.edges)
	i := math.Floor(t)
	e := pc.edges[(int(i)%n+n)%n]
	f := (t - i) * e.length
	u := e.normal.cross(e.a)
	return spherical(e.a.scale(math.Cos(f)).add(u.scale(math.Sin(f))))
}

// crossings returns where the great arc from a to b crosses the edges of
// the ring, as the angles from a, in order.
func (pc *polygonClip) crossings(a, b vec) []float64 {
	n := a.cross(b)
	l := math.Sqrt(n.dot(n))
	if l < epsilon2 {
		return nil
	}
	n = n.scale(1 / l)
	var out []float64
	for _, e := range pc.edges {
		if e.length == 0 {
			continue
		}
		d := n.cross(e.normal)
		dl := math.Sqrt(d.dot(d))
		if dl < epsilon2 {
			// The arc lies on the great circle of the edge.
			continue
		}
		d = d.scale(1 / dl)
		for _, x := range [2]vec{d, d.scale(-1)} {
			if onArc(x, a, b, n) && onArc(x, e.a, e.b, e.normal) {
				out = append(out, arcLength(a, x))
			}
		}
	}
	slices.Sort(out)
	return out
}

// polygonLine cuts a line where it crosses the ring of a polygon clip.
type polygonLine struct {
	c *polygonClip
	s stream

	// The previous point, as a place on the sphere, and whether it is
	// visible.
	p0      vec
	has0    bool
	v0, v00 bool
	crossed bool
}

func (l *polygonLine) lineStart() {
	l.has0, l.crossed = false, false
}

func (l *polygonLine) point(lambda, phi float64) {
	p1 := cartesian(lambda, phi)
	v1 := l.c.visible(lambda, phi)
	if !l.has0 {
		l.v00 = v1
		if v1 {
			l.s.lineStart()
			l.s.point(lambda, phi)
		}
		l.p0, l.has0, l.v0 = p1, true, v1
		return
	}
	l.cut(l.p0, p1, v1)
	if v1 {
		l.s.point(lambda, phi)
	}
	l.p0, l.v0 = p1, v1
}

// cut sends the places where the arc from the previous point to p1 leaves
// and enters the ring. A piece of the arc between two crossings is visible
// if its middle is.
func (l *polygonLine) cut(p0, p1 vec, v1 bool) {
	ts := l.c.crossings(p0, p1)
	if len(ts) == 0 && l.v0 == v1 {
		return
	}
	n := p0.cross(p1)
	if nl := math.Sqrt(n.dot(n)); nl > 0 {
		n = n.scale(1 / nl)
	}
	u := n.cross(p0)
	at := func(t float64) vec {
		return p0.scale(math.Cos(t)).add(u.scale(math.Sin(t)))
	}
	if len(ts) == 0 {
		// The crossing was too close to an end to find. Find it by
		// halving the arc.
		ts = []float64{l.bisect(at, arcLength(p0, p1), v1)}
	}
	v := l.v0
	for i, t := range ts {
		next := v1
		if i+1 < len(ts) {
			x, y := spherical(at((t + ts[i+1]) / 2))
			next = l.c.visible(x, y)
		}
		if next == v {
			continue
		}
		l.crossed = true
		x, y := spherical(at(t))
		if v {
			pointFlag(l.s, x, y, 1)
			l.s.lineEnd()
		} else {
			l.s.lineStart()
			pointFlag(l.s, x, y, 1)
		}
		v = next
	}
}

// bisect returns the angle along an arc of a given length where the
// visibility changes from that of the start to v1 at the end.
func (l *polygonLine) bisect(at func(float64) vec, length float64, v1 bool) float64 {
	lo, hi := 0.0, length
	for range 60 {
		mid := (lo + hi) / 2
		x, y := spherical(at(mid))
		if l.c.visible(x, y) == v1 {
			hi = mid
		} else {
			lo = mid
		}
	}
	if v1 {
		return hi
	}
	return lo
}

func (l *polygonLine) lineEnd() {
	if l.has0 && l.v0 {
		l.s.lineEnd()
	}
	l.has0 = false
}

func (l *polygonLine) polygonStart() {}
func (l *polygonLine) polygonEnd()   {}
func (l *polygonLine) sphere()       {}

func (l *polygonLine) clean() int {
	k := 0
	if !l.crossed {
		k = 1
	}
	if l.v00 && l.v0 {
		k |= 2
	}
	return k
}
