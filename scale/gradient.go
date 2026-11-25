package scale

import (
	"math"
	"time"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/internal/color"
)

// GradientStops returns the stops of a linear gradient that samples a
// continuous scale, as the expression function gradient of Vega does. The
// stops are the ticks of the domain for about count values, 15 when count
// is 0 or NaN, with the ends of the domain added where no tick falls. The
// offset of a stop is its place in the domain, from 0 to 1, after the
// transform of the scale, such as log. The value of a stop is what the
// scale maps it to, a color for a scale of colors.
//
// A domain with no span samples a linear scale from 0 to 1 in place of the
// scale: with the scheme of the scale, or else with its first two colors.
// A time scale always adds its ends, so its first and its last stop can
// repeat a tick, as in vega.js. The result is false for a scale that is not
// continuous.
func GradientStops(s Scale, count float64) ([]float64, []any, bool) {
	c, ok := s.(*continuous)
	if !ok {
		return nil, nil, false
	}
	if count == 0 || math.IsNaN(count) {
		count = 15
	}
	lo, hi := c.domain[0], c.domain[len(c.domain)-1]
	if span := hi - lo; span == 0 || math.IsNaN(span) {
		return c.unitStops(count)
	}
	var stops []any
	if count > 0 {
		stops = c.Ticks(count)
	}
	if c.isTime() {
		at := func(v float64) any { return time.UnixMilli(int64(math.Round(v))).In(c.loc) }
		stops = append(append([]any{at(lo)}, stops...), at(hi))
	} else {
		if len(stops) == 0 || expr.Number(stops[0]) != lo {
			stops = append([]any{lo}, stops...)
		}
		if expr.Number(stops[len(stops)-1]) != hi {
			stops = append(stops, hi)
		}
	}
	t0, t1 := c.fwd(lo), c.fwd(hi)
	offsets := make([]float64, len(stops))
	values := make([]any, len(stops))
	for i, v := range stops {
		offsets[i] = 0.5
		if d := t1 - t0; d != 0 && !math.IsInf(d, 0) && !math.IsNaN(d) {
			offsets[i] = (c.fwd(expr.Number(v)) - t0) / d
		}
		values[i] = c.Map(v)
	}
	return offsets, values, true
}

// unitStops returns the stops of a linear scale from 0 to 1 with the colors
// of c, for a domain with no span.
func (c *continuous) unitStops(count float64) ([]float64, []any, bool) {
	var offsets []float64
	if count > 0 {
		offsets = Ticks(0, 1, count)
	}
	if len(offsets) == 0 || offsets[0] != 0 {
		offsets = append([]float64{0}, offsets...)
	}
	if offsets[len(offsets)-1] != 1 {
		offsets = append(offsets, 1)
	}
	values := make([]any, len(offsets))
	for i, f := range offsets {
		switch {
		case c.interpolator != nil:
			values[i] = c.interpolator(f)
		case len(c.colors) >= 2:
			values[i] = color.Mix(c.colors[0], c.colors[1], f, c.interp).Hex()
		case len(c.colors) == 1:
			values[i] = c.colors[0].Hex()
		default:
			values[i] = c.rng[0] + f*(c.rng[1]-c.rng[0])
		}
	}
	return offsets, values, true
}
