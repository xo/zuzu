package vegalite

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/format"
	"github.com/xo/zuzu/vega"
)

// shared is what the layers of a view share: the marks, the scales and the
// guides, and the size.
type shared struct {
	// facetFields are the fields of the rows and the columns of a facet,
	// which the aggregate and the stack of a cell group by.
	facetFields []*channelDef
	// sizeNames are the names of the width and the height, which a cell of
	// a facet calls child_width and child_height.
	sizeNames [2]string
	// scalePrefix names the scales of the positions of a view of a
	// concatenation, such as concat_0_x.
	scalePrefix string
	// prefixAll says that the prefix names every scale, as the name of a
	// facet does, and not only the scales of position.
	prefixAll bool
	// common holds the scales that the views of a concatenation share, such
	// as color, and nil when the view shares none.
	common *shared
	cfg    *config
	marks  []vega.Mark
	scales map[string]*scaleComp
	order  []string
	width  vega.Value
	height vega.Value
}

// view is a view or a layer of a view, which names its marks with a
// prefix, such as layer_0_.
type view struct {
	*shared

	prefix string
	// independent holds the channels whose scales a layer does not share
	// with the other layers, and layerIndex the place of the layer.
	independent map[string]bool
	layerIndex  int
}

// key returns the key of the scale of a channel among the scales of the
// view: the channel, or the channel with the prefix of a layer whose
// scale of the channel is its own.
func (v *view) key(ch string) string {
	if v.independent[ch] {
		return v.prefix + ch
	}
	return ch
}

// newView returns an empty view.
func newView(prefix string, cfg *config) *view {
	return &view{shared: &shared{scales: map[string]*scaleComp{}, cfg: cfg}, prefix: prefix}
}

// layer returns a layer of the view, which shares its scales.
func (v *view) layer(prefix, parent string) *view {
	return &view{shared: v.shared, prefix: parent + prefix}
}

// sn returns the name of the scale of a channel: the channel, with the
// prefix of the view for a position.
func (v *view) sn(ch string) string {
	if v.independent[ch] {
		return v.prefix + ch
	}
	if v.scalePrefix != "" && (isPositional(ch) || v.prefixAll) {
		return v.scalePrefix + ch
	}
	return ch
}

// size takes the width and the height of a spec.
func (v *view) size(s *Spec) {
	if !s.Width.IsZero() {
		v.width = s.Width
	}
	if !s.Height.IsZero() {
		v.height = s.Height
	}
}

// scaleComp is a scale while the compiler collects it from the views that
// use it.
type scaleComp struct {
	name     string
	channel  string
	typ      string
	field    *channelDef
	discrete bool
	// refs are the fields of the datasets of the domain.
	refs []dataField
	// props holds the properties of the scale that the spec gives.
	props map[string]any
	// bins is the signal of the bins of a binned field.
	bins string
	// offset is the range of a scale of an offset: "band" for the band of
	// the scale of its position, "step" for steps of 20 pixels, and
	// "center" around the middle of the view when there is no position.
	offset string
	// normalized says that the scale draws a stack normalized to one.
	normalized bool
	// second says that the scale is the own scale of a layer after the
	// first, whose axis goes on the other side.
	second bool
	mark   string
	// titles are the titles of the fields that the scale draws, for its
	// guide.
	titles []string
	// guide holds the axis or the legend that the spec gives, and noGuide
	// says that it is null.
	guide   map[string]any
	noGuide bool
	sortVal any
	hasSort bool
	// listSorted says that the domain sorts by a field of indexes, which
	// the sort of the channel must not override.
	listSorted bool
	// dimension says that the scale is the dimension of a bar or an area,
	// which does not include zero.
	dimension bool
	// binned says that the field of the scale is binned already.
	binned bool
	// padded says that a bar is placed along the scale, which keeps it
	// clear of the ends of a continuous range.
	padded bool
}

// dataField is a field of a dataset.
type dataField struct {
	data, field string
}

// scaleType returns the type of the scale of a channel, or "".
func (v *view) scaleType(ch string) string {
	if s, ok := v.scales[v.key(ch)]; ok {
		return s.typ
	}
	return ""
}

