// Package color reads the colors of CSS and converts them between the color
// spaces of d3: RGB, HSL, CIELAB and HCL. The scales and the expressions
// share it.
package color

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// RGBA is a color with channels from 0 to 255 and an opacity from 0 to 1.
type RGBA struct {
	R, G, B, A float64
}

// named are the named colors of CSS.
var named = map[string]uint32{
	"aliceblue": 0xf0f8ff, "antiquewhite": 0xfaebd7, "aqua": 0x00ffff,
	"aquamarine": 0x7fffd4, "azure": 0xf0ffff, "beige": 0xf5f5dc,
	"bisque": 0xffe4c4, "black": 0x000000, "blanchedalmond": 0xffebcd,
	"blue": 0x0000ff, "blueviolet": 0x8a2be2, "brown": 0xa52a2a,
	"burlywood": 0xdeb887, "cadetblue": 0x5f9ea0, "chartreuse": 0x7fff00,
	"chocolate": 0xd2691e, "coral": 0xff7f50, "cornflowerblue": 0x6495ed,
	"cornsilk": 0xfff8dc, "crimson": 0xdc143c, "cyan": 0x00ffff,
	"darkblue": 0x00008b, "darkcyan": 0x008b8b, "darkgoldenrod": 0xb8860b,
	"darkgray": 0xa9a9a9, "darkgreen": 0x006400, "darkgrey": 0xa9a9a9,
	"darkkhaki": 0xbdb76b, "darkmagenta": 0x8b008b,
	"darkolivegreen": 0x556b2f, "darkorange": 0xff8c00,
	"darkorchid": 0x9932cc, "darkred": 0x8b0000, "darksalmon": 0xe9967a,
	"darkseagreen": 0x8fbc8f, "darkslateblue": 0x483d8b,
	"darkslategray": 0x2f4f4f, "darkslategrey": 0x2f4f4f,
	"darkturquoise": 0x00ced1, "darkviolet": 0x9400d3, "deeppink": 0xff1493,
	"deepskyblue": 0x00bfff, "dimgray": 0x696969, "dimgrey": 0x696969,
	"dodgerblue": 0x1e90ff, "firebrick": 0xb22222, "floralwhite": 0xfffaf0,
	"forestgreen": 0x228b22, "fuchsia": 0xff00ff, "gainsboro": 0xdcdcdc,
	"ghostwhite": 0xf8f8ff, "gold": 0xffd700, "goldenrod": 0xdaa520,
	"gray": 0x808080, "green": 0x008000, "greenyellow": 0xadff2f,
	"grey": 0x808080, "honeydew": 0xf0fff0, "hotpink": 0xff69b4,
	"indianred": 0xcd5c5c, "indigo": 0x4b0082, "ivory": 0xfffff0,
	"khaki": 0xf0e68c, "lavender": 0xe6e6fa, "lavenderblush": 0xfff0f5,
	"lawngreen": 0x7cfc00, "lemonchiffon": 0xfffacd, "lightblue": 0xadd8e6,
	"lightcoral": 0xf08080, "lightcyan": 0xe0ffff,
	"lightgoldenrodyellow": 0xfafad2, "lightgray": 0xd3d3d3,
	"lightgreen": 0x90ee90, "lightgrey": 0xd3d3d3, "lightpink": 0xffb6c1,
	"lightsalmon": 0xffa07a, "lightseagreen": 0x20b2aa,
	"lightskyblue": 0x87cefa, "lightslategray": 0x778899,
	"lightslategrey": 0x778899, "lightsteelblue": 0xb0c4de,
	"lightyellow": 0xffffe0, "lime": 0x00ff00, "limegreen": 0x32cd32,
	"linen": 0xfaf0e6, "magenta": 0xff00ff, "maroon": 0x800000,
	"mediumaquamarine": 0x66cdaa, "mediumblue": 0x0000cd,
	"mediumorchid": 0xba55d3, "mediumpurple": 0x9370db,
	"mediumseagreen": 0x3cb371, "mediumslateblue": 0x7b68ee,
	"mediumspringgreen": 0x00fa9a, "mediumturquoise": 0x48d1cc,
	"mediumvioletred": 0xc71585, "midnightblue": 0x191970,
	"mintcream": 0xf5fffa, "mistyrose": 0xffe4e1, "moccasin": 0xffe4b5,
	"navajowhite": 0xffdead, "navy": 0x000080, "oldlace": 0xfdf5e6,
	"olive": 0x808000, "olivedrab": 0x6b8e23, "orange": 0xffa500,
	"orangered": 0xff4500, "orchid": 0xda70d6, "palegoldenrod": 0xeee8aa,
	"palegreen": 0x98fb98, "paleturquoise": 0xafeeee,
	"palevioletred": 0xdb7093, "papayawhip": 0xffefd5, "peachpuff": 0xffdab9,
	"peru": 0xcd853f, "pink": 0xffc0cb, "plum": 0xdda0dd,
	"powderblue": 0xb0e0e6, "purple": 0x800080, "rebeccapurple": 0x663399,
	"red": 0xff0000, "rosybrown": 0xbc8f8f, "royalblue": 0x4169e1,
	"saddlebrown": 0x8b4513, "salmon": 0xfa8072, "sandybrown": 0xf4a460,
	"seagreen": 0x2e8b57, "seashell": 0xfff5ee, "sienna": 0xa0522d,
	"silver": 0xc0c0c0, "skyblue": 0x87ceeb, "slateblue": 0x6a5acd,
	"slategray": 0x708090, "slategrey": 0x708090, "snow": 0xfffafa,
	"springgreen": 0x00ff7f, "steelblue": 0x4682b4, "tan": 0xd2b48c,
	"teal": 0x008080, "thistle": 0xd8bfd8, "tomato": 0xff6347,
	"turquoise": 0x40e0d0, "violet": 0xee82ee, "wheat": 0xf5deb3,
	"white": 0xffffff, "whitesmoke": 0xf5f5f5, "yellow": 0xffff00,
	"yellowgreen": 0x9acd32,
}

