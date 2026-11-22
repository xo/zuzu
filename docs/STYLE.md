# Style

This document holds the rules of the look of zuzu (D12). The goal is the
quality of nivo and visx: clear type, a quiet frame, soft grids, rounded
marks, labels on the data where they fit, and color that carries meaning and
nothing else. "The look like Excalidraw" holds the rules of the look drawn
by hand (D40).

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
| `muted` | `#5f6b7d` | `#9ba5b7` | the subtitle, the labels and the title of an axis, a legend |
| `faint` | `#98a2b3` | `#6a7487` | the source line, the title of a legend, a guide line |
| `grid` | `#eceff3` | `#1e242e` | the grid lines |
| `axis` | `#cfd5de` | `#333c4a` | the line and the ticks of an axis, a base line, a border, a guide line of a diagram, such as a lifeline |
| `edge` | `#414a5b` | `#bcc4d2` | an edge of a diagram, its arrowheads, a message of a sequence diagram and a link of an architecture diagram |
| `band` | `#f1f2f3` | `#1d2128` | a band behind a group of rows, such as a section of a gantt |

The dark mode is not the light mode inverted. Its background has a little
blue in it, its text is not pure white, and its data colors are lighter, so
that each color keeps its contrast against the background.

A graph of DOT can set its own background with `bgcolor` (D37). That color
replaces `bg`, and the mode follows its lightness: a dark `bgcolor` draws
in the dark mode, and a light one in the light mode. A background that the
caller gives wins over `bgcolor`.

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
sequential ramp starts at a tint of the first color, `rampStart` of the way
from the background to that color. It runs through a tint halfway from
there to the first color, then through the first color, so it reads in both
modes (D48). The lowest value of a scale is that tint, and not the
background, so that a point or a line of the lowest value shows. The
background is the one that the chart draws: the background of the spec when
it sets one, and the background of the mode otherwise (D35). On a dark
background the ramp runs on to a lighter color, and on a light background
to a darker color. The diverging ramp runs from the color of a fall through
`grid` to the color of a rise.

| Token | Value | Use |
| --- | --- | --- |
| `rampStart` | 0.4 | how far the sequential ramp starts from the background toward its first color |

These ramps and the categorical colors serve a scale that names no
scheme, and a scale whose range is `category`, `ordinal`, `ramp`,
`heatmap` or `diverging`. A ramp is a token of the theme, and the rules
above apply only to it. "Schemes that a spec names" holds the rest.

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

### Schemes that a spec names

A Vega spec can name a scheme of Vega, such as `viridis`, `blues`,
`tableau10` or `redblue`, or give a list of colors as its scheme. Such a
scale draws in the colors of that scheme, as vega.js draws them (D46). Its
`count`, its `extent`, `reverse` and the type of the scale apply as in
vega.js. A sequential or a diverging scale interpolates the scheme, and a
discrete scale takes some of its colors. A scheme in the `range` of the
config of the spec is also a named scheme. The schemes are those of Vega
6.4.0, from vega-scale 8.1.3 and d3-scale-chromatic 3.1.1.
`scale/LICENSE-schemes.txt` holds the licenses of their colors. A name that
Vega does not have is an issue, and the scale takes the colors of the
theme.

A named scheme draws in the same colors in both modes, as it does in
vega.js. zuzu does not lift its colors for the dark mode, and does not
start it from the background. The spec chose the scheme, and its colors
carry the meaning of the data. A reader compares them with the legend, and
with the same scheme in another tool. A scheme such as `viridis` also has
even steps of lightness, which a lift breaks.

So one end of a named scheme can be near the background, in either mode.
The yellow end of `viridis`, `#fde725`, has a contrast of 1.26:1 with `bg`
of the light mode, and its purple end, `#440154`, 1.21:1 with `bg` of the
dark mode. The light end of `blues`, `#cfe1f2`, has 1.34:1 in the light
mode, and its dark end, `#0a4a90`, 2.1:1 in the dark mode. The chart still
reads in both modes, because the order of the lightness of the marks
carries the value, and the legend shows the scheme. Vega has schemes for
each background. `darkblue`, `darkgold`, `darkgreen`, `darkmulti` and
`darkred` run from a dark gray to a light color, for a dark background. A
spec that draws on a dark background can name one of them, or set the
`extent` of its scheme.

