// Package sketch turns a scene into the same scene drawn as if by hand,
// with the copy of rough-go in internal/rough (D39, D40). The runtime runs
// it after the layout and before any renderer, so that the SVG writer and
// the package raster read the same scene and draw the same look (hard
// rule 5).
//
// The pass changes every mark: the marks of the data, the axes, the grid
// lines, the legends, the frames and the background of a group. It turns
// each item into paths of type path: one path for its fill and one for its
// stroke, as rough.js draws them. A fill draws as a hachure, lines in the
// paint of the fill, and never as a pattern of SVG, over an underlay in
// the color of the background that hides what is under it. A gradient
// stays a gradient: the lines of its hachure take it, and a large shape
// draws them over a tint of it (D47, D50). Only a text, an image and the
// outline of a formula keep their shape. "The look like Excalidraw" in
// docs/STYLE.md holds the rules.
package sketch

import (
	"hash/fnv"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/internal/color"
	"github.com/xo/zuzu/internal/rough"
	"github.com/xo/zuzu/internal/shape"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/theme"
)

// Options are the settings of the pass.
type Options struct {
	// Seed is the seed of the chart. Each item takes its seed from a hash
	// of it (D40 part 6). A seed of 0 is theme.SketchSeed.
	Seed int64
	// Font is the family of each text, such as Excalifont before IBM Plex
	// Sans, or empty to keep the family of each text.
	Font string
	// Measure measures a text in the font of the look. The pass cuts a
	// text to its limit with it, so that each renderer draws the text that
	// the layout measured. A nil Measure keeps the limit of each text.
	Measure text.Measurer
}

// RoleBackground is the role of the mark that the pass adds as the first
// mark of a group with a fill or a stroke. It draws the background of the
// group, which the group then no longer draws.
const RoleBackground = "zuzu-sketch-background"

// thin is the size under which a rect draws as one line along its length,
// because a hachure needs room for its lines.
const thin = 2 * theme.SketchHachureGap

// Draw turns a scene into the scene drawn as if by hand, in place. It
// grows the canvas, and moves the root, where a stroke wobbles out of it,
// so that the image holds each line (D39 part 7). The same scene and
// options give the same scene.
func Draw(s *scene.Scene, o Options) {
	seed := o.Seed
	if seed == 0 {
		seed = theme.SketchSeed
	}
	p := &pass{
		o:      o,
		seed:   seed,
		bg:     s.Background.Color,
		gen:    rough.NewGenerator(nil),
		bounds: box{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)},
	}
	s.Root = p.mark(&s.Root, "", frame{})
	p.grow(s)
}

// pass holds the state of one call of Draw.
type pass struct {
	o    Options
	seed int64
	gen  *rough.Generator
	// bg is the color of the background of the canvas, or empty.
	bg string
	// bounds holds the bounds of the paths of the pass on the canvas,
	// outside a clip.
	bounds box
	// covers holds the box on the canvas of each shape with a fill, in the
	// order of the pass, and for a shape that draws its gradient over a
	// tint, the color that its tint shows over the background (D50).
	covers []cover
}

// cover is the box of a shape with a fill. tint says that the shape draws
// over a tint, of the color color over the background.
type cover struct {
	box

	tint  bool
	color color.RGBA
}

// frame is the place of a group on the canvas, and whether a clip holds
// its marks.
type frame struct {
	x, y    float64
	clipped bool
}

// box is a rectangle from (x1, y1) to (x2, y2).
type box struct {
	x1, y1, x2, y2 float64
}

// add grows a box to hold a point.
func (b *box) add(x, y float64) {
	b.x1, b.y1 = math.Min(b.x1, x), math.Min(b.y1, y)
	b.x2, b.y2 = math.Max(b.x2, x), math.Max(b.y2, y)
}

// key returns the name of a mark for the seeds of its items: its name, or
// else its role and its type.
func key(m *scene.Mark) string {
	if m.Name != "" {
		return m.Name
	}
	return m.Type + ":" + m.Role
}

// mark returns a mark drawn as if by hand. path names the mark in the
// scene for the seeds of its items: the names of the marks above it and
// the index of each group item, never the order of a walk.
func (p *pass) mark(m *scene.Mark, path string, f frame) scene.Mark {
	out := *m
	f.clipped = f.clipped || m.Clip
	switch m.Type {
	case scene.Group:
		out.Items = make([]scene.Item, len(m.Items))
		for i := range m.Items {
			out.Items[i] = p.group(m, &m.Items[i], path+"#"+strconv.Itoa(i), f)
		}
	case scene.Text:
		out.Items = make([]scene.Item, len(m.Items))
		for i := range m.Items {
			out.Items[i] = p.text(&m.Items[i], f)
		}
	case scene.Image:
	case scene.Path, scene.Shape:
		if m.Role == scene.RoleMath {
			// The outline of a formula is text, and keeps its shape.
			return out
		}
		out = p.shapes(m, path, f)
	case scene.Rect, scene.Rule, scene.Symbol, scene.Arc, scene.Line, scene.Area, scene.Trail:
		out = p.shapes(m, path, f)
	}
	return out
}

