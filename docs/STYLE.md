# Style

This document holds the rules of the look of zuzu (D12). The goal is the
quality of nivo and visx: clear type, a quiet frame, soft grids, rounded
marks, labels on the data where they fit, and color that carries meaning and
nothing else.

The targets in [`targets/`](targets/README.md) follow these rules, and
[`targets/gen-targets.mjs`](targets/gen-targets.mjs) holds the same tokens as
code, and so does the package `theme`. If you change a token, change it here
first, then in the script and in the package `theme`, then draw the targets
again. The code exists, so a change to this document is a decision.

## Themes

A theme is a mode and a color scheme (D14). The mode sets the neutral colors.
The scheme sets the colors of the data. A caller can swap either one, and any
scheme works in either mode. Each theme is a Vega config value (D14).

### Modes

| Token | Light | Dark | Use |
| --- | --- | --- | --- |
| `bg` | `#ffffff` | `#10141b` | the background of the chart |
| `panel` | `#f5f6f8` | `#171c25` | a panel inside the chart, such as a tooltip or a small multiple |
| `text` | `#1b2230` | `#e7ebf2` | the title, the labels on the data, a value |
| `muted` | `#5f6b7d` | `#9ba5b7` | the subtitle, the labels of an axis, a legend |
| `faint` | `#98a2b3` | `#6a7487` | the source line, the title of an axis, a guide line |
| `grid` | `#eceff3` | `#1e242e` | the grid lines |
| `axis` | `#cfd5de` | `#333c4a` | a base line, a border, a guide line of a diagram, such as a lifeline |
| `edge` | `#414a5b` | `#bcc4d2` | an edge of a diagram, its arrowheads, a message of a sequence diagram and a link of an architecture diagram |
| `band` | `#f1f2f3` | `#1d2128` | a band behind a group of rows, such as a section of a gantt |

The dark mode is not the light mode inverted. Its background has a little
blue in it, its text is not pure white, and its data colors are lighter, so
that each color keeps its contrast against the background.

A reader follows an edge of a diagram at the size of a terminal, across
the nodes of a large diagram. So `edge` is darker than `muted` in the light
mode and lighter in the dark mode, about halfway between `muted` and `text`,
as the line of a theme of Mermaid and of Graphviz is dark (D32). The label
of an edge is text in `muted`, and so it reads apart from its line:

| Mode | `edge` with `bg` | `edge` with `muted` | `edge` with `text` |
| --- | --- | --- | --- |
| light | 8.9:1 | 1.65:1 | 1.79:1 |
| dark | 10.5:1 | 1.41:1 | 1.47:1 |

These are contrast ratios of WCAG 2.1. A graphic that carries meaning needs
at least 3:1 with `bg`.

`band` is `text` at an opacity of 0.06 over `bg` (D32). zuzu draws a band
in `text` at that opacity, and not in an opaque color, so that the grid
lines and the labels under it still show.

### Color schemes

A scheme has ten categorical colors for the light mode, and names one of
them for a rise and one for a fall. A scheme can give its own colors for the
dark mode. Without them, the dark mode lifts each light color. A palette with
fewer than ten colors gets the rest mixed toward the text color. The
sequential ramp runs from the background through the first color, so it
reads in both modes.

| Scheme | Character |
| --- | --- |
| `zuzu` | the default. Blue violet, teal, saffron, raspberry and sky first, with distinct hues |
| `pastel` | soft and light, in the manner of the default scheme of nivo |
| `harbor` | cool blues and teals with two warm accents |
| `ember` | warm oranges and reds with a violet and a green for contrast |

The colors of `zuzu`, in the light mode and then in the dark mode:

| # | Light | Dark |
| --- | --- | --- |
| 1 | `#5b6ee1` | `#7d8cf0` |
| 2 | `#1fa187` | `#3cc4a6` |
| 3 | `#f0a830` | `#f6be55` |
| 4 | `#e2577a` | `#f07a97` |
| 5 | `#3aa2d6` | `#5cbdea` |
| 6 | `#9a6fd1` | `#b38ef0` |
| 7 | `#7fb04a` | `#9ccb66` |
| 8 | `#e2733b` | `#f08f5c` |
| 9 | `#c2569f` | `#df7cc0` |
| 10 | `#5d8f9b` | `#7fb2bd` |

The maintainers accepted these four on 2026-11-10 (D14).

### Schemes from upstream themes

Many more schemes come from the upstream themes that the package `styles` of
transit draws from (D15). Such a scheme brings the neutrals of its theme for
each mode, and its categorical colors are the accents of the theme. The
targets show two prototypes:

| Scheme | Light mode | Dark mode | Source |
| --- | --- | --- | --- |
| `catppuccin` | Latte | Mocha | `github.com/catppuccin/palette` at `07d02aa`, MIT |
| `solarized` | light | dark | `github.com/altercation/solarized` at `62f656a`, MIT |

