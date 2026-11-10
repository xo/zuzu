// Package svg writes a scene graph as SVG (D10).
//
// The output is deterministic: the same scene gives the same bytes. It uses
// only the parts of SVG that okage renders: shapes, paths, text, linear and
// radial gradients and clip paths, with no CSS (D13).
package svg

import (
	"cmp"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
)

// Error is an error of the package svg.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// The errors of the package svg.
const (
	// ErrUnsupportedMark is the error of a mark that the writer cannot draw.
	ErrUnsupportedMark Error = "unsupported mark"
	// ErrUnsupportedCurve is the error of a line or an area with an
	// interpolation that the writer cannot draw.
	ErrUnsupportedCurve Error = "unsupported interpolation"
	// ErrUnsupportedShape is the error of a symbol with a shape that the
	// writer cannot draw.
	ErrUnsupportedShape Error = "unsupported symbol shape"
)

// DefaultFont is the font family of a text item that names none. zuzu bundles
// IBM Plex Sans (D16).
const DefaultFont = "'IBM Plex Sans', sans-serif"

// Write writes a scene as SVG.
func Write(w io.Writer, s *scene.Scene) error {
	wr := &writer{measure: text.NewEstimate()}
	var body strings.Builder
	wr.out = &body
	if err := wr.mark(&s.Root); err != nil {
		return err
	}
	var doc strings.Builder
	fmt.Fprintf(&doc, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s">`,
		num(s.Width), num(s.Height), num(s.Width), num(s.Height), esc(DefaultFont))
	var bg string
	if !s.Background.IsZero() {
		bg = fmt.Sprintf(`<rect width="%s" height="%s"%s/>`, num(s.Width), num(s.Height), wr.paint("fill", s.Background))
	}
	if wr.defs.Len() != 0 {
		fmt.Fprintf(&doc, "<defs>%s</defs>", wr.defs.String())
	}
	doc.WriteString(bg)
	doc.WriteString(body.String())
	doc.WriteString("</svg>\n")
	if _, err := io.WriteString(w, doc.String()); err != nil {
		return fmt.Errorf("writing svg: %w", err)
	}
	return nil
}

// writer holds the state of one call of Write.
type writer struct {
	out     *strings.Builder
	defs    strings.Builder
	ids     int
	measure *text.Estimate
}

// id returns a new identifier for an element of the defs.
func (w *writer) id(prefix string) string {
	w.ids++
	return prefix + strconv.Itoa(w.ids)
}

// mark writes one mark and its items.
func (w *writer) mark(m *scene.Mark) error {
	if len(m.Items) == 0 {
		return nil
	}
	var err error
	switch m.Type {
	case scene.Group:
		for i := range m.Items {
			if err = w.group(&m.Items[i]); err != nil {
				return err
			}
		}
	case scene.Rect:
		for i := range m.Items {
			w.rect(&m.Items[i])
		}
	case scene.Rule:
		for i := range m.Items {
			w.rule(&m.Items[i])
		}
	case scene.Line:
		err = w.line(m.Items)
	case scene.Area:
		err = w.area(m.Items)
	case scene.Trail:
		w.trail(m.Items)
	case scene.Symbol:
		for i := range m.Items {
			if err = w.symbol(&m.Items[i]); err != nil {
				return err
			}
		}
	case scene.Arc:
		for i := range m.Items {
			w.arc(&m.Items[i])
		}
	case scene.Path, scene.Shape:
		for i := range m.Items {
			w.path(&m.Items[i])
		}
	case scene.Text:
		for i := range m.Items {
			w.text(&m.Items[i])
		}
	case scene.Image:
		for i := range m.Items {
			w.image(&m.Items[i])
		}
	default:
		return fmt.Errorf("writing mark %q: %w", m.Type, ErrUnsupportedMark)
	}
	if err != nil {
		return fmt.Errorf("writing mark %q: %w", m.Type, err)
	}
	return nil
}

