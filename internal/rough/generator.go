// Copied from github.com/d2lang/rough-go, file generator.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.6.6, Copyright (c) 2019 Preet Shihn.

package rough

import (
	"strings"
	"sync"
)

const noStyle = "none"

// Generator creates Rough.js-compatible drawing operations.
//
// A Generator retains the same mutable default-option random state as
// Rough.js. Its methods are safe for concurrent use, though callers that need
// independent deterministic streams should use separate generators.
type Generator struct {
	mu             sync.Mutex
	defaultOptions ResolvedOptions
	surface        *Surface
}

// NewGenerator creates a generator. A nil config uses Rough.js 4.6.6's
// defaults. The optional surface supplies patterned-path fallback dimensions.
func NewGenerator(config *Config, surface ...*Surface) *Generator {
	g := &Generator{defaultOptions: defaultResolvedOptions()}
	if config != nil && config.Options != nil {
		g.defaultOptions = mergeOptions(g.defaultOptions, config.Options)
	}
	if len(surface) != 0 {
		g.surface = surface[0]
	}
	return g
}

// DefaultOptions returns a value copy of the generator's current defaults.
func (g *Generator) DefaultOptions() ResolvedOptions {
	g.mu.Lock()
	defer g.mu.Unlock()
	return cloneResolvedOptions(g.defaultOptions)
}

func (g *Generator) options(options *Options) *ResolvedOptions {
	if options == nil {
		return &g.defaultOptions
	}
	o := mergeOptions(g.defaultOptions, options)
	return &o
}

func drawable(shape string, sets []OpSet, options *ResolvedOptions) Drawable {
	if sets == nil {
		sets = []OpSet{}
	}
	var snapshot *ResolvedOptions
	if options != nil {
		copy := cloneResolvedOptions(*options)
		snapshot = &copy
	}
	return Drawable{Shape: shape, Sets: sets, Options: snapshot}
}

func cloneResolvedOptions(options ResolvedOptions) ResolvedOptions {
	copy := options
	copy.StrokeLineDash = append([]float64(nil), options.StrokeLineDash...)
	copy.FillLineDash = append([]float64(nil), options.FillLineDash...)
	if options.FixedDecimalPlaceDigits != nil {
		v := *options.FixedDecimalPlaceDigits
		copy.FixedDecimalPlaceDigits = &v
	}
	if options.randomizer != nil {
		randomizer := *options.randomizer
		copy.randomizer = &randomizer
	}
	return copy
}

// Line creates a rough line.
func (g *Generator) Line(x1, y1, x2, y2 float64, options *Options) Drawable {
	g.mu.Lock()
	defer g.mu.Unlock()
	o := g.options(options)
	return drawable("line", []OpSet{line(x1, y1, x2, y2, o)}, o)
}

// Rectangle creates a rough rectangle.
func (g *Generator) Rectangle(x, y, width, height float64, options *Options) Drawable {
	g.mu.Lock()
	defer g.mu.Unlock()
	o := g.options(options)
	sets := []OpSet{}
	outline := rectangle(x, y, width, height, o)
	if o.Fill != "" {
		points := []Point{{x, y}, {x + width, y}, {x + width, y + height}, {x, y + height}}
		if o.FillStyle == "solid" {
			sets = append(sets, solidFillPolygons([][]Point{points}, o))
		} else {
			sets = append(sets, patternFillPolygon(points, o))
		}
	}
	if o.Stroke != noStyle {
		sets = append(sets, outline)
	}
	return drawable("rectangle", sets, o)
}

// Ellipse creates a rough ellipse centered at x,y.
func (g *Generator) Ellipse(x, y, width, height float64, options *Options) Drawable {
	g.mu.Lock()
	defer g.mu.Unlock()
	o := g.options(options)
	sets := []OpSet{}
	params := generateEllipseParams(width, height, o)
	response := ellipseWithParams(x, y, o, params)
	if o.Fill != "" {
		if o.FillStyle == "solid" {
			shape := ellipseWithParams(x, y, o, params).opset
			shape.Type = OpSetFillPath
			sets = append(sets, shape)
		} else {
			sets = append(sets, patternFillPolygon(response.estimatedPoints, o))
		}
	}
	if o.Stroke != noStyle {
		sets = append(sets, response.opset)
	}
	return drawable("ellipse", sets, o)
}

