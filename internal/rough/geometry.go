// Copied from github.com/d2lang/rough-go, file geometry.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.0.4, Copyright (c) 2019 Preet Shihn.

package rough

import "math"

func rotatePoints(points []Point, center Point, degrees float64) {
	if len(points) == 0 {
		return
	}
	cx, cy := center[0], center[1]
	angle := (math.Pi / 180) * degrees
	cos, sin := math.Cos(angle), math.Sin(angle)
	for i := range points {
		x, y := points[i][0], points[i][1]
		// JavaScript rounds each multiplication before the following add or
		// subtract. The jsNumberStep calls prevent Go from fusing these
		// expressions into FMAs, whose one-ULP difference is observable at the
		// renderer's length=200 roughness-gain boundary.
		xCos := jsNumberStep((x - cx) * cos)
		ySin := jsNumberStep((y - cy) * sin)
		xSin := jsNumberStep((x - cx) * sin)
		yCos := jsNumberStep((y - cy) * cos)
		points[i][0] = xCos - ySin + cx
		points[i][1] = xSin + yCos + cy
	}
}

func rotateLines(lines []Line, center Point, degrees float64) {
	points := make([]Point, 0, len(lines)*2)
	for i := range lines {
		points = append(points, lines[i][0], lines[i][1])
	}
	rotatePoints(points, center, degrees)
	for i := range lines {
		lines[i][0], lines[i][1] = points[i*2], points[i*2+1]
	}
}

func lineLength(line Line) float64 {
	return math.Sqrt(
		math.Pow(line[0][0]-line[1][0], 2) +
			math.Pow(line[0][1]-line[1][1], 2),
	)
}
