package format

import (
	"encoding/json/v2"
	"os"
	"testing"
	"time"
	_ "time/tzdata" // The tests read a named zone, not the zone of the machine.
)

// zone is the time zone that test/oracle/formats.mjs runs in.
func zone(t *testing.T) *time.Location {
	t.Helper()
	z, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	return z
}

// TestTime checks Time against the output of d3-time-format, which
// testdata/time.json holds.
func TestTime(t *testing.T) {
	t.Parallel()
	z := zone(t)
	buf, err := os.ReadFile("testdata/time.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases [][4]any
	if err := json.Unmarshal(buf, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		spec, _ := c[0].(string)
		ms, _ := c[1].(float64)
		local, _ := c[2].(string)
		utc, _ := c[3].(string)
		milli := int64(ms)
		f := Time(spec, EnUS)
		tm := time.UnixMilli(milli)
		if got := f(tm.In(z)); got != local {
			t.Errorf("timeFormat(%q)(%d) = %q, want %q", spec, milli, got, local)
		}
		if got := f(tm.UTC()); got != utc {
			t.Errorf("utcFormat(%q)(%d) = %q, want %q", spec, milli, got, utc)
		}
	}
}

// TestParse checks Parse against the output of d3-time-format, which
// testdata/parse.json holds.
func TestParse(t *testing.T) {
	t.Parallel()
	z := zone(t)
	buf, err := os.ReadFile("testdata/parse.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows [][4]any
	if err := json.Unmarshal(buf, &rows); err != nil {
		t.Fatal(err)
	}
	milli := func(v any) *int64 {
		f, ok := v.(float64)
		if !ok {
			return nil
		}
		n := int64(f)
		return &n
	}
	check := func(name, spec, text string, zone *time.Location, want *int64) {
		got, ok := Parse(spec, EnUS, zone)(text)
		switch {
		case want == nil && ok:
			t.Errorf("%s(%q)(%q) = %d, want no match", name, spec, text, got.UnixMilli())
		case want != nil && !ok:
			t.Errorf("%s(%q)(%q) = no match, want %d", name, spec, text, *want)
		case want != nil && got.UnixMilli() != *want:
			t.Errorf("%s(%q)(%q) = %d, want %d", name, spec, text, got.UnixMilli(), *want)
		}
	}
	for _, r := range rows {
		spec, _ := r[0].(string)
		text, _ := r[1].(string)
		check("timeParse", spec, text, z, milli(r[2]))
		check("utcParse", spec, text, time.UTC, milli(r[3]))
	}
}
