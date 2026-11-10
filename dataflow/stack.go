package dataflow

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/format"
)

// stack stacks the values of a field within groups. Positive values stack
// up from zero, and negative values stack down from zero, each from the
// end of the last value of its sign. The offset
// center centers each stack on the middle of the tallest, and normalize
// scales each stack from 0 to 1.
func stack(p *params, rows []Row) ([]Row, error) {
	f, err := p.field("field")
	if err != nil {
		return nil, err
	}
	groupby, err := p.fields("groupby")
	if err != nil {
		return nil, err
	}
	cmp, err := p.sort("sort")
	if err != nil {
		return nil, err
	}
	offset, err := p.str("offset", "zero")
	if err != nil {
		return nil, err
	}
	as, err := p.as("y0", "y1")
	if err != nil {
		return nil, err
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		out[i] = clone(r)
	}
	a := &aggregation{groupby: groupby}
	order, groups, _ := a.group(out)
	sums := make(map[string]float64, len(order))
	maxSum := 0.0
	for _, k := range order {
		g := groups[k]
		if cmp != nil {
			slices.SortStableFunc(g, cmp)
		}
		s := 0.0
		for _, r := range g {
			s += math.Abs(value(f, r))
		}
		sums[k] = s
		maxSum = math.Max(maxSum, s)
	}
	for _, k := range order {
		g := groups[k]
		switch offset {
		case "center":
			y := (maxSum - sums[k]) / 2
			for _, r := range g {
				v := math.Abs(value(f, r))
				r[as[0]], r[as[1]] = y, y+v
				y += v
			}
		case "normalize":
			s := sums[k]
			if s == 0 {
				s = 1
			}
			y := 0.0
			for _, r := range g {
				v := math.Abs(value(f, r)) / s
				r[as[0]], r[as[1]] = y, y+v
				y += v
			}
		default:
			pos, neg := 0.0, 0.0
			for _, r := range g {
				v := value(f, r)
				if v < 0 {
					r[as[0]], r[as[1]] = neg, neg+v
					neg += v
				} else {
					r[as[0]], r[as[1]] = pos, pos+v
					pos += v
				}
			}
		}
	}
	return out, nil
}

// value reads a field as a number, with zero for a value that is not one,
// or 1 when the field is nil.
func value(f *Accessor, r Row) float64 {
	if f == nil {
		return 1
	}
	v := expr.Number(f.Get(r))
	if math.IsNaN(v) {
		return 0
	}
	return v
}

// bin puts the values of a field into bins of an even width, with nice
// edges, and sets the start and the end of the bin of each row.
func bin(p *params, rows []Row) ([]Row, error) {
	f, err := p.field("field")
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, fmt.Errorf("bin without a field: %w", ErrParam)
	}
	b, err := readBins(p)
	if err != nil {
		return nil, err
	}
	as, err := p.as("bin0", "bin1")
	if err != nil {
		return nil, err
	}
	interval, err := p.boolean("interval", true)
	if err != nil {
		return nil, err
	}
	if name, err := p.str("signal", ""); err != nil {
		return nil, err
	} else if name != "" {
		p.env.SetSignal(name, map[string]any{
			"fields": []any{f.Name}, "fname": "bin_" + f.Name,
			"start": b.start, "step": b.step, "stop": b.stop,
		})
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		c := clone(r)
		v := f.Get(r)
		if !valid(v) {
			c[as[0]] = nil
			if interval {
				c[as[1]] = nil
			}
			out[i] = c
			continue
		}
		b0 := b.bin(expr.Number(v))
		c[as[0]] = b0
		if interval {
			c[as[1]] = b0 + b.step
			if math.IsInf(b0, 0) {
				c[as[1]] = b0
			}
		}
		out[i] = c
	}
	return out, nil
}

// bins is the start, the stop and the step of bins.
type bins struct {
	start, stop, step float64
}

// bin returns the start of the bin of a value. A value below the start or
// above the stop is in no bin, and gives an infinity.
func (b bins) bin(v float64) float64 {
	switch {
	case v < b.start:
		return math.Inf(-1)
	case v > b.stop:
		return math.Inf(1)
	}
	i := math.Floor((v-b.start)/b.step + 1e-14)
	b0 := b.start + i*b.step
	if b0 >= b.stop {
		// The stop is the end of the last bin, and not the start of a bin.
		b0 = b.stop - b.step
	}
	return b0
}

