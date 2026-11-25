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
| `cmd/schemes/` | the tool that writes the color schemes from the styles of transit, with their licenses and `docs/SCHEMES.md`, as "Schemes from upstream themes" in `docs/STYLE.md` says (D58) |
| `cmd/targets/` | the tool that compares the SVG writer with the targets and writes `docs/targets/compare.html` (D13) |
| `cmd/wasm/` | a program that imports zuzu, so that CI can build it with TinyGo (D29) |
| `cmd/conform/` | the tool that compares zuzu with the references (D17) |
| `cmd/gallery/` | the tool that draws the specs of the corpus with zuzu, on a page |
| `cmd/review/` | the tool that writes a page that shows each source drawn by its original renderer next to the same source drawn by zuzu |
| `cmd/compile/` | the tool that prints the Vega spec that zuzu compiles from a Vega-Lite spec |
| `cmd/survey/` | the tool that draws the zuzu cards of `docs/survey/compare.html` |
| `cmd/mmdcref/` | the tool that reads an SVG that `mmdc` drew and writes the reference of a fixture of Mermaid (D31) |
| `conform/` | the comparison of datasets, scales and marks that `cmd/conform` runs |
| `testdata/diagrams/` | the fixtures of DOT and of Mermaid, each with its reference |
| `diagrams/` | the test that checks the fixtures against their references (D31) |
| `testdata/icons/` | the digests of the bundled icons, and a small pack of turns and flips |
| `icons/` | the test that checks the package `icon` against `@iconify/utils` (D31) |
| `oracle/` | the oracles, which run vega.js, vega-lite.js, d3, mermaid.js and `@iconify/utils` in Node.js, the script that draws the references of the diagrams, `svg.mjs`, which draws the SVG of vega.js for `cmd/review`, and `mermaid-svg.mjs`, which draws diagrams of Mermaid in a chosen layout in Chrome (D44) |

## The corpus

The specs came from the testdata of `github.com/xo/vegagoja`: 579 specs of
the examples of Vega-Lite, 87 of Vega, and 13 of Deneb. Each folder holds the
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

## Write the color schemes

The command reads the styles of the version of
`github.com/xo/transit/styles` that `go.mod` requires. Run this in this
folder:

```bash
go run ./cmd/schemes
```

It writes `theme/schemes.go`, `theme/licenses/`, `docs/SCHEMES.md` and
`docs/targets/schemes.json`, and prints the styles that it leaves out. On
2026-11-25, with v0.7.8, it wrote 176 schemes, 30 of them families, and
left out 9 styles. `TestTheGeneratedFilesAreInStep` fails when the files in
the repository differ from those that the command writes. The command
imports the package `theme`, so `theme/schemes.go` must exist before it
runs. To start from nothing, write a `theme/schemes.go` that holds
`var upstream = [...]Scheme{}`.

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
at most that many. Add `-look excalidraw` to draw each spec in the look like
Excalidraw, with Excalifont (D40).

`go run ./cmd/survey` draws each spec in `docs/survey/zuzu` in the light and
the dark mode. It writes the zuzu images of `docs/survey/compare.html` to
`docs/survey/static`. `docs/survey/README.md` says more.

## Compare zuzu with the originals

`cmd/review` draws each source with its original renderer and with zuzu,
and writes a page that shows the two side by side. The sources are these:

1. The Vega, Vega-Lite and Deneb specs of the corpus that the inventory
   puts in the first target. The original is vega.js, through
   `oracle/svg.mjs`. vega-lite.js compiles a Vega-Lite spec for the
   original, and zuzu compiles it for the zuzu side.
2. The fixtures of DOT in `testdata/diagrams/dot`. The original is
   `dot -Tsvg` of Graphviz.
3. The fixtures of Mermaid in `testdata/diagrams/mermaid` and in
   `graph/mermaid/testdata`. The original is `mmdc` of mermaid-cli.

The tool needs Node.js with `npm install` done in `oracle`, and `dot` and
`mmdc` on the `PATH`. If one of them is missing, the tool draws the other
sources, and each row without an original says why. `mmdc` asks for one
exact version of chrome-headless-shell, which an update of mermaid-cli can
change. So the tool gives `mmdc` the newest chrome-headless-shell in the
cache of puppeteer, `~/.cache/puppeteer` or `PUPPETEER_CACHE_DIR`, through
a configuration of puppeteer that it writes in the folder of `-out`. A
configuration in `PUPPETEER_CONFIG` wins over it, as for the references of
the diagrams. If the cache holds no shell, install one:
`npx puppeteer browsers install chrome-headless-shell`.

