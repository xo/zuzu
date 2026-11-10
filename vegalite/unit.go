package vegalite

import (
	"encoding/json/v2"
	"fmt"
	"slices"
	"strings"

	"github.com/xo/zuzu/vega"
)

// channelOrder is the order in which the compiler reads the channels.
var channelOrder = []string{
	"x", "y", "x2", "y2", "xOffset", "yOffset", "theta", "theta2", "radius", "radius2",
	"longitude", "latitude", "longitude2", "latitude2",
	"color", "fill", "stroke", "opacity", "fillOpacity", "strokeOpacity", "strokeWidth",
	"strokeDash", "size", "shape", "angle", "text", "tooltip", "href", "url",
	"description", "detail", "key", "order", "xError", "xError2", "yError", "yError2",
	"row", "column", "facet",
}

// unit holds a single view while the compiler reads it.
type unit struct {
	c      *compiler
	v      *view
	mark   string
	props  map[string]any
	ch     map[string]*channelDef
	detail []*channelDef
	data   *dataNode
	path   string
	// stack is the channel that stacks, such as y, and stackBy the fields
	// that it stacks by.
	stack   string
	offset  string
	stackBy []*channelDef
	// sortIndex names, for each channel whose scale has a domain of its
	// own, the field that holds the index of a value in that domain.
	sortIndex map[string]string
	// dates holds the fields of the view that are dates.
	dates map[string]bool
	// listSort names, for each channel that sorts by a list of values, the
	// field that holds the index of each value in the list.
	listSort map[string]string
	// raw is the dataset before the aggregate, which a scale sorted by an
	// aggregate of another field reads.
	raw *dataNode
	// orient is vertical when the bars run up, and horizontal when they run
	// across.
	orient string
}

// unit compiles a single view.
func (c *compiler) unit(s *Spec, v *view, src *dataNode, path string, repeat map[string]string) error {
	u := &unit{c: c, v: v, mark: s.Mark.Type, props: map[string]any{}, ch: map[string]*channelDef{}, path: path}
	for k, raw := range s.Mark.Props {
		var x any
		if err := json.Unmarshal(raw, &x); err == nil {
			u.props[k] = x
		}
	}
	if u.mark == "geoshape" {
		return c.issue(path+"/mark", "the mark %s, which comes in phase 5", u.mark)
	}
	for _, name := range channelOrder {
		ch, ok := s.Encoding[name]
		if !ok || len(ch.Defs) == 0 {
			continue
		}
		if name == "detail" || name == "tooltip" {
			for i := range ch.Defs {
				d, err := readChannel(name, &ch.Defs[i], repeat)
				if err != nil {
					return err
				}
				if name == "detail" && d.isField() {
					u.detail = append(u.detail, d)
				}
			}
			continue
		}
		d, err := readChannel(name, ch.Def(), repeat)
		if err != nil {
			return err
		}
		u.ch[name] = d
	}
	for name := range s.Encoding {
		if !slices.Contains(channelOrder, name) {
			if err := c.issue(path+"/encoding/"+name, "the channel %s", name); err != nil {
				return err
			}
		}
	}
	for _, name := range []string{"row", "column", "facet", "longitude", "latitude", "xError", "yError"} {
		if _, ok := u.ch[name]; ok {
			if err := c.issue(path+"/encoding/"+name, "the channel %s", name); err != nil {
				return err
			}
			delete(u.ch, name)
		}
	}
	if src == nil {
		return c.issue(path, "a view with no data")
	}
	u.data = src
	u.orientation()
	u.stacking(s)
	if err := u.dataflow(s); err != nil {
		return err
	}
	return u.marks()
}

// orientation works out which way the marks run: vertical when x is
// discrete or absent and y is continuous, and horizontal the other way.
func (u *unit) orientation() {
	x, y := u.ch["x"], u.ch["y"]
	u.orient = "vertical"
	switch {
	case x != nil && y != nil:
		xc := x.continuous() && x.bin == nil && x.units == nil
		yc := y.continuous() && y.bin == nil && y.units == nil
		if xc && !yc {
			u.orient = "horizontal"
		}
		if xc && yc && x.aggregate != "" && y.aggregate == "" {
			u.orient = "horizontal"
		}
	case x != nil && y == nil:
		if x.continuous() {
			u.orient = "horizontal"
		}
	}
	if o, ok := u.props["orient"].(string); ok {
		u.orient = o
	}
}

// measure returns the channel of the measure: y when the marks run up, and
// x when they run across.
func (u *unit) measure() string {
	if u.mark == "arc" {
		return "theta"
	}
	if u.orient == "horizontal" {
		return "x"
	}
	return "y"
}

