package graph

import "math"

// The shapes of Mermaid that draw a figure of a fixed size with the label
// under it: the events, the gateway and the data of BPMN.
const (
	bpmnStart        = "bpmn-start"
	bpmnIntermediate = "bpmn-intermediate"
	bpmnBoundary     = "bpmn-boundary"
	bpmnEnd          = "bpmn-end"
	bpmnGateway      = "bpmn-gateway"
	bpmnData         = "bpmn-data"
	bpmnDataStore    = "bpmn-data-store"
)

// person is the shape of a person of Mermaid: a round head over a body
// with round corners, with the label in the body.
const person = "person"

// The sizes of the figures of BPMN, as Mermaid 12.3 draws them: the
// diameter of an event, the size of a gateway, of a data object and of a
// data store, and the gap between a figure and the label under it.
const (
	bpmnEventSize   = 36.0
	bpmnGatewaySize = 50.0
	bpmnDataWidth   = 36.0
	bpmnDataHeight  = 50.0
	bpmnStoreSize   = 50.0
	bpmnLabelGap    = 6.0
)

// figureSize returns the size of the figure of a shape that puts its label
// under the figure, and false for any other shape.
func figureSize(kind string) (float64, float64, bool) {
	switch kind {
	case bpmnStart, bpmnIntermediate, bpmnBoundary, bpmnEnd:
		return bpmnEventSize, bpmnEventSize, true
	case bpmnGateway:
		return bpmnGatewaySize, bpmnGatewaySize, true
	case bpmnData:
		return bpmnDataWidth, bpmnDataHeight, true
	case bpmnDataStore:
		return bpmnStoreSize, bpmnStoreSize, true
	}
	return 0, 0, false
}

// LabelBelow reports whether a shape puts its label under its figure, on
// the background, and not in its fill.
func LabelBelow(kind string) bool {
	_, _, ok := figureSize(kind)
	return ok
}

// figureNodeSize returns the size of a node of a shape with its label under
// its figure, around a label of the size tw, th. As in Mermaid, the node
// keeps a band of the height of the label and the gap above the figure
// too, so that the figure is in the middle of the node, and it is as wide
// as the wider of the figure and the label. A node with no label is the
// size of its figure.
func figureNodeSize(fw, fh, tw, th float64) (float64, float64) {
	if tw <= 0 || th <= 0 {
		return fw, fh
	}
	return math.Max(fw, tw), fh + 2*(bpmnLabelGap+th)
}

// figureTextOffset returns how far down the center of the label of a node
// of a height goes, under a figure of the height fh.
func figureTextOffset(fh, h float64) float64 {
	if h <= fh {
		return 0
	}
	th := (h-fh)/2 - bpmnLabelGap
	return fh/2 + bpmnLabelGap + th/2
}

// figurePath returns the path of the figure of a shape with its label
// under it, around 0, 0, and false for any other shape.
func figurePath(kind string) (string, bool) {
	switch kind {
	case bpmnStart, bpmnEnd:
		return circlePath(0, 0, bpmnEventSize/2), true
	case bpmnIntermediate, bpmnBoundary:
		// Two rings, 3 apart, as Mermaid draws an intermediate and a
		// boundary event.
		return circlePath(0, 0, bpmnEventSize/2) + circlePath(0, 0, bpmnEventSize/2-3), true
	case bpmnGateway:
		h := bpmnGatewaySize / 2
		return poly([2]float64{0, -h}, [2]float64{h, 0}, [2]float64{0, h}, [2]float64{-h, 0}), true
	case bpmnData:
		// A page with its top right corner folded down.
		const fold = 10.0
		x, y := bpmnDataWidth/2, bpmnDataHeight/2
		return poly([2]float64{-x, -y}, [2]float64{x - fold, -y}, [2]float64{x, -y + fold}, [2]float64{x, y}, [2]float64{-x, y}) +
			stroke([2]float64{x - fold, -y}, [2]float64{x - fold, -y + fold}, [2]float64{x, -y + fold}), true
	case bpmnDataStore:
		// A cylinder with three lines under its top, 5 apart. Each line
		// goes there and back, so that it holds no area.
		const ry = 6.0
		h := bpmnStoreSize / 2
		var p pen
		p.m(-h, -h+ry).a(h, ry, 1, h, -h+ry).v(h-ry).a(h, ry, 1, -h, h-ry).z()
		for _, off := range []float64{0, 5, 10} {
			y := -h + ry + off
			p.m(-h, y).a(h, ry, 0, h, y).a(h, ry, 1, -h, y)
		}
		return p.String(), true
	}
	return "", false
}