// Parse reads a color of CSS: a name, #rgb, #rrggbb, rgb(), rgba(),
// hsl() or hsla(). It reports false, with black, for a color that does not
// read.
func Parse(s string) (RGBA, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if v, ok := named[s]; ok {
		return RGBA{float64(v >> 16 & 0xff), float64(v >> 8 & 0xff), float64(v & 0xff), 1}, true
	}
	if s == "transparent" {
		return RGBA{0, 0, 0, 0}, true
	}
	if strings.HasPrefix(s, "#") {
		h := s[1:]
		if len(h) == 3 || len(h) == 4 {
			var b strings.Builder
			for _, c := range h {
				b.WriteRune(c)
				b.WriteRune(c)
			}
			h = b.String()
		}
		if len(h) == 6 {
			h += "ff"
		}
		v, err := strconv.ParseUint(h, 16, 32)
		if err != nil || len(h) != 8 {
			return RGBA{A: 1}, false
		}
		return RGBA{float64(v >> 24 & 0xff), float64(v >> 16 & 0xff), float64(v >> 8 & 0xff), float64(v&0xff) / 255}, true
	}
	lp, rp := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
	if lp < 0 || rp < lp {
		return RGBA{A: 1}, false
	}
	fn := s[:lp]
	parts := strings.FieldsFunc(s[lp+1:rp], func(r rune) bool { return r == ',' || r == ' ' || r == '/' })
	num := func(i int, scale float64) float64 {
		if i >= len(parts) {
			return 1
		}
		p := parts[i]
		if pct, ok := strings.CutSuffix(p, "%"); ok {
			f, _ := strconv.ParseFloat(pct, 64)
			return f / 100 * scale
		}
		f, _ := strconv.ParseFloat(strings.TrimSuffix(p, "deg"), 64)
		return f
	}
	switch fn {
	case "rgb", "rgba":
		return RGBA{num(0, 255), num(1, 255), num(2, 255), num(3, 1)}, true
	case "hsl", "hsla":
		return FromHSL(num(0, 360), num(1, 1), num(2, 1), num(3, 1)), true
	}
	return RGBA{A: 1}, false
}

