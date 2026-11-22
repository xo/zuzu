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

A tooltip comes with the browser, in phase 8. A Vega-Lite spec that
vega-lite.js compiled, as the gallery draws it with `-vl=false`, still
names `#4c78a8`.

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
added. `pancake-spider-chart` differs for this reason, and so do the rows of
`facet_bullet` when zuzu compiles it, where zuzu also computes the field of
each layer in the dataset of that layer and not in a shared parent.
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
1. The SVG writer measures the `limit` of a text without the font of
   emoji (6).
2. A weight or a style in `fontFamily` draws only in the graphs and in the
   sequence. It is an issue in the other kinds (5).
3. `href` and `tooltip` on a table or a cell of an HTML label of DOT are
   issues, because the package `html` of okage keeps no link (10).
4. The wrap of a kind with labels of more than one size in Mermaid uses
   11/14 for all of them (7): a sequence wraps its messages at 16 pixels
   and its actors and notes at 14, and a use case diagram draws its use
   cases at 12 and its actors at 14.


### Draw the look like xkcd

zuzu draws the standard look and the look like Excalidraw, in the pass of
`internal/sketch` (D39, D40). The look like xkcd, like the look that
`plt.xkcd()` of matplotlib gives, waits for a font under the OFL, which
open question 36 asks for (D40). Its lines wobble in long, smooth curves
that follow the line, with no double stroke. It is code of zuzu in the
same pass (D39 part 8), with a constant of `theme.Look` and a font of
`WithLookFont`.

### Bring the look like Excalidraw closer to Mermaid

1. The hachure of rough-go fills one polygon. zuzu joins the subpaths of a
   path with bridges, so a hole stays empty, but subpaths that overlap in
   the same direction, which the rule nonzero of SVG fills, draw as the
   rule evenodd.
2. Only `zuzu.lookSeed` and `handDrawnSeed` set the seed. No option of
   the caller sets it.
3. No target SVG shows the look, and `docs/targets/gen-targets.mjs` does
   not draw it (hard rule 13).
4. Vega-Lite turns the labels of a discrete x axis by 270 degrees, and
   the compiler of zuzu passes it on (`axisOf` in `vegalite/view.go`), in
   the standard look and in the look like Excalidraw alike, as in
   `testdata/look/bar.vl.json`. Drawing them flat when each label fits
   its band is a change to the standard look and needs a rule in
   `docs/STYLE.md` first.
5. The end state of a state diagram, a ring with a dot of 14, draws as a
   scribble, because a hachure with a gap of 2 cannot show a ring with a
   gap of 2.

### Finish the layouts of dagre and ELK

D45 makes `zuzu.dagre` the layout of `layout: dagre`, and the engine of
ELK of `github.com/xo/zuzu/elk` the layout of a graph of Mermaid that sets
no layout or an ELK layout, when the caller makes it available with
`zuzu.WithLayoutEngine`. On 2026-11-22 the mean error of the places of the
nodes over 88 fixtures was 0.016 for `zuzu.dagre` against mmdc with
`layout: dagre`, and 0.022 for `zuzu.elk` against mmdc with ELK, against
0.052 and 0.044 for `zuzu.layered`.

1. `cmd/mmdcref` does not add the translate of a nested `g class="root"`,
   which Mermaid draws for a cluster that dagre lays out on its own. The
   references of dagre were measured from SVG files that a script
   flattened first.
2. Add the passes of Mermaid that the adapter of ELK leaves out: turning
   back the edges that return into a cluster (orientFeedbackEdges), pinning
   the first node of a cycle (keepEntryNodeOnTop and the cyclic-entry
   constraint), ordering the edges by nesting, and the room for the markers
   at the ends of an edge.
3. The engine of ELK gives ELK the clusters first and then the nodes, as
   Mermaid gives a flowchart. Mermaid gives the composite states and the
   notes of a state diagram among the states, in the order of the source,
   and `graph.LayoutInput` does not hold that order. The state diagrams
   have the worst error of `zuzu.elk`: 0.738 for `40-state-notes`, 0.305
   for `39-state-concurrency` and 0.190 for `59-wrap-state`.
