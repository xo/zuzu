package vegalite

import (
	"encoding/json/v2"
	"fmt"
	"maps"

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
	var cells []cellSpec
	columns := 0
	switch {
	case len(s.HConcat) > 0:
		for i := range s.HConcat {
			cells = append(cells, cellSpec{&s.HConcat[i], fmt.Sprintf("concat_%d_", i), repeat})
		}
		columns = len(s.HConcat)
	case len(s.VConcat) > 0:
		for i := range s.VConcat {
			cells = append(cells, cellSpec{&s.VConcat[i], fmt.Sprintf("concat_%d_", i), repeat})
		}
		columns = 1
	case len(s.Concat) > 0:
		for i := range s.Concat {
			cells = append(cells, cellSpec{&s.Concat[i], fmt.Sprintf("concat_%d_", i), repeat})
		}
		columns = len(s.Concat)
		if s.Columns != nil {
			columns = *s.Columns
		}
	case len(s.Repeat) > 0 && s.Spec != nil:
		var err error
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
	if len(s.Transform) > 0 {
		src = c.derive(src)
		if err := c.transforms(src, s.Transform, path+"/transform"); err != nil {
			return err
		}
	}
	common := newView("")
	for i, cell := range cells {
		p := fmt.Sprintf("%s/concat/%d", path, i)
		if _, _, _, _, _, ok, _ := readFacet(cell.spec, cell.repeat); ok || isConcat(cell.spec) {
			if err := c.issue(p, "a facet or a concatenation inside a concatenation"); err != nil {
				return err
			}
			continue
		}
		v := newView(cell.prefix)
		v.scalePrefix = cell.prefix
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
	out.Layout = lit(map[string]any{"padding": 20, "columns": columns, "bounds": "full", "align": align})
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
