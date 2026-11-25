package zuzu

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text/emoji"
	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// testNow is the time of the render of the tests of WithNow, in a zone
// that is not UTC, so that a test sees which wall clock a gantt takes.
var testNow = time.Date(2024, 3, 15, 10, 20, 30, 0, time.FixedZone("", 9*3600))

// ganttNowSrc is a gantt of three rows whose tasks span the day of
// testNow.
const ganttNowSrc = "gantt\ndateFormat YYYY-MM-DD\na : 2024-03-10, 10d\nb : 2024-03-12, 2d\nc : 2024-03-14, 5d\n"

// specJSON returns the Vega spec of a chart as JSON.
func specJSON(t *testing.T, c *Chart) string {
	t.Helper()
	b, err := vega.Write(c.Spec())
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// rules returns the items of the rule marks of the data of a scene, and
// not those of its axes.
func rules(m *scene.Mark) []scene.Item {
	var out []scene.Item
	for _, it := range m.Items {
		for j := range it.Items {
			out = append(out, rules(&it.Items[j])...)
		}
	}
	if m.Type == scene.Rule && m.Role == "mark" {
		out = append(out, m.Items...)
	}
	return out
}

// TestNowGantt checks that a gantt with the time of WithNow draws the
// marker of today, with the style of todayMarker, and that a gantt without
// it, or with todayMarker off, draws none (D38).
func TestNowGantt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// The marker is at the wall clock that the time has in its own zone.
	const at = `"2024-03-15T10:20:30.000"`
	now := []Option{WithNow(testNow)}
	// With no stroke of todayMarker, the marker takes the color of a fall
	// of the scheme (D38).
	fall := theme.New(theme.Light, theme.Zuzu).Down
	tests := []struct {
		name, src string
		opts      []Option
		marker    bool
		stroke    string
		width     float64
		issue     string
	}{
		{"no time", ganttNowSrc, nil, false, "", 0, ""},
		{"time", ganttNowSrc, now, true, fall, 1.5, ""},
		{"off", ganttNowSrc + "todayMarker off\n", now, false, "", 0, ""},
		{"style", ganttNowSrc + "todayMarker stroke:#00ff00,stroke-width:5px,fill:red\n", now, true, "#00ff00", 5, ""},
		{"unknown style", ganttNowSrc + "todayMarker stroke-linejoin:round\n", now, true, fall, 1.5, "stroke-linejoin"},
		{"style with no time", ganttNowSrc + "todayMarker stroke:#00ff00\n", nil, false, "", 0, "WithNow"},
		{"outside the axis", "gantt\ndateFormat YYYY-MM-DD\na : 2023-01-01, 3d\n", now, false, "", 0, ""},
	}
	for _, test := range tests {
		c, report, err := Parse([]byte(test.src), test.opts...)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if got := strings.Contains(specJSON(t, c), at); got != test.marker {
			t.Errorf("%s: the spec holds the time of the marker: %t, want %t", test.name, got, test.marker)
		}
		switch {
		case test.issue == "" && len(report) != 0:
			t.Errorf("%s: the report %v, want none", test.name, report)
		case test.issue != "" && (len(report) != 1 || !strings.Contains(report[0].Message, test.issue)):
			t.Errorf("%s: the report %v, want one issue with %q", test.name, report, test.issue)
		}
		if test.issue != "" {
			if _, _, err := Parse([]byte(test.src), append(test.opts, WithStrict(true))...); err == nil {
				t.Errorf("%s: no error in the strict mode", test.name)
			}
		}
		s, _, err := c.Render(ctx)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		rs := rules(&s.Root)
		if !test.marker {
			if len(rs) != 0 {
				t.Errorf("%s: %d rules, want none", test.name, len(rs))
			}
			continue
		}
		if len(rs) == 0 {
			t.Fatalf("%s: no rule", test.name)
		}
		r := rs[0]
		if r.StrokeWidth == nil || *r.StrokeWidth != test.width {
			t.Errorf("%s: the width of the marker %v, want %v", test.name, r.StrokeWidth, test.width)
		}
		if test.stroke != "" && r.Stroke.Color != test.stroke {
			t.Errorf("%s: the stroke of the marker %q, want %q", test.name, r.Stroke.Color, test.stroke)
		}
		// The marker runs across the three rows of the gantt.
		if r.Y2-r.Y < 3*26-1 {
			t.Errorf("%s: the marker runs from %v to %v, want it across the chart", test.name, r.Y, r.Y2)
		}
	}
}

