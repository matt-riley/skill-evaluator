# Plan 018: Benchmark integrity — failed runs must count, deltas must compare like with like

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- runner.go cmd_run.go cmd_grade.go cmd_benchmark.go benchmark.go eval.go workspace.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1 (silent statistical bias in the tool's core numbers)
- **Effort**: M
- **Risk**: MED (changes reported numbers; must be additive-and-explained, not silently different)
- **Depends on**: —
- **Category**: correctness
- **Planned at**: commit `1325f07`, 2026-07-01

## Why this matters

Three related integrity holes make `benchmark.json` quietly optimistic:

1. **Failed runs vanish from the sample.** When an agent invocation
   crashes or times out, `runEval` sets `Status: "failed"` — in memory
   only. Nothing persists the failure. The grade phase skips triples with
   no `outputs/` (`cmd_grade.go:93-98`), so no `grading.json` is written;
   the benchmark phase samples only triples that *have* a `grading.json`
   (`cmd_benchmark.go:52-68`). Net effect: a skill that makes the agent
   crash on 3 of 10 evals reports a pass rate computed over the 7
   survivors — **failures make the numbers better**. The same hole hides
   with-skill vs baseline asymmetry: if the skill causes timeouts, the
   baseline keeps its samples and the delta is nonsense.
2. **`grade --benchmark` loses timing.** `cmdGrade` builds `RunResult`s
   with `Grading` only (`cmd_grade.go:109-114`) — no `Timing` — so time
   and token stats are zero when benchmarking through that path, while the
   standalone `benchmark` command loads `timing.json` (`cmd_benchmark.go:78-85`).
   Same tool, two different answers.
3. **Iteration deltas compare different corpora.** `iteration_delta`
   (`benchmark.go:59-69`) subtracts iteration N-1's numbers from N's with
   no check that the eval set or model set is the same. Add two evals,
   delete one, switch models — the "trend" silently compares apples to
   oranges. `eval-workflow.md` §5 even encourages pruning assertions
   between iterations, so this happens in normal use.

## Current state

- `runner.go:50-62` — failure detection exists; only `timing.json`
  (duration + tokens, no status) is persisted (`runner.go:64-75`).
- `eval.go:56-59`:

```go
// TimingData captured from an agent run.
type TimingData struct {
	TotalTokens int `json:"total_tokens"`
	DurationMs  int `json:"duration_ms"`
}
```

- `cmd_run.go:227-241` — failed jobs ARE added to `lock.Completed` (the
  lockfile knows the attempted set), but nothing downstream reads it for
  accounting.
- `cmd_grade.go:93-98` — missing outputs → `continue` (skip, no record).
- `cmd_benchmark.go:52-68` — missing/unreadable `grading.json` → `continue`.
- `benchmark.go:199-207` — `aggregateRuns` samples only non-nil gradings:

```go
	for _, r := range results {
		if r.Grading != nil {
			passRates = append(passRates, r.Grading.Summary.PassRate)
		}
		...
```

- `benchmark.go:59-69` — unconditional cross-iteration delta.
- `eval.go:151-168` — `BenchmarkFile` has room for additive fields.

## Design decisions (read before coding)

1. **Persist run status where timing already lives**: add
   `Status string \`json:"status,omitempty"\`` (`"ok"`/`"failed"`) to
   `TimingData`. It is written at the only place runs are observed
   (`runner.go:70-75`) and read by both benchmark paths. No new file.
2. **Failed run = pass rate 0, not a missing sample.** In both benchmark
   paths, a triple whose `timing.json` says `failed` (or which appears in
   the lockfile's `Completed` but has no grading) contributes a pass-rate
   sample of 0 and increments a visible counter. Judge cost is never spent
   on it (grade continues to skip; the zero-fill happens at aggregation).
3. **Report failure counts, don't bury them**: `RunSummary` gains
   `FailedRuns int` and `MissingRuns int` (json omitempty); the report
   renders them. "Missing" = attempted per lockfile but no grading and no
   failed status — e.g. grade was interrupted — distinct from "failed".
4. **Corpus fingerprints**: `BenchmarkFile` gains
   `CorpusHash string` (sha256 over the ordered list of
   `eval.ID + "\x00" + eval.Prompt + "\x00" + strings.Join(eval.Assertions, "\x1f")`)
   and `ModelsHash string` (sha256 over sorted model keys). When the
   previous iteration's hashes differ, **omit** `iteration_delta` and set
   `IterationDeltaSkipped string` with the reason
   (`"eval corpus changed"` / `"model set changed"`). The report explains
   instead of trending.
5. **Timing in the grade path**: extract `cmd_benchmark.go`'s
   grading+timing loader into a shared helper used by both
   `cmdGrade --benchmark` and `cmdBenchmark`, so there is exactly one
   place that materializes `RunResult`s from disk.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |

## Scope

**In scope**:
- `eval.go` (TimingData.Status, RunSummary counters, BenchmarkFile hashes + skip reason)
- `runner.go` (write status), `benchmark.go` (zero-fill, hashes, delta guard)
- `cmd_grade.go` / `cmd_benchmark.go` (shared loader; lockfile-based attempted set)
- `report.go` (render failed/missing counts and the delta-skipped reason)
- Tests: `benchmark_test.go`, `runner_test.go`, grade/benchmark command tests
- `docs/guides/reading-results.md`

**Out of scope**:
- Do NOT retry failed runs automatically (resume already exists for that).
- Do NOT distinguish timeout vs crash vs nonzero-exit in v1 (one `failed`
  status; granularity is a follow-up — note the ctx.DeadlineExceeded
  branch at `runner.go:52-53` as the hook point).
- Do NOT recompute old iterations' benchmarks; hashes/counters appear from
  this version onward, and delta-guard treats a previous benchmark without
  hashes as "unknown corpus" → delta skipped with that reason.

## Git workflow

- Branch: `advisor/018-benchmark-integrity`
- Commit message style: `fix: count failed runs in benchmarks and guard iteration deltas against corpus changes`
- Do NOT push unless instructed.

## Steps

### Step 1: Persist status

`eval.go`: add `Status` to `TimingData`. `runner.go:64-68`: set
`result.Timing.DurationMs` as today plus `Status: result.Status` before
writing. `fixEval`'s timing write (`runner.go:314-320`) sets it too (fix
attempts have their own semantics; write the observed status).

**Verify**: `runner_test.go` — failing stub `CmdBuilder` produces
`timing.json` containing `"status": "failed"`.

### Step 2: Shared result loader

New function (in `cmd_benchmark.go` or a new `results.go`):

```go
// loadRunResults materializes RunResults for an iteration from disk:
// grading.json + timing.json per (eval, model, config), with the legacy
// no-model-key fallback. Triples with no artifacts at all are omitted here;
// attempted-set reconciliation happens in reconcileAttempted.
func loadRunResults(ws string, iter int, ef *EvalFile, models []ModelConfig) []*RunResult
```

Port the existing loop from `cmd_benchmark.go:47-90` (including the legacy
fallback at `:54-60`), also populating `RunResult.Status` from
`TimingData.Status`. Change `cmdBenchmark` to call it. Change `cmdGrade`'s
`--benchmark` branch (`cmd_grade.go:119-121`) to call it too **instead of**
its in-memory `results` (which lack timing) — grading files were just
written, so disk is authoritative and consistent.

**Verify**: `go test ./... -run TestLoadRunResults` — model layout, legacy
layout, timing attached, status propagated.

### Step 3: Reconcile against the attempted set

```go
// reconcileAttempted zero-fills results for triples the lockfile says were
// attempted but which produced no grading. Returns the augmented slice plus
// failed/missing counts per (model, config).
func reconcileAttempted(results []*RunResult, lock *IterationLock) []*RunResult
```

- Read the lock via `readLock(ws, iter)` (`workspace.go:82-93`); if no
  lockfile (pre-005 workspace), skip reconciliation (counters stay 0).
- For each `RunIdentity` in `lock.Completed` with no matching result:
  append `&RunResult{EvalID, Model, Config, Status: "failed"}` with a
  synthetic `Grading` of `Summary{Passed: 0, Failed: 0, Total: 0, PassRate: 0}`?
  **No** — a synthetic grading would corrupt bucket logic (Plan 016).
  Instead give it `Grading: nil` and handle zero-fill in aggregation
  (Step 4) keyed off `Status == "failed"`.
- A loaded result whose `Status == "failed"` but grading exists (agent
  died after writing outputs; judge graded them) keeps its real grading —
  evidence beats status.

**Verify**: `TestReconcileAttempted` — attempted-but-absent triple appears
with failed status; present triples untouched; no lockfile → unchanged.

### Step 4: Honest aggregation

In `benchmark.go` `aggregateRuns` (`:193-215`):

- Pass-rate sampling becomes:

```go
	for _, r := range results {
		switch {
		case r.Grading != nil:
			passRates = append(passRates, r.Grading.Summary.PassRate)
		case r.Status == "failed":
			passRates = append(passRates, 0)
			failed++
		default:
			missing++
		}
		...
	}
```

- `RunSummary` gains `FailedRuns`, `MissingRuns` (json, omitempty);
  timing/token sampling unchanged (failed runs may have real timing —
  include it; it is real cost).
- `computeBenchmark` calls `reconcileAttempted` (needs `ws`/`iter` it
  already has; lock read inside).

**Verify**: `TestAggregateCountsFailures` — 2 ok (pass 1.0) + 1 failed →
mean 0.667, `FailedRuns: 1`; asymmetric case: baseline 2 ok, with_skill
2 ok + 1 failed → delta reflects the failure.

### Step 5: Corpus and model fingerprints

- `benchmark.go`: `func corpusHash(ef *EvalFile) string` and
  `func modelsHash(models []ModelConfig) string` per Design decision 4.
  `computeBenchmark` needs `ef` and `models` — extend its signature
  (callers: `cmd_grade.go:120`, `cmd_benchmark.go:92`; both already have
  both values in scope).
- Delta guard replacing `benchmark.go:63-69`:

```go
	if prev != nil {
		switch {
		case prev.CorpusHash == "" || prev.ModelsHash == "":
			bf.IterationDeltaSkipped = "previous iteration predates corpus fingerprints"
		case prev.CorpusHash != bf.CorpusHash:
			bf.IterationDeltaSkipped = "eval corpus changed since previous iteration"
		case prev.ModelsHash != bf.ModelsHash:
			bf.IterationDeltaSkipped = "model set changed since previous iteration"
		default:
			bf.PreviousIteration = prevIter
			bf.IterationDelta = subtractDeltas(...)
		}
	}
```

**Verify**: `TestIterationDeltaGuard` — same hashes → delta; changed
corpus → skipped with reason; hashless previous → skipped with reason.

### Step 6: Report + docs

- `report.go`: render `FailedRuns`/`MissingRuns` per model when nonzero
  (red badge — failures are the headline, not a footnote), and the
  `IterationDeltaSkipped` reason where the trend block would have been.
- `docs/guides/reading-results.md`: explain failed-vs-missing, the
  zero-fill rule, and why a delta can be "skipped".

### Step 7: Final checks

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

## Test plan

Steps 1–5 each carry tests. Cross-cutting regression: a fully green
workspace (no failures, unchanged corpus) produces numerically identical
`RunSummary` stats to the pre-change code (golden comparison), plus the
new zero-valued/omitted fields only.

## Done criteria

- [ ] `timing.json` records run status; both benchmark paths read it.
- [ ] Failed/attempted-but-ungraded runs contribute pass-rate 0 and visible `failed_runs`/`missing_runs` counts.
- [ ] `grade --benchmark` and `benchmark` produce identical output for the same workspace (shared loader).
- [ ] `iteration_delta` appears only when corpus and model hashes match; otherwise a stated reason appears.
- [ ] Green-path numbers are unchanged vs the previous version.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] Docs updated; `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- The lockfile's `Completed` turns out not to include failed runs in some
  path (re-read `cmd_run.go:227-241` against live code) — the attempted
  set is the foundation here.
- `computeBenchmark`'s signature change collides with Plan 013's pending
  change (activation results param) — coordinate ordering; whoever lands
  second rebases.
- Zero-filling changes headline pass rates so drastically on real
  workspaces that it looks like a regression — that IS the fix working,
  but flag it for the changelog/release notes before merging.

## Maintenance notes

- Any future run-phase artifact (Plan 015's process quality, Plan 011's
  run dirs) should flow through `loadRunResults` — one materialization
  point, forever.
- Follow-up: distinguish `failed_timeout` from `failed_error` statuses;
  the ctx branch in `runner.go:52` already knows.
- Follow-up: `--allow-corpus-change` on a future gate command (Plan 022)
  may consciously override the delta guard; the guard itself stays.
