package vega

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
)

// RefKind says which form a field with more than one form took.
type RefKind uint8

// The forms of a field.
const (
	// Absent is a field that the spec does not give.
	Absent RefKind = iota
	// Literal is a value that the spec gives as it is.
	Literal
	// SignalRef is a reference to a signal, {"signal": "expr"}.
	SignalRef
)

// NumberValue is a number or a reference to a signal.
type NumberValue struct {
	Kind   RefKind
	Number float64
	Signal string
}

// Num returns a NumberValue that holds a number.
func Num(f float64) NumberValue {
	return NumberValue{Kind: Literal, Number: f}
}

// UnmarshalJSONFrom reads a number or {"signal": "expr"}.
func (v *NumberValue) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	switch dec.PeekKind() {
	case '{':
		var s signalObject
		if err := json.UnmarshalDecode(dec, &s); err != nil {
			return err
		}
		*v = NumberValue{Kind: SignalRef, Signal: s.Signal}
	case 'n':
		_, err := dec.ReadToken()
		*v = NumberValue{}
		return err
	default:
		var f float64
		if err := json.UnmarshalDecode(dec, &f); err != nil {
			return err
		}
		*v = NumberValue{Kind: Literal, Number: f}
	}
	return nil
}

// MarshalJSONTo writes the number or the signal.
func (v NumberValue) MarshalJSONTo(enc *jsontext.Encoder) error {
	if v.Kind == SignalRef {
		return json.MarshalEncode(enc, signalObject{Signal: v.Signal})
	}
	return json.MarshalEncode(enc, v.Number)
}

// IsZero reports whether the field is absent.
func (v NumberValue) IsZero() bool {
	return v.Kind == Absent
}

// signalObject is the form {"signal": "expr"}.
type signalObject struct {
	Signal string `json:"signal"`
}

// Value is a property that takes a literal of any type or a reference to a
// signal. The literal stays JSON, and the runtime reads it as the type that
// the property needs.
type Value struct {
	Signal string
	JSON   jsontext.Value
}

// Lit returns a Value that holds a literal.
func Lit(v any) Value {
	b, err := json.Marshal(v, json.Deterministic(true))
	if err != nil {
		panic(fmt.Sprintf("vega.Lit(%#v): %v", v, err))
	}
	return Value{JSON: b}
}

// UnmarshalJSONFrom reads {"signal": "expr"} or any other value.
func (v *Value) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	raw, err := readValue(dec)
	if err != nil {
		return err
	}
	*v = valueOf(raw)
	return nil
}

// valueOf reads a value that is a signal reference or a literal.
func valueOf(raw jsontext.Value) Value {
	if raw.Kind() == '{' {
		var m map[string]jsontext.Value
		if err := json.Unmarshal(raw, &m); err == nil && len(m) == 1 {
			if s, ok := m["signal"]; ok && s.Kind() == '"' {
				var expr string
				if json.Unmarshal(s, &expr) == nil {
					return Value{Signal: expr}
				}
			}
		}
	}
	return Value{JSON: raw}
}

// MarshalJSONTo writes the literal or the signal.
func (v Value) MarshalJSONTo(enc *jsontext.Encoder) error {
	if v.Signal != "" {
		return json.MarshalEncode(enc, signalObject{Signal: v.Signal})
	}
	if v.JSON == nil {
		return enc.WriteToken(jsontext.Null)
	}
	return enc.WriteValue(v.JSON)
}

// IsZero reports whether the field is absent.
func (v Value) IsZero() bool {
	return v.Signal == "" && v.JSON == nil
}

// Decode reads the literal into a Go value, as json.Unmarshal does.
func (v Value) Decode(out any) error {
	if v.JSON == nil {
		return nil
	}
	return json.Unmarshal(v.JSON, out)
}

// Strings is a string or an array of strings, as a style or a group by.
// It remembers which form the spec gave, so that it writes the same form
// back.
type Strings struct {
	List  []string
	Array bool
}

// List returns Strings that holds a list, written as an array.
func List(s ...string) Strings {
	return Strings{List: s, Array: true}
}

