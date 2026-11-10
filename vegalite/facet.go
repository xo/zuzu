package vegalite

import (
	"encoding/json/v2"
	"slices"
	"strconv"

	"github.com/xo/zuzu/vega"
)

// The names of the size of a cell of a facet.
const (
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
			if o, ok := valueOf(def.Sort).(string); ok {
				f.sort = o
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
	if wrap != nil {
		// A wrapped facet lays its cells in rows of columns, which the
		// grid of a column facet draws.
		column = wrap
		if columns == 0 {
			columns = -1
		}
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
	for _, f := range []*facetDef{row, column} {
		if f != nil {
			extra = append(extra, f.def)
		}
	}
	child := newView("child_", c.cfg)
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
	// The child draws in each cell, from the data of the cell.
	for i := range child.marks {
		renameData(&child.marks[i], main, "facet")
	}
	// The child names its size child_width and child_height.
	sizes := &vega.Spec{}
	child.sizeNames = [2]string{childWidth, childHeight}
	child.finish(sizes)
	out.Scales = append(out.Scales, sizes.Scales...)
	out.Signals = append(out.Signals, sizes.Signals...)
	if !sizes.Width.IsZero() {
		out.Signals = append(out.Signals, vega.Signal{Name: childWidth, Value: mustJSON(sizes.Width.Number)})
	}
	if !sizes.Height.IsZero() {
		out.Signals = append(out.Signals, vega.Signal{Name: childHeight, Value: mustJSON(sizes.Height.Number)})
	}
	out.Legends = append(out.Legends, sizes.Legends...)
	var xAxes, yAxes, grids []vega.Axis
	for _, a := range sizes.Axes {
		switch {
		case a.Grid != nil && *a.Grid:
			grids = append(grids, a)
		case a.Scale == "y" || a.Scale == "child_y":
			yAxes = append(yAxes, a)
		default:
			xAxes = append(xAxes, a)
		}
	}
	// The datasets of the headers.
	domain := func(f *facetDef) string {
		name := f.channel + "_domain"
		if f.channel == "facet" {
			name = "column_domain"
		}
		c.data = append(c.data, vega.Data{
			Name: name, Source: vega.Strings{List: []string{main}},
			Transform: []vega.Transform{transform("aggregate", map[string]any{"groupby": []any{f.def.out}})},
		})
		return name
	}
	width := prod(map[string]any{"signal": childWidth})
	height := prod(map[string]any{"signal": childHeight})
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
		return &vega.Sort{Field: lit("datum[" + strconv.Quote(f.def.out) + "]"), Order: vega.Strings{List: []string{f.sort}}}
	}
	var marks []vega.Mark
	var rowDomain, colDomain string
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
			Type: "group", Name: "row_header", Role: "row-header", From: &vega.From{Data: rowDomain}, Sort: headerSort(row),
			Axes: yAxes, Encode: vega.Encode{"update": vega.Props{"height": height}},
		}
		if row.labels() {
			m.Title = headerTitle(row, row.orient("labelOrient"))
		}
		marks = append(marks, m)
	case len(yAxes) > 0:
		marks = append(marks, vega.Mark{
			Type: "group", Name: "row_header", Role: "row-header",
			Axes: yAxes, Encode: vega.Encode{"update": vega.Props{"height": height}},
		})
	}
	footer := xAxes
	if column != nil {
		bottom := column.orient("labelOrient") == "bottom"
		if column.labels() && !bottom {
			marks = append(marks, vega.Mark{
				Type: "group", Name: "column_header", Role: "column-header", From: &vega.From{Data: colDomain}, Sort: headerSort(column),
				Title: headerTitle(column, ""), Encode: vega.Encode{"update": vega.Props{"width": width}},
			})
		}
		if len(footer) > 0 || column.labels() && bottom {
			m := vega.Mark{
				Type: "group", Name: "column_footer", Role: "column-footer", From: &vega.From{Data: colDomain}, Sort: headerSort(column),
				Axes: footer, Encode: vega.Encode{"update": vega.Props{"width": width}},
			}
			if column.labels() && bottom {
				m.Title = headerTitle(column, "bottom")
			}
			marks = append(marks, m)
		}
	} else if len(footer) > 0 {
		marks = append(marks, vega.Mark{
			Type: "group", Name: "column_footer", Role: "column-footer",
			Axes: footer, Encode: vega.Encode{"update": vega.Props{"width": width}},
		})
	}
	var groupby []string
	var sortFields []string
	var orders []string
	for _, f := range []*facetDef{row, column} {
		if f == nil {
			continue
		}
		groupby = append(groupby, f.def.out)
		sortFields = append(sortFields, "datum["+strconv.Quote(f.def.out)+"]")
		orders = append(orders, f.sort)
	}
	facet := &vega.Facet{Name: "facet", Data: main, GroupBy: vega.List(groupby...)}
	if row != nil && column != nil {
		facet.Aggregate = mustJSON(map[string]any{"cross": true})
	}
	sortList := make([]any, len(sortFields))
	for i, f := range sortFields {
		sortList[i] = f
	}
	cellMark := vega.Mark{
		Type: "group", Name: "cell", Style: vega.Strings{List: []string{"cell"}},
		From:  &vega.From{Facet: facet},
		Sort:  &vega.Sort{Field: lit(sortList), Order: vega.List(orders...)},
		Marks: child.marks, Axes: grids,
		Encode: vega.Encode{"update": vega.Props{"width": width, "height": height}},
	}
	marks = append(marks, cellMark)
	out.Marks = marks
	cols := any(1)
	switch {
	case columns > 0:
		cols = columns
	case column != nil:
		cols = map[string]any{"signal": "length(data('" + colDomain + "'))"}
	}
	out.Layout = lit(map[string]any{"padding": 20, "columns": cols, "bounds": "full", "align": "all"})
	c.mergeAggregates(out)
	return nil
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
