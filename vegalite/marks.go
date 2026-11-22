package vegalite

import (
	"encoding/json/v2"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/theme"
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
		if err := u.position(e); err != nil {
			return nil, err
		}
	}
	u.others(e)
	u.cornerEnd(e)
	if err := u.markOffsets(e); err != nil {
		return nil, err
	}
	for k, v := range u.props {
		switch k {
		case "color", "fill", "stroke", "filled", "orient", "invalid", "type", "style", "tooltip", "point", "line", "clip", "thickness", "binSpacing", "cornerRadiusEnd", "aria", "width", "height", "x", "y", "x2", "y2", "size", "radius", "theta", "theta2", "radius2", "outerRadius", "innerRadius", "xOffset", "yOffset", "x2Offset", "y2Offset", "minBandSize":
			continue
		case "radiusOffset", "thetaOffset":
			// polar adds them to the place of a mark that is not an arc.
			if u.mark != "arc" {
				continue
			}
		}
		if _, set := e[k]; !set {
			e.set(k, propRef(v))
		}
	}
	u.textAnchor(e)
	m := &vega.Mark{Type: typ, Name: u.v.prefix + "marks", From: &vega.From{Data: u.c.name(u.data)}}
	style := u.mark
	if u.mark == "circle" || u.mark == "square" {
		style = u.mark
	}
	styles := []string{style}
	// The color of the theme is a default, as the config of a kind of mark
	// is, so a style that the spec names comes after it and wins.
	if u.paintStyle != "" {
		styles = append(styles, u.paintStyle)
	}
	switch st := u.props["style"].(type) {
	case string:
		styles = append(styles, st)
	case []any:
		for _, x := range st {
			styles = append(styles, expr.String(x))
		}
	}
	// The styles of the look of zuzu, which the config of the theme gives
	// (D14): the round end of a bar, and the ring of a point.
	switch u.mark {
	case "bar":
		styles = append(styles, theme.StyleBar+u.orient)
	case "point", "circle", "square":
		styles = append(styles, theme.StylePoint)
	case "area":
		// Vega-Lite stacks an area of one series too, so an area is a
		// layer of a stack when it splits by a channel. An area with an
		// end of its own, or whose scale leaves out zero, does not run
		// from zero, and is a band (D48 part 3).
		stacked := u.stack != "" && (u.ch["color"] != nil || u.ch["fill"] != nil || len(u.detail) > 0)
		switch {
		case u.ch["y2"] != nil || u.ch["x2"] != nil, !stacked && u.zeroOff(u.measure()):
			styles = append(styles, theme.StyleAreaRanged)
		case stacked:
			styles = append(styles, theme.StyleAreaStacked)
		default:
			styles = append(styles, theme.StyleArea)
		}
	}
	// A rule with one place across the whole plot is a reference line,
	// unless the spec gives it a color, a width or a dash of its own, and
	// so says how it looks (docs/STYLE.md).
	if u.mark == "rule" && (u.ch["x"] == nil) != (u.ch["y"] == nil) && u.ch["x2"] == nil && u.ch["y2"] == nil && !u.styled() {
		styles = append(styles, theme.StyleReference)
	}
	// A line or an area along x draws as a monotone curve, which keeps the
	// order of its values (docs/STYLE.md).
	if (u.mark == "line" || u.mark == "area") && u.orient != "horizontal" && u.ch["order"] == nil {
		styles = append(styles, theme.StyleCurve)
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
	for _, p := range u.late {
		ref, ok := e[p].(map[string]any)
		if !ok {
			continue
		}
		v, r := u.v, maps.Clone(ref)
		v.late = append(v.late, func() {
			if v.scaleType(p) == "band" {
				r["band"] = 0.5
				update[p] = prod(r)
			}
		})
	}
	for _, p := range u.lateOffsets {
		ref, ok := e[p].(map[string]any)
		if !ok {
			continue
		}
		off, ok := ref["offset"].(map[string]any)
		if !ok {
			continue
		}
		v, r, off := u.v, maps.Clone(ref), maps.Clone(off)
		r["offset"] = off
		v.late = append(v.late, func() {
			if s := v.scales[v.key(p+"Offset")]; s != nil && s.discrete && s.typ != "point" {
				off["band"] = 0.5
				update[p] = prod(r)
			}
		})
	}
	u.geoEncode(m)
	return m, nil
}

// textAnchors are the properties that place a text on its point, with the
// value that Vega-Lite gives each when nothing sets it. A text is centered
// on its point.
var textAnchors = [...]struct{ prop, value string }{{"align", "center"}, {"baseline", "middle"}}

// textAnchor sets the align and the baseline of a text mark, where the
// mark does not set them. If a style of the config that the mark names sets
// one, the mark writes nothing, because the style passes on to the Vega
// config. If the config of the text, or else of every mark, sets one, the
// mark writes that value, because that config does not pass on to the Vega
// config. Else the mark writes the value of Vega-Lite.
func (u *unit) textAnchor(e enc) {
	if u.mark != "text" {
		return
	}
	cfg := u.c.cfg
	if cfg == nil {
		cfg = &config{}
	}
	styles := []string{u.mark}
	switch st := u.props["style"].(type) {
	case string:
		styles = append(styles, st)
	case []any:
		for _, x := range st {
			styles = append(styles, expr.String(x))
		}
	}
	for _, a := range textAnchors {
		if _, set := e[a.prop]; set {
			continue
		}
		if slices.ContainsFunc(styles, func(name string) bool {
			_, ok := cfg.styles[name][a.prop]
			return ok
		}) {
			continue
		}
		v, ok := cfg.marks[u.mark][a.prop]
		if !ok {
			v, ok = cfg.mark[a.prop]
		}
		if !ok {
			v = a.value
		}
		e.set(a.prop, propRef(v))
	}
}

