package geo

import (
	"fmt"
	"math"
)

// The armadillo projection of Raisz (1943) draws the globe as if it were
// the skin of a torus, seen from above at a tilt. The formulas come from
// Snyder, "Flattening the Earth" (1993), and from Bugayevskiy and Snyder,
// "Map Projections: A Reference Manual" (1995):
//
//	x = (1 + cos φ) sin(λ/2)
//	y = (1 + sin φ₀ - cos φ₀)/2 + sin φ cos φ₀ - (1 + cos φ) sin φ₀ cos(λ/2)
//
// where φ₀ is the tilt, 20° by default, which Vega calls the parallel. The
// torus hides the points south of the latitude
//
//	φ = -arctan(cos(λ/2) / tan φ₀)
//
// and the outline of the map follows that line.

// armadilloTilt is the default tilt φ₀ of the armadillo projection.
const armadilloTilt = 20 * radians

// armadilloHide is how far a hidden point moves down, off the map, in the
// units of the raw projection. d3-geo-projection moves it so, and a spec of
// Vega clips such a point with the outline of the sphere.
const armadilloHide = 10

// armadilloSlack is how far, in radians, a point can pass the limit of
// the hidden points and still show, as in d3-geo-projection. The great
// arcs that the outline of the map draws between its points pass the limit
// by less than that.
const armadilloSlack = 1e-3

// armadilloLimit returns the latitude south of which the torus hides a
// point at the longitude lambda, for a tilt whose tangent is t0. For a
// negative tilt the torus hides the points north of it.
func armadilloLimit(lambda, t0 float64) float64 {
	return -math.Atan(math.Cos(lambda/2) / t0)
}

// armadilloRaw returns the armadillo projection with the tilt phi0.
func armadilloRaw(phi0 float64) rawFunc {
	s0, c0, t0 := math.Sin(phi0), math.Cos(phi0), math.Tan(phi0)
	sg := sign(phi0)
	k := (1 + s0 - c0) / 2
	return func(lambda, phi float64) (float64, float64) {
		cp := math.Cos(phi)
		h := lambda / 2
		x := (1 + cp) * math.Sin(h)
		y := k + math.Sin(phi)*c0 - (1+cp)*s0*math.Cos(h)
		if sg != 0 && sg*(phi-armadilloLimit(lambda, t0)) < -armadilloSlack {
			y -= sg * armadilloHide
		}
		return x, y
	}
}

// armadilloClip returns the clip of the armadillo projection: the clip of
// the antimeridian, with the outline of the map as the sphere. The outline
// runs along the pole, then back along the limit of the hidden points, in
// steps of three times the precision, in degrees, as in
// d3-geo-projection.
func armadilloClip(p *Projection, phi0 float64) *clipper {
	c := *antimeridian
	t0 := math.Tan(phi0)
	sg := sign(phi0)
	c.outline = func(s stream) {
		ps := sg
		if ps == 0 {
			ps = 1
		}
		for _, l := range []float64{-180, -90, 0, 90} {
			s.point(ps*l*radians, ps*halfPi)
		}
		if sg == 0 {
			return
		}
		step := 3 * math.Sqrt(p.delta2)
		// With a precision of 0, the loop of d3-geo-projection does not
		// end. zuzu then takes the step of the default precision. A tiny
		// precision takes a step of at least 0.001°.
		switch {
		case !(step > 0):
			step = 3 * math.Sqrt(0.5)
		case step < 1e-3:
			step = 1e-3
		}
		for l := 180 - step; l >= -180; l -= step {
			lambda := sg * l * radians
			s.point(lambda, armadilloLimit(lambda, t0))
		}
	}
	return &c
}

// armadillo sets up an armadillo projection with the defaults of
// d3-geo-projection.
func (p *Projection) armadillo() {
	p.init(armadilloRaw(armadilloTilt))
	p.preclip = armadilloClip(p, armadilloTilt)
	p.setScale(218.695)
	p.setCenter(0, 28.0974)
}

// setArmadilloParallel sets the tilt of an armadillo projection, in
// degrees, from the parameter parallel of a spec. It keeps a clip angle
// that the spec set.
func (p *Projection) setArmadilloParallel(v any) error {
	f := Number(v)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return fmt.Errorf("the parallel %v: %w", v, ErrProjection)
	}
	phi0 := f * radians
	p.raw = armadilloRaw(phi0)
	if p.preclip.outline != nil {
		p.preclip = armadilloClip(p, phi0)
	}
	p.recenter()
	return nil
}
