package expr

import (
	"math"

	"github.com/xo/zuzu/internal/color"
)

// Color is a color that the functions rgb, hsl, lab and hcl make. An
// expression reads its channels as properties, such as .r of an RGB color
// or .h of an HSL color, and writes it as rgb() or rgba(), as d3 does.
// merge keeps a color a color: its channels, and those of the objects
// merged with it, give the color in the space of the last color merged.
type Color struct {
	rgb    color.RGBA
	space  string
	fields map[string]any
}

// The spaces of a color.
const (
	spaceRGB = "rgb"
	spaceHSL = "hsl"
	spaceLab = "lab"
	spaceHCL = "hcl"
)

// newColor makes a color in a space from its channels, which are the
// properties r, g and b of RGB, h, s and l of HSL, l, a and b of CIELAB,
// and h, c and l of HCL, with the property opacity.
func newColor(space string, fields map[string]any) *Color {
	num := func(name string) float64 {
		if v, ok := fields[name]; ok {
			return Number(v)
		}
		return math.NaN()
	}
	a := num("opacity")
	var rgb color.RGBA
	switch space {
	case spaceHSL:
		h, s := num("h"), num("s")
		if math.IsNaN(h) || math.IsNaN(s) {
			// A color with no hue is a gray, as in d3.
			h, s = 0, 0
		}
		rgb = color.FromHSL(h, s, num("l"), a)
	case spaceLab:
		rgb = color.Lab{L: num("l"), A: num("a"), B: num("b"), Alpha: a}.RGBA()
	case spaceHCL:
		rgb = color.FromHCL(num("h"), num("c"), num("l"), a).RGBA()
	default:
		rgb = color.RGBA{R: num("r"), G: num("g"), B: num("b"), A: a}
	}
	if math.IsNaN(rgb.A) {
		rgb.A = 1
	}
	return &Color{rgb: rgb, space: space, fields: fields}
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
		return newColor(spaceRGB, map[string]any{"r": c.R, "g": c.G, "b": c.B, "opacity": c.A}), nil
	},
	"hsl": func(_ *Scope, args []any) (any, error) {
		var h, s, l, a float64
		if c, ok := colorArgs(args); ok {
			h, s, l = c.HSL()
			a = c.A
		} else {
			h, s, l, a = numArg(args, 0), numArg(args, 1), numArg(args, 2), opacityArg(args)
		}
		return newColor(spaceHSL, map[string]any{"h": h, "s": s, "l": l, "opacity": a}), nil
	},
	"lab": func(_ *Scope, args []any) (any, error) {
		var lab color.Lab
		if c, ok := colorArgs(args); ok {
			lab = c.Lab()
		} else {
			lab = color.Lab{L: numArg(args, 0), A: numArg(args, 1), B: numArg(args, 2), Alpha: opacityArg(args)}
		}
		return newColor(spaceLab, map[string]any{"l": lab.L, "a": lab.A, "b": lab.B, "opacity": lab.Alpha}), nil
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
		return newColor(spaceHCL, map[string]any{"h": h, "c": c, "l": l, "opacity": a}), nil
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
