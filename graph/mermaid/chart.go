package mermaid

import (
	"encoding/json/v2"
	"fmt"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/vega"
)

// chart is the state of the compile of a chart of Mermaid.
type chart struct {
	d      *Diagram
	issues *graph.Issues
	// line is the number of the line that the compile reads, for the
	// report.
	line int
	// acc holds the texts of accTitle and accDescr, which join into the
	// description of the spec.
	acc []string
}

// add reports a part of the source that zuzu does not draw: an error in
// the strict mode, and a line of the report in the loose mode.
func (c *chart) add(format string, args ...any) error {
	path := "config"
	if c.line > 0 {
		path = fmt.Sprintf("line %d", c.line)
	}
	return c.issues.Add(path, format, args...)
}

// errorf returns an error of the syntax at the current line.
func (c *chart) errorf(format string, args ...any) error {
	return fmt.Errorf("line %d: %s: %w", c.line, fmt.Sprintf(format, args...), ErrSyntax)
}

// compiler compiles the body of a chart into a spec of Vega-Lite, with
// lite true, or of Vega.
type compiler func(c *chart) (spec map[string]any, lite bool, err error)

// charts are the compilers of the charts of Mermaid, by the first word of
// the source (D31).
var charts = map[string]compiler{
	"pie":               pieChart,
	"xychart":           xyChart,
	"xychart-beta":      xyChart,
	"quadrantChart":     quadrantChart,
	"gantt":             ganttChart,
	"journey":           journeyChart,
	"wardley-beta":      wardleyChart,
	"sankey":            sankeyChart,
	"sankey-beta":       sankeyChart,
	"radar-beta":        radarChart,
	"treemap":           treemapChart,
	"treemap-beta":      treemapChart,
	"mindmap":           mindmapChart,
	"architecture-beta": architectureChart,
	"sequenceDiagram":   sequenceChart,
}

// IsChart reports whether a kind of diagram is a chart that Chart
// compiles.
func IsChart(kind string) bool {
	_, ok := charts[kind]
	return ok
}

// Chart compiles a chart of Mermaid into the JSON of a spec of Vega-Lite,
// with lite true, or of Vega. The report names each part that zuzu does not
// draw, and in the strict mode such a part is an error.
func Chart(d *Diagram, strict bool) ([]byte, bool, vega.Report, error) {
	comp, ok := charts[d.Kind]
	if !ok {
		return nil, false, nil, fmt.Errorf("%s: %w", d.Kind, ErrKind)
	}
	c := &chart{d: d, issues: &graph.Issues{Strict: strict}}
	spec, lite, err := comp(c)
	if err != nil {
		return nil, false, c.issues.Report, err
	}
	if lite {
		spec["$schema"] = "https://vega.github.io/schema/vega-lite/v6.json"
	} else {
		spec["$schema"] = "https://vega.github.io/schema/vega/v5.json"
	}
	if _, ok := spec["description"]; !ok && len(c.acc) > 0 {
		spec["description"] = strings.Join(c.acc, " ")
	}
	if _, ok := spec["title"]; !ok && d.Title != "" {
		spec["title"] = map[string]any{"text": d.Title, "anchor": "start"}
	}
	b, err := json.Marshal(spec, json.Deterministic(true))
	if err != nil {
		return nil, false, c.issues.Report, fmt.Errorf("writing the spec of a chart: %w", err)
	}
	return b, lite, c.issues.Report, nil
}

// emptyChart returns the spec of Vega of a chart with nothing in it but its
// title and its description, as Mermaid draws a gantt or a user journey
// with no task.
func emptyChart(title, desc string) map[string]any {
	spec := map[string]any{"width": 0, "height": 0}
	if title != "" {
		spec["title"] = map[string]any{"text": title, "anchor": "start"}
	}
	if desc != "" {
		spec["description"] = desc
	}
	return spec
}

// field splits a line at its first colon into a key and a value, both
// trimmed.
func field(s string) (string, string, bool) {
	k, v, ok := strings.Cut(s, ":")
	return strings.TrimSpace(k), strings.TrimSpace(v), ok
}

// keyword reads a line that starts with a word, as title My chart, and
// returns the rest, trimmed.
func keyword(s, word string) (string, bool) {
	if s == word {
		return "", true
	}
	rest, ok := strings.CutPrefix(s, word+" ")
	return strings.TrimSpace(rest), ok
}
