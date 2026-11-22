package geo

import "math"

// This file holds the raw projections of tier 1 of D52: the projections of
// vega-projection-extended that have a closed formula. Each comes from its
// published formulas, not from the source of d3-geo-projection. The
// sources are Snyder, "Map Projections: A Working Manual" (USGS 1987), and
// Snyder and Voxland, "An Album of Map Projections" (USGS 1989), which
// the comments call the Album, with the paper of each author.

// hammerRaw is the equal-area projection of Hammer (1892), with the
// coefficient b of the generalized form. Hammer takes the equatorial
// aspect of the azimuthal equal-area projection of Lambert for half the
// longitude, and doubles the width (Album). A coefficient b
// divides the longitude by b and multiplies the width by b, and 2 is the
// projection of Hammer.
func hammerRaw(b float64) rawFunc {
	return func(lambda, phi float64) (float64, float64) {
		x, y := azimuthalEqualAreaRaw(lambda/b, phi)
		return b * x, y
	}
}

// wagner6Raw is the pseudocylindrical projection VI of Wagner (1932), with
// straight parallels at the true distance from the equator (Album).
func wagner6Raw(lambda, phi float64) (float64, float64) {
	f := phi / math.Pi
	return lambda * math.Sqrt(1-3*f*f), phi
}

// winkelPhi1 is the standard parallel of the equirectangular half of the
// Winkel Tripel, arccos(2/pi), which Winkel chose in 1921.
var winkelPhi1 = math.Acos(2 / math.Pi)

// winkel3Raw is the Winkel Tripel (1921), the mean of the equirectangular
// projection with the standard parallel arccos(2/pi) and the projection
// of Aitoff (Album).
func winkel3Raw(lambda, phi float64) (float64, float64) {
	ax, ay := aitoffRaw(lambda, phi)
	return (lambda*math.Cos(winkelPhi1) + ax) / 2, (phi + ay) / 2
}

// aitoffRaw is the projection of Aitoff (1889): the azimuthal equidistant
// projection for half the longitude, with the width doubled (Album).
func aitoffRaw(lambda, phi float64) (float64, float64) {
	cp := math.Cos(phi)
	// alpha is the distance on the sphere from the center to the point
	// at half the longitude.
	alpha := acos(cp * math.Cos(lambda/2))
	k := 1.0
	if alpha != 0 {
		k = alpha / math.Sin(alpha)
	}
	return 2 * k * cp * math.Sin(lambda/2), k * math.Sin(phi)
}

// eckert1C is the constant of the projection I of Eckert, 2 sqrt(2/(3 pi)).
var eckert1C = 2 * math.Sqrt(2/(3*math.Pi))

// eckert1Raw is the projection I of Eckert (1906), with straight meridians
// that break at the equator (Album).
func eckert1Raw(lambda, phi float64) (float64, float64) {
	return eckert1C * lambda * (1 - math.Abs(phi)/math.Pi), eckert1C * phi
}

// collignonRaw is the equal-area projection of Collignon (1865), a
// triangle with the north pole at its top (Album).
func collignonRaw(lambda, phi float64) (float64, float64) {
	s := math.Sqrt(math.Max(0, 1-math.Sin(phi)))
	return 2 / math.Sqrt(math.Pi) * lambda * s, math.Sqrt(math.Pi) * (1 - s)
}

// bakerRaw is the Dinomic projection of Baker (1986). It is Mercator from
// the equator to 45 degrees. Toward a pole, the step of y over the
// latitude is 2 sqrt(2) - 1/sin(phi), which meets the step of Mercator at
// 45 degrees. Each parallel has the scale of the central meridian at that
// latitude, so the pole is a point.
func bakerRaw(lambda, phi float64) (float64, float64) {
	a := math.Abs(phi)
	if a < quarter {
		return lambda, math.Log(math.Tan(quarter + phi/2))
	}
	step := 2*math.Sqrt2 - 1/math.Sin(a)
	return lambda * math.Cos(a) * step,
		sign(phi) * (2*math.Sqrt2*(a-quarter) - math.Log(math.Tan(a/2)))
}

// littrowRaw is the conformal retroazimuthal projection of Littrow (1833)
// (Album).
func littrowRaw(lambda, phi float64) (float64, float64) {
	return math.Sin(lambda) / math.Cos(phi), math.Tan(phi) * math.Cos(lambda)
}

// bottomleyRaw is the equal-area projection of Bottomley (2003), with s
// the sine of its standard parallel. A parallel is an arc of a circle
// about the north pole at its true distance, as in the projection of
// Werner, and s pulls the arcs in. An s of 1 is Werner, and 0.5, the
// standard parallel at 30 degrees, is the default.
func bottomleyRaw(s float64) rawFunc {
	return func(lambda, phi float64) (float64, float64) {
		rho := halfPi - phi
		e := 0.0
		if rho != 0 {
			e = lambda * s * math.Sin(rho) / rho
		}
		return rho * math.Sin(e) / s, halfPi - rho*math.Cos(e)
	}
}

// wiechelRaw is the equal-area pseudoazimuthal projection of Wiechel
// (1879), centered at (0, 0) (Album). A point at the distance c
// from the center lies at 2 sin(c/2), as in the azimuthal equal-area
// projection, but its direction turns by c/2, so the meridians are arcs.
// Vega turns its center to the north pole.
func wiechelRaw(lambda, phi float64) (float64, float64) {
	cp := math.Cos(phi)
	// (x0, y0) is sin(c) times the direction of the point.
	x0, y0 := cp*math.Sin(lambda), math.Sin(phi)
	s := math.Hypot(x0, y0)
	if s == 0 && cp*math.Cos(lambda) > 0 {
		return 0, 0
	}
	// The angle from the sine and the cosine holds its precision near
	// the antipode, where the cosine alone does not.
	half := math.Atan2(s, cp*math.Cos(lambda)) / 2
	ux, uy := 0.0, 1.0
	if s != 0 {
		ux, uy = x0/s, y0/s
	}
	sh, ch := math.Sincos(half)
	return 2 * sh * (ux*ch + uy*sh), 2 * sh * (uy*ch - ux*sh)
}

// airyRaw is the minimum-error azimuthal projection of Airy (1861), with
// beta, in radians, the radius of the region in which it holds the error
// least (Snyder 1987). With z half the distance from the center and h
// half of beta, the radius is
//
//	-2 (ln(cos z)/tan z + tan z ln(cos h)/tan² h).
func airyRaw(beta float64) rawFunc {
	th := math.Tan(beta / 2)
	// b is ln(cos h)/tan² h, or its limit of -1/2 for a small beta.
	b := -0.5
	if th > epsilon {
		b = math.Log(math.Cos(beta/2)) / (th * th)
	}
	return func(lambda, phi float64) (float64, float64) {
		cp := math.Cos(phi)
		c := acos(cp * math.Cos(lambda))
		s := math.Sin(c)
		// k is the radius over sin(c), with its limit at the center.
		k := 0.5 - b
		if c > epsilon {
			z := c / 2
			tz := math.Tan(z)
			k = -2 * (math.Log(math.Cos(z))/tz + tz*b) / s
		}
		return k * cp * math.Sin(lambda), k * math.Sin(phi)
	}
}
