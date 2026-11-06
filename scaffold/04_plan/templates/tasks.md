# tasks.json — schema reference

The plan artifact is JSON (not Markdown). Shape:

```json
{
  "change": "<n>-<slug>",
  "source": "00_intake/<id>.md",
  "generatedBy": "architect",
  "tasks": [
    {
      "id": "T1",
      "title": "<what this task does>",
      "files": ["<path>", "<path>"],
      "dependsOn": [],
      "acceptance": ["AC1", "AC2"],
      "estimate": "S | M | L"
    }
  ]
}
```

- `dependsOn` names earlier task ids only (dependency order).
- `files` names the exact files the task touches.
- `acceptance` maps to acceptance criteria in requirements.md.
- `estimate` is optional sizing (S | M | L); no formal methodology.
