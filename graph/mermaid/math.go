package mermaid

import (
	"fmt"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/text"
)

// hasMath reports whether a label holds math of KaTeX, as Mermaid 12.3
// finds it: a text between two $$ on one line.
func hasMath(s string) bool {
	return text.HasMath(s)
}

// mathIssue is the issue of a label with math of KaTeX when the parse has
// no font of math, which zuzu.WithMath gives. zuzu draws the label as its
// source.
const mathIssue = "the math of KaTeX in the label %q, drawn as its source"

// The issues of a label with a formula that does not lay out: one that
// names the command of TeX at fault, and one for an error with no command.
// zuzu draws the label as its source (D36).
const (
	mathCommandIssue = "the math of KaTeX in the label %q, drawn as its source: the command %s does not lay out"
	mathLayoutIssue  = "the math of KaTeX in the label %q, drawn as its source: %v"
)

// mathCheckSize is the size in pixels at which the reader lays out each
// formula to check it. A formula that lays out at one size lays out at
// each size.
const mathCheckSize = 14

// labelMathIssue lays out each formula of a label in a font of math. It
// returns nil when each one lays out, and else the issue of the first
// that does not, as a format and its arguments.
func labelMathIssue(f *text.MathFont, label string) (string, []any) {
	for _, r := range text.SplitMath(label) {
		if !r.Math {
			continue
		}
		if _, err := f.Layout(r.Text, mathCheckSize); err != nil {
			if c := text.MathCommand(err); c != "" {
				return mathCommandIssue, []any{label, c}
			}
			return mathLayoutIssue, []any{label, err}
		}
	}
	return "", nil
}

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

// graphMath checks each label of a graph that holds math of KaTeX, in the
// order of the nodes, the edges and the clusters. Mermaid 12.3 draws such
// math in a label of HTML. With the font of math of d, zuzu draws it too,
// and marks the graph so: Math is true when a label draws math. A label
// with a formula that does not lay out draws its source, goes in
// MathSource, and is an issue that names the command at fault. With no
// font of math, each label with math draws its source and is an issue.
// With htmlLabels false, Mermaid draws the source too, so that is no
// issue (D36).
func graphMath(d *Diagram, g *graph.Graph, is *graph.Issues) error {
	if !htmlLabels(d.Config) {
		return nil
	}
	// issues holds the issue of each label that the font laid out, with
	// an empty format when each of its formulas laid out, so that the
	// reader lays out each label once.
	type issue struct {
		format string
		args   []any
	}
	issues := map[string]issue{}
	check := func(path string, texts ...string) error {
		for _, s := range texts {
			if !hasMath(s) {
				continue
			}
			if d.Math == nil {
				if err := is.Add(path, mathIssue, s); err != nil {
					return err
				}
				continue
			}
			i, done := issues[s]
			if !done {
				i.format, i.args = labelMathIssue(d.Math, s)
				if i.format != "" {
					g.MathSource = append(g.MathSource, s)
				}
				issues[s] = i
			}
			if i.format == "" {
				g.Math = true
				continue
			}
			if err := is.Add(path, i.format, i.args...); err != nil {
				return err
			}
		}
		return nil
	}
	for _, n := range g.Nodes {
		// Each line of a field, such as a member of a class, is a label of
		// its own in Mermaid.
		texts := []string{n.Label}
		for _, f := range n.Fields {
			for _, s := range append([]string{f.Text}, f.Cells...) {
				texts = append(texts, strings.Split(s, "\n")...)
			}
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
