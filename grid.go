package zuzu

import (
	"math"

	"github.com/xo/zuzu/layout"
	"github.com/xo/zuzu/scene"
)

// The two directions of a grid. The columns follow each other along x, and
// the rows along y. A property of a layout that takes an object of the form
// {"row": ..., "column": ...} keeps one value for each direction.
const (
	gridColumn = 0
	gridRow    = 1
)

// gridOptions holds the properties of a layout, as the documentation of the
// layout of Vega defines them.
type gridOptions struct {
	// columns is the number of columns, from 1 to the number of cells.
	columns int
	// flush gives each cell only its width and its height, for the bounds
	// flush, rather than the full bounds of what it draws.
	flush bool
	// align is all, each or none, for each direction.
	align [2]string
	// center centers each cell in its column or its row.
	center [2]bool
	// padding is the space between two columns or two rows.
	padding [2]float64
	// headerBand and footerBand place a header or a footer at a share of
	// the bounds of its cell, or at the origin of the cell when nil.
	headerBand, footerBand [2]*float64
	// titleBand places a title at a share of the bounds of the grid.
	titleBand [2]float64
	// titleEnd puts a title after the grid, at the right or the bottom,
	// for the titleAnchor end.
	titleEnd [2]bool
	// offset moves each kind of header, footer and title away from the
	// grid, by the names of the property offset, such as rowHeader.
	offset map[string]float64
}

// readGridOptions reads the properties of a layout for a number of cells.
// A property that the layout does not set takes the default of Vega: the
// align none, no padding, no offset and one row.
func readGridOptions(v map[string]any, cells int) gridOptions {
	o := gridOptions{columns: cells, flush: v["bounds"] == "flush", offset: map[string]float64{}}
	if c := number(v["columns"], 0); c >= 1 {
		o.columns = min(int(c), cells)
	}
	for d, key := range [2]string{"column", "row"} {
		o.align[d], _ = gridSide(v["align"], key).(string)
		o.center[d], _ = gridSide(v["center"], key).(bool)
		o.padding[d] = number(gridSide(v["padding"], key), 0)
		o.headerBand[d] = gridBand(gridSide(v["headerBand"], key))
		o.footerBand[d] = gridBand(gridSide(v["footerBand"], key))
		o.titleBand[d] = number(gridSide(v["titleBand"], key), 0.5)
		o.titleEnd[d] = gridSide(v["titleAnchor"], key) == "end"
	}
	for _, key := range []string{"rowHeader", "rowFooter", "rowTitle", "columnHeader", "columnFooter", "columnTitle"} {
		o.offset[key] = number(gridSide(v["offset"], key), 0)
	}
	return o
}

// gridSide returns the value of a property for one side: the value of its
// key when the property is an object, or else the property itself.
func gridSide(v any, key string) any {
	if m, ok := v.(map[string]any); ok {
		return m[key]
	}
	return v
}

// gridBand reads a band of a header or a footer, which is nil when the
// layout does not set it.
func gridBand(v any) *float64 {
	if v == nil {
		return nil
	}
	f := number(v, 0)
	return &f
}

// resolveSignals evaluates each object of the form {"signal": ...} inside a
// value, as a layout can give each of its properties by a signal.
func (s *scope) resolveSignals(v any) (any, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return v, nil
	}
	if sig, ok := m["signal"].(string); ok && len(m) == 1 {
		return s.eval(sig, nil)
	}
	out := make(map[string]any, len(m))
	for k, x := range m {
		r, err := s.resolveSignals(x)
		if err != nil {
			return nil, err
		}
		out[k] = r
	}
	return out, nil
}

// grid holds the cells of a layout in rows and columns, while gridLayout
// places them and then the headers beside them.
type grid struct {
	opts  gridOptions
	cells []*scene.Item
	rows  int
	// boxes are the full bounds of each cell, in the frame of the grid,
	// once the cell is in place.
	boxes []layout.Bounds
	// bounds returns the full bounds of an item, in its own frame.
	bounds func(*scene.Item) layout.Bounds
}

