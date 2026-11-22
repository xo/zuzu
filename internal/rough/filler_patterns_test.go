// Copied from github.com/d2lang/rough-go, file filler_patterns_test.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.6.6, Copyright (c) 2019 Preet Shihn.

package rough

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"reflect"
	"testing"
)

type recordedHelperCall struct {
	kind   string
	values []float64
	opts   ResolvedOptions
}

func recordingRenderHelper(calls *[]recordedHelperCall) renderHelper {
	record := func(kind string, values []float64, o *ResolvedOptions) {
		*calls = append(*calls, recordedHelperCall{kind: kind, values: append([]float64(nil), values...), opts: *o})
	}
	return renderHelper{
		randOffset: func(x float64, o *ResolvedOptions) float64 {
			record("randOffset", []float64{x}, o)
			return x
		},
		randOffsetWithRange: func(min, max float64, o *ResolvedOptions) float64 {
			record("randOffsetWithRange", []float64{min, max}, o)
			return (min + max) / 2
		},
		ellipse: func(x, y, width, height float64, o *ResolvedOptions) OpSet {
			record("ellipse", []float64{x, y, width, height}, o)
			return OpSet{Type: OpSetPath, Ops: []Op{{Op: OpMove, Data: []float64{x, y, width, height}}}}
		},
		doubleLineOps: func(x1, y1, x2, y2 float64, o *ResolvedOptions) []Op {
			record("doubleLineOps", []float64{x1, y1, x2, y2}, o)
			return []Op{{Op: OpLineTo, Data: []float64{x1, y1, x2, y2}}}
		},
	}
}

func TestHachureRenderLinesConnectsAfterEachCurrentLine(t *testing.T) {
	var calls []recordedHelperCall
	helper := recordingRenderHelper(&calls)
	filler := hachureFiller{helper: helper}
	o := defaultResolvedOptions()
	lines := []Line{{{0, 0}, {10, 0}}, {{0, 5}, {10, 5}}}

	ops := filler.renderLines(lines, &o, true)
	wantCalls := [][]float64{{0, 0, 10, 0}, {0, 5, 10, 5}, {10, 0, 0, 5}}
	if len(calls) != len(wantCalls) || len(ops) != len(wantCalls) {
		t.Fatalf("got %d calls and %d ops, want %d", len(calls), len(ops), len(wantCalls))
	}
	for i := range wantCalls {
		if calls[i].kind != "doubleLineOps" || !reflect.DeepEqual(calls[i].values, wantCalls[i]) {
			t.Fatalf("call %d = %#v, want doubleLineOps %#v", i, calls[i], wantCalls[i])
		}
	}
}

func TestDashedFillerCentersDashesAndNormalizesDirection(t *testing.T) {
	var calls []recordedHelperCall
	helper := recordingRenderHelper(&calls)
	filler := dashedFiller{helper: helper}
	o := defaultResolvedOptions()
	o.DashOffset = 4
	o.DashGap = 2

	ops := filler.dashedLine([]Line{{{20, 0}, {0, 0}}}, &o)
	want := [][]float64{{2, 0, 6, 0}, {8, 0, 12, 0}, {14, 0, 18, 0}}
	if len(ops) != len(want) || len(calls) != len(want) {
		t.Fatalf("got %d ops and %d calls, want %d", len(ops), len(calls), len(want))
	}
	for i := range want {
		assertFloatSliceClose(t, calls[i].values, want[i])
	}
}

func TestDashedFillerSnapsOneULPAtIntegerDashBoundary(t *testing.T) {
	var calls [][4]float64
	helper := renderHelper{
		doubleLineOps: func(x1, y1, x2, y2 float64, _ *ResolvedOptions) []Op {
			calls = append(calls, [4]float64{x1, y1, x2, y2})
			return nil
		},
	}
	o := defaultResolvedOptions()
	// This is the final rotated hachure line from the seeded dashed-ellipse
	// regression. Its mathematical length is 16, but Go's libm result is the
	// immediately preceding float while V8 lands on the integer boundary.
	line := Line{
		{28.948699471996157, 38.414850461521986},
		{39.445643935844274, 26.339497177957636},
	}
	if got := lineLength(line) / 8; got != math.Nextafter(2, 0) {
		t.Fatalf("regression quotient = %.17g, want the float immediately below 2", got)
	}
	(&dashedFiller{helper: helper}).dashedLine([]Line{line}, &o)
	if len(calls) != 2 {
		t.Fatalf("dash count = %d, want V8-compatible count 2", len(calls))
	}
}

