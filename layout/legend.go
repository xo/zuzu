package layout

import (
	"cmp"
	"math"
	"slices"
	"strconv"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/scene"
)

// Legend is a legend, with the settings that the view resolved from the
// spec. Its entries take their values from the first of its scales that it
// has, in the order of Vega: the size, the shape, the fill, the stroke, the
// dash, the width of the stroke or the opacity.
type Legend struct {
	Fill, Stroke, Shape, Size scale.Scale
	// StrokeDash is the scale of the dashes, whose entries are short lines.
	StrokeDash scale.Scale
	// StrokeWidth is the scale of the widths of lines, whose entries are
	// short lines, and Opacity the scale of the opacity of the symbols.
	StrokeWidth, Opacity scale.Scale
	Title                string
	// Values are the values of the entries, or nil for those of the scale.
	Values []any
	// Format writes the label of a value.
	Format func(any) string
	// Width is the width that a row of entries can fill before it wraps.
	Width float64
	// Line draws each entry as a short line, for a line chart.
	Line bool
	// Gradient draws a continuous color scale as a bar, rather than as
	// entries.
	Gradient bool
	// GradientLength is the length of the bar, and GradientThickness its
	// width. 0 takes the length or the width of the look.
	GradientLength, GradientThickness float64
	// Direction is horizontal or vertical. A vertical legend draws its
	// entries in one column, and its gradient upright.
	Direction string
	// Columns is the number of columns of the entries. 0 draws them in one
	// row that wraps at Width, or in one column for a vertical legend.
	Columns int
	// ClipHeight cuts each symbol to this height about its center, and
	// sets the height of each row. 0 cuts no symbol.
	ClipHeight float64
	// SymbolLimit is the most entries that the legend draws. The last of
	// them counts the values that do not fit. 0 takes the limit of the
	// look.
	SymbolLimit int
	// LabelLimit is the widest that a label draws before an ellipsis cuts
	// it. 0 takes the limit of the look.
	LabelLimit float64
	// SymbolType is the shape of a symbol that no scale shapes, and
	// SymbolSize is the area of a symbol that no scale sizes. Empty and 0
	// take the symbols of the look.
	SymbolType string
	SymbolSize float64
	// Hollow draws each symbol as an outline with no fill, as the marks
	// that the legend names draw.
	Hollow bool
	// Color is the color of the symbols of a legend with no scale of color,
	// or empty for the first categorical color.
	Color string
	// LineValues holds the values, written as strings, whose marks are
	// lines, and whose symbols are short lines.
	LineValues map[string]bool
	// RowPadding and ColumnPadding are the space between two rows and
	// between two columns of entries. nil takes the space of the look.
	RowPadding, ColumnPadding *float64
	// Entries draws a legend of sizes alone as entries, never as nested
	// circles, because the spec sets a property of its entries, such as
	// its direction.
	Entries bool

	// The fields below hold the properties of the spec that win over the
	// look (docs/STYLE.md). A zero value keeps the look.

	// TitleOrient is the side of the entries where the title stands: top,
	// bottom, left or right. "" keeps the side of the look: at the left of
	// a row, and above a grid or nested circles.
	TitleOrient string
	// TitleAnchor is start, middle or end: where the title stands along
	// its side. "" keeps the place of the look.
	TitleAnchor string
	// TitlePadding is the space between the title and the entries, or nil
	// for the space of the look.
	TitlePadding *float64
	// TitleAlign and TitleBaseline are the align and the baseline of the
	// title, or "" for those of its place.
	TitleAlign, TitleBaseline string
	// TitleLimit is the widest that the title draws before an ellipsis
	// cuts it, or 0 for no limit. TitleLineHeight is the height of a line
	// of the title, or 0 for the height of the look.
	TitleLimit, TitleLineHeight float64
	// Padding is the space between the edge of the legend and what it
	// holds. FillColor, StrokeColor and CornerRadius paint the frame of
	// the legend, at that edge.
	Padding                float64
	FillColor, StrokeColor string
	CornerRadius           float64
	// LabelAlign and LabelBaseline are the align and the baseline of each
	// label, or "" for those of the look.
	LabelAlign, LabelBaseline string
	// LabelOffset is the space between a symbol or a gradient and its
	// label, or nil for the space of the look.
	LabelOffset *float64
	// LabelOverlap drops every other label of a gradient, in turns, until
	// no two labels stand closer than LabelSeparation.
	LabelOverlap    bool
	LabelSeparation float64
	// SymbolOffset moves each symbol to the right, in pixels.
	SymbolOffset float64
	// SymbolBaseFill and SymbolBaseStroke paint the symbols when no scale
	// of the legend gives the fill or the stroke, or are "" for the color
	// of the look.
	SymbolBaseFill, SymbolBaseStroke string
	// TickCount is the number of values that a continuous scale gives the
	// entries, or the labels of its gradient, or 0 for those of the look.
	// TickMinStep is the least step between two of these values, or 0.
	TickCount, TickMinStep float64
	// GridAlign is how the columns of a grid of entries line up: each, the
	// default, makes each column as wide as its widest entry, all makes
	// each column as wide as the widest entry, and none puts each entry
	// right after the one before it.
	GridAlign string
}

