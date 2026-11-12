package layout

import (
	"cmp"
	"math"
	"slices"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/scene"
)

// Legend is a legend, with the settings that the view resolved from the
// spec. It draws the first of its scales that it has: the fill, the
// stroke, the shape, the size or the dash.
type Legend struct {
	Fill, Stroke, Shape, Size scale.Scale
	// StrokeDash is the scale of the dashes, whose entries are short lines.
	StrokeDash scale.Scale
	Title      string
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
	// GradientLength is the length of the bar.
	GradientLength float64
	// Direction is horizontal or vertical. A vertical legend draws its
	// entries in one column, and its gradient upright.
	Direction string
	// Columns is the number of columns of the entries. 0 draws them in one
	// row that wraps at Width, or in one column for a vertical legend.
	Columns int
}

// The sizes of a legend.
const (
	swatch    = 10
	swatchGap = 6
	entryGap  = 16
	rowHeight = 18
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
	if l.Title != "" {
		// The title is at the left of a row, as docs/STYLE.md draws a
		// legend, and above a column or a grid.
		title := s.label(0, swatch/2, l.Title, SmallSize, 400, s.Theme.Faint, "left", "middle")
		group.Items = append(group.Items, scene.Mark{Type: scene.Text, Role: "legend-title", Items: []scene.Item{title}})
		if columns > 0 || l.nested() {
			y = rowHeight
		} else {
			x = s.width(SmallSize, 400, l.Title) + entryGap
		}
	}
	switch {
	case l.Gradient && l.color() != nil && !scale.Discrete(l.color().Type()):
		group.Items = append(group.Items, s.gradient(l, x, y)...)
	case l.nested():
		group.Items = append(group.Items, s.nested(l, y)...)
	default:
		group.Items = append(group.Items, s.entries(l, x, y, columns))
	}
	return scene.Mark{Type: scene.Group, Role: "legend", Items: []scene.Item{group}}
}

// nested reports whether a legend shows only sizes, with a largest circle
// at least as high as three labels, which it draws as nested circles. A
// legend of smaller circles draws them in a row, so that their labels have
// room.
func (l *Legend) nested() bool {
	if l.Size == nil || l.Fill != nil || l.Stroke != nil || l.Shape != nil || l.StrokeDash != nil || l.Line {
		return false
	}
	rmax := 0.0
	for _, v := range l.values() {
		if f, ok := l.Size.Map(v).(float64); ok && f > 0 {
			rmax = math.Max(rmax, math.Sqrt(f)/2)
		}
	}
	return 2*rmax >= 3*LabelSize
}