// UnmarshalJSONFrom reads a string or an array of strings.
func (s *Strings) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if dec.PeekKind() == '"' {
		var one string
		if err := json.UnmarshalDecode(dec, &one); err != nil {
			return err
		}
		*s = Strings{List: []string{one}}
		return nil
	}
	var many []string
	if err := json.UnmarshalDecode(dec, &many); err != nil {
		return err
	}
	*s = Strings{List: many, Array: true}
	return nil
}

// MarshalJSONTo writes the form that the spec gave.
func (s Strings) MarshalJSONTo(enc *jsontext.Encoder) error {
	if len(s.List) == 1 && !s.Array {
		return json.MarshalEncode(enc, s.List[0])
	}
	return json.MarshalEncode(enc, s.List)
}

// IsZero reports whether the field is absent.
func (s Strings) IsZero() bool {
	return s.List == nil
}

// Expr is the text of an expression. The spec gives a string, and a
// number or a boolean is read as its text.
type Expr string

// UnmarshalJSONFrom reads a string, or any other value as its text.
func (e *Expr) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if dec.PeekKind() == '"' {
		var s string
		if err := json.UnmarshalDecode(dec, &s); err != nil {
			return err
		}
		*e = Expr(s)
		return nil
	}
	v, err := dec.ReadValue()
	if err != nil {
		return err
	}
	*e = Expr(v)
	return nil
}

// Field is a reference to a field of a datum: a name, such as "price" or
// "a.b", or an object that names the field of a datum, a group or a
// parent, or a signal.
type Field struct {
	// Name is the name of a field of the datum, or of the object below.
	Name string
	// Signal is an expression that gives the name.
	Signal string
	// Datum, Group and Parent name a field of the datum, of the group or of
	// the datum of the parent. Level is the number of groups up, for
	// Group and Parent.
	Datum  *Field
	Group  *Field
	Parent *Field
	Level  int
}

// fieldObject is the object form of a Field.
type fieldObject struct {
	Signal string `json:"signal,omitzero"`
	Datum  *Field `json:"datum,omitzero"`
	Group  *Field `json:"group,omitzero"`
	Parent *Field `json:"parent,omitzero"`
	Level  int    `json:"level,omitzero"`
}

// UnmarshalJSONFrom reads a field name or a field object.
func (f *Field) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if dec.PeekKind() == '"' {
		var name string
		if err := json.UnmarshalDecode(dec, &name); err != nil {
			return err
		}
		*f = Field{Name: name}
		return nil
	}
	var o fieldObject
	if err := json.UnmarshalDecode(dec, &o); err != nil {
		return err
	}
	*f = Field{Signal: o.Signal, Datum: o.Datum, Group: o.Group, Parent: o.Parent, Level: o.Level}
	return nil
}

// MarshalJSONTo writes a name as a string, and any other field as an
// object.
func (f Field) MarshalJSONTo(enc *jsontext.Encoder) error {
	if f.Name != "" {
		return json.MarshalEncode(enc, f.Name)
	}
	return json.MarshalEncode(enc, fieldObject{Signal: f.Signal, Datum: f.Datum, Group: f.Group, Parent: f.Parent, Level: f.Level})
}

// IsZero reports whether the field is absent.
func (f Field) IsZero() bool {
	return f.Name == "" && f.Signal == "" && f.Datum == nil && f.Group == nil && f.Parent == nil
}

// BoolValue is a boolean or a reference to a signal.
type BoolValue struct {
	Kind   RefKind
	Bool   bool
	Signal string
}

// UnmarshalJSONFrom reads a boolean or {"signal": "expr"}.
func (v *BoolValue) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if dec.PeekKind() == '{' {
		var s signalObject
		if err := json.UnmarshalDecode(dec, &s); err != nil {
			return err
		}
		*v = BoolValue{Kind: SignalRef, Signal: s.Signal}
		return nil
	}
	var b bool
	if err := json.UnmarshalDecode(dec, &b); err != nil {
		return err
	}
	*v = BoolValue{Kind: Literal, Bool: b}
	return nil
}

// MarshalJSONTo writes the boolean or the signal.
func (v BoolValue) MarshalJSONTo(enc *jsontext.Encoder) error {
	if v.Kind == SignalRef {
		return json.MarshalEncode(enc, signalObject{Signal: v.Signal})
	}
	return json.MarshalEncode(enc, v.Bool)
}

// IsZero reports whether the field is absent.
func (v BoolValue) IsZero() bool {
	return v.Kind == Absent
}
