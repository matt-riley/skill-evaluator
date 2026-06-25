---
title: Commands
description: Learn the skill-evaluator CLI commands including init, run, grade, benchmark, and loop, plus the flags that control baselines and single-eval runs.
---

# 🛠️ Handy Subcommands

| Command | What it does |
|---------|-------------|
| `init` | Sets up your `evals/evals.json` and workspace. Add `--global` for global config. |
| `run` | Executes all evals. Use `--eval <id>` for just one, or `--baseline previous` to snapshot. |
| `grade` | Asks the LLM to grade your assertions. Add `--benchmark` to auto-aggregate the stats. |
| `benchmark` | Wraps up all your grading results into a neat `benchmark.json`. |
| `loop` | Does it all: run → grade → benchmark! Add `--fix` to auto-refine failing evals. |

### The `--fix` flag 🪄

Tack `--fix` onto `loop` and skill-eval will automatically re-run any failing with-skill eval — feeding the judge's feedback back to the agent as a critique. It'll keep refining until every assertion passes, the score stops improving, or it hits the attempt limit.

```bash
skill-eval loop --fix                   # default: up to 3 fix attempts per eval
skill-eval loop --fix --max-fix-attempts 5   # crank it up if you're feeling ambitious!
```

Each fix attempt lands in `fix-N/` inside the eval directory, with its own grading and timing. The best attempt wins and gets promoted to the main `grading.json`. If the same assertions fail twice in a row, it stops early — no point burning tokens on a plateau! 🏔️
