package mermaid

import (
	"encoding/json/jsontext"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// dashed is the dash of a dashed line: a dashed relation of a class or
// an entity, and the border of a region of a state.
const dashed = "4,3"

// animatedDash is the dash of a link that Mermaid animates, with animate
// or animation, as mmdc 12.3.0 draws it: 9 pixels of line and 5 of gap.
// zuzu draws one still frame of it (D32). The dash of zuzu, 4,3, is half
// as long, so the link takes the dash of Mermaid.
const animatedDash = "9,5"

// lookSetting reports a setting of the top level of the config that every
// graph reads: the layout and the look, which a graph draws in its own
// way, the theme, which Look reads, htmlLabels and markdownAutoWrap, which
// change how a label wraps, and the settings that change nothing in an
// image. It returns false for any other setting.
func lookSetting(is *graph.Issues, kind, k string, v any) (bool, error) {
	if inert(kind, k) || hint(k, v) {
		return true, nil
	}
	switch k {
	case "layout":
		// A swimlane diagram, and a flowchart, can lay out in lanes.
		if v == "dagre" || v == "swimlane" && laneKind(kind) {
			return true, nil
		}
		return true, is.Add("config", "the layout %v", v)
	case "look":
		return true, nil
	case "theme", "themeVariables", "themeCSS":
		if k == "theme" && knownTheme(v) {
			return true, nil
		}
		return true, is.Add("config", "the %s of Mermaid, drawn in the look of zuzu", k)
	case "fontFamily":
		// Graph gives each label the weight and the style that the name
		// gives, and zuzu draws the rest of the name in IBM Plex Sans
		// (D32).
		return true, nil
	case "htmlLabels", "markdownAutoWrap":
		// labelWrap reads them.
		if _, ok := v.(bool); !ok {
			return true, is.Add("config", "the setting %s: %v", k, v)
		}
		return true, nil
	}
	return false, nil
}

// themes are the themes of Mermaid 12.3.0. A theme with dark in its name
// selects the dark mode of zuzu, and each other theme selects the scheme
// of zuzu with its colors, named mermaid- and the name of the theme (D32).
var themes = []string{
	"default", "base", "neutral", "forest", "dark", "neo", "neo-dark",
	"redux", "redux-dark", "redux-color", "redux-dark-color",
}

// knownTheme reports whether a value of the setting theme names a theme
// of Mermaid 12.3.0. Any other value stays an issue.
func knownTheme(v any) bool {
	s, ok := v.(string)
	return ok && slices.Contains(themes, s)
}

// Look returns the keys of the Vega config that the theme of a diagram
// asks for (D32): zuzu.mode dark for a theme with dark in its name, and
// zuzu.scheme with the scheme of zuzu that has the colors of each other
// theme. The runtime takes each one where the caller of a render sets no
// mode or no scheme. Look returns nil for a diagram with no theme or with a
// theme that Mermaid 12.3.0 does not have.
func Look(d *Diagram) vega.Config {
	name, _ := d.Config["theme"].(string)
	if !knownTheme(name) {
		return nil
	}
	if strings.Contains(name, "dark") {
		return vega.Config{theme.ConfigMode: jsontext.Value(`"dark"`)}
	}
	return vega.Config{theme.ConfigScheme: jsontext.Value(strconv.Quote("mermaid-" + name))}
}

// hint reports whether a setting of the top level of the config is a hint
// of a way to draw that zuzu accepts with no issue in every kind (D32):
// the look handDrawn, which zuzu draws in its own look, an ELK layout and
// the section elk of its settings, which zuzu draws with zuzu.layered, and
// the layout dagre, the default of Mermaid.
func hint(k string, v any) bool {
	switch k {
	case "look":
		return v == "handDrawn"
	case "layout":
		s, _ := v.(string)
		return s == "dagre" || slices.Contains(elkLayouts, s)
	case "elk":
		return true
	}
	return false
}

// elkLayouts are the ELK layouts that Mermaid 12.3 has: elk, which is the
// layered layout of ELK, and the other algorithms of ELK.
var elkLayouts = []string{"elk", "elk.stress", "elk.force", "elk.mrtree", "elk.sporeOverlap", "elk.box", "elk.rectpacking"}

// graphSection reports each setting of a section of the config of a graph,
// such as flowchart.curve, except the settings that read names, which the
// caller reads, the settings that change nothing in an image, and
// flowchart.htmlLabels, which labelWrap reads. A value that is not a map
// is one issue.
func graphSection(is *graph.Issues, kind, section string, v any, read ...string) error {
	m, ok := v.(map[string]any)
	if !ok {
		return is.Add("config", "the setting %s", section)
	}
	for _, k := range slices.Sorted(maps.Keys(m)) {
		path := section + "." + k
		var err error
		switch {
		case slices.Contains(read, k), accepted(kind, path, m[k]):
		case path == "flowchart.htmlLabels":
			if _, ok := m[k].(bool); !ok {
				err = is.Add("config", "the setting %s: %v", path, m[k])
			}
		default:
			err = is.Add("config", "the setting %s", path)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// isGraph reports whether a kind of diagram reads into a graph.
func isGraph(kind string) bool {
	switch kind {
	case "flowchart", "graph", "flowchart-elk", "stateDiagram", "stateDiagram-v2",
		"classDiagram", "classDiagram-v2", "erDiagram", "requirementDiagram", "agentflow-beta", "usecase-beta", "swimlane-beta":
		return true
	}
	return false
}

// accepted reports whether zuzu accepts a setting of the config with no
// issue: a setting that inert names, a theme that Look reads, a hint at
// the top level of the config, such as layout: elk, or a font family with
// no weight and no style, such as fontFamily: Arial or
// sequence.actorFontFamily: courier. zuzu draws in IBM Plex Sans only, so the name of a family
// changes nothing (D32).
func accepted(kind, path string, v any) bool {
	if inert(kind, path) || path == "theme" && knownTheme(v) || !strings.Contains(path, ".") && hint(path, v) {
		return true
	}
	key := strings.ToLower(path[strings.LastIndex(path, ".")+1:])
	if !strings.HasSuffix(key, "fontfamily") {
		return false
	}
	s, ok := v.(string)
	if !ok {
		return false
	}
	weight, style := graph.FontFace(s)
	return weight == "" && style == ""
}

// inert reports whether a setting changes nothing in a still image of a
// kind of diagram, so that zuzu accepts it with no issue. path is the keys
// of the setting joined with dots, such as er.useMaxWidth. The names match
// in any case. Mermaid matches them in their own case and ignores a key
// such as loglevel, which changes nothing either.
func inert(kind, path string) bool {
	keys := strings.Split(path, ".")
	is := func(names ...string) bool {
		return slices.ContainsFunc(names, func(n string) bool { return strings.EqualFold(path, n) })
	}
	switch {
	case is("logLevel", "securityLevel", "handDrawnSeed", "architecture.seed", "architecture.randomize", "class.htmlLabels"):
		// logLevel sets what Mermaid logs, and securityLevel the scripts
		// and the links of a page. handDrawnSeed seeds the look handDrawn,
		// which zuzu draws in its own look. The layout of an architecture
		// diagram uses no random numbers. Mermaid 12 does not read
		// class.htmlLabels.
		return true
	case len(keys) <= 2 && strings.EqualFold(keys[len(keys)-1], "useMaxWidth"):
		// useMaxWidth makes an SVG as wide as the page that holds it.
		return true
	case is("htmlLabels", "flowchart.htmlLabels"):
		// A graph and a mindmap wrap their labels by htmlLabels, with
		// flowchart.htmlLabels in its place when it is not set. A chart
		// draws the same text either way.
		return !isGraph(kind) && kind != "mindmap"
	case is("wrap"):
		// Of the kinds that zuzu draws, only the sequence diagram and the
		// C4 diagrams read wrap at the top level, which %%{wrap}%% sets.
		return kind != "sequenceDiagram" && !strings.HasPrefix(kind, "C4")
	case is("gantt.compact"):
		// Mermaid 12 has no setting gantt.compact and ignores it. Its
		// documents show it next to displayMode: compact.
		return true
	}
	return false
}

// The widths in the pixels of Mermaid at which Mermaid 12 wraps a label:
// the label of a node of a flowchart or of a state by default, and the
// label of an edge always. scaleWrap turns them into the pixels of zuzu.
const (
	nodeWrap = 120
	edgeWrap = 200
)

// scaleWrap returns the width in the pixels of zuzu of a width at which
// Mermaid wraps a label. zuzu draws a label at 11 pixels and Mermaid at
// 14, so the width scales by 11/14, and a line holds the words that it
// holds in Mermaid (D32). Each width of Mermaid that wraps a label goes
// through it.
func scaleWrap(w float64) float64 {
	return w * 11 / 14
}

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
	} else if fc, _ := cfg["flowchart"].(map[string]any); fc != nil {
		// flowchart.htmlLabels is the old form of htmlLabels, and Mermaid
		// reads it in every kind of graph when htmlLabels is not set.
		if v, ok := fc["htmlLabels"].(bool); ok {
			w.html = v
		}
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
// state diagram, an entity relationship diagram or a requirement diagram
// wrap: the label of each node at node, in the pixels of zuzu, or 0 for
// none, and the label of
// each edge at edgeWrap.
func wrapGraph(cfg map[string]any, g *graph.Graph, node float64) {
	labelWrap(cfg, g)
	for _, n := range g.Nodes {
		n.Wrap = node
	}
	for _, e := range g.Edges {
		e.Wrap = scaleWrap(edgeWrap)
	}
}

// wrapSetting reads the setting of a section of the config that sets the
// width at which a label wraps, such as flowchart.wrappingWidth, or takes
// def when the config does not set it, and returns it in the pixels of
// zuzu. A value that is not a positive number is an issue.
func wrapSetting(is *graph.Issues, cfg map[string]any, def float64, section, key string) (float64, error) {
	v, ok := configNumber(cfg, section, key)
	if !ok {
		if m, _ := cfg[section].(map[string]any); m != nil && m[key] != nil {
			return scaleWrap(def), is.Add("config", "the setting %s.%s: %v", section, key, m[key])
		}
		return scaleWrap(def), nil
	}
	if !(v > 0) {
		return scaleWrap(def), is.Add("config", "the setting %s.%s: %v", section, key, v)
	}
	return scaleWrap(v), nil
}

// css reads a style of CSS of a class or a node, as fill:#f9f,stroke:#333,
// into the names of Vega. add records a property that zuzu does not draw.
func css(s string, add func(format string, args ...any) error) (map[string]string, error) {
	return readCSS(s, false, false, add)
}

// linkCSS reads a style as css does, for a class or a style that can be on
// a link. It keeps animation as it is, for the caller.
func linkCSS(s string, add func(format string, args ...any) error) (map[string]string, error) {
	return readCSS(s, true, false, add)
}

// classDefCSS reads the style of a classDef of a flowchart as linkCSS does.
// A comma after a backslash, as in stroke-dasharray: 9\,5, is a comma of
// the value and does not end the part, as in Mermaid.
func classDefCSS(s string, add func(format string, args ...any) error) (map[string]string, error) {
	return readCSS(s, true, true, add)
}

// readCSS reads a style of CSS into the names of Vega, and keeps animation
// when link is true. When escaped is true, a comma after a backslash is a
// comma of a value. A part with no colon is an issue: CSS ignores it.
func readCSS(s string, link, escaped bool, add func(format string, args ...any) error) (map[string]string, error) {
	st := map[string]string{}
	family := ""
	// escComma stands for an escaped comma while the style splits.
	const escComma = "\x00"
	if escaped {
		s = strings.ReplaceAll(s, `\,`, escComma)
	}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		part = strings.ReplaceAll(part, escComma, ",")
		k, v, ok := strings.Cut(part, ":")
		if !ok {
			if p := strings.TrimSpace(part); p != "" {
				if err := add("the style %q, which has no colon", p); err != nil {
					return nil, err
				}
			}
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		v = strings.TrimSuffix(strings.TrimSpace(strings.TrimSuffix(v, "!important")), "px")
		if k == "animation" && link {
			st[k] = v
			continue
		}
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
		case "stroke-dashoffset":
			st["strokeDashOffset"] = v
		case "font-weight":
			st["fontWeight"] = v
		case "font-size":
			st["fontSize"] = v
		case "font-family":
			// The name of a family gives a weight and a style, such as
			// Helvetica-Bold, and zuzu draws the rest of the name in IBM
			// Plex Sans (D32). font-weight and font-style win over it.
			family = v
		case "font-style":
			switch v {
			case "normal", "italic":
				st["fontStyle"] = v
			case "oblique":
				// The bundled fonts slant only as italic.
				st["fontStyle"] = "italic"
			default:
				if err := add("the font-style %s", v); err != nil {
					return nil, err
				}
			}
		case "border", "background":
			// Mermaid puts the style on the shape of SVG of the node,
			// where border and background change nothing.
		default:
			if err := add("the style %s", k); err != nil {
				return nil, err
			}
		}
	}
	weight, style := graph.FontFace(family)
	if _, ok := st["fontWeight"]; !ok && weight != "" {
		st["fontWeight"] = weight
	}
	if _, ok := st["fontStyle"]; !ok && style != "" {
		st["fontStyle"] = style
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
		if topSpacing(k) {
			continue
		}
		if ok, err := lookSetting(is, d.Kind, k, v); ok || err != nil {
			if err != nil {
				return err
			}
			continue
		}
		var read []string
		if k == section {
			read = known
		}
		if err := graphSection(is, d.Kind, k, v, read...); err != nil {
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
