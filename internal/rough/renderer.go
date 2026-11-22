// Copied from github.com/d2lang/rough-go, file renderer.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.6.6 and path-data-parser 0.1.0.
// Copyright (c) 2019, 2020 Preet Shihn.

package rough

import (
	"math"
	"regexp"
	"strings"
)

type ellipseParams struct {
	rx        float64
	ry        float64
	increment float64
}

type ellipseResult struct {
	opset           OpSet
	estimatedPoints []Point
}

var rendererHelper = renderHelper{
	randOffset:          randOffset,
	randOffsetWithRange: randOffsetWithRange,
	ellipse:             ellipse,
	doubleLineOps:       doubleLineOps,
}

func line(x1, y1, x2, y2 float64, o *ResolvedOptions) OpSet {
	return OpSet{Type: OpSetPath, Ops: doubleLine(x1, y1, x2, y2, o)}
}

func linearPath(points []Point, close bool, o *ResolvedOptions) OpSet {
	length := len(points)
	if length > 2 {
		ops := make([]Op, 0)
		for i := 0; i < length-1; i++ {
			ops = append(ops, doubleLine(
				points[i][0], points[i][1],
				points[i+1][0], points[i+1][1],
				o,
			)...)
		}
		if close {
			ops = append(ops, doubleLine(
				points[length-1][0], points[length-1][1],
				points[0][0], points[0][1],
				o,
			)...)
		}
		return OpSet{Type: OpSetPath, Ops: ops}
	}
	if length == 2 {
		return line(points[0][0], points[0][1], points[1][0], points[1][1], o)
	}
	return OpSet{Type: OpSetPath, Ops: []Op{}}
}

func polygon(points []Point, o *ResolvedOptions) OpSet {
	return linearPath(points, true, o)
}

func rectangle(x, y, width, height float64, o *ResolvedOptions) OpSet {
	return polygon([]Point{
		{x, y},
		{x + width, y},
		{x + width, y + height},
		{x, y + height},
	}, o)
}

func curve(points []Point, o *ResolvedOptions) OpSet {
	o1 := curveWithOffset(points, 1*(1+o.Roughness*0.2), o)
	if o.DisableMultiStroke {
		return OpSet{Type: OpSetPath, Ops: o1}
	}
	o2Options := cloneOptionsAlterSeed(o)
	o2 := curveWithOffset(points, 1.5*(1+o.Roughness*0.22), o2Options)
	return OpSet{Type: OpSetPath, Ops: append(o1, o2...)}
}

func ellipse(x, y, width, height float64, o *ResolvedOptions) OpSet {
	params := generateEllipseParams(width, height, o)
	return ellipseWithParams(x, y, o, params).opset
}

func generateEllipseParams(width, height float64, o *ResolvedOptions) ellipseParams {
	psq := math.Sqrt(math.Pi * 2 * math.Sqrt((math.Pow(width/2, 2)+math.Pow(height/2, 2))/2))
	stepCount := math.Ceil(math.Max(o.CurveStepCount, (o.CurveStepCount/math.Sqrt(200))*psq))
	increment := (math.Pi * 2) / stepCount
	rx := math.Abs(width / 2)
	ry := math.Abs(height / 2)
	curveFitRandomness := 1 - o.CurveFitting
	rx += offsetOpt(rx*curveFitRandomness, o)
	ry += offsetOpt(ry*curveFitRandomness, o)
	return ellipseParams{increment: increment, rx: rx, ry: ry}
}

func ellipseWithParams(x, y float64, o *ResolvedOptions, p ellipseParams) ellipseResult {
	ap1, cp1 := computeEllipsePoints(
		p.increment, x, y, p.rx, p.ry, 1,
		p.increment*offset(0.1, offset(0.4, 1, o), o),
		o,
	)
	o1 := roughCurve(ap1, nil, o)
	if !o.DisableMultiStroke && o.Roughness != 0 {
		ap2, _ := computeEllipsePoints(p.increment, x, y, p.rx, p.ry, 1.5, 0, o)
		o1 = append(o1, roughCurve(ap2, nil, o)...)
	}
	return ellipseResult{
		estimatedPoints: cp1,
		opset: OpSet{
			Type: OpSetPath,
			Ops:  o1,
		},
	}
}

