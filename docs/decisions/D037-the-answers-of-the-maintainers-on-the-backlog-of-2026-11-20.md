# D37. The answers of the maintainers on the backlog of 2026-11-20

Status: Decided. Amends D14, D32 and D35. Amended by D48, D53 and D55.

On 2026-11-20 the work on the backlog raised five questions. The
maintainers answered each one. This decision records the answers.

## The ramp

1. A sequential ramp takes its form from the lightness of the background
   that it starts from, not from the mode. On a dark background it runs on
   to a lighter color, and on a light background to a darker color. A
   background is dark when its lightness in HSL is below 0.5. For each
   background that a mode or a scheme of zuzu has, the ramp is the same as
   before. A dark background of a spec in the light mode no longer gives a
   ramp that ends in black. This amends D14 and part 3 of D35.
   `docs/STYLE.md` holds the rule.

## The default theme of a kind of Mermaid

2. Mermaid 12.3 sets the theme `redux-color` in the sections `flowchart`,
   `swimlane`, `agentflow`, `sequence`, `class`, `state`, `er`,
   `requirement`, `usecase` and `venn` of its default configuration. A
   diagram of such a kind with no theme in its source draws in that theme.
   zuzu does the same: such a diagram takes the scheme
   `mermaid-redux-color`. A theme that the source gives, at the top of the
   configuration or in the section of the kind, wins over the default, as
   in Mermaid. A mode or a scheme that the caller gives still wins over
   both (open question 27). This amends part 4 of D32 and part 10 of D35.

## The background of a graph of DOT

3. The attribute `bgcolor` of a graph of DOT fills the page, as in
   Graphviz. It is the background of the theme, as a background that
   `WithBackground` gives is: the boxes under the labels and the open
   arrowheads take it. The mode follows its lightness, so that the text
   stays readable on it: a dark `bgcolor` draws in the dark mode, and a
   light one in the light mode. This holds when the caller gives a mode
   too. A background that the caller gives with `WithBackground` wins over
   `bgcolor`. The compiled spec carries it in its config as
   `zuzu.background`, which any spec can set with the same meaning. A
   `bgcolor` that zuzu cannot read, such as a list of colors, is an issue.

## Two more answers

4. `colorScheme: rotate` of a use case diagram gives each actor, each use
   case and each system boundary a categorical color of the scheme in
   turn, in the order in which Mermaid gives its colors. Each one draws as
   a node of a graph of that color draws. As in Mermaid, `rotate` applies
   only in a color theme of Mermaid, `redux-color` or `redux-dark-color`,
   and a use case diagram with no theme is in `redux-color` (part 2). With
   `colorScheme: role`, the default, the diagram draws as before.
   `docs/STYLE.md` holds the rule.
5. A text that is cut with no mark is not planned. vega.js writes "…" for
   an empty `ellipsis`, as zuzu does, and no caller asked for a property of
   zuzu that cuts a text with no mark.
