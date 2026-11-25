package geo

import (
	"fmt"
	"math"
)

// The star projection of Berghaus (1879) draws one hemisphere on the
// azimuthal equidistant projection about its pole, and the other
// hemisphere in pointed lobes, five by default, around it. Inside a lobe a
// meridian is the straight line from its point on the equator to the tip
// of the lobe, where the opposite pole is, and each point keeps its true
// distance from the center. Vega calls the number of lobes lobes.
//
// As in d3-geo-projection, the raw projection has its pole at (0°, 0°),
// and one lobe points up, to the north of the plane.

// berghausLobes is the default number of lobes.
const berghausLobes = 5

// berghausGap is the angle, in degrees, by which the outline stops short
// of the tip of each lobe and moves to each side of its inner corners, as
// in d3-geo-projection.
const berghausGap = 0.01

// berghausRaw returns the star projection with n lobes.
func berghausRaw(n float64) rawFunc {
	w := tau / n
	return func(lambda, phi float64) (float64, float64) {
		cl, cp := math.Cos(lambda), math.Cos(phi)
		c := acos(cp * cl)
		if c <= halfPi {
			return azimuthalEquidistantRaw(lambda, phi)
		}
		// a is the direction of the point from the center, in the plane.
		// The lobe that holds it points in the direction th.
		a := math.Atan2(math.Sin(phi), cp*math.Sin(lambda))
		th := halfPi + math.Floor((a-halfPi)/w+0.5)*w
		// In the frame of the lobe, with its tip at (π, 0), the meridian
		// runs from e on the equator to the tip. The point is where it
		// meets the circle of radius c about the center.
		ex, ey := halfPi*math.Cos(a-th), halfPi*math.Sin(a-th)
		dx, dy := math.Pi-ex, -ey
		dd := dx*dx + dy*dy
		b := ex*dx + ey*dy
		t := (-b + math.Sqrt(b*b-dd*(halfPi*halfPi-c*c))) / dd
		px, py := ex+t*dx, ey+t*dy
		ct, st := math.Cos(th), math.Sin(th)
		return px*ct - py*st, px*st + py*ct
	}
}

// berghausOutline returns the outline of the sphere of the star
// projection with n lobes, the star. For each lobe, from the one that
// points up and then clockwise, the outline passes near its tip and then
// to each side of its inner corner.
func berghausOutline(n float64) func(s stream) {
	w := 360 / n
	return func(s stream) {
		far := (180 - berghausGap) * radians
		for k := 0.0; k < n; k++ {
			th := (90 - k*w) * radians
			s.point(math.Atan2(math.Sin(far)*math.Cos(th), math.Cos(far)), asin(math.Sin(far)*math.Sin(th)))
			// The inner corner lies on the edge of the near hemisphere,
			// on the meridian of 90° east or west.
			v := 90 - k*w - w/2
			for _, a := range []float64{v + berghausGap, v - berghausGap} {
				if v >= -90 {
					s.point(halfPi, a*radians)
				} else {
					s.point(-halfPi, (-180-a)*radians)
				}
			}
		}
	}
}

// berghaus sets up a star projection with the defaults of
// d3-geo-projection: a clip circle of 179.999° about the center, with the
// star as the outline of the sphere.
func (p *Projection) berghaus() {
	p.init(berghausRaw(berghausLobes))
	p.outline = berghausOutline(berghausLobes)
	p.setClipAngle(180 - 1e-3)
	p.setScale(87.8076)
	p.setCenter(0, 17.1875)
}

// setBerghausLobes sets the number of lobes of a star projection from the
// parameter lobes of a spec. It keeps the clip angle.
func (p *Projection) setBerghausLobes(v any) error {
	n := Number(v)
	if !(n >= 1) || math.IsInf(n, 0) {
		return fmt.Errorf("the lobes %v: %w", v, ErrProjection)
	}
	p.raw = berghausRaw(n)
	p.outline = berghausOutline(n)
	p.setPreclip()
	p.recenter()
	return nil
}
