# Plan 006: Add cross-iteration benchmark diff to surface trends

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat c6dac63..HEAD -- benchmark.go eval.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: direction
- **Planned at**: commit `c6dac63`, 2026-06-25

## Why this matters

`benchmark.json` tells you how this iteration performed, but v1 users need to know whether their skill is improving over time. Comparing the current iteration's delta (with_skill - baseline) to the previous iteration's delta turns the tool from a snapshot-taker into a trend-tracker.

## Current state

- `benchmark.go:14` — `computeBenchmark` only reads the current iteration's grading results.
- `eval.go:110-120` — `BenchmarkFile` and `ModelBenchmark` use the shared `Delta` type.
- The workspace stores `iteration-N/benchmark.json` next to each run's results.

Relevant excerpt from `benchmark.go:14-53`:

```go
func computeBenchmark(results []*RunResult, workspace string, iteration int) error {
	// Group results by model
	byModel := map[string][]*RunResult{}
	for _, r := range results {
		mk := r.Model
		if mk == "" {
			mk = "_" // legacy single-model
		}
		byModel[mk] = append(byModel[mk], r)
	}

	bf := BenchmarkFile{
		GeneratedAt: time.Now(),
	}

	if len(byModel) == 1 {
		// Single model (or legacy) — use flat summary format
		for _, rs := range byModel {
			bf.RunSummary.WithSkill, bf.RunSummary.Baseline = splitAndAggregate(rs)
			bf.RunSummary.Delta = computeDelta(bf.RunSummary.WithSkill, bf.RunSummary.Baseline)
		}
	} else {
		// Multi-model — use models map
		bf.Models = map[string]ModelBenchmark{}
		bestDelta := -999.0
		worstDelta := 999.0
		for mk, rs := range byModel {
			ws, bs := splitAndAggregate(rs)
			mb := ModelBenchmark{WithSkill: ws, Baseline: bs, Delta: computeDelta(ws, bs)}
			bf.Models[mk] = mb

			if mb.Delta.PassRate > bestDelta {
				bestDelta = mb.Delta.PassRate
				bf.BestModel = mk
			}
			if mb.Delta.PassRate < worstDelta {
				worstDelta = mb.Delta.PassRate
				bf.WorstModel = mk
			}
		}
	}
```

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w *.go` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |

## Scope

**In scope**:
- `benchmark.go`
- `eval.go`
- Tests in `benchmark_test.go`

**Out of scope**:
- Do NOT add a new subcommand in this plan; just enrich `benchmark.json`.
- Do NOT change the existing single/multi-model shape structure.
- Do NOT compare across skills or workspaces.

## Git workflow

- Branch: `advisor/006-cross-iteration-benchmark-diff`
- Commit message style: `feat: add cross-iteration delta to benchmark.json`.
- Do NOT push unless instructed.

## Steps

### Step 1: Add iteration delta fields

In `eval.go`, extend `BenchmarkFile`:

```go
// BenchmarkFile is written to benchmark.json.
type BenchmarkFile struct {
	RunSummary struct {
		WithSkill RunSummary `json:"with_skill"`
		Baseline  RunSummary `json:"baseline"`
		Delta     Delta      `json:"delta"`
	} `json:"run_summary,omitempty"`
	Models      map[string]ModelBenchmark `json:"models,omitempty"`
	BestModel   string                    `json:"best_model,omitempty"`
	WorstModel  string                    `json:"worst_model,omitempty"`
	GeneratedAt time.Time                 `json:"generated_at"`

	// New fields
	PreviousIteration int    `json:"previous_iteration,omitempty"`
	IterationDelta    *Delta `json:"iteration_delta,omitempty"`
}
```

Use `*Delta` so it is omitted when there is no previous iteration.

**Verify**: `go build ./...` passes.

### Step 2: Load previous benchmark

In `benchmark.go`, add a helper `loadPreviousBenchmark(workspace string, currentIter int) (*BenchmarkFile, error)` that walks backward from `currentIter-1` to `1` and reads the first `benchmark.json` it finds.

If none is found, return `(nil, nil)` silently.

**Verify**: unit tests pass.

### Step 3: Compute iteration delta

In `computeBenchmark`, after computing current deltas, call `loadPreviousBenchmark`. If found:

- `bf.PreviousIteration = prevIter`
- For single-model: `bf.IterationDelta = subtractDeltas(bf.RunSummary.Delta, prev.RunSummary.Delta)`
- For multi-model: this is trickier. Choose a representative approach:
  - Compute an average delta across models in current and previous, then subtract.
  - Or, if both runs used a single primary model, compare that model's delta.

Recommended for v1: compute the average delta across all models for current and previous, then subtract those averages. Keep it simple.

Add helper:

```go
func subtractDeltas(a, b Delta) *Delta {
	return &Delta{
		PassRate:    a.PassRate - b.PassRate,
		TimeSeconds: a.TimeSeconds - b.TimeSeconds,
		Tokens:      a.Tokens - b.Tokens,
	}
}
```

**Verify**: `go test ./... -run TestIterationDelta` passes.

### Step 4: Update docs

In `eval-workflow.md` or `commands.md` (wherever benchmark output is discussed), mention that `benchmark.json` now includes `previous_iteration` and `iteration_delta` when a previous iteration exists.

**Verify**: `cd docs/site && pnpm build` passes.

### Step 5: Final checks

```bash
gofmt -w *.go
go test ./...
go vet ./...
golangci-lint run
cd docs/site && pnpm build
```

All pass.

## Test plan

- `TestLoadPreviousBenchmark`: finds previous iteration, skips missing files.
- `TestSubtractDeltas`: positive and negative values.
- `TestIterationDeltaSingleModel`: current delta improves over previous.
- `TestIterationDeltaNoPrevious`: `IterationDelta` is nil when no previous benchmark.

## Done criteria

- [ ] `benchmark.json` includes `previous_iteration` and `iteration_delta` when a previous iteration exists.
- [ ] `iteration_delta` shows current-iteration delta minus previous-iteration delta.
- [ ] Missing previous iteration is handled silently (nil delta).
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] Docs build passes.
- [ ] `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- The current `BenchmarkFile` struct has drifted from the excerpt (e.g. `Delta` type does not exist).
- The multi-model average-delta approach turns out to be misleading for the existing test data; in that case report and stop rather than adding complexity.

## Maintenance notes

- If Plan 005 (lockfile) lands first, ensure the previous-iteration lookup respects completed iterations only (read lock status if lockfile exists).
- Reviewers should check that `iteration_delta` sign is intuitive: positive means the skill improved relative to baseline since last iteration.
- Follow-up v1.x: a `skill-eval trend` subcommand that prints a small table across the last N iterations.
