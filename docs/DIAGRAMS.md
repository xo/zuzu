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
A `rank` of the graph, such as `rank=same` before `{a -- b}`, is the rank
of each subgraph that starts after it and sets no rank of its own, as in
dot. Another `rank` is an issue.

An ID is a name, a numeral, a string in double quotes, or a string of HTML
in angle brackets. Strings joined with `+` are one string. Comments are
`//`, `/* */`, and a line that starts with `#`.

### Attributes

| Of | Attribute | What zuzu does |
| --- | --- | --- |
| the graph | `label` | the title, or a title of HTML at the top left |
| the graph | `rankdir` | `TB`, `BT`, `LR` or `RL` |
| the graph | `ranksep`, `nodesep` | the space between the ranks and between the nodes, in inches |
| the graph | `rank` | the rank of the subgraphs that start after it, as above |
| the graph | `newrank` | nothing. zuzu always ranks the nodes of a cluster with the nodes of the whole graph, as dot does with `newrank=true` |
| the graph | `splines` | curved or straight edges. Another value is an issue |
| the graph | `layout` | `dot`. Another layout is an issue |
| the graph, a node, an edge, a cluster | `colorscheme` | the color scheme of its colors, as below |
| a node | `label` | the label, with `\n`, `\l` and `\r` as new lines and `\N` as the name, or a label of HTML, as below |
| a node | `shape` | the shape, as below |
| a node | `sides`, `skew`, `distortion`, `orientation`, `regular` | the polygon of the shape `polygon`, as below. Another shape draws without `sides`, `skew` and `distortion`, as in Graphviz. An `orientation` that is not 0, or `regular=true`, on another shape is an issue |
| a node | `width`, `height`, `fixedsize` | the size, in inches. With `fixedsize=true` or `shape`, the node has that size. Without it, the node is at least that size and grows to fit its label |
| a node | `xlabel` | a label outside the node, at a corner, as below |
| a node | `margin` | the space around the label, `x` or `x,y` in inches. A label of HTML in a node with no shape has `0.11,0.055` by default, and `plain` has 0 |
| a node | `style` | `filled`, `radial`, `dashed`, `dotted`, `bold`, `rounded` |
| a node | `color`, `fillcolor`, `fontcolor`, `penwidth`, `fontsize` | the paint. The size of the node fits its label at its `fontsize`. `setlinewidth(n)` in `style` is the old form of `penwidth` |
| a node | `gradientangle` | the angle of a fill of two colors, as below |
| an edge | `label` | the label, or a label of HTML |
| an edge | `headlabel`, `taillabel` | the labels at the ends, as below |
| an edge | `xlabel` | a label beside the middle of the edge, as below |
| an edge | `labelfontcolor`, `labelfontsize` | the paint of the labels at the ends. They take `fontcolor` and `fontsize` by default |
| an edge | `labeldistance`, `labelangle` | the place of the labels at the ends, as below |
| an edge | `arrowhead`, `arrowtail`, `dir` | the arrowheads, as below. `dir` is `forward`, `back`, `both` or `none` |
| an edge | `arrowsize` | the scale of the arrowheads |
| an edge | `minlen`, `weight`, `constraint` | the ranks that the edge spans, its weight, and whether it takes part in the ranks |
| an edge | `headport`, `tailport` | the ports, as below |
| an edge | `color`, `penwidth`, `style`, `fontcolor`, `fontsize` | the paint |
| a cluster | `label` | the title, or a title of HTML |
| a cluster | `style`, `color`, `fillcolor`, `bgcolor`, `fontcolor`, `penwidth`, `pencolor`, `gradientangle` | the paint |

A color is a name of CSS or of the X11 scheme of Graphviz, `#rrggbb`,
`#rrggbbaa`, or three numbers of hue, saturation and value. The X11 scheme
adds the numbered colors, such as `salmon2` and `lightblue4`, `gray0` to
`gray100`, `grey0` to `grey100`, and names such as `navyblue`. A name that
CSS also has, such as `gray`, `green`, `maroon` or `purple`, keeps its
color of CSS, as the SVG of Graphviz does. The case of a name does not
matter.

### Color schemes

zuzu reads the color schemes of Graphviz 16.1.1: `x11`, `svg`, and the 265
schemes of ColorBrewer, such as `blues9`, `rdylgn11` and `set312`. A color
of a scheme of ColorBrewer is its index from 1.

| Form | What zuzu reads |
| --- | --- |
| `/rdylgn11/1` | the first color of the scheme `rdylgn11` |
| `colorscheme=blues9` and `color=3` | the third color of `blues9` |
| `colorscheme=blues9` and `color=red` | `red`, a color of the default scheme |
| `/x11/green` | `green` of X11, which differs from `green` of CSS |
| `/svg/green` | `green` of CSS |
| `colorscheme=svg` | the names of CSS only, so that `salmon2` is an issue |

The `colorscheme` of a node, an edge or a cluster sets the scheme of its
colors and of the colors of its label of HTML. A cluster takes the
`colorscheme` of the graph around it. A node does not take the
`colorscheme` of its graph or its cluster, as in Graphviz. Use
`node [colorscheme=...]` for that. An index outside its scheme, a scheme
that zuzu does not know in the form `/scheme/index`, and a name that is not
in its named scheme, such as `/greens3/red`, are issues. A list of colors,
such as `/blues9/2:/reds9/8` or `1:2`, reads each color in this way.

### Gradients

A fill of two colors, such as `fillcolor="red:blue"`, is a gradient, as in
Graphviz. A node, a cluster, and a table or a cell of a label of HTML can
have one:

| Form | What zuzu draws |
| --- | --- |
| `a:b` | a linear gradient from `a` at the left to `b` at the right |
| `gradientangle=90` | the line of the gradient turns counterclockwise by that angle in degrees, so 90 goes from the bottom to the top |
| `a;0.3:b` or `a:b;0.7` | a hard change from `a` to `b` at 0.3 of the way |
| `style=radial` | a radial gradient from `a` at the center to `b` at the edge. With a `gradientangle`, the center of `a` moves toward that angle |

`style=radial` fills the node or the cluster, as `filled` does. A color
after the second, as in `a:b:c`, does not draw, as in Graphviz, and it is
an issue. A weight that is not a number from 0 to 1 is an issue. The text
on a gradient takes the color that reads on the mean of its two colors.
The gradients draw in the SVG and in the PNG.

The shapes are `box`, `rect`, `rectangle`, `square`, `ellipse`, `oval`,
`circle`, `doublecircle`, `point`, `diamond`, `hexagon`, `octagon`,
`pentagon`, `triangle`, `invtriangle`, `house`, `invhouse`, `trapezium`,
`invtrapezium`, `parallelogram`, `cylinder`, `note`, `tab`, `folder`,
`plaintext`, `plain`, `none`, `underline`, `record`, `Mrecord`,
`Mdiamond`, `Msquare`, `Mcircle` and `polygon`.

`Mdiamond` is a diamond and `Msquare` is a square, each with a short line
across each corner. `Mcircle` is a circle with a line across its top and
its bottom.

`polygon` reads its attributes as Graphviz documents them:

| Attribute | What zuzu draws |
| --- | --- |
| `sides` | the number of sides, 4 by default. The base is flat. A polygon of fewer than 3 sides is an ellipse |
| `skew` | a slant. A positive skew moves the top to the right, and a negative skew moves it to the left |
| `distortion` | a change of width from top to bottom. A positive distortion makes the top wider than the bottom |
| `orientation` | a turn of the polygon, in degrees, counterclockwise |
| `regular` | with `true`, the node is as wide as it is high |

The polygon fills the box of the node. The node is the polygon of the
least area that holds its label at its center, and it grows across at
least as much as down. An edge ends on the outline of the polygon.

### Ranks and order

The layout follows dot in these steps:

1. An edge that closes a cycle turns around. A search goes from each node
   in the order of the source, along the edges in the order of the
   source, and turns around each edge back to a node on its path. So a
   node with no edge in it stays in the first rank.
2. An edge between two nodes of the same rank and the same cluster puts
   its tail before its head in the rank, such as the edges of a row of
   `rank=same {a -- b -- c}`. A node with no edge to the next rank goes
   beside the other end of such an edge.
3. The `weight` of an edge pulls its ends into one line across the
   ranks. A node with edges to several nodes of the rank before it
   lines up with the median of their places, and each edge counts by its
   weight. `weight=0` counts as 1.
