package layout

import (
	"math"
	"slices"
	"testing"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/scene"
)

// collect returns the items of each mark of a role in a mark and in the
// groups under it, in order.
func collect(m scene.Mark, role string) []scene.Item {
	var out []scene.Item
	var walk func(m *scene.Mark)
	walk = func(m *scene.Mark) {
		for i := range m.Items {
			if m.Role == role {
				out = append(out, m.Items[i])
			}
			for j := range m.Items[i].Items {
				walk(&m.Items[i].Items[j])
			}
		}
	}
	walk(&m)
	return out
}

// labelTexts returns the text of each label of a legend.
func labelTexts(m scene.Mark) []string {
	var out []string
	for _, it := range collect(m, "legend-label") {
		out = append(out, it.Text...)
	}
	return out
}

// sizes is a scale of sizes from 0 to 3,000,000, with a range up to 5,000,
// as the size of vegalite/circle_natural_disasters.vl.json.
func sizes(t *testing.T) scale.Scale {
	t.Helper()
	return newScale(t, "linear", []any{0.0, 3000000.0}, []any{4.0, 5000.0})
}

// TestLegendClipHeight checks that clipHeight cuts each large symbol to its
// height in a group that clips it, sets the height of the row, and keeps 0
// and every other value, as vega.js draws circle_natural_disasters. Before,
// zuzu drew nested circles of 70 high, left out 0 and dropped clipHeight.
func TestLegendClipHeight(t *testing.T) {
	t.Parallel()
	st := style()
	vals := []any{0.0, 1000000.0, 2000000.0, 3000000.0}
	m := st.Legend(&Legend{Size: sizes(t), Values: vals, Title: "Annual Global Deaths", Width: 600, Format: expr.String, ClipHeight: 30})
	if got, want := labelTexts(m), []string{"0", "1000000", "2000000", "3000000"}; !slices.Equal(got, want) {
		t.Errorf("labels = %q, want %q", got, want)
	}
	clips := collect(m, "legend-clip")
	if len(clips) != 3 {
		t.Fatalf("%d clipped symbols, want 3", len(clips))
	}
	for _, c := range clips {
		if !c.Clip || c.Height != 30 {
			t.Errorf("clip = %+v, want a group that clips at a height of 30", c)
		}
	}
	// No symbol reaches into the label after it.
	labels := collect(m, "legend-label")
	for i, c := range clips {
		if lab := labels[i+1]; c.X+c.Width > lab.X {
			t.Errorf("symbol %d ends at %v, after its label at %v", i+1, c.X+c.Width, lab.X)
		}
	}
	if find(m, "legend-rule") != nil {
		t.Error("a legend with clipHeight draws nested circles")
	}
}

// TestLegendNested checks when a legend of sizes alone draws as nested
// circles, as docs/STYLE.md says: only when each value keeps its circle and
// its label, and the spec sets no property of the entries.
func TestLegendNested(t *testing.T) {
	t.Parallel()
	st := style()
	nested := func(l *Legend) bool {
		l.Format = expr.String
		return find(st.Legend(l), "legend-rule") != nil
	}
	big := newScale(t, "linear", []any{0.0, 30.0}, []any{0.0, 5000.0})
	// The domains take in 0, as a linear scale does by default.
	big1 := newScale(t, "linear", []any{10.0, 30.0}, []any{1000.0, 5000.0})
	tests := []struct {
		name string
		l    Legend
		want bool
	}{
		{"fits", Legend{Size: big, Values: []any{5.0, 15.0, 30.0}}, true},
		{"fits from a range above 0", Legend{Size: big1, Values: []any{1.0, 15.0, 30.0}}, true},
		// The tops of 20 and 30 are 10 apart.
		{"tops too close", Legend{Size: big1, Values: []any{10.0, 20.0, 30.0}}, false},
		// The tops of 1,000,000 and 2,000,000 are 12 apart, too close
		// for their labels, and 0 is a dot.
		{"labels too close", Legend{Size: sizes(t), Values: []any{0.0, 1000000.0, 2000000.0, 3000000.0}}, false},
		{"clipHeight", Legend{Size: big, Values: []any{5.0, 15.0, 30.0}, ClipHeight: 30}, false},
		{"direction of the spec", Legend{Size: big, Values: []any{5.0, 15.0, 30.0}, Entries: true}, false},
		{"square", Legend{Size: big, Values: []any{5.0, 15.0, 30.0}, SymbolType: "square"}, false},
		{"small", Legend{Size: newScale(t, "linear", []any{0.0, 30.0}, []any{0.0, 400.0}), Values: []any{10.0, 30.0}}, false},
	}
	for _, test := range tests {
		if got := nested(&test.l); got != test.want {
			t.Errorf("%s: nested = %v, want %v", test.name, got, test.want)
		}
	}
}