4. The property `algorithm` of a subgraph, such as
   `grp@{ algorithm: elk.box }`, does not reach the engine of ELK, because
   `graph.LayoutCluster` has no field for it.
5. The engine of ELK leaves out the ports of an edge, which it names as
   unsupported, and gives the labels at the ends of an edge no place in the
   layout. `graph.Finish` places them.
6. Mermaid 12.3 also gives the subgraphs of an ER diagram to its layouts in
   reverse. The reader of the ER diagram does not set `ReverseClusters`.

### Offer the fixes of the copy of elk-go upstream

1. Offer changes 3 (fixed seeds, no global `math/rand`) and 4 (no JSON) of
   `elk/internal/elkgo/README.md` to d2lang/elk-go.
2. Report that elk-go places a cluster label with
   `[H_CENTER V_TOP, INSIDE]` at the left and halfway down in DOWN and UP,
   and keeps no room for it in any direction, where ELK.js 0.12.0 places it
   at the top in the middle and moves the children down. The adapter works
   around it, but not for a title wider than its cluster in DOWN or UP (up
   to 92 px) or a long title of a cluster whose frame draws none.
3. Report that ports spread with `spacing.portsSurrounding` sit up to 0.67
   px from where ELK.js puts them, and nodes up to 1 px.
4. Report that a cluster with the radial algorithm is larger than in
   ELK.js, and its nodes move up to 35 px.

### Draw the parts of Mermaid that draw another way

1. The shape `person` and the shapes of BPMN that put the label under the
   shape, such as `bpmn-start` and `bpmn-gateway`, draw as rounded
   rectangles. zuzu always puts a label in the middle of its node.
2. A subgraph without a direction, inside a subgraph with a direction,
   lays out in the direction of the subgraph that holds it. Mermaid 12
   draws some of these in another direction. In a `flowchart LR`, a
   subgraph inside a subgraph with `direction TB` draws LR in Mermaid.
3. In a color theme of Mermaid, a use case diagram colors each system
   boundary in turn also with `colorScheme: role`. zuzu draws such a
   boundary as a plain panel. With `rotate`, zuzu turns through the 10
   categorical colors of its scheme, and Mermaid through 12 colors.

### Fix what the corpus of mermaid-go-renderer found

The corpus of `github.com/bvolpato/mermaid-go-renderer` at commit 9af8f11
holds 2811 files of Mermaid 12.2. mmdc draws 2807 of them. On 2026-11-12
zuzu drew 2808 of them, and 2732 with no issue. Of the 3 that it did not
draw, one is an example of the documents of Mermaid whose kind is `...`,
and one ends in a link with no node, which mmdc does not draw either. The
third, a gantt with `dateFormat ss` and a start of `0`, draws since
2026-11-22.

1. A flowchart does not read properties after brackets, as
   `a["A"]@{ shape: cyl }`, and it reads `@{ ... }` over more lines with
   the commas of one line. The agentflow diagram reads both.
2. Mermaid moves a node into the innermost subgraph that names it, at the
   `end` of that subgraph. zuzu keeps the node in the first subgraph that
   names it.
3. In a layout LR with subgraphs inside subgraphs, the title of an outer
   subgraph covers the border of an inner one.
4. In an ER diagram, a text where only a part is bold or italic, such as
   `*first*Name`, draws plain. A line draws in one font.
5. An outlined label of a sankey stays inside the chart, where Mermaid
   draws it past the left edge.
6. A swimlane diagram does not do `optimizeRanksByCrossings` with
   `ignoreCrossLaneEdges: false`, `automaticLaneOrdering`, or the hops of
   lines. Some of its routes share a line, and the titles of its lanes do
   not turn in a layout LR.
