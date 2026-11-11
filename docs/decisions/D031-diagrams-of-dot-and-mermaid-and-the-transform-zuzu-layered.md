# D31. Diagrams of DOT and Mermaid, and the transform zuzu.layered

Status: Proposed.

D18 says that DOT, the language of Graphviz, and the flowchart of Mermaid
compile into the same spec as a chart, with a layered layout that zuzu adds
as `zuzu.layered`. This decision proposes how, for phase 7. An agent wrote
it on 2026-11-11, and Gemini and DeepSeek reviewed a draft. "External
review" in `docs/PLAN.md` says what each said and what this decision took.

## Packages and the API

1. The package `graph` holds the model of a diagram and the layout. A
   graph has nodes, edges, clusters, which nest, and a direction. Each has
   a label and attributes.
2. The packages `graph/dot` and `graph/mermaid` read a source into a
   graph. Each has a lexer and a parser by hand, as `expr` has, and its
   grammar goes in `docs/DIAGRAMS.md`, a document of phase 7. A part of the
   language that the grammar does not hold is an issue: an error in the
   strict mode, and a line of the report in the loose mode (D26).
3. `graph` compiles a graph into a Vega spec. `zuzu.Parse` takes the
   dialects `DOT` and `Mermaid`. It finds them by the first word of the
   source: `strict`, `graph` or `digraph` and then a name or `{` for DOT,
   and `flowchart`, or `graph` and then a direction, for Mermaid.
   `WithDialect` names one. `Chart.Spec` returns the Vega spec, as it does
   for Vega-Lite.

## The compiled spec

The spec holds three datasets from the source, `nodes`, `edges` and
`clusters`, and one dataset of the layout, `layout`, that `zuzu.layered`
makes. Three datasets filter `layout` by the field `kind`. The marks draw,
in order: a rect for each cluster with its title, a path for each edge, a
path for each arrowhead, a shape for each node as a path, and a text for
each label of a node, an edge and a cluster.

## zuzu.layered

It runs on the dataset of nodes, and makes one dataset with a row for each
node, each edge and each cluster. The field `kind` says which. One
transform makes all three, so the layout runs once and no transform reads
the output of another.

| Parameter | Default | Meaning |
| --- | --- | --- |
| `edges` | none, required | the dataset of the edges |
| `clusters` | none | the dataset of the clusters, with the field `parent` for a nested one |
| `key`, `cluster` | `key`, `cluster` | the fields of a node: its key and its cluster |
| `source`, `target` | `source`, `target` | the fields of an edge |
| `minlen`, `weight` | 1 and 1 | the fields or the values of an edge: the least ranks it spans, and how much a short edge counts |
| `rank` | none | the field of a node that names a group of nodes of the same rank |
| `direction` | `TB` | `TB`, `BT`, `LR` or `RL` |
| `rankSep`, `nodeSep`, `clusterPadding` | 40, 24, 12 | the space between the ranks, between the nodes of a rank, and inside a cluster |
| `width`, `height` | none | the fields of the size of a node |
| `label`, `fontSize`, `padding` | none, 11 and 8 | the field of the label of a node, which the transform measures, when the node has no size |
| `as` | see below | the names of the output fields |

A node row holds its fields and `x`, `y`, `width`, `height`, `rank` and
`order`, with x and y at its center. An edge row holds its fields and
`path`, a path of SVG, the place of its label `labelX` and `labelY`, and
the place and the angle of each arrowhead. A cluster row holds `x`, `y`,
`width`, `height`, `depth` and the place of its title.

## The algorithm

The layout follows the published algorithms, in the order of Gansner,
Koutsofios, North and Vo, "A technique for drawing directed graphs", 1993
(D6):

1. An edge that closes a cycle turns around, by the greedy heuristic of
   Eades, Lin and Smyth, in the order of the source, so that the result is
   the same each time. An edge of `constraint=false` takes no part in the
   ranks.
2. The ranks come from network simplex, with `minlen`, `weight` and the
   groups of the same rank.
