// Copied from github.com/d2lang/rough-go, file path.go, at commit
// e7169d56ddc6 (D39, docs/UPSTREAM.md). Its MIT license is in
// internal/rough/LICENSE, and its notice in internal/rough/NOTICE.

// SPDX-License-Identifier: MIT
// Derived from Rough.js 4.6.6, Copyright (c) 2019 Preet Shihn.

package rough

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

type pathToken struct {
	type_ int
	text  string
}

const (
	pathTokenCommand = iota
	pathTokenNumber
	pathTokenEOD
)

var (
	pathWhitespaceRE = regexp.MustCompile(`^[ \t\r\n,]+`)
	pathCommandRE    = regexp.MustCompile(`^[aAcChHlLmMqQsStTvVzZ]`)
	pathNumberRE     = regexp.MustCompile(`^[-+]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][-+]?[0-9]+)?`)
)

func isPathTokenType(token pathToken, type_ int) bool {
	return token.type_ == type_
}

type pathSegment struct {
	key   string
	data  []float64
	point *Point
}

var pathParams = map[string]int{
	"A": 7,
	"a": 7,
	"C": 6,
	"c": 6,
	"H": 1,
	"h": 1,
	"L": 2,
	"l": 2,
	"M": 2,
	"m": 2,
	"Q": 4,
	"q": 4,
	"S": 4,
	"s": 4,
	"T": 2,
	"t": 2,
	"V": 1,
	"v": 1,
	"Z": 0,
	"z": 0,
}

type parsedPath struct {
	closedSet bool
	closed_   bool
	segments  []pathSegment
}

func newParsedPath(d string) *parsedPath {
	p := &parsedPath{}
	p.parseData(d)
	p.processPoints()
	return p
}

// tokenize uses independent local matches rather than deprecated regular
// expression statics. Its command arities match path-data-parser 0.1.0.
func (p *parsedPath) tokenize(d string) []pathToken {
	tokens := make([]pathToken, 0)
	for d != "" {
		whitespace := pathWhitespaceRE.FindString(d)
		command := pathCommandRE.FindString(d)
		number := pathNumberRE.FindString(d)
		switch {
		case whitespace != "":
			d = d[len(whitespace):]
		case command != "":
			tokens = append(tokens, pathToken{type_: pathTokenCommand, text: command})
			d = d[len(command):]
		case number != "":
			// The regular expression already establishes the grammar. Like
			// parseFloat, accept range overflow/underflow and keep its Inf/zero.
			v, _ := strconv.ParseFloat(number, 64)
			// The bundle stores `${parseFloat(match)}` and later applies unary
			// plus. This also turns a parsed negative zero into positive zero.
			tokens = append(tokens, pathToken{type_: pathTokenNumber, text: jsNumberString(v)})
			d = d[len(number):]
		default:
			return nil
		}
	}
	return append(tokens, pathToken{type_: pathTokenEOD, text: ""})
}

func (p *parsedPath) parseData(d string) {
	tokens := p.tokenize(d)
	if len(tokens) == 0 {
		p.segments = nil
		return
	}
	index := 0
	token := tokens[index]
	mode := "BOD"
	p.segments = make([]pathSegment, 0)
	for !isPathTokenType(token, pathTokenEOD) {
		paramLength := 0
		params := make([]float64, 0)
		if mode == "BOD" {
			if token.text == "M" || token.text == "m" {
				index++
				paramLength = pathParams[token.text]
				mode = token.text
			} else {
				p.parseData("M0,0" + d)
				return
			}
		} else if isPathTokenType(token, pathTokenNumber) {
			paramLength = pathParams[mode]
		} else {
			index++
			var ok bool
			paramLength, ok = pathParams[token.text]
			if !ok {
				return
			}
			mode = token.text
		}

		if index+paramLength < len(tokens) {
			for i := index; i < index+paramLength; i++ {
				numberToken := tokens[i]
				if !isPathTokenType(numberToken, pathTokenNumber) {
					return
				}
				v, _ := strconv.ParseFloat(numberToken.text, 64)
				params = append(params, v)
			}
			if _, ok := pathParams[mode]; !ok {
				return
			}
			p.segments = append(p.segments, pathSegment{key: mode, data: params})
			index += paramLength
			token = tokens[index]
			if mode == "M" {
				mode = "L"
			}
			if mode == "m" {
				mode = "l"
			}
		} else {
			// Rough.js logs and then loops forever for truncated parameter
			// groups. Returning retains the useful parsed prefix without
			// introducing a non-terminating Go API.
			return
		}
	}
}

