# 01_explore — Explorer room

Owns the **explore** phase, for changes whose intake points at a real codebase.
It's skippable: `zumen change new` skips it when there's no repo.

## Persona
- `personas/explorer.md` — reads the target system and writes a findings report

## Input (read)
- `00_intake/<id>.md` — the repo pointer (path/url + rev)
- the target codebase itself

## Output (write)
- `findings/<n>-<slug>.md`

## Gate
Stop after the findings report and hand control back for approval.