// group returns a group item with its marks drawn as if by hand, and its
// background as the first of its marks.
func (p *pass) group(m *scene.Mark, it *scene.Item, path string, f frame) scene.Item {
	g := *it
	inner := frame{x: f.x + it.X, y: f.y + it.Y, clipped: f.clipped || it.Clip}
	marks := make([]scene.Mark, 0, len(it.Items)+1)
	if !it.Fill.IsZero() || !it.Stroke.IsZero() {
		bg := *it
		bg.X, bg.Y, bg.Items = 0, 0, nil
		// The background draws under every mark of the group. A group
		// that clips clips the wobble of its background too.
		z := 0
		for i := range it.Items {
			z = min(z, it.Items[i].ZIndex)
		}
		parts := p.rect(m, &bg, p.itemSeed(path, -1), inner)
		if len(parts) > 0 {
			marks = append(marks, scene.Mark{Type: scene.Path, Role: RoleBackground, ZIndex: z, Items: parts})
		}
		g.Fill, g.Stroke = scene.Paint{}, scene.Paint{}
	}
	counts := map[string]int{}
	for i := range it.Items {
		c := &it.Items[i]
		k := key(c)
		if n := counts[k]; n > 0 {
			k += "~" + strconv.Itoa(n)
		}
		counts[key(c)]++
		marks = append(marks, p.mark(c, path+"/"+k, inner))
	}
	g.Items = marks
	return g
}

// text returns a text item in the font of the look, cut to its limit in
// that font.
func (p *pass) text(it *scene.Item, f frame) scene.Item {
	t := *it
	p.readOnTint(&t, f)
	if p.o.Font == "" {
		return t
	}
	t.Font = p.o.Font
	if p.o.Measure != nil && t.Limit > 0 {
		t.Text = shape.TextOf(&t, p.o.Measure).Lines
		t.Limit = 0
	}
	return t
}

// textLight and textDark are the colors of text that a graph gives a label
// so that it reads on its fill: the text of the light theme and the text of
// the dark theme (graph.Contrast).
var (
	textLight = theme.New(theme.Light, theme.Zuzu).Text
	textDark  = theme.New(theme.Dark, theme.Zuzu).Text
)

// readOnTint gives a text that sits on a shape drawn over a tint the color
// of text that reads on the tint, and a halo in the color of the
// background, when the text has one of the colors that a graph chooses to
// read on a fill. A gradient that is dark in the
// standard look, such as red to blue, gives its label light text, and its
// tint in the look is pale, so the label takes dark text there (D50). A
// color of text that the spec gives stays.
func (p *pass) readOnTint(t *scene.Item, f frame) {
	c := t.Fill.Color
	if t.Fill.Gradient != nil || !sameColor(c, textLight) && !sameColor(c, textDark) {
		return
	}
	x, y := f.x+t.X, f.y+t.Y
	for _, tb := range slices.Backward(p.covers) {
		if x < tb.x1 || x > tb.x2 || y < tb.y1 || y > tb.y2 {
			continue
		}
		// The last shape under the text is the one that it sits on. A
		// shape with no tint keeps the color of the text.
		if !tb.tint {
			return
		}
		if tb.color.Luminance() > 0.35 {
			t.Fill.Color = textLight
		} else {
			t.Fill.Color = textDark
		}
		if t.Stroke.IsZero() && p.bg != "" {
			// The lines of the hachure cross the text, so the text takes
			// the halo of a label on a busy mark (docs/STYLE.md).
			w := 3.0
			t.Stroke, t.StrokeWidth = scene.Paint{Color: p.bg}, &w
		}
		return
	}
}

