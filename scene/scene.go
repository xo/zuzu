// Package scene holds the scene graph of zuzu: the shapes of a chart with their
// final positions, sizes and colors (D8).
//
// The runtime produces a scene graph and every renderer reads it, and nothing
// else. A Mark and an Item have the shape of the scene graph of Vega, which
// vega.js writes as JSON with sceneToJSON, so that the marks and the data of a
// chart can be compared with vega.js (D17). A Scene adds the size and the
// background of the canvas, which Vega keeps on the view.
package scene

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
)

// Scene is a scene graph and the canvas that it draws on.
type Scene struct {
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
	Background Paint   `json:"background,omitzero"`
	Root       Mark    `json:"root"`
}

// Mark is a mark of the scene graph: a kind of shape and its items. A group
// mark holds the marks of each of its items.
type Mark struct {
	Type   string `json:"marktype"`
	Role   string `json:"role,omitzero"`
	Name   string `json:"name,omitzero"`
	Clip   bool   `json:"clip,omitzero"`
	ZIndex int    `json:"zindex,omitzero"`
	Items  []Item `json:"items"`

	// Extra holds the properties that zuzu does not use, such as
	// interactive, so that a scene graph of vega.js reads without loss.
	Extra map[string]jsontext.Value `json:",embed"`
}

// The types of mark, as Vega names them.
const (
	Arc    = "arc"
	Area   = "area"
	Group  = "group"
	Image  = "image"
	Line   = "line"
	Path   = "path"
	Rect   = "rect"
	Rule   = "rule"
	Shape  = "shape"
	Symbol = "symbol"
	Text   = "text"
	Trail  = "trail"
)

// Item is one item of a mark: one shape, or for a line or an area, one point
// of the shape. The fields that a kind of mark does not use are zero. A field
// that Vega gives a default other than zero is a pointer, so that an absent
// field differs from a zero.
type Item struct {
	// Position and size.
	X      float64 `json:"x,omitzero"`
	Y      float64 `json:"y,omitzero"`
	X2     float64 `json:"x2,omitzero"`
	Y2     float64 `json:"y2,omitzero"`
	Width  float64 `json:"width,omitzero"`
	Height float64 `json:"height,omitzero"`

	// Paint.
	Fill          Paint     `json:"fill,omitzero"`
	Stroke        Paint     `json:"stroke,omitzero"`
	Opacity       *float64  `json:"opacity,omitzero"`
	FillOpacity   *float64  `json:"fillOpacity,omitzero"`
	StrokeOpacity *float64  `json:"strokeOpacity,omitzero"`
	StrokeWidth   *float64  `json:"strokeWidth,omitzero"`
	StrokeDash    []float64 `json:"strokeDash,omitzero"`
	StrokeCap     string    `json:"strokeCap,omitzero"`
	StrokeJoin    string    `json:"strokeJoin,omitzero"`

	// Corners of a rect or a group, and of an arc.
	CornerRadius            float64 `json:"cornerRadius,omitzero"`
	CornerRadiusTopLeft     float64 `json:"cornerRadiusTopLeft,omitzero"`
	CornerRadiusTopRight    float64 `json:"cornerRadiusTopRight,omitzero"`
	CornerRadiusBottomLeft  float64 `json:"cornerRadiusBottomLeft,omitzero"`
	CornerRadiusBottomRight float64 `json:"cornerRadiusBottomRight,omitzero"`

	// An arc.
	StartAngle  float64 `json:"startAngle,omitzero"`
	EndAngle    float64 `json:"endAngle,omitzero"`
	InnerRadius float64 `json:"innerRadius,omitzero"`
	OuterRadius float64 `json:"outerRadius,omitzero"`
	PadAngle    float64 `json:"padAngle,omitzero"`

	// A symbol.
	Shape string   `json:"shape,omitzero"`
	Size  *float64 `json:"size,omitzero"`
	Angle float64  `json:"angle,omitzero"`

	// A path, in the syntax of the d attribute of SVG.
	Path string `json:"path,omitzero"`

	// A line or an area. Vega reads these from the first item.
	Interpolate string  `json:"interpolate,omitzero"`
	Tension     float64 `json:"tension,omitzero"`
	Orient      string  `json:"orient,omitzero"`
	Defined     *bool   `json:"defined,omitzero"`

	// Text.
	Text       Lines   `json:"text,omitzero"`
	Font       string  `json:"font,omitzero"`
	FontSize   float64 `json:"fontSize,omitzero"`
	FontWeight Weight  `json:"fontWeight,omitzero"`
	FontStyle  string  `json:"fontStyle,omitzero"`
	Align      string  `json:"align,omitzero"`
	Baseline   string  `json:"baseline,omitzero"`
	Dx         float64 `json:"dx,omitzero"`
	Dy         float64 `json:"dy,omitzero"`
	Radius     float64 `json:"radius,omitzero"`
	Theta      float64 `json:"theta,omitzero"`
	LineHeight float64 `json:"lineHeight,omitzero"`
	Limit      float64 `json:"limit,omitzero"`
	Ellipsis   string  `json:"ellipsis,omitzero"`
	Dir        string  `json:"dir,omitzero"`

	// An image: its URL, whether it keeps the ratio of its width to its
	// height, which Vega does by default, and whether it smooths its pixels
	// when it grows, which Vega also does by default.
	URL    string `json:"url,omitzero"`
	Aspect *bool  `json:"aspect,omitzero"`
	Smooth *bool  `json:"smooth,omitzero"`

	// A group: whether it clips its marks, and its marks.
	Clip  bool   `json:"clip,omitzero"`
	Items []Mark `json:"items,omitzero"`

	// Datum is the row of data that the item draws, which the runtime keeps
	// for the comparison with vega.js (D17). It is not written to JSON.
	Datum any `json:"-"`

	// Extra holds the properties that zuzu does not use, such as the datum,
	// so that a scene graph of vega.js reads without loss.
	Extra map[string]jsontext.Value `json:",embed"`
}

