# Backlog

This document lists work that is known and not done. Each item names the
decision or the document that found it.

A decision is not a backlog item. It goes in
[`decisions/`](decisions/README.md). A question for a maintainer goes at the
end of [`PLAN.md`](PLAN.md), under Open questions for the maintainers. When an
item here is done, delete it, and record in `decisions/` anything that was
decided on the way.

## Phase 0

### Run the oracle in CI

The oracle writes the references in about 8 seconds, and git ignores them
(`test/README.md`). The comparison with them exists in `test/conform` since
phase 3. Add a job to `.github/workflows/test.yml` that sets up Node.js,
installs the packages of `test/oracle`, runs the oracle with `TZ=UTC`, and
runs `go run ./cmd/conform`. Then add a test that fails when a share falls
below the exit of phase 3.

### Add a test that names a person in a decision

`TestTheDocumentsNameNoPerson` reads every document for the names of the
maintainers (D22). Its list of names is short, and a new maintainer adds a
name to it.

## Phase 1

### Review the writer against the targets

`go run ./cmd/targets` in the test module rebuilds each target as a scene
graph, writes it with `svg`, and writes `docs/targets/compare.html`, which
shows each target beside the output of zuzu. On 2026-11-10, 104 of the 106
targets differed by at most 0.04 percent of their pixels. The flowchart, in
both modes, scales a group, which a scene graph does not hold. The exit of
phase 1 is that a maintainer accepts the page.

The targets were drawn with d3, so a curve, an area or an arc of a target is
a path in its scene graph. The comparison checks the paint, the text, the
groups, the gradients, the symbols, the rects and the rules of the writer.
The curves, the arcs and the trails of the writer have their own tests
against d3, in `svg/shapes_test.go`.

### Draw what the writer does not draw yet

1. The mark `shape` returns an error. Vega draws it with a function that the
   JSON of a scene graph cannot hold, so the runtime of zuzu writes a path
   mark in its place.
2. The interpolations `basis-open`, `basis-closed`, `cardinal-open`,
   `cardinal-closed`, `catmull-rom-open` and `catmull-rom-closed` return an
   error.

### Make the Vega config of a theme

D14 says that a theme becomes one Vega config. The package `vega` holds the
type `Config` since phase 3, as a map of JSON, and the runtime does not read
it yet. The runtime reads the colors of `theme.Theme` directly: the guides,
the background, and the ranges `category`, `ordinal`, `ramp`, `heatmap` and
`diverging`. A scheme that a spec names, such as `blues`, takes a ramp of the
theme, and a scheme of categories, such as `tableau10`, takes the colors of
the categories of the theme.

## Phase 3

### Draw the marks in the look of zuzu

The marks follow the encodings of the spec, so a Vega-Lite spec that
vega-lite.js compiled draws its bars in `#4c78a8`. The rules of the marks in
`docs/STYLE.md`, such as the round end of a bar and the line of 2, come from
the config of the theme, which waits for the item above. The axes, the
legends and the titles already follow `docs/STYLE.md`.

### Finish the guides

1. A quantitative axis draws no base line at zero, which rule 2 of "Axes and
   grids" in `docs/STYLE.md` asks for.
2. An axis reads neither `labelFlush` nor `labelLimit`, and a legend reads
   neither `direction`, `orient`, `columns` nor its encodings.
3. A legend of sizes draws a circle for each value, and not the nested
   circles of the targets.
4. The autosize `fit`, `fit-x` and `fit-y` grow the canvas as `pad` does, and
   do not shrink the plot area.
5. The package `layout` has no tests of its own. The golden file
   `testdata/bar.svg` and the gallery of the test module show its output.

### Add the expression functions that zuzu does not have

An expression that calls one of these functions is an issue in the loose
mode: `contrast`, `luminance`, `rgb`, `hsl`, `lab`, `hcl`, `gradient`,
`copy`, the functions `geo*` of phase 5, `treePath`, `treeAncestors`,
`vlSelectionTest` and `vlSelectionResolve`.

### Decide whether to share rows between datasets as Vega does

A transform of Vega, such as `formula` or `stack`, changes the rows in
place, and the datasets that derive from the same source share those rows.
So a dataset of vega.js can hold fields that a transform of another dataset
added, and `facet_bullet` and `pancake-spider-chart` differ for this reason.
zuzu copies a row before a transform changes it, which keeps each dataset to
its own transforms. The maintainers can decide whether zuzu follows Vega
here. Until then, the conformance counts these as differences.

### Make the order of mixed values the same as vega.js

A domain sorted with `sort: true` that mixes strings and numbers, such as
the titles of `bar_grouped_thin`, has an order in vega.js that depends on the
sort of JavaScript, which is not a total order on mixed types. zuzu sorts
numbers before strings.

## Themes

### Write the tool that converts upstream themes into schemes

D15 calls for a tool that reads the upstream palette of each theme at a pinned
commit and writes one JSON scheme for each, with its license file. The first
sources are the styles of chroma and the editor themes that transit D98
lists. Each theme with a license that MIT does not allow stays out. The two
prototypes in the targets, `catppuccin` and `solarized`, show the mapping
from a palette to the tokens of a scheme.

### Choose a font for each scheme from an editor theme

Open question 12. A scheme can name a font that suits its theme. zuzu
bundles only IBM Plex Sans (D16).

### Pass the bundled fonts to okage

D16. The package `text` embeds IBM Plex Sans, and `text.Fonts` returns the
files. When the root package renders an image (D24), it loads them into the
`FontDatabase` of okage and sets the family as the sans serif family, so that
an image draws the same font that the layout measured.

## Survey

### Add the libraries that the comparison page leaves out

`docs/survey/compare.html` leaves out four libraries that the survey names:

1. The diagram tool D2, which has a WebAssembly build for the browser and no
   plain script.
2. Nivo, which has no build that a page can load without a bundler.
3. ggplot2 and Seaborn, which need R and the Python packages that Seaborn
   brings. `gen-static.py` can draw Seaborn as it draws Matplotlib.
4. Graphviz and Mermaid from the command line, `dot` and `mmdc`, to compare
   with their browser builds on the same graph.

### Draw the charts that the page marks as missing in Matplotlib

The treemap, the sunburst, the sankey and the map have no Matplotlib image.
Each needs another package: squarify, nested pies by hand, or geopandas.