// dimension returns the channel of the dimension, across the measure.
func (u *unit) dimension() string {
	if u.orient == "horizontal" {
		return "y"
	}
	return "x"
}

// stacking works out whether the marks stack, as a bar, an area or an arc
// does by default when its measure is quantitative.
func (u *unit) stacking(s *Spec) {
	switch u.mark {
	case "bar", "area", "arc":
	default:
		return
	}
	m := u.ch[u.measure()]
	if m == nil || m.typ != quantitative || m.bin != nil || m.binned || !m.isField() {
		return
	}
	if u.mark != "arc" {
		if _, ok := u.ch[u.measure()+"2"]; ok {
			return
		}
	}
	offset := "zero"
	if m.def != nil && !m.def.Stack.IsZero() {
		switch x := valueOf(m.def.Stack).(type) {
		case nil:
			return
		case bool:
			if !x {
				return
			}
		case string:
			offset = x
		}
	}
	if d := u.ch[u.dimension()]; d != nil && d.continuous() && d.bin == nil && d.units == nil && u.mark == "bar" {
		if m.def == nil || m.def.Stack.IsZero() {
			return
		}
	}
	var by []*channelDef
	for _, name := range []string{"color", "fill", "stroke", "opacity", "size", "shape", "strokeDash"} {
		if d := u.ch[name]; d != nil && d.isField() && d.aggregate == "" {
			by = append(by, d)
		}
	}
	by = append(by, u.detail...)
	// An aggregate gives one row for each value of the dimension, which
	// has nothing to stack on, unless a channel such as color splits it.
	explicit := m.def != nil && !m.def.Stack.IsZero()
	if len(by) == 0 && m.aggregate != "" && !explicit {
		return
	}
	u.stack, u.offset, u.stackBy = u.measure(), offset, by
	_ = s
}

// dataflow adds the transforms of the view and of its encodings: the bins,
// the time units, the aggregate, the stack and the filter of invalid
// values.
func (u *unit) dataflow(s *Spec) error {
	c := u.c
	var dates []string
	var numbers []string
	u.dates = map[string]bool{}
	for _, d := range u.allChannels() {
		switch d.aggregate {
		case "", "count", "sum", "valid", "missing", "distinct", "values":
		default:
			// An op that compares values reads them as numbers, as
			// Vega-Lite parses them.
			if d.field != "" {
				numbers = append(numbers, d.field)
			}
		}
		if d.typ == temporal && d.field != "" && d.aggregate == "" {
			dates = append(dates, d.field)
			if d.units == nil {
				u.dates[d.out] = true
			}
		}
		if d.units != nil && d.field != "" {
			dates = append(dates, d.field)
		}
	}
	needs := len(s.Transform) > 0
	for _, d := range u.allChannels() {
		if d.bin != nil || d.units != nil || d.aggregate != "" {
			needs = true
		}
	}
	if u.stack != "" || len(u.invalidFields()) > 0 {
		needs = true
	}
	if !needs {
		c.parseFields(u.data, dates, "date")
		return nil
	}
	n := c.derive(u.data)
	c.parseFields(n, dates, "date")
	c.parseFields(n, numbers, "number")
	if err := c.transforms(n, s.Transform, u.path+"/transform"); err != nil {
		return err
	}
	if u.addListSort(n) {
		u.raw = n
		n.open = false
		n = c.derive(n)
	} else if u.aggregated() && u.sortsByField() {
		// The domain sorts over the rows before the aggregate, so the
		// aggregate goes in a dataset of its own.
		u.raw = n
		n.open = false
		n = c.derive(n)
	}
	u.addSortIndex(n)
	for _, d := range u.allChannels() {
		if d.bin != nil {
			c.addBin(n, d.field, d.bin, []any{d.out, d.outEnd})
		}
	}
	for _, d := range u.allChannels() {
		if d.units != nil && d.aggregate == "" {
			params := map[string]any{"field": d.field, "units": d.units, "as": []any{d.out, d.outEnd}}
			if d.utc {
				params["timezone"] = "utc"
			}
			c.add(n, transform("timeunit", params))
		}
	}
	if u.aggregated() {
		u.addAggregate(n)
	}
	if u.stack != "" {
		u.addStack(n)
	}
	if f := u.invalidFields(); len(f) > 0 {
		var parts []string
		for _, name := range f {
			test := "isValid(" + fieldRef(name) + ") && isFinite(+" + fieldRef(name) + ")"
			if u.dates[name] {
				test = "(isDate(" + fieldRef(name) + ") || (" + test + "))"
			}
			parts = append(parts, test)
		}
		c.add(n, transform("filter", map[string]any{"expr": strings.Join(parts, " && ")}))
	}
	u.data = n
	return nil
}

