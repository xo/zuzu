# Targets

This folder holds the target SVGs of zuzu (D13). A target is a sample of the
look that zuzu aims for, at the quality of nivo and visx (D12), drawn from
real data. The targets are not output of zuzu, and they can change.
[`../STYLE.md`](../STYLE.md) holds the rules that they follow.

| File | What it is |
| --- | --- |
| `index.html` | the gallery, which the script writes: every target in the light and the dark mode, side by side, and four charts in each of the thirteen color schemes |
| `svg/<chart>-light.svg`, `svg/<chart>-dark.svg` | the targets in the default scheme |
| `svg/<chart>-<scheme>-<mode>.svg` | four charts in each of the other twelve schemes (D14, D15, D32) |
| `licenses/` | the license of each upstream theme that a scheme takes its colors from (D15) |
| `gen-targets.mjs` | the script that writes the targets and the gallery |
| `package.json` | the packages of the script, with pinned versions |

The script writes `index.html`, `svg/` and, through npm, `package-lock.json`.
The maintainers chose on 2026-11-10 not to commit them for now, so git
ignores them. Run the script to see the targets.

The data is the data of the survey.
[`../survey/LICENSE-data.txt`](../survey/LICENSE-data.txt) holds its licenses,
and a target drawn from Gapminder data names the Gapminder Foundation in its
source line, as CC BY 4.0 asks.

The 33 charts follow the tiers of "Charts" in [`../PLAN.md`](../PLAN.md):

| Tier | Charts |
| --- | --- |
| 0 | line with a crosshair, area, stacked area, grouped bar, stacked horizontal bar, histogram, bubble, donut, calendar heat map, bars and a line with two axes |
| 1 | streamgraph, small multiples, box plot, violin, candlestick, radar, choropleth map, treemap, sunburst, sankey, funnel, gauges, waterfall |
| 2 | parallel coordinates, chord, force-directed network, radial tree, circle packing, beeswarm, density contours, hexagonal bins, word cloud |
| 3 | flowchart with a layered layout |

## Open the gallery

Serve the folder, then open `http://127.0.0.1:8767/`:

```bash
python3 -m http.server 8767 --bind 127.0.0.1 --directory docs/targets
```

## Draw the targets again

The script reads the data of the survey, `../survey/data.js`. Run this in
this folder:

```bash
npm install
node gen-targets.mjs
```

d3 gives the script its math: the scales, the paths of the shapes and the
layouts. The script writes each SVG element itself, from the tokens of
`STYLE.md`.

## What a target can use

Each target uses only the parts of SVG that okage renders: paths, shapes,
text, linear gradients and markers. It uses no CSS, no `@media` rule and no
filter. On 2026-11-10, with six schemes, all 106 files rendered with the
package `svg` of okage with no error, at 640 by 220 to 560 pixels, in 5 to
220 ms each. With the thirteen schemes, the script writes 162 files.
