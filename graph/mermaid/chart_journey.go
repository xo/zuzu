package mermaid

import (
	"slices"
	"strconv"
	"strings"
)

// journeyTask is a task of a user journey: its section, its name, its
// score from 1 to 5 and its actors.
type journeyTask struct {
	section, name string
	score         float64
	actors        []string
}

// journeyChart compiles a user journey into points of Vega-Lite of the
// score of each task, by section, with the actors (D31). The tasks go from
// left to right in the order of the source. Each section is a band behind
// its tasks, with its name at the top. The tasks before the first section
// have no band. Under the plot, each task has its
// name, and its actors under that.
func journeyChart(c *chart) (map[string]any, bool, error) {
	if err := c.unreadSettings(); err != nil {
		return nil, false, err
	}
	if c.d.Header != "" {
		c.line = c.d.N
		return nil, false, c.errorf("the header %q of a user journey", c.d.Header)
	}
	var title, section string
	var tasks []journeyTask
	for i := 0; i < len(c.d.Lines); i++ {
		l := c.d.Lines[i]
		c.line = l.N
		next, ok, err := c.skipAccessibility(c.d.Lines, i)
		if err != nil {
			return nil, false, err
		}
		if ok {
			i = next
			continue
		}
		if rest, ok := keyword(l.Text, "title"); ok {
			title = rest
			continue
		}
		if rest, ok := keyword(l.Text, "section"); ok {
			section = rest
			continue
		}
		parts := strings.SplitN(l.Text, ":", 3)
		if len(parts) < 2 {
			return nil, false, c.errorf("the line %q of a user journey", l.Text)
		}
		t := journeyTask{section: section, name: strings.TrimSpace(parts[0])}
		score, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil || t.name == "" {
			return nil, false, c.errorf("the task %q", l.Text)
		}
		if score < 1 || score > 5 {
			return nil, false, c.errorf("the score %v of the task %q is not from 1 to 5", score, t.name)
		}
		t.score = score
		if len(parts) == 3 {
			for a := range strings.SplitSeq(parts[2], ",") {
				if a = strings.TrimSpace(a); a != "" {
					t.actors = append(t.actors, a)
				}
			}
		}
		tasks = append(tasks, t)
	}
	c.line = 0
	if len(tasks) == 0 {
		return nil, false, c.errorf("a user journey with no task")
	}
	return journeySpec(title, tasks), true, nil
}

// journeySpec writes the spec of a user journey.
func journeySpec(title string, tasks []journeyTask) map[string]any {
	var rows, sections []any
	var band map[string]any
	var x0 float64
	for i, t := range tasks {
		if band == nil || t.section != band["section"] {
			if !slices.Contains(sections, any(t.section)) {
				sections = append(sections, t.section)
			}
			x0 = float64(i) - 0.5
			band = map[string]any{
				"kind": "section", "section": t.section, "label": t.section,
				"x0": x0, "y0": 0, "y1": 6,
			}
			rows = append(rows, band)
		}
		// The band of a section grows to its last task.
		band["x1"] = float64(i) + 0.5
		band["cx"] = (x0 + float64(i) + 0.5) / 2
		rows = append(rows, map[string]any{
			"kind": "task", "section": t.section, "label": t.name, "x": i,
			"score": t.score, "base": 0, "actors": strings.Join(t.actors, ", "),
		})
	}
	n := float64(len(tasks))
	step := max(560/n, 120)
	text := func(filter string, mark map[string]any, y any, field string) map[string]any {
		mark["type"] = "text"
		mark["align"] = "center"
		return map[string]any{
			"transform": []any{map[string]any{"filter": filter}},
			"mark":      mark,
			"encoding": map[string]any{
				"x":    map[string]any{"field": "x", "type": "quantitative"},
				"y":    y,
				"text": map[string]any{"field": field},
			},
		}
	}
	bottom := map[string]any{"datum": 0}
	layers := []any{
		map[string]any{
			"transform": []any{map[string]any{"filter": "datum.kind == 'section' && datum.label != ''"}},
			"mark":      map[string]any{"type": "rect", "opacity": 0.1},
			"encoding": map[string]any{
				"x":  map[string]any{"field": "x0", "type": "quantitative"},
				"x2": map[string]any{"field": "x1"},
				"y":  map[string]any{"field": "y0", "type": "quantitative"},
				"y2": map[string]any{"field": "y1"},
			},
		},
		map[string]any{
			"transform": []any{map[string]any{"filter": "datum.kind == 'section' && datum.label != ''"}},
			"mark": map[string]any{
				"type": "text", "align": "center", "baseline": "top", "dy": 8,
				"fontWeight": 600, "limit": step - 8,
			},
			"encoding": map[string]any{
				"x":    map[string]any{"field": "cx", "type": "quantitative"},
				"y":    map[string]any{"datum": 6},
				"text": map[string]any{"field": "label"},
			},
		},
		map[string]any{
			"transform": []any{map[string]any{"filter": "datum.kind == 'task'"}},
			"mark":      map[string]any{"type": "rule", "strokeDash": []any{3, 3}, "opacity": 0.6},
			"encoding": map[string]any{
				"y":  map[string]any{"field": "base", "type": "quantitative"},
				"y2": map[string]any{"field": "score"},
			},
		},
		map[string]any{
			"transform": []any{map[string]any{"filter": "datum.kind == 'task'"}},
			"mark":      map[string]any{"type": "circle", "size": 220, "opacity": 1},
		},
		text("datum.kind == 'task'", map[string]any{"baseline": "top", "dy": 8, "limit": step - 8}, bottom, "label"),
		text("datum.kind == 'task' && datum.actors != ''",
			map[string]any{"baseline": "top", "dy": 24, "fontSize": 10, "opacity": 0.7, "limit": step - 8}, bottom, "actors"),
	}
	spec := map[string]any{
		"width":  step * n,
		"height": 300,
		"data":   map[string]any{"values": rows},
		"encoding": map[string]any{
			"x": map[string]any{
				"field": "x", "type": "quantitative", "axis": nil,
				"scale": map[string]any{"domain": []any{-0.5, n - 0.5}, "nice": false, "zero": false},
			},
			"y": map[string]any{
				"field": "score", "type": "quantitative",
				"scale": map[string]any{"domain": []any{0, 6}, "nice": false},
				"axis":  map[string]any{"title": "Score", "values": []any{1, 2, 3, 4, 5}},
			},
			"color": map[string]any{
				"field": "section", "type": "nominal", "legend": nil,
				"scale": map[string]any{"domain": sections},
			},
		},
		"layer": layers,
	}
	if title != "" {
		spec["title"] = map[string]any{"text": title, "anchor": "start"}
	}
	return spec
}
