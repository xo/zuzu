// Copied from github.com/d2lang/rough-go, file filler_hatch.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.0.4, Copyright (c) 2019 Preet Shihn.

package rough

type hatchFiller struct {
	hachureFiller
}

func (f *hatchFiller) fillPolygon(points []Point, o *ResolvedOptions) OpSet {
	set := f.fillPolygonWithConnectEnds(points, o, false)
	o2 := *o
	o2.HachureAngle = o.HachureAngle + 90
	set2 := f.fillPolygonWithConnectEnds(points, &o2, false)
	set.Ops = append(set.Ops, set2.Ops...)
	return set
}
