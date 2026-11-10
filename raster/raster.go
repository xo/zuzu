// Package raster draws a scene graph as an image with the drawing of okage
// directly, without the round trip through SVG (D11). It draws what the SVG
// writer writes and okage renders: the same geometry, from the package
// shape, and the same rules of paint.
package raster

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"fmt"
	"image"
	"math"
	"net/url"
	"slices"
	"strings"

	"github.com/xo/okage"

	"github.com/xo/zuzu/internal/color"
	"github.com/xo/zuzu/internal/shape"
	"github.com/xo/zuzu/scene"
)

// Error is an error of the package raster.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// The errors of the package raster.
const (
	// ErrUnsupportedMark is the error of a mark that the renderer cannot
	// draw.
	ErrUnsupportedMark Error = "unsupported mark"
	// ErrUnsupportedCurve is the error of a line or an area with an
	// interpolation that the renderer cannot draw.
	ErrUnsupportedCurve Error = "unsupported interpolation"
	// ErrUnsupportedShape is the error of a symbol with a shape that the
	// renderer cannot draw.
	ErrUnsupportedShape Error = "unsupported symbol shape"
	// ErrSize is the error of a scene whose size gives no image.
	ErrSize Error = "invalid image size"
	// ErrText is the error of a text item when the renderer has no way to
	// draw text.
	ErrText Error = "no text renderer"
)

// TextRenderer draws the text items of a scene. okage gives the outlines of
// the glyphs (D11).
type TextRenderer interface {
	// DrawText draws a text item onto a pixmap, with the transform of its
	// group.
	DrawText(pm *okage.Pixmap, it *scene.Item, ts okage.Transform) error
}

// Option is an option of Draw.
type Option func(*drawer)

// WithText sets the renderer of the text items. Without it, a scene that
// has text is an error.
func WithText(t TextRenderer) Option {
	return func(d *drawer) {
		d.text = t
	}
}

// Draw draws a scene as an image. Its size is the size of the scene,
// rounded to whole pixels, as okage rounds the size of an SVG. The pixels
// are premultiplied, as image.RGBA expects.
func Draw(s *scene.Scene, opts ...Option) (*image.RGBA, error) {
	w, h := max(1, math.Round(s.Width)), max(1, math.Round(s.Height))
	if w > math.MaxInt32 || h > math.MaxInt32 || math.IsNaN(w) || math.IsNaN(h) {
		return nil, ErrSize
	}
	pm, ok := okage.NewPixmap(uint32(w), uint32(h))
	if !ok {
		return nil, ErrSize
	}
	d := &drawer{pm: pm}
	for _, o := range opts {
		o(d)
	}
	if !s.Background.IsZero() {
		bg := &scene.Item{Width: s.Width, Height: s.Height, Fill: s.Background}
		d.rect(bg, okage.Transform{SX: 1, SY: 1})
	}
	if err := d.mark(&s.Root, okage.Transform{SX: 1, SY: 1}); err != nil {
		return nil, err
	}
	return pm.AsRGBA(), nil
}

// drawer holds the state of one call of Draw.
type drawer struct {
	pm   *okage.Pixmap
	text TextRenderer
}

