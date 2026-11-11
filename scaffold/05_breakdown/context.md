# 05_breakdown — Architect room (breakdown)

Owns the **breakdown** phase: turning the architecture into tickets.

## Persona
- `personas/architect.md` — epics, features, stories and tasks

## Input (read)
- `specs/<n>-<slug>/functional-spec.md` and `architecture.md` — approved
- `04_architecture/adr/` — decisions tickets should cite
- `templates/ticket.md` — the ticket format

## Output (write)
- `specs/<n>-<slug>/tickets/<ID>.md` — created with `zumen tickets new`

## Gate
If the breakdown changed the solution's shape, refresh the narrative in
`specs/<n>-<slug>/README.md` first. Run `zumen tickets check`, then stop and hand
control back for approval, with the tickets still drafts. A `!` warning
(`ready, but breakdown for … is "…"`) means breakdown was reopened after those
tickets were readied. Revise them as needed. Once breakdown is approved again,
each one is re-readied by id with `zumen tickets ready <id>`, because `--all`
only readies drafts.