// group writes one group item: its background, its clip and its marks.
func (w *writer) group(it *scene.Item) error {
	fmt.Fprintf(w.out, `<g transform="translate(%s,%s)">`, num(it.X), num(it.Y))
	if !it.Fill.IsZero() || !it.Stroke.IsZero() {
		bg := *it
		bg.X, bg.Y = 0, 0
		w.rect(&bg)
	}
	if it.Clip {
		id := w.id("clip")
		fmt.Fprintf(&w.defs, `<clipPath id="%s"><rect width="%s" height="%s"/></clipPath>`, id, num(it.Width), num(it.Height))
		fmt.Fprintf(w.out, `<g clip-path="url(#%s)">`, id)
	}
	// The marks draw in the order of their zindex, and in the order of the
	// spec for an equal zindex, as Vega draws them.
	order := make([]int, len(it.Items))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(it.Items[a].ZIndex, it.Items[b].ZIndex) })
	for _, i := range order {
		if err := w.mark(&it.Items[i]); err != nil {
			return err
		}
	}
	if it.Clip {
		w.out.WriteString("</g>")
	}
	w.out.WriteString("</g>")
	return nil
}

// rect writes a rect item, with round corners if it has them.
func (w *writer) rect(it *scene.Item) {
	x, y, wd, ht := box(it)
	tl, tr, br, bl := corners(it)
	if tl == tr && tr == br && br == bl {
		r := math.Min(tl, math.Min(wd, ht)/2)
		fmt.Fprintf(w.out, `<rect x="%s" y="%s" width="%s" height="%s"`, num(x), num(y), num(wd), num(ht))
		if r > 0 {
			fmt.Fprintf(w.out, ` rx="%s"`, num(r))
		}
		w.style(it, "none")
		w.out.WriteString("/>")
		return
	}
	limit := func(r float64) float64 { return math.Max(0, math.Min(r, math.Min(wd, ht)/2)) }
	tl, tr, br, bl = limit(tl), limit(tr), limit(br), limit(bl)
	d := "M" + pt(x+tl, y) + "H" + num(x+wd-tr) +
		quad(x+wd, y, x+wd, y+tr, tr) + "V" + num(y+ht-br) +
		quad(x+wd, y+ht, x+wd-br, y+ht, br) + "H" + num(x+bl) +
		quad(x, y+ht, x, y+ht-bl, bl) + "V" + num(y+tl) +
		quad(x, y, x+tl, y, tl) + "Z"
	fmt.Fprintf(w.out, `<path d="%s"`, d)
	w.style(it, "none")
	w.out.WriteString("/>")
}

// quad returns a quadratic curve through the corner (cx, cy) to (x, y), or a
// line when the radius is zero.
func quad(cx, cy, x, y, r float64) string {
	if r == 0 {
		return "L" + pt(x, y)
	}
	return "Q" + pt(cx, cy) + " " + pt(x, y)
}

// box returns the position and the size of a rect, from x2 and y2 where the
// item has no width or height, and with no negative size.
func box(it *scene.Item) (x, y, wd, ht float64) {
	x, y, wd, ht = it.X, it.Y, it.Width, it.Height
	if wd == 0 && it.X2 != 0 {
		wd = it.X2 - it.X
	}
	if ht == 0 && it.Y2 != 0 {
		ht = it.Y2 - it.Y
	}
	if wd < 0 {
		x, wd = x+wd, -wd
	}
	if ht < 0 {
		y, ht = y+ht, -ht
	}
	return x, y, wd, ht
}

// corners returns the radius of each corner: top left, top right, bottom
// right and bottom left. A corner with its own radius overrides the radius of
// every corner.
func corners(it *scene.Item) (tl, tr, br, bl float64) {
	pick := func(own float64) float64 {
		if own != 0 {
			return own
		}
		return it.CornerRadius
	}
	return pick(it.CornerRadiusTopLeft), pick(it.CornerRadiusTopRight),
		pick(it.CornerRadiusBottomRight), pick(it.CornerRadiusBottomLeft)
}

// rule writes a rule item, a line from (x, y) to (x2, y2). An end that the
// item does not set is the start.
func (w *writer) rule(it *scene.Item) {
	x2, y2 := it.X2, it.Y2
	if x2 == 0 && it.Width == 0 {
		x2 = it.X
	}
	if y2 == 0 && it.Height == 0 {
		y2 = it.Y
	}
	fmt.Fprintf(w.out, `<line x1="%s" y1="%s" x2="%s" y2="%s"`, num(it.X), num(it.Y), num(x2), num(y2))
	w.style(it, "none")
	w.out.WriteString("/>")
}

