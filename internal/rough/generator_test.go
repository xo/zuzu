// Copied from github.com/d2lang/rough-go, file generator_test.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.6.6, Copyright (c) 2019 Preet Shihn.

package rough

import (
	"reflect"
	"sync"
	"testing"
)

func TestRoughJS466OptionsResolveAndClone(t *testing.T) {
	strokeDash := []float64{3, 4}
	fillDash := []float64{5, 6}
	g := NewGenerator(&Config{Options: &Options{
		StrokeLineDash:          strokeDash,
		StrokeLineDashOffset:    Float64(7),
		FillLineDash:            fillDash,
		FillLineDashOffset:      Float64(8),
		DisableMultiStroke:      Bool(true),
		DisableMultiStrokeFill:  Bool(true),
		PreserveVertices:        Bool(true),
		FixedDecimalPlaceDigits: Float64(2),
		FillShapeRoughnessGain:  Float64(0.6),
	}})
	strokeDash[0], fillDash[0] = 99, 99
	o := g.DefaultOptions()
	if !reflect.DeepEqual(o.StrokeLineDash, []float64{3, 4}) || !reflect.DeepEqual(o.FillLineDash, []float64{5, 6}) {
		t.Fatalf("dash options were not cloned: stroke=%v fill=%v", o.StrokeLineDash, o.FillLineDash)
	}
	if o.StrokeLineDashOffset != 7 || o.FillLineDashOffset != 8 || !o.DisableMultiStroke || !o.DisableMultiStrokeFill || !o.PreserveVertices || o.FillShapeRoughnessGain != 0.6 {
		t.Fatalf("Rough.js 4.6.6 options were not resolved: %#v", o)
	}
	if o.FixedDecimalPlaceDigits == nil || *o.FixedDecimalPlaceDigits != 2 {
		t.Fatalf("fixed decimal digits = %v, want 2", o.FixedDecimalPlaceDigits)
	}
	d := g.Line(0.123, 0.456, 10.789, 20.987, nil)
	if got := len(d.Sets[0].Ops); got != 2 {
		t.Fatalf("disableMultiStroke emitted %d operations, want 2", got)
	}
	if got := g.ToPaths(d)[0].D; got == "" {
		t.Fatal("fixed-decimal serialization returned an empty path")
	}
}

func TestPathNormalizesActualNewlines(t *testing.T) {
	g1 := NewGenerator(nil)
	g2 := NewGenerator(nil)
	o := &Options{Seed: Float64(17)}
	withNewline := g1.Path("M0 0\nL10 10", o)
	withSpace := g2.Path("M0 0 L10 10", o)
	if !reflect.DeepEqual(withNewline, withSpace) {
		t.Fatal("actual SVG path newline was not normalized like Rough.js 4.6.6")
	}
}

func TestDrawableOptionsAreDetachedFromGeneratorState(t *testing.T) {
	g := NewGenerator(nil)
	d := g.Line(0, 0, 10, 0, nil)
	if d.Options == nil || d.Options.randomizer == nil {
		t.Fatal("completed drawable is missing its resolved random state")
	}
	wantGain := d.Options.RoughnessGain
	wantSeed := d.Options.randomizer.seed
	if d.Options == &g.defaultOptions || d.Options.randomizer == g.defaultOptions.randomizer {
		t.Fatal("completed drawable aliases mutable generator state")
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 1000; i++ {
			if d.Options.RoughnessGain != wantGain || d.Options.randomizer.seed != wantSeed {
				t.Errorf("completed drawable options changed while generator was reused")
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 1000; i++ {
			_ = g.Line(0, 0, float64(100+i), 0, nil)
		}
	}()
	close(start)
	wg.Wait()
}
