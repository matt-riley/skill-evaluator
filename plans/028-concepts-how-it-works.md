# Plan 028: Concepts section — publish the mental model and the pipeline internals

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- CONTEXT.md docs/site/src/utils/routing.ts runner.go grader.go benchmark.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M (writing-heavy)
- **Risk**: LOW
- **Depends on**: — (benefits from Plan 026 landing first so it builds on corrected pages; not required)
- **Category**: documentation
- **Planned at**: commit `1325f07`, 2026-07-01

## Why this matters

The docs teach *workflow* (do this, then this) but never *model* (what is
actually happening and why should you trust the numbers). Two concrete
gaps:

1. **The glossary is hidden.** `CONTEXT.md` — 91 lines of exactly the
   canonical, on-tone terminology a newcomer needs (Skill, Eval, Assertion,
   Run, Grading, Benchmark, Iteration, Judge, Baseline…) — is deliberately
   excluded from the site (`routing.ts:81`, `EXCLUDED_DOCS` includes
   `context.md`, presumably because it reads as contributor guidance).
   Site visitors never see the vocabulary the rest of the docs assume.
2. **The pipeline is a black box.** Nothing on the site answers: what
   prompt does my agent actually receive? What exactly does the judge see
   and what is it told? Where do the pass rate, stddev, and delta numbers
   come from, and what are their honest limitations (single run per eval,
   cross-eval stddev, judge nondeterminism)? Educated users write better
   evals *because* they know the judge only sees output files, that the
   prompt appends "Save outputs to: <dir>", and that a delta from one run
   per eval is noisy. Transparency here is a direct multiplier on
   Plan 025's teaching.

## Current state

- `CONTEXT.md` — the glossary source; contributor-facing framing
  ("Let's avoid: …" terminology-discipline notes) mixed with
  visitor-useful definitions.
- The facts a "how it works" page must teach, with their sources (write
  the page FROM these, verifying each at execution time):
  - Agent invocation: shell-out, `cmd.Dir` = skill dir, prompt structure
    from `buildPrompt` (`runner.go:233-253`) — non-interactive preamble,
    optional skill path line, task, input-files line, output-dir line.
  - Two runs per eval: with_skill vs baseline; baseline = no skill path or
    a snapshot path (`resolveSkillPath`, `runner.go:80-88`).
  - Grading: deterministic matchers run locally
    (`grader.go:34-42, 222-263`); everything else goes to one judge call
    whose prompt contains task + expected output + output-file contents +
    marked assertions + the strict-JSON contract and grading principles
    (`buildGradingPrompt`, `grader.go:269-310`); judge failure = all its
    assertions fail closed (`grader.go:64-73`).
  - What the judge does NOT see (today): the agent's transcript, tool
    calls, timing, the skill itself — a crucial eval-design constraint.
  - Benchmarks: per-config mean/stddev across evals
    (`aggregateRuns`, `benchmark.go:193-215`), delta = with_skill −
    baseline means, cross-model aggregation and best/worst
    (`benchmark.go:29-57`), iteration deltas (`:59-69`).
  - Honest caveats: one run per eval (until Plan 011), stddev is across
    evals not repeats, judge verdicts vary, failed-run accounting
    (until Plan 018 — check status and write to the shipped truth).
- `routing.ts:10-23` — `ORDERED_PAGES`; new pages need registration.
- Tone reference: `eval-workflow.md`, CONTEXT.md itself.

## Design decisions (read before coding)

