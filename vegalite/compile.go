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
	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// schemaVega is the $schema of a compiled spec.
const schemaVega = "https://vega.github.io/schema/vega/v6.json"

// The defaults of the size of a view, as Vega-Lite documents them.
const (
	continuousSize = 300
	discreteStep   = 20
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
	if c.markColor {
		// The scale gives the color of a mark of the theme, the first
		// categorical color, which the render fills from its theme, so
		// that a color with conditions alone falls back to it.
		out.Scales = append(out.Scales, vega.Scale{
			Name: MarkColorScale, Type: "ordinal",
			Domain: vega.ScaleDomain{Values: []vega.Value{vega.Lit(MarkColorScale)}},
			Range:  vega.ScaleRange{Name: "category"},
		})
	}
	return out, c.report, nil
}

// MarkColorScale is the name of the scale of one value that gives the
// color of a mark of the theme. vega-lite.js writes the color itself in
// its place, because it knows the config, where the compile of zuzu does
// not know the theme of the render.
const MarkColorScale = "zuzu_mark_color"

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
	// shareSources says that the views of a concatenation can read the
	// same source, so a source takes no transforms of one view.
	shareSources bool
	// shared holds the sources that several views share after a merge.
	shared map[string]bool
	// produced holds the fields that the transforms of a layer make, which
	// its views do not parse.
	produced map[string]bool
	// flattens are the expressions that flatten the fields inside objects
	// before the transforms of a view.
	flattens []string
	// projStack holds the projections of the specs around the view being
	// compiled, and proj the projection that the views share.
	projStack []jsontext.Value
	proj      *projComp
	// concatPrefix is the prefix of the names of the views of a
	// concatenation inside another.
	concatPrefix string
	// nested says that the view being compiled lies inside another, so
	// the optimizations wait for the whole spec.
	nested bool
	// innerFacet says that the facet being compiled is the cells of
	// another facet, whose scales take no prefix.
	innerFacet bool
	// markColor says that a mark falls back to the color of a mark of the
	// theme, through the scale MarkColorScale.
	markColor bool
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

// config holds the parts of the config of a spec that change the data, the
// scales or the place of a mark. The parts that change only the look are
// left to the theme of zuzu (D12).
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
	// mark is the config of every mark.
	mark map[string]any
	// composites holds the config of each composite mark, such as
	// "boxplot".
	composites map[string]map[string]any
	// styles holds each style of the config, by name.
	styles map[string]map[string]any
	// facetSpacing and concatSpacing are the default spacing between the
	// cells of a facet and between the views of a concatenation, or nil
	// for the default of 20.
	facetSpacing, concatSpacing any
	// barPaddingInner is the barBandPaddingInner of the config of the
	// scales, or nil for the default.
	barPaddingInner *float64
	// xLabelAngle says that the config of an axis of x, which passes on to
	// the Vega config, gives a labelAngle, which wins over the turn of the
	// labels of a discrete axis of x.
	xLabelAngle bool
	// grid holds the grid of the config of the axes of each channel of
	// position, x or y, when the config gives one: the grid of axisX or
	// axisY, else the grid of axis.
	grid map[string]bool
	// axisTitle holds the title of the config of the axes of each channel
	// of position, in the same way, when the config gives one, such as
	// null for no title.
	axisTitle map[string]any
	// specStyles holds the kinds of mark that the config of the spec
	// styles, through specStyle.
	specStyles map[string]bool
	// numberFormat is the default format of a quantitative field, in its
	// axis, its legend, its text and its tooltip, and customFormats says
	// that the config turns on customFormatTypes, which name functions of
	// JavaScript that zuzu does not have.
	numberFormat  string
	customFormats bool
}

// barBandPaddingInner returns the inner padding of the bands of a bar: the
// barBandPaddingInner of the config of the scales, or 0.1, the default of
// Vega-Lite.
func (c *config) barBandPaddingInner() float64 {
	if c != nil && c.barPaddingInner != nil {
		return *c.barPaddingInner
	}
	return 0.1
}