func TestDotFillerPreservesRandomAndEllipseCallOrder(t *testing.T) {
	var calls []recordedHelperCall
	helper := recordingRenderHelper(&calls)
	filler := dotFiller{helper: helper}
	o := defaultResolvedOptions()
	o.HachureGap = 4
	o.FillWeight = -1
	o.StrokeWidth = 1

	set := filler.dotsOnLines([]Line{{{0, 0}, {12, 0}}}, &o)
	wantKinds := []string{
		"randOffsetWithRange", "randOffsetWithRange", "ellipse",
		"randOffsetWithRange", "randOffsetWithRange", "ellipse",
	}
	if len(calls) != len(wantKinds) || len(set.Ops) != 2 {
		t.Fatalf("got %d calls and %d ellipse ops", len(calls), len(set.Ops))
	}
	for i, kind := range wantKinds {
		if calls[i].kind != kind {
			t.Fatalf("call %d kind = %q, want %q", i, calls[i].kind, kind)
		}
	}
	assertFloatSliceClose(t, calls[0].values, []float64{-5, -3})
	assertFloatSliceClose(t, calls[1].values, []float64{-1, 1})
	assertFloatSliceClose(t, calls[2].values, []float64{-4, 0, .5, .5})
	assertFloatSliceClose(t, calls[3].values, []float64{-9, -7})
	assertFloatSliceClose(t, calls[4].values, []float64{-1, 1})
	assertFloatSliceClose(t, calls[5].values, []float64{-8, 0, .5, .5})
}

func TestDotFillerClonesAndOverridesOptions(t *testing.T) {
	var calls []recordedHelperCall
	helper := recordingRenderHelper(&calls)
	filler := dotFiller{helper: helper}
	o := defaultResolvedOptions()
	o.HachureGap = 4
	o.HachureAngle = 17
	o.Roughness = .25
	o.CurveStepCount = 11
	points := []Point{{0, 0}, {12, 0}, {12, 12}, {0, 12}}

	_ = filler.fillPolygon(points, &o)
	if o.HachureAngle != 17 || o.Roughness != .25 || o.CurveStepCount != 11 {
		t.Fatalf("source options mutated: %#v", o)
	}
	if len(calls) == 0 {
		t.Fatal("expected dot helper calls")
	}
	for i, call := range calls {
		if call.opts.HachureAngle != 0 || call.opts.Roughness != 1 || call.opts.CurveStepCount != 4 {
			t.Fatalf("call %d options = angle %v roughness %v steps %v", i, call.opts.HachureAngle, call.opts.Roughness, call.opts.CurveStepCount)
		}
	}
}

func TestZigZagLineFillerUsesJSMathRoundAndCallOrder(t *testing.T) {
	var calls []recordedHelperCall
	helper := recordingRenderHelper(&calls)
	filler := zigZagLineFiller{helper: helper}
	o := defaultResolvedOptions()

	ops := filler.zigzagLines([]Line{{{0, 0}, {10, 0}}}, 2, &o)
	// 10 / (2*2) = 2.5; JavaScript Math.round produces three teeth.
	if len(ops) != 6 || len(calls) != 6 {
		t.Fatalf("got %d ops and %d calls, want 6", len(ops), len(calls))
	}
	wantFirst := [][]float64{{0, 0, 2, 2}, {2, 2, 4, 0}}
	for i := range wantFirst {
		assertFloatSliceClose(t, calls[i].values, wantFirst[i])
	}
	assertFloatSliceClose(t, calls[4].values, []float64{8, 0, 10, 2})
	assertFloatSliceClose(t, calls[5].values, []float64{10, 2, 12, 0})
}

func TestHatchFillerClonesAngleAndKeepsInputOptions(t *testing.T) {
	var calls []recordedHelperCall
	helper := recordingRenderHelper(&calls)
	filler := hatchFiller{hachureFiller: hachureFiller{helper: helper}}
	o := defaultResolvedOptions()
	o.HachureAngle = -90
	o.HachureGap = 4
	points := []Point{{0, 0}, {10, 0}, {10, 10}, {0, 10}}

	set := filler.fillPolygon(points, &o)
	if o.HachureAngle != -90 {
		t.Fatalf("source angle mutated to %v", o.HachureAngle)
	}
	if len(set.Ops) != 6 || len(calls) != 6 {
		t.Fatalf("got %d ops and %d calls, want three lines per hatch direction", len(set.Ops), len(calls))
	}
	for i := 0; i < 3; i++ {
		if calls[i].opts.HachureAngle != -90 {
			t.Fatalf("first-pass call %d angle = %v", i, calls[i].opts.HachureAngle)
		}
	}
	for i := 3; i < 6; i++ {
		if calls[i].opts.HachureAngle != 0 {
			t.Fatalf("second-pass call %d angle = %v", i, calls[i].opts.HachureAngle)
		}
	}
}

