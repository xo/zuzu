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
| `SemiBold`, `DemiBold`, `Demi`, `Bold`, `ExtraBold`, `UltraBold`, `Heavy`, `Black` | the weight 600, the heaviest weight that zuzu bundles |
| `Italic`, `Oblique`, `It`, `Slanted` | italic. The bundled font slants only as italic |
| `bolditalic`, `boldoblique` | the weight 600 and italic |

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

### Shapes

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

The flowcharts, the state, the class, the entity relationship, the
requirement, the agentflow and the use case diagrams of Mermaid use the
same layout. Mermaid turns cycles around in the same way. A swimlane
diagram ranks its nodes as "Swimlane diagram" says.

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

An arrowhead takes the color of its edge. Its size comes from the token
`arrowLength` of [`STYLE.md`](STYLE.md): at an `arrowsize` of 1, `normal`
is 8 long and 6.4 wide, and `arrowsize` multiplies the length. The tip ends
on the border of the node at each size.

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
another label or an edge, the label goes to the left. If the left also
covers a node or another label, as when many edges end at the same port,
the label moves away from its edge or along it, in steps, so that the
labels at the same end spread apart. A step is the height of the label,
or its width if that is smaller, plus 2 points. The label moves 6 steps at
most each way, and stays on a side of its own edge. Of the places that
cover the fewest nodes and labels, the label takes the one where the
edges that it cuts and its steps add up to the least, and of those the
nearest. A label that covers a box by 2 points or less does not move. An
edge can end inside the margin of its node, so a label can cover the part
of its own node that lies outside the end. The labels at the ends of the
relations of a class diagram of Mermaid move in the same way. The space
between two ranks grows to hold the labels at the ends of the edges
between them.
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
gives the line its weight and its style, as "Font names" says. A table
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

### Issues

An attribute that the table does not list, a shape that zuzu does not
draw, a name that is not an arrowhead, a number that does not read, and a
part of a label of HTML that okage does not read, such as `href` or
`<img>`, are issues. A label of HTML that is not valid is an
error with its line and column.

`target` and the links and tooltips of the parts of an edge, such as
`headURL`, `labeltooltip` or `edgehref`, are issues.

## Mermaid

zuzu reads Mermaid 12.3.0. The test fixtures compare zuzu with `mmdc`, the
version that bundles Mermaid 12.3.0.

### Every kind

| Part | Example |
| --- | --- |
| front matter, between two lines of `---` | `title: Orders`, `description: The orders of a shop` and `config:` |
| a directive | `%%{init: {'flowchart': {'curve': 'linear'}}}%%`, and `%%{wrap}%%`, which sets `wrap: true` |
| a comment, from `%%` to the end of the line | `%% a comment` |
| the keyword of the kind, on the first line | `flowchart LR` |

The `title` of the front matter is the title of the diagram. Its
`description` goes in the description of the Vega spec, which Mermaid
ignores (D32). Its `config` and each directive merge into one config, and
a directive wins over the front matter. A setting of the config that a kind
does not read is an issue. `displayMode` of the front matter goes in the
config too, and the gantt reads it there. Mermaid ignores each other key
of the front matter, such as `gantt:` or `theme:`, and each one is an
issue, except a hint of the look or the layout.

The front matter is YAML. zuzu reads maps by their indent, a map in
braces, as `{curve: linear}`, and a value in quotes or plain. A value can
run over more lines, indented under its key, and a value in quotes runs to
its closing quote. A block after `|` keeps its new lines, and a block
after `>` joins its lines with spaces. A `-` or a `+` after either sets
the new lines at its end, as in YAML. A `#` after a space, outside quotes,
starts a comment that runs to the end of the line, as in YAML. A `title`
or a `description` that is a map, a `config` that is not a map, a list, a
list in brackets, and a line that is not a key are issues, at their line.
The other parts of YAML, such as anchors, do not read.

`%%{wrap}%%` sets `wrap: true` at the top of the config, over the `wrap`
of an `init` directive, as in Mermaid. Of the kinds that zuzu draws, only
the sequence diagram and the C4 diagrams read `wrap` there, so it changes
nothing in the other kinds, with no issue.

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
| `C4Context`, `C4Container`, `C4Component`, `C4Dynamic`, `C4Deployment` | `c4` |
| a keyword that ends in `-beta`, such as `xychart-beta` | the keyword without `-beta`, such as `xychart` |
| `railroad-ebnf-beta`, `railroad-abnf-beta`, `railroad-peg-beta` | `railroadEbnf`, `railroadAbnf`, `railroadPeg` |
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
and `accDescr`, such as the sankey, takes only the `description`. A C4
diagram reads `accTitle` as its title, so it adds only its `accDescr`. The
`description` can be a block of YAML. A number or a boolean of YAML is
text there.

### Hints of the look and the layout

`look: handDrawn` and `layout: elk` are hints of a way to draw (D32). zuzu
accepts them with no issue in every kind, in the config or at the top of
the front matter. zuzu draws its own look for `handDrawn`, and its own
layered layout, `zuzu.layered`, for `elk`. Two looks drawn by hand, one
like Excalidraw and one like xkcd, and an ELK layout are work for later. The other looks, such
as `classic` and `neo`, draw in the look of zuzu too. The other ELK
layouts of Mermaid 12.3, `elk.stress`, `elk.force`, `elk.mrtree`,
`elk.sporeOverlap`, `elk.box` and `elk.rectpacking`, and the section `elk`
of the config, which holds the settings of ELK, are hints too. So is the
property `algorithm` of a node, such as `grp@{ algorithm: elk.box }`, which
picks an ELK layout for one subgraph. zuzu lays the subgraph out in its own
layered layout, with no issue, whatever the value. Every kind
accepts `layout: dagre`, the default layout of Mermaid, with no issue.
Another layout, such as `elk.layered` or `tidy-tree`, is an issue. A
swimlane diagram and a flowchart accept `layout: swimlane`, as "Swimlane
diagram" says. A mindmap accepts `cose-bilkent` and `tidy-tree` too.

`nodeSpacing` and `rankSpacing` at the top of the config set the spaces of
every graph, as in Mermaid 12.3, over the spaces of the section of the
kind, such as `flowchart.nodeSpacing`. A space takes the ratio of the
default of zuzu to the default of Mermaid for the kind: 140 and 80 in an
entity relationship diagram, and 50 and 50 in the other graphs. So
`nodeSpacing: 100` is a space of 48 between two nodes of a rank of a
flowchart. 0 keeps the space of the section, as in Mermaid. A value that
is not a number of 0 or more is an issue.

### The theme

The `theme` of the config selects a look of zuzu, in every kind (D32):

| Theme | Look |
| --- | --- |
| `dark`, `neo-dark`, `redux-dark`, `redux-dark-color` | the dark mode |
| `default`, `base`, `neutral`, `forest`, `neo`, `redux`, `redux-color` | the scheme `mermaid-` and the name of the theme, such as `mermaid-forest` |

"Schemes from the themes of Mermaid" in [STYLE.md](STYLE.md) holds the
colors of each scheme. A scheme changes the colors only, and the shapes,
the sizes and the type stay those of zuzu. The compiled spec carries the
look in its `config`, as `zuzu.mode` or `zuzu.scheme`. A mode that the
caller gives with `WithMode`, or a scheme that the caller gives with
`WithScheme`, wins over the theme, as [API.md](API.md) says. A theme with
`dark` in its name sets the mode only, so the scheme stays that of the
caller, or `zuzu`. Each other theme sets the scheme only, so the mode stays
that of the caller, or light.

A theme that Mermaid 12.3.0 does not have, such as `sepia` or `Forest`, is
an issue. `themeVariables` and `themeCSS` are issues too, except the theme
variables that a chart reads, as the sections of the charts say.

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
| `wrap` at the top level, in a kind other than the sequence diagram and the C4 diagrams | nothing. Of the kinds that zuzu draws, only the sequence diagram and the C4 diagrams read it |
| `gantt.compact` | nothing. Mermaid 12 has no such setting |
| a section inside a section of the same name, such as `sequence.sequence`, which `%%{init: {'config': {'sequence': {...}}}}%%` makes | nothing. Mermaid has no setting there (D32) |

zuzu matches the names of these settings in any case, so `loglevel` is
`logLevel`. Mermaid matches a name in its own case, and ignores a key such
as `loglevel`. Either way, the image is the same. `mmdc` draws each of
these settings with no change to the image, except `useMaxWidth`, which
changes only the scale of the PNG of `mmdc`.

`htmlLabels` changes the wrap of a label in a graph and in a mindmap. A
graph reads it, as "Labels that wrap" says, and a mindmap reads it as
"mindmap" says.

### Font names

zuzu draws all text in IBM Plex Sans (D16). It reads a weight and a style
from the name of a font family, as "Font names" of DOT says, and accepts
the rest of the name with no issue (D32). So `Helvetica-Bold` draws in the
weight 600, and `Times-Italic` draws in italic. These parts name a family:

| Part | What zuzu does |
| --- | --- |
| `fontFamily` at the top of the config | gives each node, link and subgraph of a flowchart, a swimlane diagram, an agentflow diagram, a state diagram, a class diagram, an entity relationship diagram, a requirement diagram and a use case diagram the weight and the style of the name, and the participants, the messages and the notes of a sequence diagram. A style of a node, a link or a class wins over it |
| `sequence.actorFontFamily`, `sequence.messageFontFamily`, `sequence.noteFontFamily` | gives the weight and the style to the participants, the messages or the notes. Each wins over `fontFamily`, and `actorFontWeight` and the other weights win over each |
| `font-family` in a style, such as `style A font-family:Times-Bold` | gives the weight and the style to the label. `font-weight` and `font-style` in the same style win over it |
| `usecase.actorFontFamily`, `usecase.usecaseFontFamily` | gives the weight and the style to the actors or the use cases of a use case diagram. Each wins over `fontFamily`, and `actorFontWeight` and `usecaseFontWeight` win over each |
| another setting whose name ends in `FontFamily`, such as `journey.taskFontFamily` | nothing |

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
and draws nothing for them. A comma or a semicolon ends each part of a
style. In a `classDef`, a comma after a backslash is a comma of the value,
as in Mermaid, so `stroke-dasharray: 9\,5` draws dashes of 9 and gaps of 5.
CSS ignores a part with no colon, such as `fill#969` or the `5` of
`stroke-dasharray: 9,5`, and Mermaid draws nothing for it. zuzu skips it
with no issue (D32).

`class` gives a class to a node, a link or a subgraph that a line before
it names. A `class` on an id that names none of them, such as `class Z hot`
before the node `Z`, makes no node and draws nothing, as in Mermaid. A
`direction` outside a subgraph changes nothing, as in Mermaid 12.3: the
graph keeps the direction of its first line.

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

These parts are issues: the target of a link, as "Links and tooltips"
says, a statement that starts with `callback` or `href`, a `theme` that Mermaid does not have and the `themeVariables`,
which draw in the look of zuzu, the other settings of the section `flowchart`, such as
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
`accDescr` join into the description of the Vega spec, as "The
description" says.
`hide empty description` is a line of the first version of the state
diagram, and Mermaid draws nothing for it, so zuzu reads it and draws
nothing too.

A line `state` and words with no brace draws nothing, as in Mermaid. The
words are not a state, so `state Idle` and `state Idle: text` add no state
and no description. The state draws only when a transition or another line
names it. Thus `state Idle: text` draws the name `Idle`, and `Idle : text`
draws the description `text`. `state fork_state &lt;&lt;fork&gt;&gt;` is
not a fork, and `fork_state` draws as a state. In `state "Text" as id:
more`, `more` is a second description, up to the next colon. The second
description can be empty, as in `state "Text" as id:`, and Mermaid reads
`state "Text" as id:::hot` in the same way, as an empty description with
no class. Another kind than a choice, a fork or a join, such as
`state h <<history>>`, is no state, as in Mermaid.

`class` gives a class to a state by its id, before or after the line that
names the state. A class makes no state of its own, as in Mermaid, so
`class end hot` draws no state `end`.
`state "Text" as id`, or `state id`, opens a composite state when the next
line starts with a brace. A blank line between them is an error, as in
Mermaid.

A state takes a link and a tooltip from `click`, as "Links and tooltips"
says.

These parts are issues: `scale`, a floating note with no state,
such as `note "text" as N1`, the settings of the section `state` of
the config other than `wrappingWidth`, a `theme` that Mermaid does not
have, the `themeVariables`, and a style that a flowchart does not read.

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

The ends of a relation draw at 1.05 times the size of an arrowhead of a
flowchart, so that the marks of UML read at the size of a label. This is
the size that they had before the arrowheads of zuzu grew (D32). The
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

These parts are issues: the target of a link, the settings of
the section `class` of the config other than `hideEmptyMembersBox` and
`hierarchicalNamespaces`, a `theme` that Mermaid does not have, the
`themeVariables`, and a style that a flowchart does not read.

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
ends of a relationship draw at 1.05 times the size of an arrowhead of a
flowchart, the size that they had before the arrowheads of zuzu grew
(D32). The cardinality `u` of Mermaid draws no mark in Mermaid 12, so
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
config, a `theme` that Mermaid does not have, the `themeVariables`, and a
style that a flowchart does not read.

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
relation draw at 1.05 times the size of an arrowhead of a flowchart. Mermaid
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

Mermaid ignores a class or a style of a name before the definition of the
name, and the body of a second definition of a name. zuzu ignores them
too, with no issue (D32). These parts are issues: the settings of the
section `requirement` of the config, a `theme` that Mermaid does not have,
the `themeVariables`, and a style that a flowchart does not read. A relation to a
name that is not a requirement or an element is an error, as in Mermaid.
A name that is a requirement and also an element is an error. Mermaid
draws its two boxes on top of each other.