// color sets the fill or the stroke of a mark from the channel color, or
// from the color of the mark.
func (u *unit) color(e enc) {
	filled := true
	switch u.mark {
	case "line", "rule", "trail":
		// A point is filled in the look of zuzu (docs/STYLE.md), and
		// "filled": false keeps it hollow.
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
	// A mark with no color of its own takes the color of the theme, through
	// a style of its config (D14).
	var ref map[string]any
	switch c := u.props["color"].(type) {
	case string:
		ref = map[string]any{"value": c}
	case map[string]any:
		// A gradient, such as {"gradient": "linear", "stops": [...]}, is
		// the color of the mark, and an expression gives it as a signal.
		if c["gradient"] != nil || c["expr"] != nil {
			ref = propRef(c)
		}
	}
	if c := u.channelRef("color"); c != nil {
		if d := u.ch["color"]; d != nil && !d.hasValue && !d.hasDatum && !d.isField() {
			// A color with conditions alone falls back to the color of the
			// mark, as vega-lite.js writes it. Otherwise a mark whose
			// conditions do not hold has no color, because the encode
			// sets the property and the style of the config does not
			// apply.
			c = u.withFallback(c, ref)
		}
		ref = c
	}
	switch {
	case ref != nil:
		e.set(prop, ref)
	case u.mark == "text":
		u.paintStyle = theme.StyleText
	case filled:
		u.paintStyle = theme.StyleFill
	default:
		u.paintStyle = theme.StyleStroke
	}
	if (u.mark == "point" || u.mark == "geoshape") && !filled {
		e.set("fill", map[string]any{"value": "transparent"})
	}
	if (u.mark == "point" || u.mark == "circle" || u.mark == "square") && u.ch["opacity"] == nil {
		u.reducedOpacity(e)
	}
	for _, name := range []string{"fill", "stroke"} {
		if c := u.channelRef(name); c != nil {
			e.set(name, c)
		} else if v, ok := u.props[name]; ok {
			e.set(name, map[string]any{"value": v})
		}
	}
}

// keepsInvalid reports whether the view keeps the invalid values of its
// data, with "invalid": null on the mark or the config of the marks.
func (u *unit) keepsInvalid() bool {
	if v, ok := u.props["invalid"]; ok {
		return v == nil
	}
	return u.c.cfg != nil && u.c.cfg.keepInvalid
}

// invalidAtZero returns the reference of a position of a field on a
// continuous scale, ref, with a rule before it that puts an invalid value
// of the field at zero, as vega-lite.js writes it when the view keeps
// invalid values: at zero of the scale when its domain holds zero, or else
// at the bottom for y and at the left for x. A line, an area and a trail
// break at an invalid value instead, and a view that filters out invalid
// values needs no rule.
func (u *unit) invalidAtZero(ch, field string, hasScale bool, ref map[string]any) map[string]any {
	switch u.mark {
	case "line", "area", "trail":
		return ref
	}
	if !hasScale || !u.keepsInvalid() || ref["field"] == nil {
		return ref
	}
	f := fieldRef(field)
	rule := map[string]any{"test": "!isValid(" + f + ") || !isFinite(+" + f + ")"}
	switch {
	case u.zeroInDomain(ch):
		rule["scale"], rule["value"] = u.v.sn(ch), 0
	case ch == "y":
		rule["field"] = map[string]any{"group": "height"}
	default:
		rule["value"] = 0
	}
	return map[string]any{rulesKey: []any{rule, ref}}
}

// zeroInDomain reports whether the domain of the scale of a channel
// surely holds zero: the scale and the config do not turn zero off, and
// the spec gives no domain or end of a domain.
func (u *unit) zeroInDomain(ch string) bool {
	sc := u.v.scales[u.v.key(ch)]
	if sc == nil {
		return true
	}
	for _, k := range []string{"domain", "domainMin", "domainMax"} {
		if sc.props[k] != nil {
			return false
		}
	}
	return !u.zeroOff(ch)
}

// zeroOff reports whether the scale of a channel or the config turns zero
// off, so that the scale can leave zero out.
func (u *unit) zeroOff(ch string) bool {
	if sc := u.v.scales[u.v.key(ch)]; sc != nil {
		if z, ok := sc.props["zero"].(bool); ok {
			return !z
		}
	}
	return u.c.cfg != nil && u.c.cfg.zero != nil && !*u.c.cfg.zero
}

// reducedOpacity gives a point the opacity of 0.7 of vega-lite.js, where
// neither the mark nor the config of its kind or of every mark sets an
// opacity or a fill opacity. The opacity of the mark wins, and the mark
// writes it itself, and so does the opacity of the config, which the
// compiler does not pass on to the Vega config (D48 part 5).
func (u *unit) reducedOpacity(e enc) {
	var marks, mark map[string]any
	if cfg := u.c.cfg; cfg != nil {
		marks, mark = cfg.marks[u.mark], cfg.mark
	}
	for _, k := range []string{"opacity", "fillOpacity"} {
		if u.props[k] != nil {
			return
		}
	}
	for _, k := range []string{"opacity", "fillOpacity"} {
		if v := marks[k]; v != nil {
			e.set(k, propRef(v))
			return
		}
		if v := mark[k]; v != nil {
			e.set(k, propRef(v))
			return
		}
	}
	e.set("opacity", map[string]any{"value": 0.7})
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
		base = d.datumRef()
		if u.v.addScale(u, scaleName, d, nil) {
			if sc := u.v.scales[u.v.key(scaleName)]; sc != nil {
				sc.addDatum(d.datum)
			}
			base["scale"] = u.v.sn(scaleName)
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
	case len(d.cond) > 0:
		// A channel with conditions alone, such as an opacity that a test
		// sets, sets the property only where a condition holds, as
		// vega-lite.js writes it.
		return u.conditional(d, nil)
	default:
		return nil
	}
	if len(d.cond) == 0 {
		return base
	}
	return u.conditional(d, base)
}

// withFallback adds to the rules of a channel with conditions alone the
// color of the mark as the last rule: own, the color that the mark gives,
// or else the color of the config of the mark, or else the color of a mark
// of the theme.
func (u *unit) withFallback(rules, own map[string]any) map[string]any {
	list, _ := rules[rulesKey].([]any)
	fallback := own
	if fallback == nil && u.c.cfg != nil {
		for _, m := range []map[string]any{u.c.cfg.marks[u.mark], u.c.cfg.mark} {
			if c, ok := m["color"].(string); ok {
				fallback = map[string]any{"value": c}
				break
			}
		}
	}
	if fallback == nil {
		u.c.markColor = true
		fallback = map[string]any{"scale": MarkColorScale, "value": MarkColorScale}
	}
	return map[string]any{rulesKey: append(slices.Clone(list), fallback)}
}

// conditional returns a list of rules for a channel with conditions: each
// condition in the order of the spec, then the channel. The first
// condition that holds gives the value.
func (u *unit) conditional(d *channelDef, base map[string]any) map[string]any {
	rules := make([]any, 0, len(d.cond)+1)
	for _, c := range d.cond {
		rule := map[string]any{"test": c.test}
		switch {
		case c.hasValue:
			rule["value"] = c.value
			if m, ok := c.value.(map[string]any); ok && m["expr"] != nil {
				// A value {"expr": ...} is the value of the expression.
				delete(rule, "value")
				rule["signal"] = m["expr"]
			}
		case c.field != "":
			rule["field"] = c.field
		}
		rules = append(rules, rule)
	}
	// With no field and no value, a condition that does not hold leaves
	// the property unset.
	if base != nil {
		rules = append(rules, base)
	}
	return map[string]any{rulesKey: rules}
}

// position sets the position and the size of a mark from its positional
// channels.
func (u *unit) position(e enc) error {
	if u.mark == "arc" {
		u.arc(e)
		return nil
	}
	if err := u.axis(e, "x", "width"); err != nil {
		return err
	}
	if err := u.axis(e, "y", "height"); err != nil {
		return err
	}
	u.polar(e)
	return nil
}

// polar sets the theta and the radius of a mark that is not an arc, such as
// the label of a slice. They move the mark from its x and y, which are the
// middle of the view when the spec gives no x and no y. A stack, a bin or a
// band places the mark at the middle of its interval. The properties
// thetaOffset and radiusOffset of the mark add to each.
func (u *unit) polar(e enc) {
	if r := u.polarRef("theta"); r != nil {
		e.set("theta", r)
	}
	if r := u.polarRef("radius"); r != nil {
		e.set("radius", r)
	}
}

// polarRef returns the reference of the channel theta or radius of a mark
// that is not an arc, or nil if the mark has neither the channel nor the
// property.
func (u *unit) polarRef(ch string) map[string]any {
	var ref map[string]any
	switch d := u.ch[ch]; {
	case d == nil:
		if v, ok := u.props[ch]; ok {
			ref = map[string]any{"value": v}
		}
	case d.hasValue:
		ref = propRef(d.value)
	case u.stack == ch:
		u.v.addScale(u, ch, d, []string{d.stackName("start"), d.stackName("end")})
		ref = u.midRef(ch, d.stackName("start"), d.stackName("end"))
	case d.bin != nil:
		u.v.addScale(u, ch, d, []string{d.out, d.outEnd})
		ref = u.midRef(ch, d.out, d.outEnd)
	case d.isField():
		u.v.addScale(u, ch, d, []string{d.out})
		ref = map[string]any{"scale": u.v.sn(ch), "field": d.out}
		if u.v.scaleType(ch) == "band" {
			ref["band"] = 0.5
		}
	}
	if ref == nil {
		return nil
	}
	if off, ok := u.props[ch+"Offset"]; ok {
		if sig, ok := ref["signal"].(string); ok {
			ref["signal"] = sig + " + " + expr.String(off)
		} else {
			ref["offset"] = off
		}
	}
	return ref
}

// axis sets the position along one axis: x and x2, or y and y2, with the
// width or the height of a band.
func (u *unit) axis(e enc, ch, size string) error {
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
				e.set(size, map[string]any{"signal": "max(" + u.minBandSize() + ", bandwidth('" + u.v.sn(ch+"Offset") + "'))"})
			}
			return nil
		}
		if v, ok := u.props[ch]; ok {
			// A position that the mark gives, such as "x": 5, places
			// each mark there, as vega-lite.js writes it, and its end
			// too when the mark gives one.
			e.set(p, posRef(v))
			if v2, ok := u.props[p2]; ok {
				e.set(p2, posRef(v2))
			}
			return nil
		}
		i := 0
		if ch == "y" {
			i = 1
		}
		switch {
		case u.mark == "tick" || u.mark == "bar" && (ch == "y" && u.orient == "horizontal" || ch == "x" && u.orient == "vertical"):
			// A tick, or a bar across its band, with no channel on this
			// axis is centered in the view. It takes the size that the spec
			// gives, or else a share of the view: the view less the inner
			// padding of the bands of a bar, or three quarters of the view
			// for a tick.
			e.set(pc, map[string]any{"signal": u.v.sizeName(i), "mult": 0.5})
			if sz, _ := u.markSize(size); sz != nil {
				if _, ok := relBand(sz); !ok {
					e.set(size, propRef(sz))
					return nil
				}
			}
			share := 0.75
			if u.mark == "bar" {
				share = 1 - u.c.cfg.barBandPaddingInner()
			}
			e.set(size, map[string]any{"signal": num(share) + " * " + u.v.sizeName(i)})
		case ch == "x" && (u.mark == "bar" || u.mark == "rect" || u.mark == "rule" && u.ch["y"] != nil && u.ch["x2"] == nil && u.ch["y2"] == nil):
			if u.mark == "rule" {
				e.set(p, map[string]any{"value": 0})
				e.set(p2, map[string]any{"field": map[string]any{"group": "width"}})
				return nil
			}
			e.set(p, map[string]any{"value": 0})
			e.set(size, map[string]any{"field": map[string]any{"group": "width"}})
		case ch == "y" && (u.mark == "bar" || u.mark == "rect" || u.mark == "rule" && u.ch["x"] != nil && u.ch["y2"] == nil && u.ch["x2"] == nil):
			if u.mark == "rule" {
				e.set(p, map[string]any{"value": 0})
				e.set(p2, map[string]any{"field": map[string]any{"group": "height"}})
				return nil
			}
			e.set(p, map[string]any{"value": 0})
			e.set(size, map[string]any{"field": map[string]any{"group": "height"}})
		default:
			e.set(p, map[string]any{"signal": u.v.sizeName(i), "mult": 0.5})
		}
		return nil
	}
	if d.hasValue && u.mark == "tick" && ch == u.measure() {
		// A tick at a value, such as a tick of an axis drawn by hand, is
		// as thick as any tick.
		e.set(pc, propRef(d.value))
		e.set(size, map[string]any{"value": u.tickThickness()})
		return nil
	}
	if d.hasValue {
		e.set(p, propRef(d.value))
		if d2 != nil && d2.hasValue {
			e.set(p2, propRef(d2.value))
		}
		return nil
	}
	discrete := d.discrete()
	if d.bin != nil {
		u.v.addScale(u, ch, d, []string{d.out, d.outEnd})
		o1, o2, err := u.spanOffsets(ch, d)
		if err != nil {
			return err
		}
		e.set(p, u.spanRef(ch, d.out, o1))
		e.set(p2, u.spanRef(ch, d.outEnd, o2))
		if u.mark != "bar" && u.mark != "rect" {
			e.set(p, map[string]any{"signal": "scale(\"" + u.v.sn(ch) + "\", 0.5 * " + fieldRef(d.out) + " + 0.5 * " + fieldRef(d.outEnd) + ")"})
			delete(e, p2)
			return nil
		}
		u.setFromEnd(ch)
		return nil
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
		e.set(size, map[string]any{"signal": "max(" + u.minBandSize() + ", bandwidth('" + band + "'))"})
		return nil
	}
	if d.units != nil && d.typ == temporal && u.spans(d) && u.mark != "bar" && u.mark != "rect" {
		// A band position places the mark at a share of the interval.
		u.v.addScale(u, ch, d, []string{d.out, d.outEnd})
		p0 := 0.5
		if b, ok := valueOf(d.def.BandPosition).(float64); ok {
			p0 = b
		}
		k := func(x float64) string { return strconv.FormatFloat(x, 'f', -1, 64) }
		at := map[string]any{"signal": "scale(\"" + u.v.sn(ch) + "\", " + k(1-p0) + " * " + fieldRef(d.out) + " + " + k(p0) + " * " + fieldRef(d.outEnd) + ")"}
		if u.mark == "tick" && ch == u.dimension() {
			// A tick across the interval is centered on its place, with
			// the size of a tick on a continuous scale.
			sz, _ := u.markSize(size)
			if _, rel := relBand(sz); rel || sz == nil {
				sz = u.continuousBandSize()
			}
			e.set(pc, at)
			e.set(size, propRef(sz))
			return nil
		}
		e.set(p, at)
		u.pathDefined(e)
		return nil
	}
	if d.units != nil && d.typ == temporal && u.spans(d) {
		start, end := d.out, d.outEnd
		if u.shifted(d) {
			start, end = d.out+"_offsetted_rect_start", d.out+"_offsetted_rect_end"
		}
		u.v.addScale(u, ch, d, []string{start, end})
		sz, at := u.markSize(size)
		if f, ok := relBand(sz); ok {
			// A size relative to the band takes that share of the
			// interval, around its middle.
			share := func(t float64) map[string]any {
				return map[string]any{"signal": "scale(\"" + u.v.sn(ch) + "\", " + num(1-t) + " * " + fieldRef(start) + " + " + num(t) + " * " + fieldRef(end) + ")"}
			}
			r, r2 := share((1-f)/2), share((1+f)/2)
			r["offset"], r2["offset"] = 0.5, -0.5
			e.set(p, r)
			e.set(p2, r2)
			u.setFromEnd(ch)
			return nil
		}
		if sz != nil {
			// The bar spans the interval, and the report says so.
			if err := u.c.issue(at, "a fixed %s of a bar over the interval of a time unit", size); err != nil {
				return err
			}
		}
		o1, o2, err := u.spanOffsets(ch, d)
		if err != nil {
			return err
		}
		e.set(p, u.spanRef(ch, start, o1))
		e.set(p2, u.spanRef(ch, end, o2))
		u.setFromEnd(ch)
		return nil
	}
	if u.stack == ch {
		u.v.addScale(u, ch, d, []string{d.stackName("start"), d.stackName("end")})
		if sc := u.v.scales[u.v.key(ch)]; sc != nil && u.offset == "normalize" {
			sc.normalized = true
		}
		if b, ok := bandPosition(d); ok && u.mark != "bar" && u.mark != "rect" && u.mark != "area" {
			// A band position places a mark, such as a label, at that
			// share of its part of the stack.
			start, end := fieldRef(d.stackName("start")), fieldRef(d.stackName("end"))
			e.set(p, map[string]any{"signal": "scale(\"" + u.v.sn(ch) + "\", " + num(1-b) + " * " + start + " + " + num(b) + " * " + end + ")"})
			return nil
		}
		e.set(p, map[string]any{"scale": u.v.sn(ch), "field": d.stackName("end")})
		e.set(p2, map[string]any{"scale": u.v.sn(ch), "field": d.stackName("start")})
		return nil
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
		if sc := u.v.scales[u.v.key(ch)]; hasScale && sc != nil {
			sc.addDatum(d.datum)
		}
		ref = func(string) map[string]any {
			r := d.datumRef()
			r["scale"] = u.v.sn(ch)
			return r
		}
	}
	switch {
	case discrete && (u.mark == "bar" || u.mark == "rect") && d2 != nil && d2.isField():
		// A bar from one band to another, such as a lane of a heat lane,
		// runs between their middles. On a point scale it runs between
		// the points.
		r, r2 := ref(d.out), ref(d2.out)
		if u.v.scaleType(ch) == "band" {
			r["band"], r2["band"] = 0.5, 0.5
		}
		e.set(p, r)
		e.set(p2, r2)
	case discrete && (u.mark == "bar" || u.mark == "rect" || u.mark == "tick"):
		if d2 != nil && d2.isField() && u.mark != "tick" && u.ch[ch+"Offset"] == nil {
			// A bar from one band to another runs between their
			// middles.
			r, r2 := ref(d.out), ref(d2.out)
			r["band"], r2["band"] = 0.5, 0.5
			e.set(p, r)
			e.set(p2, r2)
			return nil
		}
		if c := u.ch["size"]; c != nil && !c.hasValue && u.mark != "rect" {
			if r := u.channelRef("size"); r != nil {
				// The size channel sets the width of a bar or a tick,
				// around the middle of its band.
				pr := ref(d.out)
				if o := u.offsetRef(ch, true); o != nil {
					pr["offset"] = o
				} else {
					pr["band"] = 0.5
				}
				e.set(pc, pr)
				e.set(size, r)
				return nil
			}
		}
		sz, at := u.markSize(size)
		if _, ok := relBand(sz); ok && u.ch[ch+"Offset"] != nil {
			// The bar fills the band of its offset, and the report says
			// so.
			if err := u.c.issue(at, "a %s relative to the band of a bar with an offset", size); err != nil {
				return err
			}
			sz = nil
		}
		if f, ok := relBand(sz); ok {
			// A size relative to the band takes that share of the band,
			// around its middle.
			r := ref(d.out)
			r["band"] = (1 - f) / 2
			e.set(p, r)
			e.set(size, map[string]any{"signal": "max(" + u.minBandSize() + ", " + num(f) + " * bandwidth('" + u.v.sn(ch) + "'))"})
			return nil
		}
		if sz != nil {
			r := ref(d.out)
			if o := u.offsetRef(ch, true); o != nil {
				r["offset"] = o
			} else {
				r["band"] = 0.5
			}
			e.set(pc, r)
			e.set(size, propRef(sz))
			return nil
		}
		r := ref(d.out)
		band := u.v.sn(ch)
		if o := u.offsetRef(ch, false); o != nil {
			// An offset places the bar in a sub-band of the band.
			r["offset"] = o
			band = u.v.sn(ch + "Offset")
		}
		e.set(p, r)
		e.set(size, map[string]any{"signal": "max(" + u.minBandSize() + ", bandwidth('" + band + "'))"})
	case discrete && u.v.scaleType(ch) == "band":
		r := ref(d.out)
		if o := u.offsetRef(ch, false); o != nil {
			r["offset"] = o
			// A later layer, such as the box of a box plot, can turn
			// the points of the offset into bands. The mark then goes
			// in the middle of the band of its offset.
			u.lateOffsets = append(u.lateOffsets, p)
		} else {
			r["band"] = 0.5
		}
		e.set(p, r)
		if d2 != nil && d2.isField() {
			// A rule from one band to another runs between their middles.
			r2 := ref(d2.out)
			r2["band"] = 0.5
			e.set(p2, r2)
		}
	case u.mark == "tick" && u.measure() == ch:
		e.set(pc, u.invalidAtZero(ch, d.out, hasScale, ref(d.out)))
		e.set(size, map[string]any{"value": u.tickThickness()})
		return nil
	case (u.mark == "bar" || u.mark == "tick") && ch == u.dimension() && d2 == nil:
		// A bar or a tick on a continuous scale has no band. It is
		// centered on its value, with the size that the spec gives, or
		// else the continuousBandSize of the config of the mark, such as
		// a tick at the end of an error bar over the years.
		sz, at := u.markSize(size)
		if _, ok := relBand(sz); ok {
			// The bar takes the default size, and the report says so.
			if err := u.c.issue(at, "a %s relative to the band of a bar on a continuous scale", size); err != nil {
				return err
			}
			sz = nil
		}
		if sz == nil {
			sz = u.continuousBandSize()
		}
		e.set(pc, u.invalidAtZero(ch, d.out, hasScale, ref(d.out)))
		e.set(size, propRef(sz))
	default:
		if discrete {
			e.set(p, ref(d.out))
		} else {
			e.set(p, u.invalidAtZero(ch, d.out, hasScale, ref(d.out)))
		}
		if discrete && u.v.scaleType(ch) == "point" && u.ch[ch+"Offset"] == nil {
			// A later layer, such as the box of a boxplot, can turn the
			// scale into bands. The mark then goes in the middle of its
			// band.
			u.late = append(u.late, p)
		}
		switch {
		case d2 != nil && d2.isField() && d.binned && !discrete:
			// A bar over bins that the data already holds, from its start
			// to the end of x2, leaves the gap of its binSpacing between
			// bars, as a bar over the bins of a transform does.
			o1, o2, err := u.spanOffsets(ch, d)
			if err != nil {
				return err
			}
			e.set(p, u.spanRef(ch, d.out, o1))
			e.set(p2, u.spanRef(ch, d2.out, o2))
		case d2 != nil && d2.isField():
			if discrete {
				e.set(p2, ref(d2.out))
			} else {
				e.set(p2, u.invalidAtZero(ch, d2.out, hasScale, ref(d2.out)))
			}
		case d2 != nil && d2.hasValue:
			e.set(p2, propRef(d2.value))
		case u.mark == "area" && u.measure() == ch && hasScale && !u.zeroInDomain(ch):
			// An area whose scale can leave out zero runs down to the
			// bottom of its domain, as in vega-lite.js, and not past the
			// edge of the plot.
			sn := u.v.sn(ch)
			e.set(p2, map[string]any{"signal": "scale(" + strconv.Quote(sn) + ", inrange(0, domain(" + strconv.Quote(sn) + ")) ? 0 : domain(" + strconv.Quote(sn) + ")[0])"})
		case (u.mark == "bar" || u.mark == "area") && u.measure() == ch && hasScale:
			e.set(p2, map[string]any{"scale": u.v.sn(ch), "value": 0})
		case u.mark == "rule" && hasScale && !discrete:
			// A rule on a scale of numbers whose other position is a
			// field of a discrete scale, such as the stem of a lollipop,
			// runs from zero, as vega-lite.js draws it.
			other := map[string]string{"x": "y", "y": "x"}[ch]
			if o := u.ch[other]; o != nil && o.isField() && o.discrete() && u.ch[other+"2"] == nil {
				e.set(p2, map[string]any{"scale": u.v.sn(ch), "value": 0})
			}
		}
	}
	u.pathDefined(e)
	if u.mark == "area" && u.measure() == ch {
		e.set("orient", map[string]any{"value": u.orient})
	}
	return nil
}