// line writes a line mark: one path through its items, with the paint and
// the interpolation of the first item. An item that is not defined breaks the
// line.
func (w *writer) line(items []scene.Item) error {
	first := &items[0]
	var d strings.Builder
	for _, run := range runs(items) {
		pts := make([][2]float64, len(run))
		for i, it := range run {
			pts[i] = [2]float64{it.X, it.Y}
		}
		seg, err := curve(first.Interpolate, pts, true, first.Tension)
		if err != nil {
			return err
		}
		d.WriteString(seg)
	}
	fmt.Fprintf(w.out, `<path d="%s"`, d.String())
	w.style(first, "none")
	w.out.WriteString("/>")
	return nil
}

// area writes an area mark: from the line of x and y along its items, back
// along the line of x and y2, or of x2 and y for an area whose orient is
// horizontal.
func (w *writer) area(items []scene.Item) error {
	first := &items[0]
	var d strings.Builder
	for _, run := range runs(items) {
		top := make([][2]float64, len(run))
		base := make([][2]float64, len(run))
		for i, it := range run {
			top[i] = [2]float64{it.X, it.Y}
			if first.Orient == "horizontal" {
				base[len(run)-1-i] = [2]float64{it.X2, it.Y}
			} else {
				base[len(run)-1-i] = [2]float64{it.X, it.Y2}
			}
		}
		a, err := curve(first.Interpolate, top, true, first.Tension)
		if err != nil {
			return err
		}
		b, err := curve(first.Interpolate, base, false, first.Tension)
		if err != nil {
			return err
		}
		d.WriteString(a + b + "Z")
	}
	fmt.Fprintf(w.out, `<path d="%s"`, d.String())
	w.style(first, "#000")
	w.out.WriteString("/>")
	return nil
}

// trail writes a trail mark: a line through its items whose width is the size
// of each item, filled with the paint of the first item.
func (w *writer) trail(items []scene.Item) {
	var d strings.Builder
	for _, run := range runs(items) {
		pts := make([][3]float64, len(run))
		for i, it := range run {
			size := 2.0
			if it.Size != nil {
				size = *it.Size
			}
			pts[i] = [3]float64{it.X, it.Y, size}
		}
		d.WriteString(trail(pts))
	}
	fmt.Fprintf(w.out, `<path d="%s"`, d.String())
	w.style(&items[0], "#000")
	w.out.WriteString("/>")
}

// runs splits the items of a line or an area where an item is not defined.
func runs(items []scene.Item) [][]scene.Item {
	var out [][]scene.Item
	start := -1
	for i, it := range items {
		if it.Defined != nil && !*it.Defined {
			if start >= 0 {
				out = append(out, items[start:i])
			}
			start = -1
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, items[start:])
	}
	return out
}

// symbol writes a symbol item at its position, rotated by its angle.
func (w *writer) symbol(it *scene.Item) error {
	size := 64.0
	if it.Size != nil {
		size = *it.Size
	}
	d, err := shape(it.Shape, size)
	if err != nil {
		return err
	}
	fmt.Fprintf(w.out, `<path transform="%s" d="%s"`, transform(it.X, it.Y, it.Angle), d)
	w.style(it, "none")
	w.out.WriteString("/>")
	return nil
}

// transform returns a translation and, if the angle is not zero, a rotation
// in degrees.
func transform(x, y, angle float64) string {
	t := "translate(" + num(x) + "," + num(y) + ")"
	if angle != 0 {
		t += " rotate(" + num(angle) + ")"
	}
	return t
}

// arc writes an arc item, a slice of a ring, at its position.
func (w *writer) arc(it *scene.Item) {
	fmt.Fprintf(w.out, `<path transform="translate(%s,%s)" d="%s"`, num(it.X), num(it.Y),
		arcPath(it.InnerRadius, it.OuterRadius, it.StartAngle, it.EndAngle, it.PadAngle, it.CornerRadius))
	w.style(it, "none")
	w.out.WriteString("/>")
}

// path writes a path item at its position.
func (w *writer) path(it *scene.Item) {
	fmt.Fprintf(w.out, `<path transform="%s" d="%s"`, transform(it.X, it.Y, it.Angle), esc(it.Path))
	w.style(it, "none")
	w.out.WriteString("/>")
}

