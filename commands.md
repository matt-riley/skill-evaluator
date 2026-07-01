---
title: Commands
description: Learn the skill-evaluator CLI commands including init, run, grade, benchmark, and loop, plus the flags that control baselines and single-eval runs.
---

# 🛠️ Handy Subcommands

| Command | What it does |
|---------|-------------|
| `init` | Sets up your `evals/evals.json` and workspace. Add `--global` for global config. |
| `run` | Executes all evals. Use `--eval <id>` for just one, `--baseline previous` to snapshot, or `--resume` to pick up where you left off. |
| `grade` | Asks the LLM to grade your assertions. Add `--benchmark` to auto-aggregate the stats. |
| `benchmark` | Wraps up all your grading results into a neat `benchmark.json`. |
| `report` | Generates a beautiful HTML report of your benchmark results! Add `--llm-suggestions` for AI coach notes. 📊 |
| `loop` | Does it all: run → grade → benchmark! Add `--fix` to auto-refine, `--models` to compare agents. |
| `import-agit` | Converts your interactive agit sessions into an evals.json corpus. Magic! 🪄 |

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

### The `--baseline-only` flag 🎯

Sometimes you just want to see how the baseline model performs without waiting for the `with-skill` run. If you're tightening up your evals and want to verify that the baseline actually fails them (a key step in our [writing evals guide](guides/writing-evals.md)!), use `--baseline-only`:

```bash
skill-eval run --baseline-only --eval 1
skill-eval loop --baseline-only
```

This skips the `with_skill` execution entirely, saving you time and API tokens while you iterate on your fixtures and assertions. 


### The `--fix` flag 🪄

Tack `--fix` onto `loop` and skill-eval will automatically re-run any failing with-skill eval — feeding the judge's feedback back to the agent as a critique. It'll keep refining until every assertion passes, the score stops improving, or it hits the attempt limit.

```bash
skill-eval loop --fix                   # default: up to 3 fix attempts per eval
skill-eval loop --fix --max-fix-attempts 5   # crank it up if you're feeling ambitious!
```

Each fix attempt lands in `fix-N/` inside the eval directory, with its own grading and timing. The best attempt wins and gets promoted to the main `grading.json`. If the same assertions fail twice in a row, it stops early — no point burning tokens on a plateau! 🏔️

### Pick up where you left off with `--resume` 🔄

Long runs sometimes get interrupted. `skill-eval run` writes a progress lockfile at `<workspace>/iteration-N/.lock.json`, so you can resume the latest unfinished iteration instead of starting over:

```bash
skill-eval run --resume
skill-eval loop --resume
```

If the latest iteration is already complete, `--resume` tells you there's nothing to pick up. And `skill-eval grade` will refuse to grade an incomplete iteration — finish it with `--resume` first!

### Turn Sessions into Evals with `import-agit` 🪄

Writing evals by hand is great, but what if you could just do the work and let `skill-eval` build the eval for you? The `import-agit` command takes a recorded `agit` session and magically turns each big step into an eval!

```bash
skill-eval import-agit                 # Imports your most recent session
skill-eval import-agit --session 1234  # Imports a specific session
skill-eval import-agit --force         # Overwrite existing evals!
skill-eval import-agit --merge          # Append to existing evals.json
skill-eval import-agit --all-sessions  # Import every recorded session
skill-eval import-agit --all-sessions --eval-filter good  # Only quality sessions
```

This is perfect for kickstarting a brand new eval suite. Check out the [Importing Sessions guide](guides/importing-agit-sessions.md) for the full breakdown!

### Share it with a Beautiful HTML Report 📊

Want to share your amazing progress or just review it in style? Run the `report` command:

```bash
skill-eval report
skill-eval report --llm-suggestions
```

This magic command generates a gorgeous `report.html` right in your iteration directory. It shows per-model stats, pass-rate trends, and actionable suggestions. Toss in `--llm-suggestions` and your judge agent will drop in some personalized coaching notes on what to test next! ✨

### Debug with `--verbose` 🐛

When something goes wrong, `--verbose` (or `-v`) prints the agent commands, durations, and raw output to stderr. It’s super handy for CI logs or when an agent fails mysteriously:

```bash
skill-eval run --verbose
skill-eval -v loop --models pi:claude-sonnet,claude
```

> 🔒 Verbose mode never prints secrets or the contents of your input files — just operational details.

### Mix in deterministic matchers 🤖

For quick, repeatable checks, prefix assertions with `file_exists:`, `contains_text:`, or `matches_text:`. They run locally instead of burning tokens on the judge. See the [Eval Workflow guide](/eval-workflow/) for examples!
