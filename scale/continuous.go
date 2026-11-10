package scale

import (
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/internal/color"
)

// continuous is a scale with a continuous domain: linear, pow, sqrt, log,
// symlog, time, utc and sequential. It transforms the domain, for pow, log
// and symlog, and interpolates the range between the transformed values.
type continuous struct {
	typ      string
	domain   []float64 // as given, after nice and zero
	tdomain  []float64 // transformed
	rng      []float64
	colors   []color.RGBA
	color    bool
	interp   string
	clamp    bool
	round    bool
	fwd, inv func(float64) float64
	base     float64
	loc      *time.Location
}

// newContinuous builds a continuous scale.
func newContinuous(o Options) (Scale, error) {
	s := &continuous{typ: o.Type, clamp: o.Clamp, round: o.Round, base: o.Base, loc: o.Location, interp: o.Interpolate}
	if s.typ == "" {
		s.typ = "linear"
	}
	if s.loc == nil {
		s.loc = time.UTC
	}
	if s.typ == "utc" {
		s.loc = time.UTC
	}
	s.fwd, s.inv = transform(s.typ, o)
	d := numbersOf(o.Domain)
	if len(d) < 2 {
		d = []float64{0, 1}
		if len(o.Domain) == 1 {
			d = []float64{expr.Number(o.Domain[0]), expr.Number(o.Domain[0])}
		}
	}
	zero := o.Zero != nil && *o.Zero
	if o.Zero == nil {
		zero = s.typ == "linear" || s.typ == "pow" || s.typ == "sqrt"
		// Vega makes a continuous scale of colors sequential or diverging
		// from the count of its domain, before zero, and those types do
		// not include zero.
		n := len(o.Domain)
		if !math.IsNaN(o.DomainMid) {
			n++
		}
		if !s.isTime() && (o.Scheme || len(o.Range) > 0 && allStrings(o.Range)) && (n == 2 || n == 3) {
			zero = false
		}
	}
	d = includeZero(d, zero)
	if !math.IsNaN(o.DomainMin) {
		d[0] = o.DomainMin
	}
	if !math.IsNaN(o.DomainMax) {
		d[len(d)-1] = o.DomainMax
	}
	if !math.IsNaN(o.DomainMid) && len(d) == 2 {
		d = []float64{d[0], o.DomainMid, d[1]}
	}
	if !s.isTime() && (o.Scheme || len(o.Range) > 0 && allStrings(o.Range)) {
		// A continuous scale of colors is sequential, or diverging when its
		// domain has a middle, as Vega names it.
		switch len(d) {
		case 2:
			s.typ = "sequential-" + s.typ
		case 3:
			s.typ = "diverging-" + s.typ
		}
	}
	if isColor(o.Range) {
		s.color = true
		for _, c := range o.Range {
			rgb, _ := color.Parse(expr.String(c))
			s.colors = append(s.colors, rgb)
		}
		if o.Reverse {
			slices.Reverse(s.colors)
		}
	} else {
		s.rng = numbersOf(o.Range)
		if len(s.rng) < 2 {
			s.rng = []float64{0, 1}
		}
		if o.Reverse {
			s.rng = slices.Clone(s.rng)
			slices.Reverse(s.rng)
		}
	}
	if o.Padding > 0 && !s.color && len(d) == 2 {
		d = s.pad(d, o.Padding)
	}
	if o.Nice && len(d) >= 2 {
		d = s.nice(d, o.NiceCount, o.NiceInterval)
	}
	s.domain = d
	s.tdomain = make([]float64, len(d))
	for i, v := range d {
		s.tdomain[i] = s.fwd(v)
	}
	// A range of more values than the domain spreads the domain over it, as
	// a color ramp does.
	if s.color && len(s.colors) > len(d) && len(d) == 2 {
		n := len(s.colors)
		t0, t1 := s.tdomain[0], s.tdomain[1]
		s.tdomain = make([]float64, n)
		for i := range n {
			s.tdomain[i] = t0 + (t1-t0)*float64(i)/float64(n-1)
		}
	}
	return s, nil
}

// includeZero moves the first end of a domain to zero if it is above zero,
// and the last end if it is below, as the zero setting of Vega does.
func includeZero(d []float64, zero bool) []float64 {
	if !zero || len(d) < 2 {
		return d
	}
	d = slices.Clone(d)
	n := len(d) - 1
	if d[0] > 0 {
		d[0] = 0
	}
	if d[n] < 0 {
		d[n] = 0
	}
	return d
}

