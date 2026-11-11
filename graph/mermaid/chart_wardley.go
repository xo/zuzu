package mermaid

import (
	"fmt"
	"maps"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	// wardleyAt matches a thing of a map with its place, [visibility,
	// evolution], and the rest of the line.
	wardleyAt = regexp.MustCompile(`^(.+?)\s*\[\s*([-+\d.]+)\s*,\s*([-+\d.]+)\s*\](.*)$`)
	// wardleyStage matches a component of a pipeline, with its evolution
	// alone.
	wardleyStage = regexp.MustCompile(`^(.+?)\s*\[\s*([-+\d.]+)\s*\](.*)$`)
	// wardleyLabel matches the offset of a label, label [x, y].
	wardleyLabel = regexp.MustCompile(`^label\s*\[\s*([-+\d.]+)\s*,\s*([-+\d.]+)\s*\]`)
	// wardleyLink matches a link: a name, an arrow and a name, with a label
	// after a semicolon.
	wardleyLink = regexp.MustCompile(`^(.+?)\s*(-\.->|-->|->|\+<>|\+<|\+'[^']*'>|\+>)\s*(.+?)(?:\s*;\s*(.*))?$`)
	// wardleyAnnotation matches a numbered annotation, N,[v, e] "text".
	wardleyAnnotation = regexp.MustCompile(`^(\d+)\s*,\s*\[\s*([-+\d.]+)\s*,\s*([-+\d.]+)\s*\]\s*(?:"([^"]*)")?$`)
	// wardleyText matches a text in quotes and a place, as a note has.
	wardleyText = regexp.MustCompile(`^"([^"]*)"\s*\[\s*([-+\d.]+)\s*,\s*([-+\d.]+)\s*\]$`)
)

// wardleyThing is a component or an anchor of a map.
type wardleyThing struct {
	name       string
	anchor     bool
	vis, evo   float64
	dx, dy     float64
	decorators []string
	line       int
}

// wardleyLine is a link between two components, or an evolution of a
// component, from x, y to x2, y2. A link has an arrowhead at its target,
// its head, or at its source, its tail.
type wardleyLine struct {
	kind, name     string
	source, target string
	label          string
	x, y, x2, y2   float64
	dashed         bool
	head, tail     bool
}

// row returns the row of a line for the spec.
func (l *wardleyLine) row() map[string]any {
	r := map[string]any{"kind": l.kind, "x": l.x, "y": l.y, "x2": l.x2, "y2": l.y2}
	if l.kind == "link" {
		r["source"], r["target"] = l.source, l.target
	} else {
		r["name"] = l.name
	}
	return r
}

// wardleyPipeline is a pipeline: its component, and the evolutions of its
// own components, from lo to hi.
type wardleyPipeline struct {
	name        string
	line        int
	lo, hi, vis float64
	full        bool
}

// wardleyMap is a Wardley map while the compiler reads it.
type wardleyMap struct {
	things      []*wardleyThing
	byName      map[string]*wardleyThing
	links       []*wardleyLine
	evolves     []*wardleyLine
	pipelines   []*wardleyPipeline
	notes       []map[string]any
	forces      []map[string]any
	annotations []map[string]any
	// list is the place of the list of the annotations.
	list          []float64
	stages        []string
	bounds        []float64
	width, height float64
}

