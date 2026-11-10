package vegalite

import (
	"encoding/json/v2"
	"fmt"

	"github.com/xo/zuzu/expr"
)

// transforms compiles the transforms of a view into a dataset.
func (c *compiler) transforms(n *dataNode, ts []Transform, path string) error {
	for i, t := range ts {
		p := fmt.Sprintf("%s/%d", path, i)
		if err := c.transformOne(n, t, p); err != nil {
			return err
		}
	}
	return nil
}

// fields returns the transform as a map of its fields.
func (t Transform) fields() map[string]any {
	out := map[string]any{}
	for k := range t.Fields {
		var v any
		if err := t.Decode(k, &v); err == nil {
			out[k] = v
		}
	}
	return out
}

// transformOne compiles one transform.
func (c *compiler) transformOne(n *dataNode, t Transform, path string) error {
	f := t.fields()
	switch {
	case t.Has("density"), t.Has("loess"), t.Has("regression"), t.Has("quantile"):
		for _, k := range []string{"density", "loess", "regression", "quantile"} {
			if t.Has(k) {
				return c.issue(path, "the transform %s, which comes in phase 5", k)
			}
		}
	case t.Has("filter"):
		test := predicateOf(f["filter"])
		c.add(n, transform("filter", map[string]any{"expr": test}))
	case t.Has("calculate"):
		c.add(n, transform("formula", map[string]any{"expr": f["calculate"], "as": f["as"]}))
	case t.Has("aggregate"), t.Has("joinaggregate"):
		key := "aggregate"
		if t.Has("joinaggregate") {
			key = "joinaggregate"
		}
		list, _ := f[key].([]any)
		var ops, fields, as []any
		for _, e := range list {
			m, _ := e.(map[string]any)
			op := expr.String(m["op"])
			field := m["field"]
			name := m["as"]
			if name == nil {
				name = op
				if field != nil {
					name = op + "_" + expr.String(field)
				}
			}
			ops = append(ops, op)
			fields = append(fields, field)
			as = append(as, name)
		}
		params := map[string]any{"ops": ops, "fields": fields, "as": as}
		if g, ok := f["groupby"]; ok {
			params["groupby"] = g
		} else if key == "aggregate" {
			params["groupby"] = []any{}
		}
		c.add(n, transform(key, params))
	case t.Has("window"):
		list, _ := f["window"].([]any)
		var ops, fields, params, as []any
		for _, e := range list {
			m, _ := e.(map[string]any)
			ops = append(ops, m["op"])
			fields = append(fields, m["field"])
			params = append(params, m["param"])
			as = append(as, m["as"])
		}
		p := map[string]any{"ops": ops, "fields": fields, "params": params, "as": as}
		if g, ok := f["groupby"]; ok {
			p["groupby"] = g
		}
		if fr, ok := f["frame"]; ok {
			p["frame"] = fr
		}
		if ip, ok := f["ignorePeers"]; ok {
			p["ignorePeers"] = ip
		}
		if s, ok := f["sort"].([]any); ok {
			var sf, so []any
			for _, e := range s {
				m, _ := e.(map[string]any)
				sf = append(sf, m["field"])
				order := "ascending"
				if o, ok := m["order"].(string); ok {
					order = o
				}
				so = append(so, order)
			}
			p["sort"] = map[string]any{"field": sf, "order": so}
		}
		c.add(n, transform("window", p))
	case t.Has("bin"):
		field := expr.String(f["field"])
		bin, _ := f["bin"].(map[string]any)
		if bin == nil {
			bin = map[string]any{}
		}
		as := []any{f["as"], expr.String(f["as"]) + "_end"}
		if l, ok := f["as"].([]any); ok {
			as = l
		}
		c.addBin(n, field, bin, as)
	case t.Has("timeUnit"):
		units, utc := timeUnits(unitName(f["timeUnit"]))
		params := map[string]any{"field": f["field"], "units": units, "as": []any{f["as"], expr.String(f["as"]) + "_end"}}
		if utc {
			params["timezone"] = "utc"
		}
		c.add(n, transform("timeunit", params))
	case t.Has("fold"):
		params := map[string]any{"fields": f["fold"]}
		if as, ok := f["as"]; ok {
			params["as"] = as
		} else {
			params["as"] = []any{"key", "value"}
		}
		c.add(n, transform("fold", params))
	case t.Has("flatten"):
		params := map[string]any{"fields": f["flatten"]}
		if as, ok := f["as"]; ok {
			params["as"] = as
		}
		c.add(n, transform("flatten", params))
	case t.Has("lookup"):
		return c.lookup(n, f, path)
	case t.Has("pivot"):
		params := map[string]any{"field": f["pivot"], "value": f["value"]}
		for _, k := range []string{"groupby", "limit", "op"} {
			if v, ok := f[k]; ok {
				params[k] = v
			}
		}
		c.add(n, transform("pivot", params))
	case t.Has("impute"):
		params := map[string]any{"field": f["impute"], "key": f["key"]}
		for _, k := range []string{"groupby", "keyvals", "method", "value"} {
			if v, ok := f[k]; ok {
				params[k] = v
			}
		}
		c.add(n, transform("impute", params))
	case t.Has("sample"):
		c.add(n, transform("sample", map[string]any{"size": f["sample"]}))
	case t.Has("stack"):
		params := map[string]any{"field": f["stack"], "groupby": f["groupby"], "as": f["as"]}
		if o, ok := f["offset"]; ok {
			params["offset"] = o
		}
		if s, ok := f["sort"].([]any); ok {
			var sf, so []any
			for _, e := range s {
				m, _ := e.(map[string]any)
				sf = append(sf, m["field"])
				order := "ascending"
				if o, ok := m["order"].(string); ok {
					order = o
				}
				so = append(so, order)
			}
			params["sort"] = map[string]any{"field": sf, "order": so}
		}
		c.add(n, transform("stack", params))
	case t.Has("extent"):
		c.add(n, transform("extent", map[string]any{"field": f["extent"], "signal": f["param"]}))
	default:
		return c.issue(path, "a transform with the fields %v", keys(f))
	}
	return nil
}

