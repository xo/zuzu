package zuzu

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // The tests read a named zone, not the zone of the machine.

	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// renderTips renders a spec in the loose mode in a time zone, and returns
// the tooltip of the first item of each named mark, and the report.
func renderTips(t *testing.T, src []byte, zone string) (map[string]string, []error) {
	t.Helper()
	loc, err := time.LoadLocation(zone)
	if err != nil {
		t.Fatal(err)
	}
	c, _, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	s, report, err := c.Render(t.Context(), WithLocation(loc))
	if err != nil {
		t.Fatal(err)
	}
	tips := map[string]string{}
	var walk func(m *scene.Mark)
	walk = func(m *scene.Mark) {
		for i := range m.Items {
			if _, ok := tips[m.Name]; !ok && m.Name != "" {
				tips[m.Name] = string(m.Items[i].Tooltip)
			}
			for j := range m.Items[i].Items {
				walk(&m.Items[i].Items[j])
			}
		}
	}
	walk(&s.Root)
	errs := make([]error, len(report))
	for i, is := range report {
		errs[i] = is
	}
	return tips, errs
}

// itemTipSpec gives marks a tooltip that is the item itself, in each way
// that changes the fields that vega.js shows: a mark with no data, a key
// that a later set sets again, a value of undefined and of null, the place
// that x2 and yc give after a set, a date, the defaults of the config, and
// group items with no marks, with guides and with a facet.
const itemTipSpec = `{"$schema": "https://vega.github.io/schema/vega/v6.json", "width": 200, "height": 100,
 "data": [{"name": "t", "values": [{"a": 1, "b": "x"}, {"a": 2, "b": "y"}]}],
 "scales": [{"name": "x", "type": "linear", "domain": [0, 3], "range": "width"}],
 "marks": [
  {"type": "rect", "name": "nofrom", "encode": {"enter": {
    "tooltip": {"signal": "item"}, "x": {"value": 1}, "x2": {"value": 5}, "y": {"value": 2},
    "height": {"value": 3}, "fill": {"value": "red"}}}},
  {"type": "rect", "name": "x2only", "from": {"data": "t"}, "encode": {
    "enter": {"x2": {"field": "a"}, "width": {"value": 3}, "yc": {"value": 10}, "height": {"value": 4},
      "fill": {"signal": "null"}, "stroke": {"field": "missing"}, "strokeDash": {"value": [4, 2]},
      "description": {"signal": "datetime(2020, 0, 1)"}},
    "update": {"tooltip": {"signal": "item"}, "x2": {"value": 9}, "opacity": {"value": 0.5}}}},
  {"type": "group", "name": "grp", "encode": {"enter": {
    "x": {"value": 3}, "width": {"value": 20}, "height": {"value": 20}, "tooltip": {"signal": "item"}}},
   "marks": [{"type": "text", "name": "inner", "from": {"data": "t"}, "encode": {
     "enter": {"text": {"field": "b"}, "x": {"field": "a"}, "tooltip": {"signal": "item"}, "fontSize": {"value": 12}},
     "update": {"x": {"value": 7}, "angle": {"value": 0}}}}]},
  {"type": "symbol", "name": "sym", "from": {"data": "t"}, "encode": {"update": {
    "x": {"field": "a"}, "y": {"value": 2}, "tooltip": {"signal": "item"}}}},
  {"type": "line", "name": "ln", "from": {"data": "t"}, "encode": {"update": {
    "x": {"field": "a"}, "y": {"value": 2}, "tooltip": {"signal": "item"}}}},
  {"type": "group", "name": "empty", "encode": {"enter": {"tooltip": {"signal": "item"}, "fill": {"value": "red"}}}},
  {"type": "group", "name": "guides", "title": {"text": "T"},
   "axes": [{"scale": "x", "orient": "bottom"}, {"scale": "x", "orient": "top", "grid": true}],
   "legends": [{"fill": "x"}],
   "encode": {"enter": {"tooltip": {"signal": "item"}, "width": {"value": 50}}},
   "marks": [{"type": "rect", "from": {"data": "t"}, "encode": {"enter": {"x": {"value": 1}}}}]},
  {"type": "group", "name": "faceted", "from": {"facet": {"data": "t", "name": "f", "groupby": "b"}},
   "encode": {"update": {"tooltip": {"signal": "item"}}},
   "marks": [
    {"type": "rect", "from": {"data": "f"}, "encode": {"enter": {"x": {"value": 1}}}},
    {"type": "text", "from": {"data": "f"}, "encode": {"enter": {"x": {"value": 1}}}}]}
 ]}`

