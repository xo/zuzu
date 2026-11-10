// Package scale holds the scales of Vega, which map values of the data to
// values of a chart, such as positions and colors, and the ticks of each
// scale (D6). The view resolves the domain and the range of a scale from
// the spec, and this package builds the scale from them.
package scale

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/xo/zuzu/expr"
)

// Error is an error of the package scale.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// ErrType is the error of a scale type that the package does not have.
const ErrType Error = "unsupported scale type"

// Scale maps a value of its domain to a value of its range.
type Scale interface {
	// Type is the type of the scale, such as linear or band.
	Type() string
	// Map maps a value of the domain to the range. A continuous scale with
	// a range of numbers returns a float64, and with a range of colors a
	// color string.
	Map(v any) any
	// Invert maps a value of the range back to the domain, where the scale
	// can.
	Invert(v float64) any
	// Domain returns the domain.
	Domain() []any
	// Range returns the range.
	Range() []any
	// Bandwidth is the width of a band, for a band scale, and 0 for any
	// other.
	Bandwidth() float64
	// Ticks returns about count values of the domain for an axis or a
	// legend.
	Ticks(count float64) []any
}

// Options are the settings of a scale, from the spec.
type Options struct {
	Type   string
	Domain []any
	// Range holds numbers or colors.
	Range []any
	// RangeStep is the step of a band or a point scale whose range the
	// spec gives as a step. Range is then ignored.
	RangeStep float64
	// DomainMin, DomainMax and DomainMid override the domain, where they
	// are not NaN.
	DomainMin, DomainMax, DomainMid float64
	Reverse                         bool
	Round                           bool
	Clamp                           bool
	// Zero includes zero in the domain of a quantitative scale. Nil takes
	// the default: true for linear, pow and sqrt.
	Zero *bool
	// Nice extends the domain to round values. NiceCount is the number of
	// ticks to make it nice for, and NiceInterval a time interval.
	Nice         bool
	NiceCount    float64
	NiceInterval string
	// Padding is the padding of a band or a point scale, or the padding in
	// pixels of a quantitative scale.
	Padding                    float64
	PaddingInner, PaddingOuter float64
	HasPaddingInner            bool
	HasPaddingOuter            bool
	Align                      float64
	Exponent                   float64
	Base                       float64
	Constant                   float64
	// Interpolate is the color space of a color range: rgb, hcl, lab or
	// hsl.
	Interpolate string
	// Scheme says that the range is a color scheme.
	Scheme bool
	// Bins are the edges of the bins of a bin-ordinal scale.
	Bins []float64
	// Location is the time zone of a time scale.
	Location *time.Location
}

// New builds a scale.
func New(o Options) (Scale, error) {
	switch o.Type {
	case "", "linear", "pow", "sqrt", "log", "symlog", "time", "utc", "sequential":
		return newContinuous(o)
	case "band", "point":
		return newBand(o), nil
	case Trading:
		return newTrading(o), nil
	case "ordinal":
		return newOrdinal(o), nil
	case "quantize", "quantile", "threshold", "bin-ordinal":
		return newDiscretizing(o), nil
	}
	return nil, fmt.Errorf("the scale type %q: %w", o.Type, ErrType)
}

// Defaults returns Options with no overrides of the domain.
func Defaults(typ string) Options {
	return Options{
		Type: typ, DomainMin: math.NaN(), DomainMax: math.NaN(), DomainMid: math.NaN(),
		Align: 0.5, Exponent: 1, Base: 10, Constant: 1, Location: time.UTC,
	}
}

// Discrete reports whether a scale type has a discrete domain.
func Discrete(typ string) bool {
	switch typ {
	case "band", "point", "ordinal", Trading:
		return true
	}
	return false
}

// numbersOf converts values to numbers. A date becomes milliseconds.
func numbersOf(vals []any) []float64 {
	out := make([]float64, len(vals))
	for i, v := range vals {
		out[i] = expr.Number(v)
	}
	return out
}

// anys converts numbers to values.
func anys(f []float64) []any {
	out := make([]any, len(f))
	for i, v := range f {
		out[i] = v
	}
	return out
}

// isColor reports whether a range holds colors rather than numbers.
func isColor(r []any) bool {
	for _, v := range r {
		if _, ok := v.(string); ok {
			return true
		}
	}
	return false
}

// band is a band or a point scale.
type band struct {
	typ                   string
	domain                []any
	index                 map[string]int
	r0, r1                float64
	step, width, start    float64
	paddingInner, padding float64
	reverse               bool
}

