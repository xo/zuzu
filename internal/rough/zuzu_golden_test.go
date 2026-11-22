//go:build amd64 && !amd64.v3

package rough

// This file is a file of zuzu, not of rough-go. It holds the golden test of
// the copy: the path data of a fixed set of shapes, with each fill style and
// a few seeds, must be the same bytes as testdata/golden.txt. The file holds
// the bytes of amd64 below the level v3, so the test runs only there. On
// another machine the package math of Go gives other bits, and the compiler
// fuses a multiply and an add into one instruction, so the bytes differ. zuzu
// promises the same bytes on one architecture, not across machines (D41).

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "write the golden files again")

// goldenFills are the fill styles of rough.js.
var goldenFills = []string{"solid", "hachure", "zigzag", "cross-hatch", "dots", "dashed", "zigzag-line"}

// goldenSeeds are the seeds of the test: the smallest, a small one, and the
// largest seed that rough.js keeps deterministic.
var goldenSeeds = []float64{1, 42, 2147483647}

// goldenShape draws one shape with the generator g and the options o.
type goldenShape struct {
	name   string
	filled bool
	draw   func(g *Generator, o *Options) Drawable
}

var goldenShapes = []goldenShape{{
	name: "line",
	draw: func(g *Generator, o *Options) Drawable { return g.Line(3, 7, 251, 93, o) },
}, {
	name: "linearPath",
	draw: func(g *Generator, o *Options) Drawable {
		return g.LinearPath([]Point{{0, 0}, {40, 25}, {75, 5}, {120, 60}}, o)
	},
}, {
	name: "curve",
	draw: func(g *Generator, o *Options) Drawable {
		return g.Curve([]Point{{10, 10}, {40, 60}, {80, 20}, {120, 70}, {160, 30}}, o)
	},
}, {
	name:   "rectangle",
	filled: true,
	draw:   func(g *Generator, o *Options) Drawable { return g.Rectangle(5, 5, 36, 24, o) },
}, {
	name:   "ellipse",
	filled: true,
	draw:   func(g *Generator, o *Options) Drawable { return g.Ellipse(30, 20, 50, 34, o) },
}, {
	name:   "polygon",
	filled: true,
	draw: func(g *Generator, o *Options) Drawable {
		return g.Polygon([]Point{{0, 8}, {22, 0}, {40, 13}, {30, 32}, {6, 28}}, o)
	},
}, {
	name:   "arc",
	filled: true,
	draw: func(g *Generator, o *Options) Drawable {
		return g.Arc(30, 30, 56, 44, -0.4, 3.9, true, o)
	},
}, {
	name:   "path",
	filled: true,
	draw: func(g *Generator, o *Options) Drawable {
		return g.Path("M5 20 L30 4 Q45 0 52 14 C58 26 50 40 36 42 A12 9 30 0 1 12 38 Z", o)
	},
}}

// goldenProfile is a set of options that the test draws each case with.
type goldenProfile struct {
	name  string
	fills []string
	set   func(o *Options)
}

var goldenProfiles = []goldenProfile{{
	name:  "default",
	fills: goldenFills,
	set:   func(*Options) {},
}, {
	// The options of rough.js that Mermaid 12.3 gives for look: handDrawn
	// (D40 part 5).
	// Its gap of 1.5 draws many lines, so it takes only the fill styles that
	// Mermaid uses, to keep the golden file small.
	name:  "handDrawn",
	fills: []string{"solid", "hachure"},
	set: func(o *Options) {
		o.Roughness = Float64(0.7)
		o.FillWeight = Float64(1.5)
		o.HachureGap = Float64(1.5)
		o.StrokeWidth = Float64(1.3)
	},
}}

// goldenOutput draws every case and writes its path data, one line for each
// path, with the numbers as rough.js writes them. A shape without a fill
// takes each seed. A fill takes one seed, in turn, so that the file stays
// small and each seed still meets each kind of fill.
func goldenOutput() string {
	var b strings.Builder
	for _, p := range goldenProfiles {
		for j, s := range goldenShapes {
			fills := []string{""}
			if s.filled {
				fills = p.fills
			}
			for i, fill := range fills {
				seeds := goldenSeeds
				if fill != "" {
					k := (i + j) % len(goldenSeeds)
					seeds = goldenSeeds[k : k+1]
				}
				for _, seed := range seeds {
					o := &Options{Seed: Float64(seed)}
					if fill != "" {
						o.Fill = String("#4c78a8")
						o.FillStyle = String(fill)
					}
					p.set(o)
					g := NewGenerator(nil, &Surface{Width: 40, Height: 30})
					b.WriteString("# " + p.name + " " + s.name + " " + fill + " " + strconv.FormatFloat(seed, 'f', -1, 64) + "\n")
					for _, path := range g.ToPaths(s.draw(g, o)) {
						writeGoldenPath(&b, path)
					}
				}
			}
		}
	}
	return b.String()
}

// writeGoldenPath writes one path, and the path of its pattern if it has one.
func writeGoldenPath(b *strings.Builder, path PathInfo) {
	b.WriteString(path.Stroke + " " + path.Fill + " " + NumberString(path.StrokeWidth) + " " + path.D + "\n")
	if path.Pattern != nil {
		b.WriteString("pattern " + path.Pattern.ViewBox + "\n")
		writeGoldenPath(b, path.Pattern.Path)
	}
}

// TestGoldenPaths compares the path data with testdata/golden.txt. Run it
// with -update on amd64 below the level v3 to write the file again.
func TestGoldenPaths(t *testing.T) {
	t.Parallel()
	got := goldenOutput()
	name := filepath.Join("testdata", "golden.txt")
	if *update {
		if err := os.WriteFile(name, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if got == string(want) {
		return
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	var head string
	for i := 0; i < len(gotLines) && i < len(wantLines); i++ {
		if strings.HasPrefix(wantLines[i], "# ") {
			head = wantLines[i]
		}
		if gotLines[i] != wantLines[i] {
			t.Fatalf("line %d, in the case %q, differs from %s:\ngot:  %.300s\nwant: %.300s",
				i+1, head, name, gotLines[i], wantLines[i])
		}
	}
	t.Fatalf("the output has %d lines, and %s has %d", len(gotLines), name, len(wantLines))
}
