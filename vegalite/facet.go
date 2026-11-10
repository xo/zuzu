package vegalite

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"slices"
	"strconv"

	"github.com/xo/zuzu/vega"
)

// The names of the size of a cell of a facet, and of its headers.
const (
	rowHeader    = "row_header"
	columnFooter = "column_footer"

	childWidth  = "child_width"
	childHeight = "child_height"
)

// facetDef is a row or a column of a facet: its field and its header.
type facetDef struct {
	channel string
	def     *channelDef
	sort    string
	// header is the header of the facet, and hidden says that the spec
	// sets it to null, which draws no title and no labels.
	header map[string]any
	hidden bool
	// sortField and sortOp sort the values of the facet by an aggregate of
	// another field, with the op min when the sort names none.
	sortField, sortOp string
}

// sortAs returns the name of the aggregate that sorts the facet.
func (f *facetDef) sortAs() string {
	if f.sortOp == "" {
		return f.sortField
	}
	return f.sortOp + "_" + f.sortField
}

// sortAggregate returns the ops, the fields and the names of the aggregate
// that sorts the facet, with a suffix on the names.
func (f *facetDef) sortAggregate(suffix string) map[string][]any {
	op := f.sortOp
	if op == "" {
		op = "min"
	}
	return map[string][]any{"fields": {f.sortField}, "ops": {op}, "as": {f.sortAs() + suffix}}
}

// title returns the title of the facet, or "" for none.
func (f *facetDef) title() string {
	if f.hidden {
		return ""
	}
	if t, ok := f.header["title"]; ok {
		if t == nil {
			return ""
		}
		return titleString(t)
	}
	if f.def.def != nil && !f.def.def.Title.IsZero() {
		t := valueOf(f.def.def.Title)
		if t == nil {
			return ""
		}
		return titleString(t)
	}
	return titleOf(f.def)
}

// labels reports whether the header draws the values of the facet.
func (f *facetDef) labels() bool {
	return !f.hidden && f.header["labels"] != false
}

// orient returns the side of the header for its labels, with the key
// labelOrient, or for its title, with the key titleOrient. The key orient
// sets both.
func (f *facetDef) orient(key string) string {
	if o, ok := f.header[key].(string); ok {
		return o
	}
	if o, ok := f.header["orient"].(string); ok {
		return o
	}
	if f.channel == "row" {
		return "left"
	}
	return "top"
}

// readFacet reads the facet of a spec: the operator facet, with a row and
// a column, or the channels row, column and facet of an encoding. It
// returns the spec of a cell, which the caller compiles. ok is false when
// the spec has no facet.
func readFacet(s *Spec, repeat map[string]string) (row, column, wrap *facetDef, cell *Spec, columns int, ok bool, err error) {
	read := func(ch string, def *FieldDef) (*facetDef, error) {
		d, err := readChannel(ch, def, repeat)
		if err != nil {
			return nil, err
		}
		f := &facetDef{channel: ch, def: d, sort: "ascending"}
		if !def.Header.IsZero() {
			switch h := valueOf(def.Header).(type) {
			case nil:
				f.hidden = true
			case map[string]any:
				f.header = h
			}
		}
		if !def.Sort.IsZero() {
			switch o := valueOf(def.Sort).(type) {
			case string:
				f.sort = o
			case map[string]any:
				if x, ok := o["order"].(string); ok {
					f.sort = x
				}
				f.sortField, _ = o["field"].(string)
				f.sortOp, _ = o["op"].(string)
			}
		}
		return f, nil
	}
	if len(s.Facet) > 0 && s.Spec != nil {
		var m map[string]FieldDef
		if json.Unmarshal(s.Facet, &m) == nil && (hasKey(m, "row") || hasKey(m, "column")) {
			if d, has := m["row"]; has {
				if row, err = read("row", &d); err != nil {
					return row, column, wrap, cell, columns, ok, err
				}
			}
			if d, has := m["column"]; has {
				if column, err = read("column", &d); err != nil {
					return row, column, wrap, cell, columns, ok, err
				}
			}
		} else {
			var d FieldDef
			if err = json.Unmarshal(s.Facet, &d); err != nil {
				return row, column, wrap, cell, columns, ok, err
			}
			if wrap, err = read("facet", &d); err != nil {
				return row, column, wrap, cell, columns, ok, err
			}
		}
		cp := *s.Spec
		if s.Columns != nil {
			columns = *s.Columns
		}
		return row, column, wrap, &cp, columns, true, nil
	}
	if s.Mark == nil && len(s.Layer) == 0 {
		return nil, nil, nil, nil, 0, false, nil
	}
	cp := *s
	cp.Encoding = Encoding{}
	found := false
	for k, ch := range s.Encoding {
		switch k {
		case "row", "column", "facet":
			found = true
			d := ch.Def()
			if d == nil {
				continue
			}
			f, e := read(k, d)
			if e != nil {
				return nil, nil, nil, nil, 0, false, e
			}
			switch k {
			case "row":
				row = f
			case "column":
				column = f
			default:
				wrap = f
				if raw, ok := d.Extra["columns"]; ok {
					_ = json.Unmarshal(raw, &columns)
				}
			}
		default:
			cp.Encoding[k] = ch
		}
	}
	if !found {
		return nil, nil, nil, nil, 0, false, nil
	}
	cp.Data, cp.Transform = nil, nil
	return row, column, wrap, &cp, columns, true, nil
}

