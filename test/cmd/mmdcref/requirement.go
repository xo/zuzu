package main

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
)

var (
	// reqNodeRE matches a requirement or an element: its name and its
	// center.
	reqNodeRE = regexp.MustCompile(`<g class="node[^"]*" id="my-svg-([^"]+)"[^>]*transform="translate\(([-\d.e]+), ([-\d.e]+)\)"`)
	// reqLabelRE matches a line of text of a requirement or an element.
	reqLabelRE = regexp.MustCompile(`(?s)<g class="label" style="[^"]*"[^>]*>(.*?)</g>`)
	// reqTypeRE matches the type of a relation, as <<satisfies>>.
	reqTypeRE = regexp.MustCompile(`^<<(.*)>>$`)
)

// requirementRef reads the reference of a requirement diagram. A node takes
// the name of its requirement or its element as its id, and the second
// line of its text, its name, as its label. An edge has the names of its
// ends in its id. zuzu draws the type of a relation in guillemets, as
// «satisfies», where Mermaid writes <<satisfies>>, so the label of an
// edge is in guillemets.
func requirementRef(s string) Ref {
	ref := Ref{Nodes: []Node{}, Edges: []Edge{}, Clusters: []Cluster{}}
	ids := map[string]bool{}
	locs := reqNodeRE.FindAllStringSubmatchIndex(s, -1)
	for i, loc := range locs {
		sub := reqNodeRE.FindStringSubmatch(s[loc[0]:loc[1]])
		end := len(s)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		label := sub[1]
		if ls := reqLabelRE.FindAllStringSubmatch(s[loc[1]:end], 2); len(ls) == 2 {
			label = text(ls[1][1])
		}
		ref.Nodes = append(ref.Nodes, Node{ID: sub[1], Label: label, X: num(sub[2]), Y: num(sub[3])})
		ids[sub[1]] = true
	}
	labels := map[string]string{}
	for _, m := range labelRE.FindAllStringSubmatch(s, -1) {
		labels[m[1]] = reqTypeRE.ReplaceAllString(text(m[2]), "«$1»")
	}
	for _, m := range classEdgeRE.FindAllStringSubmatch(s, -1) {
		raw, err := base64.StdEncoding.DecodeString(m[2])
		if err != nil {
			continue
		}
		var pts []struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		}
		if json.Unmarshal(raw, &pts) != nil {
			continue
		}
		e := Edge{Label: labels[m[1]]}
		for _, p := range pts {
			e.Points = append(e.Points, [2]float64{p.X, p.Y})
		}
		// The id is source-target-n, and a name can hold a dash, so the
		// split is the one that names two known names.
		body := m[1][:strings.LastIndex(m[1], "-")]
		for i := range len(body) {
			if body[i] == '-' && ids[body[:i]] && ids[body[i+1:]] {
				e.Source, e.Target = body[:i], body[i+1:]
				break
			}
		}
		ref.Edges = append(ref.Edges, e)
	}
	return ref
}
