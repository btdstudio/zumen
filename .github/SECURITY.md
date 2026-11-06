# Security Policy

## Supported versions

zumen is pre-1.0. Security fixes are applied to the latest commit on `main`.
zumen is distributed as a single binary:
`go install github.com/btdstudio/zumen/cmd/zumen@latest`.

## Reporting a vulnerability

Please do not open a public issue for security problems.

Report privately through GitHub: go to the
[Security tab](https://github.com/btdstudio/zumen/security/advisories/new) and
choose **Report a vulnerability**.

Include what you found, how to reproduce it, and the affected version or
commit. This is a small project maintained in spare time; you can expect an
acknowledgement within a few days. Once a fix is available, the advisory will
be published and you will be credited unless you prefer otherwise.

## Scope

In scope: the `zumen` commands (`init`, `update`, `export`, and the workspace
helpers), the embedded scaffold, and the workspace files it creates (`.zumen/`,
`STATE.json`, `LOG.md`).

Out of scope: vulnerabilities in third-party dependencies with no impact on
zumen (report those upstream), and the contents of a user's own project.
