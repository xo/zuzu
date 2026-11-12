package format

import (
	"math"
)

// exponent returns the exponent of x in exponential notation, such as 2
// for 123 and -3 for 0.0042, and false for a number that has none, such as
// NaN.
func exponent(x float64) (int, bool) {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0, false
	}
	_, e := parts(math.Abs(x), 0)
	return e, true
}

// PrecisionFixed returns the number of decimals that fixed notation needs
// to tell apart two values that are step apart, such as 1 for 0.5 and 2 for
// 0.01, as precisionFixed of d3-format. It returns -1 for a step that is not
// a number.
func PrecisionFixed(step float64) int {
	e, ok := exponent(step)
	if !ok {
		return -1
	}
	return max(0, -e)
}

// PrecisionPrefix returns the number of decimals that the notation of an
// SI prefix needs to tell apart two values that are step apart, where the
// prefix is the one of value, such as 1 for a step of 100000 and a value
// of 1300000 (1.3M), as precisionPrefix of d3-format. It returns -1 for a
// step or a value that is not a number.
func PrecisionPrefix(step, value float64) int {
	es, ok := exponent(step)
	ev, ok2 := exponent(value)
	if !ok || !ok2 {
		return -1
	}
	return max(0, max(-8, min(8, int(math.Floor(float64(ev)/3))))*3-es)
}

// PrecisionRound returns the number of significant digits that tell apart
// two values that are step apart, up to the largest value, such as 3 for a
// step of 0.01 and a largest value of 1.01, as precisionRound of
// d3-format. It returns -1 for a step or a value that is not a number.
func PrecisionRound(step, largest float64) int {
	step = math.Abs(step)
	es, ok := exponent(step)
	em, ok2 := exponent(math.Abs(largest) - step)
	if !ok || !ok2 {
		return -1
	}
	return max(0, em-es) + 1
}

// Prefix returns a function that formats a number in fixed notation, in the
// unit of the SI prefix of value, with that prefix after it, such as 1.5M
// for 1500000 when value is 4000000. Every number takes the same prefix, as
// formatPrefix of d3-format does. The type of the spec is ignored.
func Prefix(spec string, value float64, loc Locale) (func(float64) string, error) {
	sp, err := ParseSpec(spec)
	if err != nil {
		return nil, err
	}
	return prefixed(sp, value, loc), nil
}

// prefixed returns the function of Prefix for a parsed spec.
func prefixed(sp Spec, value float64, loc Locale) func(float64) string {
	sp.Type = "f"
	f := number(sp, loc)
	ev, ok := exponent(value)
	if !ok {
		ev = 0
	}
	e := max(-8, min(8, int(math.Floor(float64(ev)/3)))) * 3
	k := math.Pow(10, float64(-e))
	prefix := prefixes[8+e/3]
	return func(x float64) string {
		return f(k*x) + prefix
	}
}

// Ticks returns the function that writes the labels of the ticks of a
// scale of numbers from lo to hi, step apart, with a spec of d3-format, as
// tickFormat of d3-scale does. The spec ",f" is the default. If the spec
// gives no precision, the precision is the one that tells two ticks apart.
// The type s writes each label with the SI prefix of the largest end, such
// as 0M, 1M and 2M.
func Ticks(spec string, lo, hi, step float64, loc Locale) (func(float64) string, error) {
	if spec == "" {
		spec = ",f"
	}
	sp, err := ParseSpec(spec)
	if err != nil {
		return nil, err
	}
	largest := math.Max(math.Abs(lo), math.Abs(hi))
	switch sp.Type {
	case "s":
		if sp.Precision < 0 {
			sp.Precision = PrecisionPrefix(step, largest)
		}
		return prefixed(sp, largest, loc), nil
	case "", "e", "g", "p", "r":
		if sp.Precision < 0 {
			if p := PrecisionRound(step, largest); p >= 0 {
				sp.Precision = p
				if sp.Type == "e" {
					sp.Precision = max(0, p-1)
				}
			}
		}
	case "f", "%":
		if sp.Precision < 0 {
			if p := PrecisionFixed(step); p >= 0 {
				sp.Precision = p
				if sp.Type == "%" {
					sp.Precision = max(0, p-2)
				}
			}
		}
	}
	return number(sp, loc), nil
}
