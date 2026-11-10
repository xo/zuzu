package zuzu

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/xo/zuzu/dataflow"
	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/vega"
)

// markTypes are the types of mark that zuzu draws.
var markTypes = map[string]bool{
	scene.Arc: true, scene.Area: true, scene.Group: true, scene.Image: true,
	scene.Line: true, scene.Path: true, scene.Rect: true, scene.Rule: true,
	scene.Symbol: true, scene.Text: true, scene.Trail: true,
}

// buildMarks builds the marks of a scope, in the order of the spec.
func (s *scope) buildMarks(path string) ([]scene.Mark, error) {
	var out []scene.Mark
	for i := range s.spec.Marks {
		m := &s.spec.Marks[i]
		p := fmt.Sprintf("%s/marks/%d", path, i)
		sm, err := s.buildMark(m, p)
		if err != nil {
			var is vega.Issue
			if errors.As(err, &is) && errors.Is(err, vega.ErrUnsupported) {
				if err := s.rt.issue(err, p, ""); err != nil {
					return nil, err
				}
				continue
			}
			return nil, err
		}
		if sm == nil {
			continue
		}
		out = append(out, *sm)
		if m.Name != "" {
			s.markItems[m.Name] = sm.Items
		}
	}
	return out, nil
}

// buildMark builds one mark: an item for each datum of its data.
func (s *scope) buildMark(m *vega.Mark, path string) (*scene.Mark, error) {
	if !markTypes[m.Type] {
		return nil, vega.Issue{Err: vega.ErrUnsupportedMark, Path: path, Message: m.Type}
	}
	for i, t := range m.Transform {
		if err := s.rt.issue(vega.ErrUnsupportedTransform, fmt.Sprintf("%s/transform/%d", path, i), "%s", t.Type); err != nil {
			return nil, err
		}
	}
	role := m.Role
	if role == "" {
		role = "mark"
		if m.Type == scene.Group {
			role = "scope"
		}
	}
	sm := &scene.Mark{Type: m.Type, Role: role, Name: m.Name}
	if m.ZIndex != nil {
		sm.ZIndex = *m.ZIndex
	}
	if !m.Clip.IsZero() {
		v, err := s.value(m.Clip)
		if err != nil {
			return nil, err
		}
		sm.Clip = expr.Truthy(v)
	}
	data, facets, err := s.markData(m)
	if err != nil {
		return nil, fmt.Errorf("%s/from: %w", path, err)
	}
	states := make([]*itemState, len(data))
	for i, d := range data {
		st := newItemState(d)
		st.facet = i
		if err := s.encodeItem(m.Encode, st, d, path); err != nil {
			return nil, err
		}
		geometry(m.Type, st)
		states[i] = st
	}
	if m.Sort != nil && !m.Sort.Field.IsZero() {
		if err := s.sortItems(m.Sort, states, facets); err != nil {
			return nil, fmt.Errorf("%s/sort: %w", path, err)
		}
	}
	sm.Items = make([]scene.Item, len(states))
	for i, st := range states {
		sm.Items[i] = *st.it
	}
	if m.Type == scene.Group {
		for i := range sm.Items {
			it := &sm.Items[i]
			child := s.rt.newScope(s, &m.Scope, path, it.Datum)
			child.item = it
			if m.From != nil && m.From.Facet != nil && facets != nil {
				f := m.From.Facet
				rows := facets[keyOfItem(i, states, sm)]
				if rows == nil && f.Field == "" {
					// A group that the cross of a facet adds has no rows,
					// and draws nothing inside, as in Vega.
					continue
				}
				child.data[f.Name] = &dataState{def: &vega.Data{Name: f.Name}, path: path + "/from/facet", facet: rows}
			}
			marks, err := child.buildScope(path)
			if err != nil {
				return nil, err
			}
			it.Items = marks
		}
	}
	return sm, nil
}

// keyOfItem returns the index of the facet of a group item, after a sort.
func keyOfItem(i int, states []*itemState, _ *scene.Mark) int {
	return states[i].facet
}

// buildScope builds the marks and the guides of a scope: its axes, its
// marks, its legends and its title, in the order that Vega puts them.
func (s *scope) buildScope(path string) ([]scene.Mark, error) {
	var out []scene.Mark
	axes, err := s.buildAxes(path)
	if err != nil {
		return nil, err
	}
	out = append(out, axes...)
	marks, err := s.buildMarks(path)
	if err != nil {
		return nil, err
	}
	out = append(out, marks...)
	legends, err := s.buildLegends(path)
	if err != nil {
		return nil, err
	}
	out = append(out, legends...)
	title, err := s.buildTitle(path, out)
	if err != nil {
		return nil, err
	}
	out = append(out, title...)
	if !s.spec.Layout.IsZero() {
		if err := s.gridLayout(out); err != nil {
			return nil, err
		}
	}
	s.placeGuides(out)
	return out, nil
}

