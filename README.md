# skill-eval

Automated skill evaluation CLI. Runs the eval-driven iteration loop from [agentskills.io](https://agentskills.io/skill-creation/evaluating-skills): define test cases, spawn agent runs with/without a skill, LLM-grade assertions, aggregate benchmarks.

## Quick start

```bash
# Install
go install github.com/biztocorp/skill-evaluator@latest

# One-time global config
skill-eval init --global

# In your skill directory
skill-eval init                  # scaffold evals/evals.json
# Edit evals/evals.json with your test cases (see below)
skill-eval loop                  # run → grade → benchmark
```

## Eval workflow

### 1. Design test cases

`skill-eval init` scaffolds a skeleton `evals/evals.json` in your skill directory. Edit it with 2-3 realistic prompts — don't over-invest before seeing your first round of results.

```json
{
  "skill_name": "csv-analyzer",
  "evals": [
    {
      "id": 1,
      "prompt": "I have a CSV of monthly sales data in data/sales_2025.csv. Can you find the top 3 months by revenue and make a bar chart?",
      "expected_output": "A bar chart image showing the top 3 months by revenue, with labeled axes and values.",
      "files": ["evals/files/sales_2025.csv"]
    }
  ]
}
```

Tips for good prompts:
- **Vary phrasing** — some casual ("hey can you clean up this csv"), others precise.
- **Cover edge cases** — include at least one boundary-condition prompt.
- **Use realistic context** — real users mention file paths, column names, and domain context.
- **Start small** — 2-3 test cases. Expand after the first iteration.

### 2. Write assertions

Assertions are verifiable pass/fail statements about what the output should contain. **Add them after your first run** — you often don't know what "good" looks like until the skill has run.

```json
"assertions": [
  "The output includes a bar chart image file",
  "The chart shows exactly 3 months",
  "Both axes are labeled",
  "The chart title or caption mentions revenue"
]
```

Good assertions are specific and observable ("a file named results.csv exists"), not vague ("the output is good") or brittle ("the output uses exactly the phrase 'Total Revenue: $X'").

### 3. Run the loop

```bash
skill-eval loop
```

This executes the full cycle for the current iteration:

1. **Run** — executes every eval twice: once with the skill loaded, once as a baseline (no skill). Each run saves outputs to `outputs/` and records timing data in `timing.json`.
2. **Grade** — shells out to the judge agent, which evaluates each assertion against the actual outputs and produces `grading.json` with PASS/FAIL verdicts and evidence.
3. **Benchmark** — aggregates across all evals into `benchmark.json` with mean pass rates, timing, token usage, and **deltas** showing what the skill adds vs. the baseline.

### 4. Review results

After the loop completes, inspect the workspace. For each eval, look at:

| Artifact | What it tells you |
|----------|------------------|
| `outputs/` | The actual files the agent produced — open them, verify they look right |
| `grading.json` | Which assertions passed/failed, with specific evidence for each verdict |
| `benchmark.json` | Aggregated stats — the delta tells you if the skill is pulling its weight |

**Record human feedback** for each eval in `feedback.json` alongside the eval directories:

```json
{
  "eval-1": "The chart is missing axis labels and the months are in alphabetical order instead of chronological.",
  "eval-2": ""
}
```

Specific complaints ("missing axis labels") are actionable; vague feedback ("looks bad") is not. Empty feedback means the output passed your review.

### 5. Analyze patterns

Aggregate stats hide important details. After grading:

- **Remove assertions that always pass in both configs** — they don't tell you anything useful about the skill.
- **Fix assertions that always fail in both configs** — either the assertion is broken, the test case is too hard, or it's checking the wrong thing.
- **Study assertions that pass with the skill but fail without** — this is where the skill adds real value. Understand why: which instructions made the difference?
- **Investigate time/token outliers** — if one eval takes 3x longer, read the execution transcript to find the bottleneck.

### 6. Iterate

You now have three sources of signal:

- **Failed assertions** → specific gaps in the skill
- **Human feedback** → broader quality issues assertions miss
- **Execution transcripts** → why things went wrong (ambiguous instructions, wasted work)

Turn these into skill improvements, then rerun:

```bash
# After editing SKILL.md...
skill-eval loop --baseline previous
```

`--baseline previous` snapshots the current skill version before the run and uses it as the comparison point — so you see improvement against your last version, not against no-skill.

A new `iteration-2/` directory is created. Grade, review, iterate. **Stop when** you're satisfied with the results, feedback is consistently empty, or the delta between iterations stops improving.

## Subcommands

| Command | What it does |
|---------|-------------|
| `init` | Scaffold `evals/evals.json` + workspace. `--global` for config. |
| `run` | Execute all evals with-skill and baseline. `--eval <id>` for single. `--baseline previous` to snapshot. |
| `grade` | LLM-grade assertions against outputs. `--benchmark` to auto-aggregate. |
| `benchmark` | Aggregate grading results into `benchmark.json`. |
| `loop` | Full cycle: run → grade → benchmark. |

## Configuration

Two-tier YAML (shared JSON Schema at `schema/config-schema.json`):

**Global** (`~/.config/skill-eval/config.yaml`):
```yaml
defaults:
  agent: pi
  model: claude-sonnet-4-5
judge:
  model: gpt-4o-mini
```

**Per-skill** (`.skill-eval.yaml` in skill root) — overrides global.

Supported agent runtimes: `pi`, `claude`, `copilot`, `codex`.

## Workspace structure

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

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | All completed successfully |
| 1 | Operational failure (agent crash, config error, malformed JSON) |
