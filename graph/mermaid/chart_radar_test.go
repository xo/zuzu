package mermaid_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/vega"
)

// radarCase is what a fixture of a radar draws: its axes, as an id and a
// label, and the values of each curve, in the order of the axes.
type radarCase struct {
	axes   [][2]string
	curves map[string][]float64
	legend bool
	report int
}

func TestRadarFixtures(t *testing.T) {
	t.Parallel()
	letters := [][2]string{{"A", "A"}, {"B", "B"}, {"C", "C"}, {"D", "D"}, {"E", "E"}}
	cases := map[string]radarCase{
		"01-grades.mmd": {
			axes: [][2]string{{"m", "Math"}, {"s", "Science"}, {"e", "English"}, {"h", "History"}, {"g", "Geography"}, {"a", "Art"}},
			curves: map[string][]float64{
				"Alice": {85, 90, 80, 70, 75, 90},
				"Bob":   {70, 75, 85, 80, 90, 85},
			},
			legend: true,
		},
		"02-restaurants.mmd": {
			axes: [][2]string{{"food", "Food Quality"}, {"service", "Service"}, {"price", "Price"}, {"ambiance", "Ambiance"}},
			curves: map[string][]float64{
				"Restaurant A": {4, 3, 2, 4},
				"Restaurant B": {3, 4, 3, 3},
				"Restaurant C": {2, 3, 4, 2},
				"Restaurant D": {2, 2, 4, 3},
			},
			legend: true,
		},
		"03-keys.mmd": {
			axes: [][2]string{{"go", "Go"}, {"sql", "SQL"}, {"ops", "Ops"}, {"ux", "Design"}, {"test", "Testing"}},
			curves: map[string][]float64{
				"Ana": {9, 6, 4, 3, 7},
				"Ben": {4, 7, 6, 8, 5},
				"Cai": {6, 8, 8, 2, 5},
			},
			legend: true,
		},
		"04-plain.mmd": {
			axes:   letters,
			curves: map[string][]float64{"c1": {1, 2, 3, 4, 5}, "c2": {5, 4, 3, 2, 1}},
		},
		"05-config.mmd": {
			axes:   letters,
			curves: map[string][]float64{"c1": {1, 2, 3, 4, 5}, "c2": {5, 4, 3, 2, 1}, "c3": {3, 3, 3, 3, 3}},
			legend: true,
		},
		"06-lines.mmd": {
			axes:   letters,
			curves: map[string][]float64{"One": {1, 2, 3, 4, 5}, "c2": {5, 4, 3, 2, 1}},
			legend: true,
		},
	}
	files := fixtures(t, "radar")
	if len(files) != len(cases) {
		t.Fatalf("%d fixtures and %d cases", len(files), len(cases))
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			test, ok := cases[filepath.Base(path)]
			if !ok {
				t.Fatal("no case")
			}
			items, report := drawFixture(t, path)
			if len(report) != test.report {
				t.Errorf("report: %v", report)
			}
			for _, a := range test.axes {
				if n := textDrawn(items, a[1]); n != 1 {
					t.Errorf("the axis %s draws its label %d times", a[1], n)
				}
				if n := countDrawn(items, scene.Rule, func(it drawnItem) bool { return it.datum["axis"] == a[0] }); n != 1 {
					t.Errorf("the axis %s draws %d times", a[0], n)
				}
			}
			for name, values := range test.curves {
				if n := countDrawn(items, scene.Path, func(it drawnItem) bool { return it.datum["curve"] == name }); n != 1 {
					t.Errorf("the curve %s draws %d times", name, n)
				}
				label := 0
				if test.legend {
					label = 1
				}
				if n := textDrawn(items, name); n != label {
					t.Errorf("the curve %s draws its label %d times", name, n)
				}
				for i, v := range values {
					n := countDrawn(items, scene.Symbol, func(it drawnItem) bool {
						return it.datum["curve"] == name && it.datum["axis"] == test.axes[i][0] && it.datum["value"] == v
					})
					if n != 1 {
						t.Errorf("the value %v of %s on %s draws %d times", v, name, test.axes[i][0], n)
					}
				}
			}
		})
	}
}

func TestRadarErrors(t *testing.T) {
	t.Parallel()
	tests := []string{
		"radar-beta\naxis A, B\ncurve c{1, 2}\n",
		"radar-beta\naxis A, B, C\ncurve c{1, 2}\n",
		"radar-beta\naxis A, B, C\ncurve c{A: 1, 2, 3}\n",
		"radar-beta\naxis A, B, C\ncurve c{A: 1, B: 2, D: 3}\n",
		"radar-beta\naxis A, B, C\ncurve c{A: 1, B: 2, B: 3}\n",
		"radar-beta\naxis A, B, C\ncurve c{1, 2, x}\n",
		"radar-beta\naxis A, B, C\ncurve c{1, 2, 3}\ngraticule square\n",
		"radar-beta\naxis A, B, C\ncurve c{1, 2, 3}\nticks 0\n",
		"radar-beta\naxis A, B, C\ncurve c{1, 2, 3}\nmax 0\n",
		"radar-beta\naxis A, B, C\ncurve c{1, 2, 3}\nshowLegend maybe\n",
		"radar-beta\naxis A, B, C\nline c{1, 2, 3}\n",
		"radar-beta\naxis A[Label], B, C\n",
	}
	for _, src := range tests {
		if _, err := chartOf(src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: the error %v", src, err)
		}
	}
}

func TestRadarIssues(t *testing.T) {
	t.Parallel()
	body := "radar-beta\naxis A, B, C\ncurve c{1, 2, 3}, d{3, 2, 1}\n"
	tests := []struct {
		src, issue string
	}{
		{"---\nconfig:\n  radar:\n    useWidth: 500\n---\n" + body, "radar.useWidth"},
		{"---\nconfig:\n  themeVariables:\n    cScale0: \"#ff0000\"\n---\n" + body, "cScale"},
		{"---\nconfig:\n  themeVariables:\n    fontSize: 20\n---\n" + body, "theme variable fontSize"},
		{body + "accTitle: A radar\n", "accessibility"},
	}
	for _, test := range tests {
		report, err := chartOf(test.src, false)
		if err != nil {
			t.Fatalf("%q: %v", test.src, err)
		}
		if len(report) != 1 || !strings.Contains(report[0].Error(), test.issue) {
			t.Errorf("%q: the report %v, not %q", test.src, report, test.issue)
		}
		if _, err := chartOf(test.src, true); !errors.Is(err, vega.ErrUnsupported) {
			t.Errorf("%q: in the strict mode, the error %v", test.src, err)
		}
	}
}

func TestRadarColors(t *testing.T) {
	t.Parallel()
	items, _ := drawFixture(t, "testdata/radar/05-config.mmd")
	want := map[string]string{"c1": "#FF0000", "c2": "#00FF00", "c3": "#0000FF"}
	for name, c := range want {
		n := countDrawn(items, scene.Path, func(it drawnItem) bool {
			return it.datum["curve"] == name && strings.EqualFold(it.item.Stroke.Color, c)
		})
		if n != 1 {
			t.Errorf("the curve %s is not in the color %s", name, c)
		}
	}
}
