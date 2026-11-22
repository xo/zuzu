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
// A point of a lobe at the longitude λ and the latitude φ goes to the
// point of the raw projection at λ - λc and φ, moved along the x axis by
// the x of the central meridian λc on the equator. On the equator every
// lobe meets its neighbors, because the raw projections that zuzu
// interrupts map the equator to the x axis in proportion to the longitude.
// The clip of a polygon on the sphere cuts each geometry along the lobes,
// so that no line crosses from one lobe to another.

// lobe is a lobe of an interrupted projection: its west, central and east
// meridians, in radians.
type lobe struct {
	west, center, east float64
	// x is the x of the central meridian on the equator.
	x float64
}

// interrupted is the state of an interrupted projection.
type interrupted struct {
	raw          rawFunc
	north, south []lobe
}

// The lobes of the projections of vega-projection-extended, in degrees, as
// west, central and east meridians. They are those of Goode for the
// homolosine, which d3-geo-projection 4.0.0 gives the interrupted
// sinusoidal and Mollweide projections.
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

// interrupt makes p an interrupted projection of raw, with lobes in
// degrees, the north first.
func (p *Projection) interrupt(raw rawFunc, lobes [2][][3]float64) {
	p.lobes = &interrupted{raw: raw}
	p.init(equirectangularRaw)
	p.setLobes(lobes)
}

// setLobes sets the lobes of an interrupted projection, in degrees.
func (p *Projection) setLobes(lobes [2][][3]float64) {
	in := p.lobes
	build := func(l [][3]float64) []lobe {
		out := make([]lobe, len(l))
		for i, m := range l {
			c := m[1] * radians
			x, _ := in.raw(c, 0)
			out[i] = lobe{west: m[0] * radians, center: c, east: m[2] * radians, x: x}
		}
		return out
	}
	in.north, in.south = build(lobes[0]), build(lobes[1])
	p.raw = in.project
	p.preclip = newPolygonClip(lobeRing(cuts(in.north), cuts(in.south), 1))
	p.recenter()
	p.reclip()
}

// project is the raw function of an interrupted projection. A point on a
// meridian between two lobes goes to the west lobe, as in d3, and so does a
// point of the strip east of it that lobeRing leaves out.
func (in *interrupted) project(lambda, phi float64) (float64, float64) {
	lobes := in.north
	if phi < 0 {
		lobes = in.south
	}
	if len(lobes) == 0 {
		return in.raw(lambda, phi)
	}
	l := lobes[len(lobes)-1]
	for _, m := range lobes {
		if lambda <= m.east+epsilon {
			l = m
			break
		}
	}
	x, y := in.raw(lambda-l.center, phi)
	return x + l.x, y
}

// cuts returns the west and east meridians of lobes.
func cuts(lobes []lobe) [][2]float64 {
	out := make([][2]float64, len(lobes))
	for i, l := range lobes {
		out[i] = [2]float64{l.west, l.east}
	}
	return out
}

// lobeRing returns the outline of lobes on the sphere, as the ring of a
// polygon clip, in radians: the lobes of the north from west to east, then
// those of the south from east to west, which keeps the lobes on the right
// of the ring.
//
// The two sides of the ring along a cut must not touch, so the ring leaves
// a strip of 1e-6 radians between them, which no lobe holds. The strip is on
// the east of a cut between two lobes if side is 1, and on the west if side
// is -1, so that the meridian of the cut stays in the lobe that the raw
// function gives it. At ±180° the strip is on both sides. The ring turns
// about each pole a little away from it, in steps of no more than 90°, so
// that each edge goes the short way about the pole.
func lobeRing(north, south [][2]float64, side float64) [][2]float64 {
	const e = epsilon
	west := func(a float64) float64 {
		if a <= -math.Pi {
			return a + e
		}
		return a + side*e + e/2
	}
	east := func(b float64) float64 {
		if b >= math.Pi {
			return b - e
		}
		return b + side*e - e/2
	}
	var ring [][2]float64
	add := func(lambda, phi float64) {
		if n := len(ring); n > 0 && math.Abs(ring[n-1][0]-lambda) < epsilon2 && math.Abs(ring[n-1][1]-phi) < epsilon2 {
			return
		}
		ring = append(ring, [2]float64{lambda, phi})
	}
	// pole adds the points of the ring about a pole, from a to b.
	pole := func(a, b, phi float64) {
		n := math.Ceil(math.Abs(b-a) / halfPi)
		for i := 0.0; i <= n; i++ {
			add(a+(b-a)*i/n, phi)
		}
	}
	for _, l := range north {
		w, x := west(l[0]), east(l[1])
		add(w, 0)
		pole(w, x, halfPi-e)
		add(x, 0)
	}
	for _, l := range slices.Backward(south) {
		w, x := west(l[0]), east(l[1])
		add(x, 0)
		pole(x, w, -halfPi+e)
		add(w, 0)
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
	l, odd, err := parseLobes(v)
	if err != nil {
		return err
	}
	p.setLobes(l)
	if odd {
		// The lobes draw with their longitudes, and the report says that
		// a latitude changes nothing (hard rule 4).
		return fmt.Errorf("a latitude of the lobes other than 0 or ±90: %w", ErrParamUnread)
	}
	return nil
}

// parseLobes reads the lobes of a spec: a list of the lobes of the north
// and a list of those of the south, where each lobe is three points
// [longitude, latitude]: on the west meridian, on the central meridian, and
// on the east meridian. zuzu reads the longitudes only, and odd reports a
// latitude other than 0 or ±90, which changes nothing.
func parseLobes(v any) (out [2][][3]float64, odd bool, err error) {
	halves, ok := v.([]any)
	if !ok || len(halves) != 2 {
		return out, false, fmt.Errorf("the lobes %v: %w", v, ErrProjection)
	}
	for h, half := range halves {
		lobes, ok := half.([]any)
		if !ok || len(lobes) == 0 {
			return out, false, fmt.Errorf("the lobes %v: %w", v, ErrProjection)
		}
		for _, l := range lobes {
			points, ok := l.([]any)
			if !ok || len(points) != 3 {
				return out, false, fmt.Errorf("the lobe %v: %w", l, ErrProjection)
			}
			var m [3]float64
			for i, q := range points {
				x, y, ok := position(q)
				if !ok || math.IsNaN(x) {
					return out, false, fmt.Errorf("the lobe %v: %w", l, ErrProjection)
				}
				if y != 0 && math.Abs(y) != 90 {
					odd = true
				}
				m[i] = x
			}
			out[h] = append(out[h], m)
		}
	}
	return out, odd, nil
}