// TestLegendSizesDoNotOverlap checks that a row of large circles makes
// each entry as wide as its circle and the row as high as the largest, so
// that no circle covers a label or the next row. Before, each entry was 10
// wide and each row 18 high, whatever the size of its circle.
func TestLegendSizesDoNotOverlap(t *testing.T) {
	t.Parallel()
	st := style()
	sc := newScale(t, "linear", []any{0.0, 40.0}, []any{4.0, 1600.0})
	m := st.Legend(&Legend{Size: sc, Width: 150, Format: expr.String})
	symbols := collect(m, "legend-symbol")
	labels := collect(m, "legend-label")
	if len(symbols) != len(labels) || len(symbols) < 3 {
		t.Fatalf("%d symbols and %d labels", len(symbols), len(labels))
	}
	rows := map[float64]bool{}
	for i, sym := range symbols {
		r := math.Sqrt(*sym.Size) / 2
		if sym.X+r > labels[i].X {
			t.Errorf("symbol %d reaches %v, past its label at %v", i, sym.X+r, labels[i].X)
		}
		rows[sym.Y] = true
	}
	ys := slices.Sorted(func(yield func(float64) bool) {
		for y := range rows {
			if !yield(y) {
				return
			}
		}
	})
	if len(ys) < 2 {
		t.Fatalf("rows at %v, want the narrow legend to wrap", ys)
	}
	if ys[1]-ys[0] < 40 {
		t.Errorf("rows %v apart, want at least the 40 of the largest circle", ys[1]-ys[0])
	}
}

// TestLegendPieces checks that the entries of a scale that cuts numbers
// into pieces name each piece by its ends, with the sizes too, and that a
// legend of bins names each bin. Before, the first entry of a legend of
// sizes read -Infinity, and each entry named only its start.
func TestLegendPieces(t *testing.T) {
	t.Parallel()
	st := style()
	o := scale.Defaults("threshold")
	o.Domain, o.Range = []any{30.0, 70.0}, []any{80.0, 200.0, 320.0}
	size, err := scale.New(o)
	if err != nil {
		t.Fatal(err)
	}
	m := st.Legend(&Legend{Size: size, Format: expr.String})
	if got, want := labelTexts(m), []string{"< 30", "30 – 70", "≥ 70"}; !slices.Equal(got, want) {
		t.Errorf("threshold labels = %q, want %q", got, want)
	}
	o = scale.Defaults("bin-ordinal")
	o.Domain, o.Bins, o.Range = []any{5.0, 25.0}, []float64{5, 10, 15, 20, 25}, []any{"#000", "#444", "#888", "#ccc"}
	bins, err := scale.New(o)
	if err != nil {
		t.Fatal(err)
	}
	m = st.Legend(&Legend{Stroke: bins, Format: expr.String})
	if got, want := labelTexts(m), []string{"5 – 10", "10 – 15", "15 – 20", "20 – 25"}; !slices.Equal(got, want) {
		t.Errorf("bin labels = %q, want %q", got, want)
	}
	// A gradient of thresholds draws a step for each color, with the
	// thresholds where the steps meet.
	o = scale.Defaults("threshold")
	o.Domain, o.Range = []any{30.0, 70.0}, []any{"#000", "#888", "#fff"}
	color, err := scale.New(o)
	if err != nil {
		t.Fatal(err)
	}
	g := st.Legend(&Legend{Fill: color, Gradient: true, Format: expr.String, GradientLength: 90})
	steps := collect(g, "legend-gradient")
	if len(steps) != 3 || steps[1].Fill.Color != "#888" || steps[1].Width != 30 {
		t.Errorf("steps = %+v, want 3 of 30 wide", steps)
	}
	if got, want := labelTexts(g), []string{"30", "70"}; !slices.Equal(got, want) {
		t.Errorf("step labels = %q, want %q", got, want)
	}
}

