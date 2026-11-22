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