// addTint records the box of a shape that draws over a tint, x1, y1 to x2,
// y2 in its group, and the color of its tint over the background: the mean
// of the colors of its gradient at its opacity.
func (p *pass) addTint(ps paints, opacity, x1, y1, x2, y2 float64, f frame) {
	bg, ok := color.Parse(p.bg)
	if !ok || ps.fill.Gradient == nil || len(ps.fill.Gradient.Stops) == 0 {
		return
	}
	var r, g, b float64
	for _, st := range ps.fill.Gradient.Stops {
		c, _ := color.Parse(st.Color)
		r, g, b = r+c.R, g+c.G, b+c.B
	}
	n := float64(len(ps.fill.Gradient.Stops))
	a := opacity * theme.SketchGradientTint
	mix := color.RGBA{R: bg.R + (r/n-bg.R)*a, G: bg.G + (g/n-bg.G)*a, B: bg.B + (b/n-bg.B)*a, A: 1}
	p.covers = append(p.covers, cover{box{f.x + x1, f.y + y1, f.x + x2, f.y + y2}, true, mix})
}

// addCover records the box of a shape with a fill and no tint, x1, y1 to
// x2, y2 in its group, so that a text on it keeps its color.
func (p *pass) addCover(ps paints, x1, y1, x2, y2 float64, f frame) {
	if ps.fill.IsZero() || ps.tint {
		return
	}
	p.covers = append(p.covers, cover{box: box{f.x + x1, f.y + y1, f.x + x2, f.y + y2}})
}

// itemSeed returns the seed of an item: a hash of the seed of the chart,
// the path of its mark and its index, from 1 to 2^31-1, so that rough.js
// never takes a random seed (D39 part 6).
func (p *pass) itemSeed(path string, index int) uint32 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(strconv.FormatInt(p.seed, 10) + "\x00" + path + "\x00" + strconv.Itoa(index)))
	return uint32(h.Sum64()%(1<<31-1)) + 1
}

// next returns another seed from a seed, for a second shape of an item.
func next(seed uint32) uint32 {
	return uint32((uint64(seed)*48271)%(1<<31-1)) | 1
}

// shapes returns a mark of paths that draws the items of a mark as if by
// hand, the fill and then the stroke of each item, in the order of the
// items, so that the order of the marks and the items stays.
func (p *pass) shapes(m *scene.Mark, path string, f frame) scene.Mark {
	out := scene.Mark{Type: scene.Path, Role: m.Role, Name: m.Name, Clip: m.Clip, ClipPath: m.ClipPath, ZIndex: m.ZIndex, Extra: m.Extra}
	if len(m.Items) == 0 {
		out.Items = m.Items
		return out
	}
	switch m.Type {
	case scene.Line:
		if d, ok := shape.Line(m.Items); ok {
			out.Items = p.path(m, &m.Items[0], d, "", 0, 0, 0, p.itemSeed(path, 0), f)
		}
	case scene.Area:
		if d, ok := shape.Area(m.Items); ok {
			out.Items = p.path(m, &m.Items[0], d, "#000", 0, 0, 0, p.itemSeed(path, 0), f)
		}
	case scene.Trail:
		out.Items = p.path(m, &m.Items[0], shape.TrailOf(m.Items), "#000", 0, 0, 0, p.itemSeed(path, 0), f)
	default:
		for i := range m.Items {
			it := &m.Items[i]
			if it.Opacity != nil && *it.Opacity <= 0 {
				continue
			}
			seed := p.itemSeed(path, i)
			switch m.Type {
			case scene.Rect:
				out.Items = append(out.Items, p.rect(m, it, seed, f)...)
			case scene.Rule:
				out.Items = append(out.Items, p.rule(m, it, seed, f)...)
			case scene.Symbol:
				out.Items = append(out.Items, p.symbol(m, it, seed, f)...)
			case scene.Arc:
				out.Items = append(out.Items, p.path(m, it, shape.ArcOf(it), "", it.X, it.Y, 0, seed, f)...)
			default:
				out.Items = append(out.Items, p.path(m, it, it.Path, "", it.X, it.Y, it.Angle, seed, f)...)
			}
		}
	}
	if out.Items == nil {
		out.Items = []scene.Item{}
	}
	return out
}

// paints are the paints of the fill and the stroke of an item, a color or
// a gradient, zero for none, and the width of its stroke. solid says that
// the fill draws solid, not as a hachure. roughness and fillWeight are the
// roughness of its lines and the width of the lines of its hachure, which
// its mark and its size set.
type paints struct {
	fill, stroke scene.Paint
	width        float64
	solid        bool
	roughness    float64
	fillWeight   float64
	// tint says that the fill is a gradient that draws over a tint of
	// itself, with a hachure of SketchGradientGap inside the outline
	// (D50).
	tint bool
}

