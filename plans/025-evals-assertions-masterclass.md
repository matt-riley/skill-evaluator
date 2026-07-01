# Plan 025: The evals & assertions masterclass — rewrite the core teaching content

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 1325f07..HEAD -- docs/guides/writing-evals.md eval-workflow.md docs/site/src/utils/routing.ts grader.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1 (the user's named biggest win)
- **Effort**: M (writing-heavy, code-light)
- **Risk**: LOW
- **Depends on**: — (Plan 026 fixes factual errors across all docs; this plan owns `writing-evals.md`, so the one factual error inside it is fixed HERE, not twice)
- **Category**: documentation
- **Planned at**: commit `1325f07`, 2026-07-01

## Why this matters

Writing evals and assertions is where users succeed or fail with this tool
— a skill benchmark is only as good as its instrument. The existing
`docs/guides/writing-evals.md` (190 lines) is genuinely good: right ideas
(the Golden Rule, fail-first, negative assertions, the traps), right tone.
What it lacks is **depth of instruction**: it tells you the principles but
never *shows a complete worked example* — no real `grading.json` output, no
before/after of a weak assertion becoming a strong one, no reference
material to copy patterns from while writing your own corpus, and no
guidance on the single hardest sub-skill: **phrasing LLM-judge assertions
so the judge can grade them with evidence**. Users read the guide, nod,
and then write `"The output is good"` anyway.

This plan upgrades the guide from "correct advice" to "teaches by worked
example", and adds a companion **Assertion Cookbook** — a reference page of
copy-adapt patterns organized by skill genre. Tone stays playful; the rule
for every section becomes *show, then tell*: no principle without a
concrete input → assertion → judge-verdict example.

## Current state

- `docs/guides/writing-evals.md` — full read done; structure:
  Golden Rule → skill's promise → anatomy → assertion types → traps →
  fail-first → prompt tips → iterate → checklist. Strengths to preserve
  verbatim where possible: the Golden Rule framing (lines 16-20), the
  traps section (107-133), the checklist (179-190).
- **Factual error at lines 140-147** ("Make Your Evals Fail First"):

```
2. Run the **baseline** (no skill) and watch them fail beautifully.
   ```bash
   # The run command will execute your evals
   skill-eval run --baseline previous
```

  `--baseline previous` does NOT mean "run without the skill" — it selects
  the previous-iteration snapshot as the baseline *config* (and per
  Plan 017 is currently buggy anyway). The correct command for "run only
  the no-skill baseline" is `skill-eval run --baseline-only`
  (`cmd_run.go:15`, documented in `commands.md` §`--baseline-only`).
- The guide never shows what the **judge actually sees or returns**. The
  real contract (`grader.go:294-308`): strict JSON, `passed` bool,
  `evidence` string, and the grading principles "Require concrete evidence
  for PASS", "right label but wrong substance = FAIL", "Evidence must
  reference specific content from the output files". These principles ARE
  the assertion-phrasing guidance — an assertion the judge can't quote
  evidence for is a bad assertion — and the docs never connect them.
- Matcher syntax subtleties undocumented: prefix parsing is exact
  (`grader.go:163-197`) — a typo'd prefix silently becomes an LLM
  assertion (until Plan 020 lints it); `contains_text:` splits on the
  FIRST colon after the prefix (`SplitN(rest, ":", 2)`), so filenames with
  colons break and regex args in `matches_text:` may contain colons
  safely; paths are output-dir-relative and traversal is rejected
  (`grader.go:214-219`).
- `docs/site/src/utils/routing.ts:10-23` — `ORDERED_PAGES` controls nav
  order; new pages must be registered there or they sort arbitrarily.
- Site build: `cd docs/site && pnpm build && pnpm test`.

## Design decisions (read before coding)

1. **Two pages, one voice.** `writing-evals.md` stays the *narrative*
   ("how to think"); new `docs/guides/assertion-cookbook.md` is the
   *reference* ("what to write"). The narrative links into cookbook
   sections at every principle. Both keep the emoji/exclamation house
   style — the tone rule is: every playful claim is immediately followed
   by a concrete artifact (JSON, command output, judge verdict).
2. **One running example corpus, used everywhere.** Pick the Sinon→Jest
   migration skill already seeded in the guide (line 39) and carry it
   through BOTH pages: its `evals.json`, one full `grading.json` verdict
   (hand-authored but schema-exact: `assertion_results[].text/passed/evidence`,
   `summary.passed/failed/total/pass_rate` — verify field names against
   `eval.go:62-96` before writing), and one before/after benchmark delta.
   Readers should be able to reconstruct the whole flow from the docs alone.
3. **The cookbook is organized by skill genre**, because that's how
   authors search: code-generation/migration skills, document/writing
   skills, data-analysis/chart skills, workflow/process skills. Each genre
   gets: 2 deterministic patterns, 2 judge patterns, 1 negative pattern,
   1 anti-pattern with its failure mode named.
4. **Judge-assertion phrasing gets its own cookbook section** — the
   highest-value teaching in the plan. The rules, each with a ❌/✅ pair
   graded against the running example:
   - *Quotable*: the judge must be able to quote evidence — assert about
     observable content, not intent ("explains WHY the migration needs
     `restoreAllMocks`, mentioning test pollution" not "explanation is good").
   - *Single-verdict*: one claim per assertion; "X and Y" hides which half
     failed.
   - *Self-contained*: the judge sees prompt + expected output + files —
     never reference "the skill" or "as described above".
   - *Calibrated*: falsifiable by a lazy-but-plausible output (the
     baseline should be able to fail it).
5. **Fail-first section is rewritten around `--baseline-only`** (fixing
   the error) and extended with "how to read the failure": show the
   grading.json from a baseline flop and point at which evidence lines
   prove the eval discriminates.
6. **Cross-references, not duplication**: matcher table stays canonical in
   `eval-workflow.md`; the cookbook links to it and documents only the
   *gotchas* (first-colon split, exact prefixes, relative paths).

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Site build | `cd docs/site && pnpm install && pnpm build` | exit 0 |
| Site tests | `cd docs/site && pnpm test` | pass |
| Link sanity | grep the new pages for `](/` targets and confirm each slug exists | all resolve |
| Schema truth | compare every JSON example against `eval.go` structs | field names match |

## Scope

**In scope**:
- `docs/guides/writing-evals.md` (rewrite/expand, ~250-300 lines)
- New `docs/guides/assertion-cookbook.md` (~250 lines)
- `docs/site/src/utils/routing.ts` (`ORDERED_PAGES`: add
  `guides/assertion-cookbook` directly after `guides/writing-evals`)
- Small cross-link touch-ups in `eval-workflow.md` and
  `docs/guides/first-eval.md` ("go deeper" pointers only)

**Out of scope**:
- Do NOT fix factual issues in other pages (Plan 026 owns those; the
  `--baseline previous` error is in-scope here only because it lives in
  `writing-evals.md`).
- Do NOT document unshipped features (activation evals, transcript
  matchers, `--runs`); each feature plan documents itself. If those have
  landed by execution time, ADD their patterns to the cookbook (check
  `plans/README.md` status column first).
- Do NOT change the matcher reference table's home (`eval-workflow.md`).
- Do NOT add videos in this plan (tape production is Plan 030's concern).

## Git workflow

- Branch: `advisor/025-evals-assertions-masterclass`
- Commit message style: `docs: expand writing-evals into worked-example teaching and add assertion cookbook`
- Do NOT push unless instructed.

## Steps

### Step 1: Build the running example artifacts

Before writing prose, author the example corpus in a scratch area and get
it schema-exact:
- `evals.json` for the Sinon→Jest skill: 3 evals (happy path, edge case —
  a sandbox with nested `sinon.stub` in `beforeEach`, negative case — a
  file that should NOT be migrated), 4-6 assertions each mixing matchers
  and judge assertions per the cookbook rules.
- One realistic `grading.json`: a baseline run where 2 assertions fail
  with specific evidence, verified field-by-field against
  `GradingFile`/`AssertionResult` (`eval.go:62-96`).
- The 6-line benchmark excerpt showing the with-skill vs baseline delta.

**Verify**: paste each JSON through `python3 -m json.tool` (or `jq .`);
cross-check every key against the Go structs.

### Step 2: Rewrite `writing-evals.md`

Keep the existing skeleton and voice; per section, add the worked-example
layer:
- **Golden Rule** → keep, then add the discrimination framing: an eval's
  value is `baseline-fail ∧ with-skill-pass`; show the four
  pass/fail combinations as a 2×2 table with what each means (this
  quietly pre-teaches Plan 016's report buckets).
- **Anatomy** → replace the assertion bullet list with the full eval-1
  JSON from Step 1 and a line-by-line "why each assertion earns its place".
- **NEW: "What the judge actually does"** → show an excerpt of the real
  grading contract (JSON shape + the three grading principles quoted from
  `grader.go:304-307`), then the eval-1 baseline `grading.json` with a
  guided read of the `evidence` fields. This is the bridge to the
  phrasing rules.
- **Assertion types** → tighten and link each type to its cookbook section.
- **Fail-first** → rewrite per Design decision 5 (`--baseline-only`).
- **Traps & checklist** → keep, add one new trap: "The Unquotable
  Assertion" (judge can't cite evidence → coin-flip verdicts), and add two
  checklist items: "Could the judge quote evidence for every LLM
  assertion?" and "Does each assertion make exactly one claim?".

**Verify**: `cd docs/site && pnpm build` passes; page renders with all
JSON blocks highlighted; no `--baseline previous` remains in the file
(`grep -n "baseline previous" docs/guides/writing-evals.md` → empty).

### Step 3: Write `docs/guides/assertion-cookbook.md`

Frontmatter (`title: The Assertion Cookbook`,
`description: Copy-adapt assertion patterns for every kind of skill — deterministic matchers, LLM-judge phrasing rules, negative assertions, and the anti-patterns to avoid.`).
Structure:
1. **How to use this page** (3 lines, playful).
2. **Matcher syntax gotchas** — exact prefixes, first-colon split,
   relative paths only, regex colons OK; link to `eval-workflow.md` table.
3. **Judge-assertion phrasing rules** — the four rules (Design decision 4)
   each with ❌/✅ pair + the evidence the judge produced for the ✅ in the
   running example.
4. **Patterns by skill genre** — the four genres (Design decision 3),
   ~6 patterns each in a compact recurring format:

   ```
   **Pattern: old API fully removed** 🧹
   `matches_text: <file>:^(?!.*require\('sinon'\))` — nope, regex lookahead
   isn't Go RE2! Use the judge instead:
   "The output file contains no import or require of sinon."
   Why it works: negative, single-claim, quotable (judge cites the import block).
   ```

   (Note the RE2 teaching moment — Go regexp has no lookahead; the
   cookbook must only show patterns that actually work in RE2. Test any
   nontrivial regex in a Go scratch test before publishing.)
5. **Anti-pattern gallery** — 5 entries: the vague vibe check, the
   double-claim, the brittle exact-string, the always-true, the
   skill-referencing assertion; each with its observed failure mode and
   the repair.
6. **Sizing guidance** — 3-7 assertions/eval, ≥1 deterministic, ≥1
   negative, 1-2 judge max; what happens to signal beyond that (links to
   reading-results).

**Verify**: every regex in the page compiles under Go RE2 (scratch
`regexp.MustCompile` test or `go run` snippet); site build green.

### Step 4: Register nav + cross-links

- `routing.ts` `ORDERED_PAGES`: insert
  `{ path: "guides/assertion-cookbook" }` after `guides/writing-evals`.
- `eval-workflow.md` §2 (assertions): add one pointer line to the cookbook.
- `first-eval.md` step with assertions: same one-liner.
- Update `docs/site/tests/routing.test.ts` if it asserts the ordered list
  (read it first).

**Verify**: `cd docs/site && pnpm build && pnpm test` — nav shows the new
page in position; tests green.

### Step 5: Final review pass

Read both pages start-to-finish once aloud-in-your-head for tone drift:
the bar is "a friendly senior engineer teaching, with emoji" — not
marketing copy. Kill any sentence that makes a claim without an artifact
within one screen of it.

## Test plan

- Site build + existing vitest suite green.
- `grep -rn "baseline previous" docs/guides/writing-evals.md` → no matches.
- All JSON examples parse; all field names verified against `eval.go`.
- All regexes RE2-valid.
- Every internal link target exists as a rendered slug.

## Done criteria

- [ ] `writing-evals.md` teaches with one complete worked corpus, includes the real judge contract and a guided `grading.json` read, and the fail-first section uses `--baseline-only`.
- [ ] `assertion-cookbook.md` ships with the four phrasing rules, four genre pattern sets, the anti-pattern gallery, and matcher gotchas — all examples runnable/valid.
- [ ] New page registered in `ORDERED_PAGES`; site build and tests pass.
- [ ] Tone matches the existing guides (checked against `giving-feedback.md`/`auto-fixing.md` as tone references).
- [ ] `plans/README.md` row updated to DONE.

## STOP conditions

Stop and report if:
- The `grading.json` schema in `eval.go` has drifted from what this plan
  quotes (field names in examples must be generated from current structs,
  never from memory).
- Plan 026 has already rewritten `writing-evals.md`'s fail-first section
  (check its status) — coordinate to avoid conflicting edits.
- The site's markdown pipeline can't render something you need (e.g.
  the 2×2 table with inline code) — simplify the presentation, don't add
  site tooling in this plan.

## Maintenance notes

- The cookbook is append-friendly by design: every future matcher
  (Plan 012's transcript pair, a future `command_succeeds:`) adds a
  syntax-gotchas row and one pattern per relevant genre — reviewers should
  request cookbook entries in the same PR as new matchers.
- The running example is load-bearing across two pages; if it ever
  changes, change it in both (grep for `sinon-sandbox`).
- Plan 030's tutorial should reuse this example corpus rather than invent
  a second one.
