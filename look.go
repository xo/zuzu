package zuzu

import (
	"cmp"
	"encoding/json/v2"
	"fmt"
	"math"
	"slices"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/format"
	"github.com/xo/zuzu/internal/color"
	"github.com/xo/zuzu/internal/shape"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// look gives a mark the rules of docs/STYLE.md that the config of the theme
// names and that no property of Vega draws, after its items are built, and
// returns the marks that it adds after the mark. A spec turns a rule off
// with its config (D14).
func (s *scope) look(m *vega.Mark, sm *scene.Mark) []scene.Mark {
	if len(sm.Items) == 0 {
		return nil
	}
	d := s.rt.markDefaults(m)
	var out []scene.Mark
	if sm.Type == scene.Area {
		if g, ok := d[theme.Gradient].([]any); ok && len(g) == 2 {
			areaGradient(sm, expr.Number(g[0]), expr.Number(g[1]))
		}
		if w := expr.Number(d[theme.TopLine]); w > 0 {
			out = append(out, topLine(sm, w))
		}
		if expr.Truthy(d[theme.MarkHigh]) {
			out = append(out, s.markHigh(m, sm)...)
		}
	}
	if expr.Truthy(d[theme.Hierarchy]) {
		out = append(out, s.hierarchyLook(m, sm)...)
	}
	if sm.Type == scene.Arc && expr.Truthy(d[theme.Total]) {
		out = append(out, s.donutTotal(m, sm)...)
	}
	if expr.Truthy(d[theme.Sankey]) {
		sankeyLook(m, sm)
	}
	if !encodes(m, "strokeCap") {
		dashCaps(sm)
	}
	if sm.Type == scene.Line && expr.Truthy(d[theme.EndLabel]) {
		out = append(out, s.endLabel(m, sm)...)
	}
	// A rule of one item is a reference line, and a mark of many rules
	// draws data.
	if sm.Type == scene.Rule && len(sm.Items) == 1 && expr.Truthy(d[theme.Reference]) {
		out = append(out, s.reference(m, sm)...)
	}
	return out
}

// dashCaps gives a dashed item a flat cap in place of the round or the
// square cap of a style, when that cap closes a gap of its pattern. Such a
// cap reaches half the width of the stroke past each end of each dash, so
// a gap no longer than the width disappears, as with the default dashes of
// Vega-Lite on a line of 2. A flat cap draws the dashes as the spec gives
// them, as the dashes of a reference line are drawn.
func dashCaps(sm *scene.Mark) {
	for i := range sm.Items {
		it := &sm.Items[i]
		if it.StrokeCap != "round" && it.StrokeCap != "square" {
			continue
		}
		w := 1.0
		if it.StrokeWidth != nil {
			w = *it.StrokeWidth
		}
		dash := it.StrokeDash
		if len(dash)%2 == 1 {
			// An odd pattern repeats, so each of its values is a gap too.
			dash = append(slices.Clone(dash), dash...)
		}
		for j := 1; j < len(dash); j += 2 {
			if dash[j] > 0 && dash[j] <= w {
				it.StrokeCap = "butt"
				break
			}
		}
	}
}

// encodedField returns the field and the scale of a property that the
// encoding of a mark reads from its datum through a scale, or empty
// strings.
func (s *scope) encodedField(m *vega.Mark, prop string) (string, string) {
	for _, set := range encodeSets {
		for _, r := range m.Encode[set][prop] {
			if r.Field.Name == "" || r.Scale.IsZero() {
				continue
			}
			name, err := s.str(r.Scale, "")
			if err != nil || name == "" {
				continue
			}
			return r.Field.Name, name
		}
	}
	return "", ""
}

// endLabel returns a dot at the last point of a line and the name of its
// series after it, in the color of the line (docs/STYLE.md). The name is
// the value of the field that the stroke of the line reads through its
// scale, so a line of one color gets none.
func (s *scope) endLabel(m *vega.Mark, sm *scene.Mark) []scene.Mark {
	field, scaleName := s.encodedField(m, "stroke")
	if field == "" {
		return nil
	}
	var last *scene.Item
	for i := range sm.Items {
		it := &sm.Items[i]
		if it.Defined == nil || *it.Defined {
			last = it
		}
	}
	if last == nil || last.Stroke.Color == "" {
		return nil
	}
	name := expr.String(readField(last.Datum, field))
	if s.rt.endLabels[scaleName] == nil {
		s.rt.endLabels[scaleName] = map[string]bool{}
	}
	s.rt.endLabels[scaleName][name] = true
	c := last.Stroke.Color
	dot := scene.Item{X: last.X, Y: last.Y, Shape: "circle", Size: new(36.0), Fill: scene.Paint{Color: c}}
	label := scene.Item{
		X: last.X + 7, Y: last.Y, Text: scene.Lines{name}, FontSize: 11, FontWeight: "600",
		Align: "left", Baseline: "middle", Fill: scene.Paint{Color: c},
		Stroke: scene.Paint{Color: s.rt.theme.Background}, StrokeWidth: new(3.0),
		Datum: map[string]any{"label": name},
	}
	return []scene.Mark{
		{Type: scene.Symbol, Role: "zuzu-end-dot", Items: []scene.Item{dot}},
		{Type: scene.Text, Role: "zuzu-end-label", Items: []scene.Item{label}},
	}
}

// reference dashes a reference line, a rule across the whole plot, and
// names its value where it starts (docs/STYLE.md).
func (s *scope) reference(m *vega.Mark, sm *scene.Mark) []scene.Mark {
	labels := scene.Mark{Type: scene.Text, Role: "zuzu-reference-label"}
	for i := range sm.Items {
		it := &sm.Items[i]
		if len(it.StrokeDash) == 0 && !encodes(m, "strokeDash") {
			it.StrokeDash = []float64{4, 3}
		}
		x2, y2 := shape.RuleEnd(it)
		var v any
		for _, prop := range []string{"y", "x"} {
			if f, _ := s.encodedField(m, prop); f != "" {
				v = readField(it.Datum, f)
				break
			}
			if c, ok := encodedValue(m, prop); ok {
				v = c
				break
			}
		}
		f, ok := v.(float64)
		if !ok {
			continue
		}
		text := s.numberText(f)
		lab := scene.Item{
			Text: scene.Lines{text}, FontSize: 10, Fill: scene.Paint{Color: s.rt.theme.Muted},
			Stroke: scene.Paint{Color: s.rt.theme.Background}, StrokeWidth: new(3.0),
			Datum: map[string]any{"value": v},
		}
		if it.Y == y2 {
			// A rule across the plot is named above its left end.
			lab.X, lab.Y, lab.Align, lab.Baseline = math.Min(it.X, x2)+2, it.Y-3, "left", "bottom"
		} else {
			// A rule up the plot is named at the right of its top.
			lab.X, lab.Y, lab.Align, lab.Baseline = it.X+3, math.Min(it.Y, y2)+2, "left", "top"
		}
		labels.Items = append(labels.Items, lab)
	}
	if len(labels.Items) == 0 {
		return nil
	}
	return []scene.Mark{labels}
}

// numberText writes a number with a thousands separator, with no decimals
// from 100 up and at most two below.
func (s *scope) numberText(f float64) string {
	spec := ",.2~f"
	if math.Abs(f) >= 100 {
		spec = ",.0f"
	}
	fn, err := format.Number(spec, *s.rt.locale)
	if err != nil {
		return expr.String(f)
	}
	return fn(f)
}

// markHigh marks the highest point of an area with a ring, the word High
// and its value, and the date below when x is a time, as the area target
// does (docs/STYLE.md).
func (s *scope) markHigh(m *vega.Mark, sm *scene.Mark) []scene.Mark {
	yField, _ := s.encodedField(m, "y")
	xField, xScale := s.encodedField(m, "x")
	if yField == "" || sm.Items[0].Orient == "horizontal" {
		return nil
	}
	var top *scene.Item
	for i := range sm.Items {
		it := &sm.Items[i]
		if (it.Defined == nil || *it.Defined) && (top == nil || it.Y < top.Y) {
			top = it
		}
	}
	if top == nil {
		return nil
	}
	v, ok := readField(top.Datum, yField).(float64)
	if !ok {
		return nil
	}
	c := sm.Items[0].Fill.Color
	if g := sm.Items[0].Fill.Gradient; g != nil && len(g.Stops) > 0 {
		c = g.Stops[0].Color
		if rgba, ok := color.Parse(c); ok {
			c = fmt.Sprintf("rgb(%d,%d,%d)", int(math.Round(rgba.R)), int(math.Round(rgba.G)), int(math.Round(rgba.B)))
		}
	}
	ring := scene.Item{
		X: top.X, Y: top.Y, Shape: "circle", Size: new(64.0), Fill: scene.Paint{Color: s.rt.theme.Background},
		Stroke: scene.Paint{Color: c}, StrokeWidth: new(2.0),
	}
	halo := scene.Paint{Color: s.rt.theme.Background}
	lines := []scene.Item{{
		X: top.X - 10, Y: top.Y - 4, Text: scene.Lines{"High " + s.numberText(v)}, FontSize: 11, FontWeight: "600",
		Align: "right", Baseline: "alphabetic", Fill: scene.Paint{Color: s.rt.theme.Text}, Stroke: halo, StrokeWidth: new(3.0),
	}}
	if xField != "" {
		if sc, err := s.scale(xScale); err == nil && (sc.Type() == "time" || sc.Type() == "utc") {
			if t, ok := toTime(readField(top.Datum, xField), s.rt.loc); ok {
				lines = append(lines, scene.Item{
					X: top.X - 10, Y: top.Y + 10, Text: scene.Lines{format.Time("%b %Y", *s.rt.locale)(t)}, FontSize: 10,
					Align: "right", Baseline: "alphabetic", Fill: scene.Paint{Color: s.rt.theme.Muted}, Stroke: halo, StrokeWidth: new(3.0),
				})
			}
		}
	}
	return []scene.Mark{
		{Type: scene.Symbol, Role: "zuzu-high-dot", Items: []scene.Item{ring}},
		{Type: scene.Text, Role: "zuzu-high-label", Items: lines},
	}
}

// dodgeLabels moves the names at the ends of lines apart, so that no two
// overlap, in the marks of a scope and in the groups among them.
func dodgeLabels(marks []scene.Mark) {
	type label struct {
		it  *scene.Item
		off float64
	}
	var labels []label
	add := func(m *scene.Mark, off float64) {
		if m.Role != "zuzu-end-label" {
			return
		}
		for i := range m.Items {
			labels = append(labels, label{&m.Items[i], off})
		}
	}
	for i := range marks {
		add(&marks[i], 0)
		if marks[i].Type != scene.Group {
			continue
		}
		for j := range marks[i].Items {
			g := &marks[i].Items[j]
			for k := range g.Items {
				add(&g.Items[k], g.Y)
			}
		}
	}
	if len(labels) < 2 {
		return
	}
	slices.SortStableFunc(labels, func(a, b label) int { return cmp.Compare(a.it.Y+a.off, b.it.Y+b.off) })
	const gap = 12
	for i := 1; i < len(labels); i++ {
		prev, cur := labels[i-1].it.Y+labels[i-1].off, labels[i].it.Y+labels[i].off
		if cur-prev < gap {
			labels[i].it.Y = prev + gap - labels[i].off
		}
	}
}

// withAlpha returns a color of CSS with its opacity times a.
func withAlpha(c string, a float64) (string, bool) {
	rgba, ok := color.Parse(c)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%s)", int(math.Round(rgba.R)), int(math.Round(rgba.G)), int(math.Round(rgba.B)), expr.String(math.Round(rgba.A*a*1000)/1000)), true
}