// wardleyChart compiles a Wardley map: components on the two axes of
// evolution and visibility, with the links of the value chain, into a
// Vega-Lite spec of points, lines and text (D31). A place in the source
// is [visibility, evolution], so the evolution is x and the visibility y.
func wardleyChart(c *chart) (map[string]any, bool, error) {
	if _, err := c.settings("wardley", nil); err != nil {
		return nil, false, err
	}
	if c.d.Header != "" {
		return nil, false, c.errorf("the text %q after %s", c.d.Header, c.d.Kind)
	}
	w := &wardleyMap{
		byName: map[string]*wardleyThing{}, width: 720, height: 440,
		stages: []string{"Genesis", "Custom Built", "Product", "Commodity"},
		bounds: []float64{0.25, 0.5, 0.75, 1},
	}
	spec := map[string]any{}
	var linkLines []Line
	var evolveLines []Line
	pipeline := ""
	for _, l := range c.d.Lines {
		c.line = l.N
		t := l.Text
		if pipeline != "" {
			if t == "}" {
				pipeline = ""
				continue
			}
			if err := c.wardleyStage(w, pipeline, t); err != nil {
				return nil, false, err
			}
			continue
		}
		word, rest, _ := strings.Cut(t, " ")
		rest = strings.TrimSpace(rest)
		var err error
		switch word {
		case "title":
			spec["title"] = map[string]any{"text": rest, "anchor": "start"}
		case "size":
			var v []float64
			if v, err = c.wardleyNumbers(rest, 2); err == nil {
				if v[0] <= 0 || v[1] <= 0 {
					return nil, false, c.errorf("the size %s is not above 0", rest)
				}
				w.width, w.height = v[0], v[1]
			}
		case "evolution":
			err = c.wardleyStages(w, rest)
		case "component", "anchor":
			err = c.wardleyThing(w, word == "anchor", rest)
		case "pipeline":
			name, ok := strings.CutSuffix(rest, "{")
			name = unquoteName(strings.TrimSpace(name))
			if !ok || name == "" {
				return nil, false, c.errorf("a pipeline is pipeline Name {")
			}
			pipeline = name
			w.pipelines = append(w.pipelines, &wardleyPipeline{name: name, line: l.N})
		case "evolve":
			evolveLines = append(evolveLines, l)
		case "note":
			err = c.wardleyNote(w, rest)
		case "accelerator", "deaccelerator":
			m := wardleyText.FindStringSubmatch(rest)
			if m == nil {
				return nil, false, c.errorf("an %s is %s \"text\" [visibility, evolution]", word, word)
			}
			vis, evo, err := c.wardleyPlace(m[2], m[3])
			if err != nil {
				return nil, false, err
			}
			shape, dx := "triangle-right", 1.0
			if word == "deaccelerator" {
				shape, dx = "triangle-left", -1
			}
			w.forces = append(w.forces, map[string]any{"kind": word, "name": m[1], "x": evo, "y": vis, "shape": shape, "dx": dx})
		case "annotations":
			w.list, err = c.wardleyNumbers(rest, 2)
		case "annotation":
			err = c.wardleyAnnotation(w, rest)
		default:
			if ok, err := c.accText(t); ok || err != nil {
				if err != nil {
					return nil, false, err
				}
				continue
			}
			if !wardleyLink.MatchString(t) {
				return nil, false, c.errorf("the statement %q", t)
			}
			linkLines = append(linkLines, l)
		}
		if err != nil {
			return nil, false, err
		}
	}
	if pipeline != "" {
		return nil, false, c.errorf("the pipeline %s has no closing brace", pipeline)
	}
	for _, l := range linkLines {
		c.line = l.N
		if err := c.wardleyLink(w, l.Text); err != nil {
			return nil, false, err
		}
	}
	for _, l := range evolveLines {
		c.line = l.N
		if err := c.wardleyEvolve(w, strings.TrimSpace(strings.TrimPrefix(l.Text, "evolve"))); err != nil {
			return nil, false, err
		}
	}
	for _, p := range w.pipelines {
		c.line = p.line
		if _, ok := w.byName[p.name]; !ok {
			return nil, false, c.errorf("the pipeline %s names no component", p.name)
		}
	}
	c.line = 0
	maps.Copy(spec, w.layers())
	return spec, true, nil
}

// wardleyStages reads the stages of evolution: names between arrows, each
// with the end of its stage after an at sign.
func (c *chart) wardleyStages(w *wardleyMap, s string) error {
	parts := strings.Split(s, "->")
	if len(parts) < 2 {
		return c.errorf("the evolution %q has fewer than two stages", s)
	}
	w.stages, w.bounds = nil, nil
	given := false
	for i, p := range parts {
		name, at, ok := strings.Cut(strings.TrimSpace(p), "@")
		name = strings.TrimSpace(name)
		if name == "" {
			return c.errorf("a stage of the evolution with no name")
		}
		end := float64(i+1) / float64(len(parts))
		if ok {
			f, err := strconv.ParseFloat(strings.TrimSpace(at), 64)
			if err != nil {
				return c.errorf("the end %q of the stage %s is not a number", at, name)
			}
			end, given = f, true
		} else if given {
			return c.errorf("the stage %s has no end, and the stages before it have one", name)
		}
		if len(w.bounds) > 0 && end <= w.bounds[len(w.bounds)-1] {
			return c.errorf("the end %v of the stage %s is not after the end of the stage before it", end, name)
		}
		w.stages = append(w.stages, name)
		w.bounds = append(w.bounds, end)
	}
	return nil
}

