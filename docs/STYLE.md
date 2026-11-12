# Style

This document holds the rules of the look of zuzu (D12). The goal is the
quality of nivo and visx: clear type, a quiet frame, soft grids, rounded
marks, labels on the data where they fit, and color that carries meaning and
nothing else.

The targets in [`targets/`](targets/README.md) follow these rules, and
[`targets/gen-targets.mjs`](targets/gen-targets.mjs) holds the same tokens as
code. If you change a token, change it here first, then in the script, then
draw the targets again. The targets and this document can change before the
code exists, and a change after that is a decision.

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
| `edge` | `#848e9f` | `#707a8d` | an edge of a diagram, its arrowheads, a message of a sequence diagram and a link of an architecture diagram |
| `band` | `#f1f2f3` | `#1d2128` | a band behind a group of rows, such as a section of a gantt |

The dark mode is not the light mode inverted. Its background has a little
blue in it, its text is not pure white, and its data colors are lighter, so
that each color keeps its contrast against the background.

A reader follows an edge of a diagram, a line of 1.25, at the size of a
terminal. So `edge` is darker than `axis` in the light mode and lighter in
the dark mode (D32). It has a contrast of at least 3:1 with `bg`, the least
contrast that WCAG 2.1 asks for a graphic that carries meaning. The label of
an edge is text in `muted`, so `edge` stays lighter than `muted` in the
light mode and darker in the dark mode, and the label reads apart from its
line.

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
| edge of a graph | a line of 1.25 in `edge`, with round caps and joins, and an arrowhead in `edge` |
| section of a gantt | a band in `band` behind the rows of every other section, across the chart, with the name of the section at its left |

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
