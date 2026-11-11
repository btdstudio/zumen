# Architecture — <short name>

- **Status:** draft | await-approval | approved
- **Date:** <ISO date>
- **Designing for:** `specs/<n>-<slug>/functional-spec.md`

## Overview

<Three to five sentences: the shape of the solution and why it's the right one.>

## Diagrams

Draw these first; the prose below must agree with them. Use standard `mermaid`
fences, about 15 nodes per diagram (split a bigger one rather than shrink it),
and real names on nodes. Every repo in the change appears in Layers. Add a
`### Flow: <name>` per main flow, and `### Deployment` or `### Data` when they
help. Write generics and raw tags in backticks, or the gate reads them as
unfilled slots.

### Context

<The system, its users and the external systems it talks to.>

```mermaid
```

### Layers

<Each repo in the change and the layers inside it. Every repo appears here.>

```mermaid
```

### Flow: <name of the main flow>

<The steps of this flow, across the layers above.>

```mermaid
```

## Context

<Current codebase state relevant to this change; cite `file:line` where useful.>

## Design decisions

- D1. <decision> — <rationale> (<alternatives rejected in brief>)
- D2. ...

## Architecture / components

<Components, responsibilities, and how they connect. The implementer's map.>

## Data flow / behavior

<Step-by-step behavior for the main flows, aligned to the acceptance criteria.>

## Interfaces

<Module boundaries, function/CLI/API shapes, config, env vars.>

## Out of scope (deferred)

- <what the design deliberately leaves out>

## Open items / risks

- <assumptions, endpoints to confirm, risks with their impact>

## Notes

<references, constraints, implementation hints.>