1. **Two new site pages**, registered between `eval-workflow` and
   `commands` in the nav (they are foundation, not guides):
   - `concepts.md` (root) — "🧠 Concepts & Vocabulary": the visitor-facing
     glossary distilled from CONTEXT.md (definitions kept nearly verbatim
     — they're good — minus the "Let's avoid" contributor notes, which
     stay in CONTEXT.md), plus a 10-line "the loop at a glance" ASCII/
     mermaid-free diagram (plain pre block: evals.json → run ×2 → outputs
     → grade → grading.json → benchmark → report) since the site pipeline
     is plain markdown.
   - `how-it-works.md` (root) — "🔬 How skill-eval Works": the pipeline
     internals per the fact list above, each section ending with a
     **"Why you care"** call-out linking the internal to an eval-design
     consequence (e.g. "the judge only sees output files → assert about
     files, and tell the agent exactly where to save them").
2. **CONTEXT.md is not published**; it gains a one-line pointer to
   `concepts.md` so the two never drift silently, and `concepts.md` notes
   it derives from CONTEXT.md. Single-source purists would sync-generate;
   with 15 definitions, a cross-pointer + review discipline is enough.
3. **Show real prompts.** `how-it-works.md` includes a verbatim example of
   (a) the agent prompt `buildPrompt` produces for the Plan 025 running
   example, and (b) an abridged judge prompt — both marked with the
   provenance comment convention from Plan 026, generated by actually
   calling the functions in a scratch test (not hand-typed).
4. **Caveats are content, not fine print.** The "reading the numbers
   honestly" section gets equal visual weight (call-out boxes): what a
   +0.15 delta from 5 evals × 1 run does and doesn't mean; when to re-run;
   pointer to `--runs` when Plan 011 lands (write to shipped state).
5. **Tone**: same playful register; the two pages may use slightly fewer
   exclamation marks than the guides — they are the "senior engineer
   explains over coffee" pages; still emoji-titled, still warm.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Prompt provenance | scratch Go test printing `buildPrompt`/`buildGradingPrompt` output | pasted verbatim into docs |
| Site build | `cd docs/site && pnpm build && pnpm test` | exit 0 |
| Go tests | `go test ./...` | exit 0 (scratch test not committed, or committed as example test — decide in Step 3) |

## Scope

**In scope**:
- New root `concepts.md` and `how-it-works.md`
- `CONTEXT.md` (pointer line only)
- `docs/site/src/utils/routing.ts` (+test) — register both pages
- Cross-link touch-ups: `docs/index.md` (add the two pages to the flow),
  `eval-workflow.md` and `docs/guides/writing-evals.md` gain one "under
  the hood" pointer each

**Out of scope**:
- Do NOT publish CONTEXT.md/CLAUDE.md/AGENTS.md (EXCLUDED_DOCS stands).
- Do NOT document internals that plans are about to change without
  checking `plans/README.md` status first — write to shipped behavior
  only, with no "coming soon" promises (Plan 026's lesson).
- Do NOT add diagrams requiring new site tooling (mermaid, images) —
  plain pre-block diagrams only in this plan.
- Do NOT duplicate the matcher reference or cookbook content — link.

## Git workflow

- Branch: `advisor/028-concepts-how-it-works`
- Commit message style: `docs: add concepts glossary and how-it-works pipeline internals pages`
- Do NOT push unless instructed.

## Steps

### Step 1: `concepts.md`

Frontmatter (`title: Concepts & Vocabulary`, description per house
style). Content: intro (why shared words matter, 3 lines) → the loop
diagram → the glossary (Skill, Eval, Assertion, Run, Grading, Benchmark,
Iteration, Workspace, Baseline, Snapshot, Feedback, Judge, Agent Runtime,
Global/Skill Config — definitions adapted from CONTEXT.md, each with a
"see it in action" link to the page that uses it). Add the pointer line
to CONTEXT.md.

**Verify**: site build; every glossary link resolves.

### Step 2: `how-it-works.md`

Sections per Design decision 1, in pipeline order: **The run** (prompt
anatomy, two configs, where files land) → **The grade** (matcher pass,
judge prompt anatomy, fail-closed rule, what the judge can't see) →
**The benchmark** (the actual formulas in words, cross-model, iteration
delta) → **Reading the numbers honestly** (Design decision 4) → **Design
consequences recap** (the collected "why you care" bullets). Each factual
claim carries the provenance comment.

**Verify**: site build; every claim traced to the Step-1-of-current-state
source list against live code (re-grep each cited line range).

### Step 3: Real prompt excerpts

Write a scratch test (or temporary main) that constructs the Plan 025
running-example `Eval` and prints `buildPrompt(...)` and
`buildGradingPrompt(...)` output; paste verbatim (trim the output-dir
absolute path to `<workspace>/.../outputs`). Decide: commit it as
`ExampleBuildPrompt` Go example test (self-maintaining — it breaks when
the prompt changes, flagging the doc) — preferred if output stability
allows; otherwise scratch-only with provenance comments.

**Verify**: `go test ./...` (if example test committed, it passes and its
output matches the doc excerpt).

### Step 4: Registration + cross-links + final checks

`ORDERED_PAGES`: insert `{ path: "concepts" }` and
`{ path: "how-it-works" }` after `eval-workflow`; update `routing.test.ts`
if it snapshots order; add the three cross-link one-liners.

```bash
go test ./...
cd docs/site && pnpm build && pnpm test
```

## Test plan

- Site build + vitest green; nav shows both pages in position.
- If the `Example` test route was taken: `go test ./...` locks prompt
  excerpts to reality.
- Manual `pnpm dev` read-through of both pages for tone and link integrity.

## Done criteria

- [ ] `concepts.md` gives visitors the full vocabulary + loop diagram; CONTEXT.md cross-pointer in place.
- [ ] `how-it-works.md` documents the real prompts, the judge's exact inputs and fail-closed rule, benchmark math in words, and honest statistical caveats — every claim provenance-marked.
- [ ] Each internals section ends with an eval-design consequence.
- [ ] Both pages registered in nav; site build/tests and `go test ./...` pass.
- [ ] `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- The cited internals have materially changed (Plans 007/011/012/017/018/
  019 landing reshapes run dirs, prompts, or math) — re-derive the fact
  list from live code; the page structure survives, the claims must be
  regenerated.
- The Example-test approach makes prompts part of the public API in a way
  that blocks legitimate prompt evolution — fall back to provenance
  comments and note the tradeoff.
- `concepts.md` and CONTEXT.md definitions need to diverge (visitor vs
  contributor needs) beyond dropping the "Let's avoid" notes — that's a
  signal to restructure CONTEXT.md itself; report rather than fork
  meaning.

## Maintenance notes

- These two pages are where every future feature plan should add its
  "under the hood" paragraph (015's process axis, 013's activation
  judging) — reviewers should ask "does how-it-works cover this?" on
  feature PRs.
- The honest-caveats section shrinks as Plans 011/018/019 land — updating
  it is part of those plans' doc steps; leave HTML comments marking the
  spots (`<!-- update when --runs ships -->`).
