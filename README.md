# Zumen (図面) 📐 — folder-driven planning workspace

Zumen scaffolds and maintains an Interpretable Context Methodology (ICM)
workspace for **planning initiatives**: work that takes weeks, often spans many
repos, and needs research and architecture before anyone writes code. It ends in
tickets. Where [tomo](https://github.com/btdstudio/tomo) executes one change in one
repo, zumen decides what those changes should be. The filesystem is the
architecture; no framework or runtime is required after scaffold.

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
| `zumen init [dir] [--no-git]` | scaffold the planning ICM structure; `--no-git` makes a local workspace (`vcs: none`) |
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
| `zumen tickets new\|check\|status\|order\|ready\|mark-published\|unfreeze` | manage tickets |
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
skipped. It also checks the phase's artifact when you approve spec, architecture
or breakdown, or complete publish:

- the spec has at least one `R` requirement;
- `architecture.md` has a `## Diagrams` section with Context, Layers and a Flow,
  each a non-empty `mermaid` block, and no unfilled template slots;
- the change's tickets exist and pass validation;
- every ready or published ticket is frozen.

Each problem names its file, and the line where there is one. `--force` overrides
a refusal and records each overridden problem in `LOG.md`; `prompt` and `auto` warn
instead. In a git workspace (the default), state changes commit `STATE.json`
and `LOG.md` for you, without pushing, and gates also commit the change's folders
under `specs/`, `01_research/` and `02_explore/`. A local workspace skips the commits;
see [Version control](#version-control).

## The workspace

```
├── AGENTS.md          root map + routing table + command list
├── STATE.json         per-change phase, gates, sources (committed under vcs: git)
├── LOG.md             gate history (committed under vcs: git)
├── REPOS.json         logical repos and remotes (committed under vcs: git)
├── .zumen/            settings + manifest (committed under vcs: git), active change + repo paths (local)
├── 00_intake/         base source (<id>.md)
├── 01_research/       researcher (skippable)
├── 02_explore/        explorer + synthesis (skippable)
├── 03_spec/           analyst
├── 04_architecture/   architect + adr/
├── 05_breakdown/      architect
├── 06_publish/        publisher + sync.md
└── specs/             per-change artifacts and tickets (<n>-<slug>/)
```

## Version control

The `vcs` setting says whether a workspace lives in git. Set it in
`.zumen/settings.json`, or globally in `~/.config/zumen/settings.json`.

| Value | Behavior |
|---|---|
| `git` (default) | `init` runs `git init` and writes `.gitignore`, `.gitattributes` and `.gitkeep` placeholders; state changes commit. Outside a git repo they warn, with a hint to set `"vcs": "none"`. |
| `none` | The workspace is plain local files: no `git init`, no commits and no warning. `.gitignore` is kept so `.zumen/active` and `.zumen/paths.json` stay out of any enclosing or later git repo; `.gitattributes` and `.gitkeep` are left out. `STATE.json` and `LOG.md` are still written. |

`zumen init --no-git`, or a global `"vcs": "none"`, creates a local workspace and
records `"vcs": "none"` in its settings. `zumen status` shows the mode. To switch a
workspace later, change `vcs` and run `zumen update`. Going to `git` restores the git
files, and you run `git init` yourself. Going to `none` leaves existing git files on
disk; zumen keeps managing `.gitignore` and stops managing the rest.

The setting covers the planning workspace only. The repos in `REPOS.json` are always
git checkouts.

## Tickets

Tickets are the only interface out of a workspace. Each is one markdown file,
`specs/<n>-<slug>/tickets/<ID>.md`, local until it's published. Tickets start as
drafts. Once breakdown is approved, `zumen tickets ready` readies them, and
`mark-published` freezes only a ready ticket whose parent and predecessors are
already published. Once a ticket is published, the PM tool owns it, and the file is
a frozen snapshot: `zumen tickets check` flags any edit as drift. To change a
published ticket, write a new one with `supersedes`, or run `zumen tickets unfreeze`
so the next sync updates the existing item.

Publishing is agent work. `06_publish/sync.md` tells an agent how to talk to your PM
tool, and the CLI never calls one. A GitHub Issues starter ships in
`06_publish/sync-examples/`; copy it to `sync.md` and fill in its settings. ADO and
Jira starters are planned.

## Handing off to a coding agent

A coding agent picks a ticket up from there, in any harness. In
[tomo](https://github.com/btdstudio/tomo):

```sh
tomo change new --from ../planning/specs/1-release/tickets/S-0014.md
```

That copies the ticket in as a snapshot and starts a change. Approvals don't carry
over, because tomo approves something different: how this one ticket lands in this
one repo. Neither tool needs the other to run; the ticket format is all they share,
through [kiso](https://github.com/btdstudio/kiso).

## Development

```sh
go build -o zumen ./cmd/zumen   # build
go test ./...                    # all tests
go test ./internal/icm -update  # regenerate golden fixtures
```

Runtime: Go >= 1.22, standard library only, plus the first-party
[kiso](https://github.com/btdstudio/kiso) core.
