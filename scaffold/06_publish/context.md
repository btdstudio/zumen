# 06_publish — Publisher room

Owns the **publish** phase: readying the approved tickets, syncing them to a PM tool,
then freezing them. It's a run phase. Start with `zumen tickets ready --all`, which
needs breakdown approved, then `zumen gate publish in-progress`, and end with
`complete`.

The CLI never calls a PM tool. An agent does, following `sync.md`.

## Persona
- `personas/publisher.md` — pushes tickets and records what it did

## Input (read)
- `sync.md` — how this workspace talks to its PM tool
- `specs/<n>-<slug>/tickets/*.md`
- `zumen tickets order` — the sequence to publish in

## Output (write)
- items in the PM tool
- each ticket's `status: ready` and `ready_hash`, through `zumen tickets ready --all`
- each ticket's `external` id and `published_hash`, through
  `zumen tickets mark-published`

## Gate
Report what was created, updated and skipped, then stop.
