# Plan 011: Add `--runs N` repetitions for statistically honest benchmarks

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- cmd_run.go cmd_grade.go cmd_loop.go benchmark.go workspace.go eval.go runner.go grader.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: MED (touches workspace layout for the new mode, lockfile identity, and aggregation math)
- **Depends on**: —
- **Category**: statistical correctness
- **Planned at**: commit `1325f07`, 2026-07-01
- **Research**: `docs/research/agengit-integration.md` §4.1

## Why this matters

Agent runs are high-variance. Today each eval/config/model triple runs
**once**, and `Stats.Stddev` is the spread *across different evals*
(`benchmark.go:193-215`) — it says nothing about run-to-run noise. So a
with-skill vs baseline pass-rate delta of ±10% can be pure sampling error,
and `iteration_delta` can report a "regression" caused by one flaky run.
Authors then edit skills to chase noise, which is the exact failure mode an
eval tool exists to prevent. Repeating each triple N times (even N=3),
averaging per-eval first, and reporting per-eval variance makes the deltas
trustworthy.

## Current state

- `cmd_run.go:13-21` — run flags; no repetition concept. Jobs are built per
  `(eval, model, config)` at `cmd_run.go:182-190`.
- `eval.go:77-82` — lockfile identity is `RunIdentity{EvalID, Model, Config}`.
- `runner.go:17-19` — output path is
  `evalPath(...)/<config>/outputs`; one run per triple by construction.
- `cmd_grade.go:80-116` — grading walks `(eval, model, config)` and reads
  the single `outputs/` dir.
- `benchmark.go:193-215` — `aggregateRuns` treats each `RunResult` as one
  sample:

```go
	var passRates, times, tokens []float64
	for _, r := range results {
		if r.Grading != nil {
			passRates = append(passRates, r.Grading.Summary.PassRate)
		}
		...
	}
	return RunSummary{
		PassRate: Stats{Mean: mean(passRates), Stddev: stddev(passRates)},
		...
```

- `eval.go:99-106` — `RunResult` has no run index.
- Cost guard at `cmd_run.go:141-151` multiplies
  `evalCount * len(models) * len(configsToRun)` — must include N.

## Design decisions (read before coding)

1. **Layout**: when `--runs 1` (default), the layout is **byte-for-byte
   unchanged** (`<config>/outputs`). When `--runs N>1`, each repetition
   writes to `<config>/run-<r>/outputs` (r = 1..N), with `timing.json` and
   `grading.json` inside each `run-<r>/`. No migration of old workspaces.
2. **Identity**: `RunIdentity` gains `Run int` (`json:"run,omitempty"`), so
   resume skips completed repetitions. `omitempty` keeps old lockfiles
   parseable (run 0 ≡ single-run mode).
3. **Aggregation**: two-level. For each `(eval, model, config)`: per-eval
   mean over its runs. Then `RunSummary` mean/stddev over per-eval means
   (same axis as today, so numbers stay comparable). Additionally report
   run-level noise: new field `RunSummary.PassRateRunStddev` = mean of
   per-eval stddevs across runs (0 when N=1).
4. **Grading cost**: N repetitions mean N gradings. Deterministic matchers
   (Plan 003) are free; the judge is not. The cost prompt must reflect it.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |

## Scope

**In scope**:
- `cmd_run.go`, `cmd_grade.go`, `cmd_loop.go` (flag + walk changes)
- `runner.go` (run-indexed output paths), `grader.go` (run-indexed grading paths)
- `workspace.go` (path helper), `eval.go` (`RunIdentity.Run`, `RunResult.Run`, `RunSummary` extension)
- `benchmark.go` (two-level aggregation)
- Tests across the matching `*_test.go` files
- `eval-workflow.md`, `docs/guides/reading-results.md`, usage text in `main.go`

**Out of scope**:
- Do NOT change the single-run layout or renumber anything existing.
- Do NOT apply repetitions to the `--fix` phase (fix already iterates; its
  attempts are not statistical samples).
- Do NOT add confidence intervals / significance tests in this plan (the
  raw per-eval stddev is the v1 deliverable; a t-test is a follow-up).
- Do NOT parallelize beyond the existing `--parallel` semaphore — reps are
  just more jobs in the same pool.

## Git workflow

- Branch: `advisor/011-repeat-runs-statistics`
- Commit message style: `feat: add --runs N repetitions with per-eval variance in benchmarks`
- Do NOT push unless instructed.

## Steps

### Step 1: Path helper

In `workspace.go` add:

```go
// runConfigPath returns the directory holding outputs/timing/grading for
// one repetition. run==0 means single-run mode (legacy layout).
func runConfigPath(workspace string, iteration, evalID int, modelKey, config string, run int) string {
	base := filepath.Join(evalPath(workspace, iteration, evalID, modelKey), config)
	if run <= 0 {
		return base
	}
	return filepath.Join(base, fmt.Sprintf("run-%d", run))
}
```

Refactor `runEval` (`runner.go:17-19`), `gradeEval` (`grader.go:16-20`),
and `cmd_grade.go`'s path probing to go through it, passing `run` down.
`fixEval`/`gradeFixAttempt` keep run 0.

**Verify**: `go test ./... -run TestRunConfigPath` — run 0 yields today's
exact paths; run 2 yields `.../with_skill/run-2`.

