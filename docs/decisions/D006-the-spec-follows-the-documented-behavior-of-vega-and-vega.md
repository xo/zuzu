# D6. The spec follows the documented behavior of Vega and Vega-Lite, and zuzu is not a port

Status: Decided. Amended by D32.

The maintainers decided on 2026-11-10 that zuzu reads the JSON of Vega and of
Vega-Lite, with the same names and the same meaning for each field, and that
zuzu follows the documented behavior of both and does not port their source.

1. The documentation of Vega and Vega-Lite, their JSON schemas and the
   references of the oracle (D17) define the behavior. Where the
   documentation is silent, zuzu chooses, and the choice is recorded.
2. No file of zuzu holds a translation of a source file of Vega, Vega-Lite,
   d3 or dagre. zuzu keeps no ledger of upstream commits, as okage does,
   because it follows no upstream.
3. Where zuzu adds a feature that Vega does not have, such as a sankey or a
   layered graph layout, the name of the feature starts with `zuzu.`, so that
   it cannot collide with a later feature of Vega.
4. zuzu never ignores a feature that it does not support without a word. It
   returns an error, or a list of what it did not draw.

D7 says which part of Vega comes first. This decision merges two decisions
made the same day: the spec follows Vega, and zuzu is not a port, which
answered open question 6.
