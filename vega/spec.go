package vega

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// Extra holds the properties of an object that the types do not know, so
// that the report can name them and a spec can be written back whole.
type Extra = map[string]jsontext.Value

// Scope holds what a spec and a group mark both hold: signals, data,
// scales, projections, axes, legends, a title and marks.
type Scope struct {
	Signals     []Signal     `json:"signals,omitzero"`
	Data        []Data       `json:"data,omitzero"`
	Scales      []Scale      `json:"scales,omitzero"`
	Projections []Projection `json:"projections,omitzero"`
	Axes        []Axis       `json:"axes,omitzero"`
	Legends     []Legend     `json:"legends,omitzero"`
	Title       *Title       `json:"title,omitzero"`
	Marks       []Mark       `json:"marks,omitzero"`
	Layout      Value        `json:"layout,omitzero"`
	Encode      Encode       `json:"encode,omitzero"`
}

// Spec is a Vega spec.
type Spec struct {
	Scope

	Schema      string      `json:"$schema,omitzero"`
	Description string      `json:"description,omitzero"`
	Width       NumberValue `json:"width,omitzero"`
	Height      NumberValue `json:"height,omitzero"`
	// Padding is a number, an object with top, bottom, left and right, or a
	// signal.
	Padding Value `json:"padding,omitzero"`
	// Autosize is a string, such as pad or fit, an object with a type, or a
	// signal.
	Autosize   Value   `json:"autosize,omitzero"`
	Background Value   `json:"background,omitzero"`
	Style      Strings `json:"style,omitzero"`
	Config     Config  `json:"config,omitzero"`

	Extra Extra `json:",embed"`
}

// Config is the config of a spec, as JSON. The theme makes one (D14).
type Config map[string]jsontext.Value

// Signal is a named value, which can depend on other signals, the data
// and the scales.
type Signal struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitzero"`
	Value       jsontext.Value `json:"value,omitzero"`
	Init        Expr           `json:"init,omitzero"`
	Update      Expr           `json:"update,omitzero"`
	React       *bool          `json:"react,omitzero"`
	// On and Bind make a signal change with events and with inputs. zuzu
	// draws a static chart and reads neither (D7).
	On   jsontext.Value `json:"on,omitzero"`
	Bind jsontext.Value `json:"bind,omitzero"`
	// Push says that the signal is one of an enclosing scope.
	Push string `json:"push,omitzero"`

	Extra Extra `json:",embed"`
}

// Data is a dataset: rows from values, a url or other datasets, and the
// transforms that run over them.
type Data struct {
	Name   string  `json:"name"`
	Source Strings `json:"source,omitzero"`
	// Values holds the rows, as an array, or as a string or an object that
	// Format reads. It can be a signal.
	Values Value `json:"values,omitzero"`
	// URL is the address of the data, which the loader reads (D25). It can
	// be a signal.
	URL       Value          `json:"url,omitzero"`
	Format    *Format        `json:"format,omitzero"`
	Transform []Transform    `json:"transform,omitzero"`
	On        jsontext.Value `json:"on,omitzero"`
	Async     *bool          `json:"async,omitzero"`

	Extra Extra `json:",embed"`
}

// Format says how to read the values or the file of a dataset.
type Format struct {
	// Type is json, csv, tsv, dsv, topojson or another format.
	Type string `json:"type,omitzero"`
	// Parse is "auto" or an object of field names and types, such as
	// {"date": "date", "price": "number"}. It can be a signal.
	Parse     Value    `json:"parse,omitzero"`
	Delimiter string   `json:"delimiter,omitzero"`
	Header    []string `json:"header,omitzero"`
	Property  string   `json:"property,omitzero"`
	Copy      *bool    `json:"copy,omitzero"`
	Feature   string   `json:"feature,omitzero"`
	Mesh      string   `json:"mesh,omitzero"`
	Filter    string   `json:"filter,omitzero"`

	Extra Extra `json:",embed"`
}

// Transform is a transform of a dataset or of a mark. Its parameters depend
// on its type, and the package dataflow reads them.
type Transform struct {
	Type   string `json:"type"`
	Params Extra  `json:",embed"`
}

// Param returns a parameter as a Value, and whether the transform has it.
func (t Transform) Param(name string) (Value, bool) {
	raw, ok := t.Params[name]
	if !ok {
		return Value{}, false
	}
	return valueOf(raw), true
}

