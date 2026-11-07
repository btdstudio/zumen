# 06_publish — Publisher room

Owns the **publish** phase: syncing tickets to a PM tool, then freezing them. It's a
run phase: `zumen gate publish in-progress`, then `complete`.

The CLI never calls a PM tool. An agent does, following `sync.md`.

## Persona
- `personas/publisher.md` — pushes tickets and records what it did

## Input (read)
- `sync.md` — how this workspace talks to its PM tool
- `specs/<n>-<slug>/tickets/*.md`
- `zumen tickets order` — the sequence to publish in

## Output (write)
- items in the PM tool
- each ticket's `external` id and `published_hash`, through
  `zumen tickets mark-published`

## Gate
Report what was created, updated and skipped, then stop.