// gridOf returns the grid of the config of the axes of a channel, and
// whether the config gives one.
func (c *config) gridOf(ch string) (bool, bool) {
	if c == nil {
		return false, false
	}
	g, ok := c.grid[ch]
	return g, ok
}

// readConfig reads the parts of the config that the compiler follows.
func readConfig(raw []byte) *config {
	cfg := &config{}
	if len(raw) == 0 {
		return cfg
	}
	var m struct {
		Mark   map[string]any `json:"mark"`
		Scale  map[string]any `json:"scale"`
		View   map[string]any `json:"view"`
		Facet  map[string]any `json:"facet"`
		Concat map[string]any `json:"concat"`
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
		cfg.composites = map[string]map[string]any{}
		for _, k := range []string{"boxplot", "errorbar", "errorband"} {
			if x, ok := all[k].(map[string]any); ok {
				cfg.composites[k] = x
			}
		}
		if st, ok := all["style"].(map[string]any); ok {
			cfg.styles = map[string]map[string]any{}
			for k, x := range st {
				if x, ok := x.(map[string]any); ok {
					cfg.styles[k] = x
				}
			}
		}
	}
	cfg.mark = m.Mark
	if nf, ok := all["numberFormat"].(string); ok {
		cfg.numberFormat = nf
	}
	if cf, ok := all["customFormatTypes"].(bool); ok && cf {
		cfg.customFormats = true
	}
	if v, ok := m.Mark["invalid"]; ok && v == nil {
		cfg.keepInvalid = true
	}
	if z, ok := m.Scale["zero"].(bool); ok {
		cfg.zero = &z
	}
	if p, ok := m.Scale["barBandPaddingInner"].(float64); ok {
		cfg.barPaddingInner = &p
	}
	if st, ok := m.View["step"].(float64); ok {
		cfg.step = st
	}
	cfg.facetSpacing = m.Facet["spacing"]
	cfg.concatSpacing = m.Concat["spacing"]
	for _, k := range []string{"axis", "axisX", "axisBottom", "axisTop", "axisBand"} {
		if x, ok := all[k].(map[string]any); ok {
			if _, ok := x["labelAngle"]; ok {
				cfg.xLabelAngle = true
			}
		}
	}
	for ch, k := range map[string]string{"x": "axisX", "y": "axisY"} {
		for _, name := range []string{"axis", k} {
			x, _ := all[name].(map[string]any)
			if g, ok := x["grid"].(bool); ok {
				if cfg.grid == nil {
					cfg.grid = map[string]bool{}
				}
				cfg.grid[ch] = g
			}
			if t, ok := x["title"]; ok {
				if cfg.axisTitle == nil {
					cfg.axisTitle = map[string]any{}
				}
				cfg.axisTitle[ch] = t
			}
		}
	}
	return cfg
}

// spacing returns the padding of the layout of a facet or a
// concatenation: the spacing of the spec, else the spacing of the config,
// else 20. A spacing can be one number or an object with a row and a
// column, as the padding of a layout of Vega can. An object that leaves out
// the row or the column takes the spacing of the config, or 20, for it.
func spacing(s *Spec, config any) any {
	def := config
	if def == nil {
		def = 20.0
	}
	v := valueOf(s.Spacing)
	m, ok := v.(map[string]any)
	if !ok {
		if v != nil {
			return v
		}
		return def
	}
	out := map[string]any{}
	for _, key := range []string{"row", "column"} {
		x := m[key]
		if x == nil {
			x = def
			if dm, ok := def.(map[string]any); ok {
				x = dm[key]
			}
		}
		if x == nil {
			x = 20.0
		}
		out[key] = x
	}
	return out
}

