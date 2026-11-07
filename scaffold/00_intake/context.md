# 00_intake — Base source

Holds the base source for each change: a prose issue/idea, or a pointer to an
existing codebase to plan against.

## Files
- `<id>.md` — one per source (created by `zumen source new`)

## Front matter
```yaml
---
source: issue          # or: repo
repo: <path/url>       # only when source: repo
rev: <branch/sha>      # optional pin
---
```

## When `source: repo`
The loop runs `explore` before `requirements`. `zumen change new --repo` sets
this up. If a change started without a repo, bring explore back with
`zumen reopen explore --reason <r>`.

## Gate
Stop after defining the source and hand control back for approval.
