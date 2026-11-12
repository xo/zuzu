package mermaid

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"testing"

	"github.com/xo/zuzu/graph"
)

// gitLayoutOf lays out a git graph from a source.
func gitLayoutOf(t *testing.T, src string) *gitLayout {
	t.Helper()
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	g, err := readGitGraph(&chart{d: d, issues: &graph.Issues{Strict: true}})
	if err != nil {
		t.Fatal(err)
	}
	return newGitLayout(g)
}

// gitNumbers are the numbers of a path, with the flag of the sweep of
// each arc and without its radii, so that a path of zuzu and a path of
// Mermaid compare.
var (
	gitArcRE = regexp.MustCompile(`A\s*[\d.]+[ ,]+[\d.]+[ ,]+0[ ,]+0[ ,]+([01])[ ,]+`)
	gitNumRE = regexp.MustCompile(`-?[\d.]+`)
)

func gitNumbers(path string) []float64 {
	var out []float64
	for _, s := range gitNumRE.FindAllString(gitArcRE.ReplaceAllString(path, " $1 "), -1) {
		f, _ := strconv.ParseFloat(s, 64)
		out = append(out, f)
	}
	return out
}

// TestGitGraphLayout checks the place of each commit and the path of each
// link against the SVG of mmdc for the same source.
func TestGitGraphLayout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		// pos are the centers of the commits in the order of the source,
		// and links the paths of the links in the order that Mermaid draws
		// them.
		pos   [][2]float64
		links []string
		// along reports that only the place along the graph compares,
		// because the space between the lanes of a vertical graph
		// depends on the width of the names of the branches.
		along bool
	}{
		{
			name: "07-parallel.mmd",
			pos: [][2]float64{
				{10, -2}, {60, -2}, {110, 88}, {160, 88}, {110, 178}, {160, 178}, {110, -2}, {210, 88}, {260, -2},
			},
			links: []string{
				"M 10 -2 L 60 -2",
				"M 60 -2 L 60 68 A 20 20, 0, 0, 0, 80 88 L 110 88",
				"M 110 88 L 160 88",
				"M 60 -2 L 60 158 A 20 20, 0, 0, 0, 80 178 L 110 178",
				"M 110 178 L 160 178",
				"M 60 -2 L 110 -2",
				"M 160 88 L 210 88",
				"M 160 178 L 190 178 A 20 20, 0, 0, 0, 210 158 L 210 88",
				"M 110 -2 L 260 -2",
				"M 210 88 L 240 88 A 20 20, 0, 0, 0, 260 68 L 260 -2",
			},
		},
		{
			name: "10-reroute.mmd",
			pos: [][2]float64{
				{10, -2}, {60, 88}, {110, -2}, {160, 88}, {210, -2}, {260, 178}, {310, 88}, {360, -2}, {410, -2},
				{460, 178}, {510, -2}, {560, 178}, {610, 178}, {660, 88},
			},
			links: []string{
				"M 10 -2 L 10 68 A 20 20, 0, 0, 0, 30 88 L 60 88",
				"M 10 -2 L 110 -2",
				"M 60 88 L 160 88",
				"M 110 -2 L 210 -2",
				"M 210 -2 L 210 158 A 20 20, 0, 0, 0, 230 178 L 260 178",
				"M 160 88 L 310 88",
				"M 260 178 L 290 178 A 20 20, 0, 0, 0, 310 158 L 310 88",
				"M 210 -2 L 360 -2",
				"M 360 -2 L 410 -2",
				"M 310 88 L 390 88 A 20 20, 0, 0, 0, 410 68 L 410 -2",
				"M 260 178 L 460 178",
				"M 410 -2 L 510 -2",
				"M 460 178 L 560 178",
				"M 560 178 L 610 178",
				"M 510 -2 L 510 60 A 10 10, 0, 0, 0, 520 70 L 600 70 A 10 10, 0, 0, 1, 610 80 L 610 178",
				"M 310 88 L 660 88",
				"M 610 178 L 640 178 A 20 20, 0, 0, 0, 660 158 L 660 88",
			},
		},
		{
			name:  "08-parallel-bt.mmd",
			pos:   [][2]float64{{0, 220}, {0, 170}, {0, 120}, {0, 70}, {0, 120}, {0, 70}, {0, 120}, {0, 20}},
			along: true,
		},
		{
			name:  "05-tb.mmd",
			pos:   [][2]float64{{0, 40}, {0, 90}, {0, 140}, {0, 190}, {0, 240}, {0, 290}, {0, 340}, {0, 390}},
			along: true,
		},
		{
			name:  "06-bt.mmd",
			pos:   [][2]float64{{0, 290}, {0, 240}, {0, 190}, {0, 140}, {0, 90}, {0, 40}},
			along: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(filepath.Join("testdata", "gitgraph", tt.name))
			if err != nil {
				t.Fatal(err)
			}
			l := gitLayoutOf(t, string(src))
			if len(l.g.ids) != len(tt.pos) {
				t.Fatalf("expected %d commits, got %d", len(tt.pos), len(l.g.ids))
			}
			for i, id := range l.g.ids {
				p, want := l.pos[id], tt.pos[i]
				if p.y != want[1] || !tt.along && p.x != want[0] {
					t.Errorf("commit %s: expected %v, got %v", id, want, p)
				}
			}
			if tt.links == nil {
				return
			}
			rows := l.rows["links"]
			if len(rows) != len(tt.links) {
				t.Fatalf("expected %d links, got %d", len(tt.links), len(rows))
			}
			for i, r := range rows {
				m, _ := r.(map[string]any)
				got, _ := m["path"].(string)
				if !slices.Equal(gitNumbers(got), gitNumbers(tt.links[i])) {
					t.Errorf("link %d: expected %s, got %s", i, tt.links[i], got)
				}
			}
		})
	}
}

// TestGitGraphLanes checks the order of the lanes and the space between
// them.
func TestGitGraphLanes(t *testing.T) {
	t.Parallel()
	l := gitLayoutOf(t, "gitGraph\n  commit\n  branch b order: 2\n  branch c order: 1\n  branch d\n  commit\n")
	want := map[string]float64{"main": 0, "d": 90, "c": 180, "b": 270}
	for name, p := range want {
		if l.lane[name] != p {
			t.Errorf("lane %s: expected %v, got %v", name, p, l.lane[name])
		}
	}
	// Without labels that turn, the lanes are 50 apart.
	l = gitLayoutOf(t, "%%{init: {'gitGraph': {'rotateCommitLabel': false}}}%%\ngitGraph\n  commit\n  branch b\n  commit\n")
	if l.lane["b"] != 50 {
		t.Errorf("expected the lane b at 50, got %v", l.lane["b"])
	}
}
