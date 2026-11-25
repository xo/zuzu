package geo

import (
	"fmt"
	"math"
	"slices"
)

// An interrupted projection cuts the globe into lobes along meridians, and
// draws each lobe with the same projection about a central meridian of its
// own (Goode 1923). The lobes of the north and of the south can differ.
// Each lobe goes from a west meridian to an east meridian, and the lobes of
// a half cover it from -180° to 180° in order.
//
// A spec gives each lobe as three points: one on the west meridian, one on
// the central meridian and one on the east meridian. The latitude of the
// west point is where the lobe parts from its neighbors, and the latitude
// of the central point is where it ends, near the pole. A point of a lobe
// at the longitude λ and the latitude φ goes to the point of the raw
// projection at λ - λc and φ, moved along the x axis by the x of the
// central meridian λc at the latitude φ0 of the west point. A point that
// is nearer to the equator than φ0 moves by the x of λc at its own
// latitude instead, so that below φ0 the lobes join as one map. With φ0 at
// 0, the lobes meet on the equator, because the raw projections that zuzu
// interrupts map the equator to the x axis in proportion to the longitude.
//
// The clip of a polygon on the sphere cuts each geometry along the outline
// of the lobes, so that no line crosses from one lobe to another.

// lobe is a lobe of an interrupted projection: its west, central and east
// meridians, in radians.
type lobe struct {
	west, center, east float64
	// phi0 is the latitude of the point on the west meridian, in radians,
	// and x is the x of the central meridian at that latitude.
	phi0, x float64
}

// interrupted is the state of an interrupted projection.
type interrupted struct {
	raw          rawFunc
	north, south []lobe
}

// The lobes of the projections of vega-projection-extended, in degrees, as
// west, central and east meridians. They are those of Goode for the
// homolosine, which d3-geo-projection 4.0.0 gives the interrupted
// sinusoidal and Mollweide projections. Each lobe parts from its neighbors
// on the equator and ends at the pole.
var (
	sinusoidalLobes = [2][][3]float64{
		{{-180, -110, -40}, {-40, 0, 40}, {40, 110, 180}},
		{{-180, -110, -40}, {-40, 0, 40}, {40, 110, 180}},
	}
	mollweideLobes = [2][][3]float64{
		{{-180, -100, -40}, {-40, 30, 180}},
		{{-180, -160, -100}, {-100, -60, -20}, {-20, 20, 80}, {80, 140, 180}},
	}
	hemisphereLobes = [2][][3]float64{
		{{-180, -90, 0}, {0, 90, 180}},
		{{-180, -90, 0}, {0, 90, 180}},
	}
)

// sansonRaw is the sinusoidal projection of Sanson and Flamsteed, which is
// equal area, in the form of Snyder (1987), page 247: x = λ cos φ, y = φ.
func sansonRaw(lambda, phi float64) (float64, float64) {
	return lambda * math.Cos(phi), phi
}

// poleLobes returns the points of lobes that part on the equator and end
// at the pole, from their meridians, in degrees.
func poleLobes(lobes [2][][3]float64) [2][][3][2]float64 {
	var out [2][][3][2]float64
	for h, half := range lobes {
		pole := 90.0
		if h == 1 {
			pole = -90
		}
		for _, m := range half {
			out[h] = append(out[h], [3][2]float64{{m[0], 0}, {m[1], pole}, {m[2], 0}})
		}
	}
	return out
}

// interrupt makes p an interrupted projection of raw, with lobes in
// degrees, the north first.
func (p *Projection) interrupt(raw rawFunc, lobes [2][][3]float64) {
	p.lobes = &interrupted{raw: raw}
	p.init(equirectangularRaw)
	p.setLobes(poleLobes(lobes))
}

// setLobes sets the lobes of an interrupted projection from their points,
// in degrees.
func (p *Projection) setLobes(lobes [2][][3][2]float64) {
	in := p.lobes
	north, south := lobesInRadians(lobes[0]), lobesInRadians(lobes[1])
	build := func(l [][3][2]float64) []lobe {
		out := make([]lobe, len(l))
		for i, m := range l {
			c, phi0 := m[1][0], m[0][1]
			x, _ := in.raw(c, phi0)
			out[i] = lobe{west: m[0][0], center: c, east: m[2][0], phi0: phi0, x: x}
		}
		return out
	}
	in.north, in.south = build(north), build(south)
	p.raw = in.project
	p.preclip = newLobeClip(north, south, false)
	p.recenter()
	p.reclip()
}

// lobesInRadians returns the points of lobes in radians.
func lobesInRadians(lobes [][3][2]float64) [][3][2]float64 {
	out := make([][3][2]float64, len(lobes))
	for i, l := range lobes {
		for j, q := range l {
			out[i][j] = [2]float64{q[0] * radians, q[1] * radians}
		}
	}
	return out
}

// project is the raw function of an interrupted projection. A point on a
// meridian between two lobes goes to the west lobe, as in d3. A point on
// the equator goes to the lobes of the north.
func (in *interrupted) project(lambda, phi float64) (float64, float64) {
	lobes, sg := in.north, 1.0
	if phi < 0 {
		lobes, sg = in.south, -1
	}
	l := lobes[len(lobes)-1]
	for _, m := range lobes {
		if lambda <= m.east {
			l = m
			break
		}
	}
	x, y := in.raw(lambda-l.center, phi)
	if sg*phi > sg*l.phi0 {
		return x + l.x, y
	}
	cx, _ := in.raw(l.center, phi)
	return x + cx, y
}

// lobeClip moves a point into the lobe that the raw function of an
// interrupted or a polyhedral projection gives it, before the clip of the
// lobes sees it.
type lobeClip struct {
	// north and south are the west and east meridians of the lobes.
	north, south [][2]float64
	// eastward is set if a point on the meridian between two lobes goes
	// to the east lobe, and not to the west lobe.
	eastward bool
}

