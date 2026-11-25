package zuzu

import (
	"slices"

	"github.com/xo/zuzu/dataflow"
	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/vega"
)

// treeIndex is the tree of a dataset that a stratify transform built,
// which treePath and treeAncestors read. A node is a row of the dataset,
// after all of its transforms, so it holds the fields of a layout, such as
// the x and the y of a tree transform. A node is known by its key as a
// string, as the property of an object of JavaScript is.
type treeIndex struct {
	rows   map[string]dataflow.Row
	parent map[string]string
}

// tree returns the tree of a dataset: the tree of its stratify transform,
// or of the dataset that it derives from. A dataset with no stratify gives
// nil.
func (s *scope) tree(name string) (*treeIndex, error) {
	sc, ds := s.findData(name)
	if ds == nil {
		_, err := s.dataset(name)
		return nil, err
	}
	if ds.tree != nil {
		return ds.tree, nil
	}
	i := slices.IndexFunc(ds.def.Transform, func(t vega.Transform) bool { return t.Type == "stratify" })
	if i < 0 {
		if len(ds.def.Source.List) == 1 {
			return sc.tree(ds.def.Source.List[0])
		}
		return nil, nil
	}
	param := func(name string) (string, error) {
		v, ok := ds.def.Transform[i].Param(name)
		if !ok {
			return "", nil
		}
		x, err := sc.value(v)
		if err != nil {
			return "", err
		}
		if m, ok := x.(map[string]any); ok {
			x = m["field"]
		}
		return expr.String(x), nil
	}
	keyField, err := param("key")
	if err != nil {
		return nil, err
	}
	parentField, err := param("parentKey")
	if err != nil {
		return nil, err
	}
	rows, err := sc.dataset(name)
	if err != nil {
		return nil, err
	}
	key, parent := dataflow.FieldAccessor(keyField), dataflow.FieldAccessor(parentField)
	t := &treeIndex{rows: map[string]dataflow.Row{}, parent: map[string]string{}}
	for _, r := range rows {
		t.rows[expr.String(key.Get(r))] = r
	}
	for _, r := range rows {
		p := parent.Get(r)
		if p == nil || expr.IsUndefined(p) {
			continue
		}
		if _, ok := t.rows[expr.String(p)]; ok {
			t.parent[expr.String(key.Get(r))] = expr.String(p)
		}
	}
	ds.tree = t
	return t, nil
}

// ancestors returns the keys of a node and of each node above it, up to
// the root, or nil for a key that the tree does not have.
func (t *treeIndex) ancestors(key string) []string {
	if _, ok := t.rows[key]; !ok {
		return nil
	}
	out := []string{key}
	for len(out) <= len(t.rows) {
		p, ok := t.parent[out[len(out)-1]]
		if !ok {
			break
		}
		out = append(out, p)
	}
	return out
}

// path returns the keys of the shortest path from a node to another: up to
// the lowest node above both, then down. It is nil when the tree does not
// have one of the keys.
func (t *treeIndex) path(from, to string) []string {
	up, down := t.ancestors(from), t.ancestors(to)
	for i, k := range up {
		if j := slices.Index(down, k); j >= 0 {
			out := slices.Clone(up[:i+1])
			for j--; j >= 0; j-- {
				out = append(out, down[j])
			}
			return out
		}
	}
	return nil
}

// treeRows returns the rows of the nodes of keys, or undefined for none.
func (t *treeIndex) treeRows(keys []string) any {
	if keys == nil {
		return expr.Undefined
	}
	out := make([]any, len(keys))
	for i, k := range keys {
		out[i] = t.rows[k]
	}
	return out
}

// treeFuncs adds treePath and treeAncestors, which read the tree of a
// dataset. A dataset with no tree, or a key that the tree does not have,
// gives undefined.
func (s *scope) treeFuncs(fns map[string]expr.Func) {
	str := func(args []any, i int) string {
		if i < len(args) {
			return expr.String(args[i])
		}
		return expr.String(expr.Undefined)
	}
	fns["treePath"] = func(_ *expr.Scope, args []any) (any, error) {
		t, err := s.tree(str(args, 0))
		if err != nil || t == nil {
			return expr.Undefined, err
		}
		return t.treeRows(t.path(str(args, 1), str(args, 2))), nil
	}
	fns["treeAncestors"] = func(_ *expr.Scope, args []any) (any, error) {
		t, err := s.tree(str(args, 0))
		if err != nil || t == nil {
			return expr.Undefined, err
		}
		return t.treeRows(t.ancestors(str(args, 1))), nil
	}
}
