# D31. Diagrams of DOT and Mermaid, and the transform zuzu.layered

Status: Decided.

D18 says that DOT, the language of Graphviz, and the flowchart of Mermaid
compile into the same spec as a chart, with a layered layout that zuzu adds
as `zuzu.layered`. This decision proposes how, for phase 7. An agent wrote
it on 2026-11-11, and Gemini and DeepSeek reviewed a draft. "External
review" in `docs/PLAN.md` says what each said and what this decision took.
The maintainers accepted it on 2026-11-11.

The maintainers then asked for a survey of the kinds of diagram of Mermaid,
of the programs that draw them without mermaid.js, and of icons.
`docs/survey/mermaid.md` holds it, and this decision follows its leads:
Mermaid 12.3.0, the shapes and the icons of its flowchart, and its charts.

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
   source, after the front matter, the directives and the comments of
   Mermaid: `strict`, `graph` or `digraph` and then a name or `{` for DOT,
   and the keyword of a kind of Mermaid, such as `flowchart`, `pie` or
   `xychart`, or `graph` and then a direction, for Mermaid. `WithDialect`
   names one. `Chart.Spec` returns the Vega spec, as it does for
   Vega-Lite.

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
or a side, so that an edge ends there. `splines=ortho`, the other engines
of Graphviz, such as neato, and every other attribute are issues.

On 2026-11-11 the maintainers added labels of HTML. The package `html` of
okage reads a label of HTML and lays it out as dot does (okage D38), with
the measure of text of zuzu. zuzu draws the boxes, the rules and the runs
of text of the layout as marks, and a port names a cell or a table of the
label. A side of a port, such as `p:e`, ends an edge on that side and
leaves it in that direction.

## Mermaid

zuzu follows Mermaid 12.3.0, the stable version of 2026-11-11. A later
version is a change of this decision. Version 13 is an alpha, and waits.

zuzu reads the front matter, between two lines of `---`, and the directive
`%%{init: ...}%%`: the `title`, and in the `config` the `layout`, the
`look`, the `theme` and the `curve` of a flowchart. Both layouts draw with
`zuzu.layered`, which follows the method of `dagre`, so `layout: elk`,
which also routes edges in its own way, is an issue. The looks `classic`
and `neo` draw in the look of zuzu, as every look does (D12). The look
`handDrawn`, another layout, and a theme, which zuzu does not draw, are
issues.

For the flowchart, zuzu reads:

- `flowchart` and `graph` with the directions `TB`, `TD`, `BT`, `LR` and
  `RL`
- the shapes of a node `[ ]`, `( )`, `([ ])`, `[[ ]]`, `[( )]`, `(( ))`,
  `> ]`, `{ }`, `{{ }}`, `[/ /]`, `[\ \]`, `[/ \]`, `[\ /]` and `((( )))`
- a node with properties, `A@{ shape: name, label: "..." }`, with the
  named shapes of 12.3.0, which `docs/DIAGRAMS.md` lists. A shape that zuzu
  does not draw yet, such as a shape of BPMN, draws as a rounded rectangle,
  and the report says so
- a node of an icon, `A@{ icon: "fa:user", form: square, label: "...",
  pos: t, h: 48 }`, and an icon in a label, `fa:fa-user`
- a node of an image, `A@{ img: "url", label: "...", pos: t, w: 60, h:
  60, constraint: on }`
- the links `-->`, `---`, `-.->`, `==>`, `--o` and `--x`, with text in
  either form, and longer links as `minlen`
- the id of a link, `e1@-->`, and its properties `e1@{ curve: linear }`.
  An animation draws still, and the report says so
- chains, and lists with `&`
- `subgraph` and `end`, with a title and a direction of its own
- `classDef`, `class`, `:::`, `style` and `linkStyle`
- comments with `%%`

A string of markdown draws as plain text, without its marks, and the
report says so. `click`, a callback and every kind of diagram that this
decision does not name are issues.

An image of a node comes through the loader that the caller gives (D25),
and draws from a URL of data, with its `w` and `h`, and with its aspect
kept for `constraint: on`. zuzu reads a PNG, a JPEG and an SVG, and cleans
an SVG first: it drops scripts, `foreignObject`, handlers of events, and
every reference outside the image, such as an `href`, a `url()` or an
`@import`. With no loader, or for a URL that the loader does not give,
the node draws a box with its label, and the report says so. zuzu never
writes a URL from outside into its SVG, so that no viewer of the SVG
fetches it.

## Icons

The package `icon` reads packs of icons in the JSON format of Iconify: the
body of each icon, an inside of an SVG element, its view box, its rotation
and its flips, and the aliases. A program gives packs with the option of a
render `WithIcons`, each under a name that replaces the prefix of the pack,
as Mermaid does. A diagram names an icon as `name:icon`.

1. An icon of one color that holds only paths, as most icons do, draws as
   a path mark, scaled into its box, in the color that the diagram gives
   it. Another icon, with several colors, a gradient or text, draws as an
   image, an SVG of the icon in a URL of data. zuzu sets its color on the
   elements that name `currentColor`, by reading the SVG and not by
   replacing text, and cleans it as it cleans an image. The SVG writer
   writes each image once, in its definitions, and uses it where it draws,
   and the direct renderer draws each image once, with okage, and keeps
   it.
2. zuzu draws the five icons of the architecture diagram of Mermaid,
   `cloud`, `database`, `disk`, `internet` and `server`, in its own look.
