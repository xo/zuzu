# zuzu Plan

This document holds the plan for `github.com/xo/zuzu`. The decisions are in
[`decisions/`](decisions/README.md), one file each (D4). The open questions
for the maintainers are at the end of this file.

The plan and the rules came before any code (D3), so this file is the place
to change the design. A change to the plan is a decision first.

## Purpose

zuzu is a pure Go package that draws charts and diagrams (D1). A caller
gives it a declarative spec. A spec is a JSON document that says what to
draw from which data, and not how to draw it. zuzu reads the spec, computes
the data, the scales and the layout, and writes the result.

The spec follows Vega and Vega-Lite (D6). Vega is a declarative grammar for
charts, and Vega-Lite is a shorter grammar that compiles to Vega. Both are
JSON, and both run in JavaScript today. zuzu reads the same JSON in Go, so a
spec written for the Vega tools works with zuzu, as far as zuzu supports it.

These are the requirements of the maintainers, as given on 2026-11-10:

1. The package is pure Go, and so are its outside packages where a choice
   exists. There is no other limit on imports (D5).
2. The first output is SVG. okage, the pure Go port of resvg, rasterizes the
   SVG (D10). okage is at `github.com/xo/okage`, and its package `svg` takes
   SVG data.
3. A second renderer draws with the primitives of okage directly, if that is
   faster than writing SVG and parsing it again (D11).
4. Styles are simple to apply, with a light and a dark theme from the start,
   and color schemes that swap apart from the mode (D14).
5. zuzu draws its own look, at the quality of nivo and visx, and does not
   reproduce the look of Vega (D12). [`STYLE.md`](STYLE.md) holds the rules,
   and the targets in [`targets/`](targets/README.md) show them (D13).
6. A spec can use an expression language, as Vega does, and a script beyond
   it through the same interface (D9 and [`EXPRESSIONS.md`](EXPRESSIONS.md)).
7. Later, live data goes to a web browser, where an HTML and JavaScript
   component updates the chart. So a canvas renderer must be possible (D19).
8. Later, the DOT language of Graphviz and the syntax of Mermaid compile
   into the same representation (D18).
9. Much later, `usql` makes a chart from the result of a query (D20).

## What came before

The `xo` namespace has tried this before, in five ways. Each one taught
something that this plan uses.

| Repository | What it did | What it taught |
| --- | --- | --- |
| `vegagoja` | ran vega.js and vega-lite.js in goja, a JavaScript engine in pure Go | the specs and the reference SVGs it made are the corpus for D17. It also shows the cost of a JavaScript engine |
| `echartsgoja` | ran ECharts in goja | the same, for the options of ECharts |
| `chartfmt` | made charts for `usql` from a result set, through the two above and `xo/resvg` | the shape of what `usql` asks for. `usql` measured the stack at 12 MB (usql backlog W13) |
| `blahvega` | generated Go types from the JSON schema of Vega-Lite with quicktype | generated types are too many and too loose. zuzu writes its types by hand |
| `graphvizzero` | ran Graphviz compiled to WebAssembly in wazero | a WebAssembly runtime is not pure Go in the sense of D5 |

`xo/resvg` wrapped resvg through cgo. okage replaced it in `usql` on
2026-11-10.

## The survey

[`survey/libraries.html`](survey/libraries.html) is a survey of the 20 most
popular chart libraries and four diagram tools, with their stars, their
downloads and the number of chart types of each, as measured on 2026-11-09.
It holds the answers of three other models to one question: does the look of
a chart decide which library people choose, or do its features? All three
said the features, the license and the fit with the rest of a project decide
it, and that a good look is a minimum that every winner meets.

[`survey/compare.html`](survey/compare.html) draws the same real data with
13 libraries, side by side, in a light and a dark theme. Open it to see what
each library draws without help, what needs a plugin, and what the caller must
prepare first. [`survey/README.md`](survey/README.md) says how to open it.

