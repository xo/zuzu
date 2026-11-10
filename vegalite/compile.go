package vegalite

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/vega"
)

// schemaVega is the $schema of a compiled spec.
const schemaVega = "https://vega.github.io/schema/vega/v6.json"

// The defaults of the size of a view, as Vega-Lite documents them.
const (
	continuousSize = 300
	discreteStep   = 20
	defaultColor   = "#4c78a8"
)

// CompileOption is a setting of Compile.
type CompileOption func(*compiler)

// WithStrict sets the feature mode (D26): a feature that the compiler does
// not support is an error, rather than an issue of the report.
func WithStrict(strict bool) CompileOption {
	return func(c *compiler) { c.strict = strict }
}

// Compile turns a Vega-Lite spec into a Vega spec. The report names each
// part that zuzu did not compile, in the loose mode.
func Compile(s *Spec, opts ...CompileOption) (*vega.Spec, vega.Report, error) {
	c := &compiler{datasets: map[string]*dataNode{}, parsed: map[int]map[string]bool{}}
	for _, o := range opts {
		o(c)
	}
	out, err := c.compile(s)
	if err != nil {
		return nil, c.report, err
	}
	return out, c.report, nil
}

// compiler holds the state of one compile.
type compiler struct {
	strict bool
	report vega.Report
	data   []vega.Data
	// datasets holds the dataset of each named dataset of the spec that a
	// view reads.
	datasets map[string]*dataNode
	// parsed holds, for each dataset of inline values that derives, the
	// fields that a formula already parsed.
	parsed  map[int]map[string]bool
	nData   int
	nSource int
	// cfg is the config of the spec.
	cfg *config
	// top is the spec being compiled.
	top *Spec
	// signals are the signals of the parameters of the spec, and
	// signalNames their names.
	signals     []vega.Signal
	signalNames map[string]bool
	// reserved holds the numbers of datasets that layers over inline
	// values take.
	reserved []reservation
}

// reservation is a number of a dataset that a layer over a dataset takes.
type reservation struct {
	data   string
	number int
}

// issue handles a feature that the compiler does not support.
func (c *compiler) issue(path, format string, args ...any) error {
	is := vega.Issue{Err: vega.ErrUnsupported, Path: path, Message: fmt.Sprintf(format, args...)}
	if c.strict {
		return is
	}
	c.report = append(c.report, is)
	return nil
}

// config holds the parts of the config of a spec that change the data or
// the scales. The parts that change only the look are left to the theme of
// zuzu (D12).
type config struct {
	// invalid is false when the config keeps invalid values, with
	// "mark": {"invalid": null}.
	keepInvalid bool
	// zero is the default zero of the scales of position.
	zero *bool
	// step is the default step of a discrete scale of position.
	step float64
	// marks holds the config of each type of mark, such as "bar".
	marks map[string]map[string]any
}

// readConfig reads the parts of the config that the compiler follows.
func readConfig(raw []byte) *config {
	cfg := &config{}
	if len(raw) == 0 {
		return cfg
	}
	var m struct {
		Mark  map[string]any `json:"mark"`
		Scale map[string]any `json:"scale"`
		View  map[string]any `json:"view"`
	}
	if json.Unmarshal(raw, &m) != nil {
		return cfg
	}
	var all map[string]any
	if json.Unmarshal(raw, &all) == nil {
		cfg.marks = map[string]map[string]any{}
		for k := range vegaMark {
			if x, ok := all[k].(map[string]any); ok {
				cfg.marks[k] = x
			}
		}
	}
	if v, ok := m.Mark["invalid"]; ok && v == nil {
		cfg.keepInvalid = true
	}
	if z, ok := m.Scale["zero"].(bool); ok {
		cfg.zero = &z
	}
	if st, ok := m.View["step"].(float64); ok {
		cfg.step = st
	}
	return cfg
}