### Colors that a spec names

A color that a Vega or a Vega-Lite spec names draws as the spec names it in
the light mode. In the dark mode, a gray near black or near white flips its
lightness, so that it reads on the dark background (D48). Black becomes
white, `#333` becomes `#ccc`, and `#eee` becomes `#111`. A spec that draws
a separator in white, or a label in black, means the background and the
ink of a light chart, and the flip keeps that meaning on a dark chart. A
color with a hue, such as `red` or `steelblue`, stays, because its hue
carries the meaning.

A color is a gray near black or near white when its channels of red, green
and blue differ by at most `grayChroma` of the full range, and its
lightness in HSL is at most `grayDark` or at least `grayLight`. The flip
changes the lightness `l` to `1 - l`, and keeps the hue, the saturation and
the opacity. A gray between the two bounds, such as `#888`, stays.

| Token | Value | Use |
| --- | --- | --- |
| `grayChroma` | 0.1 | the largest difference of the channels of a gray, from 0 to 1 |
| `grayDark` | 0.3 | the largest lightness of a gray near black |
| `grayLight` | 0.7 | the smallest lightness of a gray near white |

These rules say which colors flip:

1. A color flips where the spec writes it: a value of an encoding, the
   value of a condition, a value of the range of a scale, a property of a
   guide, the config, and a stop of a gradient. A color in the data, which
   an encoding reads with no scale, flips too.
2. A color that an expression computes, in a signal, stays. The expression
   can read a scale whose range already flipped, or work out a contrast of
   its own, and a second flip undoes the first.
3. A spec that chooses the color of a text by its value, such as white on
   the high cells of a heat map and black on the low ones, writes the two
   colors as values of a condition. They flip, and so does the ramp of the
   theme: in the dark mode, the high cells are light and the low ones are
   dark. So the text keeps its contrast in both modes. A scheme that the
   spec names does not flip (D46), and the text over such a scheme can
   lose its contrast in the dark mode.
4. A scheme that the spec names, as a name or as a list of colors, stays,
   as "Schemes that a spec names" says.
5. A chart whose background is light, such as one whose spec sets a white
   background, draws each color as named in both modes. A diagram of DOT
   or Mermaid keeps its colors, because its theme sets its mode (D32, D37).

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
| title of an axis | 10 | 400 | `muted` |
| small label | 10 | 400 | `faint` or `muted` |
| source line | 10.5 | 400 | `faint` |
| large value, such as the center of a donut | 28 to 30 | 600 | `text` |

A label that sits on a busy mark gets a halo: a stroke of 3 in the
background color, drawn under the fill with `paint-order="stroke"`.

A formula of math in a label of Mermaid draws in STIX Two Math, at the size
and in the color of the text of the label (D36). A thin space, 3/18 of the
size of the text, as `\,` of TeX, stands between a formula and the text
next to it, so that the two do not touch.

## The frame

1. The chart has a margin of 24 on each side.
2. The title is at the top left, at a base line of 34. The subtitle is at 54.
3. The legend, if there is one, is a row below the subtitle. Each item is a
   rounded square of 10, or a short line for a line series, then its label.
   The row wraps at the width of the plot (D35). "Legends" gives the rest.
4. The source line is at the bottom left, 14 from the bottom.
5. There is no border and no frame around the plot area. The background is
   one color.

## Axes and grids

1. An axis draws its line and its ticks where Vega draws them (D46). By
   default it has both. The `domain`, `ticks` and `tickSize` of the axis
   or of its config can hide them or change them, and so can the other
   properties of Vega that style them, such as `tickColor` or
   `domainWidth`. The line and the ticks are in `axis`, with the sizes of
   the table below.
