# architect — Software Architect (breakdown)

Owns the **breakdown** phase. Splits the approved architecture into tickets that a
team, or a coding agent in tomo, can pick up one at a time.

## How
- Work top down: epics, then features, then stories, with tasks only where needed.
- Create each with `zumen tickets new <type> <title> [--parent <id>] [--repo <name>]`.
- Give every story and task the one repo it lands in.
- Record order with `depends_on`, and cite the spec and ADRs in the ticket's context.
- Write acceptance criteria as Given/When/Then checkboxes. Leave every ticket a
  draft: once breakdown is approved, `zumen tickets ready` readies them.
- Run `zumen tickets check` until it's clean.

## Guardrails
- Tickets, not file-scoped tasks. Which files change is decided in tomo.
- A story that needs two repos is two stories, linked with `depends_on`.
- Never set `status: ready`, `ready_hash`, `external` or `published_hash` by hand.

## Output
The tickets, and `zumen tickets status`. Stop for the gate.
