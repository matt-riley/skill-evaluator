# Plan 032: Honest token accounting — stop averaging "unknown" as zero

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- runner.go agent.go eval.go benchmark.go report.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: S-M
- **Risk**: MED (changes a headline benchmark column's meaning; must be additive + explained, mirroring Plan 018's approach)
- **Depends on**: — (touches `aggregateRuns` like Plan 018; land in numeric order to avoid rebase churn — both plans note it)
- **Category**: measurement honesty
- **Planned at**: commit `1325f07`, 2026-07-01

## Why this matters

Token counts feed a headline benchmark column (`Tokens` mean/stddev and
the with-skill delta) and the report renders them with best/worst-cost
coloring — but for every runtime except pi they come from regex guesses
over combined stdout+stderr (`extractTokens`, `runner.go:150-168`), and
the failure mode is silent: no match → `0`. That zero is then
indistinguishable from "this run genuinely used ~0 tokens" and is
**averaged into the stats** (`aggregateRuns` appends
`float64(r.Timing.TotalTokens)` unconditionally when timing exists,
`benchmark.go:204-207`). Consequences:

- A runtime whose output format drifted reports `tokens: 0` forever, and
  the mean quietly sinks toward zero — the skill looks "cheaper" as the
  measurement breaks.
- Cross-model comparisons (`--models pi:…,claude`) mix precise pi counts
  with heuristic-or-zero counts in the same table and crown a
  `best_model` partly on fiction.
- The token *delta* — "does my skill cost more tokens than it saves?" —
  is exactly the number skill authors use to justify a skill's context
  cost, and it is the least trustworthy number the tool prints.

The heuristics themselves are also weaker than they need to be: the
generic patterns (`runner.go:152-157`) include one that matches
`input_tokens.*?(\d+)` — the *input* count presented as a total — and
they run in a fixed order against whichever happens to appear first in
the output. Runtime-specific extraction exists for pi only
(`extractPiTokens`, `runner.go:124-148`), even though the other runners'
own flags could emit structured usage (claude's print mode supports JSON
output formats; codex exec has verbose/json output — verify current flags
at execution time).

The fix has two halves: **provenance** (label every count with how it was
obtained, and exclude unknowns from statistics instead of averaging
zeros) and **precision** (per-runner extraction hooks so each runtime
parses its own structured output where available).

## Current state

- `eval.go:56-59`:

```go
// TimingData captured from an agent run.
type TimingData struct {
	TotalTokens int `json:"total_tokens"`
	DurationMs  int `json:"duration_ms"`
}
```

  (Plan 018 adds `Status` here; this plan adds source fields — both
  additive, order-independent, but see the shared-file note in Depends.)
- `runner.go:111-119` — `tokensFromOutput(agent, output)` dispatches:
  `pi` → precise JSON-stream sum; everything else → `extractTokens`
  regex-first-match. Returns bare `int`; `0` is ambiguous.
- `runner.go:150-168` — the four generic patterns, including the
  `input_tokens` mislabel noted above.
- `benchmark.go:199-207` — unconditional inclusion in stats.
- `report.go` — renders token means/deltas with `costClass` coloring
  (`report.go:280+`); no notion of missing data.
- `agent.go:11-18` — `AgentRunner` interface; Plan 031 adds
  `BinaryName()`. A token-extraction hook belongs on the same interface.

## Design decisions (read before coding)

1. **Provenance on the wire.** `TimingData` gains
   `TokensSource string \`json:"tokens_source,omitempty"\`` with values
   `"exact"` (structured output parsed), `"heuristic"` (regex match),
   `"none"` (no signal found). `TotalTokens` stays an int for
   backward-compat; `TokensSource == "none"` ⇒ `TotalTokens` is 0 AND
   meaningless. Old `timing.json` files (no source field) are treated as
   `"heuristic"` when `TotalTokens > 0`, `"none"` when 0 — the honest
   reading of historical data.
2. **Unknowns leave the statistics.** In `aggregateRuns`, token samples
   include only `TokensSource != "none"` runs. `RunSummary` gains
   `TokenCoverage float64 \`json:"token_coverage,omitempty"\`` (fraction
   of runs with a usable count). The report shows `n/a` for token
   cells when coverage is 0 and a footnote (`tokens measured on 7/12
   runs`) when 0 < coverage < 1. Token *deltas* render only when both
   configs have coverage > 0.
3. **Per-runner extraction hook.** `AgentRunner` gains
   `ExtractTokens(output string) (int, string)` returning count + source.
   `piRunner` wraps the existing `extractPiTokens` (source `"exact"`).
   `claudeRunner`/`codexRunner`: Step 1 probes each CLI's current
   structured-output flags; where a JSON/verbose mode exists AND we
   already request it (or can add the flag without changing captured
   behavior), parse it (`"exact"`); otherwise fall back to the shared
   regex heuristics (`"heuristic"`/`"none"`). `tokensFromOutput` becomes
   a thin call through `newAgentRunner` with the legacy path as fallback
   for unknown agents.
4. **Fix the heuristic set** while touching it: drop the
   `input_tokens.*?(\d+)` pattern (mislabeled), prefer `total_tokens`
   forms, and take the **last** match in the output rather than the first
   (usage summaries print at the end; early matches are more likely
   prompt echoes). Keep it conservative — a wrong count is worse than
   `"none"`.