7. The reader of every kind cuts a `%%` in the middle of a line, also
   inside single quotes and after a colon of ZenUML, as in `A->B: 50%%
   off`. In Mermaid each lexer cuts a comment by its own rule: a kind of
   jison cuts `%%` where a token can start, a timeline also cuts a single
   `%`, a kind of langium has a hidden token for `%%`, and a text token,
   such as a message of a sequence or a string, keeps `%%` as text. So
   `flowchart LR %% c` is an error in mmdc, and `sequenceDiagram%% c`
   draws. Each reader handles its comments itself. The railroad kinds
   already do.
8. A run of spaces in a label draws as one space in the PNG. Mermaid keeps
   the spaces.
9. A ZenUML comment with markdown, such as `**bold**`, draws plain, and is
   an issue.


### Draw the math of KaTeX as Mermaid does

D36. zuzu draws math with the math layout of okage v0.20.1, which added
the seven commands that zuzu waited for, `\phase` among them. These parts
differ from Mermaid 12.3:

1. The browser of Mermaid draws MathML in its own font of math. Its
   formulas are often wider than those of STIX Two Math: `x^2` by about a
   third, and a lone relation such as `\subset` more than twice. zuzu puts
   a thin space between text and a formula, where Mermaid puts none, so a
   label with both is a little wider than its formula and its text.
2. Mermaid sets a message of a sequence diagram with math closer to its
   line than a message of text. zuzu keeps the gap of its own look.

### Draw more of Mermaid

1. The title of a subgraph that is wider than its nodes draws past the box
   of the subgraph. Mermaid makes the box wider. Only a subgraph that lays
   out on its own, in `graph/nest.go`, reads the width of its title.
2. A chart of `xyChart` reports `labelPadding` and `titlePadding` of an
   axis, and `plotReservedSpacePercent`. The compile of `xyChart` does not
   pass `labelPadding` and `titlePadding` to the axis yet, though the
   runtime reads them.
3. The title of a gantt with sections starts at the names of the tasks, to
   the right of the names of the sections, because a title takes the bounds
   of the axes.
4. In a `flowchart LR` with two subgraphs that no edge links, the box of the
   second subgraph covers the box of the first by 3 pixels.
5. Outside the time axis, Mermaid draws the marker of today of a gantt in
   the margin or past the edge of the image. zuzu draws no marker there.
6. A task of a gantt that ends before it starts draws a bar from its end to
   its start, and its start makes the time axis longer. Mermaid draws no bar
   for it, and its axis runs from the first start to the last end. When
   every task ends before the first start, zuzu runs the axis backward, as
   Mermaid does. In the mode `compact`, the names of the tasks then do not
   take the backward axis into account.
7. A label with a word that is wider than `minNodeWidth` in the font of
   mmdc can be narrower than the least width in zuzu, because zuzu measures
   it with IBM Plex Sans. The node then takes the least width in zuzu, and
   in Mermaid it is as wide as its label.

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


### Fix the gaps of the Vega-Lite compiler that the charts found

The compilers of the charts work around each of these:

1. `y2: {datum}` is dropped, and `y2: {field}` beside a datum `y` takes the
   value of `y`.
2. An unfilled point draws with a stroke in the color of the background.
3. The label of a reference line writes a time as a number.
4. `x2: {"value": "width"}` is dropped with no issue, and the rect does not
   draw.
5. The axis of an encoding reports a condition on a property, such as a
   `gridColor`, a `gridDash` or a `tickDash` that changes with the value of
   the tick, because the compiler does not turn a condition into an encode
   of the axis.
6. The channel `href` is read and dropped with no issue, and a field of
   the channel `tooltip` only joins the aggregate. Neither becomes an
   encoding of Vega, though the scene now carries both.

### Fix what the review page found

`test/cmd/review` draws each source of the corpus and each fixture of the
diagrams with its original renderer and with zuzu, side by side. On
2026-11-12 it showed these differences. Each one is what the page shows, and
its cause is not known yet.

