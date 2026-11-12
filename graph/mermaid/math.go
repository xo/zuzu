package mermaid

import (
	"fmt"
	"regexp"

	"github.com/xo/zuzu/graph"
)

// katexRE matches the math of KaTeX in a label as Mermaid 12.3 finds it:
// a text between two $$ on one line.
var katexRE = regexp.MustCompile(`\$\$(.*?)\$\$`)

// hasMath reports whether a label holds math of KaTeX.
func hasMath(s string) bool {
	return katexRE.MatchString(s)
}

// mathIssue is the issue of a label with math of KaTeX. zuzu draws no
// math, and draws the label as its source.
const mathIssue = "the math of KaTeX in the label %q, drawn as its source"

// htmlLabels reports whether Mermaid draws the labels of a diagram in
// HTML: the setting htmlLabels, or flowchart.htmlLabels when it is not
// set, true by default.
func htmlLabels(cfg map[string]any) bool {
	if v, ok := cfg["htmlLabels"].(bool); ok {
		return v
	}
	if fc, _ := cfg["flowchart"].(map[string]any); fc != nil {
		if v, ok := fc["htmlLabels"].(bool); ok {
			return v
		}
	}
	return true
}

// graphMath reports each label of a graph that holds math of KaTeX, in the
// order of the nodes, the edges and the clusters. Mermaid 12.3 draws such
// math in a label of HTML, and zuzu draws the source. With htmlLabels
// false, Mermaid draws the source too, so that is no issue.
func graphMath(d *Diagram, g *graph.Graph, is *graph.Issues) error {
	if !htmlLabels(d.Config) {
		return nil
	}
	check := func(path string, texts ...string) error {
		for _, s := range texts {
			if hasMath(s) {
				if err := is.Add(path, mathIssue, s); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, n := range g.Nodes {
		texts := []string{n.Label}
		for _, f := range n.Fields {
			texts = append(texts, f.Text)
			texts = append(texts, f.Cells...)
		}
		if err := check("node "+n.ID, texts...); err != nil {
			return err
		}
	}
	for _, e := range g.Edges {
		if err := check(fmt.Sprintf("edge %s to %s", e.Source, e.Target), e.Label, e.HeadLabel, e.TailLabel, e.XLabel); err != nil {
			return err
		}
	}
	for _, c := range g.Clusters {
		if err := check("cluster "+c.ID, c.Label); err != nil {
			return err
		}
	}
	return nil
}
