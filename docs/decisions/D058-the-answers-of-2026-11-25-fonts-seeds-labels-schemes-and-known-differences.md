# D58. The answers of 2026-11-25: fonts, seeds, labels, schemes and known differences

Status: Decided. Amends D15, D16, D32, D35 and D40.

On 2026-11-25 the maintainers answered twelve questions about the items
that the backlog held after v0.9.0.

## Fonts

1. Open question 30: zuzu bundles IBM Plex Mono as the monospace, Inter as
   the humanist sans, IBM Plex Serif as the serif and Nunito as the rounded
   sans, each in an optional package as D35 part 11 says. IBM Plex Mono and
   IBM Plex Serif match IBM Plex Sans, which zuzu bundles (D16). All four
   are under the Open Font License.
2. The reader of Mermaid reads a weight and a style out of `fontFamily` in
   one place, and gives them to the text of every kind that draws text.
   This replaces the issue that D32 part 5 gives in the kinds other than
   the graphs and the sequence.

## The look

3. A caller sets the seed of a look drawn by hand with the option
   `WithLookSeed`, which `Parse` and a render share, as `WithLook` does.
   The seed of a render wins over the seed of `Parse`, which wins over
   `zuzu.lookSeed` and `handDrawnSeed` of the source.
4. A label of a discrete x axis draws flat when each label fits the step
   of its band, and turns by 270 degrees, as Vega-Lite turns it, when one
   label does not fit. This holds in both looks. A `labelAngle` that the
   spec gives, in the axis or in its config, wins. `docs/STYLE.md` holds
   the rule.
5. The look drawn by hand has no target SVG. Its rules are in
   `docs/STYLE.md` and its golden files are in `testdata/look`. Hard rule
   13 covers the targets of the standard look only.

## Schemes from the styles of transit

6. The schemes of D15 come from the package `styles` of transit, and not
   from chroma or from each upstream theme. The package holds the styles
   of chroma that transit converted and the styles that transit made by
   hand from the editor themes of transit D98, each with its source, its
   commit and its license. transit admits only a license that MIT allows,
   so each of its styles can come in. This replaces the tool of D15 that
   reads each upstream theme at a pinned commit.
7. The command `test/cmd/schemes` of the test module reads the styles of a
   version of `github.com/xo/transit/styles` that `test/go.mod` requires.
   It writes the schemes as Go values in `theme/schemes.go`, the license
   file of each source in `theme/licenses/`, and the table of the schemes
   in `docs/SCHEMES.md`. The root module does not import transit (D33).
8. A style is one mode, light or dark by its background. Each style is a
   scheme with its own name. A family that has a light style and a dark
   style, such as Catppuccin Latte and Mocha, is also a scheme with the
   name of the family. A table in the command names the two styles of each
   family. The schemes `catppuccin` and `solarized` of the targets are two
   of these families.
9. A scheme takes the background and the text of the default entry of its
   style. Its other neutrals mix the text into the background by the
   shares of `docs/STYLE.md`. In the mode that a style does not have, the
   scheme keeps the neutrals of zuzu for that mode, and its colors lift or
   darken as the colors of a scheme do without colors of their own for that
   mode.
10. The categorical colors are the colors of the captures of the style,
    with no gray, no color near the background and no color near a color
    before it. `docs/STYLE.md` gives the order and the limits. A style with
    fewer than four such colors, such as `bw`, is not a scheme. The color
    of a rise is the color nearest to green, and the color of a fall is the
    color nearest to red.

## Known differences

11. These differences stay, and the documents record them instead of the
    backlog, because only the fonts of the browser or a rule that
    vega-lite.js does not document removes them:
    - A formula of KaTeX in Mermaid is narrower in zuzu than in the browser
      of mmdc, which draws MathML in its own font of math (D36).
    - zuzu measures a label against `minNodeWidth` in IBM Plex Sans, so a
      node can be narrower than in Mermaid, which measures in its own font.
    - The dataset `source_0` of `bar_heatlane` has one more row in zuzu
      than in vega.js, because zuzu keeps the filter of invalid values in
      each layer.
12. zuzu accepts a comment on the first line of a kind, as in
    `flowchart LR %% c`, which mmdc rejects. This is a known difference.

## Other answers

13. The three details of the projections of D52 are done now: the meridian
    at ±180° on the cuts of an interrupted or a polyhedral projection, the
    latitudes of `lobes`, and `clipAngle` on `armadillo` and `berghaus`.
14. Open question 4 stays open. The maintainers decide what the browser
    receives when phase 8 starts.
15. The two items of the survey close. The survey did its work in phases 0
    to 2.
16. The agent offers the fixes of the copy of elk-go to d2lang/elk-go and
    reports its differences there. Each text is short and plain, and names
    no agent. A maintainer reads and accepts each text before the agent
    posts it.
