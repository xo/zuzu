// Package layout draws the guides of a chart in the look of zuzu: the axes,
// the legends and the titles (D12). It also measures the bounds of a scene,
// which the autosize of a spec reads. The rules of the look are in
// docs/STYLE.md. The view resolves each guide from the spec, and this
// package draws it.
package layout

import (
	"math"

	"github.com/xo/zuzu/internal/shape"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/theme"
)

// The sizes of the type of docs/STYLE.md.
const (
	// LabelSize is the size of a label of an axis or a legend.
	LabelSize = 11
	// SmallSize is the size of the title of an axis or a legend.
	SmallSize = 10
	// TitleSize is the size of the title of a chart.
	TitleSize = 16
	// SubtitleSize is the size of a subtitle.
	SubtitleSize = 12.5
	// LabelPad is the space between a label of an axis and the plot area.
	LabelPad = 8
	// MaxLabels is the most labels that an axis of numbers has.
	MaxLabels = 6
)

// Style is what a guide draws with: the colors of the theme, the font and
// the measure of text.
type Style struct {
	Theme   theme.Theme
	Font    string
	Measure text.Measurer
}

// itoa writes a small integer.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// Bounds is a box: the least and the greatest x and y.
type Bounds struct {
	X1, Y1, X2, Y2 float64
}

// Empty returns bounds that hold nothing, which any point grows.
func Empty() Bounds {
	return Bounds{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
}

// IsEmpty reports whether the bounds hold nothing.
func (b Bounds) IsEmpty() bool {
	return b.X1 > b.X2 || b.Y1 > b.Y2
}

// add grows the bounds to hold a point.
func (b *Bounds) add(x, y float64) {
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		return
	}
	b.X1, b.X2 = math.Min(b.X1, x), math.Max(b.X2, x)
	b.Y1, b.Y2 = math.Min(b.Y1, y), math.Max(b.Y2, y)
}

// union grows the bounds to hold others.
func (b *Bounds) union(o Bounds) {
	if o.IsEmpty() {
		return
	}
	b.add(o.X1, o.Y1)
	b.add(o.X2, o.Y2)
}

// MarkBounds returns the bounds of the items of a mark, in the coordinates
// of the group that holds it.
func (s Style) MarkBounds(m *scene.Mark) Bounds {
	b := Empty()
	for i := range m.Items {
		b.union(s.itemBounds(m.Type, &m.Items[i]))
	}
	return b
}

// ItemBounds returns the bounds of an item of a kind of mark, in the
// coordinates of the group that holds it.
func (s Style) ItemBounds(typ string, it *scene.Item) Bounds {
	return s.itemBounds(typ, it)
}

// TextSize returns the width and the height of the text of an item, as the
// writer draws it.
func (s Style) TextSize(it *scene.Item) (float64, float64) {
	t := *it
	t.X, t.Y, t.Dx, t.Dy, t.Angle, t.Align, t.Baseline = 0, 0, 0, 0, 0, "left", "top"
	b := s.textBounds(&t)
	if b.IsEmpty() {
		return 0, 0
	}
	return b.X2 - b.X1, b.Y2 - b.Y1
}

// font returns the font of a label at a size and a weight.
func (s Style) font(size float64, weight int) text.Font {
	return text.Font{Family: text.Family, Size: size, Weight: weight}
}

// width measures a label.
func (s Style) width(size float64, weight int, str string) float64 {
	if s.Measure == nil {
		return float64(len([]rune(str))) * size * 0.55
	}
	return s.Measure.Width(s.font(size, weight), str)
}

// label returns a text item.
func (s Style) label(x, y float64, str string, size float64, weight int, color, align, baseline string) scene.Item {
	it := scene.Item{
		X: x, Y: y, Text: scene.Lines{str}, Font: s.Font, FontSize: size,
		Fill: scene.Paint{Color: color}, Align: align, Baseline: baseline,
	}
	if weight != 400 {
		it.FontWeight = scene.Weight(itoa(weight))
	}
	return it
}