func arc(x, y, width, height, start, stop float64, closed, roughClosure bool, o *ResolvedOptions) OpSet {
	cx, cy := x, y
	rx := math.Abs(width / 2)
	ry := math.Abs(height / 2)
	rx += offsetOpt(rx*0.01, o)
	ry += offsetOpt(ry*0.01, o)
	strt, stp := start, stop
	for strt < 0 {
		strt += math.Pi * 2
		stp += math.Pi * 2
	}
	if stp-strt > math.Pi*2 {
		strt = 0
		stp = math.Pi * 2
	}
	ellipseInc := (math.Pi * 2) / o.CurveStepCount
	arcInc := math.Min(ellipseInc/2, (stp-strt)/2)
	o1 := roughArc(arcInc, cx, cy, rx, ry, strt, stp, 1, o)
	ops := o1
	if !o.DisableMultiStroke {
		ops = append(ops, roughArc(arcInc, cx, cy, rx, ry, strt, stp, 1.5, o)...)
	}
	if closed {
		if roughClosure {
			ops = append(ops, doubleLine(
				cx, cy,
				cx+rx*math.Cos(strt), cy+ry*math.Sin(strt),
				o,
			)...)
			ops = append(ops, doubleLine(
				cx, cy,
				cx+rx*math.Cos(stp), cy+ry*math.Sin(stp),
				o,
			)...)
		} else {
			ops = append(ops,
				Op{Op: OpLineTo, Data: []float64{cx, cy}},
				Op{Op: OpLineTo, Data: []float64{cx + rx*math.Cos(strt), cy + ry*math.Sin(strt)}},
			)
		}
	}
	return OpSet{Type: OpSetPath, Ops: ops}
}

var (
	pathNewlineRE = regexp.MustCompile(`\n`)
	// ECMAScript's \s includes Unicode whitespace beyond RE2's Perl \s.
	pathMinusSpaceRE = regexp.MustCompile(`-[\t\n\v\f\r \x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`)

	pathLiteralSpaceBug = "/(ss)/g"
)

func svgPath(path string, o *ResolvedOptions) OpSet {
	p := newParsedPath(path)
	if jsTruthyNumber(o.Simplification) {
		rp := newRoughPath(path)
		fitter := newPathFitter(rp.linearPoints(), rp.closed())
		p = newParsedPath(fitter.fit(o.Simplification))
	}
	ops := make([]Op, 0)
	current, first := Point{}, Point{}
	hasFirst := false
	var previousCubicControl, previousQuadraticControl *Point
	previousKey := ""
	for _, segment := range p.segments {
		key := segment.key
		relative := key >= "a" && key <= "z"
		upper := strings.ToUpper(key)
		data := append([]float64(nil), segment.data...)
		absolutePoint := func(x, y float64) Point {
			if relative {
				return Point{x + current[0], y + current[1]}
			}
			return Point{x, y}
		}
		switch upper {
		case "M":
			current = absolutePoint(data[0], data[1])
			first, hasFirst = current, true
		case "L":
			to := absolutePoint(data[0], data[1])
			ops = append(ops, doubleLine(current[0], current[1], to[0], to[1], o)...)
			current = to
		case "H":
			x := data[0]
			if relative {
				x += current[0]
			}
			to := Point{x, current[1]}
			ops = append(ops, doubleLine(current[0], current[1], to[0], to[1], o)...)
			current = to
		case "V":
			y := data[0]
			if relative {
				y += current[1]
			}
			to := Point{current[0], y}
			ops = append(ops, doubleLine(current[0], current[1], to[0], to[1], o)...)
			current = to
		case "C":
			c1 := absolutePoint(data[0], data[1])
			c2 := absolutePoint(data[2], data[3])
			to := absolutePoint(data[4], data[5])
			ops = append(ops, bezierToCurrent(c1, c2, to, current, o)...)
			current = to
			copy := c2
			previousCubicControl = &copy
		case "S":
			c1 := current
			if (previousKey == "C" || previousKey == "S") && previousCubicControl != nil {
				c1 = Point{2*current[0] - (*previousCubicControl)[0], 2*current[1] - (*previousCubicControl)[1]}
			}
			c2 := absolutePoint(data[0], data[1])
			to := absolutePoint(data[2], data[3])
			ops = append(ops, bezierToCurrent(c1, c2, to, current, o)...)
			current = to
			copy := c2
			previousCubicControl = &copy
		case "Q":
			q := absolutePoint(data[0], data[1])
			to := absolutePoint(data[2], data[3])
			c1 := Point{current[0] + (q[0]-current[0])*2/3, current[1] + (q[1]-current[1])*2/3}
			c2 := Point{to[0] + (q[0]-to[0])*2/3, to[1] + (q[1]-to[1])*2/3}
			ops = append(ops, bezierToCurrent(c1, c2, to, current, o)...)
			current = to
			copy := q
			previousQuadraticControl = &copy
		case "T":
			q := current
			if (previousKey == "Q" || previousKey == "T") && previousQuadraticControl != nil {
				q = Point{2*current[0] - (*previousQuadraticControl)[0], 2*current[1] - (*previousQuadraticControl)[1]}
			}
			to := absolutePoint(data[0], data[1])
			c1 := Point{current[0] + (q[0]-current[0])*2/3, current[1] + (q[1]-current[1])*2/3}
			c2 := Point{to[0] + (q[0]-to[0])*2/3, to[1] + (q[1]-to[1])*2/3}
			ops = append(ops, bezierToCurrent(c1, c2, to, current, o)...)
			current = to
			copy := q
			previousQuadraticControl = &copy
		case "A":
			to := absolutePoint(data[5], data[6])
			if to != current {
				if data[0] == 0 || data[1] == 0 {
					ops = append(ops, bezierToCurrent(current, to, to, current, o)...)
				} else {
					from := current
					for _, curve := range arcToCubicCurves(current[0], current[1], to[0], to[1], math.Abs(data[0]), math.Abs(data[1]), data[2], data[3], data[4], nil) {
						ops = append(ops, bezierToCurrent(Point{curve[0], curve[1]}, Point{curve[2], curve[3]}, Point{curve[4], curve[5]}, from, o)...)
						from = Point{curve[4], curve[5]}
					}
				}
			}
			current = to
		case "Z":
			if hasFirst {
				ops = append(ops, doubleLine(current[0], current[1], first[0], first[1], o)...)
				current = first
			}
		}
		if upper != "C" && upper != "S" {
			previousCubicControl = nil
		}
		if upper != "Q" && upper != "T" {
			previousQuadraticControl = nil
		}
		previousKey = upper
	}
	return OpSet{Type: OpSetPath, Ops: ops}
}