// compile compiles the spec of the top level.
func (c *compiler) compile(s *Spec) (*vega.Spec, error) {
	c.top = s
	c.cfg = readConfig(s.Config)
	out := &vega.Spec{Schema: schemaVega}
	if s.Description != nil {
		out.Description = *s.Description
	}
	out.Padding = s.Padding
	if out.Padding.IsZero() {
		out.Padding = lit(5)
	}
	out.Autosize = s.Autosize
	out.Background = s.Background
	if s.Title != nil {
		t := *s.Title
		t.Anchor = lit("start")
		out.Title = &t
	}
	if isConcat(s) {
		if err := c.concat(s, out, nil, "", map[string]string{}); err != nil {
			return nil, err
		}
		out.Signals = append(out.Signals, c.signals...)
		out.Data = c.data
		return out, nil
	}
	if _, _, _, _, _, ok, err := readFacet(s, nil); err != nil {
		return nil, err
	} else if ok {
		if err := c.facet(s, out, nil, "", map[string]string{}); err != nil {
			return nil, err
		}
		out.Signals = append(out.Signals, c.signals...)
		out.Data = c.data
		return out, nil
	}
	prefix := ""
	if s.Name != "" {
		// A named view prefixes the names of its scales and its marks.
		prefix = varName(s.Name) + "_"
	}
	v := newView(prefix, c.cfg)
	v.scalePrefix = prefix
	if err := c.view(s, v, nil, "", map[string]string{}); err != nil {
		return nil, err
	}
	v.finish(out)
	c.mergeAggregates(out)
	out.Signals = append(out.Signals, c.signals...)
	out.Data = c.data
	return out, nil
}

// view compiles a spec into a view: a single view or a layer. Facets,
// repeats and concatenations come in their own functions.
func (c *compiler) view(s *Spec, v *view, src *dataNode, path string, repeat map[string]string) error {
	if err := c.params(s, path); err != nil {
		return err
	}
	if s.Data != nil {
		d, err := c.source(s.Data, path+"/data")
		if err != nil {
			return err
		}
		src = d
	}
	switch {
	case len(s.Layer) > 0:
		if len(s.Transform) > 0 {
			src = c.derive(src)
			if err := c.transforms(src, s.Transform, path+"/transform"); err != nil {
				return err
			}
		}
		if src != nil && !src.open && !s.expanded && len(s.Layer) > 1 && len(s.Transform) == 0 {
			// A layer over inline values has a dataset of its own, which
			// vega-lite.js names and later removes, so its number is
			// taken, unless the layers end up with one dataset.
			c.reserved = append(c.reserved, reservation{data: c.name(src), number: c.nData})
			c.nData++
		}
		if src != nil && len(s.Layer) > 1 {
			// The layers share the data, so each adds its transforms to a
			// dataset of its own.
			src.open = false
		}
		for i := range s.Layer {
			l := &s.Layer[i]
			inheritEncoding(l, s.Encoding)
			if l.Mark == nil && len(l.Layer) == 0 {
				continue
			}
			sub := v.layer(fmt.Sprintf("layer_%d_", i), v.prefix)
			if err := c.view(l, sub, src, fmt.Sprintf("%s/layer/%d", path, i), repeat); err != nil {
				return err
			}
		}
		v.size(s)
		return nil
	case s.Mark != nil:
		expanded, err := composite(s)
		if err != nil {
			return c.issue(path+"/mark", "%v", err)
		}
		if expanded != nil {
			if s.Data != nil {
				expanded.Data = nil
			}
			return c.view(expanded, v, src, path, repeat)
		}
		if layered := overlay(s); layered != nil {
			return c.view(layered, v, src, path, repeat)
		}
		v.size(s)
		return c.unit(s, v, src, path, repeat)
	case len(s.HConcat) > 0 || len(s.VConcat) > 0 || len(s.Concat) > 0:
		return c.issue(path, "a concatenation of views")
	case len(s.Facet) > 0 || len(s.Repeat) > 0:
		return c.issue(path, "a facet or a repeat")
	}
	return c.issue(path, "a spec with no mark, layer, facet or concatenation")
}

// inheritEncoding gives a layer the channels of its parent that it does not
// define.
func inheritEncoding(l *Spec, parent Encoding) {
	if len(parent) == 0 {
		return
	}
	if l.Encoding == nil {
		l.Encoding = Encoding{}
	}
	for k, ch := range parent {
		own, ok := l.Encoding[k]
		if !ok {
			l.Encoding[k] = ch
			continue
		}
		// A channel that both define takes the properties of the parent
		// that the layer does not give.
		if len(own.Defs) == 1 && len(ch.Defs) == 1 && !own.Array && !ch.Array {
			if merged, ok := mergeDefs(own.Defs[0], ch.Defs[0]); ok {
				l.Encoding[k] = Channel{Defs: []FieldDef{merged}}
			}
		}
	}
}