Run this in this folder:

```bash
go run ./cmd/review
```

It writes `index.html` and the SVG files to `docs/review` in the root of
the repository, which git ignores. On 2026-11-12 it drew 863 sources in
about 17 seconds. It keeps the SVG of each original, and draws an
original again only when its source is newer. A second run takes about 2
seconds. The flags are these:

| Flag | What it does |
| --- | --- |
| `-out dir` | writes the page to this folder, `../docs/review` by default |
| `-only text` | draws only the sources whose name holds the text |
| `-front list` | draws only these front ends, a comma list of `vega`, `vegalite`, `deneb`, `dot` and `mermaid` |
| `-force` | draws each original again |
| `-j n` | draws this many sources at the same time, half of the processors by default |
| `-look name` | draws each source of zuzu in this look, such as `excalidraw`, rather than in the look of the source. Without it, a Mermaid diagram with `look: handDrawn` draws in the look like Excalidraw, with Excalifont (D40) |
| `-handdrawn=false` | leaves out the row drawn by hand. By default, each row also shows the source drawn by hand, below a dashed line: the original of a Mermaid diagram drawn by `mmdc` with `look: handDrawn` and `handDrawnSeed: 1`, and the source drawn by zuzu in each look drawn by hand, in the light and the dark mode (D40) |
| `-layouts=false` | leaves out the row of layouts. By default, each diagram also shows its layouts, below a dashed line: the original, the original of a Mermaid diagram drawn by `mmdc` with `layout: dagre`, and zuzu with `zuzu.layered`, `zuzu.dagre` and `zuzu.elk`, each forced with `zuzu.WithLayout`. Each layout of a Mermaid diagram names its error of the places of the nodes against the original of its layout: `zuzu.dagre` against dagre, and the others against the default original, which is ELK in Mermaid 12.3 (D44, D45). The tool reads the places of an original with `cmd/mmdcref`, which adds the offset of each nested `<g class="root">`, the group that Mermaid draws for a cluster that dagre lays out on its own |

The page has a tab for each front end, with the count of the sources, of
the errors on each side and of the sources with report lines. Each row
shows the original on the left and zuzu on the right, with the errors and
the report lines under the drawing. Open `index.html` in a browser, or
serve the folder from this folder:

```bash
python3 -m http.server 8000 --bind 127.0.0.1 --directory ../docs/review
```

Then open `http://127.0.0.1:8000/`.

## Draw the layouts of Mermaid

`oracle/mermaid-svg.mjs` is the oracle of the layouts of D44. It draws
diagrams of Mermaid with mermaid.js in headless Chrome, and writes the SVG
of each. It draws as `mmdc` draws: on the same page, with the same style
sheets of fonts, the same mermaid.js 12.3.0 and the same config. It draws
many diagrams with one Chrome. On 2026-11-22, it drew the 95 fixtures in
`testdata/diagrams/mermaid` in about 9 seconds. Each SVG had the same view
box, the same places of the nodes and the same paths of the edges as the
SVG of `mmdc`.

mermaid.js 12.3.0 bundles the layouts of ELK, and its default layout is
`elk`. Thus `mmdc` draws a flowchart, a state diagram, a class diagram, an
entity relationship diagram, a requirement diagram and an agentflow diagram
in ELK when the source sets no layout. The references of these diagrams in
`testdata/diagrams` hold the places of ELK. `mmdc` draws in dagre only when
the source or the config sets `layout: dagre`.

The script sets `layout: elk` in its config. A source that sets its own
layout, in its front matter or in a directive, keeps it. A job can merge
more config over the config of the script, as the flag `-c` of `mmdc`
does. The layouts are `dagre` and `elk`, and the variants of ELK
`elk.stress`, `elk.force`, `elk.mrtree`, `elk.sporeOverlap`, `elk.box` and
`elk.rectpacking`.

The script needs Node.js and Chrome. Run `npm install` in `oracle` first.
It installs mermaid.js, puppeteer-core, KaTeX and the fonts that `mmdc`
loads, at the versions of mermaid-cli 12.2.0. puppeteer-core downloads no
browser. The script starts the stable Chrome of the system. To start
another Chrome, give a config of puppeteer in `PUPPETEER_CONFIG`, the same
file that `mmdc` reads with `-p`.

The script reads one job per line on its standard input, as JSON. A job
names the source in `in` and the SVG to write in `out`, and can give more
config in `config`. A path is relative to the current folder. Run this in
this folder to draw a fixture in ELK and in dagre:

