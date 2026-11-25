package zuzu

import (
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestADerivedDatasetChangesCopiesOfItsRows checks that a transform of a
// dataset that derives from another adds its fields to its own rows only,
// and not to the rows of its source or of another dataset of the same
// source. vega.js copies the rows of a derived dataset that changes them,
// as test/oracle shows for each of these transforms, so zuzu does the same.
func TestADerivedDatasetChangesCopiesOfItsRows(t *testing.T) {
	t.Parallel()
	transforms := map[string]string{
		"formula":       `{"type": "formula", "as": "f", "expr": "datum.a * 2"}`,
		"stack":         `{"type": "stack", "field": "a", "groupby": ["g"]}`,
		"window":        `{"type": "window", "ops": ["row_number"], "as": ["rn"]}`,
		"joinaggregate": `{"type": "joinaggregate", "ops": ["sum"], "fields": ["a"], "as": ["sa"]}`,
		"bin":           `{"type": "bin", "field": "a", "extent": [0, 10]}`,
		"lookup":        `{"type": "lookup", "from": "L", "key": "k", "fields": ["c"], "values": ["v"]}`,
		"identifier":    `{"type": "identifier", "as": "id"}`,
		"pie":           `{"type": "pie", "field": "a"}`,
	}
	for _, name := range slices.Sorted(maps.Keys(transforms)) {
		src := `{
			"$schema": "https://vega.github.io/schema/vega/v6.json",
			"data": [
				{"name": "L", "values": [{"k": "x", "v": 10}, {"k": "y", "v": 20}]},
				{"name": "A", "values": [{"a": 1, "c": "x", "g": "p"}, {"a": 3, "c": "y", "g": "q"}, {"a": 5, "c": "x", "g": "p"}]},
				{"name": "B", "source": "A", "transform": [` + transforms[name] + `]},
				{"name": "C", "source": "A"},
				{"name": "D", "source": "A", "transform": [{"type": "formula", "as": "h", "expr": "1"}]}
			]
		}`
		c, _, err := Parse([]byte(src), WithStrict(true))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		v, _, err := c.NewView(t.Context(), WithLocation(time.UTC))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		fields := func(ds string) string {
			rows, err := v.Data(ds)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			set := map[string]bool{}
			for _, r := range rows {
				for k := range r {
					set[k] = true
				}
			}
			return strings.Join(slices.Sorted(maps.Keys(set)), ",")
		}
		for _, ds := range []string{"A", "C"} {
			if got := fields(ds); got != "a,c,g" {
				t.Errorf("%s: the rows of %s have the fields %s, want a,c,g", name, ds, got)
			}
		}
		if got := fields("D"); got != "a,c,g,h" {
			t.Errorf("%s: the rows of D have the fields %s, want a,c,g,h", name, got)
		}
		if got := fields("B"); got == "a,c,g" {
			t.Errorf("%s: the rows of B have no new field", name)
		}
	}
}
