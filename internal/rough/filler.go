// Copied from github.com/d2lang/rough-go, file filler.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.0.4, Copyright (c) 2019 Preet Shihn.

package rough

import "sync"

// patternFiller is the source-shaped counterpart of Rough.js's PatternFiller.
type patternFiller interface {
	fillPolygon(points []Point, o *ResolvedOptions) OpSet
}

// renderHelper is the narrow portion of renderer.go used by the pattern
// fillers. Rough.js represents this helper as an object of function fields;
// preserving that shape also makes filler behavior independently testable.
type renderHelper struct {
	randOffset          func(x float64, o *ResolvedOptions) float64
	randOffsetWithRange func(min, max float64, o *ResolvedOptions) float64
	ellipse             func(x, y, width, height float64, o *ResolvedOptions) OpSet
	doubleLineOps       func(x1, y1, x2, y2 float64, o *ResolvedOptions) []Op
}

var fillerCache = struct {
	sync.Mutex
	fillers map[string]patternFiller
}{fillers: make(map[string]patternFiller)}

// getFiller mirrors Rough.js's module-level filler cache. renderer.go always
// supplies the same helper; the lock makes first use safe across concurrent
// generators without changing the source cache semantics.
func getFiller(o *ResolvedOptions, helper renderHelper) patternFiller {
	fillerName := o.FillStyle
	if fillerName == "" {
		fillerName = "hachure"
	}

	fillerCache.Lock()
	defer fillerCache.Unlock()
	if filler := fillerCache.fillers[fillerName]; filler != nil {
		return filler
	}

	var filler patternFiller
	switch fillerName {
	case "zigzag":
		filler = &zigZagFiller{hachureFiller: hachureFiller{helper: helper}}
	case "cross-hatch":
		filler = &hatchFiller{hachureFiller: hachureFiller{helper: helper}}
	case "dots":
		filler = &dotFiller{helper: helper}
	case "dashed":
		filler = &dashedFiller{helper: helper}
	case "zigzag-line":
		filler = &zigZagLineFiller{helper: helper}
	case "hachure":
		filler = &hachureFiller{helper: helper}
	default:
		fillerName = "hachure"
		if filler = fillerCache.fillers[fillerName]; filler != nil {
			return filler
		}
		filler = &hachureFiller{helper: helper}
	}
	fillerCache.fillers[fillerName] = filler
	return filler
}
