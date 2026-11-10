package expr

import (
	"encoding/json/v2"
	"errors"
	"math"
	"math/rand/v2"
	"os"
	"strconv"
	"testing"
	"time"
	_ "time/tzdata" // The tests read a named zone, not the zone of the machine.
)

// datum is the row that test/oracle/exprs.mjs evaluates each expression
// over.
func datum() map[string]any {
	return map[string]any{
		"a": 3.0, "b": "Hello, World", "c": nil, "d": []any{1.0, 2.0, 3.0}, "n": -2.5,
		"s": "2024-02-29", "t": float64(time.Date(2024, 2, 29, 17, 30, 5, 7e6, time.UTC).UnixMilli()),
		"o": map[string]any{"x": 1.0, "y": "two"}, "z": 0.0, "e": "",
	}
}

// decode reads a value that test/oracle/exprs.mjs encoded.
func decode(v any) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = decode(e)
		}
		return out
	case map[string]any:
		if len(x) == 1 {
			switch {
			case x["undefined"] == true:
				return Undefined
			case x["date"] != nil:
				ms, _ := x["date"].(float64)
				return time.UnixMilli(int64(ms))
			case x["number"] != nil:
				switch x["number"] {
				case "NaN":
					return math.NaN()
				case "-0":
					return math.Copysign(0, -1)
				}
				s, _ := x["number"].(string)
				f, _ := strconv.ParseFloat(s, 64)
				return f
			}
		}
		out := map[string]any{}
		for k, e := range x {
			out[k] = decode(e)
		}
		return out
	}
	return v
}

// same reports whether two values are the same, with numbers within a
// relative 1e-12, since vega.js computes some functions another way.
func same(a, b any) bool {
	switch x := a.(type) {
	case float64:
		y, ok := b.(float64)
		switch {
		case !ok:
			return false
		case math.IsNaN(x) || math.IsNaN(y):
			return math.IsNaN(x) && math.IsNaN(y)
		case x == y:
			return math.Signbit(x) == math.Signbit(y)
		}
		return math.Abs(x-y) <= 1e-12*math.Max(math.Abs(x), math.Abs(y))
	case time.Time:
		y, ok := b.(time.Time)
		return ok && x.UnixMilli() == y.UnixMilli()
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !same(x[i], y[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k := range x {
			if !same(x[k], y[k]) {
				return false
			}
		}
		return true
	}
	return a == b
}

// TestOracle checks the evaluator against vega.js, with the results that
// testdata/eval.json holds.
func TestOracle(t *testing.T) {
	t.Parallel()
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	buf, err := os.ReadFile("testdata/eval.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows [][2]any
	if err := json.Unmarshal(buf, &rows); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		src, _ := r[0].(string)
		want := decode(r[1])
		p, err := Compile(src)
		if err != nil {
			t.Errorf("Compile(%q): %v", src, err)
			continue
		}
		got, err := p.Eval(&Scope{Datum: datum(), Location: zone})
		if err != nil {
			t.Errorf("Eval(%q): %v", src, err)
			continue
		}
		if !same(got, want) {
			t.Errorf("Eval(%q) = %#v, want %#v", src, got, want)
		}
	}
}

func TestCompileErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src  string
		want error
	}{
		{"1 +", ErrSyntax},
		{"(1, 2)", ErrSyntax},
		{"a = 1", ErrNotAllowed},
		{"2 ** 3", ErrSyntax},
		{"null ?? 1", ErrSyntax},
		{"'unterminated", ErrSyntax},
		{"this", ErrNotAllowed},
		{"new Date()", ErrNotAllowed},
		{"typeof 1", ErrNotAllowed},
		{"nosuch(1)", ErrUnknownFunction},
		{"datum.x.y(1)", ErrNotAllowed},
	}
	for _, test := range tests {
		_, err := Compile(test.src)
		if !errors.Is(err, test.want) {
			t.Errorf("Compile(%q) = %v, want %v", test.src, err, test.want)
		}
	}
}

// signals is a map of signals.
type signals map[string]any

func (s signals) Signal(name string) (any, bool) {
	v, ok := s[name]
	return v, ok
}

func TestScope(t *testing.T) {
	t.Parallel()
	eval := func(src string, s *Scope) any {
		t.Helper()
		p, err := Compile(src)
		if err != nil {
			t.Fatal(err)
		}
		v, err := p.Eval(s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	s := &Scope{
		Signals: signals{"width": 400.0},
		Parent:  map[string]any{"k": "p"},
		Functions: map[string]Func{
			"scale": func(_ *Scope, args []any) (any, error) {
				return String(args[0]) + ":" + String(args[1]), nil
			},
			"upper": func(_ *Scope, _ []any) (any, error) { return "mine", nil },
		},
		Rand: rand.New(rand.NewPCG(1, 2)),
	}
	if got := eval("width / 2", s); got != 200.0 {
		t.Errorf("width / 2 = %v, want 200", got)
	}
	if got := eval("parent.k", s); got != "p" {
		t.Errorf("parent.k = %v, want p", got)
	}
	if got := eval("scale('x', 3)", s); got != "x:3" {
		t.Errorf("scale('x', 3) = %v, want x:3", got)
	}
	if got := eval("upper('a')", s); got != "mine" {
		t.Errorf("upper('a') = %v, want the function of the scope", got)
	}
	r1 := eval("random()", s)
	s.Rand = rand.New(rand.NewPCG(1, 2))
	if r2 := eval("random()", s); r1 != r2 {
		t.Errorf("random() with the same source = %v and %v", r1, r2)
	}
	p, _ := Compile("nosignal + 1")
	if _, err := p.Eval(s); !errors.Is(err, ErrUnknownName) {
		t.Errorf("an unknown name = %v, want %v", err, ErrUnknownName)
	}
	p, _ = Compile("format(1, 'zz')")
	if _, err := p.Eval(s); !errors.Is(err, ErrArgument) {
		t.Errorf("a bad format = %v, want %v", err, ErrArgument)
	}
}
