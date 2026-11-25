# D59. The answers of 2026-11-26: item, canvas, legends and text on a color

Status: Decided. Amends D17 and D35.

On 2026-11-26 the maintainers chose to finish the work of phase 7 that zuzu
can do alone, and answered four questions about it. The items that wait for
elk-go stay in the backlog until upstream answers.

1. An expression of an encoding reads `item` as vega.js gives it. zuzu
   applies the properties of an encode in the order of the spec, so a
   property reads `item.x` and each other property that an earlier
   property set. It also gives `item.mark.items`, the items of a group
   item, and `item.mark` in an encoding of a guide and of the root group.
   `item.bounds` stays in the report, because vega.js gives it only from
   an earlier pulse.
2. The oracle in `test/oracle` installs the npm package `canvas`, so that
   vega.js draws the specs that need a canvas, such as `density-heatmaps`,
   and measures text with a canvas instead of 0.8 of its size for each
   character. The references are drawn again, and zuzu follows what
   changes. This amends D17, which compares no layout of text, only where
   a reference now holds rows or items that it did not hold.
3. A legend gives each entry a group of its own, as vega.js does. zuzu
   applies the encodes of its parts, and then places the entries from
   their bounds. A legend with no encode of a place draws as before. This
   closes the rest of D35 part 4.
4. A text on a colored fill, such as the label of a leaf of a treemap,
   takes the light or the dark text color, whichever has the higher
   contrast with the fill by the measure of WCAG, and has no box behind
   it. `docs/STYLE.md` holds the rule.