// bandPosition returns the band position that a channel gives, if it gives
// a number.
func bandPosition(d *channelDef) (float64, bool) {
	if d.def == nil {
		return 0, false
	}
	b, ok := valueOf(d.def.BandPosition).(float64)
	return b, ok
}

// setFromEnd records that the bar of a channel of position spans a bin or
// a unit of time.
func (u *unit) setFromEnd(ch string) {
	if u.fromEnd == nil {
		u.fromEnd = map[string]bool{}
	}
	u.fromEnd[ch] = true
}

// markOffsets adds the offsets of the mark, such as xOffset, to the offsets
// of its positions. Vega has no such properties. xOffset moves x, or xc
// when the mark is centered. On a bar that spans a bin or a unit of time,
// xOffset moves the end of the interval and x2Offset its start.
func (u *unit) markOffsets(e enc) error {
	for _, k := range []string{"xOffset", "x2Offset", "yOffset", "y2Offset"} {
		v, ok := u.props[k]
		if !ok {
			continue
		}
		name := strings.TrimSuffix(k, "Offset")
		if u.fromEnd[name[:1]] {
			// zuzu writes the start of the interval in x and its end in
			// x2.
			if len(name) == 1 {
				name += "2"
			} else {
				name = name[:1]
			}
		}
		ref, _ := e[name].(map[string]any)
		if ref == nil && len(name) == 1 {
			ref, _ = e[name+"c"].(map[string]any)
		}
		if ref == nil {
			if err := u.c.issue(u.path+"/mark/"+k, "the property %s of a mark with no %s", k, name); err != nil {
				return err
			}
			continue
		}
		f, isNum := v.(float64)
		sig, isSig := propRef(v)["signal"].(string)
		if !isNum && !isSig {
			if err := u.c.issue(u.path+"/mark/"+k, "the value %v of the property %s", v, k); err != nil {
				return err
			}
			continue
		}
		switch old := ref["offset"].(type) {
		case nil:
			if isNum {
				ref["offset"] = f
			} else {
				ref["offset"] = map[string]any{"signal": sig}
			}
		case float64:
			if isNum {
				ref["offset"] = old + f
			} else {
				ref["offset"] = map[string]any{"signal": num(old) + " + (" + sig + ")"}
			}
		default:
			if err := u.c.issue(u.path+"/mark/"+k, "the property %s of a mark whose %s has an offset of its own", k, name); err != nil {
				return err
			}
		}
	}
	return nil
}