// mergeDefs gives a definition of a channel the properties of another that
// it does not have. It reports false when either has a value, which stands
// alone.
func mergeDefs(own, parent FieldDef) (FieldDef, bool) {
	if !own.Value.IsZero() || !parent.Value.IsZero() || !own.Datum.IsZero() || !parent.Datum.IsZero() {
		return own, false
	}
	a, err := json.Marshal(own)
	if err != nil {
		return own, false
	}
	b, err := json.Marshal(parent)
	if err != nil {
		return own, false
	}
	var am, bm map[string]jsontext.Value
	if json.Unmarshal(a, &am) != nil || json.Unmarshal(b, &bm) != nil {
		return own, false
	}
	for k, v := range bm {
		if _, ok := am[k]; !ok {
			am[k] = v
		}
	}
	c, err := json.Marshal(am)
	if err != nil {
		return own, false
	}
	var out FieldDef
	if json.Unmarshal(c, &out) != nil {
		return own, false
	}
	return out, true
}

// dataNode is a dataset of the compiled spec, which transforms can still be
// added to while nothing derives from it.
type dataNode struct {
	index int
	// open says that transforms can be added to the dataset.
	open bool
	// parse holds the fields of a source that the compiler parses as
	// dates.
	parse map[string]bool
	// values says that the source is inline values.
	values bool
}

// name returns the name of the dataset.
func (c *compiler) name(n *dataNode) string {
	return c.data[n.index].Name
}

// source adds the dataset of the data of a view, and returns it.
func (c *compiler) source(d *Data, path string) (*dataNode, error) {
	if d == nil {
		return nil, nil
	}
	vd := vega.Data{}
	if raw, ok := c.top.Datasets[d.Name]; ok && d.Name != "" {
		// A named dataset of the spec keeps its name, and each view that
		// reads it shares it.
		if n, ok := c.datasets[d.Name]; ok {
			return n, nil
		}
		vd.Name = d.Name
		d = &Data{Values: vega.Value{JSON: raw}, Format: d.Format}
	} else {
		vd.Name = fmt.Sprintf("source_%d", c.nSource)
		c.nSource++
	}
	n := &dataNode{parse: map[string]bool{}}
	if _, named := c.top.Datasets[vd.Name]; named {
		c.datasets[vd.Name] = n
	}
	switch {
	case !d.Values.IsZero():
		vd.Values = d.Values
		n.values = true
	case d.URL != "":
		vd.URL = lit(d.URL)
		typ := ""
		if d.Format != nil {
			typ = d.Format.Type
		}
		if typ == "" {
			switch strings.ToLower(pathExt(d.URL)) {
			case ".csv":
				typ = "csv"
			case ".tsv":
				typ = "tsv"
			default:
				typ = "json"
			}
		}
		vd.Format = &vega.Format{Type: typ}
		if typ == "csv" {
			vd.Format.Delimiter = ","
		}
	case len(d.Sequence) > 0:
		var seq map[string]any
		if err := json.Unmarshal(d.Sequence, &seq); err != nil {
			return nil, err
		}
		as := "data"
		if a, ok := seq["as"].(string); ok {
			as = a
		}
		params := vega.Extra{"start": mustJSON(seq["start"]), "stop": mustJSON(seq["stop"]), "as": mustJSON(as)}
		if st, ok := seq["step"]; ok {
			params["step"] = mustJSON(st)
		}
		vd.Transform = append(vd.Transform, vega.Transform{Type: "sequence", Params: params})
	case d.Name != "":
		vd.Name = d.Name
		n.values = true
	default:
		// A sphere or a graticule comes with the projections of phase 5.
		return nil, c.issue(path, "this kind of data")
	}
	if d.Format != nil {
		if vd.Format == nil {
			vd.Format = &vega.Format{}
		}
		if d.Format.Type != "" {
			vd.Format.Type = d.Format.Type
		}
		vd.Format.Property = d.Format.Property
		vd.Format.Feature = d.Format.Feature
		vd.Format.Mesh = d.Format.Mesh
		if d.Format.Delimiter != "" {
			vd.Format.Delimiter = d.Format.Delimiter
		}
		vd.Format.Parse = d.Format.Parse
	}
	n.index = len(c.data)
	n.open = !n.values
	c.data = append(c.data, vd)
	return n, nil
}

// pathExt returns the extension of a url.
func pathExt(u string) string {
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	return path.Ext(u)
}

// mustJSON writes a value as JSON.
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	return b
}

// derive returns a dataset that transforms can be added to: the dataset
// itself while it is open, or a new dataset whose source it is.
func (c *compiler) derive(src *dataNode) *dataNode {
	if src == nil {
		return nil
	}
	if src.open {
		src.open = false
		return src
	}
	vd := vega.Data{Name: fmt.Sprintf("data_%d", c.nData), Source: vega.Strings{List: []string{c.name(src)}}}
	c.nData++
	n := &dataNode{index: len(c.data), parse: src.parse}
	c.data = append(c.data, vd)
	return n
}

