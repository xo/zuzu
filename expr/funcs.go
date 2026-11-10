package expr

import (
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/xo/zuzu/format"
)

// ErrArgument is the error of a function called with an argument that it
// cannot use, such as a format spec that does not parse.
const ErrArgument Error = "invalid argument"

// RegExp is a regular expression, which the function regexp makes. Go reads
// the pattern, so a pattern that only JavaScript reads, such as one with a
// lookahead, is an error.
type RegExp struct {
	*regexp.Regexp

	Source string
	Global bool
}

// builtins are the functions of the expression language of Vega that need
// nothing from a view.
var builtins = makeBuiltins()

// makeBuiltins makes the table of builtins.
func makeBuiltins() map[string]Func {
	builtins := map[string]Func{}
	for name, fn := range mathFuncs {
		builtins[name] = num1(fn)
	}
	maps.Copy(builtins, map[string]Func{
		// Type checks.
		"isArray":   is(func(v any) bool { _, ok := v.([]any); return ok }),
		"isBoolean": is(func(v any) bool { _, ok := v.(bool); return ok }),
		"isDate":    is(func(v any) bool { _, ok := v.(time.Time); return ok }),
		"isDefined": is(func(v any) bool { return !IsUndefined(v) }),
		"isNumber":  is(func(v any) bool { return typeOf(v) == "number" }),
		"isObject":  is(isObject),
		"isRegExp":  is(func(v any) bool { _, ok := v.(*RegExp); return ok }),
		"isString":  is(func(v any) bool { _, ok := v.(string); return ok }),
		"isValid": is(func(v any) bool {
			f, ok := v.(float64)
			return v != nil && !IsUndefined(v) && (!ok || !math.IsNaN(f))
		}),
		// isNaN and isFinite do not convert, as Number.isNaN and
		// Number.isFinite do not.
		"isNaN": is(func(v any) bool { f, ok := v.(float64); return ok && math.IsNaN(f) }),
		"isFinite": is(func(v any) bool {
			f, ok := v.(float64)
			return ok && !math.IsNaN(f) && !math.IsInf(f, 0)
		}),

		// Conversions.
		"toBoolean": toBoolean,
		"toDate":    toDate,
		"toNumber": func(_ *Scope, args []any) (any, error) {
			v := arg(args, 0)
			if isEmpty(v) {
				return nil, nil
			}
			return Number(v), nil
		},
		"toString": func(_ *Scope, args []any) (any, error) {
			v := arg(args, 0)
			if isEmpty(v) {
				return nil, nil
			}
			return String(v), nil
		},

		// Math with more than one argument.
		"atan2": func(_ *Scope, args []any) (any, error) {
			return math.Atan2(numArg(args, 0), numArg(args, 1)), nil
		},
		"pow": func(_ *Scope, args []any) (any, error) {
			return math.Pow(numArg(args, 0), numArg(args, 1)), nil
		},
		"hypot": func(_ *Scope, args []any) (any, error) {
			h := 0.0
			for _, a := range args {
				h = math.Hypot(h, Number(a))
			}
			return h, nil
		},
		"min": func(_ *Scope, args []any) (any, error) {
			return extreme(args, -1), nil
		},
		"max": func(_ *Scope, args []any) (any, error) {
			return extreme(args, 1), nil
		},
		"clamp": func(_ *Scope, args []any) (any, error) {
			v, lo, hi := numArg(args, 0), numArg(args, 1), numArg(args, 2)
			return math.Max(lo, math.Min(hi, v)), nil
		},
		"random": func(s *Scope, _ []any) (any, error) {
			return s.random(), nil
		},

		// Statistics.
		"sampleNormal": func(s *Scope, args []any) (any, error) {
			mean, sd := normalArgs(args, 0)
			return mean + sd*boxMuller(s), nil
		},
		"cumulativeNormal": func(_ *Scope, args []any) (any, error) {
			mean, sd := normalArgs(args, 1)
			return 0.5 * math.Erfc(-(numArg(args, 0)-mean)/(sd*math.Sqrt2)), nil
		},
		"densityNormal": func(_ *Scope, args []any) (any, error) {
			mean, sd := normalArgs(args, 1)
			z := (numArg(args, 0) - mean) / sd
			return math.Exp(-z*z/2) / (sd * math.Sqrt(2*math.Pi)), nil
		},
		"quantileNormal": func(_ *Scope, args []any) (any, error) {
			mean, sd := normalArgs(args, 1)
			p := numArg(args, 0)
			if p <= 0 || p >= 1 {
				return nan(), nil
			}
			return mean + sd*math.Sqrt2*math.Erfinv(2*p-1), nil
		},
		"sampleLogNormal": func(s *Scope, args []any) (any, error) {
			mean, sd := normalArgs(args, 0)
			return math.Exp(mean + sd*boxMuller(s)), nil
		},
		"cumulativeLogNormal": func(_ *Scope, args []any) (any, error) {
			mean, sd := normalArgs(args, 1)
			v := numArg(args, 0)
			if v <= 0 {
				return 0.0, nil
			}
			return 0.5 * math.Erfc(-(math.Log(v)-mean)/(sd*math.Sqrt2)), nil
		},
		"densityLogNormal": func(_ *Scope, args []any) (any, error) {
			mean, sd := normalArgs(args, 1)
			v := numArg(args, 0)
			if v <= 0 {
				return 0.0, nil
			}
			z := (math.Log(v) - mean) / sd
			return math.Exp(-z*z/2) / (sd * v * math.Sqrt(2*math.Pi)), nil
		},
		"quantileLogNormal": func(_ *Scope, args []any) (any, error) {
			mean, sd := normalArgs(args, 1)
			p := numArg(args, 0)
			if p <= 0 || p >= 1 {
				return nan(), nil
			}
			return math.Exp(mean + sd*math.Sqrt2*math.Erfinv(2*p-1)), nil
		},
		"sampleUniform": func(s *Scope, args []any) (any, error) {
			lo, hi := uniformArgs(args, 0)
			return lo + (hi-lo)*s.random(), nil
		},
		"cumulativeUniform": func(_ *Scope, args []any) (any, error) {
			lo, hi := uniformArgs(args, 1)
			v := numArg(args, 0)
			return math.Max(0, math.Min(1, (v-lo)/(hi-lo))), nil
		},
		"densityUniform": func(_ *Scope, args []any) (any, error) {
			lo, hi := uniformArgs(args, 1)
			v := numArg(args, 0)
			if v < lo || v > hi {
				return 0.0, nil
			}
			return 1 / (hi - lo), nil
		},
		"quantileUniform": func(_ *Scope, args []any) (any, error) {
			lo, hi := uniformArgs(args, 1)
			p := numArg(args, 0)
			if p < 0 || p > 1 {
				return nan(), nil
			}
			return lo + p*(hi-lo), nil
		},

		// Dates and times.
		"now": func(_ *Scope, _ []any) (any, error) {
			return float64(time.Now().UnixMilli()), nil
		},
		"datetime": func(s *Scope, args []any) (any, error) {
			// One argument is a date, milliseconds since 1970, or a string
			// that Date reads, as new Date takes one argument.
			read := dateOf
			if len(args) == 1 {
				read = func(args []any, loc *time.Location) (time.Time, bool) { return timeArg(args, 0, loc) }
			}
			t, ok := read(args, s.location())
			if !ok {
				return invalidDate, nil
			}
			return t, nil
		},
		"utc": func(_ *Scope, args []any) (any, error) {
			t, ok := dateOf(args, time.UTC)
			if !ok {
				return nan(), nil
			}
			return float64(t.UnixMilli()), nil
		},
		"time": func(_ *Scope, args []any) (any, error) {
			return Number(arg(args, 0)), nil
		},
		"timezoneoffset": func(s *Scope, args []any) (any, error) {
			t, ok := timeArg(args, 0, s.location())
			if !ok {
				return nan(), nil
			}
			_, off := t.Zone()
			return float64(-off / 60), nil
		},
		"timeOffset": func(s *Scope, args []any) (any, error) {
			return offset(args, s.location())
		},
		"utcOffset": func(_ *Scope, args []any) (any, error) {
			return offset(args, time.UTC)
		},
		"timeSequence": func(s *Scope, args []any) (any, error) {
			return sequenceOf(args, s.location())
		},
		"utcSequence": func(_ *Scope, args []any) (any, error) {
			return sequenceOf(args, time.UTC)
		},

		// Formats.
		"format": func(s *Scope, args []any) (any, error) {
			f, err := format.Number(String(arg(args, 1)), *s.locale())
			if err != nil {
				return nil, fmt.Errorf("calling format: %w: %w", ErrArgument, err)
			}
			return f(Number(arg(args, 0))), nil
		},
		"timeFormat": func(s *Scope, args []any) (any, error) {
			return timeFormat(args, s.location(), s.locale())
		},
		"utcFormat": func(s *Scope, args []any) (any, error) {
			return timeFormat(args, time.UTC, s.locale())
		},
		"timeParse": func(s *Scope, args []any) (any, error) {
			return timeParse(args, s.location(), s.locale())
		},
		"utcParse": func(s *Scope, args []any) (any, error) {
			return timeParse(args, time.UTC, s.locale())
		},
		"timeUnitSpecifier": func(_ *Scope, args []any) (any, error) {
			var units []string
			switch u := arg(args, 0).(type) {
			case []any:
				for _, x := range u {
					units = append(units, String(x))
				}
			case string:
				units = []string{u}
			}
			specs := map[string]string{}
			if m, ok := arg(args, 1).(map[string]any); ok {
				for k, v := range m {
					specs[k] = String(v)
				}
			}
			return format.TimeUnitSpecifier(units, specs), nil
		},
		"dayFormat": func(s *Scope, args []any) (any, error) {
			return name(s.locale().Days[:], args), nil
		},
		"dayAbbrevFormat": func(s *Scope, args []any) (any, error) {
			return name(s.locale().ShortDays[:], args), nil
		},
		"monthFormat": func(s *Scope, args []any) (any, error) {
			return name(s.locale().Months[:], args), nil
		},
		"monthAbbrevFormat": func(s *Scope, args []any) (any, error) {
			return name(s.locale().ShortMonths[:], args), nil
		},

		// Strings and arrays.
		"length": func(_ *Scope, args []any) (any, error) {
			return get(arg(args, 0), lengthProp), nil
		},
		"indexof": func(_ *Scope, args []any) (any, error) {
			return indexOf(arg(args, 0), arg(args, 1), false), nil
		},
		"lastindexof": func(_ *Scope, args []any) (any, error) {
			return indexOf(arg(args, 0), arg(args, 1), true), nil
		},
		"slice":     sliceFunc,
		"substring": substring,
		"upper": func(_ *Scope, args []any) (any, error) {
			return strings.ToUpper(String(arg(args, 0))), nil
		},
		"lower": func(_ *Scope, args []any) (any, error) {
			return strings.ToLower(String(arg(args, 0))), nil
		},
		"trim": func(_ *Scope, args []any) (any, error) {
			return strings.TrimSpace(String(arg(args, 0))), nil
		},
		"pad":        pad,
		"truncate":   truncate,
		"replace":    replace,
		"split":      split,
		"parseFloat": parseFloat,
		"parseInt":   parseInt,
		"join": func(_ *Scope, args []any) (any, error) {
			a, _ := arg(args, 0).([]any)
			sep := ","
			if v := arg(args, 1); !IsUndefined(v) {
				sep = String(v)
			}
			return joinWith(a, sep), nil
		},
		"reverse": func(_ *Scope, args []any) (any, error) {
			a, _ := arg(args, 0).([]any)
			out := slices.Clone(a)
			slices.Reverse(out)
			return out, nil
		},
		"sort": func(_ *Scope, args []any) (any, error) {
			a, _ := arg(args, 0).([]any)
			out := slices.Clone(a)
			slices.SortStableFunc(out, Ascending)
			return out, nil
		},
		"extent": func(_ *Scope, args []any) (any, error) {
			a, _ := arg(args, 0).([]any)
			lo, hi := Undefined, Undefined
			for _, v := range a {
				f := Number(v)
				if v == nil || IsUndefined(v) || math.IsNaN(f) {
					continue
				}
				if IsUndefined(lo) || f < Number(lo) {
					lo = f
				}
				if IsUndefined(hi) || f > Number(hi) {
					hi = f
				}
			}
			return []any{lo, hi}, nil
		},
		"span": func(_ *Scope, args []any) (any, error) {
			a, _ := arg(args, 0).([]any)
			if len(a) == 0 {
				return 0.0, nil
			}
			return Number(a[len(a)-1]) - Number(a[0]), nil
		},
		"peek": func(_ *Scope, args []any) (any, error) {
			switch a := arg(args, 0).(type) {
			case []any:
				if len(a) > 0 {
					return a[len(a)-1], nil
				}
			case string:
				return get(a, Number(get(a, lengthProp))-1), nil
			}
			return Undefined, nil
		},
		"lerp": func(_ *Scope, args []any) (any, error) {
			a, _ := arg(args, 0).([]any)
			if len(a) == 0 {
				return nan(), nil
			}
			f := numArg(args, 1)
			lo, hi := Number(a[0]), Number(a[len(a)-1])
			return lo + f*(hi-lo), nil
		},
		"inrange": func(_ *Scope, args []any) (any, error) {
			v := numArg(args, 0)
			r, _ := arg(args, 1).([]any)
			if len(r) == 0 {
				return false, nil
			}
			lo, hi := Number(r[0]), Number(r[len(r)-1])
			if lo > hi {
				lo, hi = hi, lo
			}
			return lo <= v && v <= hi, nil
		},
		"clampRange": func(_ *Scope, args []any) (any, error) {
			r, _ := arg(args, 0).([]any)
			if len(r) < 2 {
				return r, nil
			}
			lo, hi := numArg(args, 1), numArg(args, 2)
			a, b := Number(r[0]), Number(r[1])
			if b < a {
				a, b = b, a
			}
			span := b - a
			if span >= hi-lo {
				return []any{lo, hi}, nil
			}
			a = math.Min(math.Max(a, lo), hi-span)
			out := []any{a, a + span}
			if Number(r[1]) < Number(r[0]) {
				out[0], out[1] = out[1], out[0]
			}
			return out, nil
		},
		"sequence": func(_ *Scope, args []any) (any, error) {
			start, stop, step := 0.0, numArg(args, 0), 1.0
			if len(args) > 1 {
				start, stop = numArg(args, 0), numArg(args, 1)
			}
			if len(args) > 2 {
				step = numArg(args, 2)
			}
			n := math.Ceil((stop - start) / step)
			if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
				return []any{}, nil
			}
			out := make([]any, int(n))
			for i := range out {
				out[i] = start + float64(i)*step
			}
			return out, nil
		},
		"merge": func(_ *Scope, args []any) (any, error) {
			out := map[string]any{}
			for _, a := range args {
				if m, ok := a.(map[string]any); ok {
					maps.Copy(out, m)
				}
			}
			return out, nil
		},

		// Regular expressions.
		"regexp": func(_ *Scope, args []any) (any, error) {
			return newRegExp(String(arg(args, 0)), flags(arg(args, 1)))
		},
		"test": func(_ *Scope, args []any) (any, error) {
			re, ok := arg(args, 0).(*RegExp)
			if !ok {
				return false, nil
			}
			return re.MatchString(String(arg(args, 1))), nil
		},
	})
	maps.Copy(builtins, colorFuncs)
	for name, field := range dateFields {
		builtins[name] = dateField(field, false)
		builtins["utc"+name] = dateField(field, true)
	}
	return builtins
}

