# Plan 030: A real example skill + zero-to-benchmark tutorial (and a learning-path nav)

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- docs/guides/first-eval.md docs/site/src/utils/routing.ts docs/site/src/components/NavTree.astro cmd_init.go .github/workflows/ci.yml`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3 (highest-effort, capstone — do after 025/026 so it teaches corrected material)
- **Effort**: M-L
- **Risk**: MED (a committed example corpus is a maintenance liability unless CI validates it — mitigated below)
- **Depends on**: plans/025-evals-assertions-masterclass.md (reuses its running example), plans/026-docs-truth-reconciliation.md (tutorial must teach corrected semantics)
- **Category**: documentation
- **Planned at**: commit `1325f07`, 2026-07-01

## Why this matters

Every guide teaches with JSON *fragments* the reader can't run. There is
no complete, executable artifact anywhere in the repo: no example
`SKILL.md`, no example corpus with fixtures, no end-to-end narrative that
starts at an empty directory and ends at a benchmark delta the reader can
reproduce and *compare against their own*. For a tool whose entire value
proposition is "eval-driven iteration", the docs never actually
demonstrate one full iteration with real artifacts.

A shipped `examples/` skill closes three gaps at once: it is the
tutorial's subject, it is a copyable starting template (better than
`init`'s placeholder), and — validated in CI — it is a living integration
test of the documented workflow. The tutorial then becomes the site's
"start here" spine, which also motivates the small nav upgrade: ordered
pages exist (`ORDERED_PAGES`) but visitors get no guidance on reading
order or level.

## Current state

- No `examples/` directory exists in the repo.
- `docs/guides/first-eval.md` (111 lines) is the closest thing to a
  tutorial: good tone, but its `csv-analyzer` example is fragmentary (no
  SKILL.md content, no fixture file, no real output shown) and it stops at
  "run loop" without reading results or iterating.
- Plan 025 establishes the Sinon→Jest running example with a full
  `evals.json`, assertions, and a hand-authored `grading.json` — this plan
  makes those artifacts real files and (where feasible) real outputs.
- `cmd_init.go:40-62` — the scaffold writes placeholder text; a
  `--example` variant could copy the example corpus instead (small code
  touch, optional Step 6).
- Nav: `NavTree.astro` renders three groups (Documentation 📚 /
  guides / ADRs) from `buildNavLinks`; ordering via `ORDERED_PAGES`
  (`routing.ts:10-23`). No "learning path" concept.
- Videos: `docs/guides/tapes/*.tape` (VHS) with rendered mp4s in
  `docs/site/public/guides/`; `feedback-loop.mp4` predates Plan 027's
  implementation (its plan flags the re-record; do it here if 027 landed).
- CI: `.github/workflows/ci.yml` — where the example-validation job goes
  (read it first to match conventions).

## Design decisions (read before coding)

1. **The example skill**: `examples/jest-migration/` — the Plan 025
   Sinon→Jest skill, chosen because (a) the docs already teach with it,
   (b) it is agent-agnostic and offline-checkable (fixtures are plain JS
   files), (c) its assertions exercise all matcher types. Contents:
   `SKILL.md` (a real, spec-valid skill — name matches dir, description
   with trigger phrasing, ~60 focused lines), `evals/evals.json` (3 evals
   from Plan 025), `evals/files/` fixtures (the `.spec.js` inputs),
   `.skill-eval.yaml` (commented, showing judge/model config), and a
   `README.md` pointing at the tutorial.
2. **CI validates structure, not agent runs.** A CI job must not invoke
   real agents/judges (cost, keys, flake). It validates: `evals.json`
   parses via the real loader (a Go test in `examples_test.go` calling
   `readEvals("examples/jest-migration")`), fixtures referenced by
   `files`/prompts exist, and — if Plans 009/020 landed — `skill-eval
   validate` passes on the example. The *executed* loop is done manually
   at authoring time to produce the tutorial's real outputs.
3. **The tutorial**: new `docs/guides/tutorial.md` — "🚀 Zero to
   Benchmark" — replaces nothing; `first-eval.md` stays as the quick
   version with a banner cross-link ("want the full guided tour with a
   real skill? → Tutorial"). Structure: clone/copy the example → read its
   SKILL.md and evals (annotated tour) → `--dry-run`-or-run → baseline-only
   fail-first moment (real grading.json excerpt, from an actual authoring
   run) → full loop → reading the real benchmark numbers → one iteration:
   a deliberate skill weakness is fixed (the example SKILL.md ships with
   one documented soft spot, e.g. missing guidance on `restoreAllMocks`)
   → re-loop → delta improves. Every artifact shown is a real one produced
   during authoring, trimmed and provenance-commented.
4. **Learning path, minimally.** No new nav machinery: (a) `ORDERED_PAGES`
   gets the tutorial right after `quick-start`; (b) `docs/index.md` gains
   a short "Where do I start? 🗺️" section: three tracks (New here →
   tutorial; Writing evals → writing-evals + cookbook; Something broke →
   troubleshooting); (c) each guide gets a consistent one-line "**Next
   up:**" footer completing the chain (several already have "What's
   next" sections — normalize, don't duplicate). That is the whole IA
   change — resist a sidebar redesign.
5. **Numbers honesty**: the tutorial's benchmark numbers are labeled as
   "from our authoring run — yours will differ, and that's normal
   (here's why → how-it-works honesty section)". No fabricated precision.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Example loads | `go test ./... -run TestExampleSkill` | exit 0 |
| Authoring run | `cd examples/jest-migration && skill-eval loop -y` | real artifacts for the tutorial |
| Site build | `cd docs/site && pnpm build && pnpm test` | exit 0 |
| Spec check (if landed) | `skill-eval validate --skill examples/jest-migration` | exit 0 |
| Tape render (optional) | `vhs docs/guides/tapes/tutorial.tape` | mp4 produced |

## Scope

**In scope**:
- New `examples/jest-migration/` (SKILL.md, evals.json, fixtures, .skill-eval.yaml, README)
- New `examples_test.go` (structure validation via real loaders) + CI job hookup
- New `docs/guides/tutorial.md`; banner + footer touch-ups in
  `first-eval.md` and the guide chain; "Where do I start?" in `docs/index.md`
- `routing.ts` (+test): register `guides/tutorial`
- Optional Step 6: `init --example` copying the example corpus
- Optional: `tutorial.tape` + mp4; re-record `feedback-loop.mp4` if Plan 027 landed

**Out of scope**:
- Do NOT run agents/judges in CI.
- Do NOT create multiple example skills (one excellent one; a gallery is a
  follow-up).
- Do NOT redesign NavTree/sidebar beyond the ordering insert.
- Do NOT gate this plan on video tooling — tapes are the explicitly
  optional last step; the plan is DONE without them if VHS isn't available.

## Git workflow

- Branch: `advisor/030-tutorial-example-skill`
- Commit message style: `docs: add jest-migration example skill and zero-to-benchmark tutorial`
- Do NOT push unless instructed.

## Steps

### Step 1: Author the example skill

Build `examples/jest-migration/` per Design decision 1. The SKILL.md must
be genuinely good (it will be copied): spec-valid frontmatter, the
migration rules as crisp instructions, one deliberate documented soft spot
for the tutorial's iteration arc (mark it with an HTML comment for
maintainers, invisible to agents reading rendered markdown? No — SKILL.md
is read raw by agents; mark the soft spot only in the tutorial text, not
the skill file). Fixtures: 3 small `.spec.js` files matching Plan 025's
eval corpus.

**Verify**: `readEvals` loads it (quick scratch test); if Plan 009 landed,
`validate` passes; fixture paths in prompts and `files` agree (Plan 026's
rule).

### Step 2: Structure test + CI

`examples_test.go` (package main): `TestExampleSkillLoads` — `readEvals`
on the example dir; every `files` entry exists; every eval has ≥1
assertion; `parseAssertion` on each deterministic-looking assertion
returns a non-LLM type (catches prefix typos in our own example —
eating the Plan 020 dogfood). Add the test to CI implicitly via
`go test ./...` (confirm ci.yml runs it; no new job needed if so).

**Verify**: `go test ./... -run TestExampleSkill` green; deliberately
break a fixture path locally → test fails naming it → revert.

### Step 3: The authoring run

With a real agent + judge configured, run the loop twice: once with the
shipped (soft-spot) SKILL.md, once after applying the tutorial's fix.
Harvest and trim: one baseline `grading.json` (failing), one with-skill
`grading.json`, both `benchmark.json` excerpts, and the console output of
`loop`. These are the tutorial's real artifacts. Keep raw copies in the
plan-execution notes (not the repo).

**Verify**: the delta story the tutorial tells (baseline fails discriminating
assertions → with-skill passes → post-fix iteration improves further) is
what actually happened; if not, adjust the soft spot until the arc is
real — never the numbers.

### Step 4: Write `docs/guides/tutorial.md`

Per Design decision 3, in the house tone, ~300 lines, every artifact
real and provenance-commented, every concept linking out (cookbook,
how-it-works, troubleshooting) instead of re-explaining. End with "where
to go next" (the three tracks).

**Verify**: site build; a fresh reader can execute every command block in
order against the example dir (dry-run the doc yourself, top to bottom).

### Step 5: Learning path wiring

`docs/index.md` "Where do I start?" section; `ORDERED_PAGES` insert;
`first-eval.md` banner; normalize the "Next up:" footers across guides
(tutorial → writing-evals → cookbook → reading-results → giving-feedback →
auto-fixing → cross-model → importing-agit → troubleshooting).

**Verify**: `pnpm build && pnpm test`; click the full chain once in
`pnpm dev`.

### Step 6 (optional, small): `init --example`

In `cmd_init.go`: `--example` flag — instead of the placeholder skeleton,
copy the embedded example evals (go:embed the example's evals.json +
fixtures; keep SKILL.md out — users have their own skill). Print a pointer
to the tutorial. Skip this step entirely if embedding raises size/layout
questions — note the decision in the report.

**Verify**: `go run . init --example` in a scratch skill dir produces the
corpus; `go test ./...` green.

### Step 7: Tapes (optional) + final checks

If VHS is available: `tutorial.tape` recording the Step-4 flow;
re-record `feedback-loop.mp4` against Plan 027's shipped behavior if
landed. Then:

```bash
go test ./...
cd docs/site && pnpm build && pnpm test
```

## Test plan

- `TestExampleSkillLoads` (+ deliberate-break spot check) — the example's
  permanent regression guard.
- Site build/tests; full-chain click-through.
- The tutorial's command blocks executed verbatim by the author against a
  clean checkout (the real acceptance test).

## Done criteria

- [ ] `examples/jest-migration/` ships spec-valid, loads via the real loaders, and is CI-guarded.
- [ ] The tutorial walks empty-dir → benchmark → one real improvement iteration, with only real artifacts.
- [ ] `first-eval.md` cross-links; index has the three-track "Where do I start?"; guide footers form a complete chain.
- [ ] No agent/judge invocation added to CI.
- [ ] Site build/tests and `go test ./...` pass; `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- The authoring run (Step 3) cannot produce the improvement arc honestly
  after two soft-spot adjustments — the example skill choice may be wrong;
  report with the observed gradings rather than massaging the narrative.
- Committing fixtures trips any license/content concern (they must be
  written fresh for this repo, not copied from real projects).
- `readEvals`'s caps or `validate`'s rules (Plans 009/020) reject the
  example — fix the example, and if the rules are wrong, report to those
  plans; never loosen a rule to admit the example silently.

## Maintenance notes

- The example is now load-bearing docs: any change to eval schema,
  matchers, or init flow must keep `TestExampleSkillLoads` green — that
  test failing is the docs telling you they just went stale.
- When new matcher types land (Plan 012), add one to the example corpus
  and one beat to the tutorial — small, in the same PR.
- If a second example ever ships (a doc-writing skill would contrast
  nicely), promote `examples/` structure conventions into a short
  `examples/README.md` first.
