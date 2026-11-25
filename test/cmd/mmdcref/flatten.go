package main

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var (
	// elementRE matches the start and the end of the elements that flatten
	// reads: a group, a link, a box and a path.
	elementRE = regexp.MustCompile(`<(/?)(g|a|rect|path)\b([^>]*)>`)
	// translateRE matches the translate of an element.
	translateRE = regexp.MustCompile(`transform="translate\(([-\d.e]+),\s*([-\d.e]+)\)"`)
	// xyRE matches the x or the y of a box.
	xyRE = regexp.MustCompile(`\b(x|y)="([-\d.e]+)"`)
	// pointsRE matches the points of a route, in base64.
	pointsRE = regexp.MustCompile(`data-points="([^"]+)"`)
)

// frame is the coordinate system of a group: the offset of its root from
// the root of the SVG, and whether a translate of its own makes the places
// inside it relative to it, as inside a node.
type frame struct {
	dx, dy float64
	local  bool
}

// flatten returns an SVG with the translate of each nested root group,
// <g class="root" transform="translate(x, y)">, added to the places inside
// it: the translate of each node, link and label, the box of each cluster
// and the points of each route. Mermaid draws such a group for a cluster
// that dagre lays out on its own, with its places relative to the group.
// A place inside an element with a translate of its own stays relative to
// that element.
func flatten(s string) string {
	var b strings.Builder
	stack := []frame{{}}
	last := 0
	for _, loc := range elementRE.FindAllStringSubmatchIndex(s, -1) {
		b.WriteString(s[last:loc[0]])
		last = loc[1]
		tag := s[loc[0]:loc[1]]
		closing := s[loc[2]:loc[3]] == "/"
		name := s[loc[4]:loc[5]]
		attrs := s[loc[6]:loc[7]]
		cur := stack[len(stack)-1]
		group := name == "g" || name == "a"
		switch {
		case closing && group:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case group:
			next := cur
			m := translateRE.FindStringSubmatch(attrs)
			switch {
			case m != nil && name == "g" && strings.Contains(attrs, `class="root"`) && !cur.local:
				next.dx, next.dy = cur.dx+num(m[1]), cur.dy+num(m[2])
				tag = strings.Replace(tag, " "+m[0], "", 1)
			case m != nil:
				if !cur.local && (cur.dx != 0 || cur.dy != 0) {
					tag = strings.Replace(tag, m[0], `transform="translate(`+format(num(m[1])+cur.dx)+", "+format(num(m[2])+cur.dy)+`)"`, 1)
				}
				next.local = true
			}
			if !strings.HasSuffix(attrs, "/") {
				stack = append(stack, next)
			}
		case !closing && !cur.local && (cur.dx != 0 || cur.dy != 0):
			if name == "rect" {
				tag = xyRE.ReplaceAllStringFunc(tag, func(v string) string {
					m := xyRE.FindStringSubmatch(v)
					d := cur.dx
					if m[1] == "y" {
						d = cur.dy
					}
					return m[1] + `="` + format(num(m[2])+d) + `"`
				})
			} else {
				tag = pointsRE.ReplaceAllStringFunc(tag, func(v string) string {
					return `data-points="` + movePoints(pointsRE.FindStringSubmatch(v)[1], cur.dx, cur.dy) + `"`
				})
			}
		}
		b.WriteString(tag)
	}
	b.WriteString(s[last:])
	return b.String()
}

// movePoints moves the points of a route in base64 by dx and dy. It keeps
// points that it cannot read as they are.
func movePoints(data string, dx, dy float64) string {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return data
	}
	var pts []map[string]float64
	if json.Unmarshal(raw, &pts) != nil {
		return data
	}
	for _, p := range pts {
		p["x"] += dx
		p["y"] += dy
	}
	out, err := json.Marshal(pts)
	if err != nil {
		return data
	}
	return base64.StdEncoding.EncodeToString(out)
}

// format writes a number as the SVG of Mermaid does.
func format(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
