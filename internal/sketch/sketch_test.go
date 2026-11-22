package sketch

import (
	"math"
	"strings"
	"testing"

	"github.com/xo/zuzu/internal/shape"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/theme"
)

// root returns a scene of a size with the marks in its root group.
func root(w, h float64, marks ...scene.Mark) *scene.Scene {
	return &scene.Scene{
		Width: w, Height: h, Background: scene.Paint{Color: "#ffffff"},
		Root: scene.Mark{Type: scene.Group, Role: "frame", Items: []scene.Item{{X: 10, Y: 10, Width: w - 20, Height: h - 20, Items: marks}}},
	}
}

// withoutUnderlay returns the parts of a mark without the underlays, the
// solid fills in the color of the background of root.
func withoutUnderlay(items []scene.Item) []scene.Item {
	var out []scene.Item
	for _, it := range items {
		if it.Fill.Color != "#ffffff" {
			out = append(out, it)
		}
	}
	return out
}

// marksOf returns the marks of the root group of a scene.
func marksOf(s *scene.Scene) []scene.Mark {
	return s.Root.Items[0].Items
}

// TestSeedsAreInRange checks that each seed of an item is from 1 to
// 2^31-1, so that rough.js never takes a random seed, and that the seed
// follows the path of the mark and the index (D39 part 6).
func TestSeedsAreInRange(t *testing.T) {
	t.Parallel()
	for _, seed := range []int64{1, 7, -3, 1 << 40} {
		p := &pass{seed: seed}
		for i := range 2000 {
			s := p.itemSeed("/marks~"+strings.Repeat("x", i%7), i)
			if s < 1 || s > 1<<31-1 {
				t.Fatalf("the seed %d of item %d is out of range", s, i)
			}
			if n := next(s); n < 1 || n > 1<<31-1 {
				t.Fatalf("the next seed %d of %d is out of range", n, s)
			}
		}
	}
	p := &pass{seed: 1}
	if p.itemSeed("/a", 0) == p.itemSeed("/b", 0) || p.itemSeed("/a", 0) == p.itemSeed("/a", 1) {
		t.Error("two items take the same seed")
	}
	if q := (&pass{seed: 2}); q.itemSeed("/a", 0) == p.itemSeed("/a", 0) {
		t.Error("two seeds of a chart give an item the same seed")
	}
}