// areaGradient fills an area with its color from an opacity at its line to
// an opacity at its base (docs/STYLE.md). An area whose fill is not one
// color keeps it.
func areaGradient(sm *scene.Mark, top, base float64) {
	first := &sm.Items[0]
	if first.Fill.Gradient != nil || first.Fill.Color == "" {
		return
	}
	c0, ok0 := withAlpha(first.Fill.Color, top)
	c1, ok1 := withAlpha(first.Fill.Color, base)
	if !ok0 || !ok1 {
		return
	}
	g := &scene.Gradient{Type: scene.Linear, X2: 0, Y2: 1, Stops: []scene.Stop{{Offset: 0, Color: c0}, {Offset: 1, Color: c1}}}
	if first.Orient == "horizontal" {
		// The line of a horizontal area is at its right.
		g.X1, g.X2, g.Y2 = 1, 0, 0
	}
	for i := range sm.Items {
		sm.Items[i].Fill = scene.Paint{Gradient: g}
	}
}

// topLine returns the line along the top of an area, in the color of the
// area, of a width.
func topLine(sm *scene.Mark, width float64) scene.Mark {
	first := &sm.Items[0]
	c := first.Fill.Color
	if first.Fill.Gradient != nil && len(first.Fill.Gradient.Stops) > 0 {
		if rgba, ok := color.Parse(first.Fill.Gradient.Stops[0].Color); ok {
			c = fmt.Sprintf("rgb(%d,%d,%d)", int(math.Round(rgba.R)), int(math.Round(rgba.G)), int(math.Round(rgba.B)))
		}
	}
	line := scene.Mark{Type: scene.Line, Role: "zuzu-area-line", Items: make([]scene.Item, len(sm.Items))}
	for i, it := range sm.Items {
		line.Items[i] = scene.Item{
			X: it.X, Y: it.Y, Defined: it.Defined, Interpolate: it.Interpolate, Tension: it.Tension,
			Stroke: scene.Paint{Color: c}, StrokeWidth: &width, StrokeCap: "round", StrokeJoin: "round",
			Opacity: it.Opacity,
		}
	}
	return line
}