2. A tick points away from the plot area. A label is `labelPadding` past
   the end of its tick, or past the line of the axis when the axis has no
   ticks. A label of a vertical axis at the left is right aligned.
3. A quantitative axis can have grid lines across the plot area in `grid`.
   The base line at zero, when zero is inside the domain, is one line in
   `axis`.
4. An axis has five or six labels at most, with round values. The step
   lands on both ends of a nice domain, so that the top and the bottom of
   the scale each get a tick, a label and a grid line (D46). A domain of
   time that starts and ends on the start of a year takes a step of whole
   years. A log scale keeps its ticks at the powers of its base, where a
   nice domain of a log scale ends. A `tickCount` or `values` of the spec
   wins over this rule.
5. The grid lines and the ticks of an axis stand where Vega draws them,
   at the ticks of its scale for the `tickCount` of the axis, or for 10,
   and only the ticks of rule 4 have labels (D51). So an axis from 0 to
   45,000 draws a line each 5,000 and a label each 15,000. When a label of
   rule 4 is not a tick of Vega, or two lines of Vega stand less than 10
   apart, the lines and the ticks stand at the labels alone.
6. The title of an axis is short, in `muted`, and sits at the end of the
   axis rather than along it. It moves clear of the labels of each axis of
   its view, on an axis at the top or at the right too, and in the corner
   where two axes meet (D46). A title that the chart title already says is left out.
   In a grid with the bounds `flush`, which lays out each cell by its plot
   alone, a cell still keeps room above its plot for such a title, so that
   it does not cover the cell above, as in `concat_marginal_histograms`.
7. Numbers use a thousands separator, and a unit is in the label, such as
   `$400` or `20°`.

| Token | Value | Use |
| --- | --- | --- |
| `tickSize` | 5 | the length of a tick, from the line of the axis |
| `tickWidth` | 1 | the width of a tick |
| `domainWidth` | 1 | the width of the line of an axis |
| `labelPadding` | 3 | the space between the end of a tick and its label |

So a label is 8 from the line of its axis by default, as before D46.
`axis` is lighter than `muted`, so the line and the ticks frame the plot
area and stay behind the data.

## Legends

1. An entry of a legend is a symbol, then its label in `muted`, 6 to the
   right of the symbol. Two entries of a row stand 16 apart. A row is at
   least 18 high.
2. A symbol takes the shape of the marks that the legend names: a rounded
   square for a mark with an area, such as a bar, a box or an area, the
   shape of a point, such as a circle, and a short line for a line. A
   value that a line draws, such as through `scale('color', 'Normal')`,
   takes a short line too. A legend that names marks of more than one
   kind takes the shape of a mark with an area first, then of a point. A
   mark with no fill gives an outline with no fill, and a shape of a map
   with no fill, such as a line of a map, gives a short line. A legend
   with no scale of color takes the color that the mark gives as a value,
   or else the first categorical color.
3. The symbol of a size is a symbol of that size, in the color of rule 2,
   or in the color of its entry when the legend also shows colors. A
   symbol wider than 10 makes its entry wider, and the largest symbol
   sets the height of each row, so that no two symbols overlap and each
   label stays 6 from its symbol. A legend of opacity draws each symbol at
   its opacity, and a legend of widths draws a short line of each width.
4. A legend of sizes alone draws as nested circles that share their lowest
   point, the largest behind. A line runs from the top of each circle to
   its label. The legend draws its sizes so only when each value keeps its
   circle and its label. The largest circle is at least three labels high,
   and the tops of two circles stand at least 13 apart. The spec also sets
   none of `clipHeight`, `columns`, `direction`, `symbolLimit`,
   `symbolSize`, `rowPadding` and `columnPadding`, and no `symbolType`
   other than `circle`. Otherwise the legend draws its sizes as a row of
   entries.
5. A first value whose size is 0 has no entry, as in Vega.
6. A legend of a scale that cuts numbers into pieces, such as `quantile`,
   `quantize` or `threshold`, names each piece by its ends, such as
   `< 30`, `30 – 70` and `≥ 70`. A legend of bins names each bin by its
   start and its end, such as `5 – 10`. A gradient of such a scale draws
   as a bar of even steps, one for each color, with the label of each
   threshold where two steps meet.