func arcToCubicCurves(x1, y1, x2, y2, r1, r2, angle, largeArcFlag, sweepFlag float64, recursive []float64) [][6]float64 {
	angleRad := math.Pi * angle / 180
	params := make([]Point, 0)
	var f1, f2, cx, cy float64
	if recursive != nil {
		f1, f2, cx, cy = recursive[0], recursive[1], recursive[2], recursive[3]
	} else {
		x1, y1 = rotatePathPoint(x1, y1, -angleRad)
		x2, y2 = rotatePathPoint(x2, y2, -angleRad)
		x := (x1 - x2) / 2
		y := (y1 - y2) / 2
		h := x*x/(r1*r1) + y*y/(r2*r2)
		if h > 1 {
			h = math.Sqrt(h)
			r1 *= h
			r2 *= h
		}
		sign := 1.0
		if largeArcFlag == sweepFlag {
			sign = -1
		}
		r1p, r2p := r1*r1, r2*r2
		left := r1p*r2p - r1p*y*y - r2p*x*x
		right := r1p*y*y + r2p*x*x
		k := sign * math.Sqrt(math.Abs(left/right))
		cx = k*r1*y/r2 + (x1+x2)/2
		cy = k*-r2*x/r1 + (y1+y2)/2
		f1 = math.Asin(jsToFixedNumber((y1-cy)/r2, 9))
		f2 = math.Asin(jsToFixedNumber((y2-cy)/r2, 9))
		if x1 < cx {
			f1 = math.Pi - f1
		}
		if x2 < cx {
			f2 = math.Pi - f2
		}
		if f1 < 0 {
			f1 = math.Pi*2 + f1
		}
		if f2 < 0 {
			f2 = math.Pi*2 + f2
		}
		if sweepFlag != 0 && f1 > f2 {
			f1 -= math.Pi * 2
		}
		if sweepFlag == 0 && f2 > f1 {
			f2 -= math.Pi * 2
		}
	}
	df := f2 - f1
	if math.Abs(df) > math.Pi*120/180 {
		f2old, x2old, y2old := f2, x2, y2
		if sweepFlag != 0 && f2 > f1 {
			f2 = f1 + math.Pi*120/180
		} else {
			f2 = f1 - math.Pi*120/180
		}
		x2 = cx + r1*math.Cos(f2)
		y2 = cy + r2*math.Sin(f2)
		for _, curve := range arcToCubicCurves(x2, y2, x2old, y2old, r1, r2, angle, 0, sweepFlag, []float64{f2, f2old, cx, cy}) {
			params = append(params, Point{curve[0], curve[1]}, Point{curve[2], curve[3]}, Point{curve[4], curve[5]})
		}
	}
	df = f2 - f1
	c1, s1, c2, s2 := math.Cos(f1), math.Sin(f1), math.Cos(f2), math.Sin(f2)
	t := math.Tan(df / 4)
	hx, hy := 4.0/3*r1*t, 4.0/3*r2*t
	m1 := Point{x1, y1}
	m2 := Point{x1 + hx*s1, y1 - hy*c1}
	m3 := Point{x2 + hx*s2, y2 - hy*c2}
	m4 := Point{x2, y2}
	m2 = Point{2*m1[0] - m2[0], 2*m1[1] - m2[1]}
	params = append([]Point{m2, m3, m4}, params...)
	if recursive != nil {
		curves := make([][6]float64, 0, len(params)/3)
		for i := 0; i+2 < len(params); i += 3 {
			curves = append(curves, [6]float64{params[i][0], params[i][1], params[i+1][0], params[i+1][1], params[i+2][0], params[i+2][1]})
		}
		return curves
	}
	curves := make([][6]float64, 0, len(params)/3)
	for i := 0; i+2 < len(params); i += 3 {
		a, b := rotatePathPoint(params[i][0], params[i][1], angleRad)
		c, d := rotatePathPoint(params[i+1][0], params[i+1][1], angleRad)
		e, f := rotatePathPoint(params[i+2][0], params[i+2][1], angleRad)
		curves = append(curves, [6]float64{a, b, c, d, e, f})
	}
	return curves
}

