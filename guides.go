package zuzu

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/format"
	"github.com/xo/zuzu/layout"
	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/svg"
	"github.com/xo/zuzu/vega"
)

// guideGap is the space between the plot area, the legends and the title.
const guideGap = 16

// style returns the style that the guides draw with.
func (rt *runtime) style() layout.Style {
	return layout.Style{Theme: rt.theme, Font: svg.DefaultFont, Measure: rt.measure}
}

// isIssue reports whether an error is an issue of an unsupported feature,
// which the loose mode reports and skips.
func isIssue(err error, is *vega.Issue) bool {
	return errors.As(err, is) && errors.Is(err, vega.ErrUnsupported)
}

// size returns the width and the height of the group of the scope.
func (s *scope) size() (float64, float64) {
	if s.item != nil {
		return s.item.Width, s.item.Height
	}
	return 0, 0
}

// buildAxes builds the axes of a scope.
func (s *scope) buildAxes(path string) ([]scene.Mark, error) {
	var out []scene.Mark
	for i := range s.spec.Axes {
		a := &s.spec.Axes[i]
		p := fmt.Sprintf("%s/axes/%d", path, i)
		m, err := s.buildAxis(a, p)
		if err != nil {
			var is vega.Issue
			if isIssue(err, &is) {
				if err := s.rt.issue(err, p, ""); err != nil {
					return nil, err
				}
				continue
			}
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if m != nil {
			out = append(out, *m)
		}
	}
	return out, nil
}

// buildAxis resolves an axis and draws it.
func (s *scope) buildAxis(a *vega.Axis, path string) (*scene.Mark, error) {
	sc, err := s.scale(a.Scale)
	if err != nil {
		return nil, err
	}
	orient, err := s.str(a.Orient, "bottom")
	if err != nil {
		return nil, err
	}
	w, h := s.size()
	cfg := s.rt.axisConfig(orient, sc)
	ax := &layout.Axis{Orient: orient, Scale: sc, Length: w, Cross: h}
	if orient == "left" || orient == "right" {
		ax.Length, ax.Cross = h, w
	}
	if ax.Length <= 0 {
		// A group with no size along the axis still has an axis, along the
		// range of its scale, as in Vega.
		ax.Length = rangeEnd(sc)
	}
	ax.Grid = a.Grid != nil && *a.Grid
	if g, ok := cfg["grid"].(bool); ok && a.Grid == nil {
		ax.Grid = g
	}
	if ax.Labels, err = s.boolean(a.Labels, true); err != nil {
		return nil, err
	}
	// The config of the axis can hide the domain, as it can the grid.
	showDomain, ok := cfg["domain"].(bool)
	if !ok {
		showDomain = true
	}
	domain, err := s.boolean(a.Domain, showDomain)
	if err != nil {
		return nil, err
	}
	ax.Domain = domain
	if ax.Title, err = s.titleText(a.Title); err != nil {
		return nil, err
	}
	count, err := s.tickCount(a.TickCount, sc, ax)
	if err != nil {
		return nil, err
	}
	ax.Count = count
	if !a.Values.IsZero() {
		v, err := s.value(a.Values)
		if err != nil {
			return nil, err
		}
		if l, ok := v.([]any); ok {
			ax.Values = l
		}
	}
	if ax.LabelAngle, err = s.num(a.LabelAngle, configNum(cfg, "labelAngle", 0)); err != nil {
		return nil, err
	}
	if ax.Offset, err = s.num(a.Offset, 0); err != nil {
		return nil, err
	}
	overlap := !scale.Discrete(sc.Type())
	if !a.LabelOverlap.IsZero() {
		v, err := s.value(a.LabelOverlap)
		if err != nil {
			return nil, err
		}
		overlap = expr.Truthy(v)
	}
	ax.Overlap = overlap
	bp := 0.0
	if sc.Type() == "band" || sc.Type() == scale.Trading {
		bp = 0.5
	}
	if ax.BandPosition, err = s.num(a.BandPosition, bp); err != nil {
		return nil, err
	}
	// labelFlush is true for a distance of 1, or a distance, and none by
	// default, as in Vega.
	ax.LabelFlush = -1
	if flush, ok := cfg["labelFlush"]; ok || !a.LabelFlush.IsZero() {
		v := flush
		if !a.LabelFlush.IsZero() {
			if v, err = s.value(a.LabelFlush); err != nil {
				return nil, err
			}
		}
		switch f := v.(type) {
		case bool:
			if f {
				ax.LabelFlush = 1
			}
		case float64:
			ax.LabelFlush = f
		}
	}
	// The labels draw at most 180 wide by default, as in Vega.
	if ax.LabelLimit, err = s.num(a.LabelLimit, configNum(cfg, "labelLimit", 180)); err != nil {
		return nil, err
	}
	if ax.Format, err = s.labelFormat(sc, a.Format, a.FormatType, ax); err != nil {
		return nil, err
	}
	m := s.rt.style().Axis(ax)
	// The config and the axis style its parts, over the look of zuzu.
	styleGuide(&m, s.guideProps(cfg, a.Extra), axisParts)
	if err := s.guideEncode(&m, a.Encode, axisEncodeParts, path, "an axis"); err != nil {
		return nil, err
	}
	if a.ZIndex != nil {
		m.ZIndex = *a.ZIndex
	}
	return &m, nil
}

// rangeEnd returns the far end of the range of a scale of positions, or 0
// for a range that is not of numbers.
func rangeEnd(sc scale.Scale) float64 {
	end := 0.0
	for _, v := range sc.Range() {
		if f, ok := v.(float64); ok && !math.IsNaN(f) && !math.IsInf(f, 0) {
			end = math.Max(end, f)
		}
	}
	return end
}

// labelFormat returns the function that writes the label of a value of a
// scale: the format of the spec, or a format that fits the scale.
func (s *scope) labelFormat(sc scale.Scale, fv, ftv vega.Value, ax *layout.Axis) (func(any) string, error) {
	spec, err := s.str(fv, "")
	if err != nil {
		return nil, err
	}
	ftype, err := s.str(ftv, "")
	if err != nil {
		return nil, err
	}
	isTime := strings.HasSuffix(sc.Type(), "time") || strings.HasSuffix(sc.Type(), "utc") || sc.Type() == scale.Trading
	loc := s.rt.loc
	if strings.HasSuffix(sc.Type(), "utc") || ftype == "utc" {
		loc = time.UTC
	}
	if spec != "" {
		if ftype == "time" || ftype == "utc" || (ftype == "" && isTime) {
			f := format.Time(spec, *s.rt.locale)
			return func(v any) string {
				t, ok := toTime(v, loc)
				if !ok {
					return expr.String(v)
				}
				return f(t)
			}, nil
		}
	}
	switch {
	case isTime && spec == "":
		return timeLabel(*s.rt.locale, loc), nil
	case scale.Discrete(sc.Type()) && spec == "":
		return func(v any) string {
			if v == nil {
				return "null"
			}
			return expr.String(v)
		}, nil
	}
	f, err := s.numberLabel(sc, spec, ax)
	if err != nil {
		if err := s.rt.issue(vega.ErrUnsupported, "", "the format %q", spec); err != nil {
			return nil, err
		}
		return expr.String, nil
	}
	return func(v any) string {
		if _, ok := v.(string); ok {
			return expr.String(v)
		}
		return f(expr.Number(v))
	}, nil
}

// legendTicks is the number of ticks that a legend of a scale of numbers
// asks for, as in layout.
const legendTicks = 5

// numberLabel returns the format of the labels of the numbers of a scale,
// as Vega writes them. The labels of a linear scale take the spec, by
// default ",f", with the precision that the step between two ticks needs
// when the spec gives none. A log scale writes the spec, by default ",",
// and only some of its ticks between two powers take a label. Any other
// scale writes the spec as it is.
func (s *scope) numberLabel(sc scale.Scale, spec string, ax *layout.Axis) (func(float64) string, error) {
	loc := *s.rt.locale
	count := float64(legendTicks)
	if ax != nil {
		count = ax.TickCount()
	}
	if show := scale.LogLabels(sc, count); show != nil {
		if spec == "" {
			spec = ","
		}
		f, err := format.Number(spec, loc)
		if err != nil {
			return nil, err
		}
		return func(v float64) string {
			if !show(v) {
				return ""
			}
			return f(v)
		}, nil
	}
	dom := sc.Domain()
	if spec != "" && (scale.Discrete(sc.Type()) || !linearish(sc.Type()) || len(dom) < 2) {
		return format.Number(spec, loc)
	}
	lo, hi := 0.0, 1.0
	if len(dom) >= 2 {
		lo, hi = expr.Number(dom[0]), expr.Number(dom[len(dom)-1])
	}
	return format.Ticks(spec, lo, hi, scale.TickStep(lo, hi, count), loc)
}

// linearish reports whether a scale type maps numbers along a line or a
// curve, so that its ticks are the round numbers of its domain: linear,
// pow, sqrt and symlog, also as sequential or diverging.
func linearish(typ string) bool {
	typ = strings.TrimPrefix(strings.TrimPrefix(typ, "sequential-"), "diverging-")
	switch typ {
	case "linear", "pow", "sqrt", "symlog", "sequential":
		return true
	}
	return false
}

// timeLabel returns a format of times that names the largest unit that
// changes at each tick: the year at its start, the month, the day, the
// hour, the minute or the second.
func timeLabel(l format.Locale, loc *time.Location) func(any) string {
	formats := map[string]func(time.Time) string{}
	for _, f := range []string{"%Y", "%b", "%b %d", "%-I %p", "%-I:%M", ":%S", ".%L"} {
		formats[f] = format.Time(f, l)
	}
	return func(v any) string {
		t, ok := toTime(v, loc)
		if !ok {
			return expr.String(v)
		}
		spec := ".%L"
		switch {
		case t.Nanosecond() != 0:
		case t.Second() != 0:
			spec = ":%S"
		case t.Minute() != 0:
			spec = "%-I:%M"
		case t.Hour() != 0:
			spec = "%-I %p"
		case t.Day() != 1:
			spec = "%b %d"
		case t.Month() != 1:
			spec = "%b"
		default:
			spec = "%Y"
		}
		return formats[spec](t)
	}
}

// toTime reads a value as a time.
func toTime(v any, loc *time.Location) (time.Time, bool) {
	switch x := v.(type) {
	case time.Time:
		return x.In(loc), !x.IsZero()
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return time.Time{}, false
		}
		return time.UnixMilli(int64(x)).In(loc), true
	case string:
		t, ok := format.ParseDate(x, loc)
		return t, ok
	}
	return time.Time{}, false
}

