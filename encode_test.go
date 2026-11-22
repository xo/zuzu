package zuzu

import (
	"math"
	"testing"
	"time"

	"github.com/xo/zuzu/scene"
)

// specScene renders a Vega spec to its scene.
func specScene(t *testing.T, src string) *scene.Scene {
	t.Helper()
	c, _, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	s, _, err := c.Render(t.Context(), WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// roleItems returns the items of the marks of a role in a scene.
func roleItems(s *scene.Scene, role string) []scene.Item {
	var out []scene.Item
	var walk func(m *scene.Mark)
	walk = func(m *scene.Mark) {
		for i := range m.Items {
			if m.Role == role {
				out = append(out, m.Items[i])
			}
			for j := range m.Items[i].Items {
				walk(&m.Items[i].Items[j])
			}
		}
	}
	walk(&s.Root)
	return out
}

// TestRootEncode makes sure that the x and the y that the encode of a spec
// gives the root group move its content inside the padding, and that the
// autosize none with "contains": "padding" keeps the padding inside the
// size of the spec.
func TestRootEncode(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, autosize string
		x, w           float64
	}{
		// The width signal is 100 - 2*10 = 80, so x is 10 + 40.
		{"none with the padding", `{"type": "none", "contains": "padding"}`, 50, 100},
		{"none", `"none"`, 60, 120},
		{"pad", `"pad"`, 60, 120},
	} {
		s := specScene(t, `{"$schema": "https://vega.github.io/schema/vega/v5.json",
			"width": 100, "height": 100, "padding": 10, "autosize": `+c.autosize+`,
			"encode": {"enter": {"x": {"signal": "width / 2"}, "y": {"signal": "height / 2"}}},
			"marks": [{"type": "rule", "encode": {"enter": {"x": {"value": -5}, "x2": {"value": 5}, "y": {"value": 0}}}}]}`)
		root := s.Root.Items[0]
		if root.X != c.x || root.Y != c.x {
			t.Errorf("%s: the root group is at %v, %v, want %v, %v", c.name, root.X, root.Y, c.x, c.x)
		}
		if s.Width != c.w {
			t.Errorf("%s: the scene is %v wide, want %v", c.name, s.Width, c.w)
		}
	}
}

// TestAxisOffsetValueRef makes sure that the offset of an axis can be a
// value reference through a scale, as the axes of a chart of parallel
// coordinates use to stand each at its own place.
func TestAxisOffsetValueRef(t *testing.T) {
	t.Parallel()
	s := specScene(t, `{"$schema": "https://vega.github.io/schema/vega/v5.json",
		"width": 200, "height": 100, "padding": 0,
		"data": [{"name": "fields", "values": ["a", "b"]}],
		"scales": [
			{"name": "ord", "type": "point", "range": "width", "domain": {"data": "fields", "field": "data"}},
			{"name": "y", "type": "linear", "range": "height", "domain": [0, 10]}],
		"axes": [{"orient": "left", "scale": "y", "offset": {"scale": "ord", "value": "b", "mult": -1}}]}`)
	labels := roleItems(s, "axis-label")
	if len(labels) == 0 {
		t.Fatal("no labels")
	}
	// The axis stands at x 200, and its labels are just left of it.
	if x := labels[0].X; x < 150 || x > 200 {
		t.Errorf("a label is at x %v, want near 200", x)
	}
}

// TestDomainSignalRows makes sure that a signal in the fields of a domain
// gives a dataset whose field "data" holds the values, as Vega reads it:
// [{data: 0}] and [10] both add their value to the domain. vega-lite.js
// writes a date datum this way.
func TestDomainSignalRows(t *testing.T) {
	t.Parallel()
	s := specScene(t, `{"$schema": "https://vega.github.io/schema/vega/v5.json",
		"width": 100, "height": 100, "padding": 0,
		"data": [{"name": "rows", "values": [{"v": 5}]}],
		"scales": [{"name": "x", "type": "linear", "range": "width", "zero": false,
			"domain": {"fields": [{"data": "rows", "field": "v"}, {"signal": "[{data: 0}]"}, {"signal": "[10]"}]}}],
		"marks": [{"type": "rule", "name": "r", "from": {"data": "rows"},
			"encode": {"enter": {"x": {"scale": "x", "field": "v"}, "y": {"value": 0}, "y2": {"value": 10}}}}]}`)
	m := findMark(&s.Root, "r")
	if m == nil || len(m.Items) != 1 {
		t.Fatalf("the rule mark: %+v", m)
	}
	if x := m.Items[0].X; x != 50 {
		t.Errorf("the rule is at x %v, want 50 in the domain 0 to 10", x)
	}
}

// TestNestedFieldRef makes sure that the name of a field can itself be a
// field reference: {"parent": {"datum": "data"}} reads the field of the
// datum of the group that the field data of the datum names.
func TestNestedFieldRef(t *testing.T) {
	t.Parallel()
	s := specScene(t, `{"$schema": "https://vega.github.io/schema/vega/v5.json",
		"width": 100, "height": 100,
		"data": [{"name": "rows", "values": [{"p": 1, "q": 5}]}, {"name": "fields", "values": ["p", "q"]}],
		"marks": [{"type": "group", "from": {"data": "rows"}, "marks": [{"type": "text", "name": "t", "from": {"data": "fields"},
			"encode": {"enter": {"text": {"field": {"parent": {"datum": "data"}}}}}}]}]}`)
	m := findMark(&s.Root, "t")
	if m == nil || len(m.Items) != 2 {
		t.Fatalf("the text mark: %+v", m)
	}
	for i, want := range []string{"1", "5"} {
		if got := m.Items[i].Text; len(got) != 1 || got[0] != want {
			t.Errorf("text %d = %v, want %s", i, got, want)
		}
	}
}

// TestAreaCenterAndHeight makes sure that an area takes yc and height in
// place of y and y2, as a violin plot does.
func TestAreaCenterAndHeight(t *testing.T) {
	t.Parallel()
	s := specScene(t, `{"$schema": "https://vega.github.io/schema/vega/v5.json",
		"width": 100, "height": 100,
		"data": [{"name": "d", "values": [{"x": 0, "h": 10}, {"x": 10, "h": 20}]}],
		"marks": [{"type": "area", "name": "a", "from": {"data": "d"},
			"encode": {"enter": {"x": {"field": "x"}, "yc": {"value": 50}, "height": {"field": "h"}}}}]}`)
	m := findMark(&s.Root, "a")
	if m == nil || len(m.Items) != 2 {
		t.Fatalf("the area: %+v", m)
	}
	for i, want := range [][2]float64{{45, 55}, {40, 60}} {
		it := m.Items[i]
		if math.Abs(it.Y-want[0]) > 1e-9 || math.Abs(it.Y2-want[1]) > 1e-9 {
			t.Errorf("point %d spans %v to %v, want %v to %v", i, it.Y, it.Y2, want[0], want[1])
		}
	}
}

// TestPaintObject makes sure that an object is a gradient only when its key
// gradient is truthy, as in vega.js. Any other object is the string that
// JavaScript makes of it, which no renderer reads as a color.
func TestPaintObject(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		v        map[string]any
		color    string
		gradient string
	}{
		{"no key gradient", map[string]any{"color": "red"}, "[object Object]", ""},
		{"an empty object", map[string]any{}, "[object Object]", ""},
		{"an empty gradient", map[string]any{"gradient": ""}, "[object Object]", ""},
		{"a null gradient", map[string]any{"gradient": nil}, "[object Object]", ""},
		{"a linear gradient", map[string]any{
			"gradient": "linear",
			"stops":    []any{map[string]any{"offset": 0.0, "color": "red"}},
		}, "", scene.Linear},
		{"a radial gradient", map[string]any{"gradient": "radial", "stops": []any{}}, "", scene.Radial},
	} {
		p := paint(c.v)
		if p.Color != c.color {
			t.Errorf("%s: the color is %q, want %q", c.name, p.Color, c.color)
		}
		switch {
		case c.gradient == "" && p.Gradient != nil:
			t.Errorf("%s: got a gradient %q, want none", c.name, p.Gradient.Type)
		case c.gradient != "" && p.Gradient == nil:
			t.Errorf("%s: got no gradient, want %q", c.name, c.gradient)
		case c.gradient != "" && p.Gradient.Type != c.gradient:
			t.Errorf("%s: the gradient is %q, want %q", c.name, p.Gradient.Type, c.gradient)
		}
	}
}

