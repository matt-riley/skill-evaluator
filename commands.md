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
| `loop` | Does it all: run → grade → benchmark! Add `--fix` to auto-refine, `--models` to compare agents. |

### The `--model` flag 🔬

Want to know if your skill helps *every* agent, not just your default? Pass `--models` with a comma-separated list of `agent:model` pairs and skill-eval will run every eval against each one:

```bash
skill-eval loop --models pi:claude-sonnet,claude,copilot
```

Each model gets its own with-skill + baseline run, producing per-model stats in `benchmark.json` with a `best_model` and `worst_model`. Spot which agent your skill helps most — and which one needs work! 🔍

> 💡 **Runs in batches of 2** to keep things snappy without hammering APIs. If you've got lots of evals × models, skill-eval warns you before firing off a barrage of agent invocations.

> 🛠️ **Tired of typing it every time?** Drop `models:` into your `.skill-eval.yaml` and `--models` becomes the default.

```yaml
# .skill-eval.yaml
models:
  - agent: pi
    model: claude-sonnet-4-5
  - agent: claude
  - agent: copilot
```

### The `--fix` flag 🪄

Tack `--fix` onto `loop` and skill-eval will automatically re-run any failing with-skill eval — feeding the judge's feedback back to the agent as a critique. It'll keep refining until every assertion passes, the score stops improving, or it hits the attempt limit.

```bash
skill-eval loop --fix                   # default: up to 3 fix attempts per eval
skill-eval loop --fix --max-fix-attempts 5   # crank it up if you're feeling ambitious!
```

Each fix attempt lands in `fix-N/` inside the eval directory, with its own grading and timing. The best attempt wins and gets promoted to the main `grading.json`. If the same assertions fail twice in a row, it stops early — no point burning tokens on a plateau! 🏔️
