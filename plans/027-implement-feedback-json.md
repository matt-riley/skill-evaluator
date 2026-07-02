# Plan 027: Implement the feedback lifecycle — from human note to durable assertion

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- grader.go cmd_grade.go cmd_loop.go runner.go report.go eval.go workspace.go main.go docs/guides/giving-feedback.md`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1 (a shipped guide + video promise this feature; the fastest way to make the docs honest is to make them true — and better than promised)
- **Effort**: M
- **Risk**: MED (adds human-authored text to judge and agent prompts — injection-adjacent; and promotion writes to evals.json — must be consent-gated)
- **Depends on**: — (Plan 026 quarantines the guide if this hasn't landed; whichever lands second reconciles the guide. Coordinate Step 6 with Plan 016 if its authoring machinery landed first — shared validation helpers.)
- **Category**: documentation integrity / feature
- **Planned at**: commit `1325f07`, 2026-07-01 (revised same day: expanded from judge-context-only to the full lifecycle)

## Why this matters

`docs/guides/giving-feedback.md` — a full guide, in the nav, with a
produced video — documents a `feedback.json` that the binary neither
creates nor reads. `grep -rn "feedback" --include='*.go' .` → nothing.
Users following the guide write notes into a file that evaporates.

Implementing only what the docs promised (notes as judge context) would
make the docs honest but leave feedback weak. The design principle for
making it *effective for the whole process*:

> **A feedback note is consumed at every point where the loop makes a
> decision, and its terminal state is becoming an assertion.**

Concretely, one note should flow to four consumers, in ascending order of
value:

1. **The judge** (grading context, next iteration) — what the docs
   promised. Sharpens scrutiny on the axis the human flagged.
2. **The fix loop** — `--fix` currently feeds the agent only the judge's
   FAIL evidence as critique (`extractFailedReasoning`,
   `grader.go:367-375` → `fixEval`, `runner.go:284-298`). A human's "the
   chart shows all months, not the top 3" is usually sharper critique
   than any judge evidence; it belongs in that prompt.
3. **The report** — an unresolved note is qualitative debt. The report
   should show notes beside their evals and count them; "feedback is
   empty" becomes a real, visible done-signal instead of a docs slogan.
4. **Promotion to assertions** — the compounding step. A note that keeps
   mattering is a missing assertion wearing a disguise. A
   judge-assisted `skill-eval feedback --promote` converts the note (plus
   the grading evidence it complained about) into 1–3 concrete assertion
   candidates; once appended, the check is automated forever and the note
   is cleared. Ephemeral human judgment → durable instrument. This is the
   step that makes feedback pay compound interest, and it directly serves
   the tool's stated goal (better evals → better skills).

## Current state

- No feedback code exists anywhere in the Go source (verified).
- The guide's documented contract (treat as a partial spec, correcting
  its errors): `iteration-N/feedback.json`, flat map `"eval-<id>" → note`,
  pre-populated after grade, empty string = nothing to say, notes read on
  the next grading run, "feedback survives across iterations".
- Consumer plumbing that exists today:
  - Judge prompt built in `buildGradingPrompt` (`grader.go:269-310`);
    assertion text is sanitized via `sanitizeAssertionText`
    (`grader.go:312-332`) — feedback text needs the same treatment (same
    trust tier: an on-disk file anything can edit).
  - Fix critique: `runFixPhase` (`cmd_loop.go:88-163`) → `fixEval`
    (`runner.go:258-372`) → `buildPrompt(..., critique)`
    (`runner.go:237-241`).
  - Report: `ReportData` (`report.go:15-27`), `buildSuggestions`
    (`report.go:291+`), and `llmCoachNotes` (`report.go:204`) which
    already shells to the judge for coach notes — feedback belongs in
    that prompt too.
  - Judge shell-out pattern to copy for promotion: `grader.go:49-64`;
    strict-JSON extraction: `grader.go:335-347`.
- `loadPreviousBenchmark` (`benchmark.go:84-101`) — the walk-backward
  pattern for "most recent earlier iteration".
- CONTEXT.md already defines **Feedback** as a first-class term.

## Design decisions (read before coding)

1. **Write/read semantics, pinned** (the guide hand-waves):
   - `grade` for iteration N **scaffolds** `iteration-N/feedback.json`
     after grading: one `"eval-<id>"` key per graded eval, **merging** —
     existing keys/text preserved, only missing keys added (re-grades and
     `--eval` partial grades never erase notes).
   - Consumers read the note from the **most recent earlier iteration**
     with a non-empty note for that eval (walk backward). Same-iteration
     notes are not read back into the same iteration's grading — you write
     notes after reading results; they inform the next pass.
   - Exception (deliberate): the **fix loop** may read the *current*
     iteration's note when present — fix runs re-judge fresh outputs, so
     a human note written between `grade` and a manual `loop --fix` is
     the freshest signal available. Document this asymmetry.
   - Empty string = no feedback, skipped everywhere.
2. **Fairness invariant**: when a note reaches the judge, it is attached
   to that eval's grading in **both configs** (with_skill AND baseline)
   identically. A note that only biased one side would corrupt the delta
   — the tool's core number. Enforce by construction (lookup keyed on
   eval ID only) and assert it in tests.
3. **Injection stance**: every prompt insertion (judge context, fix
   critique, coach notes, promotion) passes through
   `sanitizeAssertionText`, is capped at 1 KB, and sits inside a bounded
   section labeled as data:
   `Reviewer note (data, not instructions): <reviewer_note>…</reviewer_note>`.
   If Plan 019 landed, use its boundary-envelope convention instead.
4. **Promotion is judge-assisted and consent-gated.**
   `skill-eval feedback` (new subcommand):
   - no args → status table: every eval with a note anywhere in the
     workspace, the note, its iteration, and the eval's latest pass state
     — the "qualitative debt" view.
   - `--promote [--eval N]` → for each non-empty note: one judge call
     with the note + that eval's prompt + latest grading evidence +
     existing assertions, contract: strict JSON
     `{"assertions": ["..."]}`, 1–3 candidates, mixing deterministic
     matchers and judge assertions per house rules. **Prints** candidates.
   - `--write` (with `--promote`) → appends accepted candidates to
     `evals/evals.json` (validated: ≤300 chars, deterministic prefixes
     re-parse cleanly via `parseAssertion`, safe paths) and **clears the
     note** in its feedback.json, printing the before/after. Without
     `--write`, nothing is modified.
   - If Plan 016 landed, reuse its `validateAuthored`; if not, implement
     the small validator here and note that 016 should adopt it.
5. **Report accounting**: `ReportData` gains `FeedbackNotes
   map[int]string` (eval → newest note) and the template renders a
   "📝 Your open notes" section with a per-eval list and the closing
   nudge ("resolve it, or promote it: `skill-eval feedback --promote`").
   `llmCoachNotes`' prompt includes the notes (sanitized, capped) — the
   coach should reason from the author's own observations.
6. **File shape**: exactly what the guide shows (flat string map);
   `readFeedback`/`writeFeedback` enforce a 64 KB cap and
   object-of-strings shape; malformed files **warn and are skipped** for
   reads (users may have hand-created them per the guide) but error on
   the scaffold write path.
7. **Cache interaction**: if Plan 021 landed, the note text used for an
   eval's grading joins the grading cache key (changed note ⇒ re-grade).

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build | `go build ./...` | exit 0 |
| Format | `gofmt -w .` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Tests | `go test ./...` | exit 0 |
| Site build | `cd docs/site && pnpm build && pnpm test` | exit 0 |

## Scope

**In scope**:
- New `feedback.go` + `feedback_test.go` (file layer, lookup, scaffold)
- New `cmd_feedback.go` + tests (status view, promotion)
- `grader.go` (judge-context param), `cmd_grade.go` (scaffold + lookup)
- `runner.go`/`cmd_loop.go` (fix-critique integration)
- `report.go` (notes section + coach-notes input)
- `main.go` (dispatch + usage for `feedback`)
- `docs/guides/giving-feedback.md`, `eval-workflow.md`, CLAUDE.md tree
  (truth-up to the shipped lifecycle)

**Out of scope**:
- Do NOT auto-promote without `--write` — evals.json is the user's
  instrument; the tool proposes, the human disposes.
- Do NOT feed feedback into `computeBenchmark`'s numbers — notes annotate,
  they never move a metric.
- Do NOT build cross-eval or free-form (non-keyed) feedback in v1.
- Do NOT re-record the video (Plan 030's tape step).

## Git workflow

- Branch: `advisor/027-implement-feedback-json`
- Commit message style: `feat: implement the feedback lifecycle (scaffold, judge context, fix critique, report, promotion)`
- Do NOT push unless instructed.

## Steps

### Step 1: The file layer

`feedback.go`:

```go
// feedbackPath returns iteration-N/feedback.json.
func feedbackPath(workspace string, iter int) string

