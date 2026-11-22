# D45. zuzu.dagre follows the dagre of Mermaid, and a layout is made available or forced

Status: Decided. Amends D24, D31, D39 and D44.

On 2026-11-22 the work of D44 measured `zuzu.dagre` against mmdc with
`layout: dagre`, over 88 fixtures of the flowchart, the state, the class
and the ER diagram. The mean error of the places of the nodes was 0.052 for
`zuzu.layered` and 0.152 for `zuzu.dagre`. The copy of dagro is dagre
3.1.1, whose crossing sweep keeps the last of equal orders and switches its
bias, while Mermaid 12.3 bundles dagre-d3-es 7.0.14, of the 0.8 line. So
many drawings came out mirrored. With the sweep of 0.8, the error of
`zuzu.dagre` was 0.024, the closest on 55 fixtures. The maintainers
answered three questions.

1. The copy of dagro takes the sweep of dagre 0.8 as a marked change: no
   branch for an equal count of crossings, and a fixed bias in each sweep.
   A copy can take a change that makes it follow the upstream of Mermaid
   12.3. This amends D39 part 2.
2. `zuzu.dagre` is now the closest to mmdc, so `layout: dagre` of Mermaid
   draws with it (D44 part 5).
3. A caller makes an engine available with `WithLayoutEngine`, and the
   source picks the layout: a diagram of Mermaid that sets no layout, in a
   kind whose default layout is ELK in Mermaid 12.3, draws with ELK when
   the engine of ELK is available, and with `zuzu.layered` otherwise.
   `layout: dagre` draws with `zuzu.dagre`. DOT draws with `zuzu.layered`.
   The key `zuzu.layout` of a spec names a layout too.
4. `WithLayout` takes the name of a layout and forces it on each graph of
   the chart, because an option of the caller wins over the source, as
   `WithLook` does. A forced layout with no engine is an issue. The review
   page forces each layout in its own column. This amends D44 part 2.
5. `zuzu.dagre` sets the sibling subgraphs of a flowchart in the reverse of
   the order of the source, as the wrap of dagre in Mermaid does. The reader
   of the flowchart tells the engine so.
