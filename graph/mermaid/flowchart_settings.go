package mermaid

import (
	"maps"
	"slices"

	"github.com/xo/zuzu/graph"
)

// flowSettings are the settings of the section flowchart of the config
// that a flowchart reads.
var flowSettings = []string{
	"curve", "wrappingWidth", "nodeSpacing", "rankSpacing", "diagramPadding",
	"titleTopMargin", "subGraphTitleMargin", "inheritDir",
}

// The defaults of Mermaid 12 and of zuzu for the spaces of a flowchart. The
// look of zuzu is smaller than the look of Mermaid, so a space of the
// config takes the ratio of the default of zuzu to the default of Mermaid:
// nodeSpacing 50 is a space of 24 between two nodes of a rank.
const (
	mermaidNodeSpacing, zuzuNodeSep = 50.0, 24.0
	mermaidRankSpacing, zuzuRankSep = 50.0, 40.0
	// The title of the diagram is titleTopMargin above the diagram in
	// Mermaid, 25 by default, and 10 above it in zuzu.
	mermaidTitleTop, zuzuTitleOffset = 25.0, 10.0
)

// section returns the section of the config that holds the spaces of the
// layout: flowchart, or agentflow in an agentflow diagram.
func (f *flowchart) section() string {
	if f.af != nil {
		return "agentflow"
	}
	return "flowchart"
}

// configNum reads a setting of the config that is a number of 0 or more,
// at a path such as flowchart.nodeSpacing. It returns false for a setting
// that is not set, and for a value that is not such a number, which is an
// issue.
func (f *flowchart) configNum(path string, v any) (float64, bool, error) {
	if v == nil {
		return 0, false, nil
	}
	n, ok := configNumber(map[string]any{"v": v}, "v")
	if !ok || n < 0 {
		return 0, false, f.issues.Add("config", "the setting %s: %v", path, v)
	}
	return n, true, nil
}

// spacing reads the settings of the config that set the spaces of the
// layout: nodeSpacing, rankSpacing, diagramPadding and titleTopMargin of
// the section that section names, and subGraphTitleMargin of the section
// flowchart. A value that is not a number of 0 or more is an issue, and
// the space keeps its default.
func (f *flowchart) spacing() error {
	cfg := f.d.Config
	num := f.configNum
	if sec, _ := cfg[f.section()].(map[string]any); sec != nil {
		if err := f.spaces(sec); err != nil {
			return err
		}
	}
	m, _ := cfg["flowchart"].(map[string]any)
	if m == nil {
		return nil
	}
	if v, ok := m["subGraphTitleMargin"]; ok {
		margin, _ := v.(map[string]any)
		if margin == nil {
			return f.issues.Add("config", "the setting flowchart.subGraphTitleMargin: %v", v)
		}
		for _, k := range slices.Sorted(maps.Keys(margin)) {
			mv := margin[k]
			path := "flowchart.subGraphTitleMargin." + k
			switch k {
			case "top", "bottom":
				n, ok, err := num(path, mv)
				if err != nil {
					return err
				}
				if ok && k == "top" {
					f.g.ClusterTitleTop = n
				} else if ok {
					f.g.ClusterTitleBottom = n
				}
			default:
				if err := f.issues.Add("config", "the setting %s", path); err != nil {
					return err
				}
			}
		}
	}
	if v, ok := m["inheritDir"]; ok {
		if _, ok := v.(bool); !ok {
			return f.issues.Add("config", "the setting flowchart.inheritDir: %v", v)
		}
	}
	return nil
}

// spaces reads nodeSpacing, rankSpacing, diagramPadding and titleTopMargin
// of the section of the config that section names.
func (f *flowchart) spaces(m map[string]any) error {
	sec := f.section()
	// Mermaid takes 0 as the default for the spaces between the nodes
	// and the ranks.
	n, ok, err := f.configNum(sec+".nodeSpacing", m["nodeSpacing"])
	if err != nil {
		return err
	}
	if ok {
		f.g.NodeSep = n * zuzuNodeSep / mermaidNodeSpacing
	}
	if n, ok, err = f.configNum(sec+".rankSpacing", m["rankSpacing"]); err != nil {
		return err
	}
	if ok {
		f.g.RankSep = n * zuzuRankSep / mermaidRankSpacing
	}
	if n, ok, err = f.configNum(sec+".diagramPadding", m["diagramPadding"]); err != nil {
		return err
	}
	if ok {
		pad := n
		f.g.Padding = &pad
	}
	if n, ok, err = f.configNum(sec+".titleTopMargin", m["titleTopMargin"]); err != nil {
		return err
	}
	if ok {
		off := n * zuzuTitleOffset / mermaidTitleTop
		f.g.TitleOffset = &off
	}
	return nil
}

// inheritDir gives each subgraph with no direction the direction of the
// flowchart, for the setting flowchart.inheritDir. Without it, such a
// subgraph takes the direction of the subgraph that holds it, as in
// Mermaid.
func (f *flowchart) inheritDir() {
	m, _ := f.d.Config["flowchart"].(map[string]any)
	if on, _ := m["inheritDir"].(bool); !on {
		return
	}
	for _, c := range f.g.Clusters {
		if c.Direction == "" {
			c.Direction = f.g.Direction
		}
	}
}

// topSpacing reports whether a key of the top level of the config is
// nodeSpacing or rankSpacing, which graphSpacing reads.
func topSpacing(k string) bool {
	return k == "nodeSpacing" || k == "rankSpacing"
}

// graphSpacing reads nodeSpacing and rankSpacing at the top level of the
// config of a graph. Mermaid 12.3 lays out every graph with them, over the
// spaces of the section of its kind, such as flowchart.nodeSpacing. A
// space takes the ratio of the default of zuzu to the default of Mermaid
// for the kind: 140 and 80 for an entity relationship diagram, and 50 and
// 50 for the other kinds. As in Mermaid, 0 keeps the space of the section.
// A value that is not a number of 0 or more is an issue.
func graphSpacing(d *Diagram, g *graph.Graph, is *graph.Issues) error {
	node, rank := mermaidNodeSpacing, mermaidRankSpacing
	if d.Kind == "erDiagram" {
		node, rank = 140, 80
	}
	for _, k := range []string{"nodeSpacing", "rankSpacing"} {
		v, ok := d.Config[k]
		if !ok {
			continue
		}
		n, ok := configNumber(d.Config, k)
		if !ok || n < 0 {
			if err := is.Add("config", "the setting %s: %v", k, v); err != nil {
				return err
			}
			continue
		}
		switch {
		case n == 0:
		case k == "nodeSpacing":
			g.NodeSep = n * zuzuNodeSep / node
		default:
			g.RankSep = n * zuzuRankSep / rank
		}
	}
	return nil
}
