package mermaid

import (
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/xo/zuzu/layout"
	"github.com/xo/zuzu/text"
)

// The look of a gantt.
const (
	// ganttCrit is the color of the outline of a critical task.
	ganttCrit = "#e5484d"
	// ganttExcluded is the color of an excluded day.
	ganttExcluded = "#98a2b3"
	// ganttBandYears is the most whole years that a gantt with bands of
	// the excluded days spans, and ganttMaxTicks the most ticks that
	// tickInterval gives, as in Mermaid. Past them, Mermaid draws no bands,
	// and its axis takes ticks of its own.
	ganttBandYears = 5
	ganttMaxTicks  = 10000
	// ganttMinBar is the least width of a bar, so that a short task in a
	// long gantt still draws.
	ganttMinBar = 2
	// ganttNameGap is the space between a bar and its name in the compact
	// mode, and ganttDiamond half the width of the diamond of a milestone.
	ganttNameGap = 4
	ganttDiamond = 7
	// ganttLayout writes a time as a date of JavaScript with no zone, which
	// reads in the time zone of the render (D27).
	ganttLayout = "2006-01-02T15:04:05.000"
	// ganttWidth is the width of the time axis, and ganttRow the height of
	// the row of a task.
	ganttWidth = 640
	ganttRow   = 26
	// ganttTint is the opacity of the band of a section, the tint of a
	// group of a hierarchy and of a node of a diagram.
	ganttTint = 0.14
)

// ganttChart compiles a chart of the kind gantt into Vega-Lite: a bar for
// each task on a time axis, colored by section, and a diamond for each
// milestone. The compiler plans the tasks in Go, and gives Vega-Lite the
// start and the end of each as a local time. zuzu has no time of the
// render here, so a date with no day takes the day 1970-01-01.
func ganttChart(c *chart) (map[string]any, bool, error) {
	g, err := planGantt(c, time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		return nil, false, err
	}
	if len(g.tasks) == 0 {
		// Mermaid draws a gantt with no task with nothing in it.
		return emptyChart(g.title, g.desc), false, nil
	}
	spec, err := g.spec()
	return spec, true, err
}

// planGantt reads and plans a gantt. The dates are in UTC, as the times of
// a wall clock, and the render reads them in its own time zone.
func planGantt(c *chart, today time.Time) (*gantt, error) {
	g, err := readGantt(c, today, time.UTC)
	if err != nil {
		return nil, err
	}
	if err := g.plan(); err != nil {
		return nil, err
	}
	return g, nil
}

