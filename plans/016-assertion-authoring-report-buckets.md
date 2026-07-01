# Plan 016: LLM-assisted assertion authoring and skill-value report buckets

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- cmd_import_agit.go internal/agit/convert.go grader.go report.go benchmark.go eval.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: M
- **Risk**: MED (adds a judge call at import time; report reshuffle touches user-facing HTML)
- **Depends on**: plans/012-transcript-artifact-toolcall-assertions.md (soft — authored assertions may emit transcript matchers; hard dependency only on Plan 003's matcher set, already DONE)
- **Category**: signal quality / author experience
- **Planned at**: commit `1325f07`, 2026-07-01
- **Research**: `docs/research/agengit-integration.md` §4.4, §4.5

## Why this matters

Two ends of the loop are weak:

1. **Going in**: imported assertions come from `keyTermFromSummary`
   (`internal/agit/convert.go:397-426`) — a heuristic that grabs ≤50 chars
   after a verb like "added " and hopes the phrase reappears in a replayed
   run. It produces brittle `contains_text:` checks and vague LLM
   assertions. A judge agent is *already configured*; one cheap call per
   imported eval — given the recorded prompt, diff file list, and assistant
   summary — can author 3–5 specific, observable assertions following the
   documented guidance (`eval-workflow.md`: specific + observable, mix
   deterministic and judged). One call at import time buys better grading
   signal on every subsequent iteration.
2. **Coming out**: the loop ends at aggregate numbers plus generic
   `buildSuggestions` heuristics (`report.go:291+`). The three
   observations an author actually acts on are per-assertion, not
   per-mean: assertions that pass **only with the skill** (the skill's
   proven value), assertions that **fail in both** configs (eval or skill
   defect), and assertions that **always pass** (dead weight — the docs
   even tell users to hunt these by hand, `eval-workflow.md` §5). The
   report should compute those buckets from `grading.json` files instead
   of asking users to eyeball them.

## Current state

- `internal/agit/convert.go:378-381` — `contains_text` from
  `keyTermFromSummary`; `:447-452` — the generic LLM fallback assertion.
- `cmd_import_agit.go` — no judge access today (imports never invoke
  agents); `LoadConfig` is available (pattern at `cmd_run.go:37-40`), and
  the judge plumbing lives at `grader.go:49-64` (`cfg.Judge` fallback to
  defaults, `cmdFn` builder, first-`{` JSON extraction at
  `grader.go:335-347`).
- `grader.go:294-303` — the strict-JSON response contract style to copy.
- `report.go:15-27` — `ReportData`; the template renders per-model means
  and `Suggestions []string`. Nothing per-assertion.
- `grading.json` files persist per `(eval, model, config)` under
  `evalPath(...)/<config>/grading.json` with `AssertionResults[].Text` and
  `.Passed` — everything the buckets need is already on disk.
- `report.go:190-202` — `loadBenchmarkFile` shows the workspace-reading
  pattern used by the report command.

## Design decisions (read before coding)

1. **Authoring is opt-in** (`import-agit --author-assertions`): it costs
   judge tokens and requires a configured agent; default off.
2. **Authored assertions replace the heuristic ones, keep the floor.**
   When authoring succeeds for an eval: keep `file_exists:` assertions from
   the diff (ground truth), drop `keyTermFromSummary`-derived
   `contains_text:` ones, use the authored set, and keep exactly one
   trailing LLM assertion (authored or the existing fallback). On judge
   failure: keep today's output unchanged and warn — imports must never
   fail because authoring did.
3. **Authored output is validated, not trusted**: response must be strict
   JSON `{"assertions": ["..."]}`; each entry ≤300 chars; deterministic
   prefixes are re-parsed with `parseAssertion` and dropped if malformed or
   path-unsafe (`isSafeAssertionPath`); cap 5 per eval. The judge sees
   *recorded* session content — wrap it with `sanitizeAssertionText`-style
   hygiene consistent with `buildGradingPrompt`.
4. **Buckets are cross-config joins on assertion text** within
   `(eval, model)`: an assertion is *skill-value* if it passed in
   `with_skill` and failed in `baseline`; *broken-everywhere* if it failed
   in both; *dead-weight* if it passed in both. Multi-run layouts
   (Plan 011) count an assertion as passed if it passed in the majority of
   runs (document the tie rule: pass on tie for pass counting; assert it
   in tests).
5. **Buckets live in `benchmark.json`**, not just HTML — CI and other
   consumers get them for free; the report renders from the same struct.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |

## Scope

**In scope**:
- New `author.go` + `author_test.go` (authoring prompt, judge call, validation)
- `cmd_import_agit.go` (flag + wiring; config load)
- New `buckets.go` + `buckets_test.go` (grading joins)
- `benchmark.go`/`eval.go` (`AssertionBuckets` in `BenchmarkFile`)
- `report.go` (render buckets; retire the overlapping generic suggestions)
- `docs/guides/importing-agit-sessions.md`, `docs/guides/reading-results.md`

**Out of scope**:
- Do NOT auto-edit `SKILL.md` from bucket findings (the report *informs*
  the author; closing that loop automatically is a different, riskier
  feature).
- Do NOT author assertions for hand-written evals (only newly imported
  ones, in the import command).
- Do NOT delete `keyTermFromSummary` — it remains the no-judge fallback.
- Do NOT bucket across models (per-model only; cross-model unions mislead).

## Git workflow

- Branch: `advisor/016-assertion-authoring-report-buckets`
- Commit message style: `feat: judge-authored import assertions and skill-value report buckets`
- Do NOT push unless instructed.

## Steps

### Step 1: Authoring prompt and validator

New `author.go`:

```go
// buildAuthoringPrompt asks the judge to write assertions for a recorded
// task. Inputs are wrapped in data markers (same injection stance as
// buildGradingPrompt). fileChanges is the sanitized added/modified list.
func buildAuthoringPrompt(prompt, assistantSummary string, fileChanges []string) string

// validateAuthored filters judge-proposed assertions: strict length cap,
// deterministic prefixes must re-parse cleanly with safe paths, max 5.
func validateAuthored(raw []string) []string
```

Prompt contract essentials: cite the house rules (specific, observable,
avoid always-true and brittle-exact-string), list available deterministic
prefixes (`file_exists:`, `contains_text:`, `matches_text:`, plus
transcript matchers if Plan 012 landed — feature-detect by checking
`parseAssertion("transcript_contains: x")` type), require
`{"assertions": [...]}` JSON only, 3–5 entries, at most one open-ended
judged assertion.

**Verify**: `TestBuildAuthoringPrompt` (markers, prefixes listed),
`TestValidateAuthored` (drops >300 chars, malformed prefix, traversal path,
caps at 5).

### Step 2: Judge call

```go
// authorAssertions runs one authoring call. Never fatal: returns nil on
// any failure and the caller keeps heuristic assertions.
func authorAssertions(ctx context.Context, cfg *Config, ce agit.ConvertedEval, cmdFn CmdBuilder) []string
```

Copy the judge plumbing from `gradeFromOutput` (`grader.go:49-64`) and the
first-`{` extraction (`grader.go:335-347`, size-limited). Log failures at
Warn with eval ID, never content.

**Verify**: `TestAuthorAssertions` with stub `CmdBuilder` (happy path,
garbage JSON → nil, judge exec error → nil).

### Step 3: Wire into import

`cmd_import_agit.go`:
- Flag: `authorFlag := fs.Bool("author-assertions", false, "Use the judge agent to write assertions for imported evals (costs tokens)")`.
- When set: `cfg, err := LoadConfig(dir)` after the skill dir resolves;
  hard-error if no judge/default agent configured.
- Per converted eval (both fast and legacy paths — do it once at the
  `allEvals` assembly level, after `evalFromConverted`): call
  `authorAssertions`; on non-nil result, rebuild `Assertions` per Design
  decision 2 (retain diff-derived `file_exists:`; replace the rest).
- Progress line: `Authoring assertions for 12 evals (judge: pi)…` and a
  final `authored 10, kept heuristics for 2`.
- Respect `ctx` — authoring must abort on Ctrl+C between calls.

**Verify**: `TestImportAuthoringRewrite` — fixture eval whose heuristic
set is {file_exists, contains_text(keyterm), llm-fallback} and authored
set is {contains_text(better), matches_text, judged} → final set is
{file_exists, contains_text(better), matches_text, judged}.

### Step 4: Bucket computation

New `buckets.go`:

```go
// AssertionBucket names one assertion's cross-config behavior.
type BucketedAssertion struct {
	EvalID int    `json:"eval_id"`
	Text   string `json:"text"`
}

// AssertionBuckets summarizes per-assertion value across configs for one model.
type AssertionBuckets struct {
	SkillValue  []BucketedAssertion `json:"skill_value"`   // pass with_skill, fail baseline
	BrokenBoth  []BucketedAssertion `json:"broken_both"`   // fail in both
	DeadWeight  []BucketedAssertion `json:"dead_weight"`   // pass in both
	BaselineWin []BucketedAssertion `json:"baseline_wins"` // fail with_skill, pass baseline (regression smell)
}

// computeBuckets joins with_skill and baseline grading results by
// assertion text within each eval.
func computeBuckets(results []*RunResult) map[string]*AssertionBuckets // keyed by model
```

Join rule: within `(eval, model)`, index baseline `AssertionResults` by
`Text`; assertions present in only one config are skipped (counted in a
`skipped_unmatched` int for transparency). Multi-run majority rule per
Design decision 4.

- `eval.go` `BenchmarkFile`: `Buckets map[string]*AssertionBuckets \`json:"assertion_buckets,omitempty"\``.
- `benchmark.go` `computeBenchmark`: populate from its `results` input.
  Note: `cmdGrade` builds `RunResult`s with `Grading` attached
  (`cmd_grade.go:109-114`) — the data is already flowing; the standalone
  `cmd_benchmark.go` path must load gradings from disk the same way it
  does today (verify it does before assuming).

**Verify**: `TestComputeBuckets` — hand-built results covering all four
buckets, unmatched skip, and the multi-run majority + tie rule.

### Step 5: Report rendering

`report.go`:
- `ReportData` gains `Buckets map[string]*AssertionBuckets`.
- Template: new "What the skill actually changed" section per model —
  three lists (skill-value 🏆, broken-both 🔧 with "fix the eval or the
  skill" hint, dead-weight 🗑 with "consider removing" hint), and
  `BaselineWin` rendered with a warning color when non-empty. Cap each
  list at 15 rows with a "+N more" line.
- `buildSuggestions`: remove any generic suggestion the buckets now state
  precisely (read the current suggestions first; keep the ones about
  timing/tokens).

**Verify**: `TestReportRendersBuckets`; template still auto-escapes
(assertion text is untrusted — same XSS stance as the existing
`LLMSuggestions` comment at `report.go:107-110`; never `template.HTML`).

### Step 6: Docs and final checks

- `docs/guides/importing-agit-sessions.md`: `--author-assertions` — what
  the judge sees, cost note, validation story.
- `docs/guides/reading-results.md`: the four buckets and the action each
  implies (this replaces the manual "spot the patterns" hunting in
  `eval-workflow.md` §5 — cross-link it).

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

## Test plan

Steps 1–5 each carry tests. End-to-end: `TestBenchmarkFileCarriesBuckets`
— grade fixtures through `computeBenchmark` and assert the JSON output
contains populated `assertion_buckets` keyed by model.

## Done criteria

- [ ] `import-agit --author-assertions` produces validated judge-authored assertions; failures fall back to heuristics without failing the import.
- [ ] Authored deterministic assertions always re-parse cleanly and pass path-safety checks.
- [ ] `benchmark.json` contains per-model assertion buckets; the HTML report renders all four with action hints.
- [ ] `keyTermFromSummary` path unchanged when the flag is off.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] Guides updated; `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- `cmd_benchmark.go`'s standalone path does not load per-assertion grading
  data from disk (buckets would be grade-phase-only) — extend it first and
  flag the asymmetry.
- Authoring needs recorded content beyond what `ConvertedEval` already
  carries (e.g. full tool-call bodies) — that's a `ConvertedEval` schema
  discussion, coordinate with Plans 010/012 rather than widening ad hoc.
- Assertion `Text` turns out not to round-trip exactly through the judge
  in `grading.json` (join key breaks) — fix the root cause in grading, do
  not fuzzy-match in buckets.

## Maintenance notes

- The authoring prompt is the quality lever; keep examples in it aligned
  with `eval-workflow.md` guidance whenever that doc changes.
- `BaselineWin` non-empty across iterations is the report's most important
  smell — if users miss it, promote it visually before adding new metrics.
- Follow-up: use bucket history across iterations (Plan 006's
  cross-iteration data) to auto-suggest pruning dead-weight assertions.
