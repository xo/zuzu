# Diagrams

zuzu reads diagrams of DOT and Mermaid (D31). This document lists the parts
of each language that zuzu reads. A part that this document does not list
is an issue: an error in the strict mode, and a line of the report in the
loose mode (D26). zuzu never drops a part of a source without a word.

`zuzu.Parse` finds a diagram by the first word of its source, after the
front matter, the directives and the comments of Mermaid. `strict`,
`digraph`, or `graph` and then a name or `{`, start DOT. The keyword of a
kind of Mermaid, or `graph` and then a direction, start Mermaid.
`WithDialect` names the dialect when the first word does not.

A graph compiles to a Vega spec that draws with the transform
`zuzu.layered` (D31). A chart of Mermaid compiles to a spec of Vega-Lite or
of Vega.

## DOT

zuzu reads the language of Graphviz 16 for the layout `dot`.

### Statements

| Statement | Example |
| --- | --- |
| a graph, with `strict`, `graph` or `digraph` and a name | `strict digraph G { ... }` |
| a node, with attributes | `a [label="Start", shape=box]` |
| an edge, or a chain of edges | `a -> b -> c [label=x]` |
| an edge with ports | `a:p1 -> b:n` |
| the defaults of a graph, a node or an edge | `node [shape=box]` |
| an attribute of the graph | `rankdir=LR` |
| a subgraph, with braces | `subgraph cluster_0 { a; b }` |
| a group of nodes as the end of an edge | `a -> {b c}` |

A subgraph whose name starts with `cluster` is a cluster, which draws as a
box with its title. A cluster that names a node made outside it takes the
node in, as dot does. `rank=same` in a subgraph puts its nodes in one rank.
Another `rank` is an issue.

An ID is a name, a numeral, a string in double quotes, or a string of HTML
in angle brackets. Strings joined with `+` are one string. Comments are
`//`, `/* */`, and a line that starts with `#`.

### Attributes

| Of | Attribute | What zuzu does |
| --- | --- | --- |
| the graph | `label` | the title |
| the graph | `rankdir` | `TB`, `BT`, `LR` or `RL` |
| the graph | `ranksep`, `nodesep` | the space between the ranks and between the nodes, in inches |
| the graph | `splines` | curved or straight edges. Another value is an issue |
| the graph | `layout` | `dot`. Another layout is an issue |
| a node | `label` | the label, with `\n`, `\l` and `\r` as new lines and `\N` as the name, or a label of HTML, as below |
| a node | `shape` | the shape, as below |
| a node | `width`, `height`, `fixedsize` | the size, in inches |
| a node | `margin` | the space around a label of HTML, `x` or `x,y` in inches. A node with no shape has `0.11,0.055` by default, and `plain` has 0 |
| a node | `style` | `filled`, `dashed`, `dotted`, `bold`, `rounded` |
| a node | `color`, `fillcolor`, `fontcolor`, `penwidth`, `fontsize` | the paint |
| an edge | `label` | the label |
| an edge | `arrowhead`, `arrowtail`, `dir` | the arrowheads |
| an edge | `minlen`, `weight`, `constraint` | the ranks that the edge spans, its weight, and whether it takes part in the ranks |
| an edge | `headport`, `tailport` | the ports, as below |
| an edge | `color`, `penwidth`, `style`, `fontcolor`, `fontsize` | the paint |
| a cluster | `label`, `style`, `color`, `fillcolor`, `bgcolor`, `fontcolor`, `penwidth`, `pencolor` | the title and the paint |

A color is a name of X11 or of SVG, `#rrggbb`, `#rrggbbaa`, or three
numbers of hue, saturation and value.

The shapes are `box`, `rect`, `rectangle`, `square`, `ellipse`, `oval`,
`circle`, `doublecircle`, `point`, `diamond`, `hexagon`, `octagon`,
`pentagon`, `triangle`, `invtriangle`, `house`, `invhouse`, `trapezium`,
`invtrapezium`, `parallelogram`, `cylinder`, `note`, `tab`, `folder`,
`plaintext`, `plain`, `none`, `underline`, `record` and `Mrecord`.

The arrowheads are `normal`, `vee`, `dot`, `odot`, `empty`, `diamond`,
`odiamond`, `none`, and the `inv` forms.

### Records

A node of the shape `record` or `Mrecord` is a box of fields:
`label="<f0> left|<f1> mid|<f2> right"`. The fields lie across the ranks,
side by side when the ranks run down and one under another when they run
across. A field of a nested group, `{b|{c|d}}`, is a field of the record,
in order. `<name>` names a port, and an edge that names the port ends at
its field. `\{`, `\}`, `\|`, `\<`, `\>` and `\ ` are text.

