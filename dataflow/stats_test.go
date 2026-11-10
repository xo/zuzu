package dataflow

import (
	"math"
	"testing"
)

func TestStatisticsTransforms(t *testing.T) {
	t.Parallel()
	rows := `[{"k":"a","x":1,"y":3},{"k":"a","x":2,"y":5},{"k":"a","x":3,"y":7},{"k":"b","x":1,"y":1},{"k":"b","x":3,"y":5}]`
	tests := []struct {
		name, ts, want string
	}{
		{"quantile", `[{"type":"quantile","field":"y","probs":[0,0.5,1]}]`,
			`[{"prob":0,"value":1},{"prob":0.5,"value":5},{"prob":1,"value":7}]`},
		{"quantile by group", `[{"type":"quantile","field":"y","groupby":["k"],"probs":[0.5]}]`,
			`[{"k":"a","prob":0.5,"value":5},{"k":"b","prob":0.5,"value":3}]`},
		{"linear regression", `[{"type":"regression","x":"x","y":"y","groupby":["k"]}]`,
			`[{"k":"a","x":1,"y":3},{"k":"a","x":3,"y":7},{"k":"b","x":1,"y":1},{"k":"b","x":3,"y":5}]`},
		{"regression params", `[{"type":"regression","x":"x","y":"y","params":true,"groupby":["k"]}]`,
			`[{"coef":[1,2],"keys":["a"],"rSquared":1},{"coef":[-1,2],"keys":["b"],"rSquared":1}]`},
	}
	for _, test := range tests {
		if got := run(t, newEnv(), rows, test.ts); got != test.want {
			t.Errorf("%s:\ngot  %s\nwant %s", test.name, got, test.want)
		}
	}
}

func TestLoessFitsALine(t *testing.T) {
	t.Parallel()
	xs, ys := []float64{1, 2, 3, 4, 5}, []float64{3, 5, 7, 9, 11}
	for i, got := range loessFit(xs, ys, 1) {
		if math.Abs(got-ys[i]) > 1e-9 {
			t.Errorf("fit at %v = %v, want %v", xs[i], got, ys[i])
		}
	}
}

func TestKDE(t *testing.T) {
	t.Parallel()
	rows := rowsOf(t, `[{"v":0},{"v":null},{"v":2}]`)
	out, err := Run(newEnv(), rows, transformsOf(t, `[{"type":"kde","field":"v","bandwidth":1,"extent":[0,2],"steps":2,"counts":true}]`), "/transform")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("got %d rows, want 3", len(out))
	}
	// The null counts as 0, as in vega.js: at 1 the three kernels sum to
	// 3 times the density of a unit normal at 1.
	want := 3 * math.Exp(-0.5) / math.Sqrt(2*math.Pi)
	if got, ok := out[1]["density"].(float64); !ok || math.Abs(got-want) > 1e-12 {
		t.Errorf("density at 1 = %v, want %v", got, want)
	}
}

func TestSampleCurve(t *testing.T) {
	t.Parallel()
	line := sampleCurve(func(x float64) float64 { return 2 * x }, 0, 1, 4, 64)
	if len(line) != 5 {
		t.Errorf("a line takes %d samples, want the 5 of the first steps", len(line))
	}
	bend := sampleCurve(func(x float64) float64 { return math.Abs(x - 0.3) }, 0, 1, 4, 64)
	if len(bend) <= 5 {
		t.Errorf("a bend takes %d samples, want more than 5", len(bend))
	}
	for i := 1; i < len(bend); i++ {
		if bend[i][0] <= bend[i-1][0] {
			t.Fatalf("the samples are not in order at %d", i)
		}
	}
}
