// Copied from github.com/d2lang/rough-go, file filler_scan_line_hachure_test.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.6.6 and hachure-fill 0.5.2.
// Copyright (c) 2019, 2023 Preet Shihn.

package rough

import (
	"math"
	"reflect"
	"testing"
)

func TestStraightHachureLinesSquare(t *testing.T) {
	o := defaultResolvedOptions()
	o.HachureAngle = -90
	o.HachureGap = 4
	points := []Point{{0, 0}, {10, 0}, {10, 10}, {0, 10}}

	got := polygonHachureLines(points, &o)
	want := []Line{
		{{0, 0}, {10, 0}},
		{{0, 4}, {10, 4}},
		{{0, 8}, {10, 8}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("polygonHachureLines() = %#v, want %#v", got, want)
	}
}

func TestStraightHachureLinesUsesJSMathRound(t *testing.T) {
	o := defaultResolvedOptions()
	o.HachureAngle = -90
	o.HachureGap = 2
	points := []Point{{-1.5, 0}, {1.5, 0}, {1.5, 2}, {-1.5, 2}}

	got := polygonHachureLines(points, &o)
	want := []Line{{{-1, 0}, {2, 0}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("polygonHachureLines() = %#v, want JS Math.round result %#v", got, want)
	}
}

func TestPolygonHachureLinesRotatesOriginalPointsBackInPlace(t *testing.T) {
	o := defaultResolvedOptions()
	o.HachureAngle = -45 // Rough.js rounds this to a 45 degree working rotation.
	o.HachureGap = 1.7
	points := []Point{{1.1, 2.2}, {3.3, 4.4}, {6.6, 1.2}}
	original := append([]Point(nil), points...)

	_ = polygonHachureLines(points, &o)
	for i := range points {
		for axis := 0; axis < 2; axis++ {
			if math.Abs(points[i][axis]-original[i][axis]) > 1e-12 {
				t.Fatalf("point %d axis %d was not rotated back: got %.17g want %.17g", i, axis, points[i][axis], original[i][axis])
			}
		}
	}
	if reflect.DeepEqual(points, original) {
		t.Fatal("points were copied instead of making Rough.js's observable mutable rotation round trip")
	}
}

func TestStraightHachureLinesStableCoincidentEdgeTies(t *testing.T) {
	o := defaultResolvedOptions()
	o.HachureAngle = -90
	o.HachureGap = 2
	// The repeated first edge creates coincident active-edge x values. JS sort
	// stability preserves insertion order; this expected pairing locks it down.
	points := []Point{{0, 0}, {0, 6}, {4, 6}, {4, 0}, {0, 0}, {0, 6}, {8, 6}, {8, 0}}

	got := polygonHachureLines(points, &o)
	want := []Line{
		{{0, 0}, {0, 0}}, {{4, 0}, {8, 0}},
		{{0, 2}, {0, 2}}, {{4, 2}, {8, 2}},
		{{0, 4}, {0, 4}}, {{4, 4}, {8, 4}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("coincident-edge lines = %#v, want %#v", got, want)
	}
}

func TestPolygonHachureLinesStabilizesCollapsedArcEndpointOrdinate(t *testing.T) {
	points := []Point{
		{-12.842610978856113, -28.455601787958667},
		{3.174601744697663, -14.813785209845975},
		{1.4591047408764013, 4.404378379709259},
		{-16.79625050692646, 15.83575579257858},
		{-38.8978087263845, 11.531667740020882},
		{-49.477500810608184, -5.515086719315997},
		{-49.477500810608184, -5.515086719316038},
		{-22.327659710687822, -6.79417277395142},
	}
	o := defaultResolvedOptions()
	o.HachureAngle = -7.7643498276563605
	o.HachureGap = 6.942933222217832 + 2.514681068860853
	// Seed the skip-offset branch so this geometry regression is independent
	// of the package-level Math.random-compatible source.
	o.Seed = 1

	lines := polygonHachureLines(points, &o)
	wantLengths := []float64{0, 11, 19, 21, 43, 29}
	if len(lines) != len(wantLengths) {
		t.Fatalf("line count = %d, want %d", len(lines), len(wantLengths))
	}
	gotLengths := make([]float64, len(lines))
	for i := range lines {
		gotLengths[i] = lineLength(lines[i])
	}
	for i := range lines {
		if math.Abs(gotLengths[i]-wantLengths[i]) > 1e-12 {
			t.Fatalf("line lengths = %#v, want %#v", gotLengths, wantLengths)
		}
	}
}

func TestPolygonHachureLinesLeavesCollapsedLongEdgeHorizontal(t *testing.T) {
	points := []Point{{1000, -1000}, {980, -980}, {1000, -1100}}
	o := defaultResolvedOptions()
	o.HachureAngle = -225
	o.HachureGap = 10

	lines := polygonHachureLines(points, &o)
	if len(lines) != 8 {
		t.Fatalf("line count = %d, want 8", len(lines))
	}
	// Frozen Rough.js omits the long first edge because its rotated endpoints
	// have equal ordinates. It therefore pairs the other two edges here instead
	// of producing the zero-length line caused by over-broad stabilization.
	want := Line{{999.8489885977782, -999.8489885977782}, {980.0499987245549, -980.0499987245548}}
	for endpoint := range want {
		for axis := range want[endpoint] {
			if got := lines[0][endpoint][axis]; math.Abs(got-want[endpoint][axis]) > 1e-12 {
				t.Fatalf("first line endpoint %d axis %d = %.17g, want %.17g", endpoint, axis, got, want[endpoint][axis])
			}
		}
	}
}