func rotatePathPoint(x, y, angle float64) (float64, float64) {
	return x*math.Cos(angle) - y*math.Sin(angle), x*math.Sin(angle) + y*math.Cos(angle)
}

func bezierToCurrent(c1, c2, to, current Point, o *ResolvedOptions) []Op {
	ros0 := jsOr(o.MaxRandomnessOffset, 1)
	ros := [2]float64{ros0, ros0 + 0.3}
	iterations := 2
	if o.DisableMultiStroke {
		iterations = 1
	}
	ops := make([]Op, 0, iterations*2)
	for i := 0; i < iterations; i++ {
		move := current
		if i != 0 && !o.PreserveVertices {
			move = Point{current[0] + offsetOpt(ros[0], o), current[1] + offsetOpt(ros[0], o)}
		}
		end := to
		if !o.PreserveVertices {
			end = Point{to[0] + offsetOpt(ros[i], o), to[1] + offsetOpt(ros[i], o)}
		}
		ops = append(ops,
			Op{Op: OpMove, Data: []float64{move[0], move[1]}},
			Op{Op: OpBCurveTo, Data: []float64{
				c1[0] + offsetOpt(ros[i], o), c1[1] + offsetOpt(ros[i], o),
				c2[0] + offsetOpt(ros[i], o), c2[1] + offsetOpt(ros[i], o),
				end[0], end[1],
			}},
		)
	}
	return ops
}

func solidFillPolygon(points []Point, o *ResolvedOptions) OpSet {
	return solidFillPolygons([][]Point{points}, o)
}

func solidFillPolygons(polygonList [][]Point, o *ResolvedOptions) OpSet {
	ops := make([]Op, 0)
	for _, points := range polygonList {
		randOffset := jsOr(o.MaxRandomnessOffset, 0)
		if len(points) > 2 {
			ops = append(ops, Op{Op: OpMove, Data: []float64{
				points[0][0] + offsetOpt(randOffset, o),
				points[0][1] + offsetOpt(randOffset, o),
			}})
			for i := 1; i < len(points); i++ {
				ops = append(ops, Op{Op: OpLineTo, Data: []float64{
					points[i][0] + offsetOpt(randOffset, o),
					points[i][1] + offsetOpt(randOffset, o),
				}})
			}
		}
	}
	return OpSet{Type: OpSetFillPath, Ops: ops}
}

func patternFillPolygon(points []Point, o *ResolvedOptions) OpSet {
	return getFiller(o, rendererHelper).fillPolygon(points, o)
}

func patternFillArc(x, y, width, height, start, stop float64, o *ResolvedOptions) OpSet {
	cx, cy := x, y
	rx := math.Abs(width / 2)
	ry := math.Abs(height / 2)
	rx += offsetOpt(rx*0.01, o)
	ry += offsetOpt(ry*0.01, o)
	strt, stp := start, stop
	for strt < 0 {
		strt += math.Pi * 2
		stp += math.Pi * 2
	}
	if stp-strt > math.Pi*2 {
		strt = 0
		stp = math.Pi * 2
	}
	increment := (stp - strt) / o.CurveStepCount
	points := make([]Point, 0)
	for angle := strt; angle <= stp; angle += increment {
		points = append(points, Point{cx + rx*math.Cos(angle), cy + ry*math.Sin(angle)})
	}
	points = append(points,
		Point{cx + rx*math.Cos(stp), cy + ry*math.Sin(stp)},
		Point{cx, cy},
	)
	return patternFillPolygon(points, o)
}

func randOffset(x float64, o *ResolvedOptions) float64 {
	return offsetOpt(x, o)
}

func randOffsetWithRange(minimum, maximum float64, o *ResolvedOptions) float64 {
	return offset(minimum, maximum, o)
}

func doubleLineOps(x1, y1, x2, y2 float64, o *ResolvedOptions) []Op {
	return doubleLineFill(x1, y1, x2, y2, o)
}

func offset(minimum, maximum float64, o *ResolvedOptions, roughnessGain ...float64) float64 {
	gain := 1.0
	if len(roughnessGain) != 0 {
		gain = roughnessGain[0]
	}
	return o.Roughness * gain * (random(o)*(maximum-minimum) + minimum)
}

func offsetOpt(x float64, o *ResolvedOptions, roughnessGain ...float64) float64 {
	return offset(-x, x, o, roughnessGain...)
}

func doubleLine(x1, y1, x2, y2 float64, o *ResolvedOptions) []Op {
	return doubleLineMode(x1, y1, x2, y2, o, false)
}

func doubleLineFill(x1, y1, x2, y2 float64, o *ResolvedOptions) []Op {
	return doubleLineMode(x1, y1, x2, y2, o, true)
}