// The sizes of a legend, in docs/STYLE.md.
const (
	swatch    = 10
	swatchGap = 6
	entryGap  = 16
	rowHeight = 18
	// symbolLimit and labelLimit are the most entries of a legend and the
	// widest label, as in Vega.
	symbolLimit = 30
	labelLimit  = 160
	// nestGap is the least space between the tops of two nested circles,
	// so that their labels do not touch.
	nestGap = LabelSize + 2
)

// Legend draws a legend as a group of the role legend, at the origin: its
// title, then a row of entries that wraps, or a gradient bar.
func (s Style) Legend(l *Legend) scene.Mark {
	group := scene.Item{}
	x, y := 0.0, 0.0
	columns := l.Columns
	if columns <= 0 && l.Direction == "vertical" {
		columns = 1
	}
	gradient := l.Gradient && l.color() != nil && !scale.Discrete(l.color().Type())
	nested := !gradient && l.nested()
	var title *scene.Item
	// A title that the spec places goes beside the entries after they are
	// laid out, and else at the place of the look.
	placed := l.Title != "" && (l.TitleOrient != "" || l.TitleAnchor != "" || l.TitlePadding != nil)
	if l.Title != "" {
		// The title is at the left of a row, as docs/STYLE.md draws a
		// legend, and above a column or a grid.
		t := s.label(0, swatch/2, l.Title, SmallSize, 400, s.Theme.Faint, "left", "middle")
		t.Limit, t.LineHeight = l.TitleLimit, l.TitleLineHeight
		title = &t
		switch {
		case placed:
		case columns > 0 || nested:
			y = rowHeight
		default:
			x = s.textWidth(&t) + entryGap
		}
	}
	var body []scene.Mark
	mid := swatch / 2.0
	switch {
	case gradient:
		body = s.gradient(l, x, y)
		if l.Direction != "vertical" {
			mid = l.thickness() / 2
		}
	case nested:
		body = s.nested(l, y)
	default:
		var m scene.Mark
		m, mid = s.entries(l, x, y, columns)
		body = []scene.Mark{m}
		if title != nil && y == 0 && !placed {
			// The title stays in the middle of the first row, which a
			// large symbol makes higher.
			title.Y = mid
		}
	}
	if placed {
		s.placeTitle(l, title, body, mid, columns > 0 || nested)
	}
	if title != nil {
		if l.TitleAlign != "" {
			title.Align = l.TitleAlign
		}
		if l.TitleBaseline != "" {
			title.Baseline = l.TitleBaseline
		}
		group.Items = append(group.Items, scene.Mark{Type: scene.Text, Role: "legend-title", Items: []scene.Item{*title}})
	}
	group.Items = append(group.Items, body...)
	if placed {
		// The legend starts at the top left of its title and its entries,
		// as in Vega.
		b := s.marksBounds(group.Items)
		shiftMarks(group.Items, -b.X1, -b.Y1)
	}
	s.frame(l, &group)
	return scene.Mark{Type: scene.Group, Role: "legend", Items: []scene.Item{group}}
}

// legendTitleGap is the space of the look between a title above or below the
// entries of a legend and the entries.
const legendTitleGap = 6

// placeTitle places the title of a legend at the side of its body that
// the spec asks for, the body at the origin. A title at the top or the
// bottom stands at the start, the middle or the end of the body, and one
// at the left or the right at its top, its middle or its bottom. With no
// anchor, a title at the left or the right stays in the middle of the
// first row, mid, as in the look. A grid or nested circles take the title
// at the top, and a row at the left, when the spec gives no side.
func (s Style) placeTitle(l *Legend, t *scene.Item, body []scene.Mark, mid float64, above bool) {
	b := s.marksBounds(body)
	if b.IsEmpty() {
		b = Bounds{}
	}
	orient := l.TitleOrient
	if orient == "" {
		orient = "left"
		if above {
			orient = "top"
		}
	}
	pad := float64(legendTitleGap)
	if orient == "left" || orient == "right" {
		pad = entryGap
	}
	if l.TitlePadding != nil {
		pad = *l.TitlePadding
	}
	switch orient {
	case "top", "bottom":
		t.X, t.Align = b.X1, "left"
		switch l.TitleAnchor {
		case "middle":
			t.X, t.Align = (b.X1+b.X2)/2, "center"
		case "end":
			t.X, t.Align = b.X2, "right"
		}
		t.Y, t.Baseline = b.Y1-pad, "bottom"
		if orient == "bottom" {
			t.Y, t.Baseline = b.Y2+pad, "top"
		}
	default:
		t.X, t.Align = b.X1-pad, "right"
		if orient == "right" {
			t.X, t.Align = b.X2+pad, "left"
		}
		t.Y, t.Baseline = mid, "middle"
		switch l.TitleAnchor {
		case "start":
			t.Y, t.Baseline = b.Y1, "top"
		case "middle":
			t.Y = (b.Y1 + b.Y2) / 2
		case "end":
			t.Y, t.Baseline = b.Y2, "bottom"
		}
	}
}