### Agentflow diagram

The keyword is `agentflow-beta`. An agentflow diagram draws the agents of a
program as a flowchart: their tasks, their tools, their inputs and their
decisions, in flows. A flow is a box around the nodes of one agent, as a
subgraph is. The direction comes after the keyword. It is `TB`, `TD`,
`BT`, `LR` or `RL`, and `TB` by default. The statements, the chains, a
group with `&`, a chain over a line break, `classDef`, `class`, `:::`,
`style`, `linkStyle`, `click`, `direction` in a flow and the text for a
screen reader read as in a flowchart. zuzu reads the diagram into a graph
and lays it out with `zuzu.layered`.

| Part | Example |
| --- | --- |
| a node | `a`, `a["Text"]`, `a(Text)` |
| a shape | `a["Text"]@{ shape: tool }` or `a@{ shape: tool }` |
| a label | `a@{ label: "Text" }` |
| a flow | `flow id["Title"]`, then its statements, then `end` |
| a flow with no title | `flow id`, then its statements, then `end` |
| a flow with no id | `flow`, then its statements, then `end` |
| a collapsed flow | `id@{ view: collapsed }` after the `end` of the flow, or `flow id["Title"]@{ view: collapsed }` |
| a block of nodes outside every flow | `global`, then its statements, then `end` |
| a connector | `connector id["Title"]` or `connector id` |
| a link | `a --> b`, or `a ---> b` for a longer one |
| a link of a failure | `a --x b` |
| a link to a reference | `a -.- b`, or `a -..- b` for a longer one |
| the text of a link | `a -- yes --> b`, `a -- no --x b`, `a -->\|yes\| b`, `a -.-\|ref\| b` |
| an id of a link | `a e1@--> b`, then `e1@{ animate: true }` |
| properties over more lines | `a@{`, then one key on each line, then `}` |

These are the shapes of a node:

| Shape | Source | Draws as |
| --- | --- | --- |
| `task` | `shape: task`, `a[Text]`, `a(Text)`, or a node with no brackets | a rounded rectangle |
| `tool` | `shape: tool`, `subroutine`, `subprocess`, `subproc` or `framed-rectangle`, or `a[[Text]]` | a subroutine |
| `input` | `shape: input` or `lean-right`, or `a[/Text/]` | a parallelogram |
| `decision` | `shape: decision` or `diamond`, or `a{Text}` | a diamond |
| `refdoc` | `shape: refdoc`, `lin-doc` or `lined-document` | a document with a second line down its left side |
| `action` | `shape: action`, `hexagon` or `hex`, or `a{{Text}}` | a hexagon |

Mermaid draws every other shape as a rounded rectangle, and so does zuzu.
This holds for another name of a shape of Mermaid, such as `rect` or
`cyl`, and for the other brackets, such as `a((Text))` or `a[(Text)]`. A
name of a shape with a capital letter or an underscore, such as `Task` or
`lean_right`, is an error, as in Mermaid. A name that Mermaid does not
have, such as `connector`, is an error too.

Mermaid reads a flow at its `end`, and so does zuzu. A flow holds each
node and each flow that its statements name, unless a flow that ended
before it holds them. So a node that a flow names, and that a flow inside
it names again, goes into the inner flow. A flow does not hold itself, or a
node of a block `global`. A block `global` draws no box. Its nodes draw
outside every flow, even when a flow names them. A flow with the id of a
flow that ended before it adds to that flow. A flow with no id takes the
id `subGraph` and the number of the flows that ended before it, as in
Mermaid. A link to the id of a flow goes to the box of the flow. A flow
does not hold a flow that holds it, as in Mermaid, and such a loop is an
issue.

`view: collapsed` draws a flow as one node, as a collapsed subgraph of a
flowchart draws. Mermaid reads it only after the `end` of the flow, and so
does zuzu. `view: collapsed` on any other id is an issue.

A connector draws as a rounded rectangle with its title, or with its id.
A flow holds a connector that it declares. The properties of a connector
draw nothing, and a shape there is no error, as in Mermaid.

The properties `shape` and `label` of a node draw, and `view` collapses a
flow. Mermaid keeps the other properties of a node, a flow and a
connector as data for other programs, and draws none of them. Such
properties are `value`, `params`, `returns`, `connectorRef`,
`description`, `model`, `instruction`, `protocol`, `endpoint`, `icon` and
`img`. zuzu draws none of them either, and reports none of them. The
properties `animate`, `animation` and `curve` of a link draw as in a
flowchart. The other properties of a link, such as `instruction`, draw
nothing, with no issue. An empty `@{ }` changes nothing.

Properties on one line are a map in braces, with a comma between two
keys. Properties over more lines are a block of YAML, with one key on
each line. There, a comma at the end of a plain value is a part of the
value, as in Mermaid. So `shape: input,` on a line of its own is an
error. A comma after a value in quotes is not a part of the value.

Mermaid reads only the links of the table above in an agentflow diagram.
Another link, such as `---`, `==>`, `-.->`, `~~~`, `--o`, `<-->` or
`-. text .-`, is an error, as in Mermaid. A statement `subgraph` or
`title` is an error too. Each dash or dot more makes a link one rank
longer.

zuzu reads these settings of the section `agentflow` of the config:

| Setting | Default of Mermaid | What zuzu does |
| --- | --- | --- |
| `wrappingWidth` | 120 | wraps the label of a node, as "Labels that wrap" says |
| `minNodeWidth` | 120 | makes a node at least as wide as a label of this width, scaled by 11/14, and its padding. A diamond and a collapsed flow fit their label |
| `nodeSpacing` | 50 | sets the space between two nodes of a rank to 24/50 of it |
| `rankSpacing` | 50 | sets the space between two ranks to 40/50 of it |
| `diagramPadding` | 8 | sets the space around the diagram to it |
| `titleTopMargin` | 25 | sets the space between the title and the diagram to 10/25 of it |

It reads `curve`, `subGraphTitleMargin`, `inheritDir` and `htmlLabels` of
the section `flowchart`, as Mermaid 12.3 does, as a flowchart reads them.

zuzu draws an agentflow diagram in its own look. A node is a shape with a
tint of its color, as in a flowchart, and a flow is a panel with its
title at the top left. A link and its arrowhead take the color `edge`, and
a link to a reference is dotted. Mermaid draws an agentflow diagram in
the theme `redux-color` by default, with a color for each kind of node and
for each flow. zuzu draws every kind in the color of the scheme.

These parts are issues: a `theme` that Mermaid does not have, the
`themeVariables`, which draw in the look of zuzu, the other settings of
the sections `agentflow` and `flowchart`, the target of a link, and a
style that a flowchart does not read.

### Use case diagram

The keyword is `usecase-beta`. zuzu reads a use case diagram with the
grammar of Mermaid 12.3.0, so a source that Mermaid reads as an error is
an error in zuzu too. A statement ends at a new line. The direction is
`LR` by default, and `direction TB` on a line of its own sets it. The
direction is `TB`, `TD`, `BT`, `LR` or `RL`, and the last one wins.

| Part | Example |
| --- | --- |
| an actor | `actor Customer`, `actor Admin("Main administrator")`, `actor A, B` |
| a use case | `Checkout`, `Login("Sign in")`, `Report[Generate report]`, `"Reset password"` |
| a system boundary, with its actors and its use cases, one on each line | `systemBoundary Store`, then `Browse`, then `end` |
| the title of a system boundary | `systemBoundary sb1["Payment service"]`, `systemBoundary "Payment service"` |
| a relation | `Customer --> Checkout` |
| a relation with an id | `Customer opens@--> Checkout` |
| include and extend | `Checkout ..> : include Payment`, `Coupon ..> : extend Checkout` |
| a generalization | `Admin --\|> Person` |
| the label of a relation | `Customer -- "places order" --> Checkout`, `Customer -- places order --> Checkout` |
| the metadata of a name | `actor Broker@{ type: hollow, business: true }`, `Payment_service@{ type: package }` |
| the metadata of an edge with an id | `opens@{ animation: fast }`, `opens@{ animate: true }` |
| a stereotype | `actor SalesAgent <<Employee>>`, `Quote("Prepare quote") <<Core>>` |
| a note | `note for Login "Requires a session"` |
| a block of JSON | `json Payload@{ "status": "pending" }` |
| a class of style | `classDef hot stroke:#c33`, then `class Login,Pay hot` or `Login("Sign in"):::hot` |
| a style | `style Login fill:#ffe6e6`, `style opens stroke:#06c` |
| the text for a screen reader | `accTitle: Shop`, `accDescr: The shop`, `accDescr { ... }` |

A label in parentheses draws an ellipse, and a label in brackets draws a
rectangle. A use case with no label is an ellipse, and so is a name that
only a relation names. A name in quotes takes an id where each character
that is not a letter, a digit or an underscore of ASCII is `_`, so
`"Reset password"` is `Reset_password`. A statement can declare a name
again with the same label, and it adds its classes.

| Arrow | Draws |
| --- | --- |
| `-->` | a solid line with an arrowhead at its end |
| `<--` | a solid line with an arrowhead at its start |
| `--` | a solid line with no arrowhead |
| `--o`, `o--` | a circle at the end, or at the start |
| `--x`, `x--` | a cross at the end, or at the start |
| `..> : include`, `..> : extend` | a dashed line with an arrowhead at its end, and `«include»` or `«extend»` in its middle |
| `--\|>` | a solid line with a hollow triangle at its end |

Each dash more than two in `-->`, `<--` or `--` adds a rank, so `--->`
spans at least 2 ranks. The words include and extend match in any case.

| Metadata | Of | Values |
| --- | --- | --- |
| `type` | an actor | `normal`, `hollow` or `awesome` |
| `icon` | an actor | the name of an icon, such as `"fa:user"` |
| `business` | an actor or a use case | `true` or `false` |
| `type` | a system boundary | `rect` or `package` |
| `animate`, `animation` | an edge with an id | `true` or `false`, and `fast` or `slow` |

zuzu draws each part in its own look:

| Part | Draws as |
| --- | --- |
| an actor | a stick figure over its name, the figure of an actor of a sequence diagram |
| a hollow actor | the outline of a figure |
| an awesome actor | a filled figure |
| an actor with an icon | the icon in a rounded box over its name. A name with no pack, such as `user`, is `fa:user` |
| a business actor | a line across the head of the figure |
| a use case | an ellipse, or a rectangle for a label in brackets |
| a business use case | an ellipse with a line across its right end |
| a stereotype | a line over the name in guillemets, such as `«Core»` |
| a system boundary | a panel with its title at the top left |
| a system boundary of the type `package` | a package of UML: a tab at the top left that holds the title, over a panel as wide as the boundary (D32) |
| a note | a note, with a dashed line and no arrowhead to its actor or its use case |
| a block of JSON | a table: its id at the top, then a row for each value, with the path of the value, such as `address.city` or `items[0].name`, and the value |
| an animated edge | a dashed line, one still frame of Mermaid (D32) |

The actors come first in the layout, then the use cases, the notes and the
blocks of JSON, as in Mermaid. Each actor and each use case takes the place
of its first mention in the source.

A block of JSON holds an object. Its rows are in the order of the source. A
key that comes twice keeps its first place and its last value, as
`JSON.parse` does. An array of values takes a row for each value, with the
path in the first row only. An empty array is `[]`, and an empty object is
`{}`.

A label in quotes or in brackets is plain text, so zuzu draws `**text**`
with its stars, as Mermaid does. A string of markdown, in "\` and \`",
draws without its marks. A line of zuzu draws in one font, so zuzu draws
a label in bold or in italic only when all of it is. An entity, such as
`#quot;`, reads as its character.

`classDef`, `class`, `:::` and `style` read the styles of a flowchart. The
class `default` styles each actor, use case, system boundary, block of
JSON and edge, as in Mermaid. A later `classDef` of a name replaces an
earlier one. A class or a style of an edge names the id of the edge, such
as `opens` in `Customer opens@--> Checkout`. An edge takes the stroke of
its classes and its styles, and not their fill. Mermaid draws the figure
of an actor in the colors of its theme, over a style of the actor. zuzu
draws the style.

zuzu reads these settings of `usecase`:

1. `nodeSpacing`, `rankSpacing` and `diagramPadding`, which zuzu reads as
   those of a flowchart.
2. `wrappingWidth`, 120 by default, where the label of an actor, a use
   case or a note wraps. Mermaid draws a use case at 12 pixels, and an
   actor and a note at 14. So the width scales by 11/12 for a use case,
   and by 11/14 for an actor and a note, as "Labels that wrap" says (D32).
3. `minNodeWidth`, 120 by default, the least width of the label of a use
   case or a note. It scales by the same ratio as the label.
4. `actorFontSize`, `actorFontWeight`, `usecaseFontSize` and
   `usecaseFontWeight`. A size is in pixels, and the default sizes are
   those of the look of zuzu, which are smaller than those of Mermaid.
5. `actorFontFamily` and `usecaseFontFamily`, which give the weight and
   the style that the name of a family holds.
6. `colorScheme: role`, the default, which draws in the colors of zuzu.

These parts are issues: `colorScheme: rotate`, which gives each actor and
each use case a color of a palette of a theme of Mermaid, another setting
of `usecase`, a `theme` that Mermaid does not have, and
the `themeVariables`. An icon that does not load draws as a box with a question mark,
and it is an issue when the diagram renders.

