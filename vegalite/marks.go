package vegalite

import (
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/vega"
)

// vegaMark is the Vega mark of each mark of Vega-Lite.
var vegaMark = map[string]string{
	"bar": "rect", "rect": "rect", "tick": "rect", "point": "symbol", "circle": "symbol",
	"square": "symbol", "line": "line", "area": "area", "rule": "rule", "text": "text",
	"arc": "arc", "trail": "trail", "image": "image", "geoshape": "shape",
}

// enc builds the encodings of a mark.
type enc map[string]any

// set sets a property to a value reference.
func (e enc) set(name string, ref map[string]any) {
	e[name] = ref
}

// markOf builds the Vega mark of the view, and registers its scales.
func (u *unit) markOf() (*vega.Mark, error) {
	typ, ok := vegaMark[u.mark]
	if !ok {
		return nil, u.c.issue(u.path+"/mark", "the mark %s", u.mark)
	}
	e := enc{}
	u.color(e)
	if u.mark != "geoshape" {
		u.position(e)
	}
	u.others(e)
	for k, v := range u.props {
		switch k {
		case "color", "fill", "stroke", "filled", "orient", "invalid", "type", "style", "tooltip", "point", "line", "clip", "thickness", "binSpacing", "cornerRadiusEnd", "aria", "width", "height", "x", "y", "x2", "y2", "size", "radius", "theta", "theta2", "radius2", "outerRadius", "innerRadius":
			continue
		}
		if _, set := e[k]; !set {
			e.set(k, propRef(v))
		}
	}
	m := &vega.Mark{Type: typ, Name: u.v.prefix + "marks", From: &vega.From{Data: u.c.name(u.data)}}
	style := u.mark
	if u.mark == "circle" || u.mark == "square" {
		style = u.mark
	}
	styles := []string{style}
	switch st := u.props["style"].(type) {
	case string:
		styles = append(styles, st)
	case []any:
		for _, x := range st {
			styles = append(styles, expr.String(x))
		}
	}
	m.Style = vega.List(styles...)
	if c, ok := u.props["clip"].(bool); ok && c {
		m.Clip = lit(true)
	}
	switch u.mark {
	case "line", "trail":
		field := "x"
		if u.orient == "horizontal" && u.ch["x"] != nil && u.ch["x"].continuous() {
			field = "y"
		}
		if o := u.ch["order"]; o != nil && o.isField() {
			field = "datum[" + strconv.Quote(o.out) + "]"
		}
		m.Sort = &vega.Sort{Field: lit(field)}
	case "area":
		m.Sort = &vega.Sort{Field: lit("x")}
		if u.orient == "horizontal" {
			m.Sort = &vega.Sort{Field: lit("y")}
		}
	}
	update := vega.Props{}
	for k, v := range e {
		update[k] = prod(v)
	}
	m.Encode = vega.Encode{"update": update}
	u.geoEncode(m)
	return m, nil
}

// color sets the fill or the stroke of a mark from the channel color, or
// from the color of the mark.
func (u *unit) color(e enc) {
	filled := true
	switch u.mark {
	case "point", "line", "rule", "trail":
		filled = false
	}
	if u.mark == "trail" {
		filled = true
	}
	if u.mark == "geoshape" && u.source != nil && u.source.graticule {
		// The lines of a graticule have no inside.
		filled = false
	}
	if f, ok := u.props["filled"].(bool); ok {
		filled = f
	}
	prop := "stroke"
	if filled {
		prop = "fill"
	}
	ref := map[string]any{"value": defaultColor}
	if c, ok := u.props["color"].(string); ok {
		ref = map[string]any{"value": c}
	}
	if c := u.channelRef("color"); c != nil {
		ref = c
	}
	e.set(prop, ref)
	if (u.mark == "point" || u.mark == "geoshape") && !filled {
		e.set("fill", map[string]any{"value": "transparent"})
	}
	if u.mark == "point" || u.mark == "circle" || u.mark == "square" {
		if _, ok := u.ch["opacity"]; !ok {
			e.set("opacity", map[string]any{"value": 0.7})
		}
	}
	for _, name := range []string{"fill", "stroke"} {
		if c := u.channelRef(name); c != nil {
			e.set(name, c)
		} else if v, ok := u.props[name]; ok {
			e.set(name, map[string]any{"value": v})
		}
	}
	if u.mark == "text" {
		if _, ok := e["fill"]; !ok || u.ch["color"] == nil && u.props["color"] == nil {
			e.set("fill", map[string]any{"value": "black"})
		}
	}
}