// boxLook draws the parts of a box plot of Vega-Lite as docs/STYLE.md does:
// the box in its color at the fill opacity of the theme, with an outline of
// 1.5 and round corners of 5, and the whiskers, their ticks and the median
// in the color of their box, the median 2.5 thick. A part takes the color
// of the box that holds it.
func boxLook(marks []scene.Mark, styles [][]string, fillOpacity float64) {
	type box struct {
		x1, y1, x2, y2 float64
		color          string
	}
	var boxes []box
	for i := range marks {
		if !slices.Contains(styles[i], "boxplot-box") || marks[i].Type != scene.Rect {
			continue
		}
		for j := range marks[i].Items {
			it := &marks[i].Items[j]
			c := it.Fill.Color
			if c == "" || it.Fill.Gradient != nil {
				continue
			}
			it.FillOpacity = &fillOpacity
			it.Stroke = scene.Paint{Color: c}
			it.StrokeWidth = new(1.5)
			it.CornerRadius = 5
			x, y, w, h := shape.Box(it)
			boxes = append(boxes, box{x, y, x + w, y + h, c})
		}
	}
	if len(boxes) == 0 {
		return
	}
	// colorAt returns the color of the box that holds a point across the
	// axis of the plot, in x or in y.
	colorAt := func(x, y float64, vertical bool) string {
		for _, b := range boxes {
			if vertical && x >= b.x1-0.5 && x <= b.x2+0.5 || !vertical && y >= b.y1-0.5 && y <= b.y2+0.5 {
				return b.color
			}
		}
		return ""
	}
	for i := range marks {
		st := styles[i]
		for j := range marks[i].Items {
			it := &marks[i].Items[j]
			switch {
			case slices.Contains(st, "boxplot-rule") && marks[i].Type == scene.Rule:
				x2, y2 := shape.RuleEnd(it)
				if c := colorAt((it.X+x2)/2, (it.Y+y2)/2, it.X == x2); c != "" {
					it.Stroke = scene.Paint{Color: c}
					it.StrokeWidth = new(1.5)
				}
			case (slices.Contains(st, "boxplot-median") || slices.Contains(st, "boxplot-ticks")) && marks[i].Type == scene.Rect:
				// A median or a tick across an upright box is wider than it
				// is high, and its box holds its x.
				x, y, w, h := shape.Box(it)
				c := colorAt(x+w/2, y+h/2, w >= h)
				if c == "" {
					continue
				}
				it.Fill = scene.Paint{Color: c}
				it.Opacity, it.FillOpacity = nil, nil
				if slices.Contains(st, "boxplot-median") {
					// The median is a line of 2.5 across the box.
					if w >= h {
						it.Y, it.Height, it.Y2 = y+h/2-1.25, 2.5, 0
					} else {
						it.X, it.Width, it.X2 = x+w/2-1.25, 2.5, 0
					}
				}
			}
		}
	}
}

