package scene

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// Paint is the fill or the stroke of an item: a color, a gradient, or
// nothing. Vega writes a color as a string and a gradient as an object.
type Paint struct {
	Color    string
	Gradient *Gradient
}

// Gradient is a linear or a radial gradient, as Vega writes it. The
// coordinates are fractions of the bounds of the item.
type Gradient struct {
	Type  string  `json:"gradient"`
	X1    float64 `json:"x1,omitzero"`
	Y1    float64 `json:"y1,omitzero"`
	X2    float64 `json:"x2,omitzero"`
	Y2    float64 `json:"y2,omitzero"`
	R1    float64 `json:"r1,omitzero"`
	R2    float64 `json:"r2,omitzero"`
	Stops []Stop  `json:"stops"`
}

// The types of gradient.
const (
	Linear = "linear"
	Radial = "radial"
)

// Stop is one color of a gradient, at an offset from 0 to 1.
type Stop struct {
	Offset float64 `json:"offset"`
	Color  string  `json:"color"`
}

// IsZero reports whether the paint is absent, which draws nothing.
func (p Paint) IsZero() bool {
	return p.Color == "" && p.Gradient == nil
}

// UnmarshalJSONFrom reads a color, a gradient or null.
func (p *Paint) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	switch dec.PeekKind() {
	case '{':
		var g Gradient
		if err := json.UnmarshalDecode(dec, &g); err != nil {
			return err
		}
		*p = Paint{Gradient: &g}
	case 'n':
		if _, err := dec.ReadToken(); err != nil {
			return err
		}
		*p = Paint{}
	default:
		var s string
		if err := json.UnmarshalDecode(dec, &s); err != nil {
			return err
		}
		*p = Paint{Color: s}
	}
	return nil
}

// MarshalJSONTo writes the color or the gradient.
func (p Paint) MarshalJSONTo(enc *jsontext.Encoder) error {
	if p.Gradient != nil {
		return json.MarshalEncode(enc, p.Gradient)
	}
	return json.MarshalEncode(enc, p.Color)
}
