package layout

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/xo/zuzu/expr"

	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/theme"
)

// Axis is an axis, with the settings that the view resolved from the spec.
type Axis struct {
	// Orient is top, bottom, left or right.
	Orient string
	Scale  scale.Scale
	// Length is the size of the plot area along the axis, and Cross the
	// size across it, which the grid lines span.
	Length, Cross float64
	// Values are the values of the ticks, or nil for those of the scale.
	Values []any
	// Count is the number of ticks the spec asks for, or 0.
	Count float64
	// MinStep is the least step between two ticks that the spec asks for,
	// in the units of the domain, which are milliseconds for times, or 0.
	MinStep float64
	// EndTicks chooses a step of the ticks that lands on both ends of a
	// nice domain, for an axis whose spec gives no tickCount and no values
	// (D46).
	EndTicks bool
	// Format writes the label of a value.
	Format func(any) string
	// Grid, Labels, Domain and Ticks say which parts the spec asks for:
	// the grid lines, the labels, the line of the axis and its ticks.
	Grid, Labels, Domain, Ticks bool
	// TickSize is the length of a tick, from the line of the axis away
	// from the plot area. A tick of 0 or less draws nothing.
	TickSize float64
	Title    string
	// LabelAngle turns the labels, in degrees.
	LabelAngle float64
	// LabelTurn is the angle to which the labels of a discrete axis turn
	// when one label does not fit the step of its band, or 0 to keep
	// LabelAngle. With a LabelAngle of 0, the labels draw flat when each
	// fits (D58 part 4).
	LabelTurn float64
	// LabelFontSize is the size of the labels that the spec gives, which
	// LabelTurn measures them at, or 0 for LabelSize.
	LabelFontSize float64
	// Overlap drops every other label, in turns, until no two overlap.
	Overlap bool
	// Offset moves the axis away from the plot area.
	Offset float64
	// BandPosition is where in a band a grid line sits, from 0 to 1. A
	// label stays in the middle of its band.
	BandPosition float64
	// TickExtra adds a tick and a grid line at the start of the band of
	// the first value, the place of the axis before its first band. With a
	// BandPosition of 1, the ticks then mark both edges of each band. Only
	// a scale with bands draws it.
	TickExtra bool
	// TickOffset moves the ticks, the labels and the grid lines along the
	// axis, in pixels.
	TickOffset float64
	// TickRound rounds the place of each tick, label and grid line along
	// the axis to a whole pixel.
	TickRound bool
	// LabelFlush is the distance from an end of the axis within which a
	// label aligns flush with that end, so that it stays inside the axis,
	// or a negative number for none.
	LabelFlush float64
	// LabelLimit is the widest that a label draws, in pixels, before it is
	// cut with an ellipsis, or 0 for no limit.
	LabelLimit float64
	// LabelAlign is the align of each label that the spec gives, over the
	// align of the look, the angle and the flush. It is "" for none.
	LabelAlign string
	// TitleX and TitleY place the title from the origin of the axis, as the
	// spec gives them, or are nil for the place of the look. The origin is
	// where the line of the axis starts, at the offset of the axis.
	TitleX, TitleY *float64
	// TitleAngle turns the title, in degrees, or is nil for no turn.
	TitleAngle *float64
	// TitleAlign and TitleBaseline are the align and the baseline of the
	// title that the spec gives, or "" for those of the look.
	TitleAlign, TitleBaseline string
	// LabelPadding is the distance between the labels and the end of the
	// ticks that the spec gives, or nil for the padding of the look. It
	// can be below 0, which puts the labels inside the plot area.
	LabelPadding *float64
	// LabelOffset moves each label along the axis, in pixels.
	LabelOffset float64
	// LabelBaseline is the baseline of each label that the spec gives, over
	// the baseline of the look, the angle and the flush. It is "" for none.
	LabelBaseline string
	// TitleAnchor is start, middle or end: the point of the range of the
	// scale at which the title sits, as the spec gives it. It is "" for the
	// place of the look.
	TitleAnchor string
	// TitlePadding is the space between the labels and the title that the
	// spec gives, or nil for the space of the look.
	TitlePadding *float64
}