// buildLegends builds the legends of a scope.
func (s *scope) buildLegends(path string) ([]scene.Mark, error) {
	var out []scene.Mark
	for i := range s.spec.Legends {
		l := &s.spec.Legends[i]
		p := fmt.Sprintf("%s/legends/%d", path, i)
		m, err := s.buildLegend(l, p)
		if err != nil {
			var is vega.Issue
			if isIssue(err, &is) {
				if err := s.rt.issue(err, p, ""); err != nil {
					return nil, err
				}
				continue
			}
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if m != nil {
			out = append(out, *m)
		}
	}
	return out, nil
}

// buildLegend resolves a legend and draws it.
func (s *scope) buildLegend(l *vega.Legend, path string) (*scene.Mark, error) {
	lg := &layout.Legend{}
	get := func(name string) (scale.Scale, error) {
		if name == "" {
			return nil, nil
		}
		return s.scale(name)
	}
	var err error
	if lg.Fill, err = get(l.Fill); err != nil {
		return nil, err
	}
	if lg.Stroke, err = get(l.Stroke); err != nil {
		return nil, err
	}
	if lg.Shape, err = get(l.Shape); err != nil {
		return nil, err
	}
	if lg.Size, err = get(l.Size); err != nil {
		return nil, err
	}
	if lg.StrokeDash, err = get(l.StrokeDash); err != nil {
		return nil, err
	}
	main := lg.Fill
	for _, sc := range []scale.Scale{lg.Stroke, lg.Shape, lg.Size, lg.StrokeDash} {
		if main == nil {
			main = sc
		}
	}
	if main == nil {
		return nil, nil
	}
	if s.namedOnChart(l, lg) {
		// The ends of the lines name each series, so the legend says
		// nothing more (docs/STYLE.md).
		return nil, nil
	}
	if lg.Title, err = s.titleText(l.Title); err != nil {
		return nil, err
	}
	if !l.Values.IsZero() {
		v, err := s.value(l.Values)
		if err != nil {
			return nil, err
		}
		if list, ok := v.([]any); ok {
			lg.Values = list
		}
	}
	symbol, err := s.str(l.SymbolType, configStr(s.rt.configObject("legend"), "symbolType", ""))
	if err != nil {
		return nil, err
	}
	lg.Line = symbol == "stroke"
	typ := l.Type
	color := lg.Fill
	if color == nil {
		color = lg.Stroke
	}
	lg.Gradient = typ == "gradient" || (typ == "" && color != nil && !scale.Discrete(color.Type()) && scale.Thresholds(color) == nil && lg.Size == nil)
	if lg.GradientLength, err = s.num(l.GradientLength, configNum(s.rt.configObject("legend"), "gradientLength", 0)); err != nil {
		return nil, err
	}
	if lg.Format, err = s.labelFormat(main, l.Format, l.FormatType, nil); err != nil {
		return nil, err
	}
	cfg := s.rt.configObject("legend")
	place := legendPlace{}
	if place.orient, err = s.str(l.Orient, configStr(cfg, "orient", "")); err != nil {
		return nil, err
	}
	if place.offset, err = s.num(l.Offset, configNum(cfg, "offset", guideGap)); err != nil {
		return nil, err
	}
	if place.x, err = s.num(l.LegendX, 0); err != nil {
		return nil, err
	}
	if place.y, err = s.num(l.LegendY, 0); err != nil {
		return nil, err
	}
	// A legend at the side or in a corner is a column, and one above or
	// below the plot is a row, as in Vega.
	dir := "vertical"
	if place.orient == "" || place.orient == "top" || place.orient == "bottom" {
		dir = "horizontal"
	}
	if lg.Direction, err = s.str(l.Direction, configStr(cfg, "direction", dir)); err != nil {
		return nil, err
	}
	columns, err := s.num(l.Columns, configNum(cfg, "columns", 0))
	if err != nil {
		return nil, err
	}
	lg.Columns = int(math.Max(0, columns))
	if lg.Direction == "horizontal" {
		w, _ := s.rt.root.size()
		lg.Width = w
	}
	m := s.rt.style().Legend(lg)
	styleGuide(&m, s.guideProps(cfg, l.Extra), legendStyleParts)
	if err := s.guideEncode(&m, l.Encode, legendParts, path, "a legend"); err != nil {
		return nil, err
	}
	if l.ZIndex != nil {
		m.ZIndex = *l.ZIndex
	}
	m.Items[0].Datum = place
	return &m, nil
}

// legendPlace is where a legend goes, which placeGuides reads: its orient,
// its offset from the plot area, and its place for the orient none.
type legendPlace struct {
	orient       string
	offset, x, y float64
}

// legendParts and axisEncodeParts are the roles of the parts of a legend
// and of an axis that the encode of a guide names. The look of zuzu draws
// no ticks on an axis (docs/STYLE.md), so an encode of ticks has nothing to
// style.
var (
	legendParts = map[string]string{
		"legend": "legend", "title": "legend-title", "labels": "legend-label", "symbols": "legend-symbol",
		"gradient": "legend-gradient", "entries": "legend-entry",
	}
	axisEncodeParts = map[string]string{
		"axis": "axis", "title": "axis-title", "labels": "axis-label", "grid": "axis-grid",
		"domain": "axis-domain", "ticks": "",
	}
)

// guideEncode applies the encode of a legend or an axis to its parts, with
// the datum of each item, which holds its value and its label. The layout
// places the parts, so an encoding of a place goes in the report.
func (s *scope) guideEncode(m *scene.Mark, enc vega.GuideEncode, parts map[string]string, path, kind string) error {
	for _, name := range slices.Sorted(maps.Keys(enc)) {
		el := enc[name]
		role, ok := parts[name]
		if !ok {
			if err := s.rt.issue(vega.ErrUnsupported, path+"/encode/"+name, "the encode of the part %s of %s", name, kind); err != nil {
				return err
			}
			continue
		}
		if role == "" {
			continue
		}
		sets := vega.Encode{}
		for _, set := range slices.Sorted(maps.Keys(el.Sets)) {
			kept := vega.Props{}
			for _, k := range slices.Sorted(maps.Keys(el.Sets[set])) {
				switch k {
				case "x", "y", "x2", "y2", "xc", "yc", "width", "height":
					if err := s.rt.issue(vega.ErrUnsupported, path+"/encode/"+name+"/"+set, "the encode of %s of the %s of %s", k, name, kind); err != nil {
						return err
					}
					continue
				}
				kept[k] = el.Sets[set][k]
			}
			sets[set] = kept
		}
		var err error
		var walk func(m *scene.Mark)
		walk = func(m *scene.Mark) {
			for i := range m.Items {
				it := &m.Items[i]
				if m.Role == role && err == nil {
					st := &itemState{it: it, set: map[string]bool{}}
					err = s.encodeItem(sets, st, it.Datum, "/encode/"+name)
				}
				for j := range it.Items {
					walk(&it.Items[j])
				}
			}
		}
		walk(m)
		if err != nil {
			return err
		}
	}
	return nil
}

// buildTitle builds the title of a scope. A title of a group stands beyond
// the guides that the group already drew, such as the labels of its axis.
func (s *scope) buildTitle(_ string, drawn []scene.Mark) ([]scene.Mark, error) {
	t := s.spec.Title
	if t == nil {
		return nil, nil
	}
	lines, err := s.lines(t.Text)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}
	sub, err := s.lines(t.Subtitle)
	if err != nil {
		return nil, err
	}
	w, h := s.size()
	lt := &layout.Title{Text: lines, Subtitle: sub, Width: w, Height: h}
	if lt.Orient, err = s.str(t.Orient, "top"); err != nil {
		return nil, err
	}
	kind := "title"
	for _, st := range t.Style.List {
		switch st {
		case "guide-label", "group-title":
			kind = "label"
		case "guide-title":
			kind = "small"
		}
	}
	lt.Kind = kind
	def := "start"
	if kind != "title" {
		def = "middle"
	}
	if lt.Anchor, err = s.str(t.Anchor, def); err != nil {
		return nil, err
	}
	off := 4.0
	if kind == "title" {
		off = 10
	}
	if lt.Offset, err = s.num(t.Offset, off); err != nil {
		return nil, err
	}
	if s.parent != nil {
		b := layout.Empty()
		st := s.rt.style()
		for i := range drawn {
			if drawn[i].Role != "axis" {
				continue
			}
			mb := st.MarkBounds(&drawn[i])
			if !mb.IsEmpty() {
				b.X1, b.Y1 = math.Min(b.X1, mb.X1), math.Min(b.Y1, mb.Y1)
				b.X2, b.Y2 = math.Max(b.X2, mb.X2), math.Max(b.Y2, mb.Y2)
			}
		}
		if !b.IsEmpty() {
			switch lt.Orient {
			case "left":
				lt.Offset += -math.Min(0, b.X1)
			case "right":
				lt.Offset += math.Max(0, b.X2-w)
			case "bottom":
				lt.Offset += math.Max(0, b.Y2-h)
			default:
				lt.Offset += -math.Min(0, b.Y1)
			}
		}
	}
	m := s.rt.style().Title(lt)
	if kind == "title" && s.parent == nil {
		m.Name = "title"
		props := s.guideProps(s.rt.configObject("title"), t.Extra)
		for k, v := range map[string]vega.Value{"color": t.Color, "font": t.Font, "fontSize": t.FontSize} {
			if !v.IsZero() {
				x, err := s.value(v)
				if err != nil {
					return nil, err
				}
				props[k] = x
			}
		}
		styleGuide(&m, props, titleParts)
	}
	return []scene.Mark{m}, nil
}