// mathFuncs are the functions of Math that take one number.
var mathFuncs = map[string]func(float64) float64{
	"abs": math.Abs, "acos": math.Acos, "asin": math.Asin, "atan": math.Atan,
	"ceil": math.Ceil, "cos": math.Cos, "exp": math.Exp, "floor": math.Floor,
	"log": math.Log, "sin": math.Sin, "sqrt": math.Sqrt, "tan": math.Tan,
	"round": func(x float64) float64 {
		// Math.round rounds a tie up, toward positive infinity, and keeps
		// the sign of a negative number that rounds to zero.
		r := math.Floor(x + 0.5)
		if r == 0 && math.Signbit(x) {
			return math.Copysign(0, -1)
		}
		return r
	},
}

// num1 makes a Func of a function of one number.
func num1(fn func(float64) float64) Func {
	return func(_ *Scope, args []any) (any, error) {
		return fn(numArg(args, 0)), nil
	}
}

// is makes a Func of a test of one value.
func is(fn func(any) bool) Func {
	return func(_ *Scope, args []any) (any, error) {
		return fn(arg(args, 0)), nil
	}
}

// arg returns an argument, or undefined if the call has too few.
func arg(args []any, i int) any {
	if i < len(args) {
		return args[i]
	}
	return Undefined
}