// markSize returns the size of a mark across its band, as the spec gives
// it, and the path of the part of the spec that gives it. The width or the
// height of a bar or a rect comes first, then the channel size and the
// size of the mark, which a rect does not read. It returns nil when the
// spec gives no size.
func (u *unit) markSize(size string) (any, string) {
	if u.mark == "bar" || u.mark == "rect" {
		if v, ok := u.props[size]; ok {
			return v, u.path + "/mark/" + size
		}
	}
	if u.mark == "rect" {
		return nil, ""
	}
	if c := u.ch["size"]; c != nil && c.hasValue {
		return c.value, u.path + "/encoding/size"
	}
	if v, ok := u.props["size"]; ok {
		return v, u.path + "/mark/size"
	}
	return nil, ""
}

// spanOffsets returns the offsets of the start and the end of a bar or a
// rect that spans an interval, a bin or a time unit, along a channel. A bar
// leaves the gap of its binSpacing between bars, 1 pixel by default, and a
// rect leaves none. Each end moves half the gap toward the middle of the
// interval. The range of y runs up, from the height to 0, so its offsets
// turn around, and so do the offsets of a scale that the spec reverses.
func (u *unit) spanOffsets(ch string, d *channelDef) (float64, float64, error) {
	if u.mark != "bar" {
		return 0, 0, nil
	}
	gap, at := any(1.0), ""
	if v, ok := u.props["binSpacing"]; ok {
		gap, at = v, u.path+"/mark/binSpacing"
	} else if u.c.cfg != nil {
		if v, ok := u.c.cfg.marks[u.mark]["binSpacing"]; ok {
			gap, at = v, "/config/bar/binSpacing"
		}
	}
	g, ok := gap.(float64)
	if !ok {
		// The bar takes the default gap, and the report says so.
		if err := u.c.issue(at, "the value %v of the property binSpacing", gap); err != nil {
			return 0, 0, err
		}
		g = 1
	}
	dir := 1.0
	if ch == "y" {
		dir = -1
	}
	if d.def != nil && !d.def.Scale.IsZero() {
		if p, ok := valueOf(d.def.Scale).(map[string]any); ok && p["reverse"] == true {
			dir = -dir
		}
	}
	return dir * g / 2, -dir * g / 2, nil
}

