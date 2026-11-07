# <project> — planning ICM workspace

Layer 1 root map (Interpretable Context Methodology). Every agent reads this
file first, then navigates by the routing table below. `CLAUDE.md` just points
here; this file is harness-agnostic and the single source of truth.

## The loop

A change flows through five gated phases, each owned by a persona. The
`explore` phase is skippable: `zumen change new` skips it unless the source is a
real codebase, and you can skip it or bring it back yourself.

`intake → [explore] → requirements → design → plan`

- **intake** — define the base source: a prose issue/idea, or a pointer to an
  existing codebase (repo path/url + rev).
- **explore** *(skippable)* — read the target system and write a
  findings report (system map, entry points, module responsibilities, data flow,
  cited `file:line`, debt/risk observations). Observation only; no prescription.
- **requirements** (analyst) — turn the source into unambiguous, verifiable
  requirements with Given/When/Then acceptance criteria.
- **design** (architect) — the smallest solution: decisions with rationale and
  rejected alternatives, component map, interfaces, data flow. Record durable
  decisions as ADRs under `03_design/adr/`.
- **plan** (architect) — dependency-ordered, file-scoped tasks in `tasks.json`.

Each phase writes its artifact and stops at a gate for human approval.

## Commands (the `zumen` CLI)

| Command | Purpose |
|---|---|
| `zumen init [dir]` | scaffold this planning ICM structure |
| `zumen update` | refresh managed files to the installed version |
| `zumen status` | show the active change's phase/status |
| `zumen switch [--all] [<change>]` | list changes (`--all` includes closed ones), or switch the active change |
| `zumen source new\|list\|show` | manage `00_intake/` |
| `zumen adr new\|list\|show` | manage `03_design/adr/` |
| `zumen change new <title> [--repo <name>]...` | create a change + intake + switch |
| `zumen change close <change> --reason <r>` | close a change |
| `zumen gate <phase> <status> [--reason <r>] [--force]` | advance the active change's gate (earlier phases must be approved or skipped); `skipped` needs a reason |
| `zumen reopen <phase> --reason <r>` | move back to a phase and clear later approvals |
| `zumen export [<change>] --to <tomo> [--force]` | copy a spec with an approved plan into a tomo workspace |
| `zumen repo add\|list\|path\|doctor` | manage the repos this workspace works with |
| `zumen env` | print harness flags (`--add-dir <path>`) for the active change's repos |
| `zumen settings` | show/merge settings |
| `zumen help` | usage |

The folders/personas below describe *what* to do; `zumen` is a plain CLI — run
it like `git` or `gh`. Prefer `zumen <command>` over hand-editing `STATE.json` or
`00_intake/` when the binary is on your PATH.

## How to start a change

1. **Create a change** — `zumen change new "Title"` (add `--repo <name>` for
   each registered repo, or `--repo <path>` for a single unregistered one). This writes the intake (`00_intake/<id>.md`), makes
   a `specs/<n>-<slug>/` folder, seeds `STATE.json` (explore starts
   skipped unless you passed `--repo`), and switches to it.
2. **Kick off** with a prompt like: *"Start the loop on `00_intake/1.md`."*

## Routing table

| Task / Intent | Directory | Persona | Read | Skip |
|---|---|---|---|---|
| Define the base source | `00_intake/` | — | `00_intake/context.md` | `specs/*` |
| Explore an existing codebase | `01_explore/` | explorer | `01_explore/context.md`, the target repo (see `00_intake/<id>.md`) | `specs/*` |
| Write requirements | `02_requirements/` | analyst | `02_requirements/context.md`, `00_intake/*`, `01_explore/findings/*` (if explore ran) | target repo |
| Design solution | `03_design/` | architect | `03_design/context.md`, `specs/<n>-<slug>/requirements.md`, `01_explore/findings/*` (if explore ran) | target repo |
| Plan tasks | `04_plan/` | architect | `04_plan/context.md`, `specs/<n>-<slug>/design.md` | target repo |

## Naming conventions

- Specs: `specs/<n>-<slug>/requirements.md|design.md|tasks.json`
- Intake: `00_intake/<id>.md`
- Findings: `01_explore/findings/<n>-<slug>.md`
- ADRs: `03_design/adr/NNNN-<slug>.md`
- Personas: `<stage>/personas/<name>.md`
- Templates: `<stage>/templates/<artifact>.md`

## Workflow rules

- Intake before requirements; explore (unless skipped) before requirements;
  requirements before design; design before plan.
- Each phase stops at a gate for human approval — never proceed past a gate on
  your own. Record transitions with `zumen gate <phase> <status>`. `--force`
  overrides the order check and is logged; only use it when a human says so.
- Read only the files the routing table names for your task; skip the rest.
- Skip a phase with `zumen gate <phase> skipped --reason <r>`. Use `zumen reopen
  <phase> --reason <r>` to go back: it clears the approvals after that phase,
  because they no longer hold. `zumen status` warns if a repo's explore is skipped.
- `zumen change new`, `gate`, `reopen` and `change close` commit `STATE.json` and
  `LOG.md` for you, leaving anything else you have staged alone. They don't push.
- Cite `file:line` when grounding design or findings in a real codebase.

## Tech stack & standards

- <FILL IN: the target system's stack, or "greenfield — to be decided in design">

## Directory map

```
├── CLAUDE.md
├── AGENTS.md
├── README.md
├── .gitignore
├── .gitattributes
├── STATE.json            # per-change phase, gates, sources (committed)
├── LOG.md                # gate history (committed)
├── REPOS.json            # logical repos and remotes (committed)
├── .zumen/               # settings + manifest (committed), active change + repo paths (local)
├── 00_intake/            # the base source (<id>.md)
├── 01_explore/           # explorer (skippable)
├── 02_requirements/      # analyst
├── 03_design/            # architect + adr/
├── 04_plan/              # architect
└── specs/                # per-change artifacts (<n>-<slug>/)
```

## Handoff

`specs/<n>-<slug>/` is portable. To build the plan, run
`zumen export <n>-<slug> --to <tomo-workspace>` — it copies the spec into the
tomo workspace's `01_planning/specs/` so tomo's implement phase can consume it.
The contract is the artifact shape, not any integration.
