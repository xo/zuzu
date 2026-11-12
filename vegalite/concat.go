package vegalite

import (
	"cmp"
	"encoding/json/v2"
	"fmt"
	"maps"
	"strings"

	"github.com/xo/zuzu/vega"
)

// cellSpec is one view of a concatenation or a repeat: its spec, the
// prefix of its names, and the fields that a repeat gives it.
type cellSpec struct {
	spec   *Spec
	prefix string
	repeat map[string]string
}

// concat compiles a concatenation or a repeat: a grid of views, each with
// its own scales of position, which share the other scales, such as
// color, and the legends.
func (c *compiler) concat(s *Spec, out *vega.Spec, src *dataNode, path string, repeat map[string]string) error {
	if err := c.params(s, path); err != nil {
		return err
	}
	var cells []cellSpec
	columns := 0
	switch {
	case len(s.HConcat) > 0:
		for i := range s.HConcat {
			cells = append(cells, cellSpec{&s.HConcat[i], fmt.Sprintf("%sconcat_%d_", c.concatPrefix, i), repeat})
		}
		columns = len(s.HConcat)
	case len(s.VConcat) > 0:
		for i := range s.VConcat {
			cells = append(cells, cellSpec{&s.VConcat[i], fmt.Sprintf("%sconcat_%d_", c.concatPrefix, i), repeat})
		}
		columns = 1
	case len(s.Concat) > 0:
		for i := range s.Concat {
			cells = append(cells, cellSpec{&s.Concat[i], fmt.Sprintf("%sconcat_%d_", c.concatPrefix, i), repeat})
		}
		columns = len(s.Concat)
		if s.Columns != nil {
			columns = *s.Columns
		}
	case len(s.Repeat) > 0 && s.Spec != nil:
		var err error
		// The views of a repeat take no prefix of an outer concatenation.
		cells, columns, err = repeatCells(s, repeat)
		if err != nil {
			return err
		}
	}
	if s.Data != nil {
		d, err := c.source(s.Data, path+"/data")
		if err != nil {
			return err
		}
		src = d
	}
	readers := 0
	for _, cell := range cells {
		if cell.spec.Data == nil {
			readers++
		}
	}
	if src != nil && src.values && readers > 1 && len(s.Transform) == 0 {
		// Views over inline values have a dataset of their own, which
		// vega-lite.js names and later removes, so its number is taken.
		c.reserved = append(c.reserved, reservation{data: c.name(src), number: c.nData})
		c.nData++
	}
	if len(s.Transform) > 0 {
		src = c.derive(src)
		if err := c.transforms(src, s.Transform, path+"/transform"); err != nil {
			return err
		}
	} else if src != nil && readers > 1 {
		// The views share the source, so each adds its transforms to a
		// dataset of its own.
		src.open = false
	}
	c.shareSources = true
	own, sharedPos := map[string]bool{}, map[string]bool{}
	if len(s.Resolve) > 0 {
		var r struct {
			Scale map[string]string `json:"scale"`
		}
		if json.Unmarshal(s.Resolve, &r) == nil {
			for ch, mode := range r.Scale {
				switch {
				case mode == "independent" && !isPositional(ch):
					own[ch] = true
				case mode == "shared" && isPositional(ch):
					sharedPos[ch] = true
				}
			}
		}
	}
	common := newView("", c.cfg)
	for i, cell := range cells {
		p := fmt.Sprintf("%s/concat/%d", path, i)
		if isConcat(cell.spec) {
			// A concatenation inside a concatenation is a group with a
			// layout of its own, whose views take its prefix too. Its
			// scales and its signals go to the top.
			outerPrefix := c.concatPrefix
			c.concatPrefix = cell.prefix
			nested := &vega.Spec{}
			err := c.concat(cell.spec, nested, src, p, cell.repeat)
			c.concatPrefix = outerPrefix
			if err != nil {
				return err
			}
			out.Scales = append(out.Scales, nested.Scales...)
			out.Signals = append(out.Signals, nested.Signals...)
			out.Legends = append(out.Legends, nested.Legends...)
			out.Marks = append(out.Marks, vega.Mark{
				Type: "group", Name: cell.prefix + "group",
				Layout: nested.Layout, Marks: nested.Marks, Title: cell.spec.Title,
			})
			continue
		}
		if _, _, _, _, _, ok, _ := readFacet(cell.spec, cell.repeat); ok {
			// A facet inside a concatenation compiles as a facet named
			// after its view, in a group with the layout of the facet.
			named := *cell.spec
			named.Name = strings.TrimSuffix(cell.prefix, "_")
			nested := &vega.Spec{}
			outer := c.nested
			c.nested = true
			err := c.facet(&named, nested, src, p, cell.repeat)
			c.nested = outer
			if err != nil {
				return err
			}
			out.Scales = append(out.Scales, nested.Scales...)
			out.Signals = append(out.Signals, nested.Signals...)
			out.Legends = append(out.Legends, nested.Legends...)
			out.Marks = append(out.Marks, vega.Mark{
				Type: "group", Name: cell.prefix + "group",
				Layout: nested.Layout, Marks: nested.Marks, Title: cmp.Or(nested.Title, cell.spec.Title),
			})
			continue
		}
		v := newView(cell.prefix, c.cfg)
		v.scalePrefix = cell.prefix
		v.own, v.sharedPos = own, sharedPos
		v.common = common.shared
		v.sizeNames = [2]string{cell.prefix + "width", cell.prefix + "height"}
		if err := c.view(cell.spec, v, src, p, cell.repeat); err != nil {
			return err
		}
		inner := &vega.Spec{}
		v.finish(inner)
		out.Scales = append(out.Scales, inner.Scales...)
		out.Signals = append(out.Signals, inner.Signals...)
		if !inner.Width.IsZero() {
			out.Signals = append(out.Signals, vega.Signal{Name: v.sizeNames[0], Value: mustJSON(inner.Width.Number)})
		}
		if !inner.Height.IsZero() {
			out.Signals = append(out.Signals, vega.Signal{Name: v.sizeNames[1], Value: mustJSON(inner.Height.Number)})
		}
		group := vega.Mark{
			Type: "group", Name: cell.prefix + "group", Style: vega.Strings{List: []string{"cell"}},
			Marks: inner.Marks, Axes: inner.Axes, Legends: inner.Legends,
			Encode: vega.Encode{"update": vega.Props{
				"width":  prod(map[string]any{"signal": v.sizeNames[0]}),
				"height": prod(map[string]any{"signal": v.sizeNames[1]}),
			}},
		}
		if cell.spec.Title != nil {
			group.Title = cell.spec.Title
		}
		out.Marks = append(out.Marks, group)
	}
	shared := &vega.Spec{}
	common.finish(shared)
	out.Scales = append(shared.Scales, out.Scales...)
	out.Legends = append(out.Legends, shared.Legends...)
	align := "each"
	if len(s.Repeat) > 0 {
		align = "all"
	}
	layout := map[string]any{"padding": spacing(s, c.cfg.concatSpacing), "columns": columns, "bounds": "full", "align": align}
	if v := valueOf(s.Bounds); v != nil {
		layout["bounds"] = v
	}
	if c.concatPrefix != "" && len(s.HConcat) > 0 {
		// A row inside a concatenation leaves its columns to Vega.
		delete(layout, "columns")
	}
	out.Layout = lit(layout)
	if c.concatPrefix == "" && !c.nested {
		c.mergeAggregates(out)
	}
	return nil
}