// TestItemTooltip checks the text of a tooltip that is the item itself
// against the text that vega.js 6.4.0 gives for the same spec, in UTC
// (D61). The fields that the config of vega.js sets are its look, and zuzu
// shows the defaults of its own theme there instead. Those marks state the
// text of vega.js and the lines of zuzu that take the place of its lines.
func TestItemTooltip(t *testing.T) {
	t.Parallel()
	th := theme.New(theme.Light, theme.Zuzu)
	c := th.Category[0]
	for _, tc := range []struct {
		name, vega string
		// config is the lines of the config of vega.js, and look the lines
		// of the theme of zuzu that take their place.
		config, look string
	}{
		{name: "nofrom", vega: "datum: {…}\nexit: false\nmark: {…}\nbounds: {…}\ntooltip: {…}\nx: 1\nx2: 5\ny: 2\nheight: 3\nfill: red\nwidth: 4"},
		{name: "x2only", vega: "datum: {…}\nexit: false\nmark: {…}\nbounds: {…}\nx2: 9\nwidth: 3\nyc: 10\nheight: 4\nfill: null\nstrokeDash: [4, 2]\ndescription: Wed Jan 01 2020 00:00:00 GMT+0000 (Coordinated Universal Time)\nx: 6\ny: 8\ntooltip: {…}\nopacity: 0.5"},
		{name: "grp", vega: "datum: {…}\nexit: false\nmark: {…}\nbounds: {…}\nitems: [{…}]\nx: 3\nwidth: 20\nheight: 20\ntooltip: {…}\ncontext: {…}"},
		{
			name: "inner", vega: "datum: {…}\nexit: false\nmark: {…}\nbounds: {…}\nfill: #000\nfont: sans-serif\ntext: x\nx: 7\ntooltip: {…}\nfontSize: 12\nangle: 0",
			config: "fill: #000\nfont: sans-serif\n", look: "fill: " + th.Text + "\n",
		},
		{
			name: "sym", vega: "datum: {…}\nexit: false\nmark: {…}\nbounds: {…}\nfill: #4c78a8\nsize: 64\nx: 1\ny: 2\ntooltip: {…}",
			config: "fill: #4c78a8\nsize: 64\n", look: "fill: " + c + "\nsize: 64\nstroke: " + th.Background + "\nstrokeWidth: 1\n",
		},
		{
			name: "ln", vega: "datum: {…}\nexit: false\nmark: {…}\nbounds: {…}\nstroke: #4c78a8\nstrokeWidth: 2\nx: 1\ny: 2\ntooltip: {…}",
			config: "stroke: #4c78a8\nstrokeWidth: 2\n", look: "stroke: " + c + "\nstrokeCap: round\nstrokeJoin: round\nstrokeWidth: 2\n",
		},
		{name: "empty", vega: "datum: {…}\nexit: false\nmark: {…}\nbounds: {…}\nitems: []\ntooltip: {…}\nfill: red"},
		{name: "guides", vega: "datum: {…}\nexit: false\nmark: {…}\nbounds: {…}\nitems: [{…}, {…}, {…}, {…}, {…}]\ntooltip: {…}\nwidth: 50\ncontext: {…}"},
		{name: "faceted", vega: "datum: {…}\nexit: false\nmark: {…}\nbounds: {…}\nitems: [{…}, {…}]\ntooltip: {…}\ncontext: {…}"},
	} {
		tips, report := renderTips(t, []byte(itemTipSpec), "UTC")
		if len(report) != 0 {
			t.Fatalf("the report = %v, want none", report)
		}
		want := strings.Replace(tc.vega, tc.config, tc.look, 1)
		if got := tips[tc.name]; got != want {
			t.Errorf("the tooltip of %s = %q, want %q", tc.name, got, want)
		}
	}
}

