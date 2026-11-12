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
| the graph | `fontname` | the weight and the style of the title, as below |
| the graph, a node, an edge, a cluster | `colorscheme` | the color scheme of its colors, as below |
| the graph, a node, an edge, a cluster | `URL`, `href`, `tooltip` | the link and the tooltip, as "Links and tooltips" says |
| a node | `label` | the label, with `\n`, `\l` and `\r` as new lines and `\N` as the name, or a label of HTML, as below |
| a node | `shape` | the shape, as below |
| a node | `sides`, `skew`, `distortion`, `orientation`, `regular` | the polygon of the shape `polygon`, as below. Another shape draws without `sides`, `skew` and `distortion`, as in Graphviz. An `orientation` that is not 0, or `regular=true`, on another shape is an issue |
| a node | `width`, `height`, `fixedsize` | the size, in inches. With `fixedsize=true` or `shape`, the node has that size. Without it, the node is at least that size and grows to fit its label |
| a node | `xlabel` | a label outside the node, at a corner, as below |
| a node | `margin` | the space around the label, `x` or `x,y` in inches. A label of HTML in a node with no shape has `0.11,0.055` by default, and `plain` has 0 |
| a node | `style` | `filled`, `radial`, `dashed`, `dotted`, `bold`, `rounded` |
| a node | `color`, `fillcolor`, `fontcolor`, `penwidth`, `fontsize` | the paint. The size of the node fits its label at its `fontsize`. `setlinewidth(n)` in `style` is the old form of `penwidth` |
| a node | `gradientangle` | the angle of a fill of two colors, as below |
| a node, an edge, a cluster | `fontname` | the weight and the style of the label, as below |
| an edge | `label` | the label, or a label of HTML |
| an edge | `headlabel`, `taillabel` | the labels at the ends, as below |
| an edge | `xlabel` | a label beside the middle of the edge, as below |
| an edge | `labelfontcolor`, `labelfontsize`, `labelfontname` | the paint of the labels at the ends. They take `fontcolor`, `fontsize` and `fontname` by default |
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

### Font names

zuzu draws all text in IBM Plex Sans (D16). It reads a weight and a style
from the words of a name in `fontname`, and accepts the rest of the name
with no issue (D32). A word ends at each character that is not a letter or
a digit, such as a space, a dash or a colon, and before an upper case
letter that follows a lower case one. So `Times-BoldItalic` gives `Bold`
and `Italic`:

| Word | What zuzu draws |
| --- | --- |
| `Thin`, `Hairline` | the weight 100 |
| `ExtraLight`, `UltraLight` | the weight 200 |
| `Light` | the weight 300 |
| `Medium` | the weight 500 |
| `SemiBold`, `DemiBold`, `Demi`, `Bold`, `ExtraBold`, `Heavy`, `Black` | the weight 600, the heaviest weight that zuzu bundles |
| `Italic`, `Oblique`, `It` | italic. The bundled font slants only as italic |

`Helvetica`, `Times-Roman` and `Courier` draw in the regular weight. The
weights under 400 draw as 400 in the PNG, because zuzu bundles no lighter
weight. The SVG names the weight, and a viewer with a lighter weight of
Plex draws it.

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
5. Edges between the same two nodes spread apart. Straight edges from one
   node to the next, in either direction, run side by side, 14 pixels
   apart, each with ends of its own on the borders of the nodes. The gap
   narrows to fit a small node. An edge that passes other ranks bends
   away from the others at its middle.

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
for a dependency. A requirement diagram of Mermaid uses `circleplus`, a
circle with a cross in it, for the relation `contains`, and `vee` for each
other relation. `circleplus` stands alone, as `circle` and `cross` of
Mermaid do.

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
does. A field of HTML has no port, as in Graphviz. The `face` of a font
gives the line its weight and its style, as `fontname` does below. A table
in a field, a line in more than one font, a font with a color or a size,
a face with a weight other than bold, a line at the right, and a field
with lines at the left and at the center are issues.

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

`target` and the links and tooltips of the parts of an edge, such as
`headURL`, `labeltooltip` or `edgehref`, are issues.

### Links and tooltips

`URL` or `href` gives a link to the graph, a cluster, a node or an edge,
and `tooltip` gives it a tooltip (D32). `href` wins over `URL`, as in
Graphviz. The SVG writes the link as an `<a href>` around each shape of
the part, and the tooltip as a `<title>` inside it. A node puts them on its
shape and its labels. An edge puts them on its line, its arrowheads and
its labels. A cluster puts them on its box and its title. The graph puts
them on the group that holds the whole diagram. The image of `Image`
draws neither.

In a link and a tooltip, `\G` is the name of the graph, or of the cluster,
`\N` is the name of the node, `\E` is the edge, as `a->b`, `\T` and `\H`
are its tail and its head, and `\L` is the label. In a tooltip, `\n`,
`\l` and `\r` end a line.

A link with a scheme that zuzu does not write, such as `javascript:`, is an
issue, and the part has no link. "Links" in `docs/API.md` lists the schemes
that zuzu writes.

## Mermaid

