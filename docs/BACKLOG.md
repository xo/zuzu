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

### Share rows between datasets as Vega does

A transform of Vega, such as `formula` or `stack`, changes the rows in
place, and the datasets that derive from the same source share those rows.
So a dataset of vega.js can hold fields that a transform of another dataset
added, and `facet_bullet` and `pancake-spider-chart` differ for this reason.
zuzu copies a row before a transform changes it. D32 part 14 says that zuzu
follows vega.js here.

### Make the order of mixed values the same as vega.js

A domain sorted with `sort: true` that mixes strings and numbers, such as
the titles of `bar_grouped_thin`, has an order in vega.js that depends on the
sort of JavaScript, which is not a total order on mixed types. zuzu sorts
numbers before strings.

## Phase 7

### Do the work of D32

D32 records the answers of the maintainers of 2026-11-12. On 2026-11-12
zuzu did all of its parts, and drew the requirement, gitGraph, block,
swimlane, use case, agentflow, timeline and C4 diagrams of part 11. These
are left:

1. The next kinds of Mermaid in the order of D32 (11): venn, treeView,
   railroad, eventmodeling, zenuml, cynefin, ishikawa, kanban, packet and
   info. They hold 171 files of the corpus of mermaid-go-renderer.
2. A kind of Mermaid that lays out when it is read, such as a sequence, a
   pie, a gantt, a journey, a mindmap, a treemap, an architecture diagram,
   a gitGraph, a block diagram, a timeline or a C4 diagram, measures an
   emoji as nothing, because `WithEmoji` is an option of the render. Its
   labels can be too narrow (6).
3. The SVG writer measures the `limit` of a text without the font of
   emoji (6).
4. A weight or a style in `fontFamily` draws only in the graphs and in the
   sequence. It is an issue in the other kinds (5).
5. `href` and `tooltip` on a table or a cell of an HTML label of DOT are
   issues, because the package `html` of okage keeps no link (10).
6. The wrap of a kind with labels of more than one size in Mermaid uses
   11/14 for all of them (7): a sequence wraps its messages at 16 pixels
   and its actors and notes at 14, and a use case diagram draws its use
   cases at 12 and its actors at 14.
7. The node property `algorithm`, an ELK layout for one subgraph, is an
   issue. D32 part 2 does not say whether it is a hint.


### Draw two looks by hand

zuzu draws its standard look today. D32 adds two more looks, which a
caller selects for any chart or diagram. zuzu draws each in its own way
and ports no code of another program. Each look changes the lines, the
fills and the font, and keeps the layout.

1. A look like Excalidraw. Lines and outlines wobble as if drawn by hand,
   and fills are hatched, as in Excalidraw. The text draws in Excalifont.
2. A look like xkcd, like the look that `plt.xkcd()` of matplotlib gives.
   Lines wobble in long, smooth curves, and the text draws in a font like
   the handwriting of xkcd.

Before the work starts:

1. Make sure that the license of each font allows zuzu to bundle it, as
   D16 asks: the Open Font License or a license like it. Check the license
   of Excalifont, and find a font for the look of xkcd under such a
   license.
2. Design the option that selects a look. `look: handDrawn` of Mermaid
   selects the look like Excalidraw (D32 part 2).
3. Make the font of a look opt-in: a caller can give another font, and by
   default a look draws with the font that zuzu tests it with (D32 part 2).
4. A wobble must be the same on each render, so that the output stays
   deterministic.

### Add an ELK layout

D32 accepts `layout: elk` of Mermaid with no issue, and zuzu draws such a
diagram with `zuzu.layered`. An ELK layout routes edges at right angles
and places nodes in another way.

### Draw the parts of Mermaid that draw another way

1. The shape `person` and the shapes of BPMN that put the label under the
   shape, such as `bpmn-start` and `bpmn-gateway`, draw as rounded
   rectangles. zuzu always puts a label in the middle of its node.
2. A subgraph without a direction, inside a subgraph with a direction,
   lays out in the direction of the subgraph that holds it. Mermaid 12
   draws some of these in another direction. In a `flowchart LR`, a
   subgraph inside a subgraph with `direction TB` draws LR in Mermaid.