// channelRef returns the value reference of a channel through its scale,
// or nil if the view does not have the channel.
func (u *unit) channelRef(name string) map[string]any {
	d := u.ch[name]
	if d == nil {
		return nil
	}
	scaleName := name
	if name == "fill" || name == "stroke" {
		scaleName = name
	}
	var base map[string]any
	switch {
	case d.hasValue:
		if m, ok := d.value.(map[string]any); ok && m["expr"] != nil {
			base = map[string]any{"signal": m["expr"]}
		} else {
			base = map[string]any{"value": d.value}
		}
	case d.hasDatum:
		if u.v.addScale(u, scaleName, d, nil) {
			if sc := u.v.scales[u.v.key(scaleName)]; sc != nil && !slices.Contains(sc.datums, d.datum) {
				sc.datums = append(sc.datums, d.datum)
			}
			base = map[string]any{"scale": u.v.sn(scaleName), "value": d.datum}
		} else {
			base = map[string]any{"value": d.datum}
		}
	case d.isField():
		fields := []string{d.out}
		if d.units != nil && d.typ == temporal && u.spans(d) {
			fields = append(fields, d.outEnd)
		}
		if u.v.addScale(u, scaleName, d, fields) {
			base = map[string]any{"scale": u.v.sn(scaleName), "field": d.out}
		} else {
			base = map[string]any{"field": d.out}
		}
	default:
		return nil
	}
	if len(d.cond) == 0 {
		return base
	}
	return u.conditional(d, base)
}

// conditional returns a list of rules for a channel with a condition: the
// condition first, then the channel.
func (u *unit) conditional(d *channelDef, base map[string]any) map[string]any {
	c := d.cond[0]
	rule := map[string]any{"test": c.test}
	switch {
	case c.hasValue:
		rule["value"] = c.value
	case c.field != "":
		rule["field"] = c.field
	}
	return map[string]any{"__rules": []any{rule, base}}
}

// position sets the position and the size of a mark from its positional
// channels.
func (u *unit) position(e enc) {
	if u.mark == "arc" {
		u.arc(e)
		return
	}
	u.axis(e, "x", "width")
	u.axis(e, "y", "height")
}

