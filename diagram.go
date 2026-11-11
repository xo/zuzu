package zuzu

import (
	"bytes"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/graph/dot"
	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/vega"
	"github.com/xo/zuzu/vegalite"
)

// sniffDiagram returns the dialect of a source of DOT or Mermaid from its
// first word, after the front matter, the directives and the comments of
// Mermaid, or 0 for neither (D31).
func sniffDiagram(src []byte) Dialect {
	s := string(bytes.TrimSpace(src))
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
		return 0
	}
	// Front matter of Mermaid, between two lines of ---.
	if strings.HasPrefix(s, "---") {
		if i := strings.Index(s[3:], "\n---"); i >= 0 {
			return Mermaid
		}
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
			// graph TD is Mermaid, and graph name { or graph { is DOT.
			if len(f) > 1 && isDirection(f[1]) {
				return Mermaid
			}
			if strings.Contains(t, "{") || len(f) == 1 && strings.HasPrefix(strings.TrimSpace(rest), "{") {
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
	"flowchart": true, "flowchart-elk": true, "sequenceDiagram": true, "classDiagram": true, "stateDiagram": true,
	"stateDiagram-v2": true, "erDiagram": true, "journey": true, "gantt": true, "pie": true, "quadrantChart": true,
	"requirementDiagram": true, "gitGraph": true, "mindmap": true, "timeline": true, "sankey": true, "sankey-beta": true,
	"xychart": true, "xychart-beta": true, "block": true, "block-beta": true, "packet": true, "packet-beta": true,
	"kanban": true, "architecture-beta": true, "radar-beta": true, "treemap": true, "treemap-beta": true,
	"C4Context": true, "C4Container": true, "C4Component": true, "C4Dynamic": true, "C4Deployment": true,
	"zenuml": true, "venn-beta": true, "ishikawa": true, "ishikawa-beta": true, "wardley-beta": true,
	"treeView-beta": true, "eventmodeling": true, "swimlane-beta": true, "cynefin-beta": true, "usecase-beta": true,
	"agentflow-beta": true, "railroad-beta": true, "info": true,
}

// parseDiagram reads a source of DOT or Mermaid and compiles it into a
// chart.
func parseDiagram(src []byte, d Dialect, o options) (*Chart, Report, error) {
	var g *graph.Graph
	var report Report
	var err error
	switch d {
	case DOT:
		g, report, err = dot.Parse(string(src), o.strict)
	default:
		var md *mermaid.Diagram
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
	c, _, err := FromVega(spec)
	if err != nil {
		return nil, report, err
	}
	c.opts = o
	c.report = report
	return c, report, nil
}
