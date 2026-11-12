// Package raster draws a scene graph as an image with the drawing of okage
// directly, without the round trip through SVG (D11). It draws what the SVG
// writer writes and okage renders: the same geometry, from the package
// shape, and the same rules of paint.
package raster

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"math"
	"net/url"
	"slices"
	"strings"

	"github.com/xo/okage"
	okagesvg "github.com/xo/okage/svg"

	"github.com/xo/zuzu/internal/color"
	"github.com/xo/zuzu/internal/shape"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
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
)

// Option is an option of Draw.
type Option func(*drawer)

// WithRenderer sets the renderer of okage whose fonts draw the text, with
// the layout of text of okage (okage D37). Make one renderer and share it,
// because it loads the fonts of the system on its first use (okage D36).
// Without it, Draw makes a renderer for a scene that has text.
func WithRenderer(r *okagesvg.Renderer) Option {
	return func(d *drawer) {
		d.fonts = r
	}
}

// WithFallbacks sets the fonts for the characters that IBM Plex Sans does
// not have, such as the font of the package text/emoji, in order (D32). A
// line measures with them for its limit, as the layout measured it, and a
// character that no font has draws nothing. The renderer of WithRenderer
// must hold the same fonts, so that it draws them.
func WithFallbacks(fallbacks ...*text.Fallback) Option {
	return func(d *drawer) {
		d.fallbacks = fallbacks
	}
}

// WithScale draws a scene at a scale, as a ratio of device pixels to the
// pixels of the scene. A scale of 2 gives an image of twice the width and
// twice the height, with the same layout and sharper lines and text. A
// scale that is not more than zero, or not finite, is ErrSize.
func WithScale(scale float64) Option {
	return func(d *drawer) {
		d.scale = scale
	}
}

// Draw draws a scene as an image. Its size is the size of the scene, times
// the scale of WithScale, rounded to whole pixels, as okage rounds the size
// of an SVG. The pixels are premultiplied, as image.RGBA expects.
func Draw(s *scene.Scene, opts ...Option) (*image.RGBA, error) {
	d := &drawer{scale: 1}
	for _, o := range opts {
		o(d)
	}
	d.measure = text.NewEstimate(d.fallbacks...)
	if !(d.scale > 0) || math.IsInf(d.scale, 0) {
		return nil, ErrSize
	}
	w, h := max(1, math.Round(s.Width*d.scale)), max(1, math.Round(s.Height*d.scale))
	if w > math.MaxInt32 || h > math.MaxInt32 || math.IsNaN(w) || math.IsNaN(h) {
		return nil, ErrSize
	}
	pm, ok := okage.NewPixmap(uint32(w), uint32(h))
	if !ok {
		return nil, ErrSize
	}
	d.pm = pm
	ts := okage.Transform{SX: float32(d.scale), SY: float32(d.scale)}
	if !s.Background.IsZero() {
		bg := &scene.Item{Width: s.Width, Height: s.Height, Fill: s.Background}
		d.rect(bg, ts)
	}
	if err := d.mark(&s.Root, ts); err != nil {
		return nil, err
	}
	return pm.AsRGBA(), nil
}

