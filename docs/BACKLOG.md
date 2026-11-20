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
zuzu did all of its parts, and drew every kind of Mermaid 12.3 that part
11 lists. These are left:
1. A kind of Mermaid that lays out when it is read, such as a sequence, a
   pie, a gantt, a journey, a mindmap, a treemap, an architecture diagram,
   a gitGraph, a block diagram, a timeline or a C4 diagram, measures an
   emoji as nothing, because `WithEmoji` is an option of the render. Its
   labels can be too narrow (6).
2. The SVG writer measures the `limit` of a text without the font of
   emoji (6).
3. A weight or a style in `fontFamily` draws only in the graphs and in the
   sequence. It is an issue in the other kinds (5).
4. `href` and `tooltip` on a table or a cell of an HTML label of DOT are
   issues, because the package `html` of okage keeps no link (10).
5. The wrap of a kind with labels of more than one size in Mermaid uses
   11/14 for all of them (7): a sequence wraps its messages at 16 pixels
   and its actors and notes at 14, and a use case diagram draws its use
   cases at 12 and its actors at 14.


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
2. Make the font of a look opt-in: a caller can give another font, and by
   default a look draws with the font that zuzu tests it with (D32 part 2).
3. A wobble must be the same on each render, so that the output stays
   deterministic.

D35 part 12 says how a caller selects a look: the option `WithLook`, the key
`zuzu.look` of the spec, and `look: handDrawn` of Mermaid.

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
5. A subgraph title with a break, such as `subgraph s ["t<br>u"]`, draws
   its lines in one text, not on two lines.
6. In a color theme of Mermaid, a use case diagram colors each system
   boundary in turn also with `colorScheme: role`. zuzu draws such a
   boundary as a plain panel. With `rotate`, zuzu turns through the 10
   categorical colors of its scheme, and Mermaid through 12 colors.

### Fix what the corpus of mermaid-go-renderer found

The corpus of `github.com/bvolpato/mermaid-go-renderer` at commit 9af8f11
holds 2811 files of Mermaid 12.2. mmdc draws 2807 of them. On 2026-11-12
zuzu drew 2808 of them, and 2732 with no issue. Of the 3 that it does not
draw, one is an example of the documents of Mermaid whose kind is `...`,
one ends in a link with no node, which mmdc does not draw either, and one
is item 9 below.

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
5. A `classDef` of a treemap splits at `\,`. Mermaid reads it as a comma of
   a value, as in a flowchart.
6. A state or a class diagram reports `flowchart.nodeSpacing`, which
   Mermaid reads there.
7. In an ER diagram, a text where only a part is bold or italic, such as
   `*first*Name`, draws plain. A line draws in one font.
8. An outlined label of a sankey stays inside the chart, where Mermaid
   draws it past the left edge.