zuzu reads Mermaid 12.3.0. The test fixtures compare zuzu with `mmdc`, the
version that bundles Mermaid 12.3.0.

### Every kind

| Part | Example |
| --- | --- |
| front matter, between two lines of `---` | `title: Orders`, `description: The orders of a shop` and `config:` |
| a directive | `%%{init: {'flowchart': {'curve': 'linear'}}}%%` |
| a comment, from `%%` to the end of the line | `%% a comment` |
| the keyword of the kind, on the first line | `flowchart LR` |

The `title` of the front matter is the title of the diagram. Its
`description` goes in the description of the Vega spec, which Mermaid
ignores (D32). Its `config` and each directive merge into one config, and
a directive wins over the front matter. Each other key of the front
matter goes in the config too, and a setting of the config that a kind
does not read is an issue. The gantt reads `displayMode` there. The front matter is YAML. zuzu reads maps by their indent, and a
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
| `requirementDiagram` | `requirement` |
| `stateDiagram`, `stateDiagram-v2` | `stateDiagram` |
| a keyword that ends in `-beta`, such as `xychart-beta` | the keyword without `-beta`, such as `xychart` |
| any other keyword, such as `gantt` | the keyword |

Mermaid reads its class diagram from the section `class` and its state
diagram from the section `state`, and so the key `config` sets nothing in
them, or in an xychart, whose section is `xyChart`. zuzu reports such a
setting, as it reports any setting that it does not read.

### The description

The description of the Vega spec is the text for a screen reader. It holds
these texts, in this order, with a space between two of them:

1. The `description` of the front matter.
2. The text of each `accTitle` and `accDescr`, in the order of the source.

A text that is empty adds nothing. A kind that does not read `accTitle`
and `accDescr`, such as the sankey, takes only the `description`. The
`description` can be a block of YAML. A number or a boolean of YAML is
text there.

### Hints of the look and the layout

`look: handDrawn` and `layout: elk` are hints of a way to draw (D32). zuzu
accepts them with no issue in every kind, in the config or at the top of
the front matter. zuzu draws its own look for `handDrawn`, and its own
layered layout, `zuzu.layered`, for `elk`. A look by hand, modeled on the
look of xkcd, and an ELK layout are work for later. The other looks, such
as `classic` and `neo`, draw in the look of zuzu too. A graph accepts
`layout: dagre`, and another layout, such as `elk.box` or `tidy-tree`, is
an issue. A mindmap accepts `cose-bilkent` and `tidy-tree` too.

### Settings that change nothing

These settings change nothing in a still image. zuzu accepts them in every
kind, with no issue:

| Setting | What it does in Mermaid |
| --- | --- |
| `logLevel` | sets what Mermaid writes to the log of the browser |
| `securityLevel` | sets which scripts, links and tags of HTML a page can run |
| `useMaxWidth`, at the top or in a section, such as `flowchart.useMaxWidth` | makes the SVG as wide as the page that holds it. zuzu writes an SVG of a fixed size |
| `handDrawnSeed` | seeds the random lines of the look `handDrawn`, which zuzu draws in its own look |
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

### Font names

zuzu draws all text in IBM Plex Sans (D16). It reads a weight and a style
from the name of a font family, as "Font names" of DOT says, and accepts
the rest of the name with no issue (D32). So `Helvetica-Bold` draws in the
weight 600, and `Times-Italic` draws in italic. These parts name a family:

| Part | What zuzu does |
| --- | --- |
| `fontFamily` at the top of the config | gives each node, link and subgraph of a flowchart, a state diagram, a class diagram and an entity relationship diagram the weight and the style of the name, and the participants, the messages and the notes of a sequence diagram. A style of a node, a link or a class wins over it |
| `sequence.actorFontFamily`, `sequence.messageFontFamily`, `sequence.noteFontFamily` | gives the weight and the style to the participants, the messages or the notes. Each wins over `fontFamily`, and `actorFontWeight` and the other weights win over each |
| `font-family` in a style, such as `style A font-family:Times-Bold` | gives the weight and the style to the label. `font-weight` and `font-style` in the same style win over it |
| another setting whose name ends in `FontFamily`, such as `usecase.actorFontFamily` | nothing |

A name with no weight and no style, such as `courier` or `Trebuchet MS`,
is no issue anywhere. A name with a weight or a style in a part that does
not read it, such as `fontFamily` in a pie, is an issue. `fontFamily` in
`themeVariables` is an issue with the rest of `themeVariables`.

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
| a curve of a link, by its id | `e1@{ curve: linear }` |
| a collapsed subgraph | `one@{ view: collapsed }` |
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
| `choice` | a diamond of 28 by 28, with no text |
| `anchor` | a filled dot of 2, with no text |
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

