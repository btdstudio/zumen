# references — reference material

Files that inform the work but aren't produced by it: notes, designs, screenshots,
exported chats, papers. Put any type of file here, in subfolders if you like. Any
phase may open a file the index lists.

## The index

`index.md` is yours: zumen never creates, edits or overwrites it. It lists what's in
this folder, and why each file matters, so an agent can decide whether to open it.

One list item per reference. Start with the path relative to this folder, in
backticks, then a dash and a title. Indent the fields under it:

```
- `design/zumen-tomo-design.md` — Zumen + Tomo design v2
  - purpose: what it is for
  - from: who or what produced it
  - added: 2026-11-12
  - summary: optional; required when an agent may not be able to read the format
```

- `purpose`, `from` and `added` are required.
- `from` says who or what produced the file. For output from an AI chat, start it
  with `AI chat` and say which session or date, such as `AI chat, 2026-11-10`.
  Material from a chat session that you use as a source goes here, marked that way.
- `summary` is optional, but required when an agent may not be able to read the
  format, such as an image, a scan or a binary file. Write what a reader would need
  from it.
- Anything else in `index.md`, such as headings and prose, is ignored.

## Check

`zumen references check` warns about each file here that the index doesn't list,
each entry whose file is missing, and each entry missing a required field. It
ignores `index.md`, `context.md` and files whose names start with a dot, and it
never fails: warnings don't change the exit code.