// TestLegendLimits checks the limits of the entries and of the labels,
// 30 and 160 as in Vega unless the spec sets others.
func TestLegendLimits(t *testing.T) {
	t.Parallel()
	st := style()
	dom := make([]any, 40)
	for i := range dom {
		dom[i] = string(rune('A'+i%26)) + string(rune('a'+i/26))
	}
	sc := newScale(t, "ordinal", dom, []any{"#000", "#fff"})
	labels := labelTexts(st.Legend(&Legend{Fill: sc, Format: expr.String}))
	if len(labels) != 30 || labels[29] != "…11 entries" {
		t.Errorf("%d labels ending in %q, want 30 ending in …11 entries", len(labels), labels[len(labels)-1])
	}
	labels = labelTexts(st.Legend(&Legend{Fill: sc, Format: expr.String, SymbolLimit: 5}))
	if len(labels) != 5 || labels[4] != "…36 entries" {
		t.Errorf("labels = %q, want 5 ending in …36 entries", labels)
	}
	m := st.Legend(&Legend{Fill: sc, Format: expr.String, LabelLimit: 40})
	if lab := collect(m, "legend-label")[0]; lab.Limit != 40 {
		t.Errorf("limit = %v, want 40", lab.Limit)
	}
}

// TestLegendSymbols checks that a symbol takes the shape, the outline and
// the color that the view gives from the marks of the legend.
func TestLegendSymbols(t *testing.T) {
	t.Parallel()
	st := style()
	sc := newScale(t, "ordinal", []any{"x", "y"}, []any{"#ff0000", "#00ff00"})
	sym := collect(st.Legend(&Legend{Stroke: sc, SymbolType: "circle", Format: expr.String}), "legend-symbol")[0]
	if sym.Shape != "circle" || sym.Fill.Color != "#ff0000" || sym.Stroke.Color != "" {
		t.Errorf("symbol = %+v, want a filled red circle", sym)
	}
	sym = collect(st.Legend(&Legend{Stroke: sc, SymbolType: "circle", Hollow: true, Format: expr.String}), "legend-symbol")[0]
	if sym.Fill.Color != "" || sym.Stroke.Color != "#ff0000" {
		t.Errorf("symbol = %+v, want a red outline", sym)
	}
	syms := collect(st.Legend(&Legend{Fill: sc, LineValues: map[string]bool{"y": true}, Format: expr.String}), "legend-symbol")
	if syms[0].Shape != "square" || syms[1].Shape != "stroke" {
		t.Errorf("shapes = %q and %q, want square and stroke", syms[0].Shape, syms[1].Shape)
	}
	sym = collect(st.Legend(&Legend{Size: sizes(t), Color: "#4682b4", Hollow: true, Format: expr.String}), "legend-symbol")[0]
	if sym.Stroke.Color != "#4682b4" || sym.Fill.Color != "" {
		t.Errorf("symbol = %+v, want a ring in the color of the mark", sym)
	}
	// A legend of opacity alone draws each symbol at its opacity. Before,
	// zuzu drew no legend for it.
	op := newScale(t, "linear", []any{0.0, 40.0}, []any{0.3, 0.8})
	syms = collect(st.Legend(&Legend{Opacity: op, SymbolType: "circle", Format: expr.String}), "legend-symbol")
	if len(syms) < 2 || syms[0].Opacity == nil || *syms[0].Opacity != 0.3 || syms[0].Shape != "circle" {
		t.Errorf("symbols = %+v, want circles from the opacity 0.3", syms)
	}
}

