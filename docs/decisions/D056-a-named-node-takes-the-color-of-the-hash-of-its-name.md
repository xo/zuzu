# D56. A named node takes the color of the hash of its name

Status: Decided. Amends D37, D54 and D55.

On 2026-11-22 the maintainers saw that Mermaid 12.3 gives each class of a
class diagram and each entity of an ER diagram the next color of its
palette, in the order in which they first appear, so one entity can take
another color in each diagram, and that zuzu drew them all in the first
color. The maintainers chose to color a named node by the hash of its
name, so that one name keeps one color across diagrams and edits.

1. A named node takes the categorical color at the FNV-1a hash of 32 bits
   of its id, modulo the number of colors, ten. The nodes take their
   colors in the order in which they first appear. When an earlier node
   took the color of the hash, the node takes the next free color, so up
   to ten nodes take ten colors. After ten, a node takes the color of the
   hash, free or not. So a name keeps its color when the diagram changes,
   and only a name whose color an earlier name took moves.
2. The id is hashed as the source writes it, with its case, and not its
   label, so a class with a label keeps its color. The hash of the
   standard library of Go gives the same color on every architecture and
   in TinyGo (D41).
3. These nodes are named nodes: a class of a class diagram, an entity of
   an ER diagram, a requirement and an element of a requirement diagram,
   a participant of a sequence diagram and of a ZenUML diagram, an actor
   and a use case of a use case diagram with `colorScheme: rotate`, and a
   branch of a gitgraph. A class, an entity, a requirement and an element
   take a color only in a color theme of Mermaid, and so by default
   (D55). A participant and a branch take a color in every theme, as
   before. This replaces the colors in turn of D37 part 4 for a use case
   diagram.
4. A panel takes the color of the hash of its id in the same way, among
   the second to the tenth colors, so a panel never takes the first
   color, the color of the nodes in it: a subgraph and a lane of a
   flowchart, a composite state, a namespace of a class diagram, a
   subgraph of an ER diagram and a system boundary of a use case diagram
   with `colorScheme: rotate`. This replaces the colors in turn of D54
   part 2 and D55 part 7. A class in a namespace does not take the color
   of its namespace.
5. A node that takes its color by its kind keeps the map of its kind,
   because the kinds are few and known, and a hash puts two kinds on one
   color most of the time: a node of an agentflow (D53), an element of a
   C4 diagram (D32) and a box of an event model. The flows of an
   agentflow keep D53 part 2.
6. A chart whose colors stand for data keeps its colors in turn, as the
   ordinal scale of a chart of Vega takes them and as its legend lists
   them, with the first series in the first color: a pie, a sankey, a
   treemap, a radar, a venn, an xychart, a mindmap, and the sections of
   a timeline, a journey and a kanban.
7. A fill or a stroke that the source gives a node or a panel wins over
   its color.
