# D42. The options and the rules of the look like Excalidraw

Status: Decided. Amends D24 and D40. Amended by D47.

The work of D39 and D40 on the look like Excalidraw made these choices.
The maintainers accepted them on 2026-11-22, with the changes of parts 3
and 8.

1. `theme.Look` is a type of string, with `LookStandard` (`standard`) and
   `LookExcalidraw` (`excalidraw`). `zuzu.look` takes the same names, and
   `zuzu.lookSeed` is a whole number. The compiler of Vega-Lite passes both
   keys on.
2. `WithLook(theme.Look)` and `WithLookFont(theme.Look, *text.Fallback)`
   are shared options. The font is an option of its own, because a source
   can ask for the look by itself, such as `look: handDrawn`. It takes the
   look, so that each look drawn by hand gets its own font. This amends
   D24.
3. A render with a look drawn by hand and no font reports `ErrNoLookFont`.
   It is a line of the report and never an error, in the strict mode too,
   because the text still draws in IBM Plex Sans and nothing is dropped.
4. As in Mermaid 12.3, the look of the section of the kind wins over the
   look at the top of the config, and a value that Mermaid does not have
   gives way to the other. A value that is not a look of Mermaid is an
   issue. `look` at the top of the front matter stays ignored, as Mermaid
   ignores it.
5. A fill in the color of the background draws solid, so that the box
   under a label still hides the edge. A rect thinner than 3 draws its
   fill as one line. The gradient that the standard look gives an area
   draws as its first stop, with no issue, because zuzu made it and not
   the spec. This amends D40 part 8 for that gradient.
6. The seed of an item is a hash of the seed of the chart, the names of
   its mark and of the marks above it, with the index of each group item,
   and its index. So a mark of the same name in two cells of a facet does
   not draw the same lines.
7. The background of a group becomes the first mark of the group. The pass
   cuts a text to its limit in the font of the look, and grows the canvas
   where a line wobbles out of it.
8. An edge of a graph, with its arrowheads, draws with a roughness of 0.3,
   and a cluster draws with a fill weight of 3, as Mermaid 12.3 draws them
   for `look: handDrawn`. Every other mark keeps the options of D40 part 5.
   This amends D40 part 5.
