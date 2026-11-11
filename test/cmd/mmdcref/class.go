package main

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"regexp"
	"strings"
)

var (
	// classNodeRE matches a node of a class diagram or of an entity
	// relationship diagram: a class, classId-Name-n, an interface of a
	// lollipop, interfacen, a note, noten, or an entity, entity-Name-n.
	// A node with a link sits in an a element that holds its place.
	classNodeRE = regexp.MustCompile(`(?:<a [^>]*transform="translate\(([-\d.e]+), ([-\d.e]+)\)">)?<g class="node[^"]*" id="my-svg-([^"]+)"[^>]*?(?:transform="translate\(([-\d.e]+), ([-\d.e]+)\)")?>`)
	// classEdgeRE matches the path of a relation, with its id and its
	// points.
	classEdgeRE = regexp.MustCompile(`<path [^>]*data-id="([^"]+)"[^>]*data-points="([^"]+)"`)
	// terminalRE matches a label at an end of a relation, a cardinality.
	terminalRE = regexp.MustCompile(`<g class="edgeTerminals" transform="translate\(([-\d.e]+), ([-\d.e]+)\)">(.*?)</foreignObject>`)
	// classIDRE takes the prefix and the number off the id of a class or
	// of an entity.
	classIDRE = regexp.MustCompile(`^(?:classId-|entity-)(.+)-\d+$`)
	// titleRE finds the name of a class in its markup.
	titleRE = regexp.MustCompile(`(?s)<g class="label-group text"[^>]*>(.*?)</g></g>`)
	// nameRE finds the name of an entity with attributes in its markup.
	nameRE = regexp.MustCompile(`(?s)<g class="label name"[^>]*>(.*?)</g>`)
)

// classRef reads the reference of a class diagram or of an entity
// relationship diagram. A node takes the id of its class or its entity,
// and the label of its name. An edge has the ids of its ends in its id,
// except the line of a note, which goes from its note to the nearest
// node. Each cardinality goes to the end of a relation that is nearest to
// it.
func classRef(s string) Ref {
	ref := Ref{Nodes: []Node{}, Edges: []Edge{}, Clusters: []Cluster{}}
	full := map[string]string{}
	var boxes []box
	locs := classNodeRE.FindAllStringSubmatchIndex(s, -1)
	for i, loc := range locs {
		sub := classNodeRE.FindStringSubmatch(s[loc[0]:loc[1]])
		end := len(s)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		if c := strings.Index(s[loc[1]:end], `<g class="cluster`); c >= 0 {
			end = loc[1] + c
		}
		mark := s[loc[1]:end]
		id := sub[3]
		if m := classIDRE.FindStringSubmatch(id); m != nil {
			id = m[1]
		}
		// An edge of a class diagram names its ends by their ids, and an
		// edge of an entity relationship diagram by the ids of their nodes.
		full[sub[3]], full[id] = id, id
		x, y := sub[1]+sub[4], sub[2]+sub[5]
		label := text(mark)
		if m := titleRE.FindStringSubmatch(mark); m != nil {
			label = text(m[1])
		} else if m := nameRE.FindStringSubmatch(mark); m != nil {
			// An entity with attributes has a label of its name.
			label = text(m[1])
		}
		n := Node{ID: id, Label: label, X: num(x), Y: num(y)}
		ref.Nodes = append(ref.Nodes, n)
		hw, hh := size(mark)
		boxes = append(boxes, box{id: id, x: n.X, y: n.Y, hw: hw, hh: hh})
	}
	for _, loc := range clusterRE.FindAllStringSubmatchIndex(s, -1) {
		sub := clusterRE.FindStringSubmatch(s[loc[0]:loc[1]])
		end := strings.Index(s[loc[1]:], `</g></g>`)
		if end < 0 {
			end = len(s) - loc[1]
		}
		ref.Clusters = append(ref.Clusters, Cluster{ID: sub[1], Label: text(s[loc[1] : loc[1]+end]),
			X: num(sub[2]), Y: num(sub[3]), Width: num(sub[4]), Height: num(sub[5])})
	}
	near := func(p [2]float64) string {
		best, id := math.Inf(1), ""
		for _, b := range boxes {
			if d := b.dist(p[0], p[1]); d < best {
				best, id = d, b.id
			}
		}
		return id
	}
	labels := map[string]string{}
	for _, m := range labelRE.FindAllStringSubmatch(s, -1) {
		labels[m[1]] = text(m[2])
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
		if json.Unmarshal(raw, &pts) != nil || len(pts) < 2 {
			continue
		}
		e := Edge{Label: labels[m[1]]}
		for _, p := range pts {
			e.Points = append(e.Points, [2]float64{p.X, p.Y})
		}
		if n, ok := strings.CutPrefix(m[1], "edgeNote"); ok {
			e.Source, e.Target = "note"+n, near(e.Points[len(e.Points)-1])
		} else {
			// The id is id_source_target_n, and an id can hold an
			// underscore, so the split is the one that names two known
			// ids.
			body := strings.TrimPrefix(m[1], "id_")
			body = body[:strings.LastIndex(body, "_")]
			for i := range len(body) {
				if body[i] == '_' && full[body[:i]] != "" && full[body[i+1:]] != "" {
					e.Source, e.Target = full[body[:i]], full[body[i+1:]]
					break
				}
			}
		}
		ref.Edges = append(ref.Edges, e)
	}
	for _, m := range terminalRE.FindAllStringSubmatch(s, -1) {
		x, y, label := num(m[1]), num(m[2]), text(m[3])
		if label == "" {
			continue
		}
		best, at, head := math.Inf(1), -1, false
		for i, e := range ref.Edges {
			for _, end := range []bool{false, true} {
				p := e.Points[0]
				if end {
					p = e.Points[len(e.Points)-1]
				}
				if d := math.Hypot(p[0]-x, p[1]-y); d < best {
					best, at, head = d, i, end
				}
			}
		}
		if at < 0 {
			continue
		}
		if head {
			ref.Edges[at].Head = label
		} else {
			ref.Edges[at].Tail = label
		}
	}
	return ref
}