A link without an id has the id that Mermaid gives it: `L_A_B_0` for the
first link from `A` to `B`, then `L_A_B_2`, `L_A_B_3` and on. A link with
the id of a link before it takes such an id too. The id of a link between
two groups, as in `A & B e1@--> C & D`, names the link from the last node
before it to the first node after it, `B` to `C`. `class` and the
properties in `e1@{ ... }` name a link by its id, when no node has that
id. A class of a link sets its style as `linkStyle` does, and `linkStyle`
goes over it. `curve` sets the curve of the link, as `linkStyle` with
`interpolate` does, and the later of the two wins. The curves are `basis`,
`linear`, `cardinal`, `catmullRom`, `monotoneX`, `natural`, `step`,
`stepAfter` and `stepBefore`. Another curve, such as `bumpY`, is an issue.
mmdc 12.3.0 draws every link with round corners and does not read a
curve, so the curves of zuzu follow the documentation of Mermaid.
`accTitle` and `accDescr` join into the description of the Vega spec, as
"The description" says.

`animate: true` and `animation: fast` or `animation: slow` in the
properties of a link draw the link dashed, as one still frame of Mermaid
(D32). A still image has no motion. The dash is the dash of Mermaid, lines
of 9 pixels and gaps of 5, and it wins over the dash of a style, as in
Mermaid. `animate: false` draws the link solid. Another value of
`animation` is an issue. The style `animation` in a `classDef` or a
`linkStyle` of a link adds no issue and no dash, because the motion of a
style moves a dash that the style gives with `stroke-dasharray`. mmdc
12.3.0 draws such a link solid when its style gives no dash. The style
`animation` of a node or a subgraph is an issue, at the line of its
`classDef`.

`id@{ view: collapsed }` on the id of a subgraph draws the subgraph as one
node, as Mermaid 12 does. The node is a rounded box with the title of the
subgraph, and a dashed line and three dots under the title. The nodes and
the subgraphs inside it do not draw. A link to a node inside it goes to
the node, and a link between two nodes inside it does not draw. A
collapsed subgraph inside a collapsed subgraph goes into the outer one.
Another `view` draws the subgraph open, as in Mermaid. `view: collapsed`
on the id of a node that is not a subgraph is an issue.

A style reads `fill`, `stroke`, `stroke-width`, `color`,
`stroke-dasharray`, `stroke-dashoffset`, `font-weight`, `font-size`,
`font-style` and `font-family`. `font-style` is `normal` or `italic`, and
`oblique` draws as `italic`. `font-family` gives a weight and a style, as
"Font names" says. The scene has no offset of a dash, so `stroke-dashoffset` starts
the dash of the line at the offset. If the offset falls in a gap, the dash
starts at the next dash. Mermaid puts the style of a node on its shape of
SVG, where `border` and `background` change nothing, so zuzu reads them
and draws nothing for them.

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

These parts are issues: a callback of `click` and the target of a link, as
"Links and tooltips" says, a statement that starts with `callback` or
`href`, the `theme` and the `themeVariables` of Mermaid, which draw in the
look of zuzu, the other settings of the section `flowchart`, such as
`minNodeWidth`, and any other style, such as `animation` on a node.

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

A state takes a link and a tooltip from `click`, as "Links and tooltips"
says.

These parts are issues: `scale`, a floating note with no state,
such as `note "text" as N1`, a kind of state other than `choice`, `fork`
and `join`, which draws as a state, the settings of the section `state` of
the config other than `wrappingWidth`, the `theme` and the
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

A class takes a link and a tooltip from `click`, `link` and `callback`, as
"Links and tooltips" says.

These parts are issues: a callback, the settings of
the section `class` of the config other than `hideEmptyMembersBox` and
`hierarchicalNamespaces`, the `theme` and the `themeVariables` of
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

The name of an entity, the type, the name and the comment of an
attribute, and the label of a relationship are markdown, as in Mermaid.
Two stars or two underscores around a text make it bold, and one star or
one underscore make it italic. An underscore inside a word, as in
`first_name`, is text. zuzu draws the text without its marks. A line of
an entity and a label draw in one font, so zuzu draws a text bold or
italic only when all of its text is. `**string**` draws bold, and
`*Customer Account*` italic. In `*first*Name`, only a part is italic, so
it draws plain, and Mermaid draws `first` in italic. The name of an
entity with attributes is bold, and markdown can make it italic too. A
class or a style of an entity with no attributes wins over the font of
its markdown.

These parts are issues: `click`, the settings of the section `er` of the
config, the `theme` and the `themeVariables` of Mermaid, and a style that
a flowchart does not read.

### Requirement diagram

The keyword is `requirementDiagram`. The direction is `TB` by default, and
`direction LR` on a line of its own sets it. The direction is `TB`, `BT`,
`LR` or `RL`. Mermaid does not read `TD` here, and neither does zuzu. A
statement ends at a new line, or at the end brace of a body.