// axis sets the position along one axis: x and x2, or y and y2, with the
// width or the height of a band.
func (u *unit) axis(e enc, ch, size string) {
	d := u.ch[ch]
	d2 := u.ch[ch+"2"]
	p, p2, pc := ch, ch+"2", ch+"c"
	if d == nil {
		if r := u.offsetRef(ch, false); r != nil {
			// The offset places the marks around the middle of the view.
			mid := "width"
			if ch == "y" {
				mid = "height"
			}
			e.set(p, map[string]any{"signal": mid, "mult": 0.5, "offset": r})
			if u.mark == "bar" || u.mark == "rect" || u.mark == "tick" {
				e.set(size, map[string]any{"signal": "max(0.25, bandwidth('" + u.v.sn(ch+"Offset") + "'))"})
			}
			return
		}
		switch {
		case ch == "x" && (u.mark == "bar" || u.mark == "rect" || u.mark == "rule" && u.ch["y"] != nil && u.ch["x2"] == nil):
			if u.mark == "rule" {
				e.set(p, map[string]any{"value": 0})
				e.set(p2, map[string]any{"field": map[string]any{"group": "width"}})
				return
			}
			e.set(p, map[string]any{"value": 0})
			e.set(size, map[string]any{"field": map[string]any{"group": "width"}})
		case ch == "y" && (u.mark == "bar" || u.mark == "rect" || u.mark == "rule" && u.ch["x"] != nil && u.ch["y2"] == nil):
			if u.mark == "rule" {
				e.set(p, map[string]any{"value": 0})
				e.set(p2, map[string]any{"field": map[string]any{"group": "height"}})
				return
			}
			e.set(p, map[string]any{"value": 0})
			e.set(size, map[string]any{"field": map[string]any{"group": "height"}})
		case ch == "x":
			e.set(p, map[string]any{"signal": "width", "mult": 0.5})
		default:
			e.set(p, map[string]any{"signal": "height", "mult": 0.5})
		}
		return
	}
	if d.hasValue {
		e.set(p, map[string]any{"value": d.value})
		if d2 != nil && d2.hasValue {
			e.set(p2, map[string]any{"value": d2.value})
		}
		return
	}
	discrete := !d.continuous() || (d.units != nil && d.typ != temporal)
	if d.bin != nil {
		u.v.addScale(u, ch, d, []string{d.out, d.outEnd})
		e.set(p, map[string]any{"scale": u.v.sn(ch), "field": d.out, "offset": 0.5})
		e.set(p2, map[string]any{"scale": u.v.sn(ch), "field": d.outEnd, "offset": -0.5})
		if u.mark != "bar" && u.mark != "rect" {
			e.set(p, map[string]any{"signal": "scale(\"" + u.v.sn(ch) + "\", 0.5 * " + fieldRef(d.out) + " + 0.5 * " + fieldRef(d.outEnd) + ")"})
			delete(e, p2)
		}
		return
	}
	if o := u.ch[ch+"Offset"]; o != nil && o.isField() && d.units != nil && d.typ == temporal && u.spans(d) {
		// Bars side by side within the interval of a time unit. A bar at
		// the start of its interval reads the domain from the shifted
		// interval.
		fields := []string{d.out, d.outEnd}
		if u.shifted(d) {
			fields = []string{d.out + "_offsetted_rect_start", d.out + "_offsetted_rect_end"}
		}
		u.v.addScale(u, ch, d, fields)
		r := map[string]any{"scale": u.v.sn(ch), "field": d.out}
		band := u.v.sn(ch)
		if u.v.addScale(u, ch+"Offset", o, []string{o.out}) {
			r["offset"] = map[string]any{"scale": u.v.sn(ch + "Offset"), "field": o.out, "band": 0.5}
			band = u.v.sn(ch + "Offset")
		}
		e.set(pc, r)
		e.set(size, map[string]any{"signal": "max(0.25, bandwidth('" + band + "'))"})
		return
	}
	if d.units != nil && d.typ == temporal && u.spans(d) && u.mark != "bar" && u.mark != "rect" {
		// A band position places the mark at a share of the interval.
		u.v.addScale(u, ch, d, []string{d.out, d.outEnd})
		p0 := 0.5
		if b, ok := valueOf(d.def.BandPosition).(float64); ok {
			p0 = b
		}
		k := func(x float64) string { return strconv.FormatFloat(x, 'f', -1, 64) }
		e.set(p, map[string]any{"signal": "scale(\"" + u.v.sn(ch) + "\", " + k(1-p0) + " * " + fieldRef(d.out) + " + " + k(p0) + " * " + fieldRef(d.outEnd) + ")"})
		u.pathDefined(e)
		return
	}
	if d.units != nil && d.typ == temporal && u.spans(d) {
		start, end := d.out, d.outEnd
		if u.shifted(d) {
			start, end = d.out+"_offsetted_rect_start", d.out+"_offsetted_rect_end"
		}
		u.v.addScale(u, ch, d, []string{start, end})
		e.set(p, map[string]any{"scale": u.v.sn(ch), "field": start, "offset": 0.5})
		e.set(p2, map[string]any{"scale": u.v.sn(ch), "field": end, "offset": -0.5})
		return
	}
	if u.stack == ch {
		u.v.addScale(u, ch, d, []string{d.stackName("start"), d.stackName("end")})
		if sc := u.v.scales[u.v.key(ch)]; sc != nil && u.offset == "normalize" {
			sc.normalized = true
		}
		e.set(p, map[string]any{"scale": u.v.sn(ch), "field": d.stackName("end")})
		e.set(p2, map[string]any{"scale": u.v.sn(ch), "field": d.stackName("start")})
		return
	}
	fields := []string{d.out}
	if d2 != nil && d2.isField() {
		fields = append(fields, d2.out)
	}
	hasScale := u.v.addScale(u, ch, d, fields)
	ref := func(field string) map[string]any {
		if hasScale {
			return map[string]any{"scale": u.v.sn(ch), "field": field}
		}
		return map[string]any{"field": field}
	}
	if d.hasDatum {
		ref = func(string) map[string]any { return map[string]any{"scale": u.v.sn(ch), "value": d.datum} }
	}
	switch {
	case discrete && (u.mark == "bar" || u.mark == "rect" || u.mark == "tick"):
		var sz any
		if c := u.ch["size"]; c != nil && c.hasValue {
			sz = c.value
		} else if v, ok := u.props["size"]; ok {
			sz = v
		} else if u.mark == "tick" {
			sz = 5.0
		}
		if sz != nil && u.mark != "rect" {
			r := ref(d.out)
			if u.mark == "tick" {
				if o := u.offsetRef(ch, false); o != nil {
					// Ticks fill the band of their offset.
					r["offset"] = o
					e.set(p, r)
					e.set(size, map[string]any{"signal": "max(0.25, bandwidth('" + u.v.sn(ch+"Offset") + "'))"})
					return
				}
			}
			if o := u.offsetRef(ch, true); o != nil {
				r["offset"] = o
			} else {
				r["band"] = 0.5
			}
			e.set(pc, r)
			e.set(size, map[string]any{"value": sz})
			return
		}
		r := ref(d.out)
		band := u.v.sn(ch)
		if o := u.offsetRef(ch, false); o != nil {
			// An offset places the bar in a sub-band of the band.
			r["offset"] = o
			band = u.v.sn(ch + "Offset")
		}
		e.set(p, r)
		e.set(size, map[string]any{"signal": "max(0.25, bandwidth('" + band + "'))"})
	case discrete && u.v.scaleType(ch) == "band":
		r := ref(d.out)
		if o := u.offsetRef(ch, u.v.scaleType(ch+"Offset") != "point"); o != nil {
			r["offset"] = o
		} else {
			r["band"] = 0.5
		}
		e.set(p, r)
	case u.mark == "tick" && u.measure() == ch:
		e.set(pc, ref(d.out))
		thick := 1.0
		if v, ok := u.props["thickness"].(float64); ok {
			thick = v
		}
		e.set(size, map[string]any{"value": thick})
		return
	default:
		e.set(p, ref(d.out))
		switch {
		case d2 != nil && d2.isField():
			e.set(p2, ref(d2.out))
		case d2 != nil && d2.hasValue:
			e.set(p2, map[string]any{"value": d2.value})
		case (u.mark == "bar" || u.mark == "area") && u.measure() == ch && hasScale:
			e.set(p2, map[string]any{"scale": u.v.sn(ch), "value": 0})
		case u.mark == "rule" && u.ch[ch+"2"] == nil:
		}
	}
	u.pathDefined(e)
	if u.mark == "area" && u.measure() == ch {
		e.set("orient", map[string]any{"value": u.orient})
	}
}