// TickCount returns the number of ticks that the axis asks its scale for:
// the count of the spec, or one each 40 pixels. A scale of numbers has no
// more than MaxLabels ticks.
func (a *Axis) TickCount() float64 {
	count := a.Count
	if count <= 0 {
		count = math.Max(2, math.Min(MaxLabels, a.Length/40))
		if a.Scale.Type() == scale.Trading {
			// A label of a trading scale is short and marks a period,
			// such as a week, so the axis takes one each 50 pixels.
			count = math.Max(2, a.Length/50)
		}
	}
	if !scale.Discrete(a.Scale.Type()) {
		count = math.Min(count, MaxLabels)
	}
	if a.MinStep > 0 {
		// A step of at least MinStep leaves room for one tick more than
		// the steps that fit in the domain, and for two at least, as Vega
		// counts them.
		count = math.Min(count, math.Max(1, math.Floor(a.span()/a.MinStep))+1)
	}
	return count
}

// TickValues returns the values of the ticks of the axis: the values of
// the spec, the ticks of its scale, or the ticks that land on both ends of
// the domain when the axis asks for them.
func (a *Axis) TickValues() []any {
	if a.Values != nil {
		return a.Values
	}
	if vals := a.endTicks(); vals != nil {
		return vals
	}
	return a.Scale.Ticks(a.TickCount())
}

// TickStep returns the step whose precision the labels of the ticks of a
// scale of numbers from lo to hi need.
func (a *Axis) TickStep(lo, hi float64) float64 {
	if a.EndTicks && a.Values == nil && Linear(a.Scale.Type()) {
		if t, step := scale.EndTicks(lo, hi, a.TickCount(), MaxLabels); t != nil {
			return step
		}
	}
	return scale.TickStep(lo, hi, a.TickCount())
}

// span returns the size of the domain of the scale of the axis, in
// milliseconds for a domain of times, or 0 for a domain that is not of
// numbers or times.
func (a *Axis) span() float64 {
	dom := a.Scale.Domain()
	if len(dom) < 2 {
		return 0
	}
	num := func(v any) float64 {
		if t, ok := v.(time.Time); ok {
			return float64(t.UnixMilli())
		}
		return expr.Number(v)
	}
	s := math.Abs(num(dom[len(dom)-1]) - num(dom[0]))
	if math.IsNaN(s) || math.IsInf(s, 0) {
		return 0
	}
	return s
}

// labelPad returns the distance between the labels and the line of the
// axis. As in Vega, a label is its padding past the end of its tick, or
// past the line when the axis has no ticks. The padding is the one that
// the spec gives, or the one of the look.
func (a *Axis) labelPad() float64 {
	pad := theme.LabelPadding
	if a.LabelPadding != nil {
		pad = *a.LabelPadding
	}
	if a.Ticks {
		pad += math.Max(0, a.TickSize)
	}
	return pad
}

// vertical reports whether the axis runs up and down.
func (a *Axis) vertical() bool {
	return a.Orient == "left" || a.Orient == "right"
}

// endTicks returns the ticks that land on both ends of the domain of a
// scale of numbers, or of a scale of time whose domain starts and ends on
// the start of a year, or nil. A log scale keeps the ticks of its scale,
// which are the powers of its base, because a nice domain of a log scale
// starts and ends on a power.
func (a *Axis) endTicks() []any {
	dom := a.Scale.Domain()
	if !a.EndTicks || len(dom) < 2 {
		return nil
	}
	if t0, ok := dom[0].(time.Time); ok {
		t1, ok := dom[len(dom)-1].(time.Time)
		if !ok {
			return nil
		}
		return a.yearTicks(t0, t1)
	}
	if !Linear(a.Scale.Type()) {
		return nil
	}
	ticks, _ := scale.EndTicks(expr.Number(dom[0]), expr.Number(dom[len(dom)-1]), a.TickCount(), MaxLabels)
	if ticks == nil {
		return nil
	}
	out := make([]any, len(ticks))
	for i, t := range ticks {
		out[i] = t
	}
	return out
}

