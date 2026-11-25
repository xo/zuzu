package zuzu

import (
	"cmp"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"math"
	"math/rand/v2"
	"os"
	"slices"
	"testing"
	"time"
)

// TestASortedDomainOfMixedValuesIsThatOfVega checks that a domain sorted
// from values that mix numbers, strings, null and booleans has the order
// that vega.js gives, which test/oracle/domains.mjs writes to
// testdata/domains.json.
func TestASortedDomainOfMixedValuesIsThatOfVega(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("testdata/domains.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases struct {
		Cases []struct {
			Values []any          `json:"values"`
			Sort   jsontext.Value `json:"sort"`
			Domain jsontext.Value `json:"domain"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases.Cases) == 0 {
		t.Fatal("no cases")
	}
	for i, c := range cases.Cases {
		rows := make([]map[string]any, len(c.Values))
		for j, v := range c.Values {
			rows[j] = map[string]any{"v": v}
		}
		spec, err := json.Marshal(map[string]any{
			"data": []any{map[string]any{"name": "table", "values": rows}},
			"scales": []any{map[string]any{
				"name": "x", "type": "band", "range": []any{0, 100},
				"domain": map[string]any{"data": "table", "field": "v", "sort": c.Sort},
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		ch, _, err := Parse(spec, WithDialect(Vega), WithStrict(true))
		if err != nil {
			t.Fatal(err)
		}
		view, _, err := ch.NewView(t.Context(), WithLocation(time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		s, err := view.Scale("x")
		if err != nil {
			t.Fatal(err)
		}
		var want []any
		if err := json.Unmarshal(c.Domain, &want); err != nil {
			t.Fatal(err)
		}
		if at := firstDifference(s.Domain(), want); at >= 0 {
			lo, hi := max(at-3, 0), min(at+4, len(want))
			t.Errorf("case %d, %d values, sort %s: the domain differs at %d of %d:\ngot  %s\nwant %s",
				i, len(c.Values), c.Sort, at, len(want), domainText(s.Domain(), lo, hi), domainText(want, lo, hi))
		}
	}
}

// firstDifference returns where two domains first differ, or -1.
func firstDifference(got, want []any) int {
	for i := range max(len(got), len(want)) {
		if i >= len(got) || i >= len(want) {
			return i
		}
		g, err1 := json.Marshal(got[i])
		w, err2 := json.Marshal(want[i])
		if err1 != nil || err2 != nil || string(g) != string(w) {
			return i
		}
	}
	return -1
}

// domainText writes the values of a domain from lo to hi.
func domainText(d []any, lo, hi int) string {
	b, err := json.Marshal(d[min(lo, len(d)):min(hi, len(d))])
	if err != nil {
		return err.Error()
	}
	return string(b)
}

// TestJSSortIsStableOnATotalOrder checks that the sort of JavaScript gives
// the order of a stable sort when the comparison is a total order, for
// lengths that take each path of the sort: a run alone, runs that merge
// from the left and from the right, and merges that gallop.
func TestJSSortIsStableOnATotalOrder(t *testing.T) {
	t.Parallel()
	type item struct{ key, index int }
	r := rand.New(rand.NewPCG(1, 2))
	for _, n := range []int{0, 1, 2, 5, 63, 64, 65, 200, 1000, 5000} {
		for _, spread := range []int{3, 50, 1 << 20} {
			for _, shape := range []string{"random", "sorted", "reversed", "runs"} {
				a := make([]item, n)
				for i := range a {
					k := r.IntN(spread)
					switch shape {
					case "sorted":
						k = i * spread / max(n, 1)
					case "reversed":
						k = (n - i) * spread / max(n, 1)
					case "runs":
						k = (i%97)*spread/97 + r.IntN(2)
					}
					a[i] = item{key: k, index: i}
				}
				want := slices.Clone(a)
				slices.SortStableFunc(want, func(x, y item) int { return cmp.Compare(x.key, y.key) })
				jsSort(a, func(x, y item) int { return cmp.Compare(x.key, y.key) })
				if !slices.Equal(a, want) {
					t.Errorf("%d items, spread %d, %s: the order differs from a stable sort", n, spread, shape)
				}
			}
		}
	}
}

// TestVegaAscendingComparesAsJavaScript checks the comparisons of the
// ascending order of Vega on values of mixed types.
func TestVegaAscendingComparesAsJavaScript(t *testing.T) {
	t.Parallel()
	day := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		a, b any
		want int
	}{
		{nil, 1.0, -1},
		{"a", nil, 1},
		{nil, nil, 0},
		{math.NaN(), 1.0, -1},
		{1.0, math.NaN(), 1},
		{math.NaN(), "a", -1},
		{"12 Angry Men", 1776.0, 0},
		{"12", 1776.0, -1},
		{" 5 ", 4.0, 1},
		{"", 0.0, 0},
		{"0x10", 15.0, 1},
		{true, 0.5, 1},
		{"B", "a", -1},
		{"\uff21", "\U0001d518", 1},
		{"\U0001d518", "\U0001d519", -1},
		{"ab", "abc", -1},
		{day, float64(day.UnixMilli()) + 1, -1},
	}
	for _, tt := range tests {
		if got := vegaAscending(tt.a, tt.b); got != tt.want {
			t.Errorf("vegaAscending(%#v, %#v) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
