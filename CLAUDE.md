# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build, test, and lint

```bash
go build -o skill-eval .           # Build local binary
go install ./...                   # Install from source
go test ./...                      # Run tests (repo currently has no *_test.go files)
go test ./... -run '^TestName$'    # Run a single test
go vet ./...                        # Static analysis
gofmt -w .                          # Format all Go files
```

## Project overview

**skill-evaluator** is a provider-agnostic CLI tool that automates the eval-driven skill iteration loop: spawning agent runs with and without a skill, LLM-grading assertions, and aggregating benchmarks.

A **skill** is a set of instructions (packaged as `SKILL.md`) that guides an LLM agent's behavior. An **eval** is a test case with a prompt, expected output description, optional input files, and assertions. The tool runs each eval twice (with-skill and baseline), grades the outputs, and aggregates results into a **benchmark** showing mean pass rate, time, tokens, and stddev deltas.

## High-level architecture

Single-package Go CLI (`package main`) at the repo root, split by responsibility:

- `main.go`: CLI entrypoint and subcommand dispatch (`init`, `run`, `grade`, `benchmark`, `loop`)
- `config.go`: Loads and merges global config (`~/.config/skill-eval/config.yaml`) and per-skill config (`.skill-eval.yaml`), with skill-level precedence
- `runner.go`: Shells out to agent CLIs (`pi`, `claude`, `copilot`, `codex`), captures timing/token data, writes run artifacts
- `grader.go`: Reads output files, builds grading prompt, shells out to judge agent, writes `grading.json`
- `benchmark.go`: Aggregates grading/timing results with means, stddevs, and with-skill vs baseline deltas
- `workspace.go`: Owns workspace/iteration/eval path conventions and snapshot copying
- `eval.go`: Shared data model for eval input, run result, grading result, benchmark schema

Runtime flow: `run` → `grade` → `benchmark` (or all three via `loop`).

## Key conventions

- **Skill directory discovery**: Commands walk upward from current working directory until finding `SKILL.md`
- **Workspace location**: Results write to `<skill-dir>/../<skill-name>-workspace/iteration-N/...`, outside the skill folder
- **Dual-config runs**: Each eval always produces two runs (with-skill and baseline); baseline can be none, explicit path, or previous-iteration snapshot
- **Config precedence**: Built-in defaults → global config → per-skill config override
- **Grader contract**: Judge responses are strict JSON; first JSON object is extracted and parsed
- **Agent integration**: Shell-out pattern (not SDK), so agent CLI flags and stdout/stderr matter; see `docs/adr/0001-shell-out-to-agent-runtimes.md`
- **Shared config schema**: Both global and skill configs follow `schema/config-schema.json` (`defaults` and `judge` blocks, supported agents enum)

## Workspace structure

```
<skill-name>-workspace/
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

## Testing strategy

No test suite yet. When adding coverage, prefer table-driven Go tests in `*_test.go` files alongside the code they exercise. Test names should describe behavior (e.g., `TestMergeConfig`, `TestWorkspacePath`).

## Language and terminology

See `CONTEXT.md` for canonical terminology (Skill, Eval, Assertion, Run, Grading, Benchmark, Iteration, Workspace, Feedback, Agent Runtime, Baseline, Snapshot, etc.). Use these terms consistently in code, docs, and commit messages — avoid synonyms like "test case" for "eval" or "score" for "benchmark."

## Related reading

- `README.md`: Quick start and subcommand overview
- `CONTEXT.md`: Full operational language and concept definitions
- `AGENTS.md`: Development guidelines and architecture
- `docs/adr/0001-shell-out-to-agent-runtimes.md`: Why we shell out to agent runtimes instead of embedding clients