## The targets

[`targets/index.html`](targets/index.html), which the script in `targets/`
writes, shows the look that zuzu aims for, in 33 charts from tier 0 to tier 3,
each in the light and the dark mode, and four of them in each of the six color
schemes. The targets came before any code (D13), from the same real data as
the survey, and they render in okage with no error. [`STYLE.md`](STYLE.md)
holds the rules they follow. They can change, and once code exists, a change
to them is a decision.

## Architecture

zuzu has four stages. Each stage hands the next one a value that can be
written as JSON, so that each boundary can be tested alone and sent over a
network.

```text
 Vega-Lite ──compile──┐
 Vega ────────────────┼──► spec ──run──► scene graph ──render──► SVG ──okage svg──► image
 DOT, Mermaid ─parse──┘   (Vega     (data, scales,     │
 usql result ─infer───┘    shape)    layout)           ├──draw──► okage Pixmap
                                                       └──send──► browser canvas
```

1. A front end reads an input language and produces a spec. The spec is a
   Go value in the shape of a Vega spec: data, transforms, scales,
   projections, axes, legends, marks and signals. A Vega spec is read as it
   is. A Vega-Lite spec is compiled to it, as the JavaScript compiler of
   Vega-Lite does. DOT and Mermaid come later (D18).
2. The runtime runs the spec. It loads the data, runs the transforms,
   evaluates the expressions, builds the scales, measures the text and
   lays out the axes, the legends and the groups.
3. The result is a scene graph (D8). A scene graph is a tree of groups and
   marks, where each mark holds items with their final positions, sizes and
   colors. zuzu uses the shape of the scene graph of Vega, which Vega writes
   as JSON with `sceneToJSON`. So the marks and the data bound to them can be
   compared with vega.js, while the look and the layout belong to zuzu (D12,
   D17).
4. A renderer writes the scene graph. The SVG writer comes first (D10). The
   direct okage renderer and the browser renderer read the same scene graph
   (D11, D19).

The spec and the scene graph are the two intermediate representations. The
spec says what to draw, and the scene graph says where each shape is.

### Packages

The names below follow D21. A package is named with one
short word, and an exported name does not repeat its package.

| Package | What it holds |
| --- | --- |
| `zuzu` | the API a caller uses: render a spec to SVG or to an image, with functional options for the theme, the size, the fonts and the logger |
| `vega` | the Go types of a Vega spec, read from and written to JSON |
| `vegalite` | the Go types of a Vega-Lite spec and the compiler to `vega` |
| `expr` | the expression language: the parser, the tree, the evaluator and the functions (D9) |
| `script` | the evaluator that runs scripts in goja, which a program imports only if it wants it (D9) |
| `format` | the formats of numbers and times, with the syntax of d3-format and d3-time-format, the tables of each locale (D27), the units of time and the dates that JavaScript reads |
| `dataflow` | the data sources and the transforms |
| `scale` | the scales, and the interpolation of a color ramp |
| `internal/color` | the colors of CSS and their spaces: RGB, HSL, CIELAB and HCL, which `scale` and `expr` share |
| `geo` | the projections and the geographic paths |
| `layout` | the axes, the legends, the titles, the size of a view and the grid of a facet |
| `scene` | the scene graph and its JSON form (D8) |
| `text` | the measurement of text, behind an interface (D11) |
| `theme` | the modes and the color schemes, and the Vega config they make (D14, D15) |
| `svg` | the SVG writer |
| `raster` | the direct okage renderer (D11) |
| `graph` | the graph model and the layered layout, with `dot` and `mermaid` below it (D18) |
| `web` | the browser component, which embeds its JavaScript (D19) |
| `test/` | a separate module with the corpus, the vega.js oracle and the comparisons (D17) |

`svg` is also the name of the package of okage that zuzu imports. Inside
zuzu, the package of okage is imported as
`okagesvg "github.com/xo/okage/svg"` (D21).

