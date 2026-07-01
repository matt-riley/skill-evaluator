# Plan 020: Lint evals.json and add `run --dry-run`

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- runner.go grader.go cmd_run.go cmd_init.go main.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: S-M
- **Risk**: LOW
- **Depends on**: plans/009-skill-md-validate-command.md (extends the `validate` command)
- **Category**: author experience / correctness
- **Planned at**: commit `1325f07`, 2026-07-01

## Why this matters

The eval corpus is the tool's measurement instrument, and it is entirely
unlinted. Concrete footguns visible in the current code:

1. **Matcher typos silently become expensive LLM assertions.**
   `parseAssertion` (`grader.go:163-197`) falls through to `MatcherLLM`
   for anything it doesn't recognize. `file_exist: results.csv` (missing
   "s"), `contains_text: summary.txt Total revenue` (missing second
   colon), `transcript-contains: x` (wrong separator) — all are shipped to
   the judge as free-text "assertions" that grade vaguely and cost tokens,
   with zero feedback that the author meant a deterministic check.
2. **Duplicate or nonpositive eval IDs corrupt the workspace.** Paths are
   keyed by `eval-<ID>` (`workspace.go:27-33`); two evals with `id: 3`
   overwrite each other's outputs and gradings, and the benchmark
   double-counts one sample. `readEvals` (`runner.go:191-231`) checks only
   size caps — never ID uniqueness, positivity, or empty prompts.
3. **Scaffold placeholders run as real evals.** `init` writes a skeleton
   eval ("Describe the task you want the agent to perform here.",
   `cmd_init.go:44-52`); nothing stops `run` from spending agent and judge
   invocations executing that placeholder verbatim.
4. **No way to preview cost or resolution.** Users discover what `run`
   will do (which agent, which model, how many invocations, which files
   are missing) only by running it. The confirmation prompt
   (`cmd_run.go:143-151`) shows a bare count, after config resolution has
   already happened invisibly.

## Current state

- `runner.go:191-231` — `readEvals` validates only file size, eval count,
  prompt/expected-output length.
- `grader.go:163-197` — the silent LLM fallthrough (by design for prose
  assertions; the lint must distinguish "prose" from "near-miss prefix").
- `eval.go:16-17` — `Files []string` exists; nothing checks the paths.
- `cmd_init.go:44-52` — placeholder text written by `init`:

```go
			Evals: []Eval{
				{
					ID:             1,
					Prompt:         "Describe the task you want the agent to perform here.",
					ExpectedOutput: "Describe what success looks like.",
					Files:          []string{},
					Assertions:     []string{"Example: The output includes a summary of results."},
				},
			},
```

- `cmd_run.go:129-151` — eval counting + cost prompt; `--models`, config,
  and baseline resolution all precede it with partial visibility.
- Plan 009 defines the `validate` command, finding model (`Finding`), and
  severity/exit conventions this plan extends.

## Rules to add (validate gains an evals.json section)

Errors:
- **EV1** Duplicate eval IDs.
- **EV2** Nonpositive ID (`id < 1`).
- **EV3** Empty/whitespace prompt.
- **EV4** `files` entry does not exist on disk (resolved against the skill
  dir), is absolute, or escapes the skill dir after cleaning.
- **EV5** Known matcher prefix with malformed payload:
  `contains_text:`/`matches_text:` missing the `file:arg` split (exactly
  the conditions that make `parseAssertion` fall through today), or
  `matches_text:` whose regex does not compile, or a matcher path that
  fails `isSafeAssertionPath`.