// frame puts a legend inside its padding, and draws the frame of the
// legend at its edge, in the colors that the spec gives, so that the
// legend starts at the top left of its frame, as in Vega.
func (s Style) frame(l *Legend, group *scene.Item) {
	if l.Padding <= 0 && l.FillColor == "" && l.StrokeColor == "" {
		return
	}
	b := s.marksBounds(group.Items)
	if b.IsEmpty() {
		return
	}
	p := math.Max(0, l.Padding)
	it := scene.Item{
		X: b.X1 - p, Y: b.Y1 - p, Width: b.X2 - b.X1 + 2*p, Height: b.Y2 - b.Y1 + 2*p,
		CornerRadius: l.CornerRadius, Fill: scene.Paint{Color: l.FillColor},
	}
	if l.StrokeColor != "" {
		it.Stroke, it.StrokeWidth = scene.Paint{Color: l.StrokeColor}, ptr(1)
	}
	group.Items = append([]scene.Mark{{Type: scene.Rect, Role: "legend-frame", Items: []scene.Item{it}}}, group.Items...)
	shiftMarks(group.Items, p-b.X1, p-b.Y1)
}

// marksBounds returns the bounds of marks.
func (s Style) marksBounds(ms []scene.Mark) Bounds {
	b := Empty()
	for i := range ms {
		b.union(s.MarkBounds(&ms[i]))
	}
	return b
}

// shiftMarks moves the items of marks by dx and dy. The far end of a rule
// moves too, and the items of a group stay in the frame of the group.
func shiftMarks(ms []scene.Mark, dx, dy float64) {
	for i := range ms {
		for j := range ms[i].Items {
			it := &ms[i].Items[j]
			it.X += dx
			it.Y += dy
			if ms[i].Type == scene.Rule {
				it.X2 += dx
				it.Y2 += dy
			}
		}
	}
}

// textWidth returns the width of a text item, as its limit cuts it.
func (s Style) textWidth(it *scene.Item) float64 {
	b := s.textBounds(it)
	return b.X2 - b.X1
}

// labelGap returns the space between a symbol and its label.
func (l *Legend) labelGap(def float64) float64 {
	if l.LabelOffset != nil {
		return *l.LabelOffset
	}
	return def
}

// thickness returns the width of the bar of a gradient.
func (l *Legend) thickness() float64 {
	if l.GradientThickness > 0 {
		return l.GradientThickness
	}
	return swatch
}

// finishLabels sets the align and the baseline of the labels that the spec
// gives.
func (l *Legend) finishLabels(items []scene.Item) {
	for i := range items {
		if l.LabelAlign != "" {
			items[i].Align = l.LabelAlign
		}
		if l.LabelBaseline != "" {
			items[i].Baseline = l.LabelBaseline
		}
	}
}

// main returns the scale that gives the values of the entries.
func (l *Legend) main() scale.Scale {
	for _, sc := range []scale.Scale{l.Size, l.Shape, l.Fill, l.Stroke, l.StrokeDash, l.StrokeWidth, l.Opacity} {
		if sc != nil {
			return sc
		}
	}
	return nil
}

// sizeOf returns the area of the circle of a value of a legend of sizes,
// and whether it has one.
func (l *Legend) sizeOf(v any) (float64, bool) {
	f, ok := l.Size.Map(v).(float64)
	return f, ok && f > 0 && !math.IsNaN(f) && !math.IsInf(f, 0)
}

// nested reports whether a legend shows only sizes, which it draws as
// nested circles, as docs/STYLE.md says: the spec sets no property that the
// circles cannot show, each value keeps its circle and its label, the
// largest circle is at least three labels high, and the tops of two
// circles stand far enough apart for their labels.
func (l *Legend) nested() bool {
	if l.Size == nil || l.Fill != nil || l.Stroke != nil || l.Shape != nil || l.StrokeDash != nil || l.StrokeWidth != nil ||
		l.Opacity != nil || l.Line {
		return false
	}
	if l.Entries || l.ClipHeight > 0 || l.Columns > 0 || l.SymbolSize > 0 || l.RowPadding != nil || l.ColumnPadding != nil ||
		(l.SymbolType != "" && l.SymbolType != "circle") {
		return false
	}
	vals, rest := l.entryValues()
	if rest > 0 || len(vals) == 0 {
		return false
	}
	rs := make([]float64, 0, len(vals))
	for _, v := range vals {
		f, ok := l.sizeOf(v)
		if !ok {
			return false
		}
		rs = append(rs, math.Sqrt(f)/2)
	}
	slices.SortFunc(rs, func(a, b float64) int { return cmp.Compare(b, a) })
	if 2*rs[0] < 3*LabelSize {
		return false
	}
	// The top of each circle is 2r below the top of the largest.
	for i := 1; i < len(rs); i++ {
		if 2*(rs[i-1]-rs[i]) < nestGap {
			return false
		}
	}
	return true
}

