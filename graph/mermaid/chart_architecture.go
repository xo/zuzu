package mermaid

import (
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/text"
)

// The parts of the grammar of an architecture diagram.
const (
	archID    = `[\w](?:[-\w]*\w)?`
	archTitle = `\[(?:"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|[^\[\]\r\n]+)\]`
	archIcon  = `\(([\w\-:]+)\)`
)

var (
	// archGroupRE matches a group: group id(icon)[Title] in parent.
	archGroupRE = regexp.MustCompile(`^group\s+(` + archID + `)\s*(?:` + archIcon + `)?\s*(` + archTitle + `)?\s*(?:in\s+(` + archID + `))?$`)
	// archServiceRE matches a service: service id(icon)[Title] in group,
	// or service id "text"[Title] in group.
	archServiceRE = regexp.MustCompile(`^service\s+(` + archID + `)\s*(?:` + archIcon + `|("(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'))?\s*(` + archTitle + `)?\s*(?:in\s+(` + archID + `))?$`)
	// archJunctionRE matches a junction: junction id in group.
	archJunctionRE = regexp.MustCompile(`^junction\s+(` + archID + `)\s*(?:in\s+(` + archID + `))?$`)
	// archEdgeRE matches an edge: a{group}:R <-[Title]-> L:b{group}.
	archEdgeRE = regexp.MustCompile(`^(` + archID + `)\s*(\{group\})?\s*:\s*([LRTB])\s*([<>])?\s*(?:--|-\s*(` + archTitle + `)\s*-)\s*([<>])?\s*([LRTB])\s*:\s*(` + archID + `)\s*(\{group\})?$`)
	// archAlignRE matches an alignment: align row a b c.
	archAlignRE = regexp.MustCompile(`^align\s+(row|column)\s+(.+)$`)
	// archIDRE matches an id alone.
	archIDRE = regexp.MustCompile(`^` + archID + `$`)
)

// archNode is a service or a junction of an architecture diagram.
type archNode struct {
	id       string
	junction bool
	icon     string
	iconText string
	title    string
	in       string
	line     int

	// The layout: the lines of the title and their size, and the center
	// of the tile, or of the junction.
	lines  []string
	lw, lh float64
	x, y   float64
}

// archGroup is a group of an architecture diagram.
type archGroup struct {
	id, icon, title, in string
	line                int
	depth               int

	// The layout: the box, and the width of the title with its icon.
	x, y, w, h float64
	tw         float64
}

// archEdge is an edge of an architecture diagram between two services or
// junctions, with the sides it leaves and enters.
type archEdge struct {
	a, b           string
	aSide, bSide   string
	aInto, bInto   bool
	aGroup, bGroup bool
	title          string
	line           int
}

// archAlign is an alignment of services and junctions in a row or a
// column.
type archAlign struct {
	row     bool
	members []string
	line    int
}

// archItem is a service, a junction or a group in the grid of the group
// that holds it, or of the diagram. gx and gy are its cell. w and h are its
// size, and ax and ay the offset of its anchor from its top left: the
// center of the tile of a service, and the center of a group or a
// junction.
type archItem struct {
	node   *archNode
	group  *archGroup
	line   int
	gx, gy int
	placed bool
	w, h   float64
	ax, ay float64
	// x and y are the top left of the item in its container.
	x, y float64
}

// archRel is a relation of two items of a grid: b is at the offset dx, dy
// from a.
type archRel struct {
	a, b   *archItem
	dx, dy int
}

// arch is the state of the compile of an architecture diagram.
type arch struct {
	c      *chart
	nodes  []*archNode
	groups []*archGroup
	edges  []*archEdge
	aligns []*archAlign
	byID   map[string]any

	tile, font, pad, gap float64
	m                    text.Measurer
}