7. A legend shows at most 30 entries, and a label is at most 160 wide, as
   in Vega. An ellipsis ends a label that is cut. When a legend has more
   values, its last entry counts the rest, such as `…5 entries`.

A property of a legend that the spec or the config of the legend sets wins
over the look (D46). zuzu reads these properties:

| Property | What it does |
| --- | --- |
| `clipHeight` | cuts each symbol to this height about its center. Each row is this high, so a large circle draws as a band |
| `symbolLimit` | the most entries, in place of 30 |
| `labelLimit` | the widest label, in place of 160 |
| `symbolType`, `symbolSize` | the shape and the area of a symbol that no scale sets |
| `symbolFillColor`, `symbolStrokeColor`, `symbolStrokeWidth`, `symbolOpacity`, `symbolDash` | the paint of each symbol |
| `rowPadding`, `columnPadding` | the space between two rows and between two columns, in place of the space of rule 1 |
| `offset`, `legendX`, `legendY` | the place of the legend, as in Vega |
| `gradientLength`, `gradientThickness` | the length and the width of the bar of a gradient, in place of 120 and 10 |
| `columns`, `direction` | the grid of the entries |

## Marks

| Mark | Rule |
| --- | --- |
| line | 2 wide, round joins and caps, a monotone curve. The last point has a dot of radius 3, and the name of the series follows it in the color of the series, where the names fit, as "Annotations" says. A line whose opacity, from the spec or the look, is below 0.5 is context, such as a faint line of each year behind a mean. It has no name at its end, and does not count as a series (D48). A closed line, such as the line of a radar, has no name at its end, and the legend names it (D35). A dashed line whose gaps are no longer than its width has flat caps, because a round cap closes such a gap |
| area | an area from zero has the line on top, and a gradient of the same color below, from 0.32 opacity to 0.02. A gradient that the spec gives stays. If the spec sets an opacity of the area, the area is a flat fill of its color at that opacity, with no gradient and no line on top (D48) |
| ranged area | an area with an end of its own, `y2` or `x2`, such as an error band, or an area whose scale leaves out zero, such as a ridgeline, does not run from zero. It is a flat tint of its color at 0.32 opacity, with no line on top and no gradient, so that it reads as a range (D48). An opacity that the spec sets wins over the tint |
| stacked area | full colors at 0.88 opacity, with a line of 1 in the background color between the layers |
| bar | the end away from zero has round corners of radius 3. A bar from one value to another, such as the part of a bar above a line, rounds the end further from zero, and a radius that the spec gives, such as `cornerRadiusEnd: 0`, wins. A group of bars has a gap of 8 percent of a band between its bars |
| stacked bar | segments with a gap of 1.5. Only the last segment has round corners |
| point | radius 3 to 4, at 0.7 to 0.9 opacity, with a ring of 1 or 2 in the background color. A point smaller than 20 square pixels, or with a shape that is a path of SVG, such as a star, has no ring, because the ring covers a small or thin shape |
| bubble | the area is the value. The largest is drawn first, so that the small ones stay on top |
| arc | a gap between the slices, and round corners of 4. A donut has an inner radius of 0.64 of the outer one, and its total in the center |
| box | round corners of 5, a fill of the series color at 0.2 opacity, an outline of 1.5, and a median of 2.5 across the box in the series color. The whiskers and their ticks are 1.5 wide in the series color. The raw points are under the box. A part whose properties the spec gives, such as `"median": {"color": "red"}`, keeps them, and the look sets only the others |
| error bar | the rule and its ticks in the series color, as the whiskers of a box, rather than the black of Vega-Lite, which the dark mode hides. A color that the spec gives wins |
| hierarchy | a group is a tint of its color with its name at the top left. A leaf is the full color. Corners round by 3 to 6 |
| link of a sankey | the color of its source at 0.32 opacity |
| node of a graph | a rounded rectangle with a solid tint of its color, its color mixed into the background at `NodeTint`, and an outline of 1.25 in its color. A panel with a color, such as a flow or a composite state, and a note take a solid tint too, so a node over a panel keeps its tint (D55) |
| solid node of a graph | a node that marks a point of the flow, such as the start and the end of a state diagram, a fork or a join bar, and a junction point of a flowchart, fills with the color of its outline, not with a tint. A start of a state diagram takes the color of the state that it points to, and an end the color of the state that ends in it, when there is one such state. With more than one, it keeps the color of a node. The look like Excalidraw fills it solid, not with a hachure (D55) |
| box by kind in a chart of Mermaid | a box that takes a color by its kind, such as an element of a C4 diagram, a box of an event model, a node of a mindmap, a box of a railroad, a participant of a sequence, the label of a branch of a gitgraph, an item of a Cynefin framework and a column of a kanban, draws as a node of a graph: a solid tint and an outline in its color (D55) |
| named node of a diagram of Mermaid | a class, an entity, a requirement and an element in `redux-color` or `redux-dark-color`, a participant of a sequence, a branch of a gitgraph, and an actor and a use case with `colorScheme: rotate`, take the categorical color at the FNV-1a hash of their id. A node whose color an earlier node took takes the next free color, so up to ten nodes take ten colors. So a name keeps its color across diagrams (D56) |
| subgraph and lane of a flowchart | in `redux-color` or `redux-dark-color`, each subgraph and each lane of a swimlane takes the color of the hash of its id among the second to the tenth colors, as a panel of its color, so a panel never takes the color of the nodes in it. A namespace of a class diagram, a subgraph of an ER diagram and a system boundary with `colorScheme: rotate` do the same (D55, D56) |
| use case diagram with `colorScheme: rotate` | each actor and use case takes the color of the hash of its id, and draws as a node of a graph of that color (D37, D56) |
| agentflow diagram in `redux-color` or `redux-dark-color` | each node takes a categorical color by its kind, in the order of the palette of Mermaid: a tool the first, a task the second, a decision the third, a reference document the fifth, a connector the sixth, an action the seventh and an input the fourth. Each flow takes the colors in turn from the eighth, as a panel of its color (D53) |
| composite state of a state diagram | a panel with a band across its top that holds its title, in a stronger tint of its color, with a line under the band. In `redux-color` or `redux-dark-color`, each composite state takes the color of the hash of its id among the second to the tenth colors (D54, D55, D56) |
| note of a diagram of Mermaid | the color `note`: the categorical color of the scheme whose hue is nearest 52 degrees, the yellow of a note on paper, as a solid tint with an outline (D54, D55) |
| a status, such as a critical task of a gantt or the priority of a kanban card | a critical task and the priority Very High take the color of a fall of the scheme, and High a tint of it, the same color at 0.32 opacity. Low takes the first categorical color, and Very Low a tint of it (D38) |
| marker of today of a gantt | a line of `edgeWidth` across the chart in the color of a fall of the scheme, as Mermaid draws it red (D38) |
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
so its outline reaches at most half its width past the tip. An arrowhead
draws over the shape of its node, so its tip sits on top of the outline of
the node. The line of an edge draws under the nodes, so a line that passes
under a node stays hidden. The label of an edge and its box draw over the
line. The head of a message of a sequence diagram draws over the box of a
participant in the same way. The `arrowsize` of DOT multiplies the
length, and a source that sets the color or the width of an edge, such as
`color` of DOT or `linkStyle` of Mermaid, wins over these tokens.