3. A long edge splits into a chain of dummy nodes, one in each rank.
4. The order in each rank comes from the weighted median, with sweeps down
   and up and the swap of neighbors, for 24 passes or until a pass gains
   nothing.
5. A cluster is a box of nodes. In each rank, its nodes and the nodes of
   the clusters inside it stay together, and the order moves a cluster as a
   block. A cluster keeps room for its padding and its title. An edge that
   leaves a cluster crosses its border at a dummy node on the border, so
   that it does not cut through another cluster.
6. The places across the ranks come from Brandes and Köpf, 2001: four
   alignments, balanced, with the space between nodes and clusters.
7. An edge is a line through its dummy nodes, drawn as a cubic B-spline,
   which ends at the border of the shape of each node. An edge from a node
   to itself is a loop at the side of the node. Edges between the same two
   nodes spread apart.

## DOT

zuzu reads:

- `graph`, `digraph` and `strict`
- statements of nodes, of edges and of attributes, and chains of edges
- edges to and from subgraphs, and default attributes
- subgraphs, clusters, which are subgraphs whose names start with
  `cluster`, and `rank=same`

It reads these attributes: `label`, `shape` (`box`, `rect`, `ellipse`,
`oval`, `circle`, `doublecircle`, `diamond`, `plaintext`, `plain`, `none`,
`point`, `record` and `Mrecord`), `style` (`filled`, `dashed`, `dotted`,
`bold`, `rounded`, `invis`), `color`, `fillcolor`, `fontcolor`, `fontsize`,
`penwidth`, `rankdir`, `ranksep`, `nodesep`, `dir`, `arrowhead` and
`arrowtail` (`normal`, `none`, `vee`, `dot`, `empty`), `weight`, `minlen`,
`constraint` and `headport` and `tailport`.

A record, `{a|b|<p>c}`, draws as a box of fields, and a port names a field
or a side, so that an edge ends there. A label of HTML, `splines=ortho`,
the other engines of Graphviz, such as neato, and every other attribute are
issues.

## Mermaid

zuzu reads:

- `flowchart` and `graph` with the directions `TB`, `TD`, `BT`, `LR` and
  `RL`
- the shapes of a node `[ ]`, `( )`, `([ ])`, `[[ ]]`, `[( )]`, `(( ))`,
  `> ]`, `{ }`, `{{ }}`, `[/ /]`, `[\ \]`, `[/ \]`, `[\ /]` and `((( )))`
- the links `-->`, `---`, `-.->`, `==>`, `--o` and `--x`, with text in
  either form, and longer links as `minlen`
- chains, and lists with `&`
- `subgraph` and `end`, with a title and a direction of its own
- `classDef`, `class`, `:::`, `style` and `linkStyle`
- comments with `%%`

A string of markdown draws as plain text, without its marks, and the
report says so. `click`, a callback, front matter and every other kind of
diagram are issues. The pie, the xychart, the sankey, the radar and the
treemap of Mermaid are charts, which compile later to the same specs as
Vega-Lite.

## The look

A node is a rounded rectangle with a tint of its color and an outline of
1.25 (`docs/STYLE.md`), and another shape draws the same way. An edge is a
line of 1.25 in the color of the axis, with a small arrowhead. A label is
11 pixels. A cluster is a panel with its title at the top left. A color
that the source gives replaces the color of the theme.

## The tests and the exit of phase 7

Each language has about 30 fixtures, with a reference from `dot -Tjson`
for DOT and from the SVG of `mmdc` for Mermaid. zuzu lays out a graph its
own way (D12), so a check does not ask for the places or the ranks of the
reference. It checks:

1. Every node, edge, cluster and label of the reference exists.
2. The hard rules hold: each edge that keeps its direction goes from a
   lower rank to a higher one, by at least its `minlen`, the nodes of a
   group of the same rank share it, no two nodes overlap, and each cluster
   holds its members and its nested clusters.
3. The crossings of the edges are at most those of the reference, plus 10
   percent and 1, and the sum of the ranks that the edges span is at most
   that of the reference, plus 10 percent and 1.

The exit of phase 7 is that the fixtures of both languages pass these
checks.
