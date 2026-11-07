# 03_design — Architect room (design)

Owns the **design** phase and the ADR store.

## Persona
- `personas/architect.md` — design (smallest solution) + durable decisions

## Input (read)
- `specs/<n>-<slug>/requirements.md` — approved requirements
- `01_explore/findings/<n>-<slug>.md` — findings (if explore ran)
- `templates/design.md` — skeleton

## Output (write)
- `specs/<n>-<slug>/design.md`
- `adr/NNNN-<slug>.md` — architecture decision records

## Gate
Stop after the design artifact (and any ADRs) and hand control back for approval.