func (p *parsedPath) closed() bool {
	if !p.closedSet {
		p.closed_ = false
		for _, segment := range p.segments {
			if strings.ToLower(segment.key) == "z" {
				p.closed_ = true
			}
		}
		p.closedSet = true
	}
	return p.closed_
}

func pointPtr(p Point) *Point {
	copy := p
	return &copy
}

func (p *parsedPath) processPoints() {
	var first *Point
	currentPoint := Point{0, 0}
	for i := range p.segments {
		segment := &p.segments[i]
		switch segment.key {
		case "M", "L", "T":
			segment.point = pointPtr(Point{segment.data[0], segment.data[1]})
		case "m", "l", "t":
			segment.point = pointPtr(Point{segment.data[0] + currentPoint[0], segment.data[1] + currentPoint[1]})
		case "H":
			segment.point = pointPtr(Point{segment.data[0], currentPoint[1]})
		case "h":
			segment.point = pointPtr(Point{segment.data[0] + currentPoint[0], currentPoint[1]})
		case "V":
			segment.point = pointPtr(Point{currentPoint[0], segment.data[0]})
		case "v":
			segment.point = pointPtr(Point{currentPoint[0], segment.data[0] + currentPoint[1]})
		case "z", "Z":
			if first != nil {
				segment.point = pointPtr(*first)
			}
		case "C":
			segment.point = pointPtr(Point{segment.data[4], segment.data[5]})
		case "c":
			segment.point = pointPtr(Point{segment.data[4] + currentPoint[0], segment.data[5] + currentPoint[1]})
		case "S":
			segment.point = pointPtr(Point{segment.data[2], segment.data[3]})
		case "s":
			segment.point = pointPtr(Point{segment.data[2] + currentPoint[0], segment.data[3] + currentPoint[1]})
		case "Q":
			segment.point = pointPtr(Point{segment.data[2], segment.data[3]})
		case "q":
			segment.point = pointPtr(Point{segment.data[2] + currentPoint[0], segment.data[3] + currentPoint[1]})
		case "A":
			segment.point = pointPtr(Point{segment.data[5], segment.data[6]})
		case "a":
			segment.point = pointPtr(Point{segment.data[5] + currentPoint[0], segment.data[6] + currentPoint[1]})
		}

		if segment.key == "m" || segment.key == "M" {
			first = nil
		}
		if segment.point != nil {
			currentPoint = *segment.point
			if first == nil {
				first = pointPtr(*segment.point)
			}
		}
		if segment.key == "z" || segment.key == "Z" {
			first = nil
		}
	}
}

type roughPath struct {
	parsed                *parsedPath
	position              Point
	first                 *Point
	linearPoints_         [][]Point
	linearPointsSet       bool
	bezierReflectionPoint *Point
	quadReflectionPoint   *Point
}

func newRoughPath(d string) *roughPath {
	return &roughPath{parsed: newParsedPath(d)}
}

func (p *roughPath) segments() []pathSegment {
	return p.parsed.segments
}

func (p *roughPath) closed() bool {
	return p.parsed.closed()
}

