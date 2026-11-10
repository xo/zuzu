package expr

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// A value of an expression is one of these Go values, which hold the values
// of JavaScript:
//
//	float64          a number
//	string           a string
//	bool             a boolean
//	nil              null
//	Undefined        undefined
//	time.Time        a date
//	[]any            an array
//	map[string]any   an object
//	*RegExp          a regular expression, which the function regexp makes
//	*Color           a color, which the functions rgb, hsl, lab and hcl make
//
// The zero time.Time is an invalid date, as new Date(NaN) is.

// undefined is the type of Undefined.
type undefined struct{}

// Undefined is the value undefined of JavaScript.
var Undefined any = undefined{}

// IsUndefined reports whether v is undefined.
func IsUndefined(v any) bool {
	_, ok := v.(undefined)
	return ok
}

// nan returns NaN.
func nan() float64 { return math.NaN() }

// inf returns an infinity of a sign.
func inf(sign int) float64 { return math.Inf(sign) }

// jsInfinity is how JavaScript writes infinity.
const jsInfinity = "Infinity"

// Number converts a value to a number, as Number() of JavaScript does.
func Number(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case bool:
		if x {
			return 1
		}
		return 0
	case nil:
		return 0
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0
		}
		if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
			n, err := strconv.ParseInt(s[2:], 16, 64)
			if err != nil {
				return nan()
			}
			return float64(n)
		}
		switch s {
		case jsInfinity, "+" + jsInfinity:
			return inf(1)
		case "-" + jsInfinity:
			return inf(-1)
		}
		n, err := strconv.ParseFloat(s, 64)
		if err != nil || strings.ContainsAny(s, "_") || strings.EqualFold(s, "inf") || strings.EqualFold(s, "nan") {
			return nan()
		}
		return n
	case time.Time:
		if x.IsZero() {
			return nan()
		}
		return float64(x.UnixMilli())
	case []any:
		switch len(x) {
		case 0:
			return 0
		case 1:
			return Number(x[0])
		}
	}
	return nan()
}

// Truthy reports whether a value is true in a condition, as JavaScript
// decides: false, 0, NaN, the empty string, null and undefined are false.
func Truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0 && !math.IsNaN(x)
	case int:
		return x != 0
	case string:
		return x != ""
	case nil, undefined:
		return false
	}
	return true
}

// String converts a value to a string, as String() of JavaScript does.
func String(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return numberString(x)
	case int:
		return strconv.Itoa(x)
	case bool:
		return strconv.FormatBool(x)
	case nil:
		return "null"
	case undefined:
		return "undefined"
	case time.Time:
		if x.IsZero() {
			return "Invalid Date"
		}
		return x.Format("Mon Jan 02 2006 15:04:05 GMT-0700 (MST)")
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			if e != nil && !IsUndefined(e) {
				parts[i] = String(e)
			}
		}
		return strings.Join(parts, ",")
	case map[string]any:
		return "[object Object]"
	case *Color:
		return x.String()
	}
	return ""
}

// numberString writes a number as JavaScript writes it: an integer with no
// decimals, and an exponent beyond 1e21 or below 1e-6.
func numberString(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return jsInfinity
	case math.IsInf(f, -1):
		return "-" + jsInfinity
	case f == 0:
		return "0"
	}
	a := math.Abs(f)
	if a >= 1e21 || a < 1e-6 {
		s := strconv.FormatFloat(f, 'e', -1, 64)
		mant, exp, _ := strings.Cut(s, "e")
		if exp[0] == '+' {
			exp = exp[1:]
			exp = strings.TrimLeft(exp, "0")
			return mant + "e+" + exp
		}
		return mant + "e-" + strings.TrimLeft(exp[1:], "0")
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// primitive converts an object to a primitive, as the ToPrimitive of
// JavaScript does for + and the comparisons: a date to its number, and an
// array or an object to its string.
func primitive(v any, hint string) any {
	switch x := v.(type) {
	case time.Time:
		if hint == "string" {
			return String(x)
		}
		return Number(x)
	case []any, map[string]any, *Color:
		return String(x)
	}
	return v
}

// add is + of JavaScript.
func add(x, y any) any {
	// A date added to a value becomes its string, as in JavaScript.
	px, py := primitive(x, "string"), primitive(y, "string")
	if _, ok := x.(time.Time); !ok {
		px = primitive(x, "number")
	}
	if _, ok := y.(time.Time); !ok {
		py = primitive(y, "number")
	}
	sx, okx := px.(string)
	sy, oky := py.(string)
	if okx || oky {
		if !okx {
			sx = String(px)
		}
		if !oky {
			sy = String(py)
		}
		return sx + sy
	}
	return Number(px) + Number(py)
}

// equal is == of JavaScript.
func equal(x, y any) bool {
	switch {
	case (x == nil || IsUndefined(x)) && (y == nil || IsUndefined(y)):
		return true
	case x == nil || IsUndefined(x) || y == nil || IsUndefined(y):
		return false
	}
	switch a := x.(type) {
	case string:
		if b, ok := y.(string); ok {
			return a == b
		}
	case bool:
		return equal(Number(a), y)
	case time.Time:
		if b, ok := y.(time.Time); ok {
			return a.Equal(b)
		}
	}
	if b, ok := y.(bool); ok {
		return equal(x, Number(b))
	}
	return Number(primitive(x, "number")) == Number(primitive(y, "number"))
}

// strictEqual is === of JavaScript.
func strictEqual(x, y any) bool {
	switch a := x.(type) {
	case float64:
		b, ok := y.(float64)
		return ok && a == b
	case string:
		b, ok := y.(string)
		return ok && a == b
	case bool:
		b, ok := y.(bool)
		return ok && a == b
	case nil:
		return y == nil
	case undefined:
		return IsUndefined(y)
	case time.Time:
		b, ok := y.(time.Time)
		return ok && a.Equal(b)
	}
	return false
}

// compare is < and > of JavaScript. It returns -1, 0 or 1, and false when
// the values do not compare, such as with NaN.
func compare(x, y any) (int, bool) {
	px, py := primitive(x, "number"), primitive(y, "number")
	sx, okx := px.(string)
	sy, oky := py.(string)
	if okx && oky {
		return strings.Compare(sx, sy), true
	}
	a, b := Number(px), Number(py)
	switch {
	case math.IsNaN(a) || math.IsNaN(b):
		return 0, false
	case a < b:
		return -1, true
	case a > b:
		return 1, true
	}
	return 0, true
}

// uint32Of converts a value to an unsigned 32-bit integer, as the bitwise
// operators of JavaScript do: the integer part, modulo 2^32.
func uint32Of(v any) uint32 {
	f := Number(v)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	f = math.Mod(math.Trunc(f), 1<<32)
	if f < 0 {
		f += 1 << 32
	}
	return uint32(f)
}

// int32Of converts a value to a signed 32-bit integer, as the bitwise
// operators of JavaScript do.
func int32Of(v any) int32 {
	return int32(uint32Of(v)) //nolint:gosec // The bits of the two's complement are the value.
}

// typeOf is typeof of JavaScript.
func typeOf(v any) string {
	switch v.(type) {
	case float64, int:
		return "number"
	case string:
		return "string"
	case bool:
		return "boolean"
	case undefined:
		return "undefined"
	}
	return "object"
}
