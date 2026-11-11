package graph

import (
	"math"
	"strconv"
	"strings"
)

// ColorList splits a list of colors of DOT, such as `a:b` or `a;0.3:b`, into
// its colors and their weights, with -1 for a color with no weight. It
// reports false for a weight that is not a number from 0 to 1.
func ColorList(v string) ([]string, []float64, bool) {
	var colors []string
	var weights []float64
	ok := true
	for part := range strings.SplitSeq(v, ":") {
		c, w, has := strings.Cut(part, ";")
		weight := -1.0
		if has {
			f, err := strconv.ParseFloat(strings.TrimSpace(w), 64)
			if err != nil || f < 0 || f > 1 {
				ok = false
			} else {
				weight = f
			}
		}
		colors, weights = append(colors, strings.TrimSpace(c)), append(weights, weight)
	}
	return colors, weights, ok
}

// Gradient returns the gradient of a fill of two colors of DOT, as the
// object of Vega: a linear gradient at an angle in degrees, counterclockwise
// from the left to the right, or a radial gradient from the center, as
// style=radial of Graphviz draws it. A radial gradient with an angle moves
// its focus toward that angle. A weight on the first color, `a;0.3:b`, or on
// the second, `a:b;0.7`, makes a linear gradient a hard change at that
// fraction. A color after the second does not draw, as in Graphviz. It
// reports false for a list of one color or a color that does not read.
func Gradient(list string, angle float64, radial bool) (map[string]any, bool) {
	colors, weights, _ := ColorList(list)
	if len(colors) < 2 {
		return nil, false
	}
	a, aok := Color(colors[0])
	b, bok := Color(colors[1])
	if !aok || !bok {
		return nil, false
	}
	stop := func(offset float64, c string) any {
		return map[string]any{"offset": round4(offset), "color": c}
	}
	rad := angle * math.Pi / 180
	cos, sin := math.Cos(rad), math.Sin(rad)
	if radial {
		fx, fy := 0.5, 0.5
		if angle != 0 {
			// zuzu reads a coordinate of 0 as the center, so the focus
			// keeps a little off the edge.
			fx, fy = math.Max(0.5*(1+cos), 0.001), math.Max(0.5*(1-sin), 0.001)
		}
		return map[string]any{
			"gradient": "radial", "x1": round4(fx), "y1": round4(fy), "r1": 0.0,
			"x2": 0.5, "y2": 0.5, "r2": 0.75, "stops": []any{stop(0, a), stop(1, b)},
		}, true
	}
	frac := 0.0
	switch {
	case weights[0] >= 0:
		frac = weights[0]
	case weights[1] >= 0:
		frac = 1 - weights[1]
	}
	stops := []any{stop(0, a), stop(1, b)}
	if frac > 0 && frac < 1 {
		stops = []any{stop(math.Max(frac-0.001, 0), a), stop(frac, b)}
	}
	return map[string]any{
		"gradient": "linear",
		"x1":       round4(0.5 - 0.5*cos), "y1": round4(0.5 + 0.5*sin),
		"x2": round4(0.5 + 0.5*cos), "y2": round4(0.5 - 0.5*sin),
		"stops": stops,
	}, true
}

// round4 rounds to four decimals, so that a gradient is the same on every
// machine.
func round4(v float64) float64 {
	return math.Round(v*1e4) / 1e4
}

// HTMLGradient is the gradient of a table or a cell of a label of HTML: its
// angle, and whether it is radial.
type HTMLGradient struct {
	Angle  float64
	Radial bool
}

// HTMLGradients reads the attribute gradientangle and the style radial of
// each <table> and each <td> of a label of HTML, which the package html of
// okage does not read. It returns the source with them blanked out, so
// that each other part keeps its line and its column, and the gradients of
// the tables and of the cells, each in the order of the source. A
// gradientangle that is not a number stays in the source.
func HTMLGradients(src string) (string, []HTMLGradient, []HTMLGradient) {
	out := []byte(src)
	blank := func(from, to int) {
		for i := from; i < to; i++ {
			out[i] = ' '
		}
	}
	var tables, cells []HTMLGradient
	for i := 0; i < len(src); i++ {
		if src[i] != '<' {
			continue
		}
		if strings.HasPrefix(src[i:], "<!--") {
			end := strings.Index(src[i:], "-->")
			if end < 0 {
				break
			}
			i += end + 2
			continue
		}
		j := i + 1
		for j < len(src) && isNameByte(src[j]) {
			j++
		}
		name := strings.ToLower(src[i+1 : j])
		var g HTMLGradient
		// The attributes, up to the end of the tag.
		for j < len(src) && src[j] != '>' {
			if !isNameByte(src[j]) {
				j++
				continue
			}
			start := j
			for j < len(src) && isNameByte(src[j]) {
				j++
			}
			attr := strings.ToLower(src[start:j])
			k := j
			for k < len(src) && isSpace(src[k]) {
				k++
			}
			if k >= len(src) || src[k] != '=' {
				continue
			}
			k++
			for k < len(src) && isSpace(src[k]) {
				k++
			}
			if k >= len(src) || src[k] != '"' && src[k] != '\'' {
				j = k
				continue
			}
			end := strings.IndexByte(src[k+1:], src[k])
			if end < 0 {
				break
			}
			vs, ve := k+1, k+1+end
			j = ve + 1
			if name != "table" && name != "td" {
				continue
			}
			switch attr {
			case "gradientangle":
				if f, err := strconv.ParseFloat(strings.TrimSpace(src[vs:ve]), 64); err == nil {
					g.Angle = f
					blank(start, j)
				}
			case "style":
				for w := vs; w < ve; {
					e := w
					for e < ve && src[e] != ',' && src[e] != ' ' {
						e++
					}
					if strings.EqualFold(src[w:e], "radial") {
						g.Radial = true
						blank(w, e)
					}
					w = e + 1
				}
			}
		}
		switch name {
		case "table":
			tables = append(tables, g)
		case "td":
			cells = append(cells, g)
		}
		i = j
	}
	return string(out), tables, cells
}

// isNameByte reports whether a byte can be part of the name of a tag or an
// attribute.
func isNameByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == ':'
}

// isSpace reports whether a byte is a space of XML.
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
