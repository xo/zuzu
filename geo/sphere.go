package geo

import (
	"math"
	"slices"
)

// The measures in this file work on the sphere, on the longitudes and the
// latitudes of a GeoJSON object with no projection, as geoArea, geoBounds
// and geoCentroid of d3-geo do. An edge between two points is the shorter
// arc of the great circle through them. A polygon follows the rule of
// d3-geo: the inside of a ring is on its right, so a small polygon winds
// clockwise. A ring that winds the other way holds the rest of the sphere.

// SphereArea returns the area of the polygons of a GeoJSON object on the
// unit sphere, in steradians. Points and lines have no area, and the
// sphere has 4π.
func SphereArea(object any) float64 {
	a := &sphereAreaSink{}
	streamObject(object, a)
	return a.sum.value() * 2
}

// sphereAreaSink sums the areas of the polygons of a stream on the sphere.
// For each edge, it adds half the spherical excess of the triangle of the
// edge and the south pole, which is signed by the direction of the edge.
// The sum over a ring is half the area on the right of the ring.
type sphereAreaSink struct {
	sum, ring adder
	inPoly    bool
	// started is set at the first point of a ring, and at is set from the
	// second.
	started, at bool
	// lon00 and lat00 are the first point of the ring, in degrees. lambda
	// is the longitude of the last point in radians, and cos and sin are of
	// half its distance from the south pole.
	lon00, lat00     float64
	lambda, cos, sin float64
}

func (a *sphereAreaSink) point(lon, lat float64) {
	if !a.inPoly || !a.started {
		return
	}
	half := lat*radians/2 + quarter
	c, s := math.Cos(half), math.Sin(half)
	lambda := lon * radians
	if a.at {
		// By the theorem of Cagnoli, the tangent of half the excess of
		// the triangle is k·sin(Δλ) over cos·cos + k·cos(Δλ).
		d := lambda - a.lambda
		dir := 1.0
		if d < 0 {
			dir = -1
		}
		k := a.sin * s
		a.ring.add(math.Atan2(k*dir*math.Sin(dir*d), a.cos*c+k*math.Cos(dir*d)))
	} else {
		a.lon00, a.lat00, a.at = lon, lat, true
	}
	a.lambda, a.cos, a.sin = lambda, c, s
}

func (a *sphereAreaSink) lineStart() {
	if a.inPoly {
		a.started, a.at = true, false
	}
}

func (a *sphereAreaSink) lineEnd() {
	if a.inPoly && a.at {
		a.point(a.lon00, a.lat00)
	}
	a.started, a.at = false, false
}

func (a *sphereAreaSink) polygonStart() {
	a.inPoly = true
	a.ring = adder{}
}

func (a *sphereAreaSink) polygonEnd() {
	a.inPoly = false
	r := a.ring.value()
	if r < 0 {
		r += tau
	}
	a.sum.add(r)
}

func (a *sphereAreaSink) sphere() { a.sum.add(tau) }

// SphereBounds returns the box of longitudes and latitudes that holds a
// GeoJSON object on the sphere, as [[west, south], [east, north]], in
// degrees. An edge counts with the highest or the lowest latitude of its
// arc. A box that crosses the antimeridian has its west greater than its
// east. A polygon that holds a pole reaches that pole, over every
// longitude. An object with no point gives NaN.
func SphereBounds(object any) [2][2]float64 {
	b := &sphereBoundsSink{
		west: math.Inf(1), south: math.Inf(1),
		east: math.Inf(-1), north: math.Inf(-1),
		cur: -1,
	}
	streamObject(object, b)
	b.merge()
	if math.IsInf(b.west, 1) || math.IsInf(b.south, 1) {
		nan := math.NaN()
		return [2][2]float64{{nan, nan}, {nan, nan}}
	}
	return [2][2]float64{{b.west, b.south}, {b.east, b.north}}
}

// sphereBoundsSink keeps a range of longitudes for each point and each
// line, and the least and the greatest latitude. A range runs east from its
// west end to its east end, and crosses the antimeridian when its west end
// is greater.
type sphereBoundsSink struct {
	ranges [][2]float64
	// cur is the index of the range of the line or the ring that the
	// stream is in, or -1. west and east are the ends of that range while
	// it grows.
	cur          int
	west, east   float64
	south, north float64

	inLine, inPoly bool
	// prev is the last point of the line on the unit sphere, and prevLon
	// its longitude, while hasPrev is set.
	prev    vec
	prevLon float64
	hasPrev bool

	// The state of a ring: its first point, the sum of the steps of its
	// longitudes, and its area.
	lon00, lat00 float64
	winding      adder
	area         sphereAreaSink
}

// eastOf returns how far east b lies from a, from 0 to 360. The distance
// from -180 to 180 is 360.
func eastOf(a, b float64) float64 {
	d := b - a
	if d < 0 {
		d += 360
	}
	return d
}

func (b *sphereBoundsSink) latitude(lat float64) {
	b.south = math.Min(b.south, lat)
	b.north = math.Max(b.north, lat)
}