// markData returns the data of a mark: the rows of a dataset, the datum of
// each facet, or one empty datum. For a facet, it also returns the rows of
// each facet.
func (s *scope) markData(m *vega.Mark) ([]any, [][]dataflow.Row, error) {
	if m.From == nil || (m.From.Data == "" && m.From.Facet == nil) {
		return []any{map[string]any{}}, nil, nil
	}
	if f := m.From.Facet; f != nil {
		rows, err := s.dataset(f.Data)
		if err != nil {
			return nil, nil, err
		}
		return s.facet(f, rows)
	}
	rows, err := s.dataset(m.From.Data)
	if err != nil {
		if items, ok := s.findMarkItems(m.From.Data); ok && errors.Is(err, ErrUnknownName) {
			out := make([]any, len(items))
			for i := range items {
				out[i] = itemDatum(&items[i])
			}
			return out, nil, nil
		}
		return nil, nil, err
	}
	out := make([]any, len(rows))
	for i, r := range rows {
		out[i] = r
	}
	return out, nil, nil
}

// itemDatum returns an item as a datum, for a mark whose data is another
// mark.
func itemDatum(it *scene.Item) map[string]any {
	return map[string]any{
		"datum": it.Datum, "x": it.X, "y": it.Y, "x2": it.X2, "y2": it.Y2,
		"width": it.Width, "height": it.Height, "text": textValue(it.Text),
	}
}

// textValue returns the text of an item as a value.
func textValue(l scene.Lines) any {
	switch len(l) {
	case 0:
		return nil
	case 1:
		return l[0]
	}
	out := make([]any, len(l))
	for i, s := range l {
		out[i] = s
	}
	return out
}

// findMarkItems returns the items of a mark of this scope or a scope above,
// by name.
func (s *scope) findMarkItems(name string) ([]scene.Item, bool) {
	for sc := s; sc != nil; sc = sc.parent {
		if items, ok := sc.markItems[name]; ok {
			return items, true
		}
	}
	return nil, false
}

// facet splits rows into groups. With groupby, each group holds the rows
// with the same values of the fields, in the order each group first
// appears, and its datum holds those values and the count of rows, or the
// measures of the aggregate of the facet. With a
// field, each row gives a group whose rows are the array in the field.
func (s *scope) facet(f *vega.Facet, rows []dataflow.Row) ([]any, [][]dataflow.Row, error) {
	if f.Field != "" {
		get := dataflow.FieldAccessor(f.Field)
		data := make([]any, len(rows))
		groups := make([][]dataflow.Row, len(rows))
		for i, r := range rows {
			data[i] = r
			l, _ := get.Get(r).([]any)
			groups[i] = dataflow.Ingest(l)
		}
		return data, groups, nil
	}
	fields := make([]*dataflow.Accessor, len(f.GroupBy.List))
	for i, g := range f.GroupBy.List {
		fields[i] = dataflow.FieldAccessor(g)
	}
	var data []any
	var groups [][]dataflow.Row
	index := map[string]int{}
	for _, r := range rows {
		vals := make([]any, len(fields))
		for i, g := range fields {
			vals[i] = g.Get(r)
		}
		k := domainKey(vals)
		j, ok := index[k]
		if !ok {
			datum := map[string]any{}
			for i, g := range fields {
				datum[g.Name] = vals[i]
			}
			j = len(data)
			index[k] = j
			data = append(data, datum)
			groups = append(groups, nil)
		}
		groups[j] = append(groups[j], r)
	}
	var agg map[string]any
	if len(f.Aggregate) > 0 {
		if err := rawUnmarshal(f.Aggregate, &agg); err != nil {
			return nil, nil, err
		}
	}
	if cross, _ := agg["cross"].(bool); cross && len(fields) > 1 {
		data, groups = crossGroups(f.GroupBy.List, data, groups, index)
	}
	if len(f.Aggregate) == 0 || len(agg) == 1 && agg["cross"] != nil {
		for i, d := range data {
			if m, ok := d.(map[string]any); ok {
				m["count"] = float64(len(groups[i]))
			}
		}
	} else {
		if err := s.facetAggregate(f, data, groups); err != nil {
			return nil, nil, err
		}
	}
	return data, groups, nil
}

// crossGroups adds an empty group for each combination of the values of
// the group by fields that no row has, as the cross of an aggregate does.
// The values of each field come in the order they first appear.
func crossGroups(names []string, data []any, groups [][]dataflow.Row, index map[string]int) ([]any, [][]dataflow.Row) {
	values := make([][]any, len(names))
	seen := make([]map[string]bool, len(names))
	for i := range names {
		seen[i] = map[string]bool{}
	}
	for _, d := range data {
		m, _ := d.(map[string]any)
		for i, n := range names {
			v := m[n]
			if k := domainKey([]any{v}); !seen[i][k] {
				seen[i][k] = true
				values[i] = append(values[i], v)
			}
		}
	}
	combo := make([]any, len(names))
	var walk func(i int)
	walk = func(i int) {
		if i == len(names) {
			k := domainKey(combo)
			if _, ok := index[k]; ok {
				return
			}
			datum := map[string]any{}
			for j, n := range names {
				datum[n] = combo[j]
			}
			index[k] = len(data)
			data = append(data, datum)
			groups = append(groups, nil)
			return
		}
		for _, v := range values[i] {
			combo[i] = v
			walk(i + 1)
		}
	}
	walk(0)
	return data, groups
}

