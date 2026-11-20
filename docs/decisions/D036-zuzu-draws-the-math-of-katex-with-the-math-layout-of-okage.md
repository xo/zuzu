# D36. zuzu draws the math of KaTeX with the math layout of okage

Status: Decided. Amends D16, D32 and D35.

Mermaid 12.3 draws a text between two `$$` in a label, such as `$$x^2$$`,
as math with KaTeX. Today zuzu draws such a label as its LaTeX source and
reports an issue (D35 part 13, and item 14 of "Phase 7" in
`docs/BACKLOG.md`). okage plans a math layout that ivy and zuzu share. Its
proposal is okage D78, with open question 32 of okage. This decision says
how zuzu uses that layout. It waits for okage D78, and zuzu builds nothing
until okage releases the layout.

## What Mermaid draws

The oracle of Mermaid 12.3 (mmdc, with Chrome) shows these facts:

1. Mermaid calls KaTeX with `displayMode: true`. A formula draws in the
   display style of TeX, so `\frac{a}{b}` draws at full size, not as a
   small inline fraction. This is true also when text is on the same line.
2. KaTeX writes MathML, and the browser lays it out in its own math font.
   The math is at the size of the text of the label.
3. A line of a label with math is a row of its text and its formulas,
   centered on the vertical axis, with no wrap. The height of the line is
   the larger of the height of its text and the height plus the depth of
   its tallest formula. In the oracle, at 14 px, a line of text is 21 px
   high, a line with only `$$x^2$$` is 16.6 px high, and a line with
   `$$\frac{a}{b}$$` is 25.8 px high.

## The decision

1. The layout. zuzu lays out each formula with the math layout of okage.
   zuzu writes no TeX parser and no math layout of its own.
2. The font. The math draws in STIX Two Math, under the SIL Open Font
   License 1.1, from the package of okage that holds it. A package of zuzu,
   `text/math`, gives it, and `zuzu.WithMath(math.Font())` adds it to a
   render, as `zuzu.WithEmoji` adds the emoji font (D32). A program that
   does not import the package does not hold the font. zuzu checks the
   license of the font as it checks each bundled font (D16). This amends
   D16.
3. The labels. zuzu splits a label into runs of text and runs of math in
   the kinds and the labels of the table "Math of KaTeX" in
   `docs/DIAGRAMS.md`. These are the graphs, the mindmap and the block
   diagram in a label of HTML, and the sequence diagram always. With
   `htmlLabels: false`, and in the other kinds, zuzu draws the source, as
   Mermaid does, with no issue.
4. The measure. A font that can lay out math gives the width, the height
   and the depth of a formula through an interface of the package `text`
   next to `text.Measurer`. A line with math takes its height as in fact 3
   of "What Mermaid draws". The kinds that lay out when they compile, such
   as the sequence diagram, measure math in the same way.
5. The scene. zuzu draws each formula as one item of a mark of type `path`
   with the role `math`. The path holds the outline of each glyph and each
   rule of the formula, at the position of the formula, and its fill is
   the color of the text of the label. The item keeps the LaTeX source in
   its field `text`, which a path does not draw. `scene.Encode` and
   `scene.Decode` need no change.
6. The writers. The SVG writer writes the path, so that a viewer needs no
   math font, and writes the source as the `aria-label` of the path. The
   raster draws the path with okage.
7. The issues. A formula draws as math with no issue. A label with math
   draws its source, and reports the issue of today, in two cases: the
   render has no math font, or okage cannot parse the formula. For a
   formula that okage cannot parse, such as one with an unknown command,
   the issue names the command. Each such issue is an error in the strict
   mode. This amends part 13 of D35, and the issue of the math of KaTeX of
   D32.
8. The fonts of the oracle. The browser of the oracle draws math in its own
   math font, not in STIX Two Math. The checks compare the size of a label
   with math to the oracle within the tolerance of the diagrams (D31), and
   do not compare the glyphs.
9. go-text. zuzu reads its fonts with `github.com/xo/okage/typesetting/font`,
   the copy of go-text in okage, and no longer requires
   `github.com/go-text/typesetting`. That copy has the functions that zuzu
   uses, and the math layout of okage takes the faces of that copy. A
   program then holds one copy of go-text, not two.

## The order of the work

1. The maintainers of okage decide okage D78. If its API differs from what this decision assumes,
   zuzu updates this decision first.
2. zuzu moves to the release of okage that holds the math layout and
   reads its fonts with the copy of go-text in okage. The corpus and the
   oracles give the same output, or the change says why each one differs.
3. zuzu builds parts 2 to 7, with specs with math in each kind of the
   table, compared with the oracle, and unit tests of the split, the
   measure and the writers.
4. `docs/DIAGRAMS.md` says what draws, and item 14 of "Phase 7" leaves
   `docs/BACKLOG.md`.

## The answer

On 2026-11-20 the maintainers accepted this decision as written, which
answers open question 31 of `docs/PLAN.md`. The work waits for the release
of okage that holds the math layout.