// TestMarkDataStyle checks that a mark whose data is another mark reads the
// fill, the size, the shape and the opacity of each item of that mark, as
// the arc diagram of Vega does.
func TestMarkDataStyle(t *testing.T) {
	t.Parallel()
	s := specScene(t, `{"$schema": "https://vega.github.io/schema/vega/v5.json", "width": 100, "height": 100,
		"data": [{"name": "rows", "values": [{"v": 1}, {"v": 2}]}],
		"marks": [
			{"type": "symbol", "name": "layout", "from": {"data": "rows"}, "encode": {
				"enter": {"opacity": {"value": 0}},
				"update": {"x": {"field": "v", "mult": 10}, "size": {"field": "v", "mult": 50}, "fill": {"value": "#123456"}, "shape": {"value": "square"}}}},
			{"type": "symbol", "name": "copy", "from": {"data": "layout"}, "encode": {
				"update": {"x": {"field": "x"}, "fill": {"field": "fill"}, "size": {"field": "size"}, "shape": {"field": "shape"}, "opacity": {"field": "opacity", "offset": 1}}}}]}`)
	var items []scene.Item
	for _, m := range s.Root.Items[0].Items {
		if m.Name == "copy" {
			items = m.Items
		}
	}
	if len(items) != 2 {
		t.Fatalf("the copy has %d items, want 2", len(items))
	}
	for i, it := range items {
		// The look can draw the items in another order, so the size
		// that each item wants comes from its x: 10 for each unit of v.
		want := it.X * 5
		size, opacity := math.NaN(), math.NaN()
		if it.Size != nil {
			size = *it.Size
		}
		if it.Opacity != nil {
			opacity = *it.Opacity
		}
		if it.Fill.Color != "#123456" || size != want || it.Shape != "square" || opacity != 1 {
			t.Errorf("item %d = fill %q, size %v, shape %q, opacity %v, want #123456, %v, square and 1", i, it.Fill.Color, size, it.Shape, opacity, want)
		}
	}
}

// TestEndWithNoStart checks that an item with an x2 and no x sits at x2
// less its width, as Vega places it, such as the square of a phase of
// deneb/gantt-chart, which drew at 0 over the names of the tasks, and the
// same for y2. A symbol has no width, so it sits at x2.
func TestEndWithNoStart(t *testing.T) {
	t.Parallel()
	s := specScene(t, `{"$schema": "https://vega.github.io/schema/vega/v5.json", "width": 200, "height": 100,
		"marks": [
			{"type": "symbol", "name": "sq", "encode": {"update": {"x2": {"value": 120}, "y2": {"value": 40}, "shape": {"value": "square"}}}},
			{"type": "text", "name": "tx", "encode": {"update": {"x2": {"value": 80}, "y": {"value": 10}, "text": {"value": "a"}}}}]}`)
	for name, want := range map[string][2]float64{"sq": {120, 40}, "tx": {80, 10}} {
		m := findMark(&s.Root, name)
		if m == nil || len(m.Items) == 0 {
			t.Fatalf("no mark %s", name)
		}
		if it := m.Items[0]; it.X != want[0] || it.Y != want[1] {
			t.Errorf("%s is at %v,%v, want %v,%v", name, it.X, it.Y, want[0], want[1])
		}
	}
}