// nested draws a legend of sizes as circles that share their lowest point,
// the largest behind, each with a line from its top to its label.
func (s Style) nested(l *Legend, y float64) []scene.Mark {
	type entry struct {
		v     any
		r     float64
		label string
	}
	vals, _ := l.entryValues()
	es := make([]entry, 0, len(vals))
	for i, v := range vals {
		f, ok := l.sizeOf(v)
		if !ok {
			continue
		}
		es = append(es, entry{v: v, r: math.Sqrt(f) / 2, label: l.label(vals, i)})
	}
	if len(es) == 0 {
		return nil
	}
	slices.SortStableFunc(es, func(a, b entry) int { return cmp.Compare(b.r, a.r) })
	rmax := es[0].r
	bottom := y + 2*rmax
	circles := scene.Mark{Type: scene.Symbol, Role: "legend-symbol"}
	rules := scene.Mark{Type: scene.Rule, Role: "legend-rule"}
	labels := scene.Mark{Type: scene.Text, Role: "legend-label"}
	lx := 2*rmax + l.labelGap(swatchGap)
	for _, e := range es {
		top := bottom - 2*e.r
		circles.Items = append(circles.Items, scene.Item{
			X: rmax, Y: bottom - e.r, Shape: "circle", Size: ptr(4 * e.r * e.r),
			Stroke: scene.Paint{Color: s.Theme.Muted}, StrokeWidth: ptr(1), Datum: map[string]any{"value": e.v},
		})
		// ruleEnds puts an end at 0 first, so that the rule keeps it.
		rule := scene.Item{Stroke: scene.Paint{Color: s.Theme.Axis}, StrokeWidth: ptr(1)}
		ruleEnds(&rule, rmax, top, lx, top)
		rules.Items = append(rules.Items, rule)
		lab := s.label(lx+2, top, e.label, LabelSize, 400, s.Theme.Muted, "left", "middle")
		lab.Limit = l.labelLimit()
		lab.Datum = map[string]any{"value": e.v, "label": e.label}
		labels.Items = append(labels.Items, lab)
	}
	l.finishLabels(labels.Items)
	return []scene.Mark{circles, rules, labels}
}

// color returns the scale of color of the legend.
func (l *Legend) color() scale.Scale {
	if l.Fill != nil {
		return l.Fill
	}
	return l.Stroke
}

// labelLimit returns the widest that a label draws.
func (l *Legend) labelLimit() float64 {
	if l.LabelLimit > 0 {
		return l.LabelLimit
	}
	return labelLimit
}

// values returns the values of the entries: the values of the spec, the
// pieces of a scale that cuts numbers into pieces, the domain of a
// discrete scale or the ticks of a continuous one.
func (l *Legend) values() []any {
	if l.Values != nil {
		return l.Values
	}
	sc := l.main()
	if sc == nil {
		return nil
	}
	if b := scale.Bins(sc); b != nil {
		// A bin-ordinal scale has one entry for each bin, named by its
		// start.
		n := min(len(sc.Range()), len(b)-1)
		vals := make([]any, n)
		for i := range vals {
			vals[i] = b[i]
		}
		return vals
	}
	if t := scale.Thresholds(sc); t != nil {
		// A discretizing scale has one entry for each value of its range,
		// named by the threshold where it starts.
		// The thresholds cut the range into one entry more than they are.
		n := min(len(sc.Range()), len(t)+1)
		vals := make([]any, n)
		for i := range vals {
			if i == 0 {
				vals[i] = math.Inf(-1)
			} else {
				vals[i] = t[i-1]
			}
		}
		return vals
	}
	if scale.Discrete(sc.Type()) {
		return sc.Domain()
	}
	return sc.Ticks(l.tickCount(sc))
}

// tickCount returns the number of ticks that a continuous scale gives the
// entries: the tickCount of the spec, or 5. A tickMinStep of the spec
// takes fewer, so that two ticks stand at least that far apart, as an axis
// counts them.
func (l *Legend) tickCount(sc scale.Scale) float64 {
	count := 5.0
	if l.TickCount > 0 {
		count = l.TickCount
	}
	dom := sc.Domain()
	if l.TickMinStep > 0 && len(dom) >= 2 {
		span := math.Abs(expr.Number(dom[len(dom)-1]) - expr.Number(dom[0]))
		if !math.IsNaN(span) && !math.IsInf(span, 0) {
			count = math.Min(count, math.Max(1, math.Floor(span/l.TickMinStep))+1)
		}
	}
	return count
}

// entryValues returns the values that have an entry, and the count of the
// values past the limit, which the last entry counts. A first value whose
// size is 0 has no entry, as in Vega.
func (l *Legend) entryValues() ([]any, int) {
	vals := l.values()
	if l.Values == nil && l.Size != nil && len(vals) > 0 {
		if f, ok := l.Size.Map(vals[0]).(float64); ok && f == 0 {
			vals = vals[1:]
		}
	}
	limit := l.SymbolLimit
	if limit <= 0 {
		limit = symbolLimit
	}
	if len(vals) <= limit {
		return vals, 0
	}
	// The last entry shows the next value and counts the rest with it.
	return vals[:limit], len(vals) - limit + 1
}