// readBins chooses the bins of an extent, as the bin transform of Vega
// documents: a step that is a power of the base, or that power divided by
// one of divide, which gives no more than maxbins bins, at least minstep
// wide. A step or a list of steps that the spec gives wins.
func readBins(p *params) (bins, error) {
	ext, err := p.numbers("extent")
	if err != nil {
		return bins{}, err
	}
	if len(ext) != 2 || math.IsNaN(ext[0]) || math.IsNaN(ext[1]) {
		return bins{}, fmt.Errorf("bin with the extent %v: %w", ext, ErrParam)
	}
	lo, hi := math.Min(ext[0], ext[1]), math.Max(ext[0], ext[1])
	maxbins, err := p.number("maxbins", 20)
	if err != nil {
		return bins{}, err
	}
	base, err := p.number("base", 10)
	if err != nil {
		return bins{}, err
	}
	minstep, err := p.number("minstep", 0)
	if err != nil {
		return bins{}, err
	}
	divide, err := p.numbers("divide")
	if err != nil {
		return bins{}, err
	}
	if divide == nil {
		divide = []float64{5, 2}
	}
	nice, err := p.boolean("nice", true)
	if err != nil {
		return bins{}, err
	}
	step, err := p.number("step", math.NaN())
	if err != nil {
		return bins{}, err
	}
	steps, err := p.numbers("steps")
	if err != nil {
		return bins{}, err
	}
	span, err := p.number("span", math.NaN())
	if err != nil {
		return bins{}, err
	}
	anchor, err := p.number("anchor", math.NaN())
	if err != nil {
		return bins{}, err
	}
	if math.IsNaN(span) || span <= 0 {
		span = hi - lo
		if span == 0 {
			span = math.Abs(lo)
			if span == 0 {
				span = 1
			}
		}
	}
	logb := math.Log(base)
	switch {
	case !math.IsNaN(step):
	case len(steps) > 0:
		step = steps[len(steps)-1]
		for _, s := range steps {
			if s >= minstep && span/s <= maxbins {
				step = s
				break
			}
		}
	default:
		level := math.Ceil(math.Log(maxbins) / logb)
		step = math.Max(minstep, math.Pow(base, math.Round(math.Log(span)/logb)-level))
		for math.Ceil(span/step) > maxbins {
			step *= base
		}
		for _, d := range divide {
			if v := step / d; v >= minstep && span/v <= maxbins {
				step = v
			}
		}
	}
	// The digits after the point of the step, to round the edges.
	v := math.Log(step)
	precision := 0.0
	if v < 0 {
		precision = math.Floor(-v/logb) + 1
	}
	eps := math.Pow(base, -precision-1)
	start, stop := lo, hi
	if nice {
		v := math.Floor(lo/step+eps) * step
		if lo < v {
			v -= step
		}
		start = v
		stop = math.Ceil(hi/step) * step
	}
	// The stop is a whole number of steps after the start, as the bin
	// transform of Vega makes it, which keeps the greatest value inside
	// the last bin when the division of the span rounds down.
	stop = start + math.Ceil((stop-start)/step)*step
	if !math.IsNaN(anchor) {
		d := math.Mod(anchor-start, step)
		if d != 0 {
			if d > 0 {
				d -= step
			}
			start += d
			stop = start + math.Ceil((hi-start)/step)*step
		}
	}
	if stop <= start {
		stop = start + step
	}
	return bins{start: start, stop: stop, step: step}, nil
}

