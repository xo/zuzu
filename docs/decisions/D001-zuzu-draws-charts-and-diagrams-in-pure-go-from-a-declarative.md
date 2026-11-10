# D1. zuzu draws charts and diagrams in pure Go from a declarative spec

Status: Decided.

The maintainers decided on 2026-11-10 to make zuzu, a Go package that draws
charts and diagrams. A caller gives it a declarative spec, which says what to
draw from which data and not how to draw it. zuzu computes the data, the
scales and the layout, and writes the result.

The earlier attempts in the `xo` namespace ran a JavaScript library in a
JavaScript engine (`vegagoja`, `echartsgoja`), or Graphviz in a WebAssembly
runtime (`graphvizzero`). zuzu does the work in Go. `docs/PLAN.md` lists what
each attempt taught.
