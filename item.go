package zuzu

import (
	"fmt"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/vega"
)

// The scene graph of Vega gives an expression of an encoding the item that
// the encoding fills as item. An item has its datum, its mark, and each
// property that an encoding set before, because vega.js applies the
// properties of each set in the order of the spec, enter before update. A
// group item also has its marks as items. A mark has its kind as marktype,
// its name, its role, its items, and its group: the item of the group mark
// that holds it. The types here give that part of the scene graph without
// a copy of an item. The bounds of an item go in the report, because
// vega.js gives them only from an earlier pulse.

// Get returns a property of the item, which an expression reads as
// item.name: its datum, its mark, the marks of a group item, or a property
// that an encoding set. A property that no encoding set yet is undefined,
// as in vega.js.
func (st *itemState) Get(name string) any {
	switch name {
	case "datum":
		if st.it.Datum == nil {
			return expr.Undefined
		}
		return st.it.Datum
	case "mark":
		if st.mark == nil {
			return expr.Undefined
		}
		return st.mark
	case "items":
		if l := st.marks(); l != nil {
			return l.value()
		}
		if st.mark != nil && st.mark.sm.Type == scene.Group {
			// The marks of a group item come after the encodings of
			// its mark, as in vega.js.
			return []any{}
		}
		return expr.Undefined
	case "bounds":
		if st.mark != nil {
			st.mark.s.boundsIssue()
		}
		return expr.Undefined
	}
	if v, ok := st.values[name]; ok {
		return v
	}
	if st.mark != nil && st.mark.guide {
		return st.guideField(name)
	}
	return expr.Undefined
}

// guideField returns a property of an item of a guide that no encode of
// the spec set. The layout of zuzu gives the item its text and its place,
// as the encoders of a guide of vega.js do. The place is in the frame of
// Vega, which zuzu knows for the parts of an axis. Another place, and a
// size, go in the report.
func (st *itemState) guideField(name string) any {
	ms := st.mark
	for i := range ms.sets {
		for _, p := range ms.sets[i].props {
			if p.name == name {
				// The encode of the spec takes the place of the encoder
				// of vega.js, so the property is undefined until the
				// encode sets it.
				return expr.Undefined
			}
		}
	}
	switch name {
	case "text":
		return textValue(st.it.Text)
	case "x", "x2", "y", "y2":
		if ms.frame == nil {
			break
		}
		ox, oy, ok := ms.frame(ms.sm.Role)
		if !ok {
			break
		}
		v := expr.Number(itemField(st.it, name))
		if name[0] == 'x' {
			return v - ox
		}
		return v - oy
	case "width", "height":
	default:
		return expr.Undefined
	}
	ms.s.itemIssuef("the property %s of an item of a guide", name)
	return expr.Undefined
}

// marks returns the marks of a group item, or nil for an item that holds
// no marks, or whose marks zuzu did not build yet.
func (st *itemState) marks() *markList {
	if st.items == nil && st.mark != nil && st.mark.guide && len(st.it.Items) > 0 {
		st.items = &markList{}
		for i := range st.it.Items {
			st.items.add(&markState{sm: &st.it.Items[i], s: st.mark.s, group: st, guide: true, frame: st.mark.frame})
		}
	}
	return st.items
}

// markState is a mark while zuzu builds it, which an expression of an
// encoding reads as item.mark.
type markState struct {
	sm *scene.Mark
	// name is the name that the spec gives a part of a guide, which the
	// scene does not keep.
	name string
	// s is the scope that holds the mark.
	s *scope
	// group is the group item that holds the mark, or nil for the mark of
	// the root group.
	group expr.Object
	// states holds the items of the mark, in the order of their data
	// before a sort, and in the order of the scene after it. items holds
	// them as an array of an expression, once an expression reads it.
	states []*itemState
	items  []any
	// guide is true for a mark of a guide, whose items take the place and
	// the text of the scene. frame is the frame of Vega of its parts, or
	// nil where zuzu does not know it. sets holds the encodings that the
	// spec gives a part of a guide.
	guide bool
	frame guideFrame
	sets  []encodeSet
	// root is true for the mark of the root group.
	root bool
}