// FromHSL converts a hue in degrees, a saturation and a lightness from 0 to
// 1, and an opacity to a color.
func FromHSL(h, s, l, a float64) RGBA {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	m2 := l + s - l*s
	if l < 0.5 {
		m2 = l * (1 + s)
	}
	m1 := 2*l - m2
	hue := func(h float64) float64 {
		switch {
		case h < 60:
			return m1 + (m2-m1)*h/60
		case h < 180:
			return m2
		case h < 240:
			return m1 + (m2-m1)*(240-h)/60
		}
		return m1
	}
	conv := func(h float64) float64 {
		h = math.Mod(h+360, 360)
		return hue(h) * 255
	}
	return RGBA{conv(h + 120), conv(h), conv(h - 120), a}
}

// Hex writes a color as #rrggbb, or as rgba() when it is not opaque.
func (c RGBA) Hex() string {
	if c.A < 1 {
		return c.String()
	}
	return fmt.Sprintf("#%02x%02x%02x", channel(c.R), channel(c.G), channel(c.B))
}

// String writes a color as rgb() or rgba(), as d3 writes a color.
func (c RGBA) String() string {
	if c.A < 1 {
		return fmt.Sprintf("rgba(%d, %d, %d, %s)", channel(c.R), channel(c.G), channel(c.B), strconv.FormatFloat(math.Max(0, c.A), 'g', -1, 64))
	}
	return fmt.Sprintf("rgb(%d, %d, %d)", channel(c.R), channel(c.G), channel(c.B))
}

// channel rounds a channel to an integer from 0 to 255.
func channel(v float64) int {
	if math.IsNaN(v) {
		return 0
	}
	return int(math.Max(0, math.Min(255, math.Round(v))))
}

// HSL returns the hue in degrees, the saturation and the lightness of a
// color. The hue of a gray is NaN, as in d3.
func (c RGBA) HSL() (h, s, l float64) {
	r, g, b := c.R/255, c.G/255, c.B/255
	lo, hi := math.Min(r, math.Min(g, b)), math.Max(r, math.Max(g, b))
	h, s, l = math.NaN(), hi-lo, (hi+lo)/2
	if s == 0 {
		if l > 0 && l < 1 {
			return h, 0, l
		}
		return h, s, l
	}
	switch hi {
	case r:
		h = (g - b) / s
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/s + 2
	default:
		h = (r-g)/s + 4
	}
	if l < 0.5 {
		s /= hi + lo
	} else {
		s /= 2 - hi - lo
	}
	return h * 60, s, l
}

// Lab is a color in CIELAB, with an opacity.
type Lab struct {
	L, A, B, Alpha float64
}

// The white point and the constants of CIELAB, as d3 uses them.
const (
	xn, yn, zn = 0.96422, 1, 0.82521
	t0, t1     = 4.0 / 29, 6.0 / 29
	t2         = 3 * t1 * t1
	t3         = t1 * t1 * t1
)

// Lab converts a color to CIELAB, with the white point of the illuminant
// that d3 uses.
func (c RGBA) Lab() Lab {
	r, g, b := linear(c.R), linear(c.G), linear(c.B)
	y := xyz((0.2225045*r + 0.7168786*g + 0.0606169*b) / yn)
	x, z := y, y
	if r != g || g != b {
		x = xyz((0.4360747*r + 0.3850649*g + 0.1430804*b) / xn)
		z = xyz((0.0139322*r + 0.0971045*g + 0.7141733*b) / zn)
	}
	return Lab{116*y - 16, 500 * (x - y), 200 * (y - z), c.A}
}