// numArg returns an argument as a number.
func numArg(args []any, i int) float64 {
	return Number(arg(args, i))
}

// isEmpty reports whether a value is null, undefined or the empty string,
// which the conversions map to null.
func isEmpty(v any) bool {
	return v == nil || IsUndefined(v) || v == ""
}

// isObject reports whether a value is an object, as v === Object(v) does in
// JavaScript.
func isObject(v any) bool {
	switch v.(type) {
	case []any, map[string]any, time.Time, *RegExp, *Color:
		return true
	}
	return false
}

// extreme returns the least of numbers when sign is -1, and the greatest
// when it is 1, as Math.min and Math.max do.
func extreme(args []any, sign float64) float64 {
	out := math.Inf(-int(sign))
	for _, a := range args {
		f := Number(a)
		switch {
		case math.IsNaN(f):
			return f
		case (f-out)*sign > 0:
			out = f
		case f == 0 && out == 0 && math.Signbit(f) == (sign < 0):
			// Math.min prefers -0, and Math.max prefers 0.
			out = f
		}
	}
	return out
}

// normalArgs returns the mean and the standard deviation of a normal
// distribution, from the arguments at i, with 0 and 1 by default.
func normalArgs(args []any, i int) (float64, float64) {
	mean, sd := 0.0, 1.0
	if v := arg(args, i); !IsUndefined(v) && v != nil {
		mean = Number(v)
	}
	if v := arg(args, i+1); !IsUndefined(v) && v != nil {
		sd = Number(v)
	}
	return mean, sd
}