// TestEachMarkTurnsIntoPaths checks that the pass turns each kind of mark
// into a mark of paths with the same name, role and zindex, and keeps a
// text, an image and the outline of a formula (D40 part 4).
func TestEachMarkTurnsIntoPaths(t *testing.T) {
	t.Parallel()
	stroke := scene.Paint{Color: "#333333"}
	fill := scene.Paint{Color: "#5b6ee1"}
	marks := []scene.Mark{
		{Type: scene.Rect, Name: "rect", ZIndex: 2, Items: []scene.Item{{X: 1, Y: 1, Width: 30, Height: 20, Fill: fill, Stroke: stroke}}},
		{Type: scene.Rect, Name: "round", Items: []scene.Item{{X: 1, Y: 1, Width: 30, Height: 20, CornerRadius: 4, Fill: fill}}},
		{Type: scene.Rule, Name: "rule", Items: []scene.Item{{X: 0, Y: 0, X2: 50, Y2: 0, Stroke: stroke}}},
		{Type: scene.Symbol, Name: "circle", Items: []scene.Item{{X: 5, Y: 5, Size: new(100.0), Fill: fill}}},
		{Type: scene.Symbol, Name: "diamond", Items: []scene.Item{{X: 5, Y: 5, Shape: "diamond", Size: new(100.0), Fill: fill, Stroke: stroke}}},
		{Type: scene.Arc, Name: "arc", Items: []scene.Item{{X: 40, Y: 40, OuterRadius: 30, InnerRadius: 10, EndAngle: 2, Fill: fill}}},
		{Type: scene.Line, Name: "line", Items: []scene.Item{{X: 0, Y: 0, Stroke: stroke}, {X: 20, Y: 10}, {X: 40, Y: 0}}},
		{Type: scene.Area, Name: "area", Items: []scene.Item{{X: 0, Y: 0, Y2: 30, Fill: fill}, {X: 20, Y: 10, Y2: 30}, {X: 40, Y: 0, Y2: 30}}},
		{Type: scene.Path, Name: "path", Items: []scene.Item{{X: 3, Y: 4, Angle: 30, Path: "M0,0L20,0L10,15Z", Fill: fill, Stroke: stroke}}},
		{Type: scene.Path, Role: scene.RoleMath, Name: "math", Items: []scene.Item{{Path: "M0,0L5,5Z", Fill: fill}}},
		{Type: scene.Text, Name: "text", Items: []scene.Item{{X: 5, Y: 5, Text: scene.Lines{"hello"}}}},
		{Type: scene.Image, Name: "image", Items: []scene.Item{{X: 5, Y: 5, Width: 4, Height: 4, URL: "data:image/png;base64,"}}},
	}
	s := root(200, 200, marks...)
	Draw(s, Options{Font: "'Excalifont', sans-serif"})
	out := marksOf(s)
	if len(out) != len(marks) {
		t.Fatalf("%d marks, want %d", len(out), len(marks))
	}
	for i, m := range out {
		in := marks[i]
		if m.Name != in.Name || m.Role != in.Role || m.ZIndex != in.ZIndex {
			t.Errorf("the mark %q became %q, role %q, zindex %d", in.Name, m.Name, m.Role, m.ZIndex)
		}
		want := scene.Path
		switch {
		case in.Type == scene.Text || in.Type == scene.Image:
			want = in.Type
		case in.Role == scene.RoleMath:
			if m.Items[0].Path != in.Items[0].Path {
				t.Error("the outline of a formula changes")
			}
		}
		if m.Type != want {
			t.Errorf("the mark %q has the type %s, want %s", in.Name, m.Type, want)
		}
		if len(m.Items) == 0 {
			t.Errorf("the mark %q draws nothing", in.Name)
		}
	}
	if f := out[10].Items[0].Font; f != "'Excalifont', sans-serif" {
		t.Errorf("the text has the family %q", f)
	}
	// The rect draws its hachure and then its outline, each one path.
	rect := withoutUnderlay(out[0].Items)
	if len(rect) != 2 || rect[0].Stroke.Color != "#5b6ee1" || *rect[0].StrokeWidth != 1.5 || rect[1].Stroke.Color != "#333333" || *rect[1].StrokeWidth != 1.3 {
		t.Errorf("the rect draws %+v", rect)
	}
	// The path keeps its place and its angle.
	for _, it := range out[8].Items {
		if it.X != 3 || it.Y != 4 || it.Angle != 30 {
			t.Errorf("a part of the path is at %g,%g at %g degrees", it.X, it.Y, it.Angle)
		}
	}
}

// TestTheOrderOfTheItemsStays checks that each item draws its fill and
// then its stroke, in the order of the items, with its link, its tooltip,
// its opacity and its dash.
func TestTheOrderOfTheItemsStays(t *testing.T) {
	t.Parallel()
	var items []scene.Item
	for i, c := range []string{"#111111", "#222222", "#333333"} {
		items = append(items, scene.Item{
			X: float64(i * 40), Y: 0, Width: 30, Height: 30, Fill: scene.Paint{Color: c}, Stroke: scene.Paint{Color: c},
			Href: "https://example.com", Tooltip: "tip", Opacity: new(0.5), StrokeDash: []float64{4, 2},
		})
	}
	s := root(200, 100, scene.Mark{Type: scene.Rect, Items: items})
	Draw(s, Options{})
	got := marksOf(s)[0].Items
	if len(got) != 9 {
		t.Fatalf("%d parts, want 9", len(got))
	}
	for i := 0; i < len(got); i += 3 {
		if got[i].Fill.Color != "#ffffff" {
			t.Errorf("the part %d is %+v, want the underlay of an item", i, got[i])
		}
	}
	got = withoutUnderlay(got)
	for i, it := range got {
		c := items[i/2].Fill.Color
		if it.Stroke.Color != c || it.Href != "https://example.com" || it.Tooltip != "tip" || *it.Opacity != 0.5 {
			t.Errorf("the part %d is %+v", i, it)
		}
		if i%2 == 1 && len(it.StrokeDash) != 2 {
			t.Errorf("the outline %d has the dash %v", i, it.StrokeDash)
		}
	}
}

