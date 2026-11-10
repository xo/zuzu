package theme

import (
	"fmt"
	"math"
	"strconv"
)

// rgb is a color with each channel from 0 to 255.
type rgb struct{ r, g, b float64 }

// parse reads a color in the form #rrggbb or #rgb. A color in another form
// reads as black.
func parse(s string) rgb {
	if len(s) == 4 && s[0] == '#' {
		s = string([]byte{'#', s[1], s[1], s[2], s[2], s[3], s[3]})
	}
	if len(s) != 7 || s[0] != '#' {
		return rgb{}
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return rgb{}
	}
	return rgb{float64(v >> 16 & 0xff), float64(v >> 8 & 0xff), float64(v & 0xff)}
}

// hex writes a color in the form #rrggbb.
func (c rgb) hex() string {
	clamp := func(v float64) int { return int(math.Round(math.Max(0, math.Min(255, v)))) }
	return fmt.Sprintf("#%02x%02x%02x", clamp(c.r), clamp(c.g), clamp(c.b))
}

// mix returns the color at t between a and b, in RGB.
func mix(a, b string, t float64) string {
	x, y := parse(a), parse(b)
	return rgb{x.r + (y.r-x.r)*t, x.g + (y.g-x.g)*t, x.b + (y.b-x.b)*t}.hex()
}

// ramp returns the color at v, from 0 to 1, on a ramp of evenly spaced stops.
func ramp(stops []string, v float64) string {
	v = math.Max(0, math.Min(1, v))
	if v == 1 {
		return parse(stops[len(stops)-1]).hex()
	}
	pos := v * float64(len(stops)-1)
	i := int(pos)
	return mix(stops[i], stops[i+1], pos-float64(i))
}

// brighter lifts the lightness of a color in HSL by the factor (1/0.7)^k, as
// the brighter method of d3-color does.
func brighter(s string, k float64) string {
	h, sat, l := toHSL(parse(s))
	return fromHSL(h, sat, l*math.Pow(1/0.7, k)).hex()
}

// toHSL converts a color to hue in degrees, saturation and lightness.
func toHSL(c rgb) (h, s, l float64) {
	r, g, b := c.r/255, c.g/255, c.b/255
	hi, lo := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (hi + lo) / 2
	d := hi - lo
	if d == 0 {
		return 0, 0, l
	}
	if l < 0.5 {
		s = d / (hi + lo)
	} else {
		s = d / (2 - hi - lo)
	}
	switch hi {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h * 60, s, l
}

// fromHSL converts hue in degrees, saturation and lightness to a color.
func fromHSL(h, s, l float64) rgb {
	l = math.Min(1, l)
	m2 := l + s - l*s
	if l < 0.5 {
		m2 = l * (1 + s)
	}
	m1 := 2*l - m2
	channel := func(h float64) float64 {
		h = math.Mod(h+360, 360)
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
	return rgb{channel(h+120) * 255, channel(h) * 255, channel(h-120) * 255}
}
