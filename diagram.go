package zuzu

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/graph/dot"
	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/vega"
	"github.com/xo/zuzu/vegalite"
)

// schemaRE finds the $schema of a spec of JSON.
var schemaRE = regexp.MustCompile(`"\$schema"\s*:\s*"([^"]*)"`)

// Sniff returns the dialect of a source from its first bytes, as a
// detector of a type of file does: DOT or Mermaid from the first word, and
// Vega or Vega-Lite from a $schema in the first bytes. It needs no more
// than the first 512 bytes, and returns false when they do not decide.
func Sniff(prefix []byte) (Dialect, bool) {
	if d := sniffDiagram(prefix); d != 0 {
		return d, true
	}
	s := bytes.TrimSpace(prefix)
	if !bytes.HasPrefix(s, []byte("{")) {
		return 0, false
	}
	m := schemaRE.FindSubmatch(s)
	switch {
	case m == nil:
		return 0, false
	case bytes.Contains(m[1], []byte("/vega-lite/")):
		return VegaLite, true
	case bytes.Contains(m[1], []byte("/vega/")):
		return Vega, true
	}
	return 0, false
}

// sniffDiagram returns the dialect of a source of DOT or Mermaid from its
// first word, after the front matter, the directives and the comments of
// Mermaid, or 0 for neither (D31).
func sniffDiagram(src []byte) Dialect {
	s := string(bytes.TrimSpace(src))
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
		return 0
	}
	// Front matter of Mermaid, between two lines of ---. DOT and JSON
	// never start with ---, so the front matter needs no end in a prefix.
	if line, _, _ := strings.Cut(s, "\n"); strings.TrimSpace(line) == "---" {
		return Mermaid
	}
	for s != "" {
		line, rest, _ := strings.Cut(s, "\n")
		t := strings.TrimSpace(line)
		switch {
		case t == "" || strings.HasPrefix(t, "%%") || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "#"):
			s = rest
			continue
		case strings.HasPrefix(t, "/*"):
			if i := strings.Index(s, "*/"); i >= 0 {
				s = s[i+2:]
				continue
			}
			return 0
		}
		f := strings.Fields(t)
		word := strings.ToLower(f[0])
		switch {
		case word == "strict" || word == "digraph":
			return DOT
		case word == "graph":
			// graph TD is Mermaid. A graph of Mermaid takes only a
			// direction, so graph name, graph "" and graph { are DOT,
			// with the brace on this line or a later one. A bare graph
			// is Mermaid, unless a brace follows it.
			if len(f) > 1 {
				if isDirection(f[1]) {
					return Mermaid
				}
				return DOT
			}
			if strings.HasPrefix(strings.TrimSpace(rest), "{") {
				return DOT
			}
			return Mermaid
		case mermaidKinds[strings.TrimSuffix(f[0], ";")]:
			return Mermaid
		}
		return 0
	}
	return 0
}

// isDirection reports whether a word is a direction of a flowchart.
func isDirection(w string) bool {
	switch strings.ToUpper(strings.TrimSuffix(w, ";")) {
	case "TB", "TD", "BT", "LR", "RL":
		return true
	}
	return false
}

// mermaidKinds are the first words of the kinds of diagram of Mermaid.
var mermaidKinds = map[string]bool{
	"flowchart": true, "flowchart-elk": true, "sequenceDiagram": true, "classDiagram": true, "classDiagram-v2": true, "stateDiagram": true,
	"stateDiagram-v2": true, "erDiagram": true, "journey": true, "gantt": true, "pie": true, "quadrantChart": true,
	"requirementDiagram": true, "gitGraph": true, "gitGraph:": true, "mindmap": true, "timeline": true, "sankey": true, "sankey-beta": true,
	"xychart": true, "xychart-beta": true, "block": true, "block-beta": true, "packet": true, "packet-beta": true,
	"kanban": true, "architecture-beta": true, "radar-beta": true, "treemap": true, "treemap-beta": true,
	"C4Context": true, "C4Container": true, "C4Component": true, "C4Dynamic": true, "C4Deployment": true,
	"zenuml": true, "venn-beta": true, "ishikawa": true, "ishikawa-beta": true, "wardley-beta": true,
	"treeView-beta": true, "eventmodeling": true, "swimlane-beta": true, "cynefin-beta": true, "usecase-beta": true,
	"agentflow-beta": true, "railroad-beta": true, "info": true,
	"cynefin-beta:":      true,
	"railroad-ebnf-beta": true, "railroad-abnf-beta": true, "railroad-peg-beta": true,
}

// parseDiagram reads a source of DOT or Mermaid and compiles it into a
// chart.
func parseDiagram(src []byte, d Dialect, o options) (*Chart, Report, error) {
	var g *graph.Graph
	var md *mermaid.Diagram
	var report Report
	var err error
	switch d {
	case DOT:
		g, report, err = dot.Parse(string(src), o.strict)
	default:
		if md, err = mermaid.Read(string(src)); err != nil {
			return nil, nil, err
		}
		if mermaid.IsChart(md.Kind) {
			return mermaidChart(md, o)
		}
		g, report, err = mermaid.Graph(md, o.strict)
	}
	if err != nil {
		return nil, report, err
	}
	spec, err := graph.Compile(g)
	if err != nil {
		return nil, report, err
	}
	if md != nil {
		addLook(spec, md)
	}
	c, _, err := FromVega(spec)
	if err != nil {
		return nil, report, err
	}
	c.opts = o
	c.report = report
	return c, report, nil
}

// mermaidChart compiles a chart of Mermaid, through Vega-Lite or Vega.
func mermaidChart(md *mermaid.Diagram, o options) (*Chart, Report, error) {
	b, lite, report, err := mermaid.Chart(md, o.strict)
	if err != nil {
		return nil, report, err
	}
	var spec *vega.Spec
	if lite {
		vl, err := vegalite.Parse(b)
		if err != nil {
			return nil, report, err
		}
		var more Report
		spec, more, err = vegalite.Compile(vl, vegalite.WithStrict(o.strict))
		report = append(report, more...)
		if err != nil {
			return nil, report, err
		}
	} else if spec, err = vega.Parse(b); err != nil {
		return nil, report, err
	}
	addLook(spec, md)
	c, _, err := FromVega(spec)
	if err != nil {
		return nil, report, err
	}
	c.opts = o
	c.report = report
	return c, report, nil
}

// addLook puts the mode or the scheme that the theme of a Mermaid diagram
// asks for in the config of its spec, as zuzu.mode or zuzu.scheme (D32).
// The runtime takes it where the caller of a render sets no mode or no
// scheme.
func addLook(spec *vega.Spec, md *mermaid.Diagram) {
	for k, v := range mermaid.Look(md) {
		if spec.Config == nil {
			spec.Config = vega.Config{}
		}
		spec.Config[k] = v
	}
}