func (p *roughPath) linearPoints() [][]Point {
	if !p.linearPointsSet {
		linearPoints := make([][]Point, 0)
		points := make([]Point, 0)
		for _, segment := range p.parsed.segments {
			key := strings.ToLower(segment.key)
			if key == "m" || key == "z" {
				if len(points) != 0 {
					linearPoints = append(linearPoints, points)
					points = make([]Point, 0)
				}
				if key == "z" {
					continue
				}
			}
			if segment.point != nil {
				points = append(points, *segment.point)
			}
		}
		if len(points) != 0 {
			linearPoints = append(linearPoints, points)
		}
		p.linearPoints_ = linearPoints
		p.linearPointsSet = true
	}
	return p.linearPoints_
}

func (p *roughPath) setPosition(x, y float64) {
	p.position = Point{x, y}
	if p.first == nil {
		p.first = pointPtr(p.position)
	}
}

func (p *roughPath) x() float64 { return p.position[0] }

func (p *roughPath) y() float64 { return p.position[1] }

type roughArcSegment struct {
	cp1 Point
	cp2 Point
	to  Point
}

// roughArcConverter implements the SVG implementation-note conversion used by
// Rough.js to approximate elliptical arcs with cubic Bezier segments.
type roughArcConverter struct {
	segIndex float64
	numSegs  float64
	rx       float64
	ry       float64
	sinPhi   float64
	cosPhi   float64
	center   Point
	theta    float64
	delta    float64
	t        float64
	from     Point
}

func newRoughArcConverter(from, to, radii Point, angle float64, largeArcFlag, sweepFlag bool) *roughArcConverter {
	c := &roughArcConverter{from: from}
	if from[0] == to[0] && from[1] == to[1] {
		return c
	}
	radPerDeg := math.Pi / 180
	c.rx = math.Abs(radii[0])
	c.ry = math.Abs(radii[1])
	c.sinPhi = math.Sin(angle * radPerDeg)
	c.cosPhi = math.Cos(angle * radPerDeg)
	x1dash := c.cosPhi*(from[0]-to[0])/2.0 + c.sinPhi*(from[1]-to[1])/2.0
	y1dash := -c.sinPhi*(from[0]-to[0])/2.0 + c.cosPhi*(from[1]-to[1])/2.0
	root := 0.0
	numerator := c.rx*c.rx*c.ry*c.ry - c.rx*c.rx*y1dash*y1dash - c.ry*c.ry*x1dash*x1dash
	if numerator < 0 {
		s := math.Sqrt(1 - numerator/(c.rx*c.rx*c.ry*c.ry))
		c.rx *= s
		c.ry *= s
		root = 0
	} else {
		sign := 1.0
		if largeArcFlag == sweepFlag {
			sign = -1
		}
		root = sign * math.Sqrt(numerator/(c.rx*c.rx*y1dash*y1dash+c.ry*c.ry*x1dash*x1dash))
	}
	cxdash := root * c.rx * y1dash / c.ry
	cydash := -root * c.ry * x1dash / c.rx
	c.center = Point{
		c.cosPhi*cxdash - c.sinPhi*cydash + (from[0]+to[0])/2.0,
		c.sinPhi*cxdash + c.cosPhi*cydash + (from[1]+to[1])/2.0,
	}
	c.theta = c.calculateVectorAngle(1.0, 0.0, (x1dash-cxdash)/c.rx, (y1dash-cydash)/c.ry)
	dtheta := c.calculateVectorAngle(
		(x1dash-cxdash)/c.rx,
		(y1dash-cydash)/c.ry,
		(-x1dash-cxdash)/c.rx,
		(-y1dash-cydash)/c.ry,
	)
	if !sweepFlag && dtheta > 0 {
		dtheta -= 2 * math.Pi
	} else if sweepFlag && dtheta < 0 {
		dtheta += 2 * math.Pi
	}
	c.numSegs = math.Ceil(math.Abs(dtheta / (math.Pi / 2)))
	c.delta = dtheta / c.numSegs
	c.t = (8.0 / 3.0) * math.Sin(c.delta/4) * math.Sin(c.delta/4) / math.Sin(c.delta/2)
	return c
}

