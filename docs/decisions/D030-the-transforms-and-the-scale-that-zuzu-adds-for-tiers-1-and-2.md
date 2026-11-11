# D30. The transforms and the scale that zuzu adds for tiers 1 and 2

Status: Decided.

On 2026-11-10 an agent proposed these names and parameters, in answer to
open question 15 of the plan, and the maintainers accepted them the same
day. Vega has no transform for a sankey, a chord
diagram or hex bins, and no scale that skips the days with no trade. D6
says that each feature that Vega does not have takes a name that starts
with `zuzu.`. Gemini and DeepSeek reviewed a draft. "External review" in
the plan says what each said and what this decision took.

The Vega-Lite of these features waits for a decision of its own. A Vega
spec draws each target of tiers 1 and 2 first.

## Common rules

1. Each transform reads its fields by the parameters of Vega: a name, or an
   object with `field` or `expr`. Each output field has a default name, and
   the parameter `as` renames the output fields.
2. A transform that needs the result of another names its dataset, as
   `lookup` names `from` and the link force of `force` names `links`.
3. A transform writes positions in pixels, from an `extent` of the form
   `[[x0, y0], [x1, y1]]`, or a `size` of the form `[width, height]` from
   the origin. A path is the `d` attribute of SVG, which a `path` mark
   draws.

## zuzu.sankey

It runs on a dataset of links, a row for each flow, and makes a row for
each node.

| Parameter | Default | Meaning |
| --- | --- | --- |
| `source`, `target`, `value` | `source`, `target`, `value` | the fields of a link |
| `extent` or `size` | none, one is required | where the nodes go |
| `nodeWidth` | 24 | the width of a node |
| `nodePadding` | 8 | the space between the nodes of a column |
| `align` | `justify` | `left`, `right`, `center` or `justify`: the column of a node with no links on one side |
| `iterations` | 6 | the passes that move the nodes toward the middle of their links |
| `nodeSort` | `value` | `value` sorts the nodes of a column by size, and null keeps the order in which each first appears |
| `as` | see below | the names of the output fields |

Each node row holds `key`, `x0`, `x1`, `y0`, `y1`, `value`, `depth`, the
column from the left, and `layer`, the column that `align` chose. The value
of a node is the larger of the sum of its links in and the sum of its links
out. A cycle of links is an error.

## zuzu.sankeylinks

It runs on the same dataset of links, and the parameter `nodes` names the
dataset that `zuzu.sankey` made. The parameters `source`, `target` and
`value` are the same, and `nodeKey`, `key` by default, is the field of the
nodes that the links name. `linkSort` is `y`, the order of the other end,
or null, the order of the rows.

Each link gets `x0`, the right edge of its source, `x1`, the left edge of
its target, `y0` and `y1`, the middle of its band at each end, `width`, and
`path`, the band as a closed shape that curves across, to fill. The links
at a node stack in the order of `linkSort`, from the places of the nodes, so
the transform does not run the layout again.

## zuzu.sankeylabels

On 2026-11-11 this transform joined the decision, so that the labels of a
large sankey do not overlap. It runs on the dataset that `zuzu.sankey`
made, and reads the fields `x0`, `x1`, `y0`, `y1` and `value` of each node.

| Parameter | Default | Meaning |
| --- | --- | --- |
| `text` | `key` | the text of a label, as a field or an expression |
| `extent` or `size` | none, one is required | the extent of the nodes. A label stays inside it from left to right |
| `offset` | 6 | the space between a node and its label |
| `font`, `fontSize`, `fontWeight` | the theme, 11, `normal` | the font that measures a label |
| `as` | see below | the names of the output fields |

Each node gets `labelX`, `labelY`, `labelAlign` and `label`, which a
`text` mark draws with the baseline `middle`. A label is as tall as its
font size. It keeps a gap of 2 from a node and of 1 from another label.
The transform places the labels in four steps:

1. The labels of the larger nodes go first. A label goes on the inner
   side of its node: at the right of a node in the left half of the
   extent, and at the left of the others. If the label hits a label or a
   node there, it goes on the other side. If it hits on both sides, it
   stays on the first side that is inside the extent and clear of the
   nodes, or else on the inner side.