// label returns the label of the value of an entry. A piece of a scale that
// cuts numbers into pieces is named by its ends, as docs/STYLE.md says.
func (l *Legend) label(vals []any, i int) string {
	f := l.Format
	if f == nil {
		f = func(any) string { return "" }
	}
	v := vals[i]
	if l.Values != nil || scale.Thresholds(l.main()) == nil {
		return f(v)
	}
	lo, ok := v.(float64)
	if !ok {
		return f(v)
	}
	if b := scale.Bins(l.main()); b != nil && len(vals) < len(b) {
		// The last bin ends at the end of the bins.
		hi := b[len(vals)]
		if i+1 < len(vals) {
			hi = expr.Number(vals[i+1])
		}
		return f(lo) + " – " + f(hi)
	}
	switch {
	case i+1 < len(vals) && math.IsInf(lo, -1):
		return "< " + f(vals[i+1])
	case i+1 < len(vals):
		return f(lo) + " – " + f(vals[i+1])
	case math.IsInf(lo, -1):
		return ""
	}
	return "≥ " + f(lo)
}

// entry is an entry of a legend before it is placed: its value, its
// label, its symbol at the origin and the side of the square that holds
// the symbol.
type entry struct {
	v     any
	label string
	sym   scene.Item
	// side is the side of the square of the symbol, at least a swatch,
	// whose middle the symbol takes. high is the height from the top of
	// that square to the bottom of the symbol, which is side, but for a
	// shape of a path.
	side, high float64
	width      float64
}

// entries draws the entries of a legend from (x, y): in a grid of columns,
// each as wide as its widest entry, or with no columns in a row that wraps
// at the width of the legend. It returns the middle of the first row too.
// A symbol wider than a swatch makes its entry wider, and the highest
// symbol sets the height of each row, unless the legend cuts its symbols
// to a height.
func (s Style) entries(l *Legend, x, y float64, columns int) (scene.Mark, float64) {
	entries := scene.Mark{Type: scene.Group, Role: "legend-entry"}
	row := scene.Item{}
	symbols := scene.Mark{Type: scene.Symbol, Role: "legend-symbol"}
	labels := scene.Mark{Type: scene.Text, Role: "legend-label"}
	clips := scene.Mark{Type: scene.Group, Role: "legend-clip"}
	vals, rest := l.entryValues()
	limit := l.labelLimit()
	gap := l.labelGap(swatchGap)
	es := make([]entry, len(vals))
	high := float64(swatch)
	for i, v := range vals {
		e := entry{v: v, label: l.label(vals, i), sym: s.swatch(l, v, i)}
		if rest > 0 && i == len(vals)-1 {
			e.label = "…" + strconv.Itoa(rest) + " entries"
		}
		e.side, e.high = swatch, swatch
		if e.sym.Size != nil {
			e.side = math.Max(swatch, math.Sqrt(*e.sym.Size))
			// The entry reaches down to the bottom of the symbol, which
			// for a shape of a path is the bottom of the path, as Vega
			// bounds an entry.
			b := s.itemBounds(scene.Symbol, &e.sym)
			e.high = math.Max(swatch, e.side/2+b.Y2)
		}
		e.width = math.Min(s.width(LabelSize, 400, e.label), limit)
		high = math.Max(high, e.high)
		es[i] = e
	}
	if l.ClipHeight > 0 {
		high = l.ClipHeight
	}
	gapX, gapY := float64(entryGap), float64(rowHeight-swatch)
	if l.ColumnPadding != nil {
		gapX = *l.ColumnPadding
	}
	if l.RowPadding != nil {
		gapY = *l.RowPadding
	}
	pitch := math.Max(rowHeight, high+gapY)
	if l.RowPadding != nil {
		pitch = high + gapY
	}
	// The left of each column of a grid, and the side of its symbols.
	var lefts, sides []float64
	if columns > 0 {
		lefts, sides = make([]float64, columns), make([]float64, columns)
		cw := make([]float64, columns)
		for i, e := range es {
			sides[i%columns] = math.Max(sides[i%columns], e.side)
		}
		for i, e := range es {
			cw[i%columns] = math.Max(cw[i%columns], sides[i%columns]+gap+e.width)
		}
		if l.GridAlign == "all" {
			// Each column is as wide as the widest.
			widest := slices.Max(cw)
			for c := range cw {
				cw[c] = widest
			}
		}
		at := x
		for c := range columns {
			lefts[c] = at
			at += cw[c] + gapX
		}
	}
	start := x
	for i, e := range es {
		side := e.side
		switch {
		case columns > 0 && l.GridAlign == "none":
			// Each entry follows the one before it in its row.
			if i > 0 && i%columns == 0 {
				x, y = start, y+pitch
			}
		case columns > 0:
			if i > 0 && i%columns == 0 {
				y += pitch
			}
			x, side = lefts[i%columns], sides[i%columns]
		case l.Width > 0 && x > start && x+side+gap+e.width > l.Width:
			x, y = start, y+pitch
		}
		sym := e.sym
		// The symbol is in the middle of its row, and one of the highest
		// stays in the middle of its square, at the top of the row.
		mid := high/2 - (e.high-e.side)/2
		sym.X, sym.Y = x+side/2+l.SymbolOffset, y+mid
		if l.ClipHeight > 0 && e.side > l.ClipHeight {
			// The symbol draws in a group that cuts it to the height of
			// its row, about its middle.
			mid = high / 2
			sym.X, sym.Y = e.side/2+l.SymbolOffset, high/2
			clips.Items = append(clips.Items, scene.Item{
				X: x + (side-e.side)/2, Y: y, Width: e.side, Height: high, Clip: true,
				Items: []scene.Mark{{Type: scene.Symbol, Role: "legend-symbol", Items: []scene.Item{sym}}},
			})
		} else {
			symbols.Items = append(symbols.Items, sym)
		}
		lab := s.label(x+side+gap, y+mid, e.label, LabelSize, 400, s.Theme.Muted, "left", "middle")
		lab.Limit = limit
		lab.Datum = map[string]any{"value": e.v, "label": e.label}
		labels.Items = append(labels.Items, lab)
		x += side + gap + e.width + gapX
	}
	l.finishLabels(labels.Items)
	row.Items = []scene.Mark{symbols, labels}
	if len(clips.Items) > 0 {
		row.Items = append(row.Items, clips)
	}
	entries.Items = []scene.Item{row}
	return entries, high / 2
}

