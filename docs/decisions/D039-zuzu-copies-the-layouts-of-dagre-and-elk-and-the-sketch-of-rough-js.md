# D39. zuzu copies the layouts of dagre and ELK and the sketch of rough.js

Status: Decided. Amends D6 and D32. Amended by D41, D44, D45 and D57.

On 2026-11-22 the maintainers asked whether the Go packages of the d2lang
organization can give zuzu better graph layouts and the looks drawn by
hand, as copies in `internal/`, the way ivy and okage keep copies. This
decision holds the facts and the plan. The maintainers answered its four
questions on the same day, which answers questions 32 to 35 of
`docs/PLAN.md`.

## The facts

### What zuzu does today

1. zuzu does not use dagre. `zuzu.layered` is its own layered layout,
   written from the documented method of dot and dagre (D6, D18, D31). A
   comment in `graph/rank.go` names dagre only to say that the two break a
   cycle the same way.
2. `layout: dagre` and `layout: elk` with its variants are hints. The
   default layout of Mermaid 12.3 is ELK, as D44 found. zuzu draws each diagram with `zuzu.layered` (D32
   part 2).
3. `zuzu.layered` has about 4,550 lines in `graph/layout.go`,
   `graph/rank.go`, `graph/place.go`, `graph/nest.go` and
   `graph/lanes*.go`. It has the lanes of a swimlane diagram, the ports of a
   record, nested clusters that lay out on their own in their own
   direction, edges that end at a cluster, `rank=same`, `constraint=false`,
   the head, tail and outside labels of dot, and the clip of an edge at the
   outline of a shape.
4. The fixtures of Mermaid check the structure, the crossings and the span
   of a layout against mmdc. They do not check the places of the nodes.
5. D6 says that no file of zuzu holds a translation of a source file of
   dagre, and that zuzu keeps no ledger of upstream commits. D32 part 2
   says that zuzu draws each look by hand in its own way and ports no code
   of Excalidraw, rough.js or matplotlib. This decision amends both.

### What Mermaid 12.3 uses

6. mermaid 12.3.0 depends on `dagre-d3-es` 7.0.14, `elkjs` ^0.12.0 and
   `roughjs` ^4.6.6. `dagre-d3-es` is a fork of `dagre-d3`, so its dagre
   is of the 0.8 line. Mermaid also wraps dagre: a cluster with no edge
   that crosses its border lays out on its own, in its own direction.
7. `look: handDrawn` draws with roughjs. The setting `handDrawnSeed`, 0 by
   default, seeds it, and 0 gives a random seed.

### The packages of d2lang

The d2lang organization holds 17 repositories. Three of them matter here.
`mathjax-go` is a port of MathJax, which okage already covers (D36), and
`util-go` holds tools for the command line of D2.

| Package | Upstream at HEAD | License | Lines of source | Lines of tests | Commits |
| --- | --- | --- | --- | --- | --- |
| `dagro` | dagre 3.1.1 and graphlib 4.0.5. The tag v0.1.1 is dagre 0.8.5 | MIT | 4,577 | 6,034 | 16 |
| `elk-go` | ELK.js 0.12.0. The tag v0.1.1 is ELK.js 0.8.2 | EPL-2.0, with two files under Apache-2.0 and one under BSD-3-Clause | 48,878 | 16,639 | 10 |
| `rough-go` | Rough.js 4.6.6 | MIT | 2,939 | 1,928 | 7 |

8. Each one is a port by the D2 team, made mostly by coding agents, from
   August to November 2026. None has a dependency outside the standard
   library. Each checks itself against its JavaScript upstream in tests
   that run only with Node.js.
9. dagro has compound graphs, nested clusters, edge labels, loops, named
   multi-edges and the four directions. It has no ports, no edges to a
   cluster, and no cluster in its own direction. It uses `reflect` only
   when a caller gives a function as a default label. Its data in
   `testdata` comes from D2 under MPL-2.0, so zuzu does not copy it.
10. elk-go has 11 algorithms: fixed, box, random, layered, stress, mrtree,
    radial, force, sporeOverlap, sporeCompaction and rectpacking. Its
    layered algorithm has ports, orthogonal routing, edges across the
    hierarchy, a layout of its own for each cluster, and the placement of
    labels. It reads `encoding/json` v1, which D29 does not allow. Its
    layered and force algorithms take a seed from the clock when the seed is
    0, and sporeOverlap jitters with the global `math/rand`.
11. rough-go draws lines, rectangles, ellipses, arcs, curves, polygons and
    SVG paths, with the fills solid, hachure, zigzag, cross-hatch, dots,
    dashed and zigzag-line. It is deterministic for a seed from 1 to
    2^31-1, and random for the seed 0. A path with a fill that is not solid
    gives an SVG pattern, which the scene of zuzu does not have.
12. EPL-2.0 is a copyleft at the level of the file. A copied file stays
    EPL-2.0 with its notices. Whoever ships a binary that holds it must
    make its source available and say how to get it. That duty passes to
    each program that uses zuzu. The files of zuzu that only call it stay
    MIT.
13. The Go compiler fuses a multiply and an add into one instruction on
    arm64 and not on amd64, so a copy can give other bits on another
    machine. okage stops the fusion with an explicit conversion (okage
    D16). rough-go does it in `jsNumberStep`. dagro and elk-go do not say.

### What the reviewers said

