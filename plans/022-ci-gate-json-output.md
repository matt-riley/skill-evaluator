# Plan 022: `skill-eval gate` — CI regression gating and machine-readable output

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- main.go benchmark.go eval.go report.go config.go schema/config-schema.json`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: LOW (read-only over existing artifacts; new command)
- **Depends on**: plans/018-benchmark-integrity.md (soft but strongly recommended — gating on biased numbers automates the bias; the corpus-hash guard and failed-run counts are what make a gate trustworthy)
- **Category**: direction (skill quality as an enforced contract)
- **Planned at**: commit `1325f07`, 2026-07-01

## Why this matters

The tool measures skill quality but cannot *defend* it. Once a skill has a
good benchmark, nothing stops the next edit from silently regressing —
the loop is advisory, human-in-the-loop, local. Skills live in repos; the
natural home for "don't merge a skill change that makes it worse" is CI.
Today that is impossible without parsing HTML or hand-rolling jq over
`benchmark.json` internals that aren't documented as stable.

A `gate` subcommand turns the benchmark into an enforceable contract:
thresholds in, exit code out, JSON for machines. This is also the piece
that makes every earlier plan pay rent — fixtures, honest stats, and judge
hardening all exist so that a CI gate can be *trusted*.

## Current state

- `main.go:36-57` — dispatch; no gate, no `--json` anywhere in the CLI.
- `benchmark.json` is the only machine artifact, and its schema is
  internal (`eval.go:151-168`); nothing documents which fields are stable.
- `benchmark.go:84-101` — `loadPreviousBenchmark` walks backward; the gate
  reuses this pattern for `--against`.
- Exit-code convention: `main.go:17-20` — any error from a subcommand →
  stderr message + exit 1. The gate needs to distinguish "threshold
  violated" from "couldn't evaluate" for CI ergonomics.
- After Plan 018 (if landed): `BenchmarkFile` carries `CorpusHash`,
  `FailedRuns`/`MissingRuns`, `IterationDeltaSkipped` — all gate inputs.

## Design decisions (read before coding)

1. **Gate reads artifacts; it never runs anything.** CI composes:
   `skill-eval run/grade/benchmark` (or `loop -y`) then `skill-eval gate`.
   Keeping execution and judgment separate means the gate is fast,
   rerunnable, and can't spend money.
2. **Checks (all optional, all combinable)**:
   - `--min-pass-rate <0..1>` — with-skill mean pass rate floor
     (cross-model aggregate; per-model with `--per-model`).
   - `--min-delta <float>` — with-skill minus baseline pass-rate floor
     (the skill must beat its baseline by at least this much; `0` =
     "must not be worse").
   - `--max-fail-runs <n>` — ceiling on `failed_runs + missing_runs`
     (0 by default when the flag is present).
   - `--max-token-delta <float>` / `--max-time-delta <seconds>` — cost
     ceilings on the with-skill vs baseline deltas.
   - `--against <iteration>` — compare the target iteration to a specific
     earlier one instead of relying on the stored `iteration_delta`:
     recompute pass-rate difference from the two benchmark files; refuse
     (exit 2) when corpus hashes differ unless `--allow-corpus-change`.
3. **Exit codes**: 0 = all checks pass; 1 = at least one check violated;
   2 = could not evaluate (missing benchmark, corpus mismatch, malformed
   artifacts). CI treats 1 as "block merge", 2 as "pipeline bug".
   Implementation note: `main.go` exits 1 on any returned error — the gate
   must call `os.Exit` itself (or main.go learns typed errors; prefer a
   small `exitCodeError` type handled in `run()` so `main.go` stays the
   single exit point).
4. **`--json` on gate** prints a stable verdict document:

```json
{
  "schema": "skill-eval-gate-v1",
  "iteration": 4,
  "checks": [
    {"name": "min_pass_rate", "threshold": 0.8, "actual": 0.85, "passed": true},
    {"name": "min_delta", "threshold": 0.0, "actual": -0.05, "passed": false}
  ],
  "passed": false
}
```

5. **Stability contract**: this plan also writes
   `docs/guides/ci.md` declaring which `benchmark.json` fields are stable
   consumer API (`models.*.with_skill.pass_rate.mean`, `.delta.*`,
   `generated_at`, and Plan 018's fields) — the gate consumes only those,
   and the doc is the promise to external consumers.
6. **No `--json` retrofit on other commands in this plan** except
   `validate --json` (trivial: the `Finding` list; Plan 009 flagged it as
   follow-up). Benchmark already emits a file; report is human-facing.
7. **Thresholds live in `.skill-eval.yaml` too — the versioned quality
   bar.** A `gate:` block in the config (validated by the schema like
   everything else) holds the same five thresholds as the flags:

   ```yaml
   gate:
     min_delta: 0
     max_fail_runs: 0
     # min_pass_rate: 0.8
   ```

   Precedence: CLI flag > config value > check not run. A bare
   `skill-eval gate` with a populated config block runs the configured
   checks; bare with *no* config block keeps the exit-2 "no checks
   requested" behavior. Why this matters beyond convenience: the config
   is the repo owner's standards, versioned next to the skill — every
   human *and every agent* that clones the repo inherits the same
   definition of "done". For agent-driven loops this is the whole
   contract: "iterate until `skill-eval gate` exits 0" is a complete,
   personality-free stopping condition, and agents have no intrinsic
   threshold of their own — the config supplies it. Use pointer fields
   (`*float64`/`*int`) so absence is distinguishable from zero (a zero
   `min_delta` is meaningful: "must not be worse").
8. **The checks are regression-shaped by design — keep them that way.**
   Deltas, regressions, and failure counts are Goodhart-resistant:
   gaming them requires actually improving the skill. Absolute pass rate
   is not — an optimizer (human or agent) told to reach 100% can get
   there by overfitting the skill to the eval suite or softening
   assertions, and per the tool's own philosophy a suite that sits at
   100% has stopped teaching you anything (always-passing assertions are
   dead weight; healthy suites get harder over time). `--min-pass-rate`
   stays available as a floor, but the docs (Step 6) must carry the
   guardrail explicitly, and no default, example, or error message in
   this plan should nudge toward pass-rate perfection.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |
| Manual | `go run . gate --min-delta 0 --json` | verdict JSON, exit per result |

## Scope

**In scope**:
- New `cmd_gate.go` + `cmd_gate_test.go`
- `main.go` (dispatch, usage, `exitCodeError` handling)
- `cmd_validate.go` (`--json` flag emitting findings)
- `config.go` + `schema/config-schema.json` (the `gate:` thresholds block)
- New `docs/guides/ci.md` (stable-fields contract + GitHub Actions example + the Goodhart guardrail)

**Out of scope**:
- Do NOT run evals from the gate.
- Do NOT post to GitHub/statuses/PRs — exit codes are the integration
  surface; wrappers belong in CI config, and the guide shows how.
- Do NOT gate on activation metrics or process quality in v1 (add checks
  when Plans 013/015 land — the check list is designed to grow).
- Do NOT weight, colorize, or editorialize failure data in any JSON
  output — the data layer stays neutral; urgency is expressed as
  explicit thresholds and exit codes, never as tone in the record.

## Git workflow

- Branch: `advisor/022-ci-gate-json-output`
- Commit message style: `feat: add gate subcommand for CI regression thresholds with JSON verdicts`
- Do NOT push unless instructed.

## Steps

### Step 1: Typed exit codes

In `main.go`:

```go
// exitCodeError carries a specific process exit code through run().
type exitCodeError struct {
	code int
	msg  string
}

