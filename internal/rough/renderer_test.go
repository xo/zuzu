// Copied from github.com/d2lang/rough-go, file renderer_test.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.6.6, Copyright (c) 2019 Preet Shihn.

package rough

import (
	"math"
	"reflect"
	"testing"
)

func rendererTestOptions(seed float64) *ResolvedOptions {
	o := defaultResolvedOptions()
	o.Seed = seed
	return &o
}

func TestRendererSeededLineMatchesRoughJS466(t *testing.T) {
	o := rendererTestOptions(1)
	got := line(0, 0, 100, 50, o)
	want := OpSet{Type: OpSetPath, Ops: []Op{
		{Op: OpMove, Data: []float64{0.8572634477168322, 0.9638847392052412}},
		{Op: OpBCurveTo, Data: []float64{
			19.265728183556348, 10.96073960699141,
			39.025807885918766, 18.06762814987451,
			99.20640534348786, 50.3923355024308,
		}},
		{Op: OpMove, Data: []float64{-0.6486655632033944, 0.26459860894829035}},
		{Op: OpBCurveTo, Data: []float64{
			22.460498989094052, 11.4495782898739,
			44.24262795830146, 21.947500267997388,
			99.41962571348995, 50.752815873362124,
		}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("seeded line differs from the Rough.js 4.6.6 oracle:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestRendererPrimitiveOperationOrder(t *testing.T) {
	tests := []struct {
		name string
		draw func(*ResolvedOptions) OpSet
		ops  []OpType
	}{
		{
			name: "line",
			draw: func(o *ResolvedOptions) OpSet { return line(0, 0, 100, 50, o) },
			ops:  []OpType{OpMove, OpBCurveTo, OpMove, OpBCurveTo},
		},
		{
			name: "linear path",
			draw: func(o *ResolvedOptions) OpSet {
				return linearPath([]Point{{0, 0}, {10, 20}, {30, 5}}, false, o)
			},
			ops: []OpType{
				OpMove, OpBCurveTo, OpMove, OpBCurveTo,
				OpMove, OpBCurveTo, OpMove, OpBCurveTo,
			},
		},
		{
			name: "rectangle",
			draw: func(o *ResolvedOptions) OpSet { return rectangle(1, 2, 20, 10, o) },
			ops: []OpType{
				OpMove, OpBCurveTo, OpMove, OpBCurveTo,
				OpMove, OpBCurveTo, OpMove, OpBCurveTo,
				OpMove, OpBCurveTo, OpMove, OpBCurveTo,
				OpMove, OpBCurveTo, OpMove, OpBCurveTo,
			},
		},
		{
			name: "curve",
			draw: func(o *ResolvedOptions) OpSet {
				return curve([]Point{{0, 0}, {10, 20}, {30, 5}, {40, 25}}, o)
			},
			ops: []OpType{
				OpMove, OpBCurveTo, OpBCurveTo, OpBCurveTo,
				OpMove, OpBCurveTo, OpBCurveTo, OpBCurveTo,
			},
		},
		{
			name: "ellipse",
			draw: func(o *ResolvedOptions) OpSet { return ellipse(10, 20, 30, 15, o) },
			ops: []OpType{
				OpMove,
				OpBCurveTo, OpBCurveTo, OpBCurveTo, OpBCurveTo, OpBCurveTo,
				OpBCurveTo, OpBCurveTo, OpBCurveTo, OpBCurveTo, OpBCurveTo,
				OpMove,
				OpBCurveTo, OpBCurveTo, OpBCurveTo, OpBCurveTo, OpBCurveTo,
				OpBCurveTo, OpBCurveTo, OpBCurveTo, OpBCurveTo, OpBCurveTo,
			},
		},
		{
			name: "closed rough arc",
			draw: func(o *ResolvedOptions) OpSet {
				return arc(10, 20, 30, 15, 0, math.Pi/2, true, true, o)
			},
			ops: []OpType{
				OpMove, OpBCurveTo, OpBCurveTo, OpBCurveTo, OpBCurveTo, OpBCurveTo,
				OpMove, OpBCurveTo, OpBCurveTo, OpBCurveTo, OpBCurveTo, OpBCurveTo,
				OpMove, OpBCurveTo, OpMove, OpBCurveTo,
				OpMove, OpBCurveTo, OpMove, OpBCurveTo,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.draw(rendererTestOptions(1))
			gotTypes := make([]OpType, len(got.Ops))
			for i := range got.Ops {
				gotTypes[i] = got.Ops[i].Op
			}
			if !reflect.DeepEqual(gotTypes, tt.ops) {
				t.Fatalf("operation order differs from Rough.js 4.6.6:\n got: %v\nwant: %v", gotTypes, tt.ops)
			}
		})
	}
}

func TestRendererOutlineThenSolidFillSharesRandomStream(t *testing.T) {
	points := []Point{{0, 0}, {10, 0}, {5, 10}}
	o := rendererTestOptions(1)
	_ = linearPath(points, true, o)
	got := solidFillPolygon(points, o)
	want := OpSet{Type: OpSetFillPath, Ops: []Op{
		{Op: OpMove, Data: []float64{-0.2638136465102434, 1.4514693040400743}},
		{Op: OpLineTo, Data: []float64{9.874775318428874, -0.7206041198223829}},
		{Op: OpLineTo, Data: []float64{4.718532053753734, 11.260766746476293}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fill did not continue the outline's random stream:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestRendererRoughnessGainDoesNotMutateOptions(t *testing.T) {
	for _, length := range []float64{199, 200, 500, 501} {
		o := rendererTestOptions(1)
		_ = roughLine(0, 0, length, 0, o, true, false)
		if o.RoughnessGain != 1 {
			t.Errorf("length %v mutated compatibility field to %v", length, o.RoughnessGain)
		}
	}
}

func TestRotatedHachureLineKeepsJavaScriptLengthBoundary(t *testing.T) {
	lines := []Line{{{-115, 189.83001211997961}, {85, 189.83001211997961}}}
	rotateLines(lines, Point{}, -66)
	if got, want := lineLength(lines[0]), 199.99999999999997; got != want {
		t.Fatalf("rotated length = %.17g, want JavaScript result %.17g", got, want)
	}

	o := rendererTestOptions(1)
	_ = doubleLineOps(lines[0][0][0], lines[0][0][1], lines[0][1][0], lines[0][1][1], o)
	if o.RoughnessGain != 1 {
		t.Fatalf("rotated length crossed Rough.js's 200 threshold: roughnessGain = %.17g, want 1", o.RoughnessGain)
	}
}

func TestRendererSVGPathCommandOrder(t *testing.T) {
	path := "M10 20 l5 6 c1 2 3 4 5 6 s7 8 9 10 q2 3 4 5 t6 7 a4 5 30 0 1 8 9 z"
	got := svgPath(path, rendererTestOptions(1))
	wantTypes := []OpType{
		OpMove, OpBCurveTo, OpMove, OpBCurveTo,
		OpMove, OpBCurveTo, OpMove, OpBCurveTo,
		OpMove, OpBCurveTo, OpMove, OpBCurveTo,
		OpMove, OpBCurveTo, OpMove, OpBCurveTo,
		OpMove, OpBCurveTo, OpMove, OpBCurveTo,
		OpMove, OpBCurveTo, OpMove, OpBCurveTo,
		OpMove, OpBCurveTo, OpMove, OpBCurveTo,
		OpMove, OpBCurveTo, OpMove, OpBCurveTo,
	}
	gotTypes := make([]OpType, len(got.Ops))
	for i := range got.Ops {
		gotTypes[i] = got.Ops[i].Op
	}
	if !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Fatalf("SVG command operation order differs from Rough.js 4.6.6:\n got: %v\nwant: %v", gotTypes, wantTypes)
	}
	if got.Ops[0].Data[0] != 10.33477207823485 || got.Ops[0].Data[1] != 20.376409023599454 {
		t.Fatalf("seeded SVG move differs from Rough.js 4.6.6: %v", got.Ops[0].Data)
	}
}

func TestJavaScriptRoundCompatibility(t *testing.T) {
	tests := []struct {
		in       float64
		want     float64
		wantSign bool
	}{
		{in: -1.5, want: -1, wantSign: true},
		{in: -0.5000000000000001, want: -1, wantSign: true},
		{in: -0.5, want: 0, wantSign: true},
		{in: -0.1, want: 0, wantSign: true},
		{in: math.Copysign(0, -1), want: 0, wantSign: true},
		{in: 0.5, want: 1},
		{in: 1.5, want: 2},
		{in: math.Inf(1), want: math.Inf(1)},
		{in: math.Inf(-1), want: math.Inf(-1), wantSign: true},
	}
	for _, tt := range tests {
		got := jsRound(tt.in)
		if got != tt.want || math.Signbit(got) != tt.wantSign {
			t.Errorf("Math.round(%v): got %v (sign=%v), want %v (sign=%v)", tt.in, got, math.Signbit(got), tt.want, tt.wantSign)
		}
	}
	if !math.IsNaN(jsRound(math.NaN())) {
		t.Error("Math.round(NaN) must remain NaN")
	}
}

func TestRandomDefersJavaScriptInt32CoercionUntilFirstDraw(t *testing.T) {
	// zuzu: a seed whose ToInt32 is 0 takes the seed 1, so its first draw is
	// the first draw of the seed 1, not 0 (README.md, change 2).
	for _, seed := range []float64{0, math.NaN(), 0.5, 1 << 32, -(1 << 32), math.Inf(1), math.Inf(-1)} {
		r := newRandom(seed)
		if got := r.next(); got != 48271.0/twoTo31 {
			t.Errorf("seed %v: first draw got %v, want %v", seed, got, 48271.0/twoTo31)
		}
		if r.seed != 48271 {
			t.Errorf("seed %v: stored seed got %v, want 48271", seed, r.seed)
		}
	}

	r := newRandom(1)
	if got := r.next(); got != 48271.0/twoTo31 {
		t.Fatalf("ordinary seeded draw got %v, want %v", got, 48271.0/twoTo31)
	}
}