### The API

[`API.md`](API.md) holds the public API (D24): `zuzu.Parse` and
a `Chart` that is safe for concurrent use, a `View` for live data, two kinds
of functional option, a loader for data (D25), a report for the loose mode
(D26), and sentinel errors. The maintainers accepted it on 2026-11-10.

### Dependencies

Each outside package of the module that a user imports has a row here, with
the reason (D5). A package that only the test module uses is not listed.

| Package | Pure Go | Why |
| --- | --- | --- |
| `github.com/xo/okage` | yes | rasterizes SVG (D10), and draws the direct renderer and measures text (D11). It brings `github.com/go-text/typesetting` and `golang.org/x/image` |
| `github.com/go-text/typesetting` | yes | reads the advance widths of the bundled font for the estimate of `text` (D11). okage uses it too, so it adds no module |

The module also embeds files that are not Go packages:

| Files | License | Why |
| --- | --- | --- |
| IBM Plex Sans, weights 400, 500 and 600 | SIL Open Font License 1.1 | the bundled font (D16) |
| the color schemes, one JSON file each | the license of each upstream theme, MIT or compatible | the schemes of D15, with the license file of each source |

The test module adds Node.js with vega.js and vega-lite.js as the oracle
(D17), and goja as the oracle of the expression language (D9).

## The Vega subset

All of Vega is large. Its signals react to events from a mouse or a keyboard,
and its dataflow updates only what changed. That is a program in a browser,
and zuzu draws a chart once. So the first target is static Vega (D7). A
static spec has data, transforms, scales, marks, axes, legends and signals
whose value is a constant or an expression. It has no event streams.

Of the 87 Vega specs in the corpus, 25 bind a signal to an event. Those 25
are out of the first target, and the report says so for each one. Events are
not in the first release, and can come at a later date (D7). Vega-Lite
adds signals for a selection or a parameter that a person changes. A
Vega-Lite spec without either is in the first target.

zuzu never drops a feature that it does not support without a word (D26).
In the strict mode, an unsupported feature is an error. In the loose mode,
zuzu draws the rest and returns a report of what it skipped.

## Expressions and scripting

Vega has an expression language. It is a small subset of JavaScript
expressions, such as `datum.price * 2` or
`datum.open < datum.close ? 'up' : 'down'`. A spec uses it in a filter, a
formula, a signal, a test in an encoding and a label expression. zuzu needs
the same language, and [`EXPRESSIONS.md`](EXPRESSIONS.md) holds the design.

The maintainers accepted choice C on 2026-11-10 (D9). A native evaluator in
Go runs every expression, with its own library of the functions of Vega. An
interface stands between the runtime and the evaluator. goja, which is pure
Go, stands behind the same interface twice: in the test module, as an oracle
that finds each difference from JavaScript, and in an optional package that
runs scripts beyond the expression language. A program that does not import
that package does not link goja. The browser component runs the same
expressions as JavaScript.

## Rendering

### SVG

The SVG writer comes first (D10). Its output is deterministic: the same scene
graph and the same fonts give the same bytes. The tests compare SVG files
against golden files, and a golden file is a file that holds the expected
output.

### okage through SVG

okage renders SVG data with `svg.Renderer`. One renderer, or one
`svg.FontDatabase`, is made once and shared, because it loads the fonts of
the system on its first call, in about 55 ms (okage D36). The renderer is
safe for concurrent use. zuzu passes its logger through `svg.WithLogger`.

### okage directly

The okage peer measured the cost of the round trip on 2026-11-10, on an AMD
Ryzen 9 9950X, with a chart of 800 by 500 pixels that has 300 rectangles,
two axis lines and 10 runs of text:

| Step | Without text | With 10 runs of text |
| --- | --- | --- |
| parse the SVG | 0.39 ms | 0.89 ms |
| parse and render | 5.9 ms | 8.6 ms |