9. A gantt with `dateFormat ss` and a start of `0` is an error. Mermaid
   reads `0` as a date of 2000 and the end as a time of the day of the
   render. It needs the time of the render (see "Give a chart of Mermaid
   the time of the render").
10. A swimlane diagram does not do `optimizeRanksByCrossings` with
    `ignoreCrossLaneEdges: false`, `automaticLaneOrdering`, or the hops of
    lines. Some of its routes share a line, and the titles of its lanes do
    not turn in a layout LR.
11. The reader of every kind cuts a `%%` in the middle of a line, also
    inside single quotes and after a colon of ZenUML, as in `A->B: 50%%
    off`. In Mermaid each lexer cuts a comment by its own rule: a kind of
    jison cuts `%%` where a token can start, a timeline also cuts a single
    `%`, a kind of langium has a hidden token for `%%`, and a text token,
    such as a message of a sequence or a string, keeps `%%` as text. So
    `flowchart LR %% c` is an error in mmdc, and `sequenceDiagram%% c`
    draws. Each reader handles its comments itself. The railroad kinds
    already do.
12. A run of spaces in a label draws as one space in the PNG. Mermaid keeps
    the spaces.
13. A ZenUML comment with markdown, such as `**bold**`, draws plain, and is
    an issue.
14. The priority line of a kanban card takes the colors of Mermaid, red,
    orange, blue and light blue, and not tokens of `docs/STYLE.md`.


### Draw the math of KaTeX as Mermaid does

D36. zuzu draws math with the math layout of okage v0.19.0. These parts
differ from Mermaid 12.3:

1. okage does not have `\phase` yet. A formula with it draws its source,
   and the report names it. okage plans it next. `\omicron`,
   `\impliedby`, `\eth`, `\Finv`, `\Game` and `\notni` come in the next
   release of okage, and zuzu then moves to it.
2. The browser of Mermaid draws MathML in its own font of math. Its
   formulas are often wider than those of STIX Two Math: `x^2` by about a
   third, and a lone relation such as `\subset` more than twice. zuzu puts
   a thin space between text and a formula, where Mermaid puts none, so a
   label with both is a little wider than its formula and its text.
3. Mermaid sets a message of a sequence diagram with math closer to its
   line than a message of text. zuzu keeps the gap of its own look.
4. `test/cmd/review` and `test/cmd/gallery` draw with no font of math, so
   their labels with math draw the source.

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
2. zuzu has no balance step of the ranks. In dot, a node whose edges in
   and out have the same weight moves to a rank with fewer nodes. So in
   `a->b->c; x->c[weight=0]`, dot puts x in rank 0, and zuzu in rank 1.
   The step changes the ranks of other graphs too, Mermaid included.
3. An edge with a list of colors, such as `color="a:b"`, draws only the
   first color, with no issue. Graphviz draws a line in each color.
4. An edge to a cluster, with `lhead` or `ltail`, whose route is cut at the
   box of the cluster, puts its end labels back at the first place, with no
   check for overlap (`clipEdge` in `graph/nest.go`).
5. The ends of a loop sit 2 or 3 pixels off the outline of an ellipse or a
   circle, because they sit on its bounding box.


### Give a chart of Mermaid the time of the render

A gantt draws no marker of today, and a `dateFormat` with no date, such as
`HH:mm`, puts its tasks on 1970-01-01, because the compile has no time of
the render. A time that the caller gives fixes both.

### Fix the gaps of the Vega-Lite compiler that the charts found

The compilers of the charts work around each of these:

1. `y2: {datum}` is dropped, and `y2: {field}` beside a datum `y` takes the
   value of `y`.
2. An axis has no `labelExpr` and no `titlePadding`. They are issues.
3. An unfilled point draws with a stroke in the color of the background.
4. A rule of Vega-Lite with one item takes the look of a reference, and its
   label writes a time as a number.
5. `x2: {"value": "width"}` is dropped with no issue, and the rect does not
   draw.
6. The axis of an encoding reports `zindex`, `labelFontSize`,
   `labelPadding`, `titleFontSize`, `titlePadding`, `tickSize`, `tickWidth`
   and `domainWidth`. `config.axisX` and `config.axisY` apply some of them.
7. The channel `href` is read and dropped with no issue, and a field of
   the channel `tooltip` only joins the aggregate. Neither becomes an
   encoding of Vega, though the scene now carries both.

### Fix what the review page found

`test/cmd/review` draws each source of the corpus and each fixture of the
diagrams with its original renderer and with zuzu, side by side. On
2026-11-12 it showed these differences. Each one is what the page shows, and
its cause is not known yet.

1. Vega-Lite:
   - A text mark gets no `align`, so it draws from the left. vega-lite.js
     writes `align: center` on 36 of the 42 text marks of the references,
     and the labels inside the bars of `waterfall_chart` are cut.
   - The title of the y axis of `waterfall_chart` names each field, not the
     title "Amount" of the spec.
   - `mark.width` and `mark.height` are dropped with no issue on a mark that
     is not a bar, such as the image of `scatter_image`.
   - The text layer of `bar_layered_weather` compiles `y` with a scale and
     no field.
   - `size: {expr}` of a bar with no y in `bar_bullet_expr_bind` is dropped,
     and the bar fills the height.
   - A legend of `opacity` alone, or of `strokeWidth` alone, draws nothing,
     with no issue.
   - `rangeMin` and `rangeMax` are dropped with no issue on a range that is
     not two values, such as a step or a named range.
   - The text of `arc_radial_histogram` has no `baseline: middle`, which
     vega-lite.js sets.
   - `radiusOffset` and `thetaOffset` of an arc go to Vega, which does not
     have them.
2. Vega:
   - The years of `calendar-view` cover "Jan", as "Read the properties of
     an axis that the runtime drops" says.
   - The fit of `autosize` diverges in `flight-passengers`, because the
     title is wider than the plot. Each pass makes the width smaller by the
     same overflow, 500, 354, 208 and 62, and the x axis keeps two labels.
     The loop is in `NewView` in `zuzu.go` and in `render.go`.
3. Deneb, both in the Vega-Lite compiler:
   - `calendar-heatmap` draws a strip 5374 pixels wide, not the grid of
     seven years. The band scale `child_x` of the facet `cell` reads the
     dataset of the top level, `data_2`, not the dataset of the cell,
     `data_0`. The renaming of the datasets in `vegalite/facet.go` or
     `vegalite/optimize.go` is the likely cause. The compiler also drops
     the `labelFontSize`, `labelColor`, `labelFont` and `labelPadding` of
     the header, the `spacing` of the facet and `config.view.stroke`, with
     no issue. An axis with `format: ""` and a time unit writes a whole
     date, where vega-lite.js writes the format of the time unit.
   - `variance-chart` has no bars of Actual and Forecast. Its shared
     `color` has no field, only a scale with a domain and a range, and a
     legend with `orient: "top"`. zuzu drops the domain and the orient
     with no issue, so the bars get no fill. The rule has no
     `x2: {scale, value: 0}`, so the stems are missing. The bar
     `height: {band: 0.5}` with a `yOffset` goes to the rect, not to the
     group. The conditional `opacity` with no default is dropped.
4. Mermaid, in `usecase/01-relations`:
   - The ellipse of CheckCard is outside the boundary Front desk, but
     covers its top border by 3 pixels, and the label «include» touches
     that border. A node that is not in a cluster, in the rank of the nodes
     of a cluster, gets no margin from the cluster.
   - The back edge from Librarian to Member crosses the long edge from
     Member to Return near Member, because of where the two edges join
     Member. The layout can avoid it.
5. The originals: `contour-plot`, `density-heatmaps`,
   `labeled-scatter-plot` and `word-cloud` need the npm package `canvas`,
   which `test/oracle/package.json` does not install. vega.js does not know
   the projection `airy` of `projections`.

### Do the work of D35

D35 records the answers of the maintainers of 2026-11-20. Part 4 is done
for the axes. An encode of the place of a part of a legend, such as the
`x` of its labels, still goes in the report. vega.js places the labels and
the symbols in a group for each entry, and then places the entries from
their bounds after the encode. The legend of zuzu is one flat group, laid
out before any encode, in `layout/legend.go`. To apply such an encode, the
layout of the legend reads the encode, or gives each entry a group of its
own. No spec of the corpus encodes the place of a part of a legend.

### Read the properties of an axis that the runtime drops

The runtime drops the properties `labelAlign`, `titleX`, `titleY`,
`titleAlign` and `titleAngle` of an axis with no issue. `vega.Axis` reads
`LabelAlign` and nothing uses it, and `guideProps` and `styleGuide` ignore
the others. So the years of `calendar-view` still cover "Jan".

### Fix the bounds of a text and the place of an axis title

The bounds of a path add the origin of the item, `it.X` and `it.Y`, so a
path drawn away from its origin makes an autosized view larger than the
drawing (`itemBounds` in `layout/layout.go`). vega.js does not add the
origin.

The bounds of a text ignore its `lineHeight`, so a text of more lines with
a `lineHeight` above its font size is cut at the bottom of a chart. The
title of the axis at the bottom has a fixed place, so it covers labels
that turn, such as with `labelAngle: 45`.

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
commit and writes a scheme for each, with its license file. D33 says that a
scheme is a Go value in `theme/theme.go`, so the tool writes Go values. The
schemes `catppuccin` and `solarized`, which exist only in
`docs/targets/gen-targets.mjs`, move into the package `theme` too. The first
sources are the styles of chroma and the editor themes that transit D98
lists. Each theme with a license that MIT does not allow stays out. The two
prototypes in the targets, `catppuccin` and `solarized`, show the mapping
from a palette to the tokens of a scheme.

### Bundle the fonts that a scheme can name

D35 part 11. zuzu bundles a monospace, a humanist sans, a serif and a
rounded sans, each in an optional package as `text/emoji` is, and a scheme
can name one of them. Open question 30 chooses each font. Each package
embeds its font in WOFF2 with its license, as `text` does (D34).

### Pass the bundled fonts to okage

D16. The package `text` embeds IBM Plex Sans as one collection in WOFF2,
and `text.Fonts` returns it, decoded into a TrueType collection (D34).
Since phase 6, `Chart.Image` loads it into a renderer of okage with no font
of the system, and sets the family as the sans serif family, so that an
image draws the same font that the layout measured. A character that IBM
Plex Sans and the font of `WithEmoji` lack draws nothing, and the report
names it (D32). The option `WithFonts` of API.md, which gives other fonts,
and a fallback to the fonts of the system, are not done.

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
