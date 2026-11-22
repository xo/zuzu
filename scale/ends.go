package scale

import (
	"math"
	"slices"
)

// endFactors are the factors of a power of ten that a step of EndTicks can
// take, the roundest first. 1, 2 and 5 are the steps of Ticks. The others
// let a step land on both ends of a domain that those miss, such as 15,000
// for a domain from 0 to 45,000.
var endFactors = []float64{1, 2, 5, 2.5, 4, 3, 1.5, 6, 8}

// EndTicks returns ticks from lo to hi whose first tick is lo and whose
// last tick is hi, near count, and no more than most or one more than
// count, so that both ends of a nice domain get a tick (D46). It also returns the step whose
// precision the labels of the ticks need, which is finer than the step
// between the ticks for a step such as 2.5. It returns nil when no round
// step lands on both ends.
//
// EndTicks takes the ticks of Ticks when they land on both ends and are no
// more than most. Then it takes a step of three ticks or more, and at most
// most, whose multiples from zero land on both ends, such as 0, 15 and 30
// for a domain from 0 to 30. Then, for a nice domain, it takes such a step
// that lands on both ends from lo, such as 50, 200, 350 and 500 for a
// domain from 50 to 500. A domain is nice when Nice with a count of 10, as
// a scale of Vega makes it nice by default, leaves it as it is. Then it
// takes the ticks of Ticks when they land on both ends, though they are
// more than most. Last, it takes the two ends alone, unless count asks for
// three ticks or more and Ticks gives three or more, when it returns nil. Of
// two steps from zero, it takes the one whose number of ticks is nearer
// count, then the rounder one. Of two steps from lo, whose labels are less
// round, it takes the one with three ticks or more, then the rounder one,
// then the one whose number of ticks is nearer count.
func EndTicks(lo, hi, count float64, most int) ([]float64, float64) {
	if lo == hi || !finite(lo) || !finite(hi) {
		return nil, 0
	}
	rev := hi < lo
	if rev {
		lo, hi = hi, lo
	}
	// A short axis asks for few ticks, and takes at most one more.
	most = max(2, min(most, int(math.Floor(count))+1))
	out := func(ticks []float64, step float64) ([]float64, float64) {
		if rev {
			slices.Reverse(ticks)
		}
		return ticks, step
	}
	span := hi - lo
	std := Ticks(lo, hi, count)
	stdEnds := len(std) >= 2 && near(std[0], lo, span) && near(std[len(std)-1], hi, span)
	if stdEnds && len(std) <= most {
		return out(std, TickStep(lo, hi, count))
	}
	target := math.Max(2, math.Min(count, float64(most)))
	var round, offset *endStep
	e0 := int(math.Floor(math.Log10(span)))
	// The span itself is the last step, with a tick at each end.
	spanFactor := span / math.Pow10(e0)
	for e := e0 - 1; e <= e0; e++ {
		factors := endFactors
		if e == e0 {
			factors = append(slices.Clip(endFactors), spanFactor)
		}
		for rank, f := range factors {
			c := newEndStep(lo, hi, f, e, rank)
			if c == nil || c.n > most {
				continue
			}
			switch {
			case c.fromZero:
				if c.better(round, target) {
					round = c
				}
			default:
				if c.rounder(offset, target) {
					offset = c
				}
			}
		}
	}
	nlo, nhi := Nice(lo, hi, 10)
	nice := near(nlo, lo, span) && near(nhi, hi, span)
	if !nice {
		offset = nil
	}
	switch {
	case round != nil && round.n >= 3:
		return out(round.ticks(), round.precision())
	case offset != nil && offset.n >= 3:
		return out(offset.ticks(), offset.precision())
	case stdEnds:
		return out(std, TickStep(lo, hi, count))
	case len(std) >= 3 && count >= 3:
		// The ends alone say less than the ticks of Ticks, such as 0,
		// 0.2 to 1.0 for a domain from 0 to 1.1, so an axis with room
		// for three ticks takes those.
		return nil, 0
	case round != nil:
		return out(round.ticks(), round.precision())
	case offset != nil:
		return out(offset.ticks(), offset.precision())
	}
	return nil, 0
}

// endStep is a step of EndTicks: a factor f of the power of ten 10^e,
// with the number of ticks n from lo to hi. fromZero reports whether lo and
// hi are multiples of the step.
type endStep struct {
	lo, hi   float64
	f        float64
	e        int
	rank, n  int
	fromZero bool
}

// newEndStep returns the step f times 10^e from lo to hi, or nil when it
// does not land on hi from lo.
func newEndStep(lo, hi, f float64, e, rank int) *endStep {
	step := f * math.Pow10(e)
	q := (hi - lo) / step
	if math.Abs(q-math.Round(q)) > 1e-9*math.Max(1, q) || math.Round(q) < 1 {
		return nil
	}
	return &endStep{
		lo: lo, hi: hi, f: f, e: e, rank: rank, n: int(math.Round(q)) + 1,
		fromZero: whole(lo/step) && whole(hi/step),
	}
}

// rounder reports whether the step is better than another, which can be
// nil, for a step from lo: it has three ticks or more, or it is rounder, or
// its number of ticks is nearer the target.
func (c *endStep) rounder(o *endStep, target float64) bool {
	if o == nil {
		return true
	}
	if (c.n >= 3) != (o.n >= 3) {
		return c.n >= 3
	}
	if c.rank != o.rank {
		return c.rank < o.rank
	}
	return c.better(o, target)
}

// better reports whether the step is better than another, which can be
// nil: its number of ticks is nearer the target, or as near and rounder.
func (c *endStep) better(o *endStep, target float64) bool {
	if o == nil {
		return true
	}
	dc, do := math.Abs(float64(c.n)-target), math.Abs(float64(o.n)-target)
	if dc != do {
		return dc < do
	}
	return c.rank < o.rank
}

// ticks returns the ticks of the step, each written as exactly as a
// decimal allows, as Ticks writes its ticks.
func (c *endStep) ticks() []float64 {
	digits := max(0, 1-c.e)
	scale := math.Pow10(digits)
	out := make([]float64, c.n)
	for i := range out {
		v := c.lo + float64(i)*c.f*math.Pow10(c.e)
		out[i] = math.Round(v*scale) / scale
	}
	out[0], out[len(out)-1] = c.lo, c.hi
	return out
}

// precision returns the step whose precision the labels of the ticks need:
// the step for a whole factor, and a tenth of it for a factor such as 2.5.
func (c *endStep) precision() float64 {
	step := c.f * math.Pow10(c.e)
	if !whole(c.f) {
		step /= 10
	}
	return step
}

// whole reports whether a number is a whole number, within the error of a
// division of floats.
func whole(x float64) bool {
	return math.Abs(x-math.Round(x)) <= 1e-9*math.Max(1, math.Abs(x))
}

// near reports whether two numbers are the same, within a small part of a
// span.
func near(a, b, span float64) bool {
	return math.Abs(a-b) <= 1e-9*span
}

// finite reports whether a number is neither NaN nor infinite.
func finite(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0)
}
