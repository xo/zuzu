package vegalite

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"slices"
	"strconv"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/vega"
)

// composite expands a composite mark, boxplot, errorbar or errorband, into
// a layer of plain marks over aggregated fields, as Vega-Lite documents
// them. It returns nil for any other spec.
func composite(s *Spec) (*Spec, error) {
	switch s.Mark.Type {
	case "boxplot", "errorbar", "errorband":
	default:
		return nil, nil
	}
	props := map[string]any{}
	for k, raw := range s.Mark.Props {
		var v any
		if err := json.Unmarshal(raw, &v); err == nil {
			props[k] = v
		}
	}
	enc := map[string]any{}
	for k, ch := range s.Encoding {
		b, err := json.Marshal(ch)
		if err != nil {
			return nil, err
		}
		var v any
		if err := json.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		enc[k] = v
	}
	// The continuous channel holds the values to summarize.
	cont := ""
	for _, ch := range []string{"y", "x"} {
		d, _ := enc[ch].(map[string]any)
		if d == nil {
			continue
		}
		t, _ := d["type"].(string)
		if t == quantitative || t == "" && d["aggregate"] == nil && d["timeUnit"] == nil && ch == "y" {
			cont = ch
			break
		}
	}
	if o, ok := props["orient"].(string); ok {
		cont = map[string]string{"horizontal": "x", "vertical": "y"}[o]
	}
	if cont == "" {
		return nil, fmt.Errorf("a %s with no quantitative x or y: %w", s.Mark.Type, vega.ErrUnsupported)
	}
	def, _ := enc[cont].(map[string]any)
	field, _ := def["field"].(string)
	title := field
	if t, ok := def["title"]; ok {
		title = expr.String(t)
	}
	if t, ok := def["title"]; ok && t == nil {
		title = ""
	}
	// The group by fields are the fields of the other channels.
	var groupby, pre []any
	others := map[string]any{}
	for _, k := range sortedChannels(enc) {
		v := enc[k]
		if k == cont || k == cont+"2" || k == cont+"Error" || k == cont+"Error2" {
			continue
		}
		others[k] = v
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		f, ok := m["field"].(string)
		if !ok || m["aggregate"] != nil {
			continue
		}
		if tu, ok := m["timeUnit"]; ok {
			// The time unit applies before the aggregate, which groups by
			// it, and the layers draw the field it gives.
			d, err := unitField(k, s.Encoding[k])
			if err != nil {
				return nil, err
			}
			if d != nil && d.units != nil {
				pre = append(pre, map[string]any{"timeUnit": tu, "field": f, "as": d.out})
				def := maps.Clone(m)
				delete(def, "timeUnit")
				def["field"] = d.out
				if def["type"] == nil {
					def["type"] = temporal
				}
				if _, ok := def["title"]; !ok {
					def["title"] = titleOf(d)
				}
				others[k] = def
				f = d.out
			}
		}
		groupby = append(groupby, f)
	}
	var out map[string]any
	switch s.Mark.Type {
	case "boxplot":
		out = boxplot(cont, field, title, def, props, others, groupby)
	default:
		out = errorMark(s.Mark.Type, cont, field, title, def, props, others, groupby, enc)
	}
	if len(pre) > 0 {
		ts, _ := out["transform"].([]any)
		out["transform"] = append(pre, ts...)
	}
	if layers, ok := out["layer"].([]any); ok && len(layers) == 1 && s.Mark.Type != "errorband" {
		// An error bar or a box plot of one part is a single view, with
		// the transforms of the layer before its own. An error band stays
		// a layer.
		one, _ := layers[0].(map[string]any)
		ts, _ := out["transform"].([]any)
		if own, ok := one["transform"].([]any); ok {
			ts = append(ts, own...)
		}
		if len(ts) > 0 {
			one["transform"] = ts
		}
		out = one
	}
	var transforms []any
	for _, t := range s.Transform {
		transforms = append(transforms, t.fields())
	}
	if len(transforms) > 0 {
		if existing, ok := out["transform"].([]any); ok {
			transforms = append(transforms, existing...)
		}
		out["transform"] = transforms
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	var spec Spec
	if err := json.Unmarshal(b, &spec); err != nil {
		return nil, err
	}
	spec.Data = s.Data
	spec.Width, spec.Height = s.Width, s.Height
	spec.expanded = true
	return &spec, nil
}

// sortedChannels returns the channels of an encoding in the order of
// channelOrder, then any others in the order of their names.
func sortedChannels(enc map[string]any) []string {
	var out []string
	for _, k := range channelOrder {
		if _, ok := enc[k]; ok {
			out = append(out, k)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(enc)) {
		if !slices.Contains(channelOrder, k) {
			out = append(out, k)
		}
	}
	return out
}

// channelWith returns a copy of the definition of the continuous channel
// with another field and the title of the original.
func channelWith(def map[string]any, field, title string) map[string]any {
	out := map[string]any{"field": field, "type": quantitative}
	if title != "" {
		out["title"] = title
	} else if _, ok := def["title"]; ok {
		out["title"] = nil
	}
	if sc, ok := def["scale"]; ok {
		out["scale"] = sc
	}
	if ax, ok := def["axis"]; ok {
		out["axis"] = ax
	}
	return out
}

// layerOf makes one layer: a mark, and the other channels with some of
// the continuous one.
func layerOf(mark map[string]any, others map[string]any, extra map[string]any, keep ...string) map[string]any {
	enc := map[string]any{}
	for k, v := range others {
		if len(keep) > 0 && !contains(keep, k) {
			continue
		}
		enc[k] = v
	}
	maps.Copy(enc, extra)
	return map[string]any{"mark": mark, "encoding": enc}
}

// position lists the channels that place a mark: the discrete channels of
// position, and the channels that split it, without color.
var position = []string{"x", "y", "xOffset", "yOffset", "detail", "row", "column", "facet", "size"}

// errorMark expands an error bar, a rule from the lower to the upper bound
// with ticks at the ends if the mark asks, or an error band, an area from
// the lower to the upper bound with lines at the edges if it asks.
func errorMark(typ, cont, field, title string, def, props, others map[string]any, groupby []any, enc map[string]any) map[string]any {
	extent := "stderr"
	if e, ok := props["extent"].(string); ok {
		extent = e
	}
	lower, upper := "lower_"+field, "upper_"+field
	var ops, as []any
	var calcs []any
	switch extent {
	case "ci":
		ops, as = []any{"ci0", "ci1", "mean"}, []any{lower, upper, "center_" + field}
	case "iqr":
		ops, as = []any{"q1", "q3", "median"}, []any{lower, upper, "center_" + field}
	default:
		ops, as = []any{extent, "mean"}, []any{"extent_" + field, "center_" + field}
		calcs = []any{
			map[string]any{"calculate": "datum['center_" + field + "'] + datum['extent_" + field + "']", "as": upper},
			map[string]any{"calculate": "datum['center_" + field + "'] - datum['extent_" + field + "']", "as": lower},
		}
	}
	fields := make([]any, len(ops))
	agg := make([]any, len(ops))
	for i := range ops {
		fields[i] = field
		agg[i] = map[string]any{"op": ops[i], "field": field, "as": as[i]}
	}
	if groupby == nil {
		groupby = []any{}
	}
	transform := []any{map[string]any{"aggregate": agg, "groupby": groupby}}
	ref := func(f string) string { return "datum['" + f + "']" }
	fieldOf := func(ch string) string {
		m, _ := enc[ch].(map[string]any)
		f, _ := m["field"].(string)
		return f
	}
	switch {
	case fieldOf(cont+"2") != "":
		// The data gives both bounds.
		transform = nil
		calcs = []any{
			map[string]any{"calculate": ref(fieldOf(cont + "2")), "as": upper},
			map[string]any{"calculate": ref(field), "as": lower},
		}
	case fieldOf(cont+"Error") != "":
		// The data gives the center and the error on each side.
		transform = nil
		low := ref(field) + " - " + ref(fieldOf(cont+"Error"))
		if e2 := fieldOf(cont + "Error2"); e2 != "" {
			low = ref(field) + " + " + ref(e2)
		}
		calcs = []any{
			map[string]any{"calculate": ref(field) + " + " + ref(fieldOf(cont+"Error")), "as": upper},
			map[string]any{"calculate": low, "as": lower},
		}
	}
	// The bounds that the formulas work out belong to the layers that
	// draw them.
	withCalcs := func(l map[string]any) map[string]any {
		if len(calcs) > 0 {
			l["transform"] = calcs
		}
		return l
	}
	color := props["color"]
	var layers []any
	if typ == "errorbar" {
		rule := map[string]any{"type": "rule", "style": "errorbar-rule"}
		if color == nil {
			color = "black"
		}
		rule["color"] = color
		// The ticks at the ends come before the rule between them.
		if t, ok := props["ticks"]; ok && t != false {
			for _, f := range []string{lower, upper} {
				tick := map[string]any{"type": "tick", "style": "errorbar-ticks", "color": color}
				if m, ok := t.(map[string]any); ok {
					maps.Copy(tick, m)
				}
				layers = append(layers, withCalcs(layerOf(tick, others, map[string]any{cont: channelWith(def, f, title)})))
			}
		}
		layers = append(layers, withCalcs(layerOf(rule, others, map[string]any{
			cont: channelWith(def, lower, title), cont + "2": map[string]any{"field": upper},
		})))
	} else {
		band := map[string]any{"type": "area", "style": "errorband-band", "opacity": 0.3}
		if color != nil {
			band["color"] = color
		}
		layers = append(layers, withCalcs(layerOf(band, others, map[string]any{
			cont: channelWith(def, lower, title), cont + "2": map[string]any{"field": upper},
		})))
		if b, ok := props["borders"]; ok && b != false {
			for _, f := range []string{lower, upper} {
				line := map[string]any{"type": "line", "style": "errorband-borders"}
				if color != nil {
					line["color"] = color
				}
				layers = append(layers, withCalcs(layerOf(line, others, map[string]any{cont: channelWith(def, f, title)})))
			}
		}
	}
	out := map[string]any{"layer": layers}
	if transform != nil {
		out["transform"] = transform
	}
	return out
}

// boxplot expands a box plot: a rule for each whisker, a bar from the
// first quartile to the third, and a tick at the median. With the extent
// min-max, the whiskers reach the least and the greatest value. With a
// number k, the default 1.5, they reach the last values within k times
// the interquartile range of the box, and points draw the values beyond.
func boxplot(cont, field, title string, def, props, others map[string]any, groupby []any) map[string]any {
	size := 14.0
	if s, ok := props["size"].(float64); ok {
		size = s
	}
	lb, ub, mb := "lower_box_"+field, "upper_box_"+field, "mid_box_"+field
	lw, uw := "lower_whisker_"+field, "upper_whisker_"+field
	if groupby == nil {
		groupby = []any{}
	}
	other := "x"
	if cont == "x" {
		other = "y"
	}
	agg := func(ops, as []string, fields ...string) map[string]any {
		var list []any
		for i, op := range ops {
			f := field
			if i < len(fields) && fields[i] != "" {
				f = fields[i]
			}
			list = append(list, map[string]any{"op": op, "field": f, "as": as[i]})
		}
		return map[string]any{"aggregate": list, "groupby": groupby}
	}
	rule := func(from, to string) map[string]any {
		return layerOf(map[string]any{"type": "rule", "style": "boxplot-rule", "color": "black"}, others, map[string]any{
			cont: channelWith(def, from, title), cont + "2": map[string]any{"field": to},
		}, position...)
	}
	boxColor := props["color"]
	box := map[string]any{"type": "bar", "style": "boxplot-box", "size": size}
	if boxColor != nil {
		box["color"] = boxColor
	}
	boxLayer := layerOf(box, others, map[string]any{
		cont: channelWith(def, lb, title), cont + "2": map[string]any{"field": ub},
	})
	median := layerOf(map[string]any{"type": "tick", "style": "boxplot-median", "color": "white", "size": size, "thickness": 1}, others, map[string]any{
		cont: channelWith(def, mb, title),
	}, position...)
	extent, isNum := props["extent"].(float64)
	if e, ok := props["extent"].(string); ok && e == "min-max" {
		return map[string]any{
			"transform": []any{agg([]string{"q1", "q3", "median", "min", "max"}, []string{lb, ub, mb, lw, uw})},
			"layer":     []any{rule(lw, lb), rule(ub, uw), boxLayer, median},
		}
	}
	if !isNum {
		extent = 1.5
	}
	k := strconv.FormatFloat(extent, 'f', -1, 64)
	f := "datum['" + field + "']"
	iqr := "(datum['" + ub + "'] - datum['" + lb + "'])"
	lowFence := "datum['" + lb + "'] - " + k + " * " + iqr
	highFence := "datum['" + ub + "'] + " + k + " * " + iqr
	join := map[string]any{"joinaggregate": []any{
		map[string]any{"op": "q1", "field": field, "as": lb},
		map[string]any{"op": "q3", "field": field, "as": ub},
	}, "groupby": groupby}
	outliers := layerOf(map[string]any{"type": "point", "style": "boxplot-outliers"}, others, map[string]any{
		cont: channelWith(def, field, title),
	})
	outliers["transform"] = []any{map[string]any{"filter": "(" + f + " < " + lowFence + ") || (" + f + " > " + highFence + ")"}}
	whiskers := map[string]any{
		"transform": []any{
			map[string]any{"filter": "(" + lowFence + " <= " + f + ") && (" + f + " <= " + highFence + ")"},
			agg([]string{"min", "max", "min", "max"}, []string{lw, uw, lb, ub}, field, field, lb, ub),
		},
		"layer": []any{rule(lw, lb), rule(ub, uw)},
	}
	_ = other
	return map[string]any{"layer": []any{
		map[string]any{"transform": []any{join}, "layer": []any{outliers, whiskers}},
		map[string]any{
			"transform": []any{agg([]string{"q1", "q3", "median", "min", "max"}, []string{lb, ub, mb, "min_" + field, "max_" + field})},
			"layer":     []any{boxLayer, median},
		},
	}}
}

// unitField reads a channel of a composite mark, to find the field that
// its time unit gives.
func unitField(name string, ch Channel) (*channelDef, error) {
	def := ch.Def()
	if def == nil {
		return nil, nil
	}
	return readChannel(name, def, nil)
}