These are errors, as in Mermaid:

1. A source that the grammar does not read, such as a relation or a
   second system boundary inside a system boundary, a name that is a
   keyword, such as `end`, a style of two names, or a statement of
   PlantUML, such as `skinparam`.
2. A name that is an actor and a use case, or a system boundary, a block
   of JSON or an id of an edge that another declaration names.
3. Two labels, two shapes, two stereotypes or two system boundaries of one
   name, and two titles of one system boundary.
4. Two declarations of a name in quotes, of a block of JSON, or of an id
   of an edge.
5. A relation to a system boundary, include or extend from or to a name
   that is not a use case, a generalization from an actor to a use case,
   and a circle or a cross at a block of JSON.
6. Metadata, a note, a class or a style of a name that the diagram does
   not declare, a note for a name that is not an actor or a use case, and
   metadata that a name does not take.
7. An actor with an icon and a type other than `normal`, a business actor
   that is `awesome` or has an icon, and a business use case in brackets.
8. A block of JSON that does not read, or that does not hold an object.
9. Classes after `:::` on a name that a relation only names.

Mermaid 12.3.0 draws no title of the front matter in a use case diagram,
and zuzu draws it. Mermaid removes the last semicolon of a line of `style`
or of `classDef` that holds a color, such as `style A fill:#f00;`, before
it reads the line. zuzu reads such a semicolon as an error. zuzu reads
`%%` after a statement as a comment, and Mermaid reads it as an error.
`accTitle` and `accDescr` join into the description of the Vega spec.

### Swimlane diagram

The keyword is `swimlane-beta`. A swimlane diagram is a flowchart in lanes.
It reads every part of a flowchart, as "Flowchart" says, with the same
errors, and it draws its nodes, its links, its styles, its icons and its
links of `click` the same way. Its labels wrap as the labels of a
flowchart. The direction is `TB`, `TD`, `BT`, `LR` or `RL`, and `TB` by
default. A flowchart with `layout: swimlane` lays out in lanes too, as in
Mermaid 12.3.

Each subgraph at the top is a lane. A lane is a band that runs along all
the ranks. The lanes lie side by side in the order of the source: from the
left to the right when the ranks run down or up, and from the top to the
bottom when the ranks run across. A node is in the lane of the subgraph at
the top that holds it. The nodes outside every subgraph go into a lane of
their own, with no title, before the other lanes, as in Mermaid. A lane
that holds no node draws nothing.

A subgraph inside a lane draws as a box around its nodes, with its title.
Its direction changes nothing, as in Mermaid. Its box can hold a node of
another subgraph that lies between its nodes, as in Mermaid. An edge to
the id of a subgraph ends at the box of the subgraph.

zuzu ranks the nodes as the swimlane layout of Mermaid 12.3 ranks them:

1. An edge that closes a cycle turns around. A search starts from the
   nodes with no edge in, and follows the edges out of a node in the order
   of the ids of their targets.
2. The nodes take their ranks in a topological order, with the ids in the
   order of `localeCompare` of JavaScript. A diagram from the left to the
   right takes the nodes one generation at a time.
3. A node takes the greatest rank of its sources, plus 1 for a source in
   its own lane. So an edge between two lanes does not move its target to
   the next rank.
4. A lane holds one node in each rank, so a node takes at least the rank
   after the last node of its lane.

With `swimlane.ignoreCrossLaneEdges: false`, each edge moves its target to
the next rank, and then 8 passes move each node toward the mean of the
ranks of its neighbors. Two nodes of a lane can then share a rank. They lie
side by side in it, in the order of the medians of their neighbors, as in
Mermaid. Mermaid ignores the length of a link, such as `--->`, in a
swimlane diagram, and so does zuzu.

zuzu places the nodes in its own look, on a grid of the lanes and the
ranks:

1. The nodes of a lane are in the middle of the lane. A lane is as wide as
   its widest node and its title, with a space of 16 on each side.
2. The ranks follow one after another. A rank is as long as its longest
   node. The space between two ranks is that of a flowchart, 40 by default.
3. The label of an edge in one rank goes beside the target of the edge, on
   the side of its source. The lane of the target makes room for it on
   both sides of its nodes, as in Mermaid.
4. The label of an edge across the ranks goes in the gap in the middle of
   the ranks of the edge, on its route. The gap grows to hold it.
5. An edge routes at right angles, around the nodes, with round corners.
   It crosses the lanes in the rank of its source when it can. It leaves
   its source and comes into its target on the side that faces the way
   from the source to the target, when it can. A route costs more where
   another route or a label already is, so that two routes share a line
   only when no other way is short.
6. A link from a node to itself draws as a loop out of the side of the
   node. Mermaid 12.3 fails to draw such a link, and zuzu draws it.

A curve of a link that is not `basis`, such as `linear`, draws the route
at right angles with that curve of Vega. `basis`, the default, draws round
corners, as the curve `rounded` of Mermaid does. Mermaid draws a hop where
two routes cross, and zuzu draws none.

The look is the look of zuzu. A lane is a panel with its title at the top
left, and two lanes have a space of 8 between them. Mermaid draws the title
of a lane in a band at its left, turned, when the ranks run across. zuzu
draws it at the top left in each direction. A node and an edge draw as in
a flowchart, and an edge and its arrowheads take the color `edge`.

zuzu reads the settings of the section `flowchart`, as a flowchart does,
and these settings:

| Setting | Default of Mermaid | What zuzu does |
| --- | --- | --- |
| `layout`, `swimlane.layout` | `swimlane` | `swimlane` lays out in lanes. `dagre` and `elk` draw the diagram as a flowchart, with its subgraphs as clusters, as in Mermaid. `swimlane.layout` wins over `layout` |
| `swimlane.ignoreCrossLaneEdges` | true | false makes each edge move its target to the next rank |
| `swimlane.optimizeRanksByCrossings` | true | nothing, when `ignoreCrossLaneEdges` is true. When it is false, true is an issue, because zuzu does not move the ranks to fewer crossings |
| `swimlane.automaticLaneOrdering` | false | true is an issue. zuzu keeps the lanes in the order of the source |
| `swimlane.lineHops` | `arc` | false draws no hop, as zuzu always does. `arc` and `gap` are issues |
| `swimlane.look`, `swimlane.useMaxWidth` | `neo`, true | nothing, as `look` and `useMaxWidth` |

A swimlane diagram with `layout: dagre` or `layout: elk` reads no other
setting of the section `swimlane`, and such a setting is an issue. Another
layout, such as `tidy-tree`, is an issue, and the diagram lays out in
lanes. The spec of a diagram in lanes draws with the transform
`zuzu.layered` and its parameter `lanes: true`, and with `laneEdges: true`
when `ignoreCrossLaneEdges` is false.

These parts are issues, as in a flowchart: a `theme` that Mermaid does
not have, the `themeVariables` and the `themeCSS`, which draw in the look
of zuzu, the other settings of the section `flowchart`, such as
`minNodeWidth`, the other settings of the section `swimlane`, such as
`swimlane.theme`, and the target of a link.

### Links and tooltips

`click` in a flowchart, a swimlane diagram, an agentflow diagram and a state diagram, and `click`, `link` and
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

A callback, such as `click A callback`, `click A call callback(arg)` or
`callback A "callback"`, is no issue. Mermaid binds no callback in its
default `securityLevel`, `strict`, so it draws nothing for one, and zuzu
does the same (D32). The tooltip of such a statement still fills the
tooltip.

These parts are issues: a target, which the SVG does not write, and a link with a scheme that zuzu
does not write, such as `javascript:`. Mermaid writes the target in the
`<a>` of its SVG, so a click on the node opens the link in another window.
The SVG of zuzu holds the link and not the target, so the link opens in
another way, and that is why the target stays an issue. Mermaid writes the
link of `javascript:` as `about:blank`. "Links" in `docs/API.md` lists the
schemes that zuzu writes.

Mermaid binds a click only to a node, and ignores an id that names no
node. zuzu skips such an id with no issue (D32). A click that names no
node at all draws nothing, so its callback, its target and its link are no
issue either.

### Labels that wrap

Mermaid 12.3.0 wraps a long label of a flowchart, a swimlane diagram, a state diagram, a
class diagram, an entity relationship diagram, a requirement diagram, an
agentflow diagram and a use case diagram. zuzu
wraps the same labels at the widths of Mermaid, scaled by 11/14, as the
last paragraph of this section says. It measures the words with the
bundled font, so the lines
are the same on each machine.

| Label | Width in Mermaid |
| --- | --- |
| a node of a flowchart and of a swimlane diagram | `flowchart.wrappingWidth`, 120 pixels by default |
| a node of an agentflow diagram | `agentflow.wrappingWidth`, 120 pixels by default |
| a state, and a note of a state | `state.wrappingWidth`, 120 pixels by default |
| the text of a link, a transition, a relation and a relationship | 200 pixels |
| an actor, a use case, and a note of a use case diagram | `usecase.wrappingWidth`, 120 pixels by default |
| a subgraph, a flow, a composite state, a state with more than one description, a class, a note of a class, an entity and its attributes, a requirement and an element, a system boundary and a block of JSON | no wrap |

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

zuzu draws a label at 11 pixels and Mermaid at 14 in most kinds. So zuzu
scales each width of Mermaid that wraps a label by 11/14, and a line of
zuzu holds the words that a line of Mermaid holds (D32). A width of 120 in
Mermaid wraps at about 94 pixels in zuzu, and a width of 200 at about 157.
The rule holds for each width of the table above, for `sequence.width` in
a sequence diagram, for `journey.maxLabelWidth` and for the texts of a C4
diagram. Mermaid 12.3 draws the labels of a timeline, of a mindmap and of
a kanban at 16 pixels, so the widths of a timeline, `mindmap.maxNodeWidth`
and the widths of a kanban scale by 11/16. A width of 200 in a timeline or a mindmap wraps at 137.5 pixels
in zuzu. In a kind whose labels have more than one size in Mermaid, each
label takes the ratio of its own size (D32). A ratio comes from the font
and the size that mmdc measures with. mmdc measures each label of a
sequence diagram at 16 pixels in Times New Roman, because the family of
fonts of Mermaid ends in a semicolon that makes it invalid. That text is
as wide as 14.5 pixels of IBM Plex Sans, so each label of a sequence
diagram wraps at 11/14.5 of its width in Mermaid, as "Sequence diagram"
says. A use case diagram draws a use case
at 12 pixels, and an actor and a note at 14. So the label of a use case
wraps at 11/12 of `usecase.wrappingWidth`, and an actor at 11/14. A
venn diagram draws a text at 20 pixels, so a text wraps at 11/20 of the
width of its cell, as "venn-beta" says. The
ratio of each kind and of each such label is in `mermaidSizes`, in
`graph/mermaid/style.go`. A node of zuzu fits its widest line. Mermaid
makes the box of a label that wraps as wide as the width. DOT does not
wrap a label.

### Charts

The charts compile to Vega-Lite or Vega, as D31 says. A setting of the
config that a chart does not read is an issue, and so are a `theme` that
Mermaid does not have and the `themeCSS`, as "The theme" says. Each chart but `sankey`, `mindmap`, `block`, `ishikawa`, `kanban` and `venn-beta` joins
`accTitle` and `accDescr` into the description of the spec, as "The
description" says. Mermaid does not read them in a sankey, a mindmap, a
block diagram or a venn diagram either, so there they are errors, as in
Mermaid. In a kanban, Mermaid reads them as the text of a node, and so
does zuzu. A C4 diagram reads `accTitle` as its title, as "C4" says.
An ishikawa diagram reads them as causes, as "ishikawa" says. A ZenUML
diagram reads them as messages, as "zenuml" says. Each chart
has 5 to 7 fixtures in `graph/mermaid/testdata`, the sequence diagram has
10, the gantt has 18, and the git graph has 12. A test draws each fixture and checks that each value of
the source draws once, with its label.

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
| `C4Context`, `C4Container`, `C4Component`, `C4Dynamic`, `C4Deployment` | Vega rects, paths and text, laid out in Go in the rows of Mermaid |
| `ishikawa`, `ishikawa-beta` | Vega rules, symbols, paths, rects and text, laid out in Go: the problem at the head of a spine, and the causes as bones |
| `cynefin-beta` | Vega rects, paths and text, laid out in Go as Mermaid lays them out: the four domains in the quarters and confusion in the middle |
| `info` | Vega text: the version of Mermaid |
| `kanban` | Vega rects, rules and text, laid out in Go: the columns side by side, each with its cards |
| `packet`, `packet-beta` | Vega rects and text, laid out in Go: the fields on rows of bits |
| `railroad-beta`, `railroad-ebnf-beta`, `railroad-abnf-beta`, `railroad-peg-beta` | Vega rects, paths and text, laid out in Go with the sizes of Mermaid: a track for each rule |
| `treeView-beta` | Vega rules, rects, paths, text and icons, laid out in Go: a row for each item, indented by its depth |
| `eventmodeling` | Vega rects, paths, symbols and text, laid out in Go: the frames along a time axis, each in the swimlane of its kind |
| `zenuml` | Vega, laid out in Go by the layout of `sequenceDiagram` |
| `venn-beta` | Vega paths and text, laid out in Go: a circle for each set, placed as Mermaid places it |

#### pie

`showData`, and `title` on the first line or on a line of its own. A slice
is `"label" : value`. With `showData`, the legend shows each value. zuzu
reads `pie.textPosition`, `pie.donutHole`, `pie.legendPosition` and the
colors `pie1` and on. A negative value is an error. Mermaid drops a
second slice with the same label, and so does zuzu, with no issue (D32).