// spanRef returns the reference of one end of a bar or a rect over an
// interval, with its offset when it has one.
func (u *unit) spanRef(ch, field string, offset float64) map[string]any {
	r := map[string]any{"scale": u.v.sn(ch), "field": field}
	if offset != 0 {
		r["offset"] = offset
	}
	return r
}

// cornerEnd rounds the end of a bar that is away from zero by the
// cornerRadiusEnd of the spec or of its config, as vega-lite.js does: each
// corner tests which end of the bar its scaled bounds put at that side. A
// cornerRadiusEnd of 0 so gives square ends, and the look leaves them. A
// stack of bars that stackGroup puts in groups rounds its end there
// instead.
func (u *unit) cornerEnd(e enc) {
	r, ok := u.props["cornerRadiusEnd"]
	if u.mark != "bar" || !ok || u.stackCorners() != nil {
		return
	}
	along := "y"
	names := [2][2]string{{"cornerRadiusTopLeft", "cornerRadiusTopRight"}, {"cornerRadiusBottomLeft", "cornerRadiusBottomRight"}}
	if u.orient == "horizontal" {
		along = "x"
		names = [2][2]string{{"cornerRadiusTopRight", "cornerRadiusBottomRight"}, {"cornerRadiusTopLeft", "cornerRadiusBottomLeft"}}
	}
	a, ok1 := refExpr(e[along])
	b, ok2 := refExpr(e[along+"2"])
	if !ok1 || !ok2 {
		return
	}
	// y grows down, so the top of a bar is its least scaled y, and the
	// right of a horizontal bar its greatest x.
	tests := [2]string{a + " < " + b, a + " > " + b}
	if along == "x" {
		tests = [2]string{a + " > " + b, a + " < " + b}
	}
	for i, pair := range names {
		for _, k := range pair {
			rule := propRef(r)
			rule["test"] = tests[i]
			e.set(k, map[string]any{rulesKey: []any{rule, map[string]any{"value": 0}}})
		}
	}
}