// RGBA converts a color in CIELAB to RGB.
func (c Lab) RGBA() RGBA {
	y := (c.L + 16) / 116
	x, z := y, y
	if !math.IsNaN(c.A) {
		x = y + c.A/500
	}
	if !math.IsNaN(c.B) {
		z = y - c.B/200
	}
	x, y, z = xn*lab2xyz(x), yn*lab2xyz(y), zn*lab2xyz(z)
	return RGBA{
		gamma(3.1338561*x - 1.6168667*y - 0.4906146*z),
		gamma(-0.9787684*x + 1.9161415*y + 0.0334540*z),
		gamma(0.0719453*x - 0.2289914*y + 1.4052427*z),
		c.Alpha,
	}
}

// HCL returns the hue in degrees, the chroma and the lightness of a color in
// CIELAB.
func (c Lab) HCL() (h, ch, l float64) {
	ch = math.Hypot(c.A, c.B)
	h = math.Atan2(c.B, c.A) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	if ch < 1e-6 {
		h = math.NaN()
	}
	return h, ch, c.L
}

// FromHCL converts a hue in degrees, a chroma and a lightness to a color in
// CIELAB.
func FromHCL(h, ch, l, alpha float64) Lab {
	if math.IsNaN(h) {
		return Lab{l, 0, 0, alpha}
	}
	r := h * math.Pi / 180
	return Lab{l, ch * math.Cos(r), ch * math.Sin(r), alpha}
}

func xyz(t float64) float64 {
	if t > t3 {
		return math.Cbrt(t)
	}
	return t/t2 + t0
}

func lab2xyz(t float64) float64 {
	if t > t1 {
		return t * t * t
	}
	return t2 * (t - t0)
}

func linear(x float64) float64 {
	x /= 255
	if x <= 0.04045 {
		return x / 12.92
	}
	return math.Pow((x+0.055)/1.055, 2.4)
}

func gamma(x float64) float64 {
	if x <= 0.0031308 {
		return 255 * 12.92 * x
	}
	return 255 * (1.055*math.Pow(x, 1/2.4) - 0.055)
}

// Luminance returns the relative luminance of a color, from 0 for black to
// 1 for white, as WCAG 2 defines it.
func (c RGBA) Luminance() float64 {
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
}

// Mix returns the color a share f of the way from a to b, in a color
// space: rgb, the default, lab, hcl or hsl. A space that ends in -long
// takes the long way round the hue.
func Mix(a, b RGBA, f float64, space string) RGBA {
	mix := func(x, y float64) float64 { return x + f*(y-x) }
	switch strings.TrimSuffix(space, "-long") {
	case "lab":
		la, lb := a.Lab(), b.Lab()
		return Lab{mix(la.L, lb.L), mix(la.A, lb.A), mix(la.B, lb.B), mix(la.Alpha, lb.Alpha)}.RGBA()
	case "hcl":
		la, lb := a.Lab(), b.Lab()
		ha, ca := math.Atan2(la.B, la.A)*180/math.Pi, math.Hypot(la.A, la.B)
		hb, cb := math.Atan2(lb.B, lb.A)*180/math.Pi, math.Hypot(lb.A, lb.B)
		if ca < 1e-6 {
			ha = hb
		}
		if cb < 1e-6 {
			hb = ha
		}
		d := hb - ha
		if !strings.HasSuffix(space, "-long") {
			d = math.Mod(d+540, 360) - 180
		}
		h := (ha + f*d) * math.Pi / 180
		c := mix(ca, cb)
		return Lab{mix(la.L, lb.L), c * math.Cos(h), c * math.Sin(h), mix(la.Alpha, lb.Alpha)}.RGBA()
	}
	return RGBA{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), mix(a.A, b.A)}
}