The parse is 7 to 10 percent of the total, and the rasterization is the rest.
A direct renderer saves about that much, and it skips the SVG writer. It
helps a program that draws many charts, and D11 builds it only when a
measurement on the corpus shows the gain.

The direct renderer draws with the root package of okage: `Pixmap.FillPath`,
`Pixmap.StrokePath` and `Pixmap.FillRect` with a `Paint`, `path.PathBuilder`
for the geometry, a `Transform`, and a `Mask` for a clip. These are the API of
the tiny-skia port, which is stable.

The tree of `svg` has no public constructors, and no builder is planned. A
builder differs from usvg, so it needs a decision of the okage
maintainers. zuzu does not ask for one.

### Text

Text is the hard part, for the layout and for the direct renderer. The layout
needs the width of each label before it places an axis or a legend. The
direct renderer needs the outlines of the glyphs.

The layout, the shaping and the fallback of fonts in okage are private.
zuzu does not parse SVG to measure text, because that defeats the purpose of
a direct renderer (D11). Instead, the okage peer writes the API that zuzu
needs, at the time that zuzu needs it: a way to measure a string in a font
and a size, and a way to get the outlines of its glyphs, with the same fonts,
fallback and shaping as the package `svg` of okage. The SVG output comes
first. Until the API exists, the measurement sits behind an interface in the
package `text`, and its first implementation estimates a width from the
characters of the string. A cache keyed by the text, the font and the size
holds each result.

The fonts decide the output. A test that must give the same bytes on every
machine uses fonts that the test brings, and never the fonts of the system.
zuzu bundles IBM Plex Sans (D16), which the targets use.

### The browser

A browser component comes later (D19). The scene graph is JSON, so a server
can send it, or the changes to it, over a stream. A small JavaScript renderer
draws it on a canvas, and the Go module embeds that JavaScript with
`go:embed`. The JavaScript is an asset that the browser runs. It is not code
that the Go module runs, so it does not conflict with D5.

There are two ways to split the work, and the choice is open question 4:

1. The server runs the spec and sends the scene graph. The browser only
   draws. The chart in the browser then matches the SVG of the server.
2. The server sends the spec and the changes to the data, and vega.js runs in
   the browser. The browser then has all of the interaction of Vega.

## Themes

A theme is a mode and a color scheme (D14). The mode, light or dark, sets the
neutral colors: the background, the text, the grid and the axes. The scheme
sets the colors of the data: ten categorical colors, the ramps, and a color
for a rise and for a fall. A caller swaps either one, and any scheme works
in either mode. [`STYLE.md`](STYLE.md) holds the tokens of both.

Many more schemes come from the same upstream themes that the package
`styles` of transit draws from: the styles of chroma and the popular editor
themes, each at a pinned commit and under a license that MIT allows (D15). A
scheme from an editor theme brings the background and the text colors of
that theme for each mode. The targets show two prototypes, `catppuccin` and
`solarized`.

Both parts become one Vega `config` object (D14). The config of Vega already
holds every default that a theme sets: the background, the fonts, the axes,
the legends, the title, the default for each kind of mark, and the color
ranges for categories, ordered values, ramps, diverging values and heat maps.
The order of precedence is: the mode and the scheme, then the `config` of
the spec, then the encoding of a mark. A theme of vega-themes, such as
`ggplot2`, is a JSON config, so zuzu reads it as it is.

The default look is the look of zuzu, not the look of Vega (D12). Some rules
of the look are not config values in Vega, such as the round corners of a
bar, a label at the end of a line, or a halo behind a label. Those are the
defaults of the zuzu renderer, and a theme can turn each one off.

The survey shows why a theme matters. Of the 13 libraries in
`compare.html`, Vega, ECharts, ApexCharts, Mermaid, Highcharts and Matplotlib
switch to dark with one setting, and Observable Plot takes the text color of
the page. Chart.js, Plotly, Recharts, uPlot, Lightweight Charts and Graphviz
need each color set by hand.