// gridLayout places the groups of a scope that has a layout in a grid, as
// the layout of Vega does. The cells fill rows of a number of columns, from
// the origin of the scope. The row headers stand at the left of the grid
// and the row footers at its right, one for each row. The column headers
// stand above the grid and the column footers below it, one for each
// column. A title stands beyond the headers or the footers.
func (s *scope) gridLayout(marks []scene.Mark) error {
	v, err := s.value(s.spec.Layout)
	if err != nil {
		return err
	}
	if v, err = s.resolveSignals(v); err != nil {
		return err
	}
	opts, _ := v.(map[string]any)
	if opts == nil {
		return nil
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
			case "axis", "legend", "title":
				// A guide is not a cell of the grid.
			default:
				cells = append(cells, it)
			}
		}
	}
	if len(cells) == 0 {
		return nil
	}
	g := &grid{
		opts:  readGridOptions(opts, len(cells)),
		cells: cells,
		bounds: func(it *scene.Item) layout.Bounds {
			m := scene.Mark{Type: scene.Group, Items: []scene.Item{*it}}
			m.Items[0].X, m.Items[0].Y = 0, 0
			return st.MarkBounds(&m)
		},
	}
	cols := g.opts.columns
	g.rows = (len(cells) + cols - 1) / cols
	// lo is how far each cell reaches before its origin, and hi how far
	// it reaches after it, along each direction. Vega rounds both up to
	// whole pixels.
	var lo, hi [2][]float64
	for d := range 2 {
		lo[d], hi[d] = make([]float64, len(cells)), make([]float64, len(cells))
	}
	for i, it := range cells {
		b := layout.Bounds{X2: it.Width, Y2: it.Height}
		if !g.opts.flush {
			if fb := g.bounds(it); !fb.IsEmpty() {
				b = fb
			}
		} else if t := s.titleTop(it, st); t < 0 {
			// The look puts the title of a vertical axis above the plot,
			// where Vega turns it beside the axis, so a flush cell keeps
			// room for it, and the title does not cover the cell above.
			b.Y1 = t
		}
		lo[gridColumn][i], hi[gridColumn][i] = math.Ceil(math.Max(0, -b.X1)), math.Ceil(math.Max(0, b.X2))
		lo[gridRow][i], hi[gridRow][i] = math.Ceil(math.Max(0, -b.Y1)), math.Ceil(math.Max(0, b.Y2))
	}
	// A row header can hold the vertical axis of its row, as in a facet of
	// Vega-Lite. Vega counts only the cells in the size of a row, but the
	// look puts the title of that axis above the plot, and the bounds of
	// the look are closer than those of Vega, so the labels of two rows
	// can meet. So the cells of a row keep room for the reach of its
	// header above and below them.
	headerAbove := make([]float64, g.rows)
	for j, it := range byRole["row-header"][:min(len(byRole["row-header"]), g.rows)] {
		hb := g.bounds(it)
		if hb.IsEmpty() {
			continue
		}
		above, below := math.Ceil(math.Max(0, -hb.Y1)), math.Ceil(math.Max(0, hb.Y2))
		headerAbove[j] = above
		for i := j * cols; i < min((j+1)*cols, len(cells)); i++ {
			lo[gridRow][i] = math.Max(lo[gridRow][i], above)
			hi[gridRow][i] = math.Max(hi[gridRow][i], below)
		}
	}
	xs := g.offsets(gridColumn, lo[gridColumn], hi[gridColumn])
	ys := g.offsets(gridRow, lo[gridRow], hi[gridRow])
	width, height := 0.0, 0.0
	g.boxes = make([]layout.Bounds, len(cells))
	for i, it := range cells {
		it.X, it.Y = xs[i], ys[i]
		width = math.Max(width, it.X+hi[gridColumn][i])
		height = math.Max(height, it.Y+hi[gridRow][i])
		b := g.bounds(it)
		if b.IsEmpty() {
			b = layout.Bounds{}
		}
		g.boxes[i] = layout.Bounds{X1: b.X1 + it.X, Y1: b.Y1 + it.Y, X2: b.X2 + it.X, Y2: b.Y2 + it.Y}
		// The bounds of a cell take the reach of its row header above it,
		// such as the title of the axis of the row, which the look puts
		// above the plot, so that a column header stands clear of it.
		if r := i / cols; r < len(headerAbove) {
			g.boxes[i].Y1 = math.Min(g.boxes[i].Y1, it.Y-headerAbove[r])
		}
	}
	off := g.opts.offset
	left := g.placeEdge(byRole["row-header"], gridRow, false, off["rowHeader"], g.opts.headerBand[gridRow])
	right := g.placeEdge(byRole["row-footer"], gridRow, true, off["rowFooter"], g.opts.footerBand[gridRow])
	top := g.placeEdge(byRole["column-header"], gridColumn, false, off["columnHeader"], g.opts.headerBand[gridColumn])
	bottom := g.placeEdge(byRole["column-footer"], gridColumn, true, off["columnFooter"], g.opts.footerBand[gridColumn])
	// A title stands beyond the headers, or beyond the footers for the
	// titleAnchor end, at its band of the bounds of the cells.
	all := g.boxes[0]
	for _, b := range g.boxes[1:] {
		all = layout.Bounds{X1: math.Min(all.X1, b.X1), Y1: math.Min(all.Y1, b.Y1), X2: math.Max(all.X2, b.X2), Y2: math.Max(all.Y2, b.Y2)}
	}
	rowX := left - off["rowTitle"]
	if g.opts.titleEnd[gridRow] {
		rowX = right + off["rowTitle"]
	}
	for _, it := range byRole["row-title"] {
		// The title was drawn for a group of no height, centered on 0.
		it.X, it.Y = rowX, jsRound(all.Y1+g.opts.titleBand[gridRow]*(all.Y2-all.Y1))
	}
	colY := top - off["columnTitle"]
	if g.opts.titleEnd[gridColumn] {
		colY = bottom + off["columnTitle"]
	}
	for _, it := range byRole["column-title"] {
		it.X, it.Y = jsRound(all.X1+g.opts.titleBand[gridColumn]*(all.X2-all.X1)), colY
	}
	if s.item != nil {
		s.item.Width, s.item.Height = math.Max(s.item.Width, width), math.Max(s.item.Height, height)
	}
	return nil
}

