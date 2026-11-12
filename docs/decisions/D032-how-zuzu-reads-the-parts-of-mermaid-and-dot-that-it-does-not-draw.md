# D32. How zuzu reads the parts of Mermaid and DOT that it does not draw

Status: Decided. Amends D6, D12, D16, D17 and D31.

On 2026-11-12 the maintainers answered open questions 14 and 17 to 28 of
the plan, and more questions about Mermaid and the look. The files
of iv and the corpus of `github.com/bvolpato/mermaid-go-renderer` raised
them. This decision records each answer. The work that each answer needs
is in `docs/BACKLOG.md`.

## The front matter and the settings of Mermaid

1. zuzu reads `description` in the front matter of Mermaid as the
   description of the Vega spec, the same field that `accTitle` and
   `accDescr` fill. Mermaid ignores the key. A key of the front matter
   that is not `title`, `displayMode`, `config` or `description` stays an
   issue (open question 22).
2. `look: handDrawn` and `layout: elk` are hints. zuzu draws its own look
   and its own layered layout, and reports neither. This is an exception
   to D6, which the maintainers chose because both name a way to draw and
   not a part of the diagram. An ELK layout is work for later. zuzu will
   have two looks drawn by hand beside its standard look: a look like
   Excalidraw, with the font Excalifont, and a look like xkcd, like the
   look of xkcd in matplotlib. zuzu draws each in its own way, and ports no
   code of Excalidraw, rough.js or matplotlib. `look: handDrawn` of Mermaid
   selects the look like Excalidraw, once zuzu has it. The font of a look
   is opt-in: a caller can give another font, as `fontFamily` does in
   Mermaid. By default, each look draws with the font that zuzu tests it
   with, such as Excalifont for the look like Excalidraw.
3. `animate` and `animation` on a link of Mermaid draw the link dashed, as
   one still frame of Mermaid, with no issue. A still image has no motion
   (open question 24).

   A part of a source that Mermaid 12.3 reads and draws nothing for, zuzu
   accepts with no issue, because zuzu then draws what Mermaid draws. The
   directions of `Rel_U`, `Rel_D`, `Rel_L`, `Rel_R`, `Node_L` and `Node_R`
   of C4, `$tags` and `$sprite` of C4, and the data of a node of an
   agentflow diagram, such as `model` or `endpoint`, are such parts. A part
   that changes what Mermaid draws, and that zuzu does not draw, stays an
   issue.
4. The `theme` of Mermaid selects a look of zuzu. Each theme with `dark`
   in its name, such as `dark`, `redux-dark` or `redux-dark-color`, selects
   the dark mode. Each other theme, such as `forest` or `neutral`, selects
   a color scheme of zuzu that has the colors of that theme. A scheme that
   zuzu does not have yet comes first in `docs/STYLE.md`, as D14 and D15
   say. `themeVariables` and `themeCSS` stay issues (open question 21),
   except the theme variables that a chart reads, such as `pie1` to
   `pie12` of a pie and `xyChart.plotColorPalette` of an xyChart.
   The theme of the source applies by default. A mode or a scheme that the
   caller gives with `WithMode` or `WithScheme` wins over it, so a program
   that sets its own look keeps it (open question 27). The compiled spec
   carries the theme in its `config` as `zuzu.mode` or `zuzu.scheme`, and
   the runtime takes each one only where the caller sets no mode or no
   scheme.

## Fonts and text

5. zuzu draws in IBM Plex Sans only (D16). It reads a weight or a style
   from `fontname` of DOT and from `fontFamily` of Mermaid, such as `Bold`,
   `Italic` or `Oblique` in `Helvetica-Bold`, and draws the text in Plex
   with them. It accepts the rest of the name with no issue (open question
   19).
6. An emoji draws in color from Noto Color Emoji. The font goes in a
   package of its own, which a caller imports and passes with an option,
   as an icon pack does (D31). A program that does not import it pays
   nothing for its size. A character that no font of the render has draws
   nothing, and the report names it (open question 20).
7. A label of Mermaid wraps where Mermaid wraps it. zuzu scales the width
   of the wrap by the ratio of the size of a label of zuzu, 11 pixels, to
   the size of a label of that kind in Mermaid, so that a line holds the
   words that it holds in Mermaid. The ratio is 11/14 for most kinds, and
   11/16 for a timeline, whose labels Mermaid draws at 16 pixels.

## The look

8. The edges, the arrowheads and the messages of a diagram take a token
   Edge of `docs/STYLE.md`, darker than Axis, in the light and the dark
   mode (open question 18). On 2026-11-12, after iv drew the ER diagrams
   of dbtpl, the maintainers made Edge darker again, the line of an edge a
   little thicker, and the arrowheads larger, so that a long edge reads at
   the size of a terminal.
9. A band behind a group of rows, such as a section of a gantt, takes a
   token Band of `docs/STYLE.md`.

## Links

10. An item of the scene graph carries a link and a tooltip, as the
    encodings `href` and `tooltip` of Vega do. The SVG writer writes them
    as `<a>` and `<title>`, and the PNG ignores them. `URL`, `href` and
    `tooltip` of DOT and `click` and `link` of Mermaid with a URL fill
    them. A callback of `click` stays an issue, because no script runs
    (open question 26).

## The order of the kinds of Mermaid

11. zuzu adds the kinds of Mermaid that it does not draw in the order of
    their files in the corpus of mermaid-go-renderer, about five kinds at a
    time: requirement, gitGraph, block, swimlane, usecase, agentflow,
    timeline, C4, venn, treeView, railroad, eventmodeling, zenuml, cynefin,
    ishikawa, kanban, packet and info (open question 17).

## The ids of a gitGraph

12. Mermaid gives a commit of a gitGraph with no id a random id. zuzu takes
    the id from a hash of the number, the branch and the message of the
    commit, so that the same source gives the same image. D28 does not
    apply, because the source asks for no random number (open question
    28).

## Conformance

13. The first check of D17 compares the structure of the Vega that zuzu
    compiles from Vega-Lite with the Vega of vega-lite.js: the same
    datasets by name, the same scales by name and type, and the same marks
    by name and type. It does not compare the JSON (open question 14).
14. A transform of Vega that changes rows in place shares them with the
    datasets that derive from the same source, as vega.js does. zuzu
    follows vega.js here.

## Reviews

15. The maintainers accepted the changes to D30 of 2026-11-11 and
    2026-11-12, the transform `zuzu.sankeylabels` and its parameter `side`
    (open question 23), and the changes to D31 of 2026-11-12 that open
    question 25 names: how the layout breaks a cycle, orders a rank and
    places the nodes, and `wrap` and `breakWords`. The parameters `lanes`
    and `laneEdges`, which came later, wait for open question 29.

## The C4 and the use case diagrams

16. An external element of a C4 diagram, such as `Person_Ext` or
    `System_Ext`, takes the neutral Muted of `docs/STYLE.md`, at the tint
    of the other elements, as Mermaid draws it gray. It needs no token of
    its own.
17. A boundary of `type: package` of a use case diagram draws with a tab
    at its top left, as a package of UML does.