// addScale registers the use of a scale by a channel, with the fields of
// its domain. It reports false when the spec turns the scale off with
// "scale": null.
func (v *view) addScale(u *unit, name string, d *channelDef, fields []string) bool {
	if v.common != nil && !isPositional(name) {
		return (&view{shared: v.common}).addScale(u, name, d, fields)
	}
	var props map[string]any
	if d.def != nil && !d.def.Scale.IsZero() {
		x := valueOf(d.def.Scale)
		if x == nil {
			return false
		}
		props, _ = x.(map[string]any)
	}
	key := v.key(name)
	s, ok := v.scales[key]
	if !ok {
		s = &scaleComp{name: key, channel: name, props: map[string]any{}, mark: u.mark, field: d}
		s.typ, s.discrete = scaleTypeOf(name, d, u.mark, props)
		s.second = v.independent[name] && v.layerIndex > 0
		v.scales[key] = s
		v.order = append(v.order, key)
	}
	maps.Copy(s.props, props)
	if s.typ == "point" && (name == "x" || name == "y") {
		if o := u.ch[name+"Offset"]; o != nil && o.isField() {
			// A position split by an offset has bands.
			s.typ = "band"
		}
	}
	if s.typ == "point" && props["type"] == nil && (u.mark == "bar" || u.mark == "rect" || u.mark == "tick" || u.mark == "rule") {
		// A discrete scale that a bar draws on has bands, whatever drew
		// on it first.
		s.typ = "band"
	}
	if d.bin != nil {
		s.bins = binPrefix(d.bin) + "_" + varName(d.field) + "_bins"
	}
	data := u.c.name(u.data)
	if idx := u.listSort[name]; idx != "" && u.raw != nil {
		data = u.c.name(u.raw)
		s.sortVal, s.hasSort = map[string]any{"op": "min", "field": idx}, true
		s.listSorted = true
	} else if u.raw != nil && s.discrete && d.def != nil && !d.def.Sort.IsZero() && !d.isAggregate() {
		// A domain sorted by an aggregate of another field reads the rows
		// before the aggregate.
		data = u.c.name(u.raw)
		if by := sortByChannel(u, d); by != nil {
			s.sortVal, s.hasSort = by, true
		}
	}
	for _, f := range fields {
		ref := dataField{data, f}
		dup := false
		for _, r := range s.refs {
			if r == ref {
				dup = true
			}
		}
		if !dup && f != "" {
			s.refs = append(s.refs, ref)
		}
	}
	if d.binned {
		s.binned = true
	}
	if u.mark == "bar" && (name == "x" || name == "y") && d.bin == nil && !d.binned && d.units == nil && name == u.dimension() {
		s.padded = true
	}
	switch u.mark {
	case "bar", "area", "rect", "rule", "tick", "trail", "line":
		if (name == "x" || name == "y") && name == u.dimension() {
			s.dimension = true
		}
	}
	if d.sortBy != "" && !s.hasSort {
		s.sortVal, s.hasSort = map[string]any{"field": d.sortBy, "op": "min"}, true
	}
	if d.def != nil && !d.def.Sort.IsZero() && !s.hasSort && !s.listSorted {
		s.sortVal, s.hasSort = valueOf(d.def.Sort), true
		if by := sortByChannel(u, d); by != nil && s.discrete {
			s.sortVal = by
		}
	}
	if t := titleOf(d); t != "" && !contains(s.titles, t) {
		s.titles = append(s.titles, t)
	}
	if d.def != nil {
		g := d.def.Axis
		if !isPositional(name) {
			g = d.def.Legend
		}
		if !g.IsZero() {
			x := valueOf(g)
			if x == nil {
				s.noGuide = true
			} else if m, ok := x.(map[string]any); ok {
				if s.guide == nil {
					s.guide = map[string]any{}
				}
				maps.Copy(s.guide, m)
			}
		}
		if !d.def.Title.IsZero() {
			x := valueOf(d.def.Title)
			if x == nil {
				s.titles = []string{""}
			} else {
				s.titles = []string{titleString(x)}
			}
		}
	}
	return true
}

// contains reports whether a list holds a string.
func contains(l []string, s string) bool {
	return slices.Contains(l, s)
}

// titleString writes a title that is a string or a list of lines.
func titleString(x any) string {
	if l, ok := x.([]any); ok {
		parts := make([]string, len(l))
		for i, e := range l {
			parts[i] = expr.String(e)
		}
		return strings.Join(parts, " ")
	}
	return expr.String(x)
}

// isPositional reports whether a channel has an axis rather than a legend.
func isPositional(ch string) bool {
	switch ch {
	case "x", "y", "xOffset", "yOffset", "theta", "radius":
		return true
	}
	return false
}