// passedConfig are the parts of the config of a spec that the Vega config
// holds with the same names and meaning: the styles of marks, the guides,
// the ranges and the background, which the runtime applies, as
// vega-lite.js passes them on, the mode, the scheme and the background of
// the theme that a spec asks for, and its look and the seed of the look
// (D32, D37, D40).
var passedConfig = []string{
	"style", "axis", "axisX", "axisY", "axisTop", "axisBottom", "axisLeft", "axisRight", "axisBand",
	"legend", "title", "range", "background", theme.ConfigMode, theme.ConfigScheme, theme.ConfigBackground,
	theme.ConfigLook, theme.ConfigLookSeed,
}

// viewProps are the properties of the view of Vega-Lite that paint the
// group of a view, its cell: the properties of a mark that a group has.
var viewProps = []string{
	"fill", "fillOpacity", "opacity", "stroke", "strokeOpacity", "strokeWidth", "strokeDash",
	"strokeDashOffset", "strokeCap", "strokeJoin", "strokeMiterLimit", "cornerRadius", "cursor",
}

// viewStyle returns the properties of a view, of the spec or of the config,
// that paint its cell, or nil for a view with none.
func viewStyle(raw []byte) map[string]any {
	var m map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &m) != nil {
		return nil
	}
	var out map[string]any
	for _, k := range viewProps {
		if v, ok := m[k]; ok {
			if out == nil {
				out = map[string]any{}
			}
			out[k] = v
		}
	}
	return out
}

// styleConfig returns the parts of the config of a spec that pass on to the
// Vega config, or nil for a config with none. The paint of the view of the
// config goes in the style cell, as vega-lite.js puts it, under a style
// cell of the spec (D48 part 5).
func styleConfig(raw []byte) vega.Config {
	var m map[string]jsontext.Value
	if len(raw) == 0 || json.Unmarshal(raw, &m) != nil {
		return nil
	}
	var out vega.Config
	for _, k := range passedConfig {
		if v, ok := m[k]; ok {
			if out == nil {
				out = vega.Config{}
			}
			out[k] = v
		}
	}
	if ms := markStyles(m); len(ms) > 0 {
		var styles map[string]any
		_ = json.Unmarshal(out["style"], &styles)
		if styles == nil {
			styles = map[string]any{}
		}
		for k, v := range ms {
			if _, ok := styles[k]; !ok {
				styles[k] = v
			}
		}
		b, err := json.Marshal(styles, json.Deterministic(true))
		if err == nil {
			if out == nil {
				out = vega.Config{}
			}
			out["style"] = b
		}
	}
	if cell := viewStyle(m["view"]); cell != nil {
		var styles map[string]any
		_ = json.Unmarshal(out["style"], &styles)
		if styles == nil {
			styles = map[string]any{}
		}
		if own, ok := styles["cell"].(map[string]any); ok {
			maps.Copy(cell, own)
		}
		styles["cell"] = cell
		b, err := json.Marshal(styles, json.Deterministic(true))
		if err == nil {
			if out == nil {
				out = vega.Config{}
			}
			out["style"] = b
		}
	}
	return out
}

// specStyle names the style that holds the config of a kind of mark of a
// spec, such as zuzu.spec.point, which the mark takes after the styles of
// the look, so that the config of the spec wins over the look (D48 part 5).
func specStyle(mark string) string { return "zuzu.spec." + mark }

// vegaLiteOnly are the properties of the config of a mark of Vega-Lite that
// Vega does not have, which the compiler reads itself or leaves out, as
// vega-lite.js does.
var vegaLiteOnly = map[string]bool{
	"color": true, "filled": true, "invalid": true, "tooltip": true, "point": true, "line": true,
	"timeUnitBandPosition": true, "timeUnitBandSize": true, "continuousBandSize": true, "discreteBandSize": true,
	"minBandSize": true, "binSpacing": true, "thickness": true, "cornerRadiusEnd": true, "style": true, "clip": true,
	"x": true, "y": true, "x2": true, "y2": true, "width": true, "height": true, "orient": true,
	"radiusOffset": true, "thetaOffset": true, "radius2Offset": true, "theta2Offset": true,
	"xOffset": true, "yOffset": true, "x2Offset": true, "y2Offset": true,
}