// uniformArgs returns the bounds of a uniform distribution, from the
// arguments at i. One bound is the upper bound from 0, and none is 0 to 1.
func uniformArgs(args []any, i int) (float64, float64) {
	lo, hi := 0.0, 1.0
	a, b := arg(args, i), arg(args, i+1)
	switch {
	case !IsUndefined(b) && b != nil:
		lo, hi = Number(a), Number(b)
	case !IsUndefined(a) && a != nil:
		hi = Number(a)
	}
	return lo, hi
}

// boxMuller returns a sample of the standard normal distribution.
func boxMuller(s *Scope) float64 {
	u := 1 - s.random()
	v := s.random()
	return math.Sqrt(-2*math.Log(u)) * math.Cos(2*math.Pi*v)
}

// toBoolean converts a value to a boolean. Null, undefined and the empty
// string are null, and the strings "false" and "0" are false.
func toBoolean(_ *Scope, args []any) (any, error) {
	v := arg(args, 0)
	switch {
	case isEmpty(v):
		return nil, nil
	case v == "false" || v == "0":
		return false, nil
	}
	return Truthy(v), nil
}

// invalidDate is a date that is not a time, as an invalid Date of
// JavaScript is. Its number is NaN.
var invalidDate = time.Time{}

// toDate converts a value to a date, as milliseconds since 1970. A date
// stays a date, a number stays a number, and a string is parsed as
// Date.parse does. Null and the empty string are null.
func toDate(s *Scope, args []any) (any, error) {
	v := arg(args, 0)
	switch x := v.(type) {
	case time.Time, float64:
		return x, nil
	case string:
		if x == "" {
			return nil, nil
		}
		t, ok := format.ParseDate(x, s.location())
		if !ok {
			return nan(), nil
		}
		return float64(t.UnixMilli()), nil
	}
	if isEmpty(v) {
		return nil, nil
	}
	return nan(), nil
}

