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
	c     *compiler
	v     *view
	mark  string
	props map[string]any
	// paintStyle is the style of the theme that gives the color of a mark
	// with no color of its own.
	paintStyle string
	ch         map[string]*channelDef
	detail     []*channelDef
	tooltip    []*channelDef
	data       *dataNode
	path       string
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
	// geo holds the channels of longitude and latitude, and geoShape a
	// shape of type geojson. proj is the projection that places them, and
	// geoAs the fields of x and y that geopoint makes.
	geo      map[string]*channelDef
	geoShape *channelDef
	proj     *projComp
	geoAs    []string
	// source is the dataset of the view before its transforms.
	source *dataNode
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
	if c.cfg != nil {
		// The corners of the config of the mark are drawn as the corners
		// of the mark.
		for k, x := range c.cfg.marks[u.mark] {
			if _, ok := u.props[k]; !ok && strings.HasPrefix(k, "cornerRadius") {
				u.props[k] = x
			}
		}
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
				if !d.isField() {
					continue
				}
				if name == "detail" {
					u.detail = append(u.detail, d)
				} else {
					// A field of a tooltip joins the aggregate and the bins,
					// as Vega-Lite does, and nothing else.
					u.tooltip = append(u.tooltip, d)
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
	u.readGeo()
	for _, name := range []string{"row", "column", "facet", "xError", "yError"} {
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
	u.data, u.source = src, src
	if u.usesGeo() {
		p, err := c.projection(u)
		if err != nil {
			return err
		}
		u.proj = p
	}
	u.orientation()
	u.stacking(s)
	if err := u.dataflow(s); err != nil {
		return err
	}
	u.fitData()
	for _, d := range u.allChannels() {
		if d.label != "" {
			// The marks and the scale read the text of the range of each
			// bin, in the order of the bins.
			d.out, d.sortBy, d.outEnd, d.bin = d.label, d.out, "", nil
		}
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
	measure := u.measure()
	if u.mark == "arc" || u.ch["theta"] != nil {
		// An arc stacks its angle, or its radius when the angle is
		// discrete.
		measure = "theta"
		if t := u.ch["theta"]; (t == nil || t.typ != quantitative) && u.ch["radius"] != nil && u.ch["radius"].typ == quantitative {
			measure = "radius"
		}
	}
	m := u.ch[measure]
	explicit := m != nil && m.def != nil && !m.def.Stack.IsZero()
	mark := u.mark
	if s.stackAs != "" {
		mark = s.stackAs
	}
	switch mark {
	case "bar", "area", "arc":
	case "rule", "point", "circle", "square", "line", "text", "tick":
		// These stack only when the channel asks.
		if !explicit {
			return
		}
	default:
		return
	}
	if m == nil || m.typ != quantitative || m.bin != nil || m.binned || !m.isField() {
		return
	}
	if measure != "theta" && measure != "radius" {
		if _, ok := u.ch[measure+"2"]; ok {
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
	var by []*channelDef
	for _, name := range []string{"color", "fill", "stroke", "opacity", "size", "shape", "strokeDash"} {
		if d := u.ch[name]; d != nil && d.isField() && d.aggregate == "" {
			by = append(by, d)
		}
	}
	by = append(by, u.detail...)
	// A field that places the bars, such as the field of an offset, does
	// not stack them.
	placed := map[string]bool{}
	for _, name := range []string{u.dimension(), u.dimension() + "Offset"} {
		if d := u.ch[name]; d != nil && d.isField() {
			placed[d.out] = true
		}
	}
	by = slices.DeleteFunc(by, func(d *channelDef) bool { return placed[d.out] })
	// An aggregate gives one row for each value of the dimension, which
	// has nothing to stack on, unless a channel such as color splits it.
	if len(by) == 0 && m.aggregate != "" && !explicit {
		return
	}
	u.stack, u.offset, u.stackBy = measure, offset, by
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
		case "min", "max":
			// An op that compares values reads them as numbers, as
			// Vega-Lite parses them.
			if d.field != "" {
				numbers = append(numbers, d.field)
			}
		}
		if d.typ == temporal && d.field != "" && d.aggregate == "" {
			dates = append(dates, d.field)
			if d.units == nil || d.binnedUnit {
				u.dates[d.out] = true
			}
		}
		if d.units != nil && d.field != "" {
			dates = append(dates, d.field)
		}
	}
	dates = slices.DeleteFunc(dates, func(f string) bool { return u.c.produced[f] })
	numbers = slices.DeleteFunc(numbers, func(f string) bool { return u.c.produced[f] })
	for _, t := range s.Transform {
		// A time unit transform reads its field as dates.
		if f := t.fields(); t.Has("timeUnit") {
			if name, ok := f["field"].(string); ok && !slices.Contains(dates, name) {
				dates = append(dates, name)
			}
		}
	}
	if f := u.pathSortField(); f != "" && u.data != nil && u.data.values {
		// A path through inline values reads the field it is sorted by as
		// numbers.
		numbers = append(numbers, f)
	}
	needs := len(s.Transform) > 0
	for _, d := range u.allChannels() {
		if d.nested != nil {
			needs = true
		}
		if d.bin != nil || d.units != nil || d.aggregate != "" {
			needs = true
		}
		if d.def != nil {
			if _, ok := valueOf(d.def.Sort).([]any); ok {
				needs = true
			}
			if len(d.def.Impute) > 0 {
				needs = true
			}
		}
	}
	if u.stack != "" || len(u.invalidFields()) > 0 || len(u.geo) > 0 || u.geoShape != nil {
		needs = true
	}
	if u.data != nil && u.data.values && (len(dates)+len(numbers) > 0 || len(u.data.explicit) > 0) {
		// Inline values parse their dates with formulas, in a dataset that
		// derives from them.
		needs = true
	}
	if !needs {
		c.parseFields(u.data, dates, "date")
		return nil
	}
	n := c.derive(u.data)
	// A field inside an object flattens before the transforms, or after
	// them when a transform makes the object.
	made := producedFields(s.Transform)
	flat := map[string]bool{}
	flatten := func(after bool) {
		for _, d := range u.allChannels() {
			if d.nested != nil && !flat[d.field] && made[d.nested[0]] == after {
				flat[d.field] = true
				e := flattenExpr(d.nested)
				if !after {
					c.flattens = append(c.flattens, e)
				}
				c.add(n, transform("formula", map[string]any{"expr": e, "as": d.field}))
			}
		}
	}
	flatten(false)
	c.parseFields(n, dates, "date")
	c.parseFields(n, numbers, "number")
	if err := c.transforms(n, s.Transform, u.path+"/transform"); err != nil {
		return err
	}
	flatten(true)
	if err := u.addGeo(n); err != nil {
		return err
	}
	unitsDone := false
	for _, d := range u.allChannels() {
		if d.def != nil && len(d.units) > 0 {
			if _, ok := valueOf(d.def.Sort).([]any); ok {
				// A list of values of a time unit sorts the rows after the
				// time units.
				u.addTimeUnits(n)
				unitsDone = true
				break
			}
		}
	}
	for _, d := range u.allChannels() {
		if d.bin != nil {
			c.addBin(n, d.field, d.bin, []any{d.out, d.outEnd})
			if (d.channel == "x" || d.channel == "y") && (d.typ == ordinal || d.typ == nominal) {
				// A discrete channel draws a bin as the text of its range.
				d.label = d.out + "_range"
				start, end := fieldRef(d.out), fieldRef(d.outEnd)
				c.add(n, transform("formula", map[string]any{
					"expr": "!isValid(" + start + ") || !isFinite(+" + start + ") ? \"null\" : format(" + start + ", \"\") + \" – \" + format(" + end + ", \"\")",
					"as":   d.label,
				}))
			}
		}
	}
	if u.addListSort(n) {
		u.raw = n
		n.open = false
		n = c.derive(n)
	} else if u.sortsByField() {
		// The domain sorts over the rows before the aggregate, so the
		// aggregate goes in a dataset of its own.
		u.raw = n
		n.open = false
		n = c.derive(n)
	}
	u.addSortIndex(n)
	if !unitsDone {
		u.addTimeUnits(n)
	}
	for _, d := range u.allChannels() {
		if u.shifted(d) {
			// A bar at the start of its interval spans from the middle of
			// the interval before to the middle of its own.
			unit := d.units[len(d.units)-1]
			start, end := "datum['"+literal(d.out)+"']", "datum['"+literal(d.outEnd)+"']"
			c.add(n, transform("formula", map[string]any{
				"expr": "0.5 * timeOffset('" + unit + "', " + start + ", -1) + 0.5 * " + start, "as": d.out + "_offsetted_rect_start",
			}))
			c.add(n, transform("formula", map[string]any{
				"expr": "0.5 * " + start + " + 0.5 * " + end, "as": d.out + "_offsetted_rect_end",
			}))
		}
	}
	if u.aggregated() {
		u.addAggregate(n)
	}
	if err := u.addEncodingImpute(n); err != nil {
		return err
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
		ref := fieldRef(d.field)
		value := func(v any) string { return string(mustJSON(v)) }
		index := name + "_" + d.field + "_sort_index"
		if len(d.units) == 1 {
			// The values are values of the time unit, which compare as
			// dates.
			ref = timeFloorExpr(d.units, d.utc, ref)
			value = func(v any) string { return dateTimeExpr(map[string]any{d.units[0]: v, "utc": d.utc}) }
			index = name + "_" + d.out + "_sort_index"
		}
		for i, v := range list {
			fmt.Fprintf(&b, "%s===%s ? %d : ", ref, value(v), i)
		}
		fmt.Fprintf(&b, "%d", len(list))
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
	pos := d.channel == "x" || d.channel == "y"
	switch {
	case pos && (u.mark == "bar" || u.mark == "rect"):
		return true
	case pos && d.def != nil && !d.def.BandPosition.IsZero():
		// A band position places the mark within the interval. At 0 the
		// mark sits at the start, which needs no end.
		b, ok := valueOf(d.def.BandPosition).(float64)
		return !ok || b != 0
	case u.c.cfg != nil && u.c.cfg.marks[u.mark]["timeUnitBandPosition"] != nil:
		return true
	}
	return false
}

// shifted reports whether a bar over a time unit starts at the start of
// its interval, with a band position of 0, so that it is centered there.
func (u *unit) shifted(d *channelDef) bool {
	if (u.mark != "bar" && u.mark != "rect") || (d.channel != "x" && d.channel != "y") || d.units == nil || d.typ != temporal {
		return false
	}
	var p any
	if d.def != nil && !d.def.BandPosition.IsZero() {
		p = valueOf(d.def.BandPosition)
	} else if u.c.cfg != nil {
		p = u.c.cfg.marks[u.mark]["timeUnitBandPosition"]
	}
	return p == 0.0
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
			if x == "descending" || u.ch[strings.TrimPrefix(x, "-")] != nil {
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
	out = append(out, u.tooltip...)
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
	if o := u.ch["order"]; o != nil && o.isField() {
		// The order channel orders the stack.
		return
	}
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
			"expr": "indexof(" + string(mustJSON(dom)) + ", datum['" + literal(d.field) + "'])",
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
			if d.label != "" {
				group(d.label)
			}
		case d.units != nil && d.typ == temporal && u.spans(d):
			group(d.out)
			group(d.outEnd)
			if u.shifted(d) {
				group(d.out + "_offsetted_rect_start")
				group(d.out + "_offsetted_rect_end")
			}
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

// addEncodingImpute adds the impute of a channel of the measure, keyed by
// the dimension and grouped by the series.
func (u *unit) addEncodingImpute(n *dataNode) error {
	for _, ch := range []string{"x", "y"} {
		d := u.ch[ch]
		if d == nil || d.def == nil || len(d.def.Impute) == 0 {
			continue
		}
		var spec map[string]any
		if err := json.Unmarshal(d.def.Impute, &spec); err != nil {
			return err
		}
		other := "x"
		if ch == "x" {
			other = "y"
		}
		key := u.ch[other]
		if key == nil || !key.isField() {
			return u.c.issue(u.path+"/encoding/"+ch+"/impute", "an impute with no field of the other position")
		}
		var groupby []any
		for _, f := range u.seriesFields() {
			groupby = append(groupby, f)
		}
		u.c.addImpute(n, d.out, key.out, groupby, spec)
	}
	return nil
}

// addTimeUnits adds the time units of the channels.
func (u *unit) addTimeUnits(n *dataNode) {
	for _, d := range u.allChannels() {
		if d.binnedUnit {
			if !u.spans(d) && (d.channel == "x" || d.channel == "y") {
				// A mark at the start of the interval needs no end.
				continue
			}
			unit := d.units[len(d.units)-1]
			fn := "timeOffset"
			if d.utc {
				fn = "utcOffset"
			}
			u.c.add(n, transform("formula", map[string]any{
				"expr": fn + "('" + unit + "', datum['" + literal(d.field) + "'], 1)", "as": d.outEnd,
			}))
			continue
		}
		if d.units != nil && d.aggregate == "" {
			params := map[string]any{"field": d.field, "units": d.units, "as": []any{d.out, d.outEnd}}
			if d.utc {
				params["timezone"] = "utc"
			}
			if d.unitStep != 0 {
				params["step"] = d.unitStep
			}
			u.c.add(n, transform("timeunit", params))
		}
	}
}

// isPath reports whether a mark draws a path through its points.
func isPath(mark string) bool {
	return mark == "area" || mark == "line" || mark == "trail"
}

// addStack adds the stack of the measure, grouped by the dimension. A
// stacked area first fills each series with zero where it has no value, so
// that the layers meet.
func (u *unit) addStack(n *dataNode) {
	m := u.ch[u.stack]
	if dim := u.ch[u.dimension()]; isPath(u.mark) && dim != nil && dim.isField() {
		var by []any
		for _, d := range u.stackBy {
			by = append(by, d.out)
		}
		u.c.add(n, transform("impute", map[string]any{
			"field": m.out, "groupby": by, "key": dim.out, "method": "value", "value": 0,
		}))
	}
	var groupby []any
	if u.stack == "radius" {
		// The rings of each angle stack apart.
		if t := u.ch["theta"]; t != nil && t.isField() {
			groupby = append(groupby, t.out)
		}
	} else if u.mark != "arc" && u.stack != "theta" {
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
		// A vertical stack puts the first value at the top, and a
		// horizontal stack or a pie puts it first.
		order := "descending"
		if u.orient == "horizontal" || u.mark == "arc" || u.stack == "theta" {
			order = "ascending"
		}
		sortFields = append(sortFields, field)
		orders = append(orders, order)
	}
	if o := u.ch["order"]; o != nil {
		order := "ascending"
		if o.def != nil && !o.def.Sort.IsZero() {
			order = str(o.def.Sort)
		}
		if o.isField() {
			sortFields, orders = []any{o.out}, []any{order}
		} else {
			for i := range orders {
				orders[i] = order
			}
		}
	}
	u.c.add(n, transform("stack", map[string]any{
		"groupby": groupby, "field": m.out, "sort": map[string]any{"field": sortFields, "order": orders},
		"as": []any{m.stackName("start"), m.stackName("end")}, "offset": u.offset,
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
	if _, ok := u.props["invalid"]; !ok && u.c.cfg != nil && u.c.cfg.keepInvalid {
		return nil
	}
	var out []string
	for _, name := range []string{"x", "y", "theta", "radius", "color", "fill", "stroke", "opacity", "fillOpacity", "strokeOpacity", "strokeWidth", "size", "angle"} {
		d := u.ch[name]
		if d == nil || !d.isField() || !d.continuous() || d.aggregate == "count" {
			continue
		}
		if !isPositional(name) && (d.typ != quantitative || d.def != nil && valueOf(d.def.Scale) == nil && !d.def.Scale.IsZero()) {
			continue
		}
		if d.typ == temporal && (d.units != nil && !d.binnedUnit || d.aggregate != "") {
			// A binned time unit reads the dates as they are, so the filter
			// keeps the valid ones.
			continue
		}
		if !slices.Contains(out, d.out) {
			out = append(out, d.out)
		}
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
	} else if group := u.stackGroup(m); group != nil {
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
	by := u.seriesFields()
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

// seriesFields returns the fields that split the marks into series: the
// fields of the channels that are not aggregates, such as color, and of
// detail. The size of a trail changes along it, and does not split it.
func (u *unit) seriesFields() []string {
	var by []string
	for _, name := range []string{"color", "fill", "stroke", "opacity", "fillOpacity", "strokeOpacity", "strokeWidth", "strokeDash", "size"} {
		if name == "size" && u.mark == "trail" {
			continue
		}
		if d := u.ch[name]; d != nil && d.isField() && d.aggregate == "" {
			by = append(by, d.out)
		}
	}
	for _, d := range u.detail {
		by = append(by, d.out)
	}
	return by
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

// stackGroup puts a stacked bar with round corners in a group for each
// stack, which clips the bars, so that the corners round the stack and not
// each bar. The group draws the corners and the stroke of the mark.
func (u *unit) stackGroup(m *vega.Mark) *vega.Mark {
	if u.mark != "bar" || u.stack == "" {
		return nil
	}
	var corners []string
	for k, x := range u.props {
		if strings.HasPrefix(k, "cornerRadius") && x != 0.0 {
			corners = append(corners, k)
		}
	}
	if len(corners) == 0 {
		return nil
	}
	slices.Sort(corners)
	measure := u.ch[u.stack]
	dim := u.ch[u.dimension()]
	if measure == nil || dim == nil || !dim.isField() {
		return nil
	}
	// The axis of the stack, and the axis of the bars.
	along, across, size := "y", "x", "width"
	if u.stack == "x" {
		along, across, size = "x", "y", "height"
	}
	start, end := measure.stackName("start"), measure.stackName("end")
	groupby := []string{dim.out}
	if dim.units != nil || dim.bin != nil {
		groupby = append(groupby, dim.outEnd)
	}
	fields := []any{start, start, end, end}
	ops := []any{"min", "max", "min", "max"}
	var scaled []string
	for i := range fields {
		scaled = append(scaled, "scale('"+u.v.sn(along)+"',datum[\""+fmt.Sprint(ops[i])+"_"+fmt.Sprint(fields[i])+"\"])")
	}
	list := strings.Join(scaled, ",")
	scale := u.v.sn(across)
	outer := vega.Props{
		across:      prod(map[string]any{"scale": scale, "field": dim.out}),
		size:        prod(map[string]any{"signal": "max(0.25, bandwidth('" + scale + "'))"}),
		along:       prod(map[string]any{"signal": "min(" + list + ")"}),
		along + "2": prod(map[string]any{"signal": "max(" + list + ")"}),
		"clip":      prod(map[string]any{"value": true}),
	}
	update := m.Encode["update"]
	for _, k := range corners {
		if k == "cornerRadiusEnd" {
			continue
		}
		outer[k] = prod(propRef(u.props[k]))
		if _, ok := u.c.cfg.marks[u.mark][k]; !ok || u.props[k] != u.c.cfg.marks[u.mark][k] {
			delete(update, k)
		}
	}
	if r, ok := u.props["cornerRadiusEnd"]; ok {
		// The end of a stack is the end that is away from zero, which a
		// test of the scaled bounds of the stack finds.
		sa := u.v.sn(along)
		ref := func(f string) string { return "scale(\"" + sa + "\", datum[\"" + f + "\"])" }
		top := ref("max_"+end) + " < " + ref("max_"+start)
		bottom := ref("min_"+end) + " > " + ref("min_"+start)
		names := [2][2]string{{"cornerRadiusTopLeft", "cornerRadiusTopRight"}, {"cornerRadiusBottomLeft", "cornerRadiusBottomRight"}}
		tests := [2]string{top, bottom}
		if along == "x" {
			ref := func(f string) string { return "scale(\"" + sa + "\", datum[\"" + f + "\"])" }
			tests = [2]string{ref("max_"+end) + " > " + ref("max_"+start), ref("min_"+end) + " < " + ref("min_"+start)}
			names = [2][2]string{{"cornerRadiusTopRight", "cornerRadiusBottomRight"}, {"cornerRadiusTopLeft", "cornerRadiusBottomLeft"}}
		}
		for i, pair := range names {
			for _, k := range pair {
				outer[k] = prod(map[string]any{"__rules": []any{map[string]any{"test": tests[i], "value": r}, map[string]any{"value": 0}}})
			}
		}
	}
	if st, ok := u.props["stroke"]; ok {
		outer["stroke"] = prod(map[string]any{"value": st})
		if w, ok := u.props["strokeWidth"]; ok {
			outer["strokeWidth"] = prod(map[string]any{"value": w})
		} else if w, ok := u.c.cfg.marks[u.mark]["strokeWidth"]; ok {
			outer["strokeWidth"] = prod(map[string]any{"value": w})
		}
		outer["strokeForeground"] = prod(map[string]any{"value": true})
		outer["strokeOffset"] = prod(map[string]any{"value": 0})
	}
	inner := *m
	inner.From = &vega.From{Data: "stack_group_main"}
	delete(update, across)
	delete(update, across+"2")
	delete(update, size)
	update[size] = prod(map[string]any{"field": map[string]any{"group": size}})
	return &vega.Mark{
		Type: "group",
		From: &vega.From{Facet: &vega.Facet{
			Name: "stack_group_main", Data: u.c.name(u.data), GroupBy: vega.List(groupby...),
			Aggregate: mustJSON(map[string]any{"fields": fields, "ops": ops}),
		}},
		Encode: vega.Encode{"update": outer},
		Marks: []vega.Mark{{
			Type: "group",
			Encode: vega.Encode{"update": vega.Props{
				along: prod(map[string]any{"field": map[string]any{"group": along}, "mult": -1}),
				size:  prod(map[string]any{"field": map[string]any{"group": size}}),
			}},
			Marks: []vega.Mark{inner},
		}},
	}
}

// propRef returns the value reference of a property of a mark: a signal
// for an expression, {"expr": "..."}, or else the value.
func propRef(v any) map[string]any {
	if m, ok := v.(map[string]any); ok && len(m) == 1 {
		if e, ok := m["expr"].(string); ok {
			return map[string]any{"signal": e}
		}
	}
	return map[string]any{"value": v}
}

// pathSortField returns the field that sorts the points of a line, an area
// or a trail: the quantitative field of x, or of y for a horizontal area,
// when no order channel sorts them.
func (u *unit) pathSortField() string {
	if !isPath(u.mark) {
		return ""
	}
	if o := u.ch["order"]; o != nil {
		return ""
	}
	ch := "x"
	if u.mark == "area" && u.orient == "horizontal" {
		ch = "y"
	}
	d := u.ch[ch]
	if d == nil || !d.isField() || d.typ != quantitative || d.aggregate != "" || d.bin != nil {
		return ""
	}
	return d.field
}

// producedFields returns the fields that transforms make: every name under
// a key "as", in the transform or in its lists of operations.
func producedFields(ts []Transform) map[string]bool {
	out := map[string]bool{}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				if k != "as" {
					walk(e)
					continue
				}
				switch a := e.(type) {
				case string:
					out[a] = true
				case []any:
					for _, s := range a {
						if name, ok := s.(string); ok {
							out[name] = true
						}
					}
				}
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	for _, t := range ts {
		walk(t.fields())
	}
	return out
}
