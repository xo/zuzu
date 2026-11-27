package zuzu

import (
	"bytes"
	"encoding/json/jsontext"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/vega"
)

// A tooltip of Vega shows its value as the default handler of vega.js
// shows it without a plugin, which zuzu writes as the text of the tooltip
// of the item (D61). A value of a tooltip can also be the item itself, as
// "tooltip": {"signal": "item"} gives. vega.js then shows the fields of
// the item when the pointer is on it, after the run, in the order that it
// set them. zuzu writes that text once the item is whole.

// encodeTooltip prepares the tooltip of an item for a value that an
// encoding gives it. A tooltip that is the item itself waits for
// itemTooltip. A tooltip that is another part of the scene, such as
// item.mark, or the item of a guide or of the root group, whose fields
// vega.js sets in its own way, goes in the report. encodeTooltip returns
// false when the value goes in the report.
func (s *scope) encodeTooltip(st *itemState, v any, path string) (bool, error) {
	obj, ok := v.(expr.Object)
	if !ok {
		st.tipItem = false
		return true, nil
	}
	self, _ := obj.(*itemState)
	ms := st.mark
	if self == st && ms != nil && !ms.guide && !ms.root {
		st.tipItem = true
		return true, nil
	}
	st.tipItem = false
	st.it.Tooltip = ""
	what := "a part of the scene other than its own item"
	switch {
	case self != st:
	case ms != nil && ms.guide:
		what = "an item of a guide"
	case ms != nil && ms.root:
		what = "the item of the root group"
	}
	return false, s.issueOncef(path, "a tooltip that shows %s", what)
}

// itemTooltip writes the text of a tooltip that is the item itself, as
// vega.js shows it (D61). The item has a datum, a mark and bounds, which
// show as {…}, and exit, which is false. A group item then has its marks,
// one {…} for each axis, legend, title and mark in it. The properties that
// the config and the encodings set follow, in the order that vega.js sets
// them, without the properties of the look of zuzu whose names start with
// zuzu. Last, a group item that holds a mark has its context.
func itemTooltip(st *itemState, group bool, children int, loc *time.Location) string {
	lines := []string{"datum: {…}", "exit: false", "mark: {…}", "bounds: {…}"}
	if group {
		lines = append(lines, "items: ["+strings.Join(slices.Repeat([]string{"{…}"}, children), ", ")+"]")
	}
	for _, k := range st.keys {
		if strings.HasPrefix(k, "zuzu.") {
			continue
		}
		lines = append(lines, k+": "+tipField(st.values[k], loc))
	}
	if group && children > 0 {
		lines = append(lines, "context: {…}")
	}
	return strings.Join(lines, "\n")
}

// childMarks returns the count of the marks that vega.js puts in a group
// item of a mark: one for each axis, each legend, the title and each mark.
func childMarks(m *vega.Mark) int {
	n := len(m.Axes) + len(m.Legends) + len(m.Marks)
	if m.Title != nil {
		n++
	}
	return n
}

// tooltipKeys returns the order of the keys of an object v that a rule
// gives a tooltip, where zuzu knows it: an object literal of an
// expression, as Program.Keys finds it, or an object of JSON in the spec.
// It returns nil for any other value or rule.
func (rt *runtime) tooltipKeys(r *vega.Rule, v any) []string {
	o, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	switch {
	case r.Signal != "":
		p, err := rt.compile(r.Signal)
		if err != nil {
			return nil
		}
		return p.Keys(o)
	case len(r.Value) > 0:
		return jsonKeys(r.Value)
	}
	return nil
}

// jsonKeys returns the keys of an object of JSON, in the order of
// Object.keys of JavaScript, or nil for a value that is not an object.
func jsonKeys(raw []byte) []string {
	d := jsontext.NewDecoder(bytes.NewReader(raw))
	if t, err := d.ReadToken(); err != nil || t.Kind() != '{' {
		return nil
	}
	var keys []string
	for d.PeekKind() == '"' {
		t, err := d.ReadToken()
		if err != nil {
			return nil
		}
		keys = append(keys, t.String())
		if err := d.SkipValue(); err != nil {
			return nil
		}
	}
	return expr.KeyOrder(keys)
}

// tooltipText writes the value of a tooltip as the text that vega.js shows
// without a plugin (D61). null and undefined show nothing. An array shows
// as [a, b]. An object shows one line "key: value" for each of its keys,
// in the order of keys, where the rule of the tooltip gives it. Any other
// value shows as its text. A date shows as Date.toString() of JavaScript
// in the time zone loc, or in its own time zone when loc is nil.
func tooltipText(v any, keys []string, loc *time.Location) string {
	switch x := v.(type) {
	case nil:
		return ""
	case []any:
		return tipArray(x, loc)
	case map[string]any:
		order := objectKeys(x, keys)
		lines := make([]string, len(order))
		for i, k := range order {
			lines[i] = k + ": " + tipField(x[k], loc)
		}
		return strings.Join(lines, "\n")
	}
	if expr.IsUndefined(v) {
		return ""
	}
	return tipString(v, loc)
}

// objectKeys returns the keys of an object in the order of keys, and then
// any other key. zuzu does not know the order of the keys of an object
// that a dataset or a function gives, because a map of Go keeps no order.
// Those keys come in the order of Object.keys of JavaScript for an object
// that sets them sorted: an index of an array first, in the order of its
// number, and then the other keys, sorted.
func objectKeys(x map[string]any, keys []string) []string {
	out := make([]string, 0, len(x))
	seen := make(map[string]bool, len(x))
	for _, k := range keys {
		if _, ok := x[k]; ok && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	var rest []string
	for _, k := range slices.Sorted(maps.Keys(x)) {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	return append(out, expr.KeyOrder(rest)...)
}

// tipField writes the value of a field of an object, after its key: an
// array as [a, b], and any other value as tipValue does.
func tipField(v any, loc *time.Location) string {
	if l, ok := v.([]any); ok {
		return tipArray(l, loc)
	}
	return tipValue(v, loc)
}

// tipArray writes an array as [a, b]. null and undefined in it show
// nothing, as Array.prototype.join of JavaScript writes them.
func tipArray(l []any, loc *time.Location) string {
	parts := make([]string, len(l))
	for i, e := range l {
		if e != nil && !expr.IsUndefined(e) {
			parts[i] = tipValue(e, loc)
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// tipValue writes a value inside an object or an array: an array as […],
// an object as {…}, and any other value as its text.
func tipValue(v any, loc *time.Location) string {
	switch v.(type) {
	case []any:
		return "[…]"
	case map[string]any, expr.Object, *expr.RegExp, *expr.Color:
		return "{…}"
	}
	return tipString(v, loc)
}

// tipString writes a value as String() of JavaScript, with a date in the
// time zone loc.
func tipString(v any, loc *time.Location) string {
	if t, ok := v.(time.Time); ok && loc != nil && !t.IsZero() {
		v = t.In(loc)
	}
	return expr.String(v)
}