// wardleyThing reads a component or an anchor: a name, a place, the
// offset of its label and its decorators.
func (c *chart) wardleyThing(w *wardleyMap, anchor bool, s string) error {
	m := wardleyAt.FindStringSubmatch(s)
	if m == nil {
		return c.errorf("a component is component Name [visibility, evolution]")
	}
	vis, evo, err := c.wardleyPlace(m[2], m[3])
	if err != nil {
		return err
	}
	t := &wardleyThing{name: unquoteName(m[1]), anchor: anchor, vis: vis, evo: evo, line: c.line}
	if err := c.wardleyExtras(t, m[4]); err != nil {
		return err
	}
	return c.wardleyAdd(w, t)
}

// wardleyStage reads a component of a pipeline, which has the visibility
// of the pipeline and its own evolution.
func (c *chart) wardleyStage(w *wardleyMap, pipeline, s string) error {
	rest, ok := strings.CutPrefix(s, "component ")
	m := wardleyStage.FindStringSubmatch(strings.TrimSpace(rest))
	if !ok || m == nil {
		return c.errorf("a component of a pipeline is component Name [evolution]")
	}
	parent, found := w.byName[pipeline]
	if !found {
		return c.errorf("the pipeline %s names no component before it", pipeline)
	}
	evo, err := strconv.ParseFloat(m[2], 64)
	if err != nil {
		return c.errorf("the evolution %q is not a number", m[2])
	}
	t := &wardleyThing{name: unquoteName(m[1]), vis: parent.vis, evo: evo, line: c.line}
	if err := c.wardleyExtras(t, m[3]); err != nil {
		return err
	}
	p := w.pipelines[len(w.pipelines)-1]
	if !p.full {
		p.lo, p.hi, p.full = evo, evo, true
	}
	p.lo, p.hi, p.vis = math.Min(p.lo, evo), math.Max(p.hi, evo), parent.vis
	return c.wardleyAdd(w, t)
}

// wardleyExtras reads what follows the place of a component: the offset
// of its label and its decorators in parentheses.
func (c *chart) wardleyExtras(t *wardleyThing, s string) error {
	t.dx, t.dy = 6, -4
	s = strings.TrimSpace(s)
	for s != "" {
		if m := wardleyLabel.FindStringSubmatch(s); m != nil {
			dx, err1 := strconv.ParseFloat(m[1], 64)
			dy, err2 := strconv.ParseFloat(m[2], 64)
			if err1 != nil || err2 != nil {
				return c.errorf("the label offset %q", m[0])
			}
			t.dx, t.dy = dx, dy
			s = strings.TrimSpace(s[len(m[0]):])
			continue
		}
		if strings.HasPrefix(s, "(") {
			end := strings.Index(s, ")")
			if end < 0 {
				return c.errorf("a decorator with no closing parenthesis")
			}
			d := strings.TrimSpace(s[1:end])
			switch d {
			case "build", "buy", "outsource", "market", "inertia":
				t.decorators = append(t.decorators, d)
			default:
				return c.errorf("the decorator %q", d)
			}
			s = strings.TrimSpace(s[end+1:])
			continue
		}
		return c.errorf("the text %q after a component", s)
	}
	return nil
}

// wardleyAdd adds a component or an anchor to a map.
func (c *chart) wardleyAdd(w *wardleyMap, t *wardleyThing) error {
	if t.name == "" {
		return c.errorf("a component with no name")
	}
	if _, ok := w.byName[t.name]; ok {
		return c.errorf("the component %s is defined twice", t.name)
	}
	w.byName[t.name] = t
	w.things = append(w.things, t)
	return nil
}

// wardleyLink reads a link between two components. A flow, +>, has an
// arrow at its target, +< at its source and +<> at both, and -.-> is
// dashed.
func (c *chart) wardleyLink(w *wardleyMap, s string) error {
	m := wardleyLink.FindStringSubmatch(s)
	from, to := w.byName[unquoteName(m[1])], w.byName[unquoteName(m[3])]
	for i, t := range []*wardleyThing{from, to} {
		if t == nil {
			return c.errorf("the link names %s, which no component defines", unquoteName(m[1+2*i]))
		}
	}
	link := &wardleyLine{
		kind: "link", source: from.name, target: to.name, label: m[4],
		x: from.evo, y: from.vis, x2: to.evo, y2: to.vis, dashed: m[2] == "-.->",
	}
	switch op := m[2]; {
	case op == "+<>":
		link.head, link.tail = true, true
	case op == "+<":
		link.tail = true
	case strings.HasPrefix(op, "+"):
		link.head = true
		if text := strings.Trim(op[1:len(op)-1], "'"); text != "" {
			if m[4] != "" {
				return c.errorf("a link with two labels")
			}
			link.label = text
		}
	}
	w.links = append(w.links, link)
	return nil
}

