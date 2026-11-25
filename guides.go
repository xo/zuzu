package zuzu

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/format"
	"github.com/xo/zuzu/internal/shape"
	"github.com/xo/zuzu/layout"
	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/svg"
	"github.com/xo/zuzu/theme"
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

// buildAxes builds the axes of a scope, and moves the title of each axis
// clear of the labels of the others (D46).
func (s *scope) buildAxes(path string) ([]scene.Mark, error) {
	var out []scene.Mark
	var placed []*layout.Axis
	for i := range s.spec.Axes {
		a := &s.spec.Axes[i]
		p := fmt.Sprintf("%s/axes/%d", path, i)
		m, ax, err := s.buildAxis(a, p)
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
			// An encode of the title can place it, and then it stays.
			if _, ok := a.Encode["title"]; ok {
				ax = nil
			}
			placed = append(placed, ax)
		}
	}
	s.rt.style().ClearTitles(out, placed)
	return out, nil
}

// buildAxis resolves an axis and draws it. It returns the mark of the axis
// and the axis as the layout drew it.
func (s *scope) buildAxis(a *vega.Axis, path string) (*scene.Mark, *layout.Axis, error) {
	sc, err := s.scale(a.Scale)
	if err != nil {
		return nil, nil, err
	}
	orient, err := s.str(a.Orient, "bottom")
	if err != nil {
		return nil, nil, err
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
	// The config of the axis can hide the labels too.
	showLabels, ok := cfg["labels"].(bool)
	if !ok {
		showLabels = true
	}
	if ax.Labels, err = s.boolean(a.Labels, showLabels); err != nil {
		return nil, nil, err
	}
	// The config of the axis can hide the domain, as it can the grid.
	showDomain, ok := cfg["domain"].(bool)
	if !ok {
		showDomain = true
	}
	domain, err := s.boolean(a.Domain, showDomain)
	if err != nil {
		return nil, nil, err
	}
	ax.Domain = domain
	if ax.Title, err = s.titleText(a.Title); err != nil {
		return nil, nil, err
	}
	count, err := s.tickCount(a.TickCount, sc, ax)
	if err != nil {
		return nil, nil, err
	}
	ax.Count = count
	if ax.MinStep, err = s.num(a.TickMinStep, 0); err != nil {
		return nil, nil, err
	}
	// An axis whose spec gives no tickCount chooses the step of its
	// ticks so that both ends of the domain get a tick (D46). The
	// compiler of Vega-Lite gives each axis of numbers the tickCount of
	// one tick each 40 pixels, which is the count of the look, so that
	// tickCount counts as none.
	ax.EndTicks = a.Values.IsZero() && (a.TickCount.IsZero() || lookCount(a.TickCount))
	if !a.Values.IsZero() {
		v, err := s.value(a.Values)
		if err != nil {
			return nil, nil, err
		}
		if l, ok := v.([]any); ok {
			ax.Values = l
		}
	}
	if ax.LabelAngle, err = s.num(a.LabelAngle, configNum(cfg, "labelAngle", 0)); err != nil {
		return nil, nil, err
	}
	if ax.Offset, err = s.num(a.Offset, 0); err != nil {
		return nil, nil, err
	}
	overlap := !scale.Discrete(sc.Type())
	if !a.LabelOverlap.IsZero() {
		v, err := s.value(a.LabelOverlap)
		if err != nil {
			return nil, nil, err
		}
		overlap = expr.Truthy(v)
	}
	ax.Overlap = overlap
	bp := 0.0
	if sc.Type() == "band" || sc.Type() == scale.Trading {
		bp = 0.5
	}
	// The config of the axis, such as config.axisBand, can give the band
	// position too.
	if ax.BandPosition, err = s.num(a.BandPosition, configNum(cfg, "bandPosition", bp)); err != nil {
		return nil, nil, err
	}
	// labelFlush is true for a distance of 1, or a distance, and none by
	// default, as in Vega.
	ax.LabelFlush = -1
	if flush, ok := cfg["labelFlush"]; ok || !a.LabelFlush.IsZero() {
		v := flush
		if !a.LabelFlush.IsZero() {
			if v, err = s.value(a.LabelFlush); err != nil {
				return nil, nil, err
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
		return nil, nil, err
	}
	if ax.Format, err = s.labelFormat(sc, a.Format, a.FormatType, ax); err != nil {
		return nil, nil, err
	}
	if ax.LabelAlign, err = s.str(a.LabelAlign, configStr(cfg, "labelAlign", "")); err != nil {
		return nil, nil, err
	}
	if ax.LabelBaseline, err = s.str(a.LabelBase, configStr(cfg, "labelBaseline", "")); err != nil {
		return nil, nil, err
	}
	props := s.guideProps(cfg, a.Extra)
	// The key zuzu.labelTurn of the compiler of Vega-Lite turns the labels
	// only when one does not fit its band, and a labelAngle of the axis or
	// of its config wins over it (D58 part 4).
	if _, ok := cfg["labelAngle"]; !ok && a.LabelAngle.IsZero() {
		ax.LabelTurn, _ = propNum(props, theme.LabelTurn)
		ax.LabelFontSize, _ = propNum(props, "labelFontSize")
	}
	if err := s.ticks(ax, a, cfg, props); err != nil {
		return nil, nil, err
	}
	axisProps(ax, props)
	if err := s.tickBand(ax, props, path); err != nil {
		return nil, nil, err
	}
	m := s.rt.style().Axis(ax)
	// The config and the axis style its parts, over the look of zuzu.
	s.styleGuide(&m, props, axisParts)
	if err := s.guideEncode(&m, a.Encode, axisEncodeParts, axisFrame(ax), path, "an axis"); err != nil {
		return nil, nil, err
	}
	if a.ZIndex != nil {
		m.ZIndex = *a.ZIndex
	}
	return &m, ax, nil
}

// ticks reads the ticks of an axis and the padding of its labels. As in
// Vega, an axis has ticks unless the axis or its config hides them, and a
// tick is tickSize long. A label is labelPadding past the end of its tick,
// or past the line of the axis without ticks. The size of a tick and the
// padding of a label take the sizes of the look when the axis and its
// config give none (D46, docs/STYLE.md).
func (s *scope) ticks(ax *layout.Axis, a *vega.Axis, cfg, props map[string]any) error {
	cfgTicks, ok := cfg["ticks"].(bool)
	if !ok {
		cfgTicks = true
	}
	ticks, err := s.boolean(a.Ticks, cfgTicks)
	if err != nil {
		return err
	}
	ax.Ticks = ticks
	ax.TickSize = theme.TickSize
	if size, ok := propNum(props, "tickSize"); ok {
		ax.TickSize = size
	}
	if _, cfgPad := cfg["labelPadding"]; !a.LabelPadding.IsZero() || cfgPad {
		pad, err := s.num(a.LabelPadding, configNum(cfg, "labelPadding", theme.LabelPadding))
		if err != nil {
			return err
		}
		ax.LabelPadding = &pad
	}
	return nil
}

// tickBand reads tickExtra, tickOffset and tickRound of an axis, and
// tickBand, which sets the band position, the extra tick and the offset of
// an axis of bands, as the axis of Vega documents it: center puts the
// ticks in the middle of each band, and extent puts them at the edges of
// the bands. The properties come from the config with the axis over it.
func (s *scope) tickBand(ax *layout.Axis, props map[string]any, path string) error {
	if v, ok := props["tickExtra"].(bool); ok {
		ax.TickExtra = v
	}
	if v, ok := propNum(props, "tickOffset"); ok {
		ax.TickOffset = v
	}
	if v, ok := props["tickRound"].(bool); ok {
		ax.TickRound = v
	}
	band, ok := props["tickBand"]
	if !ok || band == nil {
		return nil
	}
	switch band {
	case "center":
		ax.BandPosition, ax.TickExtra, ax.TickOffset = 0.5, false, 0
	case "extent":
		ax.BandPosition, ax.TickExtra, ax.TickOffset = 1, true, 0
	default:
		// A tickBand that is not center or extent, or a signal of the
		// datum, is not drawn, so the report names it (hard rule 4).
		return s.rt.issue(vega.ErrUnsupported, path+"/tickBand", "the tickBand %v of an axis", band)
	}
	return nil
}

// propNum returns a property of a guide that is a finite number, and
// whether the guide has it.
func propNum(props map[string]any, key string) (float64, bool) {
	f, ok := props[key].(float64)
	if !ok || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// axisProps reads the place, the angle, the align, the baseline, the
// anchor and the padding of the title of an axis, and the offset of its
// labels, from its properties, the config with the axis over it. A property
// that is not there keeps the one of the look.
func axisProps(ax *layout.Axis, props map[string]any) {
	num := func(k string) *float64 {
		if f, ok := propNum(props, k); ok {
			return &f
		}
		return nil
	}
	ax.TitleX, ax.TitleY, ax.TitleAngle = num("titleX"), num("titleY"), num("titleAngle")
	ax.TitlePadding = num("titlePadding")
	ax.TitleAlign = configStr(props, "titleAlign", "")
	ax.TitleBaseline = configStr(props, "titleBaseline", "")
	// A titleAnchor of null, or of no known value, leaves the place to the
	// look.
	if anchor := configStr(props, "titleAnchor", ""); anchor == "start" || anchor == "middle" || anchor == "end" {
		ax.TitleAnchor = anchor
	}
	ax.LabelOffset, _ = propNum(props, "labelOffset")
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
	if spec != "" && (scale.Discrete(sc.Type()) || !layout.Linear(sc.Type()) || len(dom) < 2) {
		return format.Number(spec, loc)
	}
	lo, hi := 0.0, 1.0
	if len(dom) >= 2 {
		lo, hi = expr.Number(dom[0]), expr.Number(dom[len(dom)-1])
	}
	step := scale.TickStep(lo, hi, count)
	if ax != nil {
		step = ax.TickStep(lo, hi)
	}
	return format.Ticks(spec, lo, hi, step, loc)
}

// lookCount reports whether a tickCount is the one that the compiler of
// Vega-Lite gives an axis of numbers by default: one tick each 40 pixels of
// the signal of a width or a height, such as ceil(child_width/40).
func lookCount(v vega.Value) bool {
	name, ok1 := strings.CutPrefix(v.Signal, "ceil(")
	name, ok2 := strings.CutSuffix(name, "/40)")
	if !ok1 || !ok2 {
		return false
	}
	if !strings.HasSuffix(name, "width") && !strings.HasSuffix(name, "height") {
		return false
	}
	return !strings.ContainsFunc(name, func(r rune) bool {
		return r != '_' && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9')
	})
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

// legendValues are the values that some properties of a legend can take.
// The report names any other value, which the legend does not draw (hard
// rule 4).
var legendValues = map[string][]string{
	"titleOrient": {"top", "bottom", "left", "right"},
	"titleAnchor": {"start", "middle", "end"},
	"gridAlign":   {"each", "all", "none"},
}

// legendUnread reports each property of a legend whose value zuzu does not
// draw, so that no property drops without a word (hard rule 4): a side, an
// anchor or an align of the grid that Vega does not name, and a tickCount
// that is not a number, such as the name of a unit of time.
func (s *scope) legendUnread(props map[string]any, path string) error {
	for _, k := range slices.Sorted(maps.Keys(legendValues)) {
		v, ok := props[k]
		if !ok || v == nil {
			continue
		}
		if str, ok := v.(string); ok && slices.Contains(legendValues[k], str) {
			continue
		}
		if err := s.rt.issue(vega.ErrUnsupported, path+"/"+k, "the %s %v of a legend", k, v); err != nil {
			return err
		}
	}
	if v, ok := props["tickCount"]; ok && v != nil {
		if _, ok := propNum(props, "tickCount"); !ok {
			if err := s.rt.issue(vega.ErrUnsupported, path+"/tickCount", "the tickCount %v of a legend", v); err != nil {
				return err
			}
		}
	}
	return nil
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
	if lg.StrokeWidth, err = get(l.StrokeWidth); err != nil {
		return nil, err
	}
	if lg.Opacity, err = get(l.Opacity); err != nil {
		return nil, err
	}
	main := lg.Fill
	for _, sc := range []scale.Scale{lg.Stroke, lg.Shape, lg.Size, lg.StrokeDash, lg.StrokeWidth, lg.Opacity} {
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
	if !lg.Line {
		lg.SymbolType = symbol
	}
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
	if lg.ClipHeight, err = s.num(l.ClipHeight, configNum(cfg, "clipHeight", 0)); err != nil {
		return nil, err
	}
	props := s.guideProps(cfg, l.Extra)
	// The properties that the legend holds in fields of its own go in
	// props too, over the config.
	for k, v := range map[string]vega.Value{"titleOrient": l.TitleOrient, "labelOverlap": l.LabelOverlap} {
		if v.IsZero() {
			continue
		}
		x, err := s.value(v)
		if err != nil {
			return nil, err
		}
		props[k] = x
	}
	if err := s.legendUnread(props, path); err != nil {
		return nil, err
	}
	legendProps(lg, props)
	s.legendMarks(l, lg)
	encodeSymbols(l, lg)
	// A direction or a limit that the spec sets is a property of the
	// entries, which nested circles cannot show (docs/STYLE.md).
	_, limited := props["symbolLimit"]
	_, directed := cfg["direction"]
	lg.Entries = limited || directed || !l.Direction.IsZero()
	if lg.Direction == "horizontal" {
		// The row wraps at the width of the plot area of the group that
		// holds the legend (docs/STYLE.md). A group that sets no width
		// has no plot area of its own, and its row wraps at the width of
		// the top level.
		w, _ := s.size()
		if w <= 0 {
			w, _ = s.rt.root.size()
		}
		lg.Width = w
	}
	// An encode of the place of a part of the legend applies in the frame
	// of an entry, and the layout places the entries after the encode, as
	// vega.js does.
	lg.EntryGroups = encodesLegendPlace(l.Encode)
	m := s.rt.style().Legend(lg)
	s.styleGuide(&m, props, legendStyleParts)
	parts, frame := legendParts, guideFrame(nil)
	if lg.EntryGroups {
		frame = entryFrame
		if lg.DrawsGradient() {
			parts = gradientLegendParts
		}
	}
	if err := s.guideEncode(&m, l.Encode, parts, frame, path, "a legend"); err != nil {
		return nil, err
	}
	if lg.EntryGroups {
		s.rt.style().PlaceEntries(&m, lg)
	}
	if off, ok := propNum(props, "symbolDashOffset"); ok && off != 0 {
		shiftDashes(&m, "legend-symbol", off)
	}
	if l.ZIndex != nil {
		m.ZIndex = *l.ZIndex
	}
	m.Items[0].Datum = place
	return &m, nil
}

// legendProps reads the properties of a legend that the spec or the config
// of the legend sets and that win over the look (D46, docs/STYLE.md): the
// limits of its entries and its labels, the size of its symbols, the space
// between its rows and its columns, and the width of its gradient.
func legendProps(lg *layout.Legend, props map[string]any) {
	num := func(name string) (float64, bool) {
		f, ok := props[name].(float64)
		return f, ok && f >= 0 && !math.IsInf(f, 0)
	}
	if f, ok := num("symbolLimit"); ok && f >= 1 {
		lg.SymbolLimit = int(f)
	}
	if f, ok := num("labelLimit"); ok && f > 0 {
		lg.LabelLimit = f
	}
	if f, ok := num("symbolSize"); ok && f > 0 {
		lg.SymbolSize = f
	}
	if f, ok := num("gradientThickness"); ok && f > 0 {
		lg.GradientThickness = f
	}
	if f, ok := num("rowPadding"); ok {
		lg.RowPadding = &f
	}
	if f, ok := num("columnPadding"); ok {
		lg.ColumnPadding = &f
	}
	str := func(name string, values ...string) string {
		v, _ := props[name].(string)
		if len(values) > 0 && !slices.Contains(values, v) {
			return ""
		}
		return v
	}
	lg.TitleOrient = str("titleOrient", legendValues["titleOrient"]...)
	lg.TitleAnchor = str("titleAnchor", legendValues["titleAnchor"]...)
	lg.GridAlign = str("gridAlign", legendValues["gridAlign"]...)
	lg.TitleAlign, lg.TitleBaseline = str("titleAlign"), str("titleBaseline")
	lg.LabelAlign, lg.LabelBaseline = str("labelAlign"), str("labelBaseline")
	lg.FillColor, lg.StrokeColor = str("fillColor"), str("strokeColor")
	lg.SymbolBaseFill, lg.SymbolBaseStroke = str("symbolBaseFillColor"), str("symbolBaseStrokeColor")
	if f, ok := propNum(props, "titlePadding"); ok {
		lg.TitlePadding = &f
	}
	if f, ok := propNum(props, "labelOffset"); ok {
		lg.LabelOffset = &f
	}
	lg.TitleLimit, _ = num("titleLimit")
	lg.TitleLineHeight, _ = num("titleLineHeight")
	lg.Padding, _ = num("padding")
	lg.CornerRadius, _ = num("cornerRadius")
	lg.LabelSeparation, _ = num("labelSeparation")
	lg.SymbolOffset, _ = propNum(props, "symbolOffset")
	if f, ok := num("tickCount"); ok && f >= 1 {
		lg.TickCount = f
	}
	lg.TickMinStep, _ = num("tickMinStep")
	// A labelOverlap of true, parity or greedy drops labels that overlap.
	if v, ok := props["labelOverlap"]; ok && v != nil {
		lg.LabelOverlap = expr.Truthy(v)
	}
}

// encodeSymbols reads the size and the shape that the encode of the
// symbols of a legend gives as a value, so that the layout of the entries
// keeps room for the symbols that the encode draws, as Vega lays out the
// entries after the encode.
func encodeSymbols(l *vega.Legend, lg *layout.Legend) {
	enc := l.Encode["symbols"].Sets
	if v, ok := encodeLiteral(enc, "size").(float64); ok && v > 0 && lg.Size == nil {
		lg.SymbolSize = v
	}
	if v, ok := encodeLiteral(enc, "shape").(string); ok && v != "" && lg.Shape == nil && !lg.Line {
		lg.SymbolType = v
	}
}

// encodeLiteral returns the value that an encode gives a property with no
// scale, no field and no signal, or nil. The update set wins over the enter
// set, and the last rule, which has no test, wins over the others.
func encodeLiteral(e vega.Encode, prop string) any {
	var out any
	for _, set := range encodeSets {
		for _, r := range e[set][prop] {
			if len(r.Value) == 0 || !r.Scale.IsZero() || r.Field.Name != "" || r.Signal != "" {
				continue
			}
			var v any
			if json.Unmarshal(r.Value, &v) == nil {
				out = v
			}
		}
	}
	return out
}

// shiftDashes starts the dash of each item of a role that far along it, as
// a dash offset of SVG does.
func shiftDashes(m *scene.Mark, role string, off float64) {
	for i := range m.Items {
		it := &m.Items[i]
		if m.Role == role && len(it.StrokeDash) > 0 {
			it.StrokeDash = shape.ShiftDash(it.StrokeDash, off)
		}
		for j := range it.Items {
			shiftDashes(&it.Items[j], role, off)
		}
	}
}

// scaleCall matches a signal that maps one value through a scale, such as
// scale('color', 'Normal Estimate').
var scaleCall = regexp.MustCompile(`^\s*scale\(\s*'([^']+)'\s*,\s*'([^']*)'\s*\)\s*$`)

// legendScaleProps are the properties of a mark that a legend names.
var legendScaleProps = map[string]bool{
	"fill": true, "stroke": true, "shape": true, "size": true, "strokeDash": true, "opacity": true,
}

// markRank orders the types of the marks that a legend can name, the one
// whose shape its symbols take first: a mark with an area, such as the box
// of a box plot, then a symbol, such as an outlier, then a line, then any
// other.
func markRank(typ string) int {
	switch typ {
	case "rect", "area", "arc", "shape", "path", "image":
		return 0
	case "symbol":
		return 1
	case "line", "rule", "trail":
		return 2
	}
	return 3
}

// legendMarks gives the symbols of a legend the look of the marks that it
// names, as docs/STYLE.md says: the shape of the mark that reads one of its
// scales, the first of the highest rank of markRank, a short line for a
// line, an outline when the mark has no fill, the color of the mark when no
// scale of the legend gives one, and a short line for each value that a
// line maps through the scale of color.
func (s *scope) legendMarks(l *vega.Legend, lg *layout.Legend) {
	names := map[string]bool{}
	for _, n := range []string{l.Fill, l.Stroke, l.Shape, l.Size, l.StrokeDash, l.Opacity} {
		if n != "" {
			names[n] = true
		}
	}
	var first *vega.Mark
	var walk func(ms []vega.Mark)
	walk = func(ms []vega.Mark) {
		for i := range ms {
			m := &ms[i]
			line := m.Type == "line" || m.Type == "trail" || m.Type == "rule"
			for _, set := range encodeSets {
				for _, prop := range slices.Sorted(maps.Keys(m.Encode[set])) {
					for _, r := range m.Encode[set][prop] {
						if sm := scaleCall.FindStringSubmatch(r.Signal); sm != nil && names[sm[1]] && line {
							if lg.LineValues == nil {
								lg.LineValues = map[string]bool{}
							}
							lg.LineValues[sm[2]] = true
						}
						if r.Scale.IsZero() || !legendScaleProps[prop] || (first != nil && markRank(m.Type) >= markRank(first.Type)) {
							continue
						}
						if name, err := s.str(r.Scale, ""); err == nil && names[name] {
							first = m
						}
					}
				}
			}
			walk(m.Marks)
		}
	}
	walk(s.spec.Marks)
	lg.Hollow = lg.Fill == nil && lg.Stroke != nil
	if first == nil {
		return
	}
	fill, hasFill := markLiteral(first, "fill")
	stroke, hasStroke := markLiteral(first, "stroke")
	encodes := func(prop string) bool {
		for _, set := range encodeSets {
			if len(first.Encode[set][prop]) > 0 {
				return true
			}
		}
		return false
	}
	switch first.Type {
	case "line", "trail", "rule":
		lg.Line = lg.Line || (lg.Shape == nil && lg.Size == nil)
		lg.Hollow = false
	case "symbol":
		if lg.SymbolType == "" && !lg.Line {
			lg.SymbolType = "circle"
			if sh, ok := markLiteral(first, "shape"); ok {
				lg.SymbolType = sh
			}
		}
		fallthrough
	default:
		// A mark with no fill, or a fill that shows nothing, draws its
		// outline only.
		none := hasFill && (fill == "transparent" || fill == "none")
		lg.Hollow = none || (!encodes("fill") && encodes("stroke"))
		if lg.Hollow && (first.Type == "shape" || first.Type == "path") && lg.Shape == nil && lg.Size == nil {
			// A shape with no fill, such as a line of a map, draws as
			// a line.
			lg.Line, lg.Hollow = true, false
		}
	}
	switch {
	case lg.Hollow && hasStroke:
		lg.Color = stroke
	case !lg.Hollow && hasFill:
		lg.Color = fill
	}
}

// markLiteral returns the string that the encoding of a mark gives a
// property with no scale, no field and no signal, such as the value of a
// fill, and whether it gives one. The update set wins over the enter set,
// and the last rule, which has no test, wins over the others.
func markLiteral(m *vega.Mark, prop string) (string, bool) {
	out, found := "", false
	for _, set := range encodeSets {
		for _, r := range m.Encode[set][prop] {
			if len(r.Value) == 0 || !r.Scale.IsZero() || r.Field.Name != "" || r.Signal != "" {
				continue
			}
			var v any
			if json.Unmarshal(r.Value, &v) != nil {
				continue
			}
			if str, ok := v.(string); ok {
				out, found = str, true
			}
		}
	}
	return out, found
}

// legendPlace is where a legend goes, which placeGuides reads: its orient,
// its offset from the plot area, and its place for the orient none.
type legendPlace struct {
	orient       string
	offset, x, y float64
}

// legendParts and axisEncodeParts are the roles of the parts of a legend
// and of an axis that the encode of a guide names.
var (
	legendParts = map[string]string{
		"legend": "legend", "title": "legend-title", "labels": "legend-label", "symbols": "legend-symbol",
		"gradient": "legend-gradient", "entries": "legend-entry",
	}
	axisEncodeParts = map[string]string{
		"axis": "axis", "title": "axis-title", "labels": "axis-label", "grid": "axis-grid",
		"domain": "axis-domain", "ticks": "axis-tick",
	}
	// gradientLegendParts are the parts of a legend with entry groups that
	// draws a gradient. Its group of entries holds the bar and its labels,
	// and the encode of the entries applies to no part of it, as in Vega.
	gradientLegendParts = map[string]string{
		"legend": "legend", "title": "legend-title", "labels": "legend-label", "symbols": "legend-symbol",
		"gradient": "legend-gradient", "entries": "",
	}
)

// encodesLegendPlace reports whether the encode of a legend sets the place
// of a part of the legend other than the legend itself.
func encodesLegendPlace(enc vega.GuideEncode) bool {
	for name, el := range enc {
		if role := legendParts[name]; role == "" || role == "legend" {
			continue
		}
		for _, set := range el.Sets {
			for k := range set {
				if placeProps[k] {
					return true
				}
			}
		}
	}
	return false
}

// entryFrame is the frame of the parts of a legend with entry groups
// (layout.Legend). The symbol and the label of an entry stand in the frame
// of the group of the entry, and a gradient and its labels in the frame of
// their group, as in Vega. The layout places the entries and the title
// after the encode, so the place that it gives them wins over the encode,
// as in Vega. The layout places the group of the legend too, so an encode
// of its place goes in the report.
func entryFrame(role string) (x, y float64, ok bool) {
	return 0, 0, role != "legend"
}

// placeProps are the properties of an encode that place an item.
var placeProps = map[string]bool{
	"x": true, "y": true, "x2": true, "y2": true, "xc": true, "yc": true, "width": true, "height": true,
}

// guideFrame returns the origin, in the frame of the scene, of the frame in
// which Vega places the items of a part of a guide, and whether an encode of
// the place of that part applies.
type guideFrame func(role string) (x, y float64, ok bool)

// axisFrame is the frame of the parts of an axis. Vega places the group of
// an axis at the edge of the plot area that the axis runs along, moved out
// by its offset, and the parts of the axis inside that group. The layout of
// zuzu places the parts in the frame of the plot area, so the frame of Vega
// starts at that edge. The layout of Vega sets the place of the group of an
// axis, so an encode of the place of the group goes in the report.
func axisFrame(ax *layout.Axis) guideFrame {
	var x, y float64
	switch ax.Orient {
	case "top":
		y = -ax.Offset
	case "bottom":
		y = ax.Cross + ax.Offset
	case "left":
		x = -ax.Offset
	default:
		x = ax.Cross + ax.Offset
	}
	return func(role string) (float64, float64, bool) {
		return x, y, role != "axis"
	}
}

// guideEncode applies the encode of a legend or an axis to its parts, with
// the datum of each item, which holds its value and its label. An encode of
// the place of an item applies in the frame of Vega that frame gives, as in
// vega.js. A nil frame, or a part whose frame does not apply, puts an
// encode of a place in the report.
func (s *scope) guideEncode(m *scene.Mark, enc vega.GuideEncode, parts map[string]string, frame guideFrame, path, kind string) error {
	var top *markState
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
		var ox, oy float64
		placed := false
		if frame != nil {
			ox, oy, placed = frame(role)
		}
		sets := vega.Encode{}
		for _, set := range slices.Sorted(maps.Keys(el.Sets)) {
			kept := vega.Props{}
			for _, k := range slices.Sorted(maps.Keys(el.Sets[set])) {
				if placeProps[k] && !placed {
					if err := s.rt.issue(vega.ErrUnsupported, path+"/encode/"+name+"/"+set, "the encode of %s of the %s of %s", k, name, kind); err != nil {
						return err
					}
					continue
				}
				kept[k] = el.Sets[set][k]
			}
			sets[set] = kept
		}
		if top == nil {
			// The marks of the guide, which an expression of an encode
			// reads as item.mark, are made once for every part.
			top = &markState{sm: m, s: s, group: s.group(), guide: true, frame: frame}
		}
		ordered := orderEncode(sets)
		var err error
		var walk func(ms *markState)
		walk = func(ms *markState) {
			if ms.sm.Role == role {
				ms.sets = ordered
				if el.Name != "" {
					ms.name = el.Name
				}
			}
			for _, st := range ms.itemStates() {
				it := st.it
				if ms.sm.Role == role && err == nil {
					// A rule whose default has one end across an
					// axis, such as a line of the grid, keeps that end
					// with the other when an encode moves the other,
					// as a rule of Vega with no x2 or y2 does.
					flatX, flatY := it.X2 == it.X, it.Y2 == it.Y
					err = s.applyEncode(ordered, st, it.Datum, "/encode/"+name)
					if err == nil {
						placeGuideItem(ms.sm.Type, st, ox, oy, flatX, flatY)
					}
				}
				if l := st.marks(); l != nil {
					for _, k := range l.marks {
						walk(k)
					}
				}
			}
		}
		walk(top)
		if err != nil {
			return err
		}
	}
	return nil
}

// placeGuideItem moves the place that an encode gave an item of a guide
// from the frame of Vega, which starts at (ox, oy), to the frame of the
// scene, and works out the place of the item from it. flatX and flatY say
// whether a rule had the same x at both ends, or the same y, before the
// encode.
func placeGuideItem(typ string, st *itemState, ox, oy float64, flatX, flatY bool) {
	it := st.it
	for _, p := range []struct {
		name string
		v    *float64
		o    float64
	}{{"x", &it.X, ox}, {"x2", &it.X2, ox}, {"xc", &st.xc, ox}, {"y", &it.Y, oy}, {"y2", &it.Y2, oy}, {"yc", &st.yc, oy}} {
		if st.set[p.name] {
			*p.v += p.o
		}
	}
	if typ != scene.Rule {
		geometry(typ, st)
		return
	}
	if st.set["xc"] && !st.set["x"] {
		it.X = st.xc
	}
	if st.set["yc"] && !st.set["y"] {
		it.Y = st.yc
	}
	if flatX && !st.set["x2"] {
		it.X2 = it.X
	}
	if flatY && !st.set["y2"] {
		it.Y2 = it.Y
	}
	for _, f := range []*float64{&it.X, &it.Y, &it.X2, &it.Y2} {
		if math.IsNaN(*f) || math.IsInf(*f, 0) {
			*f = 0
		}
	}
}

// buildTitle builds the title of a scope. A title of a group stands beyond
// the marks and the guides that the group already drew, such as the labels
// of its axis, and follows the properties and the encode that the spec
// gives it (title.go).
func (s *scope) buildTitle(path string, drawn []scene.Mark) ([]scene.Mark, error) {
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
	// The title of a group takes the config of a title and of its styles
	// where its spec sets nothing, as in Vega.
	var cfg map[string]any
	if s.parent != nil {
		cfg = s.titleConfig(t)
	}
	if lt.Orient, err = s.str(t.Orient, configStr(cfg, "orient", "top")); err != nil {
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
	if kind != "title" || s.parent != nil {
		// The title of a group is at the middle, as in Vega. The look puts
		// only the title of the chart at the start.
		def = "middle"
	}
	if lt.Anchor, err = s.str(t.Anchor, configStr(cfg, "anchor", def)); err != nil {
		return nil, err
	}
	off := 4.0
	if kind == "title" {
		off = 10
	}
	if lt.Offset, err = s.num(t.Offset, configNum(cfg, "offset", off)); err != nil {
		return nil, err
	}
	if a, ok := propNum(cfg, "angle"); ok || !t.Angle.IsZero() {
		if a, err = s.num(t.Angle, a); err != nil {
			return nil, err
		}
		lt.Angle = &a
	}
	if lt.Align, err = s.str(t.Align, configStr(cfg, "align", "")); err != nil {
		return nil, err
	}
	if lt.Baseline, err = s.str(t.Baseline, configStr(cfg, "baseline", "")); err != nil {
		return nil, err
	}
	shift := 0.0
	if s.parent != nil {
		b := s.contentBounds(drawn)
		if shift, err = s.titleFrame(t, lt, b, cfg); err != nil {
			return nil, err
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
	moveTitle(&m, lt.Orient, shift)
	if s.parent != nil {
		if err := s.styleGroupTitle(&m, t, cfg); err != nil {
			return nil, err
		}
	} else {
		s.offsetTitle(&m, t)
	}
	// The config of the title styles only the title of the chart. The
	// properties of the spec style every title, such as the labels of
	// the headers of a facet.
	var config map[string]any
	if kind == "title" && s.parent == nil {
		m.Name = "title"
		config = s.rt.configObject("title")
	}
	props := s.guideProps(config, t.Extra)
	for k, v := range map[string]vega.Value{"color": t.Color, "font": t.Font, "fontSize": t.FontSize} {
		if !v.IsZero() {
			x, err := s.value(v)
			if err != nil {
				return nil, err
			}
			props[k] = x
		}
	}
	s.styleGuide(&m, props, titleParts)
	if err := s.titleEncode(&m, t, path); err != nil {
		return nil, err
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