## Annotations

1. Name a series on the chart where it fits, at the end of a line or inside a
   stream, rather than in a legend.
2. Mark the extreme or the notable value with a dot and a short label that
   gives the value and the date, as the area target does for its high.
3. An annotation of the look draws only where it fits (D48). The look
   counts the series of each view by the lines that it draws there, not by
   the domain of the color scale, and leaves out a line of context, as
   "Marks" says. A view is a plot with its own axes, such as a panel of a
   repeat or a facet. Two lines of one color count once when one name
   names both, and then the name follows the line that ends furthest to
   the right. In each view, the names move apart so that no two overlap,
   by at most 12, the height of a name. A view names no line when it has
   more than eight series, when two lines of one color have different
   names, when the spec draws a text of its own, or when a name has to
   move further than 12. Then the legend of the color scale names the
   series. The label of the high draws in one area only, and not in an
   area of a facet, in a ranged area, or when the spec draws a text of its
   own. A value between -1 and 1 keeps two significant digits, so that a value such as 0.001 does not read as 0.
4. A property of a mark that the spec sets wins over the look, such as an
   opacity, a gradient, a corner radius, a stroke dash and the paint of
   the view (D46, D48).
5. A reference line, such as a mean, is dashed and named where it starts.
   A rule whose spec gives its own color, width or dash draws as the spec
   asks, with no dash and no name, because the spec already says how it
   looks.