// Circle creates a rough circle centered at x,y.
func (g *Generator) Circle(x, y, diameter float64, options *Options) Drawable {
	d := g.Ellipse(x, y, diameter, diameter, options)
	d.Shape = "circle"
	return d
}

// LinearPath creates an open rough path through points.
func (g *Generator) LinearPath(points []Point, options *Options) Drawable {
	g.mu.Lock()
	defer g.mu.Unlock()
	o := g.options(options)
	return drawable("linearPath", []OpSet{linearPath(points, false, o)}, o)
}

// Arc creates a rough elliptical arc.
func (g *Generator) Arc(x, y, width, height, start, stop float64, closed bool, options *Options) Drawable {
	g.mu.Lock()
	defer g.mu.Unlock()
	o := g.options(options)
	sets := []OpSet{}
	outline := arc(x, y, width, height, start, stop, closed, true, o)
	if closed && o.Fill != "" {
		if o.FillStyle == "solid" {
			fillOptions := cloneResolvedOptions(*o)
			fillOptions.DisableMultiStroke = true
			shape := arc(x, y, width, height, start, stop, true, false, &fillOptions)
			shape.Type = OpSetFillPath
			sets = append(sets, shape)
		} else {
			sets = append(sets, patternFillArc(x, y, width, height, start, stop, o))
		}
	}
	if o.Stroke != noStyle {
		sets = append(sets, outline)
	}
	return drawable("arc", sets, o)
}

// Curve creates a rough curve through points.
func (g *Generator) Curve(points []Point, options *Options) Drawable {
	g.mu.Lock()
	defer g.mu.Unlock()
	o := g.options(options)
	return drawable("curve", []OpSet{curve(points, o)}, o)
}

// Polygon creates a closed rough polygon.
func (g *Generator) Polygon(points []Point, options *Options) Drawable {
	g.mu.Lock()
	defer g.mu.Unlock()
	o := g.options(options)
	sets := []OpSet{}
	outline := linearPath(points, true, o)
	if o.Fill != "" {
		if o.FillStyle == "solid" {
			sets = append(sets, solidFillPolygons([][]Point{points}, o))
		} else {
			sets = append(sets, patternFillPolygon(points, o))
		}
	}
	if o.Stroke != noStyle {
		sets = append(sets, outline)
	}
	return drawable("polygon", sets, o)
}

// Path creates a rough rendering of SVG path data.
func (g *Generator) Path(pathData string, options *Options) Drawable {
	g.mu.Lock()
	defer g.mu.Unlock()
	o := g.options(options)
	sets := []OpSet{}
	if pathData == "" {
		return drawable("path", sets, o)
	}
	pathData = normalizePathData(pathData)
	outline := svgPath(pathData, o)
	hasFill := o.Fill != "" && o.Fill != "transparent" && o.Fill != noStyle
	if hasFill {
		if o.FillStyle == "solid" {
			pathPoints := newRoughPath(pathData).linearPoints()
			if len(pathPoints) == 1 {
				fillOptions := cloneResolvedOptions(*o)
				fillOptions.DisableMultiStroke = true
				if fillOptions.Roughness != 0 {
					fillOptions.Roughness += fillOptions.FillShapeRoughnessGain
				}
				shape := svgPath(pathData, &fillOptions)
				shape.Type = OpSetFillPath
				shape.Ops = mergedShape(shape.Ops)
				sets = append(sets, shape)
			} else {
				sets = append(sets, solidFillPolygons(pathPoints, o))
			}
		} else {
			size := g.computePathSize(pathData)
			points := []Point{{0, 0}, {size[0], 0}, {size[0], size[1]}, {0, size[1]}}
			shape := patternFillPolygon(points, o)
			shape.Type = OpSetPath2DPattern
			shape.Size = &size
			shape.Path = pathData
			sets = append(sets, shape)
		}
	}
	if o.Stroke != noStyle {
		sets = append(sets, outline)
	}
	return drawable("path", sets, o)
}

func (g *Generator) computePathSize(_ string) Point {
	if g.surface != nil {
		width, height := g.surface.Width, g.surface.Height
		if width == 0 {
			width = 100
		}
		if height == 0 {
			height = 100
		}
		return Point{width, height}
	}
	return Point{100, 100}
}