// addListSort adds, for a channel that sorts by a list of values, a field
// with the index of each value in the list, which the domain sorts by. It
// reports whether it added one.
func (u *unit) addListSort(n *dataNode) bool {
	added := false
	for _, name := range channelOrder {
		d := u.ch[name]
		if d == nil || d.def == nil || d.def.Sort.IsZero() {
			continue
		}
		list, ok := valueOf(d.def.Sort).([]any)
		if !ok {
			continue
		}
		var b strings.Builder
		for i, v := range list {
			fmt.Fprintf(&b, "%s===%s ? %d : ", fieldRef(d.field), string(mustJSON(v)), i)
		}
		fmt.Fprintf(&b, "%d", len(list))
		index := name + "_" + d.field + "_sort_index"
		u.c.add(n, transform("formula", map[string]any{"expr": b.String(), "as": index}))
		if u.listSort == nil {
			u.listSort = map[string]string{}
		}
		u.listSort[name] = index
		added = true
	}
	return added
}

// spans reports whether the marks of the view span the interval of a time
// unit on a channel, as a bar does.
func (u *unit) spans(d *channelDef) bool {
	return (u.mark == "bar" || u.mark == "rect") && (d.channel == "x" || d.channel == "y")
}

// sortsByField reports whether a channel sorts its domain by an aggregate
// of a field, as "sort": "-x" or {"op": "sum", "field": "people"} does.
func (u *unit) sortsByField() bool {
	for _, d := range u.ch {
		if d.def == nil || d.def.Sort.IsZero() {
			continue
		}
		switch x := valueOf(d.def.Sort).(type) {
		case string:
			if u.ch[strings.TrimPrefix(x, "-")] != nil {
				return true
			}
		case map[string]any:
			if x["field"] != nil || x["op"] != nil {
				return true
			}
		}
	}
	return false
}

// allChannels returns the channels of the view, in order, with the detail
// channels.
func (u *unit) allChannels() []*channelDef {
	var out []*channelDef
	for _, name := range channelOrder {
		if d, ok := u.ch[name]; ok {
			out = append(out, d)
		}
	}
	out = append(out, u.detail...)
	return append(out, u.v.facetFields...)
}

// aggregated reports whether a channel aggregates.
func (u *unit) aggregated() bool {
	for _, d := range u.allChannels() {
		if d.aggregate != "" {
			return true
		}
	}
	return false
}

// addSortIndex adds, for each channel that stacks and whose scale has a
// domain of its own, a field with the index of the value in that domain,
// which orders the stack as the domain orders the legend.
func (u *unit) addSortIndex(n *dataNode) {
	for _, d := range u.stackBy {
		if d.def == nil || d.def.Scale.IsZero() {
			continue
		}
		props, _ := valueOf(d.def.Scale).(map[string]any)
		dom, ok := props["domain"].([]any)
		if !ok {
			continue
		}
		name := "_" + d.field + "_sort_index"
		if u.sortIndex == nil {
			u.sortIndex = map[string]string{}
		}
		u.sortIndex[d.channel] = name
		u.c.add(n, transform("formula", map[string]any{
			"expr": "indexof(" + string(mustJSON(dom)) + ", datum['" + d.field + "'])",
			"as":   name,
		}))
	}
}

// addAggregate adds the aggregate of the view: the channels that aggregate
// are its measures, and every other field is a group by field.
func (u *unit) addAggregate(n *dataNode) {
	var groupby, ops, fields, as []any
	seen := map[string]bool{}
	group := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			groupby = append(groupby, name)
		}
	}
	for _, d := range u.allChannels() {
		if d.aggregate != "" || !d.isField() {
			continue
		}
		switch {
		case d.bin != nil:
			group(d.out)
			group(d.outEnd)
		case d.units != nil && d.typ == temporal && u.spans(d):
			group(d.out)
			group(d.outEnd)
		default:
			group(d.out)
		}
		group(u.sortIndex[d.channel])
	}
	measured := map[string]bool{}
	for _, d := range u.allChannels() {
		if d.aggregate == "" || measured[d.out] {
			continue
		}
		measured[d.out] = true
		ops = append(ops, d.aggregate)
		switch {
		case d.aggregate == "count":
			fields = append(fields, nil)
			as = append(as, d.out)
		case d.argField != "":
			name := d.aggregate + "_" + d.argField
			if measured[name] {
				ops = ops[:len(ops)-1]
				continue
			}
			measured[name] = true
			fields = append(fields, d.argField)
			as = append(as, name)
		default:
			fields = append(fields, d.field)
			as = append(as, d.out)
		}
	}
	if groupby == nil {
		groupby = []any{}
	}
	u.c.add(n, transform("aggregate", map[string]any{"groupby": groupby, "ops": ops, "fields": fields, "as": as}))
}