// swatch draws the symbol of one entry at the origin: a rounded square of
// the color, a short line, a shape, or a circle of the size.
func (s Style) swatch(l *Legend, v any, i int) scene.Item {
	color := s.Theme.Category[i%len(s.Theme.Category)]
	if l.StrokeDash != nil || l.StrokeWidth != nil || l.Opacity != nil || (l.Size != nil && l.color() == nil) {
		// The dashes, the widths, the opacity or the sizes tell the
		// entries apart, in one color.
		color = s.Theme.Category[0]
		if l.Color != "" {
			color = l.Color
		}
	}
	if c := l.color(); c != nil {
		if str, ok := c.Map(v).(string); ok {
			color = str
		}
		if f, ok := v.(float64); ok && math.IsInf(f, -1) {
			if r := c.Range(); len(r) > 0 {
				color = expr.String(r[0])
			}
		}
	}
	area := float64(swatch * swatch)
	if l.SymbolSize > 0 {
		area = l.SymbolSize
	}
	it := scene.Item{Shape: "square", Size: ptr(area), Fill: scene.Paint{Color: color}}
	if l.SymbolType != "" {
		it.Shape = l.SymbolType
	}
	if l.Hollow {
		it.Fill = scene.Paint{}
		it.Stroke = scene.Paint{Color: color}
		it.StrokeWidth = ptr(2)
	}
	if l.Line || l.StrokeDash != nil || l.StrokeWidth != nil || l.LineValues[expr.String(v)] {
		it.Shape = "stroke"
		it.Fill = scene.Paint{}
		it.Stroke = scene.Paint{Color: color}
		it.StrokeWidth = ptr(2)
	}
	if l.StrokeWidth != nil {
		if w, ok := l.StrokeWidth.Map(v).(float64); ok && w > 0 {
			it.StrokeWidth = ptr(w)
		}
	}
	if l.Opacity != nil {
		if o, ok := l.Opacity.Map(v).(float64); ok {
			it.Opacity = ptr(o)
		}
	}
	if l.StrokeDash != nil {
		if d, ok := l.StrokeDash.Map(v).([]any); ok {
			for _, x := range d {
				it.StrokeDash = append(it.StrokeDash, expr.Number(x))
			}
		}
	}
	if l.Shape != nil {
		if sh, ok := l.Shape.Map(v).(string); ok {
			it.Shape = sh
			if l.SymbolSize <= 0 {
				it.Size = ptr(64)
			}
		}
	}
	if l.Size != nil {
		if l.Shape == nil && l.SymbolType == "" {
			it.Shape = "circle"
		}
		if f, ok := l.Size.Map(v).(float64); ok {
			it.Size = ptr(math.Max(4, f))
		}
		if l.Hollow {
			it.StrokeWidth = ptr(1.5)
		}
	}
	// The base colors of the spec paint a symbol where no scale of the
	// legend gives its color, as in Vega.
	if l.SymbolBaseFill != "" && l.Fill == nil && it.Shape != "stroke" {
		it.Fill = scene.Paint{Color: l.SymbolBaseFill}
	}
	if l.SymbolBaseStroke != "" && l.Stroke == nil {
		it.Stroke = scene.Paint{Color: l.SymbolBaseStroke}
		if it.StrokeWidth == nil {
			it.StrokeWidth = ptr(1.5)
		}
	}
	it.Datum = map[string]any{"value": v}
	return it
}