// add adds transforms to a dataset.
func (c *compiler) add(n *dataNode, ts ...vega.Transform) {
	c.data[n.index].Transform = append(c.data[n.index].Transform, ts...)
}

// transform makes a Vega transform from its type and its parameters.
func transform(typ string, params map[string]any) vega.Transform {
	t := vega.Transform{Type: typ, Params: vega.Extra{}}
	keys := slices.Sorted(maps.Keys(params))
	for _, k := range keys {
		t.Params[k] = mustJSON(params[k])
	}
	return t
}

// parseFields adds fields of a source to the parse of its format, as dates
// or as numbers.
func (c *compiler) parseFields(n *dataNode, fields []string, typ string) {
	if n == nil {
		return
	}
	root := n
	for root != nil && c.data[root.index].Source.List != nil {
		// A derived dataset parses in its source.
		name := c.data[root.index].Source.List[0]
		root = nil
		for i, d := range c.data {
			if d.Name == name {
				root = &dataNode{index: i}
			}
		}
	}
	if root == nil {
		return
	}
	d := &c.data[root.index]
	if d.URL.IsZero() && d.Values.IsZero() {
		return
	}
	if !d.Values.IsZero() && root.index != n.index {
		// Inline values parse in the dataset that derives from them, with
		// a formula for each field, and the values stay as they are.
		fn := map[string]string{"date": "toDate", "number": "toNumber"}[typ]
		parsed := c.parsed[n.index]
		if parsed == nil {
			parsed = map[string]bool{}
			c.parsed[n.index] = parsed
		}
		for _, f := range fields {
			if f == "" || parsed[f] {
				continue
			}
			parsed[f] = true
			c.add(n, transform("formula", map[string]any{"expr": fn + "(" + fieldRef(f) + ")", "as": f}))
		}
		return
	}
	parse := map[string]any{}
	if d.Format != nil && !d.Format.Parse.IsZero() {
		_ = d.Format.Parse.Decode(&parse)
	}
	changed := false
	for _, f := range fields {
		if _, ok := parse[f]; !ok && f != "" {
			parse[f] = typ
			changed = true
		}
	}
	if !changed {
		return
	}
	if d.Format == nil {
		d.Format = &vega.Format{}
	}
	d.Format.Parse = lit(parse)
}

// valueOf decodes a Value.
func valueOf(v vega.Value) any {
	if v.Signal != "" {
		return map[string]any{"signal": v.Signal}
	}
	var out any
	_ = v.Decode(&out)
	return out
}

// str reads a Value as a string.
func str(v vega.Value) string {
	x := valueOf(v)
	if x == nil {
		return ""
	}
	return expr.String(x)
}

// overlay turns an area or a line whose mark asks for a line or points over
// it into a layer of the marks, as Vega-Lite does. It returns nil for any
// other spec.
func overlay(s *Spec) *Spec {
	if s.Mark.Type != "area" && s.Mark.Type != "line" {
		return nil
	}
	want := func(k string) bool {
		raw, ok := s.Mark.Props[k]
		if !ok {
			return false
		}
		var v any
		if json.Unmarshal(raw, &v) != nil {
			return false
		}
		switch x := v.(type) {
		case bool:
			return x
		case map[string]any:
			return true
		}
		return false
	}
	line, point := s.Mark.Type == "area" && want("line"), want("point")
	if !line && !point {
		return nil
	}
	base := &Mark{Type: s.Mark.Type, Props: Extra{}, object: true}
	for k, v := range s.Mark.Props {
		if k != "line" && k != "point" {
			base.Props[k] = v
		}
	}
	out := *s
	// The caller already read the data of the spec.
	out.Mark, out.Data = nil, nil
	out.expanded = true
	out.Layer = []Spec{{Mark: base}}
	if line {
		out.Layer = append(out.Layer, Spec{Mark: &Mark{Type: "line"}, stackAs: s.Mark.Type})
	}
	if point {
		out.Layer = append(out.Layer, Spec{Mark: &Mark{Type: "point", Props: Extra{"filled": []byte("true")}, object: true}, stackAs: s.Mark.Type})
	}
	return &out
}