// Scale maps values of the data to values of the chart.
type Scale struct {
	Name string `json:"name"`
	// Type is linear, band, ordinal, time and so on. The default is linear.
	Type         string      `json:"type,omitzero"`
	Domain       ScaleDomain `json:"domain,omitzero"`
	DomainMin    Value       `json:"domainMin,omitzero"`
	DomainMax    Value       `json:"domainMax,omitzero"`
	DomainMid    Value       `json:"domainMid,omitzero"`
	DomainRaw    Value       `json:"domainRaw,omitzero"`
	Range        ScaleRange  `json:"range,omitzero"`
	Reverse      BoolValue   `json:"reverse,omitzero"`
	Round        BoolValue   `json:"round,omitzero"`
	Clamp        BoolValue   `json:"clamp,omitzero"`
	Zero         BoolValue   `json:"zero,omitzero"`
	Nice         Value       `json:"nice,omitzero"`
	Padding      Value       `json:"padding,omitzero"`
	PaddingInner Value       `json:"paddingInner,omitzero"`
	PaddingOuter Value       `json:"paddingOuter,omitzero"`
	Align        Value       `json:"align,omitzero"`
	Exponent     Value       `json:"exponent,omitzero"`
	Base         Value       `json:"base,omitzero"`
	Constant     Value       `json:"constant,omitzero"`
	Interpolate  Value       `json:"interpolate,omitzero"`
	Bins         Value       `json:"bins,omitzero"`

	Extra Extra `json:",embed"`
}

// ScaleDomain is the domain of a scale: an array, whose elements can be
// signals, a signal, or a reference to the data.
type ScaleDomain struct {
	// Values holds an array that the spec gives.
	Values []Value
	// Signal is an expression that gives the domain.
	Signal string
	// Data is a reference to the data.
	Data *DataRef
}

// DataRef is a reference to one or more fields of one or more datasets.
type DataRef struct {
	Data   string `json:"data,omitzero"`
	Field  Field  `json:"field,omitzero"`
	Fields []any  `json:"fields,omitzero"`
	// Sort is true, or an object with a field, an op and an order.
	Sort jsontext.Value `json:"sort,omitzero"`

	Extra Extra `json:",embed"`
}

// UnmarshalJSONFrom reads an array, a signal or a data reference.
func (d *ScaleDomain) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	raw, err := readValue(dec)
	if err != nil {
		return err
	}
	return d.read(raw)
}

// MarshalJSONTo writes the domain in the form it was read.
func (d ScaleDomain) MarshalJSONTo(enc *jsontext.Encoder) error {
	switch {
	case d.Signal != "":
		return json.MarshalEncode(enc, signalObject{Signal: d.Signal})
	case d.Data != nil:
		return json.MarshalEncode(enc, d.Data)
	}
	return json.MarshalEncode(enc, d.Values)
}

// IsZero reports whether the domain is absent.
func (d ScaleDomain) IsZero() bool {
	return d.Values == nil && d.Signal == "" && d.Data == nil
}

// read reads the domain from JSON.
func (d *ScaleDomain) read(raw jsontext.Value) error {
	*d = ScaleDomain{}
	switch raw.Kind() {
	case '[':
		var vals []Value
		if err := json.Unmarshal(raw, &vals); err != nil {
			return err
		}
		d.Values = vals
		if d.Values == nil {
			d.Values = []Value{}
		}
		return nil
	case '{':
		if v := valueOf(raw); v.Signal != "" {
			d.Signal = v.Signal
			return nil
		}
		var ref DataRef
		if err := json.Unmarshal(raw, &ref); err != nil {
			return err
		}
		d.Data = &ref
		return nil
	}
	return &json.SemanticError{JSONKind: raw.Kind(), Err: ErrInvalidSpec}
}

// ScaleRange is the range of a scale: a name such as width or category, an
// array, a signal, a scheme, a step, or a reference to the data.
type ScaleRange struct {
	// Name is a name of a range, such as width, height, symbol, category,
	// diverging, ordinal, ramp or heatmap.
	Name string
	// Values holds an array that the spec gives.
	Values []Value
	// Signal is an expression that gives the range.
	Signal string
	// Scheme names a color scheme, with Count and Extent.
	Scheme *SchemeRef
	// Step is the size of a band, for a band or a point scale.
	Step Value
	// Data is a reference to the data.
	Data *DataRef
}