// gradient draws a continuous color scale as a bar from (x, y), with the
// labels of its ends below it, or upright with the labels at its right for
// a vertical legend, the high end at the top.
func (s Style) gradient(l *Legend, x, y float64) []scene.Mark {
	c := l.color()
	length := l.GradientLength
	if length <= 0 {
		length = 120
	}
	thick := l.thickness()
	if t := scale.Thresholds(c); len(t) > 0 {
		return s.steps(l, c, t, x, y, length, thick)
	}
	dom := c.Domain()
	lo, hi := expr.Number(dom[0]), expr.Number(dom[len(dom)-1])
	var stops []scene.Stop
	const n = 8
	for i := 0; i <= n; i++ {
		f := float64(i) / n
		v := lo + f*(hi-lo)
		if col, ok := c.Map(v).(string); ok {
			stops = append(stops, scene.Stop{Offset: f, Color: col})
		}
	}
	bar := scene.Item{
		X: x, Y: y, Width: length, Height: thick, CornerRadius: 2,
		Fill: scene.Paint{Gradient: &scene.Gradient{Type: "gradient", X1: 0, Y1: 0, X2: 1, Y2: 0, Stops: stops}},
	}
	labels := scene.Mark{Type: scene.Text, Role: "legend-label"}
	label := func(v any) string {
		if l.Format != nil {
			return l.Format(v)
		}
		return ""
	}
	// The look labels the ends of the bar. A tickCount or values of the
	// spec label each of those values along the bar, as Vega does.
	vals := []any{dom[0], dom[len(dom)-1]}
	if l.TickCount > 0 || l.Values != nil {
		vals = nil
		for _, v := range l.values() {
			if f := expr.Number(v); f >= math.Min(lo, hi) && f <= math.Max(lo, hi) {
				vals = append(vals, v)
			}
		}
	}
	at := func(v any) float64 {
		if hi == lo {
			return 0
		}
		return (expr.Number(v) - lo) / (hi - lo) * length
	}
	near := func(p, end float64) bool { return math.Abs(p-end) < 0.5 }
	if l.Direction == "vertical" {
		bar.Width, bar.Height = thick, length
		bar.Fill.Gradient.X2, bar.Fill.Gradient.Y1 = 0, 1
		gap := l.labelGap(swatchGap)
		for _, v := range vals {
			p := length - at(v)
			base := "middle"
			switch {
			case near(p, 0):
				base = "top"
			case near(p, length):
				base = "bottom"
			}
			labels.Items = append(labels.Items, s.label(x+thick+gap, y+p, label(v), LabelSize, 400, s.Theme.Muted, "left", base))
		}
	} else {
		gap := l.labelGap(4)
		for _, v := range vals {
			p := at(v)
			align := "center"
			switch {
			case near(p, 0):
				align = "left"
			case near(p, length):
				align = "right"
			}
			labels.Items = append(labels.Items, s.label(x+p, y+thick+gap, label(v), LabelSize, 400, s.Theme.Muted, align, "top"))
		}
	}
	if l.TickCount > 0 || l.Values != nil {
		for i, v := range vals {
			labels.Items[i].Datum = map[string]any{"value": v, "label": labels.Items[i].Text[0]}
		}
	}
	s.finishGradientLabels(l, &labels)
	return []scene.Mark{{Type: scene.Rect, Role: "legend-gradient", Items: []scene.Item{bar}}, labels}
}

// finishGradientLabels sets the align and the baseline that the spec gives
// the labels of a gradient, and drops those that overlap when the spec
// asks.
func (s Style) finishGradientLabels(l *Legend, labels *scene.Mark) {
	l.finishLabels(labels.Items)
	if l.LabelOverlap {
		labels.Items = s.dropOverlapsBy(labels.Items, l.LabelSeparation, l.LabelSeparation)
	}
}

// steps draws a scale that cuts numbers into pieces as a bar of even
// steps from (x, y), one for each color, with the label of each threshold
// where two steps meet, as Vega draws a discrete gradient. A vertical
// legend draws the bar upright, the high end at the top.
func (s Style) steps(l *Legend, c scale.Scale, t []float64, x, y, length, thick float64) []scene.Mark {
	rng := c.Range()
	n := min(len(rng), len(t)+1)
	if n == 0 {
		return nil
	}
	label := func(v any) string {
		if l.Format != nil {
			return l.Format(v)
		}
		return ""
	}
	step := length / float64(n)
	vertical := l.Direction == "vertical"
	gapX, gapY := l.labelGap(swatchGap), l.labelGap(4)
	bars := scene.Mark{Type: scene.Rect, Role: "legend-gradient"}
	labels := scene.Mark{Type: scene.Text, Role: "legend-label"}
	for i := range n {
		color := scene.Paint{Color: expr.String(rng[i])}
		it := scene.Item{X: x + float64(i)*step, Y: y, Width: step, Height: thick, Fill: color}
		if vertical {
			it = scene.Item{X: x, Y: y + length - float64(i+1)*step, Width: thick, Height: step, Fill: color}
		}
		it.Datum = map[string]any{"value": i}
		bars.Items = append(bars.Items, it)
		if i == 0 {
			continue
		}
		v := t[i-1]
		lab := s.label(x+float64(i)*step, y+thick+gapY, label(v), LabelSize, 400, s.Theme.Muted, "center", "top")
		if vertical {
			lab = s.label(x+thick+gapX, y+length-float64(i)*step, label(v), LabelSize, 400, s.Theme.Muted, "left", "middle")
		}
		lab.Datum = map[string]any{"value": v, "label": lab.Text[0]}
		labels.Items = append(labels.Items, lab)
	}
	s.finishGradientLabels(l, &labels)
	return []scene.Mark{bars, labels}
}

