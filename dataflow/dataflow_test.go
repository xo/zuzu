package dataflow

import (
	"encoding/json/v2"
	"errors"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/vega"
)

// env is an Env with a few datasets and signals.
type env struct {
	data    map[string][]Row
	signals map[string]any
	rand    *rand.Rand
}

func (e *env) Compile(src string) (*expr.Program, error) { return expr.Compile(src) }
func (e *env) Scope(datum any) *expr.Scope {
	return &expr.Scope{Datum: datum, Signals: e, Location: time.UTC}
}
func (e *env) Data(name string) ([]Row, error) { return e.data[name], nil }
func (e *env) SetSignal(name string, v any)    { e.signals[name] = v }
func (e *env) Signal(name string) (any, bool)  { v, ok := e.signals[name]; return v, ok }
func (e *env) Location() *time.Location        { return time.UTC }
func (e *env) Random() float64                 { return e.rand.Float64() }

// transformsOf reads transforms from JSON.
func transformsOf(t *testing.T, src string) []vega.Transform {
	t.Helper()
	var ts []vega.Transform
	if err := json.Unmarshal([]byte(src), &ts); err != nil {
		t.Fatal(err)
	}
	return ts
}

// rowsOf reads rows from JSON.
func rowsOf(t *testing.T, src string) []Row {
	t.Helper()
	var rows []Row
	if err := json.Unmarshal([]byte(src), &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

// run runs transforms and returns the rows as JSON.
func run(t *testing.T, e *env, rows, ts string) string {
	t.Helper()
	out, err := Run(e, rowsOf(t, rows), transformsOf(t, ts), "/transform")
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(out, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func newEnv() *env {
	return &env{data: map[string][]Row{}, signals: map[string]any{}, rand: rand.New(rand.NewPCG(1, 2))}
}

func TestTransforms(t *testing.T) {
	t.Parallel()
	rows := `[{"k":"a","v":1},{"k":"b","v":2},{"k":"a","v":3},{"k":"b","v":-4}]`
	tests := []struct {
		name, ts, want string
	}{
		{"filter", `[{"type":"filter","expr":"datum.v > 1"}]`,
			`[{"k":"b","v":2},{"k":"a","v":3}]`},
		{"formula", `[{"type":"formula","expr":"datum.v * 2","as":"w"}]`,
			`[{"k":"a","v":1,"w":2},{"k":"b","v":2,"w":4},{"k":"a","v":3,"w":6},{"k":"b","v":-4,"w":-8}]`},
		{"aggregate", `[{"type":"aggregate","groupby":["k"],"ops":["sum","count","mean"],"fields":["v",null,"v"]}]`,
			`[{"count":2,"k":"a","mean_v":2,"sum_v":4},{"count":2,"k":"b","mean_v":-1,"sum_v":-2}]`},
		{"stack", `[{"type":"stack","field":"v","groupby":[]}]`,
			`[{"k":"a","v":1,"y0":0,"y1":1},{"k":"b","v":2,"y0":1,"y1":3},{"k":"a","v":3,"y0":3,"y1":6},{"k":"b","v":-4,"y0":0,"y1":-4}]`},
		{"collect", `[{"type":"collect","sort":{"field":"v","order":"descending"}}]`,
			`[{"k":"a","v":3},{"k":"b","v":2},{"k":"a","v":1},{"k":"b","v":-4}]`},
		{"window", `[{"type":"window","ops":["row_number","sum"],"fields":[null,"v"],"as":["n","cum"],"groupby":["k"]}]`,
			`[{"cum":1,"k":"a","n":1,"v":1},{"cum":2,"k":"b","n":1,"v":2},{"cum":4,"k":"a","n":2,"v":3},{"cum":-2,"k":"b","n":2,"v":-4}]`},
		{"fold", `[{"type":"fold","fields":["v"]},{"type":"filter","expr":"datum.k == 'a'"}]`,
			`[{"k":"a","key":"v","v":1,"value":1},{"k":"a","key":"v","v":3,"value":3}]`},
		{"bin", `[{"type":"bin","field":"v","extent":[-4,3],"maxbins":4}]`,
			`[{"bin0":0,"bin1":2,"k":"a","v":1},{"bin0":2,"bin1":4,"k":"b","v":2},{"bin0":2,"bin1":4,"k":"a","v":3},{"bin0":-4,"bin1":-2,"k":"b","v":-4}]`},
	}
	for _, test := range tests {
		if got := run(t, newEnv(), rows, test.ts); got != test.want {
			t.Errorf("%s:\ngot  %s\nwant %s", test.name, got, test.want)
		}
	}
}

func TestExtentSetsSignal(t *testing.T) {
	t.Parallel()
	e := newEnv()
	run(t, e, `[{"v":3},{"v":-1},{"v":null}]`, `[{"type":"extent","field":"v","signal":"ext"}]`)
	got, _ := json.Marshal(e.signals["ext"])
	if string(got) != "[-1,3]" {
		t.Errorf("ext = %s, want [-1,3]", got)
	}
}

func TestRead(t *testing.T) {
	t.Parallel()
	rows, err := Read([]byte("a,b\n1,x\n2,\"y, z\"\n"), Format{Type: "csv", Parse: map[string]string{"a": "number"}}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(rows, json.Deterministic(true))
	if want := `[{"a":1,"b":"x"},{"a":2,"b":"y, z"}]`; string(got) != want {
		t.Errorf("Read = %s, want %s", got, want)
	}
	rows, err = Read([]byte(`{"values":[{"d":"2024-01-02"},{"d":"2024-02-03"}]}`), Format{Property: "values", Auto: true}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if d, ok := rows[0]["d"].(float64); !ok || d != 1704153600000 {
		t.Errorf("Read auto date = %v, want 1704153600000", rows[0]["d"])
	}
}

func TestUnsupported(t *testing.T) {
	t.Parallel()
	_, err := Run(newEnv(), nil, transformsOf(t, `[{"type":"nosuch"}]`), "/transform")
	if _, ok := errors.AsType[vega.Issue](err); !ok || !errors.Is(err, vega.ErrUnsupportedTransform) {
		t.Errorf("Run = %v, want an issue of an unsupported transform", err)
	}
}