4. A loop and its label take room at the side of their node, so that no
   other node or label covers them.

The flowcharts, the state, the class and the entity relationship diagrams
of Mermaid use the same layout. Mermaid turns cycles around in the same
way.

### Arrowheads

An arrowhead is a name of the `arrowType` of Graphviz: one to four shapes
in a row, such as `crowodot` or `teetee`. The first shape is at the node.
The shapes are:

| Shape | What zuzu draws |
| --- | --- |
| `normal`, `inv` | a triangle that points to the node, or away from it |
| `vee` | a triangle with a notch, that points to the node |
| `crow` | a crow's foot of three lines, open toward the node |
| `tee` | a bar across the edge |
| `box`, `diamond`, `dot` | a square, a diamond or a circle |
| `curve`, `icurve` | an arc across the edge, that bends toward the node, or away from it |
| `none` | a gap in a row of shapes, and no arrowhead when it is alone |

Before a shape, `o` draws it open, and `l` or `r` keeps only its left or
its right half, as seen along the edge toward the node. `o` changes only
`normal`, `inv`, `box`, `diamond` and `dot`, as in Graphviz. The old names
`empty`, `invempty`, `open`, `halfopen` and `ediamond` draw as `onormal`,
`oinv`, `vee`, `lvee` and `odiamond`. Another name, or a row of more than
four shapes, is an issue, and draws as `normal`.

An entity relationship diagram in crow's foot notation uses `teeodot` for
zero or one, `teetee` for one, `crowodot` for zero or more, and `crowtee`
for one or more. A class diagram of UML uses `empty` for a generalization,
`diamond` and `odiamond` for a composition and an aggregation, and `vee`
for a dependency.

### Labels at the ends and xlabels

`headlabel` and `taillabel` are labels at the head and at the tail of an
edge. Each sits beside the edge, just past its end, at the right of the
edge as it runs from the tail to the head. If that place covers a node,
another label or an edge, the label goes to the left. The space between
two ranks grows to hold the labels at the ends of the edges between them.
If an edge sets `labeldistance` or `labelangle`, the center of each label
is 10 times `labeldistance` points from the end, at `labelangle` degrees
counterclockwise from the edge, as in dot. The defaults are 1 and -25.

`xlabel` is a label that takes no space in the ranks, as in dot. The xlabel
of a node sits outside a corner of the node, first the top left. The
xlabel of an edge sits beside the middle of the edge. Each goes to the
first place where it covers no node, no other label and no edge.

### Records

A node of the shape `record` or `Mrecord` is a box of fields:
`label="<f0> left|<f1> mid|<f2> right"`. The fields lie across the ranks,
side by side when the ranks run down and one under another when they run
across. A field of a nested group, `{b|{c|d}}`, is a field of the record,
in order. `<name>` names a port, and an edge that names the port ends at
its field. `\{`, `\}`, `\|`, `\<`, `\>` and `\ ` are text.

A record whose label is one group in braces, `{a|b|c}`, lays its fields
one under another when the ranks run down, as in Graphviz. When the ranks
run across, its fields also lie one under another, and not side by side
as in Graphviz.

A record can have a label of HTML, as a class of UML of the gallery of
Graphviz does:

```dot
Class1 [shape=record, label=<{<b>I/O class</b> | + property<br align="left"/>...<br align="left"/>}>]
```

The braces and the bars outside the tags split the fields, and the text of
each field is HTML. A line of a field draws in its font: bold, italic or
with a line under it. A field whose lines each end in `<br align="left"/>`
draws them at the left of the field. A label of HTML with no brace and no
bar, such as a table, draws in the shape of the record, as a label of HTML
does. A field of HTML has no port, as in Graphviz. A table in a field, a
line in more than one font, a font with a color, a size or a face, a line
at the right, and a field with lines at the left and at the center are
issues.

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
| `bgcolor`, `color` | the fill and the border. A cell with no color takes that of its table, and then the color of the node. A `bgcolor` of two colors is a gradient, as above |
| `gradientangle` | the angle of the gradient of a table or a cell |
| `align`, `balign`, `valign` | the place of the content in a cell |
| `colspan`, `rowspan`, `width`, `height`, `fixedsize` | the size of a cell or a table |
| `sides` | the sides of a cell that have a border |
| `style` | `rounded`, `radial`, `dashed` and `dotted` |
| `rows="*"`, `columns="*"` | a rule between each two rows or columns |
| `port` | a port, as below |

A table in a node with no shape, such as `shape=none` or `shape=plaintext`,
takes the fill of a node of the theme. Text with no `color` on a cell with a
`bgcolor` takes the text color of the light theme or of the dark theme,
whichever reads on the fill, so that a header in `lightblue` reads in both
modes. Text with no `color` elsewhere takes the font color of the node, and
then that of the theme.

A label of HTML on an edge, at an end of an edge, in an xlabel, on a
cluster or on the graph draws the same way. Its table takes no fill unless
it sets a `bgcolor`. A label of HTML on the graph takes the size of the
title of a chart.

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
draw, a name that is not an arrowhead, a number that does not read, and a
part of a label of HTML that okage does not read, such as `href` or
`<img>`, are issues. A label of HTML that is not valid is an
error with its line and column.

`URL`, `href` and `tooltip` on a node or an edge are issues. Vega has the
encodings `href` and `tooltip` for them. The scene graph of zuzu holds no
link and no tooltip, and the SVG writer writes neither, so zuzu has no
place to put them.

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
and each directive merge into one config, and a directive wins over the
front matter. A setting of the config that a kind does not read is an
issue. The front matter is YAML. zuzu reads maps by their indent, and a
value in quotes or plain. A value can run over more lines, indented under
its key. A block after `|` keeps its new lines, and a block after `>` joins
its lines with spaces. A `-` or a `+` after either sets the new lines at
its end, as in YAML. A `#` after a space, outside quotes, starts a comment
that runs to the end of the line, as in YAML. The other parts of YAML, such
as lists and anchors, do not read.

The key `config` in a directive sets the section of the config of the kind,
as Mermaid 12 does. In a sequence diagram,
`%%{init: {'config': {'wrap': true}}}%%` sets `sequence.wrap`. The section
takes the name that Mermaid gives to the kind:

| Keyword | Section |
| --- | --- |
| `flowchart`, `graph` | `flowchart` |
| `sequenceDiagram` | `sequence` |
| `erDiagram` | `er` |
| `classDiagram`, `classDiagram-v2` | `classDiagram` |
| `stateDiagram`, `stateDiagram-v2` | `stateDiagram` |
| a keyword that ends in `-beta`, such as `xychart-beta` | the keyword without `-beta`, such as `xychart` |
| any other keyword, such as `gantt` | the keyword |

Mermaid reads its class diagram from the section `class` and its state
diagram from the section `state`, and so the key `config` sets nothing in
them, or in an xychart, whose section is `xyChart`. zuzu reports such a
setting, as it reports any setting that it does not read.

### Settings that change nothing

These settings change nothing in a still image. zuzu accepts them in every
kind, with no issue:

| Setting | What it does in Mermaid |
| --- | --- |
| `logLevel` | sets what Mermaid writes to the log of the browser |
| `securityLevel` | sets which scripts, links and tags of HTML a page can run |
| `useMaxWidth`, at the top or in a section, such as `flowchart.useMaxWidth` | makes the SVG as wide as the page that holds it. zuzu writes an SVG of a fixed size |
| `handDrawnSeed` | seeds the random lines of the look `handDrawn`, which is an issue of its own |
| `architecture.seed`, `architecture.randomize` | seed and start the layout of forces of an architecture diagram. The layout of zuzu uses no random numbers |
| `class.htmlLabels` | nothing. Mermaid 12 does not read it |
| `htmlLabels`, `flowchart.htmlLabels`, in a chart other than a mindmap | nothing. A chart draws its text in SVG |

zuzu matches the names of these settings in any case, so `loglevel` is
`logLevel`. Mermaid matches a name in its own case, and ignores a key such
as `loglevel`. Either way, the image is the same. `mmdc` draws each of
these settings with no change to the image, except `useMaxWidth`, which
changes only the scale of the PNG of `mmdc`.

`htmlLabels` changes the wrap of a label in a graph and in a mindmap. A
graph reads it, as "Labels that wrap" says. A mindmap does not read it, and
it is an issue there.

