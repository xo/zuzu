package expr

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"time"
	"unicode/utf16"

	"github.com/xo/zuzu/format"
)

// Func is a function that an expression can call. It takes the scope of the
// call, so that a function of the runtime, such as scale, reaches the view.
type Func func(s *Scope, args []any) (any, error)

// Signals gives the value of a signal by name.
type Signals interface {
	Signal(name string) (any, bool)
}

// Scope is what an expression reads when it runs.
type Scope struct {
	// Datum is the row that the expression reads as datum.
	Datum any
	// Parent is the datum of the enclosing group, which it reads as parent.
	Parent any
	// Item is the item of the scene that an encoding fills, which it reads
	// as item. Nil reads as undefined, as item does outside an encoding.
	Item Object
	// Signals holds the signals of the view. It can be nil.
	Signals Signals
	// Functions holds functions beyond the built in ones, such as the
	// functions that read the data and the scales of a view, and those that a
	// caller adds. A function here takes the place of a built in one of the
	// same name.
	Functions map[string]Func
	// Location is the time zone of the functions of local time (D27). Nil is
	// time.Local.
	Location *time.Location
	// Rand is the source of random numbers (D28). Nil is a source seeded
	// from the system.
	Rand *rand.Rand
	// Locale holds the names and symbols of the formats (D27). Nil is US
	// English.
	Locale *format.Locale
	// Now is the time of the render, which now() gives (D38). The zero
	// time gives 0, so that the same spec gives the same bytes.
	Now time.Time
}

// locale returns the locale of the scope.
func (s *Scope) locale() *format.Locale {
	if s == nil || s.Locale == nil {
		return &format.EnUS
	}
	return s.Locale
}

// location returns the time zone of the scope.
func (s *Scope) location() *time.Location {
	if s == nil || s.Location == nil {
		return time.Local //nolint:gosmopolitan // The zone of the system is the default (D27).
	}
	return s.Location
}

// random returns a random number from 0 to 1.
func (s *Scope) random() float64 {
	if s == nil || s.Rand == nil {
		return rand.Float64() //nolint:gosec // A chart asks for random data, not a secret (D28).
	}
	return s.Rand.Float64()
}

// now returns the time of the render in milliseconds since 1970, or 0
// when the scope has no time.
func (s *Scope) now() float64 {
	if s == nil || s.Now.IsZero() {
		return 0
	}
	return float64(s.Now.UnixMilli())
}

// Program is a parsed expression.
type Program struct {
	src  string
	root node
}

// CompileOption is an option of Compile.
type CompileOption func(*compileOptions)

// compileOptions are the options of Compile.
type compileOptions struct {
	known map[string]bool
}

// Known names functions that the scope of the evaluation gives, such as the
// functions that a caller adds with WithFunction, so that an expression can
// call them. Compile reports a call of any other function that is neither
// built in nor given by the runtime.
func Known(names ...string) CompileOption {
	return func(o *compileOptions) {
		if o.known == nil {
			o.known = map[string]bool{}
		}
		for _, n := range names {
			o.known[n] = true
		}
	}
}

// Compile parses an expression.
func Compile(src string, opts ...CompileOption) (*Program, error) {
	var o compileOptions
	for _, opt := range opts {
		opt(&o)
	}
	n, err := parse(src)
	if err != nil {
		return nil, err
	}
	if err := check(n, src, o.known); err != nil {
		return nil, err
	}
	return &Program{src: src, root: n}, nil
}

// String returns the text of the expression.
func (p *Program) String() string {
	return p.src
}

// Calls reports whether the expression calls a function of the name, such
// as now, in any branch.
func (p *Program) Calls(name string) bool {
	return calls(p.root, name)
}

// calls reports whether a node calls a function of the name.
func calls(n node, name string) bool {
	var kids []node
	switch x := n.(type) {
	case call:
		if x.name == name {
			return true
		}
		kids = x.args
	case member:
		kids = []node{x.obj, x.prop}
	case unary:
		kids = []node{x.x}
	case binary:
		kids = []node{x.x, x.y}
	case cond:
		kids = []node{x.test, x.yes, x.no}
	case array:
		kids = x.elems
	case object:
		kids = x.vals
	}
	return slices.ContainsFunc(kids, func(k node) bool { return calls(k, name) })
}

// Eval evaluates the expression in a scope.
func (p *Program) Eval(s *Scope) (any, error) {
	v, err := eval(p.root, s)
	if err != nil {
		return nil, fmt.Errorf("evaluating %q: %w", p.src, err)
	}
	return v, nil
}

