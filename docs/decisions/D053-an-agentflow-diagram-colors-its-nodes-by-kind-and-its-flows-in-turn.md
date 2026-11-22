# D53. An agentflow diagram colors its nodes by kind and its flows in turn

Status: Decided. Amends D37. Amended by D54 and D55.

On 2026-11-22 the maintainers saw that zuzu drew every node and every flow
of an agentflow diagram, such as `diagrams/100-agentflow-flows.mmd`, in
one color, where Mermaid 12.3 gives each kind of node and each flow a color
of its own. The maintainers chose to color them by kind, as Mermaid does.

1. In a color theme of Mermaid, `redux-color`, the default of an agentflow
   diagram, or `redux-dark-color`, each node takes a categorical color of
   the scheme by its kind, in the order of the palette of Mermaid: a tool
   the first color, a task the second, a decision the third, a reference
   document the fifth, a connector the sixth and an action the seventh. An
   input takes the fourth. The palette has no cyan, the color of an input
   in Mermaid, and the eighth to the tenth go to the first flows, so an
   input in a flow keeps a color of its own.
2. Each flow takes the categorical colors in turn from the eighth, in the
   order in which the flows open, and starts again after the last, as
   Mermaid gives each flow the next color of its palette. A flow draws as
   a panel of its color: a tint and an outline.
3. A fill or a stroke that the source gives a node or a flow wins over its
   color. In any other theme, the nodes and the flows keep the one color
   of the scheme, as before.
4. The colors are those of the scheme of zuzu, as for the actors of a use
   case diagram with `colorScheme: rotate` (D37 part 4), so a scheme that
   the caller gives colors the diagram too.