// yearTicks returns ticks at the start of a year, in a step of whole years
// that lands on both ends of a domain of time from t0 to t1, or nil when
// the domain does not start and end on the start of a year, or when the
// ticks of the scale already land on both ends.
func (a *Axis) yearTicks(t0, t1 time.Time) []any {
	std := a.Scale.Ticks(a.TickCount())
	if len(std) >= 2 && len(std) <= MaxLabels {
		f, ok0 := std[0].(time.Time)
		l, ok1 := std[len(std)-1].(time.Time)
		if ok0 && ok1 && f.Equal(t0) && l.Equal(t1) {
			return nil
		}
	}
	yearStart := func(t time.Time) bool {
		return t.Equal(time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location()))
	}
	if !yearStart(t0) || !yearStart(t1) {
		return nil
	}
	years, _ := scale.EndTicks(float64(t0.Year()), float64(t1.Year()), a.TickCount(), MaxLabels)
	if years == nil {
		return nil
	}
	out := make([]any, len(years))
	for i, y := range years {
		if y != math.Trunc(y) {
			return nil
		}
		out[i] = time.Date(int(y), 1, 1, 0, 0, 0, 0, t0.Location())
	}
	return out
}

// Linear reports whether a scale type maps numbers along a line or a
// curve, so that its ticks are the round numbers of its domain: linear,
// pow, sqrt and symlog, also as sequential or diverging.
func Linear(typ string) bool {
	typ = strings.TrimPrefix(strings.TrimPrefix(typ, "sequential-"), "diverging-")
	switch typ {
	case "linear", "pow", "sqrt", "symlog", "sequential":
		return true
	}
	return false
}

// ticks returns the values of the ticks and their positions along the
// axis.
func (a *Axis) ticks() ([]any, []float64) {
	return a.place(a.TickValues())
}

// vegaGridGap is the least space, in pixels, between two grid lines of
// Vega that an axis draws (D51).
const vegaGridGap = 10

// gridTicks returns the values and the positions of the grid lines and the
// ticks of the axis: the ticks that Vega draws, for the tickCount of the
// axis or for 10, when each label of the look is one of them and they
// stand at least vegaGridGap apart, and else the labels, vals at pos
// (D51). Values of the spec and a discrete scale keep the labels.
func (a *Axis) gridTicks(vals []any, pos []float64) ([]any, []float64) {
	if a.Values != nil || scale.Discrete(a.Scale.Type()) || len(vals) == 0 {
		return vals, pos
	}
	count := a.Count
	if count <= 0 {
		count = 10
	}
	if a.MinStep > 0 {
		count = math.Min(count, math.Max(1, math.Floor(a.span()/a.MinStep))+1)
	}
	gv, gp := a.place(a.Scale.Ticks(count))
	if len(gv) <= len(vals) {
		return vals, pos
	}
	for i := 1; i < len(gp); i++ {
		if math.Abs(gp[i]-gp[i-1]) < vegaGridGap {
			return vals, pos
		}
	}
	for _, v := range vals {
		if !slices.ContainsFunc(gv, func(g any) bool { return sameTick(v, g) }) {
			return vals, pos
		}
	}
	return gv, gp
}

// sameTick reports whether two values of ticks are the same: two times at
// the same instant, or two numbers that differ by less than a billionth.
func sameTick(a, b any) bool {
	if ta, ok := a.(time.Time); ok {
		tb, ok := b.(time.Time)
		return ok && ta.Equal(tb)
	}
	x, y := expr.Number(a), expr.Number(b)
	return math.Abs(x-y) <= 1e-9*math.Max(1, math.Max(math.Abs(x), math.Abs(y)))
}