// titleOf returns the title of a channel, as Vega-Lite writes it: Sum of
// price, Count of Records, price (binned) or date (month).
func titleOf(d *channelDef) string {
	switch {
	case d.aggregate == "count":
		return "Count of Records"
	case d.aggregate != "":
		op := d.aggregate
		names := map[string]string{"mean": "Mean", "average": "Average", "sum": "Sum", "median": "Median", "min": "Min", "max": "Max", "stdev": "Standard Deviation", "variance": "Variance", "distinct": "Distinct Count", "q1": "Q1", "q3": "Q3", "valid": "Valid", "missing": "Missing", "argmin": "Argmin", "argmax": "Argmax", "ci0": "CI0", "ci1": "CI1", "stderr": "Standard Error"}
		if n, ok := names[op]; ok {
			op = n
		}
		return op + " of " + d.field
	case d.bin != nil:
		return d.field + " (binned)"
	case d.units != nil:
		return d.field + " (" + strings.Join(d.units, "-") + ")"
	}
	return d.field
}

// scaleTypeOf chooses the type of the scale of a channel from the type of
// its field and the mark, as Vega-Lite documents it.
func scaleTypeOf(ch string, d *channelDef, mark string, props map[string]any) (string, bool) {
	discrete := !d.continuous() || (d.units != nil && d.typ != temporal)
	var typ string
	switch {
	case ch == "x" || ch == "y" || ch == "xOffset" || ch == "yOffset":
		switch {
		case discrete && (mark == "bar" || mark == "rect" || mark == "tick" || mark == "rule"):
			typ = "band"
		case discrete:
			typ = "point"
		case d.typ == temporal && d.utc:
			typ = "utc"
		case d.typ == temporal:
			typ = "time"
		default:
			typ = "linear"
		}
	case ch == "shape" || ch == "strokeDash":
		typ = "ordinal"
	case ch == "size" && discrete:
		// Sizes spread over their range in the order of the values.
		typ = "point"
	case ch == "theta":
		typ = "linear"
		if discrete {
			typ = "band"
		}
	case discrete:
		typ = "ordinal"
	case d.bin != nil && (ch == "color" || ch == "fill" || ch == "stroke"):
		typ, discrete = "bin-ordinal", true
	case d.typ == temporal:
		typ = "time"
	case ch == "radius":
		typ = "sqrt"
	default:
		typ = "linear"
	}
	if t, ok := props["type"].(string); ok {
		typ = t
		discrete = typ == "band" || typ == "point" || typ == "ordinal"
	}
	return typ, discrete
}

// finish writes the view into the compiled spec: its marks, scales,
// guides and size.
func (v *view) finish(out *vega.Spec) {
	out.Style = vega.Strings{List: []string{"cell"}}
	slices.SortStableFunc(v.order, func(a, b string) int {
		return slices.Index(channelOrder, a) - slices.Index(channelOrder, b)
	})
	for _, name := range v.order {
		s := v.scales[name]
		out.Scales = append(out.Scales, v.scaleOf(s))
	}
	wname, hname := "width", "height"
	if v.sizeNames[0] != "" {
		wname, hname = v.sizeNames[0], v.sizeNames[1]
	}
	w, wsig := v.dimension("x", wname, v.width)
	h, hsig := v.dimension("y", hname, v.height)
	out.Signals = append(out.Signals, wsig...)
	out.Signals = append(out.Signals, hsig...)
	if !w.IsZero() {
		out.Width = w
	}
	if !h.IsZero() {
		out.Height = h
	}
	v.guides(out)
	out.Marks = v.marks
}

// dimension returns the width or the height of the view, or the signals
// that work it out from the step of a discrete scale.
func (v *view) dimension(ch, name string, given vega.Value) (vega.NumberValue, []vega.Signal) {
	step := float64(discreteStep)
	if v.cfg != nil && v.cfg.step > 0 {
		step = v.cfg.step
	}
	if !given.IsZero() {
		switch x := valueOf(given).(type) {
		case float64:
			return vega.Num(x), nil
		case map[string]any:
			if st, ok := x["step"].(float64); ok {
				step = st
			}
		case string:
			// "container" fills a container that a static chart has not.
		}
	}
	s, ok := v.scales[ch]
	if !ok || !s.discrete || s.typ == "ordinal" {
		if !ok && hasMarkWithout(v, ch) {
			return vega.Num(step), nil
		}
		return vega.Num(continuousSize), nil
	}
	inner, outer := s.paddings()
	stepName := v.sn(ch) + "_step"
	return vega.NumberValue{}, []vega.Signal{
		{Name: stepName, Value: mustJSON(step)},
		{Name: name, Update: vega.Expr(fmt.Sprintf("bandspace(domain('%s').length, %s, %s) * %s", v.sn(ch), num(inner), num(outer), stepName))},
	}
}

// sizeName returns the name of the width, for 0, or of the height, for 1.
func (v *view) sizeName(i int) string {
	if v.sizeNames[i] != "" {
		return v.sizeNames[i]
	}
	return [2]string{"width", "height"}[i]
}