### Flowchart

The keywords are `flowchart`, `graph` and `flowchart-elk`. The direction is
`TB`, `TD`, `BT`, `LR` or `RL`, and `TB` by default.

A statement is a chain of nodes and links. A semicolon or a new line ends
it. A semicolon in quotes, in the text of a link between pipes, or at the
end of an entity such as `#quot;` does not end it. Statements can follow
the direction on the first line, as in `graph TB;A-->B`. A chain goes on
over a line break, as in Mermaid, when a line ends with a link or the next
line starts with one. A link at the end of the source, with no node after
it, is an error, as in Mermaid.

| Part | Example |
| --- | --- |
| a node | `A`, `A[Text]`, `A["Text with (brackets)"]` |
| a chain | `A --> B --> C` |
| a group with `&` | `A & B --> C & D` |
| a subgraph | `subgraph id [Title]`, then `end` |
| a direction of a subgraph | `direction LR`, inside the subgraph |
| a class | `classDef hot fill:#f96,stroke:#333`, then `class A,B hot` or `A:::hot` |
| a style of a node | `style A fill:#f9f,stroke-width:2px` |
| a style of a link | `linkStyle 0,1 stroke:#f00`, `linkStyle default stroke:#999` |
| a curve of links | `linkStyle default interpolate linear` |
| an id of a link | `A e1@--> B`, `A e1@-- Text --> B` |
| a class of a link | `class e1 hot`, after the link with the id `e1` |
| the text for a screen reader | `accTitle: Text`, `accDescr: Text`, `accDescr { ... }` |

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
`f-circ` and `text`, with the aliases of Mermaid 12. zuzu also draws these
named shapes of Mermaid 12:

| Name and aliases | What zuzu draws |
| --- | --- |
| `lin-doc`, `lined-document` | a document with a second line down its left side |
| `tag-doc`, `tagged-document` | a document with a line across its bottom right corner |
| `docs`, `documents`, `st-doc`, `stacked-document` | a stack of three documents |
| `brace`, `brace-l`, `comment` | a curly brace on the left of the label, with no fill |
| `brace-r` | a curly brace on the right of the label, with no fill |
| `braces` | a curly brace on each side of the label, with no fill |
| `notch-rect`, `card`, `notched-rectangle` | a rectangle with its top left corner cut off |
| `notch-pent`, `loop-limit`, `notched-pentagon` | a rectangle with its two top corners cut off |
| `lin-rect`, `lined-rectangle`, `lined-process`, `lin-proc`, `shaded-process` | a rectangle with a second line down its left side |
| `div-rect`, `div-proc`, `divided-rectangle`, `divided-process` | a rectangle with a line across its top |
| `win-pane`, `internal-storage`, `window-pane` | a rectangle with a line across its top and a line down its left side |
| `tag-rect`, `tagged-rectangle`, `tag-proc`, `tagged-process` | a rectangle with a line across its bottom right corner |
| `st-rect`, `procs`, `processes`, `stacked-rectangle` | a stack of three rectangles |
| `sl-rect`, `manual-input`, `sloped-rectangle` | a rectangle with a top that slopes up to the right |
| `datastore`, `data-store` | a line above and a line below the label |
| `delay`, `half-rounded-rectangle` | a rectangle with a round right end |
| `curv-trap`, `curved-trapezoid`, `display` | a point on the left and a round right end |
| `bow-rect`, `stored-data`, `bow-tie-rectangle` | a round left end and a right end that curves in |
| `h-cyl`, `das`, `horizontal-cylinder` | a cylinder on its side |
| `lin-cyl`, `disk`, `lined-cylinder` | a cylinder with a second line under its top |
| `flag`, `paper-tape` | a band with a wavy top and bottom |
| `bucket` | a bucket with an open top |
| `console` | a rounded rectangle with `>_` at its top left |
| `browser` | a rounded rectangle with the bar of a window |
| `cloud` | a cloud |
| `bang` | a spiky burst |
| `fork`, `join` | a filled bar across the flow, 70 by 10, with no text |
| `hourglass`, `collate` | an hourglass of 30 by 30, with no text |
| `bolt`, `com-link`, `lightning-bolt` | a lightning bolt of 35 by 70, with no text |
| `cross-circ`, `summary`, `crossed-circle` | a circle of 40 with a cross in it, with no text |
| `fr-circ`, `stop`, `framed-circle` | a ring of 14 with a dot in it, with no text |
| `subprocess` | a subroutine |
| `directory` | a folder |
| `bpmn-activity` | a rounded rectangle |
| `bpmn-annotation` | a square bracket on the left of the label |

A shape with no text drops its label, as Mermaid does. An edge ends on the
box of the node, except for `cross-circ`, `fr-circ`, `cloud` and `bang`,
where it ends on the ellipse. A shape that zuzu does not draw yet draws as a
rounded rectangle, and the report says so. These are `person`, and the
shapes of BPMN that put their label under the shape: `bpmn-start`,
`bpmn-intermediate`, `bpmn-boundary`, `bpmn-end`, `bpmn-gateway`,
`bpmn-data` and `bpmn-data-store`.

The links:

| Source | Link |
| --- | --- |
| `A --> B` | an arrow |
| `A --- B` | a line |
| `A -.-> B`, `A -.- B`, `A .-> B`, `A .- B` | a dotted arrow and line |
| `A ==> B`, `A === B` | a thick arrow and line |
| `A ~~~ B` | an invisible link, which still takes part in the layout |
| `A --o B`, `A --x B` | a circle and a cross at the end |
| `A <--> B`, `A o--o B`, `A x--x B` | an end at each side |
| `A -->|Text| B`, `A -- Text --> B`, `A-- Text ---B` | a link with text |
| `A -. Text .-> B`, `A -. Text -.-> B`, `A == Text ==> B` | a dotted and a thick link with text |
| `A ---> B`, `A ----> B`, `A -..-> B` | a longer link, which spans more ranks |

A dotted link is a dash, if any, then dots, then a dash, with a head after
it if any. Each dot makes it longer. Mermaid reads the mark before a link,
`<`, `o` or `x`, only when it is the head at the other end, as in `<-->`
or `o.-o`. Another mark, as in `<---`, draws no arrow at the start and
makes a link with no text one rank longer. A link with text whose marks do
not agree, as in `<-- Text ---`, draws as `-->`, as in Mermaid.

The text of a label can hold `<br>` as a new line, an entity such as
`#quot;` or `#35;`, and a string of markdown in backquotes, which draws as
plain text without its marks. A text in double quotes, such as a string of
markdown, can run over more lines, and each new line in it starts a new
line of the label. This holds for a node, a link and the title of a
subgraph. The value of a property in `A@{ ... }` can be in double quotes or
in single quotes, and zuzu takes the quotes off. In single quotes, `''` is
one quote, as in YAML. A long label wraps, as "Labels that wrap" says.

A subgraph with a direction lays out in that direction, as Mermaid does,
when no edge links a node inside it to a node outside it. zuzu lays out
such a subgraph on its own, and then places it as one node in the layout
that holds it. A subgraph inside it without a direction lays out in the
same direction. If an edge links a node inside the subgraph to a node outside
it, the subgraph follows the direction of the layout that holds it.
Mermaid does the same, so this is not an issue.

An edge to the id of a subgraph ends at the box of the subgraph, as in
Mermaid. If no edge links a node inside the subgraph to a node outside it,
the subgraph is one node of the layout, and the edge goes to that node.
Otherwise the layout takes the edge to the first node of the subgraph,
and the edge stops where it meets the box. Mermaid picks that first node
by its own order, so the layout around such a subgraph can differ from the
layout of Mermaid.

A class of a link sets its style as `linkStyle` does, and `linkStyle` goes
over it. `accTitle` and `accDescr` join into the description of the Vega
spec, as in a state diagram.

zuzu reads these settings of the section `flowchart` of the config:

| Setting | Default of Mermaid | What zuzu does |
| --- | --- | --- |
| `curve` | `basis` | draws the links with the curve of Vega of that name |
| `wrappingWidth` | 120 | wraps the label of a node, as "Labels that wrap" says |
| `htmlLabels` | true | the old form of `htmlLabels`, as "Labels that wrap" says |
| `nodeSpacing` | 50 | sets the space between two nodes of a rank to 24/50 of it |
| `rankSpacing` | 50 | sets the space between two ranks to 40/50 of it |
| `diagramPadding` | 8 | sets the space around the diagram to it |
| `titleTopMargin` | 25 | sets the space between the title and the diagram to 10/25 of it |
| `subGraphTitleMargin` | `top: 0`, `bottom: 0` | puts `top` above the title of each subgraph and `bottom` under it |
| `inheritDir` | false | gives each subgraph with no direction the direction of the flowchart |