- **EV6** Near-miss prefix: assertion matches `^[a-z][a-z_-]{2,30}:` but
  the prefix is not in the known-matcher set → error with a
  did-you-mean (nearest known prefix by Levenshtein distance ≤ 3;
  otherwise list the valid prefixes). Keep the known set in ONE place
  shared with `parseAssertion` (export a `knownMatcherPrefixes()` from
  grader.go) so new matchers (Plan 012's transcript pair) stay covered
  automatically.
- **EV7** `skill_name` mismatch with directory base name (warn-level? no —
  error; it is metadata that reports display).

Warnings:
- **EVW1** Scaffold placeholder text unchanged (exact-match the `init`
  strings).
- **EVW2** Eval has zero assertions (grading will error at run time —
  `grader.go:24-26`) — warn here, pointing at the failure that would
  otherwise appear mid-run. Exempt `type: "activation"` evals if Plan 013
  has landed (feature-detect on the field).
- **EVW3** Fewer than 2 evals (docs recommend 2–3 minimum).
- **EVW4** Duplicate prompts across evals (copy-paste corpus inflation).

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |
| Manual | `go run . run --dry-run` | plan printed, nothing executed |

## Scope

**In scope**:
- New `lint_evals.go` + `lint_evals_test.go` (rules above)
- `cmd_validate.go` (call the evals lint when `evals/evals.json` exists)
- `grader.go` (export the known-prefix set; no behavior change)
- `cmd_run.go` (`--dry-run` flag; EV-error hard stop before any invocation)
- `main.go` usage text
- `docs/guides/writing-evals.md`, `docs/guides/validating-skills.md`

**Out of scope**:
- Do NOT make prose LLM assertions warnings — free-text judging is a
  feature; only *near-miss prefixes* (EV6) are flagged.
- Do NOT auto-fix anything (no rewriting of evals.json).
- Do NOT validate assertion *quality* with an LLM here (Plan 016 authors;
  this plan lints).
- Do NOT change `readEvals`'s existing size caps or move its checks.

## Git workflow

- Branch: `advisor/020-evals-lint-dry-run`
- Commit message style: `feat: lint evals.json in validate and add run --dry-run preview`
- Do NOT push unless instructed.

## Steps

### Step 1: Shared prefix registry

In `grader.go`, extract the prefix strings used by `parseAssertion` into:

```go
// knownMatcherPrefixes returns every deterministic assertion prefix,
// including the trailing colon. parseAssertion and the evals linter must
// both consume this — never a second hardcoded list.
func knownMatcherPrefixes() []string
```

and refactor `parseAssertion` to loop over it for its prefix checks
(behavior identical; add a regression test asserting identical
`ParsedAssertion` results for the existing test corpus).

**Verify**: existing `TestParseAssertion*` green; new
`TestKnownPrefixesDriveParser`.

### Step 2: The linter

New `lint_evals.go`:

```go
// lintEvals checks an eval corpus for structural and matcher problems.
// It re-reads the file itself (via readEvals) so validate works without
// a prior run.
func lintEvals(skillDir string) ([]Finding, error)
```

Implement EV1–EV7 and EVW1–EVW4. Notes:
- EV5/EV6 need a "looks like a matcher attempt" pre-test before
  `parseAssertion`: run `parseAssertion` first; if it returns a
  deterministic type, check payload validity (EV5); if it returns LLM
  *and* the near-miss regex matches, EV6.
- Small Levenshtein helper (≤20 lines, stdlib only) — do not add a
  dependency.
- EV4 reuses the containment approach from `grader.go:isPathWithin`.
- Return `Finding` values with rule IDs so the output format matches
  Plan 009's.

**Verify**: `go test ./... -run TestLintEvals` — one table case per rule,
plus a fully-clean corpus → zero findings.

### Step 3: Wire into validate

In `cmd_validate.go` (from Plan 009): after the SKILL.md checks, if
`evals/evals.json` exists, append `lintEvals` findings; absent file →
single warning `no evals/evals.json — run skill-eval init` (not an error;
validate must be usable pre-init). Exit contract unchanged (any E → error).

**Verify**: `go run . validate` on a scratch skill with a typo'd prefix
prints `ERROR [EV6] eval 2: unknown matcher prefix "file_exist:" — did you mean "file_exists:"?`.

### Step 4: `run --dry-run`

In `cmd_run.go`:
- Flag: `dryRun := fs.Bool("dry-run", false, "Print the run plan and validation findings without invoking any agent")`.
- Hard gate for real runs too: call `lintEvals` right after `readEvals`;
  E-level findings **abort the run** before the iteration directory is
  created (unlike Plan 009's SKILL.md preflight which only warns — a
  corrupt corpus produces corrupt directories via EV1, so it must stop).
  Print findings either way.
- When `--dry-run`: after config/model/baseline resolution and eval
  counting (reuse the existing code paths up to `cmd_run.go:141`), print
  and exit 0 without creating the iteration or lock:

```
Plan: iteration 4
  skill:    /path/to/skill  (SKILL.md ok)
  agent(s): pi:claude-sonnet, claude
  judge:    pi (model: gpt-4o-mini)
  baseline: skill snapshot from iteration 3
  evals:    5 task (2 with input files), 12 deterministic + 9 LLM assertions
  invocations: 5 evals × 2 models × 2 configs = 20 agent runs, ~20 judge calls
```

  Judge-call estimate = evals with ≥1 LLM assertion × models × configs.
  Baseline line reuses Plan 017's resolution when that landed; otherwise
  print the raw flag value.
- `cmd_loop.go`: forward `--dry-run` (loop dry-runs the run phase then
  stops — grading a nonexistent iteration makes no sense; print
  `(dry-run: grade/benchmark skipped)`).

**Verify**: `go test ./... -run TestRunDryRun` — stub `CmdBuilder` asserts
zero invocations; no `iteration-N` directory created; output contains the
invocation arithmetic. `TestRunAbortsOnEvalLintErrors` — duplicate IDs →
run returns error before creating the iteration dir.

### Step 5: Docs and final checks

- `docs/guides/writing-evals.md`: the rule table, the near-miss example,
  and `--dry-run` as the pre-flight habit.
- `main.go` usage: `--dry-run` line.

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
```

## Test plan

Steps 1–4 carry the tests. The must-not-regress case: an eval whose
assertion is genuine prose containing a colon mid-sentence
(`"The summary explains: revenue rose"`) triggers neither EV5 nor EV6
(the near-miss regex requires the colon to terminate the leading token).

## Done criteria

- [ ] `validate` lints evals.json with EV1–EV7 / EVW1–EVW4 findings.
- [ ] `parseAssertion` and the linter share one prefix registry.
- [ ] `run` aborts (pre-iteration) on EV-level corpus errors; `--dry-run` previews resolution, counts, and cost with zero side effects.
- [ ] Prose assertions with incidental colons are not flagged.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` pass.
- [ ] Docs updated; `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- Plan 009 has not landed (no `cmd_validate.go`/`Finding`) — it is a hard
  dependency; do not fork a second findings model.
- The near-miss heuristic (EV6) flags legitimate prose in this repo's own
  documented examples (`eval-workflow.md`, guides) — tighten the regex
  before shipping, never ship with known false positives.
- Aborting `run` on EV errors breaks the `loop --resume` path (resume
  re-reads the corpus; a corpus edited mid-iteration into an EV state
  needs a clear message, not a stuck lock) — handle and test that case.

## Maintenance notes

- Every new matcher (Plan 012 and beyond) automatically gets typo
  protection via the shared registry — reviewers should reject any matcher
  added outside `knownMatcherPrefixes`.
- The dry-run output is a de facto interface for humans and scripts; keep
  the `invocations:` arithmetic line stable once released (Plan 022 may
  parse it — better: have Plan 022 read structured data instead).
