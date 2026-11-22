// SPDX-License-Identifier: EPL-2.0

package adapter

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"testing"

	elk "github.com/xo/zuzu/elk/internal/elkgo"
)

// TestWriteGraphsForELKjs writes the graphs of the fixtures as the JSON of
// ELK, so that a person can lay them out with ELK.js 0.12.0 and compare. It
// runs only when ZUZU_ELK_GRAPHS names a folder. For each fixture it writes
// these files:
//
//   - name.json is the graph that Mermaid gives ELK.js, with the title of
//     each cluster as a label.
//   - name.zuzu.json is the graph that the adapter gives elk-go, with the
//     room for each title in the padding (reserveTitles).
//   - name.go.json is name.zuzu.json after the layout of elk-go.
//   - name.txt is the layout that Layout returns, in the form of the golden
//     file.
//
// This module never runs Node.js (AGENTS.md, hard rule 11). The test module
// holds the oracle of ELK (D44).
func TestWriteGraphsForELKjs(t *testing.T) {
	t.Parallel()
	dir := os.Getenv("ZUZU_ELK_GRAPHS")
	if dir == "" {
		t.Skip("set ZUZU_ELK_GRAPHS to a folder to write the graphs")
	}
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	marshal := func(n *elk.Node) []byte {
		t.Helper()
		data, err := json.Marshal(nodeJSON(n), json.Deterministic(true))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	for _, f := range fixtures() {
		b, err := build(&f.g, true)
		if err != nil {
			t.Fatal(err)
		}
		write(f.name+".json", marshal(b.root))
		if b, err = build(&f.g, false); err != nil {
			t.Fatal(err)
		}
		write(f.name+".zuzu.json", marshal(b.root))
		if err := elk.Layout(b.root); err != nil {
			t.Fatal(err)
		}
		write(f.name+".go.json", marshal(b.root))
		r, err := Layout(&f.g)
		if err != nil {
			t.Fatal(err)
		}
		write(f.name+".txt", []byte(format(f.name, r, &f.g)))
	}
}

// optionsJSON returns layout options as a map for JSON.
func optionsJSON(o elk.LayoutOptions) map[string]any {
	return map[string]any(o)
}

// labelsJSON returns labels as a list for JSON.
func labelsJSON(ls []*elk.Label) []any {
	var out []any
	for _, l := range ls {
		m := map[string]any{"text": *l.Text, "width": *l.Width, "height": *l.Height}
		if l.X != nil {
			m["x"], m["y"] = *l.X, *l.Y
		}
		if l.LayoutOptions != nil {
			m["layoutOptions"] = optionsJSON(l.LayoutOptions)
		}
		out = append(out, m)
	}
	return out
}

// nodeJSON returns a node of ELK, with its children, its ports and its
// edges, as a map for JSON in the form that ELK.js reads and writes.
func nodeJSON(n *elk.Node) map[string]any {
	m := map[string]any{"id": n.ID.String()}
	if n.LayoutOptions != nil {
		m["layoutOptions"] = optionsJSON(n.LayoutOptions)
	}
	if n.Width != nil {
		m["width"], m["height"] = *n.Width, *n.Height
	}
	if n.X != nil {
		m["x"], m["y"] = *n.X, *n.Y
	}
	if n.Labels != nil {
		m["labels"] = labelsJSON(n.Labels)
	}
	if n.Children != nil {
		var cs []any
		for _, c := range n.Children {
			cs = append(cs, nodeJSON(c))
		}
		m["children"] = cs
	}
	if n.Ports != nil {
		var ps []any
		for _, p := range n.Ports {
			pm := map[string]any{"id": p.ID.String(), "width": *p.Width, "height": *p.Height}
			if p.X != nil {
				pm["x"], pm["y"] = *p.X, *p.Y
			}
			if p.LayoutOptions != nil {
				pm["layoutOptions"] = optionsJSON(p.LayoutOptions)
			}
			ps = append(ps, pm)
		}
		m["ports"] = ps
	}
	if n.Edges != nil {
		var es []any
		for _, e := range n.Edges {
			em := map[string]any{
				"id":      e.ID.String(),
				"sources": []string{e.Sources[0].String()},
				"targets": []string{e.Targets[0].String()},
				"labels":  labelsJSON(e.Labels),
			}
			var ss []any
			for _, s := range e.Sections {
				ss = append(ss, map[string]any{"startPoint": *s.StartPoint, "endPoint": *s.EndPoint, "bendPoints": s.BendPoints})
			}
			if ss != nil {
				em["sections"] = ss
			}
			es = append(es, em)
		}
		m["edges"] = es
	}
	return m
}