// spec writes the spec of Vega-Lite of a planned gantt.
func (g *gantt) spec() (map[string]any, error) {
	keys := g.keys()
	rowOf, rows := g.rows(keys)
	sections := map[string]bool{}
	var order []any
	for _, t := range g.tasks {
		if !t.vert && !sections[t.section] {
			sections[t.section] = true
			order = append(order, t.section)
		}
	}
	// tone is the color of a section when the sections take a number of
	// colors in turn.
	tone := func(section string) int {
		return slices.Index(order, any(section)) % max(g.sectionStyles, 1)
	}
	// The time axis spans the tasks. A diamond at either end of the axis
	// needs room, so a gantt with milestones pads its time axis.
	lo, hi := g.span()
	first, last := lo, hi
	hasMiles := slices.ContainsFunc(g.tasks, func(t *ganttTask) bool { return t.milestone && !t.vert })
	if hasMiles {
		pad := max(hi.Sub(lo)/40, time.Millisecond)
		lo, hi = lo.Add(-pad), hi.Add(pad)
	}
	// A bar is never less than ganttMinBar wide. A short bar grows to the
	// right, or to the left at the end of the axis, so that the axis does
	// not grow.
	least := ganttMillis(lo, hi) * ganttMinBar / ganttWidth
	var normal, done, active, crit, miles, verts, names []any
	for i, t := range g.tasks {
		if t.vert {
			verts = append(verts, map[string]any{"name": t.name, "at": t.start.Format(ganttLayout)})
			continue
		}
		start, end := t.start, t.render
		if !t.milestone && !end.Before(start) && ganttMillis(start, end) < least {
			end = ganttAdd(start, least)
			if end.After(hi) {
				start, end = ganttAdd(hi, -least), hi
			}
		}
		row := map[string]any{
			"key":     rows[rowOf[i]].key,
			"task":    t.name,
			"section": t.section,
			"start":   start.Format(ganttLayout),
			"end":     end.Format(ganttLayout),
		}
		if g.sectionStyles > 0 {
			row["tone"] = tone(t.section)
		}
		if g.compact {
			names = append(names, ganttName(t, row["key"], lo, hi))
		}
		if t.milestone {
			row["at"] = t.start.Add(t.render.Sub(t.start) / 2).Format(ganttLayout)
			miles = append(miles, row)
			if t.crit {
				crit = append(crit, row)
			}
			continue
		}
		switch {
		case t.done:
			done = append(done, row)
		case t.active:
			active = append(active, row)
		default:
			normal = append(normal, row)
		}
		if t.crit {
			crit = append(crit, row)
		}
	}
	domain := make([]any, 0, len(rows))
	for _, r := range rows {
		domain = append(domain, r.key)
	}
	axis, err := g.axis(first, last)
	if err != nil {
		return nil, err
	}
	// The compact mode places the names of the tasks by the widths of
	// their bars, so it fixes the domain of the time axis.
	var xScale map[string]any
	if hasMiles || g.compact {
		xScale = map[string]any{"domain": []any{ganttDateTime(lo), ganttDateTime(hi)}}
	}
	xOf := func(field string) map[string]any {
		x := map[string]any{"field": field, "type": "temporal", "title": nil, "axis": axis}
		if xScale != nil {
			x["scale"] = xScale
		}
		return x
	}
	x := xOf("start")
	// The outer padding is half the inner padding, so that each row of a
	// task is ganttRow high and the bands of the sections meet.
	y := map[string]any{"field": "key", "type": "nominal", "title": nil,
		"scale": map[string]any{"domain": domain, "paddingInner": 0.3, "paddingOuter": 0.15}}
	if g.compact {
		// A row holds more than one task, so the names of the tasks go on
		// their bars, and the axis has no labels.
		y["axis"] = map[string]any{"labels": false}
	}
	named := slices.ContainsFunc(g.tasks, func(t *ganttTask) bool { return !t.vert && t.section != "" })
	colorField, colorDomain := "section", order
	if g.sectionStyles > 0 {
		// The sections take the first numberSectionStyles colors in turn.
		colorField, colorDomain = "tone", nil
		for i := range g.sectionStyles {
			colorDomain = append(colorDomain, i)
		}
	}
	var color any = map[string]any{"field": colorField, "type": "nominal", "title": nil,
		"scale": map[string]any{"domain": colorDomain}, "legend": map[string]any{"orient": "bottom"}}
	if len(sections) < 2 || named {
		// The names of the sections at the left take the place of the
		// legend.
		color = map[string]any{"field": colorField, "type": "nominal", "scale": map[string]any{"domain": colorDomain}, "legend": nil}
	}
	var secs []any
	if named {
		secs = g.sections(rows, !g.compact, tone, color)
	}
	bar := func(rows []any, mark map[string]any) map[string]any {
		mark["type"] = "bar"
		mark["cornerRadius"] = 3
		return map[string]any{
			"data":     ganttData(rows, "start", "end"),
			"mark":     mark,
			"encoding": map[string]any{"x": x, "x2": map[string]any{"field": "end"}, "y": y, "color": color},
		}
	}
	var layers []any
	if secs != nil {
		layers = append(layers, secs...)
	}
	bands, err := g.bands()
	if err != nil {
		return nil, err
	}
	if len(bands) > 0 {
		layers = append(layers, map[string]any{
			"data":     ganttData(bands, "start", "end"),
			"mark":     map[string]any{"type": "rect", "color": ganttExcluded, "opacity": 0.15},
			"encoding": map[string]any{"x": x, "x2": map[string]any{"field": "end"}},
		})
	}
	if len(normal) > 0 {
		layers = append(layers, bar(normal, map[string]any{"fillOpacity": 0.9}))
	}
	if len(done) > 0 {
		layers = append(layers, bar(done, map[string]any{"fillOpacity": 0.35}))
	}
	if len(active) > 0 {
		layers = append(layers, bar(active, map[string]any{"fillOpacity": 0.2}),
			bar(active, map[string]any{"filled": false, "strokeWidth": 1.5}))
	}
	if len(crit) > 0 {
		var bars, points []any
		for _, r := range crit {
			if _, ok := r.(map[string]any)["at"]; ok {
				points = append(points, r)
			} else {
				bars = append(bars, r)
			}
		}
		if len(bars) > 0 {
			layers = append(layers, bar(bars, map[string]any{"fillOpacity": 0, "stroke": ganttCrit, "strokeWidth": 2}))
		}
		crit = points
	}
	if len(miles) > 0 {
		layers = append(layers, map[string]any{
			"data": ganttData(miles, "at"),
			"mark": map[string]any{"type": "point", "shape": "diamond", "filled": true, "size": 180, "opacity": 1},
			"encoding": map[string]any{
				"x": xOf("at"), "y": y, "color": color,
			},
		})
	}
	if len(crit) > 0 {
		layers = append(layers, map[string]any{
			"data": ganttData(crit, "at"),
			"mark": map[string]any{"type": "point", "shape": "diamond", "filled": false, "size": 180, "stroke": ganttCrit, "strokeWidth": 2},
			"encoding": map[string]any{
				"x": xOf("at"), "y": y,
			},
		})
	}
	layers = append(layers, ganttNames(names, xOf("at"), y)...)
	if len(verts) > 0 {
		vx := xOf("at")
		layers = append(layers,
			map[string]any{
				// A rule of one item is a reference line in the look of
				// zuzu, which writes its value as a number. Each marker
				// draws twice, so that the mark has more than one item.
				"data":     ganttData(append(slices.Clone(verts), verts...), "at"),
				"mark":     map[string]any{"type": "rule", "strokeDash": []any{4, 3}, "strokeWidth": 1.5},
				"encoding": map[string]any{"x": vx},
			},
			map[string]any{
				"data":     ganttData(verts, "at"),
				"mark":     map[string]any{"type": "text", "baseline": "bottom", "dy": -4, "fontSize": 11},
				"encoding": map[string]any{"x": vx, "y": map[string]any{"value": 0}, "text": map[string]any{"field": "name"}},
			})
	}
	if len(layers) == 0 {
		return nil, g.c.errorf("the gantt has no task")
	}
	spec := map[string]any{
		"width":  ganttWidth,
		"height": map[string]any{"step": ganttRow},
		"layer":  layers,
	}
	if g.rightPadding != nil {
		// rightPadding is the padding at the right of the chart. The
		// padding of Vega-Lite is 5 on each other side.
		spec["padding"] = map[string]any{"left": 5, "top": 5, "bottom": 5, "right": *g.rightPadding}
	}
	if g.title != "" {
		spec["title"] = map[string]any{"text": g.title, "anchor": "start"}
	}
	if g.desc != "" {
		spec["description"] = g.desc
	}
	return spec, nil
}