// timeArg returns an argument as a time in loc. A number is milliseconds
// since 1970. It reports false for a value that is not a time.
func timeArg(args []any, i int, loc *time.Location) (time.Time, bool) {
	v := arg(args, i)
	if t, ok := v.(time.Time); ok {
		if t.IsZero() {
			return t, false
		}
		return t.In(loc), true
	}
	if str, ok := v.(string); ok {
		// A string is a date in any form that Date reads.
		t, ok := format.ParseDate(str, loc)
		return t, ok
	}
	f := Number(v)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return time.Time{}, false
	}
	return time.UnixMilli(int64(math.Trunc(f))).In(loc), true
}

// dateOf builds a date from the fields that datetime and utc take: year,
// month from 0, day, hours, minutes, seconds and milliseconds. A year from
// 0 to 99 is 1900 to 1999, as in the Date of JavaScript.
func dateOf(args []any, loc *time.Location) (time.Time, bool) {
	f := [7]float64{0, 0, 1, 0, 0, 0, 0}
	for i := range f {
		if i < len(args) {
			f[i] = Number(args[i])
		}
	}
	for _, x := range f {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return time.Time{}, false
		}
	}
	y := int(f[0])
	if y >= 0 && y <= 99 {
		y += 1900
	}
	return format.WallTime(y, int(f[1])+1, int(f[2]), int(f[3]), int(f[4]), int(f[5]), int(f[6]), loc), true
}

// dateFields are the functions that read a field of a date. Each has a
// version in UTC, named with the prefix utc.
var dateFields = map[string]func(time.Time) int{
	"date":         time.Time.Day,
	"day":          func(t time.Time) int { return int(t.Weekday()) },
	"dayofyear":    time.Time.YearDay,
	"year":         time.Time.Year,
	"quarter":      func(t time.Time) int { return (int(t.Month())-1)/3 + 1 },
	"month":        func(t time.Time) int { return int(t.Month()) - 1 },
	"week":         func(t time.Time) int { return (t.YearDay() - 1 + 7 - int(t.Weekday())) / 7 },
	"hours":        time.Time.Hour,
	"minutes":      time.Time.Minute,
	"seconds":      time.Time.Second,
	"milliseconds": func(t time.Time) int { return t.Nanosecond() / int(time.Millisecond) },
}

