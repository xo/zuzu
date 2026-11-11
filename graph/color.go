package graph

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/xo/zuzu/internal/color"
	"github.com/xo/zuzu/theme"
)

// Color reads a color of DOT: a name of CSS or of the X11 scheme of
// Graphviz, #rrggbb, #rrggbbaa, or hue, saturation and value from 0 to 1.
// A list of colors takes its first.
func Color(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if i := strings.IndexAny(v, ":;"); i > 0 {
		v = v[:i]
	}
	if c, ok := x11[strings.ToLower(v)]; ok {
		return fmt.Sprintf("#%06x", c), true
	}
	if c, ok := color.Parse(v); ok {
		if len(v) == 9 && v[0] == '#' {
			return fmt.Sprintf("rgba(%d,%d,%d,%s)", int(c.R), int(c.G), int(c.B), strconv.FormatFloat(c.A, 'f', 3, 64)), true
		}
		_ = c
		return v, true
	}
	f := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' })
	if len(f) == 3 {
		var hsv [3]float64
		for i, s := range f {
			x, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return "", false
			}
			hsv[i] = x
		}
		r, g, b := hsvToRGB(hsv[0], hsv[1], hsv[2])
		return fmt.Sprintf("rgb(%d,%d,%d)", r, g, b), true
	}
	return "", false
}

func hsvToRGB(h, s, v float64) (int, int, int) {
	h = math.Mod(h, 1) * 6
	i := math.Floor(h)
	f := h - i
	p, q, t := v*(1-s), v*(1-s*f), v*(1-s*(1-f))
	var r, g, b float64
	switch int(i) {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	default:
		r, g, b = v, p, q
	}
	return int(math.Round(r * 255)), int(math.Round(g * 255)), int(math.Round(b * 255))
}

// Contrast returns a color of text that reads on a fill: the text of the
// light theme on a light fill, and that of the dark theme on a dark fill.
// The two hold in either mode. A fill of two colors, a gradient, reads as
// the mean of its colors, by their weights. It returns empty for a fill
// that is empty, unknown or mostly transparent.
func Contrast(fill string) string {
	colors, weights, _ := ColorList(fill)
	lum := func(v string) (float64, bool) {
		c, ok := Color(v)
		if !ok {
			return 0, false
		}
		rgba, ok := color.Parse(c)
		if !ok || rgba.A < 0.5 {
			return 0, false
		}
		return rgba.Luminance(), true
	}
	l, ok := lum(colors[0])
	if !ok {
		return ""
	}
	if len(colors) >= 2 {
		l2, ok := lum(colors[1])
		if !ok {
			return ""
		}
		w := 0.5
		switch {
		case weights[0] >= 0:
			w = weights[0]
		case weights[1] >= 0:
			w = 1 - weights[1]
		}
		l = w*l + (1-w)*l2
	}
	if l > 0.35 {
		return theme.New(theme.Light, theme.Zuzu).Text
	}
	return theme.New(theme.Dark, theme.Zuzu).Text
}