// architectureChart compiles an architecture diagram: groups, services,
// junctions and edges with the sides they join, into a Vega spec laid out
// in Go on a grid (D31). An edge `a:R -- L:b` puts b to the right of a, and
// a group is a box around a grid of its own.
func architectureChart(c *chart) (map[string]any, bool, error) {
	a := &arch{c: c, byID: map[string]any{}, tile: 56, font: 11, pad: 16, gap: 40, m: text.NewEstimate()}
	// The layout uses no random numbers, so architecture.randomize changes
	// nothing and is not an issue.
	if err := c.unreadSettings("architecture.iconSize", "architecture.fontSize", "architecture.padding", "architecture.nodeSeparation", "architecture.randomize"); err != nil {
		return nil, false, err
	}
	for _, s := range []struct {
		key string
		v   *float64
	}{{"iconSize", &a.tile}, {"fontSize", &a.font}, {"padding", &a.pad}, {"nodeSeparation", &a.gap}} {
		if v, ok := configNumber(c.d.Config, "architecture", s.key); ok && v > 0 {
			*s.v = v
		}
	}
	if c.d.Header != "" {
		c.line = c.d.N
		return nil, false, c.errorf("the text %q after %s", c.d.Header, c.d.Kind)
	}
	spec := map[string]any{}
	if err := a.parse(spec); err != nil {
		return nil, false, err
	}
	if err := a.check(); err != nil {
		return nil, false, err
	}
	c.line = 0
	a.layout()
	maps.Copy(spec, a.spec())
	return spec, false, nil
}

// parse reads the statements of the diagram.
func (a *arch) parse(spec map[string]any) error {
	c := a.c
	lines := c.d.Lines
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		c.line = l.N
		t := strings.TrimSuffix(l.Text, ";")
		if rest, ok := keyword(t, "title"); ok {
			spec["title"] = map[string]any{"text": rest, "anchor": "start"}
			continue
		}
		j, ok, err := c.accessibility(lines, i)
		if err != nil {
			return err
		}
		if ok {
			i = j
			continue
		}
		switch {
		case strings.HasPrefix(t, "group "):
			m := archGroupRE.FindStringSubmatch(t)
			if m == nil {
				return c.errorf("the group %q", t)
			}
			a.groups = append(a.groups, &archGroup{id: m[1], icon: m[2], title: archTitleText(m[3]), in: m[4], line: l.N})
		case strings.HasPrefix(t, "service "):
			m := archServiceRE.FindStringSubmatch(t)
			if m == nil {
				return c.errorf("the service %q", t)
			}
			n := &archNode{id: m[1], icon: m[2], title: archTitleText(m[4]), in: m[5], line: l.N}
			if m[3] != "" {
				n.iconText = labelText(m[3][1 : len(m[3])-1])
			}
			a.nodes = append(a.nodes, n)
		case strings.HasPrefix(t, "junction "):
			m := archJunctionRE.FindStringSubmatch(t)
			if m == nil {
				return c.errorf("the junction %q", t)
			}
			a.nodes = append(a.nodes, &archNode{id: m[1], junction: true, in: m[2], line: l.N})
		case strings.HasPrefix(t, "align "):
			m := archAlignRE.FindStringSubmatch(t)
			if m == nil {
				return c.errorf("the alignment %q", t)
			}
			al := &archAlign{row: m[1] == "row", members: strings.Fields(m[2]), line: l.N}
			for _, id := range al.members {
				if !archIDRE.MatchString(id) {
					return c.errorf("the id %q of an alignment", id)
				}
			}
			a.aligns = append(a.aligns, al)
		default:
			m := archEdgeRE.FindStringSubmatch(t)
			if m == nil {
				return c.errorf("the line %q is not a group, a service, a junction or an edge", t)
			}
			a.edges = append(a.edges, &archEdge{
				a: m[1], aGroup: m[2] != "", aSide: m[3], aInto: m[4] != "", title: archTitleText(m[5]),
				bInto: m[6] != "", bSide: m[7], b: m[8], bGroup: m[9] != "", line: l.N,
			})
		}
	}
	return nil
}

// archTitleText reads a title in brackets, [Title] or ["Title"].
func archTitleText(s string) string {
	if s == "" {
		return ""
	}
	s = strings.TrimSpace(s[1 : len(s)-1])
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		s = s[1 : len(s)-1]
	}
	return labelText(s)
}

