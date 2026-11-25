package zuzu

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu/vega"
)

// itemSpec is a spec whose encodings read item: the size, the datum, a
// property and the mark of the group of a mark, and the kind and the name
// of the mark. %s is a property of the rect that reads item.
const itemSpec = `{"$schema": "https://vega.github.io/schema/vega/v5.json",
	"width": 200, "height": 100,
	"data": [{"name": "t", "values": [{"k": "a", "v": 40}]}],
	"marks": [
		{"type": "rect", "role": "top", "encode": {"update": {"width": {"signal": "item.mark.group.width"}}}},
		{"type": "group", "name": "g", "from": {"data": "t"},
			"encode": {"update": {"x": {"value": 10}, "width": {"value": 100}, "height": {"value": 50}, "fill": {"value": "red"}}},
			"marks": [
				{"type": "rect", "role": "bar", "encode": {"update": {
					"width": {"signal": "item.mark.group.width * item.mark.group.datum.v / 100"},
					"height": {"signal": "item.mark.group.height"},
					"fill": {"signal": "item.mark.group.fill"},
					"x": {"signal": "item.mark.group.x1"}%s}}},
				{"type": "text", "name": "t", "role": "tag", "encode": {"update": {
					"text": {"signal": "item.mark.marktype + ' ' + item.mark.name + ' ' + item.mark.role + ' ' + item.mark.group.mark.name"}}}}
			]}
	]}`

