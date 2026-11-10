package format

import (
	"slices"
	"time"
)

// Interval is a unit of time, such as a month or an hour, in the location
// of the times it takes. It has the names of the time units of Vega: year,
// quarter, month, week, date, day, dayofyear, hours, minutes, seconds and
// milliseconds. date, day and dayofyear are the same interval: a day.
// week starts on Sunday.
type Interval string

// The intervals.
const (
	Year         Interval = "year"
	Quarter      Interval = "quarter"
	Month        Interval = "month"
	Week         Interval = "week"
	Date         Interval = "date"
	Day          Interval = "day"
	DayOfYear    Interval = "dayofyear"
	Hours        Interval = "hours"
	Minutes      Interval = "minutes"
	Seconds      Interval = "seconds"
	Milliseconds Interval = "milliseconds"
)

// Intervals are the intervals, from the longest to the shortest.
var Intervals = []Interval{Year, Quarter, Month, Week, Date, Day, DayOfYear, Hours, Minutes, Seconds, Milliseconds}

// Valid reports whether i is an interval.
func (i Interval) Valid() bool {
	return slices.Contains(Intervals, i)
}

// Floor returns the start of the interval that holds t.
func (i Interval) Floor(t time.Time) time.Time {
	loc := t.Location()
	y, m, d := t.Date()
	switch i {
	case Year:
		return WallTime(y, 1, 1, 0, 0, 0, 0, loc)
	case Quarter:
		return WallTime(y, int(m-1)/3*3+1, 1, 0, 0, 0, 0, loc)
	case Month:
		return WallTime(y, int(m), 1, 0, 0, 0, 0, loc)
	case Week:
		return WallTime(y, int(m), d-int(t.Weekday()), 0, 0, 0, 0, loc)
	case Date, Day, DayOfYear:
		return WallTime(y, int(m), d, 0, 0, 0, 0, loc)
	}
	// The shorter intervals take away the fields below them from the
	// instant, so that a zone with an odd offset floors on its own clock.
	ns := time.Duration(t.Nanosecond())
	switch i {
	case Hours:
		ns += time.Duration(t.Minute())*time.Minute + time.Duration(t.Second())*time.Second
	case Minutes:
		ns += time.Duration(t.Second()) * time.Second
	case Seconds:
	default:
		ns %= time.Millisecond
	}
	return t.Add(-ns)
}

// Offset returns t moved by n intervals. A month, a quarter or a year keeps
// the day of the month, and a day past the end of the month carries into
// the next. A day or a week keeps the time of day.
func (i Interval) Offset(t time.Time, n int) time.Time {
	switch i {
	case Year:
		return t.AddDate(n, 0, 0)
	case Quarter:
		return t.AddDate(0, 3*n, 0)
	case Month:
		return t.AddDate(0, n, 0)
	case Week:
		return t.AddDate(0, 0, 7*n)
	case Date, Day, DayOfYear:
		return t.AddDate(0, 0, n)
	case Hours:
		return t.Add(time.Duration(n) * time.Hour)
	case Minutes:
		return t.Add(time.Duration(n) * time.Minute)
	case Seconds:
		return t.Add(time.Duration(n) * time.Second)
	}
	return t.Add(time.Duration(n) * time.Millisecond)
}

// Ceil returns the start of the first interval that starts at t or after.
func (i Interval) Ceil(t time.Time) time.Time {
	f := i.Floor(t)
	if f.Equal(t) {
		return f
	}
	return i.Floor(i.Offset(f, 1))
}

// Range returns the starts of intervals from start, inclusive, to stop,
// exclusive, with every step-th interval. A step below 1 is 1.
func (i Interval) Range(start, stop time.Time, step int) []time.Time {
	step = max(1, step)
	var out []time.Time
	for t := i.Ceil(start); t.Before(stop); {
		out = append(out, t)
		next := i.Floor(i.Offset(t, step))
		if !next.After(t) {
			break
		}
		t = next
	}
	return out
}