// transform returns the function that a scale type applies to its domain,
// and its inverse.
func transform(typ string, o Options) (func(float64) float64, func(float64) float64) {
	id := func(x float64) float64 { return x }
	switch typ {
	case "pow", "sqrt":
		e := o.Exponent
		if typ == "sqrt" {
			e = 0.5
		}
		if e == 1 {
			return id, id
		}
		pw := func(e float64) func(float64) float64 {
			return func(x float64) float64 {
				if x < 0 {
					return -math.Pow(-x, e)
				}
				return math.Pow(x, e)
			}
		}
		return pw(e), pw(1 / e)
	case "log":
		b := o.Base
		if b <= 0 || b == 1 {
			b = 10
		}
		lb := math.Log(b)
		return func(x float64) float64 {
				if x < 0 {
					return -math.Log(-x) / lb
				}
				return math.Log(x) / lb
			}, func(x float64) float64 {
				return math.Pow(b, x)
			}
	case "symlog":
		c := o.Constant
		if c == 0 {
			c = 1
		}
		return func(x float64) float64 {
				return math.Copysign(math.Log1p(math.Abs(x)/c), x)
			}, func(x float64) float64 {
				return math.Copysign(math.Expm1(math.Abs(x))*c, x)
			}
	}
	return id, id
}

func (s *continuous) Type() string { return s.typ }

// allStrings reports whether every value is a string.
func allStrings(vals []any) bool {
	for _, v := range vals {
		if _, ok := v.(string); !ok {
			return false
		}
	}
	return true
}

func (s *continuous) Map(v any) any {
	if v == nil {
		v = 0.0
	}
	x := expr.Number(v)
	if math.IsNaN(x) {
		return expr.Undefined
	}
	t := s.fwd(x)
	i := s.segment(t)
	d0, d1 := s.tdomain[i], s.tdomain[i+1]
	f := 0.5
	if d1 != d0 {
		f = (t - d0) / (d1 - d0)
	}
	if s.clamp {
		f = math.Max(0, math.Min(1, f))
	}
	if s.color {
		j := min(i, len(s.colors)-2)
		if len(s.colors) == 1 {
			return s.colors[0].Hex()
		}
		return color.Mix(s.colors[j], s.colors[j+1], f, s.interp).Hex()
	}
	j := min(i, len(s.rng)-2)
	r := s.rng[j] + f*(s.rng[j+1]-s.rng[j])
	if s.round {
		r = math.Round(r)
	}
	return r
}

func (s *continuous) Invert(v float64) any {
	if s.color || len(s.rng) < 2 {
		return expr.Undefined
	}
	r0, r1 := s.rng[0], s.rng[len(s.rng)-1]
	f := 0.0
	if r1 != r0 {
		f = (v - r0) / (r1 - r0)
	}
	if s.clamp {
		f = math.Max(0, math.Min(1, f))
	}
	t0, t1 := s.tdomain[0], s.tdomain[len(s.tdomain)-1]
	x := s.inv(t0 + f*(t1-t0))
	if s.isTime() {
		return time.UnixMilli(int64(math.Round(x))).In(s.loc)
	}
	return x
}

func (s *continuous) Domain() []any {
	if s.isTime() {
		out := make([]any, len(s.domain))
		for i, v := range s.domain {
			out[i] = time.UnixMilli(int64(math.Round(v))).In(s.loc)
		}
		return out
	}
	return anys(s.domain)
}

func (s *continuous) Range() []any {
	if s.color {
		out := make([]any, len(s.colors))
		for i, c := range s.colors {
			out[i] = c.Hex()
		}
		return out
	}
	return anys(s.rng)
}

func (s *continuous) Bandwidth() float64 { return 0 }

func (s *continuous) Ticks(count float64) []any {
	if count <= 0 || math.IsNaN(count) {
		count = 10
	}
	lo, hi := s.domain[0], s.domain[len(s.domain)-1]
	switch {
	case s.isTime():
		var out []any
		for _, t := range TimeTicks(time.UnixMilli(int64(lo)).In(s.loc), time.UnixMilli(int64(hi)).In(s.loc), count) {
			out = append(out, t)
		}
		return out
	case s.baseType() == "log":
		return anys(logTicks(lo, hi, s.base, count))
	}
	return anys(Ticks(lo, hi, count))
}

// pad grows a domain so that its ends fall px pixels inside the range.
func (s *continuous) pad(d []float64, px float64) []float64 {
	span := math.Abs(s.rng[len(s.rng)-1] - s.rng[0])
	if span <= 2*px {
		return d
	}
	t0, t1 := s.fwd(d[0]), s.fwd(d[1])
	k := (t1 - t0) / (span - 2*px)
	return []float64{s.inv(t0 - px*k), s.inv(t1 + px*k)}
}

// segment returns the index of the piece of the domain that holds t.
func (s *continuous) segment(t float64) int {
	n := len(s.tdomain)
	if n <= 2 {
		return 0
	}
	asc := s.tdomain[n-1] >= s.tdomain[0]
	i := sort.Search(n, func(i int) bool {
		if asc {
			return s.tdomain[i] > t
		}
		return s.tdomain[i] < t
	})
	return max(0, min(n-2, i-1))
}

// isTime reports whether the domain holds times.
func (s *continuous) isTime() bool {
	return s.typ == "time" || s.typ == "utc"
}

// base returns the type of the scale without sequential- or diverging-.
func (s *continuous) baseType() string {
	t := strings.TrimPrefix(s.typ, "sequential-")
	return strings.TrimPrefix(t, "diverging-")
}

