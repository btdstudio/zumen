# Sync instructions: GitHub Issues

A starter for `06_publish/sync.md`. Copy it there, then fill in the settings.

## Settings

- **Tool name:** `github`, used as `external.github`.
- **Planning repo:** `<owner>/<repo>`, for epics and features, which have no `repo`.
- **Stories and tasks** go in their own repo: run `zumen repo list` and turn the
  ticket's `repo` into `<owner>/<repo>`.
- **External id:** `<owner>/<repo>#<number>`.

Type labels are optional. Create any label first, because `gh issue create` fails on
one that doesn't exist:

```bash
gh label create "<label>" --repo <owner>/<repo> --force
```

## Ticket body

Send only the body, not the frontmatter:

```bash
awk 'c>=2; /^---$/{c++}' <ticket-file> > /tmp/ticket-body.md
```

GitHub renders the markdown as is, so nothing needs converting.

## push

For each ticket in `zumen tickets order` sequence:

| The ticket | Do |
|---|---|
| `status: draft` | Skip it. |
| Its parent, a `depends_on` ticket or its `supersedes` target has no `external.github` | Don't create or update it. Report it as **blocked**, naming that ticket. |
| No `external.github` | Create it, as below. |
| Has the id, and no `published_hash` | Update it, as below, then re-freeze with `mark-published`. |
| Has the id and a `published_hash` | Skip it. |

Create, keeping the title in the `[id] title` form so an item can be traced back:

```bash
gh issue create --repo <owner>/<repo> --title "[<id>] <title>" --body-file /tmp/ticket-body.md
zumen tickets mark-published <id> --external github=<owner>/<repo>#<number>
```

`gh issue create` prints the new issue's URL, which ends in the number. Run
`mark-published` only if the create succeeded: it can't tell a real id from an empty
one, and it freezes whatever it's given.

If `mark-published` refuses, stop the run. Report its output and the new issue's
URL: the issue exists, but its id isn't recorded, so a human decides whether to
record it or close it. Don't retry with `--force`.

Update:

```bash
gh issue edit <number> --repo <owner>/<repo> --title "[<id>] <title>" --body-file /tmp/ticket-body.md
zumen tickets mark-published <id> --external github=<owner>/<repo>#<number>
```

## fetch

```bash
gh issue view <number> --repo <owner>/<repo> --json title,body,state,labels,comments
```

## link

**Parent/child.** Sub-issues take the child's database id, not its number. They work
across repos in the same owner, and an issue can have only one parent:

```bash
child_id=$(gh api repos/<owner>/<child-repo>/issues/<child-number> --jq .id)
gh api repos/<owner>/<parent-repo>/issues/<parent-number>/sub_issues -F sub_issue_id="$child_id"
```

**Predecessor (`depends_on`).** Mark this ticket as blocked by the other:

```bash
blocker_id=$(gh api repos/<owner>/<blocker-repo>/issues/<blocker-number> --jq .id)
gh api repos/<owner>/<repo>/issues/<number>/dependencies/blocked_by -F issue_id="$blocker_id"
```

**Supersedes.** Point the old item at the new one, and close it:

```bash
gh issue close <old-number> --repo <owner>/<old-repo> --reason "not planned" --comment "Superseded by <owner>/<repo>#<number>"
```

A link that already exists returns an error: "may not contain duplicate sub-issues"
for a sub-issue, and "has already been taken" for a dependency. On a re-run, treat
those as done and carry on.
