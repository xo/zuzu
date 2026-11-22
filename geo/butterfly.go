package geo

import "math"

// The butterfly of Cahill (1909) cuts the globe into the eight faces of an
// octahedron, whose corners are the two poles and the four points of the
// equator at 0°, 90°, 180° and -90°. It draws each face with a gnomonic
// projection about its center and lays the faces out as an unfolded net.
// The four faces of the north meet at the north pole, side by side from
// -180° to 180°, and each face of the south hangs from the face of the
// north above it, along the equator.
//
// The gnomonic projection about the center of a face maps the face to an
// equilateral triangle: it projects each point from the center of the
// sphere to the plane of the face. The corners of a face are three unit
// vectors at right angles, so a point P of the face lies on the plane at
// the weights P·v / Σ P·v of its corners v. The same weights of the
// corners of the triangle of the net give the point of the net. The
// gnomonic projection about the center of a face maps each corner to a
// distance of √2 from the center, so the side of each triangle is √6.

// butterfly holds the corners of the net, in the plane of the raw
// projection: the north pole, the points of the equator at -180°, -90°,
// 0°, 90° and 180°, and the south pole as each face of the south places it.
type butterfly struct {
	north   [2]float64
	equator [5][2]float64
	south   [4][2]float64
}

func newButterfly() *butterfly {
	r6, r2 := math.Sqrt(6), math.Sqrt2
	b := &butterfly{
		north: [2]float64{0, r6},
		equator: [5][2]float64{
			{-3 / r2, 3 * r6 / 2},
			{-3 / r2, r6 / 2},
			{0, 0},
			{3 / r2, r6 / 2},
			{3 / r2, 3 * r6 / 2},
		},
	}
	// Two faces that share an edge make a rhombus, so the south pole of a
	// face is the north pole reflected through the middle of the edge.
	for k := range b.south {
		e0, e1 := b.equator[k], b.equator[k+1]
		b.south[k] = [2]float64{e0[0] + e1[0] - b.north[0], e0[1] + e1[1] - b.north[1]}
	}
	return b
}

// project is the raw function of the butterfly. Face k holds the
// longitudes from -180° + 90°k to -90° + 90°k. A point on a meridian
// between two faces goes to the east face, except at 180°, as in d3, and so
// does a point of the strip west of it that lobeRing leaves out.
func (b *butterfly) project(lambda, phi float64) (float64, float64) {
	k := int(math.Floor((lambda + math.Pi + epsilon) / halfPi))
	k = min(max(k, 0), 3)
	l0 := -math.Pi + float64(k)*halfPi
	p := cartesian(lambda, phi)
	e0, e1 := cartesian(l0, 0), cartesian(l0+halfPi, 0)
	corner, z := b.north, p[2]
	if phi < 0 {
		corner, z = b.south[k], -z
	}
	w0, w1 := math.Max(p.dot(e0), 0), math.Max(p.dot(e1), 0)
	z = math.Max(z, 0)
	s := w0 + w1 + z
	c0, c1 := b.equator[k], b.equator[k+1]
	return (w0*c0[0] + w1*c1[0] + z*corner[0]) / s, (w0*c0[1] + w1*c1[1] + z*corner[1]) / s
}

// butterflyRing is the outline of the net on the sphere: the north is one
// lobe, cut at 180°, and the south is four lobes, cut at each corner of the
// equator.
func butterflyRing() [][2]float64 {
	return lobeRing(
		[][2]float64{{-math.Pi, math.Pi}},
		[][2]float64{{-math.Pi, -halfPi}, {-halfPi, 0}, {0, halfPi}, {halfPi, math.Pi}},
		-1,
	)
}

// polyhedralButterfly makes p the butterfly of Cahill.
func (p *Projection) polyhedralButterfly() {
	p.init(newButterfly().project)
	p.preclip = newPolygonClip(butterflyRing())
}
