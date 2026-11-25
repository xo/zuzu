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
11 lists. One part is left: the SVG writer measures the `limit` of a text
without the font of emoji (6).

### Bring the look like Excalidraw closer to Mermaid

1. The hachure of rough-go fills one polygon. zuzu joins the subpaths of a
   path with bridges, so a hole stays empty, but subpaths that overlap in
   the same direction, which the rule nonzero of SVG fills, draw as the
   rule evenodd.

### Follow what zuzu offered to elk-go

On 2026-11-25 zuzu offered these to d2lang/elk-go (D58 part 16), checked
against its commit `75f6eb7` and ELK.js 0.12.1. When upstream takes one,
update the copy and drop its change or its work-around.

1. Pull request 10 takes seed 1 when `randomSeed` is 0, which is change 3
   of `elk/internal/elkgo/README.md`. Change 4, no JSON, is not offered,
   because upstream moved the codec of JSON out of the typed request in
   its pull request 6.
2. Issue 11: the label of a cluster with `[H_CENTER, V_TOP, INSIDE]`
   is not at the top. The adapter works around it with `reserveTitles`.
3. Issue 12: the ports of a side with `spacing.portsSurrounding` spread a
   little narrower than in ELK.js.
4. Issue 13: `spacing.nodeNode` of a cluster has no effect when
   `spacing.baseValue` is set. This is the cause of the larger clusters
   that the radial graph of the adapter showed. With the algorithm radial
   alone, ELK.js gives a cluster 20 by 20, smaller than its children, and
   elk-go fits the cluster to them. zuzu did not report that.

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

### Draw more of Mermaid

In the mode `compact` of a gantt whose tasks all end before the first
start, the names of the tasks do not take the backward axis into account.

### Fix what the review page found

`test/cmd/review` draws each source of the corpus and each fixture of the
diagrams with its original renderer and with zuzu, side by side. On
2026-11-12 it showed these differences. Each one is what the page shows, and
its cause is not known yet.

1. Vega:
   - An expression of an encoding reads `item` as the scene graph of Vega
     gives it: `item.datum`, and `item.mark` with its marktype, name, role
     and group. zuzu does not give the properties that the encodings of the
     item set, such as `item.x`, because it does not apply them in the
     order of the spec. It does not give `item.bounds`, `item.mark.items`,
     the items and the bounds of a group item, `item.mark` in an encoding
     of a guide or of the root group, or a tooltip that shows the item.
     Each goes in the report.
2. The originals: `contour-plot`, `density-heatmaps`,
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

### Let a scheme name a font

D35 part 11 and D58 part 1. zuzu bundles a monospace, a humanist sans, a
serif and a rounded sans, in `text/mono`, `text/humanist`, `text/serif`
and `text/rounded`. A caller gives one to a chart with `WithFont`, and
`Chart.Image` loads it into okage with IBM Plex Sans. These are left:
1. A scheme names one of the four kinds of font, and draws in IBM Plex
   Sans when the caller has not given that font. No scheme names a font
   yet, so the name of a font in a scheme, and how it reaches the render,
   wait for the first scheme that needs one.
2. The option `WithFonts` of API.md, which gives fonts in a file system,
   such as a font that zuzu does not bundle.
3. A fallback to the fonts of the system. A character that IBM Plex Sans,
   the font of `WithFont` and the font of `WithEmoji` lack draws nothing,
   and the report names it (D32).
