package zuzu

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/xo/zuzu/dataflow"
	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// markTypes are the types of mark that zuzu draws.
var markTypes = map[string]bool{
	scene.Arc: true, scene.Area: true, scene.Group: true, scene.Image: true,
	scene.Line: true, scene.Path: true, scene.Rect: true, scene.Rule: true,
	scene.Shape: true, scene.Symbol: true, scene.Text: true, scene.Trail: true,
}

// buildMarks builds the marks of a scope, in the order of the spec.
func (s *scope) buildMarks(path string) ([]scene.Mark, error) {
	var out []scene.Mark
	// The styles of each mark of out, for the rules of the look that join
	// marks, such as the parts of a box plot.
	var styles [][]string
	defer func() {
		boxLook(out, styles, s.rt.theme.FillOpacity)
		dodgeLabels(out)
	}()
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
		styles = append(styles, m.Style.List)
		for _, x := range s.look(m, &out[len(out)-1]) {
			out = append(out, x)
			styles = append(styles, nil)
		}
		if m.Name != "" {
			s.markItems[m.Name] = sm.Items
			s.markTypes[m.Name] = sm.Type
		}
	}
	return out, nil
}

// buildMark builds one mark: an item for each datum of its data.
func (s *scope) buildMark(m *vega.Mark, path string) (*scene.Mark, error) {
	if !markTypes[m.Type] {
		return nil, vega.Issue{Err: vega.ErrUnsupportedMark, Path: path, Message: m.Type}
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
	defaults := s.rt.markDefaults(m)
	_, configStroke := defaults["stroke"]
	states := make([]*itemState, len(data))
	badLink := false
	for i, d := range data {
		st := newItemState(d)
		st.facet = i
		for k, v := range defaults {
			setProp(st, k, v)
		}
		if err := s.encodeItem(m.Encode, st, d, path); err != nil {
			return nil, err
		}
		if err := s.linkIssue(st.it, path, &badLink); err != nil {
			return nil, err
		}
		if m.Type == scene.Symbol && configStroke && st.it.Size != nil && *st.it.Size < minRingSize {
			// The ring that the config gives a point covers a point this
			// small, so it draws without one.
			st.it.Stroke = scene.Paint{}
		}
		geometry(m.Type, st)
		states[i] = st
	}
	if len(m.Transform) > 0 {
		if states, err = s.transformItems(m, states, path); err != nil {
			return nil, err
		}
	}
	for _, st := range m.Style.List {
		o, ok := strings.CutPrefix(st, theme.StyleBar)
		if !ok {
			continue
		}
		if gap := expr.Number(defaults[theme.StackGap]); gap > 0 {
			stackGaps(states, o == "horizontal", gap)
		}
		if r := expr.Number(defaults[theme.CornerRadiusEnd]); r > 0 && !encodesCorners(m) {
			roundEnds(states, o == "horizontal", r)
		}
	}
	if m.Type == scene.Symbol && expr.Truthy(defaults[theme.LargestFirst]) && (m.Sort == nil || m.Sort.Field.IsZero()) && encodes(m, "size") {
		// The largest bubble draws first, so that the small ones stay on
		// top (docs/STYLE.md).
		slices.SortStableFunc(states, func(a, b *itemState) int {
			return cmp.Compare(sizeOf(b.it), sizeOf(a.it))
		})
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
		if m.From.Data != "" && f.Field == "" {
			return s.preFaceted(m.From.Data, f, rows)
		}
		return s.facet(f, rows)
	}
	rows, err := s.dataset(m.From.Data)
	if err != nil {
		if items, ok := s.findMarkItems(m.From.Data); ok && errors.Is(err, ErrUnknownName) {
			out := make([]any, len(items))
			for i := range items {
				out[i] = s.itemDatum(s.findMarkType(m.From.Data), &items[i])
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
// mark, with its bounds, as Vega gives them.
func (s *scope) itemDatum(typ string, it *scene.Item) map[string]any {
	d := map[string]any{
		"datum": it.Datum, "x": it.X, "y": it.Y, "x2": it.X2, "y2": it.Y2,
		"width": it.Width, "height": it.Height, "text": textValue(it.Text),
	}
	switch typ {
	case scene.Rect, scene.Group, scene.Image:
		// An item that the encoding gives from x to x2 keeps x2 as its
		// encoded end, which roundEnds reads. Vega gives the larger end
		// as x2, and the smaller as x.
		if it.X2 == it.X || it.X2 == it.X+it.Width {
			d["x2"] = it.X + it.Width
		}
		if it.Y2 == it.Y || it.Y2 == it.Y+it.Height {
			d["y2"] = it.Y + it.Height
		}
	}
	if b := s.rt.style().ItemBounds(typ, it); !b.IsEmpty() {
		d["bounds"] = map[string]any{"x1": b.X1, "y1": b.Y1, "x2": b.X2, "y2": b.Y2}
	}
	return d
}

// findMarkType returns the kind of a named mark of this scope or a scope
// above.
func (s *scope) findMarkType(name string) string {
	for sc := s; sc != nil; sc = sc.parent {
		if t, ok := sc.markTypes[name]; ok {
			return t
		}
	}
	return ""
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

// preFaceted returns the groups of a facet whose items come from another
// dataset, as Vega makes them when a mark names both data and a facet: each
// row of that dataset is an item, with the rows of the facet that share its
// values of the group by fields.
func (s *scope) preFaceted(name string, f *vega.Facet, rows []dataflow.Row) ([]any, [][]dataflow.Row, error) {
	outer, err := s.dataset(name)
	if err != nil {
		return nil, nil, err
	}
	fields := make([]*dataflow.Accessor, len(f.GroupBy.List))
	for i, g := range f.GroupBy.List {
		fields[i] = dataflow.FieldAccessor(g)
	}
	key := func(r dataflow.Row) string {
		vals := make([]any, len(fields))
		for i, g := range fields {
			vals[i] = g.Get(r)
		}
		return domainKey(vals)
	}
	byKey := map[string][]dataflow.Row{}
	for _, r := range rows {
		k := key(r)
		byKey[k] = append(byKey[k], r)
	}
	data := make([]any, len(outer))
	groups := make([][]dataflow.Row, len(outer))
	for i, r := range outer {
		data[i] = r
		groups[i] = byKey[key(r)]
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
				// As in Vega, datum is the item, which holds the data in
				// its own datum.
				item := map[string]any{
					"datum": st.it.Datum, "x": st.it.X, "y": st.it.Y,
					"width": st.it.Width, "height": st.it.Height,
				}
				return s.eval(src, item)
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
		if typ == scene.Area {
			// The width of a horizontal area and the height of a vertical
			// one can take the place of x2 and y2.
			if st.set["width"] && (!st.set["x"] || !st.set["x2"]) {
				x, w := span(st, it.X, it.X2, it.Width, st.xc, "x", "x2", "xc", "width")
				it.X, it.X2 = x, x+w
			}
			if st.set["height"] && (!st.set["y"] || !st.set["y2"]) {
				y, h := span(st, it.Y, it.Y2, it.Height, st.yc, "y", "y2", "yc", "height")
				it.Y, it.Y2 = y, y+h
			}
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

// itemKey is the field of the row of an item that holds the index of the
// item, so that the items follow the rows through a transform.
const itemKey = "zuzu:item"

// transformItems runs the transforms of a mark on its items, after the
// encoding, as Vega does. Each item is a row with its datum and its
// properties of place and shape, and the transforms write those properties
// back. A transform that zuzu does not support goes in the report.
func (s *scope) transformItems(m *vega.Mark, states []*itemState, path string) ([]*itemState, error) {
	rows := make([]dataflow.Row, len(states))
	for i, st := range states {
		r := itemRow(st.it)
		for k, v := range st.values {
			if _, ok := r[k]; !ok {
				r[k] = v
			}
		}
		// A place that the encoding did not set is NaN, as in Vega, so
		// that a force places the item itself.
		if !st.set["x"] && !st.set["xc"] && !st.set["x2"] {
			r["x"] = math.NaN()
		}
		if !st.set["y"] && !st.set["yc"] && !st.set["y2"] {
			r["y"] = math.NaN()
		}
		r[itemKey] = i
		rows[i] = r
	}
	env := &dataEnv{s: s}
	for i, t := range m.Transform {
		p := fmt.Sprintf("%s/transform/%d", path, i)
		if !dataflow.Supported(t.Type) {
			if err := s.rt.issue(vega.ErrUnsupportedTransform, p, "%s", t.Type); err != nil {
				return nil, err
			}
			continue
		}
		out, err := dataflow.Run(env, rows, m.Transform[i:i+1], path+"/transform")
		if err != nil {
			if _, ok := errors.AsType[vega.Issue](err); ok {
				if err := s.rt.issue(err, p, ""); err != nil {
					return nil, err
				}
				continue
			}
			return nil, err
		}
		rows = out
	}
	out := make([]*itemState, 0, len(rows))
	for _, r := range rows {
		i, ok := r[itemKey].(int)
		if !ok || i < 0 || i >= len(states) {
			continue
		}
		st := states[i]
		setItem(st.it, r)
		if m.Type == scene.Shape {
			// The transform geoshape gives the path of a shape mark.
			if v, ok := r["shape"].(string); ok {
				st.it.Path = v
			}
		}
		out = append(out, st)
	}
	return out, nil
}

// itemRow returns the properties of an item that a transform of a mark
// reads, with its datum.
func itemRow(it *scene.Item) dataflow.Row {
	r := dataflow.Row{
		"datum": it.Datum, "x": it.X, "y": it.Y, "x2": it.X2, "y2": it.Y2,
		"width": it.Width, "height": it.Height, "path": it.Path,
		"angle": it.Angle, "startAngle": it.StartAngle, "endAngle": it.EndAngle,
		"innerRadius": it.InnerRadius, "outerRadius": it.OuterRadius,
		"text": textValue(it.Text), "font": it.Font,
		"fontStyle": it.FontStyle, "fontWeight": string(it.FontWeight),
	}
	if it.FontSize != 0 {
		// A size of 0 means the size of the theme, and a transform that
		// writes 0 hides the text.
		r["fontSize"] = it.FontSize
	}
	return r
}

// setItem writes the properties of a row back to its item.
func setItem(it *scene.Item, r dataflow.Row) {
	for k, f := range map[string]*float64{
		"x": &it.X, "y": &it.Y, "x2": &it.X2, "y2": &it.Y2,
		"width": &it.Width, "height": &it.Height, "angle": &it.Angle,
		"startAngle": &it.StartAngle, "endAngle": &it.EndAngle,
		"innerRadius": &it.InnerRadius, "outerRadius": &it.OuterRadius,
	} {
		if v, ok := r[k].(float64); ok && !math.IsNaN(v) && !math.IsInf(v, 0) {
			*f = v
		}
	}
	if v, ok := r["path"].(string); ok {
		it.Path = v
	}
	if v, ok := r["image"].(string); ok {
		it.URL = v
	}
	if v, ok := r["fontSize"].(float64); ok && !math.IsNaN(v) {
		it.FontSize = v
		if v == 0 {
			// A font size of 0, as a word cloud gives a word that does not
			// fit, hides the text, as in Vega. The scene reads 0 as the
			// size of the theme.
			zero := 0.0
			it.Opacity = &zero
		}
	}
	if v, ok := r["font"].(string); ok {
		it.Font = v
	}
	if v, ok := r["opacity"].(float64); ok && !math.IsNaN(v) {
		it.Opacity = &v
	}
	if v, ok := r["align"].(string); ok {
		it.Align = v
	}
	if v, ok := r["baseline"].(string); ok {
		it.Baseline = v
	}
	if v, ok := r["fontStyle"].(string); ok {
		it.FontStyle = v
	}
	if v, ok := r["fontWeight"]; ok && v != nil {
		it.FontWeight = scene.Weight(expr.String(v))
	}
}

// minRingSize is the size, in square pixels, of the smallest symbol that
// keeps the stroke of the config, the ring of docs/STYLE.md.
const minRingSize = 20

// markDefaults returns the defaults of the items of a mark from the config
// of the render, as Vega applies its config. First comes the config of
// every mark and of the type of the mark, or of a group for a group, for
// each property that the encoding of the mark does not set, with no fill
// and no stroke when the encoding sets either. Then come the styles of the
// mark in order, for each property that the encoding does not set.
func (rt *runtime) markDefaults(m *vega.Mark) map[string]any {
	encoded := map[string]bool{}
	for _, props := range m.Encode {
		for k := range props {
			encoded[k] = true
		}
	}
	out := map[string]any{}
	var props []map[string]any
	if m.Type == scene.Group {
		props = append(props, configMap(rt.config["group"]))
	} else {
		props = append(props, configMap(rt.config["mark"]), configMap(rt.config[m.Type]))
	}
	paint := encoded["fill"] || encoded["stroke"]
	for _, p := range props {
		for k, v := range p {
			if encoded[k] || paint && (k == "fill" || k == "stroke") {
				continue
			}
			out[k] = v
		}
	}
	styles := configMap(rt.config["style"])
	for _, name := range m.Style.List {
		for k, v := range configMap(styles[name]) {
			if !encoded[k] {
				out[k] = v
			}
		}
	}
	return out
}

// configMap returns a value of the config as an object, or nil.
func configMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// encodesCorners reports whether the encoding of a mark sets a corner.
func encodesCorners(m *vega.Mark) bool {
	for _, props := range m.Encode {
		for k := range props {
			if strings.HasPrefix(k, "cornerRadius") {
				return true
			}
		}
	}
	return false
}

// roundEnds rounds the end of each bar away from zero, the end at y, or at
// x for a horizontal bar, with a radius. The bars at one place across form a
// stack, and only the outermost bar of each side of the stack is rounded
// (docs/STYLE.md).
func roundEnds(states []*itemState, horizontal bool, r float64) {
	type place struct{ at, size float64 }
	type ends struct{ low, high *scene.Item }
	stacks := map[place]*ends{}
	var order []place
	for _, st := range states {
		it := st.it
		p := place{math.Round(it.X*100) / 100, math.Round(it.Width*100) / 100}
		end, start := it.Y, it.Y2
		if horizontal {
			p = place{math.Round(it.Y*100) / 100, math.Round(it.Height*100) / 100}
			end, start = it.X, it.X2
		}
		e := stacks[p]
		if e == nil {
			e = &ends{}
			stacks[p] = e
			order = append(order, p)
		}
		switch {
		case end < start:
			if e.low == nil || end < value(e.low, horizontal) {
				e.low = it
			}
		case end > start:
			if e.high == nil || end > value(e.high, horizontal) {
				e.high = it
			}
		}
	}
	for _, p := range order {
		e := stacks[p]
		if it := e.low; it != nil {
			if horizontal {
				it.CornerRadiusTopLeft, it.CornerRadiusBottomLeft = r, r
			} else {
				it.CornerRadiusTopLeft, it.CornerRadiusTopRight = r, r
			}
		}
		if it := e.high; it != nil {
			if horizontal {
				it.CornerRadiusTopRight, it.CornerRadiusBottomRight = r, r
			} else {
				it.CornerRadiusBottomLeft, it.CornerRadiusBottomRight = r, r
			}
		}
	}
}

// value returns the end of a bar: y, or x for a horizontal bar.
func value(it *scene.Item, horizontal bool) float64 {
	if horizontal {
		return it.X
	}
	return it.Y
}

// stackGaps opens a gap between the bars of each stack, half of it from
// each of the two bars that meet (docs/STYLE.md). The bars at one place
// across form a stack, and a stack of one bar keeps it whole.
func stackGaps(states []*itemState, horizontal bool, gap float64) {
	type side struct {
		at, size float64
		up       bool
	}
	stacks := map[side][]*scene.Item{}
	var order []side
	for _, st := range states {
		it := st.it
		k := side{math.Round(it.X*100) / 100, math.Round(it.Width*100) / 100, it.Y < it.Y2}
		if horizontal {
			k = side{math.Round(it.Y*100) / 100, math.Round(it.Height*100) / 100, it.X < it.X2}
		}
		if _, ok := stacks[k]; !ok {
			order = append(order, k)
		}
		stacks[k] = append(stacks[k], it)
	}
	// lo and hi are the ends of a bar along the stack, in pixels.
	ends := func(it *scene.Item) (float64, float64) {
		if horizontal {
			return math.Min(it.X, it.X+it.Width), math.Max(it.X, it.X+it.Width)
		}
		return math.Min(it.Y, it.Y+it.Height), math.Max(it.Y, it.Y+it.Height)
	}
	set := func(it *scene.Item, lo, hi float64) {
		if hi < lo {
			lo, hi = (lo+hi)/2, (lo+hi)/2
		}
		if horizontal {
			it.X, it.Width = lo, hi-lo
		} else {
			it.Y, it.Height = lo, hi-lo
		}
	}
	for _, k := range order {
		bars := stacks[k]
		if len(bars) < 2 {
			continue
		}
		// The bars in order along the stack, from the lowest pixel.
		slices.SortFunc(bars, func(a, b *scene.Item) int {
			la, _ := ends(a)
			lb, _ := ends(b)
			return cmp.Compare(la, lb)
		})
		for i := 0; i+1 < len(bars); i++ {
			lo, hi := ends(bars[i])
			set(bars[i], lo, hi-gap/2)
			lo, hi = ends(bars[i+1])
			set(bars[i+1], lo+gap/2, hi)
		}
	}
}

// encodes reports whether the encoding of a mark sets a property.
func encodes(m *vega.Mark, name string) bool {
	for _, props := range m.Encode {
		if _, ok := props[name]; ok {
			return true
		}
	}
	return false
}

// sizeOf returns the size of a symbol, 64 when it sets none.
func sizeOf(it *scene.Item) float64 {
	if it.Size == nil {
		return 64
	}
	return *it.Size
}