// fixed reports whether a width or a height is a number, rather than a
// step or nothing.
func fixed(v vega.Value) bool {
	_, ok := valueOf(v).(float64)
	return ok
}

// hasMarkWithout reports whether the view has no channel on an axis, so
// that its size is one step.
func hasMarkWithout(v *view, ch string) bool {
	return len(v.marks) > 0 && v.scales[ch] == nil
}

// num writes a number as JavaScript does.
func num(f float64) string {
	return expr.String(f)
}

// paddings returns the inner and the outer padding of a band or a point
// scale.
func (s *scaleComp) paddings() (float64, float64) {
	inner, outer := 0.1, 0.05
	switch {
	case s.typ == "point":
		inner, outer = 1, 0.5
	case s.mark == "rect" || s.channel == "xOffset" || s.channel == "yOffset":
		inner, outer = 0, 0
	}
	if p, ok := s.props["padding"].(float64); ok {
		inner, outer = p, p
		if s.typ == "point" {
			inner, outer = 1, p
		}
	}
	if p, ok := s.props["paddingInner"].(float64); ok {
		inner = p
	}
	if p, ok := s.props["paddingOuter"].(float64); ok {
		outer = p
	}
	return inner, outer
}

// scaleOf builds the Vega scale of a scale.
func (v *view) scaleOf(s *scaleComp) vega.Scale {
	vs := vega.Scale{Name: v.sn(s.name), Type: s.typ}
	if s.typ == "linear" {
		vs.Type = "linear"
	}
	// The domain.
	switch {
	case s.normalized && s.props["domain"] == nil:
		// A stack normalized to one fills the scale.
		vs.Domain = vega.ScaleDomain{Values: []vega.Value{lit(0), lit(1)}}
	case s.props["domain"] != nil:
		switch d := s.props["domain"].(type) {
		case []any:
			vals := make([]vega.Value, len(d))
			for i, x := range d {
				if m, ok := x.(map[string]any); ok && isDateTime(m) {
					vals[i] = vega.Value{Signal: dateTimeExpr(m)}
					continue
				}
				vals[i] = lit(x)
			}
			vs.Domain = vega.ScaleDomain{Values: vals}
		case map[string]any:
			switch {
			case d["expr"] != nil:
				vs.Domain = vega.ScaleDomain{Signal: expr.String(d["expr"])}
			case d["unionWith"] != nil:
				// The values come first, then the values of the data.
				vs.Domain = v.dataDomain(s)
				if ref := vs.Domain.Data; ref != nil {
					var fields []any
					if !ref.Field.IsZero() {
						fields = []any{map[string]any{"data": ref.Data, "field": ref.Field.Name}}
					} else {
						for _, f := range ref.Fields {
							if name, ok := f.(string); ok {
								f = map[string]any{"data": ref.Data, "field": name}
							}
							fields = append(fields, f)
						}
					}
					ref.Data, ref.Field = "", vega.Field{}
					ref.Fields = append([]any{d["unionWith"]}, fields...)
				}
			default:
				vs.Domain = v.dataDomain(s)
			}
		default:
			vs.Domain = v.dataDomain(s)
		}
	case s.bins != "" && s.typ == "bin-ordinal":
		// The bins are the domain.
		vs.Bins = vega.Value{Signal: s.bins}
	case s.bins != "":
		vs.Domain = vega.ScaleDomain{Signal: "[" + s.bins + ".start, " + s.bins + ".stop]"}
		vs.Bins = vega.Value{Signal: s.bins}
	default:
		vs.Domain = v.dataDomain(s)
	}
	for _, k := range []string{"domainMin", "domainMax", "domainMid"} {
		if x, ok := s.props[k]; ok {
			val := lit(x)
			if m, ok := x.(map[string]any); ok && isDateTime(m) {
				val = vega.Value{Signal: dateTimeExpr(m)}
			}
			switch k {
			case "domainMin":
				vs.DomainMin = val
			case "domainMax":
				vs.DomainMax = val
			default:
				vs.DomainMid = val
			}
		}
	}
	// The range.
	switch {
	case s.props["range"] != nil:
		switch r := s.props["range"].(type) {
		case string:
			vs.Range = vega.ScaleRange{Name: r}
		case []any:
			vals := make([]vega.Value, len(r))
			for i, x := range r {
				vals[i] = lit(x)
			}
			vs.Range = vega.ScaleRange{Values: vals}
		}
	case s.props["scheme"] != nil:
		ref := &vega.SchemeRef{}
		switch sc := s.props["scheme"].(type) {
		case string:
			ref.Scheme = lit(sc)
		case map[string]any:
			ref.Scheme = lit(sc["name"])
			if n, ok := sc["count"]; ok {
				ref.Count = lit(n)
			}
			if e, ok := sc["extent"]; ok {
				ref.Extent = lit(e)
			}
		}
		vs.Range = vega.ScaleRange{Scheme: ref}
	default:
		vs.Range = v.defaultRange(s)
	}
	// The other properties.
	positional := s.channel == "x" || s.channel == "y"
	quant := !s.discrete && s.typ != "time" && s.typ != "utc"
	if positional && quant && s.bins == "" && s.props["domain"] == nil {
		vs.Nice = lit(true)
	}
	if positional && !s.discrete && s.padded && s.props["padding"] == nil {
		// A bar on a continuous dimension keeps clear of the ends.
		vs.Padding = lit(5)
	}
	if s.discrete && (s.typ == "band" || s.typ == "point") {
		inner, outer := s.paddings()
		if s.typ == "band" {
			vs.PaddingInner = lit(inner)
			vs.PaddingOuter = lit(outer)
		} else {
			vs.Padding = lit(outer)
		}
	}
	switch {
	case s.channel == "size" && s.typ == "quantize", quant && (s.channel == "opacity" || s.channel == "fillOpacity" || s.channel == "strokeOpacity"):
		vs.Zero = vega.BoolValue{Kind: vega.Literal, Bool: false}
	case quant && (positional || s.channel == "size" || s.channel == "theta" || s.channel == "radius") && s.typ != "log" && !discretizing(s.typ):
		zero := (s.bins == "" || s.channel == "size") && !s.binned && s.props["domain"] == nil && !s.dimension
		if v.cfg != nil && v.cfg.zero != nil && positional {
			zero = zero && *v.cfg.zero
		}
		vs.Zero = vega.BoolValue{Kind: vega.Literal, Bool: zero}
	}
	if s.typ == "bin-ordinal" && s.props["range"] == nil && s.props["scheme"] == nil {
		vs.Interpolate = lit("hcl")
	}
	if (s.typ == "time" || s.typ == "utc") && (s.channel == "color" || s.channel == "fill" || s.channel == "stroke") && s.props["interpolate"] == nil {
		vs.Interpolate = lit("hcl")
	}
	if quant && (s.channel == "color" || s.channel == "fill" || s.channel == "stroke") {
		vs.Interpolate = lit("hcl")
		vs.Zero = vega.BoolValue{Kind: vega.Literal, Bool: false}
	}
	if s.typ == "ordinal" && (s.channel == "color" || s.channel == "fill" || s.channel == "stroke") && s.field != nil && s.field.typ == ordinal && s.props["range"] == nil && s.props["scheme"] == nil {
		vs.Interpolate = lit("hcl")
	}
	for k, x := range s.props {
		switch k {
		case "zero":
			if b, ok := x.(bool); ok {
				vs.Zero = vega.BoolValue{Kind: vega.Literal, Bool: b}
			}
		case "nice":
			vs.Nice = lit(x)
		case "reverse":
			if b, ok := x.(bool); ok {
				vs.Reverse = vega.BoolValue{Kind: vega.Literal, Bool: b}
			}
		case "round":
			if b, ok := x.(bool); ok {
				vs.Round = vega.BoolValue{Kind: vega.Literal, Bool: b}
			}
		case "clamp":
			if b, ok := x.(bool); ok {
				vs.Clamp = vega.BoolValue{Kind: vega.Literal, Bool: b}
			}
		case "interpolate":
			vs.Interpolate = lit(x)
		case "exponent":
			vs.Exponent = lit(x)
		case "base":
			vs.Base = lit(x)
		case "constant":
			vs.Constant = lit(x)
		case "align":
			vs.Align = lit(x)
		}
	}
	return vs
}