// newBand builds a band or a point scale. The domain divides the range
// into bands of equal steps. A band is the step less the inner padding, and
// the outer padding is a share of a step at each end. A point scale is a
// band scale with an inner padding of 1, so its bands have no width.
func newBand(o Options) *band {
	b := &band{typ: o.Type, domain: o.Domain, index: map[string]int{}, reverse: o.Reverse}
	for i, v := range o.Domain {
		k := keyOf(v)
		if _, ok := b.index[k]; !ok {
			b.index[k] = i
		}
	}
	inner, outer := o.Padding, o.Padding
	if o.HasPaddingInner {
		inner = o.PaddingInner
	}
	if o.HasPaddingOuter {
		outer = o.PaddingOuter
	}
	if o.Type == "point" {
		inner = 1
		outer = o.Padding
		if o.HasPaddingOuter {
			outer = o.PaddingOuter
		}
	}
	inner = math.Max(0, math.Min(1, inner))
	n := float64(len(o.Domain))
	span := n - inner + 2*outer
	if o.RangeStep > 0 {
		b.r0, b.r1 = 0, o.RangeStep*math.Max(1, span)
	} else if len(o.Range) >= 2 {
		r := numbersOf(o.Range)
		b.r0, b.r1 = r[0], r[len(r)-1]
	}
	lo, hi := b.r0, b.r1
	rev := hi < lo
	if rev {
		lo, hi = hi, lo
	}
	b.step = (hi - lo) / math.Max(1, span)
	if o.Round {
		b.step = math.Floor(b.step)
	}
	b.start = lo + (hi-lo-b.step*(n-inner))*o.Align
	b.width = b.step * (1 - inner)
	if o.Round {
		b.start = math.Round(b.start)
		b.width = math.Round(b.width)
	}
	if rev != o.Reverse {
		b.reverse = true
	} else {
		b.reverse = false
	}
	b.paddingInner, b.padding = inner, outer
	return b
}

func (b *band) Type() string { return b.typ }

func (b *band) Map(v any) any {
	i, ok := b.index[keyOf(v)]
	if !ok {
		return expr.Undefined
	}
	if b.reverse {
		i = len(b.domain) - 1 - i
	}
	return b.start + b.step*float64(i)
}

func (b *band) Invert(v float64) any {
	if b.step == 0 {
		return expr.Undefined
	}
	i := int(math.Floor((v - b.start) / b.step))
	if i < 0 || i >= len(b.domain) {
		return expr.Undefined
	}
	if b.reverse {
		i = len(b.domain) - 1 - i
	}
	return b.domain[i]
}

func (b *band) Domain() []any       { return b.domain }
func (b *band) Range() []any        { return []any{b.r0, b.r1} }
func (b *band) Bandwidth() float64  { return b.width }
func (b *band) Ticks(float64) []any { return b.domain }

// Step returns the step of a band or a point scale, and 0 for any other.
func Step(s Scale) float64 {
	if b, ok := s.(*band); ok {
		return b.step
	}
	return 0
}

// ordinal maps each value of its domain to a value of its range, in turn.
// A value that is not in the domain maps to undefined, as in Vega.
type ordinal struct {
	domain []any
	index  map[string]int
	rng    []any
}

// newOrdinal builds an ordinal scale.
func newOrdinal(o Options) *ordinal {
	s := &ordinal{index: map[string]int{}, rng: o.Range}
	if o.Reverse {
		s.rng = slices.Clone(o.Range)
		slices.Reverse(s.rng)
	}
	for _, v := range o.Domain {
		s.add(v)
	}
	return s
}

func (s *ordinal) Type() string { return "ordinal" }

func (s *ordinal) Map(v any) any {
	i, ok := s.index[keyOf(v)]
	if !ok || len(s.rng) == 0 {
		return expr.Undefined
	}
	return s.rng[i%len(s.rng)]
}

func (s *ordinal) Invert(float64) any  { return expr.Undefined }
func (s *ordinal) Domain() []any       { return s.domain }
func (s *ordinal) Range() []any        { return s.rng }
func (s *ordinal) Bandwidth() float64  { return 0 }
func (s *ordinal) Ticks(float64) []any { return s.domain }

// add adds a value to the domain, if it is not there.
func (s *ordinal) add(v any) int {
	k := keyOf(v)
	if i, ok := s.index[k]; ok {
		return i
	}
	s.index[k] = len(s.domain)
	s.domain = append(s.domain, v)
	return len(s.domain) - 1
}

// keyOf returns a string that identifies a value of a domain.
func keyOf(v any) string {
	switch x := v.(type) {
	case nil:
		return "\x00null"
	case string:
		return "s" + x
	case time.Time:
		return "n" + expr.String(float64(x.UnixMilli()))
	case bool:
		return "b" + expr.String(x)
	case float64:
		return "n" + expr.String(x)
	}
	return "o" + expr.String(v)
}