The look of zuzu is smaller than the look of Mermaid. A space of the
config takes the ratio of the default of zuzu to the default of Mermaid, so
the default of Mermaid draws the default of zuzu. The default space between
two nodes of a rank is 24 in zuzu, and between two ranks it is 40. The
default space between the title and the diagram is 10, and the default
space around the diagram is 8, as in Mermaid. The spaces of the config also
hold in a subgraph that lays out on its own. Mermaid 12.3.0 does the same,
but it adds 25 to the space between the ranks of such a subgraph. Mermaid
takes 0 for `nodeSpacing` and `rankSpacing` as the default, and zuzu does
too. A value that is not a number of 0 or more is an issue, and the space
keeps its default.

Without `inheritDir`, a subgraph with no direction takes the direction of
the subgraph that holds it, as the subgraphs above say. With
`inheritDir: true`, it takes the direction of the flowchart, as in Mermaid.

These parts are issues: `click`, a callback and `href`, the properties of
a link such as `e1@{ animate: true }`, the
layout `elk`, which draws with `zuzu.layered`, the look `handDrawn`, the
`theme` and the `themeVariables` of Mermaid, which draw in the look of zuzu,
the other settings of the section `flowchart`, such as `minNodeWidth`, and
a style that is not `fill`, `stroke`, `stroke-width`, `color`,
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

### State diagram

The keywords are `stateDiagram` and `stateDiagram-v2`. Both read the same
statements. The direction is `TB` by default, and `direction LR` on a line
of its own sets it. A statement ends at a new line.

| Part | Example |
| --- | --- |
| a state | `Still`, `Still:::hot` |
| a state with a description | `state "Waiting for input" as Wait`, `Wait : press a key`, `state "Waiting" as Wait: press a key` |
| a transition | `Still --> Moving` |
| a transition with text | `Still --> Moving : push` |
| the start and the end | `[*] --> Still`, `Still --> [*]` |
| a composite state | `state Moving { ... }`, `state "Text" as Moving { ... }`, and the brace can be on the next line |
| a choice | `state check <<choice>>`, `state check [[choice]]` |
| a fork and a join | `state split <<fork>>`, `state merge <<join>>` |
| regions of a composite state | `--` on a line of its own, inside the braces |
| a note | `note right of Still : text`, `note left of Still`, then lines of text, then `end note` |
| a direction | `direction LR`, at the top or inside a composite state |
| a class | `classDef hot fill:#f96,stroke:#333`, then `class Still,Moving hot` or `Still:::hot` |
| a style of a state | `style Still fill:#f9f,stroke-width:2px` |
| a comment | `%% a comment` |
| the text for a screen reader | `accTitle: Doors`, `accDescr: The states of a door`, `accDescr { ... }` |

zuzu draws each part in its own look:

| Part | Draws as |
| --- | --- |
| a state | a rounded rectangle with its description, or its id when it has none |
| a state with two descriptions or more | a rounded rectangle with the first description as a title, a line under it, and the rest |
| the start `[*]` | a filled circle of 14 pixels |
| the end `[*]` | a ring and a dot of 14 pixels |
| a choice | a diamond of 28 pixels |
| a fork and a join | a filled bar of 70 by 10 pixels, which turns to 10 by 70 when the ranks run to the left or to the right |
| a composite state | a cluster with its title |
| a region | a cluster with no title and a dashed border, beside the other regions of its composite state |
| a note | a node of the shape note, with a dotted line to its state |
| a transition | an edge with an arrowhead, and its text in the middle |

Each scope has its own start and its own end, as in Mermaid. A scope is the
diagram, a composite state, or a region. `[*]` in a scope ends at the start
or the end of that scope. A transition to the id of a composite state ends
at its box.

A note on the right of a state has a line from the state to the note, and
a note on the left has a line from the note to the state, as in Mermaid.
The layout puts a note on the right in the rank after its state, and a note
on the left in the rank before it. Mermaid 12 does the same. A state keeps
one note, the last.

`classDef`, `class`, `:::` and `style` read the same styles as a flowchart.
A class or a style on a composite state paints its cluster. `accTitle` and
`accDescr` join into the description of the Vega spec, as in a gantt.
`hide empty description` is a line of the first version of the state
diagram, and Mermaid draws nothing for it, so zuzu reads it and draws
nothing too.

A line `state` and words with no brace draws nothing, as in Mermaid. The
words are not a state, so `state Idle` and `state Idle: text` add no state
and no description. The state draws only when a transition or another line
names it. Thus `state Idle: text` draws the name `Idle`, and `Idle : text`
draws the description `text`. `state fork_state &lt;&lt;fork&gt;&gt;` is
not a fork, and `fork_state` draws as a state. In `state "Text" as id:
more`, `more` is a second description, up to the next colon.
`state "Text" as id`, or `state id`, opens a composite state when the next
line starts with a brace. A blank line between them is an error, as in
Mermaid.

These parts are issues: `scale`, `click`, a floating note with no state,
such as `note "text" as N1`, a kind of state other than `choice`, `fork`
and `join`, which draws as a state, the settings of the section `state` of
the config other than `wrappingWidth`, the layout `elk`, the look `handDrawn`, the `theme` and the
`themeVariables` of Mermaid, and a style that a flowchart does not read.

### Class diagram

The keywords are `classDiagram` and `classDiagram-v2`. Both read the same
statements. The direction is `TB` by default, and `direction LR` on a line
of its own sets it. A statement ends at a new line.

| Part | Example |
| --- | --- |
| a class | `class Animal`, or a line that names it, such as `Animal --> Duck` |
| a class with a label | `class Animal["An animal"]` |
| a class in backticks | `` class `Animal Class` `` |
| a generic class | `class Square~Shape~`, which draws as `Square<Shape>` |
| a member on a line of its own | `Animal : +int age`, `Animal : +isMammal()` |
| members in a body | `class Duck {`, then a member on each line, then `}` |
| a method | a member with parentheses, `+swim()`, with a return type after them, `+getPoints() List~int~` |
| the visibility of a member | `+`, `-`, `#` and `~` before it, drawn as written |
| a static member | `$` after the member or after the parentheses, `+area()$ double` |
| an abstract member | `*` after the member or after the parentheses, `+draw()*` |
| an annotation | `<<interface>>` in a body, or `<<interface>> Shape` on a line of its own |
| a relation | `A <\|-- B`, with an end on each side or none |
| the ends of a relation | `<\|` and `\|>`, `*`, `o`, `<` and `>`, and `()` for a lollipop |
| the line of a relation | `--` for a solid line and `..` for a dashed line |
| a relation both ways | `A <\|--\|> B`, `A *--* B` |
| a cardinality | `A "1" --> "*" B` |
| the label of a relation | `A --> B : uses` |
| a namespace | `namespace Shapes {`, then classes, then `}` |
| a namespace in a namespace | `namespace Shapes.Round {`, then classes, then `}` |
| a note | `note "text"`, and `note for Duck "text"` |
| a class of style | `classDef hot fill:#f96`, then `cssClass "A,B" hot`, `class A:::hot` or `A:::hot` |
| a style of a class | `style Animal fill:#f9f,stroke:#333` |
| the setting `class.hideEmptyMembersBox` | `true` draws a class with no members as its name alone |
| a comment | `%% a comment` |
| the text for a screen reader | `accTitle: Zoo`, `accDescr: The animals`, `accDescr { ... }` |

A text in double quotes or in backticks can run over more lines, and each
new line draws. The text of a member can hold an entity of HTML, such as
`&lt;`, and the entities of a label, such as `#lt;`.

zuzu draws each part in its own look:

| Part | Draws as |
| --- | --- |
| a class | a box of three parts with a line between each two: the name, the attributes and the methods. An empty part is a thin band |
| the name | the annotations, each as `«interface»`, over the name in bold |
| a member | a line of text from the left. A static member has a line under it, and an abstract member is italic |
| a method | its name and its parentheses, then ` : ` and its return type when it has one |
| the end `<\|` or `\|>` | the open triangle `empty`, for inheritance and realization |
| the end `*` | the filled diamond `diamond`, for composition |
| the end `o` | the open diamond `odiamond`, for aggregation |
| the end `<` or `>` | the open arrowhead `vee`, for association and dependency |
| the end `()` | an open circle, `odot`, at an interface. The interface draws as its name with no box |
| a relation | an edge from the class on the left to the class on the right, solid or dashed, with its label in the middle |
| a cardinality | a label at its end of the edge, beside the line |
| a namespace | a cluster with its title |
| a namespace with dots in its name | a cluster for each part of the name, each in the one before it, with its part as its title |
| a note | a node of the shape note. A note for a class has a dotted line from the note to the class |

The ends of a relation draw at 1.4 times the size of an arrowhead of a
flowchart, so that the marks of UML read at the size of a label. The
layout puts the class on the left of a relation in the rank before the
class on the right, as Mermaid does. A class that a `class` statement
names inside a namespace moves into it. `classDef`, `cssClass`, `:::` and
`style` read the same styles as a flowchart, and `classDef default`
styles every class. `accTitle` and `accDescr` join into the description
of the Vega spec.

A namespace `Company.Engineering` is a cluster `Engineering` in a cluster
`Company`, as in Mermaid. Another namespace with the same first part, or a
namespace `Company`, goes in the same cluster. The setting
`class.hierarchicalNamespaces: false` draws each namespace as one cluster
with its whole name as its title, as Mermaid does.

These parts are issues: `click`, `link` and `callback`, the settings of
the section `class` of the config other than `hideEmptyMembersBox` and
`hierarchicalNamespaces`, the
layout `elk`, the look `handDrawn`, the `theme` and the `themeVariables` of
Mermaid, and a style that a flowchart does not read.

### Entity relationship diagram

The keyword is `erDiagram`. The direction is `TB` by default, and
`direction LR` on a line of its own sets it. A statement ends at a new
line.

| Part | Example |
| --- | --- |
| an entity | `CUSTOMER`, or a line that names it, such as `CUSTOMER \|\|--o{ ORDER : places` |
| an entity in double quotes | `"Customer Account"` |
| an alias | `p[Person]`, `p["A person"]` |
| attributes | `CUSTOMER {`, then an attribute on each line, then `}` |
| an attribute | a type and a name: `string name`, `string(99) note`, `string[] parts` |
| the keys of an attribute | `PK`, `FK`, `UK` after the name, one or more, as `PK, FK` |
| the comment of an attribute | `string name PK "the full name"` |
| a relationship | `CUSTOMER \|\|--o{ ORDER : places`, with an optional label after the colon |
| zero or one | `\|o` or `o\|`, `one or zero`, `zero or one` |
| exactly one | `\|\|`, `one`, `only one`, `1` |
| zero or more | `}o` or `o{`, `zero or more`, `zero or many`, `many`, `many(0)`, `0+` |
| one or more | `}\|` or `\|{`, `one or more`, `one or many`, `many(1)`, `1+` |
| an identifying relationship | `--` or `to`, a solid line |
| a relationship that does not identify | `..`, `.-`, `-.` or `optionally to`, a dashed line |
| a subgraph | `subgraph domain`, `subgraph domain["The domain"]` or `subgraph "The domain"`, then entities, then `end` |
| a direction of a subgraph | `direction TB` inside it |
| a class of style | `classDef hot fill:#f96`, then `class A,B hot` or `A:::hot` |
| a style of an entity | `style CUSTOMER fill:#f9f,stroke:#333` |
| a comment | `%% a comment` |
| the text for a screen reader | `accTitle: Orders`, `accDescr: The orders`, `accDescr { ... }` |

zuzu draws each part in its own look:

| Part | Draws as |
| --- | --- |
| an entity with attributes | a box with the name in bold at the top, then a row for each attribute, with a line between each two |
| an attribute | its type, its name, its keys and its comment in columns, each from the left, with a line between each two. A column that is empty in every row of the entity takes no space |
| an entity with no attributes | a box with its name |
| a relationship | an edge from the first entity to the second, solid or dashed, with its label in the middle |
| zero or one | the arrowhead `teeodot`: a bar at the entity, then an open circle |
| exactly one | the arrowhead `teetee`: two bars |
| zero or more | the arrowhead `crowodot`: a crow's foot at the entity, then an open circle |
| one or more | the arrowhead `crowtee`: a crow's foot at the entity, then a bar |
| a subgraph | a cluster with its title |

Each row of an attribute has a port with the name of the attribute. The
ends of a relationship draw at 1.4 times the size of an arrowhead of a
flowchart. The cardinality `u` of Mermaid draws no mark in Mermaid 12, so
zuzu reads it and draws no mark either. A relationship to the id of a
subgraph ends at its box. An entity that a subgraph names on a line of
its own, or with its attributes, moves into the subgraph. `classDef`,
`class`, `:::` and `style` read the same styles as a flowchart.
`accTitle` and `accDescr` join into the description of the Vega spec.

These parts are issues: `click`, the settings of the section `er` of the
config, the layout `elk`, the look `handDrawn`, the `theme` and the
`themeVariables` of Mermaid, and a style that a flowchart does not read.

### Labels that wrap

Mermaid 12.3.0 wraps a long label of a flowchart, a state diagram, a class
diagram and an entity relationship diagram. zuzu wraps the same labels at
the same widths. It measures the words with the bundled font, so the lines
are the same on each machine.

| Label | Width |
| --- | --- |
| a node of a flowchart | `flowchart.wrappingWidth`, 120 pixels by default |
| a state, and a note of a state | `state.wrappingWidth`, 120 pixels by default |
| the text of a link, a transition, a relation and a relationship | 200 pixels |
| a subgraph, a composite state, a state with more than one description, a class, a note of a class, an entity and its attributes | no wrap |

A label wraps at the spaces between its words. A line that is wider than
the width breaks before the first word that does not fit. Each line of a
`<br>` wraps on its own. A word that is wider than the width stays whole
on a line of its own, and the label is wider than the width. An icon in
the line of a label, such as `fa:fa-gear`, takes its width and 4 pixels
from the width. A string of markdown wraps as plain text does.

zuzu reads two settings of the top level of the config:

1. `htmlLabels: false` makes Mermaid draw its labels as SVG text, which
   breaks a word that is wider than the width between two letters. zuzu
   breaks such a word too.
2. `markdownAutoWrap: false` keeps a string of markdown on one line, as
   Mermaid does. With `htmlLabels: false`, Mermaid ignores it, and so does
   zuzu.

`flowchart.htmlLabels` is the old form of `htmlLabels`. When `htmlLabels`
is not set, Mermaid 12 reads `flowchart.htmlLabels` in its place in every
kind of graph, and so does zuzu. Another value of one of these settings, such as a width that is not a
positive number, is an issue, and the label takes the default.

A width is in the pixels of zuzu, which draws a label at 11 pixels.
Mermaid draws a label at 14 pixels, so a line of zuzu holds more words. A
node of zuzu fits its widest line. Mermaid makes the box of a label that
wraps as wide as the width. DOT does not wrap a label.

### Charts

The charts compile to Vega-Lite or Vega, as D31 says. A setting of the
config that a chart does not read is an issue, and so are `accTitle` and
`accDescr` where a chart does not use them, the `theme`, the `themeCSS`,
and the look `handDrawn`. A `pie`, an `xychart`, a `quadrantChart`, a
`gantt`, a `journey`, a `sequenceDiagram` and an `architecture-beta` join
`accTitle` and `accDescr` into the description of the spec. Each chart has about 5 fixtures in
`graph/mermaid/testdata`, and the gantt has 18. A test draws each fixture
and checks that each value of the source draws once, with its label.

| Keyword | Compiles to |
| --- | --- |
| `pie` | an arc of Vega-Lite |
| `xychart`, `xychart-beta` | bars and lines of Vega-Lite |
| `quadrantChart` | points of Vega-Lite on two axes from 0 to 1, with the four quadrants |
| `gantt` | bars of Vega-Lite on a time axis, with the plan worked out in Go |
| `journey` | points of Vega-Lite of the score of each step |
| `wardley-beta` | points, lines and text of Vega-Lite |
| `sankey`, `sankey-beta` | Vega with `zuzu.sankey`, `zuzu.sankeylinks` and `zuzu.sankeylabels` |
| `radar-beta` | Vega, polygons on radial axes |
| `treemap`, `treemap-beta` | Vega with `stratify` and `treemap` |
| `sequenceDiagram` | Vega, with the layout worked out in Go: rects, rules, paths, symbols and text |
| `mindmap` | Vega paths and text, laid out in Go: the root in the center and a tidy tree on each side |
| `architecture-beta` | Vega rects, paths, text and icons, laid out in Go on a grid by the sides of the edges |