// mark draws one mark and its items, with the transform of its group.
func (d *drawer) mark(m *scene.Mark, ts okage.Transform) error {
	if len(m.Items) == 0 {
		return nil
	}
	switch m.Type {
	case scene.Group:
		for i := range m.Items {
			if err := d.group(&m.Items[i], ts); err != nil {
				return err
			}
		}
	case scene.Rect:
		for i := range m.Items {
			d.rect(&m.Items[i], ts)
		}
	case scene.Rule:
		for i := range m.Items {
			it := &m.Items[i]
			x2, y2 := shape.RuleEnd(it)
			pb := okage.NewPathBuilder()
			pb.MoveTo(float32(it.X), float32(it.Y))
			pb.LineTo(float32(x2), float32(y2))
			d.draw(pb, it, "", ts)
		}
	case scene.Line:
		p, ok := shape.Line(m.Items)
		if !ok {
			return fmt.Errorf("drawing a curve %q: %w", m.Items[0].Interpolate, ErrUnsupportedCurve)
		}
		d.path(p, &m.Items[0], "", ts)
	case scene.Area:
		p, ok := shape.Area(m.Items)
		if !ok {
			return fmt.Errorf("drawing a curve %q: %w", m.Items[0].Interpolate, ErrUnsupportedCurve)
		}
		d.path(p, &m.Items[0], "#000", ts)
	case scene.Trail:
		d.path(shape.TrailOf(m.Items), &m.Items[0], "#000", ts)
	case scene.Symbol:
		for i := range m.Items {
			it := &m.Items[i]
			p, ok := shape.SymbolOf(it)
			if !ok {
				return fmt.Errorf("drawing a symbol %q: %w", it.Shape, ErrUnsupportedShape)
			}
			d.path(p, it, "", place(ts, it.X, it.Y, it.Angle))
		}
	case scene.Arc:
		for i := range m.Items {
			it := &m.Items[i]
			d.path(shape.ArcOf(it), it, "", place(ts, it.X, it.Y, 0))
		}
	case scene.Path, scene.Shape:
		for i := range m.Items {
			it := &m.Items[i]
			d.path(it.Path, it, "", place(ts, it.X, it.Y, it.Angle))
		}
	case scene.Text:
		for i := range m.Items {
			if len(m.Items[i].Text) == 0 {
				continue
			}
			if d.text == nil {
				return ErrText
			}
			if err := d.text.DrawText(d.pm, &m.Items[i], ts); err != nil {
				return err
			}
		}
	case scene.Image:
		for i := range m.Items {
			d.image(&m.Items[i], ts)
		}
	default:
		return fmt.Errorf("drawing mark %q: %w", m.Type, ErrUnsupportedMark)
	}
	return nil
}

// place returns a transform moved to (x, y) and rotated by an angle in
// degrees.
func place(ts okage.Transform, x, y, angle float64) okage.Transform {
	ts = ts.PreTranslate(float32(x), float32(y))
	if angle != 0 {
		ts = ts.PreRotate(float32(angle))
	}
	return ts
}

// group draws a group item: its background, then its marks in the order
// of their zindex. A group that clips draws its marks into a layer, which
// goes onto the image through the mask of the clip, as okage draws a group
// with a clip path.
func (d *drawer) group(it *scene.Item, ts okage.Transform) error {
	ts = ts.PreTranslate(float32(it.X), float32(it.Y))
	if !it.Fill.IsZero() || !it.Stroke.IsZero() {
		bg := *it
		bg.X, bg.Y = 0, 0
		d.rect(&bg, ts)
	}
	if !it.Clip {
		return d.marks(it, ts)
	}
	pb := okage.NewPathBuilder()
	r, ok := okage.RectFromXYWH(0, 0, float32(it.Width), float32(it.Height))
	if !ok {
		return nil
	}
	pb.PushRect(r)
	clip, ok := pb.Finish()
	if !ok {
		return nil
	}
	layer, lx, ly, ok := d.layer(clip, ts, 0)
	if !ok {
		return nil
	}
	parent := d.pm
	d.pm = layer
	lts := ts.PostTranslate(float32(-lx), float32(-ly))
	err := d.marks(it, lts)
	d.pm = parent
	if err != nil {
		return err
	}
	if mask, ok := okage.NewMask(layer.Width(), layer.Height()); ok {
		mask.FillPath(clip, okage.FillRuleWinding, true, lts)
		layer.ApplyMask(mask)
	}
	pp := okage.DefaultPixmapPaint()
	d.pm.DrawPixmap(lx, ly, layer.AsRef(), &pp, okage.Transform{SX: 1, SY: 1}, nil)
	return nil
}

// marks draws the marks of a group in the order of their zindex, and in the
// order of the spec for an equal zindex.
func (d *drawer) marks(it *scene.Item, ts okage.Transform) error {
	order := make([]int, len(it.Items))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(it.Items[a].ZIndex, it.Items[b].ZIndex) })
	for _, i := range order {
		if err := d.mark(&it.Items[i], ts); err != nil {
			return err
		}
	}
	return nil
}