// mergeAggregates moves the aggregates of datasets that derive from the
// same dataset, with the same group by fields, into that dataset as one
// aggregate, as vega-lite.js does. It does so only when every dataset that
// derives from the parent starts with such an aggregate and no mark reads
// the parent.
func (c *compiler) mergeAggregates(out *vega.Spec) {
	children := map[string][]int{}
	for i, d := range c.data {
		if len(d.Source.List) == 1 {
			children[d.Source.List[0]] = append(children[d.Source.List[0]], i)
		}
	}
	read := marksData(out.Marks)
	for _, s := range out.Scales {
		if s.Domain.Data != nil {
			read[s.Domain.Data.Data] = true
			for _, f := range s.Domain.Data.Fields {
				if m, ok := f.(map[string]any); ok {
					read[fmt.Sprint(m["data"])] = true
				}
			}
		}
	}
	for pi := range c.data {
		parent := &c.data[pi]
		kids := children[parent.Name]
		if len(kids) < 2 || read[parent.Name] {
			continue
		}
		var groupby []byte
		ok := true
		for _, k := range kids {
			ts := c.data[k].Transform
			if len(ts) == 0 || ts[0].Type != "aggregate" {
				ok = false
				break
			}
			g := ts[0].Params["groupby"]
			if groupby == nil {
				groupby = g
			} else if string(g) != string(groupby) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		// The last child takes in the aggregates of the others, from the
		// last to the first. The ops group by field, then by op.
		type measure struct {
			field, op any
			as        []any
		}
		var measures []*measure
		seen := map[string]bool{}
		for _, k := range slices.Backward(kids) {
			t := c.data[k].Transform[0]
			var o, f, a []any
			_ = json.Unmarshal(t.Params["ops"], &o)
			_ = json.Unmarshal(t.Params["fields"], &f)
			_ = json.Unmarshal(t.Params["as"], &a)
			for j := range a {
				name := fmt.Sprint(a[j])
				if seen[name] {
					continue
				}
				seen[name] = true
				var m *measure
				for _, x := range measures {
					if fmt.Sprint(x.field) == fmt.Sprint(f[j]) && fmt.Sprint(x.op) == fmt.Sprint(o[j]) {
						m = x
					}
				}
				if m == nil {
					at := len(measures)
					for x, other := range slices.Backward(measures) {
						if fmt.Sprint(other.field) == fmt.Sprint(f[j]) {
							at = x + 1
							break
						}
					}
					m = &measure{field: f[j], op: o[j]}
					measures = slices.Insert(measures, at, m)
				}
				m.as = append(m.as, a[j])
			}
			c.data[k].Transform = c.data[k].Transform[1:]
		}
		var ops, fields, as []any
		for _, m := range measures {
			for _, a := range m.as {
				ops, fields, as = append(ops, m.op), append(fields, m.field), append(as, a)
			}
		}
		var g []any
		_ = json.Unmarshal(groupby, &g)
		parent.Transform = append(parent.Transform, transform("aggregate", map[string]any{
			"groupby": g, "ops": ops, "fields": fields, "as": as,
		}))
		c.reverseNames(out, read, kids)
	}
	c.removeEmpty(read)
	c.mergeIdentical(out, read)
	c.release(out, read)
}

// removeEmpty removes each dataset that derives from one other dataset
// with no transforms of its own, and that no mark reads: its datasets then
// derive from its source.
func (c *compiler) removeEmpty(read map[string]bool) {
	for {
		removed := false
		for i, d := range c.data {
			if len(d.Source.List) != 1 || len(d.Transform) > 0 || read[d.Name] || !strings.HasPrefix(d.Name, "data_") {
				continue
			}
			used := false
			for _, other := range c.data {
				if len(other.Source.List) == 1 && other.Source.List[0] == d.Name {
					used = true
				}
			}
			if !used {
				continue
			}
			for j := range c.data {
				if len(c.data[j].Source.List) == 1 && c.data[j].Source.List[0] == d.Name {
					c.data[j].Source = d.Source
				}
			}
			c.data = append(c.data[:i], c.data[i+1:]...)
			removed = true
			break
		}
		if !removed {
			return
		}
	}
}

// marksData returns the datasets that marks read, with the marks of
// groups.
func marksData(ms []vega.Mark) map[string]bool {
	out := map[string]bool{}
	var walk func(ms []vega.Mark)
	walk = func(ms []vega.Mark) {
		for _, m := range ms {
			if m.From != nil {
				out[m.From.Data] = true
				if m.From.Facet != nil {
					out[m.From.Facet.Data] = true
				}
			}
			walk(m.Marks)
		}
	}
	walk(ms)
	return out
}