// check checks the ids, the parents, the edges and the alignments, in the
// order that Mermaid reads them: the groups, the services, the junctions,
// the edges and the alignments.
func (a *arch) check() error {
	c := a.c
	parent := func(id, in string, line int) error {
		c.line = line
		if in == "" {
			return nil
		}
		if in == id {
			return c.errorf("%s is in itself", id)
		}
		switch a.byID[in].(type) {
		case *archGroup:
			return nil
		case nil:
			return c.errorf("%s is in %s, and no group %s comes before it", id, in, in)
		}
		return c.errorf("%s is in %s, which is not a group", id, in)
	}
	add := func(id string, v any, line int) error {
		c.line = line
		if _, ok := a.byID[id]; ok {
			return c.errorf("a second item with the id %s", id)
		}
		a.byID[id] = v
		return nil
	}
	for _, g := range a.groups {
		if err := parent(g.id, g.in, g.line); err != nil {
			return err
		}
		if err := add(g.id, g, g.line); err != nil {
			return err
		}
		if p, ok := a.byID[g.in].(*archGroup); ok {
			g.depth = p.depth + 1
		}
	}
	slices.SortStableFunc(a.nodes, func(x, y *archNode) int {
		if x.junction == y.junction {
			return 0
		}
		if y.junction {
			return -1
		}
		return 1
	})
	for _, n := range a.nodes {
		if err := parent(n.id, n.in, n.line); err != nil {
			return err
		}
		if err := add(n.id, n, n.line); err != nil {
			return err
		}
	}
	slices.SortStableFunc(a.nodes, func(x, y *archNode) int { return x.line - y.line })
	for _, e := range a.edges {
		c.line = e.line
		for _, id := range []string{e.a, e.b} {
			switch a.byID[id].(type) {
			case *archNode:
			case *archGroup:
				return c.errorf("the edge joins the group %s, and an edge joins services and junctions. Add {group} after a service to join its group", id)
			default:
				return c.errorf("the edge joins %s, which does not exist", id)
			}
		}
		na, _ := a.byID[e.a].(*archNode)
		nb, _ := a.byID[e.b].(*archNode)
		for _, end := range []struct {
			n     *archNode
			group bool
		}{{na, e.aGroup}, {nb, e.bGroup}} {
			if !end.group {
				continue
			}
			if na.in != "" && na.in == nb.in {
				return c.errorf("the edge from %s to %s leaves a group with {group}, and both ends are in the group %s", e.a, e.b, na.in)
			}
			if end.n.in == "" {
				if err := c.add("the {group} of %s, which is in no group", end.n.id); err != nil {
					return err
				}
			}
		}
	}
	for _, al := range a.aligns {
		c.line = al.line
		if len(al.members) < 2 {
			return c.errorf("an alignment of one item")
		}
		seen := map[string]bool{}
		for _, id := range al.members {
			if _, ok := a.byID[id].(*archNode); !ok {
				return c.errorf("the alignment names %s, which is not a service or a junction", id)
			}
			if seen[id] {
				return c.errorf("the alignment names %s twice", id)
			}
			seen[id] = true
		}
	}
	return nil
}

// container returns the id of the group that holds a node or a group, or
// empty for the diagram.
func (a *arch) container(v any) string {
	switch t := v.(type) {
	case *archNode:
		return t.in
	case *archGroup:
		return t.in
	}
	return ""
}

// itemOf returns the item of a grid of a container that holds a node: the
// node, or the group in the container that holds it.
func (a *arch) itemOf(items map[any]*archItem, cont, id string) *archItem {
	v := a.byID[id]
	for v != nil {
		if a.container(v) == cont {
			return items[v]
		}
		in := a.container(v)
		if in == "" {
			return nil
		}
		v = a.byID[in]
	}
	return nil
}

// offset returns the cell of the end of an edge from its start, by the
// sides that the edge joins. An edge from the right side goes to the
// right, and an edge into a top side comes from above, so `a:R -- T:b` puts
// b to the right of a and below it. Two ends on the same side put b below
// a, for the left and the right sides, or to the right of a, for the top
// and the bottom sides, and the edge goes around.
func offset(as, bs string) (int, int) {
	if as == bs {
		if as == "L" || as == "R" {
			return 0, 1
		}
		return 1, 0
	}
	side := map[string][2]int{"L": {-1, 0}, "R": {1, 0}, "T": {0, -1}, "B": {0, 1}}
	o := side[as]
	b := side[bs]
	dx, dy := o[0], o[1]
	if dx == 0 {
		dx = -b[0]
	}
	if dy == 0 {
		dy = -b[1]
	}
	return dx, dy
}