3. `f-circ`, `junction`, `sm-circ` and `start` keep their label, and
   `start` draws as a full circle. Mermaid draws them as small circles with
   no text.
4. The shape `subroutine` is too narrow for a short label, so the label
   covers its two inner lines.

### Fix what the corpus of mermaid-go-renderer found

The corpus of `github.com/bvolpato/mermaid-go-renderer` at commit 9af8f11
holds 2811 files of Mermaid 12.2. mmdc draws 2807 of them. On 2026-11-12
zuzu drew 2637 of them, and 2508 with no issue. 171 of the files that zuzu
does not draw are of kinds that it does not draw yet. One more is an
example of the documents of Mermaid whose kind is `...`.

1. A style or a class on the id of a subgraph of a flowchart, such as
   `style sub fill:#f00`, is dropped with no issue. zuzu makes a node of
   the id and removes it. Mermaid styles the subgraph.
2. A flowchart does not read properties after brackets, as
   `a["A"]@{ shape: cyl }`, and it reads `@{ ... }` over more lines with
   the commas of one line. The agentflow diagram reads both.
3. Mermaid moves a node into the innermost subgraph that names it, at the
   `end` of that subgraph. zuzu keeps the node in the first subgraph that
   names it.
4. In a layout LR with subgraphs inside subgraphs, the title of an outer
   subgraph covers the border of an inner one.
5. `\n` in the label of a node draws as text. With `htmlLabels: false`,
   Mermaid starts a new line there.
6. A `classDef` of a treemap splits at `\,`. Mermaid reads it as a comma of
   a value, as in a flowchart.
7. A state or a class diagram reports `flowchart.nodeSpacing`, which
   Mermaid reads there.
8. In an ER diagram, a text where only a part is bold or italic, such as
   `*first*Name`, draws plain. A line draws in one font.
9. An outlined label of a sankey stays inside the chart, where Mermaid
   draws it past the left edge.