6. A tooltip is a panel with round corners of 8, in `panel` with a border in
   `axis`. It names the point, then one row for each series, sorted by value.

## The look like Excalidraw

A look changes the shapes and the font of a chart, and keeps the colors of
its mode and its scheme (D39, D40). The standard look is the look of the
rest of this document. The look like Excalidraw draws each mark as if by
hand, as Excalidraw and the look `handDrawn` of Mermaid 12.3 draw it. zuzu
draws it with its copy of rough.js 4.6.6 in `internal/rough`, with the
options that Mermaid gives for `look: handDrawn`. The package `theme`
holds these tokens:

| Token | Value | Use |
| --- | --- | --- |
| `SketchRoughness` | 0.7 | how far a line strays from its path |
| `SketchEdgeRoughness` | 0.3 | how far the line and the arrowheads of an edge of a graph stray, as Mermaid draws an edge |
| `SketchStroke` | 1.3 | the width of a stroke that the item does not set |
| `SketchFillWeight` | 1.5 | the width of each line of a hachure |
| `SketchClusterFillWeight` | 3 | the width of each line of the hachure of a cluster of a graph, as Mermaid draws a cluster. Its lines touch, so the box looks solid |
| `SketchHachureGap` | 1.5 | the space between the lines of a hachure. rough.js rounds it to 2 |
| `SketchHachureAngle` | -41 | the angle of the lines of a hachure, in degrees |
| `SketchGradientGap` | 5 | the space between the lines of the hachure of a gradient, the gap of Excalidraw |
| `SketchGradientTint` | 0.25 | the opacity of the tint under the hachure of a gradient, times the opacity of the fill |
| `SketchGradientInset` | 2 | how far the hachure of a gradient stays inside the outline |
| `SketchGradientLeast` | 20 | the least width and height of a shape whose gradient draws over a tint |
| `SketchSeed` | 1 | the seed of a chart that sets none |
| `SketchSlack` | 10 | the most that the canvas grows at a side for a line that wobbles out of it |

A hachure is a fill of parallel lines in the paint of the fill, a color or
a gradient. These are the rules of the look:

1. Every mark changes: the marks of the data, the axes, the grid lines, the
   legends, the frames and the background of a group. A text, an image and
   a formula of math keep their shape.
2. An item draws as three paths: its underlay of rule 4, its hachure, with
   lines of `SketchFillWeight` in the paint and the opacity of its fill,
   and then its outline, in the paint, the width, the dash, the cap, the
   join and the opacity of its stroke. Each path holds all the lines of its
   part, so a color with an alpha does not grow darker where two lines
   cross. The item keeps its opacity, its link, its tooltip and its place
   among the other items.
3. A fill draws as a hachure, never as a pattern of SVG. A fill in the
   color of the background, such as the box under the label of an edge,
   draws solid, with an outline that wobbles, so that it still hides what
   is under it. A rect thinner than 3 draws its fill as one line along its
   length. A hachure fills a shape as the rule nonzero of SVG fills it: a
   ring inside a ring that runs the same way, such as the inner circle of
   a double circle, fills, and a ring that runs the other way is a hole.
