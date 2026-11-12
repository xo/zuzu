package zuzu

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/xo/zuzu/dataflow"
	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/vega"
)

// encodeSets are the sets of encodings that a static chart applies, in
// order. hover and exit need events (D7).
var encodeSets = []string{"enter", "update"}

// itemState is an item while the encodings fill it: the item, and which
// properties of position the encodings set, so that its geometry can be
// worked out.
type itemState struct {
	it     *scene.Item
	set    map[string]bool
	xc, yc float64
	// facet is the index of the facet of a group item, which a sort keeps.
	facet int
	// values holds each property as the encoding set it, with the
	// properties that zuzu does not draw, such as the xfocus that a
	// force reads.
	values map[string]any
}

// newItemState starts an item for a datum.
func newItemState(datum any) *itemState {
	return &itemState{it: &scene.Item{Datum: datum}, set: map[string]bool{}, values: map[string]any{}}
}

// encodeItem applies the encodings of a mark to an item, for a datum.
func (s *scope) encodeItem(enc vega.Encode, st *itemState, datum any, path string) error {
	it := st.it
	for _, set := range encodeSets {
		props, ok := enc[set]
		if !ok {
			continue
		}
		for name, prod := range props {
			v, ok, err := s.production(prod, datum, it)
			if err != nil {
				return fmt.Errorf("%s/encode/%s/%s: %w", path, set, name, err)
			}
			if !ok {
				continue
			}
			setProp(st, name, v)
		}
	}
	return nil
}

// production evaluates the value of a property: the first rule whose test
// is true. It reports false if no rule applies.
func (s *scope) production(p vega.Production, datum any, it *scene.Item) (any, bool, error) {
	for _, r := range p {
		if r.Test != "" {
			t, err := s.eval(r.Test, datum)
			if err != nil {
				return nil, false, err
			}
			if !expr.Truthy(t) {
				continue
			}
		}
		v, err := s.valueRef(&r.ValueRef, datum, it)
		return v, true, err
	}
	return nil, false, nil
}

// valueRef evaluates a value reference: a value, a signal or a field,
// through a scale, then with a band, an exponent, a multiple and an offset.
func (s *scope) valueRef(r *vega.ValueRef, datum any, it *scene.Item) (any, error) {
	v := expr.Undefined
	hasBase := true
	switch {
	case r.Signal != "":
		x, err := s.eval(r.Signal, datum)
		if err != nil {
			return nil, err
		}
		v = x
	case len(r.Value) > 0:
		var x any
		if err := json.Unmarshal(r.Value, &x); err != nil {
			return nil, err
		}
		v = x
	case !r.Field.IsZero():
		x, err := s.fieldRef(r.Field, datum)
		if err != nil {
			return nil, err
		}
		v = x
	default:
		hasBase = false
	}
	if !r.Scale.IsZero() {
		sc, err := s.scaleRef(r.Scale, datum)
		if err != nil {
			return nil, err
		}
		switch {
		case hasBase:
			v = sc.Map(v)
		case !r.Band.IsZero():
			v = 0.0
		default:
			// A scale alone gives the start of its range, as a band
			// scale with no value gives the edge of its first band.
			rng := sc.Range()
			if len(rng) > 0 {
				v = rng[0]
			}
		}
		if !r.Band.IsZero() {
			b, err := s.value(r.Band)
			if err != nil {
				return nil, err
			}
			f := 1.0
			if x, ok := b.(float64); ok {
				f = x
			} else if b == false {
				f = 0
			}
			if bw := sc.Bandwidth(); bw > 0 || hasBase {
				v = expr.Number(v) + f*bw
			}
		}
	}
	if !r.Exponent.IsZero() {
		e, err := s.numberRef(r.Exponent, datum, it)
		if err != nil {
			return nil, err
		}
		v = math.Pow(expr.Number(v), e)
	}
	if !r.Mult.IsZero() {
		m, err := s.numberRef(r.Mult, datum, it)
		if err != nil {
			return nil, err
		}
		v = expr.Number(v) * m
	}
	if !r.Offset.IsZero() {
		o, err := s.numberRef(r.Offset, datum, it)
		if err != nil {
			return nil, err
		}
		v = expr.Number(v) + o
	}
	if r.Round != nil && *r.Round {
		v = math.Round(expr.Number(v))
	}
	return v, nil
}