// dataDomain returns the domain of a scale from the fields of its datasets.
func (v *view) dataDomain(s *scaleComp) vega.ScaleDomain {
	if len(s.refs) == 0 {
		return vega.ScaleDomain{}
	}
	ref := &vega.DataRef{}
	same := true
	for _, r := range s.refs {
		if r.data != s.refs[0].data {
			same = false
		}
	}
	switch {
	case len(s.refs) == 1:
		ref.Data = s.refs[0].data
		ref.Field = vega.Field{Name: s.refs[0].field}
	case same:
		ref.Data = s.refs[0].data
		for _, r := range s.refs {
			ref.Fields = append(ref.Fields, r.field)
		}
	default:
		for _, r := range s.refs {
			ref.Fields = append(ref.Fields, map[string]any{"data": r.data, "field": r.field})
		}
	}
	if s.discrete && s.typ != "point" || s.discrete {
		sortV := any(true)
		if s.hasSort {
			sortV = sortOf(s.sortVal)
		}
		if sortV != nil {
			ref.Sort = mustJSON(sortV)
		}
	}
	if s.discrete && len(s.refs) > 1 && !same {
		// A sort of fields of several datasets takes an op.
		var m map[string]any
		if json.Unmarshal(ref.Sort, &m) == nil && m != nil && m["op"] == nil {
			m["op"] = "min"
			ref.Sort = mustJSON(m)
		}
	}
	return vega.ScaleDomain{Data: ref}
}

