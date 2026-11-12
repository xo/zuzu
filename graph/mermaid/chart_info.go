package mermaid

import (
	"strings"

	"github.com/xo/zuzu/graph"
)

// infoVersion is the text that the info diagram of Mermaid 12.3.0 draws:
// the version of Mermaid. zuzu reads that version of Mermaid, and draws
// the same text.
const infoVersion = "v12.3.0"

// infoChart compiles an info diagram: the version of Mermaid, as one text,
// into a Vega spec. The keyword showInfo changes nothing, as in Mermaid.
// A title, an accTitle and an accDescr can follow. Mermaid reads them and
// draws none of them, so zuzu draws no title, and puts accTitle and
// accDescr in the description of the spec.
func infoChart(c *chart) (map[string]any, bool, error) {
	if err := c.unreadSettings(); err != nil {
		return nil, false, err
	}
	lines := make([]Line, 0, len(c.d.Lines)+1)
	if head := strings.TrimSpace(c.d.Header); head != "" {
		lines = append(lines, Line{Text: head, N: c.d.N})
	}
	lines = append(lines, c.d.Lines...)
	show := false
	for i := 0; i < len(lines); i++ {
		c.line = lines[i].N
		if lines[i].Text == "showInfo" && !show && i == 0 {
			show = true
			continue
		}
		// The title and the accessibility statements come after the
		// keyword and showInfo.
		n, ok, err := c.accessibility(lines, i)
		if err != nil {
			return nil, false, err
		}
		if ok {
			i = n
			continue
		}
		if cyTitleRE.MatchString(lines[i].Text) {
			continue
		}
		return nil, false, c.errorf("the text %q in an info diagram", lines[i].Text)
	}
	// Mermaid draws no title on an info diagram, not even the title of the
	// front matter.
	c.d.Title = ""
	const w, h = 200.0, 48.0
	return map[string]any{
		"width":    w,
		"height":   h,
		"autosize": "pad",
		"padding":  8,
		"marks": []any{map[string]any{
			"type": "text", "name": "version", "style": []any{graph.StyleNodeLabel},
			"encode": map[string]any{"update": map[string]any{
				"x": map[string]any{"value": w / 2}, "y": map[string]any{"value": h / 2},
				"text":  map[string]any{"value": infoVersion},
				"align": map[string]any{"value": "center"}, "baseline": map[string]any{"value": "middle"},
				"fontSize": map[string]any{"value": 32}, "fontWeight": map[string]any{"value": 400},
			}},
		}},
	}, false, nil
}