// OpsToPath serializes operations using Rough.js's SVG punctuation and number
// formatting.
func (g *Generator) OpsToPath(drawing OpSet) string {
	return g.opsToPath(drawing, nil)
}

func (g *Generator) opsToPath(drawing OpSet, fixedDecimals *float64) string {
	var b strings.Builder
	for _, item := range drawing.Ops {
		d := item.Data
		if fixedDecimals != nil && *fixedDecimals >= 0 {
			d = append([]float64(nil), d...)
			for i := range d {
				d[i] = jsToFixedNumber(d[i], int(*fixedDecimals))
			}
		}
		switch item.Op {
		case OpMove:
			b.WriteString("M" + jsNumberString(d[0]) + " " + jsNumberString(d[1]) + " ")
		case OpBCurveTo:
			b.WriteString("C" + jsNumberString(d[0]) + " " + jsNumberString(d[1]) + ", " + jsNumberString(d[2]) + " " + jsNumberString(d[3]) + ", " + jsNumberString(d[4]) + " " + jsNumberString(d[5]) + " ")
		case OpLineTo:
			b.WriteString("L" + jsNumberString(d[0]) + " " + jsNumberString(d[1]) + " ")
		}
	}
	return strings.TrimSpace(b.String())
}

// ToPaths converts a Drawable into the ordered SVG path descriptions emitted
// by Rough.js's generator API.
func (g *Generator) ToPaths(d Drawable) []PathInfo {
	g.mu.Lock()
	defer g.mu.Unlock()
	o := d.Options
	if o == nil {
		o = &g.defaultOptions
	}
	paths := make([]PathInfo, 0, len(d.Sets))
	for _, drawing := range d.Sets {
		var p PathInfo
		switch drawing.Type {
		case OpSetPath:
			p = PathInfo{D: g.opsToPath(drawing, o.FixedDecimalPlaceDigits), Stroke: o.Stroke, StrokeWidth: o.StrokeWidth, Fill: noStyle}
		case OpSetFillPath:
			fill := o.Fill
			if fill == "" {
				fill = noStyle
			}
			p = PathInfo{D: g.opsToPath(drawing, o.FixedDecimalPlaceDigits), Stroke: noStyle, StrokeWidth: 0, Fill: fill}
		case OpSetFillSketch:
			p = g.fillSketch(drawing, o)
		case OpSetPath2DFill:
			fill := o.Fill
			if fill == "" {
				fill = noStyle
			}
			p = PathInfo{D: drawing.Path, Stroke: noStyle, StrokeWidth: 0, Fill: fill}
		case OpSetPath2DPattern:
			if drawing.Size == nil {
				continue
			}
			size := *drawing.Size
			pattern := PatternInfo{
				X: 0, Y: 0, Width: 1, Height: 1,
				ViewBox:      "0 0 " + jsNumberString(jsRound(size[0])) + " " + jsNumberString(jsRound(size[1])),
				PatternUnits: "objectBoundingBox",
				Path:         g.fillSketch(drawing, o),
			}
			p = PathInfo{D: drawing.Path, Stroke: noStyle, StrokeWidth: 0, Pattern: &pattern}
		default:
			continue
		}
		paths = append(paths, p)
	}
	return paths
}

func (g *Generator) fillSketch(drawing OpSet, o *ResolvedOptions) PathInfo {
	weight := o.FillWeight
	if weight < 0 {
		weight = o.StrokeWidth / 2
	}
	stroke := o.Fill
	if stroke == "" {
		stroke = noStyle
	}
	return PathInfo{D: g.opsToPath(drawing, o.FixedDecimalPlaceDigits), Stroke: stroke, StrokeWidth: weight, Fill: noStyle}
}

func normalizePathData(path string) string {
	path = strings.ReplaceAll(path, "\n", " ")
	path = pathMinusSpaceRE.ReplaceAllString(path, "-")
	return strings.Replace(path, pathLiteralSpaceBug, " ", 1)
}

func mergedShape(input []Op) []Op {
	out := make([]Op, 0, len(input))
	for i, op := range input {
		if i != 0 && op.Op == OpMove {
			continue
		}
		out = append(out, op)
	}
	return out
}