#### pie

`showData`, and `title` on the first line or on a line of its own. A slice
is `"label" : value`. With `showData`, the legend shows each value. zuzu
reads `pie.textPosition`, `pie.donutHole`, `pie.legendPosition` and the
colors `pie1` and on. A negative value is an error, and a second slice with
the same label is an issue.

`pie.highlightSlice` names a slice that reaches 5 percent past the rim of
the pie, as in Mermaid. The value `hover` highlights the slice under the
pointer. An image has no pointer, so `hover` is an issue.

The percent of a slice goes on the middle angle of the slice, with the
center of the text on that point. The setting `pie.zuzu.textPlacement`
gives the distance of that point from the center of the pie:

| Value | The distance |
| --- | --- |
| `bisector`, the default | `pie.textPosition` of the radius, 0.75 by default, as Mermaid does |
| `centroid` | the center of mass of the slice. A wide slice puts its label near the center of the pie |
| `circle` | the center of the largest circle in the slice. A slice of 180 degrees or more puts its label halfway to its edge, and a thin slice puts its label near its edge |

In a donut, `centroid` and `circle` keep the label in the ring. With
`centroid` or `circle`, `pie.textPosition` is an issue.

A label can be too large for its slice. zuzu measures the box of each
label in the bundled font. The box fits if all of it lies within the ring
and between the two edges of the slice, with a margin of 2 pixels. If the
box does not fit, the label goes outside the pie:

1. The label goes in a column 18 pixels out from the right side of the
   pie when the middle angle of its slice points right, and else in a
   column at the left side.
2. In each column, the labels sort from the top by the height of the bend
   of their leader lines. Each label moves down to clear the label above
   it by 2 pixels.
3. If the last label of a column leaves the chart at the bottom, the
   column moves up, as far as the top of the chart.
4. A leader line of 1 pixel, in the color of the slice, leaves the rim at
   the middle angle of the slice. It bends 10 pixels out from the rim, and
   stops 3 pixels before the label.

#### xychart

`horizontal` or `vertical` after the keyword, `title`, an `x-axis` of
categories, `[a, b, c]`, or of a range, `"title" 1 --> 10`, a `y-axis` with
a title and a range, and series of `bar` and `line`, each with a name or
not. A value of a line can have a label, `[540 "PaLM"]`. zuzu reads the
settings `xyChart.width`, `height`, `chartOrientation`, `showLegend`,
`showTitle`, `showDataLabel`, `showDataLabelOutsideBar`, and the colors of
`plotColorPalette`.

zuzu also reads these settings, and gives each to the setting of Vega-Lite
with the same meaning. The `xAxis` of Mermaid is the axis of the
categories, which is at the left of a horizontal chart.

| Setting of `xyChart` | Setting of Vega-Lite |
| --- | --- |
| `titleFontSize` | the `fontSize` of the title |
| `titlePadding` | the `offset` of the title |
| `xAxis.labelFontSize`, `yAxis.labelFontSize` | the `labelFontSize` of the axis |
| `xAxis.titleFontSize`, `yAxis.titleFontSize` | the `titleFontSize` of the axis |
| `xAxis.axisLineWidth` | the `domainWidth` of the axis of categories |

These settings are issues, because the look of zuzu has no place for them.
An axis has no ticks, so `tickLength` and `tickWidth` are issues. The
labels of an axis are 8 pixels from the plot, and the title of an axis is
at its end, so `labelPadding` and `titlePadding` of an axis are issues. An
axis of numbers has no axis line, so `axisLineWidth` on it is an issue.
`width` and `height` give the size of the plot, so
`plotReservedSpacePercent` is an issue.

With `showDataLabel`, the value of a bar is 10 pixels from the end of the
bar, inside it or outside it, as in Mermaid. In a horizontal chart, the
value is on the middle line of the bar.

Each axis, `xAxis` and `yAxis`, reads the settings that show its parts:

| Setting | What zuzu does |
| --- | --- |
| `showLabel: false` | the axis has no labels |
| `showTitle: false` | the axis has no title |
| `showAxisLine: false` | the axis of categories has no axis line. An axis of numbers has none in the look of zuzu |
| `showTick: false` | nothing, because an axis has no ticks in the look of zuzu |
| `labelRotation` | the labels turn by that many degrees, clockwise |

As in Mermaid, only the axis at the bottom turns its labels: the x axis
of a vertical chart, and the y axis of a horizontal one. A rotation
outside -90 to 90 degrees is 0. `showTick: true`, and `showAxisLine: true`
on an axis of numbers, are issues.

A bar or a line with more values than the x axis has categories draws a
value for each category. The values past the last category are an issue.
Mermaid does not draw them either.

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
| `section` | a group of tasks, in its own color, with its name at the left |
| a task | `Name : tags, id, start, end` |

A task names its start as a date, as `after a b`, or not at all, which
starts it at the end of the task before it. It names its end as a date, as
a duration such as `3d`, `12h` or `1.5w`, or as `until a b`. The tags are
`done`, `active`, `crit`, `milestone` and `vert`. zuzu works out the start
and the end of each task in Go, and a test compares them with mermaid.js.
A cycle of `after` and `until` is an error. `click` and `todayMarker` with
a style are issues. A render has no time of its own, so zuzu draws no
marker of today. A gantt with no task draws nothing but
its title, as in Mermaid.

zuzu reads these settings of `gantt`, and `displayMode` at the top level
of the front matter too:

| Setting | What zuzu does |
| --- | --- |
| `axisFormat`, `tickInterval`, `topAxis`, `weekday` | as the statements of the same names |
| `displayMode: compact` | the tasks of each section pack into as few rows as their times allow |
| `numberSectionStyles` | the sections take that many colors in turn |
| `rightPadding` | the padding at the right of the chart, in pixels |

In the mode `compact`, the tasks of a section go in the order of their
starts, as in Mermaid. Each task takes the first row of its section whose
last task ends at or before its start. A row holds more than one task, so
the axis writes no names. The name of each task goes in the middle of its
bar, if it is no wider than the bar. Else it goes 4 pixels past the end of
the bar, or 4 pixels before the start of the bar if it does not fit at the
right of the axis. The name of a milestone goes beside its diamond. A name
has a halo, so that it reads on a bar and on the grid.

A long gantt follows two limits of Mermaid. If the tasks span more than 5
whole years, zuzu draws no bands of the excluded days. If `tickInterval`
gives more than 10000 ticks, the axis takes ticks of its own. Mermaid
counts a month as 30 days for this. Each limit is an issue. A bar is at
least 2 pixels wide, so that a short task in a long gantt still draws.

As in Mermaid, the value of a statement is all the text after its keyword.
In `dateFormat :YYYY-MM-DD`, the colon is part of the format, so no date
of the source reads in it. A start date then reads as the Date of
JavaScript reads it. An end date that is not a duration does not read, so
the task has no length, and that is an issue. An item of `excludes` or
`includes` that is not a date, a day of the week or `weekends` excludes or
includes no day. Each statement with such items is one issue.

A section draws as Mermaid draws it, in the look of a group of a
hierarchy. Every other section, from the first, has a band behind its
rows. The band starts at the left of the names
of the sections and ends at the end of the time axis. It is a tint of the
color of the section, so that the grid and the names of the tasks show
through it. The name of each section is at the left of the names of its
tasks, in the middle of its rows, 11 pixels and bold. The names take the
place of the legend. A gantt with no named section draws no band and no
name.

#### journey

`title`, `section`, and steps, `Name: score: actor, actor`, with a score
from 1 to 5. A score outside 1 to 5 is an error. A score that is not a
number, such as `size: 2rem`, is an issue. The step draws with its name
and no face, and Mermaid draws its face in no place. A user journey with no
step draws its title alone, as in Mermaid.