// newLobeClip returns the clip to the outline of lobes, given as their
// points in radians. A point on a meridian between two lobes goes to the
// east lobe if eastward is set, and to the west lobe if not.
func newLobeClip(north, south [][3][2]float64, eastward bool) *clipper {
	meridians := func(lobes [][3][2]float64) [][2]float64 {
		out := make([][2]float64, len(lobes))
		for i, l := range lobes {
			out[i] = [2]float64{l[0][0], l[2][0]}
		}
		return out
	}
	lc := &lobeClip{north: meridians(north), south: meridians(south), eastward: eastward}
	c := newPolygonClip(lobeRing(north, south))
	c.move = lc.move
	return c
}

// move moves a point that lies within 2e-6 radians of a meridian of its
// lobe to 2e-6 radians inside it. This holds the meridians at ±180° and
// the meridians between two lobes, which the ring leaves out. A line on
// such a meridian then draws along the edge of its lobe, as in d3. The
// move is less than a thousandth of a pixel at the scales of a chart.
func (lc *lobeClip) move(lambda, phi float64) (float64, float64) {
	lobes := lc.north
	if phi < 0 {
		lobes = lc.south
	}
	l := lobes[len(lobes)-1]
	for _, m := range lobes[:len(lobes)-1] {
		if lambda < m[1] || !lc.eastward && lambda == m[1] {
			l = m
			break
		}
	}
	return min(max(lambda, l[0]+2*epsilon), l[1]-2*epsilon), phi
}

// lobeRing returns the outline of lobes on the sphere, as the ring of a
// polygon clip, in radians: the lobes of the north from west to east, then
// those of the south from east to west, which keeps the lobes on the right
// of the ring. Each lobe is its three points, in radians. As in
// d3-geo-projection, the outline of a lobe of the north runs up its west
// meridian from the latitude of the west point to that of the central
// point, east along that parallel, and down its east meridian to the
// latitude of the east point. A lobe of the south runs the same way, upside
// down.
//
// The two sides of the ring along a cut must not touch, so the ring keeps
// 1e-6 radians inside each meridian of a lobe. That leaves a strip of 2e-6
// radians along each cut and along the meridian at ±180°, and move of
// lobeClip moves the points out of it. The ring also keeps 1e-6 radians
// inside the parallel of the central point. About a pole, the ring turns
// in steps of no more than 90°, so that each edge goes the short way about
// the pole. Along another parallel, it takes 30 steps, as
// d3-geo-projection does.
func lobeRing(north, south [][3][2]float64) [][2]float64 {
	const e = epsilon
	var ring [][2]float64
	add := func(lambda, phi float64) {
		if n := len(ring); n > 0 && math.Abs(ring[n-1][0]-lambda) < epsilon2 && math.Abs(ring[n-1][1]-phi) < epsilon2 {
			return
		}
		ring = append(ring, [2]float64{lambda, phi})
	}
	// top adds the points of the ring along the parallel phi, from a to b.
	top := func(a, b, phi float64) {
		n := 30.0
		if math.Abs(math.Abs(phi)-halfPi) < 2*e {
			n = math.Ceil(math.Abs(b-a) / halfPi)
		}
		for i := 0.0; i <= n; i++ {
			add(a+(b-a)*i/n, phi)
		}
	}
	for _, l := range north {
		w, x, phi := l[0][0]+e, l[2][0]-e, l[1][1]-e
		add(w, l[0][1])
		top(w, x, phi)
		add(x, l[2][1])
	}
	for _, l := range slices.Backward(south) {
		w, x, phi := l[0][0]+e, l[2][0]-e, l[1][1]+e
		add(x, l[2][1])
		top(x, w, phi)
		add(w, l[0][1])
	}
	if n := len(ring); n > 1 && math.Abs(ring[0][0]-ring[n-1][0]) < epsilon2 && math.Abs(ring[0][1]-ring[n-1][1]) < epsilon2 {
		ring = ring[:n-1]
	}
	return ring
}

// readLobes sets the lobes of an interrupted projection from the value of
// a spec. A projection that is not interrupted does not take lobes, and
// keeps no word of them.
func (p *Projection) readLobes(v any) error {
	if p.lobes == nil {
		return nil
	}
	l, err := parseLobes(v)
	if err != nil {
		return err
	}
	p.setLobes(l)
	return nil
}

// parseLobes reads the lobes of a spec: a list of the lobes of the north
// and a list of those of the south, where each lobe is three points
// [longitude, latitude]: on the west meridian, on the central meridian, and
// on the east meridian.
func parseLobes(v any) (out [2][][3][2]float64, err error) {
	halves, ok := v.([]any)
	if !ok || len(halves) != 2 {
		return out, fmt.Errorf("the lobes %v: %w", v, ErrProjection)
	}
	for h, half := range halves {
		lobes, ok := half.([]any)
		if !ok || len(lobes) == 0 {
			return out, fmt.Errorf("the lobes %v: %w", v, ErrProjection)
		}
		for _, l := range lobes {
			points, ok := l.([]any)
			if !ok || len(points) != 3 {
				return out, fmt.Errorf("the lobe %v: %w", l, ErrProjection)
			}
			var m [3][2]float64
			for i, q := range points {
				x, y, ok := position(q)
				if !ok || math.IsNaN(x) || math.IsNaN(y) {
					return out, fmt.Errorf("the lobe %v: %w", l, ErrProjection)
				}
				m[i] = [2]float64{x, y}
			}
			out[h] = append(out[h], m)
		}
	}
	return out, nil
}
