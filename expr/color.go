package expr

import (
	"math"

	"github.com/xo/zuzu/internal/color"
)

// Color is a color that the functions rgb, hsl, lab and hcl make. An
// expression reads its channels as properties, such as .r of an RGB color
// or .h of an HSL color, and writes it as rgb() or rgba(), as d3 does.
type Color struct {
	rgb    color.RGBA
	fields map[string]float64
}

// String writes the color as rgb() or rgba().
func (c *Color) String() string {
	return c.rgb.String()
}

// get reads a channel of the color.
func (c *Color) get(name string) any {
	if v, ok := c.fields[name]; ok {
		return v
	}
	return Undefined
}

// colorOf reads the color of a value: a Color, or a string of CSS.
func colorOf(v any) (color.RGBA, bool) {
	switch x := v.(type) {
	case *Color:
		return x.rgb, true
	case string:
		return color.Parse(x)
	}
	return color.RGBA{}, false
}

// colorFuncs are the functions of colors.
var colorFuncs = map[string]Func{
	"rgb": func(_ *Scope, args []any) (any, error) {
		c, ok := colorArgs(args)
		if !ok {
			c = color.RGBA{R: numArg(args, 0), G: numArg(args, 1), B: numArg(args, 2), A: opacityArg(args)}
		}
		return &Color{rgb: c, fields: map[string]float64{"r": c.R, "g": c.G, "b": c.B, "opacity": c.A}}, nil
	},
	"hsl": func(_ *Scope, args []any) (any, error) {
		var h, s, l, a float64
		if c, ok := colorArgs(args); ok {
			h, s, l = c.HSL()
			a = c.A
		} else {
			h, s, l, a = numArg(args, 0), numArg(args, 1), numArg(args, 2), opacityArg(args)
		}
		rgb := color.FromHSL(zeroIfNaN(h), zeroIfNaN(s), l, a)
		return &Color{rgb: rgb, fields: map[string]float64{"h": h, "s": s, "l": l, "opacity": a}}, nil
	},
	"lab": func(_ *Scope, args []any) (any, error) {
		var lab color.Lab
		if c, ok := colorArgs(args); ok {
			lab = c.Lab()
		} else {
			lab = color.Lab{L: numArg(args, 0), A: numArg(args, 1), B: numArg(args, 2), Alpha: opacityArg(args)}
		}
		return &Color{rgb: lab.RGBA(), fields: map[string]float64{"l": lab.L, "a": lab.A, "b": lab.B, "opacity": lab.Alpha}}, nil
	},
	"hcl": func(_ *Scope, args []any) (any, error) {
		var h, c, l, a float64
		if rgb, ok := colorArgs(args); ok {
			lab := rgb.Lab()
			h, c, l = lab.HCL()
			a = rgb.A
		} else {
			h, c, l, a = numArg(args, 0), numArg(args, 1), numArg(args, 2), opacityArg(args)
		}
		return &Color{rgb: color.FromHCL(h, c, l, a).RGBA(), fields: map[string]float64{"h": h, "c": c, "l": l, "opacity": a}}, nil
	},
	"luminance": func(_ *Scope, args []any) (any, error) {
		c, ok := colorOf(arg(args, 0))
		if !ok {
			return nan(), nil
		}
		return c.Luminance(), nil
	},
	"contrast": func(_ *Scope, args []any) (any, error) {
		a, ok1 := colorOf(arg(args, 0))
		b, ok2 := colorOf(arg(args, 1))
		if !ok1 || !ok2 {
			return nan(), nil
		}
		la, lb := a.Luminance(), b.Luminance()
		return (math.Max(la, lb) + 0.05) / (math.Min(la, lb) + 0.05), nil
	},
}

// colorArgs reads the arguments of a color function that are one color:
// a string of CSS or another color.
func colorArgs(args []any) (color.RGBA, bool) {
	if len(args) != 1 {
		return color.RGBA{}, false
	}
	return colorOf(args[0])
}

// opacityArg reads the opacity of a color function, its fourth argument,
// which is 1 when the call leaves it out.
func opacityArg(args []any) float64 {
	v := arg(args, 3)
	if IsUndefined(v) || v == nil {
		return 1
	}
	return Number(v)
}

// zeroIfNaN returns 0 for NaN.
func zeroIfNaN(f float64) float64 {
	if math.IsNaN(f) {
		return 0
	}
	return f
}