3. A missing pack or icon draws as a box with a question mark, as in
   Mermaid. It is an error in the strict mode, and a line of the report in
   the loose mode.
4. zuzu bundles four packs, each in a package of its own under `icons/`,
   so that a program that draws no icon does not carry them:
   `icons/carbon`, the icons of IBM under Apache-2.0, which suit IBM Plex,
   `icons/fa`, the solid icons of Font Awesome Free under CC BY 4.0, under
   the names `fa` and `fas` that the flowcharts of Mermaid use, which holds
   no regular icon and no brand, `icons/mdi`, Material Design Icons under Apache-2.0, and
   `icons/k8s`, the icons of Kubernetes under Apache-2.0, without the logo
   of Kubernetes, which is a trademark. Each package holds the license of
   its pack, and pins its version. A program imports a package to use it.
   The package `icons/fa` gives its attribution as a value, and the SVG
   writer names it in the metadata of a chart that draws one of its icons,
   since CC BY 4.0 asks the one who shares an image for the credit.
5. zuzu bundles no icon of a cloud vendor and no logo of a brand. The terms
   of AWS, of Azure and of the CNCF limit a change and a copy, and a logo
   is a trademark. A program loads such icons itself, under the terms of
   their owner, with a pack or with an icon loader.
6. A program can also give an icon loader, with the option `WithIconLoader`.
   It works as the loader of data does (D25): zuzu asks it for each icon
   that no pack holds, by its prefix and its name, and the loader gives the
   bytes from wherever it chooses: a folder, a store of its own, or a
   download. zuzu itself still reads no file and makes no request.

   ```go
   type IconLoader interface {
       LoadIcon(ctx context.Context, req IconRequest) (io.ReadCloser, error)
   }

   type IconRequest struct {
       Prefix, Name string // aws and lambda, for aws:lambda
   }
   ```

   The bytes are an SVG document, an icon of Iconify, or a pack of
   Iconify that holds the icon. zuzu asks once for each icon in a render,
   and the loader keeps what it fetched from one render to the next. An
   error, or a context that ends, draws the box with a question mark, as
   for a missing icon. zuzu cleans the SVG of a loaded icon as it cleans an
   image, and draws an icon of several colors as it is, so that it keeps
   the colors of its owner. `FSIconLoader` reads `<prefix>/<name>.svg` or
   `<prefix>.json` from a file system that the caller gives, as
   `FSLoader` does for data. So a program that downloads the icons of AWS
   once, under the terms of AWS, draws `aws:lambda` from that folder, and
   zuzu bundles none of them.

## The charts of Mermaid

The charts of Mermaid compile to the specs that zuzu already draws, with
the values of the source as the values of a dataset:

| Keyword | Compiles to |
| --- | --- |
| `pie` | an arc of Vega-Lite, with `showData` as the values in the labels |
| `xychart` | bars and lines of Vega-Lite, with the axes that the source names |
| `quadrantChart` | points of Vega-Lite on two axes from 0 to 1, with the four quadrants and their names |
| `gantt` | bars of Vega-Lite on a time axis, by section, and milestones as points |
| `journey` | points of Vega-Lite of the score of each step, by section, with the actors |
| `wardley` | points, lines and text on the two axes of evolution and visibility |
| `sankey` | a Vega spec with `zuzu.sankey` and `zuzu.sankeylinks` |
| `radar` | a Vega spec of polygons on radial axes |
| `treemap` | a Vega spec with `stratify` and `treemap` |

A setting of a chart that zuzu does not read is an issue.

The compiler works out the plan of a gantt in Go, and gives Vega-Lite only
the start and the end of each task as times. It reads `dateFormat`,
`axisFormat`, `tickInterval`, the durations, `after` and `until` with one
task or more, `excludes` with weekends and dates, `weekend`, milestones,
and the tags `done`, `active` and `crit`. A cycle of `after` is an error.
The dates are local time (D27). `todayMarker` needs the time of the render,
which comes from the caller, and with none it draws no marker.

## The other kinds

The survey found about 33 kinds of diagram in Mermaid 12.3.0. Phase 7 holds
the flowchart and the charts above. A later decision names the next kinds,
in the order of the survey: first the sequence, the state, the class and
the ER diagrams, which the most programs draw, then the rest. The state,
the class and the ER diagrams lay out with `zuzu.layered`, and a node of a
class or of an entity is a table of fields, as a record of DOT is.

## The look

A node is a rounded rectangle with a tint of its color and an outline of
1.25 (`docs/STYLE.md`), and another shape draws the same way. An edge is a
line of 1.25 in the color of the axis, with a small arrowhead. A label is
11 pixels. A cluster is a panel with its title at the top left. A color
that the source gives replaces the color of the theme.

## The tests and the exit of phase 7

Each language has about 30 fixtures, with a reference from `dot -Tjson`
for DOT and from the SVG of `mmdc` for Mermaid, the version that bundles
Mermaid 12.3.0. zuzu lays out a graph its
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

Each chart of Mermaid has about 5 fixtures. Its check is that each value of
the source draws once, as a slice, a bar, a point or a task, with its label,
as `mmdc` draws it. A gantt has about 15 fixtures, and its oracle reads the
start and the end of each task from mermaid.js in Node, and zuzu must give
the same.

The icons have fixtures of their own: the oracle of the test module draws
each icon of each bundled pack with `@iconify/utils`, and the check is that
zuzu gives the same view box and the same body.

The exit of phase 7 is that the fixtures of both languages, of the charts
and of the icons pass these checks.