// refExpr returns an expression for the value of a reference of an encode
// with a scale, such as scale("y", datum["b"]), or false for another kind.
func refExpr(v any) (string, bool) {
	ref, ok := v.(map[string]any)
	if !ok {
		return "", false
	}
	sc, ok := ref["scale"].(string)
	if !ok || ref["band"] != nil || ref["offset"] != nil {
		return "", false
	}
	if f, ok := ref["field"].(string); ok {
		return "scale(" + strconv.Quote(sc) + ", " + fieldRef(f) + ")", true
	}
	if val, ok := ref["value"]; ok {
		b, err := json.Marshal(val)
		if err != nil {
			return "", false
		}
		return "scale(" + strconv.Quote(sc) + ", " + string(b) + ")", true
	}
	return "", false
}

// tickThickness returns the thickness of a tick: the thickness of the mark,
// or of the config of the tick, or 1.
func (u *unit) tickThickness() float64 {
	if v, ok := u.props["thickness"].(float64); ok {
		return v
	}
	if u.c.cfg != nil {
		if v, ok := u.c.cfg.marks["tick"]["thickness"].(float64); ok {
			return v
		}
	}
	return 1
}

// styled reports whether the spec gives a mark a color, a width or a dash
// of its own, by a property of the mark or by a channel.
func (u *unit) styled() bool {
	for _, k := range []string{"color", "stroke", "strokeWidth", "strokeDash", "size"} {
		if _, ok := u.props[k]; ok {
			return true
		}
		if u.ch[k] != nil {
			return true
		}
	}
	return false
}

