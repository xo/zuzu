package vegalite

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/format"
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
	// sortList is a list of values that sorts the facet, by the index of
	// each value, which a formula gives as the field sortField.
	sortList []any
}

// sortAs returns the name of the aggregate that sorts the facet.
func (f *facetDef) sortAs() string {
	if f.sortOp == "" || f.sortList != nil {
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

// labelText returns the expression of the label of a header, from the
// reference to the value of the facet. A time unit writes its value in the
// format of the unit, such as 2012 for a year.
func (f *facetDef) labelText(ref string) string {
	if d := f.def; d.units != nil {
		fn := "timeFormat"
		if d.utc {
			fn = "utcFormat"
		}
		return fn + "(" + ref + ", " + strconv.Quote(format.TimeUnitSpecifier(d.units, vlSpecifiers)) + ")"
	}
	return "isValid(" + ref + ") ? " + ref + " : \"\"+" + ref
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
			case []any:
				f.sortList = o
				f.sortField = ch + "_" + literal(d.field) + "_sort_index"
				f.sortOp = "max"
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
	if np != "" && !c.innerFacet {
		child.scalePrefix, child.prefixAll = np, true
	}
	child.facetFields = extra
	// The scale of theta is independent in a facet by default, as Vega-Lite
	// documents it, so that each cell draws a whole pie.
	indep, axisIndep := map[string]bool{}, map[string]bool{}
	if _, ok := cell.Encoding["theta"]; ok {
		indep["theta"] = true
	}
	if len(s.Resolve) > 0 {
		var r struct {
			Scale map[string]string `json:"scale"`
			Axis  map[string]string `json:"axis"`
		}
		if json.Unmarshal(s.Resolve, &r) == nil {
			for ch, mode := range r.Scale {
				switch {
				case mode == "independent" && (ch == "x" || ch == "y"):
					indep[ch] = true
				case mode == "shared" && ch == "theta":
					delete(indep, "theta")
				}
			}
			for ch, mode := range r.Axis {
				if mode == "independent" && (ch == "x" || ch == "y") {
					axisIndep[ch] = true
				}
			}
		}
	}
	if len(indep) > 0 {
		// Each cell has its own scale of an independent channel.
		child.independent, child.indepPrefix = indep, child.prefix
	}
	if len(s.Transform) > 0 && len(s.Facet) == 0 {
		cell.Transform = s.Transform
	}
	if _, _, _, _, _, ok, _ := readFacet(cell, repeat); ok && !c.innerFacet {
		if wrap != nil || np != "" {
			return c.issue(path, "a facet of a facet that wraps or has a name")
		}
		return c.facetOfFacet(s, row, column, cell, out, src, path, repeat)
	}
	if len(indep) > 0 && len(cell.Layer) > 0 && src != nil {
		// Each cell copies the datasets of the layers, so the source
		// stays as it is.
		src.open = false
	}
	before := len(c.data)
	// The child names its size child_width and child_height, which the
	// marks of an arc and the projections read.
	child.sizeNames = [2]string{cw, chh}
	if err := c.view(cell, child, src, path+"/spec", repeat); err != nil {
		return err
	}
	main := child.mainData()
	if main == "" {
		return c.issue(path+"/spec", "a facet whose cells draw no data, such as a facet of a facet")
	}
	sizes := &vega.Spec{}
	child.finish(sizes)
	// The child draws in each cell, from the data of the cell. A facet
	// sorted by an aggregate of a field facets the rows before the
	// aggregate of the child, which each cell then makes from its own rows.
	facetData, cellData := main, fname
	var cellDatasets []vega.Data
	sorted := false
	for _, f := range []*facetDef{row, column, wrap} {
		// A list sorts by an index on every row, which needs the rows
		// only when the facet crosses rows and columns.
		if f != nil && f.sortField != "" && (f.sortList == nil || row != nil && column != nil) {
			sorted = true
		}
	}
	if (sorted || len(indep) > 0) && src != nil && len(marksData(child.marks)) > 1 {
		// The marks of a layer read several datasets: each cell makes all
		// of them from its own rows, when the facet sorts or the cells have
		// scales of their own.
		var names map[string]string
		keep := extra
		if !sorted {
			// Cells with scales of their own keep the fields of the facet
			// in the groups of their aggregates.
			keep = nil
		}
		cellDatasets, names = c.copyToCell(before, c.name(src), fname, keep)
		for i := range child.marks {
			renameMarkData(&child.marks[i], func(n string) string {
				if r, ok := names[n]; ok {
					return r
				}
				return n
			})
		}
		facetData, main, cellData = c.name(src), "", ""
		if sorted {
			cellDatasets, child.marks = c.mergeCell(fname, cellDatasets, child.marks)
		}
	}
	var childNames []string
	for _, d := range c.data[before:] {
		childNames = append(childNames, d.Name)
	}
	for _, f := range []*facetDef{row, column, wrap} {
		if f != nil && f.sortField != "" && f.sortList == nil && cellDatasets == nil {
			if fd, d, inside, ok := c.splitFacet(main, extra, sizes, fname); ok {
				facetData, cellData, cellDatasets = fd, d.Name, []vega.Data{d}
				if !inside {
					main = c.shiftNames(childNames, 2, child.marks, sizes.Scales)[main]
				}
			}
		}
	}
	if sorted && cellDatasets == nil && main != "" && src != nil && main != c.name(src) {
		// With no aggregate to split at, each cell makes the datasets of
		// the child from its own rows.
		c.shiftNames(childNames, 2, child.marks, sizes.Scales)
		var names map[string]string
		cellDatasets, names = c.copyToCell(before, c.name(src), fname, extra)
		for i := range child.marks {
			renameMarkData(&child.marks[i], func(n string) string {
				if r, ok := names[n]; ok {
					return r
				}
				return n
			})
		}
		facetData, main, cellData = c.name(src), "", ""
		cellDatasets, child.marks = c.mergeCell(fname, cellDatasets, child.marks)
	}
	if main != "" {
		for i := range child.marks {
			renameData(&child.marks[i], main, cellData)
		}
	}
	cellScales, cellAxes, distinct, sizeSig := c.cellScales(child, indep, sizes, main, cellData, cw, chh)
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
	// An independent axis goes in each cell, after the grids.
	if axisIndep["x"] {
		cellAxes, xAxes = append(cellAxes, xAxes...), nil
	}
	if axisIndep["y"] {
		cellAxes, yAxes = append(cellAxes, yAxes...), nil
	}
	// A list that sorts a facet gives each row the index of its value.
	for _, f := range []*facetDef{row, column, wrap} {
		if f == nil || f.sortList == nil {
			continue
		}
		var b strings.Builder
		for i, v := range f.sortList {
			fmt.Fprintf(&b, "%s===%s ? %d : ", fieldRef(f.def.field), string(mustJSON(v)), i)
		}
		fmt.Fprintf(&b, "%d", len(f.sortList))
		for i := range c.data {
			if c.data[i].Name == facetData {
				c.data[i].Transform = append(c.data[i].Transform, transform("formula", map[string]any{"expr": b.String(), "as": f.sortField}))
			}
		}
	}
	// The datasets of the headers.
	// A facet of rows and columns with cells of their own size counts the
	// values of each cell first, and each header takes the greatest count
	// of its cells.
	crossCounts := ""
	if row != nil && column != nil && len(distinct) > 0 {
		crossCounts = np + "cross_" + np + "column_domain_" + np + "row_domain"
		var fields, ops []any
		for _, ch := range []string{"x", "y"} {
			if d, ok := distinct[ch]; ok {
				fields, ops = append(fields, d.field), append(ops, "distinct")
			}
		}
		c.data = append(c.data, vega.Data{Name: crossCounts, Source: vega.List(facetData), Transform: []vega.Transform{
			transform("aggregate", map[string]any{"groupby": []any{row.def.out, column.def.out}, "fields": fields, "ops": ops}),
		}})
	}
	domain := func(f *facetDef) string {
		name := np + f.channel + "_domain"
		groupby := []any{f.def.out}
		if f.def.bin != nil {
			groupby = append(groupby, f.def.outEnd)
		}
		params := map[string]any{"groupby": groupby}
		agg := map[string][]any{}
		ch := "x"
		if f.channel == "row" {
			ch = "y"
		}
		source := facetData
		if crossCounts != "" {
			source = crossCounts
		}
		if d, ok := distinct[ch]; ok {
			// The header has the size of the cells, from the count of the
			// values of their own scale.
			field, op := d.field, "distinct"
			if crossCounts != "" {
				source, field, op = crossCounts, d.as, "max"
			}
			agg["fields"] = append(agg["fields"], field)
			agg["ops"] = append(agg["ops"], op)
			agg["as"] = append(agg["as"], d.as)
		}
		if f.sortField != "" {
			for k, v := range f.sortAggregate("") {
				agg[k] = append(agg[k], v...)
			}
		}
		for k, v := range agg {
			params[k] = v
		}
		c.data = append(c.data, vega.Data{
			Name: name, Source: vega.Strings{List: []string{source}},
			Transform: []vega.Transform{transform("aggregate", params)},
		})
		return name
	}
	width := prod(map[string]any{"signal": cw})
	height := prod(map[string]any{"signal": chh})
	if w, ok := sizeSig["x"]; ok {
		width = w
	}
	if h, ok := sizeSig["y"]; ok {
		height = h
	}
	headerTitle := func(f *facetDef, orient string) *vega.Title {
		t := &vega.Title{
			Text:   vega.Value{Signal: f.labelText("parent[" + strconv.Quote(f.def.out) + "]")},
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
	var sortList []any
	var orders []string
	agg := map[string][]any{}
	// A facet of rows and columns crosses its fields, so a sort of either
	// by an aggregate looks the aggregate up in the dataset of its header.
	cross := row != nil && column != nil
	lookups := map[string]string{}
	if cross {
		for _, f := range []*facetDef{row, column} {
			if f.sortField == "" {
				continue
			}
			dom := rowDomain
			if f == column {
				dom = colDomain
			}
			name := np + f.channel + "_lookup_domain"
			c.data = append(c.data, vega.Data{Name: name, Source: vega.List(dom), Transform: []vega.Transform{
				transform("formula", map[string]any{"expr": facetKey(f, "datum"), "as": f.channel + "_facet_key"}),
			}})
			lookups[f.channel] = name
		}
	}
	for _, f := range []*facetDef{row, column, wrap} {
		if f == nil {
			continue
		}
		groupby = append(groupby, f.def.out)
		if f.def.bin != nil {
			groupby = append(groupby, f.def.outEnd)
		}
		orders = append(orders, f.sort)
		field := f.def.out
		switch {
		case f.sortField != "" && cross:
			l, key := lookups[f.channel], facetKey(f, "datum.datum")
			at := "indexof(pluck(data(" + strconv.Quote(l) + "), " + strconv.Quote(f.channel+"_facet_key") + "), " + key + ")"
			sortList = append(sortList, map[string]any{"expr": at + " >= 0 ? data(" + strconv.Quote(l) + ")[" + at + "][" + strconv.Quote(f.sortAs()) + "] : null"})
			continue
		case f.sortField != "":
			// The index of a list keeps its name in the cell.
			suffix := "_by_" + f.def.out
			if f.sortList != nil {
				suffix = ""
			}
			field = f.sortAs() + suffix
			for k, v := range f.sortAggregate(suffix) {
				agg[k] = append(agg[k], v...)
			}
		}
		sortList = append(sortList, "datum["+strconv.Quote(field)+"]")
	}
	facet := &vega.Facet{Name: fname, Data: facetData, GroupBy: vega.List(groupby...)}
	for _, ch := range []string{"x", "y"} {
		if d, ok := distinct[ch]; ok {
			agg["fields"] = append(agg["fields"], d.field)
			agg["ops"] = append(agg["ops"], "distinct")
			agg["as"] = append(agg["as"], d.as)
		}
	}
	aggregate := map[string]any{}
	for k, v := range agg {
		aggregate[k] = v
	}
	if row != nil && column != nil {
		aggregate["cross"] = true
	}
	if len(aggregate) > 0 {
		facet.Aggregate = mustJSON(aggregate)
	}
	cellMark := vega.Mark{
		Type: "group", Name: np + "cell", Style: vega.Strings{List: []string{"cell"}},
		From:  &vega.From{Facet: facet},
		Sort:  &vega.Sort{Field: lit(sortList), Order: vega.List(orders...)},
		Marks: child.marks, Axes: append(grids, cellAxes...),
		Encode: vega.Encode{"update": vega.Props{"width": width, "height": height}},
	}
	if wrap != nil && wrap.labels() {
		cellMark.Title = headerTitle(wrap, "")
	}
	cellMark.Data = cellDatasets
	cellMark.Scales = cellScales
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
	out.Layout = lit(map[string]any{"padding": spacing(s, c.cfg.facetSpacing), "columns": cols, "bounds": "full", "align": "all"})
	if !c.nested {
		c.mergeAggregates(out)
	}
	return nil
}

// splitFacet finds the first aggregate of the data of a child of a facet
// that groups by the fields of the facet. The facet then reads the rows
// before that aggregate, and each cell makes its data from the aggregate
// on, without the fields of the facet, from its own rows. The datasets of
// the top level keep the aggregate for the domains of the scales. It
// returns the dataset that the facet reads and the dataset of a cell.
func (c *compiler) splitFacet(main string, fields []*channelDef, sizes *vega.Spec, fname string) (string, vega.Data, bool, bool) {
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
		return "", vega.Data{}, false, false
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
			return "", vega.Data{}, false, false
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
	return facetData, cell, from > 0, true
}

// shiftNames gives the datasets of a list that the compiler numbers the
// numbers some places higher, as vega-lite.js numbers the datasets of the
// child of a facet after those of the facet. It renames them in the marks
// and the scales, and returns the new name of each name.
func (c *compiler) shiftNames(list []string, by int, marks []vega.Mark, scales []vega.Scale) map[string]string {
	names := map[string]string{}
	for _, n := range list {
		names[n] = n
		if m, ok := dataNumber(n); ok {
			names[n] = fmt.Sprintf("data_%d", m+by)
		}
	}
	c.nData += by
	rename := func(n string) string {
		if r, ok := names[n]; ok {
			return r
		}
		return n
	}
	for i := range c.data {
		c.data[i].Name = rename(c.data[i].Name)
		for k := range c.data[i].Source.List {
			c.data[i].Source.List[k] = rename(c.data[i].Source.List[k])
		}
	}
	for i := range marks {
		renameMarkData(&marks[i], rename)
	}
	for i := range scales {
		renameDomain(scales[i].Domain.Data, rename)
	}
	return names
}

// facetKey writes the key of a value of a facet, which tells the values
// apart by their text and its length, as vega-lite.js writes it.
func facetKey(f *facetDef, datum string) string {
	ref := datum + "[" + strconv.Quote(literal(f.def.out)) + "]"
	return "join([isValid(" + ref + ") ? length(toString(" + ref + ")) + ':' + toString(" + ref + ") : '-1:'], '|')"
}

// copyToCell copies the datasets that the child of a facet made, which
// derive from the data of the facet, into each cell. The copies read the
// rows of the cell, and group by the fields of the facet no more. It
// returns the copies and their names by the names of the originals.
func (c *compiler) copyToCell(from int, facetData, fname string, fields []*channelDef) ([]vega.Data, map[string]string) {
	facetFields := map[string]bool{}
	for _, f := range fields {
		facetFields[f.out] = true
		facetFields[f.outEnd] = true
	}
	names := map[string]string{facetData: fname}
	var out []vega.Data
	for _, d := range c.data[from:] {
		if len(d.Source.List) != 1 {
			continue
		}
		src, ok := names[d.Source.List[0]]
		if !ok {
			continue
		}
		name := fmt.Sprintf("data_%d", len(out))
		names[d.Name] = name
		cp := vega.Data{Name: name, Source: vega.List(src)}
		for _, t := range d.Transform {
			nt := vega.Transform{Type: t.Type, Params: vega.Extra{}}
			maps.Copy(nt.Params, t.Params)
			var g []string
			if json.Unmarshal(t.Params["groupby"], &g) == nil && g != nil {
				nt.Params["groupby"] = mustJSON(slices.DeleteFunc(slices.Clone(g), func(f string) bool { return facetFields[f] }))
			}
			cp.Transform = append(cp.Transform, nt)
		}
		out = append(out, cp)
	}
	delete(names, facetData)
	return out, names
}

// mergeCell merges the datasets of a cell as the datasets of the top level
// merge, with the facet as their parent, and renames them in the marks.
func (c *compiler) mergeCell(fname string, data []vega.Data, marks []vega.Mark) ([]vega.Data, []vega.Mark) {
	sub := &compiler{cfg: c.cfg, nData: len(data), parsed: map[int]map[string]bool{}}
	sub.data = append([]vega.Data{{Name: fname}}, data...)
	tmp := &vega.Spec{Marks: marks}
	sub.mergeAggregates(tmp)
	return sub.data[1:], tmp.Marks
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

// distinctField is the count of the values of a field that sizes the
// cells of a facet with an independent discrete scale.
type distinctField struct {
	field, as string
}

// cellScales takes the independent scales of the child of a facet out of
// the scales of the top level, with their axes, for each cell to make from
// its own data. A discrete scale with steps sizes each cell by the count
// of its values, so the signal of the size goes, and the size of a cell
// comes from that count.
func (c *compiler) cellScales(child *view, indep map[string]bool, sizes *vega.Spec, main, cellData, cw, chh string) ([]vega.Scale, []vega.Axis, map[string]distinctField, map[string]vega.Production) {
	distinct := map[string]distinctField{}
	sizeSig := map[string]vega.Production{}
	if len(indep) == 0 {
		return nil, nil, distinct, sizeSig
	}
	rename := func(n string) string {
		if n == main {
			return cellData
		}
		return n
	}
	names := map[string]string{}
	for ch := range indep {
		names[child.sn(ch)] = ch
	}
	var scales []vega.Scale
	var keep []vega.Scale
	for _, sc := range sizes.Scales {
		ch, ok := names[sc.Name]
		if !ok {
			keep = append(keep, sc)
			continue
		}
		renameDomain(sc.Domain.Data, rename)
		scales = append(scales, sc)
		ref := sc.Domain.Data
		if sc.Range.Step.IsZero() || ref == nil {
			continue
		}
		field := ref.Field.Name
		if field == "" && len(ref.Fields) > 0 {
			// The scale of a layer reads the same field of each layer.
			if m, ok := ref.Fields[0].(map[string]any); ok {
				field, _ = m["field"].(string)
			}
		}
		if field == "" {
			continue
		}
		d := distinctField{field: field, as: "distinct_" + field}
		distinct[ch] = d
		inner, outer := 0.0, 0.0
		if x, ok := valueOf(sc.PaddingInner).(float64); ok {
			inner = x
		}
		if x, ok := valueOf(sc.PaddingOuter).(float64); ok {
			outer = x
		}
		if sc.Type == "point" {
			inner = 1
			if x, ok := valueOf(sc.Padding).(float64); ok {
				outer = x
			}
		}
		k := func(x float64) string { return strconv.FormatFloat(x, 'f', -1, 64) }
		sizeSig[ch] = prod(map[string]any{"signal": "bandspace(datum[" + strconv.Quote(d.as) + "], " + k(inner) + ", " + k(outer) + ") * " + child.sn(ch) + "_step"})
	}
	sizes.Scales = keep
	drop := map[string]bool{}
	if _, ok := distinct["x"]; ok {
		drop[cw] = true
	}
	if _, ok := distinct["y"]; ok {
		drop[chh] = true
	}
	sizes.Signals = slices.DeleteFunc(sizes.Signals, func(sg vega.Signal) bool { return drop[sg.Name] })
	var axes, rest []vega.Axis
	for _, a := range sizes.Axes {
		if _, ok := names[a.Scale]; ok {
			axes = append(axes, a)
		} else {
			rest = append(rest, a)
		}
	}
	sizes.Axes = rest
	return scales, axes, distinct, sizeSig
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

// facetOfFacet compiles a facet whose cells are a facet, as vega-lite.js
// does: each cell of the outer facet is a group with the layout of the
// inner facet, which partitions the rows of the cell again. The inner
// facet takes the names of a facet named child, and its headers read the
// rows of the outer cell. The title of a field of each facet in the same
// direction joins the two, as Origin / Cylinders.
func (c *compiler) facetOfFacet(s *Spec, row, column *facetDef, cell *Spec, out *vega.Spec, src *dataNode, path string, repeat map[string]string) error {
	innerRow, innerCol, err := facetRowColumn(cell, repeat)
	if err != nil {
		return err
	}
	outer := row
	if outer == nil {
		outer = column
	}
	if outer == nil || row != nil && column != nil {
		return c.issue(path, "a facet of a facet with rows and columns outside")
	}
	// The inner facet.
	named := *cell
	named.Name = "child"
	inner := &vega.Spec{}
	if src != nil {
		// The cells copy the transforms of their data, so the source
		// stays as it is.
		src.open = false
	}
	wasInner, wasNested := c.innerFacet, c.nested
	c.innerFacet, c.nested = true, true
	before := len(c.data)
	err = c.facet(&named, inner, src, path+"/spec", repeat)
	c.innerFacet, c.nested = wasInner, wasNested
	if err != nil {
		return err
	}
	// The domains of the inner headers read the rows of the outer cell, so
	// they move into its group.
	var cellData []vega.Data
	kept := c.data[:before:before]
	for _, d := range c.data[before:] {
		if strings.HasPrefix(d.Name, "child_") && strings.HasSuffix(d.Name, "_domain") {
			d.Source = vega.List("facet")
			cellData = append(cellData, d)
			continue
		}
		kept = append(kept, d)
	}
	c.data = kept
	// The inner cells partition the rows of the outer cell, and each copies
	// the transforms of the data of the scales from its own rows.
	for i := range inner.Marks {
		m := &inner.Marks[i]
		if m.Name != "child_cell" || m.From == nil || m.From.Facet == nil {
			continue
		}
		main := m.From.Facet.Data
		m.From.Facet.Data = "facet"
		for j := range c.data {
			if c.data[j].Name != main {
				continue
			}
			n, ok := dataNumber(main)
			if !ok {
				break
			}
			// vega-lite.js names the copy in the cell first, and the data
			// of the scales two numbers later.
			top := fmt.Sprintf("data_%d", n+2)
			c.nData = max(c.nData, n+3)
			m.Data = append(m.Data, vega.Data{Name: main, Source: vega.List("child_facet"), Transform: c.data[j].Transform})
			for k := range m.Marks {
				renameData(&m.Marks[k], "child_facet", main)
			}
			c.data[j].Name = top
			for k := range inner.Scales {
				renameDomain(inner.Scales[k].Domain.Data, func(s string) string {
					if s == main {
						return top
					}
					return s
				})
			}
			break
		}
	}
	// A title in the same direction as the outer one joins it.
	title := outer.title()
	same := (row != nil && innerRow != nil) || (column != nil && innerCol != nil)
	if same {
		in := innerRow
		if column != nil {
			in = innerCol
		}
		if t := in.title(); t != "" {
			title += " / " + t
		}
		drop := "row-title"
		if column != nil {
			drop = "column-title"
		}
		inner.Marks = slices.DeleteFunc(inner.Marks, func(m vega.Mark) bool { return m.Name == drop })
	}
	out.Scales = append(out.Scales, inner.Scales...)
	out.Signals = append(out.Signals, inner.Signals...)
	out.Legends = append(out.Legends, inner.Legends...)
	// The outer facet: its domain, its title, its header and its cells.
	channel := "row"
	if column != nil {
		channel = "column"
	}
	field := outer.def.out
	domain := channel + "_domain"
	c.data = append(c.data, vega.Data{Name: domain, Source: vega.List(c.name(src)), Transform: []vega.Transform{
		transform("aggregate", map[string]any{"groupby": []any{field}}),
	}})
	label := &vega.Title{
		Text:  vega.Value{Signal: outer.labelText("parent[" + strconv.Quote(field) + "]")},
		Style: vega.Strings{List: []string{"guide-label"}}, Frame: lit("group"), Offset: lit(10),
	}
	groupTitle := &vega.Title{Text: lit(title), Style: vega.Strings{List: []string{"guide-title"}}, Offset: lit(10)}
	if channel == "row" {
		label.Orient = lit("left")
		groupTitle.Orient = lit("left")
	}
	sort := &vega.Sort{Field: lit("datum[" + strconv.Quote(field) + "]"), Order: vega.List(outer.sort)}
	var marks []vega.Mark
	if title != "" {
		marks = append(marks, vega.Mark{Type: "group", Name: channel + "-title", Role: channel + "-title", Title: groupTitle})
	}
	header := channel + "_header"
	marks = append(marks, vega.Mark{
		Type: "group", Name: header, Role: channel + "-header", From: &vega.From{Data: domain}, Sort: sort, Title: label,
	})
	facet := &vega.Facet{Name: "facet", Data: c.name(src), GroupBy: vega.List(field)}
	layout := map[string]any{}
	_ = json.Unmarshal(inner.Layout.JSON, &layout)
	var encode vega.Encode
	if column != nil && innerCol != nil {
		// The columns of each cell are the values of the inner field in it.
		as := "distinct_" + innerCol.def.field
		facet.Aggregate = mustJSON(map[string]any{"fields": []any{innerCol.def.field}, "ops": []any{"distinct"}, "as": []any{as}})
		encode = vega.Encode{"update": vega.Props{"columns": prod(map[string]any{"field": as})}}
		delete(layout, "columns")
	}
	cellMark := vega.Mark{
		Type: "group", Name: "cell", From: &vega.From{Facet: facet},
		Sort: &vega.Sort{Field: lit([]any{"datum[" + strconv.Quote(field) + "]"}), Order: vega.List(outer.sort)},
		Data: cellData, Marks: inner.Marks, Layout: lit(layout), Encode: encode,
	}
	marks = append(marks, cellMark)
	out.Marks = marks
	cols := any(1)
	if column != nil {
		cols = map[string]any{"signal": "length(data('" + domain + "'))"}
	}
	out.Layout = lit(map[string]any{"padding": spacing(s, c.cfg.facetSpacing), "columns": cols, "bounds": "full", "align": "all"})
	if !c.nested {
		c.mergeAggregates(out)
	}
	return nil
}

// facetRowColumn reads the row and the column of a facet.
func facetRowColumn(s *Spec, repeat map[string]string) (*facetDef, *facetDef, error) {
	if row, column, _, _, _, _, err := readFacet(s, repeat); err != nil || row != nil || column != nil {
		return row, column, err
	}
	return nil, nil, nil
}
