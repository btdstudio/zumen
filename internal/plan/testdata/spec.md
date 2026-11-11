# Functional spec — Synthetic fixture

- **Status:** approved
- **Date:** 2026-01-01
- **Sources:** a hand-written fixture that mirrors the layout of a real spec

## Problem / Goal

- R90. A requirement-like line outside the requirements section. — **must**

## Scope

### In scope

- Things.

## Functional requirements

### Functional

**Group one**

- R1. The first capability. — **must**
- R2. The second capability, which mentions `List<T>` in code. — **must**
- R3. The third capability. — **should**
  - A nested bullet that adds detail.
  - Another nested bullet, mentioning R4 in passing.
- R4. The fourth capability. — **could**
- R5. A requirement whose text runs on — **must**
  to a second line, with more detail.
- R6. The sixth capability. — **must**

**Group two**

- R7. The seventh capability. — **must**
- R8. The eighth capability. — **should**
- R9. The ninth capability. — **must**
- R10. The tenth capability. — **must**

```text
- R50. A fenced example of a requirement. — **must**
```

- R11. The eleventh capability. — **must**
- R12. The twelfth capability. — **must**
* R13. A star bullet. — **must**
- R14. The fourteenth capability. — **should**

**Group three**

- R15. The fifteenth capability. — **must**
- R16. The sixteenth capability. — **must**
- R17. The seventeenth capability. — **must**
- R18. The eighteenth capability. — **must**
- R19. The nineteenth capability. — **should**
- R20. The twentieth capability. — **must**
- R21. The twenty-first capability. — **must**
- R22. The twenty-second capability. — **should** *(amended later: a note after the priority, which also says — **must**)*
- R23. The twenty-third capability. — **must**

### Non-functional

- N1. Standard library only. — **must**
- N2. The core stays generic. — **must**
- N3. The schema stays the same. — **must**
- N4. Each case has a test. — **must**
- N5. Releases are tagged. — **must**
- N6. Files stay readable by hand. — **must**

## Acceptance criteria

- AC1. Given a thing, when it runs, then it works. *(R1, R2)*
- AC2. Given another thing, then it also works. *(R3; N1)*
- R99. A requirement-like line under the acceptance criteria. — **must**

## Notes

- R91. A requirement-like line under the notes. — **must**