// TestLegendTitlePlace checks that titleOrient, titleAnchor and
// titlePadding place the title of a legend beside its entries, as the
// legend of Vega documents them, and that the legend then starts at the top
// left of its title and its entries. Before, zuzu reported each and kept
// the title of the look, so the title of calendar-view stood 16 from its
// gradient rather than 20.
func TestLegendTitlePlace(t *testing.T) {
	t.Parallel()
	st := style()
	sc := newScale(t, "ordinal", []any{"x", "y"}, []any{"#ff0000", "#00ff00"})
	pad := func(f float64) *float64 { return &f }
	legend := func(l Legend) (scene.Item, Bounds) {
		t.Helper()
		l.Fill, l.Title, l.Format = sc, "Group", expr.String
		m := st.Legend(&l)
		title := find(m, "legend-title")
		if title == nil {
			t.Fatal("no title")
		}
		return title.Items[0], st.MarkBounds(find(m, "legend-entry"))
	}
	for _, c := range []struct {
		name  string
		l     Legend
		check func(t scene.Item, tb, body Bounds) bool
	}{
		{"left with a padding", Legend{TitleOrient: "left", TitlePadding: pad(20)}, func(it scene.Item, tb, body Bounds) bool {
			return math.Abs(body.X1-tb.X2-20) < 1e-9 && math.Abs(tb.X1) < 1e-9 && it.Baseline == "middle"
		}},
		{"top at the end", Legend{TitleOrient: "top", TitleAnchor: "end", TitlePadding: pad(4)}, func(it scene.Item, tb, body Bounds) bool {
			return math.Abs(body.Y1-tb.Y2-4) < 1e-9 && math.Abs(tb.X2-body.X2) < 1e-9 && tb.Y1 == 0
		}},
		{"bottom in the middle", Legend{TitleOrient: "bottom", TitleAnchor: "middle"}, func(it scene.Item, tb, body Bounds) bool {
			return math.Abs(tb.Y1-body.Y2-legendTitleGap) < 1e-9 && math.Abs((tb.X1+tb.X2)-(body.X1+body.X2)) < 1e-9
		}},
		{"right at the end", Legend{TitleOrient: "right", TitleAnchor: "end"}, func(it scene.Item, tb, body Bounds) bool {
			return math.Abs(tb.X1-body.X2-entryGap) < 1e-9 && math.Abs(tb.Y2-body.Y2) < 1e-9
		}},
		{"a padding alone keeps the side of the look", Legend{TitlePadding: pad(4)}, func(it scene.Item, tb, body Bounds) bool {
			return math.Abs(body.X1-tb.X2-4) < 1e-9
		}},
	} {
		it, body := legend(c.l)
		if tb := st.textBounds(&it); !c.check(it, tb, body) {
			t.Errorf("%s: the title is at %+v, the entries at %+v", c.name, tb, body)
		}
	}
	it, _ := legend(Legend{TitleAlign: "center", TitleBaseline: "top", TitleLimit: 20, TitleLineHeight: 15})
	if it.Align != "center" || it.Baseline != "top" || it.Limit != 20 || it.LineHeight != 15 {
		t.Errorf("the title is %+v, want the align, the baseline, the limit and the line height of the spec", it)
	}
}

// TestLegendFrame checks that the padding, the fillColor, the strokeColor
// and the cornerRadius of a legend draw a frame around it, with the
// entries the padding inside it, as the legend of Vega documents them.
func TestLegendFrame(t *testing.T) {
	t.Parallel()
	st := style()
	sc := newScale(t, "ordinal", []any{"x", "y"}, []any{"#ff0000", "#00ff00"})
	m := st.Legend(&Legend{Fill: sc, Format: expr.String, Padding: 8, FillColor: "white", StrokeColor: "gray", CornerRadius: 3})
	frame := find(m, "legend-frame")
	if frame == nil {
		t.Fatal("no frame")
	}
	f := frame.Items[0]
	if f.X != 0 || f.Y != 0 || f.Fill.Color != "white" || f.Stroke.Color != "gray" || f.CornerRadius != 3 {
		t.Errorf("the frame is %+v, want white, gray and rounded by 3 at the origin", f)
	}
	inner := st.MarkBounds(find(m, "legend-entry"))
	if math.Abs(inner.X1-8) > 1e-9 || math.Abs(inner.Y1-8) > 1e-9 || math.Abs(f.Width-(inner.X2-inner.X1)-16) > 1e-9 {
		t.Errorf("the entries are at %+v in a frame %v wide, want 8 inside it", inner, f.Width)
	}
	if find(st.Legend(&Legend{Fill: sc, Format: expr.String}), "legend-frame") != nil {
		t.Error("a legend of the look has a frame")
	}
}