func doubleLineMode(x1, y1, x2, y2 float64, o *ResolvedOptions, filling bool) []Op {
	o1 := roughLine(x1, y1, x2, y2, o, true, false)
	if (!filling && o.DisableMultiStroke) || (filling && o.DisableMultiStrokeFill) {
		return o1
	}
	o2 := roughLine(x1, y1, x2, y2, o, true, true)
	return append(o1, o2...)
}

func roughLine(x1, y1, x2, y2 float64, o *ResolvedOptions, move, overlay bool) []Op {
	lengthSq := math.Pow(x1-x2, 2) + math.Pow(y1-y2, 2)
	length := math.Sqrt(lengthSq)
	roughnessGain := 1.0
	if length < 200 {
		roughnessGain = 1
	} else if length > 500 {
		roughnessGain = 0.4
	} else {
		// JavaScript rounds the multiplication before the addition. The
		// jsNumberStep prevents an architecture-dependent fused multiply-add
		// at the observable 200/500 length boundaries.
		scaledLength := jsNumberStep(-0.0016668 * length)
		roughnessGain = scaledLength + 1.233334
	}

	randOffset := jsOr(o.MaxRandomnessOffset, 0)
	if randOffset*randOffset*100 > lengthSq {
		randOffset = length / 10
	}
	halfOffset := randOffset / 2
	divergePoint := 0.2 + random(o)*0.2
	midDispX := o.Bowing * o.MaxRandomnessOffset * (y2 - y1) / 200
	midDispY := o.Bowing * o.MaxRandomnessOffset * (x1 - x2) / 200
	midDispX = offsetOpt(midDispX, o, roughnessGain)
	midDispY = offsetOpt(midDispY, o, roughnessGain)
	randomHalf := func() float64 { return offsetOpt(halfOffset, o, roughnessGain) }
	randomFull := func() float64 { return offsetOpt(randOffset, o, roughnessGain) }
	preserveVertices := o.PreserveVertices
	ops := make([]Op, 0, 2)
	if move {
		if overlay {
			ops = append(ops, Op{Op: OpMove, Data: []float64{
				x1 + jsConditionalFloat(preserveVertices, 0, randomHalf),
				y1 + jsConditionalFloat(preserveVertices, 0, randomHalf),
			}})
		} else {
			ops = append(ops, Op{Op: OpMove, Data: []float64{
				x1 + jsConditionalFloat(preserveVertices, 0, randomFull),
				y1 + jsConditionalFloat(preserveVertices, 0, randomFull),
			}})
		}
	}
	if overlay {
		ops = append(ops, Op{Op: OpBCurveTo, Data: []float64{
			midDispX + x1 + (x2-x1)*divergePoint + randomHalf(),
			midDispY + y1 + (y2-y1)*divergePoint + randomHalf(),
			midDispX + x1 + 2*(x2-x1)*divergePoint + randomHalf(),
			midDispY + y1 + 2*(y2-y1)*divergePoint + randomHalf(),
			x2 + jsConditionalFloat(preserveVertices, 0, randomHalf),
			y2 + jsConditionalFloat(preserveVertices, 0, randomHalf),
		}})
	} else {
		ops = append(ops, Op{Op: OpBCurveTo, Data: []float64{
			midDispX + x1 + (x2-x1)*divergePoint + randomFull(),
			midDispY + y1 + (y2-y1)*divergePoint + randomFull(),
			midDispX + x1 + 2*(x2-x1)*divergePoint + randomFull(),
			midDispY + y1 + 2*(y2-y1)*divergePoint + randomFull(),
			x2 + jsConditionalFloat(preserveVertices, 0, randomFull),
			y2 + jsConditionalFloat(preserveVertices, 0, randomFull),
		}})
	}
	return ops
}

func curveWithOffset(points []Point, randOffset float64, o *ResolvedOptions) []Op {
	if len(points) == 0 {
		return []Op{}
	}
	ps := make([]Point, 0, len(points)+2)
	ps = append(ps,
		Point{points[0][0] + offsetOpt(randOffset, o), points[0][1] + offsetOpt(randOffset, o)},
		Point{points[0][0] + offsetOpt(randOffset, o), points[0][1] + offsetOpt(randOffset, o)},
	)
	for i := 1; i < len(points); i++ {
		ps = append(ps, Point{
			points[i][0] + offsetOpt(randOffset, o),
			points[i][1] + offsetOpt(randOffset, o),
		})
		if i == len(points)-1 {
			ps = append(ps, Point{
				points[i][0] + offsetOpt(randOffset, o),
				points[i][1] + offsetOpt(randOffset, o),
			})
		}
	}
	return roughCurve(ps, nil, o)
}

