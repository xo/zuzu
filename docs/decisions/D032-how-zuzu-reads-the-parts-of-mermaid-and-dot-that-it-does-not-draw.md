# D32. How zuzu reads the parts of Mermaid and DOT that it does not draw

Status: Decided. Amends D6, D12, D16, D17 and D31.

On 2026-11-12 the maintainers answered open questions 14 and 17 to 26 of
the plan, and three more questions about Mermaid and the look. The files
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
   not a part of the diagram. A look drawn by hand, modeled on the look of
   xkcd, and an ELK layout are work for later.
3. `animate` and `animation` on a link of Mermaid draw the link dashed, as
   one still frame of Mermaid, with no issue. A still image has no motion
   (open question 24).
4. The `theme` of Mermaid selects a look of zuzu. `dark` and each theme
   whose name ends in `-dark` select the dark mode. Each other theme, such
   as `forest` or `neutral`, selects a color scheme of zuzu that has the
   colors of that theme. A scheme that zuzu does not have yet comes first
   in `docs/STYLE.md`, as D14 and D15 say. `themeVariables` and `themeCSS`
   stay issues (open question 21).

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
   of the wrap by 11/14, the ratio of the size of a label of zuzu to the
   size of a label of Mermaid, so that a line holds the words that it holds
   in Mermaid.

## The look

8. The edges, the arrowheads and the messages of a diagram take a token
   Edge of `docs/STYLE.md`, darker than Axis, in the light and the dark
   mode (open question 18).
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
    (open question 23), and the changes to D31 of 2026-11-12 (open
    question 25).
