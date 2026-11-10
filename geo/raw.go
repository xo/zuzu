package geo

import "math"

// rawFunc is a projection before its scale, its translation and its
// rotation: it maps a longitude and a latitude in radians to the plane.
type rawFunc func(lambda, phi float64) (float64, float64)

func equirectangularRaw(lambda, phi float64) (float64, float64) { return lambda, phi }

func mercatorRaw(lambda, phi float64) (float64, float64) {
	return lambda, math.Log(math.Tan((halfPi + phi) / 2))
}

func transverseMercatorRaw(lambda, phi float64) (float64, float64) {
	return math.Log(math.Tan((halfPi + phi) / 2)), -lambda
}

// azimuthalRaw returns an azimuthal projection, with the scale of the
// radius as a function of the cosine of the angle from the center.
func azimuthalRaw(scale func(float64) float64) rawFunc {
	return func(x, y float64) (float64, float64) {
		cx, cy := math.Cos(x), math.Cos(y)
		k := scale(cx * cy)
		if math.IsInf(k, 1) {
			return 2, 0
		}
		return k * cy * math.Sin(x), k * math.Sin(y)
	}
}

var azimuthalEqualAreaRaw = azimuthalRaw(func(c float64) float64 { return math.Sqrt(2 / (1 + c)) })

var azimuthalEquidistantRaw = azimuthalRaw(func(c float64) float64 {
	c = acos(c)
	if c == 0 {
		return 0
	}
	return c / math.Sin(c)
})

func gnomonicRaw(x, y float64) (float64, float64) {
	cy := math.Cos(y)
	k := math.Cos(x) * cy
	return cy * math.Sin(x) / k, math.Sin(y) / k
}

func orthographicRaw(x, y float64) (float64, float64) {
	return math.Cos(y) * math.Sin(x), math.Sin(y)
}

func stereographicRaw(x, y float64) (float64, float64) {
	cy := math.Cos(y)
	k := 1 + math.Cos(x)*cy
	return cy * math.Sin(x) / k, math.Sin(y) / k
}

// The coefficients of the Equal Earth projection of Šavrič, Patterson and
// Jenny.
const (
	eeA1 = 1.340264
	eeA2 = -0.081106
	eeA3 = 0.000893
	eeA4 = 0.003796
)

var eeM = math.Sqrt(3) / 2

func equalEarthRaw(lambda, phi float64) (float64, float64) {
	l := asin(eeM * math.Sin(phi))
	l2 := l * l
	l6 := l2 * l2 * l2
	return lambda * math.Cos(l) / (eeM * (eeA1 + 3*eeA2*l2 + l6*(7*eeA3+9*eeA4*l2))),
		l * (eeA1 + eeA2*l2 + l6*(eeA3+eeA4*l2))
}

// naturalEarth1Raw is the polynomial form of the Natural Earth projection
// of Šavrič, Jenny, Patterson, Petrovič and Hurni.
func naturalEarth1Raw(lambda, phi float64) (float64, float64) {
	p2 := phi * phi
	p4 := p2 * p2
	return lambda * (0.8707 - 0.131979*p2 + p4*(-0.013791+p4*(0.003971*p2-0.001529*p4))),
		phi * (1.007226 + p2*(0.015085+p4*(-0.044475+0.028874*p2-0.005916*p4)))
}

// mollweideRaw solves for the auxiliary angle of the Mollweide projection
// by Newton's method.
func mollweideRaw(lambda, phi float64) (float64, float64) {
	cx, cy := math.Sqrt2/halfPi, math.Sqrt2
	cp := math.Pi * math.Sin(phi)
	for i := 30; ; {
		delta := (phi + math.Sin(phi) - cp) / (1 + math.Cos(phi))
		phi -= delta
		i--
		if !(math.Abs(delta) > epsilon) || i <= 0 {
			break
		}
	}
	phi /= 2
	return cx * lambda * math.Cos(phi), cy * math.Sin(phi)
}

// cylindricalEqualAreaRaw is the cylindrical equal-area projection with a
// standard parallel at phi0.
func cylindricalEqualAreaRaw(phi0 float64) rawFunc {
	c := math.Cos(phi0)
	return func(lambda, phi float64) (float64, float64) {
		return lambda * c, math.Sin(phi) / c
	}
}

// conicEqualAreaRaw is the conic equal-area projection of Albers, with
// standard parallels at y0 and y1.
func conicEqualAreaRaw(y0, y1 float64) rawFunc {
	sy0 := math.Sin(y0)
	n := (sy0 + math.Sin(y1)) / 2
	if math.Abs(n) < epsilon {
		return cylindricalEqualAreaRaw(y0)
	}
	c := 1 + sy0*(2*n-sy0)
	r0 := math.Sqrt(c) / n
	return func(x, y float64) (float64, float64) {
		r := math.Sqrt(c-2*n*math.Sin(y)) / n
		x *= n
		return r * math.Sin(x), r0 - r*math.Cos(x)
	}
}

func tany(y float64) float64 { return math.Tan((halfPi + y) / 2) }

// conicConformalRaw is the conic conformal projection of Lambert.
func conicConformalRaw(y0, y1 float64) rawFunc {
	cy0 := math.Cos(y0)
	var n float64
	if y0 == y1 {
		n = math.Sin(y0)
	} else {
		n = math.Log(cy0/math.Cos(y1)) / math.Log(tany(y1)/tany(y0))
	}
	if n == 0 {
		return mercatorRaw
	}
	f := cy0 * math.Pow(tany(y0), n) / n
	return func(x, y float64) (float64, float64) {
		if f > 0 {
			if y < -halfPi+epsilon {
				y = -halfPi + epsilon
			}
		} else if y > halfPi-epsilon {
			y = halfPi - epsilon
		}
		r := f / math.Pow(tany(y), n)
		return r * math.Sin(n*x), f - r*math.Cos(n*x)
	}
}

// conicEquidistantRaw is the conic equidistant projection.
func conicEquidistantRaw(y0, y1 float64) rawFunc {
	cy0 := math.Cos(y0)
	var n float64
	if y0 == y1 {
		n = math.Sin(y0)
	} else {
		n = (cy0 - math.Cos(y1)) / (y1 - y0)
	}
	if math.Abs(n) < epsilon {
		return equirectangularRaw
	}
	g := cy0/n + y0
	return func(x, y float64) (float64, float64) {
		gy, nx := g-y, n*x
		return gy * math.Sin(nx), g - gy*math.Cos(nx)
	}
}
