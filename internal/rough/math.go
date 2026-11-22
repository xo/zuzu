// Copied from github.com/d2lang/rough-go, file math.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.0.4, Copyright (c) 2019 Preet Shihn.

package rough

import (
	"math"
	// zuzu: math/rand is left out (README.md, change 2).
)

const twoTo31 = 2147483648.0

// jsNumberStep materializes an intermediate JavaScript Number result. Go is
// otherwise allowed to fuse adjacent floating-point operations, while the
// JavaScript evaluation model rounds after each binary operation.
//
//go:noinline
func jsNumberStep(value float64) float64 { return value }

type randomState struct {
	// Rough.js stores the constructor argument as a JavaScript number and only
	// applies Math.imul's ToInt32 coercion on the first deterministic draw.
	// Retaining the raw value matters for truthy seeds such as 0.5, 2^32, and
	// Infinity: their first draw is exactly zero, and only later draws fall
	// through to Math.random.
	seed float64
}

func newRandom(seed float64) *randomState {
	return &randomState{seed: seed}
}

func (r *randomState) next() float64 {
	// zuzu: Rough.js draws from Math.random when the seed is 0 or NaN, and
	// from the second draw on when ToInt32 of the seed is 0, as for 0.5,
	// 2^32 or Infinity. zuzu never draws from math/rand, because its output
	// is deterministic. Such a seed takes the seed 1, the seed of a chart in
	// D40. Every other seed gives the draws of Rough.js, and its state never
	// becomes 0, because 48271 is odd (README.md, change 2).
	seed := int32(jsToInt32(r.seed))
	if seed == 0 {
		seed = 1
	}
	next := int32(uint32(seed) * uint32(48271))
	r.seed = float64(next)
	return float64(uint32(next)&0x7fffffff) / twoTo31
}

func jsToInt32(v float64) int64 {
	if v == 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	t := math.Trunc(v)
	m := math.Mod(t, 4294967296)
	if m < 0 {
		m += 4294967296
	}
	if m >= 2147483648 {
		m -= 4294967296
	}
	return int64(m)
}

// zuzu: NewSeed is left out, because it returns a random seed (README.md,
// change 2).

func random(o *ResolvedOptions) float64 {
	if o.randomizer == nil {
		o.randomizer = newRandom(o.Seed)
	}
	return o.randomizer.next()
}

func jsRound(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v == 0 || math.Abs(v) >= 1<<52 {
		return v
	}
	if v >= -0.5 && v < 0 {
		return math.Copysign(0, -1)
	}
	return math.Floor(v + 0.5)
}