// dateField makes a Func that reads a field of a date.
func dateField(field func(time.Time) int, utc bool) Func {
	return func(s *Scope, args []any) (any, error) {
		loc := s.location()
		if utc {
			loc = time.UTC
		}
		t, ok := timeArg(args, 0, loc)
		if !ok {
			return nan(), nil
		}
		return float64(field(t)), nil
	}
}

// offset moves a date by a number of time units.
func offset(args []any, loc *time.Location) (any, error) {
	unit := format.Interval(String(arg(args, 0)))
	if !unit.Valid() {
		return nil, fmt.Errorf("moving a date by the unit %q: %w", unit, ErrArgument)
	}
	t, ok := timeArg(args, 1, loc)
	if !ok {
		return invalidDate, nil
	}
	step := 1
	if v := arg(args, 2); !IsUndefined(v) {
		step = int(Number(v))
	}
	return unit.Offset(t, step), nil
}

// sequenceOf returns the dates of a time unit from a start to a stop.
func sequenceOf(args []any, loc *time.Location) (any, error) {
	unit := format.Interval(String(arg(args, 0)))
	if !unit.Valid() {
		return nil, fmt.Errorf("making a sequence of the unit %q: %w", unit, ErrArgument)
	}
	start, ok1 := timeArg(args, 1, loc)
	stop, ok2 := timeArg(args, 2, loc)
	if !ok1 || !ok2 {
		return []any{}, nil
	}
	step := 1
	if v := arg(args, 3); !IsUndefined(v) {
		step = int(Number(v))
	}
	out := []any{}
	for _, t := range unit.Range(start, stop, step) {
		out = append(out, t)
	}
	return out, nil
}

// timeFormat formats a date in loc.
func timeFormat(args []any, loc *time.Location, l *format.Locale) (any, error) {
	t, ok := timeArg(args, 0, loc)
	if !ok {
		return "Invalid Date", nil
	}
	return format.Time(String(arg(args, 1)), *l)(t), nil
}

// timeParse parses a date in loc. A text that does not match is null.
func timeParse(args []any, loc *time.Location, l *format.Locale) (any, error) {
	t, ok := format.Parse(String(arg(args, 1)), *l, loc)(String(arg(args, 0)))
	if !ok {
		return nil, nil
	}
	return t, nil
}

// name returns the name at an index. An index past the end wraps around,
// as the days and the months of a date do.
func name(names []string, args []any) any {
	f := math.Trunc(numArg(args, 0))
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return Undefined
	}
	n := len(names)
	return names[(int(f)%n+n)%n]
}

// units returns a string as the UTF-16 code units of JavaScript.
func units(s string) []uint16 {
	return utf16.Encode([]rune(s))
}

// fromUnits returns UTF-16 code units as a string.
func fromUnits(u []uint16) string {
	return string(utf16.Decode(u))
}

// indexOf is indexOf and lastIndexOf of a string or an array.
func indexOf(v, x any, last bool) float64 {
	switch a := v.(type) {
	case []any:
		idx := -1
		for i, e := range a {
			if strictEqual(e, x) {
				idx = i
				if !last {
					break
				}
			}
		}
		return float64(idx)
	case string:
		s, sub := units(a), units(String(x))
		idx := -1
		for i := 0; i+len(sub) <= len(s); i++ {
			if slices.Equal(s[i:i+len(sub)], sub) {
				idx = i
				if !last {
					break
				}
			}
		}
		return float64(idx)
	}
	return -1
}

// relIndex resolves an index of slice, which counts from the end when it
// is negative, within 0 and n.
func relIndex(v any, n int, def int) int {
	if IsUndefined(v) {
		return def
	}
	f := math.Trunc(Number(v))
	switch {
	case math.IsNaN(f):
		return 0
	case f < 0:
		return max(0, n+int(math.Max(f, -float64(n))))
	}
	return min(n, int(math.Min(f, float64(n))))
}

// sliceFunc is slice of a string or an array.
func sliceFunc(_ *Scope, args []any) (any, error) {
	switch a := arg(args, 0).(type) {
	case []any:
		i, j := relIndex(arg(args, 1), len(a), 0), relIndex(arg(args, 2), len(a), len(a))
		if j < i {
			return []any{}, nil
		}
		return slices.Clone(a[i:j]), nil
	case string:
		u := units(a)
		i, j := relIndex(arg(args, 1), len(u), 0), relIndex(arg(args, 2), len(u), len(u))
		if j < i {
			return "", nil
		}
		return fromUnits(u[i:j]), nil
	}
	return Undefined, nil
}