// newRange starts a range of one longitude.
func (b *sphereBoundsSink) newRange(lon float64) {
	b.ranges = append(b.ranges, [2]float64{lon, lon})
	b.cur = len(b.ranges) - 1
	b.west, b.east = lon, lon
}

// closeRange writes the ends of the current range.
func (b *sphereBoundsSink) closeRange() {
	if b.cur >= 0 {
		b.ranges[b.cur] = [2]float64{b.west, b.east}
	}
}

func (b *sphereBoundsSink) point(lon, lat float64) {
	switch {
	case b.inPoly && b.inLine:
		b.ringPoint(lon, lat)
	case b.inLine:
		b.linePoint(lon, lat)
	default:
		b.newRange(lon)
		b.latitude(lat)
	}
}

// ringPoint adds a point of a ring: to the winding, to the area and to the
// range of the ring.
func (b *sphereBoundsSink) ringPoint(lon, lat float64) {
	if b.hasPrev {
		d := lon - b.prevLon
		if math.Abs(d) > 180 {
			d -= math.Copysign(360, d)
		}
		b.winding.add(d)
	} else {
		b.lon00, b.lat00 = lon, lat
	}
	b.area.point(lon, lat)
	b.linePoint(lon, lat)
}

// linePoint adds a point of a line, with the arc from the point before it.
func (b *sphereBoundsSink) linePoint(lon, lat float64) {
	p := cartesian(lon*radians, lat*radians)
	if !b.hasPrev {
		b.newRange(lon)
		b.latitude(lat)
		b.prev, b.prevLon, b.hasPrev = p, lon, true
		return
	}
	// The highest point of the great circle of the arc lies at the cross
	// product of its normal with a level line in its plane. The arc holds
	// that point, or the lowest point opposite it, when its longitude lies
	// between the ends of the arc.
	normal := b.prev.cross(p)
	level := vec{normal[1], -normal[0], 0}
	topLon, topLat := spherical(level.cross(normal).normalize())
	topLon *= degrees
	topLat *= degrees
	step := lon - b.prevLon
	crosses := math.Abs(step) > 180
	dir := 1.0
	if step <= 0 {
		dir = -1
	}
	between := func(x float64) bool {
		return crosses != (dir*b.prevLon < x && x < dir*lon)
	}
	switch top := topLon * dir; {
	case between(top):
		b.north = math.Max(b.north, topLat)
	case between(math.Mod(top+360, 360) - 180):
		b.south = math.Min(b.south, -topLat)
	}
	b.extend(lon, step, crosses)
	b.latitude(lat)
	b.prev, b.prevLon = p, lon
}

// extend grows the current range to a longitude that a line reaches with a
// step east, or west when the step is negative.
func (b *sphereBoundsSink) extend(lon, step float64, crosses bool) {
	east := step > 0
	switch {
	case crosses:
		// A step of more than 180 degrees crosses the antimeridian and goes
		// the other way.
		east = lon < b.prevLon
	case b.east >= b.west:
		b.west = math.Min(b.west, lon)
		b.east = math.Max(b.east, lon)
		return
	}
	width := eastOf(b.west, b.east)
	if east {
		if eastOf(b.west, lon) > width {
			b.east = lon
		}
	} else if eastOf(lon, b.east) > width {
		b.west = lon
	}
}

func (b *sphereBoundsSink) lineStart() {
	b.inLine, b.hasPrev = true, false
	if b.inPoly {
		b.area.lineStart()
	}
}

func (b *sphereBoundsSink) lineEnd() {
	if b.inPoly && b.hasPrev {
		b.ringPoint(b.lon00, b.lat00)
		b.area.lineEnd()
		// A ring that winds around a pole spans every longitude.
		if math.Abs(b.winding.value()) > epsilon {
			b.west, b.east = -180, 180
		}
	}
	b.closeRange()
	b.inLine, b.hasPrev = false, false
}

func (b *sphereBoundsSink) polygonStart() {
	b.inPoly = true
	b.winding = adder{}
	b.area.polygonStart()
}

func (b *sphereBoundsSink) polygonEnd() {
	ring := b.area.ring.value()
	b.area.polygonEnd()
	b.inPoly = false
	w := b.winding.value()
	switch {
	case ring < 0:
		// The polygon holds more than a hemisphere.
		b.west, b.east, b.south, b.north = -180, 180, -90, 90
	case w > epsilon:
		// A ring that winds east keeps the south pole on its right.
		b.south = -90
	case w < -epsilon:
		b.north = 90
	}
	b.closeRange()
}

func (b *sphereBoundsSink) sphere() {
	b.west, b.east, b.south, b.north = -180, 180, -90, 90
}

