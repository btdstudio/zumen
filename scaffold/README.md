# <project> — planning workspace

This workspace follows the Interpretable Context Methodology (ICM) for
*planning* software work: architecture, design, and task breakdown, upstream of
any code. Start with [AGENTS.md](./AGENTS.md).

- `00_intake/` — the base source (an idea/issue, or a pointer to a real codebase)
- `01_explore/` — findings when the source is an existing codebase
- `02_requirements/` — analyst: what, and how we know it's done
- `03_design/` — architect: the smallest solution + ADRs
- `04_plan/` — architect: dependency-ordered tasks
- `specs/` — one folder per change holding requirements/design/tasks

The loop is `intake → [explore] → requirements → design → plan`, each phase
gated for human approval. Driven by the `zumen` CLI.