// runtimeFuncs are the functions that the runtime gives in the scope rather
// than this package.
var runtimeFuncs = map[string]bool{
	"data": true, "indata": true, "scale": true, "invert": true, "bandwidth": true,
	"bandspace": true, "copy": true, "domain": true, "range": true, "gradient": true,
	"geoArea": true, "geoBounds": true, "geoCentroid": true, "geoShape": true,
	"pluck": true, "vlSelectionTest": true, "vlSelectionResolve": true,
	"vlSelectionIdTest": true, "treePath": true, "treeAncestors": true, "containerSize": true, "screen": true,
	"windowSize": true, "inScope": true, "info": true, "warn": true, "debug": true,
}

// check reports a call of a function that is neither built in, given by
// the runtime, nor known, as Vega reports it when it parses.
func check(n node, src string, known map[string]bool) error {
	switch x := n.(type) {
	case call:
		if _, ok := builtins[x.name]; !ok && !runtimeFuncs[x.name] && !known[x.name] && x.name != "if" {
			return fmt.Errorf("parsing %q: %s: %w", src, x.name, ErrUnknownFunction)
		}
		for _, a := range x.args {
			if err := check(a, src, known); err != nil {
				return err
			}
		}
	case member:
		if err := check(x.obj, src, known); err != nil {
			return err
		}
		return check(x.prop, src, known)
	case unary:
		return check(x.x, src, known)
	case binary:
		if err := check(x.x, src, known); err != nil {
			return err
		}
		return check(x.y, src, known)
	case cond:
		for _, c := range []node{x.test, x.yes, x.no} {
			if err := check(c, src, known); err != nil {
				return err
			}
		}
	case array:
		for _, e := range x.elems {
			if err := check(e, src, known); err != nil {
				return err
			}
		}
	case object:
		for _, v := range x.vals {
			if err := check(v, src, known); err != nil {
				return err
			}
		}
	}
	return nil
}

// constants are the constants that an expression names.
var constants = map[string]float64{
	"PI": math.Pi, "E": math.E, "LN2": math.Ln2, "LN10": math.Ln10,
	"LOG2E": math.Log2E, "LOG10E": math.Log10E, "SQRT1_2": math.Sqrt2 / 2, "SQRT2": math.Sqrt2,
	"MIN_VALUE": 5e-324, "MAX_VALUE": math.MaxFloat64,
}

