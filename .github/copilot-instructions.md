# Copilot Instructions for `skill-evaluator`

## Build, test, and lint

- Build local binary: `go build -o skill-eval .`
- Install from source: `go install github.com/biztocorp/skill-evaluator@latest`
- Run all tests: `go test ./...` (the repository currently has no `*_test.go` files)
- Run a single test by name: `go test ./... -run '^TestName$'`
- Static analysis used in this repo: `go vet ./...`

## High-level architecture

This repository is a single-package Go CLI orchestrator (`package main`) split by responsibility across files:

- `main.go`: CLI entrypoint and subcommand dispatch (`init`, `run`, `grade`, `benchmark`, `loop`).
- `config.go`: loads YAML config from global `~/.config/skill-eval/config.yaml` and per-skill `.skill-eval.yaml`, then merges with skill-level precedence.
- `runner.go`: executes eval runs by shelling out to agent CLIs (`pi`, `claude`, `copilot`, `codex`), captures timing/token data, and writes run artifacts.
- `grader.go`: reads generated output files, builds a strict grading prompt, shells out to the judge agent, and writes `grading.json`.
- `benchmark.go`: aggregates grading/timing results into `benchmark.json` with means, stddevs, and with-skill vs baseline deltas.
- `workspace.go`: owns workspace/iteration/eval path conventions and snapshot copying.
- `eval.go`: shared data model for eval input, run result, grading result, and benchmark schema.

Runtime flow:

1. `run` creates the next `iteration-N`, then runs each eval twice (`with_skill` and `baseline`).
2. `grade` grades assertions for each run config from produced output files.
3. `benchmark` aggregates graded results for the iteration.
4. `loop` executes `run` then `grade --benchmark` in sequence.

## Key repository conventions

- **Skill directory discovery is `SKILL.md`-based**: commands that operate on a skill walk upward from cwd until a directory containing `SKILL.md` is found.
- **Workspace lives beside the skill directory**: results are written to `<skill-dir>/../<skill-name>-workspace/iteration-N/...`, not inside the skill folder.
- **Each eval is always dual-config**: the orchestrator expects both `with_skill` and `baseline` runs per eval (baseline can be none, explicit path, or previous-iteration snapshot).
- **Config precedence is explicit**: built-in defaults (`pi`) → global config → per-skill config override.
- **Grader output contract is strict JSON**: judge responses are parsed by extracting the first JSON object; prompts explicitly require JSON-only output.
- **Agent integration pattern is shell-out, not SDK**: runtime CLIs are the contract boundary (per ADR `docs/adr/0001-shell-out-to-agent-runtimes.md`), so command flags and stdout/stderr behavior matter.
- **Shared config schema**: both global and skill configs are expected to follow `schema/config-schema.json` (`defaults` and `judge` blocks, supported agents enum).
