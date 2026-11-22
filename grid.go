package zuzu

import (
	"math"

	"github.com/xo/zuzu/layout"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/vega"
)

// gridLayout places the groups of a scope that has a layout in a grid, as
// the layout of Vega does: the cells in rows of a number of columns, the
// row headers at the left of each row, the column headers above each
// column and the column footers below the last row, and the titles beside
// the headers. A cell takes the full bounds of what it draws, with its
// axes, so that two cells never overlap, or only its width and its height
// with the bounds flush.
func (s *scope) gridLayout(marks []scene.Mark, path string) error {
	v, err := s.value(s.spec.Layout)
	if err != nil {
		return err
	}
	opts, _ := v.(map[string]any)
	if opts == nil {
		return nil
	}
	padRow, padCol := 20.0, 20.0
	switch p := opts["padding"].(type) {
	case float64:
		padRow, padCol = p, p
	case map[string]any:
		padRow, padCol = number(p["row"], 20), number(p["column"], 20)
	}
	st := s.rt.style()
	var cells []*scene.Item
	byRole := map[string][]*scene.Item{}
	for i := range marks {
		m := &marks[i]
		if m.Type != scene.Group {
			continue
		}
		for j := range m.Items {
			it := &m.Items[j]
			switch m.Role {
			case "row-header", "row-footer", "column-header", "column-footer", "row-title", "column-title":
				byRole[m.Role] = append(byRole[m.Role], it)
			case "scope":
				cells = append(cells, it)
			}
		}
	}
	if len(cells) == 0 {
		return nil
	}
	columns := len(cells)
	switch c := opts["columns"].(type) {
	case float64:
		columns = int(c)
	case map[string]any:
		if sig, ok := c["signal"].(string); ok {
			x, err := s.eval(sig, nil)
			if err != nil {
				return err
			}
			columns = int(number(x, float64(len(cells))))
		}
	}
	columns = max(1, min(columns, len(cells)))
	if err := s.gridIssues(opts, cells, path); err != nil {
		return err
	}
	rows := (len(cells) + columns - 1) / columns
	// The extent of each cell beyond its box, from its axes and its
	// titles.
	bounds := func(it *scene.Item) layout.Bounds {
		g := scene.Mark{Type: scene.Group, Items: []scene.Item{*it}}
		g.Items[0].X, g.Items[0].Y = 0, 0
		return st.MarkBounds(&g)
	}
	left := make([]float64, columns)
	right := make([]float64, columns)
	top := make([]float64, rows)
	bottom := make([]float64, rows)
	// The bounds flush give a cell only its width and its height, so that
	// its titles and its labels reach into the padding, as in Vega.
	flush := opts["bounds"] == "flush"
	for i, it := range cells {
		r, c := i/columns, i%columns
		b := layout.Bounds{X2: it.Width, Y2: it.Height}
		if !flush {
			b = bounds(it)
		} else if t := s.titleTop(it, st); t < 0 {
			// The look puts the title of a vertical axis above the plot,
			// where Vega turns it beside the axis, so a flush cell keeps
			// room for it, and the title does not cover the cell above.
			b.Y1 = t
		}
		left[c] = math.Max(left[c], -math.Min(0, b.X1))
		right[c] = math.Max(right[c], math.Max(it.Width, b.X2))
		top[r] = math.Max(top[r], -math.Min(0, b.Y1))
		bottom[r] = math.Max(bottom[r], math.Max(it.Height, b.Y2))
	}
	// The headers widen the first column and the first row.
	for i, it := range byRole["row-header"] {
		b := bounds(it)
		left[0] = math.Max(left[0], -math.Min(0, b.X1))
		r := min(i, rows-1)
		top[r] = math.Max(top[r], -math.Min(0, b.Y1))
	}
	for _, it := range byRole["column-header"] {
		top[0] = math.Max(top[0], -math.Min(0, bounds(it).Y1))
	}
	xs := make([]float64, columns)
	x := 0.0
	for c := range columns {
		x += left[c]
		xs[c] = x
		x += right[c] + padCol
	}
	ys := make([]float64, rows)
	y := 0.0
	for r := range rows {
		y += top[r]
		ys[r] = y
		y += bottom[r] + padRow
	}
	width, height := x-padCol, y-padRow
	for i, it := range cells {
		it.X, it.Y = xs[i%columns], ys[i/columns]
	}
	for i, it := range byRole["row-header"] {
		it.X, it.Y = xs[0], ys[min(i, rows-1)]
	}
	for i, it := range byRole["row-footer"] {
		c := columns - 1
		it.X, it.Y = xs[c]+right[c], ys[min(i, rows-1)]
	}
	for i, it := range byRole["column-header"] {
		it.X, it.Y = xs[min(i, columns-1)], ys[0]
	}
	last := ys[rows-1] + bottom[rows-1]
	for i, it := range byRole["column-footer"] {
		c := min(i, columns-1)
		it.X, it.Y = xs[c], last
		if it.Width == 0 {
			it.Width = right[c]
		}
	}
	// A title sits beyond the headers, centered on the grid.
	extent := func(role string, edge func(layout.Bounds) float64) float64 {
		out := 0.0
		for _, it := range byRole[role] {
			b := bounds(it)
			out = math.Max(out, edge(layout.Bounds{X1: b.X1 + it.X, Y1: b.Y1 + it.Y, X2: b.X2 + it.X, Y2: b.Y2 + it.Y}))
		}
		return out
	}
	for _, it := range byRole["row-title"] {
		reach := extent("row-header", func(b layout.Bounds) float64 { return -b.X1 })
		// The title was drawn for a group of no height, centered on 0.
		it.X, it.Y = -reach, height/2
	}
	for _, it := range byRole["column-title"] {
		reach := extent("column-header", func(b layout.Bounds) float64 { return -b.Y1 })
		it.X, it.Y = width/2, -reach
	}
	if s.item != nil {
		s.item.Width, s.item.Height = math.Max(s.item.Width, width), math.Max(s.item.Height, height)
	}
	return nil
}