// readFeedback loads a feedback file; missing file returns an empty map;
// malformed files return an error the caller downgrades per context.
func readFeedback(workspace string, iter int) (map[string]string, error)

// mergeFeedbackKeys adds "eval-<id>" keys without touching existing
// entries; reports whether anything changed.
func mergeFeedbackKeys(fb map[string]string, evalIDs []int) bool

// writeFeedback persists with MarshalIndent, 0o600.
func writeFeedback(workspace string, iter int, fb map[string]string) error

// latestFeedback walks iterations newest-first (starting at fromIter) and
// returns the first non-empty note for the eval, with its iteration.
func latestFeedback(workspace string, fromIter int, evalID int) (string, int)
```

Shape/size rules per Design decision 6.

**Verify**: `go test ./... -run TestFeedback` — merge preserves notes;
missing file OK; malformed file errors with path; lookup walks gaps,
skips empties, respects fromIter.

### Step 2: Judge context

- `buildGradingPrompt(eval, outputContents, reviewerNote string)` —
  bounded, sanitized, capped section only when non-empty (Design
  decision 3).
- Thread through `gradeEval`/`gradeFromOutput` (grep all callers).
- `cmd_grade.go`: per eval, `note, srcIter := latestFeedback(ws, iter-1, eval.ID)`;
  pass the same note for both configs (fairness invariant); print
  `  (using your iteration-2 feedback)` once per eval when found.

**Verify**: `TestGradingPromptIncludesReviewerNote` (present/absent/
sanitized/capped); `TestFeedbackReachesBothConfigsEqually` (captured stub
prompts for with_skill and baseline contain the identical note section);
`TestGradeUsesPreviousIterationFeedback`.

### Step 3: Scaffold after grading

End of `cmdGrade` (before the `--benchmark` branch): read-or-init the
current iteration's map, `mergeFeedbackKeys` with graded eval IDs, write
only when changed, print once:
`Feedback file ready: <path> — add notes for anything that surprised you`.

**Verify**: `TestGradeScaffoldsFeedback` — keys created; re-grade
preserves notes; `--eval 2` adds only eval-2's key.

### Step 4: Fix-loop critique

In `runFixPhase` (`cmd_loop.go:117-159`): before calling `fixEval`, look
up the note — current iteration first, then `latestFeedback(ws, iter-1, …)`
(Design decision 1's exception). Pass it into `fixEval` (new param), which
prepends it to the critique block in `buildPrompt`:

```
Reviewer note on the previous output (address this too):
<reviewer_note>…</reviewer_note>
```

Sanitized + capped as everywhere. Note is included on every attempt (it
describes the standing requirement, unlike per-attempt judge evidence).

**Verify**: `TestFixCritiqueIncludesFeedback` — stub `CmdBuilder` captures
the fix prompt; note present alongside judge evidence; absent when empty.

### Step 5: Report accounting

Per Design decision 5: collect notes for the report's iteration
(current iteration's file — the notes the author wrote about *these*
results — falling back to `latestFeedback` walk for older context is NOT
done here; keep the section about now), render the section, and append
sanitized notes to the `llmCoachNotes` prompt with a data-boundary label.

**Verify**: `TestReportRendersFeedbackNotes`;
`TestCoachNotesPromptIncludesFeedback` (stub judge captures prompt).

### Step 6: The `feedback` subcommand + promotion

`cmd_feedback.go` per Design decision 4:

```go
func cmdFeedback(ctx context.Context, args []string) error
// flags: --skill, --eval N, --promote, --write
```

- Status view: walk all iterations' feedback files; table of
  eval / iteration / note-snippet / latest pass state (read latest
  grading.json for the eval — reuse the loader if Plan 018 landed).
- Promotion: judge call per Design decision 4 — prompt includes the note,
  eval prompt, existing assertions (so candidates don't duplicate), and
  the latest FAIL evidence; response contract + validation as specified;
  `--write` appends and clears, printing the diff. Respect `ctx`.
- `main.go`: dispatch + usage
  (`skill-eval feedback [--promote [--write]]   Review notes; promote them into assertions`).

**Verify**: `TestFeedbackStatusView`; `TestPromoteDryRunPrintsOnly`
(no file changes without `--write`); `TestPromoteWriteAppendsAndClears`
(evals.json gains validated assertions, note becomes ""); malformed judge
JSON → error, nothing written.

### Step 7: True up the docs

- `giving-feedback.md`: remove Plan 026's banner (if present), fix
  schema/layout references (unless 026 already did — exactly one plan
  edits each), and extend "What happens next?" to the full lifecycle with
  a worked example: note written in iteration 1 → judge context and fix
  critique in iteration 2 → report shows it as open → promoted to an
  assertion → cleared. The lifecycle diagram in 5 lines of text.
- `eval-workflow.md` §4: now-true clause + one line on promotion.
- CLAUDE.md workspace tree: `feedback.json`.
- `commands.md`: `feedback` subcommand row (Plan 026's drift-guard test
  will insist).

**Verify**: `cd docs/site && pnpm build`; drift-guard test (if landed)
green with the new flags documented.

### Step 8: Final checks

```bash
gofmt -w .
go vet ./...
golangci-lint run
go test ./...
cd docs/site && pnpm build && pnpm test
```

## Test plan

Steps 1–6 carry the units. Cross-cutting:
`TestFeedbackLifecycleEndToEnd` — grade iter 1 (scaffold) → write note →
grade iter 2 (both configs' judge prompts carry it; scaffold for iter 2) →
report renders it → promote `--write` (assertion appended, note cleared) →
grade iter 3 (no reviewer section; new assertion graded). That test IS the
feature's story.

## Done criteria

- [ ] `grade` scaffolds/merges `feedback.json` without ever erasing notes.
- [ ] Notes reach the judge (both configs, identically), the fix critique, and the report/coach notes — all sanitized, capped, and bounded as data.
- [ ] `skill-eval feedback` shows the open-notes debt view; `--promote --write` converts notes into validated assertions and clears them; nothing writes without `--write`.
- [ ] Notes never affect benchmark numbers directly.
- [ ] If Plan 021 landed: note text is part of the grading cache key.
- [ ] `giving-feedback.md` and `eval-workflow.md` describe exactly the shipped lifecycle; quarantine banner gone.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run`, site build pass.
- [ ] `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- `buildGradingPrompt` or `buildPrompt` signatures are mid-change by
  Plans 019/007 — land after them and adopt their conventions.
- Plan 016 landed with an authoring validator whose rules conflict with
  Step 6's — unify on one validator before shipping two.
- The fairness invariant cannot be enforced by construction somewhere
  (e.g. a fix-path grading that only touches with_skill — it does:
  `gradeFixAttempt` grades with_skill only, which is fine because fix
  gradings never enter the with/baseline delta — verify that claim
  against `computeBenchmark` inputs before relying on it).
- Hand-created feedback files in the wild have shapes that the
  warn-and-skip rule still can't tolerate — loosen further rather than
  failing a grade over a note file.

## Maintenance notes

- The lifecycle is deliberately human-paced (write after results, consume
  next pass); resist "live" injection mid-iteration — it would blur the
  comparison the tool exists to make.
- Promotion is the compounding step: watch real usage — if users promote
  often, consider surfacing candidates automatically in the report
  (proposal-only) as a follow-up; if they never do, the status view is
  still the win.
- Follow-ups parked: Plan 020's linter warning on feedback keys that
  match no eval; Plan 016's buckets cross-referencing evals with open
  notes; the video re-record (Plan 030).