// addStack adds the stack of the measure, grouped by the dimension.
func (u *unit) addStack(n *dataNode) {
	m := u.ch[u.stack]
	var groupby []any
	if u.mark != "arc" {
		dim := u.dimension()
		for _, name := range []string{dim, dim + "Offset"} {
			if d := u.ch[name]; d != nil && d.isField() {
				groupby = append(groupby, d.out)
				if d.bin != nil {
					groupby = append(groupby, d.outEnd)
				}
			}
		}
	}
	for _, d := range u.v.facetFields {
		groupby = append(groupby, d.out)
	}
	if groupby == nil {
		groupby = []any{}
	}
	sortFields, orders := []any{}, []any{}
	for _, d := range u.stackBy {
		if slices.Contains(groupby, any(d.out)) {
			// A field that the stack groups by does not order it.
			continue
		}
		field := d.out
		if idx := u.sortIndex[d.channel]; idx != "" {
			field = idx
		}
		sortFields = append(sortFields, field)
		orders = append(orders, "descending")
	}
	if o := u.ch["order"]; o != nil && o.isField() {
		order := "ascending"
		if !o.def.Sort.IsZero() {
			order = str(o.def.Sort)
		}
		sortFields, orders = []any{o.out}, []any{order}
	}
	u.c.add(n, transform("stack", map[string]any{
		"groupby": groupby, "field": m.out, "sort": map[string]any{"field": sortFields, "order": orders},
		"as": []any{m.out + "_start", m.out + "_end"}, "offset": u.offset,
	}))
}

// invalidFields returns the fields of the continuous positional channels,
// whose invalid values the view filters out, as Vega-Lite does. A line,
// an area and a trail break at an invalid value instead.
func (u *unit) invalidFields() []string {
	switch u.mark {
	case "line", "area", "trail":
		return nil
	}
	if v, ok := u.props["invalid"]; ok && v == nil {
		return nil
	}
	var out []string
	for _, name := range []string{"x", "y", "theta", "radius"} {
		d := u.ch[name]
		if d == nil || !d.isField() || !d.continuous() || d.aggregate == "count" {
			continue
		}
		if d.typ == temporal && d.units == nil && d.aggregate == "" {
			out = append(out, d.out)
			continue
		}
		if d.typ == temporal {
			continue
		}
		if name == u.stack {
			out = append(out, d.out)
			continue
		}
		out = append(out, d.out)
	}
	return out
}

// marks adds the marks of the view, with their scales and their guides.
func (u *unit) marks() error {
	m, err := u.markOf()
	if err != nil {
		return err
	}
	if m == nil {
		return nil
	}
	if group := u.pathGroup(m); group != nil {
		u.v.marks = append(u.v.marks, *group)
	} else {
		u.v.marks = append(u.v.marks, *m)
	}
	return nil
}

// pathGroup puts a line, an area or a trail in a group for each series,
// when a channel such as color splits it into series.
func (u *unit) pathGroup(m *vega.Mark) *vega.Mark {
	switch u.mark {
	case "line", "area", "trail":
	default:
		return nil
	}
	var by []string
	for _, name := range []string{"color", "fill", "stroke", "opacity", "size", "strokeDash", "shape"} {
		if d := u.ch[name]; d != nil && d.isField() && d.aggregate == "" && d.typ != quantitative {
			by = append(by, d.out)
		}
	}
	for _, d := range u.detail {
		by = append(by, d.out)
	}
	if len(by) == 0 {
		return nil
	}
	facet := "faceted_path_" + strings.TrimSuffix(u.v.prefix, "_")
	if u.v.prefix == "" {
		facet = "faceted_path_main"
	}
	inner := *m
	inner.From = &vega.From{Data: facet}
	group := vega.Mark{
		Marks: []vega.Mark{inner},
		Encode: vega.Encode{"update": vega.Props{
			"width":  prod(map[string]any{"field": map[string]any{"group": "width"}}),
			"height": prod(map[string]any{"field": map[string]any{"group": "height"}}),
		}},
		Type: "group",
		Name: u.v.prefix + "pathgroup",
		From: &vega.From{Facet: &vega.Facet{Name: facet, Data: u.c.name(u.data), GroupBy: vega.List(by...)}},
	}
	return &group
}

// prod makes a production of one value reference, written as JSON. A
// reference with the key `__rules` is a list of rules.
func prod(v any) vega.Production {
	if m, ok := v.(map[string]any); ok && m["__rules"] != nil {
		v = m["__rules"]
	}
	var p vega.Production
	if err := json.Unmarshal(mustJSON(v), &p); err != nil {
		panic(fmt.Sprintf("vegalite: %v", err))
	}
	return p
}