// place returns the values that fall on the axis and their positions along
// it.
func (a *Axis) place(vals []any) ([]any, []float64) {
	bw := a.Scale.Bandwidth()
	var out []any
	var pos []float64
	for _, v := range vals {
		p, ok := a.Scale.Map(v).(float64)
		if !ok || math.IsNaN(p) {
			continue
		}
		p += bw * a.BandPosition
		if p < -0.5 || p > a.Length+0.5 {
			continue
		}
		out = append(out, v)
		pos = append(pos, a.adjust(p))
	}
	return out, pos
}

// adjust moves a place along the axis by the tick offset, and rounds it
// when the axis asks.
func (a *Axis) adjust(p float64) float64 {
	p += a.TickOffset
	if a.TickRound {
		p = math.Round(p)
	}
	return p
}

// extraTick returns the place of the extra tick and grid line of an axis
// that asks for one: the start of the band of the first value of vals.
func (a *Axis) extraTick(vals []any) (float64, bool) {
	if !a.TickExtra || len(vals) == 0 || a.Scale.Bandwidth() <= 0 {
		return 0, false
	}
	p, ok := a.Scale.Map(vals[0]).(float64)
	if !ok || math.IsNaN(p) {
		return 0, false
	}
	return a.adjust(p), true
}

// extraDatum returns the datum of the extra tick and grid line, which holds
// the first value and an extra flag.
func extraDatum(vals []any) map[string]any {
	return map[string]any{"value": vals[0], "index": 0.0, "extra": true}
}

// zero returns the position of zero along the axis, for a linear scale
// whose domain holds zero.
func (a *Axis) zero() (float64, bool) {
	typ := strings.TrimPrefix(a.Scale.Type(), "sequential-")
	if typ != "linear" && typ != "pow" && typ != "sqrt" && typ != "symlog" {
		return 0, false
	}
	dom := a.Scale.Domain()
	if len(dom) < 2 {
		return 0, false
	}
	lo, hi := expr.Number(dom[0]), expr.Number(dom[len(dom)-1])
	if math.Min(lo, hi) > 0 || math.Max(lo, hi) < 0 {
		return 0, false
	}
	p, ok := a.Scale.Map(0.0).(float64)
	if !ok || math.IsNaN(p) {
		return 0, false
	}
	return p, true
}

