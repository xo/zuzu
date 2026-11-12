# Contributing to zuzu

Read three things before you change anything.

[`docs/PLAN.md`](docs/PLAN.md) holds the plan, the phases and the open
questions. No code is written before the plan is ready, and a change to the
plan is a decision first. Do not decide an open question on your own. Ask the
maintainers.

[`AGENTS.md`](AGENTS.md) holds the rules. It is written for an AI coding
agent, and everything in it applies to a person. `CLAUDE.md` holds one line
that imports it, for Claude Code.

[`docs/decisions/`](docs/decisions/README.md) holds every decision, one file
each, with the reasons. Read the status of each. A Proposed decision waits
for the maintainers.

## Before you send a change

Run the checks that CI runs:

```bash
gofmt -l . && go vet ./... && go build ./... && CGO_ENABLED=0 go build ./... && go test -race -count=2 ./...
golangci-lint run ./... && (cd test && golangci-lint run ./... && go vet ./... && go build ./... && go test -race -count=1 ./...)
```

CI pins golangci-lint at v2.15.0 and installs it with `go install`, as
`.github/workflows/test.yml` says. Use the same version.

The tests of the root module read the documents too: the links, the decision
numbers, the index of the decisions and the rules of `simple-english`.

## The look

[`docs/STYLE.md`](docs/STYLE.md) holds the rules and the tokens of the look.
[`docs/targets/`](docs/targets/README.md) shows them in 33 target charts, in
both modes and in each color scheme. If you change a token, change it in
`STYLE.md` first, then in the package `theme`, then in
`docs/targets/gen-targets.mjs`, then draw the targets again and review them
in the gallery:

```bash
cd docs/targets && npm install && node gen-targets.mjs
```

```bash
python3 -m http.server 8767 --bind 127.0.0.1 --directory docs/targets
```

## The survey

[`docs/survey/compare.html`](docs/survey/compare.html) draws the same real
data with 14 other libraries. Serve the folder and open the page:

```bash
python3 -m http.server 8766 --bind 127.0.0.1 --directory docs/survey
```

[`docs/survey/README.md`](docs/survey/README.md) says how to generate its data
and its images again.

## Agent skills

The repository carries two agent skills. A skill is a set of instructions
that a coding agent loads for a task. `simple-english` sets how prose is
written, and `go-pedantry` sets how Go is written.

`skills-lock.json` names the source of each skill. The `skills` command from
npm writes that file, and dbmeta measured version 1.7.0. It writes each skill
into two folders. Codex and the other agents read `.agents/skills/<name>`,
and Claude Code reads `.claude/skills/<name>`.

To add a skill or to update one, run this in the root of the repository. The
example updates `simple-english`, and `skills-lock.json` holds the source of
each skill:

```bash
npx skills@1.7.0 add AminBlg/SimpleEnglish --skill simple-english --agent codex claude-code --copy -y
```

Keep `--copy`. Without it, the command writes `.claude/skills/<name>` as a
symbolic link. A Windows checkout writes a symbolic link as a text file, and
Claude Code then loads no skill and says nothing (dbmeta D89).

`.claude/settings.local.json` holds the Claude Code permissions of one
person. The root `.gitignore` ignores it.

## What the reviewer will ask

Does the change follow a decision? If it changes the design, where is the
decision?

Does a new feature of the spec have the name and the meaning that Vega gives
it? If Vega does not have it, does its name start with `zuzu.`?

Does the output stay the same, byte for byte, on another machine?

Does a change to the look start in `docs/STYLE.md`, and do the targets show
it?

Does a document name a person? Write "the maintainers".