// pathDefined sets the property defined of a path to the test that its
// continuous positions are valid numbers.
func (u *unit) pathDefined(e enc) {
	if u.mark == "line" || u.mark == "area" || u.mark == "trail" {
		var parts []string
		for _, name := range []string{"x", "y"} {
			if dd := u.ch[name]; dd != nil && dd.isField() && dd.continuous() {
				f := dd.out
				if u.stack == name {
					f = dd.stackName("end")
				}
				parts = append(parts, "isValid("+fieldRef(f)+") && isFinite(+"+fieldRef(f)+")")
			}
		}
		if len(parts) > 0 {
			e.set("defined", map[string]any{"signal": joinAnd(parts)})
		}
	}
}

// joinAnd joins expressions with &&.
func joinAnd(parts []string) string {
	return strings.Join(parts, " && ")
}

// arc sets the angles and the radii of an arc from theta and radius.
func (u *unit) arc(e enc) {
	e.set("x", map[string]any{"signal": "width", "mult": 0.5})
	e.set("y", map[string]any{"signal": "height", "mult": 0.5})
	outer := map[string]any{"signal": "min(width,height)/2"}
	inner := map[string]any{"value": 0}
	if r := u.ch["radius"]; r != nil && r.isField() {
		if u.stack == "radius" {
			// Stacked rings: each spans from its start to its end.
			u.v.addScale(u, "radius", r, []string{r.stackName("start"), r.stackName("end")})
			if sc := u.v.scales[u.v.key("radius")]; sc != nil && sc.props["type"] == nil {
				sc.typ = "linear"
			}
			outer = map[string]any{"scale": u.v.sn("radius"), "field": r.stackName("end")}
			inner = map[string]any{"scale": u.v.sn("radius"), "field": r.stackName("start")}
		} else {
			u.v.addScale(u, "radius", r, []string{r.out})
			outer = map[string]any{"scale": u.v.sn("radius"), "field": r.out}
		}
	}
	if v, ok := u.props["outerRadius"]; ok {
		outer = map[string]any{"value": v}
	}
	e.set("outerRadius", outer)
	if v, ok := u.props["innerRadius"]; ok {
		inner = map[string]any{"value": v}
	}
	e.set("innerRadius", inner)
	t := u.ch["theta"]
	switch {
	case t == nil:
		e.set("startAngle", map[string]any{"value": 0})
		e.set("endAngle", map[string]any{"value": 6.283185307179586})
	case u.stack == "theta":
		u.v.addScale(u, "theta", t, []string{t.stackName("start"), t.stackName("end")})
		e.set("startAngle", map[string]any{"scale": u.v.sn("theta"), "field": t.stackName("end")})
		e.set("endAngle", map[string]any{"scale": u.v.sn("theta"), "field": t.stackName("start")})
	case t.isField():
		u.v.addScale(u, "theta", t, []string{t.out})
		e.set("startAngle", map[string]any{"scale": u.v.sn("theta"), "field": t.out})
		if u.v.scaleType("theta") == "band" {
			e.set("endAngle", map[string]any{"scale": u.v.sn("theta"), "field": t.out, "band": 1})
		}
	}
}

