package icon

import (
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/xo/zuzu/internal/shape"
)

// Shape is an icon of one color as one path, in the units of its view
// box: filled, or stroked with a width, a cap and a join.
type Shape struct {
	Path        string
	Stroke      bool
	StrokeWidth float64
	Cap, Join   string
}

// paint is the paint that an element of an icon inherits.
type paint struct {
	fill, stroke, width, linecap, linejoin string
}

// Shape returns an icon as one path when it draws in one color: each of
// its elements is a path, a circle, an ellipse, a rect, a line, a
// polyline or a polygon, in groups, with no transform, and they are all
// filled with currentColor, or all stroked with currentColor in the same
// way. It returns false for another icon, which draws as an image.
func (ic *Icon) Shape() (Shape, bool) {
	if ic.Rotate != 0 || ic.HFlip || ic.VFlip {
		return Shape{}, false
	}
	d := xml.NewDecoder(strings.NewReader("<g>" + ic.Body + "</g>"))
	d.Strict = false
	stack := []paint{{fill: "black", width: "1", linecap: "butt", linejoin: "miter"}}
	var out Shape
	var b strings.Builder
	mode := ""
	for {
		tok, err := d.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Shape{}, false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			p := stack[len(stack)-1]
			for _, a := range t.Attr {
				switch a.Name.Local {
				case "fill":
					p.fill = a.Value
				case "stroke":
					p.stroke = a.Value
				case "stroke-width":
					p.width = a.Value
				case "stroke-linecap":
					p.linecap = a.Value
				case "stroke-linejoin":
					p.linejoin = a.Value
				case "fill-rule", "clip-rule":
					if a.Value != "nonzero" {
						return Shape{}, false
					}
				case "d", "cx", "cy", "r", "rx", "ry", "x", "y", "width", "height", "x1", "y1", "x2", "y2", "points", "id", "class",
					"stroke-miterlimit":
				default:
					return Shape{}, false
				}
			}
			stack = append(stack, p)
			if t.Name.Local == "g" {
				continue
			}
			path, ok := toPath(t)
			if !ok {
				return Shape{}, false
			}
			fill := isCurrent(p.fill)
			stroke := isCurrent(p.stroke)
			switch {
			case fill && !stroke && (p.stroke == "" || p.stroke == "none"):
				if mode == "stroke" {
					return Shape{}, false
				}
				mode = "fill"
			case stroke && p.fill == "none":
				w, err := strconv.ParseFloat(p.width, 64)
				if err != nil {
					return Shape{}, false
				}
				if mode == "fill" || mode == "stroke" && (w != out.StrokeWidth || p.linecap != out.Cap || p.linejoin != out.Join) {
					return Shape{}, false
				}
				mode = "stroke"
				out.StrokeWidth, out.Cap, out.Join = w, p.linecap, p.linejoin
			case p.fill == "none" && (p.stroke == "" || p.stroke == "none"):
				// An element that draws nothing, such as the inner path of
				// an icon of Carbon, is left out.
				continue
			default:
				return Shape{}, false
			}
			b.WriteString(path)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return Shape{}, false
			}
		}
	}
	if mode == "" {
		return Shape{}, false
	}
	out.Path = b.String()
	out.Stroke = mode == "stroke"
	return out, true
}

// isCurrent reports whether a paint is currentColor.
func isCurrent(v string) bool {
	return strings.EqualFold(strings.TrimSpace(v), "currentColor")
}

// toPath returns the path data of a shape element.
func toPath(t xml.StartElement) (string, bool) {
	attr := map[string]string{}
	for _, a := range t.Attr {
		attr[a.Name.Local] = a.Value
	}
	f := func(k string) float64 {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(attr[k], "px"), 64)
		return v
	}
	switch t.Name.Local {
	case "path":
		return attr["d"], attr["d"] != ""
	case "circle":
		return ellipse(f("cx"), f("cy"), f("r"), f("r")), f("r") > 0
	case "ellipse":
		return ellipse(f("cx"), f("cy"), f("rx"), f("ry")), f("rx") > 0 && f("ry") > 0
	case "rect":
		w, h := f("width"), f("height")
		rx, ry := f("rx"), f("ry")
		if _, ok := attr["ry"]; !ok {
			ry = rx
		}
		if _, ok := attr["rx"]; !ok {
			rx = ry
		}
		rx, ry = min(rx, w/2), min(ry, h/2)
		if rx > 0 && rx == ry {
			return shape.RoundRect(f("x"), f("y"), w, h, rx, rx, rx, rx), w > 0 && h > 0
		}
		x, y := f("x"), f("y")
		return "M" + num(x) + "," + num(y) + "h" + num(w) + "v" + num(h) + "h" + num(-w) + "Z", w > 0 && h > 0 && rx == 0
	case "line":
		return "M" + num(f("x1")) + "," + num(f("y1")) + "L" + num(f("x2")) + "," + num(f("y2")), true
	case "polyline", "polygon":
		pts := strings.FieldsFunc(attr["points"], func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' })
		if len(pts) < 4 || len(pts)%2 != 0 {
			return "", false
		}
		var b strings.Builder
		for i := 0; i < len(pts); i += 2 {
			if i == 0 {
				b.WriteString("M")
			} else {
				b.WriteString("L")
			}
			b.WriteString(pts[i] + "," + pts[i+1])
		}
		if t.Name.Local == "polygon" {
			b.WriteString("Z")
		}
		return b.String(), true
	}
	return "", false
}

// ellipse returns the path of an ellipse as two arcs.
func ellipse(cx, cy, rx, ry float64) string {
	r := num(rx) + "," + num(ry) + " 0 1,0 "
	return "M" + num(cx-rx) + "," + num(cy) + "A" + r + num(cx+rx) + "," + num(cy) + "A" + r + num(cx-rx) + "," + num(cy) + "Z"
}

// Scale returns the path of a shape scaled to fit a box of a size, with
// the center of the view box at 0, 0, and the width of its stroke in the
// same scale.
func (s Shape) Scale(vb [4]float64, size float64) (string, float64, error) {
	k := size / max(vb[2], vb[3])
	cx, cy := vb[0]+vb[2]/2, vb[1]+vb[3]/2
	w := &scaler{k: k, cx: cx, cy: cy}
	if err := shape.Parse(s.Path, w); err != nil {
		return "", 0, err
	}
	return w.b.String(), s.StrokeWidth * k, nil
}

// scaler writes the segments of a path scaled about a center.
type scaler struct {
	k, cx, cy float64
	b         strings.Builder
}

func (w *scaler) MoveTo(x, y float64) { w.b.WriteString("M" + w.pt(x, y)) }
func (w *scaler) LineTo(x, y float64) { w.b.WriteString("L" + w.pt(x, y)) }
func (w *scaler) QuadTo(x1, y1, x, y float64) {
	w.b.WriteString("Q" + w.pt(x1, y1) + " " + w.pt(x, y))
}

func (w *scaler) CubicTo(x1, y1, x2, y2, x, y float64) {
	w.b.WriteString("C" + w.pt(x1, y1) + " " + w.pt(x2, y2) + " " + w.pt(x, y))
}
func (w *scaler) Close() { w.b.WriteString("Z") }

// pt writes a point scaled about the center.
func (w *scaler) pt(x, y float64) string {
	return num(round((x-w.cx)*w.k)) + "," + num(round((y-w.cy)*w.k))
}

// round rounds a coordinate to a thousandth.
func round(v float64) float64 {
	return float64(int64(v*1000+copysignHalf(v))) / 1000
}

func copysignHalf(v float64) float64 {
	if v < 0 {
		return -0.5
	}
	return 0.5
}
