# <project> — planning ICM workspace

Layer 1 root map (Interpretable Context Methodology). Every agent reads this
file first, then navigates by the routing table below. `CLAUDE.md` just points
here; this file is harness-agnostic and the single source of truth.

## The loop

A change is an initiative, often across many repos, and ends in tickets. It flows
through eight phases, each owned by a persona. Research, explore and synthesis are
skippable, with a reason. `zumen change new` skips explore when there's no repo, and
synthesis when there are fewer than two.

`intake → [research] → [explore] → [synthesis] → spec → architecture → breakdown → publish`

- **intake** — the goal, constraints, stakeholders and repos in scope: a prose
  issue/idea, or a pointer to an existing codebase.
- **research** (researcher) — external investigation: docs, vendors, prior art,
  subject-matter experts.
- **explore** (explorer) — one findings report per repo (system map, entry points,
  module responsibilities, data flow, cited `file:line`, debt/risk observations).
  Observation only; no prescription.
- **synthesis** (architect) — what the findings add up to across repos: common
  patterns, divergences, constraints.
- **spec** (analyst) — unambiguous, verifiable system-level requirements with
  Given/When/Then acceptance criteria.
- **architecture** (architect) — the smallest solution: decisions with rationale and
  rejected alternatives, component map, interfaces, data flow. Record durable
  decisions as ADRs under `04_architecture/adr/`.
- **breakdown** (architect) — stories and tasks as ticket files, each one repo's work
  in one reviewable PR; features and epics only where grouping helps.
- **publish** (publisher) — sync tickets to a PM tool, then freeze them. A run phase.

Each phase writes its artifact and stops at a gate for human approval.

## Commands (the `zumen` CLI)

| Command | Purpose |
|---|---|
| `zumen init [dir] [--no-git] [--force]` | scaffold this planning ICM structure (`--no-git`: a local workspace, vcs: none; `--force`: overwrite existing files) |
| `zumen update` | refresh managed files to the installed version |
| `zumen status` | show the active change's phase/status |
| `zumen settings` | show/merge settings |
| `zumen switch [--all] [<change>]` | list changes (`--all` includes closed ones), or switch the active change |
| `zumen source new --title <t> [--repo <path>] [--rev <r>] [--body <b>]` | add an intake source to `00_intake/` |
| `zumen source list` | list intake sources |
| `zumen source show <id>` | show an intake source |
| `zumen adr new --title <t> [--deciders <d>] [--context <c>] [--decision <d>] [--consequences <c>] [--file <path>]` | create an ADR in `04_architecture/adr/` (`--file` reads the body from a file) |
| `zumen adr list` | list ADRs |
| `zumen adr show <number>` | show an ADR |
| `zumen change new <title> [--repo <name\|path>]... [--rev <r>] [--body <b>]` | create a change + intake + switch |
| `zumen change close <change> --reason <r>` | close a change |
| `zumen gate <phase> <status> [--reason <r>] [--force]` | advance the active change's gate (earlier phases must be approved or skipped); `skipped` needs a reason |
| `zumen reopen <phase> --reason <r>` | move back to a phase and clear later approvals |
| `zumen tickets new <type> <title> [--parent <id>] [--repo <name>] [--depends <id,id>] [--requirements <id,id>]` | create a ticket in the active change, citing the spec's requirements |
| `zumen tickets check` | validate every ticket: schema, parents, cycles, repos, readiness, drift. Errors (`✗`) exit 1; warnings (`!`) don't |
| `zumen tickets status` | list tickets by draft, ready, published and drifted |
| `zumen tickets order` | list tickets in publish order |
| `zumen tickets ready <id>... \| --all [--force]` | ready draft tickets once breakdown is approved, all or none |
| `zumen tickets mark-published <id> --external <tool>=<item-id> [--force]` | record a PM tool's id and freeze the ticket; needs publish in-progress (`--force` overrides only that), a valid ready mark, and published predecessors |
| `zumen tickets unfreeze <id> --reason <r>` | clear a ticket's freeze, so the next sync updates it |
| `zumen overview [<change>] [--check] [--stdout]` | write or check `specs/<change>/README.md`: the narrative region is yours, the generated region is rewritten; `--check` exits 1 if it's out of date |
| `zumen references check` | warn about unindexed files in `references/` and index entries that lack a file or a field |
| `zumen repo add <name> <remote> [--path <p>] [--branch <b>]` | register a repo this workspace works with |
| `zumen repo list` | list repos and their local paths |
| `zumen repo path <name> <path>` | set a repo's local path |
| `zumen repo doctor [--clone] [--into <dir>]` | check repos; `--clone` fetches missing ones |
| `zumen env` | print harness flags (`--add-dir <path>`) for the active change's repos |
| `zumen check` | validate the embedded scaffold |
| `zumen version` | print the version |
| `zumen help [<command> [<verb>]]` | usage; `zumen <command> --help` (or `-h`) prints one command's usage |

The folders/personas below describe *what* to do; `zumen` is a plain CLI — run
it like `git` or `gh`. Prefer `zumen <command>` over hand-editing `STATE.json` or
`00_intake/` when the binary is on your PATH.

## How to start a change

