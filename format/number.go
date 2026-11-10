// Package format formats numbers and times with the syntax of d3-format and
// d3-time-format, which Vega uses for its axes, its legends and the functions
// format and timeFormat of its expressions. zuzu follows the documented
// behavior and is not a port (D6). The locale is US English by default
// (D27), and a Locale holds another.
package format

import (
	"fmt"
	"math"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Error is an error of the package format.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// ErrInvalidSpec is the error of a format spec that does not parse.
const ErrInvalidSpec Error = "invalid format spec"

// Locale holds the symbols of numbers and the names of times of a language.
type Locale struct {
	Decimal     string
	Thousands   string
	Grouping    []int
	Currency    [2]string
	Minus       string
	Percent     string
	NaN         string
	DateTime    string
	Date        string
	Time        string
	Periods     [2]string
	Days        [7]string
	ShortDays   [7]string
	Months      [12]string
	ShortMonths [12]string
}

// EnUS is the locale of US English, the default of d3 and of zuzu (D27).
var EnUS = Locale{
	Decimal:     ".",
	Thousands:   ",",
	Grouping:    []int{3},
	Currency:    [2]string{"$", ""},
	Minus:       "−",
	Percent:     "%",
	NaN:         "NaN",
	DateTime:    "%x, %X",
	Date:        "%-m/%-d/%Y",
	Time:        "%-I:%M:%S %p",
	Periods:     [2]string{"AM", "PM"},
	Days:        [7]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
	ShortDays:   [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
	Months:      [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
	ShortMonths: [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
}

// specRE is the syntax of a format spec of d3-format:
// [[fill]align][sign][symbol][0][width][,][.precision][~][type].
var specRE = regexp.MustCompile(`^(?:(.)?([<>=^]))?([+\-( ])?([$#])?(0)?(\d+)?(,)?(\.\d+)?(~)?([a-zA-Z%])?$`)

// Spec is a parsed format spec of d3-format.
type Spec struct {
	Fill      string
	Align     string
	Sign      string
	Symbol    string
	Zero      bool
	Width     int
	Comma     bool
	Precision int // -1 when the spec gives none
	Trim      bool
	Type      string
}

// ParseSpec parses a format spec of d3-format.
func ParseSpec(s string) (Spec, error) {
	m := specRE.FindStringSubmatch(s)
	if m == nil {
		return Spec{}, fmt.Errorf("parsing the number format %q: %w", s, ErrInvalidSpec)
	}
	sp := Spec{Fill: m[1], Align: m[2], Sign: m[3], Symbol: m[4], Zero: m[5] != "", Comma: m[7] != "", Trim: m[9] != "", Type: m[10], Precision: -1}
	if sp.Fill == "" {
		sp.Fill = " "
	}
	if sp.Align == "" {
		sp.Align = ">"
	}
	if sp.Sign == "" {
		sp.Sign = "-"
	}
	if m[6] != "" {
		sp.Width, _ = strconv.Atoi(m[6])
	}
	if m[8] != "" {
		sp.Precision, _ = strconv.Atoi(m[8][1:])
	}
	return sp, nil
}

// prefixes are the SI prefixes from 1e-24 to 1e24.
var prefixes = []string{"y", "z", "a", "f", "p", "n", "µ", "m", "", "k", "M", "G", "T", "P", "E", "Z", "Y"}

// Number returns a function that formats a number with a format spec of
// d3-format, such as ",.2f" or "~s", in a locale.
func Number(spec string, loc Locale) (func(float64) string, error) {
	sp, err := ParseSpec(spec)
	if err != nil {
		return nil, err
	}
	return number(sp, loc), nil
}

// number returns the function that formats with a parsed spec.
func number(sp Spec, loc Locale) func(float64) string {
	typ, precision, trim, comma := sp.Type, sp.Precision, sp.Trim, sp.Comma
	switch {
	case typ == "n":
		comma, typ = true, "g"
	case !isType(typ, "efgprs%bodxXc"):
		if precision < 0 {
			precision = 12
		}
		trim, typ = true, "g"
	}
	fill, align, zero := sp.Fill, sp.Align, sp.Zero
	if zero || (fill == "0" && align == "=") {
		zero, fill, align = true, "0", "="
	}
	prefix, suffix := "", ""
	switch {
	case sp.Symbol == "$":
		prefix, suffix = loc.Currency[0], loc.Currency[1]
	case sp.Symbol == "#" && isType(typ, "boxX"):
		prefix = "0" + strings.ToLower(typ)
	}
	if typ == "%" || typ == "p" {
		suffix = loc.Percent + suffix
	}
	maybeSuffix := isType(typ, "defgprs%")
	switch {
	case precision < 0:
		precision = 6
	case isType(typ, "gprs"):
		precision = max(1, min(21, precision))
	default:
		precision = max(0, min(20, precision))
	}
	return func(x float64) string {
		valuePrefix, valueSuffix, value := prefix, suffix, ""
		if typ == "c" {
			valueSuffix = numberJS(x) + valueSuffix
		} else {
			negative := x < 0 || 1/x < 0
			exp := 0
			if math.IsNaN(x) {
				value = loc.NaN
			} else {
				value, exp = formatType(typ, math.Abs(x), precision)
			}
			if trim {
				value = trimZeros(value)
			}
			if negative && isZero(value) && sp.Sign != "+" {
				negative = false
			}
			sign := ""
			switch {
			case negative && sp.Sign == "(":
				sign = "("
			case negative:
				sign = loc.Minus
			case sp.Sign != "-" && sp.Sign != "(":
				sign = sp.Sign
			}
			valuePrefix = sign + valuePrefix
			if typ == "s" {
				valueSuffix = prefixes[8+exp/3] + valueSuffix
			}
			if negative && sp.Sign == "(" {
				valueSuffix += ")"
			}
			if maybeSuffix {
				for i := 0; i < len(value); i++ {
					if c := value[i]; c < '0' || c > '9' {
						if c == '.' {
							valueSuffix = loc.Decimal + value[i+1:] + valueSuffix
						} else {
							valueSuffix = value[i:] + valueSuffix
						}
						value = value[:i]
						break
					}
				}
			}
		}
		if comma && !zero {
			value = group(value, loc, -1)
		}
		length := utf8.RuneCountInString(valuePrefix) + utf8.RuneCountInString(value) + utf8.RuneCountInString(valueSuffix)
		padding := ""
		if length < sp.Width {
			padding = strings.Repeat(fill, sp.Width-length)
		}
		if comma && zero {
			w := -1
			if padding != "" {
				w = sp.Width - utf8.RuneCountInString(valueSuffix)
			}
			value = group(padding+value, loc, w)
			padding = ""
		}
		switch align {
		case "<":
			return valuePrefix + value + valueSuffix + padding
		case "=":
			return valuePrefix + padding + value + valueSuffix
		case "^":
			r := []rune(padding)
			i := len(r) >> 1
			return string(r[:i]) + valuePrefix + value + valueSuffix + string(r[i:])
		}
		return padding + valuePrefix + value + valueSuffix
	}
}

// isType reports whether typ is one of the types of a set, such as "boxX".
func isType(typ, set string) bool {
	return typ != "" && strings.Contains(set, typ)
}

// isZero reports whether a formatted value reads as zero.
func isZero(s string) bool {
	f, err := strconv.ParseFloat(s, 64)
	return err == nil && f == 0
}

// formatType formats a non-negative number for one type of d3-format. It
// returns the exponent of the SI prefix for the type s.
func formatType(typ string, x float64, p int) (string, int) {
	switch typ {
	case "%":
		return toFixed(x*100, p), 0
	case "b":
		return radix(x, 2), 0
	case "o":
		return radix(x, 8), 0
	case "x":
		return radix(x, 16), 0
	case "X":
		return strings.ToUpper(radix(x, 16)), 0
	case "d":
		r := math.Round(x)
		if r >= 1e21 {
			return strconv.FormatFloat(r, 'f', 0, 64), 0
		}
		return numberJS(r), 0
	case "e":
		return toExponential(x, p), 0
	case "f":
		return toFixed(x, p), 0
	case "g":
		return toPrecision(x, p), 0
	case "p":
		if x == 0 {
			return "0", 0
		}
		return rounded(x*100, p), 0
	case "r":
		if x == 0 {
			return "0", 0
		}
		return rounded(x, p), 0
	case "s":
		return prefixAuto(x, p)
	}
	return numberJS(x), 0
}

// radix writes a non-negative number, rounded to an integer, in a base.
func radix(x float64, base int) string {
	if math.IsInf(x, 0) {
		return numberJS(x)
	}
	i, _ := new(big.Float).SetFloat64(math.Round(x)).Int(nil)
	return i.Text(base)
}

// decimal is the exact decimal expansion of a finite, non-negative float64:
// its digits with no leading zeros and the power of ten of the first digit.
type decimal struct {
	digits string
	exp    int
}

// exact returns the exact decimal expansion of x.
func exact(x float64) decimal {
	if x == 0 {
		return decimal{"0", 0}
	}
	s := new(big.Float).SetFloat64(x).Text('e', 1100)
	mant, e, _ := strings.Cut(s, "e")
	exp, _ := strconv.Atoi(e)
	digits := strings.TrimRight(strings.Replace(mant, ".", "", 1), "0")
	if digits == "" {
		digits = "0"
	}
	return decimal{digits, exp}
}

// round rounds the decimal to n significant digits, with a tie rounded away
// from zero, as JavaScript rounds in toFixed, toExponential and toPrecision.
func (d decimal) round(n int) decimal {
	if n <= 0 {
		// Every digit is rounded away. The result is 1 at the next place, or 0.
		if n == 0 && d.digits[0] >= '5' {
			return decimal{"1", d.exp + 1}
		}
		return decimal{"0", d.exp}
	}
	if len(d.digits) <= n {
		return decimal{d.digits + strings.Repeat("0", n-len(d.digits)), d.exp}
	}
	keep := []byte(d.digits[:n])
	if d.digits[n] >= '5' {
		i := n - 1
		for i >= 0 {
			if keep[i] == '9' {
				keep[i] = '0'
				i--
				continue
			}
			keep[i]++
			break
		}
		if i < 0 {
			return decimal{"1" + string(keep[:n-1]), d.exp + 1}
		}
	}
	return decimal{string(keep), d.exp}
}

// toFixed is Number.prototype.toFixed of JavaScript.
func toFixed(x float64, p int) string {
	if math.IsInf(x, 0) {
		return numberJS(x)
	}
	if x >= 1e21 {
		return numberJS(x)
	}
	d := exact(x)
	r := d.round(d.exp + 1 + p)
	if r.digits == "0" {
		if p == 0 {
			return "0"
		}
		return "0." + strings.Repeat("0", p)
	}
	// The digits of r start at 10^r.exp. Write them at the places from
	// 10^max(r.exp, 0) down to 10^-p.
	var b strings.Builder
	digit := func(place int) byte {
		i := r.exp - place
		if i < 0 || i >= len(r.digits) {
			return '0'
		}
		return r.digits[i]
	}
	for place := max(r.exp, 0); place >= 0; place-- {
		b.WriteByte(digit(place))
	}
	if p > 0 {
		b.WriteByte('.')
		for place := -1; place >= -p; place-- {
			b.WriteByte(digit(place))
		}
	}
	return b.String()
}

// toExponential is Number.prototype.toExponential of JavaScript, with p
// digits after the point. A negative p gives as many digits as the number
// needs.
func toExponential(x float64, p int) string {
	if math.IsInf(x, 0) {
		return numberJS(x)
	}
	var r decimal
	if p < 0 {
		s := strconv.FormatFloat(x, 'e', -1, 64)
		mant, e, _ := strings.Cut(s, "e")
		exp, _ := strconv.Atoi(e)
		r = decimal{strings.Replace(mant, ".", "", 1), exp}
	} else {
		r = exact(x).round(p + 1)
	}
	mant := r.digits[:1]
	if len(r.digits) > 1 {
		mant += "." + r.digits[1:]
	}
	sign := "+"
	if r.exp < 0 {
		sign = "-"
	}
	return mant + "e" + sign + strconv.Itoa(abs(r.exp))
}

// toPrecision is Number.prototype.toPrecision of JavaScript.
func toPrecision(x float64, p int) string {
	if math.IsInf(x, 0) {
		return numberJS(x)
	}
	if x == 0 {
		if p == 1 {
			return "0"
		}
		return "0." + strings.Repeat("0", p-1)
	}
	r := exact(x).round(p)
	if r.exp < -6 || r.exp >= p {
		return toExponential(x, p-1)
	}
	return toFixed(x, p-1-r.exp)
}

// parts returns the digits and the exponent of x with p significant digits,
// or with as many as the number needs when p is zero, as formatDecimalParts
// of d3-format does.
func parts(x float64, p int) (string, int) {
	var s string
	if p > 0 {
		s = toExponential(x, p-1)
	} else {
		s = toExponential(x, -1)
	}
	mant, e, _ := strings.Cut(s, "e")
	exp, _ := strconv.Atoi(e)
	return strings.Replace(mant, ".", "", 1), exp
}

// rounded formats x to p significant digits with no exponent.
func rounded(x float64, p int) string {
	c, e := parts(x, p)
	switch {
	case e < 0:
		return "0." + strings.Repeat("0", -e-1) + c
	case len(c) > e+1:
		return c[:e+1] + "." + c[e+1:]
	}
	return c + strings.Repeat("0", e-len(c)+1)
}

// prefixAuto formats x to p significant digits for an SI prefix, and returns
// the exponent of the prefix.
func prefixAuto(x float64, p int) (string, int) {
	c, e := parts(x, p)
	pe := max(-8, min(8, int(math.Floor(float64(e)/3)))) * 3
	i := e - pe + 1
	n := len(c)
	switch {
	case i == n:
		return c, pe
	case i > n:
		return c + strings.Repeat("0", i-n), pe
	case i > 0:
		return c[:i] + "." + c[i:], pe
	}
	d, _ := parts(x, max(0, p+i-1))
	return "0." + strings.Repeat("0", -i) + d, pe
}

// trimZeros removes the zeros at the end of the fraction of a formatted
// number, and the point if no digit is left after it, as the ~ of
// d3-format does. Text after the digits, such as an exponent, stays.
func trimZeros(s string) string {
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		return s
	}
	end := dot + 1
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	frac := strings.TrimRight(s[dot+1:end], "0")
	if frac == "" {
		return s[:dot] + s[end:]
	}
	return s[:dot+1] + frac + s[end:]
}

// group puts the thousands separator of a locale between the groups of
// digits of value. If width is not negative, value holds padding zeros at
// its start, and group drops whole groups of them until the result fits in
// width.
func group(value string, loc Locale, width int) string {
	if len(loc.Grouping) == 0 || loc.Thousands == "" {
		return value
	}
	// Cut the digits into groups from the right.
	var groups []string
	for i, j := len(value), 0; i > 0; j++ {
		g := loc.Grouping[min(j, len(loc.Grouping)-1)]
		if g <= 0 {
			groups = append(groups, value[:i])
			break
		}
		start := max(0, i-g)
		groups = append(groups, value[start:i])
		i = start
	}
	slices.Reverse(groups)
	out := strings.Join(groups, loc.Thousands)
	if width < 0 {
		return out
	}
	// Drop the padding from the left until the result fits.
	for utf8.RuneCountInString(out) > width && len(out) > 0 {
		r, n := utf8.DecodeRuneInString(out)
		if r != '0' && string(r) != loc.Thousands {
			break
		}
		out = out[n:]
	}
	// A group never starts with a separator. Keep one zero before it.
	if strings.HasPrefix(out, loc.Thousands) {
		out = "0" + out
	}
	return out
}

// numberJS writes a number as JavaScript writes it.
func numberJS(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0"
	}
	a := math.Abs(f)
	if a >= 1e21 || a < 1e-6 {
		s := strconv.FormatFloat(f, 'e', -1, 64)
		mant, e, _ := strings.Cut(s, "e")
		exp, _ := strconv.Atoi(e)
		sign := "+"
		if exp < 0 {
			sign = "-"
		}
		return mant + "e" + sign + strconv.Itoa(abs(exp))
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// abs returns the absolute value of n.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