// merge joins the ranges that overlap, in the order of their west ends,
// and sets the bounds to the smallest range that holds them all: the
// complement of the widest gap between them.
func (b *sphereBoundsSink) merge() {
	if len(b.ranges) == 0 {
		return
	}
	slices.SortStableFunc(b.ranges, func(x, y [2]float64) int {
		switch {
		case x[0] < y[0]:
			return -1
		case x[0] > y[0]:
			return 1
		}
		return 0
	})
	holds := func(r [2]float64, x float64) bool {
		if r[0] <= r[1] {
			return r[0] <= x && x <= r[1]
		}
		return x < r[0] || r[1] < x
	}
	merged := [][2]float64{b.ranges[0]}
	for _, r := range b.ranges[1:] {
		last := &merged[len(merged)-1]
		if !holds(*last, r[0]) && !holds(*last, r[1]) {
			merged = append(merged, r)
			continue
		}
		if eastOf(last[0], r[1]) > eastOf(last[0], last[1]) {
			last[1] = r[1]
		}
		if eastOf(r[0], last[1]) > eastOf(last[0], last[1]) {
			last[0] = r[0]
		}
	}
	widest := math.Inf(-1)
	before := merged[len(merged)-1]
	for _, r := range merged {
		if gap := eastOf(before[1], r[0]); gap > widest {
			widest = gap
			b.west, b.east = r[0], before[1]
		}
		before = r
	}
}

// SphereCentroid returns the centroid of a GeoJSON object on the sphere,
// as a longitude and a latitude in degrees. It weighs the polygons by
// their area. With no area, it weighs the lines by their length, and with
// no length, it takes the mean of the points. An object with none of
// these, such as the sphere, gives NaN.
func SphereCentroid(object any) (float64, float64) {
	c := &sphereCentroidSink{}
	streamObject(object, c)
	v := vec{c.areaX.value(), c.areaY.value(), c.areaZ.value()}
	m := math.Hypot(math.Hypot(v[0], v[1]), v[2])
	if m < epsilon2 {
		v = c.line
		if c.length < epsilon {
			v = c.mean
		}
		m = math.Hypot(math.Hypot(v[0], v[1]), v[2])
		if m < epsilon2 {
			return math.NaN(), math.NaN()
		}
	}
	return math.Atan2(v[1], v[0]) * degrees, asin(v[2]/m) * degrees
}

// sphereCentroidSink sums the points, the lines and the polygons of a
// stream on the unit sphere. mean is the mean of every point, line is the
// sum of the middle of each arc times its length, and the area sums, after
// J. E. Brock, The Inertia Tensor for a Spherical Triangle (1975), the
// normal of each edge of a ring times its angle.
type sphereCentroidSink struct {
	mean                vec
	count               float64
	line                vec
	length              float64
	areaX, areaY, areaZ adder

	inPoly, inLine bool
	hasPrev        bool
	prev           vec
	lon00, lat00   float64
}

// addMean adds a point to the mean of the points.
func (c *sphereCentroidSink) addMean(p vec) {
	c.count++
	for i := range p {
		c.mean[i] += (p[i] - c.mean[i]) / c.count
	}
}

// addArc adds the arc from the last point to p, of the angle w, to the sum
// of the lines.
func (c *sphereCentroidSink) addArc(p vec, w float64) {
	c.length += w
	for i := range p {
		c.line[i] += w * (c.prev[i] + p[i])
	}
	c.prev = p
	c.addMean(p)
}

func (c *sphereCentroidSink) point(lon, lat float64) {
	p := cartesian(lon*radians, lat*radians)
	switch {
	case !c.inLine:
		c.addMean(p)
	case !c.hasPrev:
		c.prev, c.hasPrev = p, true
		c.lon00, c.lat00 = lon, lat
		c.addMean(p)
	case c.inPoly:
		c.ringArc(p)
	default:
		n := c.prev.cross(p)
		c.addArc(p, math.Atan2(math.Sqrt(n.dot(n)), c.prev.dot(p)))
	}
}

// ringArc adds an edge of a ring to the sums of the area and of the lines.
// The angle of the edge is the arcsine of the length of the normal, as in
// d3-geo, so an edge of more than 90 degrees counts as less.
func (c *sphereCentroidSink) ringArc(p vec) {
	n := c.prev.cross(p)
	m := math.Hypot(math.Hypot(n[0], n[1]), n[2])
	w := asin(m)
	k := 0.0
	if m != 0 {
		k = -w / m
	}
	c.areaX.add(k * n[0])
	c.areaY.add(k * n[1])
	c.areaZ.add(k * n[2])
	c.addArc(p, w)
}

func (c *sphereCentroidSink) lineStart() { c.inLine, c.hasPrev = true, false }

func (c *sphereCentroidSink) lineEnd() {
	if c.inPoly && c.hasPrev {
		c.ringArc(cartesian(c.lon00*radians, c.lat00*radians))
	}
	c.inLine, c.hasPrev = false, false
}

func (c *sphereCentroidSink) polygonStart() { c.inPoly = true }
func (c *sphereCentroidSink) polygonEnd()   { c.inPoly = false }
func (c *sphereCentroidSink) sphere()       {}