// numberRef evaluates an offset, a multiple or an exponent: a number, or a
// value reference.
func (s *scope) numberRef(v vega.Value, datum any, it *scene.Item) (float64, error) {
	if v.Signal != "" {
		x, err := s.eval(v.Signal, datum)
		return expr.Number(x), err
	}
	if v.JSON.Kind() == '{' {
		var r vega.ValueRef
		if err := json.Unmarshal(v.JSON, &r); err != nil {
			return 0, err
		}
		x, err := s.valueRef(&r, datum, it)
		return expr.Number(x), err
	}
	var f float64
	if err := json.Unmarshal(v.JSON, &f); err != nil {
		return 0, err
	}
	return f, nil
}

// scaleRef finds the scale that a value reference names: a name, an
// object with a name or a signal, or a field of the datum or the group
// that holds the name.
func (s *scope) scaleRef(v vega.Value, datum any) (scale.Scale, error) {
	if v.Signal != "" {
		x, err := s.eval(v.Signal, nil)
		if err != nil {
			return nil, err
		}
		return s.scale(expr.String(x))
	}
	var name any
	if err := json.Unmarshal(v.JSON, &name); err != nil {
		return nil, err
	}
	switch x := name.(type) {
	case string:
		return s.scale(x)
	case map[string]any:
		if n, ok := x["name"].(string); ok {
			return s.scale(n)
		}
		if sig, ok := x["signal"].(string); ok {
			y, err := s.eval(sig, nil)
			if err != nil {
				return nil, err
			}
			return s.scale(expr.String(y))
		}
		var f vega.Field
		if err := json.Unmarshal(v.JSON, &f); err == nil && (f.Datum != nil || f.Group != nil || f.Parent != nil) {
			y, err := s.fieldRef(f, datum)
			if err != nil {
				return nil, err
			}
			return s.scale(expr.String(y))
		}
	}
	return nil, fmt.Errorf("the scale %s: %w", v.JSON, vega.ErrInvalidSpec)
}

// fieldRef reads a field of a datum, of the group, or of the datum of a
// parent group.
func (s *scope) fieldRef(f vega.Field, datum any) (any, error) {
	switch {
	case f.Signal != "":
		x, err := s.eval(f.Signal, datum)
		if err != nil {
			return nil, err
		}
		return readField(datum, expr.String(x)), nil
	case f.Datum != nil:
		name, err := s.fieldName(*f.Datum, datum)
		if err != nil {
			return nil, err
		}
		return readField(datum, name), nil
	case f.Group != nil:
		g := s.groupAt(f.Level)
		name, err := s.fieldName(*f.Group, datum)
		if err != nil {
			return nil, err
		}
		if g == nil || g.item == nil {
			return expr.Undefined, nil
		}
		return itemField(g.item, name), nil
	case f.Parent != nil:
		g := s.groupAt(f.Level)
		name, err := s.fieldName(*f.Parent, datum)
		if err != nil {
			return nil, err
		}
		if g == nil {
			return expr.Undefined, nil
		}
		return readField(g.datum, name), nil
	}
	return readField(datum, f.Name), nil
}

// fieldName returns the name that a field reference gives.
func (s *scope) fieldName(f vega.Field, datum any) (string, error) {
	if f.Signal != "" {
		x, err := s.eval(f.Signal, datum)
		return expr.String(x), err
	}
	return f.Name, nil
}

// groupAt returns the scope of the group a number of levels up: 0 or 1 is
// the enclosing group.
func (s *scope) groupAt(level int) *scope {
	g := s
	for i := 1; i < level && g != nil; i++ {
		g = g.parent
	}
	return g
}

