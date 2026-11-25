package zuzu

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/vega"
)

// funcCase is a case of the functions of expressions that read the view,
// which test/oracle/funcs.mjs evaluates with vega.js: a part of a spec, a
// datum, and the expressions.
type funcCase struct {
	Name  string         `json:"name"`
	Spec  map[string]any `json:"spec"`
	Datum map[string]any `json:"datum"`
	Exprs []string       `json:"exprs"`
}

// funcResult is what vega.js returned for each expression of a case.
type funcResult struct {
	Name    string  `json:"name"`
	Results [][]any `json:"results"`
}

// TestFuncsOracle runs the cases of testdata/funcs/cases.json and compares
// each result with the one of vega.js in testdata/funcs/oracle.json.
func TestFuncsOracle(t *testing.T) {
	t.Parallel()
	var cases []funcCase
	var results []funcResult
	for name, out := range map[string]any{"cases": &cases, "oracle": &results} {
		b, err := os.ReadFile("testdata/funcs/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatal(err)
		}
	}
	if len(cases) != len(results) {
		t.Fatalf("%d cases and %d results: run test/oracle/funcs.mjs again", len(cases), len(results))
	}
	for i, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			if len(c.Exprs) != len(results[i].Results) {
				t.Fatalf("%d expressions and %d results: run test/oracle/funcs.mjs again", len(c.Exprs), len(results[i].Results))
			}
			for j, e := range c.Exprs {
				want := results[i].Results[j][1]
				got, err := evalFunc(t, c, e)
				if w, ok := want.(map[string]any); ok && w["error"] != nil {
					if err == nil {
						t.Errorf("%s: got %v, want the error %v", e, encodeFunc(got), w["error"])
					}
					continue
				}
				if err != nil {
					t.Errorf("%s: %v", e, err)
					continue
				}
				if msg := diffFunc(e, encodeFunc(got), want); msg != "" {
					t.Errorf("%s: %s\ngot  %v\nwant %v", e, msg, encodeFunc(got), want)
				}
			}
		})
	}
}

// evalFunc evaluates an expression in a formula over the datum of a case,
// in a dataset after the datasets of its spec.
func evalFunc(t *testing.T, c funcCase, e string) (any, error) {
	t.Helper()
	spec := map[string]any{"$schema": "https://vega.github.io/schema/vega/v5.json"}
	maps.Copy(spec, c.Spec)
	data, _ := spec["data"].([]any)
	datum := c.Datum
	if datum == nil {
		datum = map[string]any{}
	}
	spec["data"] = append(append([]any{}, data...), map[string]any{
		"name": "zuzu_out", "values": []any{datum},
		"transform": []any{map[string]any{"type": "formula", "expr": e, "as": "r"}},
	})
	src, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	chart, _, err := Parse(src, WithStrict(true))
	if err != nil {
		return nil, err
	}
	v, _, err := chart.NewView(t.Context(), WithLocation(time.UTC))
	if err != nil {
		return nil, err
	}
	rows, err := v.Data("zuzu_out")
	if err != nil {
		return nil, err
	}
	return rows[0]["r"], nil
}

// encodeFunc writes a value as test/oracle/funcs.mjs does.
func encodeFunc(v any) any {
	switch x := v.(type) {
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return map[string]any{"number": expr.String(x)}
		}
		return x
	case time.Time:
		return map[string]any{"date": float64(x.UnixMilli())}
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = encodeFunc(e)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			out[k] = encodeFunc(e)
		}
		return out
	case scaleCopy:
		return map[string]any{"scale": map[string]any{"domain": encodeFunc(x.s.Domain()), "range": encodeFunc(x.s.Range())}}
	case projectionCopy:
		return map[string]any{"projection": true}
	case geoShape:
		return map[string]any{"shape": x.path}
	}
	if expr.IsUndefined(v) {
		return map[string]any{"undefined": true}
	}
	return v
}

// diffFunc says how a result differs from the one of vega.js, or returns
// "". Numbers match to 1e-9, and colors match by their value. A set of
// vega.js matches a list. A longitude of a centroid at a pole matches any
// longitude, and -180 matches 180.
func diffFunc(e string, got, want any) string {
	if strings.HasPrefix(e, "geoCentroid(null") {
		g, ok1 := got.([]any)
		w, ok2 := want.([]any)
		if ok1 && ok2 && len(g) == 2 && len(w) == 2 {
			if lat, ok := w[1].(float64); ok && math.Abs(lat) > 90-1e-6 {
				return diffValue(g[1], w[1])
			}
			if lon, ok := g[0].(float64); ok && math.Abs(math.Abs(lon)-180) < 1e-9 {
				g = []any{math.Copysign(180, expr.Number(w[0])), g[1]}
			}
			return diffValue(g, w)
		}
	}
	return diffValue(got, want)
}

