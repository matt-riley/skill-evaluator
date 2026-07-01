# Plan 021: Content-hash grading cache — never pay the judge twice for the same evidence

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- grader.go eval.go cmd_grade.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: M
- **Risk**: MED (a wrong cache hit is a wrong grade; the key must be complete)
- **Depends on**: — (cycle-1 explicitly deferred this "until lockfile/resume exist" — plan 005 is DONE, so it is unblocked; if Plan 019 lands first, its knobs join the key — see Step 2)
- **Category**: cost / iteration speed
- **Planned at**: commit `1325f07`, 2026-07-01

## Why this matters

Grading is the tool's recurring cost, and it is recomputed even when
nothing changed. Real repeat-grading paths today:

- `skill-eval grade` re-run after an interrupted/partial grade re-judges
  every eval that already has a valid grading.
- `loop --fix` grades fix attempts; the initial grading is re-read but any
  re-entry into `grade` re-judges untouched configs.
- Iterating on *one* eval (`run --eval 5` then `grade`) re-grades all ten
  because grade has no notion of "unchanged since last verdict"
  (`cmd_grade.go:75-117` walks everything with outputs).

The cycle-1 plan round already identified output caching as high-value and
deferred it only on sequencing grounds (`plans/README.md`: "defer until
lockfile/resume exist" — they do now). A verdict is a pure function of
(evidence, assertions, judge); cache it under that key and re-grades of
unchanged evidence become free and — usefully for Plan 019's concerns —
deterministic.

## Current state

- `grader.go:22-110` — `gradeFromOutput` always evaluates matchers and
  always calls the judge when LLM assertions exist; it ends by writing
  `grading.json` via `saveGrading` unconditionally.
- `grader.go:118-155` — `readOutputContents` is the evidence reader (has
  size/count caps — the cache key must hash what the judge would *see*,
  i.e. this map, not raw disk, so cap changes naturally invalidate).
- `eval.go:84-96` — `GradingFile` is the persisted verdict; additive
  fields are safe.
- `cmd_grade.go:102-108` — the call site that would skip on a hit.
- Judge identity comes from `cfg.Judge` with defaults fallback
  (`grader.go:49-56`).
- Note: `readOutputContents` map iteration order affects the prompt today;
  Plan 019 Step 1 sorts it. If 019 has not landed, sort here (the key
  hashes sorted content regardless, but prompt determinism is required for
  the "same evidence ⇒ same verdict" claim to be honest).

## Design decisions (read before coding)

1. **Key = everything the verdict depends on.** SHA-256 over a canonical
   byte string assembled from, in order:
   - cache schema version constant (`gradeCacheV1`) — bump to mass-invalidate;
   - grading prompt version constant (add `const gradingPromptVersion = 1`
     in grader.go; **every future edit to `buildGradingPrompt` must bump
     it** — enforce by comment beside the template);
   - judge agent + judge model (resolved values, post-fallback);
   - eval ID, prompt, expected output;
   - the full ordered assertion list (deterministic ones too — they are
     part of the verdict);
   - sorted `(filename, sha256(content))` pairs from `readOutputContents`
     **plus** the sorted names of files that existed but were excluded by
     caps (exclusion affects what the judge saw);
   - if Plan 019 landed: canary-enabled flag and consistency sample count.
2. **Storage: inside `grading.json`.** Add to `GradingFile`:
   `CacheKey string \`json:"cache_key,omitempty"\`` and
   `GradedAt time.Time \`json:"graded_at,omitempty"\``. A hit = existing
   grading.json whose `cache_key` equals the freshly computed key. No
   separate cache directory, no eviction problem, artifacts stay
   self-describing.
3. **Never cache unreliable verdicts**: skip writing `CacheKey` when any
   assertion evidence starts with `judge error:` (the fail-closed path,
   `grader.go:66-73`) or when `JudgeSuspect` (Plan 019) is true — those
   must re-grade next time.
4. **`--force-grade` bypasses** (re-judges and overwrites). Cache use is
   default-on; `judge.cache: false` config knob for teams that want
   fresh verdicts always.
5. **Deterministic matchers always re-run.** They are microseconds and
   they double as an integrity check: if matcher results differ from the
   cached grading's (outputs changed in a way the key catches anyway),
   the key mismatch already forced a re-grade — so matchers re-run only on
   misses. Simpler: a hit returns the stored grading verbatim. Keep it
   simple; the key covers the evidence.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |

## Scope

**In scope**:
- `grader.go` (key computation; hit short-circuit in `gradeFromOutput`)
- `eval.go` (GradingFile additive fields)
- `config.go` + `schema/config-schema.json` (`judge.cache`)
- `cmd_grade.go`, `cmd_loop.go` (`--force-grade`)
- Tests in `grader_test.go`
- `docs/guides/reading-results.md` (one paragraph: when grades are reused)

**Out of scope**:
- Do NOT cache agent runs (different problem — runs are *supposed* to
  vary; Plan 011 measures that variance).
- Do NOT build a cross-workspace or global cache.
- Do NOT deduplicate identical evidence across different evals (same key
  component includes eval ID — two evals with identical outputs still
  grade separately; assertion context differs).

## Git workflow

- Branch: `advisor/021-grading-cache`
- Commit message style: `feat: cache gradings by evidence hash; add --force-grade`
- Do NOT push unless instructed.

## Steps

### Step 1: Key computation

In `grader.go`:

```go
const gradeCacheSchema = "grade-cache-v1"
// gradingPromptVersion MUST be bumped whenever buildGradingPrompt's
// structure or instructions change; it invalidates all cached verdicts.
const gradingPromptVersion = 1

// gradingCacheKey derives the content hash of everything a verdict
// depends on. Same key ⇒ the judge would see a byte-identical prompt.
func gradingCacheKey(cfg *Config, eval Eval, outputContents map[string]string, excludedNames []string) string
```

`readOutputContents` must also report which files it *skipped* due to caps
— change its return to `(map[string]string, []string)` or add a sibling
function; check both callers (`gradeFromOutput`, and any report usage —
grep first).

**Verify**: `go test ./... -run TestGradingCacheKey` — stable across map
ordering; changes when any input changes (one test per key component:
content, assertion order, judge model, excluded file set, prompt version).

### Step 2: Hit short-circuit

In `gradeFromOutput` (`grader.go:23-110`), after `readOutputContents`:

```go
	key := gradingCacheKey(cfg, eval, outputContents, excluded)
	if !forceGrade && cfg.judgeCache() {
		if prev, err := readGrading(gradingPath); err == nil && prev.CacheKey == key && !prev.JudgeSuspect {
			logger.Debug("grading cache hit", "eval", eval.ID, "path", gradingPath)
			return prev, nil
		}
	}
```

- `readGrading` is a tiny loader (grading.json unmarshal with a size cap —
  mirror `readEvalsFile`'s pattern in `cmd_import_agit.go:239-257`).
- Thread `forceGrade bool` through `gradeEval`/`gradeFromOutput`
  signatures (callers: `cmd_grade.go:103`, `gradeFixAttempt`,
  loop paths — grep `gradeEval(` / `gradeFromOutput(`). Fix attempts pass
  `false` (their outputs differ per attempt, so keys differ naturally).
- On the write path: set `CacheKey = key` and `GradedAt = time.Now()`
  **except** in the judge-error case (Design decision 3) — set neither, so
  the stored artifact is identifiable as non-cacheable.
- Config: `JudgeConfig` gains `Cache *bool \`yaml:"cache"\``; helper
  `judgeCache()` defaulting true; schema entry.

**Verify**: `TestGradeCacheHitSkipsJudge` — stub `cmdFn` counts calls;
second grade of identical evidence = zero judge calls, identical
`GradingFile` (minus `GradedAt`). `TestGradeCacheMissOnOutputChange`,
`TestJudgeErrorNotCached`, `TestForceGradeBypasses`.

### Step 3: CLI plumbing

- `cmd_grade.go`: `force := fs.Bool("force-grade", false, "Re-judge even when cached verdicts match the current evidence")`;
  pass through. Print a per-iteration summary line:
  `Graded 12 (9 cached, 3 judged)` — count hits/misses via a small struct
  or two counters returned/accumulated.
- `cmd_loop.go`: forward `--force-grade`.
- `main.go` usage text.

**Verify**: `go run . grade --help` shows the flag; summary line appears
in `TestGradeCacheHitSkipsJudge`'s captured output (or assert counters).

### Step 4: Docs and final checks

`docs/guides/reading-results.md`: paragraph on cached verdicts —
`graded_at`/`cache_key` fields, when reuse happens, `--force-grade`, and
the interaction with `--consistency` (different sample count = different
key = re-grade).

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

## Test plan

Step 1–2 tests are the core. Add the paranoia case:
`TestCacheKeyExcludedFilesMatter` — two workspaces identical except one
has an extra over-cap file (excluded from judge view) → different keys.
And `TestCacheRoundTripPreservesSummary` — hit returns pass/fail counts
byte-identical to the stored artifact.

## Done criteria

- [ ] Re-grading unchanged evidence performs zero judge invocations and reports the hit count.
- [ ] The key covers evidence, assertions, judge identity, prompt version, cap-excluded files (and 019's knobs when present).
- [ ] Judge-error and judge-suspect gradings are never treated as hits.
- [ ] `--force-grade` and `judge.cache: false` both bypass.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] Docs updated; `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- Anything judge-visible cannot be captured in the key (e.g. a runtime
  injects wall-clock time into prompts) — an incomplete key is worse than
  no cache; report rather than approximate.
- `readOutputContents`'s return change touches more callers than
  `gradeFromOutput` (grep first; the report or fix paths may read it).
- Plans 019/021 land in the opposite order than assumed and the key
  components disagree with 019's shipped knobs — reconcile the key before
  merging, never after release.

## Maintenance notes

- The `gradingPromptVersion` bump-on-edit rule is the whole safety story
  for prompt changes; reviewers must check it on every `buildGradingPrompt`
  diff. A CI grep guard (`git diff` touching buildGradingPrompt without
  touching gradingPromptVersion → fail) is a worthwhile follow-up.
- If cache hits with *changed* verdict expectations ever confuse users,
  print `cached verdict from <graded_at>` per eval under `--verbose`
  before considering TTLs. Verdicts don't rot; prompts change — the
  version constant is the TTL.