// TestLegendEntryProps checks the properties of the entries of a legend
// that the legend of Vega documents: labelOffset, labelAlign,
// labelBaseline, symbolOffset, gridAlign, symbolBaseFillColor and
// symbolBaseStrokeColor.
func TestLegendEntryProps(t *testing.T) {
	t.Parallel()
	st := style()
	sc := newScale(t, "ordinal", []any{"a", "bbbbbbbbbbbb", "c", "d"}, []any{"#ff0000", "#00ff00", "#0000ff", "#000000"})
	off := 2.0
	m := st.Legend(&Legend{Fill: sc, Format: expr.String, LabelOffset: &off, LabelAlign: "right", LabelBaseline: "top", SymbolOffset: 3})
	syms, labels := collect(m, "legend-symbol"), collect(m, "legend-label")
	if labels[0].X != swatch+off || labels[0].Align != "right" || labels[0].Baseline != "top" {
		t.Errorf("the first label is at %v, %s on %s, want at %v, right on top", labels[0].X, labels[0].Align, labels[0].Baseline, swatch+off)
	}
	if syms[0].X != swatch/2+3 {
		t.Errorf("the first symbol is at %v, want %v", syms[0].X, swatch/2+3)
	}
	left := func(align string) []float64 {
		var out []float64
		for _, it := range collect(st.Legend(&Legend{Fill: sc, Format: expr.String, Columns: 2, GridAlign: align}), "legend-symbol") {
			out = append(out, it.X)
		}
		return out
	}
	each, all, none := left("each"), left("all"), left("")
	if !slices.Equal(each, none) || each[1] != each[3] {
		t.Errorf("the symbols of gridAlign each are at %v, want each column in line, as the look", each)
	}
	if all[1]-all[0] <= each[1]-each[0] && all[3]-all[2] != all[1]-all[0] {
		t.Errorf("the symbols of gridAlign all are at %v, want columns as wide as the widest", all)
	}
	if flow := left("none"); flow[3] >= flow[1] {
		t.Errorf("the symbols of gridAlign none are at %v, want the second row closer, after its short first label", flow)
	}
	sizes := newScale(t, "linear", []any{0.0, 10.0}, []any{0.0, 100.0})
	base := collect(st.Legend(&Legend{Size: sizes, Entries: true, Format: expr.String, SymbolBaseFill: "pink", SymbolBaseStroke: "navy"}), "legend-symbol")
	if len(base) == 0 || base[0].Fill.Color != "pink" || base[0].Stroke.Color != "navy" {
		t.Errorf("the symbols of the base colors are %+v, want pink in navy", base)
	}
}

// TestLegendGradientLabels checks that a tickCount labels the gradient of a
// legend at its ticks, that tickMinStep takes fewer, and that labelOverlap
// drops the labels that overlap, as the legend of Vega documents them. The
// look labels only the ends.
func TestLegendGradientLabels(t *testing.T) {
	t.Parallel()
	st := style()
	lin := newScale(t, "linear", []any{0.0, 100.0}, []any{"#ffffff", "#000000"})
	labels := func(l Legend) []string {
		l.Fill, l.Gradient, l.Format = lin, true, expr.String
		return labelTexts(st.Legend(&l))
	}
	if got := labels(Legend{}); !slices.Equal(got, []string{"0", "100"}) {
		t.Errorf("the labels of the look are %q, want the ends", got)
	}
	if got := labels(Legend{TickCount: 5}); !slices.Equal(got, []string{"0", "20", "40", "60", "80", "100"}) {
		t.Errorf("the labels of tickCount 5 are %q", got)
	}
	if got := labels(Legend{TickCount: 5, TickMinStep: 50}); !slices.Equal(got, []string{"0", "50", "100"}) {
		t.Errorf("the labels of tickMinStep 50 are %q", got)
	}
	if got := labels(Legend{TickCount: 50, GradientLength: 100, LabelOverlap: true}); len(got) >= 51 || got[0] != "0" {
		t.Errorf("the labels with labelOverlap are %q, want fewer than 51 from 0", got)
	}
}

// TestLegendPathSymbolRows checks that a row of a legend whose symbols are
// the path of an SVG reaches down to the bottom of the path, as Vega
// bounds an entry, so that the rows of deneb/coronation-arc stand the
// rowPadding apart. Before, a row took the height of the square of the
// size of the symbol.
func TestLegendPathSymbolRows(t *testing.T) {
	t.Parallel()
	st := style()
	sc := newScale(t, "ordinal", []any{"a", "b"}, []any{"#ff0000", "#00ff00"})
	pad := 10.0
	// A flat bar from y -0.2 to 0.2 of the square from -1 to 1.
	m := st.Legend(&Legend{Fill: sc, Format: expr.String, Columns: 1, RowPadding: &pad, SymbolSize: 400, SymbolType: "M-1,-0.2H1V0.2H-1Z"})
	syms := collect(m, "legend-symbol")
	// The side of the square is 20, so the bar reaches 10+2 down.
	if len(syms) != 2 || math.Abs(syms[1].Y-syms[0].Y-(12+pad)) > 1e-9 {
		t.Errorf("the symbols are at %v and %v, want %v apart", syms[0].Y, syms[1].Y, 12+pad)
	}
}