`pie.highlightSlice` names a slice that reaches 5 percent past the rim of
the pie, as in Mermaid. The value `hover` highlights the slice under the
pointer. An image has no pointer, so `hover` is an issue. Mermaid writes
`hover` into its SVG as a style that a browser runs, and the SVG of zuzu
does not hold it.

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
not. A value of a line can have a label, `[540 "PaLM"]`. Mermaid draws
no label of a value of a bar, and zuzu drops it with no issue (D32). zuzu
reads the
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
value for each category. Mermaid does not draw the values past the last
category either, so they are no issue (D32).

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
| `weekday` | the first day of a week of the ticks. zuzu starts a week on `sunday`, so another day is an issue when `tickInterval` is in weeks |
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
A cycle of `after` and `until` is an error. `click`, and `todayMarker`
with a value other than `off`, are issues. A render has no time of its own, so zuzu draws no
marker of today. A gantt with no task draws nothing but
its title, as in Mermaid.

zuzu reads these settings of `gantt`, and `displayMode` at the top level
of the front matter and of the config too:

| Setting | What zuzu does |
| --- | --- |
| `axisFormat`, `tickInterval`, `topAxis`, `weekday` | as the statements of the same names |
| `displayMode: compact` | the tasks of each section pack into as few rows as their times allow |
| `numberSectionStyles` | the sections take that many colors in turn |
| `rightPadding` | the padding at the right of the chart, in pixels |
| `useWidth` | the width of the time axis: `useWidth` less 75, the `leftPadding` of Mermaid, and less `rightPadding`, 75 by default. The time axis is 640 wide by default, near the 634 of Mermaid on the page of `mmdc` |
| `compact` | nothing, with no issue. Mermaid 12 has no such setting and ignores it, and its documents show it next to `displayMode: compact` |

A text after the keyword, as in `gantt foo`, is an error, as in Mermaid. A
setting at the top level of the config other than `displayMode`, such as
`themeCSS`, is an issue, unless it changes nothing, as "Settings that
change nothing" says.

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
counts a month as 30 days for this. A `tickInterval` that is not a number
and a unit of time also gives the axis ticks of its own. zuzu then draws
what Mermaid draws, so none of these is an issue (D32). A bar is at least
2 pixels wide, so that a short task in a long gantt still draws.

As in Mermaid, the value of a statement is all the text after its keyword.
In `dateFormat :YYYY-MM-DD`, the colon is part of the format, so no date
of the source reads in it. A start date then reads as the Date of
JavaScript reads it. An end date that is not a duration does not read, so
the task has no length, as in Mermaid. An item of `excludes` or
`includes` that is not a date, a day of the week or `weekends` excludes or
includes no day, as in Mermaid. Mermaid draws nothing for either part, so
neither is an issue (D32).

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
`cScale0` and on when each curve has one. A radar can have any number of
axes, as in Mermaid. With two axes, a curve is a line, and with one or
none, it is a point or nothing.

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
start with `wrap:` or `nowrap:`. A text wraps where Mermaid 12.3 wraps it
(D32). Mermaid does not wrap each text at `sequence.width`. The width of
each text comes from where Mermaid puts its participants, and zuzu puts
them in another place. So zuzu works out the places of the participants
as Mermaid does, in the pixels of Mermaid, and takes these widths from
them:

| Text | Width in Mermaid |
| --- | --- |
| a message between two participants | the gap between the two lifelines and their activations, less 3 pixels for an arrowhead at each end that has one, plus `sequence.wrapPadding` on each side, and at least `sequence.width` |
| a message to the same participant | `sequence.width`, 150 by default |
| a note to the right of a participant | the width of the note, at least `sequence.width`, less `sequence.wrapPadding` on each side: 130 by default |
| a note to the left of a participant | the width of the text wrapped at `sequence.width`, plus `sequence.noteMargin` on each side, at least `sequence.width`, less `sequence.wrapPadding` on each side |
| a note over one participant | the width of the participant, at least `sequence.width`, less `sequence.wrapPadding` on each side |
| a note over two participants | the gap between their lifelines, plus `sequence.actorMargin`, less `sequence.wrapPadding` on each side |
| the label of a frame, such as `loop`, and of each section, such as `else` | the width of the frame less `sequence.wrapPadding` on each side |
| the name of a participant | `sequence.width` less `sequence.wrapPadding` on each side |

A participant in Mermaid is `sequence.width` wide, or as wide as its name
and `sequence.wrapPadding` on each side when the name does not wrap. Two
participants are `sequence.actorMargin` apart, 50 by default in Mermaid,
or more, so that the widest message or note between them fits. Mermaid
works out the width of a frame from the messages and the notes in it,
and takes `sequence.labelBoxWidth`, 50 by default, from it for each of
them. The label of a frame always wraps, as in Mermaid, with its
brackets. Each other text wraps only when it or the diagram asks for it.

A text wraps as Mermaid wraps it. A word goes on a new line when the width
of the line and the width of the word, without the space between them,
reach the width. A text with a break in it, such as `<br>`, does not
wrap. mmdc measures each label at 16 pixels in Times New Roman, as wide
as 14.5 pixels of IBM Plex Sans, so each width scales by 11/14.5, as
"Labels that wrap" says. zuzu keeps a
word that is wider than the width whole. Mermaid breaks it with a hyphen.
zuzu does not move the participants for a box or for a participant that a
message creates when it works out the widths.

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
of Mermaid. The families of the fonts give a weight and a style, as "Font
names" says. The other settings are issues.

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
scaled by 11/16, as "Labels that wrap" says. A word that is wider stays
whole, as in Mermaid. With `htmlLabels: false`, or
`flowchart.htmlLabels: false` when `htmlLabels` is not set, Mermaid draws
its labels in SVG, and zuzu breaks such a word into lines as Mermaid does.
A value of `htmlLabels` other than `true` or `false` is an issue. Mermaid
also puts the lines of a label in SVG at the left, and zuzu keeps them in
the middle.

The text in a shape can run over more lines, up to the line that ends the
shape, as `root(`, then `The root`, then `)`. zuzu drops the spaces at the
start and the end of each line and the blank lines. As in Mermaid, the
text draws on one line when it fits, and else each line of the source
starts a new line.

| Line after a node | What zuzu does |
| --- | --- |
| `::icon(fa fa-book)` | the icon of the node, at the left of its label |
| `:::urgent large` | nothing, with no issue |

zuzu reads the classes of a font of icons as the name of an icon:
`fa fa-book` is `fa:book`, `fas fa-book` is `fas:book`, and `mdi mdi-home`
is `mdi:home`. A name with a colon, such as `carbon:cloud`, stays as it is.
The icon comes from the packs and the icon loader of the render, as in a
flowchart. An icon that does not load draws as a box with a question mark,
and the report names it. An icon whose classes name no pack is an issue.
The classes of a node are classes of CSS that the page of a site gives.
Mermaid draws nothing for them by itself, so zuzu reads them with no
issue (D32).

zuzu reads `mindmap.padding`, `mindmap.maxNodeWidth`, `mindmap.colorBy`,
which is `branch` or `depth`, and `mindmap.layoutAlgorithm` and `layout`,
which are `cose-bilkent` or `tidy-tree`. `layout` can be `dagre` or an ELK
layout too, as "Hints of the look and the layout" says. Each draws with
the layout of zuzu.

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
Another property is an issue. Mermaid ignores a part of a style with no
colon, such as `fill#969`, and so does zuzu, with no issue (D32). A class
that no classDef names is
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
and its arrowheads take the color `edge`, as the edges of the graphs do.

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
11/16, because Mermaid draws the labels of a timeline at 16 pixels (D32).
In a timeline `LR`, each period is a box of about 131 pixels, one each
137.5 pixels, with its events in a stack under it. The header of each
section is at the top, across its periods. All the periods have one height,
and all the headers have one height, as in Mermaid. The axis runs between
the periods and the events, with an arrowhead at its end. Each period has a
dot on the axis and a dashed line down to its last event. In a timeline
`TD`, each section is a header across the timeline, and each period is a
row under it. The period is a box of about 144 pixels at the left of the
axis. Its events are boxes of about 213 pixels in a stack at the right of
the axis,
each with a dashed line from the axis.

A text wraps at the width of Mermaid, scaled by 11/16, as "Labels that
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
color. The axis and its arrowhead take the color `edge`.

zuzu reads `timeline.disableMulticolor`, and a value that is not true or
false is an issue. The other settings of `timeline`, such as `padding` and
`leftMargin`, are issues, and so are the colors of `themeVariables`, such
as `cScale0`.

#### C4

The keywords are `C4Context`, `C4Container`, `C4Component`, `C4Dynamic`
and `C4Deployment`. The five kinds read the same statements, as in
Mermaid. The keyword is alone on its line, and each statement is on a line
of its own.

| Statement | What zuzu draws |
| --- | --- |
| `Person(alias, "Label", "Description")`, `Person_Ext` | a person |
| `System(alias, "Label", "Description")`, `System_Ext`, `SystemDb`, `SystemDb_Ext`, `SystemQueue`, `SystemQueue_Ext` | a software system |
| `Container(alias, "Label", "Technology", "Description")`, with the same six forms | a container |
| `Component(alias, "Label", "Technology", "Description")`, with the same six forms | a component |
| `Enterprise_Boundary(alias, "Label") {`, `System_Boundary`, `Container_Boundary` | a boundary of the type `ENTERPRISE`, `SYSTEM` or `CONTAINER`, up to its `}` |
| `Boundary(alias, "Label", "type") {` | a boundary of a type, `system` by default |
| `Deployment_Node(alias, "Label", "type", "Description") {`, `Node`, `Node_L`, `Node_R` | a deployment node of a type, `node` by default |
| `Rel(from, to, "Label", "Technology")`, `BiRel`, `Rel_Back` | a relationship with an arrowhead at `to`, at both ends, or at `from` |
| `Rel_U`, `Rel_Up`, `Rel_D`, `Rel_Down`, `Rel_L`, `Rel_Left`, `Rel_R`, `Rel_Right` | a relationship, as `Rel` draws it |
| `RelIndex(1, from, to, "Label")` | a relationship. The index draws nothing |
| `UpdateElementStyle(alias, $bgColor="grey", $fontColor="red", $borderColor="red", $shape="cylinder")` | the colors and the shape of an element, or the colors of a boundary |
| `UpdateRelStyle(from, to, $textColor="blue", $lineColor="blue", $offsetX="5", $offsetY="-10")` | the colors of a relationship and the place of its label |
| `UpdateLayoutConfig($c4ShapeInRow="3", $c4BoundaryInRow="1")` | the number of elements and of boundaries in a row |
| `title Text`, `accTitle: Text` | the title. The last of the two wins, as in Mermaid 12.3 |
| `accDescr: Text`, `accDescr { ... }`, `accDescription Text` | the description of the Vega spec |

The arguments of a statement are in parentheses right after its keyword,
with commas between them. An argument is a text in double quotes or a text
with no quotes. A text with no quotes runs to the next comma, as in
Mermaid. So `Person(a)` takes the `)` and the lines after it, up to the
next comma, and it is an error when no comma comes. Two commas, or `""`,
give an empty argument. Each argument has a place, as the table shows.
Mermaid ignores an argument after the last place, and so does zuzu, with
no issue (D32).

A named argument, such as `$tags="v1"`, sets the part of its name from any
place. An element reads `$label`, `$descr`, `$techn`, `$sprite`, `$tags`,
`$link`, `$shape`, `$bgColor`, `$fontColor` and `$borderColor`. A boundary
reads `$label`, `$type`, `$descr`, `$sprite`, `$tags`, `$link`, `$bgColor`,
`$fontColor` and `$borderColor`. A relationship reads `$label`, `$techn`,
`$descr`, `$sprite`, `$tags`, `$link`, `$textColor`, `$lineColor`,
`$offsetX` and `$offsetY`. Another name is an issue. A second statement
with the alias of an element or of a boundary updates it, and it keeps its
place. A second relationship between the same two ends replaces the first.

An element draws as a shape of its kind: a person as a head over a body, a
database as a cylinder, a queue as a cylinder on its side, and every other
element as a rounded rectangle. `$shape`, `$sprite` or a tag can name
another shape: `person`, `box` or `rounded`, `cylinder`, `database` or
`db`, `queue` or `pipe`, and `component`, a rectangle with a line at each
side. The shape of `$shape` wins over the shape of `$sprite`, which wins
over the shape of the first tag that names one. An element shows its
name, its kind in brackets, such as `[Software System]` or
`[Container: Java]`, and its description. The texts wrap at the width of
the element less its padding, scaled by 11/14 (D32).

zuzu places the elements and the boundaries as Mermaid 12.3 does:

1. The diagram is a boundary. Each boundary places its elements first, in
   the order of the source, and then its boundaries under them.
2. Elements fill a row from left to right, up to `c4ShapeInRow`, 4 by
   default. A row also ends where the next element passes 800 pixels, the
   width of the screen of `mmdc`, so that a row holds two elements of the
   default width. A boundary that starts a row of boundaries takes the
   count of elements of the boundary before it, as in Mermaid.
3. Boundaries fill a row up to `c4BoundaryInRow`, 2 by default. The width
   of 800 pixels splits between the boundaries of a row.
4. An element is `c4.width` wide at least, 216 by default, with
   `c4.c4ShapePadding` around its texts, 20 by default. The first element
   of a row is `c4.c4ShapeMargin` from the edge of its boundary, 50 by
   default, and two elements are two times it apart. A boundary is
   `c4.diagramMarginX` and `c4.diagramMarginY` inside its parent, 50 and
   10 by default, under its title.