// substring is substring of a string, which swaps its indexes when the
// start is past the end, and takes a negative index as 0.
func substring(_ *Scope, args []any) (any, error) {
	u := units(String(arg(args, 0)))
	clampIdx := func(v any, def int) int {
		if IsUndefined(v) {
			return def
		}
		f := Number(v)
		if math.IsNaN(f) {
			return 0
		}
		return int(math.Max(0, math.Min(float64(len(u)), math.Trunc(f))))
	}
	i, j := clampIdx(arg(args, 1), 0), clampIdx(arg(args, 2), len(u))
	if i > j {
		i, j = j, i
	}
	return fromUnits(u[i:j]), nil
}

// pad pads a string to a length with a character. align says where the
// padding goes: right, the default, left or center.
func pad(_ *Scope, args []any) (any, error) {
	s := String(arg(args, 0))
	n := int(numArg(args, 1))
	c := " "
	if v := arg(args, 2); !IsUndefined(v) {
		c = String(v)
	}
	align := "right"
	if v := arg(args, 3); !IsUndefined(v) {
		align = String(v)
	}
	k := n - len(units(s))
	if k <= 0 || c == "" {
		return s, nil
	}
	switch align {
	case "left":
		return strings.Repeat(c, k) + s, nil
	case "center":
		return strings.Repeat(c, k/2) + s + strings.Repeat(c, k-k/2), nil
	}
	return s + strings.Repeat(c, k), nil
}

// truncate shortens a string to a length, with an ellipsis where the text
// is cut, on the right, the left or in the center.
func truncate(_ *Scope, args []any) (any, error) {
	u := units(String(arg(args, 0)))
	n := int(numArg(args, 1))
	align := "right"
	if v := arg(args, 2); !IsUndefined(v) {
		align = String(v)
	}
	ellipsis := units("…")
	if v := arg(args, 3); !IsUndefined(v) {
		ellipsis = units(String(v))
	}
	if len(u) <= n {
		return fromUnits(u), nil
	}
	keep := max(0, n-len(ellipsis))
	switch align {
	case "left":
		return fromUnits(ellipsis) + fromUnits(u[len(u)-keep:]), nil
	case "center":
		l := (keep + 1) / 2
		return fromUnits(u[:l]) + fromUnits(ellipsis) + fromUnits(u[len(u)-(keep-l):]), nil
	}
	return fromUnits(u[:keep]) + fromUnits(ellipsis), nil
}

// replace replaces the first match of a string or a regular expression, or
// every match of a regular expression with the flag g. $& and $1 to $9 in
// the replacement name the match and its groups.
func replace(_ *Scope, args []any) (any, error) {
	s, repl := String(arg(args, 0)), String(arg(args, 2))
	switch p := arg(args, 1).(type) {
	case *RegExp:
		expand := func(m []int) string {
			var b strings.Builder
			for i := 0; i < len(repl); i++ {
				c := repl[i]
				if c != '$' || i+1 >= len(repl) {
					b.WriteByte(c)
					continue
				}
				switch d := repl[i+1]; {
				case d == '$':
					b.WriteByte('$')
				case d == '&':
					b.WriteString(s[m[0]:m[1]])
				case d >= '1' && d <= '9' && int(d-'0')*2+1 < len(m):
					g := int(d-'0') * 2
					if m[g] >= 0 {
						b.WriteString(s[m[g]:m[g+1]])
					}
				default:
					b.WriteByte(c)
					continue
				}
				i++
			}
			return b.String()
		}
		var b strings.Builder
		last := 0
		for _, m := range p.FindAllStringSubmatchIndex(s, -1) {
			b.WriteString(s[last:m[0]])
			b.WriteString(expand(m))
			last = m[1]
			if !p.Global {
				break
			}
		}
		b.WriteString(s[last:])
		return b.String(), nil
	default:
		return strings.Replace(s, String(p), strings.ReplaceAll(repl, "$$", "$"), 1), nil
	}
}

// split splits a string by a separator or a regular expression, into at
// most limit parts.
func split(_ *Scope, args []any) (any, error) {
	s := String(arg(args, 0))
	limit := -1
	if v := arg(args, 2); !IsUndefined(v) {
		limit = int(Number(v))
	}
	var parts []string
	switch p := arg(args, 1).(type) {
	case *RegExp:
		parts = p.Split(s, -1)
	case undefined:
		parts = []string{s}
	default:
		sep := String(p)
		if sep == "" {
			for _, c := range units(s) {
				parts = append(parts, fromUnits([]uint16{c}))
			}
		} else {
			parts = strings.Split(s, sep)
		}
	}
	if limit >= 0 && limit < len(parts) {
		parts = parts[:limit]
	}
	out := make([]any, len(parts))
	for i, x := range parts {
		out[i] = x
	}
	return out, nil
}

