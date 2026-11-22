package zuzu

import (
	"fmt"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/vega"
)

// The scene graph of Vega gives an expression of an encoding the item that
// the encoding fills as item. An item has its datum and its mark. A mark
// has its kind as marktype, its name, its role, and its group: the item of
// the group mark that holds it. The values here give that part of the
// scene graph. A property that zuzu does not keep, such as the bounds of an
// item or the items of a mark, goes in the report.

// itemValue is the item that an encoding fills, which an expression reads
// as item.
type itemValue struct {
	// s is the scope of the mark of the item.
	s  *scope
	st *itemState
	// mark is the mark of the item, or nil for an item of a guide or the
	// root group, whose mark zuzu does not give.
	mark *scene.Mark
	path string
}

// Get returns a property of the item: its datum or its mark. A property
// that the encodings set, such as x, goes in the report, because zuzu does
// not apply the properties of an encoding in the order of the spec.
func (v *itemValue) Get(name string) any {
	switch name {
	case "datum":
		if v.st.it.Datum == nil {
			return expr.Undefined
		}
		return v.st.it.Datum
	case "mark":
		if v.mark != nil {
			return &markValue{s: v.s, mark: v.mark, path: v.path}
		}
		v.s.itemIssuef(v.path, "item.mark in an encoding of a guide or of the root group")
		return expr.Undefined
	}
	v.s.itemIssuef(v.path, "the property %s of item", name)
	return expr.Undefined
}

// markValue is the mark of an item, which an expression reads as
// item.mark.
type markValue struct {
	// s is the scope that holds the mark.
	s    *scope
	mark *scene.Mark
	path string
}

// Get returns a property of the mark: its group, its kind, its name or its
// role.
func (v *markValue) Get(name string) any {
	switch name {
	case "group":
		return &groupValue{g: v.s, path: v.path}
	case "marktype":
		return v.mark.Type
	case "role":
		return v.mark.Role
	case "name":
		if v.mark.Name == "" {
			return expr.Undefined
		}
		return v.mark.Name
	}
	v.s.itemIssuef(v.path, "the property %s of item.mark", name)
	return expr.Undefined
}

// groupValue is the item of the group that holds a mark, which an
// expression reads as item.mark.group. The root group is the group of a
// mark at the top of the spec.
type groupValue struct {
	g    *scope
	path string
}

// Get returns a property of the group item: its place and size, its datum,
// its mark, or a property that its encodings set, such as fill. A property
// that the group does not have is undefined, as in Vega.
func (v *groupValue) Get(name string) any {
	g := v.g
	switch name {
	case "x", "y", "width", "height":
		if g.item == nil {
			return expr.Undefined
		}
		return itemField(g.item, name)
	case "datum":
		if g.datum == nil {
			return expr.Undefined
		}
		return g.datum
	case "mark":
		if g.mark == nil || g.parent == nil {
			v.g.itemIssuef(v.path, "the mark of the root group")
			return expr.Undefined
		}
		return &markValue{s: g.parent, mark: g.mark, path: v.path}
	case "items", "bounds":
		v.g.itemIssuef(v.path, "the property %s of a group item", name)
		return expr.Undefined
	}
	if x, ok := g.values[name]; ok {
		return x
	}
	return expr.Undefined
}

// itemIssuef reports a part of item that zuzu does not give, once for each
// path and part. The strict mode keeps the error for the expression that
// reads it.
func (s *scope) itemIssuef(path, format string, args ...any) {
	if err := s.issueOncef(path, format, args...); err != nil && s.rt.err == nil {
		s.rt.err = err
	}
}

// issueOncef reports an unsupported feature once for each path and message,
// for a feature that each item of a mark meets.
func (s *scope) issueOncef(path, format string, args ...any) error {
	key := path + "\x00" + fmt.Sprintf(format, args...)
	if s.rt.itemIssues[key] {
		return nil
	}
	if s.rt.itemIssues == nil {
		s.rt.itemIssues = map[string]bool{}
	}
	s.rt.itemIssues[key] = true
	return s.rt.issue(vega.ErrUnsupported, path, format, args...)
}