5. A relationship goes from the edge of the box of one end toward the
   middle of the other end. An end can be a boundary. The first
   relationship is straight, and each other one is a curve, as in Mermaid.
6. The label of a relationship starts at the middle of its line, moved by
   `$offsetX` and `$offsetY`. The technology goes under it in brackets, in
   italic. In a `C4Dynamic`, each label starts with the number of the
   relationship in the order of the source, as in `1: Calls`.

zuzu reads `c4.c4ShapeMargin`, `c4.c4ShapePadding`, `c4.width`,
`c4.diagramMarginX`, `c4.diagramMarginY`, `c4.nextLinePaddingX`, and
`c4.wrap`, which wraps the texts of an element, true by default. `wrap` at
the top of the config wraps the texts of a boundary when `c4.wrap` is true
too, as in Mermaid. Each other setting is an issue, and so is
`c4.c4ShapeInRow`, which Mermaid 12.3 does not read.

These are errors, as in Mermaid: a diagram with no statement, a text after
the keyword, a statement with no `(` right after its keyword or with no
arguments, a text with no end quote, a named argument with an empty value,
two statements on a line, a word that is not a keyword, a line with
`direction` and `TB`, `BT`, `LR` or `RL`, a boundary with no `{`, a
statement on the line of a `{`, an empty boundary, a boundary that starts
with a title, a `}` with no boundary, a boundary with no `}`, and a
relationship to an alias that is not an element or a boundary. zuzu also
makes these errors, which draw wrong in Mermaid: a named argument in the
place of an alias, a boundary inside itself, and a boundary with the alias
`global`.

These are issues: a relationship with no label, which Mermaid does not
draw, a named argument in the place of a label, which Mermaid draws as no
label, and `$shadowing`, `$legendText` and `$techn` of
`UpdateElementStyle`.

Mermaid 12.3 reads these parts and draws nothing for them, so zuzu reads
them with no issue (D32): the direction of `Rel_U` and the other
directions, the side of `Node_L` and `Node_R`, a `$sprite`, a tag or a
`$shape` that names no shape, the `$descr`, the `$sprite` and the tags of
a relationship, the sprite and the tags of a boundary, and an update of
an element or of a relationship that does not exist yet.

zuzu draws a C4 diagram in other ways than Mermaid in four places. A
comment after a statement on its line is a comment, and Mermaid reads it
as an error. The `title` of the front matter draws when the diagram has no
`title`, and Mermaid does not draw it. A boundary with a fill draws under
its elements, and Mermaid draws it over them. The texts are in the sizes
of zuzu, so the boxes are smaller than in Mermaid.

The look is the look of zuzu. An element is a shape with a tint of its
color. A person, a system, a container and a component take the first
four colors of the category scheme, as Mermaid colors the elements by
kind. Each external element, such as `Person_Ext` or `System_Ext`, takes
the neutral color `muted` of `docs/STYLE.md` at the same tint, as Mermaid
draws it gray (D32). Its style is `zuzu.node.muted`. A color of
`UpdateElementStyle` replaces the color of the scheme, and its `bgColor`
draws opaque. A boundary is a panel with a dashed outline, and a
deployment node is a panel with a solid outline. Its title, its type and
its description are at its top left. A relationship and its arrowheads
take the color `edge`, and `$lineColor` replaces it. A `$link` gives an
element, a boundary or a relationship a link (D32).

#### ishikawa

The keywords are `ishikawa` and `ishikawa-beta`. An ishikawa diagram, or
fishbone, draws a problem at the head of a spine. The causes of the
problem are bones off the spine, and the causes of a cause are smaller
bones off its bone.

Each line after the keyword is a statement: a text with an indent. A text
after the keyword, on its line, is the first statement, as in Mermaid.
zuzu reads the statements as Mermaid 12.3 reads them:

1. The first statement is the problem.
2. The indent of the second statement is the indent of a cause.
3. Each other statement belongs to the last statement before it with a
   smaller indent. A statement with an indent that is not more than the
   indent of a cause is a cause. So when all the statements have one
   indent, each statement after the problem is a cause.
4. `<br>` starts a new line in a text, and an entity, such as `#quot;`,
   reads as its character.
5. `accTitle` and `accDescr` are causes, because Mermaid reads them as
   text in an ishikawa diagram.

A text that starts with the word `ishikawa`, in any case, is an error, as
in Mermaid. A diagram with no statement draws nothing, as in Mermaid. The
`description` of the front matter goes in the description of the spec.

zuzu lays out an ishikawa diagram in Go with the rules and the sizes of
Mermaid 12.3, scaled by 11/16, because Mermaid draws its labels at 16
pixels (D32). These sizes are in the pixels of Mermaid:

1. The head is at the right end of the spine and holds the problem, in
   bold.
2. The causes alternate above and under the spine, the first above. They
   go from right to left in pairs. Each pair starts left of the labels of
   the pair before it.
3. The bone of a cause leans 82 degrees from the spine, toward the left.
   The label of the cause is in a box at the end of its bone.
4. The causes of each side share 500 pixels by the number of the causes
   under them, at 75 pixels at least for a side. A side is at least 2
   times `fontSize` for each cause under its cause with the most causes.
   The spine is 250 pixels from the top at least. A cause with no causes
   has a bone of 0.2 of the length of its side.
5. The causes of a cause start along its bone at equal spaces. The bone of
   such a cause runs along the spine, 30 pixels long, or 60 pixels and 5
   for each of its own causes. The bones of its causes lean as the bone of
   a cause, and the bones of their causes run along the spine again.
   Above the spine, each list of causes goes in the other order, as in
   Mermaid.
6. The label of a cause of a cause ends at the left end of its bone. On a
   bone that leans, the label is over the end of the bone above the spine,
   and under it below the spine.
7. Each bone has an arrowhead at its start, which points at the spine or
   at the bone that it starts on.

Mermaid wraps the labels of an ishikawa diagram by the number of their
characters, and not by their width, so zuzu wraps them at the same
characters, with no scale. The label of a cause of a cause wraps at 15
characters, and the label of a cause does not wrap. The problem wraps at
110 divided by 0.6 times `fontSize`, and at 6 characters at least. This
is 11 characters by default.

zuzu reads `fontSize`, 16 by default, and `ishikawa.diagramPadding`, 20 by
default. `fontSize` sets where the problem wraps and the least length of a
side, as in Mermaid. A `fontSize` that is not a positive number and a
padding that is not a number of 0 or more are issues. Each other setting
is an issue, except `ishikawa.useMaxWidth`, which changes nothing.

zuzu draws some ishikawa diagrams in another way than Mermaid 12.3.0:

1. The box of a cause can reach right of the start of its bone. Mermaid
   then draws it over the box of the cause before it on its side. zuzu
   moves the pair of the cause left, so that the two boxes do not overlap.
2. Mermaid reads a `%%` in the middle of a line as text. zuzu reads it as
   the start of a comment, as in the other kinds.
3. Mermaid fails on an empty line right after the keyword. zuzu reads it.
4. Mermaid reads the keyword in any case, such as `ISHIKAWA`. zuzu reads
   only `ishikawa` and `ishikawa-beta`.
5. Mermaid counts a tab of an indent as one space, and zuzu as four. The
   causes are the same, unless a source mixes tabs and spaces.
6. Mermaid does not draw the `title` of the front matter. zuzu draws it,
   as in the other charts.
7. Mermaid draws an `&` of a label as `&amp;`. zuzu draws `&`.

The look is the look of zuzu. The head and the box of each cause take a
tint of the first color of the category scheme, as Mermaid draws them in
one color. The spine, the bones and the arrowheads take the color `edge`.
The spine and the bone of each cause are 2.5 pixels wide, and the smaller
bones take `edgeWidth`. The arrowhead of the bone of a cause is
`arrowLength` long, and the arrowhead of a smaller bone is 0.75 of it.
The padding around the diagram is 8 pixels, scaled by
`ishikawa.diagramPadding` over 20.

#### cynefin-beta

The keyword is `cynefin-beta`, or `cynefin-beta:`. A Cynefin diagram draws
the four domains of the Cynefin framework, the items in each domain, and
the transitions between them. Statements can follow the keyword on its
line, as in Mermaid.

| Statement | What zuzu draws |
| --- | --- |
| `title Text` | the title, over the `title` of the front matter |
| a domain: `complex`, `complicated`, `clear`, `chaotic` or `confusion` | the start of a block of items of that domain |
| a string in double or single quotes after a domain, as `"Restart service"` | an item of the domain |
| `complex --> complicated : "Label"` | a transition from one domain to another, with a label or none |
| `accTitle: Text`, `accDescr: Text`, `accDescr { ... }` | the description of the Vega spec, the texts joined in order |

zuzu reads the statements as the grammar of Mermaid 12.3 reads them:

1. The items of a block can be on the line of the domain, or on the lines
   after it, and two blocks can be on one line.
2. A block of a domain that the source names again replaces the items of
   the block before it, as in Mermaid. A block with no items draws its
   domain with none.
3. A transition is on one line and nothing follows it. Its label is a
   string. A transition from a domain to itself draws nothing, with no
   issue, as in Mermaid.
4. A backslash in a string escapes the next character. An escape of a new
   line or a tab draws as a space, as in the SVG of Mermaid. An entity,
   such as `#quot;`, reads as its character.
5. `title` alone sets an empty title, which draws no title.

These are errors, as in Mermaid: a string before the first domain or after
a transition, a word that is not a domain, such as `complexity`, a string
with no end quote, a label that is not a string, a transition with text
after it, and `-->` on a line of its own.

zuzu lays out a Cynefin diagram with the sizes of Mermaid 12.3. The
diagram is 800 by 600 pixels, or `cynefin.width` by `cynefin.height`.
Complex is the top left quarter, complicated the top right, chaotic the
bottom left and clear the bottom right. A dashed wavy boundary runs from
the top to the bottom and another from the left to the right, through the
middle. Each boundary is 7 curves that wave by `cynefin.boundaryAmplitude`,
8 pixels by default, and 0 makes it straight. The waves come from
`cynefin.seed`, with the numbers of Mermaid, so the same seed draws the
same waves. When the seed is 0, the default, zuzu seeds the waves with the
id of the SVG of mmdc, `my-svg`, and draws the waves of mmdc. A thick
boundary, the cliff, runs from the middle down to the bottom between
clear and chaotic. Confusion is an ellipse in the middle, 0.15 of the
width and of the height across each half.

The name of each domain is at its center, with its model and its practice
under it, such as "Probe → Sense → Respond" and "Emergent Practices".
`cynefin.showDomainDescriptions: false` hides them. The items of a domain
are badges under its name, one under the other. Confusion shows its first
3 items, and a dashed badge, such as "+2 more", counts the rest, as in
Mermaid. A transition is a curve from the center of one domain to the
center of another, bent to its left by 0.15 of its length, with an
arrowhead at its end. Its label is over the control point of the curve. A
label of a Cynefin diagram does not wrap, as in Mermaid.

Each domain takes a color of the category scheme of the theme. In the
scheme `zuzu`, complicated is blue violet, complex teal, clear saffron,
chaotic raspberry and confusion violet, the hues that Mermaid gives them.
A domain is a light tint of its color, and its items have a tint and an
outline in its color. The cliff takes the color of chaotic, and the
boundaries, the transitions and their arrowheads take the color `edge`.

zuzu reads `cynefin.width`, `cynefin.height`, `cynefin.padding`,
`cynefin.showDomainDescriptions`, `cynefin.boundaryAmplitude` and
`cynefin.seed`. zuzu draws its own margin of 8 pixels, and
`cynefin.padding` replaces it. A value of the wrong type, such as a width
that is not a positive number, is an issue. zuzu also reads these
variables of `themeVariables.cynefin`: `complexBg`, `complicatedBg`,
`clearBg`, `chaoticBg` and `confusionBg` replace the color of a domain,
`boundaryColor` the color of the boundaries, `cliffColor` the color of the
cliff, `arrowColor` the color of the transitions, `textColor` the color of
the items and the labels, and `labelColor` the color of the names. The
other variables of `themeVariables.cynefin`, such as `cliffWidth` or
`itemFontSize`, are issues.

zuzu draws a Cynefin diagram in another way than Mermaid 12.3.0:

1. Mermaid centers the title over the diagram. zuzu puts it at the top
   left, as in every chart.
2. Mermaid draws the outline of an item in the color of the boundaries.
   zuzu draws it in the color of the domain.
3. Mermaid colors the domains with the colors of `themeVariables.cynefin`,
   whatever the theme. zuzu colors them with the category scheme that the
   theme selects.

#### info

The keyword is `info`. `showInfo` can follow it, on its line or the next,
and changes nothing, as in Mermaid. A `title`, an `accTitle` and an
`accDescr` can come after them. Each other text is an error, as in
Mermaid.

mmdc draws the text `v12.3.0`, the version of Mermaid, at 32 pixels, for
`info` and for `info showInfo`. zuzu draws the same text at the same size,
because zuzu reads Mermaid 12.3.0. It does not draw the version of zuzu.
Mermaid draws no title on an info diagram, not even the `title` of the
front matter, and neither does zuzu. `accTitle` and `accDescr` go in the
description of the Vega spec. An info diagram has 2 fixtures.

#### kanban

The keyword is `kanban`. Each node is on a line of its own. The first node
is a column, and so is each node with the indent of the first. Each other
node is a card of the column before it, at any indent deeper than the
first. A node under a card is a card of the same column, as in Mermaid.

