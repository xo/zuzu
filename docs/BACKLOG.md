# Backlog

This document lists work that is known and not done. Each item names the
decision or the document that found it.

A decision is not a backlog item. It goes in
[`decisions/`](decisions/README.md). A question for a maintainer goes at the
end of [`PLAN.md`](PLAN.md), under Open questions for the maintainers. When an
item here is done, delete it, and record in `decisions/` anything that was
decided on the way.

## Phase 7

### Do the work of D32

D32 records the answers of the maintainers of 2026-11-12. On 2026-11-12
zuzu did all of its parts, and drew every kind of Mermaid 12.3 that part
11 lists. These are left:
1. The SVG writer measures the `limit` of a text without the font of
   emoji (6).
2. A weight or a style in `fontFamily` draws only in the graphs and in the
   sequence. It is an issue in the other kinds (5).

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

### Fix what the corpus of mermaid-go-renderer found

The corpus of `github.com/bvolpato/mermaid-go-renderer` at commit 9af8f11
holds 2811 files of Mermaid 12.2. mmdc draws 2807 of them. On 2026-11-12
zuzu drew 2808 of them, and 2732 with no issue. Of the 3 that it did not
draw, one is an example of the documents of Mermaid whose kind is `...`,
and one ends in a link with no node, which mmdc does not draw either. The
third, a gantt with `dateFormat ss` and a start of `0`, draws since
2026-11-22.

1. In an ER diagram, a cell of the attributes of an entity draws in one
   font, so a part of it in bold or italic, such as `*first*Name`, draws
   plain. A record has one font for each cell. The name of an entity with
   no attributes and the label of a relation draw their runs.
2. A swimlane diagram does not do `optimizeRanksByCrossings` with
   `ignoreCrossLaneEdges: false`, `automaticLaneOrdering`, or the hops of
   lines. Some of its routes share a line, and the titles of its lanes do
   not turn in a layout LR, as mmdc turns them.
3. The kinds that read a comment by the rule of the flowchart accept
   `flowchart LR %% c`, which mmdc rejects.

### Draw the math of KaTeX as Mermaid does

D36. The browser of Mermaid draws MathML in its own font of math. Its
formulas are often wider than those of STIX Two Math, which zuzu draws
with the math layout of okage: `x^2` by about a third, and a lone
relation such as `\subset` more than twice.

### Draw more of Mermaid

1. In the mode `compact` of a gantt whose tasks all end before the first
   start, the names of the tasks do not take the backward axis into
   account.
2. A label with a word that is wider than `minNodeWidth` in the font of
   mmdc can be narrower than the least width in zuzu, because zuzu measures
   it with IBM Plex Sans. The node then takes the least width in zuzu, and
   in Mermaid it is as wide as its label.

### Fix what the review page found

`test/cmd/review` draws each source of the corpus and each fixture of the
diagrams with its original renderer and with zuzu, side by side. On
2026-11-12 it showed these differences. Each one is what the page shows, and
its cause is not known yet.

1. Vega-Lite:
   - The dataset `source_0` of `bar_heatlane` has 11 rows, where vega.js
     has 10. vega-lite.js moves the filter of invalid values of its two
     layers into `source_0`. zuzu keeps a filter in each layer, because
     merging such filters in every spec makes 7 other specs differ.
2. Vega:
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
   - An expression of an encoding reads `item` as the scene graph of Vega
     gives it: `item.datum`, and `item.mark` with its marktype, name, role
     and group. zuzu does not give the properties that the encodings of the
     item set, such as `item.x`, because it does not apply them in the
     order of the spec. It does not give `item.bounds`, `item.mark.items`,
     the items and the bounds of a group item, `item.mark` in an encoding
     of a guide or of the root group, or a tooltip that shows the item.
     Each goes in the report.
3. The originals: `contour-plot`, `density-heatmaps`,
   `labeled-scatter-plot` and `word-cloud` need the npm package `canvas`,
   which `test/oracle/package.json` does not install. vega.js does not know
   the projection `airy` of `projections`. Without `canvas`, vega.js also
   guesses the width of a text as 0.8 of its size for each character, so
   it cuts a text at a limit where zuzu draws it whole, as the names of
   the tasks of `deneb/gantt-chart`, and makes the columns of
   `deneb/pancake-spider-chart` wider.

### Do the work of D35

D35 records the answers of the maintainers of 2026-11-20. Part 4 is done
for the axes. An encode of the place of a part of a legend, such as the
`x` of its labels, still goes in the report. vega.js places the labels and
the symbols in a group for each entry, and then places the entries from
their bounds after the encode. The legend of zuzu is one flat group, laid
out before any encode, in `layout/legend.go`. To apply such an encode, the
layout of the legend reads the encode, or gives each entry a group of its
own. No spec of the corpus encodes the place of a part of a legend.

### Draw a color emoji on a turned text

With `WithEmoji`, a color emoji in a turned text draws upright in the
direct renderer, through `RenderColorGlyphs` of okage.

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
