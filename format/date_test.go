package format

import (
	"encoding/json/v2"
	"os"
	"testing"
	"time"
)

// TestParseDate checks ParseDate against Date.parse of JavaScript, which
// testdata/date.json holds.
func TestParseDate(t *testing.T) {
	t.Parallel()
	z := zone(t)
	buf, err := os.ReadFile("testdata/date.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows [][2]any
	if err := json.Unmarshal(buf, &rows); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		s, _ := r[0].(string)
		got, ok := ParseDate(s, z)
		want, wantOK := r[1].(float64)
		switch {
		case ok != wantOK:
			t.Errorf("ParseDate(%q) = %v, %t, want %v", s, got, ok, r[1])
		case ok && got.UnixMilli() != int64(want):
			t.Errorf("ParseDate(%q) = %d, want %d", s, got.UnixMilli(), int64(want))
		}
	}
}

func TestInterval(t *testing.T) {
	t.Parallel()
	z := zone(t)
	at := time.Date(2024, 2, 29, 13, 45, 30, 250e6, z) // A Thursday.
	tests := []struct {
		i     Interval
		floor string
		ceil  string
		next  string
	}{
		// A year from February 29 is March 1, as setFullYear of JavaScript
		// moves it.
		{Year, "2024-01-01T00:00:00", "2025-01-01T00:00:00", "2025-03-01T13:45:30"},
		{Quarter, "2024-01-01T00:00:00", "2024-04-01T00:00:00", "2024-05-29T13:45:30"},
		{Month, "2024-02-01T00:00:00", "2024-03-01T00:00:00", "2024-03-29T13:45:30"},
		{Week, "2024-02-25T00:00:00", "2024-03-03T00:00:00", "2024-03-07T13:45:30"},
		{Day, "2024-02-29T00:00:00", "2024-03-01T00:00:00", "2024-03-01T13:45:30"},
		{Hours, "2024-02-29T13:00:00", "2024-02-29T14:00:00", "2024-02-29T14:45:30"},
		{Minutes, "2024-02-29T13:45:00", "2024-02-29T13:46:00", "2024-02-29T13:46:30"},
		{Seconds, "2024-02-29T13:45:30", "2024-02-29T13:45:31", "2024-02-29T13:45:31"},
	}
	const layout = "2006-01-02T15:04:05"
	for _, test := range tests {
		if got := test.i.Floor(at).Format(layout); got != test.floor {
			t.Errorf("%s.Floor = %s, want %s", test.i, got, test.floor)
		}
		if got := test.i.Ceil(at).Format(layout); got != test.ceil {
			t.Errorf("%s.Ceil = %s, want %s", test.i, got, test.ceil)
		}
		if got := test.i.Offset(at, 1).Format(layout); got != test.next {
			t.Errorf("%s.Offset = %s, want %s", test.i, got, test.next)
		}
	}
	r := Month.Range(time.Date(2024, 1, 15, 0, 0, 0, 0, z), time.Date(2024, 6, 1, 0, 0, 0, 0, z), 2)
	var got []string
	for _, x := range r {
		got = append(got, x.Format("2006-01-02"))
	}
	if want := []string{"2024-02-01", "2024-04-01"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Month.Range = %v, want %v", got, want)
	}
}

// TestTimeUnitSpecifier checks TimeUnitSpecifier against timeUnitSpecifier
// of vega-time 6.
func TestTimeUnitSpecifier(t *testing.T) {
	t.Parallel()
	custom := map[string]string{"year-month": "%b %Y ", "year-month-date": "%b %d, %Y "}
	tests := []struct {
		units        []string
		want, custom string
	}{
		{[]string{"year"}, "%Y", "%Y"},
		{[]string{"year", "month"}, "%Y-%m", "%b %Y"},
		{[]string{"year", "month", "date"}, "%Y-%m-%d", "%b %d, %Y"},
		{[]string{"month", "date"}, "%b %d", "%b %d"},
		{[]string{"hours", "minutes", "seconds"}, "%H:%M:%S", "%H:%M:%S"},
		{[]string{"year", "quarter"}, "%Y Q%q", "%Y Q%q"},
		{[]string{"year", "week"}, "%Y W%U", "%Y W%U"},
		{[]string{"dayofyear"}, "%j", "%j"},
		{[]string{"year", "month", "date", "hours"}, "%Y-%m-%d %H:00", "%b %d, %Y %H:00"},
		{[]string{"minutes", "seconds", "milliseconds"}, "00:%M:%S.%L", "00:%M:%S.%L"},
	}
	for _, test := range tests {
		if got := TimeUnitSpecifier(test.units, nil); got != test.want {
			t.Errorf("TimeUnitSpecifier(%v) = %q, want %q", test.units, got, test.want)
		}
		if got := TimeUnitSpecifier(test.units, custom); got != test.custom {
			t.Errorf("TimeUnitSpecifier(%v, custom) = %q, want %q", test.units, got, test.custom)
		}
	}
}