// relBand returns the share of the band that a size relative to the band,
// such as {"band": 0.5}, gives.
func relBand(v any) (float64, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return 0, false
	}
	f, ok := m["band"].(float64)
	return f, ok
}

// minBandSize returns the smallest size of a bar or a rect across a band:
// the minBandSize of the mark or of the config of the mark, or else 0.25
// pixels, as vega-lite.js draws it.
func (u *unit) minBandSize() string {
	if v, ok := u.props["minBandSize"].(float64); ok {
		return num(v)
	}
	if u.c.cfg != nil {
		if v, ok := u.c.cfg.marks[u.mark]["minBandSize"].(float64); ok {
			return num(v)
		}
	}
	return "0.25"
}

// continuousBandSize returns the size of a bar on a continuous scale that
// the config of the bar gives, or 5 pixels, the default of Vega-Lite.
func (u *unit) continuousBandSize() any {
	if u.c.cfg != nil {
		if v, ok := u.c.cfg.marks[u.mark]["continuousBandSize"]; ok {
			return v
		}
	}
	return 5.0
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
	// The size of a view in a facet or a concatenation has its own name.
	w, h := u.v.sizeName(0), u.v.sizeName(1)
	e.set("x", map[string]any{"signal": w, "mult": 0.5})
	e.set("y", map[string]any{"signal": h, "mult": 0.5})
	outer := map[string]any{"signal": "min(" + w + "," + h + ")/2"}
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
	case t.isField() && t.bin != nil:
		// A bin spans from its start to its end, as a stack does.
		u.v.addScale(u, "theta", t, []string{t.out, t.outEnd})
		e.set("startAngle", map[string]any{"scale": u.v.sn("theta"), "field": t.outEnd})
		e.set("endAngle", map[string]any{"scale": u.v.sn("theta"), "field": t.out})
	case t.isField():
		u.v.addScale(u, "theta", t, []string{t.out})
		e.set("startAngle", map[string]any{"scale": u.v.sn("theta"), "field": t.out})
		if u.v.scaleType("theta") == "band" {
			e.set("endAngle", map[string]any{"scale": u.v.sn("theta"), "field": t.out, "band": 1})
		}
	}
}