The actors of a step go under its name, on one line that ends with an
ellipsis when it is too long. Mermaid writes the actors in a legend, and
wraps the name of each actor at `journey.maxLabelWidth`. zuzu has no
legend of the actors. With `journey.maxLabelWidth`, the actors under a step
wrap at that width, or at the width of the column of the step if it is
less, by the rule of Mermaid. A line takes words while it fits. A word that
is wider than the width breaks into parts, and each part but the last ends
with a hyphen. zuzu measures the text in the bundled font.

#### wardley-beta

A place is `[visibility, evolution]`, each from 0 to 1. zuzu reads `title`,
`size`, `evolution` with the names of its stages, `anchor`, `component`
with `label` offsets and the decorators `build`, `buy`, `outsource`,
`market` and `inertia`, the links `->`, `-->`, `-.->`, `+>`, `+<`, `+<>`
and `+'text'>` with `; label`, `evolve`, `pipeline`, `note`,
`annotation`, `annotations`, `accelerator` and `deaccelerator`. The name
of an accelerator or a deaccelerator is in double quotes or not, as
`deaccelerator License Play [0.13, 0.78]`.

#### sankey

Lines of CSV, `source,target,value`, with quotes as RFC 4180 says. zuzu
reads `sankey.width`, `height`, `nodeAlignment`, `nodeWidth`,
`nodePadding`, `showValues`, `prefix`, `suffix`, `nodeColors`, `linkColor`
and `labelStyle`.

A link takes the color of its source by default. `linkColor` can be
`source`, `target`, a color, or `gradient`. A link with `gradient` goes
from the color of its source at the left to the color of its target at
the right, as in Mermaid.

`labelStyle: outlined` gives each label a halo, and changes the side of a
label as Mermaid does. The label of a node in a layer before the layer of
the largest node goes at the left of the node, and the others at the right.
The default style is `legacy`.

A label goes on the inner side of its node, as Mermaid puts it: at the
right of a node in the left half, and at the left of the others. Mermaid
draws a label over another label or a node when they meet. zuzu does not.
The transform `zuzu.sankeylabels` measures each label, and moves a label
that hits to the other side, then up or down beside its node. As a last
step, it cuts the end of the label off with an ellipsis. D30 gives the
rule.

#### radar-beta

`title`, `axis` with ids and labels, `a["Label"]`, on one line or more,
`curve` with values by place, `{1, 2, 3}`, or by axis, `{a: 1, b: 2}` or
`{a 1, b 2}`, on one line or more up to the closing brace,
`min`, `max`, `ticks`, `showLegend`, and `graticule` `circle` or
`polygon`. zuzu reads the sizes and the margins of `radar`,
`axisScaleFactor`, `axisLabelFactor`, `curveTension`, and the colors
`cScale0` and on when each curve has one.

#### treemap

Names in quotes, nested by their indent with spaces or tabs, and a value
on a leaf, `"Leaf": 12`. `classDef` and `:::class` style a node with
`fill`, `stroke`, `stroke-width`, `stroke-dasharray` and `color`. Mermaid
splits a style at commas, so the lengths of `stroke-dasharray` have spaces
between them, as `stroke-dasharray:5 3`. zuzu reads `valueFormat`,
`showValues`, `padding`, `diagramPadding`, `labelFontSize` and
`valueFontSize`. A value on a section with children, and a leaf with no
value, are issues.

The name of a leaf that does not fit gets smaller, as in Mermaid 12. zuzu
measures each name and each value in the bundled font, and the spec works
out the sizes with Vega expressions after the layout of the treemap:

1. The name starts at `labelFontSize`. It gets smaller by 1 pixel at a time
   until it fits the width of the leaf, less 6 pixels on each side.
2. Then it gets smaller until the name, the gap and the value fit the
   height of the leaf, less 6 pixels at the top and at the bottom. The
   value and the gap shrink with the name. With `showValues: false`, only
   the name must fit.
3. In a treemap of more than 20 leaves, the smallest size is 4. A name
   that does not fit at 4 is cut at the edge of its leaf, with no ellipsis.
4. In a treemap of 20 leaves or fewer, the smallest size is 8. A name that
   does not fit at 8 is not drawn.
5. A leaf whose space for text is less than 8 pixels wide or high, or 10
   in a treemap of 20 leaves or fewer, draws no name.
6. The value is not drawn when its name is not drawn, when it is wider
   than the leaf, or when it ends less than 4 pixels from the bottom of the
   leaf. Its smallest size is 4, or 6 in a treemap of 20 leaves or fewer.

#### sequenceDiagram

zuzu lays out a sequence diagram in Go. Each participant has a lifeline,
and the messages, the notes and the blocks go down the page in the order of
the source. The spec holds the place of each part as rows of data.

A participant that a line names again with no alias keeps its kind, as in
Mermaid. A line with an alias gives it a new kind and a new label.

| Statement | What zuzu draws |
| --- | --- |
| `participant A`, `actor B`, `participant A as Alice` | a participant as a box, or an actor as a figure over its name. An alias is the label, and `<br/>` starts a new line |
| `participant A@{ "type": "database" }` | `boundary`, `control` and `entity` as icons over the name, and `database`, `collections` and `queue` as boxes of their shape. The key `alias` gives the label, and `as` wins over it |
| `create participant B`, `destroy B` | the head of B at the message that creates it, and its foot at the message that destroys it. The next message after `create` goes to B, and the next message after `destroy` names B. As in Mermaid, a `create` goes first: when a `create` follows a `destroy`, the message after them is for the `create`. B is then destroyed only if the next message names it, and else its lifeline runs to the end |
| `box Aqua Title`, then `end` | a panel behind its participants, with its title. The color is the first word: a name of CSS, `rgb()`, `rgba()`, `hsl()` or `hsla()` |
| `A->>B: Text` | a line with its text above it, as the next table says |
| `A->>+B`, `B-->>-A`, `activate A`, `deactivate A` | an activation, a narrow bar on the lifeline. An activation inside another moves to the right |
| `Note right of A`, `Note left of A`, `Note over A`, `Note over A,B` | a note in a panel |
| `loop`, `alt` and `else`, `opt`, `par` and `and`, `critical` and `option`, `break` | a frame with its keyword in a tab and its text in brackets. A section is a dashed line across the frame |
| `rect rgb(191, 223, 255)`, then `end` | a band of the color behind its messages |
| `autonumber`, `autonumber 10 5`, `autonumber off` | the number of each message in a circle at its start |
| `title Text`, `title: Text` | the title |
| `accTitle: Text`, `accDescr: Text`, `accDescr { ... }` | the description of the Vega spec, the texts joined in order |
| `%% Text`, `# Text` | nothing, since each is a comment |
| `#35;`, `#infin;` | the character of the entity code. `#59;` puts a semicolon in a text |

A semicolon ends a statement. The text of a message, a note or a label can
start with `wrap:` or `nowrap:`.

| Arrow | Line | End |
| --- | --- | --- |
| `->`, `-->` | solid, dotted | none |
| `->>`, `-->>` | solid, dotted | a filled arrowhead |
| `<<->>`, `<<-->>` | solid, dotted | a filled arrowhead at each end |
| `-x`, `--x` | solid, dotted | a cross |
| `-)`, `--)` | solid, dotted | an open arrowhead |
| `-\|\`, `-\|/`, `--\|\`, `--\|/` | solid, dotted | the top or the bottom half of an arrowhead |
| `-\\`, `-//`, `--\\`, `--//` | solid, dotted | one stroke of an arrowhead, at the top or at the bottom |
| a reverse arrow, such as `//-` | solid, dotted | the same halves and strokes at the sender |
| `A->>()B`, `A()->>B`, `A()->>()B` | as the arrow says | a dot where the line meets the lifeline, a central connection |

The reverse arrows are `/|-`, `\|-`, `//-` and `\\-`, and `/|--`, `\|--`,
`//--` and `\\--` with a dotted line.

zuzu reads these settings of `sequence`: `mirrorActors`, which is true by
default, `showSequenceNumbers`, `hideUnusedParticipants`, `rightAngles`,
`wrap`, `wrapPadding`, `actorMargin`, `messageMargin`, `noteMargin`,
`boxMargin`, `boxTextMargin`, `activationWidth`, `width` and `height` as
the least size of a box, `labelBoxWidth`, `labelBoxHeight`,
`diagramMarginX`, `diagramMarginY`, `messageAlign`, `noteAlign`, and the
size and the weight of the fonts of the actors, the messages and the notes.
It reads the `wrap` of the top level too. A size is in pixels, and the
default sizes are those of the look of zuzu, which are smaller than those
of Mermaid. The families of the fonts are issues, as are the other
settings.