// sortByChannel returns the sort of a domain by another channel, such as
// "-x": the op and the field of that channel, and the order.
func sortByChannel(u *unit, d *channelDef) map[string]any {
	name, ok := valueOf(d.def.Sort).(string)
	if !ok {
		return nil
	}
	order := "ascending"
	if strings.HasPrefix(name, "-") {
		order, name = "descending", name[1:]
	}
	o := u.ch[name]
	if o == nil {
		return nil
	}
	op := o.aggregate
	switch {
	case op != "":
	case u.aggregated():
		op = "sum"
	default:
		// Rows that are not aggregated sort by the least value.
		op = "min"
	}
	out := map[string]any{"op": op, "field": o.field, "order": order}
	if op == "count" {
		delete(out, "field")
	}
	return out
}

// sortOf turns the sort of a channel into the sort of the domain of its
// scale. A list of values is not a sort of the data, and keeps the order
// of the data.
func sortOf(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		switch x {
		case "ascending":
			return true
		case "descending":
			return map[string]any{"order": "descending", "field": "key"}
		case "x", "y", "-x", "-y", "color", "-color":
			// A sort by another channel needs its field, which the
			// compiler does not read here.
			return true
		}
	case map[string]any:
		out := map[string]any{}
		if f, ok := x["field"]; ok {
			out["field"] = f
		}
		if op, ok := x["op"]; ok {
			out["op"] = op
		} else if _, ok := x["field"]; ok {
			out["op"] = "sum"
		}
		if o, ok := x["order"]; ok {
			out["order"] = o
		}
		if len(out) == 0 {
			return true
		}
		return out
	case []any:
		return true
	}
	return true
}

// defaultRange returns the range of a scale that the spec does not give.
func (v *view) defaultRange(s *scaleComp) vega.ScaleRange {
	switch s.channel {
	case "x", "xOffset":
		if s.discrete && s.typ != "ordinal" && s.channel == "x" && !fixed(v.width) {
			return vega.ScaleRange{Step: vega.Value{Signal: v.sn("x") + "_step"}}
		}
		if s.channel == "xOffset" {
			return offsetRange(s, "bandwidth('"+v.sn("x")+"')", "width")
		}
		return vega.ScaleRange{Values: []vega.Value{lit(0), {Signal: v.sizeName(0)}}}
	case "y", "yOffset":
		if s.discrete && s.typ != "ordinal" && s.channel == "y" && !fixed(v.height) {
			return vega.ScaleRange{Step: vega.Value{Signal: v.sn("y") + "_step"}}
		}
		if s.channel == "yOffset" {
			return offsetRange(s, "bandwidth('"+v.sn("y")+"')", "height")
		}
		return vega.ScaleRange{Values: []vega.Value{{Signal: v.sizeName(1)}, lit(0)}}
	case "color", "fill", "stroke":
		switch {
		case s.typ == "bin-ordinal":
			return vega.ScaleRange{Name: "ramp"}
		case s.discrete && s.field != nil && s.field.typ == ordinal:
			return vega.ScaleRange{Name: "ordinal"}
		case s.discrete:
			return vega.ScaleRange{Name: "category"}
		}
		return vega.ScaleRange{Name: "ramp"}
	case "shape":
		return vega.ScaleRange{Name: "symbol"}
	case "size":
		switch s.mark {
		case "bar", "tick":
			return vega.ScaleRange{Values: []vega.Value{lit(1), lit(20)}}
		case "text":
			return vega.ScaleRange{Values: []vega.Value{lit(8), lit(40)}}
		case "line", "rule":
			return vega.ScaleRange{Values: []vega.Value{lit(1), lit(8)}}
		}
		return vega.ScaleRange{Values: []vega.Value{lit(0), lit(361)}}
	case "opacity", "fillOpacity", "strokeOpacity":
		return vega.ScaleRange{Values: []vega.Value{lit(0.3), lit(0.8)}}
	case "strokeWidth":
		return vega.ScaleRange{Values: []vega.Value{lit(1), lit(8)}}
	case "strokeDash":
		return vega.ScaleRange{Values: []vega.Value{lit([]any{1, 0}), lit([]any{4, 2}), lit([]any{2, 1}), lit([]any{1, 1}), lit([]any{1, 2, 4, 2})}}
	case "theta":
		return vega.ScaleRange{Values: []vega.Value{lit(0), lit(2 * math.Pi)}}
	case "radius":
		return vega.ScaleRange{Values: []vega.Value{lit(0), {Signal: "min(width,height)/2"}}}
	case "angle":
		return vega.ScaleRange{Values: []vega.Value{lit(0), lit(360)}}
	}
	return vega.ScaleRange{Name: "category"}
}

