// Package svg writes a scene graph as SVG (D10).
//
// The output is deterministic: the same scene gives the same bytes. It uses
// only the parts of SVG that okage renders: shapes, paths, text, linear and
// radial gradients and clip paths, with no CSS (D13). It also writes the link
// of an item as an a element around it, and the tooltip of an item as a
// title element inside it (D32), which okage does not draw. A link with a
// scheme that scene.LinkAllowed refuses, such as javascript, is not written.
package svg

import (
	"cmp"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/internal/shape"
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
const DefaultFont = shape.DefaultFont

// emojiFont returns a list of font families with Noto Color Emoji before
// its generic family, such as sans-serif, or at its end. The writer names
// it for a text with a character that IBM Plex Sans does not have, which
// the zuzu of a render draws with the font of WithEmoji, so that a viewer
// draws an emoji in color (D32).
func emojiFont(family string) string {
	emoji := "'" + text.EmojiFamily + "'"
	i := strings.LastIndexByte(family, ',')
	last := strings.TrimSpace(family[i+1:])
	switch last {
	case "serif", "sans-serif", "monospace", "cursive", "fantasy", "system-ui":
		if i < 0 {
			return emoji + ", " + last
		}
		return family[:i] + ", " + emoji + ", " + last
	}
	return family + ", " + emoji
}

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
	if len(s.Credits) > 0 {
		fmt.Fprintf(&doc, "<metadata>%s</metadata>", esc(strings.Join(s.Credits, "\n")))
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
	out  *strings.Builder
	defs strings.Builder
	ids  int
	// images holds the id of each image in the definitions, by its
	// attributes.
	images  map[string]string
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
	w.open(it)
	fmt.Fprintf(w.out, `<g transform="translate(%s,%s)">`, num(it.X), num(it.Y))
	w.title(it)
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
	w.closeLink(it)
	return nil
}

// rect writes a rect item, with round corners if it has them.
func (w *writer) rect(it *scene.Item) {
	x, y, wd, ht := shape.Box(it)
	tl, tr, br, bl := shape.Corners(it)
	if tl == tr && tr == br && br == bl {
		r := math.Min(tl, math.Min(wd, ht)/2)
		w.open(it)
		fmt.Fprintf(w.out, `<rect x="%s" y="%s" width="%s" height="%s"`, num(x), num(y), num(wd), num(ht))
		if r > 0 {
			fmt.Fprintf(w.out, ` rx="%s"`, num(r))
		}
		w.style(it, "none")
		w.end(it, "rect")
		return
	}
	w.open(it)
	fmt.Fprintf(w.out, `<path d="%s"`, shape.RoundRect(x, y, wd, ht, tl, tr, br, bl))
	w.style(it, "none")
	w.end(it, "path")
}

// rule writes a rule item, a line from (x, y) to (x2, y2).
func (w *writer) rule(it *scene.Item) {
	x2, y2 := shape.RuleEnd(it)
	w.open(it)
	fmt.Fprintf(w.out, `<line x1="%s" y1="%s" x2="%s" y2="%s"`, num(it.X), num(it.Y), num(x2), num(y2))
	w.style(it, "none")
	w.end(it, "line")
}

// line writes a line mark: one path through its items, with the paint and
// the interpolation of the first item.
func (w *writer) line(items []scene.Item) error {
	d, ok := shape.Line(items)
	if !ok {
		return fmt.Errorf("drawing a curve %q: %w", items[0].Interpolate, ErrUnsupportedCurve)
	}
	w.open(&items[0])
	fmt.Fprintf(w.out, `<path d="%s"`, d)
	w.style(&items[0], "none")
	w.end(&items[0], "path")
	return nil
}

// area writes an area mark.
func (w *writer) area(items []scene.Item) error {
	d, ok := shape.Area(items)
	if !ok {
		return fmt.Errorf("drawing a curve %q: %w", items[0].Interpolate, ErrUnsupportedCurve)
	}
	w.open(&items[0])
	fmt.Fprintf(w.out, `<path d="%s"`, d)
	w.style(&items[0], "#000")
	w.end(&items[0], "path")
	return nil
}

// trail writes a trail mark, filled with the paint of the first item.
func (w *writer) trail(items []scene.Item) {
	w.open(&items[0])
	fmt.Fprintf(w.out, `<path d="%s"`, shape.TrailOf(items))
	w.style(&items[0], "#000")
	w.end(&items[0], "path")
}

// symbol writes a symbol item at its position, rotated by its angle.
func (w *writer) symbol(it *scene.Item) error {
	d, ok := shape.SymbolOf(it)
	if !ok {
		return fmt.Errorf("drawing a symbol %q: %w", it.Shape, ErrUnsupportedShape)
	}
	w.open(it)
	fmt.Fprintf(w.out, `<path transform="%s" d="%s"`, transform(it.X, it.Y, it.Angle), d)
	w.style(it, "none")
	w.end(it, "path")
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
	w.open(it)
	fmt.Fprintf(w.out, `<path transform="translate(%s,%s)" d="%s"`, num(it.X), num(it.Y),
		shape.ArcOf(it))
	w.style(it, "none")
	w.end(it, "path")
}

// path writes a path item at its position.
func (w *writer) path(it *scene.Item) {
	w.open(it)
	fmt.Fprintf(w.out, `<path transform="%s" d="%s"`, transform(it.X, it.Y, it.Angle), esc(it.Path))
	w.style(it, "none")
	w.end(it, "path")
}

// image writes an image item.
func (w *writer) image(it *scene.Item) {
	if it.URL == "" {
		// An image with no URL draws nothing. An empty href names the
		// document itself.
		return
	}
	// Each image of a URL of data goes once in the definitions, with its
	// size, and each item uses it, so that an icon that draws many times
	// writes its bytes once (D31).
	var def strings.Builder
	fmt.Fprintf(&def, `width="%s" height="%s" href="%s"`, num(it.Width), num(it.Height), esc(it.URL))
	if it.Aspect != nil && !*it.Aspect {
		def.WriteString(` preserveAspectRatio="none"`)
	}
	if it.Smooth != nil && !*it.Smooth {
		def.WriteString(` image-rendering="optimizeSpeed"`)
	}
	w.open(it)
	tag := "use"
	if !strings.HasPrefix(it.URL, "data:") {
		tag = "image"
		fmt.Fprintf(w.out, `<image x="%s" y="%s" %s`, num(it.X), num(it.Y), def.String())
	} else {
		id, ok := w.images[def.String()]
		if !ok {
			if w.images == nil {
				w.images = map[string]string{}
			}
			id = w.id("zi")
			w.images[def.String()] = id
			fmt.Fprintf(&w.defs, `<image id="%s" %s/>`, id, def.String())
		}
		fmt.Fprintf(w.out, `<use x="%s" y="%s" href="#%s"`, num(it.X), num(it.Y), id)
	}
	if it.Opacity != nil {
		fmt.Fprintf(w.out, ` opacity="%s"`, num(*it.Opacity))
	}
	w.end(it, tag)
}

// text writes a text item. A stroke on text is a halo, which SVG draws under
// the fill (docs/STYLE.md).
func (w *writer) text(it *scene.Item) {
	if len(it.Text) == 0 {
		return
	}
	t := shape.TextOf(it, w.measure)
	w.open(it)
	fmt.Fprintf(w.out, `<text transform="%s"`, transform(t.X, t.Y, t.Angle))
	if t.Anchor != "start" {
		fmt.Fprintf(w.out, ` text-anchor="%s"`, t.Anchor)
	}
	family := it.Font
	if slices.ContainsFunc(t.Lines, func(l string) bool { return len(w.measure.Missing(l)) > 0 }) {
		family = emojiFont(cmp.Or(family, DefaultFont))
	}
	if family != "" {
		fmt.Fprintf(w.out, ` font-family="%s"`, esc(family))
	}
	fmt.Fprintf(w.out, ` font-size="%s"`, num(t.Size))
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
	w.title(it)
	if len(t.Lines) == 1 {
		w.out.WriteString(esc(t.Lines[0]))
	} else {
		for i, line := range t.Lines {
			dy := "0"
			if i > 0 {
				dy = num(t.LineHeight)
			}
			fmt.Fprintf(w.out, `<tspan x="0" dy="%s">%s</tspan>`, dy, esc(line))
		}
	}
	w.out.WriteString("</text>")
	w.closeLink(it)
}

// open writes the start of the link of an item, if it has a link that
// scene.LinkAllowed accepts.
func (w *writer) open(it *scene.Item) {
	if it.Href != "" && scene.LinkAllowed(it.Href) {
		fmt.Fprintf(w.out, `<a href="%s">`, esc(it.Href))
	}
}

// closeLink writes the end of the link of an item, if open wrote its start.
func (w *writer) closeLink(it *scene.Item) {
	if it.Href != "" && scene.LinkAllowed(it.Href) {
		w.out.WriteString("</a>")
	}
}

// title writes the tooltip of an item as a title element, if it has one.
func (w *writer) title(it *scene.Item) {
	if it.Tooltip != "" {
		fmt.Fprintf(w.out, "<title>%s</title>", esc(string(it.Tooltip)))
	}
}

// end ends the element tag of an item, with its tooltip as a title inside
// it, and then its link.
func (w *writer) end(it *scene.Item, tag string) {
	if it.Tooltip == "" {
		w.out.WriteString("/>")
	} else {
		w.out.WriteString(">")
		w.title(it)
		fmt.Fprintf(w.out, "</%s>", tag)
	}
	w.closeLink(it)
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

// esc escapes the text of an attribute or an element.
func esc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}