## Diagrams

DOT and Mermaid come later (D18). Each one compiles to the same spec, so
that a diagram uses the same themes, text and renderers as a chart.

Vega has no layered graph layout, which a flowchart needs. A layered layout
puts the nodes of a directed graph in ranks and keeps the edges short and
uncrossed. It is also called a Sugiyama layout. zuzu adds it as a transform
of its own, with a name that cannot collide with a transform of Vega, such
as `zuzu.layered`. zuzu writes its own layout from the documented algorithm and
does not port dagre, the layout that Mermaid uses (D6).

The parsers come first for the subset that a flowchart needs: nodes, edges,
labels, subgraphs and the direction of the graph. Mermaid has about 20 kinds
of diagram. Its pie, xychart, sankey, radar and treemap are charts, and
compile to the same chart specs as Vega-Lite.

## usql

`usql` makes a chart from a query with its `\chart` command today, through
`chartfmt`, goja and ECharts. zuzu can replace that stack. The work comes
much later, and the `usql` peer does it with this repository (D20). A result
set has typed columns, so zuzu can choose an encoding from the types: a date
or a time is temporal, a number is quantitative and a string is nominal. The
API takes a result set from the start, through `WithResultSet` (D24).
zuzu never imports `usql`. The dependency runs the other way.

## Conformance

The corpus of `vegagoja` is the test of compatibility (D17). It holds:

- 579 Vega-Lite specs, with an SVG from vega.js for 578 of them.
- The 578 Vega specs that vega-lite.js compiled from them.
- 87 Vega specs, with an SVG for 85 of them.
- 13 samples from Deneb, and the data files that the specs read.

Those files came from Vega-Lite 5.18.1 and Vega 5.29.0. The oracle makes them
again with the current releases, Vega-Lite 6.5.0 and Vega 6.4.0. The test
module runs vega.js in Node.js, as an oracle, and the module that a user
imports never does.

zuzu does not reproduce the look of Vega (D12), so a comparison of SVG or
pixels with vega.js tells nothing. D17 decides what the comparison measures
instead: the meaning of a chart. These must match exactly, within a
tolerance for floats:

1. The Vega spec that `vegalite` compiles, against the one that vega-lite.js
   compiles.
2. The rows of each dataset after the transforms.
3. The type and the domain of each scale.
4. The number of items of each mark and the datum bound to each item.
5. A label whose format the spec gives.

These match as a relation, or within a tolerance: the position of each item
as a fraction of the domain of its scale, the colors of the categories, and
the tick values and the default formats where the spec does not set them.

Every report gives two numbers: the share of passes in the set of specs that
zuzu claims to support, and the share of passes in the whole corpus. A spec
that zuzu does not support counts as a failure in the second.

The look has its own tests. The output of zuzu has golden files, which catch
a change of the look, and a person reviews a change to them in a gallery, as
the targets are reviewed now. okage is tested apart from zuzu.

## Charts

The survey sorted the charts by how many of the 10 largest general libraries
draw each one without help. That order sets the order of work in phase 5 and
later.

| Tier | Charts | What the runtime needs |
| --- | --- | --- |
| 0 | line, area and stacked area, bar (grouped, stacked, horizontal), scatter and bubble, pie and donut, heat map, histogram, layers with two y axes | the marks rect, line, area, symbol, arc, rule and text. The transforms stack, bin and aggregate. Linear, log, time, band, point and ordinal scales |
| 1 | box plot and error bars, radar, choropleth map, treemap, sunburst, sankey, funnel, candlestick, violin, density and regression, streamgraph, gauge, small multiples | the transforms quantile, kde, regression, loess, stratify, treemap and partition. Polar layout, projections, facets. A time scale that skips the days with no trade |
| 2 | parallel coordinates, chord, force graph, tree, circle packing, waterfall, beeswarm, contour, hex bins, word cloud | one layout transform each: pack, tree, force, contour, hexbin, wordcloud |
| 3 | flowchart and other layered graphs, from DOT and Mermaid | the layered layout and edge routing (D18) |