// nested draws a legend of sizes as circles that share their lowest point,
// the largest behind, each with a line from its top to its label.
func (s Style) nested(l *Legend, y float64) []scene.Mark {
	type entry struct {
		v     any
		r     float64
		label string
	}
	var es []entry
	for _, v := range l.values() {
		f, ok := l.Size.Map(v).(float64)
		if !ok || f <= 0 || math.IsNaN(f) {
			continue
		}
		label := ""
		if l.Format != nil {
			label = l.Format(v)
		}
		es = append(es, entry{v: v, r: math.Sqrt(f) / 2, label: label})
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
	lx := 2*rmax + swatchGap
	lastY := math.Inf(-1)
	for _, e := range es {
		top := bottom - 2*e.r
		circles.Items = append(circles.Items, scene.Item{
			X: rmax, Y: bottom - e.r, Shape: "circle", Size: ptr(4 * e.r * e.r),
			Stroke: scene.Paint{Color: s.Theme.Muted}, StrokeWidth: ptr(1), Datum: map[string]any{"value": e.v},
		})
		// A label that overlaps the one above it is left out, as the
		// labels of an axis are.
		if top-lastY < LabelSize {
			continue
		}
		lastY = top
		rules.Items = append(rules.Items, scene.Item{X: rmax, Y: top, X2: lx, Y2: top, Stroke: scene.Paint{Color: s.Theme.Axis}, StrokeWidth: ptr(1)})
		lab := s.label(lx+2, top, e.label, LabelSize, 400, s.Theme.Muted, "left", "middle")
		lab.Datum = map[string]any{"value": e.v, "label": e.label}
		labels.Items = append(labels.Items, lab)
	}
	return []scene.Mark{circles, rules, labels}
}

// color returns the scale of color of the legend.
func (l *Legend) color() scale.Scale {
	if l.Fill != nil {
		return l.Fill
	}
	return l.Stroke
}

// values returns the values of the entries.
func (l *Legend) values() []any {
	if l.Values != nil {
		return l.Values
	}
	for _, sc := range []scale.Scale{l.Fill, l.Stroke, l.Shape, l.Size, l.StrokeDash} {
		if sc == nil {
			continue
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
		return sc.Ticks(5)
	}
	return nil
}

// entries draws the entries of a legend from (x, y): in a grid of columns,
// each as wide as its widest entry, or with no columns in a row that wraps
// at the width of the legend.
func (s Style) entries(l *Legend, x, y float64, columns int) scene.Mark {
	entries := scene.Mark{Type: scene.Group, Role: "legend-entry"}
	row := scene.Item{}
	symbols := scene.Mark{Type: scene.Symbol, Role: "legend-symbol"}
	labels := scene.Mark{Type: scene.Text, Role: "legend-label"}
	vals := l.values()
	names := make([]string, len(vals))
	widths := make([]float64, len(vals))
	for i, v := range vals {
		label := ""
		if l.Format != nil {
			label = l.Format(v)
		}
		if f, ok := v.(float64); ok && math.IsInf(f, -1) && l.color() != nil {
			if t := scale.Thresholds(l.color()); len(t) > 0 {
				label = "< " + l.Format(t[0])
			}
		}
		names[i] = label
		widths[i] = swatch + swatchGap + s.width(LabelSize, 400, label)
	}
	// The left of each column of a grid.
	var lefts []float64
	if columns > 0 {
		lefts = make([]float64, columns)
		cw := make([]float64, columns)
		for i, w := range widths {
			cw[i%columns] = math.Max(cw[i%columns], w)
		}
		at := x
		for c := range columns {
			lefts[c] = at
			at += cw[c] + entryGap
		}
	}
	start := x
	for i, v := range vals {
		w := widths[i]
		switch {
		case columns > 0:
			if i > 0 && i%columns == 0 {
				y += rowHeight
			}
			x = lefts[i%columns]
		case l.Width > 0 && x > start && x+w > l.Width:
			x, y = start, y+rowHeight
		}
		sym := s.swatch(l, v, i, x, y)
		symbols.Items = append(symbols.Items, sym)
		lab := s.label(x+swatch+swatchGap, y+swatch/2, names[i], LabelSize, 400, s.Theme.Muted, "left", "middle")
		lab.Datum = map[string]any{"value": v, "label": names[i]}
		labels.Items = append(labels.Items, lab)
		x += w + entryGap
	}
	row.Items = []scene.Mark{symbols, labels}
	entries.Items = []scene.Item{row}
	return entries
}

// swatch draws the symbol of one entry: a rounded square of the color, a
// short line, a shape, or a circle of the size.
func (s Style) swatch(l *Legend, v any, i int, x, y float64) scene.Item {
	color := s.Theme.Category[i%len(s.Theme.Category)]
	if l.StrokeDash != nil {
		// The dashes tell the entries apart, in the color of a line.
		color = s.Theme.Category[0]
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
	it := scene.Item{X: x + swatch/2, Y: y + swatch/2, Shape: "square", Size: new(float64(swatch * swatch)), Fill: scene.Paint{Color: color}}
	if l.Fill == nil && l.Stroke != nil {
		it.Fill = scene.Paint{}
		it.Stroke = scene.Paint{Color: color}
		it.StrokeWidth = ptr(2)
	}
	if l.Line || l.StrokeDash != nil {
		it.Shape = "stroke"
		it.Fill = scene.Paint{}
		it.Stroke = scene.Paint{Color: color}
		it.StrokeWidth = ptr(2)
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
			it.Size = ptr(64)
		}
	}
	if l.Size != nil && l.Fill == nil && l.Stroke == nil && l.Shape == nil {
		it.Shape = "circle"
		it.Fill = scene.Paint{Color: s.Theme.Category[0]}
		if f, ok := l.Size.Map(v).(float64); ok {
			it.Size = ptr(math.Max(4, f))
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
		X: x, Y: y, Width: length, Height: swatch, CornerRadius: 2,
		Fill: scene.Paint{Gradient: &scene.Gradient{Type: "gradient", X1: 0, Y1: 0, X2: 1, Y2: 0, Stops: stops}},
	}
	labels := scene.Mark{Type: scene.Text, Role: "legend-label"}
	label := func(v any) string {
		if l.Format != nil {
			return l.Format(v)
		}
		return ""
	}
	if l.Direction == "vertical" {
		bar.Width, bar.Height = swatch, length
		bar.Fill.Gradient.X2, bar.Fill.Gradient.Y1 = 0, 1
		labels.Items = append(labels.Items,
			s.label(x+swatch+swatchGap, y, label(dom[len(dom)-1]), LabelSize, 400, s.Theme.Muted, "left", "top"),
			s.label(x+swatch+swatchGap, y+length, label(dom[0]), LabelSize, 400, s.Theme.Muted, "left", "bottom"))
		return []scene.Mark{{Type: scene.Rect, Role: "legend-gradient", Items: []scene.Item{bar}}, labels}
	}
	for _, e := range []struct {
		v     any
		x     float64
		align string
	}{{dom[0], x, "left"}, {dom[len(dom)-1], x + length, "right"}} {
		labels.Items = append(labels.Items, s.label(e.x, y+swatch+4, label(e.v), LabelSize, 400, s.Theme.Muted, e.align, "top"))
	}
	return []scene.Mark{{Type: scene.Rect, Role: "legend-gradient", Items: []scene.Item{bar}}, labels}
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
	group.Items = append([]scene.Mark{lines}, group.Items...)
	return scene.Mark{Type: scene.Group, Role: "title", Items: []scene.Item{group}}
}
