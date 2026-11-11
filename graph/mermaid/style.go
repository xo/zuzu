package mermaid

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/xo/zuzu/graph"
)

// dashed is the dash of a dashed line: a dashed relation of a class or
// an entity, and the border of a region of a state.
const dashed = "4,3"

// lookSetting reports a setting of the top level of the config that every
// graph reads: the layout, the look and the theme, which a graph draws in
// its own way, and htmlLabels and markdownAutoWrap, which change how a
// label wraps. It returns false for any other setting.
func lookSetting(is *graph.Issues, k string, v any) (bool, error) {
	switch k {
	case "layout":
		switch v {
		case "dagre":
			return true, nil
		case "elk":
			// ELK routes edges in its own way, so the layered layout
			// stands in for it (D31).
			return true, is.Add("config", "the layout elk, drawn with zuzu.layered")
		}
		return true, is.Add("config", "the layout %v", v)
	case "look":
		if v == "handDrawn" {
			return true, is.Add("config", "the look handDrawn")
		}
		return true, nil
	case "theme", "themeVariables", "themeCSS":
		return true, is.Add("config", "the %s of Mermaid, drawn in the look of zuzu", k)
	case "htmlLabels", "markdownAutoWrap":
		// labelWrap reads them.
		if _, ok := v.(bool); !ok {
			return true, is.Add("config", "the setting %s: %v", k, v)
		}
		return true, nil
	}
	return false, nil
}

// The widths in pixels at which Mermaid 12 wraps a label: the label of a
// node of a flowchart or of a state by default, and the label of an edge
// always.
const (
	nodeWrap = 120
	edgeWrap = 200
)

// wrapping is how the labels of a graph wrap, from the settings of the top
// level of the config.
type wrapping struct {
	// html is htmlLabels, true by default. Mermaid draws its labels in
	// HTML, which keeps a long word whole, or with htmlLabels: false in
	// SVG, which breaks it.
	html bool
	// markdown is markdownAutoWrap, true by default. When it is false, a
	// string of markdown in an HTML label does not wrap.
	markdown bool
}

// labelWrap reads the settings of the top level of a config that change
// how a label wraps, and sets them on a graph.
func labelWrap(cfg map[string]any, g *graph.Graph) wrapping {
	w := wrapping{html: true, markdown: true}
	if v, ok := cfg["htmlLabels"].(bool); ok {
		w.html = v
	}
	if v, ok := cfg["markdownAutoWrap"].(bool); ok {
		w.markdown = v
	}
	g.BreakWords = !w.html
	return w
}

// width returns the width at which a label wraps: limit, or 0 for a string
// of markdown that does not wrap.
func (w wrapping) width(limit float64, markdown bool) float64 {
	if markdown && w.html && !w.markdown {
		return 0
	}
	return limit
}

// wrapGraph sets the widths at which the labels of a class diagram, a
// state diagram or an entity relationship diagram wrap: the label of each
// node at node, or 0 for none, and the label of each edge at edgeWrap.
func wrapGraph(cfg map[string]any, g *graph.Graph, node float64) {
	labelWrap(cfg, g)
	for _, n := range g.Nodes {
		n.Wrap = node
	}
	for _, e := range g.Edges {
		e.Wrap = edgeWrap
	}
}

// wrapSetting reads the setting of a section of the config that sets the
// width at which a label wraps, such as flowchart.wrappingWidth, or returns
// def when the config does not set it. A value that is not a positive
// number is an issue.
func wrapSetting(is *graph.Issues, cfg map[string]any, def float64, section, key string) (float64, error) {
	v, ok := configNumber(cfg, section, key)
	if !ok {
		if m, _ := cfg[section].(map[string]any); m != nil && m[key] != nil {
			return def, is.Add("config", "the setting %s.%s: %v", section, key, m[key])
		}
		return def, nil
	}
	if !(v > 0) {
		return def, is.Add("config", "the setting %s.%s: %v", section, key, v)
	}
	return v, nil
}