// hasKey reports whether a map has a key.
func hasKey(m map[string]FieldDef, k string) bool {
	_, ok := m[k]
	return ok
}

// facet compiles a facet: a grid of cells, one for each value of the row
// and the column, each drawing the spec of a cell from its own part of the
// data, with headers that name the values and hold the axes.
func (c *compiler) facet(s *Spec, out *vega.Spec, src *dataNode, path string, repeat map[string]string) error {
	if err := c.params(s, path); err != nil {
		return err
	}
	row, column, wrap, cell, columns, _, err := readFacet(s, repeat)
	if err != nil {
		return err
	}
	if s.Data != nil {
		if src, err = c.source(s.Data, path+"/data"); err != nil {
			return err
		}
	}
	if len(s.Transform) > 0 && cell.Transform == nil && len(s.Facet) > 0 {
		n := c.derive(src)
		if err := c.transforms(n, s.Transform, path+"/transform"); err != nil {
			return err
		}
		src = n
	}
	var extra []*channelDef
	for _, f := range []*facetDef{row, column, wrap} {
		if f != nil {
			extra = append(extra, f.def)
		}
	}
	// A named facet prefixes the names of its scales, its signals, its
	// datasets and its groups.
	np := ""
	if s.Name != "" {
		np = varName(s.Name) + "_"
	}
	cw, chh, fname := np+childWidth, np+childHeight, np+"facet"
	child := newView(np+"child_", c.cfg)
	if np != "" {
		child.scalePrefix, child.prefixAll = np, true
	}
	child.facetFields = extra
	if len(s.Transform) > 0 && len(s.Facet) == 0 {
		cell.Transform = s.Transform
	}
	if err := c.view(cell, child, src, path+"/spec", repeat); err != nil {
		return err
	}
	main := child.mainData()
	if main == "" {
		return c.issue(path+"/spec", "a facet whose cells draw no data, such as a facet of a facet")
	}
	// The child names its size child_width and child_height.
	sizes := &vega.Spec{}
	child.sizeNames = [2]string{cw, chh}
	child.finish(sizes)
	// The child draws in each cell, from the data of the cell. A facet
	// sorted by an aggregate of a field facets the rows before the
	// aggregate of the child, which each cell then makes from its own rows.
	facetData, cellData := main, fname
	var cellDatasets []vega.Data
	for _, f := range []*facetDef{row, column, wrap} {
		if f != nil && f.sortField != "" && cellDatasets == nil {
			if fd, d, ok := c.splitFacet(main, extra, sizes, fname); ok {
				facetData, cellData, cellDatasets = fd, d.Name, []vega.Data{d}
			}
		}
	}
	for i := range child.marks {
		renameData(&child.marks[i], main, cellData)
	}
	out.Scales = append(out.Scales, sizes.Scales...)
	out.Signals = append(out.Signals, sizes.Signals...)
	if !sizes.Width.IsZero() {
		out.Signals = append(out.Signals, vega.Signal{Name: cw, Value: mustJSON(sizes.Width.Number)})
	}
	if !sizes.Height.IsZero() {
		out.Signals = append(out.Signals, vega.Signal{Name: chh, Value: mustJSON(sizes.Height.Number)})
	}
	out.Legends = append(out.Legends, sizes.Legends...)
	var xAxes, yAxes, grids []vega.Axis
	for _, a := range sizes.Axes {
		switch {
		case a.Grid != nil && *a.Grid:
			grids = append(grids, a)
		case a.Scale == "y" || a.Scale == "child_y" || a.Scale == np+"y":
			yAxes = append(yAxes, a)
		default:
			xAxes = append(xAxes, a)
		}
	}
	// The datasets of the headers.
	domain := func(f *facetDef) string {
		name := np + f.channel + "_domain"
		groupby := []any{f.def.out}
		if f.def.bin != nil {
			groupby = append(groupby, f.def.outEnd)
		}
		params := map[string]any{"groupby": groupby}
		if f.sortField != "" {
			for k, v := range f.sortAggregate("") {
				params[k] = v
			}
		}
		c.data = append(c.data, vega.Data{
			Name: name, Source: vega.Strings{List: []string{facetData}},
			Transform: []vega.Transform{transform("aggregate", params)},
		})
		return name
	}
	width := prod(map[string]any{"signal": cw})
	height := prod(map[string]any{"signal": chh})
	headerTitle := func(f *facetDef, orient string) *vega.Title {
		ref := "parent[" + strconv.Quote(f.def.out) + "]"
		t := &vega.Title{
			Text:   vega.Value{Signal: "isValid(" + ref + ") ? " + ref + " : \"\"+" + ref},
			Style:  vega.Strings{List: []string{"guide-label"}},
			Frame:  lit("group"),
			Offset: lit(10),
		}
		if orient != "" {
			t.Orient = lit(orient)
		}
		return t
	}
	groupTitle := func(f *facetDef) *vega.Title {
		t := &vega.Title{Text: lit(f.title()), Style: vega.Strings{List: []string{"guide-title"}}, Offset: lit(10)}
		if o := f.orient("titleOrient"); o != "top" {
			t.Orient = lit(o)
		}
		return t
	}
	headerSort := func(f *facetDef) *vega.Sort {
		field := f.def.out
		if f.sortField != "" {
			field = f.sortAs()
		}
		return &vega.Sort{Field: lit("datum[" + strconv.Quote(field) + "]"), Order: vega.Strings{List: []string{f.sort}}}
	}
	var marks []vega.Mark
	var rowDomain, colDomain, wrapDomain string
	if wrap != nil {
		wrapDomain = domain(wrap)
		marks = c.wrapHeaders(wrap, columns, wrapDomain, np, xAxes, yAxes, width, height)
		row, column = nil, nil
	}
	if row != nil {
		rowDomain = domain(row)
		if row.title() != "" {
			marks = append(marks, vega.Mark{Type: "group", Name: "row-title", Role: "row-title", Title: groupTitle(row)})
		}
	}
	if column != nil {
		colDomain = domain(column)
		if column.title() != "" {
			marks = append(marks, vega.Mark{Type: "group", Name: "column-title", Role: "column-title", Title: groupTitle(column)})
		}
	}
	switch {
	case row != nil:
		m := vega.Mark{
			Type: "group", Name: np + rowHeader, Role: "row-header", From: &vega.From{Data: rowDomain}, Sort: headerSort(row),
			Axes: yAxes, Encode: vega.Encode{"update": vega.Props{"height": height}},
		}
		if row.labels() {
			m.Title = headerTitle(row, row.orient("labelOrient"))
		}
		marks = append(marks, m)
	case len(yAxes) > 0 && wrap == nil:
		marks = append(marks, vega.Mark{
			Type: "group", Name: np + rowHeader, Role: "row-header",
			Axes: yAxes, Encode: vega.Encode{"update": vega.Props{"height": height}},
		})
	}
	footer := xAxes
	if column != nil {
		bottom := column.orient("labelOrient") == "bottom"
		if column.labels() && !bottom {
			marks = append(marks, vega.Mark{
				Type: "group", Name: np + "column_header", Role: "column-header", From: &vega.From{Data: colDomain}, Sort: headerSort(column),
				Title: headerTitle(column, ""), Encode: vega.Encode{"update": vega.Props{"width": width}},
			})
		}
		if len(footer) > 0 || column.labels() && bottom {
			m := vega.Mark{
				Type: "group", Name: np + columnFooter, Role: "column-footer", From: &vega.From{Data: colDomain}, Sort: headerSort(column),
				Axes: footer, Encode: vega.Encode{"update": vega.Props{"width": width}},
			}
			if column.labels() && bottom {
				m.Title = headerTitle(column, "bottom")
			}
			marks = append(marks, m)
		}
	} else if len(footer) > 0 && wrap == nil {
		marks = append(marks, vega.Mark{
			Type: "group", Name: np + columnFooter, Role: "column-footer",
			Axes: footer, Encode: vega.Encode{"update": vega.Props{"width": width}},
		})
	}
	var groupby []string
	var sortFields []string
	var orders []string
	agg := map[string][]any{}
	for _, f := range []*facetDef{row, column, wrap} {
		if f == nil {
			continue
		}
		groupby = append(groupby, f.def.out)
		if f.def.bin != nil {
			groupby = append(groupby, f.def.outEnd)
		}
		field := f.def.out
		if f.sortField != "" {
			field = f.sortAs() + "_by_" + f.def.out
			for k, v := range f.sortAggregate("_by_" + f.def.out) {
				agg[k] = append(agg[k], v...)
			}
		}
		sortFields = append(sortFields, "datum["+strconv.Quote(field)+"]")
		orders = append(orders, f.sort)
	}
	facet := &vega.Facet{Name: fname, Data: facetData, GroupBy: vega.List(groupby...)}
	switch {
	case row != nil && column != nil:
		facet.Aggregate = mustJSON(map[string]any{"cross": true})
	case len(agg) > 0:
		facet.Aggregate = mustJSON(agg)
	}
	sortList := make([]any, len(sortFields))
	for i, f := range sortFields {
		sortList[i] = f
	}
	cellMark := vega.Mark{
		Type: "group", Name: np + "cell", Style: vega.Strings{List: []string{"cell"}},
		From:  &vega.From{Facet: facet},
		Sort:  &vega.Sort{Field: lit(sortList), Order: vega.List(orders...)},
		Marks: child.marks, Axes: grids,
		Encode: vega.Encode{"update": vega.Props{"width": width, "height": height}},
	}
	if wrap != nil && wrap.labels() {
		cellMark.Title = headerTitle(wrap, "")
	}
	cellMark.Data = cellDatasets
	marks = append(marks, cellMark)
	out.Marks = marks
	cols := any(1)
	switch {
	case columns > 0:
		cols = columns
	case wrap != nil:
		cols = map[string]any{"signal": "length(data('" + wrapDomain + "'))"}
	case column != nil:
		cols = map[string]any{"signal": "length(data('" + colDomain + "'))"}
	}
	out.Layout = lit(map[string]any{"padding": 20, "columns": cols, "bounds": "full", "align": "all"})
	c.mergeAggregates(out)
	return nil
}