// ganttLane is a row of a gantt: its key on the axis, and its section.
type ganttLane struct {
	key, section string
}

// rows returns the index of the row of each task that is not a marker, by
// the index of the task, and the rows from the top. Each task has a row of
// its own, with the key of the task. In the compact mode, the tasks of each
// section pack into as few rows as their times allow, as in Mermaid. The
// sections go in the order of their first tasks. In a section, the tasks
// go in the order of their starts, and each task takes the first row whose
// last task ends at or before its start.
func (g *gantt) rows(keys []string) ([]int, []ganttLane) {
	rowOf := make([]int, len(g.tasks))
	var rows []ganttLane
	if !g.compact {
		for i, t := range g.tasks {
			if !t.vert {
				rowOf[i] = len(rows)
				rows = append(rows, ganttLane{key: keys[i], section: t.section})
			}
		}
		return rowOf, rows
	}
	var order []string
	bySection := map[string][]int{}
	for i, t := range g.tasks {
		if t.vert {
			continue
		}
		if _, ok := bySection[t.section]; !ok {
			order = append(order, t.section)
		}
		bySection[t.section] = append(bySection[t.section], i)
	}
	for _, sec := range order {
		tasks := bySection[sec]
		slices.SortStableFunc(tasks, func(a, b int) int { return g.tasks[a].start.Compare(g.tasks[b].start) })
		first := len(rows)
		var ends []time.Time
		for _, i := range tasks {
			t := g.tasks[i]
			lane := slices.IndexFunc(ends, func(e time.Time) bool { return !t.start.Before(e) })
			if lane < 0 {
				lane = len(ends)
				ends = append(ends, time.Time{})
				rows = append(rows, ganttLane{key: strconv.Itoa(len(rows) + 1), section: sec})
			}
			ends[lane] = t.end
			rowOf[i] = first + lane
		}
	}
	return rowOf, rows
}