1. Vega-Lite:
   - The title of the y axis of `waterfall_chart` names each field, not the
     title "Amount" of the spec.
   - `mark.width` and `mark.height` are dropped with no issue on a mark that
     is not a bar, such as the image of `scatter_image`.
   - `size: {expr}` of a bar with no y in `bar_bullet_expr_bind` is dropped,
     and the bar fills the height.
   - A legend of `opacity` alone, or of `strokeWidth` alone, draws nothing,
     with no issue.
   - `rangeMin` and `rangeMax` are dropped with no issue on a range that is
     not two values, such as a step or a named range.
   - `radiusOffset` and `thetaOffset` of an arc go to Vega, which does not
     have them.
   - The title of the rows of `trellis_area_seattle` overlaps its labels,
     which a negative titlePadding brings close. The layout of the grid
     places the title of the rows without the width of the headers.
   - `shape.RuleEnd` reads an end at 0 as an end that the item does not
     set. The runtime turns such a rule of a spec around, but a rule that
     layout builds with an end at 0 still draws from its start.
   - The compiler drops `config.text` and `config.mark`, except their
     `align` and `baseline`, which it writes on the mark. vega-lite.js moves
     `config.text` into `config.style.text`. So the `dx` and `dy` of the
     config of `layer_line_co2_concentration` are lost, with no issue.
   - The title of an axis in a facet can cover a header or the title of
     the axis of the next cell, as in `bar_column_fold`,
     `trellis_cross_sort`, `trellis_cross_sort_array`,
     `facet_cross_independent_scale`, `config_numberFormatType_test` and
     `trellis_column_year`. The title moves clear only of the labels of the
     axes of its own view (D46), and a header or the axis of another cell
     is in another group.
   - The title of an axis of a layer joins the titles of the layers, such
     as `x, Median of median_x_by_a` in `trellis_cross_sort`, where
     vega-lite.js writes `x`.
   - The rules of `wheat_wages` do not draw. Their years are strings in
     inline values, which the linear scale of zuzu does not turn into
     numbers, as the scale of vega.js does. vega-lite.js also parses the
     field `year` of `wheat.json` as a number, and zuzu does not.
   - The dataset `source_0` of `bar_heatlane` has 11 rows, where vega.js
     has 10, and its ordinal color scale with a scheme has no
     `interpolate: hcl`.
   - The x scale of `rect_mosaic_labelled` has `zero: false`, where
     vega-lite.js writes `true`. zuzu counts the x of a rect with an x2 as
     a dimension of the bar.
   - The legend of a quarter in `line_quarter_legend` has no
     `type: symbol`, which vega-lite.js writes.
   - The compiler passes on `config.point`, `config.circle` and the config
     of every other mark only for the opacity of a point.
   - The diverging ramp of the theme runs through `grid`, so a value near
     the middle almost disappears, as in `trail_comet`.
   - A bar whose scale leaves out zero still runs from scale(0), which
     vega-lite.js gives as the bottom of the domain.
   - The compiler drops `config.numberFormat`, `config.numberFormatType`
     and `config.customFormatTypes` with no issue, as in
     `config_numberFormatType_test`.