// TestAHoleStaysEmpty checks that the hachure of a ring leaves its hole
// empty, as a fill of SVG does.
func TestAHoleStaysEmpty(t *testing.T) {
	t.Parallel()
	ring := "M0,0H60V60H0Z M15,15V45H45V15Z"
	s := root(100, 100, scene.Mark{Type: scene.Path, Items: []scene.Item{{Path: ring, Fill: scene.Paint{Color: "#5b6ee1"}}}})
	Draw(s, Options{})
	parts := withoutUnderlay(marksOf(s)[0].Items)
	if len(parts) != 1 {
		t.Fatalf("%d parts, want 1", len(parts))
	}
	var pts pointList
	if err := shape.Parse(parts[0].Path, &pts); err != nil {
		t.Fatal(err)
	}
	if len(pts) < 20 {
		t.Fatalf("the hachure has %d points", len(pts))
	}
	for _, p := range pts {
		if p[0] > 19 && p[0] < 41 && p[1] > 19 && p[1] < 41 {
			t.Fatalf("the hachure reaches %v, inside the hole", p)
		}
	}
}

// pointList is a sink of shape.Parse that holds the ends of each segment.
type pointList [][2]float64

func (l *pointList) MoveTo(x, y float64) { *l = append(*l, [2]float64{x, y}) }
func (l *pointList) LineTo(x, y float64) { *l = append(*l, [2]float64{x, y}) }
func (l *pointList) QuadTo(_, _, x, y float64) {
	*l = append(*l, [2]float64{x, y})
}

func (l *pointList) CubicTo(_, _, _, _, x, y float64) {
	*l = append(*l, [2]float64{x, y})
}
func (l *pointList) Close() {}

// TestAGroupDrawsItsBackgroundFirst checks that the background of a group
// becomes the first of its marks, under the others, and that the group no
// longer draws it.
func TestAGroupDrawsItsBackgroundFirst(t *testing.T) {
	t.Parallel()
	inner := scene.Mark{Type: scene.Rect, ZIndex: -1, Items: []scene.Item{{Width: 10, Height: 10, Fill: scene.Paint{Color: "red"}}}}
	g := scene.Mark{Type: scene.Group, Items: []scene.Item{{X: 5, Y: 5, Width: 50, Height: 40, Fill: scene.Paint{Color: "#eeeeee"}, Stroke: scene.Paint{Color: "#999999"}, CornerRadius: 4, Items: []scene.Mark{inner}}}}
	s := root(100, 100, g)
	Draw(s, Options{})
	it := marksOf(s)[0].Items[0]
	if !it.Fill.IsZero() || !it.Stroke.IsZero() {
		t.Error("the group still draws its background")
	}
	if len(it.Items) != 2 || it.Items[0].Role != RoleBackground || it.Items[0].ZIndex > -1 {
		t.Fatalf("the marks of the group are %+v", it.Items)
	}
}

// TestTheCanvasHoldsTheWobble checks that the canvas grows, and the root
// moves, where a stroke wobbles out of it (D39 part 7).
func TestTheCanvasHoldsTheWobble(t *testing.T) {
	t.Parallel()
	s := &scene.Scene{Width: 40, Height: 30, Root: scene.Mark{Type: scene.Group, Items: []scene.Item{{Items: []scene.Mark{{
		Type: scene.Rect, Items: []scene.Item{{Width: 40, Height: 30, Stroke: scene.Paint{Color: "#000"}, StrokeWidth: new(4.0)}},
	}}}}}}
	Draw(s, Options{})
	if s.Width <= 40 || s.Height <= 30 || s.Root.Items[0].X <= 0 || s.Root.Items[0].Y <= 0 {
		t.Errorf("the canvas is %gx%g and the root at %g,%g", s.Width, s.Height, s.Root.Items[0].X, s.Root.Items[0].Y)
	}
}