```bash
cat >/tmp/jobs <<'EOF'
{"in": "testdata/diagrams/mermaid/10-subgraph.mmd", "out": "/tmp/layouts/10-subgraph.elk.svg"}
{"in": "testdata/diagrams/mermaid/10-subgraph.mmd", "out": "/tmp/layouts/10-subgraph.dagre.svg", "config": {"layout": "dagre"}}
EOF
node oracle/mermaid-svg.mjs </tmp/jobs
```

If mermaid.js cannot draw a source, the script writes the message to the
file of `out` with the extension `.err`, and no SVG. Add `-j n` to draw n
sources at the same time. The default is half of the processors.

The script differs from `mmdc` in three ways:

1. It does not register ZenUML and the layout `tidy-tree`, which `mmdc`
   registers. It cannot draw a ZenUML diagram, and it draws a mindmap with
   `layout: tidy-tree` in `cose-bilkent`, the default layout of the
   mindmap.
2. It adds the fonts that an SVG uses to the SVG, as `mmdc` does, but
   without the text of their licenses.
3. mermaid.js 12.3.0 fails on a mindmap with `layout: elk`, and on some
   diagrams with `layout: elk.mrtree`. The script then writes the error.
   `mmdc` fails the same way with that config.

## Write the cases of the expressions, the formats, the schemes and the domains

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

The test `TestFuncsOracle` in the package `zuzu` compares the functions of
expressions that read the view, such as `gradient`, `copy`, `geoShape`,
`treePath` and `vlSelectionTest`, with vega.js. `oracle/funcs-cases.mjs`
writes the cases in `testdata/funcs/cases.json` in the root, with random
shapes from a fixed seed. `oracle/funcs.mjs` reads them, and writes what
vega.js returns in `testdata/funcs/oracle.json`. Run these in this folder:

```bash
node oracle/funcs-cases.mjs ../testdata/funcs/cases.json
TZ=UTC node oracle/funcs.mjs ../testdata/funcs
```

The tests of the named schemes in the package `zuzu` compare each scheme of
vega.js, and some scales with a scheme, with zuzu (D46). Run this in this
folder to write their cases in `testdata/schemes.json`:

```bash
node oracle/schemes.mjs ../testdata
```

A test of the package `zuzu` compares the domains that vega.js sorts from
values of mixed types, such as strings and numbers, with zuzu. JavaScript
compares a string and a number as two numbers, so the order depends on the
sort of V8, which zuzu does in the same way. Run this in this folder to
write the cases in `testdata/domains.json`:

```bash
node oracle/domains.mjs ../testdata
```

## Check the diagrams and the icons

The test in `diagrams` checks the fixtures of DOT, and of the flowcharts,
the state diagrams, the class diagrams, the entity relationship diagrams,
the requirement diagrams and the agentflow diagrams of Mermaid, in
`testdata/diagrams` against their references
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

`TestPositionError` in `diagrams` measures how far the nodes of zuzu lie
from the nodes of these references, for the flowcharts, the state
diagrams, the class diagrams and the entity relationship diagrams (D40
part 11, D44 part 4). It prints a column for `zuzu.layered`, `zuzu.dagre`
and `zuzu.elk`, and never fails. The test module requires the module of
ELK in `../elk` for it, and for the review page. Run it with `-v` to see
the numbers. The references hold the places of ELK. To compare with dagre
too, set `ZUZU_DAGRE_REFS` to a folder of references that `cmd/mmdcref`
wrote from `mmdc` with the config `{"layout": "dagre"}`:

```bash
ZUZU_DAGRE_REFS=/tmp/dagre-refs go test -count=1 -run 'TestPositionError$' -v ./diagrams
```

The test in `icons` checks that the package `icon` reads each bundled icon
with the same view box and the same body as `@iconify/utils`, and a small
pack of turns and flips in `testdata/icons`. Run this in this folder after
`npm install` in `oracle` to write the digests again:

```bash
node oracle/icons.mjs
```

The tests of the package `graph/mermaid` ask zuzu for the same start and
end of each task of a gantt as mermaid.js. `oracle/gantt.mjs` writes them
with the mermaid.js that mermaid-cli bundles, with the clock fixed at
2024-03-15T10:20:30.400Z. Run this in the folder `oracle`:

```bash
TZ=UTC node gantt.mjs ../../graph/mermaid/testdata/gantt > ../../graph/mermaid/testdata/gantt/oracle.json
```

If mermaid-cli is not in `~/.local/lib/node_modules`, set `MERMAID` to the
folder of its package `mermaid`.

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