func (e *exitCodeError) Error() string { return e.msg }
```

In `main()` (`main.go:16-21`): `errors.As` for `*exitCodeError` → print
msg (unless empty) and `os.Exit(e.code)`; otherwise current behavior.

**Verify**: `go test ./... -run TestExitCodeError` (unit-test `run()`'s
error classification if feasible; otherwise cover via gate tests).

### Step 2: Gate core

New `cmd_gate.go`:

```go
type gateCheck struct {
	Name      string  `json:"name"`
	Threshold float64 `json:"threshold"`
	Actual    float64 `json:"actual"`
	Passed    bool    `json:"passed"`
}

type gateVerdict struct {
	Schema    string      `json:"schema"` // "skill-eval-gate-v1"
	Iteration int         `json:"iteration"`
	Checks    []gateCheck `json:"checks"`
	Passed    bool        `json:"passed"`
}

func cmdGate(ctx context.Context, args []string) error
```

- Flags per Design decision 2 plus `--iteration N` (default latest,
  resolved like `report.go`'s `--iteration`), `--json`, `--per-model`.
- Float flags need presence detection (a zero threshold is meaningful):
  use `flag.String` + parse, or track with `fs.Visit` — pick one, test it.
- Load `benchmark.json` via the existing loader (`loadBenchmarkFile`,
  `report.go:190-202` — it is exactly this: workspace + iter → file).
- Evaluate only requested checks; no flags AND no config `gate:` block
  (Step 5) → exit 2 with "no checks requested — pass flags or add a
  gate: block to .skill-eval.yaml".
- Human output: one line per check
  (`PASS min_pass_rate: 0.85 >= 0.80`), summary line, then return
  `&exitCodeError{code: 1, msg: "gate failed"}` on violation, nil on pass.
- `--per-model`: apply pass-rate/delta checks to every entry in
  `bf.Models` (worst model must pass); actual reported is the worst.

**Verify**: `go test ./... -run TestGateChecks` — fixture benchmark files;
each check passing/failing; exit-code mapping; zero-threshold `--min-delta 0`
distinguishable from flag-absent.

### Step 3: `--against`

- Load both iterations' benchmarks; if either lacks `CorpusHash` or they
  differ → exit 2 with the reason (unless `--allow-corpus-change`, which
  degrades to a warning). Pre-018 files have empty hashes → "unknown
  corpus" exit-2 path with a message naming Plan 018's version as the fix.
- Check `min_delta_against`: (target with-skill mean − against with-skill
  mean) ≥ threshold via `--min-improvement <float>` (naming: improvement
  vs a *past iteration*, distinct from `--min-delta`'s baseline-config
  meaning — document the distinction prominently).

**Verify**: `TestGateAgainst` — improvement pass/fail; corpus mismatch →
exit 2; `--allow-corpus-change` proceeds.

### Step 4: `validate --json`

In `cmd_validate.go`: `--json` prints
`{"schema":"skill-eval-validate-v1","findings":[{rule,severity,message}...]}`
and suppresses the human lines; exit contract unchanged.

**Verify**: `TestValidateJSON`.

### Step 5: Thresholds in `.skill-eval.yaml`

Per Design decision 7:

- `config.go`: add to `Config`:

```go
// GateConfig is the repo's versioned quality bar; nil fields mean
// "check not configured". Zero values are meaningful (min_delta: 0 =
// "must not be worse"), hence pointers.
type GateConfig struct {
	MinPassRate   *float64 `yaml:"min_pass_rate"`
	MinDelta      *float64 `yaml:"min_delta"`
	MaxFailRuns   *int     `yaml:"max_fail_runs"`
	MaxTokenDelta *float64 `yaml:"max_token_delta"`
	MaxTimeDelta  *float64 `yaml:"max_time_delta"`
}
```

  with `Gate GateConfig \`yaml:"gate"\`` on `Config` and non-nil copy in
  `mergeConfig` (global config may set an org-wide bar; skill config
  overrides per field).