// timeUnit sets the start and the end of the unit of time of a date
// field, such as the month or the year and month.
func timeUnit(p *params, rows []Row) ([]Row, error) {
	f, err := p.field("field")
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, fmt.Errorf("timeunit without a field: %w", ErrParam)
	}
	units, err := p.strings("units")
	if err != nil {
		return nil, err
	}
	if len(units) == 0 {
		return nil, fmt.Errorf("timeunit without units: %w", ErrParam)
	}
	for _, u := range units {
		if !format.Interval(u).Valid() {
			return nil, fmt.Errorf("the time unit %q: %w", u, ErrParam)
		}
	}
	tz, err := p.str("timezone", "local")
	if err != nil {
		return nil, err
	}
	step, err := p.number("step", 1)
	if err != nil {
		return nil, err
	}
	as, err := p.as("unit0", "unit1")
	if err != nil {
		return nil, err
	}
	interval, err := p.boolean("interval", true)
	if err != nil {
		return nil, err
	}
	loc := p.env.Location()
	if tz == "utc" {
		loc = time.UTC
	}
	floor := TimeFloor(units, int(step), loc)
	smallest := smallestUnit(units)
	out := make([]Row, len(rows))
	for i, r := range rows {
		c := clone(r)
		t, ok := asTime(f.Get(r), loc)
		if !ok {
			c[as[0]] = nil
			if interval {
				c[as[1]] = nil
			}
			out[i] = c
			continue
		}
		t0 := floor(t)
		c[as[0]] = t0
		if interval {
			c[as[1]] = smallest.Offset(t0, int(step))
		}
		out[i] = c
	}
	return out, nil
}

// asTime reads a value as a time: a date, or milliseconds since 1970.
func asTime(v any, loc *time.Location) (time.Time, bool) {
	switch x := v.(type) {
	case time.Time:
		return x.In(loc), !x.IsZero()
	case nil:
		return time.Time{}, false
	}
	if expr.IsUndefined(v) {
		return time.Time{}, false
	}
	if s, ok := v.(string); ok {
		t, ok := format.ParseDate(s, loc)
		return t, ok
	}
	f := expr.Number(v)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return time.Time{}, false
	}
	return time.UnixMilli(int64(f)).In(loc), true
}

// smallestUnit returns the shortest of the units.
func smallestUnit(units []string) format.Interval {
	best := format.Year
	bi := 0
	for _, u := range units {
		i := slices.Index(format.Intervals, format.Interval(u))
		if i > bi {
			best, bi = format.Interval(u), i
		}
	}
	return best
}

// TimeFloor returns a function that floors a time to a set of units, as
// the timeunit transform of Vega documents. The fields of the units stay,
// and every other field takes the start of its range, with the year 2012
// when the units have no year. With day, the date is the day of the week
// of the first week of 2012, which starts on a Sunday. With step, the
// smallest unit floors to a multiple of step.
func TimeFloor(units []string, step int, loc *time.Location) func(time.Time) time.Time {
	has := func(u format.Interval) bool { return slices.Contains(units, string(u)) }
	step = max(1, step)
	small := smallestUnit(units)
	floorStep := func(v, base int, u format.Interval) int {
		if u != small || step == 1 {
			return v
		}
		return base + (v-base)/step*step
	}
	return func(t time.Time) time.Time {
		year, month, day := 2012, 1, 1
		hour, minute, sec, ms := 0, 0, 0, 0
		if has(format.Year) {
			year = floorStep(t.Year(), 0, format.Year)
		}
		if has(format.Quarter) {
			month = (int(t.Month())-1)/3*3 + 1
			month = floorStep(month-1, 0, format.Quarter) + 1
		}
		if has(format.Month) {
			month = floorStep(int(t.Month())-1, 0, format.Month) + 1
		}
		switch {
		case has(format.Week):
			// The Sunday that starts the week, counted from the first
			// Sunday of the year.
			w := format.Week.Floor(t)
			if !has(format.Year) {
				w = w.AddDate(2012-w.Year(), 0, 0)
			}
			year, month, day = w.Year(), int(w.Month()), w.Day()
		case has(format.Date):
			day = floorStep(t.Day(), 1, format.Date)
		case has(format.Day):
			day = 1 + int(t.Weekday())
			if has(format.Year) || has(format.Month) {
				// The day of the week within its own week.
				day = t.Day()
			}
		case has(format.DayOfYear):
			month, day = 1, floorStep(t.YearDay(), 1, format.DayOfYear)
		}
		if has(format.Hours) {
			hour = floorStep(t.Hour(), 0, format.Hours)
		}
		if has(format.Minutes) {
			minute = floorStep(t.Minute(), 0, format.Minutes)
		}
		if has(format.Seconds) {
			sec = floorStep(t.Second(), 0, format.Seconds)
		}
		if has(format.Milliseconds) {
			ms = floorStep(t.Nanosecond()/int(time.Millisecond), 0, format.Milliseconds)
		}
		return format.WallTime(year, month, day, hour, minute, sec, ms, loc)
	}
}