// midRef returns the reference to the middle of an interval through the
// scale of a channel, such as the middle of a bin or of a stack.
func (u *unit) midRef(ch, start, end string) map[string]any {
	return map[string]any{"signal": "scale(" + strconv.Quote(u.v.sn(ch)) + ", 0.5 * " + fieldRef(start) + " + 0.5 * " + fieldRef(end) + ")"}
}

// others sets the channels that are not positions or colors: size, shape,
// opacity, text and the like.
func (u *unit) others(e enc) {
	for _, name := range []string{"opacity", "fillOpacity", "strokeOpacity", "strokeWidth", "strokeDash", "angle", "shape"} {
		if r := u.channelRef(name); r != nil {
			e.set(name, r)
		}
	}
	if d := u.ch["angle"]; d != nil && d.isField() && d.bin != nil && len(d.cond) == 0 {
		// A bin turns a mark by the angle of its middle.
		u.v.addScale(u, "angle", d, []string{d.out, d.outEnd})
		e.set("angle", u.midRef("angle", d.out, d.outEnd))
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
		case "line", "rule":
			// The size of a line or a rule is its width.
			e.set("strokeWidth", map[string]any{"value": s})
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
			ref := propRef(d.value)
			if len(d.cond) > 0 {
				ref = u.conditional(d, ref)
			}
			e.set("text", ref)
		case len(d.cond) > 0 && !d.isField():
			// A text with conditions alone, such as a label on some rows.
			var base map[string]any
			if t, ok := u.props["text"]; ok {
				base = map[string]any{"value": t}
			}
			e.set("text", u.conditional(d, base))
		case d.isField():
			var ref map[string]any
			switch d.typ {
			case quantitative:
				spec := ""
				if !d.def.Format.IsZero() {
					spec = str(d.def.Format)
				}
				ref = map[string]any{"signal": "format(" + fieldRef(d.out) + ", " + strconv.Quote(spec) + ")"}
				if d.bin != nil {
					// A bin writes its range, as its tooltip does.
					start, end := fieldRef(d.out), fieldRef(d.outEnd)
					ref = map[string]any{"signal": "!isValid(" + start + ") || !isFinite(+" + start + ") ? \"null\" : format(" + start + ", " + strconv.Quote(spec) + ") + \" – \" + format(" + end + ", " + strconv.Quote(spec) + ")"}
				}
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
		if o.hasDatum {
			s.addDatum(o.datum)
		}
		if s.offset == "" {
			switch {
			case u.ch[ch] == nil:
				s.offset = "center"
			case !s.discrete:
				// A continuous offset, such as a random jitter, spans
				// the band of the position.
			case u.mark != "bar" && u.mark != "rect" || o.hasDatum:
				s.offset = "step"
			}
		}
		if !s.discrete {
			// A continuous offset has no band of its own.
			band = false
		}
	}
	r := map[string]any{"scale": u.v.sn(ch + "Offset"), "field": o.out}
	if o.hasDatum {
		r = o.datumRef()
		r["scale"] = u.v.sn(ch + "Offset")
	}
	if band {
		r["band"] = 0.5
	}
	return r
}

// unitWidth returns the signal of the width of one unit of time of a channel
// on its scale of time: the place of a date one unit after 2001-01-01 less
// the place of 2001-01-01, as vega-lite.js measures it. The smallest unit of
// the time unit, times its step, is the unit.
func unitWidth(scale string, c *channelDef) string {
	// The parts of a date of Vega: year, month (from 0), date, hours,
	// minutes, seconds and milliseconds.
	start := [7]int{2001, 0, 1, 0, 0, 0, 0}
	end := start
	step := max(int(c.unitStep), 1)
	smallest := ""
	for _, u := range []string{"year", "quarter", "month", "week", "date", "day", "dayofyear", "hours", "minutes", "seconds", "milliseconds"} {
		if slices.Contains(c.units, u) {
			smallest = u
		}
	}
	switch smallest {
	case "year":
		end[0] += step
	case "quarter":
		end[1] += 3 * step
	case "month":
		end[1] += step
	case "week":
		end[2] += 7 * step
	case "date", "day", "dayofyear":
		end[2] += step
	case "hours":
		end[3] += step
	case "minutes":
		end[4] += step
	case "seconds":
		end[5] += step
	default:
		end[6] += step
	}
	fn := "datetime"
	if c.utc {
		fn = "utc"
	}
	date := func(d [7]int) string {
		parts := make([]string, len(d))
		for i, v := range d {
			parts[i] = strconv.Itoa(v)
		}
		return fn + "(" + strings.Join(parts, ", ") + ")"
	}
	return "scale('" + scale + "', " + date(end) + ") - scale('" + scale + "', " + date(start) + ")"
}

// posRef returns the reference of a position that a mark gives: "width" or
// "height" for that size of the group, as vega-lite.js writes it, and else
// the value or the expression of the property.
func posRef(v any) map[string]any {
	if v == "width" || v == "height" {
		return map[string]any{"field": map[string]any{"group": v}}
	}
	return propRef(v)
}
