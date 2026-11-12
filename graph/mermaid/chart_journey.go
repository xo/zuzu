package mermaid

import (
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/text"
)

// journeyTask is a task of a user journey: its section, its name, its
// score and its actors. The score is a number from 1 to 5, or nil when the
// source gives no number.
type journeyTask struct {
	section, name string
	score         any
	actors        []string
}

// journeyChart compiles a user journey into points of Vega-Lite of the
// score of each task, by section, with the actors (D31). The tasks go from
// left to right in the order of the source. Each section is a band behind
// its tasks, with its name at the top. The tasks before the first section
// have no band. Under the plot, each task has its
// name, and its actors under that.
func journeyChart(c *chart) (map[string]any, bool, error) {
	// maxLabelWidth is the width at which Mermaid wraps the name of an
	// actor in its legend. zuzu has no legend of the actors, and wraps the
	// actors under each task at that width, scaled by scaleWrap, instead.
	// A width that is not a number stays an issue.
	wrap, wrapped := configNumber(c.d.Config, "journey", "maxLabelWidth")
	var read []string
	if wrapped {
		read = append(read, "journey.maxLabelWidth")
	}
	if err := c.unreadSettings(read...); err != nil {
		return nil, false, err
	}
	if wrapped && wrap <= 0 {
		if err := c.add("the setting journey.maxLabelWidth %v, which is not more than 0", wrap); err != nil {
			return nil, false, err
		}
		wrapped = false
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
		next, ok, err := c.accessibility(c.d.Lines, i)
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
		if t.name == "" {
			return nil, false, c.errorf("the task %q", l.Text)
		}
		text := strings.TrimSpace(parts[1])
		score, err := strconv.ParseFloat(text, 64)
		switch {
		case err != nil:
			// Mermaid reads a score that is not a number, such as size:
			// 2rem, as a task with no number, and draws its face in no
			// place.
			if err := c.add("the score %q of the task %q is not a number, so the task has no face", text, t.name); err != nil {
				return nil, false, err
			}
		case score < 1 || score > 5:
			return nil, false, c.errorf("the score %v of the task %q is not from 1 to 5", score, t.name)
		default:
			t.score = score
		}
		if len(parts) == 3 {
			for a := range strings.SplitSeq(parts[2], ",") {
				if a = strings.TrimSpace(a); a != "" {
					t.actors = append(t.actors, a)
				}
			}
		}
		tasks = append(tasks, t)
	}
	if len(tasks) == 0 {
		// Mermaid draws the title of a user journey with no task.
		return emptyChart(title, ""), false, nil
	}
	if !wrapped {
		wrap = 0
	}
	wrap = scaleWrap(c.d.Kind, wrap)
	return journeySpec(title, tasks, wrap), true, nil
}

// journeySpec writes the spec of a user journey. When wrap is more than 0,
// the actors under a task wrap at that width, or at the width of the
// column of the task if it is less.
func journeySpec(title string, tasks []journeyTask, wrap float64) map[string]any {
	n := float64(len(tasks))
	step := max(560/n, 120)
	var rows, sections []any
	var band map[string]any
	var x0 float64
	lines := 0
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
		row := map[string]any{
			"kind": "task", "section": t.section, "label": t.name, "x": i,
			"score": t.score, "base": 0, "actors": strings.Join(t.actors, ", "),
		}
		rows = append(rows, row)
		if wrap > 0 && len(t.actors) > 0 {
			// Each line of the actors is a row of its own, so that each
			// draws as a text of one line.
			for k, l := range journeyWrap(strings.Join(t.actors, ", "), min(wrap, step-8)) {
				rows = append(rows, map[string]any{"kind": "actors", "section": t.section, "x": i, "line": k, "label": l})
				lines = max(lines, k+1)
			}
		}
	}
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
	}
	if wrap > 0 {
		// The lines of the actors are 13 apart.
		for k := range lines {
			layers = append(layers, text("datum.kind == 'actors' && datum.line == "+strconv.Itoa(k),
				map[string]any{"baseline": "top", "dy": 24 + 13*k, "fontSize": journeyActorSize, "opacity": 0.7},
				bottom, "label"))
		}
	} else {
		layers = append(layers, text("datum.kind == 'task' && datum.actors != ''",
			map[string]any{"baseline": "top", "dy": 24, "fontSize": journeyActorSize, "opacity": 0.7, "limit": step - 8}, bottom, "actors"))
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

// journeyActorSize is the font size of the actors under a task.
const journeyActorSize = 10

// journeyWrap wraps a text into lines no wider than a width, as Mermaid
// wraps the name of an actor: a text that fits stays one line. Else each
// word goes on the line before it when the line still fits, and on a new
// line when not. A word wider than the width breaks into parts that each
// end with a hyphen. zuzu measures the text in the bundled font.
func journeyWrap(s string, width float64) []string {
	m := text.NewEstimate()
	font := text.Font{Family: text.Family, Size: journeyActorSize, Weight: 400}
	fits := func(t string) bool { return m.Width(font, t) <= width }
	if fits(s) {
		return []string{s}
	}
	var lines []string
	line := ""
	for word := range strings.SplitSeq(s, " ") {
		test := word
		if line != "" {
			test = line + " " + word
		}
		if fits(test) {
			line = test
			continue
		}
		if line != "" {
			lines = append(lines, line)
		}
		line = word
		if fits(word) {
			continue
		}
		// The word breaks before the first letter that makes the part
		// with its hyphen too wide. A part keeps at least one letter.
		part := ""
		for _, r := range word {
			if part != "" && !fits(part+string(r)+"-") {
				lines = append(lines, part+"-")
				part = ""
			}
			part += string(r)
		}
		line = part
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