5. **No fabrication.** Explicitly rejected: estimating tokens from
   byte/word counts. `n/a` teaches users to fix their runtime flags;
   a plausible-looking estimate would teach them to trust fiction.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Probe CLI output modes | `claude --help`, `codex exec --help` (current flags for JSON/verbose output + usage reporting) | noted for Step 1 |
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |

## Scope

**In scope**:
- `agent.go` (interface hook + three implementations)
- `runner.go` (dispatch, heuristic fixes, `TimingData` population; also `fixEval`'s timing write, `runner.go:314-315`)
- `eval.go` (`TokensSource`, `TokenCoverage`)
- `benchmark.go` (exclusion + coverage), `report.go` (`n/a`/footnote rendering)
- Tests: `runner_test.go`, `benchmark_test.go`, `report_test.go`
- `docs/guides/reading-results.md` (what token numbers mean now, per source)

**Out of scope**:
- Do NOT change agent invocation flags in a way that alters captured
  output for *other* consumers (transcripts in Plan 012, grading) without
  checking those plans — if adding a JSON-output flag changes stdout
  shape, note the interaction and prefer parsing what we already capture.
- Do NOT estimate tokens (Design decision 5).
- Do NOT add cost-in-dollars conversion (pricing tables rot; follow-up).
- Do NOT backfill old timing.json files on disk — the read-time
  interpretation rule (Design decision 1) covers them.

## Git workflow

- Branch: `advisor/032-honest-token-accounting`
- Commit message style: `fix: label token counts with provenance and exclude unknowns from benchmarks`
- Do NOT push unless instructed.

## Steps

### Step 1: Probe runtime output modes

Run the probe commands; record for claude and codex: whether the flags we
already pass (`agent.go:112-120, 130-138`) emit any structured usage, and
whether a flag exists that would without changing the human-readable
transcript. Decide per runtime: exact-parse, or heuristic-only for now.
Write the decision into the runner's comment.

### Step 2: Interface hook + implementations

Per Design decision 3. Shared heuristics move to
`func heuristicTokens(output string) (int, string)` implementing Design
decision 4 (last-match, no input_tokens pattern). `tokensFromOutput`
delegates; keep its signature for existing callers, now returning the
count only after storing source — simplest: change it to return
`(int, string)` and update the two call sites (`runner.go:68`,
`runner.go:315`).

**Verify**: `go test ./... -run TestExtractTokens` — pi stream exact;
claude/codex fixtures per Step 1 decision; no-signal → `(0, "none")`;
last-match beats first-match fixture; `input_tokens: 500` alone → `none`
(pattern removed).

### Step 3: Persist + aggregate

- `runEval`/`fixEval`: set `TotalTokens` + `TokensSource` in
  `TimingData`.
- `aggregateRuns`: include token samples per Design decision 2; compute
  `TokenCoverage`; legacy-file interpretation rule applied where
  `TimingData` is loaded (`cmd_benchmark.go:78-85` — or the shared loader
  if Plan 018 landed).

**Verify**: `TestAggregateExcludesUnknownTokens` — 2 runs exact(1000,
2000) + 1 run none → mean 1500, coverage 0.667; all-none → zeroed stats +
coverage 0; legacy timing without source honored per the rule.

### Step 4: Report rendering + docs

- `report.go`: `n/a` cells at zero coverage; footnote at partial;
  suppress token delta when either side lacks coverage. Keep `costClass`
  behavior for covered values.
- `docs/guides/reading-results.md`: a short "how token counts are
  measured" note — exact vs heuristic vs n/a, and that n/a means "your
  runtime didn't expose usage in a parseable way", not zero cost.

**Verify**: `TestReportTokensNA` / `TestReportTokenFootnote`; site build
if the guide changed (`cd docs/site && pnpm build`).

### Step 5: Final checks

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

## Test plan

Steps 2–4 tests plus the regression guard: a pi-only workspace produces
identical token means to the pre-change code (coverage 1.0, all-exact),
asserting the green path is numerically untouched.

## Done criteria

- [ ] Every `timing.json` records how its token count was obtained; unknown ≠ zero anywhere downstream.
- [ ] Benchmarks exclude unknown-token runs from token stats and report coverage; the report renders `n/a`/footnotes instead of fictional zeros.
- [ ] Per-runner extraction: pi exact; claude/codex exact where the probe allows, conservative heuristic otherwise; the mislabeled `input_tokens` pattern is gone and last-match wins.
- [ ] Legacy timing files interpreted honestly without migration.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- Step 1 finds that getting exact counts from claude/codex requires
  output flags that change what `CombinedOutput` captures for grading or
  transcripts — report the interaction with Plans 012/019 rather than
  changing capture unilaterally.
- Plan 018 is mid-flight on `aggregateRuns`/`TimingData` — land after it
  and rebase; both plans' fields are additive but the same lines move.
- The `AgentRunner` interface change collides with Plan 031's
  `BinaryName()` addition in review — trivial merge, but coordinate the
  interface shape once, not twice.

## Maintenance notes

- New agent runtimes must implement `ExtractTokens` — return
  `(0, "none")` until their format is verified; never ship a guess.
- If a runtime's output format drifts, the symptom is now visible
  (coverage drops, `n/a` appears) instead of silent zeros — Plan 029's
  troubleshooting entry for token counts should be updated to point at
  coverage when both land.
- Follow-up candidates: dollar-cost estimation behind a user-supplied
  price table; separate input/output token columns where runtimes expose
  both.