// rect draws a rect item. A rect whose corners share one radius has
// elliptical corners, as a rect of SVG with rx, and one whose corners
// differ has the corners of shape.RoundRect.
func (d *drawer) rect(it *scene.Item, ts okage.Transform) {
	x, y, wd, ht := shape.Box(it)
	tl, tr, br, bl := shape.Corners(it)
	if tl != tr || tr != br || br != bl {
		d.path(shape.RoundRect(x, y, wd, ht, tl, tr, br, bl), it, "", ts)
		return
	}
	if wd <= 0 || ht <= 0 {
		return
	}
	r := math.Min(tl, math.Min(wd, ht)/2)
	if r <= 0 {
		pb := okage.NewPathBuilder()
		if rc, ok := okage.RectFromXYWH(float32(x), float32(y), float32(wd), float32(ht)); ok {
			pb.PushRect(rc)
		}
		d.draw(pb, it, "", ts)
		return
	}
	// The path of a rect of SVG with rx: lines and four quarters of an
	// ellipse.
	f := func(v float64) string { return fmt.Sprintf("%g", v) }
	a := "A" + f(r) + "," + f(r) + " 0 0,1 "
	p := "M" + f(x+r) + "," + f(y) + "H" + f(x+wd-r) + a + f(x+wd) + "," + f(y+r) +
		"V" + f(y+ht-r) + a + f(x+wd-r) + "," + f(y+ht) +
		"H" + f(x+r) + a + f(x) + "," + f(y+ht-r) +
		"V" + f(y+r) + a + f(x+r) + "," + f(y) + "Z"
	d.path(p, it, "", ts)
}

// builder adapts a path builder of okage to the sink of shape.Parse.
type builder struct {
	pb *okage.PathBuilder
}

func (b builder) MoveTo(x, y float64) { b.pb.MoveTo(float32(x), float32(y)) }
func (b builder) LineTo(x, y float64) { b.pb.LineTo(float32(x), float32(y)) }
func (b builder) QuadTo(x1, y1, x, y float64) {
	b.pb.QuadTo(float32(x1), float32(y1), float32(x), float32(y))
}

func (b builder) CubicTo(x1, y1, x2, y2, x, y float64) {
	b.pb.CubicTo(float32(x1), float32(y1), float32(x2), float32(y2), float32(x), float32(y))
}
func (b builder) Close() { b.pb.Close() }

// path draws path data. Data that does not read draws its part that reads,
// as SVG draws it.
func (d *drawer) path(data string, it *scene.Item, fill string, ts okage.Transform) {
	pb := okage.NewPathBuilder()
	_ = shape.Parse(data, builder{pb})
	d.draw(pb, it, fill, ts)
}

// draw fills and strokes a path with the paint of an item. fill is the
// color of a fill that the item does not set, empty for none.
func (d *drawer) draw(pb *okage.PathBuilder, it *scene.Item, fill string, ts okage.Transform) {
	p, ok := pb.Finish()
	if !ok {
		return
	}
	opacity := 1.0
	if it.Opacity != nil {
		opacity = clamp(*it.Opacity)
	}
	if opacity == 0 {
		return
	}
	fillPaint := it.Fill
	if fillPaint.IsZero() && fill != "" {
		fillPaint = scene.Paint{Color: fill}
	}
	hasFill := !fillPaint.IsZero()
	hasStroke := !it.Stroke.IsZero() && strokeWidth(it) > 0
	target, layerOpacity := d.pm, float32(1)
	var lx, ly int32
	if opacity < 1 && (hasFill || hasStroke) {
		// The opacity of an element applies to its fill and its stroke
		// together, as a layer over the bounds of the element, as okage
		// draws a group with an opacity.
		if layer, x, y, ok := d.layer(p, ts, strokeWidth(it)); ok {
			target, layerOpacity, lx, ly = layer, float32(opacity), x, y
			ts = ts.PostTranslate(float32(-x), float32(-y))
			opacity = 1
		}
	}
	if hasFill {
		if paint, ok := d.paint(fillPaint, opacityOf(it.FillOpacity)*opacity, p); ok {
			target.FillPath(p, paint, okage.FillRuleWinding, ts, nil)
		}
	}
	if hasStroke {
		if paint, ok := d.paint(it.Stroke, opacityOf(it.StrokeOpacity)*opacity, p); ok {
			target.StrokePath(p, paint, stroke(it), ts, nil)
		}
	}
	if target != d.pm {
		pp := okage.DefaultPixmapPaint()
		pp.Opacity = layerOpacity
		d.pm.DrawPixmap(lx, ly, target.AsRef(), &pp, okage.Transform{SX: 1, SY: 1}, nil)
	}
}

