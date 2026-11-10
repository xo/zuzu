// Package vegalite holds the Go types of a Vega-Lite spec and the compiler
// that turns it into a Vega spec (D6, D21). zuzu follows the documented
// behavior of Vega-Lite: the names of the fields that it derives, the
// defaults of its scales and its guides, and the marks it draws. It is not
// a port of vega-lite.js.
package vegalite

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"maps"

	"github.com/xo/zuzu/vega"
)

// Extra holds the properties of an object that the types do not know.
type Extra = map[string]jsontext.Value

// Spec is a Vega-Lite spec: a single view, a layer, a facet, a repeat or a
// concatenation. The fields of each kind share one type, since Vega-Lite
// tells them apart by the fields that a spec has.
type Spec struct {
	Schema      string         `json:"$schema,omitzero"`
	Name        string         `json:"name,omitzero"`
	Description *string        `json:"description,omitzero"`
	Title       *vega.Title    `json:"title,omitzero"`
	Data        *Data          `json:"data,omitzero"`
	Datasets    Extra          `json:"datasets,omitzero"`
	Transform   []Transform    `json:"transform,omitzero"`
	Mark        *Mark          `json:"mark,omitzero"`
	Encoding    Encoding       `json:"encoding,omitzero"`
	Projection  jsontext.Value `json:"projection,omitzero"`
	Params      jsontext.Value `json:"params,omitzero"`
	Width       vega.Value     `json:"width,omitzero"`
	Height      vega.Value     `json:"height,omitzero"`
	View        jsontext.Value `json:"view,omitzero"`

	Layer   []Spec `json:"layer,omitzero"`
	HConcat []Spec `json:"hconcat,omitzero"`
	VConcat []Spec `json:"vconcat,omitzero"`
	Concat  []Spec `json:"concat,omitzero"`
	Columns *int   `json:"columns,omitzero"`
	// Facet is a field definition, or an object with row and column.
	Facet  jsontext.Value `json:"facet,omitzero"`
	Repeat jsontext.Value `json:"repeat,omitzero"`
	// Spec is the spec of each cell of a facet or a repeat.
	Spec    *Spec          `json:"spec,omitzero"`
	Resolve jsontext.Value `json:"resolve,omitzero"`
	Spacing vega.Value     `json:"spacing,omitzero"`
	Align   vega.Value     `json:"align,omitzero"`
	Bounds  vega.Value     `json:"bounds,omitzero"`
	Center  vega.Value     `json:"center,omitzero"`

	Autosize   vega.Value     `json:"autosize,omitzero"`
	Padding    vega.Value     `json:"padding,omitzero"`
	Background vega.Value     `json:"background,omitzero"`
	Config     jsontext.Value `json:"config,omitzero"`

	Extra Extra `json:",embed"`

	// expanded says that the compiler made the layer of the spec from a
	// composite mark or an overlay.
	expanded bool
	// stackAs is the mark whose rules of stacking a layer follows: the
	// area that a line or points overlay.
	stackAs string
}

// Data is the data of a view: inline values, a url, a named dataset, or a
// generator such as a sequence.
type Data struct {
	Values    vega.Value     `json:"values,omitzero"`
	URL       string         `json:"url,omitzero"`
	Name      string         `json:"name,omitzero"`
	Format    *Format        `json:"format,omitzero"`
	Sequence  jsontext.Value `json:"sequence,omitzero"`
	Graticule jsontext.Value `json:"graticule,omitzero"`
	Sphere    jsontext.Value `json:"sphere,omitzero"`

	Extra Extra `json:",embed"`
}

// Format says how to read data.
type Format struct {
	Type      string     `json:"type,omitzero"`
	Parse     vega.Value `json:"parse,omitzero"`
	Property  string     `json:"property,omitzero"`
	Feature   string     `json:"feature,omitzero"`
	Mesh      string     `json:"mesh,omitzero"`
	Delimiter string     `json:"delimiter,omitzero"`

	Extra Extra `json:",embed"`
}

// Transform is a transform of Vega-Lite. Its kind is the name of the
// field that names it, such as filter, calculate or aggregate, and the
// compiler reads the rest.
type Transform struct {
	Fields Extra `json:",embed"`
}

// Has reports whether the transform has a field.
func (t Transform) Has(name string) bool {
	_, ok := t.Fields[name]
	return ok
}