// css reads a style of CSS of a class or a node, as fill:#f9f,stroke:#333,
// into the names of Vega. add records a property that zuzu does not draw.
func css(s string, add func(format string, args ...any) error) (map[string]string, error) {
	st := map[string]string{}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		k, v, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		v = strings.TrimSuffix(strings.TrimSpace(strings.TrimSuffix(v, "!important")), "px")
		switch k {
		case "fill":
			st["fill"], st["fillOpacity"] = v, "1"
		case "stroke":
			st["stroke"] = v
		case "stroke-width":
			st["strokeWidth"] = v
		case "color":
			st["color"] = v
		case "stroke-dasharray":
			st["strokeDash"] = strings.Join(strings.Fields(strings.ReplaceAll(v, ",", " ")), ",")
		case "font-weight":
			st["fontWeight"] = v
		case "font-size":
			st["fontSize"] = v
		default:
			if err := add("the style %s", k); err != nil {
				return nil, err
			}
		}
	}
	return st, nil
}

// classStyle returns the style of a node or a cluster: the class default,
// then each of its classes in order, then each style of own in order. A
// later style wins over an earlier one.
func classStyle(classes map[string]map[string]string, names []string, own ...map[string]string) map[string]string {
	st := maps.Clone(classes["default"])
	if st == nil {
		st = map[string]string{}
	}
	for _, c := range names {
		maps.Copy(st, classes[c])
	}
	for _, o := range own {
		maps.Copy(st, o)
	}
	return st
}

// sectionConfig reads the config of a diagram whose own section of the
// config, such as state or class, sets only the spacing and the look of
// Mermaid, so each setting of it is an issue, except the settings that
// known names, which the caller reads. A diagram of such a kind has
// nothing after its keyword.
func sectionConfig(d *Diagram, is *graph.Issues, section string, known ...string) error {
	cfg := d.Config
	for _, k := range slices.Sorted(maps.Keys(cfg)) {
		v := cfg[k]
		if ok, err := lookSetting(is, k, v); ok || err != nil {
			if err != nil {
				return err
			}
			continue
		}
		if m, ok := v.(map[string]any); ok && k == section {
			for _, sk := range slices.Sorted(maps.Keys(m)) {
				if slices.Contains(known, sk) {
					continue
				}
				if err := is.Add("config", "the setting %s.%s", section, sk); err != nil {
					return err
				}
			}
			continue
		}
		if err := is.Add("config", "the setting %s", k); err != nil {
			return err
		}
	}
	if strings.TrimSpace(d.Header) != "" {
		return fmt.Errorf("line %d: %q after %s: %w", d.N, d.Header, d.Kind, ErrSyntax)
	}
	return nil
}

// joinQuoted joins a line that opens a text in double quotes or in
// backticks and does not close it with the lines after it, up to the line
// that closes it, with a new line between them.
func joinQuoted(lines []Line) []Line {
	return joinOpen(lines, "\"`")
}

// joinOpen joins a line that opens a text with one of the marks and does
// not close it with the lines after it, up to the line that closes it, with
// a new line between them.
func joinOpen(lines []Line, marks string) []Line {
	open := func(s string) bool {
		for _, m := range marks {
			if strings.Count(s, string(m))%2 == 1 {
				return true
			}
		}
		return false
	}
	var out []Line
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		for open(l.Text) && i+1 < len(lines) {
			i++
			l.Text += "\n" + lines[i].Text
		}
		out = append(out, l)
	}
	return out
}

// readStatements reads the lines of the body of a class diagram or of an
// entity relationship diagram, joined by joinQuoted. It sets line to the
// number of each line, joins the text for a screen reader into acc, and
// passes each other line to body while inBody reports true, or else to
// stmt.
func readStatements(d *Diagram, line *int, acc *[]string, inBody func() bool, body, stmt func(string) error) error {
	lines := joinQuoted(d.Lines)
	for i := 0; i < len(lines); i++ {
		*line = lines[i].N
		t := lines[i].Text
		if inBody() {
			if err := body(t); err != nil {
				return err
			}
			continue
		}
		if accRE.MatchString(t) {
			text, n, ok := ganttAccText(lines, i)
			if !ok {
				return fmt.Errorf("line %d: an accDescr with no end brace: %w", *line, ErrSyntax)
			}
			i = n
			*acc = append(*acc, text)
			continue
		}
		if err := stmt(t); err != nil {
			return err
		}
	}
	return nil
}
