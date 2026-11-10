# D4. Each decision is a file of its own

Status: Decided.

Each decision is a file in `docs/decisions/`, as dbmeta D111 says for a large
project. zuzu is new, but it already holds more than 20 decisions, and one file
each keeps the history of each one readable. The file is named
`D<nnn>-<title>.md`. It opens with `# D<n>. <title>`, a blank line, and
`Status: <status>.` Its row goes in `docs/decisions/README.md`.

A decision that the maintainers chose has the status Decided. A decision that
an agent or a peer proposed has the status Proposed until a maintainer
confirms it.