| Part | Example |
| --- | --- |
| a requirement | `requirement test_req {`, then a part of its body on each line, then `}` |
| a kind of requirement | `requirement`, `functionalRequirement`, `interfaceRequirement`, `performanceRequirement`, `physicalRequirement` or `designConstraint` |
| the id of a requirement | `id: 1` |
| the text of a requirement | `text: the test text.` |
| the risk of a requirement | `risk: low`, `risk: medium` or `risk: high` |
| the method of verification of a requirement | `verifymethod: analysis`, `demonstration`, `inspection` or `test` |
| an element | `element test_entity {`, then a part of its body on each line, then `}` |
| the type of an element | `type: simulation` |
| the reference of an element | `docRef: reqs/test_entity` |
| a name or a value in double quotes | `requirement "test req" {`, `type: "test suite"` |
| a relation | `test_entity - satisfies -> test_req` |
| a relation that is written the other way | `test_req <- copies - test_entity`, which goes from `test_entity` to `test_req` |
| the type of a relation | `contains`, `copies`, `derives`, `satisfies`, `verifies`, `refines` or `traces` |
| a class of style | `classDef hot fill:#f96`, then `class a,b hot`, `a:::hot` or `requirement a:::hot {` |
| a style of a requirement or an element | `style a fill:#f9f,stroke:#333` |
| a comment | `%% a comment`, or a line that starts with `#` |
| the text for a screen reader | `accTitle: Reqs`, `accDescr: The requirements`, `accDescr { ... }` |

The keywords, the risks and the methods of verification match in any
case, as in Mermaid. A name matches only in its own case. Each part of a
body is optional, and a part that the body does not set draws no line.

A name or a value without double quotes starts with a letter, a digit or
an underscore of ASCII. It ends at a colon, a comma, a brace, an angle
bracket, a dash or an equals sign, and its line must end there. It cannot
start with a keyword, such as `test` or `high`, and it cannot hold a
direction, such as `direction LR`. Mermaid reads each of these as an
error, and so does zuzu. A text in double quotes can hold them, and it can
run over more lines.

zuzu draws each part in its own look:

| Part | Draws as |
| --- | --- |
| a requirement | a box of two parts with a line between them: its kind in guillemets, such as `«Functional Requirement»`, over its name in bold, then a line for each part of its body, from the left |
| an element | a box of two parts: `«Element»` over its name in bold, then its type and its reference |
| the lines of a body | `ID: 1`, `Text: the test text.`, `Risk: High`, `Verification: Test`, `Type: simulation` and `Doc Ref: reqs/test_entity`, in this order |
| a requirement or an element with an empty body | a box of one part, with its kind and its name |
| the relation `contains` | a solid edge from the requirement that contains to the one that it contains, with the arrowhead `circleplus` at its start: a circle with a cross in it |
| any other relation | a dashed edge with the arrowhead `vee` at its end |
| the label of a relation | its type in guillemets, such as `«satisfies»`, in the middle of the edge |

Mermaid writes a kind and the type of a relation between `<<` and `>>`.
zuzu writes them between guillemets, as it does for a class. The ends of a
relation draw at 1.4 times the size of an arrowhead of a flowchart. Mermaid
draws no line between the two parts of a box when its body holds only a
method of verification or only a reference. zuzu draws the line.

The requirements come first in the layout, then the elements, as in
Mermaid. `classDef`, `class`, `:::` and `style` read the same styles as a
flowchart, and `classDef default` styles each requirement and each
element. `accTitle` and `accDescr` join into the description of the Vega
spec.

The name and the parts of a body are markdown, as in Mermaid. zuzu draws a
text without its marks. A line draws in one font, so zuzu draws a name in
italic only when all of the name is in italic. A line of a body starts
with a plain word, such as `Text:`, and so it draws plain. Mermaid draws
the bold and the italic words of such a line in bold and in italic.

These parts are issues: a class or a style of a name before the definition
of the name, the body of a second definition of a name, which Mermaid
ignores, the settings of the section `requirement` of the config, the
layout `elk`, the look `handDrawn`, the `theme` and the `themeVariables`
of Mermaid, and a style that a flowchart does not read. A relation to a
name that is not a requirement or an element is an error, as in Mermaid.
A name that is a requirement and also an element is an error. Mermaid
draws its two boxes on top of each other.

### Links and tooltips

`click` in a flowchart and a state diagram, and `click`, `link` and
`callback` in a class diagram, give a node a link and a tooltip (D32).
These are the forms:

```
click A "https://example.com" "a tooltip"
click A href "https://example.com" "a tooltip" _blank
click A callback "a tooltip"
click A call callback(arg) "a tooltip"
click A,B "page.html"
link A "https://example.com" "a tooltip"
callback A "callback" "a tooltip"
```

The tooltip and the target, such as `_blank`, are optional. A URL fills
the link, and a tooltip fills the tooltip. The SVG writes the link as an
`<a href>` around the shape and the label of the node, and the tooltip as a
`<title>` inside them. The image of `Image` draws neither.

These parts are issues: a callback, because no script runs, a target,
which the SVG does not write, a link with a scheme that zuzu does not
write, such as `javascript:`, and an id that names no node. Mermaid writes
the link of `javascript:` as `about:blank` and ignores an id that names no
node. "Links" in `docs/API.md` lists the schemes that zuzu writes.

### Labels that wrap

Mermaid 12.3.0 wraps a long label of a flowchart, a state diagram, a class
diagram, an entity relationship diagram and a requirement diagram. zuzu
wraps the same labels at the widths of Mermaid, scaled by 11/14, as the
last paragraph of this section says. It measures the words with the
bundled font, so the lines
are the same on each machine.