// titleText reads the text of a title: a string or an array of lines,
// which it joins with spaces.
func (s *scope) titleText(v vega.Value) (string, error) {
	lines, err := s.lines(v)
	return strings.Join(lines, " "), err
}

// lines reads a text that is a string or an array of lines.
func (s *scope) lines(v vega.Value) ([]string, error) {
	if v.IsZero() {
		return nil, nil
	}
	x, err := s.value(v)
	if err != nil {
		return nil, err
	}
	switch t := x.(type) {
	case nil:
		return nil, nil
	case []any:
		out := make([]string, len(t))
		for i, e := range t {
			out[i] = expr.String(e)
		}
		return out, nil
	case string:
		if t == "" {
			return nil, nil
		}
		return []string{t}, nil
	}
	if expr.IsUndefined(x) {
		return nil, nil
	}
	return []string{expr.String(x)}, nil
}

// str reads a property that is a string or a signal.
func (s *scope) str(v vega.Value, def string) (string, error) {
	if v.IsZero() {
		return def, nil
	}
	x, err := s.value(v)
	if err != nil || x == nil || expr.IsUndefined(x) {
		return def, err
	}
	return expr.String(x), nil
}

// num reads a property that is a number, a signal or a value reference,
// such as an offset of an axis that a scale gives:
// {"scale": "x", "value": "a"}.
func (s *scope) num(v vega.Value, def float64) (float64, error) {
	if v.IsZero() {
		return def, nil
	}
	if v.Signal == "" && v.JSON.Kind() == '{' {
		x, err := s.numberRef(v, nil, nil)
		if err != nil {
			return def, err
		}
		return number(x, def), nil
	}
	x, err := s.value(v)
	if err != nil {
		return def, err
	}
	return number(x, def), nil
}