func (c *roughArcConverter) getNextSegment() *roughArcSegment {
	if c.segIndex == c.numSegs {
		return nil
	}
	cosTheta1 := math.Cos(c.theta)
	sinTheta1 := math.Sin(c.theta)
	theta2 := c.theta + c.delta
	cosTheta2 := math.Cos(theta2)
	sinTheta2 := math.Sin(theta2)

	to := Point{
		c.cosPhi*c.rx*cosTheta2 - c.sinPhi*c.ry*sinTheta2 + c.center[0],
		c.sinPhi*c.rx*cosTheta2 + c.cosPhi*c.ry*sinTheta2 + c.center[1],
	}
	cp1 := Point{
		c.from[0] + c.t*(-c.cosPhi*c.rx*sinTheta1-c.sinPhi*c.ry*cosTheta1),
		c.from[1] + c.t*(-c.sinPhi*c.rx*sinTheta1+c.cosPhi*c.ry*cosTheta1),
	}
	cp2 := Point{
		to[0] + c.t*(c.cosPhi*c.rx*sinTheta2+c.sinPhi*c.ry*cosTheta2),
		to[1] + c.t*(c.sinPhi*c.rx*sinTheta2-c.cosPhi*c.ry*cosTheta2),
	}

	c.theta = theta2
	c.from = Point{to[0], to[1]}
	c.segIndex++
	return &roughArcSegment{cp1: cp1, cp2: cp2, to: to}
}

func (c *roughArcConverter) calculateVectorAngle(ux, uy, vx, vy float64) float64 {
	ta := math.Atan2(uy, ux)
	tb := math.Atan2(vy, vx)
	if tb >= ta {
		return tb - ta
	}
	return 2*math.Pi - (ta - tb)
}

type pathFitter struct {
	sets   [][]Point
	closed bool
}

func newPathFitter(sets [][]Point, closed bool) *pathFitter {
	return &pathFitter{sets: sets, closed: closed}
}

func (f *pathFitter) fit(simplification float64) string {
	outSets := make([][]Point, 0)
	for _, set := range f.sets {
		length := len(set)
		estLength := math.Floor(simplification * float64(length))
		if estLength < 5 {
			if length <= 5 {
				continue
			}
			estLength = 5
		}
		outSets = append(outSets, f.reduce(set, estLength))
	}

	var d strings.Builder
	for _, set := range outSets {
		for i, point := range set {
			if i == 0 {
				d.WriteByte('M')
			} else {
				d.WriteByte('L')
			}
			d.WriteString(jsNumberString(point[0]))
			d.WriteByte(',')
			d.WriteString(jsNumberString(point[1]))
		}
		if f.closed {
			d.WriteString("z ")
		}
	}
	return d.String()
}

func sourceLineLength(line Line) float64 {
	return math.Sqrt(math.Pow(line[0][0]-line[1][0], 2) + math.Pow(line[0][1]-line[1][1], 2))
}

func (f *pathFitter) reduce(set []Point, count float64) []Point {
	if float64(len(set)) <= count {
		return set
	}
	points := append([]Point(nil), set...)
	for float64(len(points)) > count {
		areas := make([]float64, 0, len(points)-2)
		minArea := -1.0
		minIndex := -1
		for i := 1; i < len(points)-1; i++ {
			a := sourceLineLength(Line{points[i-1], points[i]})
			b := sourceLineLength(Line{points[i], points[i+1]})
			c := sourceLineLength(Line{points[i-1], points[i+1]})
			s := (a + b + c) / 2.0
			area := math.Sqrt(s * (s - a) * (s - b) * (s - c))
			areas = append(areas, area)
			if minArea < 0 || area < minArea {
				minArea = area
				minIndex = i
			}
		}
		_ = areas
		if minIndex > 0 {
			copy(points[minIndex:], points[minIndex+1:])
			points = points[:len(points)-1]
		} else {
			break
		}
	}
	return points
}