// encodedValue returns the constant that the encoding of a mark gives a
// property through a scale, such as the datum of a rule of Vega-Lite.
func encodedValue(m *vega.Mark, prop string) (any, bool) {
	for _, set := range encodeSets {
		for _, r := range m.Encode[set][prop] {
			if len(r.Value) == 0 || r.Scale.IsZero() {
				continue
			}
			var v any
			if json.Unmarshal(r.Value, &v) == nil {
				return v, true
			}
		}
	}
	return nil, false
}

// node returns the depth of the node of a hierarchy that an item draws,
// whether the node has children, and whether the datum is a node, with the
// fields depth and children that the transforms of a hierarchy write.
func node(it *scene.Item) (int, bool, bool) {
	d, ok := it.Datum.(map[string]any)
	if !ok {
		return 0, false, false
	}
	depth, ok1 := d["depth"].(float64)
	children, ok2 := d["children"].(float64)
	if !ok1 || !ok2 {
		return 0, false, false
	}
	return int(depth), children > 0, true
}

// nodeName returns the name of a node: its field name, or else its id.
func nodeName(it *scene.Item) string {
	d, _ := it.Datum.(map[string]any)
	for _, k := range []string{"name", "id"} {
		if v, ok := d[k]; ok && v != nil {
			return expr.String(v)
		}
	}
	return ""
}

