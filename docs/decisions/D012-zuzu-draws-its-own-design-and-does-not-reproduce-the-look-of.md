# D12. zuzu draws its own design and does not reproduce the look of Vega

Status: Decided.

The maintainers decided on 2026-11-10 that zuzu does not reproduce the
output of Vega. It reads the specs of Vega and Vega-Lite with their meaning
(D6), and it writes its own SVG, new, with its own layout of axes, legends,
titles and padding. The goal for the look is the quality of nivo and visx:
clear type, soft grids, rounded bars, direct labels and quiet color. nivo is
a set of chart components for React with polished defaults, and visx is the
set of chart primitives of Airbnb. The maintainers named visx first and added
nivo the same day.

This answers open question 3 of `docs/PLAN.md`: the default theme has the
look of zuzu. A theme is still a Vega config (D14), so a spec can still set
any default that Vega lets it set.

The look is set before any code, by target SVGs (D13), and
`docs/STYLE.md` holds the rules that the targets follow. The comparison with
vega.js no longer covers SVG or pixels (D17).
