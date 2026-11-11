# Architecture — Synthetic fixture

- **Status:** approved
- **Date:** 2026-01-01
- **Designing for:** a hand-written fixture that mirrors the layout of a real architecture

## Overview

A short overview, with a sketch that isn't one of the diagrams:

```mermaid
flowchart LR
  overview --> notcounted
```

## Diagrams

Each diagram below is a mermaid source block.

### Context

Who uses the system, and what it talks to.

```mermaid
flowchart LR
  user([User]) --> app[App]
  app --> store[(Store)]
```

### Layers

The packages, top to bottom.

```text
a plain text block before the diagram
```

```mermaid
flowchart TB
  cli --> state
  state --> plan
  plan --> adr
```

### Flow: approving a gate

```mermaid
sequenceDiagram
  participant U as User
  participant C as CLI
  U->>C: gate approve
  C-->>U: ok
```

What happens when the check fails is described in the text below the diagram.

### Flow: regenerating the overview

```mermaid
sequenceDiagram
  participant S as State
  participant O as Overview
  S->>O: refresh
```

### Flow: tomo PR link

```mermaid
flowchart LR
  ticket --> pr
```

## Context

Some context after the diagrams.

## Interfaces

```go
// package plan
func Load(root, change string) (*Model, error)
## not a heading, because it's inside a fence
```

## Notes

- Nothing else.
