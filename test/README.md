# Test module

This folder is a Go module of its own. It holds the corpus, the tools that
read it and the oracle, which a user of zuzu never imports (D5, D17).

| Path | What it holds |
| --- | --- |
| `testdata/corpus/` | the specs of the corpus and the data they read, with the license of each source beside its files |
| `testdata/corpus/inventory.tsv` | the features that each spec uses, which `cmd/inventory` writes |
| `testdata/references/` | the references of vega.js, which the oracle writes and git ignores |
| `testdata/targets/` | a Vega spec for each target of tiers 1 and 2, which draws the chart of the target from the data of the corpus |
| `targets/` | the test that runs those specs in the strict mode. Set `ZUZU_TARGETS_OUT` to a folder to write their SVG |
| `testdata/topojson/` | a small topology, and the meshes that the reader of vega.js gives, which `oracle/topojson.mjs` writes |
| `topojson/` | the test that compares the mesh of zuzu with those meshes |
| `raster/` | the test that compares the direct renderer with okage on the corpus, with the bundled fonts (D11). It needs the references |
| `cmd/rasterbench/` | the tool that times each step of the image of each spec, through SVG and directly (D11) |
| `cmd/inventory/` | the tool that writes the inventory |
| `cmd/targets/` | the tool that compares the SVG writer with the targets and writes `docs/targets/compare.html` (D13) |
| `cmd/wasm/` | a program that imports zuzu, so that CI can build it with TinyGo (D29) |
| `cmd/conform/` | the tool that compares zuzu with the references (D17) |
| `cmd/gallery/` | the tool that draws the specs of the corpus with zuzu, on a page |
| `cmd/compile/` | the tool that prints the Vega spec that zuzu compiles from a Vega-Lite spec |
| `conform/` | the comparison of datasets, scales and marks that `cmd/conform` runs |
| `oracle/` | the oracles, which run vega.js, vega-lite.js and d3 in Node.js |

## The corpus

The specs came from the testdata of `github.com/xo/vegagoja`: 580 specs of
the examples of Vega-Lite, 88 of Vega, and 11 of Deneb. Each folder holds the
license of its source. The data folder holds the 53 files that the specs
read, except three large files that only specs that bind events read.
`testdata/corpus/data/LICENSE-data.txt` says which.

## Write the inventory

Run this in this folder:

```bash
go run ./cmd/inventory
```

On 2026-11-10 it listed 679 specs. 540 of them are in the first target: they
bind no event and the corpus holds all of their data (D7).

## Write the references

The oracle needs Node.js. Run this in this folder:

```bash
(cd oracle && npm install)
TZ=UTC node oracle/oracle.mjs testdata
```

The time zone is UTC, so that the references are the same on every machine.
On 2026-11-10 it wrote 540 references in about 10 seconds, 174 MB in all, and
no spec failed in vega.js. Each reference holds what D17 compares: the
compiled Vega spec, the rows of each dataset, the type and the domain of each
scale, and the data of each item of each mark.

The oracle runs each spec twice, with two seeds of the random numbers. If the
two runs differ, the spec reads random numbers, through `random()` or through
a transform such as the bootstrap of a confidence interval or a word cloud,
and the reference is marked random (D28). On 2026-11-10, 13 specs were. For
such a spec, the check compares only the count of the rows and the items and
the type of each scale.

## Compare the writer with the targets

Draw the targets first, as `docs/targets/README.md` says. Then run this in
this folder:

```bash
go run ./cmd/targets
```

It writes the scene graph of each target and the SVG that zuzu writes from
it, prints the share of pixels that differ, and writes
`docs/targets/compare.html`, which shows each pair. Add
`-testdata ../svg/testdata/targets` to copy the scene graphs of tier 0 into
the golden tests of `svg`.

## Compare zuzu with the references

Write the references first, as above. Then run this in this folder:

```bash
go run ./cmd/conform
```

