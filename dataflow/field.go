package dataflow

import (
	"strconv"
	"strings"

	"github.com/xo/zuzu/expr"
)

// Accessor reads a value from a row.
type Accessor struct {
	// Name is the name of the field, which names the output of a transform,
	// such as sum_price.
	Name string
	get  func(Row) any
}

// Get reads the value from a row.
func (a *Accessor) Get(r Row) any {
	return a.get(r)
}

// FieldAccessor returns an accessor of a field name in the syntax of Vega:
// a dot reads a nested field, as in a.b, brackets read an index or a
// quoted name, as in a[0] and a["b c"], and a backslash makes the next
// character plain, as in a\.b.
func FieldAccessor(name string) *Accessor {
	path := SplitField(name)
	if len(path) == 1 {
		// A path of one field names the field without its escapes, as
		// the field accessor of Vega does.
		f := path[0]
		return &Accessor{Name: f, get: func(r Row) any {
			if v, ok := r[f]; ok {
				return v
			}
			return expr.Undefined
		}}
	}
	return &Accessor{Name: name, get: func(r Row) any {
		var v any = r
		for _, f := range path {
			v = member(v, f)
		}
		return v
	}}
}

// member reads a property of a value, as obj[prop] does.
func member(v any, prop string) any {
	switch x := v.(type) {
	case map[string]any:
		if out, ok := x[prop]; ok {
			return out
		}
	case []any:
		if i, err := strconv.Atoi(prop); err == nil && i >= 0 && i < len(x) {
			return x[i]
		}
		if prop == "length" {
			return float64(len(x))
		}
	}
	return expr.Undefined
}

// SplitField splits a field name in the syntax of Vega into the names of
// its path.
func SplitField(name string) []string {
	var (
		path  []string
		b     strings.Builder
		quote byte
		inBr  bool
	)
	flush := func() {
		if b.Len() > 0 || quote != 0 {
			path = append(path, b.String())
		}
		b.Reset()
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c == '\\' && i+1 < len(name):
			i++
			b.WriteByte(name[i])
		case quote != 0:
			if c == quote {
				quote = 0
				path = append(path, b.String())
				b.Reset()
				// The closing bracket follows.
				continue
			}
			b.WriteByte(c)
		case inBr && (c == '\'' || c == '"') && b.Len() == 0:
			quote = c
		case c == '[':
			flush()
			inBr = true
		case c == ']' && inBr:
			if b.Len() > 0 {
				path = append(path, b.String())
				b.Reset()
			}
			inBr = false
		case c == '.' && !inBr:
			flush()
		default:
			b.WriteByte(c)
		}
	}
	flush()
	if len(path) == 0 {
		path = []string{""}
	}
	return path
}
