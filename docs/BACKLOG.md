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

## Phase 3

### Draw the rest of the marks in the look of zuzu

A tooltip comes with the browser, in phase 8. A scheme that a spec names,
such as `blues`, takes a ramp of the theme, and a scheme of categories,
such as `tableau10`, takes the colors of the categories of the theme. A
Vega-Lite spec that vega-lite.js compiled, as the gallery draws it with
`-vl=false`, still names `#4c78a8`.

### Make the grid layout follow the layout of Vega

`grid.go` lays out the cells, the headers, the footers and the titles of a
facet and of a concatenation in a grid, with the full bounds of each cell.
It does not read `align`, `center`, `offset`, `headerBand` or a padding for
the rows apart from the columns, and a header with one item serves only the
first row or column.

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

## Phase 7

### Draw the parts of Mermaid that draw another way

1. A named shape that zuzu does not draw yet, such as a shape of BPMN, draws
   as a rounded rectangle.
2. The properties of a link, such as `animate`, are issues.
3. A label of HTML on an edge, a cluster or the graph draws as plain text.
   `headlabel`, `taillabel` and `xlabel` are issues. dbtpl writes
   `headlabel` on each edge of a schema.
4. The error of the first line of a diagram says line 1, even after front
   matter, because `Diagram` does not keep the number of that line.
5. A subgraph without a direction, inside a subgraph with a direction,
   lays out in the direction of the subgraph that holds it. Mermaid 12
   draws some of these in another direction. In a `flowchart LR`, a
   subgraph inside a subgraph with `direction TB` draws LR in Mermaid.

### Give a chart of Mermaid the time of the render

A gantt draws no marker of today, and a `dateFormat` with no date, such as
`HH:mm`, puts its tasks on 1970-01-01, because the compile has no time of
the render. A time that the caller gives fixes both.

### Fix the gaps of the Vega-Lite compiler that the charts found

The compilers of the charts work around each of these:

1. A bar on a continuous x with no width does not draw. Vega-Lite gives it
   a size.
2. A nominal y with a fixed height draws its first category at the bottom.
   Vega-Lite draws it at the top.
3. `y2: {datum}` is dropped, and `y2: {field}` beside a datum `y` takes the
   value of `y`.
4. A text mark of an arc does not read `theta` and `radius`.
5. An axis has no `labelExpr` and no `titlePadding`. They are issues.
6. A mark's own `style` goes before the style of its paint, so a mark cannot
   take the colors of `zuzu.edge`, and an unfilled point draws with a
   stroke in the color of the background.
7. A rule of Vega-Lite with one item takes the look of a reference, and its
   label writes a time as a number.

### Fix the text of the direct renderer

A rotated text whose mark holds a glyph that the font lacks, such as ❤,
also draws a mirrored copy of each text. `dx` and `dy` on a rotated text
move it on the page, not along the text as in Vega.

### Give the look a color of a critical task

A critical task of a gantt has an outline of `#e5484d`, which is not a
token of `docs/STYLE.md`. A text on a leaf of a treemap takes the style
`zuzu.edgeLabelBox` for the color of the background. A style of text on a
color is cleaner.

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
files. Since phase 6, `Chart.Image` loads them into a renderer of okage
with no font of the system, and sets the family as the sans serif family,
so that an image draws the same font that the layout measured. A character
that IBM Plex Sans lacks draws nothing. The option `WithFonts` of API.md,
which gives other fonts, and a fallback to the fonts of the system, are not
done.

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
