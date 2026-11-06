# Planning ICM Tool — Design (v1)

> **Zumen** (図面, "blueprint") — the architecture/planning sibling to tomo
> (友, "friend").

## Purpose

A standalone, folder-driven workspace for the **architecture, design, and
planning** of software work — the upstream half of the SDLC that tomo compresses
into a single `01_planning/` room. Tomo is for *building*; this is for *deciding
what and how to build* before anyone writes code.

## Principles (inherited from ICM)

1. **The filesystem is the architecture.** Markdown personas + templates are the
   instructions; any harness reads them directly. No runtime required after
   `init`.
2. **No skills, no framework.** A Go binary scaffolds and refreshes a tree. The
   markdown does the rest. (A skill layer *could* be added later; it is not part
   of the tool.)
3. **Decoupled from tomo.** No dependency in either direction. The output is
   *artifact-compatible* with tomo's spec folder so it *can* hand off to tomo's
   implement phase — but it can also hand off to any other harness.
4. **Gated.** Every phase stops for human approval. Nothing advances on its own.

## Decisions locked so far

| Decision | Choice |
|---|---|
| Relationship to tomo | Standalone sibling, decoupled |
| Starting point | Both greenfield **and** existing codebase |
| Form factor | CLI + filesystem only (no skills) |
| v1 scope | Core loop only (no roadmap/portfolio) |
| Explore depth | Structured findings report (not full audit) |
| Stack | Go, standard library only |

## The loop

A change flows through four to five gated phases, each owned by a persona:

```
intake ──> [explore] ──> requirements ──> design ──> plan
 source     only when        analyst        architect   architect
 (issue    source = a                       (design)    (plan)
  OR repo)  real codebase
```

- **intake** — define the base source: a prose issue/idea *or* a pointer to an
  existing codebase (repo path/url + rev).
- **explore** *(existing-codebase mode only)* — read the target system and write a
  findings report (system map, entry points, module responsibilities, data flow,
  cited `file:line`, debt/risk observations). Does not prescribe changes.
- **requirements** *(analyst)* — turn the source into unambiguous, verifiable
  requirements (Given/When/Then acceptance criteria).
- **design** *(architect)* — the smallest solution: decisions with rationale and
  rejected alternatives, component map, interfaces, data flow. Record durable
  decisions as ADRs.
- **plan** *(architect)* — dependency-ordered, file-scoped tasks in `tasks.json`.

Every phase writes its artifact and stops at a gate.

## Workspace tree

```
├── AGENTS.md                    root map + routing table + command list
├── CLAUDE.md                    points to AGENTS.md
├── .gitignore                   ignores .zumen/active
├── .gitattributes               LOG.md merge=union
├── STATE.json                   per-change phase machine (committed)
├── LOG.md                       gate history (committed)
├── .zumen/                      settings + manifest (committed), active (local)
├── 00_intake/                   the base source
│   ├── context.md
│   └── <id>.md                  issue/idea OR repo pointer (see below)
├── 01_explore/                  [existing-codebase mode only]
│   ├── context.md
│   ├── personas/explorer.md
│   ├── templates/findings.md
│   └── findings/<n>-<slug>.md
├── 02_requirements/
│   ├── context.md
│   ├── personas/analyst.md
│   └── templates/requirements.md
├── 03_design/
│   ├── context.md
│   ├── personas/architect.md
│   ├── templates/design.md
│   ├── templates/adr.md
│   └── adr/                     architecture decision records
│       └── 0001-<slug>.md
├── 04_plan/
│   ├── context.md
│   ├── personas/architect.md
│   └── templates/tasks.md
├── specs/<n>-<slug>/            one folder per change, accumulates artifacts
│   ├── requirements.md
│   ├── design.md
│   └── tasks.json
└── README.md
```

Mirrors tomo's layout deliberately: the handoff seam is a spec folder with the
same shape as tomo's `01_planning/specs/<n>-<slug>/`.

## Intake (the "both starting points" mechanic)

`00_intake/<id>.md` carries front matter that selects the mode:

```yaml
---
source: issue          # or: repo
repo: ~/code/myapp     # only when source: repo
rev: main              # optional pin
---
```

- `source: issue` → greenfield; the loop runs `intake → requirements → …`.
- `source: repo` → existing-codebase; the loop inserts `explore` after intake,
  and `01_explore/findings/<n>-<slug>.md` becomes an input to the analyst.

## Artifacts