// wardleyEvolve reads the evolution of a component to a new evolution.
func (c *chart) wardleyEvolve(w *wardleyMap, s string) error {
	name, to, ok := strings.CutLast(s, " ")
	if !ok {
		return c.errorf("an evolve is evolve Name evolution")
	}
	t := w.byName[unquoteName(name)]
	if t == nil {
		return c.errorf("the evolve names %s, which no component defines", unquoteName(name))
	}
	evo, err := strconv.ParseFloat(to, 64)
	if err != nil {
		return c.errorf("the evolution %q is not a number", to)
	}
	w.evolves = append(w.evolves, &wardleyLine{kind: "evolve", name: t.name, x: t.evo, x2: evo, y: t.vis, y2: t.vis})
	return nil
}

// wardleyNote reads a note, a text at a place.
func (c *chart) wardleyNote(w *wardleyMap, s string) error {
	m := wardleyText.FindStringSubmatch(s)
	if m == nil {
		return c.errorf("a note is note \"text\" [visibility, evolution]")
	}
	vis, evo, err := c.wardleyPlace(m[2], m[3])
	if err != nil {
		return err
	}
	w.notes = append(w.notes, map[string]any{"kind": "note", "text": m[1], "x": evo, "y": vis})
	return nil
}

// wardleyAnnotation reads a numbered annotation, its place and its text.
func (c *chart) wardleyAnnotation(w *wardleyMap, s string) error {
	m := wardleyAnnotation.FindStringSubmatch(s)
	if m == nil {
		return c.errorf("an annotation is annotation N,[visibility, evolution] \"text\"")
	}
	vis, evo, err := c.wardleyPlace(m[2], m[3])
	if err != nil {
		return err
	}
	w.annotations = append(w.annotations, map[string]any{"kind": "annotation", "number": m[1], "text": m[4], "x": evo, "y": vis})
	return nil
}

// wardleyPlace reads a place, [visibility, evolution].
func (c *chart) wardleyPlace(vis, evo string) (float64, float64, error) {
	v, err1 := strconv.ParseFloat(vis, 64)
	e, err2 := strconv.ParseFloat(evo, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, c.errorf("the place [%s, %s] is not two numbers", vis, evo)
	}
	return v, e, nil
}

// wardleyNumbers reads a list of n numbers in brackets, as [1100, 600].
func (c *chart) wardleyNumbers(s string, n int) ([]float64, error) {
	inner, ok := strings.CutPrefix(strings.TrimSpace(s), "[")
	inner, ok2 := strings.CutSuffix(inner, "]")
	parts := strings.Split(inner, ",")
	if !ok || !ok2 || len(parts) != n {
		return nil, c.errorf("%q is not a list of %d numbers in brackets", s, n)
	}
	out := make([]float64, n)
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, c.errorf("%q is not a number", p)
		}
		out[i] = f
	}
	return out, nil
}