// TestThinAndSolidFills checks that a rect thinner than a hachure draws
// one line, and that a fill in the color of the background draws solid.
func TestThinAndSolidFills(t *testing.T) {
	t.Parallel()
	s := root(200, 100,
		scene.Mark{Type: scene.Rect, Name: "thin", Items: []scene.Item{{X: 0, Y: 10, Width: 80, Height: 1, Fill: scene.Paint{Color: "#5b6ee1"}}}},
		scene.Mark{Type: scene.Rect, Name: "box", Items: []scene.Item{{X: 0, Y: 30, Width: 40, Height: 20, Fill: scene.Paint{Color: "white"}}}},
	)
	Draw(s, Options{})
	thin, box := marksOf(s)[0].Items, marksOf(s)[1].Items
	if len(thin) != 1 || thin[0].Stroke.Color != "#5b6ee1" || *thin[0].StrokeWidth != 1 {
		t.Errorf("the thin rect draws %+v", thin)
	}
	if len(box) != 1 || box[0].Fill.Color != "white" || !box[0].Stroke.IsZero() {
		t.Errorf("the box in the color of the background draws %+v", box)
	}
}

// TestGradients checks that a gradient stays a gradient: the lines of the
// hachure of a fill and the outline of a stroke take the gradient, so the
// look keeps its colors (D40 part 8).
func TestGradients(t *testing.T) {
	t.Parallel()
	g := &scene.Gradient{Type: scene.Linear, X2: 1, Stops: []scene.Stop{{Color: "#ff0000"}, {Offset: 1, Color: "#0000ff"}}}
	items := []scene.Item{
		{Width: 60, Height: 16, Fill: scene.Paint{Gradient: g}},
		{X: 70, Width: 20, Height: 20, Stroke: scene.Paint{Gradient: g}},
		{X: 100, Width: 20, Height: 20, Fill: scene.Paint{Gradient: &scene.Gradient{Type: scene.Linear}}},
	}
	s := root(140, 60, scene.Mark{Type: scene.Rect, Name: "bars", Items: items})
	Draw(s, Options{})
	got := withoutUnderlay(marksOf(s)[0].Items)
	if len(got) != 2 {
		t.Fatalf("the rects draw %d parts, want 2: a gradient with no stops draws nothing", len(got))
	}
	if !got[0].Fill.IsZero() || got[0].Stroke.Gradient != g {
		t.Errorf("the hachure has the stroke %+v, want the gradient", got[0].Stroke)
	}
	if got[1].Stroke.Gradient != g {
		t.Errorf("the outline has the stroke %+v, want the gradient", got[1].Stroke)
	}
}

// TestGradientOverATint checks that a shape at least SketchGradientLeast
// across draws its gradient over a tint of itself, at SketchGradientTint
// of the opacity of the fill, then its hachure of the gradient, with the
// gap of Excalidraw and inside the outline, then its outline (D50).
func TestGradientOverATint(t *testing.T) {
	t.Parallel()
	g := &scene.Gradient{Type: scene.Linear, X2: 1, Stops: []scene.Stop{{Color: "#ff0000"}, {Offset: 1, Color: "#0000ff"}}}
	box := scene.Item{X: 10, Y: 10, Width: 60, Height: 40, Fill: scene.Paint{Gradient: g}, Stroke: scene.Paint{Color: "#5b6ee1"}}
	s := root(100, 80, scene.Mark{Type: scene.Rect, Items: []scene.Item{box}})
	Draw(s, Options{})
	got := withoutUnderlay(marksOf(s)[0].Items)
	if len(got) != 3 {
		t.Fatalf("the rect draws %d parts, want the tint, the hachure and the outline", len(got))
	}
	tint, hachure, outline := got[0], got[1], got[2]
	if tint.Fill.Gradient != g || tint.FillOpacity == nil || *tint.FillOpacity != theme.SketchGradientTint {
		t.Errorf("the tint has the fill %+v at %v, want the gradient at %v", tint.Fill, tint.FillOpacity, theme.SketchGradientTint)
	}
	if hachure.Stroke.Gradient != g {
		t.Errorf("the hachure has the stroke %+v, want the gradient", hachure.Stroke)
	}
	if outline.Stroke.Color != "#5b6ee1" {
		t.Errorf("the outline has the stroke %+v, want #5b6ee1", outline.Stroke)
	}
	// The hachure stays inside the outline: no point of it reaches past
	// the rect less the inset and the wobble of rough.js.
	var b pointBox
	b.box = box2()
	_ = shape.Parse(hachure.Path, &b)
	if b.x1 < box.X || b.x2 > box.X+box.Width || b.y1 < box.Y || b.y2 > box.Y+box.Height {
		t.Errorf("the hachure reaches %v,%v to %v,%v, past the rect", b.x1, b.y1, b.x2, b.y2)
	}
	// The lines of the hachure stand SketchGradientGap apart, so a 40 high
	// rect at -41 degrees holds far fewer lines than at the gap of 1.5.
	if n := strings.Count(hachure.Path, "M"); n > 40 {
		t.Errorf("the hachure has %d lines, want the sparse lines of the gap %v", n, theme.SketchGradientGap)
	}
}