// gridIssues reports the parts of a layout that the grid does not follow
// (hard rule 4). The grid aligns each column and each row on its own, as
// the align each of Vega does. The align all gives the same grid when every
// cell has the same size, as in a facet, so it goes in the report only when
// the sizes differ. The align none and center go in the report.
func (s *scope) gridIssues(opts map[string]any, cells []*scene.Item, path string) error {
	align := opts["align"]
	if m, ok := align.(map[string]any); ok {
		if m["row"] == m["column"] {
			align = m["row"]
		}
	}
	report := false
	switch align {
	case nil, "each":
	case "all":
		for _, c := range cells[1:] {
			if c.Width != cells[0].Width || c.Height != cells[0].Height {
				report = true
			}
		}
	default:
		report = true
	}
	if report {
		if err := s.rt.issue(vega.ErrUnsupported, path+"/align", "the align %v of a layout", align); err != nil {
			return err
		}
	}
	if _, ok := opts["center"]; ok {
		if err := s.rt.issue(vega.ErrUnsupported, path+"/center", "the center of a layout"); err != nil {
			return err
		}
	}
	return nil
}

// titleTop returns the top of the highest title of an axis in a cell, in
// the frame of the cell, or 0 when no title reaches above the cell. It
// looks into the groups inside the cell, such as the views of a
// concatenation.
func (s *scope) titleTop(cell *scene.Item, st layout.Style) float64 {
	top := 0.0
	var walk func(marks []scene.Mark, dx, dy float64)
	walk = func(marks []scene.Mark, dx, dy float64) {
		for i := range marks {
			m := &marks[i]
			if m.Role == "axis-title" {
				b := st.MarkBounds(m)
				if !b.IsEmpty() {
					top = math.Min(top, dy+b.Y1)
				}
				continue
			}
			if m.Type != scene.Group {
				continue
			}
			for j := range m.Items {
				g := &m.Items[j]
				walk(g.Items, dx+g.X, dy+g.Y)
			}
		}
	}
	walk(cell.Items, 0, 0)
	return top
}
