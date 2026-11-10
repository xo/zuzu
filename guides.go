package zuzu

import (
	"errors"
	"fmt"
	"math"
	"strconv"
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
		m, err := s.buildAxis(a)
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
func (s *scope) buildAxis(a *vega.Axis) (*scene.Mark, error) {
	sc, err := s.scale(a.Scale)
	if err != nil {
		return nil, err
	}
	orient, err := s.str(a.Orient, "bottom")
	if err != nil {
		return nil, err
	}
	w, h := s.size()
	ax := &layout.Axis{Orient: orient, Scale: sc, Length: w, Cross: h}
	if orient == "left" || orient == "right" {
		ax.Length, ax.Cross = h, w
	}
	ax.Grid = a.Grid != nil && *a.Grid
	if ax.Labels, err = s.boolean(a.Labels, true); err != nil {
		return nil, err
	}
	domain, err := s.boolean(a.Domain, true)
	if err != nil {
		return nil, err
	}
	// The look of zuzu draws a base line only along a band axis (D12).
	ax.Domain = domain && scale.Discrete(sc.Type())
	if ax.Title, err = s.titleText(a.Title); err != nil {
		return nil, err
	}
	count, err := s.num(a.TickCount, 0)
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
	if ax.LabelAngle, err = s.num(a.LabelAngle, 0); err != nil {
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
	if sc.Type() == "band" {
		bp = 0.5
	}
	if ax.BandPosition, err = s.num(a.BandPosition, bp); err != nil {
		return nil, err
	}
	if ax.Format, err = s.labelFormat(sc, a.Format, a.FormatType, ax); err != nil {
		return nil, err
	}
	m := s.rt.style().Axis(ax)
	if a.ZIndex != nil {
		m.ZIndex = *a.ZIndex
	}
	return &m, nil
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
	isTime := strings.HasSuffix(sc.Type(), "time") || strings.HasSuffix(sc.Type(), "utc")
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
		f, err := format.Number(spec, *s.rt.locale)
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
	switch {
	case isTime:
		return timeLabel(*s.rt.locale, loc), nil
	case scale.Discrete(sc.Type()):
		return func(v any) string {
			if v == nil {
				return "null"
			}
			return expr.String(v)
		}, nil
	}
	return s.numberLabel(sc, ax), nil
}

// numberLabel returns a format of numbers with a thousands separator and
// as many decimals as the step between the ticks needs.
func (s *scope) numberLabel(sc scale.Scale, ax *layout.Axis) func(any) string {
	dom := sc.Domain()
	step := 1.0
	if len(dom) >= 2 {
		count := 10.0
		if ax != nil {
			count = math.Min(layout.MaxLabels, math.Max(2, ax.Count))
			if ax.Count <= 0 {
				count = math.Max(2, math.Min(layout.MaxLabels, ax.Length/40))
			}
		}
		step = scale.TickStep(expr.Number(dom[0]), expr.Number(dom[len(dom)-1]), count)
	}
	decimals := 0
	if step > 0 && !math.IsNaN(step) && !math.IsInf(step, 0) {
		str := strconv.FormatFloat(step, 'f', -1, 64)
		if _, frac, ok := strings.Cut(str, "."); ok {
			decimals = min(len(frac), 6)
		}
	}
	f, _ := format.Number(",."+strconv.Itoa(decimals)+"f", *s.rt.locale)
	return func(v any) string {
		if _, ok := v.(string); ok {
			return expr.String(v)
		}
		return f(expr.Number(v))
	}
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
		m, err := s.buildLegend(l)
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
func (s *scope) buildLegend(l *vega.Legend) (*scene.Mark, error) {
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
	main := lg.Fill
	for _, sc := range []scale.Scale{lg.Stroke, lg.Shape, lg.Size} {
		if main == nil {
			main = sc
		}
	}
	if main == nil {
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
	symbol, err := s.str(l.SymbolType, "")
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
	if lg.GradientLength, err = s.num(l.GradientLength, 0); err != nil {
		return nil, err
	}
	if lg.Format, err = s.labelFormat(main, l.Format, l.FormatType, nil); err != nil {
		return nil, err
	}
	w, _ := s.rt.root.size()
	lg.Width = w
	m := s.rt.style().Legend(lg)
	if l.ZIndex != nil {
		m.ZIndex = *l.ZIndex
	}
	return &m, nil
}

// buildTitle builds the title of a scope.
func (s *scope) buildTitle(_ string) ([]scene.Mark, error) {
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
	m := s.rt.style().Title(lt)
	if kind == "title" && s.parent == nil {
		m.Name = "title"
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

// num reads a property that is a number or a signal.
func (s *scope) num(v vega.Value, def float64) (float64, error) {
	if v.IsZero() {
		return def, nil
	}
	x, err := s.value(v)
	if err != nil {
		return def, err
	}
	return number(x, def), nil
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

// placeGuides puts the legends and the title of a scope above what the
// scope draws, in the order of docs/STYLE.md: the title, then the legends,
// then the plot area.
func (s *scope) placeGuides(marks []scene.Mark) {
	st := s.rt.style()
	top := 0.0
	for i := range marks {
		m := &marks[i]
		if m.Role == "legend" || m.Role == "title" {
			continue
		}
		if b := st.MarkBounds(m); !b.IsEmpty() {
			top = math.Min(top, b.Y1)
		}
	}
	// The legends stack upward from the top of the plot area.
	for i := len(marks) - 1; i >= 0; i-- {
		m := &marks[i]
		if m.Role != "legend" || len(m.Items) == 0 {
			continue
		}
		b := st.MarkBounds(m)
		if b.IsEmpty() {
			continue
		}
		y := top - guideGap - b.Y2
		m.Items[0].Y = y
		top = y + b.Y1
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
