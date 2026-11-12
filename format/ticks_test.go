package format

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"
	"testing"
)

// TestTicks checks Ticks against the labels that tickFormat of d3-scale
// writes for the ticks of a linear scale, which testdata/ticks.json holds.
// test/oracle/formats.mjs writes it. A spec without a precision, such as s,
// takes the precision that tells two ticks apart.
func TestTicks(t *testing.T) {
	t.Parallel()
	buf, err := os.ReadFile("testdata/ticks.json")
	if err != nil {
		t.Fatal(err)
	}
	type tickCase struct {
		spec         string
		lo, hi, step float64
		ticks        []float64
		labels       []string
	}
	var cases []tickCase
	var raw [][]jsontext.Value
	if err := json.Unmarshal(buf, &raw); err != nil {
		t.Fatal(err)
	}
	for _, r := range raw {
		var c tickCase
		for i, dst := range []any{&c.spec, &c.lo, &c.hi, &c.step, &c.ticks, &c.labels} {
			if err := json.Unmarshal(r[i], dst); err != nil {
				t.Fatal(err)
			}
		}
		cases = append(cases, c)
	}
	if len(cases) == 0 {
		t.Fatal("no cases")
	}
	for _, c := range cases {
		f, err := Ticks(c.spec, c.lo, c.hi, c.step, EnUS)
		if err != nil {
			t.Errorf("Ticks(%q): %v", c.spec, err)
			continue
		}
		for i, v := range c.ticks {
			if got := f(v); got != c.labels[i] {
				t.Errorf("Ticks(%q, %v, %v)(%v) = %q, want %q", c.spec, c.lo, c.hi, v, got, c.labels[i])
			}
		}
	}
}

func TestPrecision(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"fixed 0.5", PrecisionFixed(0.5), 1},
		{"fixed 0.01", PrecisionFixed(0.01), 2},
		{"fixed 10", PrecisionFixed(10), 0},
		{"prefix 1e5 1.3e6", PrecisionPrefix(1e5, 1.3e6), 1},
		{"prefix 1e6 1.2e7", PrecisionPrefix(1e6, 1.2e7), 0},
		{"round 0.01 1.01", PrecisionRound(0.01, 1.01), 3},
		{"round 0.1 1.1", PrecisionRound(0.1, 1.1), 2},
	}
	for _, test := range tests {
		if test.got != test.want {
			t.Errorf("%s = %d, want %d", test.name, test.got, test.want)
		}
	}
}
