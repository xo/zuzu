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
