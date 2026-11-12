# ideas — ideas room

Where a half-formed idea is worked through before anything is decided. An idea isn't
a change: it has no gates and no phases, and nothing about it goes in `STATE.json`.
Open one when you aren't ready for an intake, and promote it when you are.

## Layout

One folder per idea, `ideas/<n>-<slug>/`. Ids are sequential and never reused while
a folder exists.

- `summary.md` — frontmatter (`id`, `title`, `status`, `reason`, `updated`,
  `changes`), then sections for the problem, options considered, cuts and no-gos,
  open questions, references cited and resulting changes. You can edit it by hand:
  `zumen idea` rewrites only the frontmatter lines it owns.
- `log.md` — append-only. Session entries use the shape in `templates/log-entry.md`,
  and `zumen idea` adds a dated line for each status change.
- Any other file in the folder is an attachment.

## Status

An idea is `open`, `parked`, `dropped` or `promoted`. Ideas are dropped, never
deleted.

- `zumen idea park <id> --reason <r>` and `zumen idea drop <id> --reason <r>` need an
  open idea and a reason.
- `zumen idea reopen <id> --reason <r>` brings a parked or dropped idea back.
- `zumen idea list` shows every idea with its status and reason.

## Working an idea

Open `personas/facilitator.md`, then read the idea's `summary.md` and `log.md` and
`references/index.md`. The facilitator works in a fixed order and writes the
session's entry in the log before it stops.

## Promoting

`zumen idea promote <id> --title <t> [--repo <name>]...` creates a change the way
`zumen change new` does, and its intake is the idea's summary. An idea can be
promoted more than once, to split it, and every change is listed in `changes`. The
summary and the log stay readable afterwards. Promote only when the person asks.

## Templates

`zumen idea new` uses a built-in copy of `templates/summary.md`, as `zumen tickets
new` does with its ticket template, so editing the file here doesn't change new
ideas. `templates/log-entry.md` is the shape of a session entry.

## Chat output

Thinking toward an idea, such as a session with an AI chat, goes in that idea's log.
Material you use as a source goes in `references/`, with a `from` that starts with
`AI chat`.
