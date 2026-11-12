package mermaid_test

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/icon"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/vega"
)

// archWant is what a fixture of an architecture diagram draws: the label
// of each service and of each group, the parent of each group, and the
// number of junctions, edges, arrows and labels of edges.
type archWant struct {
	services                          map[string]string
	groups                            map[string]string
	parents                           map[string]string
	junctions, edges, arrows, eLabels int
}

func TestArchitectureFixtures(t *testing.T) {
	t.Parallel()
	cases := map[string]archWant{
		"01-basic.mmd": {
			services: map[string]string{"db": "Database", "disk1": "Storage", "disk2": "Storage", "server": "Server"},
			groups:   map[string]string{"api": "API"},
			edges:    3,
		},
		"02-junctions.mmd": {
			services: map[string]string{
				"left_disk": "Disk", "top_disk": "Disk", "bottom_disk": "Disk",
				"top_gateway": "Gateway", "bottom_gateway": "Gateway",
			},
			junctions: 2, edges: 6,
		},
		"03-edges.mmd": {
			services: map[string]string{"a": "A", "b": "B", "c": "C", "d": "D", "e": "E", "f": "F"},
			edges:    5, arrows: 4, eLabels: 1,
		},
		"04-groups.mmd": {
			services: map[string]string{"web": "Web", "api": "API", "db": "Database", "disk": "Backups", "user": "User"},
			groups:   map[string]string{"cloud": "Cloud", "private": "Private subnet", "public": "Public subnet"},
			parents:  map[string]string{"private": "cloud", "public": "cloud"},
			edges:    4, arrows: 2,
		},
		"05-icons.mmd": {
			services: map[string]string{"queue": "Queue", "worker": "Worker", "store": "Store", "legacy": "Legacy system", "blank": "No icon"},
			groups:   map[string]string{"k8s": "Cluster"},
			edges:    3,
		},
	}
	files := fixtures(t, "architecture")
	if len(files) != len(cases) {
		t.Fatalf("%d fixtures and %d cases", len(files), len(cases))
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			want, ok := cases[filepath.Base(path)]
			if !ok {
				t.Fatal("no case")
			}
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			items, _, err := drawDiagram(t, string(src), true)
			if err != nil {
				t.Fatalf("draw in the strict mode: %v", err)
			}
			checkArchitecture(t, items, want)
			checkSides(t, items, string(src))
		})
	}
}

// datumID returns the id of the datum of an item.
func datumID(it drawnItem) string {
	id, _ := it.datum["id"].(string)
	return id
}

// box is a box of a tile or a group.
type box struct {
	x0, y0, x1, y1 float64
}

// inside reports whether a box is inside another.
func (b box) inside(o box) bool {
	return b.x0 >= o.x0 && b.y0 >= o.y0 && b.x1 <= o.x1 && b.y1 <= o.y1
}

// overlaps reports whether two boxes overlap.
func (b box) overlaps(o box) bool {
	return b.x0 < o.x1-0.5 && o.x0 < b.x1-0.5 && b.y0 < o.y1-0.5 && o.y0 < b.y1-0.5
}

// boxOf returns the box of an item of a rect.
func boxOf(it drawnItem) box {
	return box{it.item.X, it.item.Y, it.item.X + it.item.Width, it.item.Y + it.item.Height}
}