### Step 2: Identity and result plumbing

- `eval.go`: add `Run int \`json:"run,omitempty"\`` to `RunIdentity`; add
  `Run int` to `RunResult`.
- `workspace.go:isCompleted` (`:172-179`): compare `Run` too.
- `runEval` signature gains `run int`; writes into `runConfigPath`.

**Verify**: `go build ./...`; existing lockfile JSON without `run` still
round-trips (`TestLockBackCompat`).

### Step 3: Run phase

In `cmd_run.go`:
- Add `runsFlag := fs.Int("runs", 1, "Repetitions per eval/config (higher = better statistics, more cost)")`;
  validate 1..10.
- Job struct gains `run int`; job expansion nests one more loop:
  `for r := 1; r <= *runsFlag; r++` (use `r=0` when `*runsFlag == 1` to
  keep the legacy layout).
- Cost guard: `totalRuns := evalCount * len(models) * len(configsToRun) * *runsFlag`
  and mention runs in the warning text.
- Resume: pass `job.run` into `isCompleted`/`RunIdentity`.
- Progress line includes the rep: `pi/with_skill run 2/3 ok (8123ms)`
  (only when runs > 1).

**Verify**: `go test ./... -run TestRunJobExpansion` (stub `CmdBuilder`
counting invocations: 2 evals × 1 model × 2 configs × 3 runs = 12).

### Step 4: Grade phase

In `cmd_grade.go`:
- Add the same `--runs` flag? **No** — grade must not need to be told.
  Instead auto-detect: for each `(eval, model, config)`, if
  `<config>/run-1` exists, grade every `run-*` dir found; else grade the
  legacy single dir. Store `Run` on each produced `RunResult`.
- `cmd_loop.go`: add `--runs` and forward to run phase only
  (`runArgs = append(runArgs, "--runs", ...)` when != 1).

**Verify**: `go test ./... -run TestGradeDetectsRunDirs`.

### Step 5: Two-level aggregation

In `benchmark.go`:

1. Add to `RunSummary` (additive, `omitempty`):

```go
	// PassRateRunStddev is the mean across evals of the per-eval stddev
	// over repeated runs. 0 when each eval ran once.
	PassRateRunStddev float64 `json:"pass_rate_run_stddev,omitempty"`
```

   and to `BenchmarkFile`: `Runs int \`json:"runs,omitempty"\``.

2. In `aggregateRuns` (or a new `aggregateRunsGrouped` called from
   `splitAndAggregate`): group results by `EvalID` first, compute each
   eval's mean pass-rate/time/tokens over its runs and its pass-rate
   stddev; then compute `Stats` over per-eval means and
   `PassRateRunStddev` as the mean of per-eval stddevs.
   With one run per eval this degenerates to exactly today's math —
   assert that in tests.

**Verify**: `go test ./... -run TestAggregateGrouped` with hand-computed
fixtures (e.g. eval 1 runs {1.0, 0.5}, eval 2 runs {0.5, 0.5} → per-eval
means {0.75, 0.5}, mean 0.625; run-stddevs {0.25, 0} → PassRateRunStddev 0.125).

### Step 6: Docs and final checks

- `main.go` usage: `--runs <n>` line.
- `eval-workflow.md` + `docs/guides/reading-results.md`: explain
  `pass_rate_run_stddev` ("how noisy is a single run of this skill") and
  recommend `--runs 3` before trusting small deltas.

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

## Test plan

- `TestRunConfigPath` (legacy vs run-indexed paths).
- `TestLockBackCompat` (old lockfile parses; `Run` zero-value).
- `TestRunJobExpansion` (job count = evals × models × configs × runs).
- `TestResumeSkipsCompletedRun` (rep 2 of 3 completed → only reps 1,3 run).
- `TestGradeDetectsRunDirs` (mixed legacy + run-dir workspaces).
- `TestAggregateGrouped` (hand-computed two-level stats; N=1 degeneracy
  equals current `aggregateRuns` output on the same input).

## Done criteria

- [ ] `--runs N` on `run` and `loop`; default 1 keeps today's layout and math byte-identical.
- [ ] Lockfile/resume are repetition-aware; old lockfiles still parse.
- [ ] `grade` auto-detects run dirs; no flag needed.
- [ ] `benchmark.json` carries `runs` and `pass_rate_run_stddev`; cross-eval stats are computed over per-eval means.
- [ ] Cost warning accounts for repetitions.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] Docs updated; `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- The `runEval`/`gradeEval` signature changes ripple into more than the
  call sites listed in Scope (search for callers first; `fixEval` and
  `gradeFixAttempt` must keep run 0).
- Grading auto-detection is ambiguous for a workspace that has *both* a
  legacy `outputs/` and `run-*` dirs for the same triple — define
  precedence (run dirs win) and test it; if real workspaces mix them in
  other ways, stop.
- Report templates (`report.go`) break on the extended `RunSummary` —
  additive fields shouldn't, but verify rendering before merging.

## Maintenance notes

- Keep the two-level aggregation the only place that knows about run
  grouping; benchmark consumers read the same `Stats` shapes as before.
- Plan 015 (process quality) will attach per-run artifacts under the same
  `run-<r>/` dirs — the layout choice here is load-bearing for it.
- Follow-up: significance testing (paired t-test across evals between
  with_skill and baseline) once N>1 data is common.
