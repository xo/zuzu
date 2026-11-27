# Backlog

This document lists work that is known and not done. Each item names the
decision or the document that found it.

A decision is not a backlog item. It goes in
[`decisions/`](decisions/README.md). A question for a maintainer goes at the
end of [`PLAN.md`](PLAN.md), under Open questions for the maintainers. When an
item here is done, delete it, and record in `decisions/` anything that was
decided on the way.

## Phase 7

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

In a swimlane diagram from the left to the right, a title of a lane that
holds HTML does not turn, as a plain title and a title of math do. The
rows of the HTML take no angle.

### Fix what the review page found

`test/cmd/review` draws each source of the corpus and each fixture of the
diagrams with its original renderer and with zuzu, side by side. On
2026-11-12 it showed these differences. Each one is what the page shows, and
its cause is not known yet.

In Vega, a tooltip whose value is the item, such as
`"tooltip": {"signal": "item"}`, shows each field of the item in vega.js.
zuzu draws no such tooltip, and it goes in the report.

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