| Statement | What zuzu draws |
| --- | --- |
| a node with an id and a text in a shape, as `todo[Todo]` or `id2("A text")` | a column or a card with the text |
| a text in a shape with no id, as `[A card]` | a column or a card with the text |
| an id alone, as `Todo` | a column or a card with the id as its text |
| the data of a node after it, in `@{` and `}`, as `a[A]@{ ticket: MC-2038, assigned: 'knsv', priority: 'High' }` | the ticket, the assignee and the priority of a card, and a text from `label` |
| `::icon(fa fa-book)` after a node | nothing, as in Mermaid |
| `:::urgent` after a node | nothing, as in Mermaid |

zuzu reads the statements as the grammar of Mermaid 12.3 reads them:

1. A shape is `[…]`, `(…)`, `((…))`, `)…(`, `))…((` or `{{…}}`, as in a
   mindmap. Mermaid draws each shape as a column or a card, and so does
   zuzu.
2. A text in a shape can be in double quotes, or a string of markdown in
   double quotes and backticks. A text with no quotes, a string and the
   data of a node can run over more lines, up to their end.
3. Each text is markdown, as in Mermaid, so `**bold**` draws in bold. `<br>`
   starts a new line, and an entity, such as `#quot;`, reads as its
   character.
4. The data of a node is a map of YAML. `label` replaces the text of the
   node. `ticket`, `assigned` and `priority` draw on a card. Mermaid draws
   nothing for `icon` and for each other key, and zuzu reads them with no
   issue (D32).
5. `accTitle`, `accDescr` and `title` are not statements of a kanban.
   Mermaid reads each as the text of a node, and so does zuzu.

These are errors, as in Mermaid: two nodes on a line, a text after a node,
a shape with no text or with no end, a space between a string and the end
of its shape, a node after a node with less indent than the first node, a
`shape` in the data of a node with a capital letter or an underscore, a
`^` in the data of a node, an icon or a class before the first node, and
the keyword `kanban` on a later line.

zuzu lays out a kanban in Go with the sizes of Mermaid 12.3, scaled by
11/16, because Mermaid draws the labels of a kanban at 16 pixels (D32):

1. The columns are side by side, in the order of the source. A column is
   `kanban.sectionWidth` wide, 200 pixels of Mermaid by default, which is
   137.5 pixels in zuzu. Its name is at its top, and wraps at its width.
2. The cards of every column start under the tallest name, as in Mermaid.
   A card is 15 pixels of Mermaid narrower than its column, and its text
   wraps at 10 pixels less than the card. Each column is as tall as its
   cards.
3. The ticket of a card is at its bottom left, and the assignee at its
   bottom right, under the text.
4. A card with the priority `Very High`, `High`, `Low` or `Very Low` has a
   line at its left in the color of Mermaid: red, orange, blue or light
   blue. Mermaid draws no line for `Medium` or another value, and neither
   does zuzu.

With `kanban.ticketBaseUrl`, the ticket of a card is a link (D32): the URL
with the ticket in place of `#TICKET#`. Mermaid opens the link in another
window. The SVG of zuzu holds the link and not the target, as "Links and
tooltips" says. A link with a scheme that zuzu does not write, such as
`javascript:`, is an issue, and the ticket draws with no link.

zuzu reads `kanban.sectionWidth`, a positive number, and
`kanban.ticketBaseUrl`, a text. Another value of either is an issue.
Mermaid 12.3 reads `kanban.padding` and `mindmap.maxNodeWidth` and draws
nothing for them, so zuzu reads them with no issue (D32). Each other
setting is an issue, such as `mindmap.padding`, which sets the margin of
the image of Mermaid.

zuzu draws a kanban in other ways than Mermaid in three places. Mermaid
does not draw the `title` of the front matter, and zuzu draws it. The
texts are in the sizes of zuzu, so a line of a text can hold a word more
or less than in Mermaid. Mermaid colors only the first ten columns. zuzu
colors each column, and the eleventh column takes the first color again.

The look is the look of zuzu. Each column takes the next color of the
category scheme, as Mermaid colors the columns by their order. A column is
a box with a tint of its color, and its name is in its color. A card is a
box of the background with an outline in the color of its column. The
ticket and the assignee are in the color `muted`.

#### packet

The keyword is `packet` or `packet-beta`. Each statement is on a line of
its own, and a statement can follow the keyword on its line, as in
Mermaid.

| Statement | What zuzu draws |
| --- | --- |
| `0-15: "Source Port"` | a field of the bits 0 to 15 |
| `106: "URG"` | a field of one bit |
| `+16: "Source Port"` | a field of 16 bits after the field before it |
| `title Text` | the title |
| `accTitle: Text`, `accDescr: Text`, `accDescr { ... }` | the description of the Vega spec, the texts joined in order |

A label is a string in double or single quotes. A backslash in it takes
the character after it, as in Mermaid, so `"a \"b\""` is `a "b"`. The
last `title` with a text is the title, and the `title` of the front matter
draws when the packet has none.

These are errors, as in Mermaid: a field that does not start at the bit
after the field before it, or a first field that does not start at 0, a
field that ends before it starts, a field of `+0` bits, a label with no
quotes, two statements on a line, a number with a 0 before it, such as
`01`, a number with a fraction, and a word that is not a statement, such
as `TITLE`.

zuzu lays out a packet with the sizes of Mermaid 12.3, as Mermaid lays it
out:

1. A row holds `packet.bitsPerRow` bits, 32 by default. A bit is
   `packet.bitWidth` wide, 32 pixels by default, and a row is
   `packet.rowHeight` high, 32 pixels by default.
2. A field that does not fit in the rest of its row goes on in the next
   row, with its label in each row.
3. A field is as wide as its bits, less `packet.paddingX`, 5 pixels by
   default. Two rows have `packet.paddingY` between them, 5 pixels by
   default, and 10 more when the numbers of the bits draw.
4. With `packet.showBits`, true by default, the number of the first bit of
   each field is over its left end, and the number of its last bit over
   its right end. A field of one bit has one number, in its middle.
5. With `packet.bitOrder: descending`, the bits of a row run from right to
   left, and the number over the left end of a field is its last bit.

zuzu reads `packet.rowHeight`, `packet.bitWidth`, `packet.bitsPerRow`,
`packet.paddingX`, `packet.paddingY`, `packet.showBits` and
`packet.bitOrder`. A value that is not a positive number, a `bitsPerRow`
with a fraction, a `showBits` that is not true or false, and a `bitOrder`
that is not `ascending` or `descending` are issues. Each other setting is
an issue.

zuzu reads the theme variables of a packet in `themeVariables.packet`, as
Mermaid does (D32): the colors `blockFillColor`, `blockStrokeColor`,
`labelColor`, `startByteColor`, `endByteColor` and `titleColor`, and the
sizes `blockStrokeWidth`, `labelFontSize`, `byteFontSize` and
`titleFontSize`, in pixels. A color that is not a text, a size that is not
a positive number, and each other theme variable are issues.

zuzu draws the title of a packet at its top left, as the title of every
chart. Mermaid draws it under the packet, in the middle. A label that is
wider than its field draws past the field, as in Mermaid.

The look is the look of zuzu. A field is a box with a tint of the first
color of the scheme and an outline. Its label is in the color `text`, and
the numbers of its bits are in the color `muted`.

#### railroad-beta

A railroad diagram draws each rule of a grammar as a track. A terminal and
a nonterminal are boxes on the track, and a choice, an optional part and a
repeat are tracks that branch and loop. Mermaid 12.3 reads four notations
of a grammar into the same model, each with its own keyword:

| Keyword | Notation | A rule |
| --- | --- | --- |
| `railroad-beta` | calls of functions | `number = oneOrMore(nonterminal("digit")) ;` |
| `railroad-ebnf-beta` | EBNF | `number = digit+ ;` or `number ::= digit , { digit } ;` |
| `railroad-abnf-beta` | ABNF | `number = 1*DIGIT ;` |
| `railroad-peg-beta` | PEG | `Number <- Digit+ ;` |

The keyword comes first, in its own case. Then come `title`, `accTitle`
and `accDescr`, in any order, and then the rules. Each rule ends with a
`;`. A new line is a space, so a rule can run over many lines. The title
is the title of the diagram. A title in quotes, such as `title "JSON"`,
draws without its quotes, and in each notation but ABNF, a backslash
escapes the next character, as `\"` or `\n`. `accTitle` and `accDescr` join
into the description of the Vega spec. A later `title`, `accTitle` or
`accDescr` wins over an earlier one of its kind, as in Mermaid.

The parts of `railroad-beta`:

| Part | What zuzu draws |
| --- | --- |
| `terminal("text")` | a terminal: a box with round corners |
| `nonterminal("name")` | a nonterminal: a box |
| `special("text")` | a special: a box with a dashed outline and the text between `?` and `?` |
| `sequence(a, b, ...)` | the parts in a row on one track |
| `choice(a, b, ...)` | the parts as alternatives, one under another |
| `optional(a)` | the part with a bypass over it |
| `oneOrMore(a)` | the part with a loop under it |
| `zeroOrMore(a)` | the part with a loop under it and a bypass over it |

A string is in double or single quotes, and a backslash escapes the next
character. A name of a rule starts with a letter or `_`, and holds
letters, digits, `_` and `-`. A comment runs from `/*` to `*/`.

The parts of EBNF:

| Part | What zuzu draws |
| --- | --- |
| `a = ...`, `a ::= ...` | a rule |
| `"text"`, `'text'` | a terminal |
| a name, as `digit` | a nonterminal |
| `? text ?` | a special |
| `a b`, `a , b` | a sequence |
| `a \| b` | a choice |
| `( ... )` | a group |
| `[ ... ]`, `a?` | an optional part |
| `{ ... }`, `a*` | a repeat with a bypass |
| `a+` | a repeat |
| `a - b` | a sequence of `a`, a terminal `-` and `b`, as Mermaid draws an exception |

A comment runs from `(*` to `*)`, or from `/*` to `*/`.

The parts of ABNF:

| Part | What zuzu draws |
| --- | --- |
| `"text"` | a terminal, with no escapes |
| `%x41`, `%x41-5A`, `%d13.10` | a terminal with the value as its text |
| a name, as `ALPHA` or `local-part` | a nonterminal |
| `a b` | a sequence |
| `a / b` | a choice |
| `( ... )` | a group |
| `[ ... ]`, `*1a` | an optional part |
| `*a`, `0*5a` | a repeat with a bypass |
| `1*a`, `2*5a`, `3a` | a repeat |

The parts of PEG:

| Part | What zuzu draws |
| --- | --- |
| `A <- ...` | a rule |
| `"text"`, `'text'` | a terminal |
| a name, as `Digit` | a nonterminal |
| `.` | a special with the text `.` |
| `a b` | a sequence |
| `a / b` | a choice |
| `( ... )` | a group |
| `a?`, `a*`, `a+` | an optional part, a repeat with a bypass, a repeat |
| `!a`, `&a` | a special with the text `!a` or `&a`. A part that is not a terminal, a nonterminal or a special shows as `(...)`, as `!(...)` |

A comment runs from `#` to the end of the line.

zuzu reads the tokens as the lexer of Mermaid 12.3 reads them. At each
place, the first kind of token that matches wins: the keywords first, then
`title`, `accTitle` and `accDescr`, then the names and the strings. This
has effects that a reader does not expect, and zuzu keeps them:

1. A name that starts with `title`, such as `titled`, reads as `title`
   and a rule `d`.
2. A keyword, such as `terminal`, is not a name, but `terminals` is.
3. In EBNF, `b? c? ;` reads as `b` and a special `c`, because `? c?` is a
   special sequence.
4. In ABNF, a `;` with more text after it on its line starts a comment,
   so `a = b ; c` is a rule with no end.

These are errors, as in Mermaid: a part that the notation does not have,
such as `[a-z]` in PEG, `=/` in ABNF or `'x'` in ABNF, a rule with no `;`,
a rule with no definition, a call of `sequence` or `choice` with no part,
a call of `optional`, `oneOrMore` or `zeroOrMore` with more than one part,
a `title` after the first rule, two `!` or two `?` in a row in PEG, and a
keyword with another case, such as `Railroad-beta`.

Mermaid 12.3 reads these parts and draws nothing for them, so zuzu reads
them with no issue (D32): the counts of a repeat of ABNF, so that `3DIGIT`
draws as `1*DIGIT` does, and the settings `railroad.compactMode` and
`railroad.showMarkers`. Mermaid draws the markers either way.

zuzu lays out a railroad diagram in Go with the sizes of Mermaid 12.3,
scaled by 11/16, because Mermaid draws its text at 16 pixels. The rules
stack from top to bottom. Each rule has its name at the left, then a start
marker, its track and an end marker. A box is as wide as its text, with
the padding at each side. A sequence puts its parts in a row, with a space
between two parts. A choice puts its alternatives one under another, each
in the middle of the widest. Its track enters at the middle of the
choice, and curves to each alternative. An optional part has a bypass
over it. A repeat has a loop that runs back under it, and a bypass over it
when it can be empty.

zuzu reads these settings of `railroad`, in the pixels of Mermaid, which
scale by 11/16:

| Setting | What it sets | Default |
| --- | --- | --- |
| `padding` | the space between the text of a box and its outline | 10 |
| `verticalSeparation` | the space between two alternatives, and between two rules | 8 |
| `horizontalSeparation` | the space between two parts of a sequence | 10 |
| `arcRadius` | the radius of the curves of the tracks | 10 |
| `markerRadius` | the radius of a start or an end marker | 5 |