Vega has the transforms for most of tiers 1 and 2. It has no sankey, no
layered layout and no time scale that skips days. zuzu adds those as its own
transforms and scales, with names that cannot collide.

## Phases

The order of the phases follows the advice of three other models, which
are summarized under External review below. Each phase has an exit criterion
that a test measures.

### Phase 0. The repository and the harness

The documents, the skills and the tests that check them. The CI workflow and
the lint configuration. The first commit goes straight to `main`, when the
maintainers say so. The corpus in the test module, with the license file of
each source beside its files, and an inventory that
lists the features each spec uses: its marks, its transforms, its
expression functions and whether it binds events. The oracle that writes the
references of D17 for each spec with vega.js: the compiled spec, the
datasets, the scales and the bound items.

Exit: CI passes, the inventory exists, and the oracle writes the references
for the whole corpus. No code of the package exists.

Phase 0 was done on 2026-11-10. The tests of the documents, the lint
configuration and the CI workflow exist. The corpus holds 679 specs, and 540
are in the first target. The oracle writes their references with no failure.
[`../test/README.md`](../test/README.md) says how to run each tool.

### Phase 1. The scene graph and the SVG writer

The Go types of the scene graph, their JSON form, and the SVG writer. The
theme types: the two modes, the color schemes and their config (D14). An
image through okage.

Exit: scene graphs built by hand for the tier 0 targets render to SVG that a
person accepts against the targets in a gallery, in both modes and in every
scheme. Their golden files exist, and okage renders each one.

Do not start the runtime yet.

On 2026-11-10 the packages `scene`, `theme` and `svg` exist. `scene` reads
and writes the scene graph of vega.js. `theme` holds the two modes and the
four schemes, and a test checks them against [`STYLE.md`](STYLE.md). `svg`
draws every kind of mark of Vega but `shape`, with the curves, the symbols
and the arcs of d3-shape, and its golden files render in okage. The tier 0
targets are golden tests as scene graphs, and `docs/targets/compare.html`,
which the test module writes, shows each target beside the output of zuzu.
The exit waits for a maintainer to accept that page. "Phase 1" in
[`BACKLOG.md`](BACKLOG.md) says how to make it.

### Phase 2. Text

The `text` interface with the estimate and its cache, the fonts of the tests,
and the truncation of a label to a limit. The okage measurer comes when the
okage peer has written its API (D11).

Exit: the same scene graph gives the same bytes twice, and the estimate is
within a stated tolerance of the widths that a browser measures for the
labels of the targets.

Phase 2 was done on 2026-11-10. The package `text` embeds IBM Plex Sans 1.1.0
in the weights 400, 500 and 600, with its license (D16). Its estimate sums
the advance widths of the font. Over the 182 labels of the tier 0 targets,
it differs from the shaped and kerned layout of okage by 0.14 pixels on
average and by 1.49 pixels at most, within the tolerance of one pixel or two
percent that `TestEstimateIsCloseToOkage` holds. okage is the reference
rather than a browser, because okage draws the output of zuzu. The SVG writer
cuts a text to its limit with the estimate.

### Phase 3. Static Vega

The `vega` types and their parser, the expression language (D9), the
formats, the dataflow with the core transforms, the scales, the axes, the
legends, the title, the size of a view and the facets.

Exit: the datasets, the scales and the bound items match the references of
vega.js (D17) for a stated share of the Vega specs in the first target. The
share is set at the start of the phase, from the inventory.

Do not build signals that bind events.

