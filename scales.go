package zuzu

import (
	"encoding/json/v2"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/xo/zuzu/dataflow"
	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/internal/color"
	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// scale returns a scale that this scope can see.
func (s *scope) scale(name string) (scale.Scale, error) {
	for sc := s; sc != nil; sc = sc.parent {
		st, ok := sc.scales[name]
		if !ok {
			continue
		}
		switch st.state {
		case done:
			return st.s, st.err
		case busy:
			return nil, fmt.Errorf("the scale %q: %w", name, ErrCycle)
		}
		st.state = busy
		st.s, st.err = sc.buildScale(st)
		st.state = done
		return st.s, st.err
	}
	return nil, fmt.Errorf("the scale %q: %w", name, ErrUnknownName)
}

// buildScale resolves the domain, the range and the settings of a scale
// from the spec, and builds it.
func (s *scope) buildScale(st *scaleState) (scale.Scale, error) {
	d := st.def
	typ := d.Type
	if typ == "" {
		typ = "linear"
	}
	o := scale.Defaults(typ)
	o.Location = s.rt.loc
	if typ == "utc" {
		o.Location = time.UTC
	}
	num := func(v vega.Value, def float64) (float64, error) {
		if v.IsZero() {
			return def, nil
		}
		x, err := s.value(v)
		if err != nil {
			return def, err
		}
		return number(x, def), nil
	}
	boolean := func(v vega.BoolValue) (bool, bool, error) {
		switch v.Kind {
		case vega.Literal:
			return v.Bool, true, nil
		case vega.SignalRef:
			x, err := s.eval(v.Signal, nil)
			return expr.Truthy(x), true, err
		}
		return false, false, nil
	}
	var err error
	if o.Domain, err = s.domain(d, typ); err != nil {
		return nil, fmt.Errorf("%s/domain: %w", st.path, err)
	}
	if o.DomainMin, err = num(d.DomainMin, math.NaN()); err != nil {
		return nil, err
	}
	if o.DomainMax, err = num(d.DomainMax, math.NaN()); err != nil {
		return nil, err
	}
	if o.DomainMid, err = num(d.DomainMid, math.NaN()); err != nil {
		return nil, err
	}
	if !d.DomainRaw.IsZero() {
		raw, err := s.value(d.DomainRaw)
		if err != nil {
			return nil, err
		}
		if l, ok := raw.([]any); ok && raw != nil {
			o.Domain = l
		}
	}
	if o.Reverse, _, err = boolean(d.Reverse); err != nil {
		return nil, err
	}
	if o.Round, _, err = boolean(d.Round); err != nil {
		return nil, err
	}
	if o.Clamp, _, err = boolean(d.Clamp); err != nil {
		return nil, err
	}
	if z, ok, err := boolean(d.Zero); err != nil {
		return nil, err
	} else if ok {
		o.Zero = &z
	}
	if !d.Nice.IsZero() {
		v, err := s.value(d.Nice)
		if err != nil {
			return nil, err
		}
		switch x := v.(type) {
		case bool:
			o.Nice = x
		case float64:
			o.Nice, o.NiceCount = true, x
		case string:
			o.Nice, o.NiceInterval = true, x
		case map[string]any:
			o.Nice = true
			o.NiceInterval = expr.String(x["interval"])
		}
	}
	if o.Padding, err = num(d.Padding, 0); err != nil {
		return nil, err
	}
	if !d.PaddingInner.IsZero() {
		o.HasPaddingInner = true
		if o.PaddingInner, err = num(d.PaddingInner, 0); err != nil {
			return nil, err
		}
	}
	if !d.PaddingOuter.IsZero() {
		o.HasPaddingOuter = true
		if o.PaddingOuter, err = num(d.PaddingOuter, 0); err != nil {
			return nil, err
		}
	}
	if o.Align, err = num(d.Align, 0.5); err != nil {
		return nil, err
	}
	if o.Exponent, err = num(d.Exponent, 1); err != nil {
		return nil, err
	}
	if o.Base, err = num(d.Base, 10); err != nil {
		return nil, err
	}
	if o.Constant, err = num(d.Constant, 1); err != nil {
		return nil, err
	}
	if !d.Interpolate.IsZero() {
		v, err := s.value(d.Interpolate)
		if err != nil {
			return nil, err
		}
		if m, ok := v.(map[string]any); ok {
			v = m["type"]
		}
		o.Interpolate = expr.String(v)
	}
	if !d.Bins.IsZero() {
		v, err := s.value(d.Bins)
		if err != nil {
			return nil, err
		}
		o.Bins = binEdges(v)
		if typ == "bin-ordinal" && len(o.Domain) == 0 {
			// The edges of the bins are the domain of a bin-ordinal scale
			// that names none.
			for _, e := range o.Bins {
				o.Domain = append(o.Domain, e)
			}
		}
		if o.Zero == nil {
			// A scale with bins does not include zero by default.
			z := false
			o.Zero = &z
		}
	}
	if err := s.scaleRange(d, typ, st.path+"/range", &o); err != nil {
		return nil, fmt.Errorf("%s/range: %w", st.path, err)
	}
	return scale.New(o)
}

// binEdges reads the bins of a scale: an array of edges, or an object with
// a start, a stop and a step.
func binEdges(v any) []float64 {
	switch x := v.(type) {
	case []any:
		out := make([]float64, len(x))
		for i, e := range x {
			out[i] = expr.Number(e)
		}
		return out
	case map[string]any:
		start, stop, step := expr.Number(x["start"]), expr.Number(x["stop"]), expr.Number(x["step"])
		if step <= 0 || math.IsNaN(step) {
			return nil
		}
		var out []float64
		for v := start; v <= stop+step/1e9; v += step {
			out = append(out, v)
		}
		return out
	}
	return nil
}

// domain resolves the domain of a scale.
func (s *scope) domain(d *vega.Scale, typ string) ([]any, error) {
	dom := d.Domain
	switch {
	case dom.Signal != "":
		v, err := s.eval(dom.Signal, nil)
		if err != nil {
			return nil, err
		}
		l, _ := v.([]any)
		return l, nil
	case dom.Values != nil:
		out := make([]any, len(dom.Values))
		for i, v := range dom.Values {
			x, err := s.value(v)
			if err != nil {
				return nil, err
			}
			out[i] = x
		}
		return out, nil
	case dom.Data != nil:
		return s.dataDomain(dom.Data, typ)
	}
	return nil, nil
}

// fieldRef is one field of one dataset, or a list of values that the spec
// gives.
type fieldRef struct {
	data   string
	field  vega.Field
	values []any
}

// signalValues returns the values of a list that a signal in the fields of
// a domain gives. As in Vega, the list is a dataset whose field "data"
// holds each value. An object is a row, and any other value is the field
// "data" of a row of its own, so that [{data: 1}] and [1] give the same
// values.
func signalValues(l []any) []any {
	out := make([]any, len(l))
	for i, x := range l {
		if m, ok := x.(map[string]any); ok {
			out[i] = m["data"]
			continue
		}
		out[i] = x
	}
	return out
}

// dataDomain collects the domain of a scale from the data: the distinct
// values of a discrete scale, every value of a quantile scale, and the
// extent of any other.
func (s *scope) dataDomain(ref *vega.DataRef, typ string) ([]any, error) {
	var refs []fieldRef
	if !ref.Field.IsZero() {
		refs = append(refs, fieldRef{data: ref.Data, field: ref.Field})
	}
	for _, f := range ref.Fields {
		switch x := f.(type) {
		case string:
			refs = append(refs, fieldRef{data: ref.Data, field: vega.Field{Name: x}})
		case []any:
			refs = append(refs, fieldRef{values: x})
		case map[string]any:
			fr := fieldRef{data: ref.Data}
			if d, ok := x["data"].(string); ok {
				fr.data = d
			}
			switch fx := x["field"].(type) {
			case string:
				fr.field = vega.Field{Name: fx}
			case map[string]any:
				if sig, ok := fx["signal"].(string); ok {
					fr.field = vega.Field{Signal: sig}
				}
			}
			if sig, ok := x["signal"].(string); ok {
				v, err := s.eval(sig, nil)
				if err != nil {
					return nil, err
				}
				l, _ := v.([]any)
				fr = fieldRef{values: signalValues(l)}
			}
			refs = append(refs, fr)
		}
	}
	var vals []any
	var rows []dataflow.Row
	for _, r := range refs {
		if r.data == "" {
			vals = append(vals, r.values...)
			rows = append(rows, make([]dataflow.Row, len(r.values))...)
			continue
		}
		data, err := s.dataset(r.data)
		if err != nil {
			return nil, err
		}
		name := r.field.Name
		if r.field.Signal != "" {
			v, err := s.eval(r.field.Signal, nil)
			if err != nil {
				return nil, err
			}
			name = expr.String(v)
		}
		f := dataflow.FieldAccessor(name)
		for _, row := range data {
			vals = append(vals, f.Get(row))
			rows = append(rows, row)
		}
	}
	if !scale.Discrete(typ) {
		if typ == "quantile" {
			return validValues(vals), nil
		}
		return extentOf(vals), nil
	}
	// The distinct values, in the order they first appear.
	var out []any
	firsts := map[string]int{}
	groups := [][]dataflow.Row{}
	for i, v := range vals {
		k := domainKey(v)
		if j, ok := firsts[k]; ok {
			groups[j] = append(groups[j], rows[i])
			continue
		}
		firsts[k] = len(out)
		out = append(out, v)
		groups = append(groups, []dataflow.Row{rows[i]})
	}
	if len(ref.Sort) > 0 {
		if err := sortDomain(out, groups, ref.Sort); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// sortDomain sorts a discrete domain: true sorts the values, and an object
// sorts them by an aggregate of a field over the rows of each value.
func sortDomain(vals []any, groups [][]dataflow.Row, raw []byte) error {
	var spec any
	if err := json.Unmarshal(raw, &spec); err != nil {
		return err
	}
	idx := make([]int, len(vals))
	for i := range idx {
		idx[i] = i
	}
	desc := false
	var keys []any
	switch x := spec.(type) {
	case bool:
		if !x {
			return nil
		}
	case map[string]any:
		desc = x["order"] == "descending"
		if op, ok := x["op"].(string); ok {
			var f *dataflow.Accessor
			if name, ok := x["field"].(string); ok {
				f = dataflow.FieldAccessor(name)
			}
			keys = make([]any, len(vals))
			for i, g := range groups {
				keys[i] = aggregateOf(op, f, g)
			}
		}
	}
	cmp := func(a, b int) int {
		var c int
		if keys != nil {
			c = compare(keys[a], keys[b])
		} else {
			c = compare(vals[a], vals[b])
		}
		if desc {
			c = -c
		}
		return c
	}
	slices.SortStableFunc(idx, cmp)
	sorted := make([]any, len(vals))
	for i, j := range idx {
		sorted[i] = vals[j]
	}
	copy(vals, sorted)
	return nil
}

// aggregateOf computes an aggregate of a field over rows, to sort a domain.
func aggregateOf(op string, f *dataflow.Accessor, rows []dataflow.Row) any {
	var nums []float64
	if f != nil {
		for _, r := range rows {
			v := f.Get(r)
			if v == nil || expr.IsUndefined(v) {
				continue
			}
			if x := expr.Number(v); !math.IsNaN(x) {
				nums = append(nums, x)
			}
		}
	}
	switch op {
	case "count":
		return float64(len(rows))
	case "sum":
		t := 0.0
		for _, x := range nums {
			t += x
		}
		return t
	case "mean", "average":
		if len(nums) == 0 {
			return expr.Undefined
		}
		t := 0.0
		for _, x := range nums {
			t += x
		}
		return t / float64(len(nums))
	case "min", "max":
		if len(nums) == 0 {
			return expr.Undefined
		}
		m := nums[0]
		for _, x := range nums {
			if (op == "min" && x < m) || (op == "max" && x > m) {
				m = x
			}
		}
		return m
	case "median":
		if len(nums) == 0 {
			return expr.Undefined
		}
		slices.Sort(nums)
		h := float64(len(nums)-1) / 2
		i := int(h)
		if i+1 >= len(nums) {
			return nums[i]
		}
		return nums[i] + (h-float64(i))*(nums[i+1]-nums[i])
	}
	return expr.Undefined
}

// compare compares two values in ascending order, with null and undefined
// first.
func compare(a, b any) int {
	if t, ok := a.(time.Time); ok {
		a = float64(t.UnixMilli())
	}
	if t, ok := b.(time.Time); ok {
		b = float64(t.UnixMilli())
	}
	return expr.Ascending(a, b)
}

// domainKey identifies a value of a discrete domain.
func domainKey(v any) string {
	switch x := v.(type) {
	case nil:
		return "\x00null"
	case string:
		return "s" + x
	case time.Time:
		return "n" + expr.String(float64(x.UnixMilli()))
	case float64:
		return "n" + expr.String(x)
	case bool:
		return "b" + expr.String(x)
	}
	if expr.IsUndefined(v) {
		return "\x00undefined"
	}
	return "o" + expr.String(v)
}

// validValues returns the values that are valid numbers.
func validValues(vals []any) []any {
	var out []any
	for _, v := range vals {
		if v == nil || expr.IsUndefined(v) {
			continue
		}
		if f := expr.Number(v); !math.IsNaN(f) {
			out = append(out, f)
		}
	}
	return out
}

// extentOf returns the least and the greatest valid value, or nil if no
// value is valid. Dates compare by their time.
func extentOf(vals []any) []any {
	var lo, hi any
	for _, v := range vals {
		if v == nil || expr.IsUndefined(v) {
			continue
		}
		f := expr.Number(v)
		if math.IsNaN(f) {
			continue
		}
		if lo == nil || f < expr.Number(lo) {
			lo = v
		}
		if hi == nil || f > expr.Number(hi) {
			hi = v
		}
	}
	if lo == nil {
		return nil
	}
	return []any{lo, hi}
}

// pieces returns the number of colors that a scale that cuts numbers into
// pieces takes from a ramp, one for each piece, as Vega counts them: one
// more than the thresholds of a threshold scale, one less than the edges
// of the bins of a bin-ordinal scale, and 5 for a quantile or a quantize
// scale. It returns 0 for any other scale.
func pieces(typ string, o *scale.Options) int {
	switch typ {
	case "threshold":
		return len(o.Domain) + 1
	case "bin-ordinal":
		if len(o.Bins) > 1 {
			return len(o.Bins) - 1
		}
	case "quantile", "quantize":
		return 5
	}
	return 0
}

// scaleRange resolves the range of a scale into the options. path names
// the range in the spec.
func (s *scope) scaleRange(d *vega.Scale, typ, path string, o *scale.Options) error {
	r := d.Range
	t := s.rt.theme
	switch {
	case r.Name != "" && r.Name != "width" && r.Name != "height":
		// The range of the config of the spec goes over the theme.
		if vals, ref, ok := s.namedRange(r.Name); ok && ref != nil {
			return s.scheme(ref, typ, path, o)
		} else if ok {
			o.Range = vals
			return nil
		}
		fallthrough
	case r.Name != "":
		switch r.Name {
		case "width":
			w, err := s.signal("width")
			if err != nil {
				return err
			}
			o.Range = []any{0.0, number(w, 0)}
		case "height":
			h, err := s.signal("height")
			if err != nil {
				return err
			}
			o.Range = []any{number(h, 0), 0.0}
			if scale.Discrete(typ) {
				// A discrete scale runs from the top, so its first value is
				// at the top.
				o.Range = []any{0.0, number(h, 0)}
			}
		case "symbol":
			o.Range = []any{"circle", "square", "triangle-up", "cross", "diamond", "triangle-right", "triangle-down", "triangle-left"}
		case "category":
			o.Range = strs(t.Category)
		case "ordinal", "ramp", "heatmap":
			seq := s.rampTheme().Sequential
			o.Range = ramp(seq, 9)
			if scale.Discrete(typ) || typ == "quantize" || typ == "quantile" || typ == "threshold" || typ == "bin-ordinal" {
				n := max(len(o.Domain), 2)
				if typ != "ordinal" {
					n = max(n-1, 2)
				}
				if c := pieces(typ, o); c > 0 {
					n = c
				}
				o.Range = ramp(seq, n)
			}
		case "diverging":
			o.Range = ramp(t.Diverging, 3)
		default:
			return fmt.Errorf("the range %q: %w", r.Name, vega.ErrInvalidSpec)
		}
	case r.Values != nil:
		out := make([]any, len(r.Values))
		for i, v := range r.Values {
			x, err := s.value(v)
			if err != nil {
				return err
			}
			if v.Signal == "" {
				// A color that the range names flips in the dark mode, as
				// a color of an encoding does (D48 part 1).
				x = s.rt.specColor(x)
			}
			out[i] = x
		}
		o.Range = out
	case r.Signal != "":
		v, err := s.eval(r.Signal, nil)
		if err != nil {
			return err
		}
		l, _ := v.([]any)
		o.Range = l
	case r.Scheme != nil:
		return s.scheme(r.Scheme, typ, path, o)
	case !r.Step.IsZero():
		v, err := s.value(r.Step)
		if err != nil {
			return err
		}
		o.RangeStep = number(v, 20)
	case r.Data != nil:
		vals, err := s.dataDomain(r.Data, "ordinal")
		if err != nil {
			return err
		}
		o.Range = vals
	}
	return nil
}

// scheme resolves a scheme of a range into the options, as Vega does
// (D46). A scheme that Vega names, such as viridis or tableau10, or a list
// of colors, draws in its own colors, in either mode. A sequential or a
// diverging scale maps through a continuous scheme, from the first end of
// its extent to the other, and reverse swaps the ends. Any other scale
// takes colors of the scheme: each color of a discrete scheme, or as many
// as the scale needs, and samples of a continuous scheme without its ends.
// A name that Vega does not have is an issue, and takes the colors of the
// theme.
func (s *scope) scheme(ref *vega.SchemeRef, typ, path string, o *scale.Options) error {
	o.Scheme = true
	v, err := s.value(ref.Scheme)
	if err != nil {
		return err
	}
	count := 0
	if !ref.Count.IsZero() {
		c, err := s.value(ref.Count)
		if err != nil {
			return err
		}
		count = int(number(c, 0))
	}
	var sch scale.Scheme
	if l, ok := v.([]any); ok {
		sch = scale.Scheme{Continuous: true, Interpolate: o.Interpolate}
		for _, c := range l {
			sch.Colors = append(sch.Colors, expr.String(c))
		}
	} else if sch, ok = scale.NamedScheme(expr.String(v)); !ok {
		if err := s.rt.issue(vega.ErrInvalidSpec, path+"/scheme", "the scheme %q, which Vega does not have", expr.String(v)); err != nil {
			return err
		}
		o.Range = s.themeScheme(typ, count, len(o.Domain))
		return nil
	}
	lo, hi := 0.0, 1.0
	if !ref.Extent.IsZero() {
		e, err := s.value(ref.Extent)
		if err != nil {
			return err
		}
		if l, ok := e.([]any); ok && len(l) >= 2 {
			lo, hi = number(l[0], 0), number(l[len(l)-1], 1)
		}
	}
	// at is the scheme over its extent. A discrete scale reverses its
	// colors itself, so reverse swaps the ends of the interpolator only.
	at := func(t float64) string { return sch.At(lo + t*(hi-lo)) }
	n := len(o.Domain)
	switch {
	case !scale.Discrete(typ) && !discretizing(typ):
		if !sch.Continuous {
			// A sequential or a diverging scale interpolates between all the
			// colors, and any other takes one color for each value of the
			// domain.
			o.Range = strs(sch.Colors)
			if !colorRamp(o) {
				o.Range = o.Range[:min(n, len(o.Range))]
			}
			return nil
		}
		if o.Reverse {
			o.Interpolator = func(t float64) string { return sch.At(hi + t*(lo-hi)) }
		} else {
			o.Interpolator = at
		}
		o.Range = scale.Sample(at, n)
		return nil
	case typ == "threshold":
		n++
	case typ == "bin-ordinal":
		if p := pieces(typ, o); p > 0 {
			n = p
		} else {
			n--
		}
	case typ == "ordinal":
		if count > 0 {
			n = count
		}
		if n <= 0 {
			n = 5
		}
	case typ == "quantize" || typ == "quantile":
		n = 5
		if count > 0 {
			n = count
		}
	}
	switch {
	case sch.Continuous:
		o.Range = scale.Sample(at, n)
	case typ == "ordinal":
		o.Range = strs(sch.Colors)
	default:
		o.Range = strs(sch.Colors[:max(0, min(n, len(sch.Colors)))])
	}
	return nil
}

// discretizing reports whether a scale type cuts a continuous domain into
// the values of a discrete range.
func discretizing(typ string) bool {
	switch typ {
	case "quantize", "quantile", "threshold", "bin-ordinal":
		return true
	}
	return false
}

// colorRamp reports whether a continuous scale of colors is sequential or
// diverging, as Vega names it from the count of its domain and its middle.
// Such a scale interpolates over the whole of its range.
func colorRamp(o *scale.Options) bool {
	if o.Type == "time" || o.Type == "utc" {
		return false
	}
	n := len(o.Domain)
	if !math.IsNaN(o.DomainMid) {
		n++
	}
	return n <= 3
}

// themeScheme returns the colors of the theme for a scheme that Vega does
// not have: the categorical colors for an ordinal scale with no count, and
// the sequential ramp for any other scale.
func (s *scope) themeScheme(typ string, count, n int) []any {
	if typ == "ordinal" && count == 0 {
		return strs(s.rt.theme.Category)
	}
	if count <= 0 {
		count = 9
		if scale.Discrete(typ) || typ == "quantize" || typ == "quantile" || typ == "threshold" {
			count = max(n, 2)
		}
	}
	return ramp(s.rampTheme().Sequential, count)
}

// rampTheme returns the theme of the render with the background that the
// chart draws, from which a ramp starts (D35). A background that the caller
// gives, or that zuzu.background gives (D37), is already the background of
// the theme, and wins. Else it is the
// background of the spec, or of the config of the spec, when either sets
// an opaque color. A color with some opacity counts as drawn over the
// background of the theme.
func (s *scope) rampTheme() theme.Theme {
	t := s.rt.theme
	if s.rt.opts.hasBackground {
		return t
	}
	bg, err := s.signal("background")
	c, ok := bg.(string)
	if err != nil || !ok || c == "" {
		c, _ = s.rt.config["background"].(string)
	}
	over, ok1 := color.Parse(t.Background)
	v, ok2 := color.Parse(c)
	if !ok1 || !ok2 || v.A <= 0 {
		return t
	}
	t.Background = color.Mix(over, color.RGBA{R: v.R, G: v.G, B: v.B, A: 1}, v.A, "rgb").Hex()
	return t
}

// ramp samples n colors of a ramp.
func ramp(fn func(float64) string, n int) []any {
	out := make([]any, n)
	for i := range out {
		f := 0.0
		if n > 1 {
			f = float64(i) / float64(n-1)
		}
		out[i] = fn(f)
	}
	return out
}

// strs converts strings to values.
func strs(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}
