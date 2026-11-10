<div align="center">
  <a href="#about" title="About">About</a> |
  <a href="#status" title="Status">Status</a> |
  <a href="#design" title="Design">Design</a> |
  <a href="#documents" title="Documents">Documents</a> |
  <a href="#contributing" title="Contributing">Contributing</a> |
  <a href="https://pkg.go.dev/github.com/xo/zuzu" title="Go Reference">Reference</a>
</div>

<br/>

[![Unit Tests][zuzu-ci-status]][zuzu-ci]
[![Go Reference][goref-zuzu-status]][goref-zuzu]
[![Releases][release-status]][releases]
[![Discord Discussion][discord-status]][discord]

[zuzu-ci]: https://github.com/xo/zuzu/actions/workflows/test.yml "Test CI"
[zuzu-ci-status]: https://github.com/xo/zuzu/actions/workflows/test.yml/badge.svg "Test CI"
[goref-zuzu]: https://pkg.go.dev/github.com/xo/zuzu "Go Reference"
[goref-zuzu-status]: https://pkg.go.dev/badge/github.com/xo/zuzu.svg "Go Reference"
[release-status]: https://img.shields.io/github/v/release/xo/zuzu?display_name=tag "Latest Release"
[releases]: https://github.com/xo/zuzu/releases "Releases"
[discord]: https://discord.gg/WDWAgXwJqN "Discord Discussion"
[discord-status]: https://img.shields.io/discord/829150509658013727.svg?label=Discord&logo=Discord&colorB=7289da&style=flat-square "Discord Discussion"

# About

`zuzu` draws charts and diagrams in pure Go. A caller gives it a declarative
spec, which says what to draw from which data, and zuzu writes the chart as
SVG or as an image.

The spec follows [Vega][vega] and [Vega-Lite][vega-lite]. A spec written for
the Vega tools works with zuzu, as far as zuzu supports it. zuzu draws its
own design, at the quality of nivo and visx, with a light and a dark mode and
color schemes that swap. zuzu renders the SVG to an image with
[`okage`][okage], a pure Go port of resvg.

Later, zuzu reads the DOT language of Graphviz and the syntax of Mermaid,
sends live charts to a web browser, and draws the results of a query for
[`usql`][usql].

# Status

zuzu is in its first phases. Phase 0, the repository, the tests of the
documents, CI, the corpus and the oracle, is done. The scene graph, the
themes, the SVG writer and the measurement of text with the bundled IBM Plex
Sans exist (phases 1 and 2). zuzu reads a Vega spec and draws it (phase 3).
On the 540 specs of the first target, its datasets, scales and marks match
those of vega.js for 85 percent of the specs. zuzu compiles a Vega-Lite spec
itself (phase 4), and its result matches for 63 percent of the 470
Vega-Lite specs.
[`docs/PLAN.md`](docs/PLAN.md) holds the plan, its phases and the questions
that the maintainers have not answered.

```go
chart, _, err := zuzu.Parse(src)
if err != nil {
	return err
}
report, err := chart.SVG(ctx, w, zuzu.WithLoader(zuzu.FSLoader(os.DirFS("data"))))
```

The first step was a survey of what other libraries do. Open
[`docs/survey/compare.html`](docs/survey/compare.html) to see the same real
data drawn by 13 chart and diagram libraries, side by side, in a light and a
dark theme. [`docs/survey/README.md`](docs/survey/README.md) says how to open
it.

The second step was the look. Open
[`docs/targets/index.html`](docs/targets/index.html) to see 33 target charts,
from a line to a flowchart, in the light and the dark mode and in six color
schemes. They came before any code. The script in `docs/targets/` draws them,
and [`docs/targets/README.md`](docs/targets/README.md) says how to draw and
open them.

# Design

zuzu has four stages, and each stage hands the next one a value that can be
written as JSON:

1. A front end reads Vega, Vega-Lite, and later DOT or Mermaid, and produces
   a spec in the shape of a Vega spec.
2. The runtime loads the data, runs the transforms and the expressions,
   builds the scales and lays out the axes and the legends.
3. The result is a scene graph: a tree of shapes with their final positions,
   sizes and colors.
4. A renderer writes the scene graph as SVG, draws it with okage, or later
   sends it to a browser.

The first target is static Vega, which is Vega without the events of a mouse
or a keyboard, and the Vega-Lite that compiles to it. The corpus of 579
Vega-Lite specs and 87 Vega specs from [`vegagoja`][vegagoja], with the output
of vega.js for each, measures how far zuzu agrees with Vega.

# Documents

| Document | What it holds |
| --- | --- |
| [`docs/PLAN.md`](docs/PLAN.md) | the plan, the architecture, the phases and the open questions |
| [`docs/API.md`](docs/API.md) | the public API |
| [`docs/EXPRESSIONS.md`](docs/EXPRESSIONS.md) | the expression language, and the choices for scripting beyond it |
| [`docs/decisions/`](docs/decisions/README.md) | every decision, one file each, with an index |
| [`docs/BACKLOG.md`](docs/BACKLOG.md) | the work that is known and not done |
| [`docs/STYLE.md`](docs/STYLE.md) | the rules and the tokens of the look, the modes and the color schemes |
| [`docs/targets/`](docs/targets/README.md) | the target SVGs that show the look zuzu aims for |
| [`docs/survey/`](docs/survey/README.md) | the survey of other libraries, and the page that compares them |
| [`AGENTS.md`](AGENTS.md) | the rules for a coding agent, which apply to a person too |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | how to change zuzu |

# Contributing

Read [`CONTRIBUTING.md`](CONTRIBUTING.md) first. A change to the design is a
decision first, and an open question goes to the maintainers.

<br/>

<div align="center">
  <a href="https://github.com/xo/usql" title="A command line client for many databases">usql</a> |
  <a href="https://github.com/xo/dburl" title="Database connection URLs">dburl</a> |
  <a href="https://github.com/xo/dbmeta" title="Database metadata">dbmeta</a> |
  <a href="https://github.com/xo/dbimp" title="Database drivers in pure Go">dbimp</a> |
  <a href="https://github.com/xo/cassandra" title="A database/sql driver for Cassandra">cassandra</a> |
  <a href="https://github.com/xo/dbtpl" title="Go code generated from a database">dbtpl</a> |
  <a href="https://github.com/xo/tblfmt" title="Tables of database results">tblfmt</a> |
  <a href="https://github.com/xo/rline" title="The line editor of usql">rline</a> |
  <a href="https://github.com/xo/transit" title="tree-sitter in pure Go">transit</a> |
  <a href="https://github.com/xo/okage" title="tiny-skia and resvg in pure Go">okage</a> |
  <a href="https://github.com/xo/zuzu" title="Charts and diagrams in pure Go, this project">zuzu</a>
</div>

[vega]: https://vega.github.io/vega/ "Vega"
[vega-lite]: https://vega.github.io/vega-lite/ "Vega-Lite"
[okage]: https://github.com/xo/okage "okage"
[usql]: https://github.com/xo/usql "usql"
[vegagoja]: https://github.com/xo/vegagoja "vegagoja"
