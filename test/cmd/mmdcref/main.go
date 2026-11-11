// Command mmdcref reads the SVG of a flowchart, a state diagram or a class
// diagram that mmdc drew, and writes the nodes, the edges and the clusters
// that it holds as JSON, the reference of the check of D31. Run test/oracle/diagrams.sh to draw the
// SVGs and to write the references.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Ref is the reference of a flowchart.
type Ref struct {
	Nodes    []Node    `json:"nodes"`
	Edges    []Edge    `json:"edges"`
	Clusters []Cluster `json:"clusters"`
}

// Node is a node: its id, its label and its center.
type Node struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
}

// Edge is an edge: its ends, its label, the labels at its head and at its
// tail, and the points of its route.
type Edge struct {
	Source string       `json:"source"`
	Target string       `json:"target"`
	Label  string       `json:"label,omitempty"`
	Head   string       `json:"head,omitempty"`
	Tail   string       `json:"tail,omitempty"`
	Points [][2]float64 `json:"points"`
}

// Cluster is a cluster: its id, its title and its box.
type Cluster struct {
	ID     string  `json:"id"`
	Label  string  `json:"label"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

var (
	nodeRE = regexp.MustCompile(`<g class="[^"]*" id="my-svg-(?:flowchart|state)-([^"]+?)-\d+"[^>]*transform="translate\(([-\d.e]+), ([-\d.e]+)\)"`)
	// A node with a link sits in an a element that holds its place.
	linkedRE  = regexp.MustCompile(`<a [^>]*transform="translate\(([-\d.e]+), ([-\d.e]+)\)"><g class="[^"]*" id="my-svg-flowchart-([^"]+?)-\d+"`)
	clusterRE = regexp.MustCompile(`<g class="cluster[^"]*" id="my-svg-([^"]+)"[^>]*><rect[^>]*x="([-\d.e]+)" y="([-\d.e]+)" width="([-\d.e]+)" height="([-\d.e]+)"`)
	edgeRE    = regexp.MustCompile(`<path [^>]*data-id="(L_[^"]+)"[^>]*data-points="([^"]+)"`)
	labelRE   = regexp.MustCompile(`<g class="label" data-id="([^"]+)"[^>]*>(.*?)</g>`)
	textRE    = regexp.MustCompile(`<(?:p|span)[^>]*>([^<]*)</`)
	tagRE     = regexp.MustCompile(`<br\s*/?>`)
)

// text returns the text of the HTML of a label.
func text(s string) string {
	s = tagRE.ReplaceAllString(s, "\n")
	var out []string
	for _, m := range textRE.FindAllStringSubmatch(s, -1) {
		if t := strings.TrimSpace(html.UnescapeString(m[1])); t != "" {
			out = append(out, t)
		}
	}
	return strings.Join(out, " ")
}

func num(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func main() {
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	s := string(b)
	ref := Ref{Nodes: []Node{}, Edges: []Edge{}, Clusters: []Cluster{}}
	ids := map[string]bool{}
	if strings.Contains(s, `aria-roledescription="classDiagram"`) {
		write(classRef(s))
		return
	}
	ns, marks := nodes(s)
	if strings.Contains(s, `aria-roledescription="stateDiagram"`) {
		ref = stateRef(s, ns, marks)
		write(ref)
		return
	}
	for _, n := range ns {
		ref.Nodes = append(ref.Nodes, n)
		ids[n.ID] = true
	}
	for _, m := range linkedRE.FindAllStringSubmatch(s, -1) {
		ref.Nodes = append(ref.Nodes, Node{ID: m[3], Label: m[3], X: num(m[1]), Y: num(m[2])})
		ids[m[3]] = true
	}
	for _, loc := range clusterRE.FindAllStringSubmatchIndex(s, -1) {
		sub := clusterRE.FindStringSubmatch(s[loc[0]:loc[1]])
		end := strings.Index(s[loc[1]:], `</g></g>`)
		if end < 0 {
			end = len(s) - loc[1]
		}
		ref.Clusters = append(ref.Clusters, Cluster{ID: sub[1], Label: text(s[loc[1] : loc[1]+end]),
			X: num(sub[2]), Y: num(sub[3]), Width: num(sub[4]), Height: num(sub[5])})
		ids[sub[1]] = true
	}
	labels := map[string]string{}
	for _, m := range labelRE.FindAllStringSubmatch(s, -1) {
		labels[m[1]] = text(m[2])
	}
	for _, m := range edgeRE.FindAllStringSubmatch(s, -1) {
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
		// The id is L_source_target_n, and an id can hold an underscore,
		// so the split is the one that names two known ids.
		body := strings.TrimPrefix(m[1], "L_")
		body = body[:strings.LastIndex(body, "_")]
		for i := range len(body) {
			if body[i] == '_' && ids[body[:i]] && ids[body[i+1:]] {
				e.Source, e.Target = body[:i], body[i+1:]
				break
			}
		}
		ref.Edges = append(ref.Edges, e)
	}
	write(ref)
}

// nodes returns the nodes of an SVG, and the markup of each up to the end
// of its group.
func nodes(s string) ([]Node, []string) {
	var ns []Node
	var marks []string
	for _, loc := range nodeRE.FindAllStringSubmatchIndex(s, -1) {
		sub := nodeRE.FindStringSubmatch(s[loc[0]:loc[1]])
		end := strings.Index(s[loc[1]:], `</g></g>`)
		if end < 0 {
			end = len(s) - loc[1]
		}
		// A node with no label, such as the start of a state diagram, ends
		// before the next node.
		if next := strings.Index(s[loc[1]:], ` id="my-svg-`); next >= 0 && next < end {
			end = next
		}
		ns = append(ns, Node{ID: sub[1], Label: text(s[loc[1] : loc[1]+end]), X: num(sub[2]), Y: num(sub[3])})
		marks = append(marks, s[loc[1]:loc[1]+end])
	}
	return ns, marks
}

// write writes a reference as JSON, with its nodes in the order of their
// ids.
func write(ref Ref) {
	slices.SortStableFunc(ref.Nodes, func(a, b Node) int { return strings.Compare(a.ID, b.ID) })
	out, err := json.Marshal(ref)
	if err == nil {
		_, err = os.Stdout.Write(append(out, '\n'))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