// SchemeRef names a color scheme of a range.
type SchemeRef struct {
	Scheme Value `json:"scheme"`
	Count  Value `json:"count,omitzero"`
	Extent Value `json:"extent,omitzero"`

	Extra Extra `json:",embed"`
}

// UnmarshalJSONFrom reads the forms of a range.
func (r *ScaleRange) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	raw, err := readValue(dec)
	if err != nil {
		return err
	}
	*r = ScaleRange{}
	switch raw.Kind() {
	case '"':
		return json.Unmarshal(raw, &r.Name)
	case '[':
		if err := json.Unmarshal(raw, &r.Values); err != nil {
			return err
		}
		if r.Values == nil {
			r.Values = []Value{}
		}
		return nil
	case '{':
		var m map[string]jsontext.Value
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		switch {
		case m["scheme"] != nil:
			r.Scheme = &SchemeRef{}
			return json.Unmarshal(raw, r.Scheme)
		case m["step"] != nil:
			r.Step = valueOf(m["step"])
			return nil
		case m["data"] != nil:
			r.Data = &DataRef{}
			return json.Unmarshal(raw, r.Data)
		}
		if v := valueOf(raw); v.Signal != "" {
			r.Signal = v.Signal
			return nil
		}
	}
	return &json.SemanticError{JSONKind: raw.Kind(), Err: ErrInvalidSpec}
}

// MarshalJSONTo writes the range in the form it was read.
func (r ScaleRange) MarshalJSONTo(enc *jsontext.Encoder) error {
	switch {
	case r.Name != "":
		return json.MarshalEncode(enc, r.Name)
	case r.Signal != "":
		return json.MarshalEncode(enc, signalObject{Signal: r.Signal})
	case r.Scheme != nil:
		return json.MarshalEncode(enc, r.Scheme)
	case !r.Step.IsZero():
		return json.MarshalEncode(enc, map[string]Value{"step": r.Step})
	case r.Data != nil:
		return json.MarshalEncode(enc, r.Data)
	}
	return json.MarshalEncode(enc, r.Values)
}

// IsZero reports whether the range is absent.
func (r ScaleRange) IsZero() bool {
	return r.Name == "" && r.Values == nil && r.Signal == "" && r.Scheme == nil && r.Step.IsZero() && r.Data == nil
}

// Projection maps longitude and latitude to the chart.
type Projection struct {
	Name   string `json:"name"`
	Type   Value  `json:"type,omitzero"`
	Params Extra  `json:",embed"`
}

// Axis draws a scale along an edge of a group.
type Axis struct {
	Scale        string      `json:"scale"`
	Orient       Value       `json:"orient"`
	Title        Value       `json:"title,omitzero"`
	Format       Value       `json:"format,omitzero"`
	FormatType   Value       `json:"formatType,omitzero"`
	Values       Value       `json:"values,omitzero"`
	TickCount    Value       `json:"tickCount,omitzero"`
	TickMinStep  Value       `json:"tickMinStep,omitzero"`
	Grid         *bool       `json:"grid,omitzero"`
	GridScale    string      `json:"gridScale,omitzero"`
	Domain       BoolValue   `json:"domain,omitzero"`
	Ticks        BoolValue   `json:"ticks,omitzero"`
	Labels       BoolValue   `json:"labels,omitzero"`
	LabelAngle   Value       `json:"labelAngle,omitzero"`
	LabelAlign   Value       `json:"labelAlign,omitzero"`
	LabelBase    Value       `json:"labelBaseline,omitzero"`
	LabelOverlap Value       `json:"labelOverlap,omitzero"`
	LabelFlush   Value       `json:"labelFlush,omitzero"`
	LabelPadding Value       `json:"labelPadding,omitzero"`
	LabelLimit   Value       `json:"labelLimit,omitzero"`
	BandPosition Value       `json:"bandPosition,omitzero"`
	MinExtent    Value       `json:"minExtent,omitzero"`
	MaxExtent    Value       `json:"maxExtent,omitzero"`
	Offset       Value       `json:"offset,omitzero"`
	ZIndex       *int        `json:"zindex,omitzero"`
	Aria         *bool       `json:"aria,omitzero"`
	Encode       GuideEncode `json:"encode,omitzero"`

	Extra Extra `json:",embed"`
}