### Labels of HTML

A label in angle brackets, `label=<...>`, is a label of HTML of Graphviz.
The package `html` of okage reads it and lays it out as dot 16.1.0 does
(okage D38), and zuzu draws the layout with its own fonts and theme:

| Part | What zuzu draws |
| --- | --- |
| text with `<b>`, `<i>`, `<u>`, `<o>`, `<s>`, `<sub>`, `<sup>` and `<font color point-size face>` | runs of text in their fonts. A line under, over or through a run draws in the color of the text |
| `<br align="left">` | a new line, placed in the text by its `align` |
| `<table>` with `<tr>`, `<td>`, `<hr/>` and `<vr/>` | the boxes of the table and its cells, and the rules |
| `border`, `cellborder`, `cellspacing`, `cellpadding` | the borders and the spaces, in points |
| `bgcolor`, `color` | the fill and the border. A cell with no color takes that of its table, and then the color of the node |
| `align`, `balign`, `valign` | the place of the content in a cell |
| `colspan`, `rowspan`, `width`, `height`, `fixedsize` | the size of a cell or a table |
| `sides` | the sides of a cell that have a border |
| `style` | `rounded`, `dashed` and `dotted` |
| `rows="*"`, `columns="*"` | a rule between each two rows or columns |
| `port` | a port, as below |

A table in a node with no shape, such as `shape=none` or `shape=plaintext`,
takes the fill of a node of the theme. Text with no `color` on a cell with a
`bgcolor` takes the text color of the light theme or of the dark theme,
whichever reads on the fill, so that a header in `lightblue` reads in both
modes. Text with no `color` elsewhere takes the font color of the node, and
then that of the theme.

A label of HTML on an edge, a cluster or the graph draws as plain text.

### Ports

An end of an edge can name a port, a side, or both: `a:p`, `a:n` or
`a:p:e`. A port is a field of a record or a cell or a table of a label of
HTML with a `port`. A side is `n`, `ne`, `e`, `se`, `s`, `sw`, `w`, `nw`,
`c` or `_`. An edge to a port ends at the border of the node, on the line
from the center of the port. An edge to a side ends on that side of the
port, or of the node when the end names no port, and leaves it in the
direction of the side.

### Issues

An attribute that the table does not list, a shape that zuzu does not
draw, and a part of a label of HTML that okage does not read, such as
`href` or `<img>`, are issues. A label of HTML that is not valid is an
error with its line and column.

## Mermaid

zuzu reads Mermaid 12.3.0. The test fixtures compare zuzu with `mmdc`, the
version that bundles Mermaid 12.3.0.

### Every kind

| Part | Example |
| --- | --- |
| front matter, between two lines of `---` | `title: Orders` and `config:` |
| a directive | `%%{init: {'flowchart': {'curve': 'linear'}}}%%` |
| a comment, from `%%` to the end of the line | `%% a comment` |
| the keyword of the kind, on the first line | `flowchart LR` |

The `title` of the front matter is the title of the diagram. Its `config`
and each directive merge into one config. A setting of the config that a
kind does not read is an issue.

### Flowchart

The keywords are `flowchart`, `graph` and `flowchart-elk`. The direction is
`TB`, `TD`, `BT`, `LR` or `RL`, and `TB` by default.

A statement is a chain of nodes and links. A semicolon or a new line ends
it.

| Part | Example |
| --- | --- |
| a node | `A`, `A[Text]`, `A["Text with (brackets)"]` |
| a chain | `A --> B --> C` |
| a group with `&` | `A & B --> C & D` |
| a subgraph | `subgraph id [Title]`, then `end` |
| a direction of a subgraph | `direction LR`. It is an issue, and the subgraph follows the graph |
| a class | `classDef hot fill:#f96,stroke:#333`, then `class A,B hot` or `A:::hot` |
| a style of a node | `style A fill:#f9f,stroke-width:2px` |
| a style of a link | `linkStyle 0,1 stroke:#f00`, `linkStyle default stroke:#999` |
| a curve of links | `linkStyle default interpolate linear` |
| an id of a link | `A e1@--> B` |

The shapes of a node:

| Source | Shape |
| --- | --- |
| `A[Text]` | rectangle |
| `A(Text)` | rounded rectangle |
| `A([Text])` | stadium |
| `A[[Text]]` | subroutine |
| `A[(Text)]` | cylinder |
| `A((Text))` | circle |
| `A(((Text)))` | double circle |
| `A>Text]` | flag |
| `A{Text}` | diamond |
| `A{{Text}}` | hexagon |
| `A[/Text/]`, `A[\Text\]` | parallelogram, to the right and to the left |
| `A[/Text\]`, `A[\Text/]` | trapezoid, wide at the bottom and wide at the top |

