# Plan 015: Process-quality benchmarking — run `agit eval` on the eval runs themselves

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- runner.go cmd_run.go benchmark.go eval.go internal/agit/client.go report.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: M-L
- **Risk**: HIGH (best-effort correlation between skill-eval runs and agit sessions; must degrade to a no-op cleanly)
- **Depends on**: — (soft: plans/011-repeat-runs-statistics.md shares the per-run artifact layout; land 011 first if both are scheduled)
- **Category**: direction (deepest agengit integration)
- **Planned at**: commit `1325f07`, 2026-07-01
- **Research**: `docs/research/agengit-integration.md` §2.7

## Why this matters

Grading is output-based: the judge sees produced files. But a skill's value
is often in the *process* — fewer repeated failures, more verification,
less churn — and two runs with identical outputs can differ wildly in cost
and flail. skill-evaluator already shells out to the same agent CLIs
(claude, pi, codex, gemini, copilot) that agengit hooks into; **if agit
hooks are installed, every eval run is itself recorded as an agit
session.** Running `agit eval` on those fresh sessions and attaching the
six quality dimensions to each `RunResult` gives the benchmark a second
axis: "did the skill make the agent *work better*, not just produce passing
output?" A skill that yields the same pass rate with a 30-point churn-risk
drop is a better skill, and today the tool cannot see that.

This is opt-in (`--process-quality`) and strictly best-effort: no agit, no
hooks, no matching session → the run proceeds exactly as today.

## Current state

- `runner.go:42-48` — run timing exists (`start := time.Now()`,
  `elapsed`), but wall-clock start/end are not persisted; `timing.json`
  (`runner.go:70-75`) stores only duration + tokens.
- `internal/agit/types.go:89-94` — `SessionRow` carries `Origin`,
  `SessionID`, `UpdatedAt` (unix ms) — the correlation surface.
- `internal/agit/client.go:83-89` — `FetchSessions` exists;
  `FetchEvalReport(target)` exists (`:93-103`).
- `internal/agit/convert.go:23-40` — `EvalQualityScore` already collapses
  the six dimensions to 0-100 (reuse for the summary number).
- `benchmark.go:12-80` — `computeBenchmark` aggregates `RunResult`s;
  `eval.go:99-106` — `RunResult` has no process field.
- `cmd_run.go:196-224` — runs execute concurrently (`--parallel`, default
  2), which makes *time-window* correlation across simultaneous runs
  ambiguous — the design below must not rely on timestamps alone.

## Design decisions (read before coding)