// tinted sets whether a gradient fill of a shape w by h draws over a tint
// of itself: a shape at least SketchGradientLeast across does, and a
// thinner shape, such as the bar of a legend, keeps the dense hachure
// (D50).
func (ps *paints) tinted(w, h float64) {
	ps.tint = ps.fill.Gradient != nil && !ps.solid && math.Min(w, h) >= theme.SketchGradientLeast
	if ps.tint {
		// The lines of the hachure take the weight of every hachure, and
		// not the weight of a cluster, whose lines touch, so that the
		// tint shows between them.
		ps.fillWeight = theme.SketchFillWeight
	}
}

// paintsOf returns the paints of an item. fill is the color of a fill that
// the item does not set, empty for none. A gradient stays a gradient, so
// the lines of a hachure take its colors. The role of the mark sets the
// roughness and the weight of the hachure.
func (p *pass) paintsOf(m *scene.Mark, it *scene.Item, fill string) paints {
	ps := paints{roughness: theme.SketchRoughness, fillWeight: theme.SketchFillWeight}
	// An edge of a graph and the box of a cluster draw as Mermaid 12.3
	// draws them for look: handDrawn: an edge with a lower roughness, and
	// a cluster with thicker lines of hachure, which look solid.
	switch m.Role {
	case scene.RoleEdge:
		ps.roughness = theme.SketchEdgeRoughness
	case scene.RoleCluster:
		ps.fillWeight = theme.SketchClusterFillWeight
	}
	ps.fill = scene.Paint{Color: fill}
	if !it.Fill.IsZero() {
		ps.fill = paintOf(it.Fill)
	}
	if it.FillOpacity != nil && *it.FillOpacity <= 0 {
		ps.fill = scene.Paint{}
	}
	// A fill in the color of the background hides what is under it, such
	// as the box under the label of an edge. A hachure does not hide it,
	// so such a fill draws solid.
	ps.solid = ps.fill.Gradient == nil && ps.fill.Color != "" && sameColor(ps.fill.Color, p.bg)
	// A solid node of a graph, such as the start of a state diagram, fills
	// solid, because a hachure draws it as a pale scribble (D55).
	ps.solid = ps.solid || m.Role == scene.RoleSolid && !ps.fill.IsZero()
	ps.stroke = paintOf(it.Stroke)
	ps.width = theme.SketchStroke
	if it.StrokeWidth != nil {
		ps.width = *it.StrokeWidth
	}
	if ps.width <= 0 || (it.StrokeOpacity != nil && *it.StrokeOpacity <= 0) {
		ps.stroke = scene.Paint{}
	}
	return ps
}

// calm lowers the roughness of a small shape, as Excalidraw does for a
// small element, so that a dot, a small square or a tick keeps its shape
// and does not draw as a scribble ("The look like Excalidraw" in
// docs/STYLE.md). w and h are the size of the shape. A shape keeps its
// roughness when it is at least 20 by 50, a round shape, such as a
// circle, when it is at least 15 by 15, and a line when it is at least
// 150 long. Each other shape takes a half of the roughness, or a third
// when it is shorter than 10. An edge of a graph keeps the roughness that
// Mermaid gives it. A shape shorter than 10 also fills solid, because a
// hachure has room for only one to four short lines in it, which draw a
// dot as a dash.
func (ps *paints) calm(m *scene.Mark, w, h float64, round, line bool) {
	if m.Role == scene.RoleEdge {
		return
	}
	lo, hi := math.Min(w, h), math.Max(w, h)
	if lo >= 20 && hi >= 50 || round && lo >= 15 || line && hi >= 150 {
		return
	}
	if hi >= 10 {
		ps.roughness /= 2
		return
	}
	ps.roughness /= 3
	ps.solid = ps.solid || !ps.fill.IsZero()
}

// paintOf returns a paint of an item, or the zero paint for none: a color
// that is none or transparent, or a gradient with no stops. A gradient
// stays a gradient. Each renderer spans it over the bounds of the path
// that it paints, so the lines of a hachure, which fill the shape from
// edge to edge, draw its colors where the shape draws them (D40 part 8).
func paintOf(pt scene.Paint) scene.Paint {
	if pt.Gradient != nil {
		if len(pt.Gradient.Stops) == 0 {
			return scene.Paint{}
		}
		return pt
	}
	switch strings.ToLower(strings.TrimSpace(pt.Color)) {
	case "", "none", "transparent":
		return scene.Paint{}
	}
	return pt
}