// Axis draws an axis as a group of the role axis, with its grid, its
// labels, its base line and its title. The group sits at the origin of the
// plot area.
func (s Style) Axis(a *Axis) scene.Mark {
	t := s.Theme
	group := scene.Item{}
	vals, pos := a.ticks()
	// The grid lines and the ticks stand where Vega draws them, and the
	// labels at the fewer ticks of the look (D51).
	gvals, gpos := a.gridTicks(vals, pos)
	if a.Grid {
		grid := scene.Mark{Type: scene.Rule, Role: "axis-grid"}
		for i, p := range gpos {
			it := scene.Item{Stroke: scene.Paint{Color: t.Grid}, StrokeWidth: ptr(1), Datum: tickDatum(gvals, i)}
			if a.vertical() {
				it.X, it.X2, it.Y, it.Y2 = 0, a.Cross, p, p
			} else {
				it.X, it.X2, it.Y, it.Y2 = p, p, 0, a.Cross
			}
			grid.Items = append(grid.Items, it)
		}
		if p, ok := a.extraTick(gvals); ok {
			it := scene.Item{Stroke: scene.Paint{Color: t.Grid}, StrokeWidth: ptr(1), Datum: extraDatum(gvals)}
			if a.vertical() {
				it.X, it.X2, it.Y, it.Y2 = 0, a.Cross, p, p
			} else {
				it.X, it.X2, it.Y, it.Y2 = p, p, 0, a.Cross
			}
			grid.Items = append(grid.Items, it)
		}
		group.Items = append(group.Items, grid)
		if z, ok := a.zero(); ok {
			// The base line at zero of a scale of numbers is one line in
			// the color of an axis (docs/STYLE.md).
			it := scene.Item{Stroke: scene.Paint{Color: t.Axis}, StrokeWidth: ptr(1)}
			if a.vertical() {
				it.X, it.X2, it.Y, it.Y2 = 0, a.Cross, z, z
			} else {
				it.X, it.X2, it.Y, it.Y2 = z, z, 0, a.Cross
			}
			group.Items = append(group.Items, scene.Mark{Type: scene.Rule, Role: "axis-zero", Items: []scene.Item{it}})
		}
	}
	labels := Empty()
	if a.Labels && len(vals) > 0 {
		m := s.axisLabels(a, vals, pos)
		labels = s.MarkBounds(&m)
		group.Items = append(group.Items, m)
	}
	// The line and the ticks of an axis are where Vega draws them, in the
	// color and the widths of the look (D46, docs/STYLE.md).
	if a.Ticks && a.TickSize > 0 && len(gpos) > 0 {
		group.Items = append(group.Items, a.tickMarks(t.Axis, gvals, gpos))
	}
	if a.Domain {
		group.Items = append(group.Items, a.domainMark(t.Axis))
	}
	if a.Title != "" {
		group.Items = append(group.Items, s.axisTitle(a, labels))
	}
	return scene.Mark{Type: scene.Group, Role: "axis", Items: []scene.Item{group}}
}

// tickDatum returns the datum of the grid line, the tick or the label of
// the value i of vals: the value, and its index from 0 at the first value
// to 1 at the last, as Vega gives an encode or a signal of an axis.
func tickDatum(vals []any, i int) map[string]any {
	return map[string]any{"value": vals[i], "index": float64(i) / float64(max(1, len(vals)-1))}
}

// tickMarks draws a tick at the place of each value, out from the line of
// the axis by its tick size.
func (a *Axis) tickMarks(color string, vals []any, pos []float64) scene.Mark {
	m := scene.Mark{Type: scene.Rule, Role: "axis-tick"}
	x0, y0 := a.origin()
	data := make([]map[string]any, 0, len(pos)+1)
	for i := range pos {
		data = append(data, tickDatum(vals, i))
	}
	if p, ok := a.extraTick(vals); ok {
		pos = append(slices.Clip(pos), p)
		data = append(data, extraDatum(vals))
	}
	for i, p := range pos {
		it := scene.Item{Stroke: scene.Paint{Color: color}, StrokeWidth: ptr(theme.TickWidth), Datum: data[i]}
		switch a.Orient {
		case "bottom":
			ruleEnds(&it, p, y0, p, y0+a.TickSize)
		case "top":
			ruleEnds(&it, p, y0, p, y0-a.TickSize)
		case "left":
			ruleEnds(&it, x0, p, x0-a.TickSize, p)
		default:
			ruleEnds(&it, x0, p, x0+a.TickSize, p)
		}
		m.Items = append(m.Items, it)
	}
	return m
}

// domainMark draws the line of an axis along the range of its scale, as
// Vega draws it.
func (a *Axis) domainMark(color string) scene.Mark {
	r0, r1 := a.rangeEnds()
	x0, y0 := a.origin()
	it := scene.Item{Stroke: scene.Paint{Color: color}, StrokeWidth: ptr(theme.DomainWidth)}
	if a.vertical() {
		ruleEnds(&it, x0, r0, x0, r1)
	} else {
		ruleEnds(&it, r0, y0, r1, y0)
	}
	return scene.Mark{Type: scene.Rule, Role: "axis-domain", Items: []scene.Item{it}}
}