// eval evaluates a node.
func eval(n node, s *Scope) (any, error) {
	switch x := n.(type) {
	case literal:
		return x.v, nil
	case ident:
		return lookup(x.name, s)
	case member:
		obj, err := eval(x.obj, s)
		if err != nil {
			return nil, err
		}
		prop, err := eval(x.prop, s)
		if err != nil {
			return nil, err
		}
		return get(obj, prop), nil
	case call:
		return callFunc(x, s)
	case unary:
		v, err := eval(x.x, s)
		if err != nil {
			return nil, err
		}
		switch x.op {
		case "!":
			return !Truthy(v), nil
		case "-":
			return -Number(primitive(v, "number")), nil
		case "+":
			return Number(primitive(v, "number")), nil
		}
		return float64(^int32Of(v)), nil
	case binary:
		return evalBinary(x, s)
	case cond:
		t, err := eval(x.test, s)
		if err != nil {
			return nil, err
		}
		if Truthy(t) {
			return eval(x.yes, s)
		}
		return eval(x.no, s)
	case array:
		out := make([]any, len(x.elems))
		for i, e := range x.elems {
			v, err := eval(e, s)
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	case object:
		out := make(map[string]any, len(x.keys))
		for i, k := range x.keys {
			v, err := eval(x.vals[i], s)
			if err != nil {
				return nil, err
			}
			out[k] = v
		}
		return out, nil
	}
	return nil, fmt.Errorf("evaluating a node of type %T: %w", n, ErrSyntax)
}

// Object is a value whose properties an expression reads by name, as
// obj.prop reads them, such as an item of the scene that the runtime gives
// as item. Get returns Undefined for a property that the value does not
// have.
type Object interface {
	Get(name string) any
}

// ErrUnknownName is the error of a name that is neither datum, parent, a
// constant nor a signal.
const ErrUnknownName Error = "unknown name"

// lookup returns the value of a name.
func lookup(name string, s *Scope) (any, error) {
	switch name {
	case "datum":
		if s == nil {
			return Undefined, nil
		}
		return s.Datum, nil
	case "parent":
		if s == nil || s.Parent == nil {
			return Undefined, nil
		}
		return s.Parent, nil
	case "item":
		if s == nil || s.Item == nil {
			return Undefined, nil
		}
		return s.Item, nil
	case "event":
		return Undefined, nil
	}
	if v, ok := constants[name]; ok {
		return v, nil
	}
	if s != nil && s.Signals != nil {
		if v, ok := s.Signals.Signal(name); ok {
			return v, nil
		}
	}
	return nil, fmt.Errorf("reading %s: %w", name, ErrUnknownName)
}

// lengthProp is the property that holds the length of a string or an
// array.
const lengthProp = "length"

// get reads a property of a value, as obj[prop] does in JavaScript. A
// property of null or of undefined is undefined, where JavaScript throws
// an error.
func get(obj, prop any) any {
	switch o := obj.(type) {
	case map[string]any:
		if v, ok := o[String(prop)]; ok {
			return v
		}
	case []any:
		if String(prop) == lengthProp {
			return float64(len(o))
		}
		if i, ok := index(prop); ok && i < len(o) {
			return o[i]
		}
	case *Color:
		return o.get(String(prop))
	case Object:
		return o.Get(String(prop))
	case string:
		if String(prop) == lengthProp {
			return float64(len(utf16.Encode([]rune(o))))
		}
		if i, ok := index(prop); ok {
			u := utf16.Encode([]rune(o))
			if i < len(u) {
				return string(utf16.Decode(u[i : i+1]))
			}
		}
	}
	return Undefined
}

// index reads a property as an index of an array.
func index(prop any) (int, bool) {
	f := Number(prop)
	if f < 0 || f != math.Trunc(f) || math.IsNaN(f) {
		return 0, false
	}
	if s, ok := prop.(string); ok && s != strconv.Itoa(int(f)) {
		return 0, false
	}
	return int(f), true
}

// evalBinary evaluates a binary operator, with && and || short circuited.
func evalBinary(x binary, s *Scope) (any, error) {
	a, err := eval(x.x, s)
	if err != nil {
		return nil, err
	}
	switch x.op {
	case "&&":
		if !Truthy(a) {
			return a, nil
		}
		return eval(x.y, s)
	case "||":
		if Truthy(a) {
			return a, nil
		}
		return eval(x.y, s)
	}
	b, err := eval(x.y, s)
	if err != nil {
		return nil, err
	}
	switch x.op {
	case "+":
		return add(a, b), nil
	case "-":
		return Number(primitive(a, "number")) - Number(primitive(b, "number")), nil
	case "*":
		return Number(primitive(a, "number")) * Number(primitive(b, "number")), nil
	case "/":
		return Number(primitive(a, "number")) / Number(primitive(b, "number")), nil
	case "%":
		return math.Mod(Number(primitive(a, "number")), Number(primitive(b, "number"))), nil
	case "==":
		return equal(a, b), nil
	case "!=":
		return !equal(a, b), nil
	case "===":
		return strictEqual(a, b), nil
	case "!==":
		return !strictEqual(a, b), nil
	case "<", ">", "<=", ">=":
		c, ok := compare(a, b)
		if !ok {
			return false, nil
		}
		switch x.op {
		case "<":
			return c < 0, nil
		case ">":
			return c > 0, nil
		case "<=":
			return c <= 0, nil
		}
		return c >= 0, nil
	case "&":
		return float64(int32Of(a) & int32Of(b)), nil
	case "|":
		return float64(int32Of(a) | int32Of(b)), nil
	case "^":
		return float64(int32Of(a) ^ int32Of(b)), nil
	case "<<":
		return float64(int32Of(a) << (uint32Of(b) & 31)), nil
	case ">>":
		return float64(int32Of(a) >> (uint32Of(b) & 31)), nil
	case ">>>":
		return float64(uint32Of(a) >> (uint32Of(b) & 31)), nil
	case "in":
		if m, ok := b.(map[string]any); ok {
			_, has := m[String(a)]
			return has, nil
		}
		return false, nil
	}
	return nil, fmt.Errorf("evaluating the operator %s: %w", x.op, ErrSyntax)
}

// callFunc calls a function. if evaluates only the branch that it takes.
func callFunc(c call, s *Scope) (any, error) {
	if c.name == "if" {
		if len(c.args) != 3 {
			return nil, fmt.Errorf("calling if with %d arguments: %w", len(c.args), ErrSyntax)
		}
		t, err := eval(c.args[0], s)
		if err != nil {
			return nil, err
		}
		if Truthy(t) {
			return eval(c.args[1], s)
		}
		return eval(c.args[2], s)
	}
	args := make([]any, len(c.args))
	for i, a := range c.args {
		v, err := eval(a, s)
		if err != nil {
			return nil, err
		}
		args[i] = v
	}
	if s != nil {
		if f, ok := s.Functions[c.name]; ok {
			return f(s, args)
		}
	}
	if f, ok := builtins[c.name]; ok {
		return f(s, args)
	}
	return nil, fmt.Errorf("calling %s: %w", c.name, ErrUnknownFunction)
}