// Legend draws the scales of color, size, shape and the like.
type Legend struct {
	Type           string      `json:"type,omitzero"`
	Fill           string      `json:"fill,omitzero"`
	Stroke         string      `json:"stroke,omitzero"`
	Size           string      `json:"size,omitzero"`
	Shape          string      `json:"shape,omitzero"`
	Opacity        string      `json:"opacity,omitzero"`
	StrokeDash     string      `json:"strokeDash,omitzero"`
	StrokeWidth    string      `json:"strokeWidth,omitzero"`
	Direction      Value       `json:"direction,omitzero"`
	Orient         Value       `json:"orient,omitzero"`
	Title          Value       `json:"title,omitzero"`
	Format         Value       `json:"format,omitzero"`
	FormatType     Value       `json:"formatType,omitzero"`
	Values         Value       `json:"values,omitzero"`
	SymbolType     Value       `json:"symbolType,omitzero"`
	GradientLength Value       `json:"gradientLength,omitzero"`
	Columns        Value       `json:"columns,omitzero"`
	LabelOverlap   Value       `json:"labelOverlap,omitzero"`
	Offset         Value       `json:"offset,omitzero"`
	LegendX        Value       `json:"legendX,omitzero"`
	LegendY        Value       `json:"legendY,omitzero"`
	ClipHeight     Value       `json:"clipHeight,omitzero"`
	TitleOrient    Value       `json:"titleOrient,omitzero"`
	ZIndex         *int        `json:"zindex,omitzero"`
	Aria           *bool       `json:"aria,omitzero"`
	Encode         GuideEncode `json:"encode,omitzero"`

	Extra Extra `json:",embed"`
}

// Title is the title of a chart or a group. The spec gives a string, or an
// object.
type Title struct {
	// Text is a string, an array of lines, or a signal.
	Text     Value   `json:"text,omitzero"`
	Subtitle Value   `json:"subtitle,omitzero"`
	Orient   Value   `json:"orient,omitzero"`
	Anchor   Value   `json:"anchor,omitzero"`
	Frame    Value   `json:"frame,omitzero"`
	Offset   Value   `json:"offset,omitzero"`
	Align    Value   `json:"align,omitzero"`
	Baseline Value   `json:"baseline,omitzero"`
	Angle    Value   `json:"angle,omitzero"`
	Color    Value   `json:"color,omitzero"`
	Font     Value   `json:"font,omitzero"`
	FontSize Value   `json:"fontSize,omitzero"`
	Style    Strings `json:"style,omitzero"`
	ZIndex   *int    `json:"zindex,omitzero"`
	// Encode holds the encodings of the title, its subtitle and its group,
	// or the sets of encodings of the title.
	Encode jsontext.Value `json:"encode,omitzero"`

	Extra Extra `json:",embed"`

	// short says that the spec gave the title as a string, so that it
	// writes back as one.
	short bool
}

// titleObject is Title without its methods, to decode the object form.
type titleObject Title

// UnmarshalJSONFrom reads a string or an object.
func (t *Title) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if dec.PeekKind() == '"' {
		raw, err := readValue(dec)
		if err != nil {
			return err
		}
		*t = Title{Text: Value{JSON: raw}, short: true}
		return nil
	}
	var o titleObject
	if err := json.UnmarshalDecode(dec, &o); err != nil {
		return err
	}
	*t = Title(o)
	return nil
}

// MarshalJSONTo writes a title that the spec gave as a string as a string,
// and any other title as an object.
func (t Title) MarshalJSONTo(enc *jsontext.Encoder) error {
	if t.short && t.Text.Signal == "" {
		cp := t
		cp.Text, cp.short = Value{}, false
		if isZeroTitle(cp) {
			return t.Text.MarshalJSONTo(enc)
		}
	}
	return json.MarshalEncode(enc, titleObject(t))
}

// isZeroTitle reports whether a title sets nothing.
func isZeroTitle(t Title) bool {
	b, err := json.Marshal(titleObject(t))
	return err == nil && string(b) == "{}"
}

