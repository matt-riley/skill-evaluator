# Plan 033: The assertion ledger — identity, history, flips, and retirement

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- eval.go grader.go workspace.go benchmark.go report.go main.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: LOW-MED (read-only over persisted artifacts; the risk is interpretive — the join key must be honest about assertion edits)
- **Depends on**: — (soft: plans/017 for snapshot diffs in `--flips`; plans/018 for corpus-hash interpretation guards; plans/016 shares bucket terminology — land 016 first or align the vocabulary deliberately)
- **Category**: direction (assertion-centric identity)
- **Planned at**: commit `1325f07`, 2026-07-01
- **Origin**: the assertion-centrism discussion — the assertion is the unit of meaning for both humans and agents; the tool's numbers are all aggregates of assertion verdicts, yet assertions have no identity and no history.

## Why this matters

Every number the tool produces is an aggregate of assertion verdicts, and
the assertion is the one artifact both audiences consume natively — a
human reads it as intent, an agent evaluates it as a predicate. Yet in the
data model an assertion is an anonymous string inside an eval: no
identity, no memory. Three questions that assertion-centrism makes obvious
are therefore unanswerable today:

1. **Attribution**: "assertion X flipped fail→pass between iterations 2
   and 3 — what changed in the skill?" This is the closest the tool can
   ever get to *which instruction moved which outcome*, and with Plan
   017's always-snapshot convention the answer is literally sitting on
   disk (diff the two iterations' `skill-snapshot/SKILL.md`). Nothing
   joins the flip to the diff.
2. **Retirement**: the docs tell users to prune assertions that always
   pass (`eval-workflow.md` §5, "Toss out assertions that always pass"),
   but "always" is a longitudinal claim and nothing computes it — users
   must eyeball grading files across iterations by hand. Dead-weight
   assertions accumulate, diluting pass rates and wasting judge tokens.
3. **Instrument stability**: when an assertion's verdict oscillates
   across iterations with no skill change, that assertion (or the judge
   grading it) is noisy — exactly the thing to know before trusting a
   delta. Per-assertion verdict history across iterations is the noise
   detector at the right granularity.

Plan 016's buckets answer "what did assertions do *this* iteration"; the
ledger is the longitudinal complement: "what has each assertion done
*across* iterations." Together they make the assertion a first-class,
trackable instrument instead of a disposable string.

## Current state

- `eval.go:12-19` — `Eval.Assertions []string`; no identity anywhere.
- `grading.json` persists per (iteration, eval, model, config):
  `AssertionResults[].Text/Passed/Evidence` (`eval.go:62-66`) — the raw
  material for a ledger already exists on disk for every past iteration.
- `workspace.go:41-53` — `nextIteration` shows the iteration-enumeration
  pattern; `evalPath` (`:27-33`) the artifact layout, including the
  model-key nesting and the legacy no-model-key layout that
  `cmd_benchmark.go:54-60` falls back to — the ledger walker must handle
  both, and `run-<r>/` dirs if Plan 011 landed.
- `benchmark.go:84-101` — `loadPreviousBenchmark`'s walk-backward pattern.
- After Plan 017: `iteration-N/skill-snapshot/` exists per iteration —
  the flip-diff source.
- After Plan 018: `benchmark.json` carries `CorpusHash` — the guard for
  "same corpus" claims across iterations.
- After Plan 016: buckets exist per-iteration in `benchmark.json`; the
  ledger reuses its bucket names (skill_value / broken_both / dead_weight
  / baseline_wins) as the per-iteration classification vocabulary.
- Assertion text round-trips exactly through grading (`parseGradingOutput`
  maps results by position and the text is echoed back; Plan 016 has a
  STOP condition on this same property — it holds or both plans stop).

## Design decisions (read before coding)

1. **Identity = content hash, scoped to the eval.** An assertion's ID is
   `sha256(evalID + "\x00" + normalize(text))[:12]` where `normalize` is
   trim-whitespace-collapse-inner-runs (case-sensitive otherwise). This
   requires **no evals.json schema change** — identity is derived, not
   stored, so plans 003/016/020/027's string-based handling is untouched.
   The honest consequence, stated in docs and output: **editing an
   assertion's text creates a new instrument with a fresh history.**
   That is semantically right (a reworded check measures something
   different) and it sidesteps rename-tracking entirely. If real usage
   later demands continuity across rewording, explicit IDs can be added
   as an optional evals.json field — out of scope here, noted in
   maintenance.
2. **The ledger is computed, not maintained.** No new write-path state:
   `skill-eval ledger` walks every iteration's grading files and derives
   the ledger fresh each run. This keeps run/grade untouched (zero risk
   to the hot path) and makes the ledger correct-by-construction for
   historical workspaces the moment this ships. A `ledger.json` cache in
   the workspace root is written for other consumers (report, agents)
   but is always safe to regenerate.
3. **Ledger entry shape** (the `--json` contract, schema
   `skill-eval-ledger-v1`):

```json
{
  "assertion_id": "a1b2c3d4e5f6",
  "eval_id": 3,
  "text": "The output file contains no import or require of sinon.",
  "first_seen_iteration": 1,
  "last_seen_iteration": 4,
  "history": [
    {"iteration": 1, "with_skill": false, "baseline": false, "bucket": "broken_both"},
    {"iteration": 3, "with_skill": true,  "baseline": false, "bucket": "skill_value"}
  ],
  "flips": [
    {"iteration": 3, "config": "with_skill", "from": false, "to": true}
  ],
  "consecutive_dead_weight": 0,
  "prune_candidate": false
}
```

   Per-iteration verdict: majority across runs (Plan 011 layouts; tie →
   fail, matching 019's fail-closed convention), per model key — the
   ledger is **per-model** like the buckets (`--model` flag selects;
   default: the sole model, error listing options when several exist).
4. **Flips join to skill diffs.** `skill-eval ledger --flips` renders,
   per flip, the iteration boundary and — when both iterations have
   snapshots (Plan 017) — a unified diff of `SKILL.md` between
   `iteration-(N-1)/skill-snapshot/` and `iteration-N/skill-snapshot/`
   (capped at 100 lines, plain `diff`-style computed in Go or via
   comparing file contents; do NOT shell out to git). No snapshots → the
   flip renders with `skill diff unavailable (no snapshots; see plan 017)`.
   This is the attribution feature and the reason the plan exists;
   corpus-hash mismatch between the two iterations (Plan 018) annotates
   the flip as `corpus changed — flip may reflect eval edits, not skill
   edits`.
5. **Retirement is nominated, never automatic.** `prune_candidate: true`
   when the last `N` (default 3, `--prune-after N`) consecutive
   iterations were dead-weight (pass in both configs). The human output
   ends with a candidates section: the text, the streak, and the exact
   `evals.json` location — the user deletes; the tool never edits the
   corpus (same consent posture as Plan 027's `--write`, but here not
   even a write flag: deleting measurement instruments is the author's
   hand only).
6. **Instability surfaced, not scored.** An assertion whose with-skill
   verdict changed in ≥2 of the last 3 iterations *without* an
   intervening skill-snapshot diff gets `"unstable": true` — a noisy
   instrument (or noisy judge) flag rendered prominently, feeding the
   trust story (Plans 011/019) at assertion granularity. No numeric
   "stability score" in v1 — a boolean and the history are enough.
7. **Report integration is one section, not a rewrite**: the HTML report
   gains "Assertion history highlights" — flips this iteration (with the
   diff link/summary), new prune candidates, unstable assertions. The
   full ledger stays in the CLI/JSON where agents live.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |
| Manual | `go run . ledger --json` over a multi-iteration workspace | ledger document, exit 0 |

## Scope

**In scope**:
- New `ledger.go` + `ledger_test.go` (walker, identity, history, flips, nominations)
- New `cmd_ledger.go` + tests (flags: `--json`, `--flips`, `--model`, `--prune-after`, `--skill`)
- `main.go` (dispatch + usage)
- `report.go` (highlights section)
- `docs/guides/reading-results.md` (ledger section: identity rule, flips, pruning workflow)
- `workspace.go` only if a shared iteration-walker helper is extracted

**Out of scope**:
- Do NOT change the evals.json schema (identity is derived — Design decision 1).
- Do NOT auto-delete or auto-edit assertions (nomination only).
- Do NOT track assertion identity across text edits in v1.
- Do NOT compute the ledger during run/grade (read-path only; the
  `ledger.json` cache is written by the ledger command alone).
- Do NOT gate on ledger properties in this plan (a future
  `max_unstable_assertions` gate check is a natural Plan 022 extension —
  maintenance note, not scope).

## Git workflow

- Branch: `advisor/033-assertion-ledger`
- Commit message style: `feat: add assertion ledger — per-assertion history, flips, and prune nominations`
- Do NOT push unless instructed.

## Steps

### Step 1: Identity and normalization

In `ledger.go`:

```go
// assertionID derives a stable identity for an assertion within its eval.
// Editing the text creates a new identity by design — a reworded check is
// a different instrument.
func assertionID(evalID int, text string) string

// normalizeAssertion trims and collapses whitespace; case-sensitive.
func normalizeAssertion(text string) string
```

**Verify**: `go test ./... -run TestAssertionID` — stable across calls;
whitespace-insensitive; case-sensitive; eval-scoped (same text, different
eval → different ID); 12-hex output.

### Step 2: The walker

```go
// ledgerEntry / ledgerHistory / ledgerFlip types per Design decision 3.

// buildLedger walks every iteration's grading artifacts for one model key
// and derives the full ledger. Handles model-keyed, legacy, and run-<r>
// layouts; majority verdict across runs; skips iterations whose lock
// status is "running".
func buildLedger(ws string, modelKey string, pruneAfter int) ([]ledgerEntry, error)
```

Implementation notes: enumerate iterations via the `nextIteration`
convention; within each, read `grading.json` per (eval, config) using the
same layout fallbacks as `cmd_benchmark.go:52-60` (extract a shared
helper if Plan 018's loader hasn't already — check first); index results
by `assertionID`; compute buckets per iteration with Plan 016's rules if
its helper exists, else a local four-way classification with the same
names; derive flips, dead-weight streaks, `unstable` per Design
decision 6.

**Verify**: `go test ./... -run TestBuildLedger` — synthetic 4-iteration
workspace fixture covering: an assertion that flips (history + flip
recorded), a 3-streak dead-weight (nominated), an assertion added in
iteration 2 (`first_seen_iteration: 2`), an edited assertion (old ID ends
at iteration 2, new ID starts at 3 — both present, documented behavior),
a running-lock iteration (skipped), majority-across-runs when `run-*`
dirs exist.

### Step 3: Flip attribution via snapshot diffs

```go
// skillDiff returns a plain unified diff of SKILL.md between two
// iterations' snapshots, or ("", false) when either snapshot is missing.
func skillDiff(ws string, fromIter, toIter int) (string, bool)
```

Pure-Go line diff (a minimal LCS over lines is ~40 lines; no dependency),
capped per Design decision 4. Corpus-hash annotation when Plan 018's
fields are present in the two iterations' benchmark files.

**Verify**: `TestSkillDiff` — changed line rendered; missing snapshot →
false; cap honored.

### Step 4: The command

`cmd_ledger.go` per the scope's flag list. Human output: a compact table
(assertion snippet ≤60 chars, eval, verdict sparkline like
`bl:FFFF ws:FFPP`, bucket-now, flags for unstable/prune), then the flips
section (with diffs under `--flips`), then prune candidates. `--json`
prints the `skill-eval-ledger-v1` document and nothing else. Write the
`ledger.json` cache to the workspace root on every run. Wire into
`main.go` dispatch + usage.

**Verify**: `TestCmdLedgerJSON` (golden document, field names locked),
`TestCmdLedgerModelSelection` (multi-model workspace requires `--model`,
error lists options), `TestLedgerCacheWritten`.

### Step 5: Report highlights + docs

- `report.go`: read `ledger.json` if present (never compute in-report —
  keep report fast and the dependency one-directional); render the
  highlights section per Design decision 7; omit entirely when no cache
  exists.
- `docs/guides/reading-results.md`: "The assertion ledger" section — the
  identity rule stated plainly ("rewording an assertion restarts its
  history — that's intentional"), the flip-diff attribution workflow, the
  pruning ritual (run ledger → review candidates → delete → corpus hash
  changes → iteration deltas guard kicks in, and that's correct), and the
  `unstable` flag's meaning with a pointer to `--consistency`/`--runs`.

**Verify**: `TestReportRendersLedgerHighlights` (+ absent-cache no-op);
`cd docs/site && pnpm build`.

### Step 6: Final checks

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
cd docs/site && pnpm build && pnpm test
```

## Test plan

Steps 1–5 carry the units; the fixture workspace from Step 2 is the
heart — build it once as a test helper and reuse across command/report
tests. Cross-cutting regression: running `ledger` must not modify any
iteration artifact (hash the tree before/after in a test).

## Done criteria

- [ ] `skill-eval ledger` derives per-assertion history, flips, dead-weight streaks, prune nominations, and instability flags from existing artifacts — including historical workspaces created before this feature.
- [ ] `--flips` joins each flip to the SKILL.md snapshot diff when snapshots exist, with the corpus-change annotation when hashes differ.
- [ ] `--json` emits the locked `skill-eval-ledger-v1` schema; `ledger.json` cache written; report renders highlights from the cache only.
- [ ] The identity rule (edit = new instrument) is documented and tested.
- [ ] No write-path (run/grade/benchmark) code changes; ledger is read-only over the workspace.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run`, site build pass.
- [ ] `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- Assertion text does NOT round-trip byte-exact through `grading.json`
  in real workspaces (shared STOP with Plan 016 — fix grading, never
  fuzzy-match in the ledger).
- Plan 016 landed with bucket names or classification rules that differ
  from Design decision 3's — align on 016's shipped vocabulary before
  writing code.
- The layout fallback matrix (model-keyed × legacy × run-dirs × fix-dirs)
  produces ambiguous joins on any real workspace — enumerate the case and
  report rather than guessing.
- The pure-Go diff balloons past ~80 lines — take the simplest correct
  approach (whole-line equality LCS) or report; do not add a dependency
  for this.

## Maintenance notes

- The ledger is the assertion's system of record; future features that
  touch assertions (016's authoring, 027's promotion, 020's lint) should
  emit/consume `assertion_id` where useful — promotion in particular
  should note the promoted assertion's ID so its provenance survives.
- Explicit assertion IDs in evals.json (continuity across rewording) is
  the known v2 if users ask for it; the derived-ID design was chosen to
  avoid a schema migration until the demand is proven.
- Natural Plan 022 extension once trusted: `max_unstable_assertions` /
  `max_prune_candidates` gate checks — regression-shaped, Goodhart-resistant.
- If ledger computation gets slow on large workspaces (many iterations ×
  models), incrementalize via the cache (only walk iterations newer than
  the cache's high-water mark) — do not optimize before it hurts.
