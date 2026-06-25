---
title: Workspace
description: Understand the skill-evaluator workspace layout, including the evals directory, per-iteration result folders, and generated benchmark artifacts.
---

# 📂 Workspace Structure

Here's how your files will be organized:

```
<skill>-workspace/
└── iteration-1/
    ├── eval-1/
    │   ├── with_skill/
    │   │   ├── outputs/
    │   │   ├── timing.json
    │   │   └── grading.json
    │   └── baseline/
    │       ├── outputs/
    │       ├── timing.json
    │       └── grading.json
    └── benchmark.json
```

---

## 🚦 Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Yay! Everything completed successfully. |
| 1 | Oops! Something went wrong (agent crash, config error, etc.). |