No share was set at the start of the phase. This plan proposes one, and
open question 13 asks the maintainers to accept it. The check runs over the
540 specs of the first target: the 70 Vega specs, and the Vega specs that
vega-lite.js compiled from the 470 Vega-Lite specs, which the references
hold. The transforms of tiers 1 and 2, the projections and topojson come in
phase 5, so the specs that need them fail here. The proposed exit is that 80
percent of the specs match in all three checks, and that 85 percent of the
datasets, 90 percent of the scales and 90 percent of the marks that draw
data match.

On 2026-11-10 the code of phase 3 exists, and `go run ./cmd/conform` in the
test module measures it:

| Check | Match | Of |
| --- | --- | --- |
| specs, in all three checks | 450 (83.3%) | 540 |
| datasets | 1103 (89.8%) | 1228 |
| scales, by type and domain | 1329 (97.2%) | 1367 |
| marks that draw data, by the data of their items | 3671 (94.3%) | 3893 |

Every spec runs. In the loose mode, the report names each feature that zuzu
skipped. The most common are the mark `shape` with its geographic
transforms, and the transforms `density`, `geojson`, `geopoint`, `kde`,
`stratify` and `regression`, all of phase 5. Most of the other differences
come from vega.js itself. A transform of Vega changes the rows that the
datasets which derive from the same source share, and zuzu copies a row
before it changes it. `docs/BACKLOG.md` lists what is left.

The phase holds these packages:

1. `vega` reads and writes every spec of the corpus and gives back the same
   JSON.
2. `expr` parses the grammar of vega-expression. It matches vega.js on 253
   expressions that `test/oracle/exprs.mjs` evaluates, with the functions of
   colors.
3. `format` matches d3-format on 1176 numbers, d3-time-format on 820 times
   and 42 parses, and `Date.parse` on 32 dates. `test/oracle/formats.mjs`
   writes the cases.
4. `dataflow` reads JSON, CSV, TSV and DSV, and runs 20 transforms.
5. `scale` holds the scales of Vega and their ticks.
6. `layout` draws the axes, the legends and the titles in the look of zuzu.
   The conformance job of CI runs the oracle and fails when a share falls
   below the proposed exit.
7. The root package runs a spec: it works out each signal, dataset and scale
   when something first asks for it, builds the marks and the facets, and
   grows the canvas to hold the guides, as the autosize `pad` of Vega does.

### Phase 4. Vega-Lite

The `vegalite` types and the compiler to `vega`, for Vega-Lite without
selections and parameters that a person changes.

Exit: the compiled spec matches the reference of vega-lite.js, and the
datasets, the scales and the bound items match (D17), for a stated share of
the Vega-Lite specs in the first target.

### Phase 5. Tier 1 and tier 2

The transforms and the scales of tiers 1 and 2, in the order of the table
above, with the projections of `geo`. Then the sankey of zuzu and the time
scale that skips days.

Exit: a stated share of the whole corpus passes, and each chart of tiers 1
and 2 draws its target from a spec.

### Phase 6. The direct okage renderer

Measure first. If the corpus renders faster through `raster` than through
SVG and okage, build it (D11).

Exit: the image of each chart is the same as the image through SVG, within
the tolerance of okage D31, and the time saved is measured and written down.

### Phase 7. DOT and Mermaid

The graph model, the layered layout, the DOT parser and the flowchart of
Mermaid (D18).

Exit: a set of fixtures for each language, with references from Graphviz
and from Mermaid.

### Phase 8. The browser

The scene graph as a stream, the JavaScript canvas renderer that `web`
embeds, and the updates of live data (D19).

Exit: a page updates a chart from a stream of rows, and its first frame
matches the SVG of the server.

### Phase 9. usql

With the `usql` peer (D20).

## External review

Gemini 3.1 Pro, GPT-6 Astra and DeepSeek V4 Pro each received the
requirements and the corpus on 2026-11-10 and proposed the phases. This is
where they agreed:

1. Target static Vega first, with the Vega-Lite that compiles to it. Leave
   signals that bind events, and the reactive dataflow, for later. All three
   said this, and D7 follows it.
