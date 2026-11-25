# D57. zuzu draws no look like xkcd, and phases 0 and 3 are done

Status: Decided. Amends D22, D32, D39 and D40.

On 2026-11-25 the maintainers set the order of the work that is left: clear
the items of phase 3, then work the items of phase 7 by how much they
change what a source draws, compatibility and looks first.

1. The look like Excalidraw is the only look drawn by hand. zuzu draws no
   look like xkcd. This replaces the parts of D32, D39 part 8 and D40 part 3
   that plan one. Open question 36, which asked for its font, closes.
   `theme.Look` keeps its two values, `LookStandard` and `LookExcalidraw`.
2. Phase 0 is done. The test of D22 that reads the documents for the names
   of the maintainers keeps a list of names that a maintainer adds to by
   hand. That is how it works, and not a gap.
3. The item of phase 3 that draws the rest of the marks in the look of
   zuzu closes. A tooltip draws in the browser of phase 8 (D19). A Vega
   spec that vega-lite.js compiled names the colors of the config of
   Vega-Lite, such as `#4c78a8`, as values, and a color that a spec sets
   wins over the look (D48 part 5). So the gallery keeps that color with
   `-vl=false`, and zuzu draws its own colors when it compiles the
   Vega-Lite itself.
4. Phase 3 is done. The grid layout reads every property of a layout of
   Vega, as its documents say: `align`, `bounds`, `center`, `columns`,
   `padding`, `offset`, `headerBand`, `footerBand`, `titleBand` and
   `titleAnchor`. `center` centers each cell in its row and its column, as
   the documents say, where vega.js 6.4 centers only the cells of the first
   row and the first column. Each row keeps room for its header above and
   below its cells, as docs/STYLE.md says, so that the title of the y axis
   above the plot does not meet the row before it.
5. vega.js gives a derived dataset copies of the rows that its transforms
   change, and does not share them with its source, as its output shows
   for every transform. zuzu copies them too. This replaces D32 part 14,
   which said that the rows are shared.
6. A domain sorted with `sort: true` that mixes strings and numbers sorts
   as V8 sorts it, with the comparator of Vega, so its order is the order
   of vega.js.