// markStyles returns the style of each kind of mark from the config of a
// spec: the properties of config.mark and of the config of the kind, such
// as config.point, with those of the kind winning, without the properties
// that only Vega-Lite has. The size of a bar or a tick is its width in
// Vega-Lite, and not a property of Vega, so it stays out too.
func markStyles(m map[string]jsontext.Value) map[string]any {
	var common map[string]any
	_ = json.Unmarshal(m["mark"], &common)
	out := map[string]any{}
	for kind := range vegaMark {
		var own map[string]any
		_ = json.Unmarshal(m[kind], &own)
		st := map[string]any{}
		for _, src := range []map[string]any{common, own} {
			for k, v := range src {
				if vegaLiteOnly[k] || k == "size" && (kind == "bar" || kind == "tick") {
					continue
				}
				st[k] = v
			}
		}
		if len(st) > 0 {
			out[specStyle(kind)] = st
		}
	}
	return out
}

// compile compiles the spec of the top level.
func (c *compiler) compile(s *Spec) (*vega.Spec, error) {
	c.top = s
	c.cfg = readConfig(s.Config)
	if c.cfg.customFormats {
		// A custom format type names a function of JavaScript, which zuzu
		// does not have, so the formats stay those of d3.
		if err := c.issue("/config/customFormatTypes", "the custom format types of the config"); err != nil {
			return nil, err
		}
	}
	out := &vega.Spec{Schema: schemaVega}
	if s.Description != nil {
		out.Description = *s.Description
	}
	out.Config = styleConfig(s.Config)
	var raw map[string]jsontext.Value
	if len(s.Config) > 0 && json.Unmarshal(s.Config, &raw) == nil {
		for k := range markStyles(raw) {
			if c.cfg.specStyles == nil {
				c.cfg.specStyles = map[string]bool{}
			}
			c.cfg.specStyles[strings.TrimPrefix(k, specStyle(""))] = true
		}
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
	if l := layerRepeat(s); l != nil {
		s = l
	}
	defer c.pushProjection(s.Projection)()
	if isConcat(s) {
		if err := c.concat(s, out, nil, "", map[string]string{}); err != nil {
			return nil, err
		}
		out.Signals = append(out.Signals, c.signals...)
		out.Data = c.data
		c.finishProjection(out)
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
		c.finishProjection(out)
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
	if paint := viewStyle(s.View); paint != nil {
		// The view of a single view paints the group of the top level, as
		// in vega-lite.js (D48 part 5).
		update := vega.Props{}
		for k, x := range paint {
			update[k] = prod(map[string]any{"value": x})
		}
		out.Encode = vega.Encode{"update": update}
	}
	c.mergeAggregates(out)
	out.Signals = append(out.Signals, c.signals...)
	out.Data = c.data
	c.finishProjection(out)
	return out, nil
}

// view compiles a spec into a view: a single view or a layer. Facets,
// repeats and concatenations come in their own functions.
func (c *compiler) view(s *Spec, v *view, src *dataNode, path string, repeat map[string]string) error {
	defer c.pushProjection(s.Projection)()
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
			// A time unit transform reads its field as dates.
			var dates []string
			for _, t := range s.Transform {
				if f := t.fields(); t.Has("timeUnit") {
					if name, ok := f["field"].(string); ok {
						dates = append(dates, name)
					}
				}
			}
			c.parseFields(src, dates, "date")
			if err := c.transforms(src, s.Transform, path+"/transform"); err != nil {
				return err
			}
			// The layers do not parse the fields that the transforms make.
			prev := c.produced
			c.produced = maps.Clone(prev)
			if c.produced == nil {
				c.produced = map[string]bool{}
			}
			maps.Copy(c.produced, producedFields(s.Transform))
			defer func() { c.produced = prev }()
		}
		if src != nil && (src.values || len(c.data[src.index].Source.List) > 0) && !src.open && (!s.expanded || s.composite) && len(s.Layer) > 1 && len(s.Transform) == 0 {
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
		for _, l := range s.Layer {
			if l.Data != nil {
				// Layers that read the same url share one source.
				c.shareSources = true
			}
		}
		independent := map[string]bool{}
		if len(s.Resolve) > 0 {
			var r struct {
				Scale  map[string]string `json:"scale"`
				Legend map[string]string `json:"legend"`
			}
			if err := json.Unmarshal(s.Resolve, &r); err == nil {
				for ch, mode := range r.Scale {
					if mode == "independent" {
						independent[ch] = true
					}
				}
				for ch, mode := range r.Legend {
					if mode == "independent" && !independent[ch] {
						// The layers share the scale, and each draws
						// its own legend of it.
						if v.ownLegend == nil {
							v.ownLegend = map[string]bool{}
						}
						v.ownLegend[ch] = true
					}
				}
			}
		}
		for i := range s.Layer {
			inheritEncoding(&s.Layer[i], s.Encoding)
		}
		for _, ch := range conflictingChannels(s.Layer) {
			// Layers that put fields of a discrete and of a continuous type
			// on a channel cannot share its scale.
			independent[ch] = true
		}
		for i := range s.Layer {
			l := &s.Layer[i]
			if l.Mark == nil && len(l.Layer) == 0 {
				continue
			}
			name := fmt.Sprintf("layer_%d_", i)
			if l.layerPrefix != "" {
				name = l.layerPrefix
			}
			sub := v.layer(name, v.prefix)
			if len(independent) > 0 {
				// The layer has its own scale of each independent channel.
				sub.independent, sub.layerIndex, sub.indepPrefix = independent, i, sub.prefix
			}
			lr := repeat
			if l.repeatWith != nil {
				lr = maps.Clone(repeat)
				maps.Copy(lr, l.repeatWith)
			}
			if err := c.view(l, sub, src, fmt.Sprintf("%s/layer/%d", path, i), lr); err != nil {
				return err
			}
		}
		v.size(s)
		return nil
	case s.Mark != nil:
		var cc map[string]any
		if c.cfg != nil {
			cc = c.cfg.composites[s.Mark.Type]
		}
		expanded, err := composite(s, cc)
		if err != nil {
			return c.issue(path+"/mark", "%v", err)
		}
		if expanded != nil {
			if s.Data != nil {
				expanded.Data = nil
			}
			expanded.composite = true
			return c.view(expanded, v, src, path, repeat)
		}
		if layered := overlay(s, c.cfg); layered != nil {
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
// it does not have, as vega-lite.js merges the encoding of a layer: a field
// or a datum of the layer takes the scale, the legend and the other
// properties of the channel of its parent, such as the domain and the range
// of a color that each layer gives as a datum. The field, the datum and the
// value of the parent do not pass on. It reports false when either has a
// value, which stands alone.
func mergeDefs(own, parent FieldDef) (FieldDef, bool) {
	if !own.Value.IsZero() || !parent.Value.IsZero() {
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
	_, hasField := am["field"]
	_, hasDatum := am["datum"]
	for k, v := range bm {
		if (k == "field" || k == "datum") && (hasField || hasDatum) {
			continue
		}
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
	// graticule says that the source is the lines of a graticule.
	graticule bool
	// explicit holds the parse of the format of inline values, which the
	// datasets that derive from them run as formulas.
	explicit map[string]any
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
	} else if d.Name != "" && (d.URL != "" || !d.Values.IsZero()) {
		// A dataset of a url or of values with a name keeps the name.
		vd.Name = d.Name
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
	case len(d.Sphere) > 0:
		vd.Values = vega.Value{JSON: jsontext.Value(`[{"type":"Sphere"}]`)}
		n.values = true
	case len(d.Graticule) > 0:
		params := vega.Extra{}
		var g map[string]any
		if json.Unmarshal(d.Graticule, &g) == nil {
			for k, v := range g {
				params[k] = mustJSON(v)
			}
		}
		vd.Transform = append(vd.Transform, vega.Transform{Type: "graticule", Params: params})
		n.graticule = true
	default:
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
		if n.values && !d.Format.Parse.IsZero() {
			// Inline values keep their text, and the datasets that derive
			// from them parse it with formulas, as vega-lite.js does.
			var parse map[string]any
			if d.Format.Parse.Decode(&parse) == nil {
				n.explicit = parse
				vd.Format.Parse = vega.Value{}
				if vd.Format.Type == "" && vd.Format.Property == "" && vd.Format.Feature == "" && vd.Format.Mesh == "" {
					vd.Format = nil
				}
			}
		}
	}
	n.index = len(c.data)
	n.open = !n.values && !c.shareSources
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
	c.parseExplicit(src, n)
	return n
}

// add adds transforms to a dataset.
func (c *compiler) add(n *dataNode, ts ...vega.Transform) {
	c.data[n.index].Transform = append(c.data[n.index].Transform, ts...)
}

// transform makes a Vega transform from its type and its parameters.
func transform(typ string, params map[string]any) vega.Transform {
	t := vega.Transform{Type: typ, Params: vega.Extra{}}
	// The compiler names a field as Vega reads it, with each dot of a
	// flat name escaped, and a transform writes it under its plain name.
	switch as := params["as"].(type) {
	case string:
		params["as"] = literal(as)
	case []any:
		out := make([]any, len(as))
		for i, a := range as {
			if s, ok := a.(string); ok {
				a = literal(s)
			}
			out[i] = a
		}
		params["as"] = out
	}
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
func overlay(s *Spec, cfg *config) *Spec {
	if s.overlaid || s.Mark.Type != "area" && s.Mark.Type != "line" {
		return nil
	}
	want := func(k string) bool {
		var v any
		if raw, ok := s.Mark.Props[k]; ok {
			if json.Unmarshal(raw, &v) != nil {
				return false
			}
		} else if cfg != nil {
			// The config of the mark can ask for the overlay.
			v = cfg.marks[s.Mark.Type][k]
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
	if _, ok := s.Encoding["shape"]; ok && s.Mark.Type == "line" {
		// A shape on a line draws points along it.
		point = true
	}
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
	out.Layer = []Spec{{Mark: base, overlaid: true}}
	// A line or a point that the spec gives as an object, such as
	// "line": {"color": "darkgreen"}, takes its properties.
	overlayProps := func(k string, props Extra) Extra {
		var m map[string]jsontext.Value
		if raw, ok := s.Mark.Props[k]; ok && json.Unmarshal(raw, &m) == nil {
			for p, v := range m {
				if p != "type" {
					props[p] = v
				}
			}
		}
		return props
	}
	if line {
		out.Layer = append(out.Layer, Spec{Mark: &Mark{Type: "line", Props: overlayProps("line", Extra{}), object: true}, stackAs: s.Mark.Type})
	}
	if point {
		out.Layer = append(out.Layer, Spec{Mark: &Mark{Type: "point", Props: overlayProps("point", Extra{"filled": []byte("true")}), object: true}, stackAs: s.Mark.Type})
	}
	return &out
}

// mergeAggregates moves the aggregates of datasets that derive from the
// same dataset, with the same group by fields, into that dataset as one
// aggregate, as vega-lite.js does. It does so only when every dataset that
// derives from the parent starts with such an aggregate and no mark reads
// the parent.
func (c *compiler) mergeAggregates(out *vega.Spec) {
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
	for _, d := range c.data {
		// A lookup reads its source as it is, so no transform of a view
		// moves into that source.
		for _, t := range d.Transform {
			var from string
			if t.Type == "lookup" && json.Unmarshal(t.Params["from"], &from) == nil {
				read[from] = true
			}
		}
	}
	// A merge of datasets can bring aggregates side by side, and a merge
	// of aggregates can leave datasets that match.
	c.mergeSources(out, read)
	c.liftFacetFlattens(out, read)
	c.liftFlattens(read)
	for range 10 {
		c.liftBins(out, read)
		c.liftTimeUnits(out, read)
		a := c.mergeSiblingAggregates(out, read)
		c.removeEmpty(read)
		if m := c.mergeIdentical(out, read); !a && !m {
			break
		}
	}
	c.release(out, read)
	c.numberBySource(out, read)
}

// mergeSiblingAggregates merges the aggregates of the datasets that derive
// from the same dataset, and reports whether it merged any.
func (c *compiler) mergeSiblingAggregates(out *vega.Spec, read map[string]bool) bool {
	children := map[string][]int{}
	for i, d := range c.data {
		if len(d.Source.List) == 1 {
			children[d.Source.List[0]] = append(children[d.Source.List[0]], i)
		}
	}
	merged := false
	for pi := range c.data {
		parent := &c.data[pi]
		all := children[parent.Name]
		if len(all) < 2 {
			continue
		}
		// The children that start with an aggregate with the same group
		// by fields merge.
		byGroup := map[string][]int{}
		var groupOrder []string
		for _, k := range all {
			ts := c.data[k].Transform
			if len(ts) == 0 || ts[0].Type != "aggregate" {
				continue
			}
			g := string(ts[0].Params["groupby"])
			if _, ok := byGroup[g]; !ok {
				groupOrder = append(groupOrder, g)
			}
			byGroup[g] = append(byGroup[g], k)
		}
		var kids []int
		var groupby []byte
		for _, g := range groupOrder {
			if len(byGroup[g]) >= 2 {
				kids, groupby = byGroup[g], []byte(g)
				break
			}
		}
		if kids == nil {
			continue
		}
		whole := len(kids) == len(all) && !read[parent.Name]
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
		agg := transform("aggregate", map[string]any{"groupby": g, "ops": ops, "fields": fields, "as": as})
		if whole {
			parent.Transform = append(parent.Transform, agg)
			c.reverseNames(out, read, kids)
			return true
		}
		// The merged aggregate goes in a dataset of its own, which takes
		// the name of the first child, and the children derive from it.
		first := kids[0]
		name := c.data[first].Name
		// The merged dataset takes the number of the first child, which
		// takes the next number, and the datasets after it move up one.
		if n, ok := dataNumber(name); ok {
			names := map[string]string{}
			for _, d := range c.data {
				if m, ok := dataNumber(d.Name); ok && m >= n {
					names[d.Name] = fmt.Sprintf("data_%d", m+1)
				}
			}
			c.nData++
			c.renameAll(out, read, names)
		} else {
			renamed := fmt.Sprintf("data_%d", c.nData)
			c.nData++
			c.renameAll(out, read, map[string]string{name: renamed})
		}
		mid := vega.Data{Name: name, Source: vega.List(parent.Name), Transform: []vega.Transform{agg}}
		for _, k := range kids {
			c.data[k].Source = vega.List(name)
		}
		c.data = slices.Insert(c.data, first, mid)
		var moved []int
		for i, d := range c.data {
			if len(d.Source.List) == 1 && d.Source.List[0] == name {
				moved = append(moved, i)
			}
		}
		c.reverseNames(out, read, moved)
		return true
	}
	return merged
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

// layerRepeat turns a repeat of layers, {"repeat": {"layer": [...]}}, into
// a layer of copies of its spec, one for each value. It returns nil for any
// other spec.
func layerRepeat(s *Spec) *Spec {
	if len(s.Repeat) == 0 || s.Spec == nil {
		return nil
	}
	var r map[string][]string
	if json.Unmarshal(s.Repeat, &r) != nil || len(r) != 1 || r["layer"] == nil {
		return nil
	}
	out := *s
	out.Repeat, out.Spec = nil, nil
	for _, v := range r["layer"] {
		l := *s.Spec
		l.layerPrefix = "child__layer_" + varName(v) + "_"
		l.repeatWith = map[string]string{"layer": v}
		out.Layer = append(out.Layer, l)
	}
	return &out
}

// conflictingChannels returns the channels other than positions that the
// layers give fields of both a discrete type and a continuous type.
func conflictingChannels(layers []Spec) []string {
	var out []string
	for _, ch := range []string{"color", "fill", "stroke", "opacity", "fillOpacity", "strokeOpacity", "strokeWidth", "size", "shape", "strokeDash", "angle"} {
		discrete, continuous := false, false
		for _, l := range layers {
			c, ok := l.Encoding[ch]
			if !ok {
				continue
			}
			def := c.Def()
			if def == nil || def.Field.IsZero() {
				continue
			}
			d, err := readChannel(ch, def, nil)
			if err != nil {
				continue
			}
			if d.continuous() && (d.units == nil || d.typ == temporal) {
				continuous = true
			} else {
				discrete = true
			}
		}
		if discrete && continuous {
			out = append(out, ch)
		}
	}
	return out
}

// parseExplicit adds to a dataset that derives from inline values the
// formulas that parse the fields that the format of the values names: a
// date, a date of a format of d3-time-format in local time or in UTC, a
// number, a boolean or a string.
func (c *compiler) parseExplicit(src, n *dataNode) {
	if len(src.explicit) == 0 {
		return
	}
	names := make([]string, 0, len(src.explicit))
	for f := range src.explicit {
		names = append(names, f)
	}
	slices.Sort(names)
	parsed := c.parsed[n.index]
	if parsed == nil {
		parsed = map[string]bool{}
		c.parsed[n.index] = parsed
	}
	for _, f := range names {
		typ, _ := src.explicit[f].(string)
		ref := fieldRef(f)
		var e string
		switch {
		case typ == "date":
			e = "toDate(" + ref + ")"
		case typ == "number":
			e = "toNumber(" + ref + ")"
		case typ == "boolean":
			e = "toBoolean(" + ref + ")"
		case typ == "string":
			e = "toString(" + ref + ")"
		case strings.HasPrefix(typ, "date:"):
			e = "timeParse(" + ref + "," + strings.TrimPrefix(typ, "date:") + ")"
		case strings.HasPrefix(typ, "utc:"):
			e = "utcParse(" + ref + "," + strings.TrimPrefix(typ, "utc:") + ")"
		default:
			continue
		}
		parsed[f] = true
		c.add(n, transform("formula", map[string]any{"expr": e, "as": f}))
	}
}

// numberBySource gives the derived datasets of each source their numbers
// after those of the sources before it, as vega-lite.js numbers them. The
// datasets of one source keep their order, and the numbers stay the same
// set, with the gaps that the layers took.
func (c *compiler) numberBySource(out *vega.Spec, read map[string]bool) {
	children := map[string][]string{}
	var roots []string
	for _, d := range c.data {
		if len(d.Source.List) == 0 {
			roots = append(roots, d.Name)
			continue
		}
		p := d.Source.List[0]
		children[p] = append(children[p], d.Name)
	}
	// The datasets of each source keep the order of their numbers, and
	// the sources come in their order.
	var order []string
	seen := map[string]bool{}
	for _, r := range roots {
		var group []string
		var walk func(name string)
		walk = func(name string) {
			if seen[name] {
				return
			}
			seen[name] = true
			if _, ok := dataNumber(name); ok {
				group = append(group, name)
			}
			for _, k := range children[name] {
				walk(k)
			}
		}
		walk(r)
		slices.SortFunc(group, func(a, b string) int {
			x, _ := dataNumber(a)
			y, _ := dataNumber(b)
			return x - y
		})
		order = append(order, group...)
	}
	var numbers []int
	for _, n := range order {
		m, _ := dataNumber(n)
		numbers = append(numbers, m)
	}
	slices.Sort(numbers)
	names := map[string]string{}
	for i, n := range order {
		if want := fmt.Sprintf("data_%d", numbers[i]); want != n {
			names[n] = want
		}
	}
	if len(names) > 0 {
		c.renameAll(out, read, names)
	}
}
