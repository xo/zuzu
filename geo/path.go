package geo

import (
	"math"
	"strconv"
	"strings"
)

// identityStream is the stream of no projection: the coordinates are
// already in the plane.
func identityStream(sink stream) stream { return sink }

// streamOf returns the stream of a projection, or of no projection if p is
// nil.
func streamOf(p *Projection, sink stream) stream {
	if p == nil {
		return identityStream(sink)
	}
	return p.stream(sink)
}

// Path returns the SVG path of a GeoJSON object, with the numbers rounded
// to 3 decimals as in Vega, and a circle of radius r for each point. It
// reports false if the path is empty. A nil projection draws the
// coordinates as they are.
func Path(p *Projection, object any, r float64) (string, bool) {
	s := &pathSink{radius: r, line: -1, pt: -1}
	streamObject(object, streamOf(p, s))
	if s.b.Len() == 0 {
		return "", false
	}
	return s.b.String(), true
}

// pathSink writes the SVG path of a stream.
type pathSink struct {
	b      strings.Builder
	radius float64
	// line is 0 inside a polygon, and pt is the count of points of the
	// line so far, both -1 outside.
	line, pt int
}

// jsRound rounds half up, as Math.round of JavaScript.
func jsRound(x float64) float64 {
	return math.Floor(x + 0.5)
}

func (s *pathSink) pair(x, y float64) {
	s.b.WriteString(fmtNum(x))
	s.b.WriteByte(',')
	s.b.WriteString(fmtNum(y))
}

