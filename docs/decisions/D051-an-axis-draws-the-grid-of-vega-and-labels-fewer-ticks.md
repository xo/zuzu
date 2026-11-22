# D51. An axis draws the grid of Vega and labels fewer ticks

Status: Decided. Amends D46.

On 2026-11-22 the maintainers saw that the look of zuzu, with its few
labels, also drew few grid lines, such as the lines at 0, 15,000, 30,000
and 45,000 of `vega/falkensee-population.vg.json`, where vega.js draws a
line each 5,000. The maintainers asked for the grid lines of Vega with the
labels of the look.

1. An axis draws its grid lines and its ticks at the ticks that Vega draws:
   the ticks of its scale for the `tickCount` of the axis, or for 10, the
   count of Vega, when the axis has none.
2. The axis draws its labels at the ticks of the look, as D46 part 2 says,
   such as one each 15,000. A tick of the look always has a grid line and a
   tick.
3. The axis draws its grid lines and its ticks at the ticks of the look
   alone when a tick of the look is not a tick of Vega, or when two grid
   lines of Vega stand less than 10 pixels apart.
4. `values` of the spec gives the ticks, the grid lines and the labels, as
   before.

`docs/STYLE.md` gives the rule. This amends D46 part 2.