// others sets the channels that are not positions or colors: size, shape,
// opacity, text and the like.
func (u *unit) others(e enc) {
	for _, name := range []string{"opacity", "fillOpacity", "strokeOpacity", "strokeWidth", "strokeDash", "angle", "shape"} {
		if r := u.channelRef(name); r != nil {
			e.set(name, r)
		}
	}
	if r := u.channelRef("size"); r != nil {
		switch u.mark {
		case "text":
			e.set("fontSize", r)
		case "point", "circle", "square", "trail":
			e.set("size", r)
		case "line", "rule":
			e.set("strokeWidth", r)
		}
	} else if s, ok := u.props["size"]; ok {
		switch u.mark {
		case "text":
			e.set("fontSize", map[string]any{"value": s})
		case "point", "circle", "square":
			e.set("size", map[string]any{"value": s})
		}
	}
	switch u.mark {
	case "circle", "square":
		e.set("shape", map[string]any{"value": u.mark})
	case "point":
		if s, ok := u.props["shape"]; ok {
			e.set("shape", map[string]any{"value": s})
		}
	}
	if d := u.ch["text"]; d != nil {
		switch {
		case d.hasValue:
			e.set("text", map[string]any{"value": d.value})
		case d.isField():
			var ref map[string]any
			switch d.typ {
			case quantitative:
				spec := ""
				if !d.def.Format.IsZero() {
					spec = str(d.def.Format)
				}
				ref = map[string]any{"signal": "format(" + fieldRef(d.out) + ", " + strconv.Quote(spec) + ")"}
			case temporal:
				spec := "%b %d, %Y"
				if !d.def.Format.IsZero() {
					spec = str(d.def.Format)
				}
				ref = map[string]any{"signal": "timeFormat(" + fieldRef(d.out) + ", " + strconv.Quote(spec) + ")"}
			default:
				ref = map[string]any{"signal": "isValid(" + fieldRef(d.out) + ") ? " + fieldRef(d.out) + " : \"\"+" + fieldRef(d.out)}
			}
			if len(d.cond) > 0 {
				ref = u.conditional(d, ref)
			}
			e.set("text", ref)
		}
	} else if t, ok := u.props["text"]; ok {
		e.set("text", map[string]any{"value": t})
	}
	if d := u.ch["url"]; d != nil && d.isField() {
		e.set("url", map[string]any{"field": d.out})
	}
	_ = expr.String
}

// offsetRef adds the scale of the offset of a channel of position, and returns
// the reference that moves a mark by it, in the middle of its band when
// band is true. It returns nil when the channel has no offset.
func (u *unit) offsetRef(ch string, band bool) map[string]any {
	o := u.ch[ch+"Offset"]
	if o == nil || !o.isField() && !o.hasDatum {
		return nil
	}
	var fields []string
	if o.isField() {
		fields = []string{o.out}
	}
	if !u.v.addScale(u, ch+"Offset", o, fields) {
		return nil
	}
	if s := u.v.scales[u.v.key(ch+"Offset")]; s != nil {
		if o.hasDatum && !slices.Contains(s.datums, o.datum) {
			s.datums = append(s.datums, o.datum)
		}
		if s.offset == "" {
			switch {
			case u.ch[ch] == nil:
				s.offset = "center"
			case u.mark != "bar" && u.mark != "rect" || o.hasDatum:
				s.offset = "step"
			}
		}
	}
	r := map[string]any{"scale": u.v.sn(ch + "Offset"), "field": o.out}
	if o.hasDatum {
		r = map[string]any{"scale": u.v.sn(ch + "Offset"), "value": o.datum}
	}
	if band {
		r["band"] = 0.5
	}
	return r
}