// layer returns a pixmap that covers the bounds of a path with its stroke,
// in whole pixels inside the image, and its place.
func (d *drawer) layer(p *okage.Path, ts okage.Transform, width float64) (*okage.Pixmap, int32, int32, bool) {
	b := p.Bounds()
	pts := []okage.Point{
		okage.PointFromXY(b.Left(), b.Top()), okage.PointFromXY(b.Right(), b.Top()),
		okage.PointFromXY(b.Left(), b.Bottom()), okage.PointFromXY(b.Right(), b.Bottom()),
	}
	ts.MapPoints(pts)
	sx, sy := ts.GetScale()
	pad := float64(width)*float64(max(sx, sy)) + 2
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, q := range pts {
		x0, y0 = math.Min(x0, float64(q.X)), math.Min(y0, float64(q.Y))
		x1, y1 = math.Max(x1, float64(q.X)), math.Max(y1, float64(q.Y))
	}
	x0, y0 = math.Max(0, math.Floor(x0-pad)), math.Max(0, math.Floor(y0-pad))
	x1 = math.Min(float64(d.pm.Width()), math.Ceil(x1+pad))
	y1 = math.Min(float64(d.pm.Height()), math.Ceil(y1+pad))
	if x1 <= x0 || y1 <= y0 {
		return nil, 0, 0, false
	}
	layer, ok := okage.NewPixmap(uint32(x1-x0), uint32(y1-y0))
	return layer, int32(x0), int32(y0), ok
}

// opacityOf returns an opacity, 1 when it is not set.
func opacityOf(v *float64) float64 {
	if v == nil {
		return 1
	}
	return clamp(*v)
}

// clamp keeps an opacity from 0 to 1.
func clamp(v float64) float64 {
	if math.IsNaN(v) {
		return 1
	}
	return math.Max(0, math.Min(1, v))
}

// strokeWidth returns the width of the stroke of an item, 1 by default.
func strokeWidth(it *scene.Item) float64 {
	if it.StrokeWidth != nil {
		return *it.StrokeWidth
	}
	return 1
}

// stroke returns the stroke of an item: its width, its dash, its cap and
// its join, with the miter limit of SVG.
func stroke(it *scene.Item) *okage.Stroke {
	s := &okage.Stroke{Width: float32(strokeWidth(it)), MiterLimit: 4}
	switch it.StrokeCap {
	case "round":
		s.LineCap = okage.LineCapRound
	case "square":
		s.LineCap = okage.LineCapSquare
	}
	switch it.StrokeJoin {
	case "round":
		s.LineJoin = okage.LineJoinRound
	case "bevel":
		s.LineJoin = okage.LineJoinBevel
	}
	if len(it.StrokeDash) != 0 {
		dash := make([]float32, 0, 2*len(it.StrokeDash))
		for _, v := range it.StrokeDash {
			dash = append(dash, float32(v))
		}
		if len(dash)%2 == 1 {
			// SVG repeats a list of odd length.
			dash = append(dash, dash...)
		}
		if sd, ok := okage.NewStrokeDash(dash, 0); ok {
			s.Dash = sd
		}
	}
	return s
}

