// Copied from github.com/d2lang/rough-go, file core.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.6.6, Copyright (c) 2019 Preet Shihn.

package rough

// Point is a two-dimensional coordinate.
type Point [2]float64

// Line is a line segment represented by its two endpoints.
type Line [2]Point

// Rectangle is an axis-aligned rectangle.
type Rectangle struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// Config configures a Generator.
type Config struct {
	Options *Options `json:"options,omitempty"`
}

// Surface supplies the fallback dimensions used for patterned SVG path fills.
type Surface struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Options contains optional Rough.js drawing options. Pointer fields preserve
// the distinction between an omitted option and an explicit zero value.
type Options struct {
	MaxRandomnessOffset     *float64  `json:"maxRandomnessOffset,omitempty"`
	Roughness               *float64  `json:"roughness,omitempty"`
	Bowing                  *float64  `json:"bowing,omitempty"`
	Stroke                  *string   `json:"stroke,omitempty"`
	StrokeWidth             *float64  `json:"strokeWidth,omitempty"`
	CurveFitting            *float64  `json:"curveFitting,omitempty"`
	CurveTightness          *float64  `json:"curveTightness,omitempty"`
	CurveStepCount          *float64  `json:"curveStepCount,omitempty"`
	Fill                    *string   `json:"fill,omitempty"`
	FillStyle               *string   `json:"fillStyle,omitempty"`
	FillWeight              *float64  `json:"fillWeight,omitempty"`
	HachureAngle            *float64  `json:"hachureAngle,omitempty"`
	HachureGap              *float64  `json:"hachureGap,omitempty"`
	Simplification          *float64  `json:"simplification,omitempty"`
	DashOffset              *float64  `json:"dashOffset,omitempty"`
	DashGap                 *float64  `json:"dashGap,omitempty"`
	ZigzagOffset            *float64  `json:"zigzagOffset,omitempty"`
	Seed                    *float64  `json:"seed,omitempty"`
	StrokeLineDash          []float64 `json:"strokeLineDash,omitempty"`
	StrokeLineDashOffset    *float64  `json:"strokeLineDashOffset,omitempty"`
	FillLineDash            []float64 `json:"fillLineDash,omitempty"`
	FillLineDashOffset      *float64  `json:"fillLineDashOffset,omitempty"`
	DisableMultiStroke      *bool     `json:"disableMultiStroke,omitempty"`
	DisableMultiStrokeFill  *bool     `json:"disableMultiStrokeFill,omitempty"`
	PreserveVertices        *bool     `json:"preserveVertices,omitempty"`
	FixedDecimalPlaceDigits *float64  `json:"fixedDecimalPlaceDigits,omitempty"`
	FillShapeRoughnessGain  *float64  `json:"fillShapeRoughnessGain,omitempty"`
	// zuzu: the fill rule of the hachure of a polygon, "nonzero" or
	// "evenodd", which is the default (README.md, change 3).
	FillRule *string `json:"fillRule,omitempty"`
}

// Float64 returns a pointer suitable for a numeric Options field.
func Float64(v float64) *float64 { return &v }

// String returns a pointer suitable for a string Options field.
func String(v string) *string { return &v }

// Bool returns a pointer suitable for a boolean Options field.
func Bool(v bool) *bool { return &v }

// ResolvedOptions is the complete option set used by the renderer.
type ResolvedOptions struct {
	MaxRandomnessOffset     float64
	Roughness               float64
	Bowing                  float64
	Stroke                  string
	StrokeWidth             float64
	CurveFitting            float64
	CurveTightness          float64
	CurveStepCount          float64
	Fill                    string
	FillStyle               string
	FillWeight              float64
	HachureAngle            float64
	HachureGap              float64
	Simplification          float64
	DashOffset              float64
	DashGap                 float64
	ZigzagOffset            float64
	Seed                    float64
	StrokeLineDash          []float64
	StrokeLineDashOffset    float64
	FillLineDash            []float64
	FillLineDashOffset      float64
	DisableMultiStroke      bool
	DisableMultiStrokeFill  bool
	PreserveVertices        bool
	FixedDecimalPlaceDigits *float64
	FillShapeRoughnessGain  float64
	// zuzu: FillRule is the fill rule of the hachure (README.md, change 3).
	FillRule string
	// RoughnessGain is retained for source compatibility with v0.1.0. Rough.js
	// 4.6.6 computes the gain per line and no longer mutates this field.
	RoughnessGain float64
	randomizer    *randomState
}

// OpType identifies a primitive path operation.
type OpType string

const (
	OpMove     OpType = "move"
	OpBCurveTo OpType = "bcurveTo"
	OpLineTo   OpType = "lineTo"
	OpQCurveTo OpType = "qcurveTo"
)

// OpSetType identifies the rendering role of an operation set.
type OpSetType string

const (
	OpSetPath          OpSetType = "path"
	OpSetFillPath      OpSetType = "fillPath"
	OpSetFillSketch    OpSetType = "fillSketch"
	OpSetPath2DFill    OpSetType = "path2Dfill"
	OpSetPath2DPattern OpSetType = "path2Dpattern"
)