// TestItemInEncode checks that an encoding reads the item that it fills, as
// the scene graph of Vega gives it, and that a part of item that zuzu does
// not keep goes in the report, or is an error in the strict mode.
func TestItemInEncode(t *testing.T) {
	t.Parallel()
	s := specScene(t, strings.Replace(itemSpec, "%s", "", 1))
	top := roleItems(s, "top")
	if len(top) != 1 || top[0].Width != 200 {
		t.Fatalf("the rect at the top = %+v, want the width 200 of the root group", top)
	}
	bar := roleItems(s, "bar")
	if len(bar) != 1 {
		t.Fatalf("%d bars, want 1", len(bar))
	}
	// x1 is no property of a group, so it is undefined and x is 0, as in
	// Vega.
	if b := bar[0]; b.Width != 40 || b.Height != 50 || b.Fill.Color != "red" || b.X != 0 {
		t.Errorf("the bar = %v %v %v at %v, want 40 by 50, red, at 0", b.Width, b.Height, b.Fill.Color, b.X)
	}
	tag := roleItems(s, "tag")
	if len(tag) != 1 || strings.Join(tag[0].Text, "") != "text t tag g" {
		t.Errorf("the tag = %+v, want the text \"text t tag g\"", tag)
	}

	bounds := strings.Replace(itemSpec, "%s", `, "y": {"signal": "item.bounds"}`, 1)
	c, _, err := Parse([]byte(bounds))
	if err != nil {
		t.Fatal(err)
	}
	_, report, err := c.Render(t.Context(), WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, is := range report {
		if errors.Is(is, vega.ErrUnsupported) && strings.Contains(is.Error(), "bounds") {
			found++
		}
	}
	if found != 1 {
		t.Errorf("the report = %v, want item.bounds once", report)
	}
	c, _, err = Parse([]byte(bounds), WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.Render(t.Context(), WithLocation(time.UTC)); !errors.Is(err, vega.ErrUnsupported) {
		t.Errorf("the strict mode = %v, want %v", err, vega.ErrUnsupported)
	}
}

// TestItemTooltip checks that a tooltip that shows the item goes in the
// report once for its mark, and draws no tooltip.
func TestItemTooltip(t *testing.T) {
	t.Parallel()
	c, _, err := Parse([]byte(`{"$schema": "https://vega.github.io/schema/vega/v5.json",
		"width": 100, "height": 100,
		"data": [{"name": "t", "values": [{"v": 1}, {"v": 2}]}],
		"marks": [{"type": "rect", "role": "bar", "from": {"data": "t"},
			"encode": {"update": {"width": {"value": 5}, "tooltip": {"signal": "item"}}}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	s, report, err := c.Render(t.Context(), WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 1 || !errors.Is(report[0], vega.ErrUnsupported) {
		t.Errorf("the report = %v, want one tooltip of an item", report)
	}
	for _, it := range roleItems(s, "bar") {
		if it.Tooltip != "" {
			t.Errorf("the tooltip = %q, want none", it.Tooltip)
		}
	}
}

// itemOrderSpec is a spec whose encodings read item in each way that
// vega.js gives it: a property that an earlier property or an earlier set
// set, the place that x2 and y2 give after each set, the items of a mark
// before and after their encodings run, the marks of a group item, and the
// mark of the root group.
const itemOrderSpec = `{"$schema": "https://vega.github.io/schema/vega/v5.json", "width": 200, "height": 100,
 "data": [{"name": "t", "values": [{"v": 1}, {"v": 2}, {"v": 3}]}],
 "encode": {"update": {"description": {"signal": "item.mark.marktype + '|' + item.mark.role + '|' + item.mark.name + '|' + item.mark.items.length + '|' + item.mark.group + '|' + item.items.length"}}},
 "marks": [
  {"type": "rect", "name": "r", "from": {"data": "t"}, "encode": {
    "enter": {"x": {"value": 50}, "x2": {"value": 10}, "y": {"signal": "item.width"}},
    "update": {"height": {"signal": "item.width"}, "y2": {"value": 70}}}},
  {"type": "text", "name": "a", "role": "a", "from": {"data": "t"}, "encode": {
    "enter": {"x": {"value": 5}, "y": {"signal": "item.x + datum.v"}},
    "update": {"text": {"signal": "item.x + '|' + item.y + '|' + item.width + '|' + item.text + '|' + item.mark.items.length + '|' + item.mark.items[0].y + '|' + item.mark.items[2].x + '|' + item.datum.v"}}}},
  {"type": "text", "name": "b", "role": "b", "encode": {"update": {"text": {"signal": "item.mark.group.items.length + '|' + item.mark.group.items[0].marktype + '|' + item.mark.group.items[0].items[1].x + '|' + item.mark.group.items[0].items[1].x2 + '|' + item.mark.group.items[0].items[1].y + '|' + item.mark.group.items[0].items[1].width + '|' + item.mark.group.items[0].items[1].height + '|' + item.mark.group.items[1].name + '|' + item.mark.group.mark.name + '|' + item.mark.group.mark.role + '|' + item.mark.group.mark.group + '|' + item.mark.group.description"}}}},
  {"type": "group", "name": "g", "from": {"data": "t"}, "encode": {"update": {"x": {"value": 1}, "width": {"signal": "item.items.length"}, "height": {"signal": "item.x + 7"}}},
   "marks": [{"type": "text", "name": "gt", "role": "gt", "encode": {"update": {"text": {"signal": "item.mark.group.items.length + '|' + item.mark.group.width + '|' + item.mark.group.height + '|' + item.mark.group.mark.items.length + '|' + item.mark.group.mark.items[0].items.length + '|' + item.mark.group.datum.v + '|' + item.mark.group.mark.group.items.length"}}}}]}
 ]}`

// itemAxisSpec is a spec whose encoding of the labels of an axis reads
// item.mark, the place that the layout gives a label, and the text that
// the encoding sets.
const itemAxisSpec = `{"$schema": "https://vega.github.io/schema/vega/v5.json", "width": 200, "height": 100,
 "scales": [{"name": "x", "type": "linear", "domain": [0, 3], "range": "width"}],
 "axes": [{"orient": "bottom", "scale": "x", "values": [0, 3], "encode": {"labels": {"name": "lab", "update": {"text": {"signal": "item.mark.marktype + '|' + item.mark.role + '|' + item.mark.name + '|' + item.mark.items.length + '|' + item.mark.group.mark.role + '|' + item.mark.group.mark.group.mark.name + '|' + item.datum.value + '|' + item.x + '|' + item.text"}}}}}]
}`

// TestItemOrder checks that an encoding reads item as vega.js 6.4.0 gives
// it. The texts that each case wants are the texts of the scene that
// vega.js draws for the same spec.
func TestItemOrder(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		spec string
		role string
		want []string
	}{
		{"properties in order", itemOrderSpec, "a", []string{
			"5|6|undefined|undefined|3|6|undefined|1",
			"5|7|undefined|undefined|3|6|undefined|2",
			"5|8|undefined|undefined|3|6|5|3",
		}},
		{"marks of the root group", itemOrderSpec, "b", []string{
			"3|rect|10|50|30|40|40|a|root|frame|undefined|group|frame|root|1|undefined|0",
		}},
		{"marks of a group item", itemOrderSpec, "gt", []string{
			"1|0|8|3|1|1|4",
			"1|0|8|3|1|2|4",
			"1|0|8|3|1|3|4",
		}},
		{"mark of a guide", itemAxisSpec, "axis-label", []string{
			"text|axis-label|lab|2|axis|root|0|0|undefined",
			"text|axis-label|lab|2|axis|root|3|200|undefined",
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			c, _, err := Parse([]byte(test.spec), WithStrict(true))
			if err != nil {
				t.Fatal(err)
			}
			s, _, err := c.Render(t.Context(), WithLocation(time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, it := range roleItems(s, test.role) {
				got = append(got, strings.Join(it.Text, ""))
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("the texts =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}

// TestPathScale checks that the scaleX and the scaleY of a path mark scale
// its path before it moves to its x and y.
func TestPathScale(t *testing.T) {
	t.Parallel()
	s := specScene(t, `{"$schema": "https://vega.github.io/schema/vega/v5.json",
		"width": 100, "height": 100,
		"marks": [{"type": "path", "role": "p", "encode": {"update": {
			"x": {"value": 5}, "path": {"value": "M0,0 l10,0 0,10z"},
			"scaleX": {"value": 2}, "scaleY": {"signal": "3"}}}}]}`)
	p := roleItems(s, "p")
	if len(p) != 1 || p[0].Path != "M0,0L20,0L20,30Z" || p[0].X != 5 {
		t.Errorf("the path = %+v, want M0,0L20,0L20,30Z at 5", p)
	}
}
