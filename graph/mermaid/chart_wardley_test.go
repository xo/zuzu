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

// wardleyCase is what a fixture of a Wardley map draws.
type wardleyCase struct {
	components, anchors, evolves, notes, forces, pipelines, stages []string
	links                                                          [][2]string
	// annotations are the numbers and the texts of the annotations.
	annotations [][2]string
	// heads is the number of arrowheads of the links.
	heads int
}

func TestWardleyFixtures(t *testing.T) {
	t.Parallel()
	plain := []string{"Genesis", "Custom Built", "Product", "Commodity"}
	cases := map[string]wardleyCase{
		"01-tea.mmd": {
			components: []string{"Cup of Tea", "Tea", "Hot Water", "Kettle", "Power"},
			anchors:    []string{"Business"},
			links:      [][2]string{{"Business", "Cup of Tea"}, {"Cup of Tea", "Tea"}, {"Cup of Tea", "Hot Water"}, {"Hot Water", "Kettle"}, {"Kettle", "Power"}},
			evolves:    []string{"Kettle", "Power"},
			notes:      []string{"Standardising power allows Kettles to evolve faster"},
			stages:     plain,
		},
		"02-sourcing.mmd": {
			components: []string{"Custom App", "Off-the-shelf Tool", "Managed Service", "Cloud Platform", "Legacy System"},
			anchors:    []string{"Customer"},
			links: [][2]string{
				{"Customer", "Custom App"}, {"Custom App", "Off-the-shelf Tool"}, {"Custom App", "Managed Service"},
				{"Off-the-shelf Tool", "Cloud Platform"}, {"Managed Service", "Legacy System"},
			},
			stages: plain,
		},
		"03-links.mmd": {
			components: []string{"User", "App", "API", "Cache", "Database", "real-time processing"},
			links: [][2]string{
				{"User", "App"}, {"App", "API"}, {"API", "Database"}, {"API", "Cache"},
				{"Cache", "Database"}, {"App", "real-time processing"}, {"real-time processing", "API"},
			},
			evolves: []string{"Cache"},
			notes:   []string{"persists", "backup"},
			heads:   5,
			stages:  plain,
		},
		"04-pipeline.mmd": {
			components: []string{"Database", "Reports", "File System", "SQL DB", "NoSQL", "Cloud DB"},
			anchors:    []string{"Analyst"},
			links:      [][2]string{{"Analyst", "Reports"}, {"Reports", "Database"}},
			notes:      []string{"Pick a store per workload"},
			pipelines:  []string{"Database"},
			stages:     []string{"Genesis", "Custom", "Product", "Commodity"},
		},
		"05-complete.mmd": {
			components: []string{"Mobile App", "Web App", "API Gateway", "Auth Service", "Database", "Cloud Platform"},
			anchors:    []string{"Customer"},
			links: [][2]string{
				{"Customer", "Mobile App"}, {"Customer", "Web App"}, {"Mobile App", "API Gateway"}, {"Web App", "API Gateway"},
				{"API Gateway", "Auth Service"}, {"API Gateway", "Database"}, {"Database", "Cloud Platform"},
			},
			evolves:     []string{"API Gateway", "Database"},
			forces:      []string{"Cloud Native", "Legacy Data"},
			annotations: [][2]string{{"1", "User touchpoints"}, {"2", "Integration layer"}, {"3", "Data persistence"}},
			notes:       []string{"Build mobile-first experience", "Migrate to cloud-native database"},
			stages:      []string{"Genesis / Concept", "Custom / Emerging", "Product / Converging", "Commodity / Accepted"},
		},
		"06-forces.mmd": {
			components: []string{"Thing"},
			forces:     []string{"Fast Lane", "License Play"},
		},
	}
	files := fixtures(t, "wardley")
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
			if len(report) != 0 {
				t.Errorf("report: %v", report)
			}
			kind := func(mark, k, name string) int {
				return countDrawn(items, mark, func(it drawnItem) bool { return it.datum["kind"] == k && it.datum["name"] == name })
			}
			for _, name := range test.components {
				if n, l := kind(scene.Symbol, "component", name), textDrawn(items, name); n != 1 || l != 1 {
					t.Errorf("the component %q draws %d points and %d labels", name, n, l)
				}
			}
			for _, name := range test.anchors {
				if n, l := kind(scene.Text, "anchor", name), textDrawn(items, name); n != 1 || l != 1 {
					t.Errorf("the anchor %q draws %d times and %d labels", name, n, l)
				}
			}
			if n := countDrawn(items, scene.Rule, func(it drawnItem) bool { return it.datum["kind"] == "link" }); n != len(test.links) {
				t.Errorf("%d links for %d", n, len(test.links))
			}
			for _, l := range test.links {
				n := countDrawn(items, scene.Rule, func(it drawnItem) bool {
					return it.datum["kind"] == "link" && it.datum["source"] == l[0] && it.datum["target"] == l[1]
				})
				if n != 1 {
					t.Errorf("the link %v draws %d times", l, n)
				}
			}
			heads := countDrawn(items, scene.Symbol, func(it drawnItem) bool { return it.datum["kind"] == "arrow" && it.datum["of"] == "link" })
			if heads != test.heads {
				t.Errorf("%d arrowheads of links for %d", heads, test.heads)
			}
			for _, name := range test.evolves {
				head := countDrawn(items, scene.Symbol, func(it drawnItem) bool { return it.datum["of"] == "evolve" && it.datum["name"] == name })
				if n := kind(scene.Rule, "evolve", name); n != 1 || head != 1 {
					t.Errorf("the evolve of %q draws %d times with %d arrows", name, n, head)
				}
			}
			for _, s := range append(test.notes, test.stages...) {
				if n := textDrawn(items, s); n != 1 {
					t.Errorf("the text %q draws %d times", s, n)
				}
			}
			for _, name := range test.forces {
				if n, l := countDrawn(items, scene.Symbol, func(it drawnItem) bool { return it.datum["name"] == name }), textDrawn(items, name); n != 1 || l != 1 {
					t.Errorf("the force %q draws %d times and %d labels", name, n, l)
				}
			}
			for _, name := range test.pipelines {
				if n := kind(scene.Rect, "pipeline", name); n != 1 {
					t.Errorf("the pipeline %q draws %d times", name, n)
				}
			}
			for _, a := range test.annotations {
				n := countDrawn(items, scene.Symbol, func(it drawnItem) bool { return it.datum["kind"] == "annotation" && it.datum["number"] == a[0] })
				if n != 1 || textDrawn(items, a[0]) != 1 || textDrawn(items, a[0]+". "+a[1]) != 1 {
					t.Errorf("the annotation %v does not draw once with its number and its text", a)
				}
			}
		})
	}
}