A value that is not a number of 0 or more is an issue, and the size takes
the default. The other settings of `railroad`, such as `terminalFill` or
`fontSize`, are issues, and so are the colors of `themeVariables`. The four
notations read the section `railroad`. The key `config` of a directive
sets the section `railroadEbnf`, `railroadAbnf` or `railroadPeg` in the
other three notations, as "Every kind" says, and Mermaid does not read
those sections.

zuzu draws some railroad diagrams in another way than Mermaid 12.3.0:

1. Mermaid draws no title. zuzu draws the `title` of the diagram, or else
   the `title` of the front matter.
2. Mermaid cleans a text with `<` in it as HTML, and draws `<` as `&lt;`.
   zuzu draws the text as the source gives it, and reads an entity, such as
   `#quot;`.
3. zuzu removes the spaces at the end of a line before it reads the line.
   So in ABNF, a `;` with only spaces after it ends the rule, and Mermaid
   reads it as the start of a comment.
4. zuzu measures the text in the size of zuzu, so the boxes are smaller
   than in Mermaid.

The look is the look of zuzu. A nonterminal, a terminal and a special take
the first three colors of the category scheme, as Mermaid colors them by
kind. Each box has a tint of its color and an outline in its color. A
terminal has round corners, and a special has a dashed outline. A text
with only spaces draws an empty box, as in Mermaid. The name of a rule is
bold. The tracks and the markers take the color `edge`.

#### treeView-beta

The keyword is `treeView-beta`, alone on its line. A tree view draws a tree
of files and directories, one item on each row. Each item after the
keyword is a name and its annotations, in any order:

| Part | What zuzu draws |
| --- | --- |
| a name, as `index.js`, `"my file"` or `'my file'` | an item, a file |
| a name that ends in `/`, as `src/` | a directory, with its name in bold and without the `/` |
| `:::highlight` | a band behind the row of the item |
| `:::name` with another name | nothing, as in Mermaid |
| `icon(pack:name)`, `icon(name)` | the icon before the name |
| `icon(none)`, `icon()` | no icon |
| `## Text` | the description of the item, in a column at the right |
| `title Text`, before the first item | the title |
| `accTitle: Text`, `accDescr: Text`, `accDescr { ... }`, before the first item | the description of the Vega spec, the texts joined in order |

The indent of an item sets its parent. The parent is the last item before
it with a smaller indent. An item with no such item has the root as its
parent. Mermaid draws the root as a directory `/` on the first row, and so
does zuzu. An indent counts its spaces and tabs, and a tab counts as one,
as in Mermaid. The last annotation of a kind wins, so `a :::x :::y` has
the class `y`.

A tree can also be drawn with the characters of boxes, as the command
`tree` draws it:

```
treeView-beta
├── src/
│   ├── index.ts
│   └── utils.ts
└── README.md
```

When a line after the keyword holds `─`, `━`, `│`, `┃`, `├`, `┣`, `└` or
`┗`, zuzu reads the tree as Mermaid 12.3 does. The first `├`, `┣`, `└` or
`┗` that is not at the start of its line sets the width of a level, 4 by
default. The depth of an item is its column over that width, rounded,
plus 1. A line of only `│` and spaces drops.

zuzu reads the source as the lexer of Mermaid 12.3 reads it:

1. A comment on a line of its own drops, as in Mermaid. `%%` after a name
   with no quotes is part of the name.
2. A name with no quotes runs to the end of the line, or to the space
   before an annotation. Its spaces at the end drop, and the spaces in it
   stay.
3. A name in quotes ends at its end quote. Two names in quotes on one line
   are two items.
4. A word that starts with `title`, such as `titles.txt`, starts a title,
   as in Mermaid. So `titles.txt` is the title `title` and the item
   `s.txt`, before the first item, and an error after it.
5. An annotation takes a space before it. A description runs to the end of
   the line, and can hold `:::` and `##`.

Spaces at the end of a line after a name in quotes or after an
annotation, a line of only spaces, and `%%` alone on an indented line read
as the indent of the next item, as in Mermaid. So each is an error when
the next line has an indent too, or at the end of the source.

These are errors, as in Mermaid: text after the keyword on its line, even
a space, a title, an accTitle or an accDescr after the first item, a
character that starts no part, such as the `i` of `icon(a:b:c)` or the `:`
of `"a":::b`, and the keyword on a later line. In a tree drawn with boxes,
a branch with no name and a line with an indent and no branch are errors
too.

zuzu reads these settings of `treeView`, in the pixels of Mermaid, and a
value that is not a number of 0 or more, or not true or false, is an
issue:

| Setting | What it does |
| --- | --- |
| `rowIndent` | the indent of a level, with `paddingX`. 10 by default |
| `paddingX` | the space at the left and the right of a name. 5 by default |
| `paddingY` | the space above and under a name. 5 by default |
| `lineThickness` | the width of the lines, as a multiple of `edgeWidth`. 1 by default |
| `showIcons` | draws an icon before each item: a folder, a file, or the icon of its name |
| `defaultIconPack` | the pack of an icon with no pack, such as `bell` |
| `filenameIcons` | an icon by the name of a file, such as `Dockerfile: docker` |
| `extensionIcons` | an icon by the extension of a file, such as `.ts: typescript`. `none` hides the icon |

An icon with no pack takes the pack of `defaultIconPack`, except `folder`
and `file`, which are the folder and the file of a tree view. zuzu draws
these two as shapes of its own. Every other icon draws as "Icons and
images" says, and a missing icon draws as a box with a question mark, as
in Mermaid. In zuzu, a missing icon is an issue too, when the chart
renders.

zuzu reads these theme variables of `treeView`, and each other theme
variable is an issue:

| Theme variable | What it does |
| --- | --- |
| `labelFontSize` | the size of a label, as 16 pixels in Mermaid are 11 in zuzu. A size of 20 draws at 13.75 |
| `labelColor` | the color of a name |
| `lineColor` | the color of the lines |
| `iconColor` | the color of an icon of one color |
| `descriptionColor` | the color of a description |
| `highlightBg`, `highlightStroke` | the fill and the outline of a band |

zuzu lays out a tree view in Go with the sizes of Mermaid 12.3, scaled by
11/16, because Mermaid draws its labels at 16 pixels (D32). A label does
not wrap, as in Mermaid. Each item is a row, in the order of the source.
A line runs down from each item with children to the middle of the row of
its last child, and a line runs from it to each child. The descriptions
are in a column after the widest name. A band runs from the left of its
row to past the right of the tree, and each band is a little wider than
the band before it, as in Mermaid.

The look is the look of zuzu. A line takes the color `edge` and the width
`edgeWidth`. A name takes the color of a label, and a description is in
italic in the color `muted`. A band is a tint of the first color of the
category scheme with an outline, and an icon takes that color too.

zuzu draws a tree view in another way than Mermaid 12.3.0 in one place.
Mermaid draws no title, from the front matter or from `title`. zuzu draws
the title, as in the other kinds.

#### eventmodeling

The keyword is `eventmodeling`. An event model is a time axis of frames
from left to right. Each frame is a box in the swimlane of its kind, and an
arrow goes to each frame from the frame that leads to it. A statement can
run over more lines, and a line can hold more statements, as in Mermaid
12.3.

| Statement | What zuzu draws |
| --- | --- |
| `tf 01 ui CartUI`, `timeframe 01 ui CartUI` | a frame: a box with its name, at the next place on the time axis |
| `rf 04 evt Stock.Changed`, `resetframe 04 evt Stock.Changed` | a frame that takes no arrow from the frame before it |
| `->> 02` after a frame, one time or more | an arrow from each frame that the frame names |
| `[[Item]]` after a frame | the data of the data block `Item` in the box, under the name |
| `{ price: number }`, `'text'` or `"text"` at the end of a frame, with a type such as `` `json` `` before it or not | the data in the box, under the name |
| `data Item {`, its lines and a `}` at the start of a line | a data block, which a frame names with `[[Item]]` |
| `entity Cart.Item`, `note 01 {` up to its `}`, `gwt 01 given evt A when cmd B then evt C` | nothing, as in Mermaid 12.3 (D32) |
| `accTitle: Text`, `accDescr: Text` or `accDescr { ... }` after a space in a line | the description of the Vega spec, the texts joined in order |
| `title Text` after a space in a line | the title |

The id of a frame is one to three digits. `1` and `01` are two frames.
The kind is `ui`, `cmd` or `command`, `evt` or `event`, `rmo` or
`readmodel`, and `pcr` or `processor`. The name is a word of letters,
digits and underscores, or more such words with a dot between two. A
keyword of the grammar, such as `data` or `ui`, is not a name.

| Kind | Swimlane | Color |
| --- | --- | --- |
| `ui` | UI/Automation | the neutral color `muted` |
| `pcr`, `processor` | UI/Automation | the fourth color of the category scheme |
| `cmd`, `command` | Command/Read Model | the first color |
| `rmo`, `readmodel` | Command/Read Model | the second color |
| `evt`, `event` | Events | the third color |

Mermaid colors a frame by its kind: a screen white, a processor pink, a
command blue, a read model green and an event orange. The colors of the
table are near these in the scheme `zuzu`. A name with one dot, such as
`Cart.ItemAdded`, has a namespace. Its box shows `ItemAdded`, in a
swimlane of its own under the swimlanes of its kind, with the label
`UI/A: Cart`, `C/RM: Cart` or `Stream: Cart`. Each such frame takes a new
swimlane, also when a frame before it has the same namespace, as in
Mermaid 12.3. A name with two dots or more shows whole, in the swimlane of
its kind.

zuzu lays out an event model in Go with the rules and the sizes of Mermaid
12.3, scaled by 11/16, because Mermaid draws the text of an event model at
16 pixels (D32):

1. The frames take their places in the order of the source. The first box
   starts 172 pixels from the left, 250 in Mermaid. A box in the swimlane
   of the box before it starts 6.9 pixels after that box. A box in another
   swimlane starts 48 pixels before the end of the box before it, so that
   the boxes make steps.
2. A box is at least 69 pixels wide and 69 pixels high, 100 in Mermaid.
   Mermaid measures the name with its tags of HTML, `<b>` and `</b>`, and
   zuzu measures the same text, so that a box is as wide as in Mermaid.
3. The swimlanes go from top to bottom: UI/Automation, Command/Read Model
   and Events, each followed by the swimlanes of its namespaces. Each
   swimlane is as wide as the model, and as high as its highest box with a
   padding.
4. A frame with `->>` takes an arrow from each frame that it names, in the
   order of the source, when that frame is before it or is itself. A frame
   that it names and that is not there, or that comes later, gives no
   arrow. A frame with no `->>` takes an arrow from the last frame before
   it in another swimlane. The first frame and a reset frame take none.
5. An arrow is a straight line from two thirds of the width of its source
   to one third of the width of its target. It goes from the bottom of one
   box to the top of the other, or from the top to the bottom when the
   target is in a higher swimlane.

The name wraps at 430 pixels of Mermaid, scaled by 11/16, as "Labels that
wrap" says. A name that is wider breaks with a hyphen, as in Mermaid. The
data is the text after the first `{`, and after its new line in a data
block, up to the character before the last `}`, as in Mermaid 12.3. So
`{a: 1}` shows `a:`, and data in quotes shows nothing. zuzu draws each
line of the data on a line of its own, at the left of the box, under an
empty line. Each line wraps at 430 pixels of Mermaid. Mermaid draws the
lines of the data as one text in a font of code, which wraps where the box
ends.

These are errors, as in Mermaid: a frame id of more than three digits, a
frame id two times, a kind that is not in the table above, such as `UI`, a
keyword as a name, a dot at the end of a name, a character that is not in
the grammar, such as `-`, a `->>` with no frame id, a type of data that
Mermaid does not have, a data block in the line of a frame, data in braces
on one line after `data` or `note`, a `}` of a data block that does not
start a line, a text right after the `}` of a data block, a namespace in a
`gwt` or in the name of a data block, a `/*` with no `*/`, and the keyword
`eventmodeling` after the first line. `title`, `accTitle` and `accDescr` at
the start of a line are errors too, because Mermaid reads them as names
there.

zuzu reads no setting of `eventmodeling`. Mermaid 12.3 does not read
`eventmodeling.rowHeight`, so zuzu accepts it with no issue (D32). The other
settings, such as `eventmodeling.padding`, are issues, and so are the
colors of `themeVariables`, such as `emUiFill`.

zuzu draws an event model in other ways than Mermaid 12.3.0 in five
places:

1. Mermaid draws no title. zuzu draws the `title` of the source, or the
   `title` of the front matter when the source has none.
2. Mermaid puts the text of `accTitle` and `accDescr` in the SVG with the
   keyword, as `accTitle: Text`. zuzu puts the text only in the description.
3. The first box starts right of the widest label of a swimlane, when that
   label is wider than the space before the first box in Mermaid.
4. A box starts after the last box of its swimlane. Mermaid can draw a box
   over the box before it when the boxes are narrow.
5. zuzu draws the lines of the data as lines, as the paragraph above says.

The look is the look of zuzu. A box is a tint of the color of its kind,
with an outline and its name in bold in the middle. A swimlane is a panel
with its label at its top left. An arrow and its arrowhead take the color
`edge`, `edgeWidth` and `arrowLength`, and draw over the boxes, as in
Mermaid.

#### zenuml

A ZenUML diagram is a sequence diagram written as code. mmdc draws it with
the plugin `@mermaid-js/mermaid-zenuml`, which holds `@zenuml/core`
3.49. zuzu reads the grammar of `@zenuml/core`, and turns each statement
into the participants, the messages, the activations and the frames of a
sequence diagram, as "sequenceDiagram" says. The same layout then draws
it, so the two kinds look the same.