// Decode reads a field of the transform into a Go value.
func (t Transform) Decode(name string, out any) error {
	v, ok := t.Fields[name]
	if !ok {
		return nil
	}
	return json.Unmarshal(v, out)
}

// Mark is the mark of a view: its type, and its properties, such as color
// or interpolate. The spec gives a string or an object.
type Mark struct {
	Type  string
	Props Extra
	// object says that the spec gave the mark as an object.
	object bool
}

// UnmarshalJSONFrom reads a mark type or a mark definition.
func (m *Mark) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if dec.PeekKind() == '"' {
		var t string
		if err := json.UnmarshalDecode(dec, &t); err != nil {
			return err
		}
		*m = Mark{Type: t}
		return nil
	}
	var props Extra
	if err := json.UnmarshalDecode(dec, &props); err != nil {
		return err
	}
	var t string
	if raw, ok := props["type"]; ok {
		if err := json.Unmarshal(raw, &t); err != nil {
			return fmt.Errorf("reading the type of a mark: %w", err)
		}
		delete(props, "type")
	}
	*m = Mark{Type: t, Props: props, object: true}
	return nil
}

// MarshalJSONTo writes a mark with no properties as a string, and any other
// as an object.
func (m Mark) MarshalJSONTo(enc *jsontext.Encoder) error {
	if len(m.Props) == 0 && !m.object {
		return json.MarshalEncode(enc, m.Type)
	}
	out := Extra{}
	maps.Copy(out, m.Props)
	t, err := json.Marshal(m.Type)
	if err != nil {
		return err
	}
	out["type"] = t
	return json.MarshalEncode(enc, out, json.Deterministic(true))
}

// Encoding maps each channel, such as x or color, to its definitions.
type Encoding map[string]Channel

// Channel is the definition of a channel. The channels tooltip and detail
// take a list, which Defs holds, and every other channel one definition.
type Channel struct {
	Defs  []FieldDef
	Array bool
}

// UnmarshalJSONFrom reads one definition or a list.
func (c *Channel) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	switch dec.PeekKind() {
	case '[':
		var defs []FieldDef
		if err := json.UnmarshalDecode(dec, &defs); err != nil {
			return err
		}
		*c = Channel{Defs: defs, Array: true}
		return nil
	case 'n':
		_, err := dec.ReadToken()
		*c = Channel{}
		return err
	}
	var d FieldDef
	if err := json.UnmarshalDecode(dec, &d); err != nil {
		return err
	}
	*c = Channel{Defs: []FieldDef{d}}
	return nil
}

// MarshalJSONTo writes the form that the spec gave.
func (c Channel) MarshalJSONTo(enc *jsontext.Encoder) error {
	if c.Array || len(c.Defs) != 1 {
		return json.MarshalEncode(enc, c.Defs)
	}
	return json.MarshalEncode(enc, c.Defs[0])
}

// Def returns the one definition of a channel, or nil.
func (c Channel) Def() *FieldDef {
	if len(c.Defs) == 0 {
		return nil
	}
	return &c.Defs[0]
}

// FieldDef defines a channel: a field of the data, a value, or a datum,
// with how to aggregate, bin and draw it.
type FieldDef struct {
	// Field is a field name, or an object such as {"repeat": "row"}.
	Field        vega.Value     `json:"field,omitzero"`
	Type         string         `json:"type,omitzero"`
	Aggregate    vega.Value     `json:"aggregate,omitzero"`
	Bin          vega.Value     `json:"bin,omitzero"`
	TimeUnit     vega.Value     `json:"timeUnit,omitzero"`
	Title        vega.Value     `json:"title,omitzero"`
	Scale        vega.Value     `json:"scale,omitzero"`
	Axis         vega.Value     `json:"axis,omitzero"`
	Legend       vega.Value     `json:"legend,omitzero"`
	Header       vega.Value     `json:"header,omitzero"`
	Sort         vega.Value     `json:"sort,omitzero"`
	Stack        vega.Value     `json:"stack,omitzero"`
	Value        vega.Value     `json:"value,omitzero"`
	Datum        vega.Value     `json:"datum,omitzero"`
	Condition    jsontext.Value `json:"condition,omitzero"`
	Impute       jsontext.Value `json:"impute,omitzero"`
	Format       vega.Value     `json:"format,omitzero"`
	FormatType   vega.Value     `json:"formatType,omitzero"`
	BandPosition vega.Value     `json:"bandPosition,omitzero"`
	Band         vega.Value     `json:"band,omitzero"`

	Extra Extra `json:",embed"`
}