// ganttName returns the row of the name of a task in the compact mode, as
// Mermaid places it: in the middle of its bar when it is no wider than the
// bar, else past the end of the bar, and else before the start of the bar
// when it does not fit at the right of the time axis. The time axis runs
// from lo to hi. The name of a milestone is beside its diamond.
func ganttName(t *ganttTask, key any, lo, hi time.Time) map[string]any {
	scale := max(ganttMillis(lo, hi), 1) / ganttWidth
	px := func(at time.Time) float64 { return ganttMillis(lo, at) / scale }
	start, end := t.start, t.render
	if t.milestone {
		// The diamond draws in the middle of the task, and the name keeps
		// clear of it.
		mid := t.start.Add(t.render.Sub(t.start) / 2)
		start, end = ganttAdd(mid, -ganttDiamond*scale), ganttAdd(mid, ganttDiamond*scale)
	}
	w := text.NewEstimate().Width(text.Font{Family: text.Family, Size: layout.LabelSize, Weight: 600}, t.name)
	x0, x1 := px(start), px(end)
	out := map[string]any{"key": key, "name": t.name}
	switch {
	case !t.milestone && w <= x1-x0:
		out["place"], out["at"] = "inside", ganttAdd(start, ganttMillis(start, end)/2).Format(ganttLayout)
	case x1+ganttNameGap+w <= ganttWidth:
		out["place"], out["at"] = "right", end.Format(ganttLayout)
	default:
		out["place"], out["at"] = "left", start.Format(ganttLayout)
	}
	return out
}

// ganttNames returns the layers of the names of the tasks in the compact
// mode, a layer for each place of a name. A name has a halo, so that it
// reads on a bar and on the grid.
func ganttNames(names []any, x, y map[string]any) []any {
	var layers []any
	for _, p := range []struct {
		place, align string
		dx           int
	}{{"inside", "center", 0}, {"right", "left", ganttNameGap}, {"left", "right", -ganttNameGap}} {
		var rows []any
		for _, n := range names {
			if m, _ := n.(map[string]any); m["place"] == p.place {
				rows = append(rows, n)
			}
		}
		if len(rows) == 0 {
			continue
		}
		layers = append(layers, map[string]any{
			"data": ganttData(rows, "at"),
			"mark": map[string]any{"type": "text", "align": p.align, "baseline": "middle", "dx": p.dx,
				"fontSize": layout.LabelSize, "fontWeight": 600, "style": "zuzu.halo"},
			"encoding": map[string]any{"x": x, "y": y, "text": map[string]any{"field": "name"}},
		})
	}
	return layers
}

