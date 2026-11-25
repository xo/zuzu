# rough

This folder is a copy of [rough-go](https://github.com/d2lang/rough-go) at
the commit `e7169d56ddc6`, under the MIT license in `LICENSE` (D39).
`NOTICE` names the upstream projects of rough-go: Rough.js 4.6.6,
hachure-fill 0.5.2 and path-data-parser 0.1.0. Both files are the files of
upstream, byte for byte. The rules for a copy are in
[`docs/UPSTREAM.md`](../../docs/UPSTREAM.md).

rough-go is a port of Rough.js 4.6.6 to Go, which is the version that
Mermaid 12.3 uses. It draws lines, curves, rectangles, ellipses, arcs,
polygons and the paths of SVG as if by hand, with the fills solid, hachure,
zigzag, cross-hatch, dots, dashed and zigzag-line. The look drawn by hand of
D39 and D40 draws with it.

## The files

Each copied file has the same name as in upstream: `core.go`, `filler.go`,
`filler_dashed.go`, `filler_dot.go`, `filler_hachure.go`, `filler_hatch.go`,
`filler_scan_line_hachure.go`, `filler_zigzag.go`, `filler_zigzag_line.go`,
`generator.go`, `geometry.go`, `math.go`, `number.go`, `path.go` and
`renderer.go`, with the tests `filler_patterns_test.go`,
`filler_scan_line_hachure_test.go`, `generator_test.go`, `number_test.go`,
`path_test.go` and `renderer_test.go`.

These files are files of zuzu, not of upstream:

1. `zuzu_doc.go` holds the comment of the package.
2. `zuzu_fillrule.go` holds the spans of the rule nonzero of change 3.
3. `zuzu_test.go` holds the tests of changes 2 and 3.
4. `zuzu_golden_test.go` and `testdata/golden.txt` hold the golden test of
   the copy. It draws each kind of shape with each fill style, with the
   default options and with the options that Mermaid gives for
   `look: handDrawn`, and with the seeds 1, 42 and 2147483647. It compares
   the path data with the golden file, byte for byte.

The golden file holds the bytes of amd64 below the level v3, which are the
bytes of upstream there. The test runs only on such a machine. On arm64,
ppc64le, s390x, riscv64 and loong64, and on amd64 at the level v3, the
compiler of Go fuses a multiply and an add into one instruction, and the
package `math` gives other bits for `Sin`, `Cos` and their kind. The path
data then differs in the last digits. zuzu promises the same bytes on one
architecture, not across machines (D41). To write the file again, run
`go test ./internal/rough -run TestGoldenPaths -update` on amd64 below the
level v3.

The copy leaves out these files of upstream:

1. `differential_test.go` and `testdata/differential/oracle.js`. They run
   Rough.js in Node.js as an oracle.
2. The files of the project: `go.mod`, `README.md`, `PROVENANCE.md`,
   `.gitignore` and `.github`.

The copy imports only the standard library. It builds with TinyGo for
`wasip1`.

## The changes

Changes 2 and 3 are marked in the code with a comment that starts with
`zuzu:`, which names the number of the change.

1. Each copied file opens with a comment that names the project, the file
   and the commit, and that starts with `Copied from`.
2. The generator never draws from `math/rand`. Rough.js draws from
   `Math.random` when the seed is 0 or NaN, and from the second draw on when
   the conversion ToInt32 of JavaScript gives 0 for the seed, as for 0.5,
   2^32 or Infinity. In `math.go`, such a seed takes the seed 1, the seed of
   a chart in D40. Every other seed gives the draws of Rough.js. The state
   of the generator never becomes 0 after that, because the multiplier 48271
   is odd. This also holds for the second stroke of a seed of -1, whose seed
   plus 1 is 0. The function `NewSeed`, which returns a random seed, is left
   out, and so is the import of `math/rand`. The test
   `TestRandomDefersJavaScriptInt32CoercionUntilFirstDraw` of
   `renderer_test.go` expects the seed 1 for such a seed. The look of D39
   gives each item a seed from 1 to 2^31-1, so it never meets this case.
3. The hachure of a polygon follows the rule nonzero of SVG when the option
   `FillRule` is `nonzero`. Rough.js fills a polygon with the rule evenodd.
   The look drawn by hand joins the subpaths of a path into one polygon
   with bridges, and SVG fills a path with the rule nonzero. With the rule
   evenodd, two subpaths that overlap and run the same way leave their
   overlap empty. With the option, each edge keeps its direction, and a
   line of the hachure runs where the winding number is not 0. A bridge
   goes out and comes back along the same line, so it does not change the
   winding number. `core.go` adds the option, and
   `filler_scan_line_hachure.go` keeps the direction of each edge and
   calls `nonzeroLines` of `zuzu_fillrule.go`. Without the option, the
   hachure is the hachure of upstream, byte for byte. This change stays in
   zuzu, because Rough.js has no rule of fill for a hachure.
