# zumen — folder-driven planning workspace tool

This file applies to agents working on the zumen repo itself.

## Non-negotiable rules

- Go >= 1.22, standard library only — no third-party runtime or build deps.
  `go test ./...` must stay green from a clean checkout.
- The ICM scaffold is the embedded content store: `scaffold/`, embedded via
  `//go:embed all:scaffold` (`embed.go`). Edit the scaffold there; `zumen check`
  and the golden test (`internal/icm/golden_test.go`) pin it. After editing the
  scaffold, run `go test ./internal/icm -update` to regenerate the golden.
- Workspace state is committed so approvals survive a clone: `STATE.json`,
  `LOG.md`, `.zumen/settings.json`, `.zumen/manifest.json`. Only `.zumen/active`
  (the current change) is local and gitignored — never commit it.

## Layout

```
scaffold/           the embedded planning ICM template (copied out by `zumen init`)
embed.go            //go:embed all:scaffold
cmd/zumen/          the CLI dispatch (init, update, check, helpers)
internal/icm/       the scaffold/update/check engine
internal/state/     STATE.json phase machine, LOG.md, active-change pointer
internal/settings/  mergeable settings (global ← project)
internal/source/    intake store (frontmatter, sequential ids)
internal/adr/       ADR store (numbered, immutable)
internal/export/    copy a spec into a tomo workspace
internal/version/   version reporting (build info)
internal/cli/       helper subcommands (status, settings, switch, source, adr, change, gate)
testdata/golden/    byte-identity fixtures (icm/manifest.json)
```

## Process

- Intake before requirements; explore (existing mode) before requirements;
  requirements before design; design before plan.
- `zumen init` scaffolds a project; `zumen update` refreshes its managed files.

## Development

- `go build -o zumen ./cmd/zumen` — build the binary.
- `go test ./...` — run all tests.
- `go test ./internal/icm -update` — regenerate golden fixtures.