// options returns the options of rough.js for a shape, with the fill and
// the stroke that rough.js draws or not. rough.js draws a fill or a stroke
// whose paint is not zero, and the parts take the paints of the item. The
// fill is a hachure, or solid when solid says so.
func (ps paints) options(seed uint32) *rough.Options {
	o := &rough.Options{
		Roughness:    new(ps.roughness),
		FillStyle:    new("hachure"),
		FillWeight:   new(ps.fillWeight),
		HachureGap:   new(ps.gap()),
		HachureAngle: new(theme.SketchHachureAngle),
		Seed:         new(float64(seed)),
		Stroke:       new("none"),
	}
	if !ps.fill.IsZero() {
		o.Fill = new("fill")
		if ps.solid {
			o.FillStyle = new("solid")
		}
	}
	if !ps.stroke.IsZero() {
		o.Stroke = new("stroke")
	}
	return o
}

// gap returns the space between the lines of the hachure.
func (ps paints) gap() float64 {
	if ps.tint {
		return theme.SketchGradientGap
	}
	return theme.SketchHachureGap
}

// fillOnly and strokeOnly return the paints with the stroke or the fill
// taken away.
func (ps paints) fillOnly() paints {
	ps.stroke = scene.Paint{}
	return ps
}

func (ps paints) strokeOnly() paints {
	ps.fill = scene.Paint{}
	return ps
}

// parts returns the items that draw the sets of a shape of rough.js: the
// lines of its hachure in the paint of the fill, and its outline in the
// paint of the stroke. Each set is one path, so that a stroke with an
// alpha does not grow darker where its lines cross. The parts keep the
// opacity, the link, the tooltip and the datum of the item.
func (p *pass) parts(it *scene.Item, ps paints, d rough.Drawable, x, y, angle float64, f frame) []scene.Item {
	var out []scene.Item
	for _, set := range d.Sets {
		data := pathData(set.Ops)
		if data == "" {
			continue
		}
		part := scene.Item{
			X: x, Y: y, Angle: angle, Path: data, Opacity: it.Opacity,
			Href: it.Href, Tooltip: it.Tooltip, Datum: it.Datum,
		}
		switch set.Type {
		case rough.OpSetFillSketch:
			w := ps.fillWeight
			part.Stroke, part.StrokeWidth, part.StrokeOpacity = ps.fill, &w, it.FillOpacity
		case rough.OpSetFillPath:
			part.Fill, part.FillOpacity = ps.fill, it.FillOpacity
		case rough.OpSetPath:
			w := ps.width
			part.Stroke, part.StrokeWidth, part.StrokeOpacity = ps.stroke, &w, it.StrokeOpacity
			part.StrokeDash, part.StrokeCap, part.StrokeJoin = it.StrokeDash, it.StrokeCap, it.StrokeJoin
		default:
			continue
		}
		p.measure(&part, f)
		out = append(out, part)
	}
	return out
}

// rect draws a rect item: a rectangle of rough.js, or the path of its
// round corners. A rect thinner than a hachure draws its fill as one line
// along its length.
func (p *pass) rect(m *scene.Mark, it *scene.Item, seed uint32, f frame) []scene.Item {
	x, y, wd, ht := shape.Box(it)
	if !(wd > 0) || !(ht > 0) {
		return nil
	}
	ps := p.paintsOf(m, it, "")
	tl, tr, br, bl := shape.Corners(it)
	rounded := tl > 0 || tr > 0 || br > 0 || bl > 0
	ps.calm(m, wd, ht, rounded, false)
	if !ps.fill.IsZero() && !ps.solid && math.Min(wd, ht) < thin {
		// The fill draws as a line as thick as the rect, and the outline
		// draws around it.
		line := *it
		w := math.Min(wd, ht)
		line.Stroke, line.StrokeWidth, line.StrokeOpacity, line.StrokeDash = ps.fill, &w, it.FillOpacity, nil
		x1, y1, x2, y2 := x, y+ht/2, x+wd, y+ht/2
		if ht > wd {
			x1, y1, x2, y2 = x+wd/2, y, x+wd/2, y+ht
		}
		lp := paints{stroke: ps.fill, width: w, roughness: ps.roughness, fillWeight: ps.fillWeight}
		out := p.parts(&line, lp, p.gen.Line(x1, y1, x2, y2, lp.options(seed)), 0, 0, 0, f)
		if !ps.stroke.IsZero() {
			ps.fill = scene.Paint{}
			out = append(out, p.parts(it, ps, p.gen.Rectangle(x, y, wd, ht, ps.options(next(seed))), 0, 0, 0, f)...)
		}
		return out
	}
	if ps.fill.IsZero() && ps.stroke.IsZero() {
		return nil
	}
	if rounded {
		return p.path(m, it, shape.RoundRect(x, y, wd, ht, tl, tr, br, bl), "", 0, 0, 0, seed, f)
	}
	ps.tinted(wd, ht)
	out := p.underlay(it, ps, func(o *rough.Options) rough.Drawable { return p.gen.Rectangle(x, y, wd, ht, o) }, seed, 0, 0, 0, f)
	if ps.tint {
		// The hachure fills the rect inside the outline, and the outline
		// draws around the whole rect.
		p.addTint(ps, fillAlpha(it, ps), x, y, x+wd, y+ht, f)
		i := theme.SketchGradientInset
		fp := ps.fillOnly()
		out = append(out, p.parts(it, fp, p.gen.Rectangle(x+i, y+i, wd-2*i, ht-2*i, fp.options(seed)), 0, 0, 0, f)...)
		if !ps.stroke.IsZero() {
			sp := ps.strokeOnly()
			out = append(out, p.parts(it, sp, p.gen.Rectangle(x, y, wd, ht, sp.options(seed)), 0, 0, 0, f)...)
		}
		return out
	}
	p.addCover(ps, x, y, x+wd, y+ht, f)
	return append(out, p.parts(it, ps, p.gen.Rectangle(x, y, wd, ht, ps.options(seed)), 0, 0, 0, f)...)
}