// nice extends a domain to round values: to the ticks of count for a
// linear scale, to powers of the base for a log scale, and to an interval
// for a time scale.
func (s *continuous) nice(d []float64, count float64, interval string) []float64 {
	if count <= 0 || math.IsNaN(count) {
		count = 10
	}
	out := slices.Clone(d)
	n := len(out) - 1
	rev := out[n] < out[0]
	lo, hi := out[0], out[n]
	if rev {
		lo, hi = hi, lo
	}
	switch {
	case s.isTime():
		t0, t1 := time.UnixMilli(int64(lo)).In(s.loc), time.UnixMilli(int64(hi)).In(s.loc)
		iv := tickInterval(t0, t1, count)
		if interval != "" {
			iv = timeStep{unit: interval, step: 1}
		}
		lo = float64(iv.floor(t0).UnixMilli())
		hi = float64(iv.ceil(t1).UnixMilli())
	case s.baseType() == "log":
		b := s.base
		if lo > 0 {
			lo = math.Pow(b, math.Floor(math.Log(lo)/math.Log(b)))
			hi = math.Pow(b, math.Ceil(math.Log(hi)/math.Log(b)))
		}
	default:
		lo, hi = Nice(lo, hi, count)
	}
	if rev {
		lo, hi = hi, lo
	}
	out[0], out[n] = lo, hi
	return out
}

// Nice extends lo and hi to multiples of the step of about count ticks.
func Nice(lo, hi, count float64) (float64, float64) {
	prev := math.NaN()
	for range 10 {
		step := TickIncrement(lo, hi, count)
		if step == prev || step == 0 || math.IsNaN(step) || math.IsInf(step, 0) {
			break
		}
		if step > 0 {
			lo = math.Floor(lo/step) * step
			hi = math.Ceil(hi/step) * step
		} else {
			lo = math.Ceil(lo*step) / step
			hi = math.Floor(hi*step) / step
		}
		prev = step
	}
	return lo, hi
}

// TickIncrement returns the step of about count ticks from lo to hi: 1, 2
// or 5 times a power of ten. A step below 1 is returned as the negative of
// its inverse, such as -10 for 0.1, so that it stays exact.
func TickIncrement(lo, hi, count float64) float64 {
	step := (hi - lo) / math.Max(0, count)
	if step <= 0 || math.IsNaN(step) || math.IsInf(step, 0) {
		return math.NaN()
	}
	power := math.Floor(math.Log10(step))
	errv := step / math.Pow(10, power)
	m := 1.0
	switch {
	case errv >= math.Sqrt(50):
		m = 10
	case errv >= math.Sqrt(10):
		m = 5
	case errv >= math.Sqrt(2):
		m = 2
	}
	if power >= 0 {
		return m * math.Pow(10, power)
	}
	return -math.Pow(10, -power) / m
}

// Ticks returns the multiples of the step of about count ticks from lo to
// hi, inclusive.
func Ticks(lo, hi, count float64) []float64 {
	if lo == hi {
		return []float64{lo}
	}
	rev := hi < lo
	if rev {
		lo, hi = hi, lo
	}
	inc := TickIncrement(lo, hi, count)
	if inc == 0 || math.IsNaN(inc) {
		return nil
	}
	var out []float64
	if inc > 0 {
		i0, i1 := math.Ceil(lo/inc), math.Floor(hi/inc)
		for i := i0; i <= i1; i++ {
			out = append(out, i*inc)
		}
	} else {
		inc = -inc
		i0, i1 := math.Ceil(lo*inc), math.Floor(hi*inc)
		for i := i0; i <= i1; i++ {
			out = append(out, i/inc)
		}
	}
	if rev {
		slices.Reverse(out)
	}
	return out
}

// TickStep returns the step between the ticks that Ticks returns, as a
// positive number.
func TickStep(lo, hi, count float64) float64 {
	inc := TickIncrement(math.Min(lo, hi), math.Max(lo, hi), count)
	if inc < 0 {
		return 1 / -inc
	}
	return inc
}

// logTicks returns the ticks of a log scale: the powers of the base, and
// the multiples of each power when there are few powers.
func logTicks(lo, hi, base, count float64) []float64 {
	if lo <= 0 || hi <= 0 {
		return Ticks(lo, hi, count)
	}
	rev := hi < lo
	if rev {
		lo, hi = hi, lo
	}
	i, j := math.Floor(math.Log(lo)/math.Log(base)), math.Ceil(math.Log(hi)/math.Log(base))
	var out []float64
	if j-i < count && base == math.Trunc(base) {
		for p := i; p <= j; p++ {
			pw := math.Pow(base, p)
			for k := 1.0; k < base; k++ {
				t := pw * k
				if t >= lo && t <= hi {
					out = append(out, t)
				}
			}
		}
		if len(out)*2 < int(count) {
			return Ticks(lo, hi, count)
		}
	} else {
		for p := i; p <= j; p++ {
			if t := math.Pow(base, p); t >= lo && t <= hi {
				out = append(out, t)
			}
		}
	}
	if rev {
		slices.Reverse(out)
	}
	return out
}
