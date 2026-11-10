# Backlog

This document lists work that is known and not done. Each item names the
decision or the document that found it.

A decision is not a backlog item. It goes in
[`decisions/`](decisions/README.md). A question for a maintainer goes at the
end of [`PLAN.md`](PLAN.md), under Open questions for the maintainers. When an
item here is done, delete it, and record in `decisions/` anything that was
decided on the way.

## Phase 0

### Run the oracle in CI

The oracle writes the references in about 8 seconds, and git ignores them
(`test/README.md`). When the first comparison with them exists, in phase 3,
add a job to `.github/workflows/test.yml` that sets up Node.js, installs the
packages of `test/oracle` and runs the oracle with `TZ=UTC` before the tests
of the test module.

### Add a test that names a person in a decision

`TestTheDocumentsNameNoPerson` reads every document for the names of the
maintainers (D22). Its list of names is short, and a new maintainer adds a
name to it.

## Phase 1

### Review the writer against the targets

`go run ./cmd/targets` in the test module rebuilds each target as a scene
graph, writes it with `svg`, and writes `docs/targets/compare.html`, which
shows each target beside the output of zuzu. On 2026-11-10, 104 of the 106
targets differed by at most 0.04 percent of their pixels. The flowchart, in
both modes, scales a group, which a scene graph does not hold. The exit of
phase 1 is that a maintainer accepts the page.

The targets were drawn with d3, so a curve, an area or an arc of a target is
a path in its scene graph. The comparison checks the paint, the text, the
groups, the gradients, the symbols, the rects and the rules of the writer.
The curves, the arcs and the trails of the writer have their own tests
against d3, in `svg/shapes_test.go`.

### Draw what the writer does not draw yet

1. The writer does not cut a text to its `limit`, which needs the
   measurement of text (D11).
2. The mark `shape` returns an error. Vega draws it with a function that the
   JSON of a scene graph cannot hold, so the runtime of zuzu writes a path
   mark in its place.
3. The interpolations `basis-open`, `basis-closed`, `cardinal-open`,
   `cardinal-closed`, `catmull-rom-open` and `catmull-rom-closed` return an
   error.

### Make the Vega config of a theme

D14 says that a theme becomes one Vega config. The package `vega` holds the
type of a config, and it comes in phase 3. Until then, `theme.Theme` holds the
colors, and the config waits.

## Themes

### Write the tool that converts upstream themes into schemes

D15 calls for a tool that reads the upstream palette of each theme at a pinned
commit and writes one JSON scheme for each, with its license file. The first
sources are the styles of chroma and the editor themes that transit D98
lists. Each theme with a license that MIT does not allow stays out. The two
prototypes in the targets, `catppuccin` and `solarized`, show the mapping
from a palette to the tokens of a scheme.

### Choose a font for each scheme from an editor theme

Open question 12. A scheme can name a font that suits its theme. zuzu
bundles only IBM Plex Sans (D16).

### Embed IBM Plex Sans

D16. Embed the weights 400, 500 and 600 and the license file, when the first
code exists.

## Survey

### Add the libraries that the comparison page leaves out

`docs/survey/compare.html` leaves out four libraries that the survey names:

1. The diagram tool D2, which has a WebAssembly build for the browser and no
   plain script.
2. Nivo, which has no build that a page can load without a bundler.
3. ggplot2 and Seaborn, which need R and the Python packages that Seaborn
   brings. `gen-static.py` can draw Seaborn as it draws Matplotlib.
4. Graphviz and Mermaid from the command line, `dot` and `mmdc`, to compare
   with their browser builds on the same graph.

### Draw the charts that the page marks as missing in Matplotlib

The treemap, the sunburst, the sankey and the map have no Matplotlib image.
Each needs another package: squarify, nested pies by hand, or geopandas.
