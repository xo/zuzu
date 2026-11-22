# Decisions

Every decision of zuzu is a file in this folder, one for each decision, named
by its number and its title (D4). This table is the index. Find the number
here, then open the file.

Each file opens with its status. Decided means the maintainers chose it.
Proposed means an agent or a peer session proposed it, and a maintainer has
not confirmed it. A decision that changes an earlier one says so in its
status, as "Amends D9", and the earlier one says it back, as "Amended by
D21". Read the status before the decision.

A new decision gets the next number and a file of its own. Add its row here.

| # | Decision | Status |
| --- | --- | --- |
| [D1](D001-zuzu-draws-charts-and-diagrams-in-pure-go-from-a-declarative.md) | zuzu draws charts and diagrams in pure Go from a declarative spec | Decided |
| [D2](D002-zuzu-is-set-up-for-coding-agents-as-dbmeta-d110-says.md) | zuzu is set up for coding agents as dbmeta D110 says | Decided |
| [D3](D003-the-plan-and-the-rules-come-before-any-code.md) | The plan and the rules come before any code | Decided |
| [D4](D004-each-decision-is-a-file-of-its-own.md) | Each decision is a file of its own | Decided |
| [D5](D005-outside-packages-are-allowed-and-pure-go-ones-are-preferred.md) | Outside packages are allowed, and pure Go ones are preferred | Decided |
| [D6](D006-the-spec-follows-the-documented-behavior-of-vega-and-vega.md) | The spec follows the documented behavior of Vega and Vega-Lite, and zuzu is not a port | Decided. Amended by D32 and D39 |
| [D7](D007-the-first-release-supports-static-vega-and-the-vega-lite.md) | The first release supports static Vega and the Vega-Lite that compiles to it | Decided. Amended by D52 |
| [D8](D008-the-scene-graph-is-the-boundary-between-the-layout-and-every.md) | The scene graph is the boundary between the layout and every renderer | Decided |
| [D9](D009-expressions-run-in-a-native-evaluator-and-goja-is-optional.md) | Expressions run in a native evaluator, and goja is optional | Decided. Amended by D33 |
| [D10](D010-svg-is-the-first-output-and-okage-rasterizes-it.md) | SVG is the first output, and okage rasterizes it | Decided. Amended by D43 |
| [D11](D011-a-direct-okage-renderer-with-text-through-an-api-that-okage.md) | A direct okage renderer, with text through an API that okage exposes | Decided |
| [D12](D012-zuzu-draws-its-own-design-and-does-not-reproduce-the-look-of.md) | zuzu draws its own design and does not reproduce the look of Vega | Decided. Amended by D32, D46 and D48 |
| [D13](D013-target-svgs-set-the-look-before-any-code.md) | Target SVGs set the look before any code | Decided |
| [D14](D014-a-theme-is-a-mode-and-a-color-scheme-as-one-vega-config.md) | A theme is a mode and a color scheme, as one Vega config | Decided. Amended by D35 and D37 |
| [D15](D015-color-schemes-come-from-the-upstream-themes-that-transit.md) | Color schemes come from the upstream themes that transit draws from | Decided. Amended by D33 |
| [D16](D016-zuzu-bundles-ibm-plex-sans.md) | zuzu bundles IBM Plex Sans | Decided. Amended by D32, D34, D35 and D36 |
| [D17](D017-conformance-with-vega-js-measures-meaning-not-look.md) | Conformance with vega.js measures meaning, not look | Decided. Amended by D32 |
| [D18](D018-dot-and-mermaid-compile-into-the-same-spec.md) | DOT and Mermaid compile into the same spec | Decided |
| [D19](D019-live-data-in-a-browser-comes-later-so-the-scene-graph-is.md) | Live data in a browser comes later, so the scene graph is JSON | Decided |
| [D20](D020-usql-makes-charts-with-zuzu-much-later-with-the-usql-peer.md) | usql makes charts with zuzu much later, with the usql peer | Decided |
| [D21](D021-the-names-of-the-packages.md) | The names of the packages | Decided |
| [D22](D022-the-documents-name-no-person.md) | The documents name no person | Decided |
| [D23](D023-the-survey-of-other-libraries-lives-in-docs-survey.md) | The survey of other libraries lives in docs/survey | Decided |
| [D24](D024-the-public-api.md) | The public API | Decided. Amended by D38, D42, D44 and D45 |
| [D25](D025-zuzu-reads-data-only-through-a-loader-that-the-caller-gives.md) | zuzu reads data only through a loader that the caller gives | Decided |
| [D26](D026-a-feature-mode-decides-between-an-error-and-a-report.md) | A feature mode decides between an error and a report | Decided |
| [D27](D027-time-is-in-the-time-zone-of-the-system-and-the-locale-is-us.md) | Time is in the time zone of the system, and the locale is US English | Decided |
| [D28](D028-random-numbers-are-random-and-the-caller-can-give-a-source.md) | Random numbers are random, and the caller can give a source | Decided. Amended by D38 |
| [D29](D029-zuzu-builds-with-tinygo-for-webassembly-and-has-no-size.md) | zuzu builds with TinyGo for WebAssembly, and has no size target | Decided |
| [D30](D030-the-transforms-and-the-scale-that-zuzu-adds-for-tiers-1-and-2.md) | The transforms and the scale that zuzu adds for tiers 1 and 2 | Decided |
| [D31](D031-diagrams-of-dot-and-mermaid-and-the-transform-zuzu-layered.md) | Diagrams of DOT and Mermaid, and the transform zuzu.layered | Decided. Amended by D32, D35, D38, D45 and D55 |
| [D32](D032-how-zuzu-reads-the-parts-of-mermaid-and-dot-that-it-does-not-draw.md) | How zuzu reads the parts of Mermaid and DOT that it does not draw | Decided. Amends D6, D12, D16, D17 and D31. Amended by D35, D36, D37, D38, D39 and D40 |
| [D33](D033-vegajs-is-the-oracle-of-expressions-and-schemes-are-go-values.md) | vega.js is the oracle of expressions, and schemes are Go values | Decided. Amends D9 and D15 |
| [D34](D034-ibm-plex-sans-is-embedded-in-woff2.md) | IBM Plex Sans is embedded in WOFF2 | Decided. Amends D16 |
| [D35](D035-the-answers-of-the-maintainers-of-2026-11-20.md) | The answers of the maintainers of 2026-11-20 | Decided. Amends D14, D16, D31 and D32. Amended by D36, D37, D40, D46 and D48 |
| [D36](D036-zuzu-draws-the-math-of-katex-with-the-math-layout-of-okage.md) | zuzu draws the math of KaTeX with the math layout of okage | Decided. Amends D16, D32 and D35 |
| [D37](D037-the-answers-of-the-maintainers-on-the-backlog-of-2026-11-20.md) | The answers of the maintainers on the backlog of 2026-11-20 | Decided. Amends D14, D32 and D35. Amended by D48, D53 and D55 |
| [D38](D038-the-answers-of-the-maintainers-on-the-backlog-of-2026-11-22.md) | The answers of the maintainers on the backlog of 2026-11-22 | Decided. Amends D24, D28, D31 and D32 |
| [D39](D039-zuzu-copies-the-layouts-of-dagre-and-elk-and-the-sketch-of-rough-js.md) | zuzu copies the layouts of dagre and ELK and the sketch of rough.js | Decided. Amends D6 and D32. Amended by D41, D44 and D45 |
| [D40](D040-the-details-of-the-look-drawn-by-hand.md) | The details of the look drawn by hand | Decided. Amends D32 and D35. Amended by D41, D42, D43, D47 and D49 |
| [D41](D041-the-output-is-the-same-on-one-architecture.md) | The output is the same on one architecture | Decided. Amends D39 and D40 |
| [D42](D042-the-options-and-the-rules-of-the-look-like-excalidraw.md) | The options and the rules of the look like Excalidraw | Decided. Amends D24 and D40. Amended by D47 |
| [D43](D043-the-svg-of-a-look-embeds-its-font.md) | The SVG of a look embeds its font | Decided. Amends D10 and D40 |
| [D44](D044-the-layouts-of-dagre-and-elk-and-their-review.md) | The layouts of dagre and ELK, and their review | Decided. Amends D24 and D39. Amended by D45 |
| [D45](D045-zuzu-dagre-follows-the-dagre-of-mermaid-and-a-layout-is-made-available-or-forced.md) | zuzu.dagre follows the dagre of Mermaid, and a layout is made available or forced | Decided. Amends D24, D31, D39 and D44 |
| [D46](D046-axes-ticks-and-named-schemes-follow-vega-more-closely.md) | Axes, ticks and named schemes follow Vega more closely | Decided. Amends D12 and D35. Amended by D51 |
| [D47](D047-a-gradient-keeps-its-colors-and-a-small-shape-wobbles-less-in-the-look.md) | A gradient keeps its colors and a small shape wobbles less in the look | Decided. Amends D40 and D42. Amended by D49 and D50 |
| [D48](D048-dark-colors-annotations-ranged-areas-and-the-start-of-the-ramp.md) | Dark colors, annotations, ranged areas and the start of the ramp | Decided. Amends D12, D35 and D37 |
| [D49](D049-a-hachure-draws-over-an-underlay-in-the-look.md) | A hachure draws over an underlay in the look | Decided. Amends D40 and D47. Amended by D50 |
| [D50](D050-a-gradient-draws-as-a-hachure-over-a-tint-in-the-look.md) | A gradient draws as a hachure over a tint in the look | Decided. Amends D47 and D49 |
| [D51](D051-an-axis-draws-the-grid-of-vega-and-labels-fewer-ticks.md) | An axis draws the grid of Vega and labels fewer ticks | Decided. Amends D46 |
| [D52](D052-zuzu-draws-the-projections-of-vega-projection-extended.md) | zuzu draws the projections of vega-projection-extended | Decided. Amends D7 |
| [D53](D053-an-agentflow-diagram-colors-its-nodes-by-kind-and-its-flows-in-turn.md) | An agentflow diagram colors its nodes by kind and its flows in turn | Decided. Amends D37. Amended by D54 and D55 |
| [D54](D054-composite-states-draw-a-title-band-and-notes-take-the-yellow-of-the-scheme.md) | Composite states draw a title band, and notes take the yellow of the scheme | Decided. Amends D53. Amended by D55 |
| [D55](D055-mermaid-takes-the-scheme-of-zuzu-and-tints-are-solid.md) | Mermaid takes the scheme of zuzu, and tints are solid | Decided. Amends D31, D37, D53 and D54 |