// offsets returns the offset of the origin of each cell along a direction.
// The first column or row starts at 0. Each next one starts after the
// greatest reach hi of the cells of the one before it, the padding, and the
// reach lo of the cell before its origin. The align each takes the greatest
// lo of the column or the row, so that its cells line up. The align all
// takes the greatest lo and the greatest hi of the whole grid, so that every
// column or row has one size. The align none takes the lo of each cell, so
// that the cells of a column or a row follow each other and need not line
// up.
func (g *grid) offsets(d int, lo, hi []float64) []float64 {
	cols := g.opts.columns
	track, back, tracks := func(i int) int { return i % cols }, 1, cols
	if d == gridRow {
		track, back, tracks = func(i int) int { return i / cols }, cols, g.rows
	}
	pad, align := g.opts.padding[d], g.opts.align[d]
	ext, reach := make([]float64, tracks), make([]float64, tracks)
	extAll, reachAll := 0.0, 0.0
	for i := range lo {
		t := track(i)
		ext[t] = math.Max(ext[t], hi[i])
		extAll = math.Max(extAll, hi[i])
		reach[t] = math.Max(reach[t], pad+lo[i])
		if t > 0 {
			reachAll = math.Max(reachAll, pad+lo[i])
		}
	}
	out := make([]float64, len(lo))
	for i := range lo {
		t := track(i)
		if t > 0 {
			switch align {
			case "all":
				out[i] = out[i-back] + reachAll + extAll
			case "each":
				out[i] = out[i-back] + reach[t] + ext[t-1]
			default:
				out[i] = out[i-back] + pad + lo[i] + ext[t-1]
			}
		}
	}
	if !g.opts.center[d] {
		return out
	}
	// The center moves each cell to the middle of its column or its row,
	// as the documentation of Vega says. The align none has no columns or
	// rows to center in. vega.js 6.4 centers only the cells of the first
	// column and of the first row.
	for i := range out {
		switch align {
		case "all":
			out[i] += (extAll - hi[i]) / 2
		case "each":
			out[i] += (ext[track(i)] - hi[i]) / 2
		}
	}
	return out
}

// placeEdge places the headers or the footers of one kind and returns the
// edge that they reach, where a title beyond them starts. The headers and
// the footers of the rows stand along the rows, at the left or at the
// right of the grid, and those of the columns stand along the columns,
// above or below it. Each stands off the bounds of the cells by its
// offset, and lines up with the origin of its cell, or with its band of the
// bounds of the cell. A kind that has more items than rows or columns
// leaves the rest where they are, as Vega does.
func (g *grid) placeEdge(items []*scene.Item, along int, end bool, offset float64, band *float64) float64 {
	cols, n := g.opts.columns, len(g.cells)
	// The edge starts at the origin of the grid, as in Vega.
	edge := 0.0
	grow := func(lo, hi float64) {
		if end {
			edge = math.Max(edge, hi)
		} else {
			edge = math.Min(edge, lo)
		}
	}
	for i, b := range g.boxes {
		r, c := i/cols, i%cols
		switch {
		case along == gridRow && !end && c == 0, along == gridRow && end && c == cols-1:
			grow(b.X1, b.X2)
		case along == gridColumn && !end, along == gridColumn && end && r == g.rows-1:
			grow(b.Y1, b.Y2)
		}
	}
	if len(items) == 0 {
		return edge
	}
	at := edge - offset
	if end {
		at = edge + offset
	}
	edge = 0
	limit := cols
	if along == gridRow {
		limit = g.rows
	}
	for j, it := range items[:min(len(items), limit)] {
		// i is the cell that the item lines up with: the first or the last
		// cell of its row, or the first or the last cell of its column.
		var i int
		switch {
		case along == gridRow && !end:
			i = j * cols
		case along == gridRow:
			i = min(j*cols+cols-1, n-1)
		case !end:
			i = j
		default:
			i = (g.rows-1)*cols + j
			if i >= n {
				i -= cols
			}
		}
		cell, b := g.cells[i], g.boxes[i]
		if along == gridRow {
			it.X, it.Y = at, cell.Y
			if band != nil {
				it.Y = jsRound(b.Y1 + *band*(b.Y2-b.Y1))
			}
		} else {
			it.X, it.Y = cell.X, at
			if band != nil {
				it.X = jsRound(b.X1 + *band*(b.X2-b.X1))
			}
			if end && it.Width == 0 {
				// A column footer without a width takes that of its cell.
				it.Width = cell.Width
			}
		}
		hb := g.bounds(it)
		if hb.IsEmpty() {
			continue
		}
		if along == gridRow {
			grow(hb.X1+it.X, hb.X2+it.X)
		} else {
			grow(hb.Y1+it.Y, hb.Y2+it.Y)
		}
	}
	return edge
}

// jsRound rounds a number as Math.round of JavaScript does, with a half
// rounded up.
func jsRound(v float64) float64 {
	return math.Floor(v + 0.5)
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