The menus of a participant, `link`, `links`, `properties` and `details`,
are issues. A type or a key of the
metadata that zuzu does not know is an issue, and the participant draws as
a box. A color of a rect that does not read is an issue, and the rect draws
as a panel.

Each participant takes the next color of the scheme. A message is an edge,
and a note, a frame and a box without a color are panels. A color that the
source gives to a box or a rect draws at its own opacity, or at 0.25 when
it is opaque, so that the text on it reads in both modes.

#### mindmap

The indent of each line makes the tree. The first node is the root. Each
other node is a child of the last node above it with a smaller indent. A
node with no such node above it is a second root, which is an error. A tab
counts as 4 spaces.

| Source | Shape |
| --- | --- |
| `text` | the default shape: a tint with a line under it |
| `id[text]` | a square |
| `id(text)` | a rounded square |
| `id((text))` | a circle |
| `id))text((` | a bang |
| `id)text(` | a cloud |
| `id{{text}}` | a hexagon |

The text of a node can be in double quotes, or a string of markdown in
double quotes and backquotes, which can run over more lines. In every
label, as in Mermaid, `**bold**` and `__bold__` draw in a bold font, and
`*italic*` and `_italic_` in an italic font. An underscore inside a word is
text. Text between two backquotes is code, and it draws as it is written,
with its backquotes and its marks. `<br>` and a new line in a string of
markdown start a new line. A label wraps at the spaces to fit
`mindmap.maxNodeWidth`, 200 by default.

The text in a shape can run over more lines, up to the line that ends the
shape, as `root(`, then `The root`, then `)`. zuzu drops the spaces at the
start and the end of each line and the blank lines. As in Mermaid, the
text draws on one line when it fits, and else each line of the source
starts a new line.

| Line after a node | What zuzu does |
| --- | --- |
| `::icon(fa fa-book)` | the icon of the node, at the left of its label |
| `:::urgent large` | an issue for each class |

zuzu reads the classes of a font of icons as the name of an icon:
`fa fa-book` is `fa:book`, `fas fa-book` is `fas:book`, and `mdi mdi-home`
is `mdi:home`. A name with a colon, such as `carbon:cloud`, stays as it is.
The icon comes from the packs and the icon loader of the render, as in a
flowchart. An icon that does not load draws as a box with a question mark,
and the report names it. An icon whose classes name no pack is an issue.
The classes of a node are classes of CSS that the page of a site gives.
zuzu reads no CSS, so each class is an issue.

zuzu reads `mindmap.padding`, `mindmap.maxNodeWidth`, `mindmap.colorBy`,
which is `branch` or `depth`, and `mindmap.layoutAlgorithm` and `layout`,
which are `cose-bilkent` or `tidy-tree`. Both draw with the layout of zuzu.

Mermaid lays a mindmap out with cose-bilkent, a layout of forces around
the root. zuzu uses a tidy tree on two sides instead. The layout is the
same on each run, it keeps the order of the source, and each label stays
level, so that no two labels overlap. A radial tree puts long labels at a
slant or lets them overlap. The root is in the center. Its first branches
go to its right side, from the top down, and the rest go to its left side,
from the bottom up, so that the branches read clockwise. The split makes
the heights of the two sides as close as it can. On each side, each
subtree takes a band of the height, and a node is in the middle of the band
of its subtree. A child is at a fixed space from the side of its parent.
zuzu measures the labels with the bundled font, as the render does.

The root takes the first color of the category scheme of the theme. Each
branch of the root takes the next color, and each node and edge of the
branch takes the color of the branch. With `colorBy: depth`, each depth
takes a color. A node is a tint of its color with an outline, and a node of
the default shape has a line under it in place of the outline. An edge is
a curve in the color of its branch, and it gets thinner at each depth.

#### architecture-beta

| Statement | Example |
| --- | --- |
| a group, with an icon, a title and a parent group | `group api(cloud)[API] in cloud` |
| a service, with an icon, a title and a group | `service db(database)[Database] in api` |
| a service with a text in place of an icon | `service erp "ERP"[Legacy system]` |
| a junction | `junction hub in api` |
| an edge, with the sides it joins | `db:L -- R:server` |
| an arrow at an end | `a:R --> L:b`, `a:R <-- L:b`, `a:R <--> L:b` |
| a title on an edge | `a:R -[reads]- L:b` |
| an edge from the box of a group | `api{group}:B --> T:db{group}` |
| an alignment | `align row a b c`, `align column a b` |
| a title | `title Services` |

A side is `L`, `R`, `T` or `B`. A title in brackets can be in quotes. An
edge joins two services or junctions. These are errors, as in Mermaid: a
second item with an id, a parent that is not a group, a parent that no
group statement declares, a group in a group that comes after it, an edge
to a group or to an id that does not exist,
`{group}` on an edge whose two ends are in one group, and an alignment of
fewer than two services or junctions. `{group}` on a service in no group
is an issue.

zuzu draws the built-in icons `cloud`, `database`, `disk`, `internet` and
`server` in its own look, with no pack (D31). Another icon is `pack:name`.
It comes from the packs and the icon loader of the render, as in a
flowchart. zuzu bundles no pack `logos`, because its logos of AWS and of
other vendors are trademarks (D31). A program that has the right to use
them gives them with `WithIcons` or an icon loader. An icon that does not
load draws as a box with a question mark, and the report names it. In the
strict mode, it is an error.

Mermaid lays the diagram out with fcose, a layout of forces, with the
sides of the edges as constraints. zuzu places the items on a grid instead,
so that the layout is the same on each run and each edge leaves and enters
the sides it names:

1. `a:R -- L:b` puts b in the column to the right of a, and `a:B -- T:b`
   puts b in the row below a.
2. An edge between a side across and a side up or down, such as
   `a:R -- T:b`, puts b to the right of a and below it, so that the edge
   enters the top of b. Mermaid places b above a for this edge.
3. Two ends on the same side, such as `a:R -- R:b`, put b below a, and the
   edge goes around. `T` and `B` put b to the right of a.
4. `align row` puts its items in a row, in their order, and
   `align column` puts them in a column.

zuzu walks the items that the edges join in depth, from the first item of
the source, so that a chain of items stays together. If the cell of an item
is full, the item goes on to the next free cell: down for an edge to a
side, to the right for an edge up or down, and on up or down for an edge
at a slant. Each set of joined items goes to the right of the set before.
A group is a box around a grid of its own, and it takes one cell in the
grid of its parent. An edge from an item in the group to an item outside
it places the group in that grid. A column takes the width of its widest
item, and the tiles of a row line up on their centers. An edge leaves its
side straight, and turns at right angles with round corners. An edge with
`{group}` ends on the box of the group.

A service is a square tile with a tint of the first color of the scheme
and an outline, with its icon in that color and its title below it. A
group is a panel with its icon and its title at the top left. A junction is
a dot. An edge and its arrows are in the color of the axis, and the title
of an edge sits on its middle. zuzu reads `architecture.iconSize`, the size
of a tile, 56 by default, `architecture.fontSize`, 11 by default,
`architecture.padding`, the space inside a group, 16 by default, and
`architecture.nodeSeparation`, the space between two cells, 40 by default.
A title on an edge widens that space to fit. The layout of zuzu uses no
random numbers, so it is the same on each run. zuzu accepts
`architecture.randomize` and `architecture.seed`, and they change nothing.
The other settings, which tune fcose, such as `numIter`, are issues.

### Other kinds

zuzu draws the flowchart, the state diagram, the class diagram, the entity
relationship diagram, and the charts above, which hold the sequence
diagram, the mindmap and the architecture diagram. Every other kind of
Mermaid is an error that wraps `mermaid.ErrKind`. The order of the next
kinds is open question 17 of `docs/PLAN.md`.

## The look

A node is a shape with a tint of its color and an outline of 1.25. An edge
is a line of 1.25 in the color of the axis, with a small arrowhead. A label
is 11 pixels. A cluster is a panel with its title at the top left. A color
that the source gives replaces the color of the theme. A node with an
opaque fill from the source and no color of text takes the text color of
the light theme or of the dark theme, whichever reads on the fill. The styles are in
`theme/config.go`, from `zuzu.node` to `zuzu.icon`.
