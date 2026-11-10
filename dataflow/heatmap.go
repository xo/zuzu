package dataflow

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"math"
	"strings"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/internal/color"
)

// heatmap draws the grid of each row, such as the grid of kde2d, as an
// image: a pixel for each cell inside the margin of the grid, with a color
// and an opacity for each cell. The image is a URL of data, of a PNG. The
// color and the opacity can be expressions of the row and of the cell, as
// datum.$x, datum.$y, datum.$value and datum.$max, the greatest value of
// the grid or, with a shared resolve, of every grid. The color is gray by
// default, and the opacity the value over the greatest.
func heatmap(p *params, rows []Row) ([]Row, error) {
	field, err := p.field("field")
	if err != nil {
		return nil, err
	}
	get := func(r Row) any {
		if field == nil {
			return r
		}
		return field.Get(r)
	}
	colorOf, colorPer, err := p.cellValue("color", "#888")
	if err != nil {
		return nil, err
	}
	opacityOf, opacityPer, err := p.cellValue("opacity", nil)
	if err != nil {
		return nil, err
	}
	resolve, err := p.str("resolve", "independent")
	if err != nil {
		return nil, err
	}
	as, err := p.as("image")
	if err != nil {
		return nil, err
	}
	shared := math.NaN()
	if resolve == "shared" {
		for _, r := range rows {
			_, values, _, _, _ := gridOf(get(r))
			if m := gridMax(values); !math.IsNaN(m) && (math.IsNaN(shared) || m > shared) {
				shared = m
			}
		}
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		c := clone(r)
		grid, values, n, m, ok := gridOf(get(r))
		if !ok {
			out[i] = c
			continue
		}
		hi := shared
		if resolve != "shared" {
			hi = gridMax(values)
		}
		obj := clone(r)
		obj["$x"], obj["$y"], obj["$value"], obj["$max"] = 0.0, 0.0, 0.0, hi
		bound := func(name string, def int) int {
			v, ok := grid[name]
			if !ok || v == nil {
				return def
			}
			return int(expr.Number(v))
		}
		x1, y1, x2, y2 := bound("x1", 0), bound("y1", 0), bound("x2", n), bound("y2", m)
		if x2 <= x1 || y2 <= y1 {
			out[i] = c
			continue
		}
		img := image.NewNRGBA(image.Rect(0, 0, x2-x1, y2-y1))
		var fixedColor color.RGBA
		if !colorPer {
			fixedColor = parseColor(colorOf(obj))
		}
		fixedAlpha := 0.0
		if !opacityPer && opacityOf != nil {
			fixedAlpha = expr.Number(opacityOf(obj))
		}
		for j, k := y1, 0; j < y2; j++ {
			obj["$y"] = float64(j - y1)
			for x := x1; x < x2; x, k = x+1, k+4 {
				obj["$x"] = float64(x - x1)
				v := 0.0
				if idx := x + j*n; idx >= 0 && idx < len(values) {
					v = values[idx]
				}
				obj["$value"] = v
				col := fixedColor
				if colorPer {
					col = parseColor(colorOf(obj))
				}
				alpha := fixedAlpha
				switch {
				case opacityOf == nil:
					alpha = v / hi
					if math.IsNaN(alpha) || math.IsInf(alpha, 0) {
						alpha = 0
					}
				case opacityPer:
					alpha = expr.Number(opacityOf(obj))
				}
				img.Pix[k], img.Pix[k+1], img.Pix[k+2] = channel(col.R), channel(col.G), channel(col.B)
				img.Pix[k+3] = uint8(int32(255*alpha) & 0xff)
			}
		}
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			return nil, fmt.Errorf("heatmap: %w", err)
		}
		c[as[0]] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())
		out[i] = c
	}
	return out, nil
}

// cellValue reads a parameter of a heatmap: a constant, or an expression
// of the row and the cell. It reports whether the value depends on the
// cell, as an expression that names a field with a dollar sign does.
func (p *params) cellValue(name string, def any) (func(Row) any, bool, error) {
	v, ok, err := p.raw(name)
	if err != nil {
		return nil, false, err
	}
	if !ok || v == nil {
		if def == nil {
			return nil, false, nil
		}
		return func(Row) any { return def }, false, nil
	}
	if m, ok := v.(map[string]any); ok {
		if src, ok := m["expr"].(string); ok {
			prog, err := p.env.Compile(src)
			if err != nil {
				return nil, false, err
			}
			env := p.env
			return func(r Row) any {
				out, err := prog.Eval(env.Scope(r))
				if err != nil {
					return expr.Undefined
				}
				return out
			}, strings.Contains(src, "$"), nil
		}
	}
	return func(Row) any { return v }, false, nil
}

// parseColor reads a color of CSS, or black.
func parseColor(v any) color.RGBA {
	c, _ := color.Parse(expr.String(v))
	return c
}

// channel rounds a channel of a color to a byte.
func channel(f float64) uint8 {
	return uint8(math.Max(0, math.Min(255, math.Round(f))))
}
