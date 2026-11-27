package expr

import (
	"slices"
	"testing"
	"time"
)

// TestKeyOrder checks the order of Object.keys of JavaScript: an index of
// an array first, in the order of its number, and then the other keys in
// the order that the object first set them.
func TestKeyOrder(t *testing.T) {
	t.Parallel()
	got := KeyOrder([]string{"b", "10", "a", "2", "b", "01", "-1", "4294967295", "4294967294"})
	want := []string{"2", "10", "4294967294", "b", "a", "01", "-1", "4294967295"}
	if !slices.Equal(got, want) {
		t.Errorf("KeyOrder = %q, want %q", got, want)
	}
}

// TestProgramKeys checks that Keys finds the object literal that gave an
// object, in the whole expression or in a branch, by its keys.
func TestProgramKeys(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		src  string
		v    map[string]any
		want []string
	}{
		{"{b: 1, a: 2, '3': 0}", map[string]any{"a": 2.0, "b": 1.0, "3": 0.0}, []string{"3", "b", "a"}},
		{"datum.x ? {d: 1, c: 2} : {b: 1, a: 2}", map[string]any{"a": 2.0, "b": 1.0}, []string{"b", "a"}},
		{"datum.x || {b: 1, a: 2}", map[string]any{"a": 2.0, "b": 1.0}, []string{"b", "a"}},
		{"{b: 1, a: 2}", map[string]any{"a": 2.0, "c": 1.0}, nil},
		{"datum", map[string]any{"a": 2.0, "b": 1.0}, nil},
		{"merge({b: 1}, {a: 2})", map[string]any{"a": 2.0, "b": 1.0}, nil},
	} {
		p, err := Compile(tc.src)
		if err != nil {
			t.Fatal(err)
		}
		if got := p.Keys(tc.v); !slices.Equal(got, tc.want) {
			t.Errorf("Keys of %s = %q, want %q", tc.src, got, tc.want)
		}
	}
}

// TestDateString checks the name of the time zone at the end of the text
// of a date, against what toString of Node.js 22 gives with the zone in
// TZ. A zone that Node.js does not know takes the name GMT+hh:mm.
func TestDateString(t *testing.T) {
	t.Parallel()
	zone := func(name string) *time.Location {
		l, err := time.LoadLocation(name)
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	for _, tc := range []struct {
		t    time.Time
		want string
	}{
		{time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), "Thu Jan 01 1970 00:00:00 GMT+0000 (Coordinated Universal Time)"},
		{time.Date(1970, 1, 1, 0, 0, 0, 0, zone("Etc/GMT")), "Thu Jan 01 1970 00:00:00 GMT+0000 (Greenwich Mean Time)"},
		{time.Date(2024, 1, 10, 12, 0, 0, 0, zone("Europe/Dublin")), "Wed Jan 10 2024 12:00:00 GMT+0000 (Greenwich Mean Time)"},
		{time.Date(2024, 7, 10, 13, 0, 0, 0, zone("Europe/Dublin")), "Wed Jul 10 2024 13:00:00 GMT+0100 (Irish Standard Time)"},
		{time.Date(2024, 1, 10, 17, 30, 0, 0, zone("Asia/Kolkata")), "Wed Jan 10 2024 17:30:00 GMT+0530 (India Standard Time)"},
		{time.Date(2024, 3, 20, 12, 0, 0, 0, zone("Africa/Casablanca")), "Wed Mar 20 2024 12:00:00 GMT+0000 (GMT+00:00)"},
		{time.Date(1880, 1, 1, 0, 0, 0, 0, zone("America/New_York")), "Thu Jan 01 1880 00:00:00 GMT-0456 (Eastern Standard Time)"},
		{time.Date(1970, 1, 1, 0, 0, 0, 0, zone("Etc/GMT+5")), "Thu Jan 01 1970 00:00:00 GMT-0500 (GMT-05:00)"},
		{time.Date(2024, 1, 1, 0, 0, 0, 0, time.FixedZone("here", 5*3600+45*60)), "Mon Jan 01 2024 00:00:00 GMT+0545 (GMT+05:45)"},
	} {
		if got := String(tc.t); got != tc.want {
			t.Errorf("String(%v) = %q, want %q", tc.t, got, tc.want)
		}
	}
}