// TestTextOnATint checks that a label of a graph that sits on a shape
// drawn over a tint takes the color of text that reads on the tint, with a
// halo, that a label on a plain fill inside it keeps its color, and that a
// color of text that the spec gives stays (D50).
func TestTextOnATint(t *testing.T) {
	t.Parallel()
	g := &scene.Gradient{Type: scene.Linear, X2: 1, Stops: []scene.Stop{{Color: "#ff0000"}, {Offset: 1, Color: "#0000ff"}}}
	box := scene.Item{X: 0, Y: 0, Width: 80, Height: 60, Fill: scene.Paint{Gradient: g}}
	plain := scene.Item{X: 50, Y: 30, Width: 25, Height: 25, Fill: scene.Paint{Color: "#d3d3d3"}}
	texts := []scene.Item{
		{X: 20, Y: 20, Text: []string{"a"}, Fill: scene.Paint{Color: textDark}},
		{X: 62, Y: 42, Text: []string{"f"}, Fill: scene.Paint{Color: textLight}},
		{X: 20, Y: 40, Text: []string{"own"}, Fill: scene.Paint{Color: "#ffffff"}},
	}
	s := root(120, 100, scene.Mark{Type: scene.Rect, Items: []scene.Item{box, plain}}, scene.Mark{Type: scene.Text, Items: texts})
	Draw(s, Options{})
	got := marksOf(s)[1].Items
	if got[0].Fill.Color != textLight || got[0].Stroke.Color != "#ffffff" || got[0].StrokeWidth == nil || *got[0].StrokeWidth != 3 {
		t.Errorf("the label on the tint has the fill %q and the halo %q, want %q with a halo of 3 in #ffffff", got[0].Fill.Color, got[0].Stroke.Color, textLight)
	}
	if got[1].Fill.Color != textLight || !got[1].Stroke.IsZero() {
		t.Errorf("the label on the plain fill has the fill %q and the stroke %+v, want %q and no halo", got[1].Fill.Color, got[1].Stroke, textLight)
	}
	if got[2].Fill.Color != "#ffffff" {
		t.Errorf("the label with a color of its own has the fill %q, want #ffffff", got[2].Fill.Color)
	}
}