// drawer holds the state of one call of Draw.
type drawer struct {
	pm *okage.Pixmap
	// scale is the ratio of device pixels to the pixels of the scene.
	scale float64
	// fonts shapes the text, and measure measures a line for its limit, as
	// the SVG writer measures it.
	fonts     *okagesvg.Renderer
	measure   *text.Estimate
	fallbacks []*text.Fallback
	// images holds each image that the call decoded, by its URL.
	images map[string]*picture
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
			if err := d.text(&m.Items[i], ts); err != nil {
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
	d.paintPath(pb, it, fill, ts, false)
}

// paintPath fills and strokes a path with the paint of an item, the stroke
// first when strokeFirst is true, as SVG draws the halo of a text.
func (d *drawer) paintPath(pb *okage.PathBuilder, it *scene.Item, fill string, ts okage.Transform, strokeFirst bool) {
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
	fillIt := func() {
		if paint, ok := d.paint(fillPaint, opacityOf(it.FillOpacity)*opacity, p); ok {
			target.FillPath(p, paint, okage.FillRuleWinding, ts, nil)
		}
	}
	strokeIt := func() {
		if paint, ok := d.paint(it.Stroke, opacityOf(it.StrokeOpacity)*opacity, p); ok {
			target.StrokePath(p, paint, stroke(it), ts, nil)
		}
	}
	if hasStroke && strokeFirst {
		strokeIt()
	}
	if hasFill {
		fillIt()
	}
	if hasStroke && !strokeFirst {
		strokeIt()
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
	// The opacity is a byte, as okage gives the color of a paint.
	return okage.ColorFromRGBAUnchecked(ch(c.R), ch(c.G), ch(c.B), float32(math.Round(a*255))/255), true
}

// image draws an image item from a URL of data: a PNG, a JPEG or an SVG.
// An image that does not decode, or whose URL is not data, draws nothing,
// as okage draws it from SVG. Each URL decodes once in a call of Draw
// (D31).
func (d *drawer) image(it *scene.Item, ts okage.Transform) {
	if it.Width <= 0 || it.Height <= 0 {
		return
	}
	src, ok := d.images[it.URL]
	if !ok {
		src = d.decode(it.URL)
		if d.images == nil {
			d.images = map[string]*picture{}
		}
		d.images[it.URL] = src
	}
	if src == nil {
		return
	}
	iw, ih := src.w, src.h
	sx, sy := it.Width/iw, it.Height/ih
	x, y := it.X, it.Y
	if it.Aspect == nil || *it.Aspect {
		// preserveAspectRatio xMidYMid meet: the whole image, centered.
		k := math.Min(sx, sy)
		x += (it.Width - iw*k) / 2
		y += (it.Height - ih*k) / 2
		sx, sy = k, k
	}
	opacity := float32(opacityOf(it.Opacity))
	if src.tree != nil {
		if opacity <= 0 {
			return
		}
		t := ts.PreTranslate(float32(x), float32(y)).PreScale(float32(sx), float32(sy))
		m := d.pm.AsMut()
		okagesvg.Render(src.tree, t, &m)
		return
	}
	quality := okage.FilterQualityBicubic
	if it.Smooth != nil && !*it.Smooth {
		quality = okage.FilterQualityNearest
	}
	pattern := okage.NewPattern(src.pm.AsRef(), okage.SpreadModePad, quality, opacity,
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

// picture is a decoded image: its pixels, or the tree of an SVG, and its
// size.
type picture struct {
	pm   *okage.Pixmap
	tree *okagesvg.Tree
	w, h float64
}

// decode decodes the image of a URL of data, or returns nil.
func (d *drawer) decode(u string) *picture {
	data, ok := dataURL(u)
	if !ok {
		return nil
	}
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG")):
		img, err := okage.DecodePNG(data)
		if err != nil {
			return nil
		}
		return &picture{pm: img, w: float64(img.Width()), h: float64(img.Height())}
	case bytes.HasPrefix(data, []byte("\xff\xd8")):
		img, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			return nil
		}
		b := img.Bounds()
		if b.Dx() <= 0 || b.Dy() <= 0 || b.Dx() > 1<<15 || b.Dy() > 1<<15 {
			return nil
		}
		pm, ok := okage.NewPixmap(uint32(b.Dx()), uint32(b.Dy())) //nolint:gosec // The size is checked above.
		if !ok {
			return nil
		}
		rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
		copy(pm.DataMut(), rgba.Pix)
		return &picture{pm: pm, w: float64(b.Dx()), h: float64(b.Dy())}
	}
	r := d.fonts
	if r == nil {
		r = okagesvg.New(okagesvg.WithLoadSystemFonts(false))
	}
	doc, err := r.Parse(data)
	if err != nil {
		return nil
	}
	tree := doc.Tree()
	size := tree.Size()
	if size.Width() <= 0 || size.Height() <= 0 {
		return nil
	}
	return &picture{tree: tree, w: float64(size.Width()), h: float64(size.Height())}
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

// text draws a text item: the outlines of the glyphs of each line, from the
// layout of text of okage, at the place that shape.TextOf gives, as the SVG
// writer writes it and okage draws it. A stroke is a halo under the fill. A
// glyph in color, such as an emoji of Noto Color Emoji, draws over the
// outlines in its own colors (D32). A character that no font has draws
// nothing, and a line that no font draws draws nothing, as in SVG.
func (d *drawer) text(it *scene.Item, ts okage.Transform) error {
	if len(it.Text) == 0 {
		return nil
	}
	t := shape.TextOf(it, d.measure)
	if !(t.Size > 0) || math.IsInf(t.Size, 0) {
		// A text of no size draws nothing, as in SVG.
		return nil
	}
	if d.fonts == nil {
		d.fonts = okagesvg.New()
	}
	family := it.Font
	if family == "" {
		family = shape.DefaultFont
	}
	style := okagesvg.TextStyle{Family: family, Size: float32(t.Size), Weight: uint16(min(1000, max(1, text.Weight(string(it.FontWeight)))))}
	switch it.FontStyle {
	case "italic":
		style.Style = okagesvg.FontStyleItalic
	case "oblique":
		style.Style = okagesvg.FontStyleOblique
	}
	font := text.Font{Family: text.Family, Size: t.Size, Weight: int(style.Weight), Italic: it.FontStyle != ""}
	pb := okage.NewPathBuilder()
	var colors []placed
	for i, line := range t.Lines {
		// Each piece of a line shapes on its own, so that a sequence of
		// emoji, such as a flag, draws as one glyph, and the pieces follow
		// one another. A piece of spaces alone draws no glyph, and okage
		// gives no run for it, so it takes its measured width.
		var runs []*okagesvg.TextRun
		var widths []float64
		width := 0.0
		pieces := d.measure.Split(d.drawable(shape.Collapse(line)))
		for _, piece := range pieces {
			run, err := d.fonts.ShapeText(piece, style)
			w := 0.0
			switch {
			case errors.Is(err, okage.ErrSVGTextNoFont):
				run = nil
				if len(pieces) > 1 && strings.TrimSpace(piece) == "" {
					w = d.measure.Width(font, piece)
				}
			case err != nil:
				return fmt.Errorf("drawing the text %q: %w", line, err)
			default:
				w = float64(run.Width())
			}
			runs, widths = append(runs, run), append(widths, w)
			width += w
		}
		x := 0.0
		switch t.Anchor {
		case "middle":
			x = -width / 2
		case "end":
			x = -width
		}
		y := float64(i) * t.LineHeight
		for j, run := range runs {
			if run != nil {
				run.AppendPath(pb, float32(x), float32(y))
				if run.HasColorGlyphs() {
					colors = append(colors, placed{run, x, y})
				}
			}
			x += widths[j]
		}
	}
	at := place(ts, t.X, t.Y, t.Angle)
	d.paintPath(pb, it, "#000", at, true)
	d.colorGlyphs(colors, it, at)
	return nil
}

// drawable returns a line without the characters that no font has, which
// draw nothing (D32). okage draws such a character as the box of its
// missing glyph.
func (d *drawer) drawable(line string) string {
	miss := d.measure.Missing(line)
	if len(miss) == 0 {
		return line
	}
	return strings.Map(func(r rune) rune {
		if slices.Contains(miss, r) {
			return -1
		}
		return r
	}, line)
}

// placed is a run with glyphs in color, and the origin of the run.
type placed struct {
	run  *okagesvg.TextRun
	x, y float64
}

// colorGlyphs draws the glyphs in color of the runs of a text item, in
// their own colors, with the transform of the item. An item with an
// opacity draws them in a layer with that opacity.
func (d *drawer) colorGlyphs(colors []placed, it *scene.Item, ts okage.Transform) {
	if len(colors) == 0 {
		return
	}
	opacity := opacityOf(it.Opacity) * opacityOf(it.FillOpacity)
	if opacity <= 0 {
		return
	}
	target := d.pm
	if opacity < 1 {
		layer, ok := okage.NewPixmap(d.pm.Width(), d.pm.Height())
		if !ok {
			return
		}
		target = layer
	}
	m := target.AsMut()
	for _, c := range colors {
		c.run.RenderColorGlyphs(&m, ts.PreTranslate(float32(c.x), float32(c.y)))
	}
	if target != d.pm {
		pp := okage.DefaultPixmapPaint()
		pp.Opacity = float32(opacity)
		d.pm.DrawPixmap(0, 0, target.AsRef(), &pp, okage.Transform{SX: 1, SY: 1}, nil)
	}
}
