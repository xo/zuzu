package layout

import (
	"math"
	"testing"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/theme"
)

// style is the style of the tests: the light mode of the zuzu scheme.
func style() Style {
	return Style{Theme: theme.New(theme.Light, theme.Zuzu), Font: "sans-serif", Measure: text.NewEstimate()}
}

// newScale builds a scale for a test.
func newScale(t *testing.T, typ string, domain, rng []any) scale.Scale {
	t.Helper()
	o := scale.Defaults(typ)
	o.Domain, o.Range = domain, rng
	s, err := scale.New(o)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// find returns the first mark of a role in a group mark.
func find(m scene.Mark, role string) *scene.Mark {
	for i := range m.Items {
		for j := range m.Items[i].Items {
			if c := &m.Items[i].Items[j]; c.Role == role {
				return c
			}
		}
	}
	return nil
}

func TestNumberAxis(t *testing.T) {
	t.Parallel()
	st := style()
	sc := newScale(t, "linear", []any{-20.0, 180.0}, []any{200.0, 0.0})
	a := &Axis{
		Orient: "left", Scale: sc, Length: 200, Cross: 300, Grid: true, Labels: true,
		Domain: true, Title: "Sales", Overlap: true,
		Format: expr.String,
	}
	m := st.Axis(a)
	if m.Role != "axis" {
		t.Errorf("role = %q, want axis", m.Role)
	}
	labels := find(m, "axis-label")
	if labels == nil || len(labels.Items) == 0 || len(labels.Items) > MaxLabels {
		t.Fatalf("labels = %v, want 1 to %d", labels, MaxLabels)
	}
	for _, it := range labels.Items {
		if it.Align != "right" || it.X != -LabelPad {
			t.Errorf("a label at %v, aligned %s, want right at %d", it.X, it.Align, -LabelPad)
		}
	}
	grid := find(m, "axis-grid")
	if grid == nil || len(grid.Items) != len(labels.Items) || grid.Items[0].X2 != 300 {
		t.Errorf("grid = %v, want a line across 300 for each label", grid)
	}
	zero := find(m, "axis-zero")
	if zero == nil || math.Abs(zero.Items[0].Y-180) > 1e-9 {
		t.Errorf("zero = %v, want a line at 180", zero)
	}
	if find(m, "axis-domain") != nil {
		t.Error("an axis of numbers draws a domain line, want none")
	}
	if title := find(m, "axis-title"); title == nil || title.Items[0].Text[0] != "Sales" || title.Items[0].Y >= 0 {
		t.Errorf("title = %v, want Sales above the axis", title)
	}
}

func TestBandAxis(t *testing.T) {
	t.Parallel()
	st := style()
	sc := newScale(t, "band", []any{"a", "b", "c", "d"}, []any{0.0, 400.0})
	m := st.Axis(&Axis{
		Orient: "bottom", Scale: sc, Length: 400, Cross: 200, Labels: true, Domain: true, BandPosition: 0.5,
		Format: expr.String,
	})
	labels := find(m, "axis-label")
	if labels == nil || len(labels.Items) != 4 {
		t.Fatalf("labels = %v, want 4", labels)
	}
	if it := labels.Items[0]; it.X != 50 || it.Y != 200+LabelPad || it.Align != "center" {
		t.Errorf("the first label is at %v, %v, aligned %s, want 50, %d, center", it.X, it.Y, it.Align, 200+LabelPad)
	}
	if dom := find(m, "axis-domain"); dom == nil || dom.Items[0].Y != 200 {
		t.Errorf("domain = %v, want a line at 200", dom)
	}
}

func TestOverlap(t *testing.T) {
	t.Parallel()
	st := style()
	vals := make([]any, 40)
	for i := range vals {
		vals[i] = float64(i)
	}
	sc := newScale(t, "point", vals, []any{0.0, 100.0})
	m := st.Axis(&Axis{
		Orient: "bottom", Scale: sc, Length: 100, Labels: true, Overlap: true,
		Format: func(v any) string { return "label " + expr.String(v) },
	})
	labels := find(m, "axis-label")
	if labels == nil || len(labels.Items) >= 40 || len(labels.Items) < 2 {
		t.Errorf("%d labels, want fewer than 40 and at least 2", len(labels.Items))
	}
}

func TestLegend(t *testing.T) {
	t.Parallel()
	st := style()
	sc := newScale(t, "ordinal", []any{"x", "y", "z"}, []any{"#ff0000", "#00ff00", "#0000ff"})
	m := st.Legend(&Legend{Fill: sc, Title: "Group", Width: 600, Format: expr.String})
	if m.Role != "legend" {
		t.Errorf("role = %q, want legend", m.Role)
	}
	entries := find(m, "legend-entry")
	if entries == nil {
		t.Fatal("no entries")
	}
	symbols := entries.Items[0].Items[0]
	if len(symbols.Items) != 3 || symbols.Items[1].Fill.Color != "#00ff00" {
		t.Errorf("symbols = %v, want 3 with y in green", symbols.Items)
	}
	// A narrow legend wraps to a second row.
	narrow := st.Legend(&Legend{Fill: sc, Width: 60, Format: expr.String})
	rows := find(narrow, "legend-entry").Items[0].Items[0]
	if rows.Items[2].Y <= rows.Items[0].Y {
		t.Error("a narrow legend did not wrap")
	}
	lin := newScale(t, "linear", []any{0.0, 10.0}, []any{"#000000", "#ffffff"})
	g := st.Legend(&Legend{Fill: lin, Gradient: true, Format: expr.String})
	if find(g, "legend-gradient") == nil {
		t.Error("a continuous color legend draws no gradient")
	}
}

func TestTitle(t *testing.T) {
	t.Parallel()
	st := style()
	m := st.Title(&Title{Text: []string{"Revenue"}, Subtitle: []string{"by year"}, Orient: "top", Anchor: "start", Width: 400, Height: 300, Offset: 10, Kind: "title"})
	text := find(m, "title-text")
	if text == nil || text.Items[0].FontSize != TitleSize || text.Items[0].Y >= 0 || text.Items[0].Align != "left" {
		t.Errorf("title = %v, want a title of size %d, left, above the plot", text, TitleSize)
	}
	sub := find(m, "title-subtitle")
	if sub == nil || sub.Items[0].Y <= text.Items[0].Y {
		t.Errorf("subtitle = %v, want it below the title", sub)
	}
	left := st.Title(&Title{Text: []string{"Group"}, Orient: "left", Anchor: "middle", Height: 300, Offset: 4, Kind: "label"})
	if it := find(left, "title-text").Items[0]; it.Angle != -90 || it.Y != 150 || it.FontSize != LabelSize {
		t.Errorf("a left label = %+v, want it turned, at 150", it)
	}
}

func TestBounds(t *testing.T) {
	t.Parallel()
	st := style()
	m := scene.Mark{Type: scene.Text, Items: []scene.Item{{X: 100, Y: 50, Text: scene.Lines{"Hello"}, FontSize: 10, Align: "right", Baseline: "top"}}}
	b := st.MarkBounds(&m)
	if b.X2 != 100 || b.X1 >= 100 || b.Y1 != 50 || b.Y2 != 60 {
		t.Errorf("bounds = %+v, want a box that ends at x 100 and spans y 50 to 60", b)
	}
	g := scene.Mark{Type: scene.Group, Items: []scene.Item{{X: 10, Y: 20, Width: 30, Height: 40, Items: []scene.Mark{
		{Type: scene.Rect, Items: []scene.Item{{X: -5, Y: 0, Width: 100, Height: 10}}},
	}}}}
	if b := st.MarkBounds(&g); b.X1 != 5 || b.X2 != 105 || b.Y1 != 20 || b.Y2 != 60 {
		t.Errorf("group bounds = %+v, want 5 to 105 and 20 to 60", b)
	}
	if !Empty().IsEmpty() {
		t.Error("Empty is not empty")
	}
}

// TestLegendThresholds checks that a discretizing scale with fewer
// thresholds than colors draws one entry for each piece, and no more.
func TestLegendThresholds(t *testing.T) {
	t.Parallel()
	st := style()
	o := scale.Defaults("threshold")
	o.Domain = []any{10.0}
	o.Range = []any{"#000000", "#888888", "#ffffff"}
	sc, err := scale.New(o)
	if err != nil {
		t.Fatal(err)
	}
	m := st.Legend(&Legend{Fill: sc, Format: expr.String})
	symbols := find(m, "legend-entry").Items[0].Items[0]
	if len(symbols.Items) != 2 {
		t.Errorf("%d entries, want 2", len(symbols.Items))
	}
}

// TestAxisTitleClearsTurnedLabels checks that the title of a bottom axis
// sits below its labels when they turn, and keeps its place when they do
// not. Before, the title had a fixed place and covered the turned labels.
func TestAxisTitleClearsTurnedLabels(t *testing.T) {
	t.Parallel()
	st := style()
	sc := newScale(t, "band", []any{"a long label", "another long label"}, []any{0.0, 200.0})
	axis := func(angle float64) (Bounds, scene.Item) {
		m := st.Axis(&Axis{
			Orient: "bottom", Scale: sc, Length: 200, Cross: 100, Labels: true, BandPosition: 0.5,
			Title: "Name", LabelAngle: angle, Format: expr.String,
		})
		return st.MarkBounds(find(m, "axis-label")), find(m, "axis-title").Items[0]
	}
	if _, title := axis(0); title.Y != 100+LabelPad+LabelSize+titleGap {
		t.Errorf("the title of labels that do not turn is at y %v, want %d", title.Y, 100+LabelPad+LabelSize+titleGap)
	}
	labels, title := axis(90)
	if labels.Y2 < 150 || title.Y != labels.Y2+titleGap {
		t.Errorf("the title of turned labels that end at y %v is at y %v, want %v", labels.Y2, title.Y, labels.Y2+titleGap)
	}
}

// TestAxisSpecPlace checks that the align of the labels, and the place, the
// angle, the align and the baseline of the title, that the spec gives win
// over those of the look. titleX and titleY start at the origin of the
// axis, which an offset moves, as in Vega.
func TestAxisSpecPlace(t *testing.T) {
	t.Parallel()
	st := style()
	sc := newScale(t, "linear", []any{0.0, 10.0}, []any{100.0, 0.0})
	x, y, angle := -8.0, -2.0, 0.0
	m := st.Axis(&Axis{
		Orient: "left", Scale: sc, Length: 100, Cross: 200, Labels: true, Offset: 10,
		Title: "Year", Format: expr.String, LabelFlush: 1, LabelAlign: "center",
		TitleX: &x, TitleY: &y, TitleAngle: &angle, TitleAlign: "right", TitleBaseline: "top",
	})
	for _, it := range find(m, "axis-label").Items {
		if it.Align != "center" {
			t.Errorf("a label is aligned %s, want center", it.Align)
		}
	}
	if it := find(m, "axis-title").Items[0]; it.X != -18 || it.Y != -2 || it.Angle != 0 || it.Align != "right" || it.Baseline != "top" {
		t.Errorf("the title is at %v, %v, turned %v, aligned %s on %s, want -18, -2, 0, right on top", it.X, it.Y, it.Angle, it.Align, it.Baseline)
	}
	// A bottom axis keeps the x of the look when the spec gives only a y.
	y = 30
	m = st.Axis(&Axis{Orient: "bottom", Scale: sc, Length: 100, Cross: 50, Labels: true, Title: "Value", Format: expr.String, TitleY: &y})
	if it := find(m, "axis-title").Items[0]; it.X != 100 || it.Y != 80 || it.Align != "right" {
		t.Errorf("the title is at %v, %v, aligned %s, want 100, 80, right", it.X, it.Y, it.Align)
	}
}

// TestPathAndTextBounds checks that the bounds of a path are those of its
// points, without its origin, and that the bounds of a text of several
// lines take its line height, as the writers draw it.
func TestPathAndTextBounds(t *testing.T) {
	t.Parallel()
	st := style()
	path := scene.Item{X: 10, Y: 20, Path: "M50,50L60,60"}
	if b := st.ItemBounds(scene.Path, &path); b != (Bounds{60, 70, 70, 80}) {
		t.Errorf("path bounds = %+v, want 60 to 70 and 70 to 80", b)
	}
	for _, c := range []struct {
		baseline string
		y1, y2   float64
	}{
		// Three lines of 10 pixels, 20 apart, are 50 high.
		{"top", 0, 50},
		{"middle", -25, 25},
		{"bottom", -50, 0},
		// The last line sits on the alphabetic baseline.
		{"alphabetic", -48, 2},
	} {
		it := scene.Item{Text: scene.Lines{"a", "b", "c"}, FontSize: 10, LineHeight: 20, Baseline: c.baseline}
		if b := st.ItemBounds(scene.Text, &it); math.Abs(b.Y1-c.y1) > 1e-9 || math.Abs(b.Y2-c.y2) > 1e-9 {
			t.Errorf("the bounds of a text on %s span y %v to %v, want %v to %v", c.baseline, b.Y1, b.Y2, c.y1, c.y2)
		}
	}
}

// TestAxisAnchorOfReversedRange checks that a titleAnchor names a point of
// the range of the scale, so that the start of a reversed range is at its
// right, and that the title keeps the align of the anchor, as in Vega. A
// labelPadding of the spec wins over LabelPad.
func TestAxisAnchorOfReversedRange(t *testing.T) {
	t.Parallel()
	st := style()
	sc := newScale(t, "linear", []any{0.0, 10.0}, []any{200.0, 0.0})
	pad := 3.0
	m := st.Axis(&Axis{
		Orient: "bottom", Scale: sc, Length: 200, Cross: 100, Labels: true, Title: "X",
		Format: expr.String, TitleAnchor: "start", LabelPadding: &pad, LabelOffset: 2,
	})
	if it := find(m, "axis-title").Items[0]; it.X != 200 || it.Align != "left" {
		t.Errorf("the title is at x %v, aligned %s, want 200, left", it.X, it.Align)
	}
	for _, it := range find(m, "axis-label").Items {
		if it.Y != 103 {
			t.Errorf("a label is at y %v, want 103", it.Y)
		}
	}
}