// layout places the items: the grid of each group, from the innermost
// out, and then the grid of the diagram.
func (a *arch) layout() {
	for _, n := range a.nodes {
		if n.junction {
			continue
		}
		limit := max(2*a.tile, 100)
		if n.title != "" {
			n.lines, n.lw = plainLines(n.title, a.font, 400, limit, a.m)
			n.lh = float64(len(n.lines)) * a.font * 1.3
		}
	}
	for _, e := range a.edges {
		if e.title != "" {
			w := a.m.Width(text.Font{Family: text.Family, Size: a.font, Weight: 400}, e.title)
			a.gap = max(a.gap, w+24)
		}
	}
	items := map[any]*archItem{}
	for _, n := range a.nodes {
		it := &archItem{node: n, line: n.line}
		if n.junction {
			it.w, it.h = 10, 10
			it.ax, it.ay = 5, 5
		} else {
			it.w = max(a.tile, n.lw)
			it.h = a.tile
			if len(n.lines) > 0 {
				it.h += 6 + n.lh
			}
			it.ax, it.ay = it.w/2, a.tile/2
		}
		items[n] = it
	}
	for _, g := range a.groups {
		items[g] = &archItem{group: g, line: g.line}
		g.tw = a.m.Width(text.Font{Family: text.Family, Size: a.font, Weight: boldWeight}, g.title)
		if g.icon != "" {
			g.tw += 20
		}
	}
	// The groups lay out from the deepest, so that the size of each group
	// is known in the grid of its parent.
	groups := slices.Clone(a.groups)
	slices.SortStableFunc(groups, func(x, y *archGroup) int { return y.depth - x.depth })
	for _, g := range groups {
		w, h := a.grid(items, g.id)
		titleH := a.titleHeight()
		it := items[g]
		it.w = max(w, g.tw, a.tile) + 2*a.pad
		it.h = h + 2*a.pad + titleH
		it.ax, it.ay = it.w/2, it.h/2
	}
	a.grid(items, "")
	a.place(items, "", 0, 0)
}

// titleHeight returns the height of the title of a group.
func (a *arch) titleHeight() float64 {
	return math.Round(max(a.font*1.3, 14) + 8)
}

// children returns the items of a container in the order of the source.
func (a *arch) children(items map[any]*archItem, cont string) []*archItem {
	var out []*archItem
	for _, g := range a.groups {
		if g.in == cont {
			out = append(out, items[g])
		}
	}
	for _, n := range a.nodes {
		if n.in == cont {
			out = append(out, items[n])
		}
	}
	slices.SortStableFunc(out, func(x, y *archItem) int { return x.line - y.line })
	return out
}