// hierarchyLook draws the nodes of a hierarchy as docs/STYLE.md does, for
// the properties that the encoding does not set. A rect of a group is a
// tint of its color with round corners of 6 and its name at the top left,
// and a rect of a leaf is its color with round corners of 3 and its name
// inside, where it fits. A circle of a group is a tint with an outline, the
// deeper the stronger, and the root is the color of a panel. An arc fades
// with its depth. It returns the marks of the names.
func (s *scope) hierarchyLook(m *vega.Mark, sm *scene.Mark) []scene.Mark {
	if _, _, ok := node(&sm.Items[0]); !ok {
		return nil
	}
	setOpacity := !encodes(m, "fillOpacity") && !encodes(m, "opacity")
	setCorner := !encodesCorners(m)
	names := scene.Mark{Type: scene.Text, Role: "zuzu-hierarchy-label"}
	st := s.rt.style()
	halo := scene.Paint{Color: s.rt.theme.Background}
	for i := range sm.Items {
		it := &sm.Items[i]
		depth, group, _ := node(it)
		switch sm.Type {
		case scene.Rect:
			op, r := 0.85, 3.0
			if group {
				op, r = 0.14, 6
			}
			if setOpacity {
				it.FillOpacity = new(op)
			}
			if setCorner {
				it.CornerRadius = r
			}
			if depth == 0 {
				continue
			}
			x, y, w, h := shape.Box(it)
			name := nodeName(it)
			lab := scene.Item{X: x + 6, Y: y + 13, Text: scene.Lines{name}, FontSize: 11, FontWeight: "600", Fill: scene.Paint{Color: s.rt.theme.Text}}
			if group {
				if w <= 40 {
					continue
				}
			} else {
				lab.X, lab.FontSize, lab.Fill = x+5, 10, halo
				if tw, _ := st.TextSize(&lab); w <= tw+10 || h <= 18 {
					continue
				}
			}
			lab.Datum = map[string]any{"name": name}
			names.Items = append(names.Items, lab)
		case scene.Symbol:
			if setOpacity {
				switch {
				case depth == 0 && group:
					it.Fill = scene.Paint{Color: s.rt.theme.Panel}
				case depth == 1 && group:
					it.FillOpacity = new(0.16)
				case group:
					it.FillOpacity = new(0.45)
				default:
					it.FillOpacity = new(0.85)
				}
			}
			if depth == 1 && group && !encodes(m, "stroke") {
				it.Stroke, it.StrokeWidth = it.Fill, new(1.0)
			}
			r := 0.0
			if it.Size != nil {
				r = math.Sqrt(*it.Size) / 2
			}
			if depth == 1 && group && r > 24 {
				name := nodeName(it)
				names.Items = append(names.Items, scene.Item{
					X: it.X, Y: it.Y - r + 14, Text: scene.Lines{name}, FontSize: 11, FontWeight: "600",
					Align: "center", Fill: scene.Paint{Color: s.rt.theme.Text}, Stroke: halo, StrokeWidth: new(3.0),
					Datum: map[string]any{"name": name},
				})
			}
		case scene.Arc:
			if setOpacity && depth >= 1 {
				it.FillOpacity = new([]float64{1, 0.7, 0.42}[min(depth, 3)-1])
			}
		}
	}
	if len(names.Items) == 0 || s.hasText() {
		// A spec that names its nodes with a text mark keeps its names.
		return nil
	}
	return []scene.Mark{names}
}

