# Plan 001: Add Go test coverage to the CLI core

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat c6dac63..HEAD -- *.go`
> If any `*.go` file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: LOW
- **Depends on**: none
- **Category**: tests
- **Planned at**: commit `c6dac63`, 2026-06-25

## Why this matters

The CLI currently has zero Go tests (`go test ./...` compiles nothing). That means every refactor — like the recent `Delta` and `gradeFromOutput` cleanups — is manually validated, and regressions in config merging, model parsing, aggregation, or output parsing can only be caught by running real agent invocations. A small suite of table-driven unit tests for the pure functions gives us a safety net for v1 without requiring live LLM calls.

## Current state

- Root package is a single `main` package with all logic in flat files at the repo root.
- Pure functions available to test today:
  - `config.go`: `mergeConfig`, `ModelConfig.Key`, `resolveModels`
  - `main.go`: `parseModels`
  - `benchmark.go`: `mean`, `stddev`, `computeDelta`, `aggregateRuns`
  - `runner.go`: `extractTokens`
  - `grader.go`: `parseGradingOutput`, `extractFailedReasoning`, `truncate`
  - `workspace.go`: `workspacePath`, `iterationPath`, `evalPath`, `nextIteration`
- Repo conventions: Go 1.26.4, standard formatting (`gofmt`), `golangci-lint run`, no external test framework (use `testing` + `cmp` optional). Existing docs-site tests use vitest, but this plan is for Go only.

Relevant excerpt from `config.go:64-74` — `mergeConfig`:

```go
func mergeConfig(dst, src *Config) {
	if src.Defaults.Agent != "" {
		dst.Defaults.Agent = src.Defaults.Agent
	}
	if src.Defaults.Model != "" {
		dst.Defaults.Model = src.Defaults.Model
	}
	if src.Judge.Agent != "" {
		dst.Judge.Agent = src.Judge.Agent
	}
	if src.Judge.Model != "" {
		dst.Judge.Model = src.Judge.Model
	}
	if len(src.Models) > 0 {
		dst.Models = src.Models
	}
}
```

## Commands you will need

| Purpose   | Command                  | Expected on success |
|-----------|--------------------------|---------------------|
| Build     | `go build ./...`         | exit 0              |
| Format    | `gofmt -w *.go *_test.go`| exit 0              |
| Lint      | `golangci-lint run`      | exit 0              |
| Tests     | `go test ./...`          | exit 0, >0 tests    |

## Scope

**In scope** (create/modify):
- `config_test.go`
- `benchmark_test.go`
- `runner_test.go`
- `grader_test.go`
- `workspace_test.go`

**Out of scope**:
- Do NOT change behavior of production code to make it testable. If a function is genuinely untestable today, leave it for a future plan; this plan focuses on pure functions.
- Do NOT add tests for agent shell-out paths (those need a test harness planned separately).
- Do NOT modify docs site tests.

## Git workflow

- Branch: `advisor/001-go-test-coverage`
- Commit message style: `test: add coverage for Config merge and model keys` (repo uses conventional commits-ish; see `git log --oneline`).
- Do NOT push unless instructed.

## Steps

### Step 1: Create config tests

Create `config_test.go` with table-driven tests for `mergeConfig`, `ModelConfig.Key`, and `resolveModels`.

Cases to cover:
- `mergeConfig`: global defaults preserved; skill overrides override; empty skill fields don't clobber.
- `ModelConfig.Key`: with model (`agent-model-name`), without model (`agent`).
- `resolveModels`: CLI wins over config `models:`; config `models:` wins over defaults; empty everything falls back to defaults.

**Verify**: `go test ./... -run TestConfig` → pass

### Step 2: Create benchmark math tests

Create `benchmark_test.go` with tests for `mean`, `stddev`, `computeDelta`, and `aggregateRuns`.

Cases to cover:
- `mean` of empty slice returns 0.
- `stddev` of <2 values returns 0.
- `computeDelta` computes with_skill - baseline means.
- `aggregateRuns` handles nil grading/timing gracefully (some results missing those fields).

**Verify**: `go test ./... -run TestBenchmark` → pass

### Step 3: Create runner tests

Create `runner_test.go` with tests for `extractTokens`.

Cases to cover:
- Matches each of the four regex patterns at `runner.go:85-96`.
- Returns 0 when no pattern matches.

**Verify**: `go test ./... -run TestRunner` → pass

### Step 4: Create grader tests

Create `grader_test.go` with tests for `parseGradingOutput` and `extractFailedReasoning`.

Cases to cover:
- `parseGradingOutput`: extracts JSON from plain output, from markdown-fenced output, returns error when no JSON.
- `extractFailedReasoning`: concatenates evidence from failed assertions, returns empty when all pass.

**Verify**: `go test ./... -run TestGrader` → pass

### Step 5: Create workspace tests

Create `workspace_test.go` with tests for `workspacePath`, `iterationPath`, `evalPath`, and `nextIteration`.

For `nextIteration`, create a temporary directory with a few `iteration-N` dirs and verify it returns max+1. Clean up with `t.TempDir()`.

Cases to cover:
- Path construction with and without model key.
- `nextIteration` ignores non-iteration directories/files.

**Verify**: `go test ./... -run TestWorkspace` → pass

### Step 6: Create main.go parseModels test

Create `main_test.go` with tests for `parseModels`.

Cases to cover:
- Empty string returns nil.
- Single pair with agent and model.
- Multiple pairs.
- Missing model (just agent) is allowed.
- Whitespace trimmed.
- Empty list after trimming returns error.

**Verify**: `go test ./... -run TestParseModels` → pass

### Step 7: Final cleanup

Run formatter and linter.

**Verify**:
- `gofmt -w *.go *_test.go`
- `go test ./...` → all pass
- `go vet ./...` → exit 0
- `golangci-lint run` → exit 0

## Test plan

- New test files created in step 1-6.
- Use standard Go table-driven tests (`[]struct{name string; ...}` + `t.Run`).
- For floating-point comparisons, use a small tolerance or exact equality where values are deterministic.

## Done criteria

- [ ] `go test ./...` exits 0 with >10 tests.
- [ ] `go vet ./...` exits 0.
- [ ] `golangci-lint run` exits 0.
- [ ] Only `*_test.go` files and any `go.mod`/`go.sum` updates from dependencies were created.
- [ ] No behavior of production code is changed.
- [ ] `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- The functions in the "Current state" excerpt are not at the stated locations (drift).
- A test requires changing production function signatures to be testable. Defer that function instead of changing it.
- `go test` fails and the failure is not a simple test-data issue after two fix attempts.

## Maintenance notes

- This is the foundation plan. Future plans (especially iteration lockfile/resume) depend on a working test command.
- Reviewers should check that tests are deterministic and do not write to the real filesystem outside `t.TempDir()`.
- Follow-up: integration tests for agent shell-outs once a command-builder seam is added.