// ruleEnds sets the ends of a rule that runs along x or along y. A rule
// whose x2 or y2 is 0 ends at its x or its y (scene.Item), so the end at 0
// of the coordinate that changes goes first.
func ruleEnds(it *scene.Item, x1, y1, x2, y2 float64) {
	if (x2 == 0 && x1 != 0) || (y2 == 0 && y1 != 0) {
		x1, y1, x2, y2 = x2, y2, x1, y1
	}
	it.X, it.Y, it.X2, it.Y2 = x1, y1, x2, y2
}

// axisLabels draws the labels of an axis, past its ticks, without those
// that overlap when the axis asks.
func (s Style) axisLabels(a *Axis, vals []any, pos []float64) scene.Mark {
	m := scene.Mark{Type: scene.Text, Role: "axis-label"}
	color := s.Theme.Muted
	pad := a.labelPad()
	// A label stays in the middle of its band, where the bandPosition of
	// the axis puts its tick and its grid line, as in Vega.
	shift := a.Scale.Bandwidth() * (0.5 - a.BandPosition)
	angle := a.LabelAngle
	if angle == 0 && a.LabelTurn != 0 && !s.labelsFit(a, vals, pos) {
		angle = a.LabelTurn
	}
	for i, v := range vals {
		str := a.Format(v)
		if str == "" {
			// A format writes no label for some ticks, such as the ticks
			// of a log scale between two powers.
			continue
		}
		var it scene.Item
		at := pos[i] + shift
		if a.TickRound {
			at = math.Round(at)
		}
		at += a.LabelOffset
		switch a.Orient {
		case "bottom":
			it = s.label(at, a.Cross+a.Offset+pad, str, LabelSize, 400, color, "center", "top")
		case "top":
			it = s.label(at, -a.Offset-pad, str, LabelSize, 400, color, "center", "bottom")
		case "left":
			it = s.label(-a.Offset-pad, at, str, LabelSize, 400, color, "right", "middle")
		default:
			it = s.label(a.Cross+a.Offset+pad, at, str, LabelSize, 400, color, "left", "middle")
		}
		if angle != 0 {
			it.Angle = angle
			turnLabel(&it, a.Orient, angle)
		} else if a.LabelFlush >= 0 {
			flush(&it, a, pos[i])
		}
		if a.LabelAlign != "" {
			it.Align = a.LabelAlign
		}
		if a.LabelBaseline != "" {
			it.Baseline = a.LabelBaseline
		}
		it.Limit = a.LabelLimit
		d := tickDatum(vals, i)
		d["label"] = str
		it.Datum = d
		m.Items = append(m.Items, it)
	}
	if a.Overlap {
		m.Items = s.dropOverlaps(m.Items)
	}
	return m
}

// labelsFit reports whether each label of a discrete axis fits the step of
// its band, with the gap of theme.LabelGap, as docs/STYLE.md measures it:
// the width of the label in the font of the look at the size of the
// labels, up to its limit (D58 part 4). The step is the least distance
// between two places next to each other, or the length of the axis for
// one place.
func (s Style) labelsFit(a *Axis, vals []any, pos []float64) bool {
	step := a.Length
	for i := 1; i < len(pos); i++ {
		step = math.Min(step, math.Abs(pos[i]-pos[i-1]))
	}
	size := a.LabelFontSize
	if size <= 0 {
		size = LabelSize
	}
	for _, v := range vals {
		w := s.width(size, 400, a.Format(v))
		if a.LabelLimit > 0 {
			w = math.Min(w, a.LabelLimit)
		}
		if w+theme.LabelGap > step {
			return false
		}
	}
	return true
}

// flush aligns a label at an end of its axis with that end: the first label
// of a horizontal axis starts at the start, the last ends at the end, and
// the labels of a vertical axis hang below the top and sit on the bottom.
func flush(it *scene.Item, a *Axis, pos float64) {
	near := func(end float64) bool { return math.Abs(pos-end) <= a.LabelFlush }
	if a.vertical() {
		switch {
		case near(0):
			it.Baseline = "top"
		case near(a.Length):
			it.Baseline = "bottom"
		}
		return
	}
	switch {
	case near(0):
		it.Align = "left"
	case near(a.Length):
		it.Align = "right"
	}
}