`A@{ shape: name, label: "Text" }` names a shape. zuzu draws the names that
the map `shapes` in `graph/shapes.go` lists, such as `rect`, `rounded`,
`stadium`, `subproc`, `cyl`, `circle`, `dbl-circ`, `diam`, `hex`,
`lean-r`, `lean-l`, `trap-b`, `trap-t`, `odd`, `tri`, `flip-tri`, `doc`,
`f-circ` and `text`. A shape that zuzu does not draw yet draws as a rounded
rectangle, and the report says so.

The links:

| Source | Link |
| --- | --- |
| `A --> B` | an arrow |
| `A --- B` | a line |
| `A -.-> B`, `A -.- B` | a dotted arrow and line |
| `A ==> B`, `A === B` | a thick arrow and line |
| `A ~~~ B` | an invisible link, which still takes part in the layout |
| `A --o B`, `A --x B` | a circle and a cross at the end |
| `A <--> B`, `A o--o B`, `A x--x B` | an end at each side |
| `A -->|Text| B`, `A -- Text --> B` | a link with text |
| `A ---> B`, `A ----> B` | a longer link, which spans more ranks |

The text of a label can hold `<br>` as a new line, an entity such as
`#quot;` or `#35;`, and a string of markdown in backquotes, which draws as
plain text without its marks.

An edge to the id of a subgraph draws to the first node of the subgraph,
and the report says so. Mermaid draws it to the box of the subgraph.

These parts are issues: `click`, a callback and `href`, `accTitle` and
`accDescr`, the properties of a link such as `e1@{ animate: true }`, the
layout `elk`, which draws with `zuzu.layered`, the look `handDrawn`, the
`theme` and the `themeVariables` of Mermaid, which draw in the look of zuzu,
and a style that is not `fill`, `stroke`, `stroke-width`, `color`,
`stroke-dasharray`, `font-weight` or `font-size`.

### Icons and images

A node can hold an icon or an image:

```
A@{ icon: "fa:user", form: square, label: "User", pos: t, h: 48 }
B@{ img: "https://example.com/a.png", label: "Logo", w: 60, h: 60, constraint: on }
C[fa:fa-gear Settings]
```

`form` is `square`, `circle` or `rounded`, a box around the icon, or none.
`pos` puts the label above the icon, `t`, or below it, `b`. `h` is the size
of the icon, 48 by default. `fa:fa-name` in a label draws the icon
`fa:name` before the text.

An icon of one color whose elements are paths, circles, ellipses, rects,
lines, polylines and polygons, all filled with `currentColor` or all
stroked with it in the same way, draws as a path in the color of the theme.
Another icon draws as an image, an SVG in a URL of data, which the SVG
writer writes once in its definitions. A missing pack or icon draws as a
box with a question mark.

An image comes through the loader of data. zuzu reads a PNG, a JPEG and an
SVG. It cleans an SVG first: it drops scripts, `foreignObject`, the
handlers of events, and every reference outside the image. zuzu never
writes a URL from outside into its SVG. `w` and `h` give the size, the size
of the image by default, and `constraint: on` keeps its aspect.

The packs and the icon loader are in `docs/API.md`, under "Icons".

### Charts

The charts compile to Vega-Lite or Vega, as D31 says. A setting of the
config that a chart does not read is an issue, and so are `accTitle` and
`accDescr` where a chart does not use them, the `theme`, the `themeCSS`,
and the look `handDrawn`. Each chart has about 5 fixtures in
`graph/mermaid/testdata`, and the gantt has 16. A test draws each fixture
and checks that each value of the source draws once, with its label.

| Keyword | Compiles to |
| --- | --- |
| `pie` | an arc of Vega-Lite |
| `xychart`, `xychart-beta` | bars and lines of Vega-Lite |
| `quadrantChart` | points of Vega-Lite on two axes from 0 to 1, with the four quadrants |
| `gantt` | bars of Vega-Lite on a time axis, with the plan worked out in Go |
| `journey` | points of Vega-Lite of the score of each step |
| `wardley-beta` | points, lines and text of Vega-Lite |
| `sankey`, `sankey-beta` | Vega with `zuzu.sankey` and `zuzu.sankeylinks` |
| `radar-beta` | Vega, polygons on radial axes |
| `treemap`, `treemap-beta` | Vega with `stratify` and `treemap` |

#### pie

`showData`, and `title` on the first line or on a line of its own. A slice
is `"label" : value`. With `showData`, the legend shows each value. zuzu
reads `pie.textPosition`, `pie.donutHole`, `pie.legendPosition` and the
colors `pie1` and on. A negative value is an error, and a second slice with
the same label is an issue.