// readField reads a field of a datum, by a name in the syntax of Vega.
func readField(datum any, name string) any {
	m, ok := datum.(map[string]any)
	if !ok {
		return expr.Undefined
	}
	return dataflow.FieldAccessor(name).Get(m)
}

// itemField reads a property of an item, such as the width of a group.
func itemField(it *scene.Item, name string) any {
	switch name {
	case "x":
		return it.X
	case "y":
		return it.Y
	case "width":
		return it.Width
	case "height":
		return it.Height
	case "x2":
		return it.X2
	case "y2":
		return it.Y2
	}
	return expr.Undefined
}

// setProp sets a property of an item to a value.
func setProp(st *itemState, name string, v any) {
	if st.values == nil {
		st.values = map[string]any{}
	}
	st.values[name] = v
	it := st.it
	num := func() float64 {
		f := expr.Number(v)
		if v == nil || expr.IsUndefined(v) {
			return math.NaN()
		}
		return f
	}
	ptr := func() *float64 {
		if v == nil || expr.IsUndefined(v) {
			return nil
		}
		f := expr.Number(v)
		return &f
	}
	str := func() string {
		if v == nil || expr.IsUndefined(v) {
			return ""
		}
		return expr.String(v)
	}
	switch name {
	case "x":
		it.X = num()
		st.set[name] = true
	case "y":
		it.Y = num()
		st.set[name] = true
	case "x2":
		it.X2 = num()
		st.set[name] = true
	case "y2":
		it.Y2 = num()
		st.set[name] = true
	case "xc":
		st.xc = num()
		st.set[name] = true
	case "yc":
		st.yc = num()
		st.set[name] = true
	case "width":
		it.Width = num()
		st.set[name] = true
	case "height":
		it.Height = num()
		st.set[name] = true
	case "fill":
		it.Fill = paint(v)
		st.set[name] = true
	case "stroke":
		it.Stroke = paint(v)
		st.set[name] = true
	case "opacity":
		it.Opacity = ptr()
	case "fillOpacity":
		it.FillOpacity = ptr()
	case "strokeOpacity":
		it.StrokeOpacity = ptr()
	case "strokeWidth":
		it.StrokeWidth = ptr()
	case "strokeDash":
		it.StrokeDash = nil
		if l, ok := v.([]any); ok {
			for _, e := range l {
				it.StrokeDash = append(it.StrokeDash, expr.Number(e))
			}
		}
	case "strokeCap":
		it.StrokeCap = str()
	case "strokeJoin":
		it.StrokeJoin = str()
	case "cornerRadius":
		it.CornerRadius = zeroNaN(num())
	case "cornerRadiusTopLeft":
		it.CornerRadiusTopLeft = zeroNaN(num())
	case "cornerRadiusTopRight":
		it.CornerRadiusTopRight = zeroNaN(num())
	case "cornerRadiusBottomLeft":
		it.CornerRadiusBottomLeft = zeroNaN(num())
	case "cornerRadiusBottomRight":
		it.CornerRadiusBottomRight = zeroNaN(num())
	case "startAngle":
		it.StartAngle = num()
	case "endAngle":
		it.EndAngle = num()
	case "innerRadius":
		it.InnerRadius = num()
	case "outerRadius":
		it.OuterRadius = num()
	case "padAngle":
		it.PadAngle = num()
	case "shape":
		it.Shape = str()
	case "size":
		it.Size = ptr()
	case "angle":
		it.Angle = zeroNaN(num())
	case "path":
		it.Path = str()
	case "interpolate":
		it.Interpolate = str()
	case "tension":
		it.Tension = num()
	case "orient":
		it.Orient = str()
	case "defined":
		b := expr.Truthy(v)
		it.Defined = &b
	case "text":
		it.Text = textOf(v)
	case "font":
		it.Font = str()
	case "fontSize":
		it.FontSize = num()
	case "fontWeight":
		it.FontWeight = scene.Weight(str())
	case "fontStyle":
		it.FontStyle = str()
	case "align":
		it.Align = str()
	case "baseline":
		it.Baseline = str()
	case "dx":
		it.Dx = zeroNaN(num())
	case "dy":
		it.Dy = zeroNaN(num())
	case "radius":
		it.Radius = zeroNaN(num())
	case "theta":
		it.Theta = zeroNaN(num())
	case "lineHeight":
		it.LineHeight = num()
	case "limit":
		it.Limit = zeroNaN(num())
	case "ellipsis":
		it.Ellipsis = str()
	case "dir":
		it.Dir = str()
	case "url", "image":
		// An image of Vega can also come from the field image, which the
		// transform heatmap writes as a URL of data.
		it.URL = str()
	case "aspect":
		b := expr.Truthy(v)
		it.Aspect = &b
	case "smooth":
		b := expr.Truthy(v)
		it.Smooth = &b
	case "clip":
		it.Clip = expr.Truthy(v)
	case "href":
		it.Href = str()
	case "tooltip":
		it.Tooltip = scene.Tooltip(tooltipText(v))
	}
	// Other properties, such as description and cursor, change nothing that
	// zuzu draws.
}