- `schema/config-schema.json`: add the `gate` object with the five
  numeric properties (bounds: min_pass_rate 0..1, max_fail_runs ≥ 0).
- `cmd_gate.go`: resolution order per check = CLI flag if set, else
  config field if non-nil, else check not run. The verdict's `checks`
  entries gain `"source": "flag" | "config"` so CI logs show where the
  bar came from.

**Verify**: `go test ./... -run 'TestGateConfigThresholds|TestGateFlagOverridesConfig|TestMergeConfigGate'`
— config-only run executes configured checks; a flag overrides the same
check's config value; per-field merge (global sets min_delta, skill sets
max_fail_runs → both active); invalid bounds rejected by schema
validation.

### Step 6: Wire-up, docs, final checks

- `main.go`: `case "gate":` + usage lines.
- `docs/guides/ci.md`:
  - the stable-fields contract table;
  - exit-code table (0/1/2);
  - the `gate:` config block with the flag-precedence rule, framed as
    "your repo's versioned quality bar — every human and agent that
    clones the skill inherits it", including the agent-loop contract
    sentence: *for agent-driven iteration, "improve the skill until
    `skill-eval gate` exits 0" is a complete stopping condition*;
  - **the guardrail, verbatim or close to it**: *set your bar on
    regressions and deltas, not on reaching 100% — a suite you can
    saturate is a suite that's stopped teaching you anything. Prefer
    `min_delta`/`max_fail_runs`; treat a climbing pass rate as a
    prompt to add harder evals, and be wary of optimizing (or asking an
    agent to optimize) `min_pass_rate` toward 1.0 — that incentivizes
    overfitting the skill to the eval suite and softening assertions
    rather than genuine improvement*;
  - a complete GitHub Actions example:

```yaml
- run: skill-eval loop -y --baseline previous --runs 3
- run: skill-eval gate --json   # thresholds come from .skill-eval.yaml's gate: block
```

  with the note that agent/judge credentials are the workflow's problem
  and a caution about judge cost on every push (gate on schedule/label,
  not every commit, for expensive corpora).

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

## Test plan

- `TestGateChecks` (per-check table, both outcomes each).
- `TestGateExitCodes` (0/1/2 paths).
- `TestGatePerModel` (worst-model semantics).
- `TestGateAgainst` (+corpus guard both ways).
- `TestGateJSONSchema` (golden verdict document; field names locked, incl. `source`).
- `TestValidateJSON`.
- `TestGateNoBenchmark` (fresh workspace → exit 2, actionable message).
- `TestGateConfigThresholds` / `TestGateFlagOverridesConfig` /
  `TestMergeConfigGate` (Step 5; per-field precedence and merge).
- `TestGateNoChecksAnywhere` (no flags, no config block → exit 2 naming both options).

## Done criteria

- [ ] `skill-eval gate` evaluates any combination of the five threshold checks against an iteration's benchmark and exits 0/1/2 per the contract.
- [ ] Thresholds are configurable via the `gate:` block in `.skill-eval.yaml` (schema-validated, per-field merge, flags override); verdict entries record their `source`.
- [ ] A bare `skill-eval gate` runs the repo's configured bar; with neither flags nor config it exits 2 naming both options.
- [ ] `--against` compares iterations with the corpus-hash guard.
- [ ] `--json` verdicts and `validate --json` findings have locked schemas (`*-v1`); no JSON output editorializes failure.
- [ ] `docs/guides/ci.md` documents stable fields, exit codes, the config block with the agent-loop contract sentence, the regressions-not-perfection guardrail, and a working Actions example.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- Plan 018 is unlanded AND the user wants `--max-fail-runs` — that check
  reads 018's fields; ship the gate without it (documented) or land 018
  first; do not fake it from grading presence.
- The `exitCodeError` pattern conflicts with how any existing command
  signals errors (grep for direct `os.Exit` calls first).
- Gate output schemas would need to change within the release they ship —
  the `-v1` suffix is a promise; get the field names reviewed before
  merging.

## Maintenance notes

- New benchmark axes (activation precision from Plan 013, process-quality
  deltas from Plan 015) should each add a gate check in the same release
  they stabilize — the gate is the tool's public contract for "what
  quality means".
- Keep `gateVerdict` flat and boring; CI parsers live forever.
- Guard the regression-shaped philosophy in review: any future check,
  example, or default that pushes toward absolute pass-rate perfection
  should be challenged with Design decision 8 — Goodhart-resistant
  checks (deltas, regressions, failure counts) are the gate's identity.
- The `gate:` config block is the natural home for future axes' thresholds
  (activation precision, process-quality deltas) — extend the block and
  the schema together, one release after the axis stabilizes.
