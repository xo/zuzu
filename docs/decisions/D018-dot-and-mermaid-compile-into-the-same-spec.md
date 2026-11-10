# D18. DOT and Mermaid compile into the same spec

Status: Decided.

The maintainers asked on 2026-11-10 that zuzu later read the DOT language of
Graphviz and the syntax of Mermaid, by compiling each into the same
representation as a chart. A diagram then uses the same themes, text and
renderers as a chart.

This is phase 7. It needs a layered graph layout, which Vega does not have,
so zuzu adds it as the transform `zuzu.layered`. zuzu writes the layout from
the documented algorithm and does not port dagre (D6).