// guides adds the axes of the positional scales and the legends of the
// others.
func (v *view) guides(out *vega.Spec) {
	legends := map[string]*vega.Legend{}
	var legendOrder []string
	var grids, mains []vega.Axis
	for _, name := range v.order {
		s := v.scales[name]
		if s.noGuide {
			continue
		}
		title := strings.Join(s.titles, ", ")
		if t, ok := s.guide["title"]; ok {
			if t == nil {
				title = ""
			} else {
				title = titleString(t)
			}
		}
		switch s.channel {
		case "x", "y":
			other := "y"
			if s.channel == "y" {
				other = "x"
			}
			if _, ok := v.scales[other]; !ok {
				other = ""
			}
			grid, main := axisOf(s, title, other, v)
			if grid != nil {
				grids = append(grids, *grid)
			}
			mains = append(mains, main)
		case "xOffset", "yOffset", "theta", "radius":
		default:
			field := ""
			if s.field != nil {
				field = s.field.out
			}
			key := field + "|" + title
			l, ok := legends[key]
			if !ok {
				l = &vega.Legend{}
				legends[key] = l
				legendOrder = append(legendOrder, key)
				if title != "" {
					l.Title = lit(title)
				}
			}
			sn := v.sn(name)
			switch s.channel {
			case "color", "fill":
				if s.mark == "line" || s.mark == "rule" || (s.mark == "point" && s.channel == "color") {
					l.Stroke = sn
					if s.mark == "line" || s.mark == "rule" {
						l.SymbolType = lit("stroke")
					}
				} else {
					l.Fill = sn
				}
			case "stroke":
				l.Stroke = sn
			case "size":
				l.Size = sn
			case "shape":
				l.Shape = sn
			case "opacity":
				l.Opacity = sn
			case "strokeDash":
				l.StrokeDash = sn
			}
			for k, x := range s.guide {
				switch k {
				case "orient":
					l.Orient = lit(x)
				case "direction":
					l.Direction = lit(x)
				case "format":
					l.Format = lit(x)
				case "type":
					l.Type = expr.String(x)
				case "values":
					l.Values = lit(x)
				case "symbolType":
					l.SymbolType = lit(x)
				case "columns":
					l.Columns = lit(x)
				}
			}
			if l.SymbolType.IsZero() && l.Fill != "" {
				switch s.mark {
				case "bar", "rect", "area", "tick", "square", "arc", "image":
					l.SymbolType = lit("square")
				case "circle", "point", "text":
					l.SymbolType = lit("circle")
				case "trail", "line", "rule":
					l.SymbolType = lit("stroke")
				}
			}
			if l.Format.IsZero() && s.field != nil && s.field.units != nil {
				l.Format = lit(format.TimeUnitSpecifier(s.field.units, vlSpecifiers))
			}
		}
	}
	// The grids go first, under the axes, then the axes, x before y.
	slices.SortStableFunc(grids, func(a, b vega.Axis) int { return strings.Compare(b.Scale, a.Scale) })
	slices.SortStableFunc(mains, func(a, b vega.Axis) int { return strings.Compare(a.Scale, b.Scale) })
	out.Axes = append(out.Axes, grids...)
	out.Axes = append(out.Axes, mains...)
	for _, k := range legendOrder {
		out.Legends = append(out.Legends, *legends[k])
	}
}

// vlSpecifiers are the parts of the formats of time units that Vega-Lite
// uses for its guides.
var vlSpecifiers = map[string]string{"year-month": "%b %Y ", "year-month-date": "%b %d, %Y "}