// Op is one path operation and its ordered numeric arguments.
type Op struct {
	Op   OpType    `json:"op"`
	Data []float64 `json:"data"`
}

// OpSet is one ordered set of path operations.
type OpSet struct {
	Type OpSetType `json:"type"`
	Ops  []Op      `json:"ops"`
	Size *Point    `json:"size,omitempty"`
	Path string    `json:"path,omitempty"`
}

// Drawable is the source-shaped output of Generator drawing methods.
type Drawable struct {
	Shape   string
	Options *ResolvedOptions
	Sets    []OpSet
}

// PathInfo is the SVG-oriented representation of an operation set.
type PathInfo struct {
	D           string
	Stroke      string
	StrokeWidth float64
	Fill        string
	Pattern     *PatternInfo
}

// PatternInfo describes the pattern used by a patterned SVG path fill.
type PatternInfo struct {
	X            float64
	Y            float64
	Width        float64
	Height       float64
	ViewBox      string
	PatternUnits string
	Path         PathInfo
}

func defaultResolvedOptions() ResolvedOptions {
	return ResolvedOptions{
		MaxRandomnessOffset:    2,
		Roughness:              1,
		Bowing:                 1,
		Stroke:                 "#000",
		StrokeWidth:            1,
		CurveFitting:           0.95,
		CurveTightness:         0,
		CurveStepCount:         9,
		FillStyle:              "hachure",
		FillWeight:             -1,
		HachureAngle:           -41,
		HachureGap:             -1,
		DashOffset:             -1,
		DashGap:                -1,
		ZigzagOffset:           -1,
		Seed:                   0,
		DisableMultiStroke:     false,
		DisableMultiStrokeFill: false,
		PreserveVertices:       false,
		FillShapeRoughnessGain: 0.8,
		RoughnessGain:          1,
	}
}

func mergeOptions(base ResolvedOptions, o *Options) ResolvedOptions {
	if o == nil {
		return base
	}
	if o.MaxRandomnessOffset != nil {
		base.MaxRandomnessOffset = *o.MaxRandomnessOffset
	}
	if o.Roughness != nil {
		base.Roughness = *o.Roughness
	}
	if o.Bowing != nil {
		base.Bowing = *o.Bowing
	}
	if o.Stroke != nil {
		base.Stroke = *o.Stroke
	}
	if o.StrokeWidth != nil {
		base.StrokeWidth = *o.StrokeWidth
	}
	if o.CurveFitting != nil {
		base.CurveFitting = *o.CurveFitting
	}
	if o.CurveTightness != nil {
		base.CurveTightness = *o.CurveTightness
	}
	if o.CurveStepCount != nil {
		base.CurveStepCount = *o.CurveStepCount
	}
	if o.Fill != nil {
		base.Fill = *o.Fill
	}
	if o.FillStyle != nil {
		base.FillStyle = *o.FillStyle
	}
	if o.FillWeight != nil {
		base.FillWeight = *o.FillWeight
	}
	if o.HachureAngle != nil {
		base.HachureAngle = *o.HachureAngle
	}
	if o.HachureGap != nil {
		base.HachureGap = *o.HachureGap
	}
	if o.Simplification != nil {
		base.Simplification = *o.Simplification
	}
	if o.DashOffset != nil {
		base.DashOffset = *o.DashOffset
	}
	if o.DashGap != nil {
		base.DashGap = *o.DashGap
	}
	if o.ZigzagOffset != nil {
		base.ZigzagOffset = *o.ZigzagOffset
	}
	if o.Seed != nil {
		base.Seed = *o.Seed
	}
	if o.StrokeLineDash != nil {
		base.StrokeLineDash = append([]float64(nil), o.StrokeLineDash...)
	}
	if o.StrokeLineDashOffset != nil {
		base.StrokeLineDashOffset = *o.StrokeLineDashOffset
	}
	if o.FillLineDash != nil {
		base.FillLineDash = append([]float64(nil), o.FillLineDash...)
	}
	if o.FillLineDashOffset != nil {
		base.FillLineDashOffset = *o.FillLineDashOffset
	}
	if o.DisableMultiStroke != nil {
		base.DisableMultiStroke = *o.DisableMultiStroke
	}
	if o.DisableMultiStrokeFill != nil {
		base.DisableMultiStrokeFill = *o.DisableMultiStrokeFill
	}
	if o.PreserveVertices != nil {
		base.PreserveVertices = *o.PreserveVertices
	}
	if o.FixedDecimalPlaceDigits != nil {
		v := *o.FixedDecimalPlaceDigits
		base.FixedDecimalPlaceDigits = &v
	}
	if o.FillShapeRoughnessGain != nil {
		base.FillShapeRoughnessGain = *o.FillShapeRoughnessGain
	}
	// zuzu: README.md, change 3.
	if o.FillRule != nil {
		base.FillRule = *o.FillRule
	}
	return base
}
