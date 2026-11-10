# Backlog

This document lists work that is known and not done. Each item names the
decision or the document that found it.

A decision is not a backlog item. It goes in
[`decisions/`](decisions/README.md). A question for a maintainer goes at the
end of [`PLAN.md`](PLAN.md), under Open questions for the maintainers. When an
item here is done, delete it, and record in `decisions/` anything that was
decided on the way.

## Phase 0

### Add a test that names a person in a decision

`TestTheDocumentsNameNoPerson` reads every document for the names of the
maintainers (D22). Its list of names is short, and a new maintainer adds a
name to it.

## Phase 1

### Draw what the writer does not draw yet

1. The interpolations `basis-open`, `basis-closed`, `cardinal-open`,
   `cardinal-closed`, `catmull-rom-open` and `catmull-rom-closed` return an
   error.

### Make the Vega config of a theme

D14 says that a theme becomes one Vega config. The package `vega` holds the
type `Config` since phase 3, as a map of JSON, and the runtime does not read
it yet. The runtime reads the colors of `theme.Theme` directly: the guides,
the background, and the ranges `category`, `ordinal`, `ramp`, `heatmap` and
`diverging`. A scheme that a spec names, such as `blues`, takes a ramp of the
theme, and a scheme of categories, such as `tableau10`, takes the colors of
the categories of the theme. A mark of a Vega spec that sets neither a
fill nor a stroke takes the first color of the categories of the theme, or
its text color.

## Phase 3

### Draw the marks in the look of zuzu

The marks follow the encodings of the spec, so a Vega-Lite spec that
vega-lite.js compiled draws its bars in `#4c78a8`. The rules of the marks in
`docs/STYLE.md`, such as the round end of a bar and the line of 2, come from
the config of the theme, which waits for the item above. The axes, the
legends and the titles already follow `docs/STYLE.md`.

### Make the grid layout follow the layout of Vega

`grid.go` lays out the cells, the headers, the footers and the titles of a
facet and of a concatenation in a grid, with the full bounds of each cell.
It does not read `align`, `center`, `offset`, `headerBand` or a padding for
the rows apart from the columns, and a header with one item serves only the
first row or column.

### Finish the guides

1. An axis reads neither `labelFlush` nor `labelLimit`, and a legend reads
   neither `direction`, `orient`, `columns` nor its encodings.
2. A legend of sizes draws a circle for each value, and not the nested
   circles of the targets.
3. The autosize `fit`, `fit-x` and `fit-y` grow the canvas as `pad` does, and
   do not shrink the plot area.

### Add the expression functions that zuzu does not have

An expression that calls one of these functions is an issue in the loose
mode: `gradient`, `copy`, `geoShape`, `treePath`, `treeAncestors`,
`vlSelectionTest` and `vlSelectionResolve`. The functions `geoArea`,
`geoBounds` and `geoCentroid` measure in the plane of a projection. With no
projection, Vega measures on the sphere, and zuzu does not.

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