1. **Create a change** — `zumen change new "Title"` (add `--repo <name>` for
   each registered repo, or `--repo <path>` for a single unregistered one). This writes the intake (`00_intake/<id>.md`), makes
   a `specs/<n>-<slug>/` folder, seeds `STATE.json`, and switches to it.
2. **Kick off** with a prompt like: *"Start the loop on `00_intake/1.md`."*

## Routing table

| Task / Intent | Directory | Persona | Read | Skip |
|---|---|---|---|---|
| Define the base source | `00_intake/` | — | `00_intake/context.md` | `specs/*` |
| External research | `01_research/` | researcher | `01_research/context.md`, `00_intake/<id>.md` | `specs/*`, the repos |
| Explore the repos | `02_explore/` | explorer | `02_explore/context.md`, the repos (`zumen repo list`) | `specs/*` |
| Synthesize findings | `02_explore/` | architect | `02_explore/context.md`, `02_explore/<n>-<slug>/*` | `specs/*` |
| Write the spec | `03_spec/` | analyst | `03_spec/context.md`, `00_intake/*`, `01_research/<n>-<slug>/*`, `02_explore/<n>-<slug>/*` | the repos |
| Design the architecture | `04_architecture/` | architect | `04_architecture/context.md`, `specs/<n>-<slug>/functional-spec.md`, `02_explore/<n>-<slug>/*` | the repos |
| Break down into tickets | `05_breakdown/` | architect | `05_breakdown/context.md`, `specs/<n>-<slug>/functional-spec.md`, `specs/<n>-<slug>/architecture.md` | the repos |
| Publish tickets | `06_publish/` | publisher | `06_publish/context.md`, `06_publish/sync.md`, `specs/<n>-<slug>/tickets/*` | the repos, `01_research/*`, `02_explore/*` |

## Naming conventions

- Specs: `specs/<n>-<slug>/functional-spec.md|architecture.md`
- Tickets: `specs/<n>-<slug>/tickets/<ID>.md`, with ids like `S-0014` from one
  workspace-wide sequence
- Intake: `00_intake/<id>.md`
- Research: `01_research/<n>-<slug>/<topic>.md`
- Findings: `02_explore/<n>-<slug>/<repo>.md`, `02_explore/<n>-<slug>/synthesis.md`
- ADRs: `04_architecture/adr/NNNN-<slug>.md`
- Sync instructions: `06_publish/sync.md`, with starters in `06_publish/sync-examples/`
- Personas: `<stage>/personas/<name>.md`
- Templates: `<stage>/templates/<artifact>.md`

## Version control

`vcs` in `.zumen/settings.json` says whether this workspace lives in git: `git`
(the default) or `none`. Under `vcs: git`, zumen commits workspace state as
described below. Under `vcs: none` the workspace is plain local files: zumen
writes the same files but never runs git. It keeps `.gitignore`, so
`.zumen/active` and `.zumen/paths.json` stay out of any enclosing or later git
repo, but writes no `.gitattributes` or `.gitkeep`.

## Workflow rules

- Each phase in order: a phase can't be gated until the earlier ones are approved,
  skipped or complete.
- Each phase stops at a gate for human approval — never proceed past a gate on
  your own. Record transitions with `zumen gate <phase> <status>`. `--force`
  overrides the order check and is logged; only use it when a human says so.
- Read only the files the routing table names for your task; skip the rest.
- Skip a phase with `zumen gate <phase> skipped --reason <r>`. Use `zumen reopen
  <phase> --reason <r>` to go back: it clears the approvals after that phase,
  because they no longer hold. `zumen status` warns if a repo's explore is skipped.
- `zumen change new`, `gate`, `reopen` and `change close` write `STATE.json` and
  `LOG.md`. Under `vcs: git` they also commit them for you, leaving anything else
  you have staged alone, and `gate` commits the change's folders under `specs/`,
  `01_research/` and `02_explore/`, so write the artifact before you gate it. Under
  `vcs: git`, commit ADRs yourself; they aren't per change. They don't push.
- Cite `file:line` when grounding design or findings in a real codebase.

## Tech stack & standards

- <FILL IN: the target systems' stacks, or "greenfield — to be decided in architecture">

## Directory map

```
├── CLAUDE.md
├── AGENTS.md
├── README.md
├── .gitignore            # keeps .zumen/active and paths.json out of git
├── .gitattributes        # vcs: git only
├── STATE.json            # per-change phase, gates, sources (committed under vcs: git)
├── LOG.md                # gate history (committed under vcs: git)
├── REPOS.json            # logical repos and remotes (committed under vcs: git)
├── .zumen/               # settings + manifest (committed under vcs: git), active change + repo paths (local)
├── 00_intake/            # the base source (<id>.md)
├── 01_research/          # researcher (skippable)
├── 02_explore/           # explorer + synthesis (skippable)
├── 03_spec/              # analyst
├── 04_architecture/      # architect + adr/
├── 05_breakdown/         # architect
├── 06_publish/           # publisher + sync.md
└── specs/                # per-change artifacts and tickets (<n>-<slug>/)
```

## Handoff

Tickets are the only interface out of this workspace. They're local markdown until
published; then the PM tool owns them, and the file is a frozen snapshot. A coding
agent picks one up from there, in tomo or any other harness.