// Mark is a mark: a group, or a rect, a line, a symbol and so on, with one
// item for each datum of its data.
type Mark struct {
	// Scope holds the fields of a group mark.
	Scope

	Type        string      `json:"type"`
	Name        string      `json:"name,omitzero"`
	Description string      `json:"description,omitzero"`
	Role        string      `json:"role,omitzero"`
	Style       Strings     `json:"style,omitzero"`
	From        *From       `json:"from,omitzero"`
	Key         string      `json:"key,omitzero"`
	Sort        *Sort       `json:"sort,omitzero"`
	Clip        Value       `json:"clip,omitzero"`
	Interactive *BoolValue  `json:"interactive,omitzero"`
	ZIndex      *int        `json:"zindex,omitzero"`
	Aria        *bool       `json:"aria,omitzero"`
	Transform   []Transform `json:"transform,omitzero"`

	Extra Extra `json:",embed"`
}

// From names the data of a mark: a dataset, or a facet that splits a
// dataset into groups.
type From struct {
	Data  string `json:"data,omitzero"`
	Facet *Facet `json:"facet,omitzero"`

	Extra Extra `json:",embed"`
}

// Facet splits a dataset into groups, one for each value of the group by
// fields, or one for each datum with an array in a field.
type Facet struct {
	Name      string         `json:"name"`
	Data      string         `json:"data"`
	GroupBy   Strings        `json:"groupby,omitzero"`
	Field     string         `json:"field,omitzero"`
	Aggregate jsontext.Value `json:"aggregate,omitzero"`

	Extra Extra `json:",embed"`
}

// Sort orders the items of a mark.
type Sort struct {
	// Field is a field name, an object {"expr": "..."}, or an array of
	// them.
	Field Value   `json:"field,omitzero"`
	Order Strings `json:"order,omitzero"`

	Extra Extra `json:",embed"`
}

// Encode holds the sets of encodings of a mark, such as enter and update.
type Encode map[string]Props

// Props holds the encodings of the properties of a mark, such as x and
// fill.
type Props map[string]Production

// GuideEncode holds the encodings of the parts of an axis, a legend or a
// title, such as labels, ticks and symbols.
type GuideEncode map[string]GuideElement

// GuideElement holds the encodings of one part of a guide, with its name
// and style.
type GuideElement struct {
	Name        string  `json:"name,omitzero"`
	Interactive *bool   `json:"interactive,omitzero"`
	Style       Strings `json:"style,omitzero"`
	// Sets holds the sets of encodings, such as enter and update.
	Sets Encode `json:",embed"`
}

// Production is the value of a property: one reference, or a list of rules
// whose first rule with a true test gives the value.
type Production []Rule

// Rule is a value reference with an optional test.
type Rule struct {
	ValueRef

	Test string `json:"test,omitzero"`
}

// ValueRef gives the value of a property: a literal, a signal, a field, or
// a field through a scale, with an offset, a multiple and a band.
type ValueRef struct {
	Value  jsontext.Value `json:"value,omitzero"`
	Signal string         `json:"signal,omitzero"`
	Field  Field          `json:"field,omitzero"`
	// Scale names a scale. It is a name, or an object such as
	// {"name": "x"} or {"signal": "expr"}.
	Scale Value `json:"scale,omitzero"`
	// Band is a number from 0 to 1 of the width of a band, or true for 1.
	Band Value `json:"band,omitzero"`
	// Offset and Mult are a number or a value reference.
	Offset   Value `json:"offset,omitzero"`
	Mult     Value `json:"mult,omitzero"`
	Exponent Value `json:"exponent,omitzero"`
	Round    *bool `json:"round,omitzero"`
	Extra    Extra `json:",embed"`
}

// UnmarshalJSONFrom reads one rule or an array of rules.
func (p *Production) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if dec.PeekKind() == '[' {
		var rules []Rule
		if err := json.UnmarshalDecode(dec, &rules); err != nil {
			return err
		}
		*p = rules
		return nil
	}
	var r Rule
	if err := json.UnmarshalDecode(dec, &r); err != nil {
		return err
	}
	*p = Production{r}
	return nil
}

// MarshalJSONTo writes one rule with no test as an object, and any other
// production as an array.
func (p Production) MarshalJSONTo(enc *jsontext.Encoder) error {
	if len(p) == 1 && p[0].Test == "" {
		return json.MarshalEncode(enc, p[0])
	}
	return json.MarshalEncode(enc, []Rule(p))
}
