package scale

import (
	"math"
	"slices"
	"sort"

	"github.com/xo/zuzu/expr"
)

// discretizing maps a continuous domain to a discrete range, at thresholds:
// quantize, quantile, threshold and bin-ordinal.
type discretizing struct {
	typ        string
	domain     []any
	thresholds []float64
	rng        []any
}

// newDiscretizing builds a quantize, a quantile, a threshold or a
// bin-ordinal scale. A quantize scale cuts the extent of its domain into
// even pieces, one for each value of the range. A quantile scale cuts its
// domain, a sample, at its quantiles. A threshold scale cuts at its domain,
// and a bin-ordinal scale at its bins.
func newDiscretizing(o Options) *discretizing {
	s := &discretizing{typ: o.Type, domain: o.Domain, rng: o.Range}
	if o.Reverse {
		s.rng = slices.Clone(o.Range)
		slices.Reverse(s.rng)
	}
	d := numbersOf(o.Domain)
	n := len(s.rng)
	switch o.Type {
	case "quantize":
		lo, hi := 0.0, 1.0
		if len(d) >= 2 {
			d = includeZero(d, o.Zero != nil && *o.Zero)
			lo, hi = d[0], d[len(d)-1]
		}
		if !math.IsNaN(o.DomainMin) {
			lo = o.DomainMin
		}
		if !math.IsNaN(o.DomainMax) {
			hi = o.DomainMax
		}
		if o.Nice {
			lo, hi = Nice(lo, hi, float64(max(1, n)))
		}
		s.domain = []any{lo, hi}
		for i := 1; i < n; i++ {
			s.thresholds = append(s.thresholds, lo+(hi-lo)*float64(i)/float64(n))
		}
	case "quantile":
		var vals []float64
		for _, v := range d {
			if !math.IsNaN(v) {
				vals = append(vals, v)
			}
		}
		sort.Float64s(vals)
		s.domain = anys(vals)
		if len(vals) > 0 {
			for i := 1; i < n; i++ {
				s.thresholds = append(s.thresholds, quantileSorted(vals, float64(i)/float64(n)))
			}
		}
	case "bin-ordinal":
		s.thresholds = o.Bins
		if len(s.thresholds) == 0 {
			s.thresholds = d
		}
		if len(s.thresholds) > 2 {
			s.thresholds = s.thresholds[1 : len(s.thresholds)-1]
		}
	default:
		s.thresholds = d
	}
	return s
}

// quantileSorted returns a quantile of sorted numbers, interpolated.
func quantileSorted(v []float64, q float64) float64 {
	h := float64(len(v)-1) * q
	i := int(math.Floor(h))
	if i+1 >= len(v) {
		return v[len(v)-1]
	}
	return v[i] + (h-float64(i))*(v[i+1]-v[i])
}

func (s *discretizing) Type() string { return s.typ }

func (s *discretizing) Map(v any) any {
	x := expr.Number(v)
	if math.IsNaN(x) || len(s.rng) == 0 {
		return expr.Undefined
	}
	i := sort.Search(len(s.thresholds), func(i int) bool { return s.thresholds[i] > x })
	return s.rng[min(i, len(s.rng)-1)]
}

func (s *discretizing) Invert(float64) any  { return expr.Undefined }
func (s *discretizing) Domain() []any       { return s.domain }
func (s *discretizing) Range() []any        { return s.rng }
func (s *discretizing) Bandwidth() float64  { return 0 }
func (s *discretizing) Ticks(float64) []any { return anys(s.thresholds) }

// Thresholds returns the values where a discretizing scale changes from one
// value of its range to the next.
func Thresholds(s Scale) []float64 {
	if d, ok := s.(*discretizing); ok {
		return d.thresholds
	}
	return nil
}