#### xychart

`horizontal` or `vertical` after the keyword, `title`, an `x-axis` of
categories, `[a, b, c]`, or of a range, `"title" 1 --> 10`, a `y-axis` with
a title and a range, and series of `bar` and `line`, each with a name or
not. A value of a line can have a label, `[540 "PaLM"]`. zuzu reads the
settings `xyChart.width`, `height`, `chartOrientation`, `showLegend`,
`showTitle`, `showDataLabel`, `showDataLabelOutsideBar`, and the colors of
`plotColorPalette`.

#### quadrantChart

`title`, `x-axis` and `y-axis` with one end or both, `low --> high`,
`quadrant-1` to `quadrant-4`, and points, `Name: [x, y]`, with `radius`,
`color`, `stroke-color` and `stroke-width`. `classDef` and `:::class` style
points. A style of the point wins over its class. A point outside 0 to 1
is an error. zuzu reads `chartWidth`, `chartHeight`, `pointRadius`,
`xAxisPosition` and `yAxisPosition`.

#### gantt

| Statement | What zuzu does |
| --- | --- |
| `dateFormat` | the format of the dates of the source, in the tokens of Day.js, `YYYY-MM-DD` by default |
| `axisFormat` | the format of the time axis, in the directives of d3 |
| `tickInterval` | a tick at each step of a unit, such as `1week` |
| `excludes` | `weekends`, the names of days, and dates |
| `includes` | dates that `excludes` leaves in |
| `weekend` | `friday` or `saturday` |
| `inclusiveEndDates` | an end date that holds its whole day |
| `topAxis` | the axis at the top |
| `section` | a group of tasks, in its own color |
| a task | `Name : tags, id, start, end` |

A task names its start as a date, as `after a b`, or not at all, which
starts it at the end of the task before it. It names its end as a date, as
a duration such as `3d`, `12h` or `1.5w`, or as `until a b`. The tags are
`done`, `active`, `crit`, `milestone` and `vert`. zuzu works out the start
and the end of each task in Go, and a test compares them with mermaid.js.
A cycle of `after` and `until` is an error. `click`, the mode `compact`,
and `todayMarker` with a style are issues. A render has no time of its own,
so zuzu draws no marker of today.

#### journey

`title`, `section`, and steps, `Name: score: actor, actor`, with a score
from 1 to 5. A score outside 1 to 5 is an error.

#### wardley-beta

A place is `[visibility, evolution]`, each from 0 to 1. zuzu reads `title`,
`size`, `evolution` with the names of its stages, `anchor`, `component`
with `label` offsets and the decorators `build`, `buy`, `outsource`,
`market` and `inertia`, the links `->`, `-->`, `-.->`, `+>`, `+<`, `+<>`
and `+'text'>` with `; label`, `evolve`, `pipeline`, `note`,
`annotation`, `annotations`, `accelerator` and `deaccelerator`.

#### sankey

Lines of CSV, `source,target,value`, with quotes as RFC 4180 says. zuzu
reads `sankey.width`, `height`, `nodeAlignment`, `nodeWidth`,
`nodePadding`, `showValues`, `prefix`, `suffix`, `nodeColors` and
`linkColor`. A `linkColor` of `gradient` draws in the color of the source,
and the report says so.

#### radar-beta

`title`, `axis` with ids and labels, `a["Label"]`, on one line or more,
`curve` with values by place, `{1, 2, 3}`, or by axis, `{a: 1, b: 2}`,
`min`, `max`, `ticks`, `showLegend`, and `graticule` `circle` or
`polygon`. zuzu reads the sizes and the margins of `radar`,
`axisScaleFactor`, `axisLabelFactor`, `curveTension`, and the colors
`cScale0` and on when each curve has one.

#### treemap

Names in quotes, nested by their indent with spaces or tabs, and a value
on a leaf, `"Leaf": 12`. `classDef` and `:::class` style a node with
`fill`, `stroke`, `stroke-width` and `color`. zuzu reads `valueFormat`,
`showValues`, `padding`, `diagramPadding`, `labelFontSize` and
`valueFontSize`. A value on a section with children, and a leaf with no
value, are issues.

### Other kinds

Every other kind of Mermaid is an error that wraps `mermaid.ErrKind`. D31
names the order of the next kinds.

## The look

A node is a shape with a tint of its color and an outline of 1.25. An edge
is a line of 1.25 in the color of the axis, with a small arrowhead. A label
is 11 pixels. A cluster is a panel with its title at the top left. A color
that the source gives replaces the color of the theme. The styles are in
`theme/config.go`, from `zuzu.node` to `zuzu.icon`.
