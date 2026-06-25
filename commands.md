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
| `loop` | Does it all: run → grade → benchmark! |