// fmtNum formats a rounded number as JavaScript prints it.
func fmtNum(x float64) string {
	v := jsRound(x*1000) / 1000
	if v == 0 {
		return "0"
	}
	a := math.Abs(v)
	if a >= 1e21 || a < 1e-6 {
		return strconv.FormatFloat(v, 'g', -1, 64)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func (s *pathSink) point(x, y float64) {
	switch s.pt {
	case 0:
		s.b.WriteByte('M')
		s.pair(x, y)
		s.pt = 1
	case 1:
		s.b.WriteByte('L')
		s.pair(x, y)
	default:
		s.b.WriteByte('M')
		s.pair(x, y)
		r := s.radius
		s.b.WriteString("m0," + fmtNum(r) + "a" + fmtNum(r) + "," + fmtNum(r) + " 0 1,1 0," + fmtNum(-2*r) +
			"a" + fmtNum(r) + "," + fmtNum(r) + " 0 1,1 0," + fmtNum(2*r) + "z")
	}
}

func (s *pathSink) lineStart()    { s.pt = 0 }
func (s *pathSink) polygonStart() { s.line = 0 }
func (s *pathSink) polygonEnd()   { s.line = -1 }
func (s *pathSink) sphere()       {}

func (s *pathSink) lineEnd() {
	if s.line == 0 {
		s.b.WriteByte('Z')
	}
	s.pt = -1
}

// boundsSink keeps the bounds of the points of a stream.
type boundsSink struct {
	x0, y0, x1, y1 float64
}

func (b *boundsSink) reset() {
	b.x0, b.y0 = math.Inf(1), math.Inf(1)
	b.x1, b.y1 = math.Inf(-1), math.Inf(-1)
}

func (b *boundsSink) point(x, y float64) {
	if x < b.x0 {
		b.x0 = x
	}
	if x > b.x1 {
		b.x1 = x
	}
	if y < b.y0 {
		b.y0 = y
	}
	if y > b.y1 {
		b.y1 = y
	}
}

func (b *boundsSink) lineStart()    {}
func (b *boundsSink) lineEnd()      {}
func (b *boundsSink) polygonStart() {}
func (b *boundsSink) polygonEnd()   {}
func (b *boundsSink) sphere()       {}

// Bounds returns the bounds in the plane of a GeoJSON object, as
// [[x0, y0], [x1, y1]].
func Bounds(p *Projection, object any) [2][2]float64 {
	b := &boundsSink{}
	b.reset()
	streamObject(object, streamOf(p, b))
	return [2][2]float64{{b.x0, b.y0}, {b.x1, b.y1}}
}

// areaSink sums the areas of the polygons of a stream in the plane.
type areaSink struct {
	sum, ring        adder
	inPoly, inRing   bool
	first            bool
	x00, y00, x0, y0 float64
}

func (a *areaSink) point(x, y float64) {
	if !a.inRing {
		return
	}
	if a.first {
		a.first = false
		a.x00, a.y00, a.x0, a.y0 = x, y, x, y
		return
	}
	a.ring.add(a.y0*x - a.x0*y)
	a.x0, a.y0 = x, y
}

func (a *areaSink) lineStart() {
	if a.inPoly {
		a.inRing, a.first = true, true
	}
}

func (a *areaSink) lineEnd() {
	if a.inRing {
		a.point(a.x00, a.y00)
		a.inRing = false
	}
}

func (a *areaSink) polygonStart() { a.inPoly = true }

func (a *areaSink) polygonEnd() {
	a.inPoly = false
	a.sum.add(math.Abs(a.ring.value()))
	a.ring = adder{}
}

func (a *areaSink) sphere() {}

// Area returns the area in the plane of a GeoJSON object.
func Area(p *Projection, object any) float64 {
	a := &areaSink{}
	streamObject(object, streamOf(p, a))
	return a.sum.value() / 2
}

// centroidSink sums the points, the lines and the polygons of a stream,
// each weighted by its count, its length or its area.
type centroidSink struct {
	x0s, y0s, z0, x1s, y1s, z1, x2s, y2s, z2 float64

	inPoly         bool
	mode           int // 0 point, 1 first of a line, 2 line, 3 first of a ring, 4 ring
	x00, y00, x, y float64
}

func (c *centroidSink) addPoint(x, y float64) {
	c.x0s += x
	c.y0s += y
	c.z0++
}

func (c *centroidSink) point(x, y float64) {
	switch c.mode {
	case 0:
		c.addPoint(x, y)
	case 1:
		c.mode = 2
		c.x, c.y = x, y
		c.addPoint(x, y)
	case 2:
		dx, dy := x-c.x, y-c.y
		z := math.Sqrt(dx*dx + dy*dy)
		c.x1s += z * (c.x + x) / 2
		c.y1s += z * (c.y + y) / 2
		c.z1 += z
		c.x, c.y = x, y
		c.addPoint(x, y)
	case 3:
		c.mode = 4
		c.x00, c.y00, c.x, c.y = x, y, x, y
		c.addPoint(x, y)
	case 4:
		c.ringPoint(x, y)
	}
}

func (c *centroidSink) ringPoint(x, y float64) {
	dx, dy := x-c.x, y-c.y
	z := math.Sqrt(dx*dx + dy*dy)
	c.x1s += z * (c.x + x) / 2
	c.y1s += z * (c.y + y) / 2
	c.z1 += z
	z = c.y*x - c.x*y
	c.x2s += z * (c.x + x)
	c.y2s += z * (c.y + y)
	c.z2 += z * 3
	c.x, c.y = x, y
	c.addPoint(x, y)
}

func (c *centroidSink) lineStart() {
	if c.inPoly {
		c.mode = 3
		return
	}
	c.mode = 1
}

func (c *centroidSink) lineEnd() {
	if c.inPoly {
		if c.mode == 4 {
			c.ringPoint(c.x00, c.y00)
		}
	}
	c.mode = 0
}

func (c *centroidSink) polygonStart() { c.inPoly = true }
func (c *centroidSink) polygonEnd()   { c.inPoly = false }
func (c *centroidSink) sphere()       {}

// Centroid returns the centroid in the plane of a GeoJSON object: of its
// polygons, else of its lines, else of its points.
func Centroid(p *Projection, object any) (float64, float64) {
	c := &centroidSink{}
	streamObject(object, streamOf(p, c))
	switch {
	case c.z2 != 0:
		return c.x2s / c.z2, c.y2s / c.z2
	case c.z1 != 0:
		return c.x1s / c.z1, c.y1s / c.z1
	case c.z0 != 0:
		return c.x0s / c.z0, c.y0s / c.z0
	}
	return math.NaN(), math.NaN()
}