// checkArchitecture checks the services, the groups, the junctions, the
// edges and their arrows and labels of an architecture diagram that drew.
func checkArchitecture(t *testing.T, items []drawnItem, want archWant) {
	t.Helper()
	tiles := map[string]box{}
	parents := map[string]string{}
	for _, it := range drawnOf(items, "tiles") {
		id := datumID(it)
		if _, ok := tiles[id]; ok {
			t.Errorf("the service %s draws twice", id)
		}
		tiles[id] = boxOf(it)
		parents[id], _ = it.datum["parent"].(string)
	}
	if len(tiles) != len(want.services) {
		t.Errorf("%d services, expected %d", len(tiles), len(want.services))
	}
	labels := map[string]string{}
	for _, it := range drawnOf(items, "labels") {
		labels[datumID(it)] = it.text
	}
	for id, label := range want.services {
		if labels[id] != label {
			t.Errorf("the service %s draws the label %q, expected %q", id, labels[id], label)
		}
	}
	groups := map[string]box{}
	for _, it := range drawnOf(items, "groups") {
		id := datumID(it)
		groups[id] = boxOf(it)
		parents[id], _ = it.datum["parent"].(string)
	}
	titles := map[string]string{}
	for _, it := range drawnOf(items, "groupTitles") {
		titles[datumID(it)] = it.text
	}
	if len(groups) != len(want.groups) {
		t.Errorf("%d groups, expected %d", len(groups), len(want.groups))
	}
	for id, title := range want.groups {
		if titles[id] != title {
			t.Errorf("the group %s draws the title %q, expected %q", id, titles[id], title)
		}
		if parents[id] != want.parents[id] {
			t.Errorf("the group %s is in %q, expected %q", id, parents[id], want.parents[id])
		}
	}
	// A service and a group are inside their group, and two that share a
	// group do not overlap.
	all := maps.Clone(tiles)
	maps.Copy(all, groups)
	for id, b := range all {
		if p := parents[id]; p != "" && !b.inside(groups[p]) {
			t.Errorf("%s is not inside its group %s", id, p)
		}
		for other, o := range all {
			if id < other && parents[id] == parents[other] && b.overlaps(o) {
				t.Errorf("%s and %s overlap", id, other)
			}
		}
	}
	counts := []struct {
		name      string
		got, want int
	}{
		{"junctions", len(drawnOf(items, "junctions")), want.junctions},
		{"edges", len(drawnOf(items, "edges")), want.edges},
		{"arrows", len(drawnOf(items, "arrows")), want.arrows},
		{"labels of edges", len(drawnOf(items, "edgeLabels")), want.eLabels},
	}
	for _, c := range counts {
		if c.got != c.want {
			t.Errorf("%d %s, expected %d", c.got, c.name, c.want)
		}
	}
}

// archEdgeRE reads an edge of a fixture, apart from the compiler.
var archEdgeRE = regexp.MustCompile(`^(\w+)(?:\{group\})?:([LRTB])\s*\S+\s*([LRTB]):(\w+)`)

// checkSides checks that each edge of a source joins its services on the
// sides it names: for `a:R -- L:b`, b is to the right of a.
func checkSides(t *testing.T, items []drawnItem, src string) {
	t.Helper()
	center := map[string][2]float64{}
	for _, it := range drawnOf(items, "tiles") {
		center[datumID(it)] = [2]float64{num(it.datum["cx"]), num(it.datum["cy"])}
	}
	for _, it := range drawnOf(items, "junctions") {
		center[datumID(it)] = [2]float64{it.item.X, it.item.Y}
	}
	dir := map[string][2]float64{"L": {-1, 0}, "R": {1, 0}, "T": {0, -1}, "B": {0, 1}}
	for l := range strings.SplitSeq(src, "\n") {
		m := archEdgeRE.FindStringSubmatch(strings.TrimSpace(l))
		if m == nil {
			continue
		}
		a, b := center[m[1]], center[m[4]]
		da, db := dir[m[2]], dir[m[3]]
		want := [2]float64{da[0] - db[0], da[1] - db[1]}
		if m[2] == m[3] {
			// Two ends on the same side stack across the side.
			want = [2]float64{da[1] * da[1], da[0] * da[0]}
		}
		for k := range 2 {
			d := b[k] - a[k]
			if want[k] > 0 && d <= 0 || want[k] < 0 && d >= 0 {
				t.Errorf("the edge %q: %s is at %v and %s at %v", strings.TrimSpace(l), m[1], a, m[4], b)
			}
		}
	}
}

func TestArchitectureIcons(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("testdata/architecture/05-icons.mmd")
	if err != nil {
		t.Fatal(err)
	}
	items, _, err := drawDiagram(t, string(src), true)
	if err != nil {
		t.Fatal(err)
	}
	drawn := map[string]int{}
	for _, it := range items {
		if name, ok := it.datum["icon"].(string); ok && (it.mark == scene.Path || it.mark == scene.Image) {
			drawn[name]++
		}
	}
	for _, name := range []string{"k8s:namespace", "mdi:tray-full", "carbon:bare-metal-server", "fa:hard-drive"} {
		if drawn[name] != 1 {
			t.Errorf("the icon %s draws %d times", name, drawn[name])
		}
	}
	text := 0
	for _, it := range drawnOf(items, "iconTexts") {
		if it.text == "ERP" {
			text++
		}
	}
	if text != 1 {
		t.Errorf("the text of an icon draws %d times", text)
	}
}