2. A label that hits a node moves up or down, away from the node. A label
   stays beside its node: its box stays within the height of the node, or
   its middle moves at most half its height from the middle of the node.
   If no such move clears the node, the label loses the end of its text to
   an ellipsis, at the side and the height that keep the most of it.
3. Labels that hit each other move apart, up and down, within the same
   reach.
4. A label that still hits loses the end of its text in the same way.
   The labels of the smaller nodes lose their ends first. A label that
   cannot keep one letter keeps all of its text where it is.

A sankey with few nodes keeps each label whole, at the middle of its node,
on the inner side.

## zuzu.chord

It runs on a dataset of links and makes a row for each group, which the
`arc` mark of Vega draws.

| Parameter | Default | Meaning |
| --- | --- | --- |
| `source`, `target`, `value` | `source`, `target`, `value` | the fields of a link |
| `padAngle` | 0 | the angle between two groups, in radians |
| `sortGroups` | null | `ascending`, `descending` or null, by the size of each group |
| `sortSubgroups` | null | the same, for the links inside a group |
| `directed` | false | see below |
| `as` | see below | the names of the output fields |

Each group row holds `key`, `index`, `startAngle`, `endAngle` and `value`.
The angles follow the arc mark of Vega: 0 at the top, and clockwise.
Without `directed`, the size of a group is the flow out of it, and a link
from A to B and a link from B to A are the two ends of one ribbon, each as
wide as its own flow. With `directed`, the size of a group is the flow out
of it and the flow into it, and each link is a ribbon of its own, as wide at
both ends.

## zuzu.chordribbons

It runs on the dataset of links, and the parameter `groups` names the
dataset that `zuzu.chord` made. The parameters `source`, `target`, `value`,
`sortSubgroups` and `directed` are the same as those of `zuzu.chord`. `radius` is the radius of the ends of the
ribbons, the inner radius of the arcs of the groups, with no default. Each
link gets `sourceStartAngle`, `sourceEndAngle`, `targetStartAngle`,
`targetEndAngle` and `path`, the ribbon around the center. A link from B to
A that forms the other end of a ribbon gets no path.

## zuzu.hexbin

It runs on points and makes a row for each hexagon that holds a point.

| Parameter | Default | Meaning |
| --- | --- | --- |
| `x`, `y` | none, both required | the place of a point in pixels, such as `{"expr": "scale('x', datum.fertility)"}`, as `kde2d` of Vega reads it |
| `radius` | 20 | the distance from the center of a hexagon to a corner |
| `extent` | none | only the hexagons whose centers lie inside |
| `groupby` | none | a set of hexagons for each group |
| `ops`, `fields`, `as` | `count` | the aggregates of each hexagon, as the transform `aggregate` reads them |

Each row holds `x` and `y`, the center, the group by fields, the aggregates,
and `path`, the hexagon around 0, 0, for a `path` mark placed at x and y. A
hexagon has a corner at the top.

## The scale zuzu.trading

A scale of type `zuzu.trading` places the days of its domain at even steps,
so that a day with no trade, such as a Saturday, leaves no gap.

1. Its domain is the list of days, as a band scale reads a domain: from a
   field of a dataset or from a list. The scale sorts the days and keeps
   each once. A time in a day is that day.
2. It has the range, `paddingInner`, `paddingOuter`, `align` and `round` of
   a band scale, and a mark reads its `bandwidth` with `band`, as a
   candlestick needs.
3. A time inside a day maps inside the band of the day, by its share of
   the 24 hours. A time outside the domain maps to nothing. `invert` gives
   the day of a place.
4. Its ticks are the first day in the domain of each week, month, quarter
   or year, at the coarsest step that gives no more ticks than `tickCount`.
   A boundary that falls on a day with no trade moves to the next day of
   the domain. An axis formats a tick as a time.

A candlestick draws with the marks of Vega: a `rule` from the low to the
high, and a `rect` from the open to the close, on this scale. zuzu adds no
mark for it.

Sessions inside a day, such as the hours of a market, are not part of this
decision.
