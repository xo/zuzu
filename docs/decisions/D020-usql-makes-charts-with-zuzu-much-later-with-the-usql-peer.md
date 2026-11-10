# D20. usql makes charts with zuzu much later, with the usql peer

Status: Decided.

The maintainers decided on 2026-11-10 that zuzu later gives `usql` a chart
from the result of a query, and that the work is much later, with the `usql`
peer. This is phase 9. zuzu never imports `usql`.

The API takes the result of a query from the start, as a `ResultSet` (D24),
so the later work of `usql` is the choice of a chart for a result, and not a
new input.