// sections returns the layers that draw the sections as Mermaid does: a
// band behind the rows of every other section, across the chart, and the
// name of each section at the left of the names of its tasks. A section
// draws as a group of a hierarchy does in the look of zuzu: the band is a
// tint of the color of the section, and the name is 11 pixels, bold, in the
// color of the text. The band is a tint and not a panel, so that the grid
// and the names of the tasks under it still show. labeled tells whether
// the axis writes the key of each row. tone gives the color of a section
// when the sections take colors in turn.
func (g *gantt) sections(rows []ganttLane, labeled bool, tone func(string) int, color any) []any {
	type run struct {
		name        string
		first, last int
	}
	var runs []run
	m := text.NewEstimate()
	var keyWidth, nameWidth float64
	for i, r := range rows {
		if labeled {
			keyWidth = max(keyWidth, m.Width(text.Font{Family: text.Family, Size: layout.LabelSize, Weight: 400}, r.key))
		}
		if n := len(runs); n > 0 && runs[n-1].name == r.section {
			runs[n-1].last = i
		} else {
			runs = append(runs, run{name: r.section, first: i, last: i})
			nameWidth = max(nameWidth, m.Width(text.Font{Family: text.Family, Size: 11, Weight: 600}, r.section))
		}
	}
	// The band starts 8 before the name of the section. The name ends 16
	// before the names of the tasks, which end 8 before the plot and draw
	// at most 180 wide.
	left := layout.LabelPad + min(keyWidth, 180) + 16 + nameWidth + 8
	var bands, names []any
	for i, r := range runs {
		y0, y1 := float64(r.first*ganttRow), float64((r.last+1)*ganttRow)
		if i%2 == 0 {
			band := map[string]any{"section": r.name, "y0": y0, "y1": y1}
			if g.sectionStyles > 0 {
				band["tone"] = tone(r.name)
			}
			bands = append(bands, band)
		}
		if r.name != "" {
			names = append(names, map[string]any{"section": r.name, "y": (y0 + y1) / 2})
		}
	}
	pixels := func(field string) map[string]any {
		return map[string]any{"field": field, "type": "quantitative", "scale": nil}
	}
	return []any{
		map[string]any{
			"data": map[string]any{"values": bands},
			"mark": map[string]any{"type": "rect", "fillOpacity": ganttTint},
			"encoding": map[string]any{
				"x": map[string]any{"value": -left}, "x2": map[string]any{"value": ganttWidth},
				"y": pixels("y0"), "y2": map[string]any{"field": "y1"}, "color": color,
			},
		},
		map[string]any{
			"data": map[string]any{"values": names},
			"mark": map[string]any{"type": "text", "align": "left", "baseline": "middle", "fontSize": 11, "fontWeight": 600},
			"encoding": map[string]any{
				"x": map[string]any{"value": -left + 8}, "y": pixels("y"), "text": map[string]any{"field": "section"},
			},
		},
	}
}

// keys returns the key of the row of each task: its name, with spaces in
// front of a name that an earlier task has, so that each task has a row of
// its own.
func (g *gantt) keys() []string {
	keys := make([]string, len(g.tasks))
	seen := map[string]bool{}
	for i, t := range g.tasks {
		k := t.name
		for seen[k] {
			k = " " + k
		}
		seen[k] = true
		keys[i] = k
	}
	return keys
}

// axis returns the axis of time of tasks from lo to hi: the format of
// axisFormat, the ticks of tickInterval, and the top for topAxis. As in
// Mermaid, a tickInterval that gives more than ganttMaxTicks ticks is an
// issue, and the axis takes ticks of its own. Mermaid counts a month as 30
// days.
func (g *gantt) axis(lo, hi time.Time) (map[string]any, error) {
	f := g.axisFormat
	if f == "" {
		f = "%Y-%m-%d"
	}
	axis := map[string]any{"format": f, "formatType": "time", "grid": true}
	if g.topAxis {
		axis["orient"] = "top"
	}
	if g.tickInterval == "" {
		return axis, nil
	}
	m := ganttTickRe.FindStringSubmatch(g.tickInterval)
	if m == nil {
		return axis, g.c.add("tickInterval %q is not a number and a unit of time", g.tickInterval)
	}
	unit := m[2]
	n, _ := strconv.Atoi(m[1])
	step := map[string]time.Duration{
		"millisecond": time.Millisecond, "second": time.Second, "minute": time.Minute, "hour": time.Hour,
		"day": 24 * time.Hour, "week": 7 * 24 * time.Hour, "month": 30 * 24 * time.Hour,
	}[unit] * time.Duration(n)
	if ticks := math.Ceil(ganttMillis(lo, hi) / float64(step.Milliseconds())); ticks > ganttMaxTicks {
		return axis, g.c.add("tickInterval %s gives %.0f ticks, more than %d, so the axis takes ticks of its own, as in Mermaid",
			g.tickInterval, ticks, ganttMaxTicks)
	}
	if unit == "week" && g.weekday != "" && g.weekday != "sunday" {
		if err := g.c.add("zuzu starts a week of the ticks on sunday, not on %s", g.weekday); err != nil {
			return nil, err
		}
	}
	axis["tickCount"] = map[string]any{"interval": unit, "step": n}
	return axis, nil
}