| Label | Width in Mermaid |
| --- | --- |
| a node of a flowchart | `flowchart.wrappingWidth`, 120 pixels by default |
| a state, and a note of a state | `state.wrappingWidth`, 120 pixels by default |
| the text of a link, a transition, a relation and a relationship | 200 pixels |
| a subgraph, a composite state, a state with more than one description, a class, a note of a class, an entity and its attributes, a requirement and an element | no wrap |

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

zuzu draws a label at 11 pixels and Mermaid at 14. So zuzu scales each
width of Mermaid that wraps a label by 11/14, and a line of zuzu holds the
words that a line of Mermaid holds (D32). A width of 120 in Mermaid wraps
at about 94 pixels in zuzu, and a width of 200 at about 157. The rule
holds for each width of the table above, for `sequence.width` in a
sequence diagram, for `mindmap.maxNodeWidth`, for
`journey.maxLabelWidth` and for the widths of a timeline. A node of zuzu fits its widest line. Mermaid
makes the box of a label that wraps as wide as the width. DOT does not
wrap a label.

### Charts

The charts compile to Vega-Lite or Vega, as D31 says. A setting of the
config that a chart does not read is an issue, and so are the `theme` and
the `themeCSS`. Each chart but `sankey`, `mindmap` and `block` joins
`accTitle` and `accDescr` into the description of the spec, as "The
description" says. Mermaid does not read them in a sankey, a mindmap or a
block diagram either, so there they are errors, as in Mermaid. Each chart
has about 5 fixtures in `graph/mermaid/testdata`, the gantt has 18, the git
graph has 12, the block diagram has 6, and the timeline has 6. A test
draws each fixture
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
| `gitGraph` | Vega rules, paths and text, laid out in Go with the rules of Mermaid: a lane for each branch and a step for each commit |
| `block`, `block-beta` | Vega rects, paths and text, laid out in Go on a grid of columns |
| `timeline` | Vega rects, rules, paths and text, laid out in Go: the periods along an axis, each with its events |

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

A section draws as Mermaid draws it. Every other section, from the
first, has a band behind its rows. The band starts at the left of the
names of the sections and ends at the end of the time axis. It is in the
color `band` of `docs/STYLE.md`, the same for each section (D32). zuzu
draws it in the text color at a low opacity, so that the grid and the
names of the tasks show through it. The name of each section is at the
left of the names of its tasks, in the middle of its rows, 11 pixels and
bold. The names take the place of the legend. A gantt with no named
section draws no band and no name.

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
wrap at that width scaled by 11/14, as "Labels that wrap" says, or at the
width of the column of the step if it is less, by the rule of Mermaid. A line takes words while it fits. A word that
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
start with `wrap:` or `nowrap:`. A text that wraps wraps at
`sequence.width`, 150 by default, scaled by 11/14, as "Labels that wrap"
says. The name of a participant wraps at `sequence.width` less
`sequence.wrapPadding` on each side, scaled in the same way.

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
`mindmap.maxNodeWidth`, 200 by default, less the padding on its two sides,
scaled by 11/14, as "Labels that wrap" says.

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
which are `cose-bilkent` or `tidy-tree`. `layout` can be `elk` too. Each
draws with the layout of zuzu.

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
a dot. An edge, its arrows and its junctions are in the color `edge` of
`docs/STYLE.md`, and the title
of an edge sits on its middle. zuzu reads `architecture.iconSize`, the size
of a tile, 56 by default, `architecture.fontSize`, 11 by default,
`architecture.padding`, the space inside a group, 16 by default, and
`architecture.nodeSeparation`, the space between two cells, 40 by default.
A title on an edge widens that space to fit. The layout of zuzu uses no
random numbers, so it is the same on each run. zuzu accepts
`architecture.randomize` and `architecture.seed`, and they change nothing.
The other settings, which tune fcose, such as `numIter`, are issues.

#### gitGraph

The keyword is `gitGraph` or `gitGraph:`. A direction and a colon can
follow it, as in `gitGraph TB:`. The direction is `LR`, `TB` or `BT`, and
`LR` by default. A direction with no colon after it is an error, as in
Mermaid. A statement can follow the keyword on its line, as in
`gitGraph: commit`.

| Statement | What zuzu draws |
| --- | --- |
| `commit`, with `id: "a"`, `msg: "text"` or `"text"`, `tag: "v1"`, and `type: NORMAL`, `REVERSE` or `HIGHLIGHT` | a commit on the current branch |
| `branch name`, with `order: 2` | a branch from the current commit. The new branch is the current branch |
| `checkout name`, `switch name` | nothing. The branch is the current branch |
| `merge name`, with `id:`, `tag:` and `type:` | a merge of the branch into the current branch. `type:` sets how the merge draws |
| `cherry-pick id: "a"`, with `parent: "b"` and `tag:` | a copy of the commit on the current branch |
| `title Text` | the title |
| `accTitle: Text`, `accDescr: Text`, `accDescr { ... }` | the description of the Vega spec, the texts joined in order |