// turnLabel sets the align and the baseline of a turned label, so that its
// end meets the axis.
func turnLabel(it *scene.Item, orient string, angle float64) {
	a := math.Mod(angle+360, 360)
	if orient == "bottom" || orient == "top" {
		switch {
		case a > 0 && a < 180:
			it.Align, it.Baseline = "left", "middle"
		case a > 180 && a < 360:
			it.Align, it.Baseline = "right", "middle"
		}
		if orient == "top" {
			switch it.Align {
			case "left":
				it.Align = "right"
			case "right":
				it.Align = "left"
			}
		}
	}
}

// dropOverlaps keeps every other label until no two labels overlap, and
// always keeps the first.
func (s Style) dropOverlaps(items []scene.Item) []scene.Item {
	return s.dropOverlapsBy(items, 2, 0)
}

// dropOverlapsBy keeps every other label until no two labels stand closer
// than sepX across and sepY up and down, and always keeps the first.
func (s Style) dropOverlapsBy(items []scene.Item, sepX, sepY float64) []scene.Item {
	overlaps := func(list []scene.Item) bool {
		for i := 1; i < len(list); i++ {
			a, b := s.textBounds(&list[i-1]), s.textBounds(&list[i])
			if a.X1 < b.X2+sepX && b.X1 < a.X2+sepX && a.Y1 < b.Y2+sepY && b.Y1 < a.Y2+sepY {
				return true
			}
		}
		return false
	}
	for len(items) > 2 && overlaps(items) {
		var next []scene.Item
		for i := 0; i < len(items); i += 2 {
			next = append(next, items[i])
		}
		items = next
	}
	return items
}

// axisTitle draws the title of an axis at the end of the axis, rather than
// along it: above a vertical axis, and at the right of a horizontal one,
// past the bounds of its labels. A titleAnchor of the spec moves the title
// to that point of the range, and the place, the angle, the align and the
// baseline that the spec gives win over those of the look.
func (s Style) axisTitle(a *Axis, labels Bounds) scene.Mark {
	m := scene.Mark{Type: scene.Text, Role: "axis-title"}
	color := s.Theme.Muted
	gap := float64(titleGap)
	if a.TitlePadding != nil {
		gap = *a.TitlePadding
	}
	var it scene.Item
	switch {
	case a.TitleAnchor != "":
		it = s.anchoredTitle(a, labels, gap)
	case a.Orient == "bottom":
		it = s.label(a.Length, a.titleCross(labels)+gap, a.Title, SmallSize, 400, color, "right", "top")
	case a.Orient == "top":
		it = s.label(a.Length, a.titleCross(labels)-gap, a.Title, SmallSize, 400, color, "right", "bottom")
	case a.Orient == "left":
		it = s.label(-a.Offset, a.topTitle(labels), a.Title, SmallSize, 400, color, "left", "bottom")
	default:
		it = s.label(a.Cross+a.Offset, a.topTitle(labels), a.Title, SmallSize, 400, color, "right", "bottom")
	}
	x, y := a.origin()
	if a.TitleX != nil {
		it.X = x + *a.TitleX
	}
	if a.TitleY != nil {
		it.Y = y + *a.TitleY
	}
	if a.TitleAngle != nil {
		it.Angle = *a.TitleAngle
	}
	if a.TitleAlign != "" {
		it.Align = a.TitleAlign
	}
	if a.TitleBaseline != "" {
		it.Baseline = a.TitleBaseline
	}
	m.Items = []scene.Item{it}
	return m
}