func roughCurve(points []Point, closePoint *Point, o *ResolvedOptions) []Op {
	length := len(points)
	ops := make([]Op, 0)
	if length > 3 {
		s := 1 - o.CurveTightness
		ops = append(ops, Op{Op: OpMove, Data: []float64{points[1][0], points[1][1]}})
		for i := 1; i+2 < length; i++ {
			cached := points[i]
			b1 := Point{
				cached[0] + (s*points[i+1][0]-s*points[i-1][0])/6,
				cached[1] + (s*points[i+1][1]-s*points[i-1][1])/6,
			}
			b2 := Point{
				points[i+1][0] + (s*points[i][0]-s*points[i+2][0])/6,
				points[i+1][1] + (s*points[i][1]-s*points[i+2][1])/6,
			}
			b3 := points[i+1]
			ops = append(ops, Op{Op: OpBCurveTo, Data: []float64{
				b1[0], b1[1], b2[0], b2[1], b3[0], b3[1],
			}})
		}
		if closePoint != nil {
			ro := o.MaxRandomnessOffset
			ops = append(ops, Op{Op: OpLineTo, Data: []float64{
				(*closePoint)[0] + offsetOpt(ro, o),
				(*closePoint)[1] + offsetOpt(ro, o),
			}})
		}
	} else if length == 3 {
		ops = append(ops,
			Op{Op: OpMove, Data: []float64{points[1][0], points[1][1]}},
			Op{Op: OpBCurveTo, Data: []float64{
				points[1][0], points[1][1],
				points[2][0], points[2][1],
				points[2][0], points[2][1],
			}},
		)
	} else if length == 2 {
		ops = append(ops, doubleLine(
			points[0][0], points[0][1],
			points[1][0], points[1][1],
			o,
		)...)
	}
	return ops
}

func computeEllipsePoints(increment, cx, cy, rx, ry, randOffset, overlap float64, o *ResolvedOptions) ([]Point, []Point) {
	corePoints := make([]Point, 0)
	allPoints := make([]Point, 0)
	if o.Roughness == 0 {
		increment /= 4
		allPoints = append(allPoints, Point{cx + rx*math.Cos(-increment), cy + ry*math.Sin(-increment)})
		for angle := 0.0; angle <= math.Pi*2; angle += increment {
			p := Point{cx + rx*math.Cos(angle), cy + ry*math.Sin(angle)}
			corePoints = append(corePoints, p)
			allPoints = append(allPoints, p)
		}
		allPoints = append(allPoints,
			Point{cx + rx, cy},
			Point{cx + rx*math.Cos(increment), cy + ry*math.Sin(increment)},
		)
		return allPoints, corePoints
	}
	radOffset := offsetOpt(0.5, o) - math.Pi/2

	allPoints = append(allPoints, Point{
		offsetOpt(randOffset, o) + cx + 0.9*rx*math.Cos(radOffset-increment),
		offsetOpt(randOffset, o) + cy + 0.9*ry*math.Sin(radOffset-increment),
	})
	for angle := radOffset; angle < math.Pi*2+radOffset-0.01; angle += increment {
		p := Point{
			offsetOpt(randOffset, o) + cx + rx*math.Cos(angle),
			offsetOpt(randOffset, o) + cy + ry*math.Sin(angle),
		}
		corePoints = append(corePoints, p)
		allPoints = append(allPoints, p)
	}
	allPoints = append(allPoints,
		Point{
			offsetOpt(randOffset, o) + cx + rx*math.Cos(radOffset+math.Pi*2+overlap*0.5),
			offsetOpt(randOffset, o) + cy + ry*math.Sin(radOffset+math.Pi*2+overlap*0.5),
		},
		Point{
			offsetOpt(randOffset, o) + cx + 0.98*rx*math.Cos(radOffset+overlap),
			offsetOpt(randOffset, o) + cy + 0.98*ry*math.Sin(radOffset+overlap),
		},
		Point{
			offsetOpt(randOffset, o) + cx + 0.9*rx*math.Cos(radOffset+overlap*0.5),
			offsetOpt(randOffset, o) + cy + 0.9*ry*math.Sin(radOffset+overlap*0.5),
		},
	)
	return allPoints, corePoints
}

func roughArc(increment, cx, cy, rx, ry, strt, stp, randOffset float64, o *ResolvedOptions) []Op {
	radOffset := strt + offsetOpt(0.1, o)
	points := make([]Point, 0)
	points = append(points, Point{
		offsetOpt(randOffset, o) + cx + 0.9*rx*math.Cos(radOffset-increment),
		offsetOpt(randOffset, o) + cy + 0.9*ry*math.Sin(radOffset-increment),
	})
	for angle := radOffset; angle <= stp; angle += increment {
		points = append(points, Point{
			offsetOpt(randOffset, o) + cx + rx*math.Cos(angle),
			offsetOpt(randOffset, o) + cy + ry*math.Sin(angle),
		})
	}
	end := Point{cx + rx*math.Cos(stp), cy + ry*math.Sin(stp)}
	points = append(points, end, end)
	return roughCurve(points, nil, o)
}

