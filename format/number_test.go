package format

import (
	"encoding/json/v2"
	"math"
	"os"
	"testing"
)

// TestNumber checks Number against the output of d3-format, which
// testdata/number.json holds. test/oracle/formats.mjs writes it.
func TestNumber(t *testing.T) {
	t.Parallel()
	buf, err := os.ReadFile("testdata/number.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases [][3]any
	if err := json.Unmarshal(buf, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		spec, _ := c[0].(string)
		want, _ := c[2].(string)
		v, ok := c[1].(float64)
		if !ok {
			v = math.Copysign(0, -1)
		}
		f, err := Number(spec, EnUS)
		if err != nil {
			t.Errorf("Number(%q): %v", spec, err)
			continue
		}
		if got := f(v); got != want {
			t.Errorf("format(%q)(%v) = %q, want %q", spec, v, got, want)
		}
	}
}

func TestNumberSpecial(t *testing.T) {
	t.Parallel()
	tests := []struct {
		spec string
		v    float64
		want string
	}{
		{",.0f", math.NaN(), "NaN"},
		{".1f", math.Inf(1), "Infinity"},
		{".1f", math.Inf(-1), "−Infinity"},
		{",d", math.Copysign(0, -1), "0"},
		{"+,d", 0, "+0"},
	}
	for _, test := range tests {
		f, err := Number(test.spec, EnUS)
		if err != nil {
			t.Fatal(err)
		}
		if got := f(test.v); got != test.want {
			t.Errorf("format(%q)(%v) = %q, want %q", test.spec, test.v, got, test.want)
		}
	}
	if _, err := Number("zz", EnUS); err == nil {
		t.Errorf("Number(%q) = nil error", "zz")
	}
}