14. Gemini 3.1 Pro, GPT-6 Astra and DeepSeek V4 reviewed the plan, and the
    okage peer answered for okage. `docs/PLAN.md` holds what each one said,
    under External review. All of them said that the sketch is a pass from
    scene to scene in zuzu, except DeepSeek, which said a path effect in
    the renderer. All of them said to keep ELK out of the MIT module, or to
    make it an optional module. Astra and DeepSeek said to import dagro and
    rough-go as modules at first, and Gemini said to copy them.

## The plan

### The process of a copy

1. zuzu takes the rules of ivy for a copy: `docs/UPSTREAM.md`, a table of
   upstreams in `docs/upstream/upstreams.tsv` with the base commit and the
   newest commit that zuzu has, and a checkout in an ignored `upstream/`
   folder. Each copy goes in a folder of its own under `internal/`, with
   the `LICENSE` and the `NOTICE` of the project byte for byte, a comment
   on each file that names the project, the file and the base commit, a
   comment `zuzu:` on each change, and a `README.md` that lists the files
   and the changes. golangci-lint does not check a copy.
2. A copy leaves out the commands, the debug code, and the test data that
   comes from D2. Each change that zuzu needs for determinism, for TinyGo or
   for D29 goes in the copy as a marked change, and zuzu offers it upstream.
3. D6 then reads: no file of zuzu holds a translation of Vega, Vega-Lite or
   d3 that zuzu made. zuzu can copy a Go package, under its license, with
   the process above. D32 part 2 then allows rough.js through a copy of
   rough-go.

### The sketch

4. `internal/rough` is a copy of rough-go at Rough.js 4.6.6, the version
   that Mermaid 12.3 uses.
5. The look is a pass from scene to scene in zuzu, after the layout and
   before every renderer. It turns each rule, rect, path, arc, symbol and
   area into the paths of rough-go: one path for the strokes and one for the
   fill, as rough.js draws them. The SVG writer and `raster` read the same
   scene, so both draw the same look (hard rule 5). okage needs nothing new.
6. Each item gets its own seed, from a hash of the seed of the chart, the
   name of its mark and its index, never from the order of a walk. The seed
   is from 1 to 2^31-1, so rough-go never takes a random seed.
   `handDrawnSeed` of Mermaid gives the seed of the chart. Its default 0
   gives a fixed seed of zuzu, so that the output stays deterministic.
7. A fill that is not solid draws as polygons, never as an SVG pattern.
   The pass gives a stroke room to wobble, so that the bounds of a view hold
   it.
8. The look like Excalidraw comes first: the options of rough.js that
   Mermaid passes for `look: handDrawn`, and the font Excalifont, once
   its license is checked (D16, D32). The look like xkcd is code of zuzu in
   the same pass: long, smooth wobbles that follow the line, with no double
   stroke.
9. WithLook, `zuzu.look` and `look: handDrawn` select a look (D35 part
   12).

### The layouts

10. A layout engine sits behind one interface in `graph`: a graph with its
    clusters, ports and label boxes goes in, and the places of the nodes,
    the points of the edges and the places of the labels come out. An
    engine says what it does not support, and the report names each such
    part (hard rule 4). zuzu does not fall back to another engine without a
    word.
11. `zuzu.layered` stays, and stays the layout of DOT and of a swimlane
    diagram. It has what dagre lacks: the lanes, the ports, and the
    clusters in their own direction.
12. `internal/dagre` is a copy of dagro at its HEAD, dagre 3.1.1, with the
    later fixes of dagro. A new transform, `zuzu.dagre`,
    lays out a Mermaid diagram with it, with the wrap of Mermaid around it:
    a cluster with no edge across its border lays out on its own. It
    becomes the layout of `layout: dagre` only after a spike shows that it
    draws closer to mmdc than `zuzu.layered`. The spike adds a check of the
    places of the nodes to the fixtures, and compares both layouts on each
    fixture of the flowchart, the state, the class and the ER diagram.
13. ELK takes `zuzu.elk` for `layout: elk` and its variants, with its
    layered algorithm first. Its code lives in a separate Go module of this
    repository, `github.com/xo/zuzu/elk`, under EPL-2.0, with the copy of
    elk-go in its own `internal/`. A caller imports it and passes it to
    zuzu as an option, so the module `github.com/xo/zuzu` stays MIT and
    holds no EPL-2.0 file. Without it, `layout: elk` stays a hint that
    `zuzu.layered` draws (D32). The copy forces each seed of elk-go to a
    fixed value, drops the global `math/rand`, and reads JSON with
    `encoding/json/v2` (D29).

### The order of the work

14. First the process of a copy, and the amendments of D6 and D32. Then the
    sketch, because it is small, its upstream matches Mermaid, and D35
    already says how a caller selects it. Then the spike of dagro. ELK
    comes last, with a measure of its size in the TinyGo build.
15. Each copy gets a golden test of its output that runs on amd64 and on
    arm64, so that a fused multiply-add cannot change the bytes without a
    failure.

## The answers

On 2026-11-22 the maintainers answered the four questions of this plan:

1. They accept the amendments of D6 and D32 (question 32). D6 now reads as
   part 3 says, and D32 part 2 allows rough.js through a copy of rough-go.
2. ELK lives in a separate Go module under EPL-2.0 (question 33), as part
   13 says.
3. The copy of dagro starts at its HEAD, dagre 3.1.1 (question 34). The
   spike of part 12 still decides whether `zuzu.dagre` becomes the layout
   of `layout: dagre`.
4. zuzu copies dagro and rough-go, and does not import them (question 35),
   because zuzu must change each of them for the seeds, for D29 or for
   TinyGo.
