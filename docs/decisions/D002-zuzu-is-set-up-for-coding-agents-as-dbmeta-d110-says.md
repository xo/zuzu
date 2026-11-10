# D2. zuzu is set up for coding agents as dbmeta D110 says

Status: Decided.

zuzu has the files that dbmeta D110 lists for every `xo` repository:
`README.md`, `AGENTS.md`, `CLAUDE.md` with the one line `@AGENTS.md`,
`CONTRIBUTING.md`, `skills-lock.json`, `.gitignore`, `.gitattributes`,
`docs/PLAN.md`, the decisions with an index, and `docs/BACKLOG.md`.

It commits the skills `simple-english` and `go-pedantry` as copies in
`.agents/skills/` and `.claude/skills/`, as dbmeta D89 says. The copies came
from dbmeta on 2026-11-10 and match the hashes in `skills-lock.json`.

The maintainers asked that the documents of dbmeta be a template and not a
copy. zuzu keeps the shape of each document and writes its own content.