// tickCount reads the tickCount of an axis: a number, or an interval of
// time with a step, {"interval": "week", "step": 2}, which sets the values
// of the ticks of a time scale.
func (s *scope) tickCount(v vega.Value, sc scale.Scale, ax *layout.Axis) (float64, error) {
	if v.IsZero() {
		return 0, nil
	}
	x, err := s.value(v)
	if err != nil {
		return 0, err
	}
	m, ok := x.(map[string]any)
	if !ok {
		return number(x, 0), nil
	}
	unit := expr.String(m["interval"])
	step := int(number(m["step"], 1))
	dom := sc.Domain()
	if len(dom) < 2 {
		return 0, nil
	}
	t0, ok0 := dom[0].(time.Time)
	t1, ok1 := dom[len(dom)-1].(time.Time)
	ticks, ok := scale.IntervalTicks(t0, t1, unit, step)
	if !ok0 || !ok1 || !ok {
		return 0, s.rt.issue(vega.ErrUnsupported, "axis", "the tickCount %v of a scale of type %s", m, sc.Type())
	}
	ax.Values = make([]any, len(ticks))
	for i, t := range ticks {
		ax.Values[i] = t
	}
	return float64(len(ticks)), nil
}

// boolean reads a property that is a boolean or a signal.
func (s *scope) boolean(v vega.BoolValue, def bool) (bool, error) {
	switch v.Kind {
	case vega.Literal:
		return v.Bool, nil
	case vega.SignalRef:
		x, err := s.eval(v.Signal, nil)
		return expr.Truthy(x), err
	}
	return def, nil
}