[`targets/licenses/`](targets/licenses/) holds the license of each source.

### Schemes from the themes of Mermaid

The `theme` of a Mermaid diagram selects a look of zuzu (D32). A theme with
`dark` in its name, such as `dark`, `neo-dark`, `redux-dark` or
`redux-dark-color`, selects the dark mode. Each other theme selects the
scheme with its colors, named `mermaid-` and the name of the theme. The
colors come from the theme variables of Mermaid 12.3.0, as mmdc resolves
them, under the MIT license in
[`targets/licenses/mermaid.txt`](targets/licenses/mermaid.txt).

1. The categorical colors take the hues of `cScale0` to `cScale11` of the
   theme, the colors that Mermaid gives to the sections of a diagram. A
   gray, and a hue within 20 degrees of a hue before it, are left out. The
   first hue is that of `cScale0`. Each next hue is the first in the order
   of `cScale` that differs from the hue before it by 90 degrees or more.
   If none does, it is the hue that differs most. `base` and `forest` keep
   nine hues, so their tenth color is mixed toward the text color.
2. Each of these colors has a lightness of 58 and a chroma of 50 in CIE LCh.
   So it has a contrast of about 3.3:1 with `bg`, as the colors of `zuzu`
   have. Mermaid draws these hues as pale fills, which do not read as a line
   or a small mark.
3. `neutral` and `redux` have only grays in `cScale`. So `mermaid-neutral`
   takes ten grays, as the pie of `neutral` has grays, and `mermaid-redux`
   takes ten grays in the hue of the ink of `redux`, `#28253d`, at a chroma
   of 8, as the border of its clusters, `#bdbccc`, has. Their lightness in
   CIE LCh is 50, 72, 56, 80, 63, 84, 53, 76, 60 and 67, so that neighbors
   differ and the text reads on each. A light gray lifted for the dark mode
   is almost white, so in the dark mode each gray is 10 darker, and the
   text of the dark mode reads on it:

   | Scheme | Colors of the dark mode, from 1 |
   | --- | --- |
   | `mermaid-neutral` | `#5e5e5e` `#969696` `#6d6d6d` `#ababab` `#7f7f7f` `#b6b6b6` `#666666` `#a0a0a0` `#777777` `#898989` |
   | `mermaid-redux` | `#5f5d6a` `#9794a3` `#6e6b79` `#aca9b8` `#807d8b` `#b7b4c3` `#676472` `#a29fae` `#787583` `#8a8796` |
4. `mermaid-redux-color` takes `cScale0` to `cScale9` of `redux-color` as
   they are. These colors are light, as those of `pastel` are.
5. A scheme brings two neutrals of its theme for the light mode: `text` is
   the `textColor` of the theme, and `edge` is its `lineColor`. `band`
   follows the text. The other neutrals, and the neutrals of the dark mode,
   are those of zuzu, because a theme of Mermaid has one variant (D15). The
   line of a theme of Mermaid is darker than `muted`.
6. A scheme changes colors only. The shapes, the sizes and the type stay
   those of zuzu.

| Scheme | Colors, from 1 | Rise, fall | `text` | `edge` |
| --- | --- | --- | --- | --- |
| `mermaid-default` | `#8683de` `#8e8f2e` `#bf6fbe` `#bb7d39` `#09a069` `#d4659d` `#6a983d` `#d86769` `#00a1a4` `#1a94e1` | 5, 8 | `#333333` | `#333333` |
| `mermaid-base` | `#ab8530` `#b075ca` `#829332` `#5f8ce3` `#609a43` `#da6386` `#00a176` `#d26e57` `#009fc1` | 5, 6 | `#333333` | `#0b0b0b` |
| `mermaid-neutral` | `#777777` `#b0b0b0` `#868686` `#c6c6c6` `#989898` `#d1d1d1` `#7f7f7f` `#bbbbbb` `#919191` `#a3a3a3` | 1, 2 | `#000000` | `#666666` |
| `mermaid-forest` | `#789536` `#009cd1` `#da647d` `#359e5b` `#7188e2` `#00a189` `#b672c6` `#9d8a2d` `#cc724c` | 4, 3 | `#000000` | `#000000` |
| `mermaid-neo` | `#8982dd` `#8d902e` `#bf6ebe` `#bb7d3a` `#129f68` `#d4659c` `#6a983d` `#d86867` `#00a1a4` `#2493e1` | 5, 8 | `#333333` | `#000000` |
| `mermaid-redux` | `#787583` `#b2afbe` `#878593` `#c8c5d4` `#9a97a6` `#d3d0df` `#807d8b` `#bdbac9` `#928f9e` `#a4a1b0` | 1, 2 | `#28253d` | `#000000` |
| `mermaid-redux-color` | `#f4a8ff` `#46ecd5` `#ffb86a` `#dab2ff` `#7bf1a8` `#c4b4ff` `#ffa2a2` `#ffdf20` `#a3b3ff` `#bbf451` | 5, 7 | `#28253d` | `#000000` |

