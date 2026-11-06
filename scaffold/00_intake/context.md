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
Set the change's `mode` to `existing` in `STATE.json` so the loop runs
`explore` before `requirements`.

## Gate
Stop after defining the source and hand control back for approval.