2. Write the expression language as a parser and a tree walker in Go, by
   hand. Do not run a JavaScript engine for it. All three said this, and D9
   follows it.
3. Fix the scene graph before the runtime, and test the SVG writer first.
   Gemini and Astra said this, and D8 and phase 1 follow it.
4. Compare structure before pixels. Pixels differ with the fonts, so a
   comparison of pixels alone gives false failures. All three said this.
   D17 went further after D12 and compares no pixels with vega.js at all.
5. Bundle a font for the tests, because the fonts of a system change the
   layout. Gemini and DeepSeek said this, and D16 bundles IBM Plex Sans.

This is where they disagreed, and what the plan does:

1. Astra said to cut the browser component, because it needs JavaScript.
   The plan keeps it, because the JavaScript runs in the browser and is not
   code that the Go module runs.
2. DeepSeek put the rasterizer in its own phase. okage exists, so the plan
   needs no such phase.
3. Gemini put the themes after the Vega-Lite compiler. The plan puts the
   theme types in phase 1, because the maintainers asked for light and dark
   from the start, and a theme is only a config value.
4. Astra asked for the eligible set and the whole corpus to be reported
   apart, and for okage to be tested apart from zuzu, so that one defect does
   not hide another. D17 takes both.

After D12, Gemini and DeepSeek were asked on 2026-11-10 what a comparison
with vega.js still measures when the look differs on purpose. Both said to
compare the meaning: the compiled spec, the datasets after the transforms,
the domains of the scales and the data bound to each item, exactly, and the
positions only as fractions of a domain. They disagreed on the ranges of the
scales. DeepSeek said to compare them exactly, and Gemini said that they
depend on the layout. D17 follows Gemini, because a range is a size in
pixels.

The okage peer answered four questions about rendering on 2026-11-10. Its
measurements are under Rendering above. Its advice on text no longer
applies, because the maintainers rejected the parse of an SVG fragment
(D11).

## Open questions for the maintainers

A question keeps its number after it is answered, so that a reference to it
stays true.

1. Answered on 2026-11-10 by D9: scripting beyond the expression language
   runs in goja, in an optional package behind the same interface.
2. Answered on 2026-11-10 by D16: zuzu bundles IBM Plex Sans.
3. Answered on 2026-11-10 by D12: the default theme has the look of zuzu.
4. Does the browser component receive the scene graph from the server, or
   the spec and the data for vega.js? See Rendering above. The maintainers
   left this to be decided later, on 2026-11-10.
5. Answered on 2026-11-10 by D7: events are not in the first release, and
   can come at a later date.
6. Answered on 2026-11-10 by D6: zuzu follows the documented behavior and
   is not a port of Vega, Vega-Lite or dagre.
7. Answered on 2026-11-10 by D21: the names follow the advice of Astra and
   Gemini.
8. Answered on 2026-11-10 by D12 and D17: no comparison of layout with
   vega.js remains, so the way vega.js measures text does not matter.
9. Answered on 2026-11-10: `LICENSE` has the same copyright line as the
   other `xo` repositories, with the years 2015-2026, under the MIT License
   (D22).
10. Answered on 2026-11-10: the okage peer writes the API when zuzu needs
    it, and the SVG output comes first (D11).
11. Answered on 2026-11-10: the four first schemes stay (D14), and many more
    come from the upstream themes that transit draws from (D15).
12. Which fonts suit the schemes from editor themes, and does zuzu list a
    font for each? The fonts must be under the Open Font License or a
    license like it, and zuzu bundles none of them (D15, D16).
13. Is the share of phase 3 accepted? Phase 3 above proposes that 80 percent
    of the 540 specs of the first target match in all three checks of D17,
    and that 85 percent of the datasets, 90 percent of the scales and 90
    percent of the marks that draw data match. On 2026-11-10 the code meets
    it.