// bands returns the excluded days from the first start to the last end, a
// row for each run of days. As in Mermaid, a gantt that spans more than
// ganttBandYears whole years draws no bands, and that is an issue.
func (g *gantt) bands() ([]any, error) {
	if len(g.excludes) == 0 {
		return nil, nil
	}
	var lo, hi time.Time
	for _, t := range g.tasks {
		if lo.IsZero() || t.start.Before(lo) {
			lo = t.start
		}
		for _, e := range []time.Time{t.start, t.end} {
			if hi.IsZero() || e.After(hi) {
				hi = e
			}
		}
	}
	years := hi.Year() - lo.Year()
	if hi.AddDate(-years, 0, 0).Before(lo) {
		years--
	}
	if years > ganttBandYears {
		return nil, g.c.add("the gantt spans %d years, more than %d, so it draws no excluded days, as in Mermaid", years, ganttBandYears)
	}
	y, m, d := lo.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, g.loc)
	var rows []any
	var run time.Time
	for day.Before(hi) {
		next := day.AddDate(0, 0, 1)
		if g.excluded(day) {
			if run.IsZero() {
				run = day
			}
		} else if !run.IsZero() {
			rows = append(rows, ganttBand(run, day))
			run = time.Time{}
		}
		day = next
	}
	if !run.IsZero() {
		rows = append(rows, ganttBand(run, day))
	}
	return rows, nil
}

// ganttBand returns the row of a run of excluded days.
func ganttBand(start, end time.Time) map[string]any {
	return map[string]any{"start": start.Format(ganttLayout), "end": end.Format(ganttLayout)}
}

// ganttData returns inline data whose fields of times read as dates.
func ganttData(rows []any, fields ...string) map[string]any {
	parse := map[string]any{}
	for _, f := range fields {
		parse[f] = "date"
	}
	return map[string]any{"values": rows, "format": map[string]any{"parse": parse}}
}

// span returns the first and the last time that the tasks draw.
func (g *gantt) span() (time.Time, time.Time) {
	var lo, hi time.Time
	for _, t := range g.tasks {
		for _, at := range []time.Time{t.start, t.render} {
			if lo.IsZero() || at.Before(lo) {
				lo = at
			}
			if hi.IsZero() || at.After(hi) {
				hi = at
			}
		}
	}
	return lo, hi
}

// ganttMillis returns the milliseconds from one time to another. It does
// not overflow, as a time.Duration does past 292 years.
func ganttMillis(from, to time.Time) float64 {
	return float64(to.UnixMilli() - from.UnixMilli())
}

// ganttAdd adds milliseconds to a time.
func ganttAdd(t time.Time, ms float64) time.Time {
	return time.UnixMilli(t.UnixMilli() + int64(math.Round(ms))).In(t.Location())
}

// ganttDateTime writes a time as a date time of Vega-Lite, in local time.
func ganttDateTime(t time.Time) map[string]any {
	return map[string]any{
		"year": t.Year(), "month": int(t.Month()), "date": t.Day(),
		"hours": t.Hour(), "minutes": t.Minute(), "seconds": t.Second(), "milliseconds": t.Nanosecond() / 1e6,
	}
}