// facetAggregate adds the measures of the aggregate of a facet to the datum
// of each group.
func (s *scope) facetAggregate(f *vega.Facet, data []any, groups [][]dataflow.Row) error {
	t := vega.Transform{Type: "aggregate", Params: vega.Extra{}}
	var agg map[string]any
	if err := rawUnmarshal(f.Aggregate, &agg); err != nil {
		return err
	}
	for k, v := range agg {
		if k == "groupby" || k == "cross" {
			continue
		}
		t.Params[k] = rawMarshal(v)
	}
	env := &dataEnv{s: s}
	for i, g := range groups {
		out, err := dataflow.Run(env, g, []vega.Transform{t}, "/from/facet/aggregate")
		if err != nil {
			return err
		}
		if m, ok := data[i].(map[string]any); ok && len(out) == 1 {
			maps.Copy(m, out[0])
		}
	}
	return nil
}

// sortItems sorts the items of a mark by fields of the items, such as
// datum.price, or by expressions over the items.
func (s *scope) sortItems(sort *vega.Sort, states []*itemState, _ [][]dataflow.Row) error {
	var raw any
	if err := sort.Field.Decode(&raw); err != nil {
		return err
	}
	if sort.Field.Signal != "" {
		v, err := s.eval(sort.Field.Signal, nil)
		if err != nil {
			return err
		}
		raw = v
	}
	list, ok := raw.([]any)
	if !ok {
		list = []any{raw}
	}
	type key struct {
		get func(st *itemState) (any, error)
	}
	keys := make([]key, len(list))
	for i, f := range list {
		switch x := f.(type) {
		case string:
			keys[i] = key{func(st *itemState) (any, error) { return itemProp(st, x), nil }}
		case map[string]any:
			src, _ := x["expr"].(string)
			keys[i] = key{func(st *itemState) (any, error) {
				return s.eval(src, map[string]any{"datum": st.it.Datum})
			}}
		}
	}
	vals := make([][]any, len(states))
	for i, st := range states {
		vals[i] = make([]any, len(keys))
		for j, k := range keys {
			if k.get == nil {
				continue
			}
			v, err := k.get(st)
			if err != nil {
				return err
			}
			vals[i][j] = v
		}
	}
	idx := make([]int, len(states))
	for i := range idx {
		idx[i] = i
	}
	orders := sort.Order.List
	slices.SortStableFunc(idx, func(a, b int) int {
		for j := range keys {
			c := compare(vals[a][j], vals[b][j])
			if j < len(orders) && orders[j] == "descending" {
				c = -c
			}
			if c != 0 {
				return c
			}
		}
		return 0
	})
	sorted := make([]*itemState, len(states))
	for i, j := range idx {
		sorted[i] = states[j]
	}
	copy(states, sorted)
	return nil
}

// itemProp reads a field of an item for a sort: datum.x reads the datum,
// and x reads the item.
func itemProp(st *itemState, name string) any {
	if rest, ok := strings.CutPrefix(name, "datum"); ok && (strings.HasPrefix(rest, ".") || strings.HasPrefix(rest, "[")) {
		return readField(st.it.Datum, strings.TrimPrefix(rest, "."))
	}
	return itemField(st.it, name)
}

// geometry works out the position and the size of an item from the
// properties that its encodings set: x and x2, xc and width, or x2 and
// width, and the same for y.
func geometry(typ string, st *itemState) {
	it := st.it
	switch typ {
	case scene.Rect, scene.Group, scene.Image:
		it.X, it.Width = span(st, it.X, it.X2, it.Width, st.xc, "x", "x2", "xc", "width")
		it.Y, it.Height = span(st, it.Y, it.Y2, it.Height, st.yc, "y", "y2", "yc", "height")
	case scene.Text, scene.Symbol, scene.Arc, scene.Path, scene.Shape, scene.Line, scene.Area, scene.Trail:
		if st.set["xc"] && !st.set["x"] {
			it.X = st.xc
		}
		if st.set["yc"] && !st.set["y"] {
			it.Y = st.yc
		}
	case scene.Rule:
		if !st.set["x2"] {
			it.X2 = it.X
		}
		if !st.set["y2"] {
			it.Y2 = it.Y
		}
	}
	for _, f := range []*float64{&it.X, &it.Y, &it.X2, &it.Y2, &it.Width, &it.Height} {
		if math.IsNaN(*f) || math.IsInf(*f, 0) {
			*f = 0
		}
	}
}

// span returns the start and the size of an item along one axis.
func span(st *itemState, x, x2, w, xc float64, nx, nx2, nxc, nw string) (float64, float64) {
	hasX, hasX2, hasW, hasXC := st.set[nx], st.set[nx2], st.set[nw], st.set[nxc]
	switch {
	case hasXC && hasW:
		return xc - w/2, w
	case hasX && hasX2:
		if x2 < x {
			x, x2 = x2, x
		}
		return x, x2 - x
	case hasX2 && hasW:
		return x2 - w, w
	case hasX && hasW && w < 0:
		return x + w, -w
	}
	return x, w
}