// TestCoronationArcTooltip checks the tooltips of the item of the five arcs
// of the spec coronation-arc of the corpus, the only spec of the corpus
// that shows the item, against the text that vega.js 6.4.0 gives. The
// theme of zuzu gives an arc a corner radius and a pad angle, which vega.js
// does not, and which come first, with the defaults of the config (D61).
func TestCoronationArcTooltip(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("test/testdata/corpus/deneb/coronation-arc.vg.json")
	if err != nil {
		t.Fatal(err)
	}
	tips, report := renderTips(t, src, "UTC")
	for _, is := range report {
		if strings.Contains(is.Error(), "tooltip") {
			t.Errorf("the report holds %v", is)
		}
	}
	for name, vega := range map[string]string{
		"life":  "datum: {…}\nexit: false\nmark: {…}\nbounds: {…}\nfill: silver\nx: 35\ny: 35\nstartAngle: 0\nendAngle: 3.1415926535897936\ninnerRadius: 28\nouterRadius: 29\ntooltip: {…}",
		"reign": "datum: {…}\nexit: false\nmark: {…}\nbounds: {…}\nfill: #e7298a\nx: 35\ny: 35\nstartAngle: 2.324778563656447\nendAngle: 3.19185813604723\ninnerRadius: 25\nouterRadius: 32\ntooltip: {…}",
		"key":   "datum: {…}\nexit: false\nmark: {…}\nbounds: {…}\nfill: silver\nx: 817\ny: 1270\nstartAngle: 0\nendAngle: 4.71238898038469\ninnerRadius: 28\nouterRadius: 29\ntooltip: {…}",
		"key2":  "datum: {…}\nexit: false\nmark: {…}\nbounds: {…}\nfill: silver\nx: 817\ny: 1270\nstartAngle: 1.5707963267948968\nendAngle: 3.1415926535897936\ninnerRadius: 25\nouterRadius: 32\ntooltip: {…}",
		"key3":  "datum: {…}\nexit: false\nmark: {…}\nbounds: {…}\nfill: silver\nx: 942\ny: 1270\nstartAngle: 0\nendAngle: 6.283185307179586\ninnerRadius: 28\nouterRadius: 29\ntooltip: {…}",
	} {
		want := strings.Replace(vega, "bounds: {…}\n", "bounds: {…}\ncornerRadius: 4\npadAngle: 0.01\n", 1)
		if got := tips[name]; got != want {
			t.Errorf("the tooltip of %s = %q, want %q", name, got, want)
		}
	}
}