// splitFacet finds the first aggregate of the data of a child of a facet
// that groups by the fields of the facet. The facet then reads the rows
// before that aggregate, and each cell makes its data from the aggregate
// on, without the fields of the facet, from its own rows. The datasets of
// the top level keep the aggregate for the domains of the scales. It
// returns the dataset that the facet reads and the dataset of a cell.
func (c *compiler) splitFacet(main string, fields []*channelDef, sizes *vega.Spec, fname string) (string, vega.Data, bool) {
	facetFields := map[string]bool{}
	for _, f := range fields {
		facetFields[f.out] = true
		facetFields[f.outEnd] = true
	}
	index := func(name string) int {
		for i, d := range c.data {
			if d.Name == name {
				return i
			}
		}
		return -1
	}
	var chain []int
	for i := index(main); i >= 0; {
		chain = append([]int{i}, chain...)
		if len(c.data[i].Source.List) != 1 {
			break
		}
		i = index(c.data[i].Source.List[0])
	}
	at, from := -1, 0
	for k, i := range chain {
		for j, t := range c.data[i].Transform {
			if t.Type != "aggregate" || at >= 0 {
				continue
			}
			var g []string
			_ = json.Unmarshal(t.Params["groupby"], &g)
			for _, f := range g {
				if facetFields[f] {
					at, from = k, j
				}
			}
		}
	}
	if at < 0 {
		return "", vega.Data{}, false
	}
	var rest []vega.Transform
	rest = append(rest, c.data[chain[at]].Transform[from:]...)
	for _, i := range chain[at+1:] {
		rest = append(rest, c.data[i].Transform...)
	}
	cell := vega.Data{Name: fmt.Sprintf("data_%d", c.nData), Source: vega.List(fname)}
	c.nData++
	for _, t := range rest {
		cp := vega.Transform{Type: t.Type, Params: vega.Extra{}}
		maps.Copy(cp.Params, t.Params)
		var g []string
		if json.Unmarshal(t.Params["groupby"], &g) == nil && g != nil {
			g = slices.DeleteFunc(slices.Clone(g), func(f string) bool { return facetFields[f] })
			cp.Params["groupby"] = mustJSON(g)
		}
		cell.Transform = append(cell.Transform, cp)
	}
	facetData := c.data[chain[at]].Name
	if from == 0 {
		if at == 0 {
			return "", vega.Data{}, false
		}
		// The aggregate starts its dataset, so the facet reads the
		// dataset before it.
		facetData = c.data[chain[at-1]].Name
	}
	if from > 0 {
		// The dataset splits before the aggregate: the rows before it stay
		// in the dataset, and a new dataset derives the aggregate.
		parent := chain[at]
		split := vega.Data{
			Name: fmt.Sprintf("data_%d", c.nData), Source: vega.List(c.data[parent].Name),
			Transform: slices.Clone(c.data[parent].Transform[from:]),
		}
		c.nData++
		c.data[parent].Transform = c.data[parent].Transform[:from]
		if at+1 < len(chain) {
			c.data[chain[at+1]].Source = vega.List(split.Name)
		} else {
			rename := func(s string) string {
				if s == facetData {
					return split.Name
				}
				return s
			}
			for i := range sizes.Scales {
				renameDomain(sizes.Scales[i].Domain.Data, rename)
			}
		}
		c.data = slices.Insert(c.data, parent+1, split)
	}
	return facetData, cell, true
}