// Decode reads a scene from JSON.
func Decode(r io.Reader) (*Scene, error) {
	var s Scene
	if err := json.UnmarshalRead(r, &s); err != nil {
		return nil, fmt.Errorf("decoding scene: %w", err)
	}
	return &s, nil
}

// Encode writes a scene as JSON. The same scene gives the same bytes each
// time.
func Encode(w io.Writer, s *Scene) error {
	if err := json.MarshalWrite(w, s, json.Deterministic(true)); err != nil {
		return fmt.Errorf("encoding scene: %w", err)
	}
	return nil
}

// Lines is the text of a text item: one line, or several lines, which Vega
// writes as an array.
type Lines []string

// UnmarshalJSONFrom reads a string, an array of strings, or a number or a
// boolean, which Vega writes as its text.
func (l *Lines) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	switch dec.PeekKind() {
	case '[':
		var lines []string
		if err := json.UnmarshalDecode(dec, &lines); err != nil {
			return err
		}
		*l = lines
	case '"':
		var s string
		if err := json.UnmarshalDecode(dec, &s); err != nil {
			return err
		}
		*l = Lines{s}
	default:
		v, err := dec.ReadValue()
		if err != nil {
			return err
		}
		*l = Lines{string(v)}
	}
	return nil
}

// MarshalJSONTo writes one line as a string and several as an array.
func (l Lines) MarshalJSONTo(enc *jsontext.Encoder) error {
	if len(l) == 1 {
		return json.MarshalEncode(enc, l[0])
	}
	return json.MarshalEncode(enc, []string(l))
}

// Weight is the weight of a font, such as bold or 600. Vega writes it as a
// string or a number.
type Weight string

// UnmarshalJSONFrom reads a string or a number.
func (w *Weight) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	v, err := dec.ReadValue()
	if err != nil {
		return err
	}
	*w = Weight(bytes.Trim(v, `"`))
	return nil
}

// MarshalJSONTo writes the weight as a string.
func (w Weight) MarshalJSONTo(enc *jsontext.Encoder) error {
	return json.MarshalEncode(enc, string(w))
}