func bezierTo(x1, y1, x2, y2, x, y float64, path *roughPath, o *ResolvedOptions) []Op {
	ros0 := jsOr(o.MaxRandomnessOffset, 1)
	ros := [2]float64{ros0, ros0 + 0.3}
	ops := make([]Op, 0, 4)
	f := Point{0, 0}
	iterations := 2
	if o.DisableMultiStroke {
		iterations = 1
	}
	for i := 0; i < iterations; i++ {
		if i == 0 {
			ops = append(ops, Op{Op: OpMove, Data: []float64{path.x(), path.y()}})
		} else {
			ops = append(ops, Op{Op: OpMove, Data: []float64{
				path.x() + jsConditionalFloat(o.PreserveVertices, 0, func() float64 { return offsetOpt(ros[0], o) }),
				path.y() + jsConditionalFloat(o.PreserveVertices, 0, func() float64 { return offsetOpt(ros[0], o) }),
			}})
		}
		if o.PreserveVertices {
			f = Point{x, y}
		} else {
			f = Point{x + offsetOpt(ros[i], o), y + offsetOpt(ros[i], o)}
		}
		ops = append(ops, Op{Op: OpBCurveTo, Data: []float64{
			x1 + offsetOpt(ros[i], o), y1 + offsetOpt(ros[i], o),
			x2 + offsetOpt(ros[i], o), y2 + offsetOpt(ros[i], o),
			f[0], f[1],
		}})
	}
	path.setPosition(f[0], f[1])
	return ops
}

func cloneOptionsAlterSeed(options *ResolvedOptions) *ResolvedOptions {
	copy := cloneResolvedOptions(*options)
	copy.randomizer = nil
	if options.Seed != 0 {
		copy.Seed = options.Seed + 1
	}
	return &copy
}

func jsConditionalFloat(condition bool, whenTrue float64, whenFalse func() float64) float64 {
	if condition {
		return whenTrue
	}
	return whenFalse()
}

