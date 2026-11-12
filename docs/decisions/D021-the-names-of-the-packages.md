# D21. The names of the packages

Status: Decided.

The maintainers asked on 2026-11-10, in open question 7, for the advice of
the flagship models on the conventions of Go for the names of the packages.
GPT-6 Astra and Gemini 3.1 Pro answered the same day. DeepSeek did not
answer in time. The maintainers accepted the result the same day.

Both models said: keep short names of one word, do not mangle a name to
avoid a collision, and resolve a collision with an alias at the import. So
the SVG writer is `svg`, and inside zuzu the package of okage is imported as
`okagesvg "github.com/xo/okage/svg"`. Astra said that a name such as `svgw`
gives up a natural name to solve a problem of one file.

The models differed on three points, and zuzu takes the clearer choice of
each:

1. The Vega types are in `vega`, not `spec`, and the Vega-Lite types and
   compiler are in `vegalite`, not `lite`. Gemini proposed this, because the
   name then says what the types are. A reader of `vega.Spec` and
   `vegalite.Compile` knows the source of the grammar.
2. The renderers are not grouped under `render/`. Astra said that a folder
   only for grouping adds length to the path and still imports as `svg`, so
   it solves nothing. `svg` and `raster` stay at the top level.
3. `format` stays `format`. Gemini proposed `d3fmt`, but D6 says zuzu
   follows documented behavior and is not a port of d3, so the name of d3
   does not belong in it.

The other names stay as `docs/PLAN.md` lists them: `zuzu`, `expr`,
`dataflow`, `scale`, `geo`, `layout`, `scene`, `text`, `theme`, `raster`,
`graph` with `graph/dot` and `graph/mermaid`, and `web`. An exported name does
not repeat its package: `svg.Write`, not `svg.WriteSVG`.
