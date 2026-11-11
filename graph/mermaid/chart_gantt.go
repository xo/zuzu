package mermaid

import (
	"slices"
	"time"
)

// The look of a gantt.
const (
	// ganttCrit is the color of the outline of a critical task.
	ganttCrit = "#e5484d"
	// ganttExcluded is the color of an excluded day.
	ganttExcluded = "#98a2b3"
	// ganttMaxDays is the most days that the bands of the excluded days
	// cover.
	ganttMaxDays = 5000
	// ganttLayout writes a time as a date of JavaScript with no zone, which
	// reads in the time zone of the render (D27).
	ganttLayout = "2006-01-02T15:04:05.000"
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
	sections := map[string]bool{}
	var normal, done, active, crit, miles, verts, order []any
	for i, t := range g.tasks {
		if t.vert {
			verts = append(verts, map[string]any{"name": t.name, "at": t.start.Format(ganttLayout)})
			continue
		}
		if !sections[t.section] {
			sections[t.section] = true
			order = append(order, t.section)
		}
		row := map[string]any{
			"key":     keys[i],
			"task":    t.name,
			"section": t.section,
			"start":   t.start.Format(ganttLayout),
			"end":     t.render.Format(ganttLayout),
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
	var domain []any
	for i, t := range g.tasks {
		if !t.vert {
			domain = append(domain, keys[i])
		}
	}
	axis, err := g.axis()
	if err != nil {
		return nil, err
	}
	// A diamond at either end of the axis needs room, so a gantt with
	// milestones pads its time axis.
	var xScale map[string]any
	if len(miles) > 0 {
		lo, hi := g.span()
		pad := max(hi.Sub(lo)/40, time.Millisecond)
		xScale = map[string]any{"domain": []any{ganttDateTime(lo.Add(-pad)), ganttDateTime(hi.Add(pad))}}
	}
	xOf := func(field string) map[string]any {
		x := map[string]any{"field": field, "type": "temporal", "title": nil, "axis": axis}
		if xScale != nil {
			x["scale"] = xScale
		}
		return x
	}
	x := xOf("start")
	y := map[string]any{"field": "key", "type": "nominal", "title": nil, "scale": map[string]any{"domain": domain, "paddingInner": 0.3}}
	var color any = map[string]any{"field": "section", "type": "nominal", "title": nil,
		"scale": map[string]any{"domain": order}, "legend": map[string]any{"orient": "bottom"}}
	if len(sections) < 2 {
		color = map[string]any{"field": "section", "type": "nominal", "legend": nil}
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
	if bands := g.bands(); len(bands) > 0 {
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
		"width":  640,
		"height": map[string]any{"step": 26},
		"layer":  layers,
	}
	if g.title != "" {
		spec["title"] = map[string]any{"text": g.title, "anchor": "start"}
	}
	if g.desc != "" {
		spec["description"] = g.desc
	}
	return spec, nil
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

// axis returns the axis of time: the format of axisFormat, the ticks of
// tickInterval, and the top for topAxis.
func (g *gantt) axis() (map[string]any, error) {
	f := g.axisFormat
	if f == "" {
		f = "%Y-%m-%d"
	}
	axis := map[string]any{"format": f, "formatType": "time", "grid": true}
	if g.topAxis {
		axis["orient"] = "top"
	}
	if g.tickInterval != "" {
		m := ganttTickRe.FindStringSubmatch(g.tickInterval)
		if m == nil {
			if err := g.c.add("tickInterval %q is not a number and a unit of time", g.tickInterval); err != nil {
				return nil, err
			}
		} else {
			unit := m[2]
			if unit == "week" && g.weekday != "" && g.weekday != "sunday" {
				if err := g.c.add("zuzu starts a week of the ticks on sunday, not on %s", g.weekday); err != nil {
					return nil, err
				}
			}
			n := 0
			for _, r := range m[1] {
				n = n*10 + int(r-'0')
			}
			axis["tickCount"] = map[string]any{"interval": unit, "step": n}
		}
	}
	return axis, nil
}

// bands returns the excluded days from the first start to the last end, a
// row for each run of days.
func (g *gantt) bands() []any {
	if len(g.excludes) == 0 {
		return nil
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
	y, m, d := lo.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, g.loc)
	var rows []any
	var run time.Time
	for n := 0; n < ganttMaxDays && day.Before(hi); n++ {
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
	return rows
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

// ganttDateTime writes a time as a date time of Vega-Lite, in local time.
func ganttDateTime(t time.Time) map[string]any {
	return map[string]any{
		"year": t.Year(), "month": int(t.Month()), "date": t.Day(),
		"hours": t.Hour(), "minutes": t.Minute(), "seconds": t.Second(), "milliseconds": t.Nanosecond() / 1e6,
	}
}
