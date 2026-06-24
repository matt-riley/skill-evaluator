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
# Edit evals/evals.json with your test cases
skill-eval loop                  # run → grade → benchmark
```

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
