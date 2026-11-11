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
| [D6](D006-the-spec-follows-the-documented-behavior-of-vega-and-vega.md) | The spec follows the documented behavior of Vega and Vega-Lite, and zuzu is not a port | Decided. Amended by D32 |
| [D7](D007-the-first-release-supports-static-vega-and-the-vega-lite.md) | The first release supports static Vega and the Vega-Lite that compiles to it | Decided |
| [D8](D008-the-scene-graph-is-the-boundary-between-the-layout-and-every.md) | The scene graph is the boundary between the layout and every renderer | Decided |
| [D9](D009-expressions-run-in-a-native-evaluator-and-goja-is-optional.md) | Expressions run in a native evaluator, and goja is optional | Decided |
| [D10](D010-svg-is-the-first-output-and-okage-rasterizes-it.md) | SVG is the first output, and okage rasterizes it | Decided |
| [D11](D011-a-direct-okage-renderer-with-text-through-an-api-that-okage.md) | A direct okage renderer, with text through an API that okage exposes | Decided |
| [D12](D012-zuzu-draws-its-own-design-and-does-not-reproduce-the-look-of.md) | zuzu draws its own design and does not reproduce the look of Vega | Decided. Amended by D32 |
| [D13](D013-target-svgs-set-the-look-before-any-code.md) | Target SVGs set the look before any code | Decided |
| [D14](D014-a-theme-is-a-mode-and-a-color-scheme-as-one-vega-config.md) | A theme is a mode and a color scheme, as one Vega config | Decided |
| [D15](D015-color-schemes-come-from-the-upstream-themes-that-transit.md) | Color schemes come from the upstream themes that transit draws from | Decided |
| [D16](D016-zuzu-bundles-ibm-plex-sans.md) | zuzu bundles IBM Plex Sans | Decided. Amended by D32 |
| [D17](D017-conformance-with-vega-js-measures-meaning-not-look.md) | Conformance with vega.js measures meaning, not look | Decided. Amended by D32 |
| [D18](D018-dot-and-mermaid-compile-into-the-same-spec.md) | DOT and Mermaid compile into the same spec | Decided |
| [D19](D019-live-data-in-a-browser-comes-later-so-the-scene-graph-is.md) | Live data in a browser comes later, so the scene graph is JSON | Decided |
| [D20](D020-usql-makes-charts-with-zuzu-much-later-with-the-usql-peer.md) | usql makes charts with zuzu much later, with the usql peer | Decided |
| [D21](D021-the-names-of-the-packages.md) | The names of the packages | Decided |
| [D22](D022-the-documents-name-no-person.md) | The documents name no person | Decided |
| [D23](D023-the-survey-of-other-libraries-lives-in-docs-survey.md) | The survey of other libraries lives in docs/survey | Decided |
| [D24](D024-the-public-api.md) | The public API | Decided |
| [D25](D025-zuzu-reads-data-only-through-a-loader-that-the-caller-gives.md) | zuzu reads data only through a loader that the caller gives | Decided |
| [D26](D026-a-feature-mode-decides-between-an-error-and-a-report.md) | A feature mode decides between an error and a report | Decided |
| [D27](D027-time-is-in-the-time-zone-of-the-system-and-the-locale-is-us.md) | Time is in the time zone of the system, and the locale is US English | Decided |
| [D28](D028-random-numbers-are-random-and-the-caller-can-give-a-source.md) | Random numbers are random, and the caller can give a source | Decided |
| [D29](D029-zuzu-builds-with-tinygo-for-webassembly-and-has-no-size.md) | zuzu builds with TinyGo for WebAssembly, and has no size target | Decided |
| [D30](D030-the-transforms-and-the-scale-that-zuzu-adds-for-tiers-1-and-2.md) | The transforms and the scale that zuzu adds for tiers 1 and 2 | Decided |
| [D31](D031-diagrams-of-dot-and-mermaid-and-the-transform-zuzu-layered.md) | Diagrams of DOT and Mermaid, and the transform zuzu.layered | Decided. Amended by D32 |
| [D32](D032-how-zuzu-reads-the-parts-of-mermaid-and-dot-that-it-does-not-draw.md) | How zuzu reads the parts of Mermaid and DOT that it does not draw | Decided. Amends D6, D12, D16, D17 and D31 |
