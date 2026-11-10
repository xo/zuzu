package dataflow

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/xo/zuzu/expr"
)

// Hierarchy is the tree that a stratify transform builds, which the
// transforms after it read, in its dataset and in the datasets that derive
// from it. It holds the fields that name each node and its parent, so that
// a transform builds the tree again from its own rows.
type Hierarchy struct {
	key, parentKey string
}

// HierarchyEnv is an Env that keeps the hierarchy of a view across its
// datasets. An Env without it keeps the hierarchy for one run of
// transforms.
type HierarchyEnv interface {
	Hierarchy() *Hierarchy
}

// hierarchyOf returns the hierarchy of the environment of a transform.
func (p *params) hierarchyOf() *Hierarchy {
	if h, ok := p.env.(HierarchyEnv); ok {
		return h.Hierarchy()
	}
	if p.local == nil {
		p.local = &Hierarchy{}
	}
	return p.local
}

// node is a node of a tree: a row, its parent and its children, and what
// the layouts work out.
type node struct {
	row      int
	parent   *node
	children []*node
	depth    int
	height   int
	value    float64
	x, y     float64
	x0, y0   float64
	x1, y1   float64
	r        float64
	// The fields of the tidy layout.
	prelim, mod, shift, change float64
	thread, ancestor           *node
	number                     int
}