// wrapHeaders adds the datasets of the rows and the columns of a wrapped
// facet, and returns its title and the headers that hold its axes. The
// cells draw the labels.
func (c *compiler) wrapHeaders(wrap *facetDef, columns int, domain, np string, xAxes, yAxes []vega.Axis, width, height vega.Production) []vega.Mark {
	n := "length(data(\"" + domain + "\"))"
	rows, cols := n, n
	if columns > 0 {
		k := strconv.Itoa(columns)
		rows = "ceil(" + n + " / " + k + ")"
		cols = "min(" + n + ", " + k + ")"
	}
	sequence := func(name, stop string) {
		c.data = append(c.data, vega.Data{Name: name, Transform: []vega.Transform{
			transform("sequence", map[string]any{"start": 0, "stop": map[string]any{"signal": stop}}),
		}})
	}
	var marks []vega.Mark
	if t := wrap.title(); t != "" {
		marks = append(marks, vega.Mark{Type: "group", Name: "facet-title", Role: "column-title", Title: &vega.Title{
			Text: lit(t), Style: vega.Strings{List: []string{"guide-title"}}, Offset: lit(10),
		}})
	}
	if len(yAxes) > 0 {
		sequence(domain+"_row", rows)
		marks = append(marks, vega.Mark{
			Type: "group", Name: np + rowHeader, Role: "row-header", From: &vega.From{Data: domain + "_row"},
			Axes: yAxes, Encode: vega.Encode{"update": vega.Props{"height": height}},
		})
	}
	if len(xAxes) > 0 {
		sequence(domain+"_column", cols)
		marks = append(marks, vega.Mark{
			Type: "group", Name: np + columnFooter, Role: "column-footer", From: &vega.From{Data: domain + "_column"},
			Axes: xAxes, Encode: vega.Encode{"update": vega.Props{"width": width}},
		})
	}
	return marks
}

// mainData returns the dataset that most marks of a view draw.
func (v *view) mainData() string {
	counts := map[string]int{}
	var order []string
	var walk func(ms []vega.Mark)
	walk = func(ms []vega.Mark) {
		for _, m := range ms {
			if m.From != nil {
				name := m.From.Data
				if m.From.Facet != nil {
					name = m.From.Facet.Data
				}
				if name != "" && !slices.Contains(order, name) {
					order = append(order, name)
				}
				counts[name]++
			}
			walk(m.Marks)
		}
	}
	walk(v.marks)
	best := ""
	for _, n := range order {
		if best == "" || counts[n] > counts[best] {
			best = n
		}
	}
	return best
}

// renameData makes a mark and its marks read one dataset in place of
// another.
func renameData(m *vega.Mark, from, to string) {
	if m.From != nil {
		if m.From.Data == from {
			m.From.Data = to
		}
		if m.From.Facet != nil && m.From.Facet.Data == from {
			m.From.Facet.Data = to
		}
	}
	for i := range m.Marks {
		renameData(&m.Marks[i], from, to)
	}
}