func TestWardleyAxes(t *testing.T) {
	t.Parallel()
	items, _ := drawFixture(t, "testdata/wardley/01-tea.mmd")
	for _, s := range []string{"Evolution", "Visibility"} {
		if n := textDrawn(items, s); n != 1 {
			t.Errorf("the title %s of an axis draws %d times", s, n)
		}
	}
	// The place [0.95, 0.63] of Business is a visibility, then an
	// evolution: the anchor is right of the middle and near the top.
	var business, power *scene.Item
	for _, it := range items {
		switch {
		case it.datum["kind"] == "anchor" && it.datum["name"] == "Business":
			business = it.item
		case it.datum["kind"] == "component" && it.datum["name"] == "Power":
			power = it.item
		}
	}
	if business == nil || power == nil {
		t.Fatal("no Business or no Power")
	}
	if business.Y >= power.Y || business.X >= power.X {
		t.Errorf("Business at %v,%v and Power at %v,%v", business.X, business.Y, power.X, power.Y)
	}
}

func TestWardleyErrors(t *testing.T) {
	t.Parallel()
	tests := []string{
		"wardley-beta\ncomponent A [0.5]\n",
		"wardley-beta\ncomponent A [0.5, x]\n",
		"wardley-beta\ncomponent A [0.5, 0.5] (rent)\n",
		"wardley-beta\ncomponent A [0.5, 0.5] extra\n",
		"wardley-beta\ncomponent A [0.5, 0.5]\ncomponent A [0.6, 0.6]\n",
		"wardley-beta\ncomponent A [0.5, 0.5]\nA -> B\n",
		"wardley-beta\ncomponent A [0.5, 0.5]\nevolve B 0.8\n",
		"wardley-beta\ncomponent A [0.5, 0.5]\nA -.- (0.7, 0.6)\n",
		"wardley-beta\ncomponent A [0.5, 0.5]\npipeline A {\ncomponent B [0.2]\n",
		"wardley-beta\npipeline A {\ncomponent B [0.2]\n}\n",
		"wardley-beta\nevolution Genesis@0.5 -> Custom@0.4\n",
		"wardley-beta\nnote text [0.5, 0.5]\n",
		"wardley-beta\nsize [800]\n",
		"wardley-beta\nannotation x,[0.5, 0.5] \"text\"\n",
		"wardley-beta\nfoo bar\n",
		"wardley-beta\naccelerator Fast Lane\n",
	}
	for _, src := range tests {
		if _, err := chartOf(src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: the error %v", src, err)
		}
	}
}

func TestWardleyIssues(t *testing.T) {
	t.Parallel()
	body := "wardley-beta\ncomponent A [0.5, 0.5]\n"
	tests := []struct {
		src, issue string
	}{
		{"---\nconfig:\n  wardley:\n    width: 900\n---\n" + body, "wardley.width"},
		{"---\nconfig:\n  look: handDrawn\n---\n" + body, "look handDrawn"},
		{body + "accTitle: A map\n", "accessibility"},
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