1. **Correlation strategy — probe first, timestamps last.** In order of
   preference (Step 1 decides which is available):
   a. **Env/ID injection**: if the installed agit exposes a way to tag or
      predetermine a session ID (env var like `AGIT_SESSION_ID`, or the
      hooks record the agent process's env) — use it: generate a UUID per
      run, set it in `cmd.Env`, match exactly. Check `agit init`-generated
      hook scripts and `agit observe --help` for any such channel.
   b. **Workdir scoping**: agit stores per-repo; if hermetic workdirs
      (Plan 007) are in play and agit records per-directory origins,
      the freshest session in that scope is unambiguous.
   c. **Time-window fallback**: sessions with `UpdatedAt` within
      `[start-5s, end+5s]` of the run AND matching the agent's origin name
      (`claude`, `codex`, `pi`, …). If more than one candidate matches
      (parallel runs of the same agent), mark the run's process quality as
      `"ambiguous"` and attach nothing — never guess.
2. **Artifact**: per run, `process_quality.json` beside `timing.json`:

```json
{
  "matched_session": "claude/abc123",
  "match_method": "env|window",
  "classification": "good",
  "quality_score": 74,
  "dimensions": { "goal_clarity": 80, "execution_focus": 71, "failure_recovery": 60,
                   "verification": 55, "completion_signal": 90, "churn_risk": 30 }
}
```

3. **Benchmark**: `ModelBenchmark` gains
   `ProcessQuality *ProcessQualitySummary` — mean quality score and mean
   per-dimension scores for with_skill vs baseline plus the delta, computed
   only from runs that matched unambiguously; the summary records
   `MatchedRuns`/`TotalRuns` so consumers see coverage.
4. **Flag**: `--process-quality` on `run` and `loop`. Off by default.
   When on but `agit` is not in PATH → one warning, then no-op.
5. **Timing capture**: persist run start/end (RFC3339) into `timing.json`
   (additive fields) regardless of the flag — cheap, useful for debugging,
   and required by the fallback correlator.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Probe session tagging | inspect hooks after `agit init`; `agit observe --help`; `agit sessions --json` | see Step 1 |
| Build | `go build ./...` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |

## Scope

**In scope**:
- `runner.go` (persist start/end; `cmd.Env` tagging if available)
- New `process_quality.go` + tests (correlator + fetch + artifact)
- `cmd_run.go`, `cmd_loop.go` (flag; post-run collection pass)
- `eval.go` (`TimingData` additive fields; `RunResult.Process`; summary types)
- `benchmark.go` (+summary), `report.go` (render dimension deltas)
- `internal/agit/client.go` only if a session-scoping call is missing
- `docs/guides/process-quality.md` (new)

**Out of scope**:
- Do NOT install or configure agit hooks for the user (`agit init` is
  theirs to run; the guide explains it).
- Do NOT block or fail runs on any agit error — every failure path logs
  once and produces a missing/ambiguous artifact.
- Do NOT feed process dimensions into pass/fail grading — they are a
  separate reporting axis, never a gate (in this plan).
- Do NOT correlate fix-phase attempts (`fixEval`) in v1.

## Git workflow

- Branch: `advisor/015-process-quality-benchmarking`
- Commit message style: `feat: attach agit process-quality dimensions to eval runs behind --process-quality`
- Do NOT push unless instructed.

## Steps

### Step 1: Probe the correlation channel

With agit installed and hooks active for at least one agent:
1. Run `agit init` in a scratch repo; read the generated hook files —
   do they forward the agent's environment? Is any env var recorded onto
   the session/step?
2. `agit sessions --json` — confirm `UpdatedAt` granularity and origin
   naming per agent runtime (map agit origin strings to our agent names:
   `claude`, `codex`, `pi`, `gemini`, `copilot`).
3. Trigger one manual agent invocation and confirm the session appears and
   is attributable.

Record which of strategies (a)/(b)/(c) is implementable. If only (c), the
plan proceeds but `--parallel > 1` with duplicate agents degrades to
"ambiguous" — acceptable, documented.

### Step 2: Persist run timestamps (+optional tag)

- `eval.go` `TimingData`: add
  `StartedAt time.Time \`json:"started_at,omitempty"\`` and
  `EndedAt time.Time \`json:"ended_at,omitempty"\``.
- `runEval`: populate both around the existing `start`/`elapsed`
  (`runner.go:42-48`).
- If strategy (a) exists: generate `runTag := uuid-ish` (use
  `crypto/rand` hex; no new dependency), set
  `cmd.Env = append(os.Environ(), "SKILL_EVAL_RUN_TAG="+runTag)` and store
  the tag in `TimingData` too.

**Verify**: `go test ./... -run TestTimingTimestamps`.

### Step 3: The correlator

New `process_quality.go`:

```go
// ProcessQuality is the per-run artifact written as process_quality.json.
type ProcessQuality struct {
	MatchedSession string         `json:"matched_session,omitempty"`
	MatchMethod    string         `json:"match_method,omitempty"` // "env" | "window" | "" (unmatched) | "ambiguous"
	Classification string         `json:"classification,omitempty"`
	QualityScore   int            `json:"quality_score,omitempty"`
	Dimensions     map[string]int `json:"dimensions,omitempty"`
}

// collectProcessQuality matches completed runs to agit sessions and runs
// agit eval on each match. Pure best-effort: every error path returns a
// ProcessQuality documenting why nothing was attached.
func collectProcessQuality(runs []*RunResult, timings map[runKey]*TimingData, agent string) map[runKey]*ProcessQuality
```

Implementation notes:
- One `agit.FetchSessions()` call for the whole batch (not per run).
- Apply the strategy from Step 1; the window matcher takes the run's
  `StartedAt/EndedAt` ±5s and the origin mapping table.
- Per matched session: `agit.FetchEvalReport(origin + "/" + sessionID)`;
  map `Dimensions` via a small extractor (reuse
  `agit.EvalQualityScore` for the summary; invert churn consistently
  with `convert.go:33`).
- Sessions matched by two different runs → both become `"ambiguous"`.

**Verify**: table-driven `process_quality_test.go` with a swapped
`runAgit`/fetch layer: exact-tag match; single window match; two runs one
window → ambiguous; no agit → all unmatched, zero errors surfaced.

### Step 4: Wire into the run phase

`cmd_run.go`:
- `pq := fs.Bool("process-quality", false, "Correlate runs with agit sessions and attach agit eval dimensions")`.
- After the per-eval `wg.Wait()` loop completes for ALL evals (i.e. after
  the main loop, before the final lock write): when `*pq`, call the
  collector for the iteration's results and write each non-nil artifact to
  `<evalDir>/<config>/process_quality.json` (or `run-<r>/` under Plan 011's
  layout — use `runConfigPath` if it exists).
- `cmd_loop.go`: forward the flag.

**Verify**: `go test ./... -run TestRunProcessQualityArtifacts` (stubbed
collector).

### Step 5: Benchmark + report

- `eval.go`:

```go
// ProcessQualitySummary aggregates matched runs' agit dimensions.
type ProcessQualitySummary struct {
	MatchedRuns int                `json:"matched_runs"`
	TotalRuns   int                `json:"total_runs"`
	QualityMean float64            `json:"quality_mean"`
	Dimensions  map[string]float64 `json:"dimensions"` // mean per dimension
}
```

  `ModelBenchmark` gains `ProcessWithSkill`, `ProcessBaseline`
  (`*ProcessQualitySummary`, omitempty) and `ProcessDelta map[string]float64`.
- `benchmark.go`: read `process_quality.json` files when aggregating (the
  benchmark command already walks the iteration tree for grading files —
  mirror that pattern in `cmd_benchmark.go`; check how it loads gradings
  first and match it).
- `report.go`: when present, render a "Process quality (agit)" table —
  per-dimension with_skill vs baseline vs delta, plus the coverage line
  (`matched 7/12 runs`).

**Verify**: `TestSummarizeProcessQuality` (hand-computed means/deltas,
unmatched runs excluded, coverage counts right); `TestReportRendersProcessQuality`.

### Step 6: Docs and final checks

`docs/guides/process-quality.md`: prerequisites (`agit init` + hooks for
the agent runtime you benchmark), what the six dimensions mean (link
agengit's eval-v1 docs), the ambiguity story with `--parallel`, and an
honest "signals, not guarantees" framing (agit's own disclaimer).

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

## Test plan

Steps 3–5 carry the substance. The non-negotiable regression test:
`--process-quality` on a machine with **no agit at all** completes the run
with a single warning and produces a benchmark identical to a run without
the flag (plus absent process fields).

## Done criteria

- [ ] `run --process-quality` attaches `process_quality.json` to matched runs; unmatched/ambiguous runs are explicit, never guessed.
- [ ] `timing.json` gains start/end timestamps unconditionally.
- [ ] Benchmark and report show per-dimension with_skill vs baseline deltas with coverage counts.
- [ ] Zero behavior change without the flag; clean no-op without agit.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] New guide; `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- Step 1 finds no reliable correlation channel AND typical usage is
  `--parallel > 1` with a single agent — window matching would mark nearly
  everything ambiguous; report and propose serializing runs under the flag
  instead (`--process-quality` implies `--parallel 1`) as a design change.
- `agit eval` on a fresh session is slow (>5s per session) — batch or cache
  before shipping, or the post-run pass becomes the bottleneck.
- Session `UpdatedAt` turns out to be coarser than seconds.

## Maintenance notes

- The origin-name mapping table (agit origins ↔ skill-eval agent names) is
  a maintenance hotspot: update it when either tool adds a runtime.
- If agit later exposes first-class session tagging, delete the window
  correlator rather than keeping both paths.
- Keep process metrics strictly out of grading/gating until there is real
  corpus evidence that they are stable enough to gate on.
