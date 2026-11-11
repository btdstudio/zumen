# Sync instructions

**Not configured yet.** Replace this file with instructions for your PM tool. Start
from a file in `sync-examples/`. `zumen update` won't overwrite your edits.

Whatever the tool, this file must define three verbs:

## push

For each ticket, in `zumen tickets order` sequence, using `<tool>` as this PM tool's
name in `external`:

| The ticket | Do |
|---|---|
| `status: draft` | Skip it. It hasn't been readied for publishing. |
| Its parent, a `depends_on` ticket or its `supersedes` target has no `external.<tool>` | Don't create or update it. Report it as **blocked**, naming that ticket. |
| No `external.<tool>` | Create the item, then run `zumen tickets mark-published <id> --external <tool>=<item-id>`. |
| Has the id, and no `published_hash` (unfrozen) | Update the item, then run `mark-published` with the same id to re-freeze. |
| Has the id and a `published_hash` (frozen) | Skip it. |

The rows are checked in order, so a blocked ticket is never created.

If `mark-published` refuses, stop the run. Report what it printed, and the id or
link of any item you just created for that ticket, so a human can record or remove
it. Don't retry with `--force`.

Re-running push is always safe, because finished tickets are skipped.

## fetch

Read the live item by its external id. tomo intake and drift review use this.

## link

Create the relations, each of which needs the other item's id: parent/child,
predecessor (`depends_on`), and `supersedes`. A superseded item is also linked to its
replacement or closed.