// TestNowGanttMargin checks that the marker of today just past the end of
// the time axis draws in the margin of Mermaid, at its place, and does not
// stretch the axis.
func TestNowGanttMargin(t *testing.T) {
	t.Parallel()
	// The axis runs 14 days to 2024-03-15, and the time is 10 hours and
	// 20 minutes after its end.
	src := "gantt\ndateFormat YYYY-MM-DD\na : 2024-03-01, 14d\n"
	c, report, err := Parse([]byte(src), WithNow(testNow))
	if err != nil || len(report) != 0 {
		t.Fatalf("the report %v and the error %v", report, err)
	}
	s, _, err := c.Render(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	rs := rules(&s.Root)
	if len(rs) == 0 {
		t.Fatal("no marker")
	}
	want := 640 * (14 + (10+20.0/60+30.0/3600)/24) / 14
	if x := rs[0].X; math.Abs(x-want) > 0.01 {
		t.Errorf("the marker is at %v, want %v", x, want)
	}
	if r := rs[0]; r.Y2-r.Y < 26-1 {
		t.Errorf("the marker runs from %v to %v, want it across the chart", r.Y, r.Y2)
	}
}

// TestNowGanttDay checks that a dateFormat with no date puts the times of a
// gantt on the day of the time of WithNow, and on 1970-01-01 without it
// (D38, D31).
func TestNowGanttDay(t *testing.T) {
	t.Parallel()
	const src = "gantt\ndateFormat HH:mm\naxisFormat %H:%M\na : 09:00, 2h\nb : 13:30, 30m\n"
	for _, test := range []struct {
		opts []Option
		want []string
	}{
		{[]Option{WithNow(testNow)}, []string{`"2024-03-15T09:00:00.000"`, `"2024-03-15T11:00:00.000"`, `"2024-03-15T13:30:00.000"`}},
		{nil, []string{`"1970-01-01T09:00:00.000"`, `"1970-01-01T11:00:00.000"`, `"1970-01-01T13:30:00.000"`}},
	} {
		c, report, err := Parse([]byte(src), test.opts...)
		if err != nil {
			t.Fatal(err)
		}
		if len(report) != 0 {
			t.Errorf("the report %v, want none", report)
		}
		spec := specJSON(t, c)
		for _, w := range test.want {
			if !strings.Contains(spec, w) {
				t.Errorf("the spec does not hold the time %s", w)
			}
		}
	}
}

// TestNowExpression checks that now() gives the time of WithNow, and 0
// without it, with an issue that names the expression, or ErrNoNow in the
// strict mode (D38).
func TestNowExpression(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const src = `{"$schema": "https://vega.github.io/schema/vega/v5.json",
		"signals": [{"name": "t", "update": "now()"}, {"name": "u", "update": "t + 1"}],
		"marks": [{"type": "text", "encode": {"enter": {"text": {"signal": "u"}}}}]}`
	signal := func(c *Chart, name string) (any, Report) {
		t.Helper()
		v, report, err := c.NewView(ctx)
		if err != nil {
			t.Fatal(err)
		}
		got, err := v.Signal(name)
		if err != nil {
			t.Fatal(err)
		}
		return got, report
	}
	c, _, err := Parse([]byte(src), WithNow(testNow))
	if err != nil {
		t.Fatal(err)
	}
	if got, report := signal(c, "t"); got != float64(testNow.UnixMilli()) || len(report) != 0 {
		t.Errorf("now() with the time: %v and the report %v, want %d and none", got, report, testNow.UnixMilli())
	}
	c, _, err = Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got, report := signal(c, "t")
	if got != 0.0 {
		t.Errorf("now() with no time: %v, want 0", got)
	}
	if len(report) != 1 || !errors.Is(report[0], ErrNoNow) || !strings.Contains(report[0].Message, `"now()"`) {
		t.Errorf("the report %v, want one issue of ErrNoNow that names now()", report)
	}
	strict, _, err := Parse([]byte(src), WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := strict.NewView(ctx); !errors.Is(err, ErrNoNow) {
		t.Errorf("the error %v in the strict mode, want ErrNoNow", err)
	}
	// A function now that the caller gives needs no time.
	own, _, err := Parse([]byte(src), WithStrict(true), WithFunction("now", func(*expr.Scope, []any) (any, error) { return 7.0, nil }))
	if err != nil {
		t.Fatal(err)
	}
	if got, report := signal(own, "u"); got != 8.0 || len(report) != 0 {
		t.Errorf("the own now(): %v and the report %v, want 8 and none", got, report)
	}
}

// TestEmojiParse checks that a kind of Mermaid that lays out when it
// compiles measures an emoji with the font of WithEmoji of Parse, and that
// a render with no font of its own draws with that font (D38).
func TestEmojiParse(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, src := range []string{
		"sequenceDiagram\n  participant A as 🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀\n  participant B\n  A->>B: hi\n",
		"mindmap\n  root((🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀))\n",
	} {
		kind, _, _ := strings.Cut(src, "\n")
		width := func(opts ...Option) (float64, Report) {
			t.Helper()
			c, _, err := Parse([]byte(src), opts...)
			if err != nil {
				t.Fatalf("%s: %v", kind, err)
			}
			s, report, err := c.Render(ctx)
			if err != nil {
				t.Fatalf("%s: %v", kind, err)
			}
			return s.Width, report
		}
		bare, report := width()
		if len(report) == 0 || !errors.Is(report[0], ErrNoGlyph) {
			t.Errorf("%s: the report %v with no font, want ErrNoGlyph", kind, report)
		}
		wide, report := width(WithEmoji(emoji.Font()))
		if len(report) != 0 {
			t.Errorf("%s: the report %v with the font of the parse, want none", kind, report)
		}
		// Sixteen emoji are more than 200 pixels wide at the size of a
		// label.
		if wide < bare+150 {
			t.Errorf("%s: the width %v with the font, and %v with none, want it wider by the emoji", kind, wide, bare)
		}
	}
}