// tooltipText writes the value of a tooltip as the text that Vega shows
// without a plugin: an object as one line "key: value" for each of its
// fields, in the order of their keys, an array as [a, b], and any other
// value as its text. A value in an object or an array that is itself an
// object or an array shows as {…} or […]. null and undefined show nothing.
func tooltipText(v any) string {
	if v == nil || expr.IsUndefined(v) {
		return ""
	}
	inner := func(v any) string {
		switch x := v.(type) {
		case []any:
			return "[…]"
		case map[string]any:
			return "{…}"
		case nil:
			return "null"
		default:
			return expr.String(x)
		}
	}
	array := func(l []any) string {
		parts := make([]string, len(l))
		for i, e := range l {
			parts[i] = inner(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	switch x := v.(type) {
	case []any:
		return array(x)
	case map[string]any:
		lines := make([]string, 0, len(x))
		for _, k := range slices.Sorted(maps.Keys(x)) {
			if l, ok := x[k].([]any); ok {
				lines = append(lines, k+": "+array(l))
				continue
			}
			lines = append(lines, k+": "+inner(x[k]))
		}
		return strings.Join(lines, "\n")
	}
	return expr.String(v)
}

// linkIssue clears the link of an item when LinkAllowed refuses it, and
// returns the issue of the mark for the first such item. reported says
// whether the mark has its issue already.
func (s *scope) linkIssue(it *scene.Item, path string, reported *bool) error {
	if it.Href == "" || scene.LinkAllowed(it.Href) {
		return nil
	}
	href := it.Href
	it.Href = ""
	if *reported {
		return nil
	}
	*reported = true
	return s.rt.issue(vega.ErrUnsupported, path+"/encode", "the link %q, whose scheme zuzu does not write", href)
}

// zeroNaN returns 0 for NaN.
func zeroNaN(f float64) float64 {
	if math.IsNaN(f) {
		return 0
	}
	return f
}

// paint reads a color, or a gradient that Vega gives as an object.
func paint(v any) scene.Paint {
	switch x := v.(type) {
	case string:
		return scene.Paint{Color: x}
	case *expr.Color:
		return scene.Paint{Color: x.String()}
	case map[string]any:
		b, err := json.Marshal(x)
		if err != nil {
			return scene.Paint{}
		}
		var p scene.Paint
		if json.Unmarshal(b, &p) == nil {
			return p
		}
	}
	return scene.Paint{}
}

// textOf reads the text of a text item: a string, or an array of lines.
func textOf(v any) scene.Lines {
	switch x := v.(type) {
	case nil:
		return nil
	case []any:
		out := make(scene.Lines, len(x))
		for i, e := range x {
			out[i] = expr.String(e)
		}
		return out
	case string:
		return scene.Lines{x}
	}
	if expr.IsUndefined(v) {
		return nil
	}
	return scene.Lines{expr.String(v)}
}