The values of `id:`, `msg:`, `tag:` and `parent:` are strings in double or
single quotes. A backslash escapes the next character, and a string can
hold a new line. A statement can have more than one `tag:`, and the last
`id:` wins. The name of a branch is a string, or a word of letters,
digits, `_`, `-`, `.` and `/`. The word does not start with a digit, it
ends in a letter, a digit, `_` or `-`, and it is not a keyword. A new line,
the two characters `\n` and `<br>` in a name start a new line of its
label. `order:` is a whole number. Each statement is on a line of its own,
and the keywords are in the case of the table. Any other text is an error,
as in Mermaid 12.3. The message of a commit draws nothing, as in Mermaid.

zuzu checks the statements as Mermaid does. These are errors:

1. A branch with the name of a branch that exists.
2. A checkout of a branch that does not exist.
3. A merge of a branch into itself, of a branch that does not exist or has
   no commits, into a branch with no commits, of a branch with the same
   head as the current branch, or with the id of a commit that exists.
4. A cherry-pick of a commit that does not exist or that is on the current
   branch, into a branch with no commits, of a merge with no `parent:`, or
   with a `parent:` that is not a parent of the commit.

A commit with the id of an earlier commit takes its place, as in Mermaid.
A commit with no id gets an id in the form of Mermaid: its number, a dash
and seven hexadecimal digits, such as `0-3f2a9c1`. Mermaid picks the
digits at random. zuzu takes them from a hash of the number, the branch
and the message, so that the same source gives the same image.

zuzu lays out a git graph in Go with the rules of Mermaid 12.3. The
branches take their lanes in the order of `order:`, and branches of the
same order keep the order of the source. The main branch has the order
`gitGraph.mainBranchOrder`, 0 by default, and a branch with no `order:` has
the order 0. Two lanes are 90 pixels apart, or 50 when `rotateCommitLabel`
is false. In a graph `TB` or `BT`, the space also grows by half of the
width of the name of the branch. Each commit takes the next step of 50
pixels along the graph, in the order of the source. With `parallelCommits`,
a commit takes the step after its closest parent, so that the commits of
two branches can share a step. In a graph `BT`, the first commit is at the
bottom.

A link goes from each parent of a commit to the commit. A link between two
lanes runs along one lane and turns into the other with an arc of 20. A
link to a merge from the branch that it merges runs along that branch, and
every other link runs along the lane of its commit. When a commit of the
source lies between the two ends, on the lane that the link runs along,
the link goes around it. The link then runs in a lane of its own between
the two lanes, with arcs of 10, at least 10 pixels away from each other
line, as in Mermaid.

Each branch takes the next color of the category scheme of the theme, and
the eleventh branch takes the first color again. A branch has a lane, a
dashed line in its color, and its name in a label with a tint of its color
at the start of the lane. A link is in the color of the branch of its
commit. A link to a merge from the branch that it merges, and a link that
goes around from a later lane to an earlier one, are in the color of the
branch of the parent. A commit is a dot in the color of its branch, with a ring of the
background around it. A merge has a hole, a reverse commit has a cross, a
highlight is a square with a hole, and a cherry-pick has two cherries. The
id of a commit is under it, or at its left in a graph `TB` or `BT`. With
`rotateCommitLabel`, true by default, the id turns 45 degrees. A merge with
no `id:` and a cherry-pick have no label, as in Mermaid. A cherry-pick with
no `tag:` gets the tag `cherry-pick:` and the id of its commit, and an
empty `tag: ""` draws no tag. The tags of a commit are labels with a point
and a hole. They stack above the commit, the first tag of the source at
the top, or at its right in a graph `TB` or `BT`.

zuzu reads `gitGraph.showBranches`, `gitGraph.showCommitLabel`,
`gitGraph.rotateCommitLabel`, `gitGraph.parallelCommits`,
`gitGraph.mainBranchName`, `gitGraph.mainBranchOrder` and
`gitGraph.diagramPadding`, the space around the graph, 8 by default. With
`showBranches` false, zuzu draws no lane and no name of a branch. The other
settings of `gitGraph` are issues, and so are the colors of
`themeVariables`, such as `git0` and `gitBranchLabel0`. A value of the
wrong type, such as `showBranches: 1`, is an issue too.

#### block-beta

The keyword is `block` or `block-beta`. White space and new lines only part
the statements, as in Mermaid. A line can hold more than one statement,
and a statement can go on over lines. A text after the keyword on the first
line is a statement too, so `block-beta bogus` is a block named `bogus`.

| Statement | Example |
| --- | --- |
| a block, with an id and a label in double quotes | `a`, `a["A label"]` |
| a block with a shape | `a("Rounded")`, `a(("Circle"))`, `a{"Diamond"}` |
| a width in columns | `a:3`, `a["A label"]:2` |
| the number of columns of a grid | `columns 3`, `columns auto` |
| one cell of space, or more | `space`, `space:3` |
| a composite block with an id, a label and a width | `block:group1["Group"]:2 ... end` |
| a composite block with no id | `block ... end` |
| a block arrow, with its directions | `arrow<["Label"]>(right)`, `arrow<["Label"]>(x, down)` |
| an edge, with or without a label | `a --> b`, `a -- "label" --> b` |
| a class of style, its use, and a style of a block | `classDef blue fill:#66f`, `class a,b blue`, `style a fill:#f9f` |

