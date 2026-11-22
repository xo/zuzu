package vegalite

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"math"
	"reflect"
	"slices"
	"strconv"
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
	// own holds the channels other than positions whose scales a view of
	// a concatenation keeps for itself, as an independent resolve asks.
	own map[string]bool
	// sharedPos holds the channels of position whose scales the views of
	// a concatenation share, as a shared resolve asks.
	sharedPos map[string]bool
	// common holds the scales that the views of a concatenation share, such
	// as color, and nil when the view shares none.
	common *shared
	// sharedAxes holds, for each scale of position that the view shares
	// with the other views of a concatenation, the titles and the axis
	// that the channels of the view give.
	sharedAxes map[string]*scaleComp
	cfg        *config
	marks      []vega.Mark
	scales     map[string]*scaleComp
	order      []string
	width      vega.Value
	height     vega.Value
	// late holds the changes to the marks that wait until every layer has
	// set the type of its scales.
	late []func()
	// ownLegend holds the channels whose legend each layer keeps for
	// itself, as an independent resolve of the legends asks.
	ownLegend map[string]bool
}

// layerLegend is the legend that a layer keeps for itself: its title, and
// the color of the mark of the layer, or nil.
type layerLegend struct {
	title string
	color any
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
	// indepPrefix names the independent scales: the prefix of the view
	// that made them independent.
	indepPrefix string
}

// key returns the key of the scale of a channel among the scales of the
// view: the channel, or the channel with the prefix of a layer whose
// scale of the channel is its own.
func (v *view) key(ch string) string {
	if v.independent[ch] {
		return v.indepPrefix + ch
	}
	return ch
}

// newView returns an empty view.
func newView(prefix string, cfg *config) *view {
	return &view{shared: &shared{scales: map[string]*scaleComp{}, cfg: cfg}, prefix: prefix}
}

// layer returns a layer of the view, which shares its scales.
func (v *view) layer(prefix, parent string) *view {
	return &view{shared: v.shared, prefix: parent + prefix, independent: v.independent, indepPrefix: v.indepPrefix}
}

