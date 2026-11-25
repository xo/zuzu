# D35. The answers of the maintainers of 2026-11-20

Status: Decided. Amends D14, D16, D31 and D32. Amended by D36, D37, D40, D46, D48 and D58.

On 2026-11-20 the maintainers answered the questions on the look that the
review page raised, open questions 12 and 29, and three questions of the
backlog. This decision records each answer. The work that follows from it is
in `docs/BACKLOG.md`.

## The colors of the data

These parts amend D14.

1. A scheme name of Vega that is diverging, such as `pinkyellowgreen`,
   `redblue` or `spectral`, takes the diverging ramp of the theme. Today it
   takes the sequential ramp.
2. A scheme name of Vega that is cyclical, `rainbow` or `sinebow`, takes a
   wheel of hues: the ten categorical colors of the scheme, in order, with
   the colors between them interpolated, and the last color joined to the
   first. Today it takes the sequential ramp, and its hues are lost.
3. A ramp that starts from the background starts from the background that
   the chart draws. That is the background of the spec when the spec sets
   one, and the background of the theme otherwise. A background that the
   caller gives wins over both, as it does today.

## The guides and the labels

4. zuzu applies an encode of the `x` or the `y` of a part of an axis or a
   legend, such as the labels of an axis, as vega.js does. Today such an
   encode goes in the report.
5. The row of a legend wraps at the width of the plot, not at the width of
   the whole chart.
6. `zuzu.endLabel` names no closed line. A closed line has a closed
   interpolation, such as `linear-closed`, or ends at its first point, as
   the line of a radar does. The legend names its series.
7. The numbers of the bits of a packet diagram of Mermaid draw at 10, the
   size of a small label in `docs/STYLE.md`. Today they draw at 9.

## The diagrams

8. Open question 29: the maintainers accept the parameters `lanes` and
   `laneEdges` of `zuzu.layered`, and the fields `titleWidth` and
   `titleHeight` of a row of a cluster, as D31 describes them. This amends
   D31.
9. The node property `algorithm` of Mermaid, which asks for an ELK layout
   of one subgraph, is a hint. zuzu reads it with no issue and lays the
   subgraph out with `zuzu.layered`, as it does for `layout: elk` (D32
   part 2). This amends D32.
10. A theme in the section of a kind of Mermaid, such as `venn.theme`,
    applies to a diagram of that kind, as in Mermaid. A mode or a scheme
    that the caller gives still wins, as open question 27 and D32 say.
    This amends part 4 of D32, which names only the theme at the top of the
    configuration.

## The fonts

11. Open question 12: zuzu bundles more fonts. Each one is in an optional
    package of its own, as `text/emoji` is, so that a program that does not
    import the package does not hold the font. Each one is under the Open
    Font License or a license like it (D16). zuzu bundles four kinds of
    font: a monospace, a humanist sans, a serif and a rounded sans. A scheme
    can name one of them, and draws in IBM Plex Sans when the caller has
    not given that font. This amends D16. Which font of each kind zuzu
    bundles is open question 30.

## The looks

12. A render option, `WithLook`, and a key of the spec, `zuzu.look`, select
    a look, such as a look drawn by hand. `look: handDrawn` of Mermaid
    selects the look like Excalidraw. The option of the caller wins over the
    spec, as it does for the mode and the scheme. The looks are in
    `docs/BACKLOG.md`, under "Draw two looks by hand".

## What waits

13. Math in a label of Mermaid, written as `$$...$$`, keeps its issue, and
    draws as its LaTeX source. The maintainers do not plan the work now.
    The okage peer said on 2026-11-20 that okage has no math layout. Its
    copy of go-text does not read the table `MATH` of OpenType, and no
    parser of TeX or layout of math exists in Go for zuzu to build on. The
    okage peer puts a reader of the table `MATH` in okage, after a decision
    there, and the parser and the layout in zuzu or in a package of their
    own.
14. Open question 4, on the split of the work of the browser, stays open
    until phase 8 starts.
