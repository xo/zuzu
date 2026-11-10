package geo

import "math"

// rotation turns the globe about its three axes: lambda about the poles,
// then phi and gamma about the axes in the plane of the equator. The angles
// are in radians.
type rotation struct {
	lambda, phi, gamma float64
}

// wrap returns a longitude in the range from -π to π, after a turn.
func wrap(lambda float64) float64 {
	switch {
	case lambda > math.Pi:
		return lambda - tau
	case lambda < -math.Pi:
		return lambda + tau
	}
	return lambda
}

// forward turns a point.
func (r rotation) forward(lambda, phi float64) (float64, float64) {
	if r.lambda != 0 {
		lambda = wrap(lambda + r.lambda)
	}
	if r.phi == 0 && r.gamma == 0 {
		return lambda, phi
	}
	return rotatePhiGamma(lambda, phi, r.phi, r.gamma)
}

// invert turns a point back.
func (r rotation) invert(lambda, phi float64) (float64, float64) {
	if r.phi != 0 || r.gamma != 0 {
		lambda, phi = unrotatePhiGamma(lambda, phi, r.phi, r.gamma)
	}
	if r.lambda != 0 {
		lambda = wrap(lambda - r.lambda)
	}
	return lambda, phi
}

// rotatePhiGamma turns a point about the two axes in the plane of the
// equator.
func rotatePhiGamma(lambda, phi, dphi, dgamma float64) (float64, float64) {
	cp, sp := math.Cos(dphi), math.Sin(dphi)
	cg, sg := math.Cos(dgamma), math.Sin(dgamma)
	c := math.Cos(phi)
	x, y, z := math.Cos(lambda)*c, math.Sin(lambda)*c, math.Sin(phi)
	k := z*cp + x*sp
	return math.Atan2(y*cg-k*sg, x*cp-z*sp), asin(k*cg + y*sg)
}

// unrotatePhiGamma undoes rotatePhiGamma.
func unrotatePhiGamma(lambda, phi, dphi, dgamma float64) (float64, float64) {
	cp, sp := math.Cos(dphi), math.Sin(dphi)
	cg, sg := math.Cos(dgamma), math.Sin(dgamma)
	c := math.Cos(phi)
	x, y, z := math.Cos(lambda)*c, math.Sin(lambda)*c, math.Sin(phi)
	k := z*cg - y*sg
	return math.Atan2(y*cg+z*sg, x*cp+k*sp), asin(k*cp - x*sp)
}