// paint returns the paint of a color or a gradient, with an opacity. A
// gradient spans the bounds of the path, as a gradient of SVG does by
// default. It returns false for a paint that draws nothing.
func (d *drawer) paint(p scene.Paint, opacity float64, path *okage.Path) (*okage.Paint, bool) {
	paint := okage.DefaultPaint()
	if p.Gradient == nil {
		c, ok := parseColor(p.Color, opacity)
		if !ok {
			return nil, false
		}
		paint.SetColor(c)
		return &paint, true
	}
	g := p.Gradient
	stops := make([]okage.GradientStop, 0, len(g.Stops))
	for _, s := range g.Stops {
		c, ok := parseColor(s.Color, opacity)
		if !ok {
			c = okage.ColorFromRGBA8(0, 0, 0, uint8(math.Round(255*opacity)))
		}
		stops = append(stops, okage.NewGradientStop(float32(s.Offset), c))
	}
	b, ok := path.ComputeTightBounds()
	if !ok {
		b = path.Bounds()
	}
	box := okage.TransformFromRow(b.Width(), 0, 0, b.Height(), b.Left(), b.Top())
	var shader okage.Shader
	if g.Type == scene.Radial {
		half := func(v float64) float32 {
			if v == 0 {
				return 0.5
			}
			return float32(v)
		}
		shader, ok = okage.NewRadialGradient(okage.PointFromXY(half(g.X1), half(g.Y1)), float32(g.R1),
			okage.PointFromXY(half(g.X2), half(g.Y2)), half(g.R2), stops, okage.SpreadModePad, box)
	} else {
		x2 := g.X2
		if g.X1 == 0 && g.Y1 == 0 && g.X2 == 0 && g.Y2 == 0 {
			x2 = 1
		}
		shader, ok = okage.NewLinearGradient(okage.PointFromXY(float32(g.X1), float32(g.Y1)),
			okage.PointFromXY(float32(x2), float32(g.Y2)), stops, okage.SpreadModePad, box)
	}
	if !ok {
		return nil, false
	}
	paint.Shader = shader
	return &paint, true
}

// parseColor reads a color of CSS with an opacity. none and a color that
// does not read draw nothing.
func parseColor(s string, opacity float64) (okage.Color, bool) {
	if s == "" || strings.EqualFold(s, "none") {
		return okage.Color{}, false
	}
	c, ok := color.Parse(s)
	if !ok {
		return okage.Color{}, false
	}
	a := clamp(c.A) * opacity
	if a <= 0 {
		return okage.Color{}, false
	}
	ch := func(v float64) float32 { return float32(math.Round(math.Max(0, math.Min(255, v)))) / 255 }
	return okage.ColorFromRGBAUnchecked(ch(c.R), ch(c.G), ch(c.B), float32(a)), true
}

// image draws an image item from a URL of data. An image that does not
// decode, or whose URL is not data, draws nothing, as okage draws it from
// SVG.
func (d *drawer) image(it *scene.Item, ts okage.Transform) {
	data, ok := dataURL(it.URL)
	if !ok || it.Width <= 0 || it.Height <= 0 {
		return
	}
	img, err := okage.DecodePNG(data)
	if err != nil {
		return
	}
	iw, ih := float64(img.Width()), float64(img.Height())
	sx, sy := it.Width/iw, it.Height/ih
	x, y := it.X, it.Y
	if it.Aspect == nil || *it.Aspect {
		// preserveAspectRatio xMidYMid meet: the whole image, centered.
		k := math.Min(sx, sy)
		x += (it.Width - iw*k) / 2
		y += (it.Height - ih*k) / 2
		sx, sy = k, k
	}
	quality := okage.FilterQualityBicubic
	if it.Smooth != nil && !*it.Smooth {
		quality = okage.FilterQualityNearest
	}
	opacity := float32(opacityOf(it.Opacity))
	pattern := okage.NewPattern(img.AsRef(), okage.SpreadModePad, quality, opacity,
		okage.TransformFromRow(float32(sx), 0, 0, float32(sy), float32(x), float32(y)))
	paint := okage.DefaultPaint()
	paint.Shader = pattern
	r, ok := okage.RectFromXYWH(float32(x), float32(y), float32(iw*sx), float32(ih*sy))
	if !ok {
		return
	}
	pb := okage.NewPathBuilder()
	pb.PushRect(r)
	if p, ok := pb.Finish(); ok {
		d.pm.FillPath(p, &paint, okage.FillRuleWinding, ts, nil)
	}
}

// dataURL returns the bytes of a URL of data in base64 or in plain text.
func dataURL(u string) ([]byte, bool) {
	rest, ok := strings.CutPrefix(u, "data:")
	if !ok {
		return nil, false
	}
	meta, payload, ok := strings.Cut(rest, ",")
	if !ok {
		return nil, false
	}
	if strings.HasSuffix(meta, ";base64") {
		b, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, false
		}
		return b, true
	}
	s, err := url.PathUnescape(payload)
	if err != nil {
		return nil, false
	}
	return bytes.Clone([]byte(s)), true
}