// grid places the items of a container in cells, from the edges and the
// alignments between them, and sets the place of each item in the
// container. It returns the size of the grid.
//
// A walk in depth of each set of joined items places the first item, and
// then each item at its offset from the item that leads to it, so that a
// chain of items stays together. A cell that holds an item already sends
// the new item on to the first free cell: down for a step to the side,
// to the right for a step up or down, and on in its own direction up or
// down for a step at a slant. Each set of joined items goes
// to the right of the one before.
func (a *arch) grid(items map[any]*archItem, cont string) (float64, float64) {
	kids := a.children(items, cont)
	if len(kids) == 0 {
		return 0, 0
	}
	var rels []archRel
	for _, e := range a.edges {
		x, y := a.itemOf(items, cont, e.a), a.itemOf(items, cont, e.b)
		if x == nil || y == nil || x == y {
			continue
		}
		dx, dy := offset(e.aSide, e.bSide)
		rels = append(rels, archRel{x, y, dx, dy})
	}
	for _, al := range a.aligns {
		for i := 1; i < len(al.members); i++ {
			x, y := a.itemOf(items, cont, al.members[i-1]), a.itemOf(items, cont, al.members[i])
			if x == nil || y == nil || x == y {
				continue
			}
			if al.row {
				rels = append(rels, archRel{x, y, 1, 0})
			} else {
				rels = append(rels, archRel{x, y, 0, 1})
			}
		}
	}
	type step struct {
		to     *archItem
		dx, dy int
	}
	adj := map[*archItem][]step{}
	for _, r := range rels {
		adj[r.a] = append(adj[r.a], step{r.b, r.dx, r.dy})
		adj[r.b] = append(adj[r.b], step{r.a, -r.dx, -r.dy})
	}
	nextCol := 0
	for _, start := range kids {
		if start.placed {
			continue
		}
		occ := map[[2]int]*archItem{}
		var comp []*archItem
		put := func(it *archItem, x, y int) {
			it.gx, it.gy, it.placed = x, y, true
			occ[[2]int{x, y}] = it
			comp = append(comp, it)
		}
		var walk func(cur *archItem)
		walk = func(cur *archItem) {
			for _, s := range adj[cur] {
				if s.to.placed {
					continue
				}
				x, y := cur.gx+s.dx, cur.gy+s.dy
				sx, sy := 0, 1
				switch {
				case s.dx == 0:
					sx, sy = 1, 0
				case s.dy != 0:
					sy = s.dy
				}
				for occ[[2]int{x, y}] != nil {
					x, y = x+sx, y+sy
				}
				put(s.to, x, y)
				walk(s.to)
			}
		}
		put(start, 0, 0)
		walk(start)
		minX, minY, maxX := 0, 0, 0
		for _, it := range comp {
			minX, minY, maxX = min(minX, it.gx), min(minY, it.gy), max(maxX, it.gx)
		}
		for _, it := range comp {
			it.gx += nextCol - minX
			it.gy -= minY
		}
		nextCol += maxX - minX + 1
	}
	// The cells take the size of their items, so that the anchors of a
	// column line up, and so do those of a row.
	left, right, top, bottom := map[int]float64{}, map[int]float64{}, map[int]float64{}, map[int]float64{}
	cols, rows := 0, 0
	for _, it := range kids {
		left[it.gx] = max(left[it.gx], it.ax)
		right[it.gx] = max(right[it.gx], it.w-it.ax)
		top[it.gy] = max(top[it.gy], it.ay)
		bottom[it.gy] = max(bottom[it.gy], it.h-it.ay)
		cols, rows = max(cols, it.gx+1), max(rows, it.gy+1)
	}
	colX, rowY := make([]float64, cols+1), make([]float64, rows+1)
	for i := range cols {
		colX[i+1] = colX[i] + left[i] + right[i]
		if i < cols-1 && left[i+1]+right[i+1] > 0 {
			colX[i+1] += a.gap
		}
	}
	for i := range rows {
		rowY[i+1] = rowY[i] + top[i] + bottom[i]
		if i < rows-1 && top[i+1]+bottom[i+1] > 0 {
			rowY[i+1] += a.gap
		}
	}
	for _, it := range kids {
		it.x = colX[it.gx] + left[it.gx] - it.ax
		it.y = rowY[it.gy] + top[it.gy] - it.ay
	}
	return colX[cols], rowY[rows]
}

// place sets the places of the items of a container whose grid starts at
// x, y: the center of each node, and the box of each group.
func (a *arch) place(items map[any]*archItem, cont string, x, y float64) {
	for _, it := range a.children(items, cont) {
		ix, iy := x+it.x, y+it.y
		if n := it.node; n != nil {
			n.x, n.y = ix+it.ax, iy+it.ay
			continue
		}
		g := it.group
		g.x, g.y, g.w, g.h = ix, iy, it.w, it.h
		w, _ := a.size(items, g.id)
		inner := it.w - 2*a.pad
		a.place(items, g.id, ix+a.pad+(inner-w)/2, iy+a.pad+a.titleHeight())
	}
}

// size returns the size of the grid of a container that grid laid out.
func (a *arch) size(items map[any]*archItem, cont string) (float64, float64) {
	w, h := 0.0, 0.0
	for _, it := range a.children(items, cont) {
		w, h = max(w, it.x+it.w), max(h, it.y+it.h)
	}
	return w, h
}

// anchor returns the point where an edge meets a node on a side, and how
// far the edge goes straight out of the side. An edge with {group} meets
// the box of the group of the node instead.
func (a *arch) anchor(n *archNode, side string, group bool) (float64, float64, float64) {
	if g, ok := a.byID[n.in].(*archGroup); ok && group {
		switch side {
		case "L":
			return g.x, n.y, 12
		case "R":
			return g.x + g.w, n.y, 12
		case "T":
			return n.x, g.y, 12
		}
		return n.x, g.y + g.h, 12
	}
	if n.junction {
		return n.x, n.y, 0
	}
	h := a.tile / 2
	switch side {
	case "L":
		return n.x - h, n.y, 14
	case "R":
		return n.x + h, n.y, 14
	case "T":
		return n.x, n.y - h, 14
	}
	if len(n.lines) > 0 {
		return n.x, n.y + h + 6 + n.lh + 2, 12
	}
	return n.x, n.y + h, 14
}