It runs each spec of the first target in zuzu, in the loose mode, and prints
the share of the specs, the datasets, the scales and the marks that match
the references. A Vega-Lite spec runs as the Vega spec that vega-lite.js
compiled, which its reference holds. Add `-v` to print what differs, and
`-only text` to check only the specs whose path holds the text. The plan
states the share that phase 3 must reach, and `TestPhase3Share` in
`conform` fails below it. That test skips when the references are not
there. The conformance job of CI writes them and runs it.

A group mark of a spec can define a dataset with the name of a dataset of
the top level, as the cells of a facet of vega-lite.js do. vega.js answers
that name with the dataset of the last group, so the reference holds the
data of one cell. The check leaves out those names, and does not count
them.

Add `-vl` to compile each Vega-Lite spec with zuzu rather than run the Vega
spec of vega-lite.js. It also prints the share of the specs whose compiled
spec has the structure of the one of vega-lite.js, which phase 4 measures.
`TestPhase4Share` fails below the share that the plan states for phase 4.
`go run ./cmd/compile spec.vl.json` prints the Vega spec that zuzu compiles.

## Look at what zuzu draws

Run this in this folder:

```bash
go run ./cmd/gallery -out /tmp/gallery
```

It draws each spec of the first target with zuzu, and writes the SVG files
and a page, `index.html`, that shows them with the issues of each. Add
`-dark` for the dark mode, `-only text` to draw some specs, and `-n` to draw
at most that many.

## Write the cases of the expressions and the formats

The tests of the packages `expr` and `format` compare zuzu with vega.js and
d3 on cases that two scripts write. Run these in this folder after
`npm install` in `oracle`:

```bash
TZ=America/New_York node oracle/exprs.mjs ../expr/testdata
```

```bash
TZ=America/New_York node oracle/formats.mjs ../format/testdata
```

The time zone is New York, so that the cases of local time cross a change
of the clock. `oracle/exprs.mjs` reads the expressions from
`expr/testdata/exprs.txt`.

## Check the diagrams and the icons

The test in `diagrams` checks the fixtures of DOT, and of the flowcharts,
the state diagrams, the class diagrams, the entity relationship diagrams
and the requirement diagrams of Mermaid, in `testdata/diagrams` against
their references
(D31). It asks that every node, edge, cluster and label of the reference
exists, with the cardinalities at the ends of a relation, that the hard rules of the layout hold, and that the
crossings and the span of the edges are no worse than those of the
reference, plus 10 percent and 1. The references come from `dot -Tjson0`
of Graphviz 16, and from `mmdc` of mermaid-cli 12.2.0, which bundles
Mermaid 12.3.0, through `cmd/mmdcref`. Run this in this folder to write
them again:

```bash
./oracle/diagrams.sh
```

If `mmdc` cannot find its Chrome, give it a config of puppeteer, a file
that holds `{"executablePath": "..."}`, in `PUPPETEER_CONFIG`.

The test in `icons` checks that the package `icon` reads each bundled icon
with the same view box and the same body as `@iconify/utils`, and a small
pack of turns and flips in `testdata/icons`. Run this in this folder after
`npm install` in `oracle` to write the digests again:

```bash
node oracle/icons.mjs
```

## Write the meshes of TopoJSON

The test in `topojson` compares the mesh of a topology with the rows that
the reader of vega.js gives, on the small topology in `testdata/topojson`
and on the maps of the corpus. Run this in this folder after `npm install`
in `oracle`:

```bash
node oracle/topojson.mjs testdata
```

## Time the images

`cmd/rasterbench` times each step of the image of each spec of the first
target: the scene, the SVG writer, the parse and the rasterization of okage,
and the direct renderer. It also counts the pixels that differ between the
two images of each spec, by the metric of okage D31. It draws the text with
the bundled fonts. Run it in this folder after the oracle:

```bash
go run ./cmd/rasterbench
```

Add `-only text` for some specs, `-n` for the runs of each step, and `-dump
dir` to write the two images of each spec and a map of the pixels that
differ.