// anchoredTitle draws the title of an axis at the start, the middle or the
// end of the range of its scale, aligned left, center or right, as Vega
// places a title with a titleAnchor. The title of a horizontal axis sits
// past the labels, as in the look. The title of a vertical axis turns, as
// in Vega, and sits past the labels, because the look has no place for it
// along the axis.
func (s Style) anchoredTitle(a *Axis, labels Bounds, gap float64) scene.Item {
	r0, r1 := a.rangeEnds()
	at, align := r1, "right"
	switch a.TitleAnchor {
	case "start":
		at, align = r0, "left"
	case "middle":
		at, align = (r0+r1)/2, "center"
	}
	color := s.Theme.Muted
	switch a.Orient {
	case "bottom":
		return s.label(at, a.titleCross(labels)+gap, a.Title, SmallSize, 400, color, align, "top")
	case "top":
		return s.label(at, a.titleCross(labels)-gap, a.Title, SmallSize, 400, color, align, "bottom")
	case "left":
		x := -a.Offset
		if !labels.IsEmpty() {
			x = math.Min(x, labels.X1)
		}
		it := s.label(x-gap, at, a.Title, SmallSize, 400, color, align, "bottom")
		it.Angle = -90
		return it
	}
	x := a.Cross + a.Offset
	if !labels.IsEmpty() {
		x = math.Max(x, labels.X2)
	}
	it := s.label(x+gap, at, a.Title, SmallSize, 400, color, align, "bottom")
	it.Angle = 90
	return it
}

// titleCross returns the y of the edge of the labels of a horizontal axis
// away from the plot area, from which its title keeps its space.
func (a *Axis) titleCross(labels Bounds) float64 {
	if !a.Labels {
		// An axis with no labels keeps no room for them, as in Vega. The
		// title sits past the ticks.
		reach := 0.0
		if a.Ticks {
			reach = math.Max(0, a.TickSize)
		}
		if a.Orient == "top" {
			return -a.Offset - reach
		}
		return a.Cross + a.Offset + reach
	}
	if a.Orient == "top" {
		y := -a.Offset - a.labelPad() - LabelSize
		if !labels.IsEmpty() {
			y = math.Min(y, labels.Y1)
		}
		return y
	}
	y := a.Cross + a.Offset + a.labelPad() + LabelSize
	if !labels.IsEmpty() {
		// Labels that turn reach further from the axis.
		y = math.Max(y, labels.Y2)
	}
	return y
}

// topTitle returns the y of the title of a vertical axis, which sits above
// the axis. A titlePadding of the spec keeps that space from the top of the
// labels.
func (a *Axis) topTitle(labels Bounds) float64 {
	if a.TitlePadding == nil {
		return -LabelPad - 4
	}
	y := 0.0
	if !labels.IsEmpty() {
		y = math.Min(y, labels.Y1)
	}
	return y - *a.TitlePadding
}

// rangeEnds returns the start and the end of the range of the scale of an
// axis, in pixels, or 0 and the length of the axis for a range that is not
// of numbers.
func (a *Axis) rangeEnds() (float64, float64) {
	r := a.Scale.Range()
	if len(r) >= 2 {
		lo, ok1 := r[0].(float64)
		hi, ok2 := r[len(r)-1].(float64)
		if ok1 && ok2 && !math.IsNaN(lo) && !math.IsNaN(hi) && !math.IsInf(lo, 0) && !math.IsInf(hi, 0) {
			return lo, hi
		}
	}
	return 0, a.Length
}

// titleGap is the space between the labels of a horizontal axis and its
// title.
const titleGap = 6

// origin returns where the line of an axis starts, at its offset, in the
// coordinates of the plot area: the place from which Vega reads the titleX
// and the titleY of the axis.
func (a *Axis) origin() (float64, float64) {
	switch a.Orient {
	case "bottom":
		return 0, a.Cross + a.Offset
	case "top":
		return 0, -a.Offset
	case "left":
		return -a.Offset, 0
	}
	return a.Cross + a.Offset, 0
}

// ptr returns a pointer to a number.
func ptr(f float64) *float64 {
	return new(f)
}