// box2 returns an empty box for a pointBox.
func box2() box {
	return box{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
}

// TestAnUnderlayHidesWhatIsUnder checks that a hachure draws over a solid
// fill in the color of the background, at the opacity of the fill, so that
// a mark under it does not show through its gaps. A fill in the color of
// the background, and a canvas with no background, take no underlay.
func TestAnUnderlayHidesWhatIsUnder(t *testing.T) {
	t.Parallel()
	under := func(bg string, it scene.Item) *scene.Item {
		s := root(100, 60, scene.Mark{Type: scene.Rect, Items: []scene.Item{it}})
		s.Background.Color = bg
		Draw(s, Options{})
		first := marksOf(s)[0].Items[0]
		if first.Fill.IsZero() || first.Fill.Color != bg {
			return nil
		}
		return &first
	}
	box := scene.Item{Width: 40, Height: 30, Fill: scene.Paint{Color: "#5b6ee1"}}
	if u := under("#ffffff", box); u == nil || u.FillOpacity != nil {
		t.Errorf("an opaque fill takes the underlay %+v, want an opaque one", u)
	}
	faint := box
	faint.FillOpacity = new(0.5)
	faint.Fill = scene.Paint{Color: "rgba(91,110,225,0.5)"}
	if u := under("#ffffff", faint); u == nil || u.FillOpacity == nil || *u.FillOpacity != 0.25 {
		t.Errorf("a fill at 0.25 takes the underlay %+v, want one at 0.25", u)
	}
	if u := under("", box); u != nil {
		t.Errorf("a canvas with no background gives the underlay %+v", u)
	}
	white := box
	white.Fill = scene.Paint{Color: "#ffffff"}
	s := root(100, 60, scene.Mark{Type: scene.Rect, Items: []scene.Item{white}})
	Draw(s, Options{})
	if n := len(marksOf(s)[0].Items); n != 1 {
		t.Errorf("a fill in the color of the background draws %d parts, want 1", n)
	}
}

// TestSmallShapesWobbleLess checks that a small shape takes a part of the
// roughness, as Excalidraw lowers it for a small element, and that a large
// shape, a long line and an edge of a graph keep theirs. A shape shorter
// than 10 fills solid.
func TestSmallShapesWobbleLess(t *testing.T) {
	t.Parallel()
	r := theme.SketchRoughness
	cases := []struct {
		name        string
		role        string
		w, h        float64
		round, line bool
		want        float64
	}{
		{"a large rect", "", 20, 50, false, false, r},
		{"a narrow bar", "", 12, 200, false, false, r / 2},
		{"a dot", "", 5, 5, true, false, r / 3},
		{"a circle of 15", "", 15, 15, true, false, r},
		{"a square of 15", "", 15, 15, false, false, r / 2},
		{"a tick", "", 0, 5, false, true, r / 3},
		{"a grid line", "", 0, 150, false, true, r},
		{"a short rule", "", 0, 60, false, true, r / 2},
		{"an arrowhead", scene.RoleEdge, 8, 6, false, false, r},
	}
	for _, c := range cases {
		ps := paints{roughness: r, fill: scene.Paint{Color: "#5b6ee1"}}
		ps.calm(&scene.Mark{Role: c.role}, c.w, c.h, c.round, c.line)
		if ps.roughness != c.want {
			t.Errorf("%s: the roughness is %g, want %g", c.name, ps.roughness, c.want)
		}
		if solid := c.want == r/3; ps.solid != solid {
			t.Errorf("%s: the fill draws solid %t, want %t", c.name, ps.solid, solid)
		}
	}
}

// TestATextFitsItsLimit checks that a text with a limit is cut to it in
// the font of the look, so that a renderer needs no limit.
func TestATextFitsItsLimit(t *testing.T) {
	t.Parallel()
	it := scene.Item{Text: scene.Lines{"a long label that does not fit"}, FontSize: 11, Limit: 60}
	s := root(200, 60, scene.Mark{Type: scene.Text, Items: []scene.Item{it}})
	Draw(s, Options{Font: "'Excalifont', sans-serif", Measure: text.NewEstimate()})
	got := marksOf(s)[0].Items[0]
	if got.Limit != 0 || !strings.HasSuffix(got.Text[0], text.Ellipsis) {
		t.Errorf("the text is %q with the limit %g", got.Text, got.Limit)
	}
}

// TestEdgesAndClusters checks that an edge of a graph wobbles less than
// another line, with the roughness theme.SketchEdgeRoughness, and that the
// box of a cluster draws its hachure with lines of
// theme.SketchClusterFillWeight, as Mermaid 12.3 draws them.
func TestEdgesAndClusters(t *testing.T) {
	t.Parallel()
	line := func(role string) float64 {
		s := root(400, 100, scene.Mark{Type: scene.Rule, Role: role, Items: []scene.Item{{X: 0, Y: 40, X2: 360, Y2: 40, Stroke: scene.Paint{Color: "#000"}}}})
		Draw(s, Options{})
		var pts pointList
		if err := shape.Parse(marksOf(s)[0].Items[0].Path, &pts); err != nil {
			t.Fatal(err)
		}
		most := 0.0
		for _, p := range pts {
			most = max(most, math.Abs(p[1]-40))
		}
		return most
	}
	if edge, other := line(scene.RoleEdge), line("mark"); edge >= other || edge == 0 {
		t.Errorf("an edge strays %g from its line, and another line %g", edge, other)
	}
	box := func(role string) float64 {
		s := root(200, 100, scene.Mark{Type: scene.Rect, Role: role, Items: []scene.Item{{Width: 80, Height: 40, Fill: scene.Paint{Color: "#eeeeee"}}}})
		Draw(s, Options{})
		return *withoutUnderlay(marksOf(s)[0].Items)[0].StrokeWidth
	}
	if c, o := box(scene.RoleCluster), box("mark"); c != theme.SketchClusterFillWeight || o != theme.SketchFillWeight {
		t.Errorf("the hachure of a cluster is %g wide, and of another rect %g", c, o)
	}
}

// TestUnderlayOfRings checks that the underlay of a path of several rings,
// such as a band of a contour with islands, fills the path as it is. A
// solid fill of rough.js of the bridged polygon draws a sliver along each
// bridge between the rings, which drew white lines that met at one point
// in vega/annual-precipitation.
func TestUnderlayOfRings(t *testing.T) {
	t.Parallel()
	const d = "M0,0L40,0L40,40L0,40ZM60,0L100,0L100,40L60,40Z"
	s := root(140, 80, scene.Mark{Type: scene.Path, Items: []scene.Item{{Path: d, Fill: scene.Paint{Color: "#5b6ee1"}}}})
	Draw(s, Options{})
	items := marksOf(s)[0].Items
	if len(items) == 0 || items[0].Fill.Color != "#ffffff" {
		t.Fatalf("the first part is %+v, want the underlay in #ffffff", items)
	}
	if items[0].Path != d {
		t.Errorf("the underlay has the path %q, want the path %q as it is", items[0].Path, d)
	}
}

// TestCanvasGrowsForWobbleOnly checks that the canvas grows at a side for
// a line that wobbles a little out of it, and not for a mark that reaches
// far past the side, such as the land of vega/world-map on a canvas that
// its spec fixes, which stays cut as in the standard look.
func TestCanvasGrowsForWobbleOnly(t *testing.T) {
	t.Parallel()
	draw := func(d string) *scene.Scene {
		s := &scene.Scene{Width: 100, Height: 80, Background: scene.Paint{Color: "#ffffff"},
			Root: scene.Mark{Type: scene.Group, Items: []scene.Item{{Width: 100, Height: 80, Items: []scene.Mark{
				{Type: scene.Path, Items: []scene.Item{{Path: d, Stroke: scene.Paint{Color: "#000000"}}}}}}}}}
		Draw(s, Options{})
		return s
	}
	if s := draw("M-3,10L60,10L60,50L-3,50Z"); s.Root.Items[0].X <= 0 {
		t.Errorf("a line 3 past the left keeps the canvas at %v, want it to grow at the left", s.Root.Items[0].X)
	}
	if s := draw("M5,10L600,10L600,50L5,50Z"); s.Width != 100 {
		t.Errorf("a line 500 past the right makes the canvas %v wide, want 100", s.Width)
	}
}

// TestNonzeroRings checks that the hachure fills a ring inside a ring that
// runs the same way, as the rule nonzero of SVG fills the inner circle of a
// double circle, keeps a ring that runs the other way as a hole, and keeps
// the dot of a bullseye, which runs the way of the outer ring inside the
// hole (D55).
func TestNonzeroRings(t *testing.T) {
	t.Parallel()
	const outer = "M0,0L100,0L100,100L0,100Z"
	same := outer + "M20,20L80,20L80,80L20,80Z"
	hole := outer + "M20,20L20,80L80,80L80,20Z"
	count := func(d string) int {
		var f flattener
		_ = shape.Parse(d, &f)
		f.end()
		return len(nonzero(f.rings))
	}
	if got := count(same); got != 1 {
		t.Errorf("a ring inside a ring of the same way leaves %d rings, want 1", got)
	}
	if got := count(hole); got != 2 {
		t.Errorf("a ring inside a ring of the other way leaves %d rings, want 2, a hole", got)
	}
	if got := count(hole + "M40,40L60,40L60,60L40,60Z"); got != 3 {
		t.Errorf("a bullseye leaves %d rings, want 3, the dot in the hole", got)
	}
}
