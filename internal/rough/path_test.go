// Copied from github.com/d2lang/rough-go, file path_test.go, at commit
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

func TestParsedPathSourceCommandAritiesAndPoints(t *testing.T) {
	t.Parallel()
	p := newParsedPath(`M10 20 l5 -2 H30 v4 C1 2 3 4 5 6 c1 2 3 4 5 6 S7 8 9 10 s1 2 3 4 Q11 12 13 14 q1 2 3 4 T15 16 17 18 t2 3 A4 5 30 0 1 40 50 a4 5 30 1 0 6 7 z`)
	wantKeys := []string{"M", "l", "H", "v", "C", "c", "S", "s", "Q", "q", "T", "T", "t", "A", "a", "z"}
	wantPoints := []Point{
		{10, 20}, {15, 18}, {30, 18}, {30, 22}, {5, 6},
		{10, 12}, {9, 10}, {12, 14}, {13, 14}, {16, 18},
		{15, 16}, {17, 18}, {19, 21}, {40, 50}, {46, 57}, {10, 20},
	}
	if len(p.segments) != len(wantKeys) {
		t.Fatalf("got %d segments, want %d: %#v", len(p.segments), len(wantKeys), p.segments)
	}
	for i, segment := range p.segments {
		if segment.key != wantKeys[i] {
			t.Errorf("segment %d key = %q, want %q", i, segment.key, wantKeys[i])
		}
		if segment.point == nil || *segment.point != wantPoints[i] {
			t.Errorf("segment %d point = %v, want %v", i, segment.point, wantPoints[i])
		}
	}
	if got := len(p.segments[10].data); got != 2 {
		t.Errorf("absolute T consumed %d values, want Rough.js 4.6.6 arity 2", got)
	}
	if got := len(p.segments[12].data); got != 2 {
		t.Errorf("relative t consumed %d values, want arity 2", got)
	}
	if !p.closed() {
		t.Error("closed path reported open")
	}
}

func TestParsedPathImplicitMoveAndTokenNormalization(t *testing.T) {
	t.Parallel()
	p := newParsedPath("L.5e1,-0 10. 2e1")
	if len(p.segments) != 3 {
		t.Fatalf("got %d segments, want implicit M plus two L segments", len(p.segments))
	}
	want := []pathSegment{
		{key: "M", data: []float64{0, 0}, point: pointPtr(Point{0, 0})},
		{key: "L", data: []float64{5, 0}, point: pointPtr(Point{5, 0})},
		{key: "L", data: []float64{10, 20}, point: pointPtr(Point{10, 20})},
	}
	if !reflect.DeepEqual(p.segments, want) {
		t.Fatalf("segments mismatch\n got: %#v\nwant: %#v", p.segments, want)
	}
	if math.Signbit(p.segments[1].data[1]) {
		t.Error("template-string normalization should turn parsed -0 into +0")
	}
}

func TestParsedPathParseFloatRangeBehavior(t *testing.T) {
	t.Parallel()
	p := newParsedPath("M1e999 -1e-999")
	if len(p.segments) != 1 {
		t.Fatalf("got %d segments, want 1", len(p.segments))
	}
	if !math.IsInf(p.segments[0].data[0], 1) {
		t.Errorf("overflow = %v, want +Infinity", p.segments[0].data[0])
	}
	if p.segments[0].data[1] != 0 || math.Signbit(p.segments[0].data[1]) {
		t.Errorf("underflow = %v (negative=%v), want +0 after JS string coercion", p.segments[0].data[1], math.Signbit(p.segments[0].data[1]))
	}
}

func TestRoughPathLinearPointsAndPosition(t *testing.T) {
	t.Parallel()
	p := newRoughPath("M0 0 L1 1 Z M2 2 L3 3")
	want := [][]Point{{{0, 0}, {1, 1}}, {{2, 2}, {3, 3}}}
	if got := p.linearPoints(); !reflect.DeepEqual(got, want) {
		t.Fatalf("linearPoints() = %#v, want %#v", got, want)
	}
	if !p.closed() {
		t.Error("path containing a closed subpath reported open")
	}
	p.setPosition(4, 5)
	p.setPosition(6, 7)
	if p.x() != 6 || p.y() != 7 {
		t.Fatalf("position = (%v,%v), want (6,7)", p.x(), p.y())
	}
	if p.first == nil || *p.first != (Point{4, 5}) {
		t.Fatalf("first = %v, want [4 5]", p.first)
	}
}

func TestRoughArcConverterQuarterCircle(t *testing.T) {
	t.Parallel()
	c := newRoughArcConverter(Point{1, 0}, Point{0, 1}, Point{1, 1}, 0, false, true)
	segment := c.getNextSegment()
	if segment == nil {
		t.Fatal("expected one cubic segment")
	}
	assertPointNear(t, segment.cp1, Point{1, 0.5522847498307933}, 1e-15)
	assertPointNear(t, segment.cp2, Point{0.5522847498307935, 1}, 1e-15)
	assertPointNear(t, segment.to, Point{0, 1}, 1e-15)
	if next := c.getNextSegment(); next != nil {
		t.Fatalf("unexpected second segment: %#v", next)
	}
}

func TestRoughArcConverterCoincidentEndpoints(t *testing.T) {
	t.Parallel()
	c := newRoughArcConverter(Point{3, 4}, Point{3, 4}, Point{10, 20}, 45, true, true)
	if segment := c.getNextSegment(); segment != nil {
		t.Fatalf("coincident endpoints produced segment %#v", segment)
	}
}

func TestPathFitterSourceReduction(t *testing.T) {
	t.Parallel()
	set := []Point{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}, {5, 0}}
	if got, want := newPathFitter([][]Point{set}, true).fit(0.5), "M0,0L2,0L3,0L4,0L5,0z "; got != want {
		t.Fatalf("fit() = %q, want %q", got, want)
	}
	if got := newPathFitter([][]Point{{{0, 0}, {1, 1}, {2, 2}}}, false).fit(0.1); got != "" {
		t.Fatalf("short low-estimate set should be dropped, got %q", got)
	}
}

func assertPointNear(t *testing.T, got, want Point, tolerance float64) {
	t.Helper()
	for i := range got {
		if math.Abs(got[i]-want[i]) > tolerance {
			t.Errorf("point[%d] = %.17g, want %.17g (tolerance %g)", i, got[i], want[i], tolerance)
		}
	}
}