// TestTooltipObject checks the text of a tooltip that is an object, an
// array or a date against the text that vega.js 6.4.0 gives in the time
// zone America/New_York: the keys in the order of the object, null in an
// array as nothing, and a date as Date.toString() in the time zone of the
// call (D61). A field that the type date parses is a number in vega.js, as
// in zuzu.
func TestTooltipObject(t *testing.T) {
	t.Parallel()
	src := `{"$schema": "https://vega.github.io/schema/vega/v6.json", "width": 50, "height": 40,
	 "data": [
	  {"name": "t", "values": [{"d": "2020-07-01"}], "format": {"parse": {"d": "date:'%Y-%m-%d'"}}},
	  {"name": "u", "values": [{"d": "2020-07-01"}], "format": {"parse": {"d": "date"}}}],
	 "marks": [
	  {"type": "rect", "name": "lit", "encode": {"enter": {"x": {"value": 1}, "tooltip": {"signal": "{'b': 1, 'a': 2, '10': 3, '2': [1, null, 'x', {}], 'd': datetime(2020, 0, 1)}"}}}},
	  {"type": "rect", "name": "val", "encode": {"enter": {"tooltip": {"value": {"z": 1, "y": [2, null], "1": 0}}}}},
	  {"type": "rect", "name": "arr", "encode": {"enter": {"tooltip": {"signal": "[1, null, datetime(2020, 6, 1), [2], {'a': 1}]"}}}},
	  {"type": "rect", "name": "date", "encode": {"enter": {"tooltip": {"signal": "datetime(2020, 6, 1)"}}}},
	  {"type": "rect", "name": "field", "from": {"data": "t"}, "encode": {"enter": {"tooltip": {"field": "d"}}}},
	  {"type": "rect", "name": "plain", "from": {"data": "u"}, "encode": {"enter": {"tooltip": {"field": "d"}}}},
	  {"type": "rect", "name": "cond", "from": {"data": "t"}, "encode": {"enter": {"tooltip": {"signal": "datum.x ? 1 : {b: 1, a: 2}"}}}}
	 ]}`
	tips, report := renderTips(t, []byte(src), "America/New_York")
	if len(report) != 0 {
		t.Fatalf("the report = %v, want none", report)
	}
	for name, want := range map[string]string{
		"lit":   "2: [1, , x, {…}]\n10: 3\nb: 1\na: 2\nd: Wed Jan 01 2020 00:00:00 GMT-0500 (Eastern Standard Time)",
		"val":   "1: 0\nz: 1\ny: [2, ]",
		"arr":   "[1, , Wed Jul 01 2020 00:00:00 GMT-0400 (Eastern Daylight Time), […], {…}]",
		"date":  "Wed Jul 01 2020 00:00:00 GMT-0400 (Eastern Daylight Time)",
		"field": "Wed Jul 01 2020 00:00:00 GMT-0400 (Eastern Daylight Time)",
		"plain": "1593561600000",
		"cond":  "b: 1\na: 2",
	} {
		if got := tips[name]; got != want {
			t.Errorf("the tooltip of %s = %q, want %q", name, got, want)
		}
	}
}

// TestTooltipOfAnotherPart checks that a tooltip that shows a part of the
// scene other than its own item, or the item of the root group, goes in
// the report and shows nothing, and is an error in the strict mode.
func TestTooltipOfAnotherPart(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, src, want string }{
		{"mark", `{"$schema": "https://vega.github.io/schema/vega/v6.json", "width": 50, "height": 40,
		  "marks": [{"type": "rect", "name": "r", "encode": {"enter": {"tooltip": {"signal": "item.mark"}}}}]}`,
			"a part of the scene other than its own item"},
		{"root", `{"$schema": "https://vega.github.io/schema/vega/v6.json", "width": 50, "height": 40,
		  "encode": {"enter": {"tooltip": {"signal": "item"}}}}`,
			"the item of the root group"},
	} {
		tips, report := renderTips(t, []byte(tc.src), "UTC")
		if len(report) != 1 || !errors.Is(report[0], vega.ErrUnsupported) || !strings.Contains(report[0].Error(), tc.want) {
			t.Errorf("%s: the report = %v, want one issue of %s", tc.name, report, tc.want)
		}
		if tips["r"] != "" {
			t.Errorf("%s: the tooltip = %q, want none", tc.name, tips["r"])
		}
		c, _, err := Parse([]byte(tc.src), WithStrict(true))
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := c.Render(t.Context(), WithLocation(time.UTC)); !errors.Is(err, vega.ErrUnsupported) {
			t.Errorf("%s: the strict mode = %v, want %v", tc.name, err, vega.ErrUnsupported)
		}
	}
}
