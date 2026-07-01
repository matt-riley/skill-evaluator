# Plan 029: Troubleshooting & FAQ — turn failure modes into teaching moments

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- runner.go grader.go cmd_run.go cmd_grade.go workspace.go config.go main.go docs/site/src/utils/routing.ts`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: S-M
- **Risk**: LOW
- **Depends on**: — (pairs well after Plan 028 exists to link into; not required)
- **Category**: documentation
- **Planned at**: commit `1325f07`, 2026-07-01

## Why this matters

The docs teach the happy path only. But this tool sits on top of the two
flakiest substrates in software — shelling out to third-party agent CLIs
and parsing LLM output — so *first-session failure is the normal case*:
agent not on PATH, judge returns prose instead of JSON, outputs saved to
the wrong place, a stale lock from a Ctrl+C'd run, token counts showing 0.
Every one of these has a specific cause and a specific fix visible in the
code, and none is documented. A user who hits "no output files found" on
eval 1 doesn't just lose time — they conclude the *tool* is broken and
leave. A symptom-indexed troubleshooting page converts the tool's known
failure modes into quick recoveries, and the FAQ answers the judgment
questions (cost, judge choice, trust) that block adoption.

## Current state

The failure modes to document, each traced to source (re-verify all at
execution time — this list is the page's skeleton):

| Symptom the user sees | Actual cause (source) |
|---|---|
| `(no output files found)` in grading; everything fails | Agent answered in chat instead of writing files; prompt's `Save outputs to:` line ignored, or agent lacks file-write ability in non-interactive mode (`buildPrompt`, `runner.go:250`; `readOutputContents`, `grader.go:118-155`) |
| `judge error: ...` evidence on every LLM assertion | Judge invocation failed; fail-closed by design (`grader.go:64-73`) |
| `parsing grading output ... no JSON found` | Judge replied with prose/markdown; first-`{` extraction found nothing (`parseGradingOutput`, `grader.go:335-347`) |
| Every run instantly `failed` | Agent binary not installed / not in allowlist — `buildAgentCmd` swaps in `exec.CommandContext(ctx, "false")` with only a Warn log (`runner.go:98-108`) — a genuinely cryptic one |
| `iteration N is still running` from grade | Lock status running; finish with `run --resume` (`cmd_grade.go:49-69`) |
| `iteration N appears stale` | >1h-old running lock; `--force-unlock` (`cmd_run.go:59-80`, `workspace.go:69-79`) |
| `another process is running this iteration` | flock held — second concurrent run (`workspace.go:148-161`) |
| Token counts are 0 | Regex heuristics missed the runtime's output format (`extractTokens`, `runner.go:150-168`); pi is precise, others best-effort |
| Runs time out at exactly 10m | Default `--timeout` (`cmd_run.go:20`) |
| `config ... invalid value` on startup | Schema validation, by design (`validateConfigYAML`, `config.go:85-124`) |
| `evals.json is too large` / `too many evals` | Hard caps (`readEvals`, `runner.go:191-217`: 1 MB / 100 evals / 10 KB fields) |
| `no SKILL.md found in current directory or any parent` | Ran outside a skill dir (`detectSkillDir`, `runner.go:171-187`) |
| `--fix` plateaus without passing | Convergence rule: identical failure text twice (`fixEval`, `runner.go:337-343`) |
| import-agit: `agit not found in PATH` / `no task-like turns found` | (`internal/agit/client.go:31-33`; `cmd_import_agit.go:192-194` incl. the MinPromptLen explanation) |

FAQ questions worth answering (adoption blockers, not covered anywhere):
which judge model should I use and why cheaper-is-fine (read-only task,
`CONTEXT.md` Judge entry); what does a loop cost (arithmetic: evals ×
models × 2 configs runs + judge calls; point at the `--parallel`/warning
behavior `cmd_run.go:141-151`); why do numbers differ between identical
runs (agent + judge nondeterminism — link Plan 028's honesty section);
can I run one eval (`--eval`); can I test without the skill
(`--baseline-only`); is my data sent anywhere (shell-out model — only to
whatever the agent/judge CLIs talk to); how do I start over (delete
workspace dir — safe, regenerable).

Site mechanics: `routing.ts:10-23` ORDERED_PAGES; guides glob picks up
new files automatically.

## Design decisions (read before coding)

1. **One page, two halves**: `docs/guides/troubleshooting.md` —
   "🚑 Troubleshooting & FAQ". Half 1: symptom-indexed table of contents
   (the exact error strings, greppable/searchable) linking to entries;
   each entry: **What you're seeing** (verbatim message) → **What's
   actually happening** (one honest paragraph, source-accurate) → **The
   fix** (commands) → optionally **Avoid it next time**. Half 2: the FAQ,
   grouped Q&A. Symptom-first ordering because users arrive via
   copy-pasted error text.
2. **Verbatim error strings are sacred** — copy each from source at
   execution time (they are the page's search keys and Ctrl-F targets);
   provenance-comment each per Plan 026's convention.
3. **The `exec.CommandContext(ctx, "false")` trap gets special treatment** (top-3
   placement): the symptom is maximally confusing (instant failure, no
   error mentioning the agent) and the fix is trivial (install the CLI /
   check `defaults.agent`). Also file a one-line improvement note in the
   entry: run with `--verbose` to see the Warn. (Improving the error
   itself is a code change — note it as a candidate follow-up in
   maintenance, not scope.)
4. **Write to shipped behavior.** Before writing each entry, check
   `plans/README.md`: if Plan 020 landed, the placeholder/lint entries
   change; if 017/018 landed, baseline/failed-run answers change. No
   forward promises.
5. **Tone**: empathetic-playful ("Don't panic — this one's a classic 🧯"),
   but the *cause* paragraphs are precise and unhedged; users in an error
   state want competence with the warmth, not fluff.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Error-string harvest | `grep -rn 'fmt.Errorf\|Warn(' *.go internal/agit/*.go` | verbatim strings for entries |
| Site build | `cd docs/site && pnpm build && pnpm test` | exit 0 |
| Reproduce top entries | scratch skill dir + deliberately broken config/agent | symptoms match the page |

## Scope

**In scope**:
- New `docs/guides/troubleshooting.md`
- `docs/site/src/utils/routing.ts` (+test): register at the end of the
  guides ordering
- One-line "hit a snag?" pointers from `quick-start.md`,
  `docs/guides/first-eval.md`, and `eval-workflow.md`
- `workspace.md`: move/copy its exit-codes table reference (troubleshooting
  links to it; don't duplicate)

**Out of scope**:
- Do NOT change any error messages or logging in Go code (candidates go
  to maintenance notes).
- Do NOT document failure modes of the agent CLIs themselves beyond
  "not installed / not authenticated → see that tool's docs" one-liners.
- Do NOT write a support/issue-template process page.

## Git workflow

- Branch: `advisor/029-troubleshooting-faq`
- Commit message style: `docs: add symptom-indexed troubleshooting guide and FAQ`
- Do NOT push unless instructed.

## Steps

### Step 1: Harvest and reproduce

Run the error-string grep; collect verbatim messages for every entry in
the Current-state table. Reproduce the top four (no-outputs, judge-prose,
missing-agent, stale-lock) in a scratch skill dir with a stub/broken
setup, confirming symptom text and the fix commands actually recover.

**Verify**: a scratch note pairing each entry with its verbatim string +
source line + tested fix.

### Step 2: Write the troubleshooting half

Entries in user-probability order: no-outputs → missing-agent →
judge-prose/judge-error → lock family (running/stale/flock) → timeout →
token-zero → config validation → evals caps → no-SKILL.md → fix-plateau →
import-agit pair. Symptom index table at top. Apply Design decisions 2/3/5.

**Verify**: every entry's symptom string greps back to source
(`grep -rn "<string>" *.go internal/agit/`); site build.

### Step 3: Write the FAQ half

The questions from Current state, ~8-10 entries, each ≤8 lines, linking
into concepts/how-it-works (Plan 028, if landed) and the cookbook
(Plan 025) instead of re-explaining. Cost arithmetic uses the Plan 025
running example for concreteness.

**Verify**: links resolve; no forward-looking feature promises
(`grep -in "coming soon\|will be\|planned" docs/guides/troubleshooting.md`
→ empty or justified).

### Step 4: Register, cross-link, final checks

`ORDERED_PAGES`: append `{ path: "guides/troubleshooting" }` last among
guides; routing test update if needed; add the three "hit a snag?"
pointers.

```bash
cd docs/site && pnpm build && pnpm test
```

## Test plan

- Site build + vitest green; page in nav.
- Symptom-string grep-back check (Step 2) — this is the page's regression
  guard until errors change; note each in provenance comments.
- Manual: paste three symptom strings into the rendered page's browser
  find — all land on their entries.

## Done criteria

- [ ] Troubleshooting covers all ~14 failure modes with verbatim symptoms, true causes, and tested fixes; top-4 reproduced for real.
- [ ] FAQ answers the cost/judge/trust/reset questions with links, not duplication.
- [ ] Registered in nav; pointers added from the three entry pages.
- [ ] No fictional or forward-promised behavior anywhere on the page.
- [ ] Site build/tests pass; `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- Reproduction reveals behavior *different* from the code reading (e.g.
  the missing-agent path errors differently on some platform) — document
  observed reality and flag the discrepancy.
- The error-string harvest finds user-facing messages so poor they should
  change *before* being enshrined in docs (candidates: the silent `false`
  swap) — list them in the report; documenting-then-changing churns the
  page.
- More than ~3 entries depend on unlanded plans' behavior — pause and
  land those first or scope the entries to shipped truth.

## Maintenance notes

- Every plan that changes an error message must update the matching
  troubleshooting entry (the provenance comments make them findable);
  reviewers should treat message changes without doc updates as
  incomplete.
- Code-improvement candidates surfaced here (better missing-agent error;
  judge-prose retry hint) are cheap goodwill — consider batching them as a
  small future plan.
- As Plans 020 (lint), 022 (gate exit codes), and 029-adjacent features
  land, the FAQ absorbs their "why did X happen" questions — keep the
  symptom half for hard failures only.