// figureOutline returns the half size of the box or the radius that an
// edge of a shape with its label under it ends on, and the kind of that
// outline: an event ends on its ring, a gateway on its diamond and the
// data on their boxes, and not on the box of the label.
func figureOutline(kind string) (string, float64, float64) {
	switch kind {
	case bpmnStart, bpmnIntermediate, bpmnBoundary, bpmnEnd:
		return "circle", bpmnEventSize / 2, bpmnEventSize / 2
	case bpmnGateway:
		return "diamond", bpmnGatewaySize / 2, bpmnGatewaySize / 2
	case bpmnData:
		return "rect", bpmnDataWidth / 2, bpmnDataHeight / 2
	}
	return "rect", bpmnStoreSize / 2, bpmnStoreSize / 2
}

// personHead returns the radius of the head of a person of a width, as
// Mermaid 12.3 draws it: 0.23 of the width, from 16 to 56.
func personHead(w float64) float64 {
	return math.Min(math.Max(w*0.23, 16), 56)
}

// personOverlap returns how far the head of a radius reaches into the
// body.
func personOverlap(r float64) float64 { return r * 0.27 }

// personSize returns the size of a person around a body of the size w, h:
// the body at least 100 wide, and the head over it.
func personSize(w, h float64) (float64, float64) {
	w = math.Max(w, 100)
	r := personHead(w)
	return w, h + 2*r - personOverlap(r)
}

// personShape holds the parts of a person of a size: the radius of its
// head and the y of its center, the top of its body and the radius of the
// corners of its body.
type personShape struct {
	r, headY, bodyTop, corner float64
}

// personParts returns the parts of a person of a size.
func personParts(w, h float64) personShape {
	r := personHead(w)
	top := -h / 2
	bodyTop := top + 2*r - personOverlap(r)
	return personShape{r: r, headY: top + r, bodyTop: bodyTop, corner: math.Min(w*0.177, (h/2-bodyTop)*0.45)}
}

// personTextOffset returns how far down the center of the label of a
// person of a size goes, to the middle of its body.
func personTextOffset(w, h float64) float64 {
	return (personParts(w, h).bodyTop + h/2) / 2
}

// personPath returns the path of a person of a size around 0, 0: the body
// with an opening under the head, and the whole head, so that the outline
// of the head shows over the body and the top of the body does not show in
// the head, as in Mermaid.
func personPath(w, h float64) string {
	x, y := w/2, h/2
	ps := personParts(w, h)
	r, headY, bodyTop, c := ps.r, ps.headY, ps.bodyTop, ps.corner
	xi := math.Sqrt(math.Max(r*r-(bodyTop-headY)*(bodyTop-headY), 0))
	var p pen
	p.m(xi, bodyTop).h(x-c).a(c, c, 1, x, bodyTop+c).v(y-c).a(c, c, 1, x-c, y).h(-x+c).a(c, c, 1, -x, y-c).v(bodyTop+c).a(c, c, 1, -x+c, bodyTop).h(-xi)
	return p.String() + circlePath(0, headY, r)
}

// personPolygon returns the outline of a person of a size as a polygon
// around 0, 0: the arc of the head over the body, then the body.
func personPolygon(w, h float64) [][2]float64 {
	x, y := w/2, h/2
	ps := personParts(w, h)
	r, headY, bodyTop, c := ps.r, ps.headY, ps.bodyTop, ps.corner
	var pts [][2]float64
	// The head from where it meets the top of the body on the right, over
	// its top, to where it meets the top of the body on the left.
	a0 := math.Asin(math.Min(1, (bodyTop-headY)/r))
	const steps = 24
	for i := range steps + 1 {
		a := a0 - (math.Pi+2*a0)*float64(i)/steps
		pts = append(pts, [2]float64{r * math.Cos(a), headY + r*math.Sin(a)})
	}
	// Each corner of the body turns a quarter, the same way as the head.
	corner := func(cx, cy, from float64) {
		const n = 6
		for i := range n + 1 {
			a := from - math.Pi/2*float64(i)/n
			pts = append(pts, [2]float64{cx + c*math.Cos(a), cy + c*math.Sin(a)})
		}
	}
	corner(-x+c, bodyTop+c, -math.Pi/2)
	corner(-x+c, y-c, math.Pi)
	corner(x-c, y-c, math.Pi/2)
	corner(x-c, bodyTop+c, 0)
	return pts
}