// image writes an image item.
func (w *writer) image(it *scene.Item) {
	if it.URL == "" {
		// An image with no URL draws nothing. An empty href names the
		// document itself.
		return
	}
	fmt.Fprintf(w.out, `<image x="%s" y="%s" width="%s" height="%s" href="%s"`,
		num(it.X), num(it.Y), num(it.Width), num(it.Height), esc(it.URL))
	if it.Aspect != nil && !*it.Aspect {
		w.out.WriteString(` preserveAspectRatio="none"`)
	}
	if it.Smooth != nil && !*it.Smooth {
		w.out.WriteString(` image-rendering="optimizeSpeed"`)
	}
	if it.Opacity != nil {
		fmt.Fprintf(w.out, ` opacity="%s"`, num(*it.Opacity))
	}
	w.out.WriteString("/>")
}

// The baselines and the alignments of Vega that the writer reads.
const (
	baselineTop    = "top"
	baselineMiddle = "middle"
	baselineBottom = "bottom"
)

// baselines are the offsets of the baselines of Vega, as fractions of the
// size of the font, from the alphabetic baseline that SVG draws on.
var baselines = map[string]float64{
	baselineTop:    0.79,
	baselineMiddle: 0.3,
	baselineBottom: -0.21,
}

// anchors are the alignments of Vega as the text-anchor of SVG.
var anchors = map[string]string{
	"center": baselineMiddle,
	"right":  "end",
}

// text writes a text item. A stroke on text is a halo, which SVG draws under
// the fill (docs/STYLE.md).
func (w *writer) text(it *scene.Item) {
	if len(it.Text) == 0 {
		return
	}
	size := it.FontSize
	if size == 0 {
		size = 11
	}
	x, y := it.X, it.Y
	if it.Radius != 0 {
		x += it.Radius * math.Sin(it.Theta)
		y -= it.Radius * math.Cos(it.Theta)
	}
	lh := it.LineHeight
	if lh == 0 {
		lh = size
	}
	y += it.Dy + baselines[it.Baseline]*size - lh*float64(len(it.Text)-1)*multiline(it.Baseline)
	x += it.Dx
	fmt.Fprintf(w.out, `<text transform="%s"`, transform(x, y, it.Angle))
	if a, ok := anchors[it.Align]; ok {
		fmt.Fprintf(w.out, ` text-anchor="%s"`, a)
	}
	if it.Font != "" {
		fmt.Fprintf(w.out, ` font-family="%s"`, esc(it.Font))
	}
	fmt.Fprintf(w.out, ` font-size="%s"`, num(size))
	if it.FontWeight != "" {
		fmt.Fprintf(w.out, ` font-weight="%s"`, esc(string(it.FontWeight)))
	}
	if it.FontStyle != "" {
		fmt.Fprintf(w.out, ` font-style="%s"`, esc(it.FontStyle))
	}
	if !it.Stroke.IsZero() {
		w.out.WriteString(` paint-order="stroke"`)
	}
	w.style(it, "#000")
	w.out.WriteString(">")
	lines := w.limit(it, size)
	if len(lines) == 1 {
		w.out.WriteString(esc(lines[0]))
	} else {
		for i, line := range lines {
			dy := "0"
			if i > 0 {
				dy = num(lh)
			}
			fmt.Fprintf(w.out, `<tspan x="0" dy="%s">%s</tspan>`, dy, esc(line))
		}
	}
	w.out.WriteString("</text>")
}

// limit cuts each line of a text item to its limit, in pixels, with its
// ellipsis, as Vega does. It measures with the estimate of the package text
// (D11). A text whose direction is rtl loses its start rather than its end.
func (w *writer) limit(it *scene.Item, size float64) []string {
	if it.Limit <= 0 {
		return it.Text
	}
	f := text.Font{Family: it.Font, Size: size, Weight: text.Weight(string(it.FontWeight)), Italic: it.FontStyle == "italic"}
	out := make([]string, len(it.Text))
	for i, line := range it.Text {
		if it.Dir == "rtl" {
			out[i] = reverse(text.Truncate(w.measure, f, reverse(line), it.Limit))
		} else {
			out[i] = text.Truncate(w.measure, f, line, it.Limit)
		}
		if it.Ellipsis != "" && it.Ellipsis != text.Ellipsis {
			if s, ok := strings.CutSuffix(out[i], text.Ellipsis); ok && out[i] != line {
				out[i] = s + it.Ellipsis
			}
		}
	}
	return out
}