// stratify checks that rows make a tree by a key and a parent key, and
// keeps the keys for the transforms after it. The rows stay as they are.
func stratify(p *params, rows []Row) ([]Row, error) {
	key, err := p.str("key", "")
	if err != nil {
		return nil, err
	}
	parent, err := p.str("parentKey", "")
	if err != nil {
		return nil, err
	}
	if key == "" || parent == "" {
		return nil, fmt.Errorf("stratify needs a key and a parent key: %w", ErrParam)
	}
	h := p.hierarchyOf()
	h.key, h.parentKey = key, parent
	if _, err := buildTree(h, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// buildTree builds the tree of rows by the keys of a hierarchy, with the
// children of each node in the order of the rows.
func buildTree(h *Hierarchy, rows []Row) (*node, error) {
	if h.key == "" {
		return nil, fmt.Errorf("a hierarchy transform with no stratify before it: %w", ErrParam)
	}
	key, parent := FieldAccessor(h.key), FieldAccessor(h.parentKey)
	nodes := make([]*node, len(rows))
	byKey := map[string]*node{}
	for i, r := range rows {
		nodes[i] = &node{row: i}
		byKey[expr.String(key.Get(r))] = nodes[i]
	}
	var root *node
	for i, r := range rows {
		pk := parent.Get(r)
		if pk == nil || expr.IsUndefined(pk) || expr.String(pk) == "" {
			if root != nil {
				return nil, fmt.Errorf("stratify: more than one root: %w", ErrParam)
			}
			root = nodes[i]
			continue
		}
		up, ok := byKey[expr.String(pk)]
		if !ok {
			return nil, fmt.Errorf("stratify: no parent %v: %w", pk, ErrParam)
		}
		nodes[i].parent = up
		up.children = append(up.children, nodes[i])
	}
	if root == nil {
		return nil, fmt.Errorf("stratify: no root: %w", ErrParam)
	}
	var walk func(n *node, depth int) int
	walk = func(n *node, depth int) int {
		n.depth = depth
		for _, c := range n.children {
			n.height = max(n.height, walk(c, depth+1)+1)
		}
		return n.height
	}
	walk(root, 0)
	return root, nil
}

// eachBefore visits the nodes of a tree, each before its children.
func eachBefore(n *node, f func(*node)) {
	f(n)
	for _, c := range n.children {
		eachBefore(c, f)
	}
}

// eachAfter visits the nodes of a tree, each after its children.
func eachAfter(n *node, f func(*node)) {
	for _, c := range n.children {
		eachAfter(c, f)
	}
	f(n)
}

// treeSeparation is the gap between two neighbors of the tidy and the
// cluster layouts: 1 between siblings and 2 between cousins, or always 1.
func treeSeparation(siblingsOnly bool) func(a, b *node) float64 {
	return func(a, b *node) float64 {
		if !siblingsOnly && a.parent != b.parent {
			return 2
		}
		return 1
	}
}

// sizeOf reads the parameter size, [width, height].
func sizeOf(p *params, name string) (float64, float64, bool, error) {
	l, err := p.list(name)
	if err != nil || len(l) < 2 {
		return 1, 1, false, err
	}
	return expr.Number(l[0]), expr.Number(l[1]), true, nil
}

// tree lays out a tree as a tidy tree, or as a cluster with every leaf at
// the same depth.
func tree(p *params, rows []Row) ([]Row, error) {
	root, err := buildTree(p.hierarchyOf(), rows)
	if err != nil {
		return nil, err
	}
	method, err := p.str("method", "tidy")
	if err != nil {
		return nil, err
	}
	sep, err := p.boolean("separation", true)
	if err != nil {
		return nil, err
	}
	dx, dy, _, err := sizeOf(p, "size")
	if err != nil {
		return nil, err
	}
	nx, ny, fixed, err := sizeOf(p, "nodeSize")
	if err != nil {
		return nil, err
	}
	as, err := p.as("x", "y", "depth", "children")
	if err != nil {
		return nil, err
	}
	separation := treeSeparation(!sep)
	if method == "cluster" {
		cluster(root, separation, dx, dy, fixed, nx, ny)
	} else {
		tidy(root, separation, dx, dy, fixed, nx, ny)
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		out[i] = clone(r)
	}
	eachBefore(root, func(n *node) {
		r := out[n.row]
		r[as[0]], r[as[1]] = n.x, n.y
		r[as[2]], r[as[3]] = float64(n.depth), float64(len(n.children))
	})
	return out, nil
}

// tidy lays out a tree by the algorithm of Walker as Buchheim, Jünger and
// Leipert improved it, in linear time. Each node is centered over its
// children, neighbors keep their separation, and subtrees move apart as
// little as they must. The layout then scales to the size, with half a
// separation at each side, or takes the size of a node.
func tidy(root *node, separation func(a, b *node) float64, dx, dy float64, fixed bool, nx, ny float64) {
	eachBefore(root, func(n *node) {
		n.ancestor = n
		n.thread, n.prelim, n.mod, n.shift, n.change = nil, 0, 0, 0, 0
		for i, c := range n.children {
			c.number = i
		}
	})
	firstWalk(root, separation)
	secondWalk(root, -root.prelim)
	if fixed {
		eachBefore(root, func(n *node) {
			n.x *= nx
			n.y = float64(n.depth) * ny
		})
		return
	}
	left, right, bottom := root, root, root
	eachBefore(root, func(n *node) {
		if n.x < left.x {
			left = n
		}
		if n.x > right.x {
			right = n
		}
		if n.depth > bottom.depth {
			bottom = n
		}
	})
	s := 1.0
	if left != right {
		s = separation(left, right) / 2
	}
	tx := s - left.x
	kx := dx / (right.x + s + tx)
	ky := dy / float64(max(bottom.depth, 1))
	eachBefore(root, func(n *node) {
		n.x = (n.x + tx) * kx
		n.y = float64(n.depth) * ky
	})
}

// leftSibling returns the sibling before a node, or nil.
func leftSibling(v *node) *node {
	if v.parent == nil || v.number == 0 {
		return nil
	}
	return v.parent.children[v.number-1]
}

// leftmostSibling returns the first sibling of a node, or nil when it is
// the first itself.
func leftmostSibling(v *node) *node {
	if v.parent == nil || v.number == 0 {
		return nil
	}
	return v.parent.children[0]
}

func nextLeft(v *node) *node {
	if len(v.children) > 0 {
		return v.children[0]
	}
	return v.thread
}

func nextRight(v *node) *node {
	if len(v.children) > 0 {
		return v.children[len(v.children)-1]
	}
	return v.thread
}

// firstWalk works out the preliminary place of each node, after the places
// of its children.
func firstWalk(v *node, separation func(a, b *node) float64) {
	w := leftSibling(v)
	if len(v.children) == 0 {
		if w != nil {
			v.prelim = w.prelim + separation(v, w)
		}
		return
	}
	defaultAncestor := v.children[0]
	for _, c := range v.children {
		firstWalk(c, separation)
		defaultAncestor = apportion(c, defaultAncestor, separation)
	}
	executeShifts(v)
	mid := (v.children[0].prelim + v.children[len(v.children)-1].prelim) / 2
	if w != nil {
		v.prelim = w.prelim + separation(v, w)
		v.mod = v.prelim - mid
	} else {
		v.prelim = mid
	}
}

// apportion moves a subtree away from the subtrees to its left, as far as
// the contours of both ask, and spreads the move over the subtrees between.
func apportion(v, defaultAncestor *node, separation func(a, b *node) float64) *node {
	w := leftSibling(v)
	if w == nil {
		return defaultAncestor
	}
	vip, vop := v, v
	vim, vom := w, leftmostSibling(v)
	sip, sop := vip.mod, vop.mod
	sim, som := vim.mod, vom.mod
	for nextRight(vim) != nil && nextLeft(vip) != nil {
		vim, vip = nextRight(vim), nextLeft(vip)
		vom, vop = nextLeft(vom), nextRight(vop)
		vop.ancestor = v
		shift := (vim.prelim + sim) - (vip.prelim + sip) + separation(vim, vip)
		if shift > 0 {
			moveSubtree(ancestorOf(vim, v, defaultAncestor), v, shift)
			sip += shift
			sop += shift
		}
		sim += vim.mod
		sip += vip.mod
		som += vom.mod
		sop += vop.mod
	}
	if nextRight(vim) != nil && nextRight(vop) == nil {
		vop.thread = nextRight(vim)
		vop.mod += sim - sop
	}
	if nextLeft(vip) != nil && nextLeft(vom) == nil {
		vom.thread = nextLeft(vip)
		vom.mod += sip - som
		defaultAncestor = v
	}
	return defaultAncestor
}

// ancestorOf returns the ancestor of vim that is a sibling of v, or the
// default ancestor.
func ancestorOf(vim, v, defaultAncestor *node) *node {
	if vim.ancestor.parent == v.parent {
		return vim.ancestor
	}
	return defaultAncestor
}

// moveSubtree moves the subtree of wp right by a shift, and spreads the
// shift over the subtrees between wm and wp.
func moveSubtree(wm, wp *node, shift float64) {
	change := shift / float64(wp.number-wm.number)
	wp.change -= change
	wp.shift += shift
	wm.change += change
	wp.prelim += shift
	wp.mod += shift
}

// executeShifts applies the shifts that apportion spread over the children
// of a node.
func executeShifts(v *node) {
	shift, change := 0.0, 0.0
	for _, w := range slices.Backward(v.children) {
		w.prelim += shift
		w.mod += shift
		change += w.change
		shift += w.shift + change
	}
}

// secondWalk works out the final place of each node from the sum of the
// modifiers of its ancestors.
func secondWalk(v *node, m float64) {
	v.x = v.prelim + m
	for _, c := range v.children {
		secondWalk(c, m+v.mod)
	}
}

// cluster lays out a tree as a dendrogram: the leaves side by side, each
// parent over the middle of its children, and every leaf at the bottom.
func cluster(root *node, separation func(a, b *node) float64, dx, dy float64, fixed bool, nx, ny float64) {
	var prev *node
	x := 0.0
	eachAfter(root, func(n *node) {
		if len(n.children) > 0 {
			s, top := 0.0, 0.0
			for _, c := range n.children {
				s += c.x
				top = math.Max(top, c.y)
			}
			n.x = s / float64(len(n.children))
			n.y = 1 + top
			return
		}
		if prev != nil {
			x += separation(n, prev)
		}
		n.x, n.y = x, 0
		prev = n
	})
	leftLeaf, rightLeaf := root, root
	for len(leftLeaf.children) > 0 {
		leftLeaf = leftLeaf.children[0]
	}
	for len(rightLeaf.children) > 0 {
		rightLeaf = rightLeaf.children[len(rightLeaf.children)-1]
	}
	if fixed {
		eachAfter(root, func(n *node) {
			n.x = (n.x - root.x) * nx
			n.y = (root.y - n.y) * ny
		})
		return
	}
	x0 := leftLeaf.x - separation(leftLeaf, rightLeaf)/2
	x1 := rightLeaf.x + separation(rightLeaf, leftLeaf)/2
	eachAfter(root, func(n *node) {
		n.x = (n.x - x0) / (x1 - x0) * dx
		if root.y != 0 {
			n.y = (1 - n.y/root.y) * dy
		} else {
			n.y = 0
		}
	})
}

// treelinks writes a link from each node of a tree to each of its
// children, with the rows of both.
func treelinks(p *params, rows []Row) ([]Row, error) {
	root, err := buildTree(p.hierarchyOf(), rows)
	if err != nil {
		return nil, err
	}
	var out []Row
	// The links come in the order of a walk of the tree by depth.
	queue := []*node{root}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, c := range n.children {
			out = append(out, Row{"source": rows[n.row], "target": rows[c.row]})
			queue = append(queue, c)
		}
	}
	return out, nil
}

// linkpath writes the path of a link from a source point to a target
// point, as a line, an arc, a curve, a diagonal or an orthogonal path, in
// horizontal, vertical or radial coordinates.
func linkpath(p *params, rows []Row) ([]Row, error) {
	get := func(name, def string) (*Accessor, error) {
		a, err := p.field(name)
		if err != nil || a != nil {
			return a, err
		}
		return FieldAccessor(def), nil
	}
	sx, err := get("sourceX", "source.x")
	if err != nil {
		return nil, err
	}
	sy, err := get("sourceY", "source.y")
	if err != nil {
		return nil, err
	}
	tx, err := get("targetX", "target.x")
	if err != nil {
		return nil, err
	}
	ty, err := get("targetY", "target.y")
	if err != nil {
		return nil, err
	}
	orient, err := p.str("orient", "vertical")
	if err != nil {
		return nil, err
	}
	shape, err := p.str("shape", "line")
	if err != nil {
		return nil, err
	}
	as, err := p.str("as", "path")
	if err != nil {
		return nil, err
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		a := [2]float64{expr.Number(sx.Get(r)), expr.Number(sy.Get(r))}
		b := [2]float64{expr.Number(tx.Get(r)), expr.Number(ty.Get(r))}
		path, err := linkShape(shape, orient, a, b)
		if err != nil {
			return nil, err
		}
		out[i] = clone(r)
		out[i][as] = path
	}
	return out, nil
}

// num writes a number of a path as JavaScript writes it.
func num(x float64) string {
	return expr.String(x)
}

// linkShape writes the path of one link. In radial coordinates, x is an
// angle in radians and y a radius.
func linkShape(shape, orient string, a, b [2]float64) (string, error) {
	polar := func(p [2]float64) [2]float64 {
		return [2]float64{p[1] * math.Cos(p[0]), p[1] * math.Sin(p[0])}
	}
	pt := func(p [2]float64) string { return num(p[0]) + "," + num(p[1]) }
	var sb strings.Builder
	switch shape {
	case "line":
		if orient == "radial" {
			a, b = polar(a), polar(b)
		}
		sb.WriteString("M" + pt(a) + "L" + pt(b))
	case "arc":
		if orient == "radial" {
			a, b = polar(a), polar(b)
		}
		r := math.Hypot(b[0]-a[0], b[1]-a[1]) / 2
		sb.WriteString("M" + pt(a) + "A" + num(r) + "," + num(r) + " 0 0 1 " + pt(b))
	case "curve", "diagonal":
		switch orient {
		case "horizontal":
			m := (a[0] + b[0]) / 2
			if shape == "curve" {
				dx := b[0] - a[0]
				sb.WriteString("M" + pt(a) + "C" + pt([2]float64{a[0] + dx/2, a[1]}) + " " + pt([2]float64{b[0] - dx/2, b[1]}) + " " + pt(b))
				break
			}
			sb.WriteString("M" + pt(a) + "C" + pt([2]float64{m, a[1]}) + " " + pt([2]float64{m, b[1]}) + " " + pt(b))
		case "radial":
			m := (a[1] + b[1]) / 2
			p0, p1 := polar(a), polar([2]float64{a[0], m})
			p2, p3 := polar([2]float64{b[0], m}), polar(b)
			sb.WriteString("M" + pt(p0) + "C" + pt(p1) + " " + pt(p2) + " " + pt(p3))
		default:
			m := (a[1] + b[1]) / 2
			sb.WriteString("M" + pt(a) + "C" + pt([2]float64{a[0], m}) + " " + pt([2]float64{b[0], m}) + " " + pt(b))
		}
	case "orthogonal":
		switch orient {
		case "horizontal":
			sb.WriteString("M" + pt(a) + "V" + num(b[1]) + "H" + num(b[0]))
		case "radial":
			p0, p1 := polar(a), polar([2]float64{b[0], a[1]})
			r := math.Abs(a[1])
			sweep := "1"
			if b[0] < a[0] {
				sweep = "0"
			}
			sb.WriteString("M" + pt(p0) + "A" + num(r) + "," + num(r) + " 0 0 " + sweep + " " + pt(p1) + "L" + pt(polar(b)))
		default:
			sb.WriteString("M" + pt(a) + "H" + num(b[0]) + "V" + num(b[1]))
		}
	default:
		return "", fmt.Errorf("linkpath shape %q: %w", shape, ErrParam)
	}
	return sb.String(), nil
}

// sortNodes sorts the children of each node by a comparator of the nodes,
// which reads the fields value, depth and height of a node and the fields
// of its row under data.
func sortNodes(root *node, cmp func(a, b Row) int, rows []Row) {
	if cmp == nil {
		return
	}
	view := func(n *node) Row {
		return Row{"value": n.value, "depth": float64(n.depth), "height": float64(n.height), "data": rows[n.row]}
	}
	eachBefore(root, func(n *node) {
		slices.SortStableFunc(n.children, func(a, b *node) int { return cmp(view(a), view(b)) })
		for i, c := range n.children {
			c.number = i
		}
	})
}

// sumNodes sets the value of each node to its own value of a field plus
// the values of its children, or to the count of its leaves when there is
// no field.
func sumNodes(root *node, field *Accessor, rows []Row) {
	eachAfter(root, func(n *node) {
		v := 0.0
		if field != nil {
			if x := expr.Number(field.Get(rows[n.row])); !math.IsNaN(x) {
				v = x
			}
		} else if len(n.children) == 0 {
			v = 1
		}
		for _, c := range n.children {
			v += c.value
		}
		n.value = v
	})
}
