# D48. Dark colors, annotations, ranged areas and the start of the ramp

Status: Decided. Amends D12, D35 and D37.

On 2026-11-22 a census of the review page, every Vega, Deneb and Vega-Lite
spec beside vega.js, found charts that the look made hard to read. The
maintainers answered four questions.

1. In the dark mode, a color that a spec names and that is a gray near
   black or near white, such as `black`, `#333` or `#eee`, flips its
   lightness, so that it reads on the dark background. A color with a hue,
   such as `red` or `steelblue`, stays as the spec names it. The light mode
   draws each color as named.
2. The look adds a name at the end of each line and a label of the high
   point of an area only where they fit: end labels for a few series, at
   most about eight, when the spec draws no text of its own and the labels
   do not overlap, and the high label only in one area that is not in a
   facet. Otherwise the look adds neither.
3. An area with an end, `y2` or `x2`, draws as a flat tint of its color,
   with no line along its top and no gradient, so that a band reads as a
   range. Only an area from zero keeps the gradient and the top line.
4. The sequential ramp of the theme starts at a visible tint of its first
   color, not at the background, so that the lowest value of a scale shows
   in both modes. A named scheme is not affected (D46). This amends D35
   part 3 and D37 part 1.
5. A property of a mark that a spec sets wins over the look, as D46 says
   of axes and schemes: an opacity, a gradient fill, a corner radius, a
   stroke dash, and the fill of the view of `config.view`.

The rules that follow are in `docs/STYLE.md`. A gray near black or near
white has channels that differ by at most 0.1 of the full range and a
lightness in HSL of at most 0.3 or at least 0.7. Its lightness flips from l
to 1 − l. A color that the spec writes flips: a value of an encoding or of a
condition, a value of a range, a property of a guide, the config, a stop of
a gradient, and a color in the data with no scale. A color that an
expression computes stays, because the expression can read a scale whose
range already flipped. A named scheme stays (D46). A chart on a light
background, and a diagram of DOT or Mermaid, keep their colors. A line
below 0.5 opacity is context and gets no name. The look counts series per
view, by the lines that it draws. A view names no line when it has more
than eight series, when two lines of one color have different names, when
the spec draws text, or when a name moves more than 12. Then the legend
draws. An area whose scale leaves out zero is a ranged area too. An area
whose spec sets an opacity is a flat fill at that opacity, with no gradient
and no line on top, and no area of a spec that draws text gets the label of
its high. The ramp starts at 0.4 of the way from the background to the
first color, because 0.2 left the lowest value of an ordinal scale near
white.