10. A gantt with `dateFormat ss` and a start of `0` is an error. Mermaid
    reads `0` as a date of 2000 and the end as a time of the day of the
    render. It needs the time of the render (see "Give a chart of Mermaid
    the time of the render").
11. A swimlane diagram does not do `optimizeRanksByCrossings` with
    `ignoreCrossLaneEdges: false`, `automaticLaneOrdering`, or the hops of
    lines. Some of its routes share a line, and the titles of its lanes do
    not turn in a layout LR.
12. In a use case diagram, `colorScheme: rotate` is an issue. Mermaid
    drops the last `;` of a `style` line, and zuzu reports an error.
13. A line of a timeline that ends in `: ` with a space is an error in
    Mermaid, and zuzu reads the next line as its event.
14. Math of KaTeX in a label, written as `$$...$$`, is an issue in each kind
    where Mermaid draws it as math, and draws as its LaTeX source. To draw
    the math, zuzu needs a renderer of math, and that needs a decision.
15. zuzu reads `text: a, b` in the body of a requirement, and Mermaid gives
    an error of the parse at the comma.
16. A sequence diagram decodes `#lt;br#gt;` into `<br>` and then starts a
    new line there. Mermaid draws the text `<br>`.


### Draw more of Mermaid

1. `flowchart.minNodeWidth`, 120 by default in Mermaid 12.3.0, is not read,
   so a node with a short label is narrower than in Mermaid.
2. The title of a subgraph that is wider than its nodes draws past the box
   of the subgraph. Mermaid makes the box wider. Only a subgraph that lays
   out on its own, in `graph/nest.go`, reads the width of its title.
3. A chart of `xyChart` reports `tickLength`, `tickWidth`, `labelPadding`
   and `titlePadding` of an axis, the width of the line of an axis of
   numbers, and `plotReservedSpacePercent`. The look of zuzu has no ticks
   and no line on an axis of numbers, and the runtime has no padding of the
   labels or of the title of an axis.
4. The title of a gantt with sections starts at the names of the tasks, to
   the right of the names of the sections, because a title takes the bounds
   of the axes.

### Draw more of DOT

The DOT files come from the gallery of Graphviz, in `testdata/zuzu` of iv.

1. In LR or RL, a record that is one group in braces lies one field under
   another. Graphviz puts the fields side by side. A group inside a group
   is flat. `\l` in a plain record label centers the line.
2. A record drops an empty field that has no port: `<f0> a| | |2` draws 2
   fields, and Graphviz draws 4.
3. `weight=0` counts as 1, because the graph and the transform read 0 as
   the default weight.
4. In LR, Graphviz draws a loop above its node. zuzu draws it below.
5. `label=""` draws the id of the node. Graphviz draws no text.
6. An edge with a list of colors, such as `color="a:b"`, draws only the
   first color, with no issue. Graphviz draws a line in each color.
7. `bgcolor` of the graph is an issue. Graphviz fills the page with it.
8. An edge to a cluster, with `lhead` or `ltail`, whose route is cut at the
   box of the cluster, puts its end labels back at the first place, with no
   check for overlap (`clipEdge` in `graph/nest.go`).


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
6. An unfilled point draws with a stroke in the color of the background.
7. A rule of Vega-Lite with one item takes the look of a reference, and its
   label writes a time as a number.
8. `x2: {"value": "width"}` is dropped with no issue, and the rect does not
   draw.
9. The axis of an encoding reports `zindex`, `labelFontSize`,
   `labelPadding`, `titleFontSize`, `titlePadding`, `tickSize`, `tickWidth`
   and `domainWidth`. `config.axisX` and `config.axisY` apply some of them.
10. The channel `href` is read and dropped with no issue, and a field of
    the channel `tooltip` only joins the aggregate. Neither becomes an
    encoding of Vega, though the scene now carries both.

### Fix the bounds of a text and the place of an axis title

The bounds of a text ignore its `lineHeight`, so a text of more lines with
a `lineHeight` above its font size is cut at the bottom of a chart. The
title of the axis at the bottom has a fixed place, so it covers labels
that turn, such as with `labelAngle: 45`.

### Fix the text of the direct renderer

A rotated text whose mark holds a glyph that the font lacks, such as ❤,
also draws a mirrored copy of each text. `dx` and `dy` on a rotated text
move it on the page, not along the text as in Vega.
An empty `ellipsis` writes the default "…", because the field of the scene
item drops an empty value. A text cannot be cut with no mark.

### Give the look a color of a critical task

A critical task of a gantt has an outline of `#e5484d`, which is not a
token of `docs/STYLE.md`. A text on a leaf of a treemap takes the style
`zuzu.edgeLabelBox` for the color of the background. A style of text on a
color is cleaner.

## Themes

### Write the tool that converts upstream themes into schemes

D15 calls for a tool that reads the upstream palette of each theme at a pinned
commit and writes a scheme for each, with its license file. D33 says that a
scheme is a Go value in `theme/theme.go`, so the tool writes Go values. The
schemes `catppuccin` and `solarized`, which exist only in
`docs/targets/gen-targets.mjs`, move into the package `theme` too. The first
sources are the styles of chroma and the editor themes that transit D98
lists. Each theme with a license that MIT does not allow stays out. The two
prototypes in the targets, `catppuccin` and `solarized`, show the mapping
from a palette to the tokens of a scheme.

### Choose a font for each scheme from an editor theme

Open question 12. A scheme can name a font that suits its theme. zuzu
bundles IBM Plex Sans for text (D16), and Noto Color Emoji for an emoji
(D32).

### Pass the bundled fonts to okage

D16. The package `text` embeds IBM Plex Sans, and `text.Fonts` returns the
files. Since phase 6, `Chart.Image` loads them into a renderer of okage
with no font of the system, and sets the family as the sans serif family,
so that an image draws the same font that the layout measured. A character
that IBM Plex Sans and the font of `WithEmoji` lack draws nothing, and the
report names it (D32). The option `WithFonts` of API.md,
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
