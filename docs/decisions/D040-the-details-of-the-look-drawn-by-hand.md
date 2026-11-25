# D40. The details of the look drawn by hand

Status: Decided. Amends D32 and D35. Amended by D41, D42, D43, D47, D49 and D57.

On 2026-11-22, before the work of D39 started, the maintainers answered
four questions on the look drawn by hand, and accepted the defaults below.

## The answers

1. The layout measures a text in the font of the look, so that each label
   fits its box. A look that changes the font changes the layout.
   `WithLook` is an option of Parse as well as of the render, as
   `WithEmoji` is (D38 part 6), because the kinds of Mermaid that lay out
   when they compile measure their text then. When both give a look, the
   option of the render wins. This amends part 12 of D35.
2. Excalifont, under OFL-1.1, ships in the optional package
   `text/excalifont`, as `text/emoji` and `text/math` do. A caller imports
   it and gives it to zuzu. Without it, the look like Excalidraw draws its
   text in IBM Plex Sans, and the report says so. This amends part 2 of
   D32, which said that a look draws by default with the font that zuzu
   tests it with.
3. The look like Excalidraw comes first. The look like xkcd waits for a
   font: xkcd-script is under CC BY-NC 3.0, and Humor Sans has no clear
   license, so neither meets D16. Open question 36 asks for a font under
   the OFL.
4. In a chart of Vega or Vega-Lite, the look changes every mark: the marks
   of the data, the axes, the grid lines, the legends and the frames. Only
   a text and an image keep their shape.

## The defaults

5. The look like Excalidraw takes the options of rough.js that Mermaid 12.3
   gives for `look: handDrawn`: a roughness of 0.7, the fill style
   hachure with a fill weight of 1.5 and a gap of 1.5, and a stroke of 1.3
   unless a style sets one.
6. The seed of a chart is 1. `handDrawnSeed` of Mermaid and the new key
   `zuzu.lookSeed` of a spec give another. Each item takes its seed from a
   hash of the seed of the chart, the name of its mark and its index (D39
   part 6).
7. The pass is the package `internal/sketch`, and `Render` runs it. The
   scene that `Render` returns, and its JSON form, hold the paths of the
   look.
8. A fill with a gradient draws its hachure in the color of its first stop,
   and the report names it.
9. golangci-lint and the tests of the comments of zuzu skip a copy, such as
   `internal/rough`. A copy keeps the tests of its upstream that need no
   Node.js and no data of D2.
10. CI gets a job on arm64, so that the golden tests of D39 part 15 run on
    a machine that fuses a multiply and an add.
11. The spike of dagro reports the error of the places of the nodes against
    mmdc for each fixture, for both layouts. The number informs the choice,
    and is not a test that fails.
12. How a caller gives the module of ELK to zuzu is an option of the API,
    which amends D24. It waits for the work on ELK.
