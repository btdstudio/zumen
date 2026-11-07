# <project> — planning workspace

This workspace follows the Interpretable Context Methodology (ICM) for *planning*
software work: research, architecture and work breakdown, upstream of any code.
Start with [AGENTS.md](./AGENTS.md).

- `00_intake/` — the base source (an idea/issue, or a pointer to real codebases)
- `01_research/` — researcher: external investigation
- `02_explore/` — explorer and synthesis: one findings report per repo, and what they add up to
- `03_spec/` — analyst: what, and how we know it's done
- `04_architecture/` — architect: the smallest solution + ADRs
- `05_breakdown/` — architect: tickets
- `06_publish/` — publisher: sync tickets to a PM tool, then freeze them
- `specs/` — one folder per change, holding its spec, architecture and tickets

The loop is `intake → [research] → [explore] → [synthesis] → spec → architecture →
breakdown → publish`, each gated phase stopping for human approval. Driven by the
`zumen` CLI.