## Type

zuzu bundles IBM Plex Sans (D16). The targets name it first, then Inter,
Helvetica Neue, Arial and the generic sans-serif.

| Role | Size | Weight | Color |
| --- | --- | --- | --- |
| title | 16 | 600 | `text` |
| subtitle | 12.5 | 400 | `muted` |
| label of an axis, a legend | 11 | 400 | `muted` |
| label on the data | 11 | 600 | the color of the series, or `text` |
| small label, title of an axis | 10 | 400 | `faint` or `muted` |
| source line | 10.5 | 400 | `faint` |
| large value, such as the center of a donut | 28 to 30 | 600 | `text` |

A label that sits on a busy mark gets a halo: a stroke of 3 in the
background color, drawn under the fill with `paint-order="stroke"`.

## The frame

1. The chart has a margin of 24 on each side.
2. The title is at the top left, at a base line of 34. The subtitle is at 54.
3. The legend, if there is one, is a row below the subtitle. Each item is a
   rounded square of 10, or a short line for a line series, then its label.
   The row wraps when it is full.
4. The source line is at the bottom left, 14 from the bottom.
5. There is no border and no frame around the plot area. The background is
   one color.

## Axes and grids

1. A quantitative axis has horizontal grid lines in `grid` and no axis line.
   Its labels are at the left, right aligned, 8 from the plot area.
2. The base line at zero, or at the bottom of a band axis, is one line in
   `axis`.
3. There are no tick marks. A label is enough.
4. An axis has five or six labels at most, with round values.
5. The title of an axis is short, in `faint`, and sits at the end of the axis
   rather than along it. A title that the chart title already says is left
   out.
6. Numbers use a thousands separator, and a unit is in the label, such as
   `$400` or `20°`.

## Marks

| Mark | Rule |
| --- | --- |
| line | 2 wide, round joins and caps, a monotone curve. The last point has a dot of radius 3, and the name of the series follows it in the color of the series |
| area | the line on top, and a gradient of the same color below, from 0.32 opacity to 0.02 |
| stacked area | full colors at 0.88 opacity, with a line of 1 in the background color between the layers |
| bar | the end away from zero has round corners of radius 3. A group of bars has a gap of 8 percent of a band between its bars |
| stacked bar | segments with a gap of 1.5. Only the last segment has round corners |
| point | radius 3 to 4, at 0.7 to 0.9 opacity, with a ring of 1 or 2 in the background color |
| bubble | the area is the value. The largest is drawn first, so that the small ones stay on top |
| arc | a gap between the slices, and round corners of 4. A donut has an inner radius of 0.64 of the outer one, and its total in the center |
| box | round corners of 5, a fill of the series color at 0.2 opacity, an outline and a median of 2.5. The raw points are under the box |
| hierarchy | a group is a tint of its color with its name at the top left. A leaf is the full color. Corners round by 3 to 6 |
| link of a sankey | the color of its source at 0.32 opacity |
| node of a graph | a rounded rectangle with a tint of its color and an outline of 1.25 |
| external element of a C4 diagram | a node in `muted`, with the tint and the outline of the other nodes, as Mermaid draws it gray (D32) |
| edge of a graph | a line of `edgeWidth` in `edge`, with round caps and joins, and an arrowhead in `edge`, as "Edges" says |
| guide line of a diagram | a line of 1.25 in `axis`, with round caps and joins, such as a lifeline of a sequence diagram or an axis of a radar |
| section of a gantt | a band in `band` behind the rows of every other section, across the chart, with the name of the section at its left |

### Edges

An edge of a diagram, a message of a sequence diagram and a link of an
architecture diagram take these sizes:

| Token | Value | Use |
| --- | --- | --- |
| `edgeWidth` | 1.5 | the width of the line of an edge, and of the outline of an open arrowhead |
| `arrowLength` | 8 | the length of the arrowhead `normal` of DOT. It is 0.8 of this length wide, and the other shapes take the same scale |

A filled arrowhead has an outline of 1 in `edge`. The tip of an arrowhead
ends on the border of the node at each size. An arrowhead has round joins,
so its outline reaches at most half its width past the tip, under the
outline of the node. The `arrowsize` of DOT multiplies the
length, and a source that sets the color or the width of an edge, such as
`color` of DOT or `linkStyle` of Mermaid, wins over these tokens.

## Annotations

1. Name a series on the chart where it fits, at the end of a line or inside a
   stream, rather than in a legend.
2. Mark the extreme or the notable value with a dot and a short label that
   gives the value and the date, as the area target does for its high.
3. A reference line, such as a mean, is dashed and named where it starts.
4. A tooltip is a panel with round corners of 8, in `panel` with a border in
   `axis`. It names the point, then one row for each series, sorted by value.

## Size

The targets are 640 wide, and from 220 to 560 high as the chart needs. zuzu
takes the size from the spec, and the rules above are in units of the
default size.