func TestSeededD2RelevantPatternFillersMatchRoughJS466(t *testing.T) {
	tests := []struct {
		style     string
		opCount   int
		oracleSum string
	}{
		{"hachure", 12, "9835cdd5543f4b2c28b0c146871daedbe236c4c42dba6577620ac0d126673aa0"},
		{"zigzag", 24, "a4e08db1cd3e087f49077f6062faa81ffd4b4a88a29b1d6ee95f5422c091793d"},
		{"cross-hatch", 24, "9d8cf6e8e4e192d2d04210b6fbefd5299b03660795a64155720a16914b4f47a7"},
		{"dashed", 24, "57d689f796da6492e8ddcdc81e09d286c9216de1ee3a7419967ecd90bca0829e"},
		{"zigzag-line", 48, "00482911a64d629aed6cb5370edd8027864141232a1315a12ce6718412b0c003"},
	}
	for _, tt := range tests {
		t.Run(tt.style, func(t *testing.T) {
			g := NewGenerator(nil)
			drawing := g.Polygon(
				[]Point{{0, 0}, {10, 0}, {10, 10}, {0, 10}},
				&Options{
					Fill:         String("red"),
					Stroke:       String("none"),
					FillStyle:    String(tt.style),
					HachureAngle: Float64(-90),
					HachureGap:   Float64(4),
					FillWeight:   Float64(2),
					StrokeWidth:  Float64(1),
					Seed:         Float64(1),
					DashOffset:   Float64(2),
					DashGap:      Float64(2),
					ZigzagOffset: Float64(2),
				},
			)
			if len(drawing.Sets) != 1 {
				t.Fatalf("got %d sets, want one fillSketch set", len(drawing.Sets))
			}
			ops := drawing.Sets[0].Ops
			if len(ops) != tt.opCount {
				t.Fatalf("got %d ops, want %d", len(ops), tt.opCount)
			}
			if got := hashOps(ops); got != tt.oracleSum {
				preview := ops
				if len(preview) > 2 {
					preview = preview[:2]
				}
				t.Fatalf("operation stream hash = %s, want Rough.js 4.6.6 hash %s; first ops: %#v", got, tt.oracleSum, preview)
			}
		})
	}
}

func TestGetFillerSourceStyleMappingAndCache(t *testing.T) {
	fillerCache.Lock()
	savedFillers := fillerCache.fillers
	fillerCache.fillers = make(map[string]patternFiller)
	fillerCache.Unlock()
	t.Cleanup(func() {
		fillerCache.Lock()
		fillerCache.fillers = savedFillers
		fillerCache.Unlock()
	})

	var calls []recordedHelperCall
	helper := recordingRenderHelper(&calls)
	tests := []struct {
		style string
		want  any
	}{
		{"hachure", (*hachureFiller)(nil)},
		{"zigzag", (*zigZagFiller)(nil)},
		{"cross-hatch", (*hatchFiller)(nil)},
		{"dots", (*dotFiller)(nil)},
		{"dashed", (*dashedFiller)(nil)},
		{"zigzag-line", (*zigZagLineFiller)(nil)},
	}
	for _, tt := range tests {
		o := defaultResolvedOptions()
		o.FillStyle = tt.style
		got := getFiller(&o, helper)
		if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
			t.Fatalf("getFiller(%q) type = %T, want %T", tt.style, got, tt.want)
		}
		if again := getFiller(&o, helper); again != got {
			t.Fatalf("getFiller(%q) did not reuse the module cache", tt.style)
		}
	}
	unknown := defaultResolvedOptions()
	unknown.FillStyle = "unknown"
	if _, ok := getFiller(&unknown, helper).(*hachureFiller); !ok {
		t.Fatalf("unknown fill style returned %T, want hachure", getFiller(&unknown, helper))
	}
}

func assertFloatSliceClose(t *testing.T, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-12 {
			t.Fatalf("value %d = %.17g, want %.17g (all got %#v)", i, got[i], want[i], got)
		}
	}
}

func hashOps(ops []Op) string {
	// Rough.js and Go may differ in the last few binary digits of libm
	// results. Nine decimal places are much stricter than D2's six-place SVG
	// normalization while accepting those immaterial runtime differences.
	const precision = 1e9
	h := sha256.New()
	var encoded [8]byte
	for _, op := range ops {
		_, _ = h.Write([]byte(op.Op))
		_, _ = h.Write([]byte{0})
		for _, value := range op.Data {
			value = jsRound(value*precision) / precision
			binary.BigEndian.PutUint64(encoded[:], math.Float64bits(value))
			_, _ = h.Write(encoded[:])
		}
	}
	return stringHex(h.Sum(nil))
}

func stringHex(data []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(data)*2)
	for i, value := range data {
		out[i*2] = digits[value>>4]
		out[i*2+1] = digits[value&0xf]
	}
	return string(out)
}