// rule draws a rule item as a line of rough.js.
func (p *pass) rule(m *scene.Mark, it *scene.Item, seed uint32, f frame) []scene.Item {
	ps := p.paintsOf(m, it, "")
	if ps.stroke.IsZero() {
		return nil
	}
	x2, y2 := shape.RuleEnd(it)
	ps.fill = scene.Paint{}
	ps.calm(m, math.Abs(x2-it.X), math.Abs(y2-it.Y), false, true)
	return p.parts(it, ps, p.gen.Line(it.X, it.Y, x2, y2, ps.options(seed)), 0, 0, 0, f)
}

// symbol draws a symbol item: a circle as an ellipse of rough.js, and each
// other shape as its path.
func (p *pass) symbol(m *scene.Mark, it *scene.Item, seed uint32, f frame) []scene.Item {
	if it.Shape == "" || it.Shape == "circle" {
		size := 64.0
		if it.Size != nil {
			size = *it.Size
		}
		ps := p.paintsOf(m, it, "")
		if size <= 0 || (ps.fill.IsZero() && ps.stroke.IsZero()) {
			return nil
		}
		d := math.Sqrt(size)
		ps.calm(m, d, d, true, false)
		ps.tinted(d, d)
		out := p.underlay(it, ps, func(o *rough.Options) rough.Drawable { return p.gen.Ellipse(0, 0, d, d, o) }, seed, it.X, it.Y, it.Angle, f)
		if ps.tint {
			p.addTint(ps, fillAlpha(it, ps), it.X-d/2, it.Y-d/2, it.X+d/2, it.Y+d/2, f)
			fp, in := ps.fillOnly(), d-2*theme.SketchGradientInset
			out = append(out, p.parts(it, fp, p.gen.Ellipse(0, 0, in, in, fp.options(seed)), it.X, it.Y, it.Angle, f)...)
			if !ps.stroke.IsZero() {
				sp := ps.strokeOnly()
				out = append(out, p.parts(it, sp, p.gen.Ellipse(0, 0, d, d, sp.options(seed)), it.X, it.Y, it.Angle, f)...)
			}
			return out
		}
		p.addCover(ps, it.X-d/2, it.Y-d/2, it.X+d/2, it.Y+d/2, f)
		return append(out, p.parts(it, ps, p.gen.Ellipse(0, 0, d, d, ps.options(seed)), it.X, it.Y, it.Angle, f)...)
	}
	d, ok := shape.SymbolOf(it)
	if !ok {
		return nil
	}
	return p.path(m, it, d, "", it.X, it.Y, it.Angle, seed, f)
}

