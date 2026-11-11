# Mermaid

This survey looks at Mermaid on 2026-11-11, for phase 7 and D31: which kinds
of diagram Mermaid has, which other programs draw them without mermaid.js,
which kinds zuzu can compile to a Vega spec, and how Mermaid draws icons.
Three agents gathered the facts from the sources named below. Each count and
each license is as the source gave it on that day.

## Versions

| Line | Version | Date | Notes |
| --- | --- | --- | --- |
| stable | 12.3.0 | 2026-11-03 | the `latest` tag on npm. The local `mmdc` 12.2.0 bundles it |
| alpha | 13.0.0-alpha.2 | 2026-11-09 | the `v13-alpha` tag. A rewrite of the engine, which its page calls a proof of concept |

Version 12.0.0, on 2026-09-10, made these changes:

- ELK became the default layout of the flowchart, the state, class, ER,
  requirement and use case diagrams, with `layout: dagre` to get the old
  one back.
- The look `neo` and the theme `redux-color` became the defaults of those
  types.
- A diagram type can set its own theme, look and layout.
- The use case and agentflow diagrams are new.
- The legacy renderers were removed, and ES2024 and Node 22.12 became
  required.

Versions 12.1 to 12.3 added a direction for the feedback edges of ELK, the
order of the bits of a packet, color by depth in a mindmap, and the shapes
of BPMN in a flowchart.

Version 13 alpha draws with no DOM, so it runs on a server, and its output
is the same each time. It draws labels as SVG text, and it writes ASCII for
six kinds. It splits Mermaid into a core and a package for each kind of
diagram. Icons, the hand drawn look, math, clicks and ELK move into
plugins. Its own layered engine answers to `dagre`, and a tree engine draws
the mindmap. It ports 22 kinds and leaves out 15, which throw
`UNSUPPORTED_DIAGRAM`. It adds no kind of diagram.

Sources: https://registry.npmjs.org/mermaid,
https://github.com/mermaid-js/mermaid/releases/tag/mermaid@12.0.0,
https://mermaid.ai/open-source/intro/v13-alpha.html, and the tarballs of
12.3.0 and 13.0.0-alpha.2.

## The kinds of diagram

The table lists each kind in 12.3.0, with the version that first holds it in
the bundle where the survey checked it, and whether version 13 alpha ports
it. "Implementations" counts the 23 programs outside mermaid.js below that
draw it.

| Keyword | Status | Since | v13 alpha | What it draws | Implementations |
| --- | --- | --- | --- | --- | --- |
| `flowchart`, `graph` | stable | old | yes | nodes and edges, layered | 23 |
| `sequenceDiagram` | stable | old | yes | lifelines, time down the page | 21 |
| `stateDiagram-v2` | stable | old | yes | states, layered | 21 |
| `classDiagram` | stable | old | yes | UML classes, layered | 20 |
| `erDiagram` | stable | old | yes | entities and relations, layered | 19 |
| `pie` | stable | old | yes | a pie | 18 |
| `gantt` | stable | old | yes | tasks on a time axis | 17 |
| `mindmap` | stable | old | yes | a radial tree | 15 |
| `gitGraph` | stable | old | yes | commits and branches on lanes | 15 |
| `timeline` | stable | old | yes | events in periods | 14 |
| `xychart` | stable | before 11 | yes | bars and lines on axes | 13 |
| `quadrantChart` | stable | before 11 | yes | points in four quadrants | 13 |
| `block` | stable | before 11 | no | blocks on a grid of columns | 13 |
| `kanban` | stable | 11.4.0 | yes | cards in columns | 13 |
| `sankey` | stable | before 11 | yes | weighted flows | 12 |
| `journey` | stable | old | yes | steps of a user with scores | 12 |
| `packet` | stable | 11.0.0 | yes | the bit fields of a packet | 12 |
| `C4Context` and four others | stable | old | no | C4 boxes, placed by Mermaid | 12 |
| `requirementDiagram` | stable | old | no | requirements, layered | 12 |
| `architecture-beta` | beta | 11.1.0 | no | services and groups with icons, by a force layout | 11 |
| `radar-beta` | beta | 11.6.0 | yes | values on radial axes | 11 |
| `treemap` | stable | 11.8.0 | yes | nested rects sized by value | 11 |
| `venn-beta` | beta | 11.13.0 | yes | overlapping sets | few |
| `ishikawa` | stable | 11.13.0 | no | a fishbone of causes | few |
| `wardley-beta` | beta | 11.14.0 | yes | a map of evolution and visibility | few |
| `treeView-beta` | beta | 11.14.0 | no | a tree of folders and files | few |
| `eventmodeling` | stable | 11.15.0 | no | swimlanes of events by time frame | few |
| `swimlane-beta` | beta | 11.16.0 | no | steps in lanes | few |
| `cynefin-beta` | beta | 11.16.0 | yes | five domains in fixed places | few |
| `railroad-beta` and three grammars | beta | 11.16.0 | no | syntax diagrams | few |
| `usecase-beta` | beta | 12.0.0 | no | actors, use cases and a boundary | few |
| `agentflow-beta` | beta | 12.0.0 | no | agents, tasks and tools | few |
| `zenuml` | plugin | none | no | sequences in the syntax of ZenUML | 5 |