// sn returns the name of the scale of a channel: the channel, with the
// prefix of the view for a position.
func (v *view) sn(ch string) string {
	if v.independent[ch] {
		return v.indepPrefix + ch
	}
	if v.scalePrefix != "" && (isPositional(ch) && !v.sharedPos[ch] || v.prefixAll || v.own[ch]) {
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
	// origin is the view of the unit that added the scale first. A scale
	// that the views of a concatenation share reads its default range
	// from the positions of that view.
	origin *view
	// offset is the range of a scale of an offset: "band" for the band of
	// the scale of its position, "step" for steps of 20 pixels, "center"
	// around the middle of the view when there is no position, and "unit"
	// for the width of one unit of time on a scale of time.
	offset string
	// unitWidth is the signal of the width of one unit of time on the scale
	// of the position, and unitBand the place of the bars in that unit, from
	// 0 at its start to 1 at its end, for an offset of "unit".
	unitWidth string
	unitBand  float64
	// datums are the constants of the data that channels put through the
	// scale, which its domain holds.
	datums []any
	// normalized says that the scale draws a stack normalized to one.
	normalized bool
	// second says that the scale is the own scale of a layer after the
	// first, whose axis goes on the other side.
	second bool
	mark   string
	// titles are the titles of the fields that the scale draws, for its
	// guide.
	titles []string
	// layerLegends are the legends of the layers that keep a legend of
	// their own, in the order of the layers, and titled says that a
	// channel gives the title of the scale.
	layerLegends []layerLegend
	titled       bool
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
	// zeroKept says that a rule or a tick draws on the scale, which keeps
	// zero in a dimension.
	zeroKept bool
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
	if v.common != nil && (!isPositional(name) || v.sharedPos[name]) && !v.own[name] {
		ok := (&view{shared: v.common}).addScale(u, name, d, fields)
		if s := v.common.scales[name]; s != nil && s.origin == nil {
			s.origin = v
		}
		if ok && (name == "x" || name == "y") {
			// Each view draws the axis of the shared scale with its own
			// title and properties.
			if v.sharedAxes == nil {
				v.sharedAxes = map[string]*scaleComp{}
			}
			g := v.sharedAxes[name]
			if g == nil {
				g = &scaleComp{}
				v.sharedAxes[name] = g
			}
			g.takeGuide(name, d)
		}
		return ok
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
		if o := u.ch[name+"Offset"]; o != nil && (o.isField() || o.hasDatum) {
			// A position split by an offset has bands.
			s.typ = "band"
		}
	}
	if s.typ == "point" && props["type"] == nil && isPositional(name) && (u.mark == "bar" || u.mark == "rect" || u.mark == "tick" || u.mark == "rule") {
		// A discrete scale that a bar draws on has bands, whatever drew
		// on it first.
		s.typ = "band"
	}
	if d.bin != nil {
		s.bins = binPrefix(d.bin) + "_" + varName(d.field) + "_bins"
	}
	if !s.discrete && d.def != nil && valueOf(d.def.Sort) == "descending" {
		// A continuous scale sorted descending runs backward, as
		// vega-lite.js writes it, unless the scale says otherwise.
		if _, ok := s.props["reverse"]; !ok {
			s.props["reverse"] = true
		}
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
		} else if m, ok := valueOf(d.def.Sort).(map[string]any); ok {
			// An op over the rows, such as count, sorts the domain, and
			// not the order of the bins.
			s.sortVal, s.hasSort = m, true
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
	if (u.mark == "rule" || u.mark == "tick") && (name == "x" || name == "y") {
		// A rule or a tick on the scale, as a box plot draws, keeps zero
		// in the domain, as vega-lite.js merges it.
		s.zeroKept = true
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
	if v.ownLegend[name] {
		// Each layer draws a legend of its own, with its own title and
		// the color of its mark.
		lg := layerLegend{title: titleOf(d)}
		if d.def != nil && !d.def.Title.IsZero() {
			lg.title = ""
			if x := valueOf(d.def.Title); x != nil {
				lg.title = titleString(x)
			}
		}
		if c, ok := u.props["color"]; ok {
			lg.color = c
		}
		if c := u.ch["color"]; c != nil && c.def != nil && c.def.Field.IsZero() && !c.def.Value.IsZero() {
			lg.color = valueOf(c.def.Value)
		}
		s.layerLegends = append(s.layerLegends, lg)
	}
	s.takeGuide(name, d)
	return true
}

// takeGuide adds the title and the axis or the legend that a channel gives
// to a scale.
func (s *scaleComp) takeGuide(name string, d *channelDef) {
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
			s.titled = true
			x := valueOf(d.def.Title)
			if x == nil {
				s.titles = []string{""}
			} else {
				s.titles = []string{titleString(x)}
			}
		}
	}
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
	field := d.name
	if field == "" {
		field = d.field
	}
	switch {
	case d.aggregate == "count":
		return "Count of Records"
	case d.aggregate != "":
		op := d.aggregate
		names := map[string]string{"mean": "Mean", "average": "Average", "sum": "Sum", "median": "Median", "min": "Min", "max": "Max", "stdev": "Standard Deviation", "variance": "Variance", "distinct": "Distinct Count", "q1": "Q1", "q3": "Q3", "valid": "Valid", "missing": "Missing", "argmin": "Argmin", "argmax": "Argmax", "ci0": "CI0", "ci1": "CI1", "stderr": "Standard Error"}
		if n, ok := names[op]; ok {
			op = n
		}
		return op + " of " + field
	case d.bin != nil:
		return field + " (binned)"
	case d.units != nil:
		return field + " (" + strings.Join(d.units, "-") + ")"
	}
	return field
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
	for _, f := range v.late {
		f()
	}
	v.late = nil
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
	s, ok := v.scales[v.key(ch)]
	if !ok && v.common != nil && v.sharedPos[ch] {
		// The views of a concatenation share the scale.
		s, ok = v.common.scales[ch]
	}
	if !ok || !s.discrete || s.typ == "ordinal" {
		if !ok && hasMarkWithout(v, ch) {
			return vega.Num(step), nil
		}
		return vega.Num(continuousSize), nil
	}
	inner, outer := v.bandPaddings(s)
	stepName := v.sn(ch) + "_step"
	stepSig := vega.Signal{Name: stepName, Value: mustJSON(step)}
	if off := v.offsetOf(ch); off != nil {
		// The step goes to the offset, and each band holds the steps of
		// its offsets, as vega-lite.js sizes a group of bars.
		oi, oo := off.paddings()
		stepSig = vega.Signal{Name: stepName, Update: vega.Expr(fmt.Sprintf("%s * bandspace(domain('%s').length, %s, %s) / (1-%s)",
			num(step), v.sn(ch+"Offset"), num(oi), num(oo), num(inner)))}
	}
	return vega.NumberValue{}, []vega.Signal{
		stepSig,
		{Name: name, Update: vega.Expr(fmt.Sprintf("bandspace(domain('%s').length, %s, %s) * %s", v.sn(ch), num(inner), num(outer), stepName))},
	}
}

// offsetOf returns the discrete scale of the offset of a channel of
// position whose own scale is a band with a step, or nil. Such an offset
// takes the step of the view, and the band holds its steps, unless the
// step of the view is "for" the position.
func (v *view) offsetOf(ch string) *scaleComp {
	s := v.scales[v.key(ch)]
	off := v.scales[v.key(ch+"Offset")]
	size := v.width
	if ch == "y" {
		size = v.height
	}
	if s == nil || off == nil || !s.discrete || s.typ != "band" || !off.discrete || fixed(size) {
		return nil
	}
	// A step "for" the position is the step of the band, and the offset
	// fills the band.
	if m, ok := valueOf(size).(map[string]any); ok && m["for"] == "position" {
		return nil
	}
	return off
}

// stepOf returns the step of a discrete channel of position: the step of
// the width or the height, of the config, or 20.
func (v *view) stepOf(ch string) float64 {
	step := float64(discreteStep)
	if v.cfg != nil && v.cfg.step > 0 {
		step = v.cfg.step
	}
	size := v.width
	if ch == "y" {
		size = v.height
	}
	if m, ok := valueOf(size).(map[string]any); ok {
		if st, ok := m["step"].(float64); ok {
			step = st
		}
	}
	return step
}

// bandPaddings returns the paddings of a band or a point scale, with the
// paddings of 0.2 that vega-lite.js gives a band of position whose groups
// an offset splits, offsetBandPaddingInner and offsetBandPaddingOuter of
// its config.
func (v *view) bandPaddings(s *scaleComp) (float64, float64) {
	inner, outer := s.paddings()
	if (s.channel == "x" || s.channel == "y") && v.scales[v.key(s.channel+"Offset")] != nil && s.typ == "band" {
		if _, ok := s.props["paddingInner"]; !ok {
			if _, ok := s.props["padding"]; !ok {
				inner = 0.2
			}
		}
		if _, ok := s.props["paddingOuter"]; !ok {
			if _, ok := s.props["padding"]; !ok {
				outer = 0.2
			}
		}
	}
	return inner, outer
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
// that its size is one step. A view of arcs, which place by theta and
// radius, has the continuous size, as Vega-Lite gives it.
func hasMarkWithout(v *view, ch string) bool {
	if len(v.marks) == 0 || v.scales[v.key(ch)] != nil {
		return false
	}
	for _, m := range v.marks {
		if m.Type == "arc" {
			return false
		}
	}
	return true
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
	case s.mark == "tick":
		// A tick fills its band, which leaves a quarter of a step
		// between ticks, as vega-lite.js draws them.
		inner, outer = 0.25, 0.125
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
		case map[string]any:
			vs.Range = v.fieldRange(s, r)
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
	// rangeMin and rangeMax replace the ends of a range of two values.
	if r := vs.Range.Values; len(r) == 2 {
		if x, ok := s.props["rangeMin"]; ok {
			r[0] = lit(x)
		}
		if x, ok := s.props["rangeMax"]; ok {
			r[1] = lit(x)
		}
		if s.channel == "size" && (s.typ == "quantile" || s.typ == "quantize") && s.props["range"] == nil {
			// A quantile or a quantize scale of sizes cuts its domain into
			// four pieces, quantileCount and quantizeCount of the config
			// of Vega-Lite, so it takes four sizes from end to end.
			end := func(v vega.Value) string {
				if v.Signal != "" {
					return "(" + v.Signal + ")"
				}
				return string(v.JSON)
			}
			lo, hi := end(r[0]), end(r[1])
			step := "(" + hi + " - " + lo + ") / 3"
			vs.Range = vega.ScaleRange{Signal: "sequence(" + lo + ", " + hi + " + " + step + ", " + step + ")"}
		}
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
	if p, ok := s.props["padding"]; ok && !s.discrete {
		// The padding of a continuous scale is in pixels.
		vs.Padding = lit(p)
	}
	if s.discrete && (s.typ == "band" || s.typ == "point") && s.channel != "size" {
		// The paddings are of a position or an offset. A point scale
		// of size has none.
		inner, outer := v.bandPaddings(s)
		if s.typ == "band" {
			vs.PaddingInner = lit(inner)
			vs.PaddingOuter = lit(outer)
		} else {
			vs.Padding = lit(outer)
		}
	}
	switch {
	case s.channel == "size" && s.typ == "quantize", quant && (s.channel == "opacity" || s.channel == "fillOpacity" || s.channel == "strokeOpacity" || s.channel == "xOffset" || s.channel == "yOffset"):
		vs.Zero = vega.BoolValue{Kind: vega.Literal, Bool: false}
	case quant && (positional || s.channel == "size" || s.channel == "theta" || s.channel == "radius") && s.typ != "log" && !discretizing(s.typ):
		zero := (s.bins == "" || s.channel == "size") && !s.binned && s.props["domain"] == nil && (!s.dimension || s.zeroKept)
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

// binnedMaxSize returns the signal of the largest size of a point of the
// size scale s when x and y are both binned, or "" when they are not. A
// point then fills 95% of the smaller step of the two bins, as vega-lite.js
// draws it. The size of a point is an area, so the signal is the square of
// that width.
func (v *view) binnedMaxSize(s *scaleComp) string {
	if s.origin != nil {
		v = s.origin
	}
	x, y := v.scales[v.key("x")], v.scales[v.key("y")]
	if x == nil || y == nil || x.bins == "" || y.bins == "" || x.discrete || y.discrete {
		return ""
	}
	step := func(size, bins string) string {
		return size + " / ((" + bins + ".stop - " + bins + ".start) / " + bins + ".step)"
	}
	return "pow(0.95 * min(" + step(v.sizeName(0), x.bins) + ", " + step(v.sizeName(1), y.bins) + "), 2)"
}

// fieldRange returns the range {"field": ...} of a scale: the values of that
// field in the dataset of the domain. Each value goes with the value of the
// domain on its row, so the range sorts by the domain, as vega-lite.js
// writes it. A range with no field, or a scale with no field in its
// domain, keeps the default range.
func (v *view) fieldRange(s *scaleComp, r map[string]any) vega.ScaleRange {
	f, ok := r["field"].(string)
	if !ok || len(s.refs) == 0 {
		return v.defaultRange(s)
	}
	ref := s.refs[0]
	return vega.ScaleRange{Data: &vega.DataRef{
		Data:  ref.data,
		Field: vega.Field{Name: f},
		Sort:  mustJSON(map[string]any{"op": "min", "field": ref.field}),
	}}
}

// dataDomain returns the domain of a scale from the fields of its datasets.
func (v *view) dataDomain(s *scaleComp) vega.ScaleDomain {
	if len(s.refs) == 0 {
		if len(s.datums) > 0 {
			// The constants alone make the domain.
			ref := &vega.DataRef{}
			for _, x := range s.datums {
				ref.Fields = append(ref.Fields, datumField(x))
			}
			return vega.ScaleDomain{Data: ref}
		}
		return vega.ScaleDomain{}
	}
	ref := &vega.DataRef{}
	// A constant is a list of its own, so the fields name each dataset.
	same := len(s.datums) == 0
	for _, r := range s.refs {
		if r.data != s.refs[0].data {
			same = false
		}
	}
	switch {
	case len(s.refs) == 1 && same:
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
		for _, x := range s.datums {
			ref.Fields = append(ref.Fields, datumField(x))
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
			if v.offsetOf("x") != nil && s.offset == "" {
				return vega.ScaleRange{Step: lit(v.stepOf("x"))}
			}
			v.unitOffset(s, "x")
			return offsetRange(s, "bandwidth('"+v.sn("x")+"')", "width")
		}
		return vega.ScaleRange{Values: []vega.Value{lit(0), {Signal: v.sizeName(0)}}}
	case "y", "yOffset":
		if s.discrete && s.typ != "ordinal" && s.channel == "y" && !fixed(v.height) {
			return vega.ScaleRange{Step: vega.Value{Signal: v.sn("y") + "_step"}}
		}
		if s.channel == "yOffset" {
			if v.offsetOf("y") != nil && s.offset == "" {
				return vega.ScaleRange{Step: lit(v.stepOf("y"))}
			}
			v.unitOffset(s, "y")
			if !s.discrete && s.offset == "" {
				// A continuous offset runs up the band, as the scale
				// of y does.
				return vega.ScaleRange{Values: []vega.Value{{Signal: "bandwidth('" + v.sn("y") + "')"}, lit(0)}}
			}
			return offsetRange(s, "bandwidth('"+v.sn("y")+"')", "height")
		}
		if s.discrete {
			// A discrete y puts its first value at the top, as a step
			// does.
			return vega.ScaleRange{Values: []vega.Value{lit(0), {Signal: v.sizeName(1)}}}
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
		case s.props["domainMid"] != nil:
			// A middle of the domain asks for a diverging range, as
			// vega-lite.js gives it.
			return vega.ScaleRange{Name: "diverging"}
		case s.typ == "linear" && (s.mark == "rect" || s.mark == "geoshape"):
			// A linear color of rects or shapes fills a heat map.
			return vega.ScaleRange{Name: "heatmap"}
		}
		return vega.ScaleRange{Name: "ramp"}
	case "shape":
		return vega.ScaleRange{Name: "symbol"}
	case "size":
		switch s.mark {
		case "bar", "tick":
			// minBandSize and maxBandSize of the config of the scales:
			// 2, and the step of the band less 1.
			dim := "x"
			if ys, xs := v.scales[v.key("y")], v.scales[v.key("x")]; ys != nil && ys.discrete && (xs == nil || !xs.discrete) {
				dim = "y"
			}
			return vega.ScaleRange{Values: []vega.Value{lit(2), lit(v.stepOf(dim) - 1)}}
		case "text":
			return vega.ScaleRange{Values: []vega.Value{lit(8), lit(40)}}
		case "line", "rule":
			return vega.ScaleRange{Values: []vega.Value{lit(1), lit(8)}}
		case "trail":
			// The size of a trail is its width, as vega-lite.js gives it.
			return vega.ScaleRange{Values: []vega.Value{lit(1), lit(4)}}
		}
		// The smallest point is 4 square pixels, as vega-lite.js writes
		// it, so that a point of the least value still shows.
		low := lit(0)
		if s.mark == "point" || s.mark == "circle" || s.mark == "square" {
			low = lit(4)
		}
		if sig := v.binnedMaxSize(s); sig != "" {
			return vega.ScaleRange{Values: []vega.Value{low, {Signal: sig}}}
		}
		return vega.ScaleRange{Values: []vega.Value{low, lit(361)}}
	case "opacity", "fillOpacity", "strokeOpacity":
		return vega.ScaleRange{Values: []vega.Value{lit(0.3), lit(0.8)}}
	case "strokeWidth":
		return vega.ScaleRange{Values: []vega.Value{lit(1), lit(8)}}
	case "strokeDash":
		return vega.ScaleRange{Values: []vega.Value{lit([]any{1, 0}), lit([]any{4, 2}), lit([]any{2, 1}), lit([]any{1, 1}), lit([]any{1, 2, 4, 2})}}
	case "theta":
		return vega.ScaleRange{Values: []vega.Value{lit(0), lit(2 * math.Pi)}}
	case "radius":
		return vega.ScaleRange{Values: []vega.Value{lit(0), {Signal: "min(" + v.sizeName(0) + "," + v.sizeName(1) + ")/2"}}}
	case "angle":
		return vega.ScaleRange{Values: []vega.Value{lit(0), lit(360)}}
	}
	return vega.ScaleRange{Name: "category"}
}

// guides adds the axes of the positional scales and the legends of the
// others.
func (v *view) guides(out *vega.Spec) {
	// A channel shares the legend of a channel of the same field, as in
	// Vega-Lite, unless the two have titles of their own that differ. A
	// title that a channel gives wins over the title of the field.
	type legendOf struct {
		l        *vega.Legend
		field    string
		title    string
		explicit bool
		layers   []layerLegend
	}
	var legends []*legendOf
	var grids, mains []vega.Axis
	scales := make([]*scaleComp, 0, len(v.order)+len(v.sharedAxes))
	for _, name := range v.order {
		scales = append(scales, v.scales[name])
	}
	for _, ch := range []string{"x", "y"} {
		if g := v.sharedAxes[ch]; g != nil && v.common.scales[ch] != nil {
			// The shared scale with the axis of this view.
			s := *v.common.scales[ch]
			s.guide, s.noGuide, s.titles = g.guide, g.noGuide, g.titles
			scales = append(scales, &s)
		}
	}
	for _, s := range scales {
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
			if _, ok := v.scales[other]; !ok && v.sharedAxes[other] == nil {
				other = ""
			}
			grid, main := axisOf(s, title, other, v)
			if grid != nil {
				grids = append(grids, *grid)
			}
			mains = append(mains, main)
		case "xOffset", "yOffset", "theta", "radius", "angle":
			// These have no guide, as in Vega-Lite.
		default:
			field := ""
			if s.field != nil {
				field = s.field.out
			}
			_, explicit := s.guide["title"]
			explicit = explicit || s.titled
			var lo *legendOf
			for _, c := range legends {
				if c.field == field && (c.title == title || !c.explicit || !explicit) {
					lo = c
					break
				}
			}
			switch {
			case lo == nil:
				lo = &legendOf{l: &vega.Legend{}, field: field, title: title, explicit: explicit}
				legends = append(legends, lo)
				if title != "" {
					lo.l.Title = lit(title)
				}
			case explicit && !lo.explicit:
				lo.title, lo.explicit, lo.l.Title = title, true, vega.Value{}
				if title != "" {
					lo.l.Title = lit(title)
				}
			}
			l := lo.l
			if len(s.layerLegends) > 1 {
				lo.layers = s.layerLegends
			}
			sn := v.sn(s.name)
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
					if x != "" || s.field == nil || s.field.units == nil {
						l.Format = lit(x)
					}
				case "type":
					l.Type = expr.String(x)
				case "values":
					l.Values = lit(x)
				case "symbolType":
					l.SymbolType = lit(x)
				case "columns":
					l.Columns = lit(x)
				case "clipHeight":
					l.ClipHeight = lit(x)
				case "gradientLength":
					l.GradientLength = lit(x)
				case "offset":
					l.Offset = lit(x)
				case "legendX":
					l.LegendX = lit(x)
				case "legendY":
					l.LegendY = lit(x)
				default:
					if !slices.Contains(passedLegendKeys, k) {
						continue
					}
					b, err := json.Marshal(x)
					if err != nil {
						continue
					}
					if l.Extra == nil {
						l.Extra = vega.Extra{}
					}
					l.Extra[k] = b
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
				if s.field.typ != temporal && l.FormatType.IsZero() {
					// The labels of a discrete scale are dates only when
					// the legend says so. A scale of time says it itself.
					l.FormatType = lit("time")
					if s.field.utc {
						l.FormatType = lit("utc")
					}
				}
			}
		}
	}
	// The grids go first, under the axes, then the axes, x before y.
	slices.SortStableFunc(grids, func(a, b vega.Axis) int { return strings.Compare(b.Scale, a.Scale) })
	slices.SortStableFunc(mains, func(a, b vega.Axis) int { return strings.Compare(a.Scale, b.Scale) })
	out.Axes = append(out.Axes, grids...)
	out.Axes = append(out.Axes, mains...)
	for _, lo := range legends {
		if len(lo.layers) < 2 {
			out.Legends = append(out.Legends, *lo.l)
			continue
		}
		// Each layer keeps its own legend of the scale, in the color of
		// its mark.
		for _, ll := range lo.layers {
			l := *lo.l
			l.Title = vega.Value{}
			if ll.title != "" {
				l.Title = lit(ll.title)
			}
			if ll.color != nil {
				l.Extra = maps.Clone(l.Extra)
				if l.Extra == nil {
					l.Extra = vega.Extra{}
				}
				if b, err := json.Marshal(ll.color); err == nil {
					l.Extra["symbolFillColor"] = b
				}
			}
			out.Legends = append(out.Legends, l)
		}
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
	if g, ok := v.cfg.gridOf(s.channel); ok && showGrid {
		showGrid = g
	}
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
	if s.discrete && s.channel == "x" && (s.field == nil || s.field.units == nil) && (v.cfg == nil || !v.cfg.xLabelAngle) {
		main.LabelAngle = lit(270)
		main.LabelAlign = lit("right")
		main.LabelBase = lit("middle")
	}
	if !s.discrete {
		main.TickCount = vega.Value{Signal: "ceil(" + size + "/40)"}
		if s.bins != "" {
			main.TickCount = vega.Value{Signal: "ceil(" + size + "/10)"}
		}
		// The ticks of a time unit are at least one unit apart, as
		// vega-lite.js asks with a tickMinStep, so that two labels of a
		// format such as %Y do not repeat.
		if s.field != nil && s.field.units != nil {
			if step, ok := unitMinStep(s.field.units, s.field.unitStep); ok {
				main.TickMinStep = step
				if grid != nil {
					grid.TickMinStep = step
				}
			}
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
	if s.normalized {
		// A stack normalized to one shows its shares in percent.
		main.Format = lit(".0%")
	}
	// The keys go in order, so that a labelBaseline of the spec wins over
	// the one that a labelAngle of 0 sets.
	for _, k := range slices.Sorted(maps.Keys(s.guide)) {
		x := s.guide[k]
		if passedAxisKey(k) {
			if v, ok := guideValue(x); ok {
				if strings.HasPrefix(k, "grid") {
					if grid != nil {
						setExtra(grid, k, v)
					}
				} else {
					setExtra(&main, k, v)
				}
			}
			continue
		}
		switch k {
		case "orient":
			main.Orient = lit(x)
			if grid != nil {
				grid.Orient = lit(x)
			}
		case "format":
			// An empty format of a time unit is no format, as in
			// vega-lite.js, so the labels keep the format of the unit,
			// such as Mon for a day.
			if x != "" || s.field == nil || s.field.units == nil {
				main.Format = lit(x)
			}
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
		case "labelBaseline":
			if v, ok := guideValue(x); ok {
				main.LabelBase = v
			}
		case "labelPadding":
			if v, ok := guideValue(x); ok {
				main.LabelPadding = v
			}
		case "bandPosition":
			if v, ok := guideValue(x); ok {
				main.BandPosition = v
				if grid != nil {
					grid.BandPosition = v
				}
			}
		case "zindex":
			if f, ok := x.(float64); ok {
				z := int(f)
				main.ZIndex = &z
				if grid != nil {
					grid.ZIndex = &z
				}
			}
		case "labelExpr":
			if e, ok := x.(string); ok {
				text := vega.Production{{Signal: e}}
				main.Encode = vega.GuideEncode{"labels": {Sets: vega.Encode{"update": vega.Props{"text": text}}}}
			}
		}
	}
	return grid, main
}

// axisStyleKeys are the properties of an axis of Vega-Lite that style a
// part of the axis, which Vega reads with the same name and meaning: the
// text of the labels and of the title, and the lines of the grid, of the
// domain and of the ticks. The other keys of passedAxisKey place the labels
// and the title, or size the ticks.
var axisStyleKeys = map[string][]string{
	"label": textStyleEnds, "title": textStyleEnds, "grid": lineStyleEnds, "domain": lineStyleEnds,
	"tick": lineStyleEnds,
}

// textStyleEnds and lineStyleEnds are the ends of the names of the
// properties that style a text and a line, such as Color in labelColor.
var (
	textStyleEnds = []string{"Color", "Font", "FontSize", "FontStyle", "FontWeight", "Opacity"}
	lineStyleEnds = []string{"Cap", "Color", "Dash", "Opacity", "Width"}
)

// passedAxisKey reports whether a property of the axis of a channel passes
// to the Vega axis as it is: a property of the style of a part, or of the
// place of the labels or the title.
func passedAxisKey(k string) bool {
	switch k {
	case "labelOffset", "titleAnchor", "titlePadding", "titleX", "titleY", "titleAngle", "titleAlign",
		"titleBaseline", "tickSize":
		return true
	}
	for prefix, ends := range axisStyleKeys {
		if end, ok := strings.CutPrefix(k, prefix); ok && slices.Contains(ends, end) {
			return true
		}
	}
	return false
}

// valueAxisKey reports whether a property of the axis of a channel passes
// to the Vega axis only as a value, which guideValue reads.
func valueAxisKey(k string) bool {
	return passedAxisKey(k) || k == "labelBaseline" || k == "labelPadding" || k == "bandPosition"
}

// guideValue returns the value of a property of a guide as a value of
// Vega: a literal, or a signal for an expression of Vega-Lite, such as
// {"expr": "width / 2"}. A condition, or any other object, is not a value
// that the compiler passes on.
func guideValue(x any) (vega.Value, bool) {
	m, ok := x.(map[string]any)
	if !ok {
		return lit(x), true
	}
	if e, ok := m["expr"].(string); ok && len(m) == 1 {
		return vega.Value{Signal: e}, true
	}
	return vega.Value{}, false
}

// setExtra sets a property of a Vega axis that it holds with no field of
// its own.
func setExtra(a *vega.Axis, k string, v vega.Value) {
	if a.Extra == nil {
		a.Extra = vega.Extra{}
	}
	if v.Signal != "" {
		a.Extra[k] = lit(map[string]string{"signal": v.Signal}).JSON
		return
	}
	a.Extra[k] = v.JSON
}

// discretizing reports whether a scale type maps a continuous domain to
// discrete values.
func discretizing(typ string) bool {
	return typ == "quantize" || typ == "quantile" || typ == "threshold"
}

// unitOffset makes the range of the scale of an offset span one unit of
// time, when its channel of position has a time unit on a scale of time,
// which has no band. The bars of a unit sit at the band position of the
// channel, 0.5 by default, as vega-lite.js places them.
func (v *view) unitOffset(s *scaleComp, ch string) {
	p := v.scales[v.key(ch)]
	if s.offset != "" || p == nil || p.discrete || p.field == nil || len(p.field.units) == 0 {
		return
	}
	s.offset = "unit"
	s.unitWidth = unitWidth(v.sn(ch), p.field)
	s.unitBand = 0.5
	if p.field.def != nil {
		if b, ok := valueOf(p.field.def.BandPosition).(float64); ok {
			s.unitBand = b
		}
	}
}

// offsetRange returns the range of the scale of an offset: the band of the
// position, steps, the size of the view around its middle, or a share of a
// unit of time.
func offsetRange(s *scaleComp, band, size string) vega.ScaleRange {
	switch s.offset {
	case "step":
		return vega.ScaleRange{Step: lit(20)}
	case "center":
		return vega.ScaleRange{Values: []vega.Value{{Signal: "-" + size + "/2"}, {Signal: size + "/2"}}}
	case "unit":
		// The bars of a unit of time fill 80 percent of it, around the
		// place of their band, as vega-lite.js draws them.
		return vega.ScaleRange{Values: []vega.Value{
			{Signal: num(math.Round((s.unitBand-0.4)*1e9)/1e9) + " * (" + s.unitWidth + ")"},
			{Signal: num(math.Round((s.unitBand+0.4)*1e9)/1e9) + " * (" + s.unitWidth + ")"},
		}}
	}
	return vega.ScaleRange{Values: []vega.Value{lit(0), {Signal: band}}}
}

// addDatum adds a constant of the data to the domain of the scale, once.
// The constant can be a date time object, which == cannot compare.
func (s *scaleComp) addDatum(x any) {
	if !slices.ContainsFunc(s.datums, func(y any) bool { return reflect.DeepEqual(x, y) }) {
		s.datums = append(s.datums, x)
	}
}

// datumField returns the entry of the fields of a domain that holds a
// constant: a list of the value, or a signal for a date time object, as
// vega-lite.js writes them.
func datumField(x any) any {
	if m, ok := x.(map[string]any); ok && isDateTime(m) {
		return map[string]any{"signal": "[{data: " + dateTimeExpr(m) + "}]"}
	}
	return []any{x}
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

// unitMinStep returns the tickMinStep of an axis of a time unit: the
// length of the smallest of its units, times its step, as the difference
// of two times, as vega-lite.js writes it.
func unitMinStep(units []string, step float64) (vega.Value, bool) {
	// The place of each unit among the arguments of datetime, and how many
	// of that place one unit holds.
	places := map[string][2]int{
		"year": {0, 1}, "quarter": {1, 3}, "month": {1, 1}, "week": {2, 7}, "day": {2, 1}, "date": {2, 1},
		"dayofyear": {2, 1}, "hours": {3, 1}, "minutes": {4, 1}, "seconds": {5, 1}, "milliseconds": {6, 1},
	}
	best := [2]int{-1, 0}
	for _, u := range units {
		if p, ok := places[u]; ok && (p[0] > best[0] || (p[0] == best[0] && p[1] < best[1])) {
			best = p
		}
	}
	if best[0] < 0 {
		return vega.Value{}, false
	}
	if step <= 0 {
		step = 1
	}
	args := []int{2001, 0, 1, 0, 0, 0, 0}
	start := datetimeCall(args)
	args[best[0]] += best[1] * int(step)
	return vega.Value{Signal: datetimeCall(args) + " - " + start}, true
}

// datetimeCall writes a call of datetime with its arguments.
func datetimeCall(args []int) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = strconv.Itoa(a)
	}
	return "datetime(" + strings.Join(parts, ", ") + ")"
}