// parseFloat reads the number at the start of a string, as parseFloat of
// JavaScript does.
func parseFloat(_ *Scope, args []any) (any, error) {
	s := strings.TrimLeft(String(arg(args, 0)), " \t\n\r\v\f")
	for _, p := range []string{jsInfinity, "+" + jsInfinity, "-" + jsInfinity} {
		if strings.HasPrefix(s, p) {
			if p[0] == '-' {
				return inf(-1), nil
			}
			return inf(1), nil
		}
	}
	end, seenDot, seenExp, digits := 0, false, false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			digits = true
			end = i + 1
		case (c == '+' || c == '-') && (i == 0 || s[i-1] == 'e' || s[i-1] == 'E'):
		case c == '.' && !seenDot && !seenExp:
			seenDot = true
		case (c == 'e' || c == 'E') && digits && !seenExp:
			seenExp = true
		default:
			i = len(s)
		}
	}
	if !digits {
		return nan(), nil
	}
	f, err := strconv.ParseFloat(s[:end], 64)
	if err != nil {
		// The digits can end in an exponent with no digits, such as 1e.
		f, _ = strconv.ParseFloat(strings.TrimRight(s[:end], "eE+-"), 64)
	}
	return f, nil
}

// parseInt reads the integer at the start of a string in a radix, as
// parseInt of JavaScript does.
func parseInt(_ *Scope, args []any) (any, error) {
	s := strings.TrimLeft(String(arg(args, 0)), " \t\n\r\v\f")
	sign := 1.0
	if s != "" && (s[0] == '-' || s[0] == '+') {
		if s[0] == '-' {
			sign = -1
		}
		s = s[1:]
	}
	radix := 10
	if v := arg(args, 1); !IsUndefined(v) && Number(v) != 0 {
		radix = int(Number(v))
	}
	if (radix == 16 || IsUndefined(arg(args, 1))) && (strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X")) {
		radix, s = 16, s[2:]
	}
	if radix < 2 || radix > 36 {
		return nan(), nil
	}
	n, out := 0, 0.0
	for ; n < len(s); n++ {
		d := digitOf(s[n])
		if d < 0 || d >= radix {
			break
		}
		out = out*float64(radix) + float64(d)
	}
	if n == 0 {
		return nan(), nil
	}
	return sign * out, nil
}

// digitOf returns the value of a digit or a letter in a radix up to 36.
func digitOf(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	}
	return -1
}

// joinWith joins the elements of an array, with null and undefined as
// empty strings.
func joinWith(a []any, sep string) string {
	parts := make([]string, len(a))
	for i, e := range a {
		if e != nil && !IsUndefined(e) {
			parts[i] = String(e)
		}
	}
	return strings.Join(parts, sep)
}

// flags reads the flags of a regular expression.
func flags(v any) string {
	if IsUndefined(v) || v == nil {
		return ""
	}
	return String(v)
}

// newRegExp compiles a regular expression with the flags of JavaScript:
// g, i, m and s. y and u change nothing that Go reads.
func newRegExp(pattern, flags string) (*RegExp, error) {
	var mods strings.Builder
	global := false
	for _, f := range flags {
		switch f {
		case 'g':
			global = true
		case 'i', 'm', 's':
			mods.WriteRune(f)
		case 'y', 'u':
		default:
			return nil, fmt.Errorf("compiling the flags %q: %w", flags, ErrArgument)
		}
	}
	src := pattern
	if mods.Len() > 0 {
		src = "(?" + mods.String() + ")" + pattern
	}
	re, err := regexp.Compile(src)
	if err != nil {
		return nil, fmt.Errorf("compiling the pattern %q: %w: %w", pattern, ErrArgument, err)
	}
	return &RegExp{Regexp: re, Source: pattern, Global: global}, nil
}

// Ascending compares two values in the natural order that the sorts of
// Vega use: null and undefined first, then numbers, dates and strings by
// their values. NaN is equal to everything, as it is not ordered.
func Ascending(a, b any) int {
	an, bn := a == nil || IsUndefined(a), b == nil || IsUndefined(b)
	switch {
	case an && bn:
		return 0
	case an:
		return -1
	case bn:
		return 1
	}
	c, ok := compare(a, b)
	if !ok {
		return 0
	}
	return c
}