Mermaid has these layouts: `dagre`, ELK with its variants, `cose-bilkent`
for the mindmap, `fcose` for the architecture diagram, `tidy-tree`, and an
internal one for swimlanes.

A node of a flowchart can name one of about 50 shapes with
`A@{ shape: name }` since 11.3.0, and the shapes of BPMN since 12.3.0. A
node can also be an icon, `A@{ icon: "fa:user" }`, or an image,
`A@{ img: "url" }`.

Source: https://mermaid.ai/open-source/intro/syntax-reference.html and the
page of each kind under https://mermaid.ai/open-source/syntax/.

## Implementations outside mermaid.js

The survey found 23 programs that parse Mermaid, lay it out and draw it
without a JavaScript engine. Most started in 2026, and several say that
coding agents wrote them. Each claim of fidelity is the claim of its own
authors.

| Program | Language | License | What it covers | How it checks |
| --- | --- | --- | --- | --- |
| [merman](https://github.com/Latias94/merman) | Rust | MIT or Apache-2.0 | all 23 classic kinds and the new ones, with ports of dagre and of ELK layered | baselines of the SVG of 12.3.0 for 37 kinds. Zed uses it |
| [mermaid-rs-renderer](https://github.com/1jehuang/mermaid-rs-renderer), `mmdr` | Rust | MIT | 23 kinds, with its own layered layout | a gallery, with no claim of parity |
| [sebastian](https://github.com/aovestdipaperino/sebastian) | Rust | MIT | 18 kinds the same to the byte as `mmdc` 11.15, with dagre | corpora by kind |
| [selkie](https://github.com/btucker/selkie), [frankenmermaid](https://github.com/Dicklesworthstone/frankenmermaid) | Rust | MIT, with a rider for frankenmermaid | 22 kinds and more | comparison with mermaid.js |
| [mmdg](https://github.com/bvolpato/mermaid-go-renderer) | Go | MIT | 22 kinds and more. It says it ports the parsers and the layouts of 12.2.0 | 2142 of 2145 diagrams the same, an untested claim of a project of 4 stars |
| [go-mermaid](https://github.com/zkrebbekx/go-mermaid) | Go | MIT | 20 kinds, with its own layered layout | golden SVGs and fuzzing |
| [mermaid-ascii](https://github.com/AlexanderGrooff/mermaid-ascii) | Go | MIT | flowchart, sequence and ER, as text | none |
| [Naiad](https://github.com/Papyrine/Naiad) | .NET | MIT | 23 kinds and more, with a port of dagre | snapshots |
| [Mermaider](https://github.com/nullean/mermaider) | .NET | MIT | 22 kinds and more, with its own layered layout | none named |
| [Maradona](https://github.com/rock3r/maradona) | Kotlin | Apache-2.0 | 23 kinds, with a port of dagre | 71 fixtures against mermaid.js |
| [termaid](https://github.com/fasouto/termaid) | Python | MIT | 18 kinds, as text | none named |
| [pozeiden](https://github.com/sc2in/pozeiden) | Zig | not named | 17 kinds | golden baselines |

The other programs in the count draw fewer kinds, or draw text in a
terminal:

- Rust: [mmdflux](https://github.com/kevinswiber/mmdflux),
  [mermaid-render](https://github.com/alitrack/mermaid-render),
  [meraid](https://github.com/Binlogo/meraid),
  [graphs-tui](https://github.com/decisiongraph/graphs-tui) and
  [oxdraw](https://github.com/RohanAdwankar/oxdraw)
- Go: [mmaid-go](https://github.com/aaronsb/mmaid-go) and
  [mermaidtext](https://github.com/spacedock-dev/mermaidtext)
- Python: [merm](https://github.com/alexeygrigorev/merm)
- Zig: [zig-mmdc](https://github.com/pastel-sketchbook/zig-mmdc)
- Typst: [umlaut](https://github.com/robertjndw/umlaut)

[Kroki](https://github.com/yuzutech/kroki),
[goldmark-mermaid](https://github.com/abhinav/goldmark-mermaid),
[mermaid-py](https://github.com/ouhammmourachid/mermaid-py),
[mermaidx](https://github.com/MohammadRaziei/mermaidx) and
[mermaid-java](https://github.com/aresstack/mermaid-java) run mermaid.js,
so they are not in the count.
[beautiful-mermaid](https://github.com/lukilabs/beautiful-mermaid) and
[pele](https://github.com/obsidianmd/pele) draw with their own code but
still need a JavaScript engine. PlantUML does not read Mermaid, and D2 has
no import of its own. Mermaid itself is
[mermaid-js/mermaid](https://github.com/mermaid-js/mermaid), and its
version 13 is [mermaid-js/mermaid-v13](https://github.com/mermaid-js/mermaid-v13).

Sources: the README and the status pages of each project on GitHub, with
crates.io, pkg.go.dev and PyPI.

## Which kinds compile to a Vega spec

Every kind can draw as marks of Vega once something works out the places.
The question is how much zuzu must add. This sorts the kinds by what each
needs.

1. Charts that compile to the specs of Vega-Lite or Vega with what zuzu
   has: `pie`, `xychart`, `sankey` (with `zuzu.sankey`), `radar`,
   `treemap`, `quadrantChart`, `gantt` (bars on a time axis), `journey`
   (scores by step) and `wardley` (points and lines on two axes from 0 to
   1). These are the cheapest and the closest to what zuzu already draws.
2. A chart that needs a transform of its own: `venn`, whose circles need a
   layout of overlapping areas, `zuzu.venn`.
3. Diagrams of nodes and edges that `zuzu.layered` of D31 lays out:
   `flowchart`, `stateDiagram`, `classDiagram` and `erDiagram`, whose nodes
   are tables of fields like the records of DOT, `requirementDiagram`,
   `usecase`, `agentflow`, the C4 diagrams, and `swimlane` and
   `eventmodeling` with lanes as clusters. `architecture` uses a force
   layout in Mermaid, and the layered layout or the force of Vega can stand
   in.
4. Trees that the transforms of a hierarchy of Vega lay out: `mindmap`
   (tree or cluster, radial), `treeView` (a tree with an indent) and
   `ishikawa` (a fishbone, a small layout of its own).
5. Diagrams of rows, columns and lanes, whose places follow from the source
   and the size of the text: `sequenceDiagram`, `gitGraph`, `kanban`,
   `packet`, `block`, `timeline`, `cynefin` and `railroad`. Each needs a
   small layout of its own, as a transform that measures text, but no graph
   layout.

`zenuml` is a plugin with a grammar of its own, and waits.

The counts of the implementations give an order of value: the flowchart,
the sequence, the state, the class and the ER diagrams first, then the
charts of group 1, which cost the least.

## Icons

Mermaid draws icons from packs in the JSON format of Iconify. A program
registers a pack with `registerIconPacks`, under a name that replaces the
prefix of the pack, and a diagram names an icon as `pack:name`. A missing
pack or icon draws an icon with a question mark, and the diagram does not
fail. In version 13 alpha, the icons move into a plugin.

| Kind | How it names an icon |
| --- | --- |
| `architecture-beta` | `service db(database)[Title]`, with five icons built in (`cloud`, `database`, `disk`, `internet`, `server`), or `logos:aws-lambda` from a pack |
| `flowchart` | `A@{ icon: "fa:user", form: square, label: "...", pos: t }`, and `fa:fa-twitter` inside a label |
| `mindmap` | `::icon(fa fa-book)`, classes of a font of icons that the page loads, not Iconify |
| `usecase-beta`, `treeView-beta` | `@{ icon: "fa:user" }` and `icon(logos:react)` |

An icon of Iconify is the inside of an SVG element, `body`, with a width, a
height, a left and a top that make its view box, a rotation and two flips.
An alias names a parent and adds its own rotation and flips. A set of one
color draws in `currentColor`.

Sources: https://mermaid.ai/open-source/config/icons.html and
https://iconify.design/docs/types/iconify-json.html.

### Icon packs

| Pack | Icons | License | Size of the JSON | Fit for zuzu |
| --- | --- | --- | --- | --- |
| `carbon`, by IBM | 2,644 | Apache-2.0 | 1.26 MB | can be bundled. It matches IBM Plex |
| `lucide` | 2,046 | ISC | 0.67 MB | can be bundled |
| `tabler` | 6,287 | MIT | 2.16 MB | can be bundled |
| `heroicons`, `bi` | 1,288 and 2,078 | MIT | 0.6 and 1.1 MB | can be bundled |
| `mdi` | 7,447 | Apache-2.0 | 3.48 MB | can be bundled, but large. The mindmaps of Mermaid name it |
| `fa7-solid` | 2,000 | CC BY 4.0 | 1.21 MB | can be bundled, with its attribution. Flowcharts name `fa:` |
| `codicon` | 657 | CC BY 4.0 | about 0.4 MB | can be bundled, with its attribution |
| `k8s`, Kubernetes | 38 | Apache-2.0 or CC BY 4.0 | 73 KB | can be bundled, without the logo, which is a trademark |
| `logos` | 2,064 | CC0, but each is a brand | 8.74 MB | load at run time. It holds 65 icons of AWS, and those of Azure and Google Cloud |
| `simple-icons`, `devicon`, `vscode-icons`, `fa7-brands` | 600 to 3,500 | CC0 or MIT, but brands | 1 to 5.5 MB | load at run time |
| `gcp` on Iconify | 214 | labeled Apache-2.0, with no source | 162 KB | load at run time until the source is clear |
| AWS Architecture Icons | 800 and more | for diagrams only. The repository of AWS for PlantUML ships them under CC BY-ND 2.0, and the trademark rules forbid a change | none | do not bundle |
| Azure icons | | for diagrams, training and documents only | none | do not bundle |
| CNCF artwork | | the trademark rules of the Linux Foundation | none | do not bundle |

The terms of AWS say that customers and partners can use the icons to make
diagrams, and the trademark rules forbid any change to a logo. A pack that
recolors or crops them for Iconify, such as aws-mermaid-icons, changes
them, and its license is not stated. Other tools do the same as Mermaid:
D2 names icons by URL, Structurizr names hosted images, and draw.io ships
the stencils of the vendors in an application.

Sources: https://aws.amazon.com/architecture/icons/,
https://aws.amazon.com/trademark-guidelines/,
https://github.com/awslabs/aws-icons-for-plantuml,
https://learn.microsoft.com/en-us/azure/architecture/icons/,
https://github.com/kubernetes/community/tree/master/icons,
https://github.com/cncf/artwork/blob/master/LICENSE.md,
https://raw.githubusercontent.com/iconify/icon-sets/master/collections.json,
and the license of each pack.

## What follows for zuzu

These are leads for the maintainers, not decisions:

1. D31 follows 12.3.0, the version of the local `mmdc`. Its subset of the
   flowchart needs the shapes of `A@{ shape: name }`, which the flowcharts
   of 12 use, and the nodes of an icon and of an image.
2. The charts of group 1 are a cheap next step after the flowchart. Each
   compiles to a spec that zuzu already draws.
3. zuzu reads packs of Iconify, with a registry by prefix as Mermaid has,
   and draws an icon with a question mark for a missing one. The five icons
   of the architecture diagram are drawn by zuzu in its own look.
4. Each bundled pack is a package of its own that a program imports, such
   as a pack of `carbon`, `fa7-solid`, `mdi` and `k8s`, so that a program
   that draws no icon does not carry them. A program loads the packs of a
   vendor, such as those of AWS, itself, under the terms of that vendor.
