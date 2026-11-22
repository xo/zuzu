# D46. Axes, ticks and named schemes follow Vega more closely

Status: Decided. Amends D12 and D35. Amended by D51.

On 2026-11-22 the maintainers looked at the review page and found charts
that read as broken: axes with no line, a top value with no grid line,
titles over labels and a heatmap in the wrong colors. Some came from the
look of D12. The maintainers answered four questions.

1. An axis draws its domain line and its ticks where Vega draws them: by
   default, and as `domain`, `ticks` and `tickSize` of the axis or of its
   config say. They draw in the colors and the widths of zuzu. This amends
   D12 and rules 1 to 3 of "Axes and grids" in `docs/STYLE.md`.
2. An axis keeps the few labels of the look, but it chooses a step that
   lands on both ends of a nice domain, so that the top and the bottom of a
   scale always get a label and a grid line. A `tickCount` or `values` of
   the spec still wins.
3. The title of an axis stays at the end of the axis, in the look of zuzu.
   It moves clear of the labels, on an axis at the top or at the right too,
   so that it never covers a label. `titleX`, `titleY`, `titleAngle` and
   `titleAnchor` still win.
4. A scheme that a spec names, such as `viridis`, `blues` or `tableau10`,
   draws in the colors of that scheme. A scale that names no scheme keeps
   the colors of the theme. This amends parts 1 to 3 of D35. A named
   scheme draws in the same colors in both modes, as in vega.js, and
   `count`, `extent` and `reverse` apply as in vega.js. Parts 1 and 2 of
   D35 no longer apply, and part 3 applies only to a range that names no
   scheme. A name that Vega does not have is an issue.
5. A property of a legend that the spec or its config sets wins over the
   look: `clipHeight`, `symbolLimit`, `labelLimit`, `symbolType`,
   `symbolSize`, the paint of the symbols, `rowPadding`, `columnPadding`,
   `gradientLength`, `gradientThickness`, `columns`, `direction`,
   `offset`, `legendX` and `legendY`. The symbol of an entry takes the
   shape, the fill and the stroke of the marks that the legend names. A
   legend of sizes alone draws as nested circles only when each value
   keeps its circle and its label and the spec sets none of these
   properties. "Legends" in `docs/STYLE.md` gives the rules.