func diffValue(got, want any) string {
	switch w := want.(type) {
	case float64:
		g, ok := got.(float64)
		if !ok || math.Abs(g-w) > 1e-9*math.Max(1, math.Abs(w)) {
			return fmt.Sprintf("%v is not %v", got, want)
		}
		return ""
	case string:
		g, ok := got.(string)
		if !ok || g != w && !sameColor(g, w) {
			return fmt.Sprintf("%q is not %q", got, want)
		}
		return ""
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return fmt.Sprintf("%v is not a list of %d", got, len(w))
		}
		for i := range w {
			if msg := diffValue(g[i], w[i]); msg != "" {
				return fmt.Sprintf("[%d]: %s", i, msg)
			}
		}
		return ""
	case map[string]any:
		if set, ok := w["set"]; ok && len(w) == 1 {
			return diffValue(got, set)
		}
		if _, ok := w["projection"]; ok {
			if g, ok := got.(map[string]any); !ok || g["projection"] == nil {
				return fmt.Sprintf("%v is not a projection", got)
			}
			return ""
		}
		g, ok := got.(map[string]any)
		if !ok || len(g) != len(w) {
			return fmt.Sprintf("%v has not the keys of %v", got, want)
		}
		for k := range w {
			if msg := diffValue(g[k], w[k]); msg != "" {
				return fmt.Sprintf(".%s: %s", k, msg)
			}
		}
		return ""
	}
	if got != want {
		return fmt.Sprintf("%v is not %v", got, want)
	}
	return ""
}

// TestGeoShapeMark draws a shape mark whose shape is geoShape.
func TestGeoShapeMark(t *testing.T) {
	t.Parallel()
	src := `{"$schema": "https://vega.github.io/schema/vega/v5.json", "width": 400, "height": 300,
		"projections": [{"name": "p", "type": "mercator", "scale": 100, "translate": [300, 200]}],
		"data": [{"name": "d", "values": [{"type": "Polygon", "coordinates": [[[0, 0], [0, 10], [10, 10], [10, 0], [0, 0]]]}]}],
		"marks": [{"type": "shape", "from": {"data": "d"},
			"encode": {"update": {"shape": {"signal": "geoShape('p', datum)"}, "fill": {"value": "red"}}}}]}`
	c, _, err := Parse([]byte(src), WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	sc, _, err := c.Render(t.Context(), WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(sc)
	if err != nil {
		t.Fatal(err)
	}
	if want := "M300,200L300,182.457L317.453,182.457L317.453,200Z"; !strings.Contains(string(b), want) {
		t.Errorf("the scene has no path %s", want)
	}
}

// TestGradientOfDiscreteScale reports gradient of a scale that is not
// continuous, which zuzu does not sample.
func TestGradientOfDiscreteScale(t *testing.T) {
	t.Parallel()
	src := `{"$schema": "https://vega.github.io/schema/vega/v5.json",
		"scales": [{"name": "c", "type": "ordinal", "domain": ["a", "b"], "range": ["red", "blue"]}],
		"data": [{"name": "d", "values": [{}], "transform": [{"type": "formula", "expr": "gradient('c')", "as": "g"}]}]}`
	c, _, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	_, report, err := c.NewView(t.Context(), WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, is := range report {
		found = found || errors.Is(is, vega.ErrUnsupportedFunction) && strings.Contains(is.Error(), "gradient")
	}
	if !found {
		t.Errorf("the report = %v, want gradient of an ordinal scale", report)
	}
}

// TestWithFunctionAddsAName checks that an expression can call a function
// that the caller adds with WithFunction under a name that Vega does not
// have, as docs/EXPRESSIONS.md says.
func TestWithFunctionAddsAName(t *testing.T) {
	t.Parallel()
	src := `{"$schema": "https://vega.github.io/schema/vega/v6.json", "width": 10, "height": 10,
		"signals": [{"name": "s", "update": "double(21)"}]}`
	double := func(_ *expr.Scope, args []any) (any, error) { return expr.Number(args[0]) * 2, nil }
	c, report, err := Parse([]byte(src), WithStrict(true), WithFunction("double", double))
	if err != nil || len(report) != 0 {
		t.Fatalf("the error %v and the report %v", err, report)
	}
	v, _, err := c.NewView(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := v.Signal("s"); err != nil || got != 42.0 {
		t.Errorf("the signal %v, %v, want 42", got, err)
	}
}
