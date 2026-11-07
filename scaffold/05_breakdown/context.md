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
Run `zumen tickets check`, then stop and hand control back for approval.
