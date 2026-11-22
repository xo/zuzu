// Copied from github.com/d2lang/rough-go, file number_test.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.0.4, Copyright (c) 2019 Preet Shihn.

package rough

import (
	"math"
	"testing"
)

func TestJSNumberString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   float64
		want string
	}{
		{name: "zero", in: 0, want: "0"},
		{name: "negative zero", in: math.Copysign(0, -1), want: "0"},
		{name: "integer", in: -42, want: "-42"},
		{name: "fraction", in: 1.2345678901234567, want: "1.2345678901234567"},
		{name: "fixed lower boundary", in: 1e-6, want: "0.000001"},
		{name: "exponential below boundary", in: 1e-7, want: "1e-7"},
		{name: "negative exponential", in: -1e-7, want: "-1e-7"},
		{name: "fixed upper range", in: 1e20, want: "100000000000000000000"},
		{name: "shortest large integer", in: 1000000000000000128, want: "1000000000000000100"},
		{name: "exponential upper boundary", in: 1e21, want: "1e+21"},
		{name: "minimum subnormal", in: math.SmallestNonzeroFloat64, want: "5e-324"},
		{name: "maximum", in: math.MaxFloat64, want: "1.7976931348623157e+308"},
		{name: "nan", in: math.NaN(), want: "NaN"},
		{name: "positive infinity", in: math.Inf(1), want: "Infinity"},
		{name: "negative infinity", in: math.Inf(-1), want: "-Infinity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := jsNumberString(tt.in); got != tt.want {
				t.Fatalf("jsNumberString(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
	if got := NumberString(1e-7); got != "1e-7" {
		t.Fatalf("NumberString wrapper = %q, want %q", got, "1e-7")
	}
}
