package scale

import (
	"math"
	"time"

	"github.com/xo/zuzu/format"
)

// timeStep is a unit of time and a number of units, such as 3 months.
type timeStep struct {
	unit string
	step int
	ms   float64
}

// timeSteps are the steps of the ticks of a time scale, from the shortest.
var timeSteps = []timeStep{
	{"seconds", 1, 1e3}, {"seconds", 5, 5e3}, {"seconds", 15, 15e3}, {"seconds", 30, 30e3},
	{"minutes", 1, 6e4}, {"minutes", 5, 3e5}, {"minutes", 15, 9e5}, {"minutes", 30, 18e5},
	{"hours", 1, 36e5}, {"hours", 3, 108e5}, {"hours", 6, 216e5}, {"hours", 12, 432e5},
	{"date", 1, 864e5}, {"date", 2, 1728e5}, {"week", 1, 6048e5},
	{"month", 1, 2592e6}, {"month", 3, 7776e6}, {"year", 1, 31536e6},
}

// tickInterval chooses the step of about count ticks from t0 to t1: the
// step of timeSteps nearest, by ratio, to the span divided by count. A
// span longer than the longest step takes years in steps of 1, 2 or 5
// times a power of ten, and a span shorter than a second takes
// milliseconds.
func tickInterval(t0, t1 time.Time, count float64) timeStep {
	target := math.Abs(float64(t1.Sub(t0).Milliseconds())) / count
	i := 0
	for i < len(timeSteps) && timeSteps[i].ms <= target {
		i++
	}
	switch {
	case i == len(timeSteps):
		y0 := float64(t0.Year()) + float64(t0.YearDay())/366
		y1 := float64(t1.Year()) + float64(t1.YearDay())/366
		return timeStep{unit: "year", step: int(math.Max(1, TickStep(y0, y1, count))), ms: 31536e6}
	case i == 0:
		ms := float64(t1.Sub(t0).Milliseconds())
		return timeStep{unit: "milliseconds", step: int(math.Max(1, TickStep(0, ms, count))), ms: 1}
	}
	if target/timeSteps[i-1].ms < timeSteps[i].ms/target {
		return timeSteps[i-1]
	}
	return timeSteps[i]
}

// field returns the value that the step counts in multiples of: the year,
// the month from 0, the day of the month from 0, the hour, and so on.
func (s timeStep) field(t time.Time) int {
	switch s.unit {
	case "year":
		return t.Year()
	case "month":
		return int(t.Month()) - 1
	case "date":
		return t.Day() - 1
	case "hours":
		return t.Hour()
	case "minutes":
		return t.Minute()
	case "seconds":
		return t.Second()
	case "milliseconds":
		return t.Nanosecond() / int(time.Millisecond)
	}
	return 0
}

// on reports whether a start of a unit is on a multiple of the step.
func (s timeStep) on(t time.Time) bool {
	return s.step <= 1 || s.field(t)%s.step == 0
}

// floor returns the start of the step that holds t.
func (s timeStep) floor(t time.Time) time.Time {
	iv := format.Interval(s.unit)
	f := iv.Floor(t)
	for range 1000 {
		if s.on(f) {
			return f
		}
		f = iv.Floor(iv.Offset(f, -1))
	}
	return f
}

// ceil returns the start of the first step at t or after.
func (s timeStep) ceil(t time.Time) time.Time {
	f := s.floor(t)
	if f.Equal(t) {
		return f
	}
	iv := format.Interval(s.unit)
	n := iv.Ceil(t)
	for range 1000 {
		if s.on(n) {
			return n
		}
		n = iv.Floor(iv.Offset(n, 1))
	}
	return n
}

// TimeTicks returns about count times from t0 to t1, inclusive, at the
// starts of a unit of time, such as each month or every 6 hours.
func TimeTicks(t0, t1 time.Time, count float64) []time.Time {
	rev := t1.Before(t0)
	if rev {
		t0, t1 = t1, t0
	}
	s := tickInterval(t0, t1, count)
	iv := format.Interval(s.unit)
	var out []time.Time
	for t := iv.Ceil(t0); !t.After(t1) && len(out) < 10000; {
		if s.on(t) {
			out = append(out, t)
		}
		next := iv.Floor(iv.Offset(t, 1))
		if !next.After(t) {
			break
		}
		t = next
	}
	if rev {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out
}

// TimeTickUnit returns the unit and the step of the ticks that TimeTicks
// returns, so that an axis can choose a format for them.
func TimeTickUnit(t0, t1 time.Time, count float64) (string, int) {
	if t1.Before(t0) {
		t0, t1 = t1, t0
	}
	s := tickInterval(t0, t1, count)
	return s.unit, s.step
}

// intervalUnits are the units of time of the tickCount of Vega, by the
// names of the intervals of format.
var intervalUnits = map[string]string{
	"millisecond": "milliseconds", "second": "seconds", "minute": "minutes", "hour": "hours",
	"day": "date", "week": "week", "month": "month", "year": "year",
}

// IntervalTicks returns the times from t0 to t1, inclusive, at the start
// of every step of a unit, as a tickCount of Vega such as {"interval":
// "week", "step": 2} asks for. A day and a unit below it count from the
// start of the next unit up, as d3 counts them, and a week counts from the
// first tick. It returns false for a unit that Vega does not name.
func IntervalTicks(t0, t1 time.Time, unit string, step int) ([]time.Time, bool) {
	name, ok := intervalUnits[unit]
	if !ok {
		return nil, false
	}
	if t1.Before(t0) {
		t0, t1 = t1, t0
	}
	step = max(step, 1)
	s := timeStep{unit: name, step: step}
	iv := format.Interval(name)
	var out []time.Time
	n := 0
	for t := iv.Ceil(t0); !t.After(t1) && len(out) < 10000; {
		if name == "week" && n%step == 0 || name != "week" && s.on(t) {
			out = append(out, t)
		}
		n++
		next := iv.Floor(iv.Offset(t, 1))
		if !next.After(t) {
			break
		}
		t = next
	}
	return out, true
}