The shapes are those of the flowchart: `["…"]` a rectangle, `("…")` round
corners, `(("…"))` a circle, `((("…")))` a double circle, `{"…"}` a
diamond, `{{"…"}}` a hexagon, `(["…"])` a stadium, `[["…"]]` a subroutine,
`[("…")]` a cylinder, `>"…"]` an odd shape, `[/"…"/]` and `[\"…"\]` lean
right and left, and `[/"…"\]` and `[\"…"/]` a trapezoid and an inverted
trapezoid. Another pair of delimiters draws a rectangle, as in Mermaid. A
block arrow points `right`, `left`, `up` or `down`, `x` is right and left,
and `y` is up and down. A link is `---`, `-->`, `<-->`, `--o` or `--x`,
thick with `=` in place of `-`, and dotted as `-.-` or `-.->`. A label is
plain text. `<br>` breaks it, and an entity of HTML, such as `&lt;`, reads
as its character.

zuzu places the blocks as Mermaid 12.3.0 does:

1. A grid fills its cells row by row. With `columns auto`, the default,
   all the blocks of a grid are in one row.
2. A block with no label shows its id. A block that a link names and no
   statement declares joins the grid where the link is.
3. A second statement of a block sets its shape and its label, and the
   block stays in its first cell. Its width does not change.
4. The width of a block with a link after it, as in `a:2 --> b`, does not
   count, as in Mermaid.
5. A block that is wider than the columns left in its row takes the rest
   of the row, and draws past the edge of the grid.
6. The cells of a grid all have one size: the width of the widest block,
   with the width of a wide block shared by its columns, and the height of
   the tallest block. A composite block that is larger than its grid grows
   its cells to fill it.
7. A circle, a double circle, a diamond, a stadium and a block arrow keep
   their own size in the middle of their cells. A block arrow that spans
   more than one column takes the width of its cells. The other shapes
   fill their cells.
8. An edge is a straight line from the middle of one block toward the
   middle of the other, from the edge of one shape to the edge of the
   other. Its label sits on its middle.

Mermaid 12.3.0 reads one link in a statement, and draws a chain such as
`a --> b --> c` wrong. zuzu draws each link of a chain. Mermaid draws the
label of a composite block in its middle, under its blocks. zuzu draws it
as the title of a panel, at its top left, and the grid starts under it.

A class applies to a block in the order of the class statements, and the
style statement of the block applies last. A block with no class takes the
classDef `default`. zuzu reads `fill`, `stroke`, `stroke-width`,
`stroke-dasharray`, `color`, `font-size`, `font-weight` and `font-style`.
Another property is an issue, and so is a part of a style with no colon,
such as `fill#969`, which Mermaid ignores. A class that no classDef names is
an issue, and so is a class of an id that is not a block. zuzu reads
`block.padding`, the space between two cells and inside a block, 8 by
default.

These are errors, as in Mermaid: a diagram with no statement, a label that
is not in double quotes, an empty label, a label of markdown, a block arrow
with no direction, `accTitle`, `accDescr` and `linkStyle`, a composite
block with no statement or no `end`, an `end` with no block, a second
composite block with an id that is taken, a style of more than one block,
as `style a,b fill:#f00`, and a style of an id that is not a block. A shape
on a composite block is an issue, and so is an edge from a block to
itself.

The look is the look of zuzu. A block is a shape with a tint of the first
color of the scheme and an outline. A composite block is a panel. An edge
and its arrowheads take the token Edge, as the edges of the graphs do.

#### timeline

The keyword is `timeline`. `LR` or `TD` can follow it, in any case, and
sets the direction. `LR`, the default, runs from left to right, and `TD`
runs from top to bottom. Other text after the keyword is a statement, as in
Mermaid, so `timeline TB` is a period with the text `TB`.

| Statement | What zuzu draws |
| --- | --- |
| `title Text` | the title |
| `section Name` | a section: a header over the periods after it |
| a period, as `2004` or `2004 : Facebook : Google` | a period, with an event for each colon after it |
| an event on a line of its own, as `: YouTube` | one more event of the last period |
| `accTitle: Text`, `accDescr: Text`, `accDescr { ... }` | the description of the Vega spec, the texts joined in order |

zuzu reads the statements as the lexer of Mermaid 12.3 reads them:

1. The keywords are in any case, as `Section` or `TITLE`.
2. A period runs up to a colon or a `#`. A `#` after a period starts a
   comment, and so do a `#` and a `%` at the start of a line.
3. An event starts with a colon and a space. It runs up to the next colon
   with a space after it, or to the end of the line. Another colon is
   text, as in `https://mermaid.js.org`.
4. The name of a section runs up to a colon, and events can follow it on
   its line. An event belongs to the last period of the source, also when
   that period is in an earlier section.
5. A colon at the end of a line takes the next line as its event.
6. `<br>` starts a new line in a text, and an entity, such as `#quot;`,
   reads as its character.