// underlay returns the parts of a solid fill in the color of the
// background under the hachure of an item, so that the item hides what is
// under it, as its fill does in the standard look. Without it, a mark
// under the item shows through the gaps of the hachure ("The look like
// Excalidraw" in docs/STYLE.md). The underlay takes the opacity of the fill, so a fill
// that lets a mark show through in the standard look lets it show through
// as much here. draw draws the shape of the item with the options of
// rough.js that it takes, and the underlay wobbles as the item does. An
// item with no hachure, and a canvas with no opaque background, take no
// underlay.
func (p *pass) underlay(it *scene.Item, ps paints, draw func(*rough.Options) rough.Drawable, seed uint32, x, y, angle float64, f frame) []scene.Item {
	if ps.fill.IsZero() || ps.solid {
		return nil
	}
	a := fillAlpha(it, ps)
	if a <= 0 {
		return nil
	}
	var out []scene.Item
	if bg, ok := color.Parse(p.bg); ok && bg.A >= 1 {
		u := *it
		u.FillOpacity = nil
		if a < 1 {
			u.FillOpacity = &a
		}
		up := ps
		up.fill, up.stroke, up.solid, up.tint = scene.Paint{Color: p.bg}, scene.Paint{}, true, false
		out = p.parts(&u, up, draw(up.options(seed)), x, y, angle, f)
	}
	if ps.tint {
		// A gradient draws over a tint of itself, in the shape of the
		// item, with the same wobble (D50).
		u := *it
		t := a * theme.SketchGradientTint
		u.FillOpacity = &t
		tp := ps
		tp.stroke, tp.solid, tp.tint = scene.Paint{}, true, false
		out = append(out, p.parts(&u, tp, draw(tp.options(seed)), x, y, angle, f)...)
	}
	return out
}

// flatUnderlay returns the underlay and the tint of an item as underlay
// draws them, but filling the path data d as it is, with no wobble.
func (p *pass) flatUnderlay(it *scene.Item, ps paints, d string, x, y, angle float64, f frame) []scene.Item {
	if ps.fill.IsZero() || ps.solid {
		return nil
	}
	a := fillAlpha(it, ps)
	if a <= 0 {
		return nil
	}
	part := func(fill scene.Paint, opacity float64) scene.Item {
		u := scene.Item{
			X: x, Y: y, Angle: angle, Path: d, Fill: fill, Opacity: it.Opacity,
			Href: it.Href, Tooltip: it.Tooltip, Datum: it.Datum,
		}
		if opacity < 1 {
			u.FillOpacity = &opacity
		}
		p.measure(&u, f)
		return u
	}
	var out []scene.Item
	if bg, ok := color.Parse(p.bg); ok && bg.A >= 1 {
		out = append(out, part(scene.Paint{Color: p.bg}, a))
	}
	if ps.tint {
		out = append(out, part(ps.fill, a*theme.SketchGradientTint))
	}
	return out
}

// inset returns a polygon moved toward the middle of its box by d at each
// side: each point scales toward the middle, so that the polygon is 2d
// narrower and 2d lower. A box less than 2d across keeps the polygon.
func inset(poly []rough.Point, b box, d float64) []rough.Point {
	w, h := b.x2-b.x1, b.y2-b.y1
	if w <= 2*d || h <= 2*d {
		return poly
	}
	cx, cy := (b.x1+b.x2)/2, (b.y1+b.y2)/2
	sx, sy := (w-2*d)/w, (h-2*d)/h
	out := make([]rough.Point, len(poly))
	for i, pt := range poly {
		out[i] = rough.Point{cx + (pt[0]-cx)*sx, cy + (pt[1]-cy)*sy}
	}
	return out
}

// fillAlpha returns the opacity of the fill of an item: the alpha of its
// paint times its fill opacity.
func fillAlpha(it *scene.Item, ps paints) float64 {
	a := alpha(ps.fill)
	if it.FillOpacity != nil {
		a *= *it.FillOpacity
	}
	return a
}

// alpha returns the opacity of a paint: the alpha of its color, or the
// largest alpha of the stops of its gradient.
func alpha(pt scene.Paint) float64 {
	if pt.Gradient == nil {
		c, ok := color.Parse(pt.Color)
		if !ok {
			return 1
		}
		return c.A
	}
	a := 0.0
	for _, st := range pt.Gradient.Stops {
		if c, ok := color.Parse(st.Color); ok {
			a = math.Max(a, c.A)
		}
	}
	return a
}