// unitName reads the name of a time unit, a string or {"unit": ...}.
func unitName(v any) string {
	if m, ok := v.(map[string]any); ok {
		return expr.String(m["unit"])
	}
	return expr.String(v)
}

// keys returns the keys of a map.
func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// addBin adds the extent and the bin of a field to a dataset. The extent
// is a signal named after the field, as Vega-Lite names it.
func (c *compiler) addBin(n *dataNode, field string, bin map[string]any, as []any) {
	prefix := binPrefix(bin)
	sig := prefix + "_" + varName(field)
	params := map[string]any{"field": field, "as": as, "signal": sig + "_bins"}
	if ext, ok := bin["extent"]; ok {
		params["extent"] = ext
	} else {
		c.add(n, transform("extent", map[string]any{"field": field, "signal": sig + "_extent"}))
		params["extent"] = map[string]any{"signal": sig + "_extent"}
	}
	maxbins := any(10.0)
	if m, ok := bin["maxbins"]; ok {
		maxbins = m
	}
	params["maxbins"] = maxbins
	for _, k := range []string{"step", "steps", "base", "divide", "minstep", "nice", "anchor"} {
		if v, ok := bin[k]; ok {
			params[k] = v
		}
	}
	c.add(n, transform("bin", params))
}

// lookup compiles a lookup: a dataset of the secondary data, and the
// lookup transform that reads it.
func (c *compiler) lookup(n *dataNode, f map[string]any, path string) error {
	from, _ := f["from"].(map[string]any)
	d, _ := from["data"].(map[string]any)
	var data Data
	if b, err := jsonOf(d); err == nil {
		if err := unmarshal(b, &data); err != nil {
			return err
		}
	}
	src, err := c.source(&data, path+"/from/data")
	if err != nil || src == nil {
		return err
	}
	if p, ok := from["param"]; ok {
		_ = p
		return c.issue(path, "a lookup of a selection")
	}
	params := map[string]any{"from": c.name(src), "key": from["key"], "fields": []any{f["lookup"]}}
	if vals, ok := from["fields"]; ok {
		params["values"] = vals
		if as, ok := f["as"]; ok {
			params["as"] = as
		}
	} else if as, ok := f["as"]; ok {
		if s, ok := as.(string); ok {
			params["as"] = []any{s}
		} else {
			params["as"] = as
		}
	}
	if def, ok := f["default"]; ok {
		params["default"] = def
	}
	c.add(n, transform("lookup", params))
	return nil
}

// jsonOf writes a value as JSON.
func jsonOf(v any) ([]byte, error) {
	return json.Marshal(v)
}

// unmarshal reads JSON into a value.
func unmarshal(b []byte, out any) error {
	return json.Unmarshal(b, out)
}