func TestArchitectureErrors(t *testing.T) {
	t.Parallel()
	tests := []string{
		"architecture-beta\nservice a(server)[A]\nservice a(disk)[B]\n",
		"architecture-beta\nservice a(server)[A] in g\n",
		"architecture-beta\nservice a(server)[A]\nservice b(server)[B] in a\n",
		"architecture-beta\ngroup g(cloud)[G] in g\n",
		"architecture-beta\ngroup g(cloud)[G] in h\ngroup h(cloud)[H]\n",
		"architecture-beta\nservice a(server)[A]\na:R -- L:b\n",
		"architecture-beta\ngroup g(cloud)[G]\nservice a(server)[A]\na:R -- L:g\n",
		"architecture-beta\ngroup g(cloud)[G]\nservice a(server)[A] in g\nservice b(server)[B] in g\na{group}:R -- L:b\n",
		"architecture-beta\nservice a(server)[A]\nservice b(server)[B]\na:X -- L:b\n",
		"architecture-beta\nservice a(server)[A]\nservice b(server)[B]\na:R - L:b\n",
		"architecture-beta\nservice a(server)[A]\nalign row a\n",
		"architecture-beta\nservice a(server)[A]\nservice b(server)[B]\nalign row a a b\n",
		"architecture-beta\ngroup g(cloud)[G]\nservice a(server)[A]\nalign column a g\n",
		"architecture-beta\nservice a(server)[A\n",
		"architecture-beta\nnode a\n",
		"architecture-beta LR\nservice a(server)[A]\n",
	}
	for _, src := range tests {
		if _, err := chartOf(src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: the error %v", src, err)
		}
	}
}

func TestArchitectureIssues(t *testing.T) {
	t.Parallel()
	body := "architecture-beta\nservice a(server)[A]\nservice b(disk)[B]\na:R -- L:b\n"
	tests := []struct {
		src, issue string
	}{
		{"---\nconfig:\n  architecture:\n    numIter: 10\n---\n" + body, "architecture.numIter"},
		{"---\nconfig:\n  theme: sepia\n---\n" + body, "the theme of Mermaid"},
		{body + "a{group}:B -- T:b\n", "the {group} of a"},
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
	src := "---\nconfig:\n  architecture:\n    iconSize: 64\n    fontSize: 12\n    padding: 20\n    nodeSeparation: 50\n---\n" + body
	if report, err := chartOf(src, true); err != nil || len(report) != 0 {
		t.Errorf("%q: the report %v and the error %v", src, report, err)
	}
	// The layout uses no random numbers, so randomize is no issue.
	for _, v := range []string{"true", "false"} {
		src := "---\nconfig:\n  architecture:\n    randomize: " + v + "\n---\n" + body
		if report, err := chartOf(src, true); err != nil || len(report) != 0 {
			t.Errorf("%q: the report %v and the error %v", src, report, err)
		}
	}
	// An icon that no pack holds, as a logo of a vendor that zuzu does not
	// bundle, draws as a box with a question mark, and the report names it.
	src = "---\nconfig:\n  theme: dark\n---\narchitecture-beta\n" +
		"service fn(logos:aws-lambda)[Function]\nservice db(logos:aws-dynamodb)[Table]\nservice disk(disk)[Disk]\n" +
		"fn:R --> L:db\ndb:B --> T:disk\n"
	items, report, err := drawDiagram(t, src, false)
	if err != nil {
		t.Fatal(err)
	}
	logos := 0
	for _, is := range report {
		switch {
		case strings.Contains(is.Error(), `"logos:aws-`):
			logos++
		case !strings.Contains(is.Error(), "the theme of Mermaid"):
			t.Errorf("the issue %v", is)
		}
	}
	if logos != 2 || len(drawnOf(items, "tiles")) != 3 {
		t.Errorf("%d issues of logos and %d tiles, the report %v", logos, len(drawnOf(items, "tiles")), report)
	}
	if _, _, err := drawDiagram(t, strings.Replace(src, "  theme: dark\n", "  architecture:\n    padding: 10\n", 1), true); !errors.Is(err, icon.ErrMissing) {
		t.Errorf("a missing icon in the strict mode: the error %v", err)
	}
}
