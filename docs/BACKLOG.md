# Backlog

This document lists work that is known and not done. Each item names the
decision or the document that found it.

A decision is not a backlog item. It goes in
[`decisions/`](decisions/README.md). A question for a maintainer goes at the
end of [`PLAN.md`](PLAN.md), under Open questions for the maintainers. When an
item here is done, delete it, and record in `decisions/` anything that was
decided on the way.

## Phase 0

### Port the tests that check the documents

dbmeta has five tests that keep the documents honest. zuzu needs the same
tests, written for its own layout, before the first code of the package:

1. `TestSkillsAreCopies` fails on a skill that is a symbolic link, a skill
   that has only one copy, and two copies that differ (dbmeta D89).
2. `TestClaudeImportsAgents` fails when `CLAUDE.md` holds anything but
   `@AGENTS.md` (dbmeta D110).
3. `TestTheRootHoldsFourDocuments` fails on a Markdown file in the root other
   than the four (dbmeta D50).
4. `TestTheDecisionIndexIsComplete` fails when a decision has no row in
   `docs/decisions/README.md`, and prints the row (dbmeta D111).
5. `TestProseIsSimpleEnglish` checks the rules of `simple-english` that a
   machine can check (dbmeta D156).

A sixth test fails when a document names a person rather than the
maintainers (D22). The list of names it looks for is the open part.

### Add the CI workflow and the lint configuration

`README.md` shows the badge of `.github/workflows/test.yml`, which does not
exist. Add the workflow with the tests above, and a `.golangci.yml` with the
posture of dbmeta: `default: all`, a disable list, and the reason beside each
entry. The badge has no status until then.

### Bring the corpus into the test module

Copy the specs, the data and the reference SVGs from the testdata of
`vegagoja` into the test module. Put the license file of each source in the
folder that holds its files: the BSD 3-Clause License of Vega beside the
Vega examples, the same of Vega-Lite beside the Vega-Lite examples, and the
license of Deneb, which `vegagoja/testdata/deneb/LICENSE` holds, beside the
Deneb samples. Write an
inventory that lists, for each spec, its marks, its transforms, the
expression functions it calls and whether it binds an event (D7, D17).

### Write the oracle

A tool in the test module that runs vega.js and vega-lite.js in Node.js and
writes the references for each spec that D17 lists: the compiled Vega spec,
the rows of each dataset after its transforms, the scales, and the items of
each mark with their data. It pins Vega 6.4.0 and Vega-Lite 6.5.0, or the
releases current when it is written.

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