// reverse reverses the runes of a string.
func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// multiline returns how far the first line of a text of several lines moves
// up, in lines, for a baseline: none for the top, half for the middle and all
// of them for the bottom.
func multiline(baseline string) float64 {
	switch baseline {
	case baselineMiddle:
		return 0.5
	case baselineBottom, "alphabetic", "":
		return 1
	}
	return 0
}

// style writes the paint of an item. fill is the fill to write when the item
// has none, which is none for most marks.
func (w *writer) style(it *scene.Item, fill string) {
	if it.Fill.IsZero() {
		fmt.Fprintf(w.out, ` fill="%s"`, fill)
	} else {
		w.out.WriteString(w.paint("fill", it.Fill))
	}
	if !it.Stroke.IsZero() {
		w.out.WriteString(w.paint("stroke", it.Stroke))
		width := 1.0
		if it.StrokeWidth != nil {
			width = *it.StrokeWidth
		}
		fmt.Fprintf(w.out, ` stroke-width="%s"`, num(width))
		if len(it.StrokeDash) != 0 {
			parts := make([]string, len(it.StrokeDash))
			for i, v := range it.StrokeDash {
				parts[i] = num(v)
			}
			fmt.Fprintf(w.out, ` stroke-dasharray="%s"`, strings.Join(parts, " "))
		}
		if it.StrokeCap != "" {
			fmt.Fprintf(w.out, ` stroke-linecap="%s"`, esc(it.StrokeCap))
		}
		if it.StrokeJoin != "" {
			fmt.Fprintf(w.out, ` stroke-linejoin="%s"`, esc(it.StrokeJoin))
		}
	}
	for _, o := range []struct {
		name string
		v    *float64
	}{{"opacity", it.Opacity}, {"fill-opacity", it.FillOpacity}, {"stroke-opacity", it.StrokeOpacity}} {
		if o.v != nil {
			fmt.Fprintf(w.out, ` %s="%s"`, o.name, num(*o.v))
		}
	}
}

// paint returns the attribute of a fill or a stroke, and writes a gradient
// to the defs.
func (w *writer) paint(attr string, p scene.Paint) string {
	if p.Gradient == nil {
		return fmt.Sprintf(` %s="%s"`, attr, esc(p.Color))
	}
	g := p.Gradient
	id := w.id("grad")
	if g.Type == scene.Radial {
		fmt.Fprintf(&w.defs, `<radialGradient id="%s" fx="%s" fy="%s" fr="%s" cx="%s" cy="%s" r="%s">`, id,
			num(half(g.X1)), num(half(g.Y1)), num(g.R1), num(half(g.X2)), num(half(g.Y2)), num(half(g.R2)))
	} else {
		x2 := g.X2
		if g.X1 == 0 && g.Y1 == 0 && g.X2 == 0 && g.Y2 == 0 {
			x2 = 1
		}
		fmt.Fprintf(&w.defs, `<linearGradient id="%s" x1="%s" y1="%s" x2="%s" y2="%s">`, id,
			num(g.X1), num(g.Y1), num(x2), num(g.Y2))
	}
	for _, s := range g.Stops {
		fmt.Fprintf(&w.defs, `<stop offset="%s" stop-color="%s"/>`, num(s.Offset), esc(s.Color))
	}
	if g.Type == scene.Radial {
		w.defs.WriteString("</radialGradient>")
	} else {
		w.defs.WriteString("</linearGradient>")
	}
	return fmt.Sprintf(` %s="url(#%s)"`, attr, id)
}

// half returns v, or one half when v is zero, which is the default of a
// coordinate of a radial gradient.
func half(v float64) float64 {
	if v == 0 {
		return 0.5
	}
	return v
}

// num writes a number with at most three decimals and no trailing zeros, so
// that the output is short and the same on every machine.
func num(v float64) string {
	v = math.Round(v*1000) / 1000
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// pt writes a point as x,y.
func pt(x, y float64) string {
	return num(x) + "," + num(y)
}

// esc escapes the text of an attribute or an element.
func esc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}