// route returns the points of an edge between two sides: straight out of
// each side, and then across in steps at right angles.
func route(x0, y0, s0, x1, y1, s1 float64, side0, side1 string) [][2]float64 {
	dir := map[string][2]float64{"L": {-1, 0}, "R": {1, 0}, "T": {0, -1}, "B": {0, 1}}
	d0, d1 := dir[side0], dir[side1]
	p0 := [2]float64{x0 + d0[0]*s0, y0 + d0[1]*s0}
	p1 := [2]float64{x1 + d1[0]*s1, y1 + d1[1]*s1}
	h0, h1 := d0[0] != 0, d1[0] != 0
	pts := [][2]float64{{x0, y0}, p0}
	switch {
	case h0 && h1:
		mx := (p0[0] + p1[0]) / 2
		if side0 == side1 {
			mx = max(p0[0], p1[0])
			if side0 == "L" {
				mx = min(p0[0], p1[0])
			}
		}
		pts = append(pts, [2]float64{mx, p0[1]}, [2]float64{mx, p1[1]})
	case !h0 && !h1:
		my := (p0[1] + p1[1]) / 2
		if side0 == side1 {
			my = max(p0[1], p1[1])
			if side0 == "T" {
				my = min(p0[1], p1[1])
			}
		}
		pts = append(pts, [2]float64{p0[0], my}, [2]float64{p1[0], my})
	case h0:
		pts = append(pts, [2]float64{p1[0], p0[1]})
	default:
		pts = append(pts, [2]float64{p0[0], p1[1]})
	}
	pts = append(pts, p1, [2]float64{x1, y1})
	// Drop the points that repeat or that lie on a straight line.
	out := [][2]float64{pts[0]}
	for i := 1; i < len(pts); i++ {
		p := pts[i]
		last := out[len(out)-1]
		if math.Abs(p[0]-last[0]) < 0.01 && math.Abs(p[1]-last[1]) < 0.01 {
			continue
		}
		if len(out) >= 2 {
			prev := out[len(out)-2]
			if math.Abs((last[0]-prev[0])*(p[1]-last[1])-(last[1]-prev[1])*(p[0]-last[0])) < 0.01 {
				out[len(out)-1] = p
				continue
			}
		}
		out = append(out, p)
	}
	return out
}

// roundPath writes the path of the points of a route, with round corners.
func roundPath(pts [][2]float64) string {
	var b strings.Builder
	b.WriteString("M" + num(pts[0][0]) + "," + num(pts[0][1]))
	for i := 1; i < len(pts)-1; i++ {
		p, prev, next := pts[i], pts[i-1], pts[i+1]
		l0, l1 := math.Hypot(p[0]-prev[0], p[1]-prev[1]), math.Hypot(next[0]-p[0], next[1]-p[1])
		r := min(6, l0/2, l1/2)
		ax, ay := p[0]-(p[0]-prev[0])/l0*r, p[1]-(p[1]-prev[1])/l0*r
		bx, by := p[0]+(next[0]-p[0])/l1*r, p[1]+(next[1]-p[1])/l1*r
		fmt.Fprintf(&b, "L%s,%sQ%s,%s %s,%s", num(ax), num(ay), num(p[0]), num(p[1]), num(bx), num(by))
	}
	last := pts[len(pts)-1]
	b.WriteString("L" + num(last[0]) + "," + num(last[1]))
	return b.String()
}

// middle returns the point halfway along a route.
func middle(pts [][2]float64) (float64, float64) {
	total := 0.0
	for i := 1; i < len(pts); i++ {
		total += math.Hypot(pts[i][0]-pts[i-1][0], pts[i][1]-pts[i-1][1])
	}
	half := total / 2
	for i := 1; i < len(pts); i++ {
		l := math.Hypot(pts[i][0]-pts[i-1][0], pts[i][1]-pts[i-1][1])
		if l >= half && l > 0 {
			t := half / l
			return pts[i-1][0] + (pts[i][0]-pts[i-1][0])*t, pts[i-1][1] + (pts[i][1]-pts[i-1][1])*t
		}
		half -= l
	}
	return pts[0][0], pts[0][1]
}