// Title is a title, with the settings that the view resolved from the spec.
type Title struct {
	Text     []string
	Subtitle []string
	// Orient is top, bottom, left or right.
	Orient string
	// Anchor is start, middle or end.
	Anchor string
	// Width and Height are the size of the plot area that the title names.
	Width, Height float64
	// Offset is the space between the title and what it names.
	Offset float64
	// Kind is title for the title of a chart, label for a header of a
	// facet, and small for the title of a guide in a facet.
	Kind string
	// Angle, when it is not nil, turns the text by that many degrees in
	// place of the turn of the side. Align and Baseline, when they are not
	// empty, place the text in place of the anchor and the side.
	Angle           *float64
	Align, Baseline string
}

// Title draws a title as a group of the role title, with the title and the
// subtitle. The title of a chart is at the top left, above the plot area.
func (s Style) Title(t *Title) scene.Mark {
	size, weight, color := float64(TitleSize), 600, s.Theme.Text
	switch t.Kind {
	case "label":
		size, weight, color = LabelSize, 400, s.Theme.Muted
	case "small":
		size, weight, color = SmallSize, 400, s.Theme.Faint
	}
	group := scene.Item{}
	lines := scene.Mark{Type: scene.Text, Role: "title-text"}
	anchor := t.Anchor
	align := map[string]string{"start": "left", "middle": "center", "end": "right"}[anchor]
	switch t.Orient {
	case "left", "right":
		y := t.Height / 2
		switch anchor {
		case "start":
			y, align = 0, "right"
		case "end":
			y, align = t.Height, "left"
		default:
			align = "center"
		}
		x, angle := -t.Offset, -90.0
		if t.Orient == "right" {
			x, angle = t.Width+t.Offset, 90
		}
		for i, line := range t.Text {
			it := s.label(x-float64(len(t.Text)-1-i)*size*1.2, y, line, size, weight, color, align, "bottom")
			it.Angle = angle
			lines.Items = append(lines.Items, it)
		}
	default:
		x := 0.0
		switch anchor {
		case "middle":
			x = t.Width / 2
		case "end":
			x = t.Width
		default:
			align = "left"
		}
		subH := 0.0
		if len(t.Subtitle) > 0 {
			subH = float64(len(t.Subtitle))*SubtitleSize*1.3 + 4
		}
		if t.Orient == "bottom" {
			y := t.Height + t.Offset
			for i, line := range t.Text {
				lines.Items = append(lines.Items, s.label(x, y+float64(i)*size*1.2, line, size, weight, color, align, "top"))
			}
			break
		}
		y := -t.Offset - subH - float64(len(t.Text)-1)*size*1.2
		for i, line := range t.Text {
			lines.Items = append(lines.Items, s.label(x, y+float64(i)*size*1.2, line, size, weight, color, align, "bottom"))
		}
		if len(t.Subtitle) > 0 {
			sub := scene.Mark{Type: scene.Text, Role: "title-subtitle"}
			sy := -t.Offset - subH + SubtitleSize*1.3
			for i, line := range t.Subtitle {
				sub.Items = append(sub.Items, s.label(x, sy+float64(i)*SubtitleSize*1.3, line, SubtitleSize, 400, s.Theme.Muted, align, "bottom"))
			}
			group.Items = append(group.Items, sub)
		}
	}
	for i := range lines.Items {
		it := &lines.Items[i]
		if t.Angle != nil {
			it.Angle = *t.Angle
		}
		if t.Align != "" {
			it.Align = t.Align
		}
		if t.Baseline != "" {
			it.Baseline = t.Baseline
		}
	}
	if (t.Orient == "left" || t.Orient == "right") && (t.Angle != nil || t.Align != "" || t.Baseline != "") {
		// A title at a side that the spec turns or aligns stays beside
		// what it names: its text ends at the offset, as Vega places it.
		b := Empty()
		for i := range lines.Items {
			b.union(s.textBounds(&lines.Items[i]))
		}
		if !b.IsEmpty() {
			dx := -t.Offset - b.X2
			if t.Orient == "right" {
				dx = t.Width + t.Offset - b.X1
			}
			for i := range lines.Items {
				lines.Items[i].X += dx
			}
		}
	}
	group.Items = append([]scene.Mark{lines}, group.Items...)
	return scene.Mark{Type: scene.Group, Role: "title", Items: []scene.Item{group}}
}
