# D44. The layouts of dagre and ELK, and their review

Status: Decided. Amends D24 and D39. Amended by D45.

On 2026-11-22 the maintainers asked to see the layouts of D39 side by side
in the review page, and answered three questions.

1. One batch builds both layouts: the interface of a layout engine in
   `graph`, the copy of dagro with the transform `zuzu.dagre`, and the
   separate module `github.com/xo/zuzu/elk` with the transform `zuzu.elk`
   (D39 parts 10 to 13).
2. A caller gives an engine to zuzu with `WithLayout`, a shared option of
   Parse and of the render, as `WithLook` is. `elk.Layout()` returns the
   engine of ELK, and the chart keeps it. Another engine uses the same
   option. zuzu registers no engine of its own at the level of a package
   (hard rule 9). This amends D24, as D40 part 12 said.
3. The test module gets an oracle of the layouts of Mermaid: mermaid
   12.3.0 and puppeteer-core in `test/oracle`, and `oracle/mermaid-svg.mjs`,
   which draws many diagrams in one Chrome as mmdc draws them, each in the
   layout that its job asks for. The maintainers asked for an oracle of
   ELK, on the word that mermaid-cli lacks it. The work found that
   mermaid.js 12.3.0 bundles the layouts of ELK and that its default layout
   is `elk`. So mmdc draws a diagram that sets no layout in ELK, and draws
   in dagre only with `layout: dagre`. `@mermaid-js/layout-elk` adds
   nothing to this bundle, and the test module does not install it. The
   references of the fixtures of Mermaid in `test/testdata/diagrams` hold
   the places of ELK.
4. The review page shows a row of layouts under each diagram: the original
   in its default layout, which is ELK for a diagram of Mermaid, and the
   original with `layout: dagre`, beside `zuzu.layered`, `zuzu.dagre` and
   `zuzu.elk`. Each drawing of zuzu names its error of the places of the
   nodes against the original of its kind: `zuzu.dagre` against dagre, and
   `zuzu.elk` against ELK (D40 part 11).
5. `zuzu.dagre` becomes the layout of `layout: dagre` only after that error
   shows it closer to mmdc with `layout: dagre` than `zuzu.layered` (D39
   part 12). Until then, the transform exists and the review shows it.
6. The copy of elk-go reads and writes no JSON. zuzu builds the graph of ELK
   in Go, so the copy leaves out the JSON form of upstream and
   `encoding/json` with it. This meets D29 with fewer changes than the move
   to `encoding/json/v2` that D39 part 13 named.