func processSegment(path *roughPath, seg, prevSeg *pathSegment, o *ResolvedOptions) []Op {
	ops := make([]Op, 0)
	switch seg.key {
	case "M", "m":
		delta := seg.key == "m"
		if len(seg.data) >= 2 {
			x, y := seg.data[0], seg.data[1]
			if delta {
				x += path.x()
				y += path.y()
			}
			ro := jsOr(o.MaxRandomnessOffset, 0)
			x += offsetOpt(ro, o)
			y += offsetOpt(ro, o)
			path.setPosition(x, y)
			ops = append(ops, Op{Op: OpMove, Data: []float64{x, y}})
		}
	case "L", "l":
		delta := seg.key == "l"
		if len(seg.data) >= 2 {
			x, y := seg.data[0], seg.data[1]
			if delta {
				x += path.x()
				y += path.y()
			}
			ops = append(ops, doubleLine(path.x(), path.y(), x, y, o)...)
			path.setPosition(x, y)
		}
	case "H", "h":
		delta := seg.key == "h"
		if len(seg.data) > 0 {
			x := seg.data[0]
			if delta {
				x += path.x()
			}
			ops = append(ops, doubleLine(path.x(), path.y(), x, path.y(), o)...)
			path.setPosition(x, path.y())
		}
	case "V", "v":
		delta := seg.key == "v"
		if len(seg.data) > 0 {
			y := seg.data[0]
			if delta {
				y += path.y()
			}
			ops = append(ops, doubleLine(path.x(), path.y(), path.x(), y, o)...)
			path.setPosition(path.x(), y)
		}
	case "Z", "z":
		if path.first != nil {
			ops = append(ops, doubleLine(path.x(), path.y(), (*path.first)[0], (*path.first)[1], o)...)
			path.setPosition((*path.first)[0], (*path.first)[1])
			path.first = nil
		}
	case "C", "c":
		delta := seg.key == "c"
		if len(seg.data) >= 6 {
			x1, y1 := seg.data[0], seg.data[1]
			x2, y2 := seg.data[2], seg.data[3]
			x, y := seg.data[4], seg.data[5]
			if delta {
				x1 += path.x()
				x2 += path.x()
				x += path.x()
				y1 += path.y()
				y2 += path.y()
				y += path.y()
			}
			ops = append(ops, bezierTo(x1, y1, x2, y2, x, y, path, o)...)
			ref := Point{x + (x - x2), y + (y - y2)}
			path.bezierReflectionPoint = &ref
		}
	case "S", "s":
		delta := seg.key == "s"
		if len(seg.data) >= 4 {
			x2, y2 := seg.data[0], seg.data[1]
			x, y := seg.data[2], seg.data[3]
			if delta {
				x2 += path.x()
				x += path.x()
				y2 += path.y()
				y += path.y()
			}
			x1, y1 := x2, y2
			prevKey := ""
			if prevSeg != nil {
				prevKey = prevSeg.key
			}
			if prevKey == "c" || prevKey == "C" || prevKey == "s" || prevKey == "S" {
				if path.bezierReflectionPoint != nil {
					x1, y1 = (*path.bezierReflectionPoint)[0], (*path.bezierReflectionPoint)[1]
				}
			}
			ops = append(ops, bezierTo(x1, y1, x2, y2, x, y, path, o)...)
			ref := Point{x + (x - x2), y + (y - y2)}
			path.bezierReflectionPoint = &ref
		}
	case "Q", "q":
		delta := seg.key == "q"
		if len(seg.data) >= 4 {
			x1, y1 := seg.data[0], seg.data[1]
			x, y := seg.data[2], seg.data[3]
			if delta {
				x1 += path.x()
				x += path.x()
				y1 += path.y()
				y += path.y()
			}
			offset1 := 1 * (1 + o.Roughness*0.2)
			offset2 := 1.5 * (1 + o.Roughness*0.22)
			ops = append(ops, Op{Op: OpMove, Data: []float64{
				path.x() + offsetOpt(offset1, o), path.y() + offsetOpt(offset1, o),
			}})
			f := Point{x + offsetOpt(offset1, o), y + offsetOpt(offset1, o)}
			ops = append(ops, Op{Op: OpQCurveTo, Data: []float64{
				x1 + offsetOpt(offset1, o), y1 + offsetOpt(offset1, o), f[0], f[1],
			}})
			ops = append(ops, Op{Op: OpMove, Data: []float64{
				path.x() + offsetOpt(offset2, o), path.y() + offsetOpt(offset2, o),
			}})
			f = Point{x + offsetOpt(offset2, o), y + offsetOpt(offset2, o)}
			ops = append(ops, Op{Op: OpQCurveTo, Data: []float64{
				x1 + offsetOpt(offset2, o), y1 + offsetOpt(offset2, o), f[0], f[1],
			}})
			path.setPosition(f[0], f[1])
			ref := Point{x + (x - x1), y + (y - y1)}
			path.quadReflectionPoint = &ref
		}
	case "T", "t":
		delta := seg.key == "t"
		if len(seg.data) >= 2 {
			x, y := seg.data[0], seg.data[1]
			if delta {
				x += path.x()
				y += path.y()
			}
			x1, y1 := x, y
			prevKey := ""
			if prevSeg != nil {
				prevKey = prevSeg.key
			}
			if prevKey == "q" || prevKey == "Q" || prevKey == "t" || prevKey == "T" {
				if path.quadReflectionPoint != nil {
					x1, y1 = (*path.quadReflectionPoint)[0], (*path.quadReflectionPoint)[1]
				}
			}
			offset1 := 1 * (1 + o.Roughness*0.2)
			offset2 := 1.5 * (1 + o.Roughness*0.22)
			ops = append(ops, Op{Op: OpMove, Data: []float64{
				path.x() + offsetOpt(offset1, o), path.y() + offsetOpt(offset1, o),
			}})
			f := Point{x + offsetOpt(offset1, o), y + offsetOpt(offset1, o)}
			ops = append(ops, Op{Op: OpQCurveTo, Data: []float64{
				x1 + offsetOpt(offset1, o), y1 + offsetOpt(offset1, o), f[0], f[1],
			}})
			ops = append(ops, Op{Op: OpMove, Data: []float64{
				path.x() + offsetOpt(offset2, o), path.y() + offsetOpt(offset2, o),
			}})
			f = Point{x + offsetOpt(offset2, o), y + offsetOpt(offset2, o)}
			ops = append(ops, Op{Op: OpQCurveTo, Data: []float64{
				x1 + offsetOpt(offset2, o), y1 + offsetOpt(offset2, o), f[0], f[1],
			}})
			path.setPosition(f[0], f[1])
			ref := Point{x + (x - x1), y + (y - y1)}
			path.quadReflectionPoint = &ref
		}
	case "A", "a":
		delta := seg.key == "a"
		if len(seg.data) >= 7 {
			rx, ry := seg.data[0], seg.data[1]
			angle := seg.data[2]
			largeArcFlag := seg.data[3]
			sweepFlag := seg.data[4]
			x, y := seg.data[5], seg.data[6]
			if delta {
				x += path.x()
				y += path.y()
			}
			if x == path.x() && y == path.y() {
				break
			}
			if rx == 0 || ry == 0 {
				ops = append(ops, doubleLine(path.x(), path.y(), x, y, o)...)
				path.setPosition(x, y)
			} else {
				arcConverter := newRoughArcConverter(
					Point{path.x(), path.y()}, Point{x, y}, Point{rx, ry}, angle,
					jsTruthyNumber(largeArcFlag), jsTruthyNumber(sweepFlag),
				)
				for segment := arcConverter.getNextSegment(); segment != nil; segment = arcConverter.getNextSegment() {
					ops = append(ops, bezierTo(
						segment.cp1[0], segment.cp1[1],
						segment.cp2[0], segment.cp2[1],
						segment.to[0], segment.to[1],
						path, o,
					)...)
				}
			}
		}
	}
	return ops
}

func jsTruthyNumber(v float64) bool {
	return v != 0 && !math.IsNaN(v)
}

func jsOr(v, fallback float64) float64 {
	if jsTruthyNumber(v) {
		return v
	}
	return fallback
}
