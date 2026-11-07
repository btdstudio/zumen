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
| `zumen adr new\|list\|show` | manage `04_architecture/adr/` |
| `zumen change new <title> [--repo <name>]...` | create a change + intake + switch |
| `zumen change close <change> --reason <r>` | close a change |
| `zumen gate <phase> <status> [--reason <r>] [--force]` | advance the active change's gate |
| `zumen reopen <phase> --reason <r>` | move back to a phase and clear later approvals |
| `zumen tickets new\|check\|status\|order\|mark-published\|unfreeze` | manage tickets |
| `zumen repo add\|list\|path\|doctor` | manage the repos this workspace works with |
| `zumen env` | print harness flags for the active change's repos |
| `zumen check` | validate the embedded scaffold |
| `zumen version` | print the version |
| `zumen help` | usage |

## The loop

A change is an initiative, often across many repos, and ends in tickets:

1. **intake** — the goal, constraints and repos in scope.
2. **research** (researcher) *(skippable)* — external investigation.
3. **explore** (explorer) *(skippable)* — one findings report per repo.
4. **synthesis** (architect) *(skippable)* — what the findings add up to.
5. **spec** (analyst) — system-level requirements.
6. **architecture** (architect) — the smallest solution + ADRs.
7. **breakdown** (architect) — epics, features, stories and tasks as ticket files.
8. **publish** (publisher) — sync tickets to a PM tool, then freeze them.

Each phase stops at a gate for your approval. With the default `gates: block`
setting, `zumen gate` refuses a phase until the earlier ones are approved or
skipped. `--force` overrides this and records it in `LOG.md`; `prompt` and `auto` warn instead. State changes
commit `STATE.json` and `LOG.md` for you, without pushing, and gates also commit the
change's folders under `specs/`, `01_research/` and `02_explore/`.

## The workspace

```
├── AGENTS.md          root map + routing table + command list
├── STATE.json         per-change phase, gates, sources (committed)
├── LOG.md             gate history (committed)
├── REPOS.json         logical repos and remotes (committed)
├── .zumen/            settings + manifest (committed), active change + repo paths (local)
├── 00_intake/         base source (<id>.md)
├── 01_research/       researcher (skippable)
├── 02_explore/        explorer + synthesis (skippable)
├── 03_spec/           analyst
├── 04_architecture/   architect + adr/
├── 05_breakdown/      architect
├── 06_publish/        publisher + sync.md
└── specs/             per-change artifacts and tickets (<n>-<slug>/)
```

## Handoff

Tickets are the only interface out of a workspace. Publishing them is agent work:
`06_publish/sync.md` tells an agent how to talk to your PM tool, and the CLI never
calls one. A GitHub Issues starter ships in `06_publish/sync-examples/`.
 They're local markdown until
published; then the PM tool owns them, and the file is a frozen snapshot. A coding
agent picks one up from there, in [tomo](https://github.com/btdstudio/tomo) or any
other harness.

## Development

```sh
go build -o zumen ./cmd/zumen   # build
go test ./...                    # all tests
go test ./internal/icm -update  # regenerate golden fixtures
```

Runtime: Go >= 1.22, standard library only.