// Get returns a property of the mark: its group, its items, its kind, its
// name or its role.
func (ms *markState) Get(name string) any {
	switch name {
	case "group":
		if ms.group == nil {
			return expr.Undefined
		}
		return ms.group
	case "items":
		if ms.items == nil {
			states := ms.itemStates()
			ms.items = make([]any, len(states))
			for i, st := range states {
				ms.items[i] = st
			}
		}
		return ms.items
	case "marktype":
		return ms.sm.Type
	case "role":
		return ms.sm.Role
	case "name":
		switch {
		case ms.name != "":
			return ms.name
		case ms.sm.Name != "":
			return ms.sm.Name
		}
		return expr.Undefined
	}
	ms.s.itemIssuef("the property %s of item.mark", name)
	return expr.Undefined
}

// itemStates returns the items of the mark. A mark of a guide makes them
// from the items of its scene, when first asked.
func (ms *markState) itemStates() []*itemState {
	if ms.states == nil && ms.guide && len(ms.sm.Items) > 0 {
		ms.states = make([]*itemState, len(ms.sm.Items))
		for i := range ms.sm.Items {
			ms.states[i] = &itemState{it: &ms.sm.Items[i], mark: ms}
		}
	}
	return ms.states
}

// setStates sets the items of the mark, after a transform or a sort.
func (ms *markState) setStates(states []*itemState) {
	ms.states, ms.items = states, nil
}

// markList is the marks of a group item, in the order that zuzu builds
// them, which an expression reads as item.items.
type markList struct {
	marks []*markState
	// view holds the marks as an array of an expression, as far as an
	// expression read them.
	view []any
}

// add adds a mark to the list.
func (l *markList) add(ms *markState) {
	l.marks = append(l.marks, ms)
}

// truncate drops the marks after the first n, such as a mark that goes in
// the report.
func (l *markList) truncate(n int) {
	if n < len(l.marks) {
		l.marks, l.view = l.marks[:n], nil
	}
}

// value returns the marks as an array of an expression. The array grows as
// zuzu builds more marks, and the marks that it held stay.
func (l *markList) value() []any {
	for i := len(l.view); i < len(l.marks); i++ {
		l.view = append(l.view, l.marks[i])
	}
	return l.view[:len(l.marks):len(l.marks)]
}

// groupValue is the item of the group that holds a mark, which an
// expression reads as item.mark.group. The root group is the group of a
// mark at the top of the spec.
type groupValue struct {
	g *scope
}

// group returns the group item of the scope, as an expression reads it.
func (s *scope) group() *groupValue {
	if s.groupItem == nil {
		s.groupItem = &groupValue{g: s}
	}
	return s.groupItem
}

// Get returns a property of the group item: its place and size, its datum,
// its mark, its marks, or a property that its encodings set, such as fill.
// A property that the group does not have is undefined, as in Vega.
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
		if g.mark == nil {
			return expr.Undefined
		}
		return g.mark
	case "items":
		return g.marks.value()
	case "bounds":
		g.boundsIssue()
		return expr.Undefined
	}
	if x, ok := g.values[name]; ok {
		return x
	}
	return expr.Undefined
}

// boundsIssue reports that an encoding reads the bounds of an item.
func (s *scope) boundsIssue() {
	s.itemIssuef("item.bounds, which vega.js gives only from an earlier pulse, after it draws the item")
}

// itemIssuef reports a part of item that zuzu does not give, once for each
// encode and part. The strict mode keeps the error for the expression that
// reads it.
func (s *scope) itemIssuef(format string, args ...any) {
	if err := s.issueOncef(s.rt.encodePath, format, args...); err != nil && s.rt.err == nil {
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