// hasText reports whether the spec of a scope has a text mark.
func (s *scope) hasText() bool {
	for i := range s.spec.Marks {
		if s.spec.Marks[i].Type == scene.Text {
			return true
		}
	}
	return false
}

// donutTotal writes the total of a donut in its center (docs/STYLE.md): the
// sum of the values that the arcs span, when the angles of the arcs read
// their start and their end through a scale, as a donut of Vega-Lite does.
func (s *scope) donutTotal(m *vega.Mark, sm *scene.Mark) []scene.Mark {
	start, _ := s.encodedField(m, "startAngle")
	end, _ := s.encodedField(m, "endAngle")
	if start == "" || end == "" {
		return nil
	}
	first := &sm.Items[0]
	inner := first.InnerRadius
	total := 0.0
	for i := range sm.Items {
		it := &sm.Items[i]
		if it.X != first.X || it.Y != first.Y || it.InnerRadius <= 0 {
			// Arcs of more than one center, or a pie, have no middle to
			// write in.
			return nil
		}
		inner = math.Min(inner, it.InnerRadius)
		a, ok1 := readField(it.Datum, start).(float64)
		b, ok2 := readField(it.Datum, end).(float64)
		if !ok1 || !ok2 {
			return nil
		}
		total += math.Abs(b - a)
	}
	size := math.Min(28, inner*0.45)
	if size < 10 {
		return nil
	}
	text := scene.Item{
		X: first.X, Y: first.Y, Text: scene.Lines{s.numberText(total)}, FontSize: size, FontWeight: "600",
		Align: "center", Baseline: "middle", Fill: scene.Paint{Color: s.rt.theme.Text},
		Datum: map[string]any{"value": total},
	}
	return []scene.Mark{{Type: scene.Text, Role: "zuzu-donut-total", Items: []scene.Item{text}}}
}

// sankeyLook draws the links of a sankey at 0.32 opacity, and its nodes
// with round corners of 3 (docs/STYLE.md), for the properties that the
// encoding does not set. The rows of zuzu.sankeylinks hold a path and a
// width, and the rows of zuzu.sankey a layer.
func sankeyLook(m *vega.Mark, sm *scene.Mark) {
	d, ok := sm.Items[0].Datum.(map[string]any)
	if !ok {
		return
	}
	has := func(keys ...string) bool {
		for _, k := range keys {
			if _, ok := d[k]; !ok {
				return false
			}
		}
		return true
	}
	switch {
	case sm.Type == scene.Path && has("path", "width", "x0", "x1", "y0", "y1"):
		if encodes(m, "fillOpacity") || encodes(m, "opacity") {
			return
		}
		for i := range sm.Items {
			sm.Items[i].FillOpacity = new(0.32)
		}
	case sm.Type == scene.Rect && has("layer", "depth", "value", "x0", "x1"):
		if encodesCorners(m) {
			return
		}
		for i := range sm.Items {
			sm.Items[i].CornerRadius = 3
		}
	}
}