| Statement | What zuzu draws |
| --- | --- |
| `title Text`, before any other statement | the title |
| `A`, `"Name with spaces"`, `A as Alice` at the start | a participant. An alias is the label |
| `@Actor A`, `@Database A`, `@Boundary A`, `@Control A`, `@Entity A` | an actor, a database, a boundary, a control or an entity, as "sequenceDiagram" draws them |
| `<<service>> A` | a participant with `«service»` over its label |
| `group Name { A B }` | a panel behind the participants, with its title |
| `@Starter(A)`, after the participants | A sends each message that names no sender |
| `A.method(x)`, `A->B.method()`, `A.first().second()` | a message with a filled arrowhead, and an activation of the receiver |
| `A.method() { ... }` | the same, and the statements of the block inside the activation |
| `method()` | a message from the sender of the block to itself |
| `result = A.method()`, `Type result = A.method()` | the message, and a dashed message back with the name `result` |
| `A->B: text`, `B: text`, `A->B`, `A - B` | a message with an open arrowhead and no activation |
| `new A(x)`, `a = new A()` | a dashed message to the head of A, which starts there, with `«x»` or `«create»`, and an activation of A. With an assignment, the participant is `a:A`, and a dashed message goes back with the name `a` |
| `return x`, `@return A->B: text`, `@reply A->B: text`, `A-->B: text` | a dashed message with an open arrowhead. `return` goes back to the sender of the block |
| `if (x) { } else if (y) { } else { }` | a frame `alt` with a section for each `else` |
| `while (x) { }`, `for`, `forEach`, `foreach`, `loop` | a frame `loop` |
| `opt { }`, `par { }`, `critical (x) { }` | a frame `opt`, `par` or `critical` |
| `section (x) { }`, `frame (x) { }`, `{ }` | a frame `section` with no label, as ZenUML draws it |
| `try { } catch (e) { } finally { }` | a frame `try` with a section for each `catch` and the `finally` |
| `ref(Name, A, B)` | a frame `ref` across A and B, with the text of the ref |
| `==Text==`, at the start of a line | a dashed line across the diagram, with the text in a box |
| `// Text`, before a statement | the text over the message or the frame |

A participant that no statement declares takes the place where a message
first names it, as in ZenUML. When a message names no sender and the
diagram has no `@Starter`, zuzu draws the starter of ZenUML as the first
participant: an actor with no label. A participant that the head declares
twice keeps the first value of each part, as in ZenUML.

ZenUML numbers each statement in the order of the source, and a statement
in a block takes the number of the block before its own, such as `1.2.1`.
The statements of a frame take the number of the frame, and the sections
of a frame count on. zuzu draws each number in a small text at the left of
its message or of its frame, as ZenUML does.

A colon starts the text of a message, which runs to the end of the line,
as in ZenUML. So `accTitle: Text` and `accDescr: Text` are a message to a
participant named `accTitle` or `accDescr`, as ZenUML draws them, and not
the description of the spec. The `description` of the front matter is
still the description. ZenUML draws its own `title`, and not the `title`
of the front matter, so zuzu does not draw the front matter title either.

ZenUML reads no setting of the config. Each setting is an issue, as in the
other charts, and a known `theme` selects the look, as "The theme" says.

Mermaid never reports an error in a ZenUML diagram, because the parser of
ZenUML skips each token that does not fit its grammar. zuzu reports each
token that does not fit the grammar as an issue, and each token that the
grammar wants and that is not there, such as the `}` of a block. It then
draws the rest.

These are issues, because ZenUML draws them and zuzu does not: the color
of a participant, such as `A #FF0000`, an emoji of a participant, such as
`[smile] A`, the icons of the clouds, `@EC2`, `@Lambda`, `@AzureFunction`,
`@SQS`, `@SNS` and `@IAM`, which draw as participants, a style at the start
of the last line of a comment, such as `[red]`, `<bold>` or `(blue)`, and
the Markdown of a comment, `**`, `__` and `` ` ``, which zuzu takes out
and draws as plain text.

ZenUML reads these parts and draws nothing for them, so zuzu reads them
with no issue (D32): the width of a participant, such as `A 200`, a group
with no name, a type that has no icon, such as `@Queue`, the words
`const`, `readonly`, `static` and `await`, and a comment before a
participant or before a `}`.

zuzu draws a ZenUML diagram in other ways than mmdc in these places. A
return of a participant to itself is a dashed loop, and ZenUML draws an
icon and the text. A frame spans the messages in it, and ZenUML spans
other lifelines. The heads of the participants draw at the top only, as in
ZenUML, in the look of zuzu, and the texts are in the sizes of zuzu.
Labels do not wrap, as in ZenUML.

#### venn-beta

The keyword is `venn-beta`. A venn diagram draws each set as a circle,
and each union of sets as the overlap of their circles.

| Statement | What zuzu draws |
| --- | --- |
| `set A`, `set "A"`, `set A["Alpha"]`, `set A:20`, `set A["Alpha"]:20` | a set: a circle with the area of its size, 10 by default, and its label, or its id when it has no label |
| `union A, B`, with a label and a size as a set has them | the overlap of two sets or more, and its label. A union of n sets has the size 10/n² by default |
| `text "Apple"` or `text T1["Apple"]`, on an indented line under a set or a union | a text in the area of that set or union |
| `text A,B T1["Apple"]` | a text in the area of the union of A and B |
| `style A fill:#ff6b6b, stroke:teal` | the colors of a set, of a union or of a text, by its id |
| `title Text` | the title |

zuzu reads the statements as the lexer of Mermaid 12.3 reads them:

1. The keywords are in any case, as `SET` or `Title`. Statements can
   follow each other on one line, and on the line of the keyword.
2. An id is a word of letters, digits, `-` and `_` that starts with a
   letter or `_`, or a text in double quotes. A text and the id of a
   text can be a number too.
3. A text on an indented line belongs to the last set or union, when
   only texts come between them. Each other statement ends this mode.
4. The title runs up to a `#`, a `;` or the end of the line.
5. A label in brackets loses its spaces at each end, and a text in double
   quotes loses its quotes.
6. The value of a style is a text in double quotes, or the words,
   numbers and colors up to the next comma, joined by spaces. So Mermaid
   reads `2px` as `2 px`.

These are errors, as in Mermaid: a diagram with no set, a union of one
set, a union of a set that no `set` before it names, a negative size, a
size that is not a number, a text with no id, a text with no sets before
its id after a title or a style, a style with no value, a `;` or a `#` in a
statement, and `accTitle` and `accDescr`, which Mermaid does not read in a
venn diagram.

zuzu places the circles in the way that Mermaid 12.3 places them with
venn.js:

1. A circle has the area of the size of its set. A set of size 0 draws
   nothing, and the unions with it draw nothing either.
2. Two sets with a union stand at the distance that gives the size of the
   union. Two sets with no union stand apart. When a union of three sets
   or more has no union for two of its sets, that union of two takes a
   quarter of the size of the smaller set.
3. The circles go in place one at a time, from the set that overlaps the
   others most. Then the method of the simplex of Nelder and Mead moves
   them, so that each union comes as near its size as it can.
4. The largest circle comes first, the second largest at its right, and
   the third above them. A group of circles apart from the others goes at
   the right of the largest group, or under it.
5. The circles fill a box of `venn.width` by `venn.height`, 800 by 450 by
   default, less the title, with 15 pixels around them.
6. The label of a set or of a union goes at the point of its area that is
   farthest inside its circles and outside the other circles. Mermaid
   12.3 draws these labels on one line, and so does zuzu.
7. The texts of an area fill a grid in the largest circle around the
   point of its label, under the label, with as many columns as the
   square root of their count. A text wraps at the width of its cell.
   Mermaid places the texts in the same circles fitted with
   `venn.padding` around them, 8 by default, and so does zuzu. A word
   that is wider than its cell does not wrap, and Mermaid draws it over
   the text beside it. zuzu then stacks the texts of that area in one
   column, the first where the first row of the grid is, so that no two
   texts overlap.

With four sets or more, Mermaid also lays the circles out from 10 random
points, and keeps that layout when it fits better. zuzu starts from 10
points of a fixed sequence, so the same source gives the same image. Its
layout of four sets or more can differ from the layout of Mermaid.

zuzu draws a venn diagram at 440/`venn.width` of the size of Mermaid.
Mermaid draws a text at 20 pixels at a width of 800, and zuzu draws it at
11. The label of a set or a union is bold, at 13.2 pixels. zuzu leaves out
the empty space around the circles.

Each set takes the next color of the category scheme. Its circle is a tint
of its color with an outline of 1.5 in its color, and its label is in its
color. The tints add where the circles overlap, so each union shows
through. The label of a union and a text take the color `text`. A style
can change these colors:

| Style | On a set | On a union | On the id of a text |
| --- | --- | --- | --- |
| `fill` | the color of the circle, of its outline and of its label | an opaque fill of the overlap, as in Mermaid | nothing |
| `stroke` | the color of the outline | nothing | nothing |
| `fill-opacity` | the opacity of the tint | nothing | nothing |
| `stroke-width` | the width of the outline, in the pixels of Mermaid | nothing | nothing |
| `color` | the color of the label | the color of the label | the color of the text |

A color that is not a color of CSS is an issue. A `fill-opacity` that is
not a number draws the tint opaque, and a `stroke-width` that is not a
number draws an outline 1 pixel of Mermaid wide, as in Mermaid. So
`stroke-width:2px` is 1 pixel wide, and `stroke-width:"2px"` is 2. Mermaid
reads the other properties, such as `font-size`, and the style of an id
that names nothing, and draws nothing for them. zuzu accepts them with no
issue (D32).

zuzu reads `venn.width`, `venn.height` and `venn.padding`. A width or a
height that is not a positive number is an issue, and so is a negative
padding. `venn.useDebugLayout: true`
draws the grid of the texts, which zuzu does not draw, so it is an issue.
`venn.look` is a hint, as `look` is. Mermaid 12.3 draws a venn diagram in
the theme of `venn.theme`, `redux-color` by default. zuzu reads the
`theme` of the top level only, as "The theme" says. So `venn.theme` is an
issue, and a venn diagram with no `theme` draws in the scheme of the
caller, or `zuzu`. The colors `venn1` to `venn8` of `themeVariables` are
issues, as the other theme variables are.

zuzu draws a venn diagram in other ways than Mermaid in two places.
Mermaid draws the label of a set in a darker shade of its color, and zuzu
in its color. Mermaid draws the opaque fill of a union over the circles
before it in the source, and zuzu draws all the outlines over it.

### Other kinds

zuzu draws the flowchart, the swimlane diagram, the state diagram, the class diagram, the entity
relationship diagram, the requirement diagram, the agentflow diagram, the use case
diagram, and
the charts above, which hold the sequence diagram, the mindmap, the
architecture diagram, the git graph, the block diagram, the timeline,
the five kinds of C4 diagram, the ishikawa diagram, the Cynefin diagram,
the info diagram, the kanban, the packet, the four kinds of railroad
diagram, the tree view, the event model, the ZenUML diagram and the venn
diagram. The agentflow diagram reads into a graph, as
the flowchart does. Every other kind of Mermaid is an error that wraps
`mermaid.ErrKind`. D32 sets the order of the next kinds.

## Math of KaTeX

Mermaid 12.3 draws a text between two `$$` in a label, such as
`$$x^2$$`, as math with KaTeX. zuzu draws no math. It draws the label as
its source, with the `$$` and the LaTeX, and reports it. Each label with
math is one issue in the loose mode, and an error in the strict mode.
Labels with the same text give one issue, as the report names each text
once.

Mermaid draws math in these kinds, and zuzu reports it in them:

| Kind | Labels | When |
| --- | --- | --- |
| flowchart, swimlane, state, class, entity relationship, requirement, agentflow and use case diagram | the label of a node, its fields, such as an attribute or a member, the label of an edge, and the title of a cluster | in a label of HTML: `htmlLabels`, or `flowchart.htmlLabels` when it is not set, is true, the default |
| mindmap and block diagram | the label of a node or a block, and the label of an edge of a block diagram | in a label of HTML, as above |
| sequence diagram | a participant, a message, a note, and a block and its sections, such as `loop` and `else`. The title of a `box` draws as text in Mermaid | always |

With `htmlLabels: false`, Mermaid draws the source of the math as text in a
graph, a mindmap and a block diagram. zuzu draws the same, so that is no
issue. The other kinds, such as a gantt, a timeline, a pie or a C4
diagram, draw the source in Mermaid too, and that is no issue either.

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
is a line of `edgeWidth`, 1.5, in the color `edge` of `docs/STYLE.md`, with
an arrowhead of `arrowLength`, 8, in the same color (D32). The messages of
a sequence diagram, the links of an architecture diagram and their
arrowheads take the same tokens. A guide line, such as a lifeline or a
divider of a sequence diagram, or a ring of a radar, takes the lighter
color `axis`. A label
is 11 pixels. A cluster is a panel with its title at the top left. A
cluster of the shape `package`, such as a system boundary of the type
`package` of a use case diagram, is a tab that holds its title over a
panel, as a package of UML (D32). The layout gives each cluster row the
size of its title, `titleWidth` and `titleHeight`, for the tab. A color
that the source gives replaces the color of the theme. A node with an
opaque fill from the source and no color of text takes the text color of
the light theme or of the dark theme, whichever reads on the fill. The styles are in
`theme/config.go`, from `zuzu.node` to `zuzu.band`.
