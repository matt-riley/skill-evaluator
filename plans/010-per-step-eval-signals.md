# Plan 010: Use `agit eval --include-steps` for per-step quality filtering

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- internal/agit/client.go internal/agit/types.go internal/agit/convert.go cmd_import_agit.go eval.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: S
- **Risk**: LOW-MED (depends on agit ≥1.26 flag behavior; graceful fallback required)
- **Depends on**: —
- **Category**: signal quality
- **Planned at**: commit `1325f07`, 2026-07-01
- **Research**: `docs/research/agengit-integration.md` §2.2

## Why this matters

`agit eval` scores sessions on six dimensions, and agit ≥1.26 can break
that down per step (`--include-steps` → `step_assessments`, keyed by step
hash, each carrying the 10 signal counters — see agengit
`docs/format/eval-v1.md`). skill-evaluator fetches only the session-level
report, so **one classification is stamped onto every eval imported from a
session**. Real sessions are mixed: a "mixed" session usually contains
clean turns worth importing and churny turns worth skipping. Today
`--eval-filter good` throws away the whole mixed session; without a filter
you import the churn too. Per-step signals fix both, and storing them in
`EvalSource` lets later analysis (Plan 016's report buckets) correlate
failing evals with the quality of the turn they came from.

## Current state

- `internal/agit/client.go:93-103` — no `--include-steps`:

```go
func FetchEvalReport(session string) (*EvalReport, error) {
	args := []string{"eval", "--json"}
	if session != "" {
		args = append(args, session)
	}
	out, err := runAgit(args...)
	if err != nil {
		return nil, fmt.Errorf("agit eval: %w", err)
	}
	return decodeEnvelope[EvalReport](out)
}
```

- `internal/agit/types.go:101-107` — `EvalReport` has no
  `step_assessments` field.
- `internal/agit/convert.go:87-103` — `ConvertSteps` applies the
  **session-level** classification/filter once, before the step loop; every
  emitted eval gets the same `QualityScore`/`Classification`.
- `internal/agit/convert.go:310-347` — `buildAssertionsWithSignals` already
  wants per-step signal data and works around not having it.
- `eval.go:23-31` — `EvalSource` has session-level `QualityScore` and
  `Classification` only.
- `internal/agit/types.go:139-146` — `Signals` struct exists with 6 of the
  10 documented counters; eval-v1 also documents `concrete_terms`,
  `success_criteria_phrases`, `related_tool_calls`, `repeated_failures`,
  `final_summary_terms`, `steps` (verify exact set against live output).

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Probe | `agit eval --include-steps --json \| head -c 4000` | `step_assessments` array visible |
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |

## Scope

**In scope**:
- `internal/agit/client.go`, `internal/agit/types.go`, `internal/agit/convert.go`
- `cmd_import_agit.go` (new flag plumbed through)
- `eval.go` (additive `EvalSource` fields)
- Tests in `internal/agit/client_test.go`, `internal/agit/convert_test.go`
- `docs/guides/importing-agit-sessions.md`

**Out of scope**:
- Do NOT apply per-step filtering on the legacy `log+show+diff` fallback
  path (`ConvertSession`) — it has no step assessments; session-level
  filtering there is unchanged.
- Do NOT remove or change the meaning of the existing session-level
  `--eval-filter` (it remains the session gate).
- Do NOT invent per-step *classifications* — eval-v1 gives per-step
  *signals* only; thresholds live in our code and must be documented.

## Git workflow

- Branch: `advisor/010-per-step-eval-signals`
- Commit message style: `feat: per-step agit eval signals for import filtering and provenance`
- Do NOT push unless instructed.

## Steps

### Step 1: Probe the live shape

Run `agit eval --include-steps --json` against a recorded session. Confirm:
- envelope field name (`step_assessments`), keys per entry
  (`hash`, `turn_id`, `timestamp`, `signals`),
- the exact signal counter names (eval-v1 documents 10; `types.go` has 6 —
  reconcile),
- behavior on older agit versions (unknown-flag error? Capture the error
  string for the fallback path).

**Verify**: you can name the exact JSON keys before editing `types.go`.

### Step 2: Types

In `internal/agit/types.go`:

```go
type EvalReport struct {
	// ... existing fields ...
	StepAssessments []StepAssessment `json:"step_assessments,omitempty"`
}

type StepAssessment struct {
	Hash      string  `json:"hash"`
	TurnID    string  `json:"turn_id"`
	Timestamp int64   `json:"timestamp"`
	Signals   Signals `json:"signals"`
}
```

Extend `Signals` with the counters confirmed in Step 1 (additive,
`omitempty` not needed — ints zero-default):

```go
type Signals struct {
	// ... existing 6 ...
	ConcreteTerms          int `json:"concrete_terms"`
	SuccessCriteriaPhrases int `json:"success_criteria_phrases"`
	RelatedToolCalls       int `json:"related_tool_calls"`
	RepeatedFailures       int `json:"repeated_failures"`
	FinalSummaryTerms      int `json:"final_summary_terms"`
	Steps                  int `json:"steps"`
}
```

**Verify**: `go build ./...`.

### Step 3: Client

Change `FetchEvalReport` to request steps, with graceful degradation:

```go
func FetchEvalReport(session string) (*EvalReport, error) {
	args := []string{"eval", "--json", "--include-steps"}
	if session != "" {
		args = append(args, session)
	}
	out, err := runAgit(args...)
	if err != nil {
		// Older agit may not support --include-steps; retry without.
		Logger.Info("agit eval --include-steps failed, retrying without", "error", err)
		args = []string{"eval", "--json"}
		if session != "" {
			args = append(args, session)
		}
		if out, err = runAgit(args...); err != nil {
			return nil, fmt.Errorf("agit eval: %w", err)
		}
	}
	return decodeEnvelope[EvalReport](out)
}
```

**Verify**: `client_test.go` — swap `runAgit`; assert first call carries
`--include-steps`; simulate first-call failure → second call without the
flag succeeds.

### Step 4: Per-step gating in ConvertSteps

In `internal/agit/convert.go`:

1. Add a step-signal index and a skip predicate:

```go
// stepAssessmentIndex maps step hash -> assessment for O(1) lookup.
func stepAssessmentIndex(ae *EvalReport) map[string]StepAssessment

// skipStepByQuality reports whether a step's own signals disqualify it
// when the caller asked for good-only imports. Thresholds (documented in
// the import guide): repeated_failures > 0, or repeated_commands > 3, or
// (error_results > 0 && recovered_errors == 0).
func skipStepByQuality(sa StepAssessment, strict bool) bool
```

2. In `ConvertSteps`, after the existing prompt/diff/ack filters, when the
   caller passed `strictSteps=true` (new parameter — see Step 5) and an
   assessment exists for `row.Hash`, apply `skipStepByQuality`; log skipped
   steps at Info with hash and the triggering signal.
3. Store per-step provenance on the converted eval: add to
   `agit.EvalSource` (the one in `convert.go:74-82`):

```go
	StepSignals *Signals // per-step counters when --include-steps was available
```

4. In `buildAssertionsWithSignals`, when `StepSignals` is available for the
   step, use the **step's** counters instead of the session dimensions for
   the churn-trim heuristic (`convert.go:330-334`).

**Verify**: `go test ./internal/agit -run TestConvertStepsPerStepFilter`.

### Step 5: Flag plumbing and main-schema provenance

- `cmd_import_agit.go`: add
  `strictSteps := fs.Bool("strict-steps", false, "Skip individual steps whose agit eval signals show churn or unrecovered failures")`,
  pass it to `agit.ConvertSteps` (signature gains the parameter; update the
  one other caller if any — grep first).
- `eval.go` `EvalSource`: additive fields mirroring the signals we act on
  (keep the JSON small — store the three inputs of the predicate, not all 10):

```go
	StepRepeatedFailures int `json:"step_repeated_failures,omitempty"`
	StepRepeatedCommands int `json:"step_repeated_commands,omitempty"`
	StepVerification     int `json:"step_verification_commands,omitempty"`
```

- Map them in `evalFromConverted` (`cmd_import_agit.go:20-36`).

**Verify**: `go build ./...`; `go run . import-agit --help` shows the flag.

### Step 6: Tests

- `TestFetchEvalReportIncludeSteps` / `...FallbackWithoutFlag` (Step 3).
- `TestConvertStepsPerStepFilter`: session classified "mixed", two steps —
  one clean, one with `repeated_failures: 2`; `strictSteps=true` imports
  only the clean one; `strictSteps=false` imports both. Session-level
  `--eval-filter` still gates the whole session first.
- `TestConvertStepsStoresStepSignals`: emitted eval carries the step's
  counters, not the session's.
- `TestChurnTrimUsesStepSignals`: assertion trimming responds to step
  signals when present.

### Step 7: Documentation and final checks

Document `--strict-steps` and the exact thresholds in
`docs/guides/importing-agit-sessions.md` (thresholds are ours, not agit's —
say so). Then:

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

## Test plan

See Step 6. The fallback test matters most operationally — users on older
agit must see no behavior change.

## Done criteria

- [ ] `FetchEvalReport` requests `--include-steps` and degrades gracefully on older agit.
- [ ] `import-agit --strict-steps` filters individual steps by their own signals; default behavior unchanged.
- [ ] Imported evals record per-step signal provenance in `EvalSource`.
- [ ] Thresholds documented in the guide as skill-evaluator policy.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- Live `step_assessments` keys differ from eval-v1 docs (report actual JSON).
- `--include-steps` on the installed agit changes the *session-level*
  fields' shape too (schema fork) — do not paper over with custom decoding.
- The `ConvertSteps` signature change ripples beyond `cmd_import_agit.go`
  and its tests.

## Maintenance notes

- Threshold constants belong next to `skipStepByQuality` with the guide
  cross-referenced; tune them from real corpora, not intuition.
- Plan 016 (report buckets) reads the `Step*` provenance fields — keep
  their JSON names stable once released.