2. Vega:
   - The years of `calendar-view` cover "Jan", as "Read the properties of
     an axis that the runtime drops" says.
   - An axis reports `tickBand`, `tickOffset`, `tickRound` and
     `tickExtra` and does not draw them.
   - The interrupted and polyhedral projections of D52 leave the meridian
     at ±180° and a strip of 1e-6 radians along each cut out of their clip.
     d3-geo-projection draws the meridian at -180° along the west edge of
     the first lobe or face. zuzu does not, because the same place on the
     sphere cannot belong to both edges of one ring.
   - The parameter `lobes` of an interrupted projection reads only the
     longitudes of the three points of each lobe. A latitude other than 0
     and ±90 changes nothing, and the report says so.
   - If a spec sets `clipAngle` on `armadillo` or `berghaus`, zuzu clips
     with that circle and draws the circle as the sphere. d3-geo-projection
     keeps the torus or the star as the sphere. No spec of the corpus sets
     it.
   - A legend reports these properties and does not draw them:
     `cornerRadius`, `fillColor`, `strokeColor`, `padding`, `gridAlign`,
     `labelAlign`, `labelBaseline`, `labelOffset`, `labelOverlap`,
     `labelSeparation`, `symbolDashOffset`, `symbolOffset`,
     `symbolBaseFillColor`, `symbolBaseStrokeColor`, `titleAnchor`,
     `titleAlign`, `titleBaseline`, `titleLimit`, `titleLineHeight`,
     `titleOrient`, `titlePadding`, `tickCount`, `tickMinStep`,
     `gradientOpacity`, `gradientStrokeColor` and `gradientStrokeWidth`.
   - An expression of an encoding reads `item` as the scene graph of Vega
     gives it: `item.datum`, and `item.mark` with its marktype, name, role
     and group. zuzu does not give the properties that the encodings of the
     item set, such as `item.x`, because it does not apply them in the
     order of the spec. It does not give `item.bounds`, `item.mark.items`,
     the items and the bounds of a group item, `item.mark` in an encoding
     of a guide or of the root group, or a tooltip that shows the item.
     Each goes in the report.
   - A background that a signal gives does not decide the gray flip of the
     dark mode (D48).
   - A dark mode over a light background of the spec keeps the neutrals of
     the dark mode for the guides, as in `point_background`.
   - A layout of a grid aligns each row and each column on its own. It
     reports the align `none`, the align `all` of cells of more than one
     size, and `center`.
   - The title of a group follows the properties that its spec sets, but
     not the config title or the config of the style `group-title`.
   - In `deneb/gantt-chart`, vega.js cuts the names of the tasks, such as
     "Requirements …", at the limit of 130, and zuzu does not.
   - In `deneb/coronation-arc`, the rows of the legend are closer together
     than the `rowPadding` of 10 that vega.js draws.
   - zuzu moves the content of `deneb/pancake-spider-chart` inside its
     padding by 62 pixels more than vega.js does, with autosize `none` and
     `"contains": "padding"`.
3. Mermaid, in `usecase/01-relations`:
   - The ellipse of CheckCard is outside the boundary Front desk, but
     covers its top border by 3 pixels, and the label «include» touches
     that border. A node that is not in a cluster, in the rank of the nodes
     of a cluster, gets no margin from the cluster.
   - The back edge from Librarian to Member crosses the long edge from
     Member to Return near Member, because of where the two edges join
     Member. The layout can avoid it.
4. The originals: `contour-plot`, `density-heatmaps`,
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

### Write an infinite number as vega.js does

`format(1000/0, 'd')` writes `+Inf`, and vega.js writes `∞`. So
`histogram_nonlinear` shows `+Inf` on its x axis.

### Read the band position of an axis from its config

The runtime does not read `bandPosition` from `config.axisBand`. The specs
`box-plot`, `violin-plot`, `error-bars` and `budget-forecasts` set it
there.

### Draw a text of more lines as vega.js draws it

The writers move the first line of a text of more lines up, by the line
height for each line after the first, for the baselines `alphabetic` and
`bottom`, and by half of that for `middle` (`multiline` in
`internal/shape/text.go`). vega.js does not. It puts the first line on
its baseline and the other lines below it, for each baseline. So the
source note of `sankey-chart` sits 16 pixels higher than in vega.js. The
bounds of a text follow the writers. The diagrams use texts of more lines,
so a change to the writers must check them too.

### Fix the text of the direct renderer

A rotated text whose mark holds a glyph that the font lacks, such as ❤,
also draws a mirrored copy of each text. `dx` and `dy` on a rotated text
move it on the page, not along the text as in Vega.

### Give the look a style of a text on a color

A text on a leaf of a treemap takes the style `zuzu.edgeLabelBox` for the
color of the background. A style of text on a color is cleaner.

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