// axisOf returns the axes of a positional scale: an axis of grid lines for
// a continuous scale, and the axis of labels, as Vega-Lite writes them.
func axisOf(s *scaleComp, title string, other string, v *view) (grid *vega.Axis, main vega.Axis) {
	orient := "bottom"
	size := v.sizeName(0)
	if s.channel == "y" {
		orient, size = "left", v.sizeName(1)
	}
	if s.second {
		orient = map[string]string{"bottom": "top", "left": "right"}[orient]
	}
	f, t, z := false, true, 0
	main = vega.Axis{Scale: v.sn(s.name), Orient: lit(orient), Grid: &f, ZIndex: &z}
	if title != "" {
		main.Title = lit(title)
	}
	showGrid := !s.discrete && !s.second
	if g, ok := s.guide["grid"].(bool); ok {
		showGrid = g
	}
	if showGrid {
		grid = &vega.Axis{
			Scale: v.sn(s.name), Orient: lit(orient), Grid: &t, ZIndex: &z,
			Domain: vega.BoolValue{Kind: vega.Literal}, Labels: vega.BoolValue{Kind: vega.Literal},
			Ticks: vega.BoolValue{Kind: vega.Literal}, MaxExtent: lit(0), MinExtent: lit(0),
		}
		if other != "" {
			grid.GridScale = v.sn(other)
		}
		if !s.discrete {
			grid.TickCount = vega.Value{Signal: "ceil(" + size + "/40)"}
		}
	}
	if s.discrete && s.channel == "x" && (s.field == nil || s.field.units == nil) {
		main.LabelAngle = lit(270)
		main.LabelAlign = lit("right")
		main.LabelBase = lit("middle")
	}
	if !s.discrete {
		main.TickCount = vega.Value{Signal: "ceil(" + size + "/40)"}
		if s.bins != "" {
			main.TickCount = vega.Value{Signal: "ceil(" + size + "/10)"}
		}
		main.LabelOverlap = lit(true)
		if s.channel == "x" {
			main.LabelFlush = lit(true)
		}
	}
	if s.discrete && s.field != nil && s.field.units != nil {
		main.LabelOverlap = lit(true)
	}
	if s.field != nil && s.field.units != nil {
		main.Format = lit(format.TimeUnitSpecifier(s.field.units, vlSpecifiers))
		main.FormatType = lit("time")
		if s.field.utc {
			main.FormatType = lit("utc")
		}
	}
	for k, x := range s.guide {
		switch k {
		case "orient":
			main.Orient = lit(x)
			if grid != nil {
				grid.Orient = lit(x)
			}
		case "format":
			main.Format = lit(x)
		case "formatType":
			main.FormatType = lit(x)
		case "labelAngle":
			main.LabelAngle = lit(x)
			if f, ok := x.(float64); ok && f == 0 && s.channel == "x" {
				main.LabelAlign = vega.Value{}
				main.LabelBase = lit("top")
			}
		case "labelAlign":
			main.LabelAlign = lit(x)
		case "labelOverlap":
			main.LabelOverlap = lit(x)
		case "tickCount":
			main.TickCount = lit(x)
			if grid != nil {
				grid.TickCount = lit(x)
			}
		case "values":
			main.Values = lit(x)
			if grid != nil {
				grid.Values = lit(x)
			}
		case "domain":
			if b, ok := x.(bool); ok {
				main.Domain = vega.BoolValue{Kind: vega.Literal, Bool: b}
			}
		case "labels":
			if b, ok := x.(bool); ok {
				main.Labels = vega.BoolValue{Kind: vega.Literal, Bool: b}
			}
		case "ticks":
			if b, ok := x.(bool); ok {
				main.Ticks = vega.BoolValue{Kind: vega.Literal, Bool: b}
			}
		case "offset":
			main.Offset = lit(x)
		}
	}
	return grid, main
}

// discretizing reports whether a scale type maps a continuous domain to
// discrete values.
func discretizing(typ string) bool {
	return typ == "quantize" || typ == "quantile" || typ == "threshold"
}

// offsetRange returns the range of the scale of an offset: the band of the
// position, steps, or the size of the view around its middle.
func offsetRange(s *scaleComp, band, size string) vega.ScaleRange {
	switch s.offset {
	case "step":
		return vega.ScaleRange{Step: lit(20)}
	case "center":
		return vega.ScaleRange{Values: []vega.Value{{Signal: "-" + size + "/2"}, {Signal: size + "/2"}}}
	}
	return vega.ScaleRange{Values: []vega.Value{lit(0), {Signal: band}}}
}

// isDateTime reports whether an object is a date time of Vega-Lite, with a
// part of a date such as a year or hours.
func isDateTime(m map[string]any) bool {
	for _, k := range []string{"year", "quarter", "month", "date", "day", "hours", "minutes", "seconds", "milliseconds"} {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}