### `requirements.md`
Reuses tomo's template: Problem/Goal, In/Out scope, functional (R1…, must/should/
could) and non-functional (N1…) requirements, Given/When/Then acceptance criteria,
clarifying questions & resolutions, open questions.

### `design.md`
Reuses tomo's template: Overview, Context (cite `file:line`), design decisions
(D1… with rationale + rejected alternatives), architecture/components, data flow,
interfaces, out of scope (deferred), open items/risks.

### `tasks.json`
```json
{
  "change": "1-auth-flow",
  "source": "00_intake/1.md",
  "generatedBy": "architect",
  "tasks": [
    {
      "id": "T1",
      "title": "…",
      "files": ["path/to/file.go"],
      "dependsOn": [],
      "acceptance": ["AC1", "AC2"],
      "estimate": "S | M | L"
    }
  ]
}
```
Dependency-ordered; each task names the files it touches and its dependencies.

### `findings.md` (explore output)
System map, entry points, module responsibilities, data flow, debt/risk
observations — all cited to `file:line`. Observation only, no prescription.

### `adr/0001-<slug>.md`
```
# 0001 — <title>
- Status: proposed | accepted | superseded
- Date: <ISO>
- Deciders: <personas/people>
## Context
## Decision
## Consequences
```

## CLI commands

| Command | Purpose |
|---|---|
| `init [dir] [--no-git] [--force]` | scaffold the planning ICM workspace |
| `update` | refresh managed files, preserving user edits (manifest hashes) |
| `status` | show the active change's mode + phase |
| `switch <change>` | switch the active change |
| `source new\|list\|show` | manage `00_intake/` |
| `adr new\|list\|show` | manage `03_design/adr/` |
| `change new <title> [--repo <path>]` | create a change + intake + switch (derives mode) |
| `gate <phase> <status> [--force]` | advance the active change's gate; earlier phases must be approved |
| `export [<change>] --to <tomo> [--force]` | copy a spec with an approved plan into a tomo workspace's `01_planning/specs/`; won't overwrite without `--force` |
| `settings` | show/merge settings |
| `help` | usage |

Deliberately **no** `add`/`commit`/`pr`/build commands — those belong to tomo's
build side. The planning tool ends at a plan.

## State machine

`STATE.json` is committed so approvals and modes survive a clone. It holds no
per-checkout data: the active change is a one-line pointer in the gitignored
`.zumen/active`, so checkouts working on different changes don't conflict.

```json
{
  "changes": {
    "1-auth-flow": {
      "phase": "requirements",
      "status": "await-approval",
      "mode": "existing",
      "gates": { "intake": "approved", "explore": "approved", "requirements": "await-approval", "design": "", "plan": "" },
      "sources": ["00_intake/1.md"],
      "updatedAt": "…"
    }
  }
}
```

## Settings

`.zumen/settings.json` (committed; mergeable global ← project), minimal for v1:
`baseSource`, `gates` (`block`, `prompt` or `auto`), `sourceDir`, `artifactDir`,
`adrDir`. Mirrors tomo's
`internal/settings`.

## Handoff seam (the decoupling contract)

`specs/<n>-<slug>/` is the portable output. `zumen export <n>-<slug> --to <tomo>`
copies it into a tomo workspace's `01_planning/specs/`; start tomo's implement
phase from there. No code linkage — the contract is the artifact *shape*
(requirements.md / design.md / tasks.json), which both tools already share by
construction.

## Out of scope (v1)

- Roadmap/portfolio — multi-change sequencing, cross-change dependencies.
- Estimation depth — a single `estimate` field only; no methodology.
- Full architecture audit — explore is a findings report, not a dep-graph tool.
- Skills layer — the scaffold is the deliverable.
- Building/implementing — that's tomo.

## Repo layout (this tool's own source)

Mirrors tomo:

```
cmd/<name>/main.go      CLI dispatch
embed.go                //go:embed all:scaffold
internal/icm/           scaffold/update/manifest engine
internal/state/         phase machine
internal/settings/      mergeable settings
internal/source/        intake store (frontmatter, sequential ids)
internal/adr/           ADR store (numbered, immutable)
internal/version/       build info
internal/cli/           status/switch/settings helpers
scaffold/               the embedded planning ICM template
testdata/golden/        byte-identity fixtures
```

## Resolved

- Name: **zumen**.
- `04_plan` reuses the **architect** persona (as tomo does).
- ADRs live under `03_design/adr/`.