// itemBounds returns the bounds of an item.
func (s Style) itemBounds(typ string, it *scene.Item) Bounds {
	b := Empty()
	switch typ {
	case scene.Group:
		b.add(it.X, it.Y)
		b.add(it.X+it.Width, it.Y+it.Height)
		if it.Clip {
			// A group that clips draws nothing outside its box, as Vega
			// bounds it.
			return b
		}
		for i := range it.Items {
			if it.Items[i].Clip {
				continue
			}
			c := s.MarkBounds(&it.Items[i])
			if !c.IsEmpty() {
				b.add(it.X+c.X1, it.Y+c.Y1)
				b.add(it.X+c.X2, it.Y+c.Y2)
			}
		}
	case scene.Rect, scene.Image:
		b.add(it.X, it.Y)
		b.add(it.X+it.Width, it.Y+it.Height)
		if it.Width == 0 && it.X2 != 0 {
			b.add(it.X2, it.Y)
		}
		if it.Height == 0 && it.Y2 != 0 {
			b.add(it.X, it.Y2)
		}
	case scene.Rule:
		b.add(it.X, it.Y)
		b.add(it.X2, it.Y2)
	case scene.Line, scene.Trail, scene.Area:
		if it.Defined != nil && !*it.Defined {
			// A point that is not defined is not drawn, and Vega leaves
			// it out of the bounds.
			return b
		}
		b.add(it.X, it.Y)
		if typ == scene.Area {
			if it.Orient == "horizontal" {
				b.add(it.X2, it.Y)
			} else {
				b.add(it.X, it.Y2)
			}
		}
	case scene.Symbol:
		r := 4.0
		if it.Size != nil {
			r = math.Sqrt(math.Max(0, *it.Size)) / 2
		}
		b.add(it.X-r, it.Y-r)
		b.add(it.X+r, it.Y+r)
	case scene.Arc:
		r := math.Max(it.OuterRadius, it.InnerRadius)
		b.add(it.X-r, it.Y-r)
		b.add(it.X+r, it.Y+r)
	case scene.Text:
		b.union(s.textBounds(it))
	case scene.Path, scene.Shape:
		b.add(it.X, it.Y)
		if it.Path != "" {
			pathBounds(&b, it)
		}
	}
	return b
}

// pathSink adds the points of a path to bounds, after the rotation and the
// move of its item. A curve adds points along it, not its control points,
// which lie outside the curve.
type pathSink struct {
	b              *Bounds
	x, y           float64
	sin, cos       float64
	curX, curY     float64
	startX, startY float64
}

// pathSteps is the count of points that a curve adds.
const pathSteps = 16

func (p *pathSink) MoveTo(x, y float64) {
	p.startX, p.startY = x, y
	p.add(x, y)
}

func (p *pathSink) LineTo(x, y float64) { p.add(x, y) }

func (p *pathSink) QuadTo(x1, y1, x, y float64) {
	x0, y0 := p.curX, p.curY
	for i := 1; i <= pathSteps; i++ {
		t := float64(i) / pathSteps
		u := 1 - t
		p.add(u*u*x0+2*u*t*x1+t*t*x, u*u*y0+2*u*t*y1+t*t*y)
	}
}

func (p *pathSink) CubicTo(x1, y1, x2, y2, x, y float64) {
	x0, y0 := p.curX, p.curY
	for i := 1; i <= pathSteps; i++ {
		t := float64(i) / pathSteps
		u := 1 - t
		a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
		p.add(a*x0+b*x1+c*x2+d*x, a*y0+b*y1+c*y2+d*y)
	}
}

func (p *pathSink) Close() { p.curX, p.curY = p.startX, p.startY }

// add adds a point of the path, and makes it the current point.
func (p *pathSink) add(x, y float64) {
	p.curX, p.curY = x, y
	p.b.add(p.x+x*p.cos-y*p.sin, p.y+x*p.sin+y*p.cos)
}

// pathBounds adds the bounds of the path of an item. Path data that does not
// read adds the segments before the error, as SVG draws them.
func pathBounds(b *Bounds, it *scene.Item) {
	sin, cos := math.Sincos(it.Angle * math.Pi / 180)
	_ = shape.Parse(it.Path, &pathSink{b: b, x: it.X, y: it.Y, sin: sin, cos: cos})
}

// textBounds returns the bounds of a text item, from the estimate of its
// width, its align, its baseline and its angle.
func (s Style) textBounds(it *scene.Item) Bounds {
	size := it.FontSize
	if size == 0 {
		size = LabelSize
	}
	w := 0.0
	for _, l := range it.Text {
		w = math.Max(w, s.width(size, text.Weight(string(it.FontWeight)), l))
	}
	if it.Limit > 0 {
		// A line longer than its limit is cut to it.
		w = math.Min(w, it.Limit)
	}
	h := size * float64(max(1, len(it.Text)))
	x0 := 0.0
	switch it.Align {
	case "center":
		x0 = -w / 2
	case "right":
		x0 = -w
	}
	y0 := -size * 0.8
	switch it.Baseline {
	case "top":
		y0 = 0
	case "middle":
		y0 = -h / 2
	case "bottom", "line-bottom":
		y0 = -h
	}
	x0 += it.Dx
	y0 += it.Dy
	corners := [][2]float64{{x0, y0}, {x0 + w, y0}, {x0, y0 + h}, {x0 + w, y0 + h}}
	a := it.Angle * math.Pi / 180
	sin, cos := math.Sin(a), math.Cos(a)
	b := Empty()
	for _, c := range corners {
		b.add(it.X+c[0]*cos-c[1]*sin, it.Y+c[0]*sin+c[1]*cos)
	}
	return b
}