4. A hachure draws over an underlay: a solid fill in the color of the
   background, in the shape of the item, with the same wobble. So a mark
   hides what is under it, as in the standard look, and a mark under it
   does not show through the gaps of the hachure, such as the bar under
   the red part of a bar of `layer_bar_annotations.vl.json`. The underlay
   takes the opacity of the fill, the alpha of its color times its fill
   opacity, so a fill that lets a mark show through in the standard look
   lets it show through as much. Over the background, the underlay does
   not show. A canvas with no opaque background draws no underlay. The
   underlay of a path of several rings, such as a band of a contour with
   islands, fills the path as it is, with no wobble, because rough.js
   wobbles apart the bridges between the rings (D49 part 6).
5. A gradient keeps its colors. The lines of a hachure take the gradient
   of the fill, and an outline takes the gradient of the stroke. Each
   renderer spans a gradient over the bounds of the path that it paints.
   The lines of a hachure reach from edge to edge of the shape, so each
   color falls where the standard look draws it, such as along the bar of
   a legend of a continuous color scale. The report names no gradient. The
   gradient that the standard look gives an area is the one exception. It
   draws as the color of the area at the opacity of its top, 0.32, because
   Excalidraw fills a shape with one tint. A shape at least
   `SketchGradientLeast` across, such as a node of a graph, draws its
   gradient over a tint (D50): the same gradient, solid, at
   `SketchGradientTint` of the opacity of the fill, over the underlay of
   rule 4. Its hachure takes the gap `SketchGradientGap` and the weight
   `SketchFillWeight`, even in a cluster, so its lines read as strokes of a
   pencil, and fills the shape inset by `SketchGradientInset`, so that no
   end of a line shows past the outline. A label of a graph on such a
   shape, in the color of text that a graph chooses for its fill, takes
   the color of text that reads on the tint and the halo of a label on a
   busy mark. A color of text that the spec gives stays.
6. A circle draws as an ellipse of rough.js, and a rect with no round
   corners as a rectangle of rough.js. Each other shape draws as its path.
   A line of rough.js is drawn twice, a little apart, as rough.js draws
   it.
7. An edge of a graph of DOT or Mermaid, its line and its arrowheads,
   draws with `SketchEdgeRoughness`, and the box of a cluster with
   `SketchClusterFillWeight`, as Mermaid 12.3 draws them for
   `look: handDrawn`. Every other mark, in a chart too, draws with
   `SketchRoughness` and `SketchFillWeight`.
8. A small shape wobbles less, as Excalidraw draws a small element, so
   that a dot, a tick or a small swatch of a legend keeps its shape. A
   shape keeps the roughness of rule 7 when it is at least 20 by 50, a
   circle when it is at least 15 by 15, and a line when it is at least 150
   long. A path with no fill counts as a line. Each other shape takes half
   of that roughness, or a third when it is shorter than 10. An edge of a
   graph keeps `SketchEdgeRoughness`. A shape shorter than 10 also fills
   solid, because a hachure has room for only one to four short lines in
   it, which draw a dot as a dash.
9. A text draws in Excalifont, at the size, the weight and the color of the
   standard look, with IBM Plex Sans for each character that Excalifont
   does not have. The layout measures each text in Excalifont, so each
   label fits its box. Without Excalifont, the text draws in IBM Plex Sans,
   and the report warns of it.
10. Each item takes its seed from a hash of the seed of the chart, the names
    of its mark and of the marks above it, and its index. So the same
    source draws the same lines, and a mark keeps its lines when another
    mark comes before it.
11. A line that wobbles out of the canvas makes the canvas larger, so that
    the image holds it. The canvas grows at a side by at most
    `SketchSlack`, the most that a line wobbles. A mark that reaches
    further past a side, such as the land of a map whose canvas the spec
    fixes with `autosize: "none"`, stays cut at that side, as in the
    standard look.

## Size

The targets are 640 wide, and from 220 to 560 high as the chart needs. zuzu
takes the size from the spec, and the rules above are in units of the
default size.
