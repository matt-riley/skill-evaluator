---
title: Workspace
description: Understand the skill-evaluator workspace layout, including the evals directory, per-iteration result folders, and generated benchmark artifacts.
---

# 📂 Workspace Structure

Here's how your files will be organized:

```
<skill>-workspace/
└── iteration-1/
    ├── .lock.json
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

> 🏷️ Running with `--models`? Each eval gets a model-key subdirectory: `eval-1/pi-claude-sonnet/with_skill/...`.

### The `.lock.json` file 🔒

Every iteration writes a `.lock.json` file that tracks which evals have finished. It’s what makes `--resume` work! You don’t need to edit it by hand, but it’s useful to know it exists. If a run is interrupted, the lock stays in `"running"` status; finish it up with `skill-eval run --resume`.

---

## 🚦 Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Yay! Everything completed successfully. |
| 1 | Oops! Something went wrong (agent crash, config error, etc.). |