These are errors, as in Mermaid: an event before the first period, a colon
with no space after it outside an event, as in `10:30 : x`, an event with
no text, a colon at the end of the source, a colon in the name of a
section, and the keyword `timeline` on a later line.

zuzu lays out a timeline in Go with the sizes of Mermaid 12.3, scaled by
11/14 (D32). In a timeline `LR`, each period is a box of 149 pixels, one
each 157 pixels, with its events in a stack under it. The header of each
section is at the top, across its periods. All the periods have one height,
and all the headers have one height, as in Mermaid. The axis runs between
the periods and the events, with an arrowhead at its end. Each period has a
dot on the axis and a dashed line down to its last event. In a timeline
`TD`, each section is a header across the timeline, and each period is a
row under it. The period is a box of 165 pixels at the left of the axis.
Its events are boxes of 244 pixels in a stack at the right of the axis,
each with a dashed line from the axis.

A text wraps at the width of Mermaid, scaled by 11/14, as "Labels that
wrap" says:

| Text | Width in Mermaid |
| --- | --- |
| a period or an event of a timeline `LR` | 150 pixels |
| a section of a timeline `LR` | 200 pixels for each period of the section, less 50 |
| a period of a timeline `TD` | 200 pixels |
| an event of a timeline `TD` | 300 pixels |
| a section of a timeline `TD` | 580 pixels |

zuzu draws some timelines in another way than Mermaid 12.3.0:

1. Mermaid draws no period before the first section. zuzu draws these
   periods first, with no header.
2. Mermaid draws the periods of two sections with the same name under each
   of them. zuzu draws each section with its own periods.
3. In a timeline `TD`, Mermaid gives each row the height of the tallest
   row, and can draw a section over the events of the row above it. zuzu
   gives each row its own height.
4. Mermaid starts a new line only at `<br>`. zuzu also starts one at
   `<br/>` and `<BR>`.
5. Mermaid draws the axis of a timeline with no period and no section.
   zuzu draws only its title.

Each section takes the next color of the category scheme of the theme,
and its periods and their events take its color. The periods before the
first section take a color as a section does. With no section, each period
takes the next color. With `timeline.disableMulticolor: true`, each period
takes the first color. The eleventh color is the first again. A section is
a box with a tint of its color and its name in its color. A period has a
lighter tint, and an event has the lightest. Each box has an outline in its
color. The axis and its arrowhead take the token Edge.

zuzu reads `timeline.disableMulticolor`, and a value that is not true or
false is an issue. The other settings of `timeline`, such as `padding` and
`leftMargin`, are issues, and so are the colors of `themeVariables`, such
as `cScale0`.

### Other kinds

zuzu draws the flowchart, the state diagram, the class diagram, the entity
relationship diagram, the requirement diagram, and the charts above,
which hold the sequence diagram, the mindmap, the architecture diagram, the
git graph, the block diagram and the timeline. Every other kind of Mermaid is an error that wraps
`mermaid.ErrKind`. D32 sets the order of the next kinds.

## Emoji and missing characters

zuzu draws all text in IBM Plex Sans (D16). An emoji draws in color when
the render has the font of the package `text/emoji`, which
`zuzu.WithEmoji(emoji.Font())` gives (D32). The font is Noto Color Emoji.
With it, an emoji measures at the width that the font gives it, so that a
node holds its label. A flag, such as `🇯🇵`, a sequence with a joiner,
such as `👨‍👩‍👧`, and an emoji with a tone of skin draw as one glyph, as
in a browser.

A character that no font of the render has draws nothing and measures as
nothing. Such a character is an emoji without the font, or a character of
a script that Plex does not cover, such as Chinese or Japanese. The report
names each such character once for each render, and it is an error that
wraps `zuzu.ErrNoGlyph` in the strict mode. The check runs when the chart
renders, so a source with such a character parses with no issue.

The SVG names the family `'Noto Color Emoji'` for a text with an emoji, so
that a viewer with that font, or with another font of emoji, draws it in
color.

The charts of Mermaid that lay out their text when they compile, such as
a sequence diagram, a pie, a gantt, a journey, a mindmap, a treemap and
an architecture diagram, measure an emoji as nothing, because the font
comes with the render. Their labels with an emoji can be too narrow for
the emoji. The graphs of DOT and of Mermaid lay out when they render, and
measure an emoji with the font.

## The look

A node is a shape with a tint of its color and an outline of 1.25. An edge
is a line of 1.25 in the color `edge` of `docs/STYLE.md`, with a small
arrowhead in the same color (D32). The messages of a sequence diagram and
their arrowheads take `edge` too. A guide line, such as a lifeline or a
divider of a sequence diagram, or a ring of a radar, takes the lighter
color `axis`. A label
is 11 pixels. A cluster is a panel with its title at the top left. A color
that the source gives replaces the color of the theme. A node with an
opaque fill from the source and no color of text takes the text color of
the light theme or of the dark theme, whichever reads on the fill. The styles are in
`theme/config.go`, from `zuzu.node` to `zuzu.band`.