// placeGuides puts the legends and the title of a scope around what the
// scope draws. A legend with no orient, or the orient top, goes in a row
// above the plot area, in the order of docs/STYLE.md: the title, then the
// legends, then the plot area. A legend with the orient left, right or
// bottom goes beside or below the plot and its axes, one in a corner goes
// inside the plot area, and one with the orient none goes at its legendX
// and legendY.
func (s *scope) placeGuides(marks []scene.Mark) {
	st := s.rt.style()
	w, h := s.size()
	body := layout.Empty()
	for i := range marks {
		m := &marks[i]
		if m.Role == "legend" || m.Role == "title" {
			continue
		}
		if b := st.MarkBounds(m); !b.IsEmpty() {
			body.X1, body.Y1 = math.Min(body.X1, b.X1), math.Min(body.Y1, b.Y1)
			body.X2, body.Y2 = math.Max(body.X2, b.X2), math.Max(body.Y2, b.Y2)
		}
	}
	top := math.Min(0, body.Y1)
	if body.IsEmpty() {
		body = layout.Bounds{X2: w, Y2: h}
	}
	// The next place of each side and corner, as legends stack there.
	left, right, bottom := 0.0, 0.0, math.Max(h, body.Y2)
	corner := map[string]float64{}
	// The legends of the top stack upward from the top of the plot area.
	for i := len(marks) - 1; i >= 0; i-- {
		m := &marks[i]
		if m.Role != "legend" || len(m.Items) == 0 {
			continue
		}
		b := st.MarkBounds(m)
		if b.IsEmpty() {
			continue
		}
		p, _ := m.Items[0].Datum.(legendPlace)
		it := &m.Items[0]
		switch p.orient {
		case "", "top":
			y := top - guideGap - b.Y2
			it.Y = y
			top = y + b.Y1
		}
	}
	for i := range marks {
		m := &marks[i]
		if m.Role != "legend" || len(m.Items) == 0 {
			continue
		}
		b := st.MarkBounds(m)
		if b.IsEmpty() {
			continue
		}
		p, _ := m.Items[0].Datum.(legendPlace)
		it := &m.Items[0]
		switch p.orient {
		case "left":
			it.X, it.Y = math.Min(0, body.X1)-p.offset-b.X2, left-b.Y1
			left += b.Y2 - b.Y1 + guideGap
		case "right":
			it.X, it.Y = math.Max(w, body.X2)+p.offset-b.X1, right-b.Y1
			right += b.Y2 - b.Y1 + guideGap
		case "bottom":
			it.X, it.Y = -b.X1, bottom+p.offset-b.Y1
			bottom += p.offset + b.Y2 - b.Y1
		case "top-left", "top-right", "bottom-left", "bottom-right":
			it.X = p.offset - b.X1
			if strings.HasSuffix(p.orient, "right") {
				it.X = w - p.offset - b.X2
			}
			at := corner[p.orient]
			if strings.HasPrefix(p.orient, "top") {
				it.Y = p.offset + at - b.Y1
			} else {
				it.Y = h - p.offset - at - b.Y2
			}
			corner[p.orient] = at + b.Y2 - b.Y1 + guideGap/2
		case "none":
			it.X, it.Y = p.x, p.y
		}
	}
	for i := range marks {
		m := &marks[i]
		if m.Role != "title" || m.Name != "title" || len(m.Items) == 0 {
			continue
		}
		b := st.MarkBounds(m)
		if b.IsEmpty() {
			continue
		}
		m.Items[0].Y = top - guideGap/2 - b.Y2
	}
}

// configNum returns a number of the config of a guide, or def.
func configNum(cfg map[string]any, key string, def float64) float64 {
	if f, ok := cfg[key].(float64); ok {
		return f
	}
	return def
}

// configStr returns a string of the config of a guide, or def.
func configStr(cfg map[string]any, key, def string) string {
	if v, ok := cfg[key].(string); ok {
		return v
	}
	return def
}

// namedOnChart reports whether a legend of the stroke alone has each of its
// values named at the end of a line.
func (s *scope) namedOnChart(l *vega.Legend, lg *layout.Legend) bool {
	if lg.Stroke == nil || lg.Fill != nil || lg.Shape != nil || lg.Size != nil || lg.StrokeDash != nil || !l.Values.IsZero() {
		return false
	}
	named := s.rt.endLabels[l.Stroke]
	dom := lg.Stroke.Domain()
	if len(named) == 0 || len(dom) == 0 {
		return false
	}
	for _, v := range dom {
		if !named[expr.String(v)] {
			return false
		}
	}
	return true
}
