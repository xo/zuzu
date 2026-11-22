// Copied from github.com/d2lang/rough-go, file number.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.6.6, Copyright (c) 2019 Preet Shihn.

package rough

import (
	"math"
	"strconv"
	"strings"
)

func jsToFixedNumber(v float64, digits int) float64 {
	if digits < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return v
	}
	formatted := strconv.FormatFloat(v, 'f', digits, 64)
	result, err := strconv.ParseFloat(formatted, 64)
	if err != nil {
		return v
	}
	return result
}

// jsNumberString implements the formatting used when JavaScript coerces a
// Number to a string. Rough.js builds SVG path data with that coercion, so the
// fixed/exponential thresholds and negative-zero handling are observable.
func jsNumberString(v float64) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "Infinity"
	case math.IsInf(v, -1):
		return "-Infinity"
	case v == 0:
		// String(-0) is "0" in JavaScript.
		return "0"
	}

	abs := math.Abs(v)
	format := byte('f')
	if abs < 1e-6 || abs >= 1e21 {
		format = 'e'
	}
	s := strconv.FormatFloat(v, format, -1, 64)
	if format != 'e' {
		return s
	}

	// strconv pads single-digit negative exponents (e-07), while JavaScript
	// emits e-7. Positive exponents already include the required plus sign.
	if i := strings.LastIndexByte(s, 'e'); i >= 0 && i+2 < len(s) {
		sign := s[i+1]
		if sign == '+' || sign == '-' {
			j := i + 2
			for j+1 < len(s) && s[j] == '0' {
				j++
			}
			if j != i+2 {
				s = s[:i+2] + s[j:]
			}
		}
	}
	return s
}

// NumberString formats v the way JavaScript coerces a Number to a string.
// It is useful when surrounding SVG serialization must remain byte-compatible
// with Rough.js output.
func NumberString(v float64) string {
	return jsNumberString(v)
}