// unquoteName removes the double quotes around a name.
func unquoteName(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// layers returns the size, the encoding and the layers of the spec of a
// map. A place is in the units of the axes, from 0 to 1, so an offset in
// pixels divides by the size of the plot.
func (w *wardleyMap) layers() map[string]any {
	px, py := 1/w.width, 1/w.height
	var layer []any
	add := func(mark map[string]any, values []map[string]any, enc map[string]any) {
		if len(values) == 0 {
			return
		}
		rows := make([]any, len(values))
		for i, v := range values {
			rows[i] = v
		}
		if enc == nil {
			enc = map[string]any{}
		}
		layer = append(layer, map[string]any{"data": map[string]any{"values": rows}, "mark": mark, "encoding": enc})
	}
	q := func(f string) map[string]any { return map[string]any{"field": f} }
	opaque := map[string]any{"opacity": map[string]any{"value": 1}}
	// The stages of evolution: their names under the plot, in the middle
	// of each stage. The grid of the axis of x marks their ends.
	var stageRows []map[string]any
	start := 0.0
	for i, name := range w.stages {
		stageRows = append(stageRows, map[string]any{"kind": "stage", "text": name, "x": (start + w.bounds[i]) / 2, "y": 0})
		start = w.bounds[i]
	}
	add(map[string]any{"type": "text", "baseline": "top", "dy": 8, "fontSize": 11}, stageRows, map[string]any{"text": q("text")})
	add(map[string]any{"type": "text", "baseline": "top", "dy": 26, "fontSize": 11, "fontWeight": 600},
		[]map[string]any{{"kind": "axis", "text": "Evolution", "x": 0.5, "y": 0}}, map[string]any{"text": q("text")})
	// The pipelines, as boxes around their components.
	var boxes []map[string]any
	for _, p := range w.pipelines {
		if !p.full {
			continue
		}
		boxes = append(boxes, map[string]any{
			"kind": "pipeline", "name": p.name, "x": p.lo - 12*px, "x2": p.hi + 12*px,
			"y": p.vis - 10*py, "y2": p.vis + 10*py,
		})
	}
	add(map[string]any{"type": "rect", "filled": false, "fill": "transparent", "cornerRadius": 4, "strokeWidth": 1.25, "opacity": 0.6}, boxes,
		map[string]any{"x2": q("x2"), "y2": q("y2")})
	// The links, solid or dashed, with an arrow at each end of a flow.
	var solid, dashed, heads, labels []map[string]any
	for _, l := range w.links {
		if l.dashed {
			dashed = append(dashed, l.row())
		} else {
			solid = append(solid, l.row())
		}
		if l.head {
			heads = append(heads, w.arrow(l, l.x, l.y, l.x2, l.y2))
		}
		if l.tail {
			heads = append(heads, w.arrow(l, l.x2, l.y2, l.x, l.y))
		}
		if l.label != "" {
			labels = append(labels, map[string]any{"kind": "linkLabel", "text": l.label, "x": (l.x + l.x2) / 2, "y": (l.y + l.y2) / 2})
		}
	}
	rule := map[string]any{"type": "rule", "strokeWidth": 1.25, "opacity": 0.55}
	ends := map[string]any{"x2": q("x2"), "y2": q("y2")}
	add(rule, solid, ends)
	add(map[string]any{"type": "rule", "strokeWidth": 1.25, "opacity": 0.55, "strokeDash": []any{4, 3}}, dashed, ends)
	add(map[string]any{"type": "point", "shape": "triangle", "filled": true, "size": 50}, heads, map[string]any{
		"angle": map[string]any{"field": "angle", "type": "quantitative", "scale": nil}, "opacity": opaque["opacity"],
	})
	add(map[string]any{"type": "text", "fontSize": 10, "dy": -6}, labels, map[string]any{"text": q("text")})
	// The evolutions: a dashed line to the new evolution, with an arrow,
	// in the second color of the theme.
	evolveColor := map[string]any{"datum": "evolve", "scale": map[string]any{"domain": []any{"map", "evolve"}}, "legend": nil}
	var evolves, evolveHeads []map[string]any
	for _, e := range w.evolves {
		evolves = append(evolves, e.row())
		evolveHeads = append(evolveHeads, w.arrow(e, e.x, e.y, e.x2, e.y2))
	}
	add(map[string]any{"type": "rule", "strokeWidth": 1.25, "strokeDash": []any{4, 3}}, evolves, map[string]any{"x2": q("x2"), "color": evolveColor})
	add(map[string]any{"type": "point", "shape": "triangle", "filled": true, "size": 50}, evolveHeads, map[string]any{
		"angle": map[string]any{"field": "angle", "type": "quantitative", "scale": nil}, "opacity": opaque["opacity"], "color": evolveColor,
	})
	// The decorators: a tint of a shape behind a component, and a bar at
	// the right of a component with inertia.
	shapes := map[string]string{"build": "triangle-up", "buy": "diamond", "outsource": "square", "market": "circle"}
	var tints, inertia, points, names, anchors []map[string]any
	for _, t := range w.things {
		for _, d := range t.decorators {
			if d == "inertia" {
				inertia = append(inertia, map[string]any{"kind": "inertia", "name": t.name, "x": t.evo + 14*px, "y": t.vis - 10*py, "y2": t.vis + 10*py})
				continue
			}
			tints = append(tints, map[string]any{"kind": d, "name": t.name, "x": t.evo, "y": t.vis, "shape": shapes[d]})
		}
		label := map[string]any{"kind": "label", "name": t.name, "text": t.name, "x": t.evo + t.dx*px, "y": t.vis - t.dy*py}
		if t.anchor {
			label["kind"], label["x"], label["y"] = "anchor", t.evo, t.vis
			anchors = append(anchors, label)
			continue
		}
		points = append(points, map[string]any{"kind": "component", "name": t.name, "x": t.evo, "y": t.vis})
		names = append(names, label)
	}
	add(map[string]any{"type": "point", "filled": true, "size": 420}, tints, map[string]any{
		"shape": map[string]any{"field": "shape", "type": "nominal", "scale": nil}, "opacity": map[string]any{"value": 0.25},
	})
	add(map[string]any{"type": "rule", "strokeWidth": 4}, inertia, map[string]any{"y2": q("y2")})
	add(map[string]any{"type": "point", "filled": true, "size": 70}, points, opaque)
	add(map[string]any{"type": "text", "fontSize": 11, "align": "left", "baseline": "bottom"}, names, map[string]any{"text": q("text")})
	add(map[string]any{"type": "text", "fontSize": 12, "fontWeight": 600}, anchors, map[string]any{"text": q("text")})
	// The forces, notes and annotations.
	var forceNames []map[string]any
	for _, f := range w.forces {
		forceNames = append(forceNames, map[string]any{"kind": "forceLabel", "text": f["name"], "x": f["x"], "y": f["y"]})
	}
	add(map[string]any{"type": "point", "filled": true, "size": 900}, w.forces, map[string]any{
		"shape": map[string]any{"field": "shape", "type": "nominal", "scale": nil}, "opacity": map[string]any{"value": 0.3},
	})
	add(map[string]any{"type": "text", "fontSize": 11, "fontWeight": 600, "baseline": "top", "dy": 20}, forceNames, map[string]any{"text": q("text")})
	add(map[string]any{"type": "text", "fontSize": 11, "fontWeight": 600, "align": "left"}, w.notes, map[string]any{"text": q("text")})
	add(map[string]any{"type": "point", "filled": true, "size": 260}, w.annotations, map[string]any{"opacity": map[string]any{"value": 0.2}})
	add(map[string]any{"type": "text", "fontSize": 10, "fontWeight": 600, "baseline": "middle"}, w.annotations, map[string]any{"text": q("number")})
	if len(w.annotations) > 0 {
		vis, evo := 0.97, 0.02
		if len(w.list) == 2 {
			vis, evo = w.list[0], w.list[1]
		}
		var list []map[string]any
		for i, a := range w.annotations {
			list = append(list, map[string]any{
				"kind": "annotationText", "number": a["number"], "text": fmt.Sprint(a["number"], ". ", a["text"]),
				"x": evo, "y": vis - float64(i)*15*py,
			})
		}
		add(map[string]any{"type": "text", "fontSize": 11, "align": "left", "baseline": "top"}, list, map[string]any{"text": q("text")})
	}
	axis := func(title any, values []float64, grid bool) map[string]any {
		vals := make([]any, len(values))
		for i, v := range values {
			vals[i] = v
		}
		return map[string]any{"title": title, "labels": false, "ticks": false, "grid": grid, "values": vals}
	}
	var inner []float64
	for _, b := range w.bounds {
		if b > 0 && b < 1 {
			inner = append(inner, b)
		}
	}
	return map[string]any{
		"width":  w.width,
		"height": w.height,
		"encoding": map[string]any{
			"x": map[string]any{"field": "x", "type": "quantitative", "scale": map[string]any{"domain": []any{0, 1}}, "axis": axis(nil, inner, true)},
			"y": map[string]any{"field": "y", "type": "quantitative", "scale": map[string]any{"domain": []any{0, 1}}, "axis": axis("Visibility", nil, false)},
		},
		"layer": layer,
	}
}

// arrow returns the row of an arrowhead at the end of a line from one
// place to another, set back by the radius of a point and turned along
// the line in pixels.
func (w *wardleyMap) arrow(of *wardleyLine, x1, y1, x2, y2 float64) map[string]any {
	dx, dy := (x2-x1)*w.width, (y1-y2)*w.height
	d := math.Hypot(dx, dy)
	if d == 0 {
		d = 1
	}
	back := 7.0
	r := of.row()
	r["kind"], r["of"] = "arrow", of.kind
	r["x"], r["y"] = x2-dx/d*back/w.width, y2+dy/d*back/w.height
	r["angle"] = math.Mod(math.Atan2(dx, -dy)*180/math.Pi+360, 360)
	delete(r, "x2")
	delete(r, "y2")
	return r
}