// path draws path data: its fill as the hachure of a polygon through its
// subpaths, and its stroke as the path of rough.js. fill is the color of a
// fill that the item does not set, empty for none. The parts take the
// place x and y and the angle.
func (p *pass) path(m *scene.Mark, it *scene.Item, d, fill string, x, y, angle float64, seed uint32, f frame) []scene.Item {
	if d == "" {
		return nil
	}
	ps := p.paintsOf(m, it, fill)
	var b pointBox
	b.box = box{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	_ = shape.Parse(d, &b)
	if b.x1 <= b.x2 {
		// A line, or a path with no fill, is a line as Excalidraw counts
		// it.
		ps.calm(m, b.x2-b.x1, b.y2-b.y1, false, m.Type == scene.Line || ps.fill.IsZero())
	}
	var out []scene.Item
	if !ps.fill.IsZero() {
		if poly := polygon(d); len(poly) >= 3 {
			only := ps.fillOnly()
			if b.x1 <= b.x2 {
				only.tinted(b.x2-b.x1, b.y2-b.y1)
			}
			if rings(d) > 1 {
				// rough.js wobbles the two lines of each bridge between
				// the rings apart, so a solid fill of the bridged polygon
				// draws a sliver along each bridge. The underlay and the
				// tint of a path of several rings fill the path as it is,
				// as the standard look fills it, so a hole stays a hole.
				out = p.flatUnderlay(it, only, d, x, y, angle, f)
			} else {
				out = p.underlay(it, only, func(o *rough.Options) rough.Drawable { return p.gen.Polygon(poly, o) }, next(seed), x, y, angle, f)
			}
			hachure := poly
			if only.tint {
				hachure = inset(poly, b.box, theme.SketchGradientInset)
				p.addTint(only, fillAlpha(it, only), x+b.x1, y+b.y1, x+b.x2, y+b.y2, f)
			} else if b.x1 <= b.x2 {
				p.addCover(only, x+b.x1, y+b.y1, x+b.x2, y+b.y2, f)
			}
			// SVG fills a path by the rule nonzero, so the hachure does too.
			// Two subpaths that overlap and run the same way fill their
			// overlap, and a subpath that runs the other way is a hole.
			o := only.options(next(seed))
			o.FillRule = new("nonzero")
			out = append(out, p.parts(it, only, p.gen.Polygon(hachure, o), x, y, angle, f)...)
		}
	}
	if !ps.stroke.IsZero() {
		only := ps
		only.fill = scene.Paint{}
		out = append(out, p.parts(it, only, p.gen.Path(absolute(d), only.options(seed)), x, y, angle, f)...)
	}
	return out
}

// measure adds the bounds of a part to the bounds of the pass, unless a
// clip holds it.
func (p *pass) measure(part *scene.Item, f frame) {
	if f.clipped {
		return
	}
	var b pointBox
	b.box = box{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	_ = shape.Parse(part.Path, &b)
	if b.x1 > b.x2 {
		return
	}
	half := 0.0
	if part.StrokeWidth != nil {
		half = *part.StrokeWidth / 2
	}
	sin, cos := math.Sincos(part.Angle * math.Pi / 180)
	for _, c := range [][2]float64{{b.x1, b.y1}, {b.x2, b.y1}, {b.x1, b.y2}, {b.x2, b.y2}} {
		px := c[0]*cos - c[1]*sin + part.X + f.x
		py := c[0]*sin + c[1]*cos + part.Y + f.y
		p.bounds.add(px-half, py-half)
		p.bounds.add(px+half, py+half)
	}
}

// grow grows the canvas to hold the bounds of the pass, and moves the
// root where a line wobbles out of its left or its top. A side grows by at
// most SketchSlack, the most that a line wobbles. A mark that reaches
// further past a side, such as the land of a map on a canvas that the spec
// fixes, stays cut there, as in the standard look.
func (p *pass) grow(s *scene.Scene) {
	b := p.bounds
	if b.x1 > b.x2 {
		return
	}
	side := func(over float64) float64 {
		if over <= 0 || over > theme.SketchSlack {
			return 0
		}
		return math.Ceil(over)
	}
	dx, dy := side(-b.x1), side(-b.y1)
	right, bottom := side(b.x2-s.Width), side(b.y2-s.Height)
	if dx == 0 && dy == 0 && right == 0 && bottom == 0 {
		return
	}
	for i := range s.Root.Items {
		s.Root.Items[i].X += dx
		s.Root.Items[i].Y += dy
	}
	s.Width += dx + right
	s.Height += dy + bottom
}

// pointBox is a sink of shape.Parse that holds the bounds of the points of
// a path and of its control points, which hold its curves.
type pointBox struct {
	box
}

func (b *pointBox) MoveTo(x, y float64) { b.add(x, y) }
func (b *pointBox) LineTo(x, y float64) { b.add(x, y) }
func (b *pointBox) QuadTo(x1, y1, x, y float64) {
	b.add(x1, y1)
	b.add(x, y)
}

func (b *pointBox) CubicTo(x1, y1, x2, y2, x, y float64) {
	b.add(x1, y1)
	b.add(x2, y2)
	b.add(x, y)
}
func (b *pointBox) Close() {}

// sameColor reports whether two colors of CSS are the same opaque color.
func sameColor(a, b string) bool {
	ca, oka := color.Parse(a)
	cb, okb := color.Parse(b)
	return oka && okb && ca.A >= 1 && ca == cb
}