// spec writes the Vega spec of the diagram that layout laid out.
func (a *arch) spec() map[string]any {
	type edgeRoute struct {
		e   *archEdge
		pts [][2]float64
	}
	var routes []edgeRoute
	for _, e := range a.edges {
		na, _ := a.byID[e.a].(*archNode)
		nb, _ := a.byID[e.b].(*archNode)
		x0, y0, s0 := a.anchor(na, e.aSide, e.aGroup)
		x1, y1, s1 := a.anchor(nb, e.bSide, e.bGroup)
		routes = append(routes, edgeRoute{e, route(x0, y0, s0, x1, y1, s1, e.aSide, e.bSide)})
	}
	// The diagram starts at 0, 0.
	minX, minY := math.Inf(1), math.Inf(1)
	for _, g := range a.groups {
		minX, minY = min(minX, g.x), min(minY, g.y)
	}
	for _, n := range a.nodes {
		h := a.tile / 2
		if n.junction {
			h = 5
		}
		minX, minY = min(minX, n.x-max(h, n.lw/2)), min(minY, n.y-h)
	}
	for _, r := range routes {
		for _, p := range r.pts {
			minX, minY = min(minX, p[0]), min(minY, p[1])
		}
	}
	for _, g := range a.groups {
		g.x, g.y = g.x-minX, g.y-minY
	}
	for _, n := range a.nodes {
		n.x, n.y = n.x-minX, n.y-minY
	}
	for _, r := range routes {
		for i := range r.pts {
			r.pts[i][0] -= minX
			r.pts[i][1] -= minY
		}
	}
	var groups, tiles, junctions, edges, arrows, labels, icons, iconTexts []any
	titleH := a.titleHeight()
	for _, g := range a.groups {
		tx := g.x + a.pad
		if g.icon != "" {
			icons = append(icons, map[string]any{"id": g.id, "icon": g.icon, "iconSize": 14, "x": tx + 7, "y": g.y + titleH/2 + 2})
			tx += 20
		}
		groups = append(groups, map[string]any{
			"id": g.id, "label": g.title, "x": g.x, "y": g.y, "width": g.w, "height": g.h, "depth": g.depth,
			"parent": nilIfEmpty(g.in), "titleX": tx, "titleY": g.y + titleH/2 + 2,
		})
	}
	for _, n := range a.nodes {
		if n.junction {
			junctions = append(junctions, map[string]any{"id": n.id, "x": n.x, "y": n.y, "parent": nilIfEmpty(n.in)})
			continue
		}
		lines := []any{}
		for _, l := range n.lines {
			lines = append(lines, l)
		}
		tiles = append(tiles, map[string]any{
			"id": n.id, "label": n.title, "lines": lines, "x": n.x - a.tile/2, "y": n.y - a.tile/2,
			"cx": n.x, "cy": n.y, "size": a.tile, "labelY": n.y + a.tile/2 + 6, "parent": nilIfEmpty(n.in),
		})
		if n.icon != "" {
			icons = append(icons, map[string]any{"id": n.id, "icon": n.icon, "iconSize": math.Round(a.tile * 0.6), "x": n.x, "y": n.y})
		}
		if n.iconText != "" {
			ls, _ := plainLines(n.iconText, a.font, 400, a.tile-8, a.m)
			var tl []any
			for _, l := range ls {
				tl = append(tl, l)
			}
			iconTexts = append(iconTexts, map[string]any{"id": n.id, "x": n.x, "y": n.y, "lines": tl})
		}
	}
	arrow, _ := graph.ArrowPaths("normal")
	for i, r := range routes {
		e := r.e
		edges = append(edges, map[string]any{"key": i, "source": e.a, "target": e.b, "path": roundPath(r.pts), "label": e.title})
		last := len(r.pts) - 1
		angle := func(p, q [2]float64) float64 { return math.Atan2(p[1]-q[1], p[0]-q[0]) * 180 / math.Pi }
		if e.aInto {
			arrows = append(arrows, map[string]any{"edge": i, "end": e.a, "x": r.pts[0][0], "y": r.pts[0][1], "angle": angle(r.pts[0], r.pts[1]), "shape": arrow})
		}
		if e.bInto {
			arrows = append(arrows, map[string]any{"edge": i, "end": e.b, "x": r.pts[last][0], "y": r.pts[last][1], "angle": angle(r.pts[last], r.pts[last-1]), "shape": arrow})
		}
		if e.title != "" {
			x, y := middle(r.pts)
			w := a.m.Width(text.Font{Family: text.Family, Size: a.font, Weight: 400}, e.title)
			labels = append(labels, map[string]any{"edge": i, "text": e.title, "x": x, "y": y, "width": w + 8, "height": a.font + 6})
		}
	}
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	value := func(v any) map[string]any { return map[string]any{"value": v} }
	from := func(d string) map[string]any { return map[string]any{"data": d} }
	data := []any{
		map[string]any{"name": "groups", "values": orEmpty(groups)},
		map[string]any{"name": "edges", "values": orEmpty(edges)},
		map[string]any{"name": "edgeLabels", "values": orEmpty(labels)},
		map[string]any{"name": "services", "values": orEmpty(tiles)},
		map[string]any{"name": "iconTexts", "values": orEmpty(iconTexts)},
		map[string]any{"name": "junctions", "values": orEmpty(junctions)},
		map[string]any{"name": "arrows", "values": orEmpty(arrows)},
	}
	marks := []any{
		map[string]any{"type": "rect", "name": "groups", "from": from("groups"), "style": []any{graph.StyleCluster}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "width": field("width"), "height": field("height"),
		}}},
		map[string]any{"type": "text", "name": "groupTitles", "from": from("groups"), "style": []any{graph.StyleClusterTitle}, "encode": map[string]any{"update": map[string]any{
			"x": field("titleX"), "y": field("titleY"), "text": field("label"), "fontSize": value(a.font),
			"align": value("left"), "baseline": value("middle"),
		}}},
		map[string]any{"type": "path", "name": "edges", "from": from("edges"), "style": []any{graph.StyleEdge}, "encode": map[string]any{"update": map[string]any{
			"x": value(0), "y": value(0), "path": field("path"),
		}}},
		map[string]any{"type": "rect", "name": "edgeLabelBoxes", "from": from("edgeLabels"), "style": []any{graph.StyleEdgeLabelBox}, "encode": map[string]any{"update": map[string]any{
			"xc": field("x"), "yc": field("y"), "width": field("width"), "height": field("height"),
		}}},
		map[string]any{"type": "text", "name": "edgeLabels", "from": from("edgeLabels"), "style": []any{graph.StyleEdgeLabel}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "text": field("text"), "fontSize": value(a.font),
			"align": value("center"), "baseline": value("middle"),
		}}},
		map[string]any{"type": "rect", "name": "tiles", "from": from("services"), "style": []any{graph.StyleNode}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "width": field("size"), "height": field("size"), "cornerRadius": value(8),
		}}},
	}
	if len(icons) > 0 {
		data = append(data, iconData("icons", icons)...)
		marks = append(marks, iconMarks("icons", nil)...)
	}
	marks = append(marks,
		map[string]any{"type": "text", "name": "iconTexts", "from": from("iconTexts"), "style": []any{graph.StyleNodeLabel}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "text": field("lines"), "fontSize": value(a.font), "lineHeight": value(a.font * 1.3),
			"align": value("center"), "baseline": value("middle"), "fontWeight": value(boldWeight),
		}}},
		map[string]any{"type": "text", "name": "labels", "from": from("services"), "style": []any{graph.StyleNodeLabel}, "encode": map[string]any{"update": map[string]any{
			"x": field("cx"), "y": field("labelY"), "text": field("lines"), "fontSize": value(a.font), "lineHeight": value(a.font * 1.3),
			"align": value("center"), "baseline": value("top"),
		}}},
		map[string]any{"type": "symbol", "name": "junctions", "from": from("junctions"), "style": []any{graph.StyleArrow}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "shape": value("circle"), "size": value(40),
		}}},
		map[string]any{"type": "symbol", "name": "arrows", "from": from("arrows"), "style": []any{graph.StyleArrow}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "angle": field("angle"), "shape": field("shape"), "size": value(36),
		}}},
	)
	return map[string]any{
		"padding":  8,
		"autosize": "pad",
		"data":     data,
		"marks":    marks,
	}
}

// nilIfEmpty returns nil for an empty string, and the string otherwise.
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// orEmpty returns an empty list for nil, so that a dataset with no rows
// writes [] and not null.
func orEmpty(v []any) []any {
	if v == nil {
		return []any{}
	}
	return v
}