// isConcat reports whether a spec is a concatenation or a repeat.
func isConcat(s *Spec) bool {
	return len(s.HConcat) > 0 || len(s.VConcat) > 0 || len(s.Concat) > 0 || (len(s.Repeat) > 0 && s.Spec != nil)
}

// repeatCells returns the views of a repeat: one for each field of a list,
// in rows of columns, or one for each row and column of a grid.
func repeatCells(s *Spec, parent map[string]string) ([]cellSpec, int, error) {
	with := func(k, v string) map[string]string {
		out := map[string]string{}
		maps.Copy(out, parent)
		out[k] = v
		return out
	}
	var list []string
	if json.Unmarshal(s.Repeat, &list) == nil {
		var cells []cellSpec
		for _, f := range list {
			cells = append(cells, cellSpec{s.Spec, "child__" + varName(f) + "_", with("repeat", f)})
		}
		columns := len(list)
		if s.Columns != nil {
			columns = *s.Columns
		}
		return cells, columns, nil
	}
	var grid struct {
		Row    []string `json:"row"`
		Column []string `json:"column"`
		Layer  []string `json:"layer"`
	}
	if err := json.Unmarshal(s.Repeat, &grid); err != nil {
		return nil, 0, err
	}
	rows := grid.Row
	if rows == nil {
		rows = []string{""}
	}
	cols := grid.Column
	if cols == nil {
		cols = []string{""}
	}
	var cells []cellSpec
	for _, r := range rows {
		for _, col := range cols {
			m := with("row", r)
			m["column"] = col
			prefix := "child__"
			if r != "" {
				prefix += "row_" + varName(r)
			}
			if col != "" {
				prefix += "column_" + varName(col)
			}
			cells = append(cells, cellSpec{s.Spec, prefix + "_", m})
		}
	}
	return cells, len(cols), nil
}
