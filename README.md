# Zumen (図面) 📐 — folder-driven planning workspace

Zumen scaffolds and maintains an Interpretable Context Methodology (ICM)
workspace for the **architecture, design, and planning** of software work — the
upstream half of the SDLC. Where [tomo](https://github.com/btdstudio/tomo) is
for *building*, zumen is for *deciding what and how to build* before code
exists. The filesystem is the architecture; no framework or runtime is required
after scaffold.

## Attribution

The Interpretable Context Methodology (ICM) that zumen implements is inspired
by:

- **Jake Van Clief**, **David McDermott** — ["Interpretable Context Methodology:
  Folder Structure as Agentic Architecture"](https://arxiv.org/abs/2603.16021)

## Install

```sh
go install github.com/btdstudio/zumen/cmd/zumen@latest
```

## Commands

| Command | Purpose |
|---|---|
| `zumen init [dir]` | scaffold the planning ICM structure |
| `zumen update` | refresh managed files to the installed version |
| `zumen status` | show the active change's phase/status |
| `zumen settings` | show/merge settings |
| `zumen switch [--all] [<change>]` | list changes, or switch the active change |
| `zumen source new\|list\|show` | manage `00_intake/` |
| `zumen adr new\|list\|show` | manage `03_design/adr/` |
| `zumen change new <title> [--repo]` | create a change + intake + switch |
| `zumen change close <change> --reason <r>` | close a change |
| `zumen gate <phase> <status> [--reason <r>] [--force]` | advance the active change's gate |
| `zumen reopen <phase> --reason <r>` | move back to a phase and clear later approvals |
| `zumen export [<change>] --to <tomo> [--force]` | copy a spec with an approved plan into a tomo workspace |
| `zumen check` | validate the embedded scaffold |
| `zumen version` | print the version |
| `zumen help` | usage |

## The loop

A change flows through five gated phases, each owned by a persona:

1. **intake** — define the base source (an issue/idea, or a repo pointer).
2. **explore** *(skippable)* — findings report for a real codebase.
3. **requirements** (analyst) — turn the source into an unambiguous spec.
4. **design** (architect) — the smallest solution + ADRs.
5. **plan** (architect) — dependency-ordered tasks.

Each phase stops at a gate for your approval. With the default `gates: block`
setting, `zumen gate` refuses a phase until the earlier ones are approved or
skipped, and
`zumen export` refuses a change whose plan isn't approved. `--force` overrides
this and records it in `LOG.md`; `prompt` and `auto` warn instead. State changes
commit `STATE.json` and `LOG.md` for you, without pushing.

## The workspace

```
├── AGENTS.md          root map + routing table + command list
├── STATE.json         per-change phase, gates, sources (committed)
├── LOG.md             gate history (committed)
├── .zumen/            settings + manifest (committed), active change (local)
├── 00_intake/         base source (<id>.md)
├── 01_explore/        explorer (skippable)
├── 02_requirements/   analyst
├── 03_design/         architect + adr/
├── 04_plan/           architect
└── specs/             per-change artifacts (<n>-<slug>/)
```

## Handoff

`specs/<n>-<slug>/` is portable. Run `zumen export <n>-<slug> --to <tomo>` to
copy it into a tomo workspace's `01_planning/specs/`, then run tomo's implement
phase — or feed it to any other harness.

## Development

```sh
go build -o zumen ./cmd/zumen   # build
go test ./...                    # all tests
go test ./internal/icm -update  # regenerate golden fixtures
```

Runtime: Go >= 1.22, standard library only.
