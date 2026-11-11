package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

var (
	// stateClusterRE matches a composite state, or a region of one, which
	// has a rect of the class divider.
	stateClusterRE = regexp.MustCompile(`<g class="[^"]*statediagram-cluster[^"]*" id="my-svg-state-([^"]+?)-\d+"[^>]*>(?:<g>)?<rect class="(outer|divider)" x="([-\d.e]+)" y="([-\d.e]+)" width="([-\d.e]+)" height="([-\d.e]+)"`)
	stateEdgeRE    = regexp.MustCompile(`<path [^>]*data-id="([^"]+)"[^>]*data-points="([^"]+)"`)
	sizeRE         = regexp.MustCompile(`^>?<(?:rect|circle)[^>]*width="([\d.e]+)" height="([\d.e]+)"`)
	pathRE         = regexp.MustCompile(`<path d="([^"]+)"`)
	numberRE       = regexp.MustCompile(`-?[\d.]+(?:e-?\d+)?`)
)

// box is the box of a node or a cluster, by its center and its half size.
type box struct {
	id           string
	x, y, hw, hh float64
	cluster      bool
}

// dist returns the distance from a point to a node, 0 inside it, or to the
// border of a cluster.
func (b box) dist(px, py float64) float64 {
	dx, dy := math.Abs(px-b.x)-b.hw, math.Abs(py-b.y)-b.hh
	if !b.cluster || dx > 0 || dy > 0 {
		return math.Hypot(math.Max(dx, 0), math.Max(dy, 0))
	}
	return math.Min(-dx, -dy)
}

// size returns the half size of a node from its markup: the width and the
// height of its first rect or circle, or else the reach of its first path.
func size(mark string) (float64, float64) {
	if m := sizeRE.FindStringSubmatch(mark); m != nil {
		return num(m[1]) / 2, num(m[2]) / 2
	}
	m := pathRE.FindStringSubmatch(mark)
	if m == nil {
		return 7, 7
	}
	var hw, hh float64
	for i, v := range numberRE.FindAllString(m[1], -1) {
		if i%2 == 0 {
			hw = math.Max(hw, math.Abs(num(v)))
		} else {
			hh = math.Max(hh, math.Abs(num(v)))
		}
	}
	return hw, hh
}

// stateRef reads the reference of a state diagram. An edge of a state
// diagram has no ids of its ends, so each end is the node or the cluster
// nearest to the end of its route. The id of the last region of a
// composite state is random, so each region takes the id divider-id-n, by
// its order.
func stateRef(s string, ns []Node, marks []string) Ref {
	ref := Ref{Nodes: []Node{}, Edges: []Edge{}, Clusters: []Cluster{}}
	rename := map[string]string{}
	var boxes []box
	for _, loc := range stateClusterRE.FindAllStringSubmatchIndex(s, -1) {
		sub := stateClusterRE.FindStringSubmatch(s[loc[0]:loc[1]])
		id, label := sub[1], ""
		if sub[2] == "divider" {
			rename[id] = fmt.Sprintf("divider-id-%d", len(rename)+1)
			id = rename[id]
		} else {
			rest := s[loc[1]:]
			end := strings.Index(rest, `class="inner"`)
			if end < 0 {
				end = len(rest)
			}
			label = text(rest[:end])
		}
		c := Cluster{ID: id, Label: label, X: num(sub[3]), Y: num(sub[4]), Width: num(sub[5]), Height: num(sub[6])}
		ref.Clusters = append(ref.Clusters, c)
		boxes = append(boxes, box{id: id, x: c.X + c.Width/2, y: c.Y + c.Height/2, hw: c.Width / 2, hh: c.Height / 2, cluster: true})
	}
	for i, n := range ns {
		for old, id := range rename {
			if rest, ok := strings.CutPrefix(n.ID, old+"_"); ok {
				n.ID = id + "_" + rest
			}
		}
		hw, hh := size(marks[i])
		boxes = append(boxes, box{id: n.ID, x: n.X, y: n.Y, hw: hw, hh: hh})
		ref.Nodes = append(ref.Nodes, n)
	}
	near := func(p [2]float64) string {
		best, id := math.Inf(1), ""
		for _, b := range boxes {
			// A node wins a tie with a cluster, since its border can lie
			// on the border of its cluster.
			d := b.dist(p[0], p[1])
			if b.cluster {
				d += 0.5
			}
			if d < best {
				best, id = d, b.id
			}
		}
		return id
	}
	labels := map[string]string{}
	for _, m := range labelRE.FindAllStringSubmatch(s, -1) {
		labels[m[1]] = text(m[2])
	}
	for _, m := range stateEdgeRE.FindAllStringSubmatch(s, -1) {
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
		e.Source, e.Target = near(e.Points[0]), near(e.Points[len(e.Points)-1])
		ref.Edges = append(ref.Edges, e)
	}
	return ref
}
